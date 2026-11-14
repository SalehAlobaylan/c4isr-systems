import {expect, test} from '@playwright/test'
import {createHash} from 'node:crypto'
import type {ScopeInput} from '../../src/lib/api'
import {json, mockOperationalWorkflow} from './fixtures-operational'
import {resolveScopeFixture} from './fixtures-analyst-scope'
import {mockSession} from './session'
import {geographicCoordinates, geographicSurfacePoint, makeBuildingVolumes, pickPolygon, type SceneFeature} from '../../src/features/workspace/GeographicSceneLayers'
import {projectContextStatus} from '../../src/features/workspace/contextLayerStatus'
import type {CoverageResult} from '../../src/lib/api'

test('context status distinguishes failed, empty, partial, stale and reduced reads', ({}, info) => {
  test.skip(info.project.name !== 'no-token', 'One status projection check')
  const now = Date.parse('2026-11-13T10:00:00Z')
  const source = {id:'fixture',name:'Fixture provider',support:'supported',spatialRelation:'full',retrieval:'complete_query_footprint',
    dataResult:'successful_empty_retrieval',latestEvidenceAt:'2026-11-13T08:00:00Z',latestRetrieval:'2026-11-13T09:59:00Z',receipts:[]} as CoverageResult['sources'][number]
  const layer = {type:'FeatureCollection' as const, features:[], total:0}
  const receipts = [{sourceId:source.id,admissionId:'fixture-v1',freshForSeconds:300,runState:{status:'SUCCEEDED'}}]
  expect(projectContextStatus({layer,sources:[source],receipts,now}).state).toMatch(/Completed query/)
  expect(projectContextStatus({layer,sources:[source],receipts,now}).age[0].freshness).toBe('stale')
  expect(projectContextStatus({layer,sources:[{...source,spatialRelation:'none'}],now}).state).toBe('Outside admitted coverage')
  expect(projectContextStatus({layer,sources:[{...source,retrieval:'partial_retrieval'}],now}).state).toBe('Partial retrieval')
  expect(projectContextStatus({layer,sources:[source],receipts:[{...receipts[0],runState:{status:'FAILED'}}],now}).state).not.toMatch(/Completed query/)
  expect(projectContextStatus({layer,sources:[source],now,failed:true}).total).toBeUndefined()
  expect(projectContextStatus({layer:{...layer,total:100,state:'aggregated'},sources:[source],now}).limited).toBe(true)
})

test('Riyadh frame inversion and multipart feature picking preserve spatial identity', ({}, info) => {
  test.skip(info.project.name !== 'no-token', 'One coordinate and topology check')
  const origin = [46.762, 24.714, 0]
  const [west, south] = geographicCoordinates(origin, -3036.2023818194416, -1.5920426831698933, 3322.720248751048)
  const [east, north] = geographicCoordinates(origin, 3035.222999596609, 998.4080485184159, -3323.921685120282)
  expect(west).toBeCloseTo(46.732, 9)
  expect(south).toBeCloseTo(24.684, 9)
  expect(east).toBeCloseTo(46.792, 9)
  expect(north).toBeCloseTo(24.744, 9)
  expect(geographicSurfacePoint(origin, -3036.2023818194416, -1.5920426831698933, 3322.720248751048)[2]).toBeCloseTo(0, 5)
  expect(geographicSurfacePoint(origin, 3035.222999596609, 998.4080485184159, -3323.921685120282)[2]).toBeCloseTo(1000, 5)

  const feature: SceneFeature = {type:'Feature', id:'source-edition:osm-relation-1', paths:[], properties:{
    edition:'source-edition', sourceId:'osm-relation-1', product:'fixture', dataTime:'fixture', producer:'fixture', tags:{}, height:{kind:'unknown'},
  }, geometry:{type:'MultiPolygon',coordinates:[
    [[[0,0],[4,0],[4,4],[0,4],[0,0]],[[1,1],[3,1],[3,3],[1,3],[1,1]]],
    [[[5,5],[6,5],[6,6],[5,6],[5,5]]],
  ]}}
  expect(pickPolygon([feature], .5, .5)?.id).toBe(feature.id)
  expect(pickPolygon([feature], 2, 2)).toBeUndefined()
  expect(pickPolygon([feature], 5.5, 5.5)?.id).toBe(feature.id)

  const rings = feature.geometry.type === 'MultiPolygon' ? feature.geometry.coordinates[0] : []
  const footprint: SceneFeature = {...feature, geometry:{type:'Polygon',coordinates:rings},
    paths:rings.map(ring=>ring.map(([x,y])=>[x,0,-y]))}
  const tagged: SceneFeature = {...footprint, id:'source-edition:osm-way-2', properties:{...footprint.properties,
    sourceId:'osm-way-2', tags:{height:'12 m'}, height:{kind:'source-tag',metres:12}}}
  const volumes = makeBuildingVolumes([footprint, tagged])
  expect(volumes.count).toBe(1)
  volumes.dispose()
})

