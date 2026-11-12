import {expect, test} from '@playwright/test'
import type {ScopeInput} from '../../src/lib/api'
import {json, mockOperationalWorkflow} from './fixtures-operational'
import {resolveScopeFixture} from './fixtures-analyst-scope'
import {mockSession} from './session'

const scope: ScopeInput = {
  area: {kind: 'drawn', name: 'Al Ain reference area', geometry: {
    type: 'Polygon', coordinates: [[[55.73,24.2],[55.75,24.2],[55.75,24.22],[55.73,24.22],[55.73,24.2]]],
  }},
  temporal: {mode:'historical', revisionView:'latest', timezone:'Asia/Dubai', backgroundImagery:false,
    interval:{from:'2025-06-01T00:00:00Z',to:'2025-07-01T00:00:00Z'}},
}

for (const size of [{width:390,height:844},{width:740,height:360}]) {
  test(`environment library saves the selected area and presents a staged result at ${size.width}×${size.height}`, async ({page}, info) => {
    test.skip(info.project.name !== 'no-token', 'One browser workflow per viewport')
    await page.setViewportSize(size)
    await mockOperationalWorkflow(page)
    await page.route('**/api/v1/auth/me', route => {
      const session = mockSession({'x-emad-partition':'PUBLIC_EVIDENCE','x-emad-run-id':''})
      return json(route,{...session,productProfile:'map',permissions:[...session.permissions,
        'environments.read','environments.manage','environments.build','environments.qualify']})
    })
    await page.route('**/api/v1/environment-profiles', route => json(route,[
      {id:'reference-context-v1',name:'Reference context',requiredLayers:['surface']},
    ]))
    const asset = {id:'surface-input',bytes:32,state:'READY',representation:{
      layer:'surface',producer:'Fixture terrain',productEdition:'2025',dataTime:'June 2025',
      nativeResolution:'1 arc-second',uncertainty:'Source accuracy evidence retained',
    }}
    await page.route('**/api/v1/environment-assets', route => json(route,{items:[asset]}))
    await page.route('**/api/v1/areas/revisions', async route => {
      expect(route.request().postDataJSON().geometry).toEqual(scope.area.geometry)
      const result = await resolveScopeFixture(page,scope)
      return json(route,{...result.area,persistent:true},201)
    })
    let definition: Record<string,unknown> | undefined
    await page.route('**/api/v1/geographic-environments', route => {
      if (route.request().method()==='GET') return json(route,{items:definition?[definition]:[]})
      const input = route.request().postDataJSON()
      expect(input.assetIds).toEqual([asset.id])
      definition = {...input,id:'definition',revision:1}
      return json(route,definition,201)
    })
    await page.route('**/api/v1/geographic-environments/definition/preflight', route =>
      json(route,{eligible:true,diagnostics:[],inputs:[asset],inputBytes:32,rebuildAllowed:false}))
    const build = {id:'build',definitionId:'definition',definitionRevision:1,attempt:1,state:'SUCCEEDED'}
    await page.route('**/api/v1/geographic-environments/definition/builds', route => {
      expect(route.request().postDataJSON().revision).toBe(1)
      return json(route,build,202)
    })
    const release = {id:'release',buildId:'build',state:'STAGED',manifestSha256:'a'.repeat(64),createdAt:'2026-11-11T00:00:00Z'}
    await page.route('**/api/v1/environment-releases?*', route => json(route,{items:[release]}))
    await page.route('**/api/v1/environment-releases', route => json(route,{items:[release]}))
    await page.route('**/api/v1/environment-releases/release', route => json(route,{...release,manifest:{
      profileId:'reference-context-v1',members:[{name:'surface.tif',layer:'surface',bytes:32}],
      inputs:[{...asset,rights:{attribution:'Retained fixture attribution'}}],
      quality:{meaning:'Native surface with retained masks',heightMethod:'Source vertical datum retained',
        physicalAccuracy:'UNKNOWN',unsupported:['Survey accuracy']},
    }}))
    await page.goto('/map?'+new URLSearchParams({tool:'environments',scope:JSON.stringify(scope)}))
    const panel = page.getByRole('region',{name:'Environment Library'})
    await expect(panel).toBeVisible()
    await panel.getByRole('checkbox',{name:/Fixture terrain/}).check()
    await panel.getByRole('button',{name:'Save selected area and layers'}).click()
    await expect(panel.getByRole('heading',{name:'Eligible to prepare'})).toBeVisible()
    await panel.getByRole('button',{name:'Prepare saved revision'}).click()
    await expect(panel.getByText('Preparation succeeded · attempt 1')).toBeVisible()
    await panel.getByLabel('Available releases').selectOption('release')
    await expect(panel.getByText('Release status: staged')).toBeVisible()
    await expect(panel.getByText(/Physical accuracy: unknown/)).toBeVisible()
    await panel.getByText('Source dates, resolution and attribution').click()
    await expect(panel.getByText('Retained fixture attribution')).toBeVisible()
    await expect(panel.getByRole('button',{name:'Record verification'})).toBeDisabled()
  })
}