const scope: ScopeInput = {
  area: {kind: 'drawn', name: 'Al Ain reference area', geometry: {
    type: 'Polygon', coordinates: [[[55.73,24.2],[55.75,24.2],[55.75,24.22],[55.73,24.22],[55.73,24.2]]],
  }},
  temporal: {mode:'historical', revisionView:'latest', timezone:'Asia/Dubai', backgroundImagery:false,
    interval:{from:'2025-06-01T00:00:00Z',to:'2025-07-01T00:00:00Z'}},
}

function terrainFixture() {
  const doc = JSON.stringify({
    asset:{version:'2.0'}, scene:0, scenes:[{nodes:[0]}], nodes:[{mesh:0}],
    meshes:[{primitives:[{attributes:{POSITION:0,NORMAL:1},indices:2,mode:4}]}],
    buffers:[{byteLength:84}],
    bufferViews:[{buffer:0,byteLength:36},{buffer:0,byteOffset:36,byteLength:36},{buffer:0,byteOffset:72,byteLength:12}],
    accessors:[
      {bufferView:0,componentType:5126,count:3,type:'VEC3',min:[0,0,0],max:[100,10,100]},
      {bufferView:1,componentType:5126,count:3,type:'VEC3'},
      {bufferView:2,componentType:5125,count:3,type:'SCALAR'},
    ],
  })
  const jsonBytes = Buffer.from(doc.padEnd(Math.ceil(doc.length/4)*4))
  const header = Buffer.alloc(20)
  header.write('glTF')
  header.writeUInt32LE(2,4)
  header.writeUInt32LE(28+jsonBytes.length+84,8)
  header.writeUInt32LE(jsonBytes.length,12)
  header.write('JSON',16)
  const bin = Buffer.alloc(92)
  bin.writeUInt32LE(84)
  bin.write('BIN',4)
  const verticesAndNormals = [0,0,0,0,10,100,100,0,0,0,1,0,0,1,0,0,1,0]
  const indices = [0,1,2]
  verticesAndNormals.forEach((value,i) => bin.writeFloatLE(value,8+4*i))
  indices.forEach((value,i) => bin.writeUInt32LE(value,80+4*i))
  return Buffer.concat([header,jsonBytes,bin])
}

for (const size of [{width:390,height:844},{width:740,height:360}]) {
  test(`environment library saves the selected area and presents a staged result at ${size.width}×${size.height}`, async ({page}, info) => {
    test.skip(info.project.name !== 'no-token', 'One browser workflow per viewport')
    await page.setViewportSize(size)
    await mockOperationalWorkflow(page)
    await page.route('**/api/v1/auth/me', route => {
      const session = mockSession({'x-emad-partition':'PUBLIC_EVIDENCE','x-emad-run-id':''})
      return json(route,{...session,productProfile:'map',permissions:[...session.permissions,
        'environments.read','environments.manage','environments.build','environments.qualify','environments.download','sources.manage']})
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
      json(route,{eligible:true,diagnostics:[],inputs:[asset],inputBytes:32,rebuildAllowed:false,
        coverage:[{layer:'surface',required:true,status:'DECLARED_COVERED'}]}))
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
    await page.route('**/api/v1/environment-releases/release/lineage', route => json(route,{
      releaseId:release.id,manifestSha256:release.manifestSha256,
      inputs:[{assetId:asset.id,sourceId:'retained-source',representation:asset.representation,attribution:'Fixture source lineage',
        retainUntil:'2027-02-10T00:00:00Z',originalDownload:false}],
      members:[{name:'surface.tif',layer:'surface',bytes:32,inputId:asset.id}],
    }))
    await page.route('**/api/v1/environment-releases/release/quality-report', route => json(route,{
      manifestSha256:release.manifestSha256,profileContractSha256:'',
      coordinateCheck:{maximumErrorM:0.0002,limitM:0.001},
      coverage:[{member:'surface.tif',layer:'surface',state:'SAMPLE_MASK_RECORDED',validSamples:21,selectedSamples:32,registration:'Point'}],
    }))
    await page.route('**/api/v1/environment-sources/retained-source/impact', route => json(route,{
      currentPolicy:{productVersion:'2025',enabled:true,analystQuestion:'Riyadh reference preparation'},
      retainedInputs:1,retainedBytes:32,pendingInputs:0,tombstones:0,releases:{items:[{id:release.id,state:release.state,displays:1}]},
    }))
    await page.route('**/api/v1/environment-operations/storage', route => json(route,{items:[{
      assetId:'expired-fixture',bytes:32,state:'EXPIRED',purgeAttempts:1,purgeRetryAt:'2027-02-10T00:00:00Z',
    }]}))
    await page.route('**/api/v1/environment-releases/release/decisions', route => json(route,[{
      state:'STAGED', manifestSha256:release.manifestSha256, note:'Prepared fixture evidence', occurredAt:'2026-11-12T08:00:00Z',
    }]))
    const glb = terrainFixture()
    const sha256 = createHash('sha256').update(glb).digest('hex')
    const display = {id:'preview-fixture',releaseId:release.id,schemaVersion:1,state:'READY',recipe:'terrain-glb-v1',manifestSha256:release.manifestSha256,
      bytes:glb.length,sha256,physicalAccuracy:'UNKNOWN',verticalDatum:'WGS84 ellipsoid',
      retainUntil:'2100-01-01T00:00:00Z'}
    const displayJob = {id:'geobuild_'+'c'.repeat(26),state:'QUEUED',attempt:0}
    let savedJobReads = 0
    await page.route('**/api/v1/environment-releases/release/display-builds', route => {
      expect(route.request().postDataJSON()).toMatchObject({manifestSha256:release.manifestSha256,recipe:'terrain-glb-v2'})
      return json(route,displayJob,202)
    })
    await page.route(`**/api/v1/environment-builds/${displayJob.id}`, route => {
      savedJobReads++
      return json(route,displayJob)
    })
    let metadataReads = 0, contentReads = 0
    let corrupt = true
    await page.route('**/api/v1/environment-releases/release/displays', route => {
      metadataReads++
      return json(route,{items:[display]})
    })
    await page.route('**/api/v1/environment-displays/preview-fixture/content', route => {
      contentReads++
      return route.fulfill({body:corrupt ? Buffer.alloc(glb.length) : glb,
        contentType:'model/gltf-binary',headers:{etag:`"${sha256}"`}})
    })
    const libraryURL = '/map?'+new URLSearchParams({tool:'environments',scope:JSON.stringify(scope)})
    await page.goto(libraryURL)
    const panel = page.getByRole('region',{name:'Environment Library'})
    await expect(panel).toBeVisible()
    await panel.getByRole('checkbox',{name:/Fixture terrain/}).check()
    await panel.getByRole('button',{name:'Save selected area and layers'}).click()
    await expect(panel.getByRole('heading',{name:'Eligible to prepare'})).toBeVisible()
    await panel.getByText('Declared source coverage').click()
    await expect(panel.getByText('surface · required · footprints cover area')).toBeVisible()
    await panel.getByRole('button',{name:'Prepare saved revision'}).click()
    await expect(panel.getByText('Preparation succeeded · attempt 1')).toBeVisible()
    await panel.getByLabel('Available releases').selectOption('release')
    await expect(panel.getByText('Release status: staged')).toBeVisible()
    await expect(panel.getByText(/Physical accuracy: unknown/)).toBeVisible()
    await panel.getByText('Source dates, resolution and attribution').click()
    await expect(panel.getByText('Retained fixture attribution')).toBeVisible()
    await panel.getByText('Source lineage and download rights').click()
    await expect(panel.getByText('Source restricts distribution of originals.')).toBeVisible()
    await expect(panel.getByText('Prepared members: surface.tif')).toBeVisible()
    await panel.getByRole('button',{name:'Inspect this source policy'}).click()
    await expect(panel.getByText(/Riyadh reference preparation/)).toBeVisible()
    await panel.getByRole('button',{name:'Close source policy impact'}).click()
    await panel.getByText('Coordinate and sample checks', {exact:true}).click()
    await expect(panel.getByText(/21 of 32 selected samples valid/)).toBeVisible()
    await panel.getByText('Storage and cleanup', {exact:true}).click()
    await expect(panel.getByText(/expired-fixture.*cleanup retry/)).toBeVisible()
    await expect(panel.getByRole('button',{name:'Record verification'})).toBeDisabled()
    const decisionSummary = panel.getByText('Release decision history', {exact:true})
    await decisionSummary.focus()
    await page.keyboard.press('Enter')
    await expect(panel.getByText('Prepared fixture evidence')).toBeVisible()
    await page.keyboard.press('Enter')
    await expect(panel.getByText('Prepared fixture evidence')).toBeHidden()
    expect(metadataReads).toBe(0)
    await panel.getByText('3D terrain preview').click()
    const openTerrain = panel.getByRole('button',{name:/Open terrain/})
    await expect(openTerrain).toBeVisible()
    expect(metadataReads).toBe(1)
    expect(contentReads).toBe(0)
    await openTerrain.click()
    const preview = panel.getByRole('region',{name:'Reference terrain'})
    await expect(preview.getByRole('alert')).toHaveText('Terrain integrity check failed.')
    const failedReads = contentReads
    corrupt = false
    await preview.getByRole('button',{name:'Retry terrain preview'}).click()
    await expect(preview.getByRole('status')).toHaveText('Verified reference terrain')
    await expect(preview.getByLabel('Interactive reference terrain')).toBeVisible()
    expect(contentReads).toBe(failedReads+1)
    await panel.getByText('3D terrain preview').click()
    await expect(preview).toHaveCount(0)
    await page.goto(libraryURL)
    await panel.getByLabel('Available releases').selectOption('release')
    await panel.getByText('3D terrain preview').click()
    await panel.getByText('Build a display', {exact:true}).click()
    await panel.getByRole('button',{name:'Queue display'}).click()
    await expect(panel.getByText('Display queued · attempt 0')).toBeVisible()
    const readsBeforeReload = savedJobReads
    await page.reload()
    await panel.getByLabel('Available releases').selectOption('release')
    await panel.getByText('3D terrain preview').click()
    await panel.getByText('Build a display', {exact:true}).click()
    await expect(panel.getByText('Display queued · attempt 0')).toBeVisible()
    expect(savedJobReads).toBeGreaterThan(readsBeforeReload)
  })
}
