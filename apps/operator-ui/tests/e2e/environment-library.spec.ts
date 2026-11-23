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

test('review forms reset when switching between cached releases with the same manifest', async ({page}, info) => {
  test.skip(info.project.name !== 'no-token', 'One release identity check')
  await mockOperationalWorkflow(page)
  const session = mockSession({'x-emad-partition': 'PUBLIC_EVIDENCE', 'x-emad-run-id': ''})
  await page.route('**/api/v1/auth/me', route => json(route, {...session, productProfile:'map',
    permissions:[...session.permissions, 'environments.read']}))
  await page.route('**/api/v1/environment-profiles', route => json(route,[{id:'fixture-profile'}]))
  const manifest = {profileId:'fixture-profile',members:[],quality:{physicalAccuracy:'UNKNOWN',unsupported:[]}}
  const first = {id:'georelease_'+'1'.repeat(26),name:'First release',state:'RELEASED',manifestSha256:'a'.repeat(64),createdAt:'2026-11-20T10:00:00Z'}
  const second = {...first,id:'georelease_'+'2'.repeat(26),name:'Second release'}
  await page.route('**/api/v1/environment-releases', route => json(route,{items:[first,second]}))
  for (const release of [first,second]) {
    await page.route(`**/api/v1/environment-releases/${release.id}`, route => json(route,{...release,manifest}))
  }
  await page.goto('/map?'+new URLSearchParams({tool:'environments',scope:JSON.stringify(scope)}))
  const panel = page.getByRole('region',{name:'Environment Library'})
  const releases = panel.getByLabel('Available releases')
  await releases.selectOption(second.id)
  await expect(panel.getByRole('heading',{name:second.name})).toBeVisible()
  await releases.selectOption(first.id)
  await expect(panel.getByRole('heading',{name:first.name})).toBeVisible()
  const label = 'Regional response and coastal evidence'
  await panel.getByText(label,{exact:true}).click()
  const form = panel.locator('details').filter({has:page.getByText(label,{exact:true})})
  await form.getByLabel('Source ID',{exact:true}).fill('first-release-source')
  await releases.selectOption(second.id)
  await expect(panel.getByRole('heading',{name:second.name})).toBeVisible()
  await expect(form.getByLabel('Source ID',{exact:true})).toHaveValue('')
})

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
      areaRevisionId:'fixture-terrain-area',frame:{origin:[55.74,24.21,0]},
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
    if (size.width === 390) {
      const common = {releaseId:release.id,manifestSha256:release.manifestSha256,inputSha256:'b'.repeat(64),
        sourceId:'fixture-source',admissionId:'fixture-admission',currentAdmissionId:'fixture-head',attribution:'Fixture source attribution'}
      await page.route('**/api/v1/environment-releases/release/municipal-workload?*', route => {
        expect(new URL(route.request().url()).searchParams.get('manifestSha256')).toBe(release.manifestSha256)
        expect(route.request().postData()).toBe('[]')
        return json(route,{...common,productVersion:'2026-q1',workload:{rowCount:12,rowsWithoutPlaceLabel:1,
          temporalMeaning:'retrospective-export',categoryCounts:{'الطرق':12},utcDayCounts:{'2026-01-01':12}}})
      })
      const municipal = panel.getByText('Riyadh 940 workload',{exact:true}).locator('..')
      await municipal.getByText('Riyadh 940 workload',{exact:true}).click()
      await municipal.getByLabel('Source ID',{exact:true}).fill(common.sourceId)
      await municipal.getByLabel('Reviewed admission ID').fill(common.admissionId)
      await municipal.getByLabel('Municipal Q1 JSON').setInputFiles({name:'q1.json',mimeType:'application/json',buffer:Buffer.from('[]')})
      await municipal.getByRole('button',{name:'Review workload'}).click()
      await expect(municipal.getByText('12 reports')).toBeVisible()
      await municipal.getByText('Riyadh 940 workload',{exact:true}).click()

      await page.route('**/api/v1/environment-releases/release/port-activity?*', route => {
        expect(new URL(route.request().url()).searchParams.get('inputSha256')).toBe(common.inputSha256)
        expect(new URL(route.request().url()).searchParams.get('consecutiveDays')).toBe('3')
        return json(route,{...common,productVersion:'daily-ports-v1',activity:{portId:'port24',
          baselineFrom:'2023-01-01',from:'2024-01-01',toExclusive:'2024-01-03',missingDays:1,
          recovery:{minimumWeekdays:4,consecutiveDays:3,firstBelowDate:'2024-01-01',returnConfirmedOn:null,daysToConfirmation:null,status:'unresolved'},days:[
            {date:'2024-01-01',portCalls:0,baselineDays:4,baselineMedian:4,comparison:'below-history'},
            {date:'2024-01-02',portCalls:null,baselineDays:4,baselineMedian:4,comparison:'missing-day'},
          ]}})
      })
      const ports = panel.getByText('Historical port activity',{exact:true}).locator('..')
      await ports.getByText('Historical port activity',{exact:true}).click()
      await ports.getByLabel('Source ID',{exact:true}).fill(common.sourceId)
      await ports.getByLabel('Reviewed admission ID').fill(common.admissionId)
      await ports.getByLabel('Reviewed file SHA-256').fill(common.inputSha256)
      await ports.getByLabel('PortWatch daily JSON').setInputFiles({name:'ports.json',mimeType:'application/json',buffer:Buffer.from('{}')})
      await ports.getByRole('button',{name:'Review activity'}).click()
      await expect(ports.getByRole('row',{name:/2024-01-01/}).getByRole('cell',{name:'0',exact:true})).toBeVisible()
      await expect(ports.getByRole('row',{name:/2024-01-02/}).getByRole('cell',{name:'Unavailable',exact:true})).toBeVisible()
      await expect(ports.getByText(/Sustained return remains unresolved/)).toBeVisible()
      await ports.getByText('Historical port activity',{exact:true}).click()
      await page.route('**/api/v1/environment-releases/release/flood-mapping?*', route => {
        expect(new URL(route.request().url()).searchParams.get('knownAt')).toBe('2024-09-21T10:00:00.000Z')
        return json(route,{...common,productVersion:'emergency-response-v1',floodMapping:{
          activation:'EMSR761',cutoff:'2024-09-21T10:00:00Z',versionCoverage:'versions-in-supplied-response',
          products:[{productId:1061,areaNumber:1,areaName:'Dresden',type:'DEL',versionId:'v2',versionNumber:2,
            deliveredAt:'2024-09-20T13:13:21Z',monitoringNumber:0,
            sourceImages:[{sourceImageId:'image',sensor:'Radar',acquisitionAt:null}]}]}})
      })
      await page.route('**/api/v1/environment-releases/release/radar-outages?*', route => {
        expect(new URL(route.request().url()).searchParams.get('country')).toBe('SA')
        return json(route,{...common,productVersion:'v4',outages:{
          country:'SA',from:'2024-09-20T00:00:00Z',toExclusive:'2024-09-21T00:00:00Z',
          availabilityAt:null,coverage:'annotations-in-supplied-response',
          annotations:[{id:'outage',startAt:'2024-09-20T12:00:00Z',endAt:null,providerType:'REGIONAL'}]}})
      })
      const evidenceForm = async (title:string, fileLabel:string, digestLabel = 'Reviewed file SHA-256') => {
        const entry = panel.getByText(title,{exact:true}).locator('..')
        await entry.getByText(title,{exact:true}).click()
        await entry.getByLabel('Source ID',{exact:true}).fill(common.sourceId)
        await entry.getByLabel('Reviewed admission ID').fill(common.admissionId)
        await entry.getByLabel(digestLabel).fill(common.inputSha256)
        await entry.getByLabel(fileLabel).setInputFiles({name:'evidence.json',mimeType:'application/json',buffer:Buffer.from('{}')})
        return entry
      }
      const flood = await evidenceForm('Copernicus flood mapping','Activation JSON')
      await flood.getByLabel('Public activation code').fill('emsr761')
      await flood.getByLabel('Delivery cutoff in UTC').fill('2024-09-21T10:00')
      await flood.getByRole('button',{name:'Review evidence'}).click()
      await expect(flood.getByText('Dresden · Delineation · version 2')).toBeVisible()
      await flood.getByText('Copernicus flood mapping',{exact:true}).click()

      const outages = await evidenceForm('Historical Internet outage annotations','Radar annotations JSON')
      await outages.getByLabel('Interval start in UTC').fill('2024-09-20T00:00')
      await outages.getByLabel('Interval end in UTC, exclusive').fill('2024-09-21T00:00')
      await outages.getByRole('button',{name:'Review evidence'}).click()
      await expect(outages.getByText(/End time unknown/)).toBeVisible()
      await expect(outages.getByText(/availability at the time is unknown/)).toBeVisible()
      await outages.getByText('Historical Internet outage annotations',{exact:true}).click()

      await page.route('**/api/v1/environment-releases/release/civilian-access-replay?*', route => {
        const cutoff = new URL(route.request().url()).searchParams.get('knownAt')
        expect(cutoff).toMatch(/^2026-11-21T(06:45|07:05):00\.000Z$/)
        const early = cutoff!.includes('06:45')
        return json(route,{...common,productVersion:'v1',replay:{
          title:'Riyadh road access tabletop',kind:'authored-exercise',graphEdition:'owned-example',origin:'junction-a',
          serviceClass:'civilian service vehicle',cutoff,
          destinations:[{id:'service-zone',label:'Service zone',knownPeople:90,unknownCells:0,
            observed:{outcome:'reachable',verifiedDistanceM:early ? 230 : 120,edgeIds:early ? ['detour'] : ['eastbound']},
            withPlannedWork:early ? {outcome:'unreachable-in-graph',verifiedDistanceM:null,edgeIds:[]} :
              {outcome:'reachable',verifiedDistanceM:120,edgeIds:['eastbound']}}],
          reports:[{id:'report-1',sourceId:'owned',category:'عوائق الطريق',availableAt:'2026-11-21T06:10:00Z'}],
          associations:[{reportId:'report-1',otherId:'report-2',decision:'suggested'}],
          associationChecks:{reviewedPairs:0,correctCandidates:0,falseCandidates:0,missedLinks:0},
          sources:[],edges:[{id:'eastbound',from:'junction-a',to:'junction-b',featureId:'fixture',
            retainedOSMFeatureId:'osm-way-1',state:early ? 'closed' : 'open',sourceId:'owned',evidenceIds:[],plannedPermits:[]}],
          works:[],inspections:[],assignments:[],restoration:[],populationYear:2024,unallocatedPeople:10,
        }})
      })
      const replay = await evidenceForm('Riyadh reports and road access replay','Reviewed road exercise JSON','Exercise file SHA-256')
      await replay.getByLabel('Evidence cutoff in UTC').fill('2026-11-21T06:45')
      await replay.getByRole('button',{name:'Replay access'}).click()
      await expect(replay.getByRole('cell',{name:/Reachable · 230 m/})).toBeVisible()
      await expect(replay.getByRole('cell',{name:'Unreachable on this graph'})).toBeVisible()
      await replay.getByText('Reports and associations',{exact:true}).click()
      await replay.getByRole('button',{name:'Keep separate in this review'}).click()
      await expect(replay.getByRole('button',{name:'Keep separate in this review'})).toHaveAttribute('aria-pressed','true')
      await replay.getByRole('button',{name:'Use this cutoff as comparison'}).click()
      await replay.getByLabel('Evidence cutoff in UTC').fill('2026-11-21T07:05')
      await replay.getByRole('button',{name:'Replay access'}).click()
      await expect(replay.getByRole('cell',{name:/Reachable · 230 m/})).toBeVisible()
      await expect(replay.getByRole('cell',{name:/Reachable · 120 m/}).first()).toBeVisible()
      await replay.getByText('Riyadh reports and road access replay',{exact:true}).click()

      await page.route('**/api/v1/environment-releases/release/civilian-aerial-review?*', route => {
        const cutoff = new URL(route.request().url()).searchParams.get('knownAt')
        expect(cutoff).toMatch(/^2026-11-18T(09:25|10:05):00\.000Z$/)
        const early = cutoff!.includes('09:25')
        const preview = {sensorFamily:'Authored civil survey',pixelM:1,usableFraction:.83,coveredFraction:1,
          timeBasis:'Authored review timeline', assets:['visual','quality-mask'].map(role => ({role,
            png:'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+a0b0AAAAASUVORK5CYII='}))}
        return json(route,{...common,productVersion:'v1',review:{title:'Civilian flood inspection tabletop',kind:'authored-exercise',
          cutoff,incidentAt:'2026-11-18T08:30:00Z',areaRevision:'exercise-road-cells',
          before:early ? null : {...preview,acquiredAt:'2026-11-18T08:00:00Z'},
          after:early ? null : {...preview,acquiredAt:'2026-11-18T08:45:00Z',
            assets:[...preview.assets,{...preview.assets[0],role:'flood-overlay'}]},
          candidates:[{id:'unavailable',state:'assets-unavailable',catalogCoverage:'partial-page',usableFraction:null,coveredFraction:null}],
          sources:[],assessments:[],traffic:[],notices:[],survey:null,energy:[],
          briefing:[{id:'receipt',availableAt:'2026-11-18T09:00:00Z',briefing:'The crew asks who should verify the report.'}],
        }})
      })
      const aerial = await evidenceForm('Civilian aerial inspection replay','Reviewed aerial JSON','Aerial package SHA-256')
      await aerial.getByLabel('Evidence cutoff in UTC').fill('2026-11-18T09:25')
      await aerial.getByRole('button',{name:'Review aerial evidence'}).click()
      await expect(aerial.getByText(/Another observation is needed/)).toBeVisible()
      await aerial.getByText('Scene selection evidence',{exact:true}).click()
      await expect(aerial.getByText(/catalog query is partial/)).toBeVisible()
      await aerial.getByLabel('Evidence cutoff in UTC').fill('2026-11-18T10:05')
      await aerial.getByRole('button',{name:'Review aerial evidence'}).click()
      await expect(aerial.getByRole('img',{name:'After reviewed image'})).toBeVisible()
      await aerial.getByLabel('Show local quality mask').check()
      await expect(aerial.getByRole('img',{name:'After review quality mask'})).toBeVisible()
      await aerial.getByLabel('Show suggested flooded-road pixels').check()
      await expect(aerial.getByRole('img',{name:'Suggested flooded-road pixels'})).toBeVisible()
      await aerial.getByText('Civilian handoff exercise',{exact:true}).click()
      await aerial.getByRole('button',{name:'Acknowledge briefing'}).click()
      await aerial.getByLabel('Follow-up owner').fill('Municipal inspection desk')
      await aerial.getByRole('button',{name:'Record handoff'}).click()
      await expect(aerial.getByText(/Follow-up assigned.*Municipal inspection desk/)).toBeVisible()
      const saved = page.waitForEvent('download')
      await aerial.getByRole('button',{name:'Download handoff for scoring'}).click()
      expect((await saved).suggestedFilename()).toBe('civilian-handoff.json')
      await aerial.getByText('Civilian aerial inspection replay',{exact:true}).click()

      await page.route('**/api/v1/environment-releases/release/regional-response-review?*', route => {
        const query = new URL(route.request().url()).searchParams
        expect(query.get('knownAt')).toBe('2026-11-20T08:40:00.000Z')
        expect(query.get('staffedSlots')).toBe('2')
        return json(route,{...common,productVersion:'v1',review:{
          title:'Asir service evidence',cutoff:query.get('knownAt'),sources:[],coast:null,
          indicators:[{id:'beds',sourceId:'moh',measure:'Annual ER beds',nativeLabel:'أسرة الطوارئ',value:367,unit:'beds',
            region:'Aseer',regionCode:'moh-aseer',geography:'MOH health region',populationScope:'MOH hospitals',
            channel:'Workbook',periodFrom:'2023-01-01',periodToExclusive:'2024-01-01'}],reconciliations:[],
          service:{region:'Asir tabletop',referenceAt:'2026-11-20T08:05:00Z',graphEdition:'owned',zones:[{
            id:'ridge',label:'Ridge zone',reference:{outcome:'reachable',verifiedDistanceM:620,edgeIds:['ridge']},
            current:{outcome:'unreachable-in-graph',verifiedDistanceM:null,edgeIds:[]},
          }],capacity:[{id:'analyst-override',label:'Edited staffing assumption',kind:'authored',staffedSlots:2}],
          workloads:[{id:'peak',label:'Peaked arrivals',kind:'authored',plannedArrivals:6,arrivals:6,reachable:3,disconnected:2,accessUnknown:1,
            reference:{reachable:5,disconnected:0,accessUnknown:1,queues:{'analyst-override':{started:5,completed:4,awaitingService:0,inServiceAtCutoff:1,medianStartedWaitSeconds:60,p90StartedWaitSeconds:120}}},
            byZone:{ridge:2},queues:{'analyst-override':{started:3,completed:3,awaitingService:0,inServiceAtCutoff:0,medianStartedWaitSeconds:0,p90StartedWaitSeconds:60}}}]},
        }})
      })
      const regional = await evidenceForm('Regional response and coastal evidence','Reviewed regional JSON','Regional package SHA-256')
      await regional.getByLabel('Evidence cutoff in UTC').fill('2026-11-20T08:40')
      await regional.getByLabel('Additional staffing assumption, slots').fill('2')
      await regional.getByRole('button',{name:'Compare regional evidence'}).click()
      await expect(regional.getByText(/Reachable · 620 m → unreachable in graph/)).toBeVisible()
      await expect(regional.getByText('3 completed · 0 in service · 0 waiting')).toBeVisible()
      await expect(regional.getByText('Reference roads · 4 completed · 1 in service · 0 waiting')).toBeVisible()
      await regional.getByText('Published regional statistics and scope',{exact:true}).click()
      await expect(regional.getByText('Annual ER beds · 367 beds')).toBeVisible()
      await regional.getByText('Regional response and coastal evidence',{exact:true}).click()

      await page.route('**/api/v1/environment-releases/release/supporting-source-review?*', route => {
        const query = new URL(route.request().url()).searchParams
        expect(query.get('knownAt')).toBe('2026-11-20T08:10:00.000Z')
        expect(query.get('inputSha256')).toBe(common.inputSha256)
        return json(route,{...common,productVersion:'v1',review:{title:'Civilian supporting evidence tabletop',
          cutoff:query.get('knownAt'),uniqueOrigins:1,sources:[],fields:[],indicators:[],comparisons:[],
          events:[{id:'scene',sourceId:'owned',origin:'shared',kind:'image-opportunity',status:'possible',revision:1,
            area:'Civilian exercise',label:'Possible observation opportunity',availableAt:'2026-11-20T08:08:00Z'},
          {id:'nav-missing',sourceId:'owned',origin:'authored-nav',kind:'navigation-training',status:'missing',revision:1,
            area:'Training area',label:'Navigation coverage missing',availableAt:'2026-11-20T08:08:00Z'}],
          delivery:[{mode:'authored-delay',pending:3,peakBacklog:3,received:1,p50LagSeconds:60,p90LagSeconds:60,
            recoverySeconds:null,recoveryStatus:'unresolved',receipts:[]}],
        }})
      })
      const supporting = await evidenceForm('Supporting evidence replay','Reviewed supporting JSON','Supporting package SHA-256')
      await supporting.getByLabel('Evidence cutoff in UTC').fill('2026-11-20T08:10')
      await supporting.getByRole('button',{name:'Replay supporting evidence'}).click()
      await expect(supporting.getByText(/3 pending · peak backlog 3/)).toBeVisible()
      await supporting.getByText('Event and publication history',{exact:true}).click()
      await expect(supporting.getByText('Possible observation opportunity',{exact:true})).toBeVisible()
      await supporting.getByText('Navigation uncertainty exercise',{exact:true}).click()
      await supporting.getByLabel('Coverage state').selectOption('missing')
      await supporting.getByLabel('Cause',{exact:true}).selectOption('unknown')
      await supporting.getByLabel('Request an independent observation').check()
      const trainingDownload = page.waitForEvent('download')
      await supporting.getByRole('button',{name:'Download training decisions'}).click()
      expect((await trainingDownload).suggestedFilename()).toBe('civilian-navigation-decisions.json')
      await supporting.getByText('Supporting evidence replay',{exact:true}).click()
    }
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
    if (size.width === 740) {
      const reference = (id:string, name:string, longitude:number) => ({
        type:'Feature',id,geometry:{type:'Point',coordinates:[longitude,24.2099]},
        properties:{name,sourceId:'owned-places',releaseId:'georel_owned',allowTerrainDisplay:true,
          attribution:'Owned terrain label fixture'},
      })
      const selectedPlace = reference('owned-first','وادي المثال',55.7401)
      const nearbyPlace = reference('owned-nearby','جبل المثال',55.7403)
      await page.route('**/api/v1/geography/regions?*', route => {
        const query = new URL(route.request().url()).searchParams
        return query.get('id') === selectedPlace.id
          ? json(route,{type:'FeatureCollection',features:[selectedPlace]}) : route.fallback()
      })
      await page.route('**/api/v1/catalog/reads', route => {
        const read = route.request().postDataJSON()
        if (read.filters?.releaseId !== 'georel_owned') return route.fallback()
        expect(read.scope.area.revisionId).toBe('fixture-terrain-area')
        return json(route,{items:[selectedPlace,nearbyPlace]})
      })
      await panel.getByText('Reference place',{exact:true}).click()
      await panel.getByLabel('Place reference from the Atlas').fill(selectedPlace.id)
      await panel.getByRole('button',{name:'Locate place on terrain'}).click()
      await expect(panel.getByText(/Placed on the preview surface/)).toBeVisible()
      await panel.getByRole('checkbox',{name:'Show nearby places from this edition'}).check()
      await expect(panel.getByRole('list',{name:'Nearby reference places'})).toContainText('جبل المثال')
    }
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
    if (size.width === 390) {
      const roadBytes = Buffer.from(JSON.stringify({schema:'emad.display-layer.v1',manifestSha256:release.manifestSha256,
        layer:'roads',data:{displayedFeatures:1,availableFeatures:1,vertices:2,method:'Owned test',
          features:[{type:'Feature',id:'fixture-edition:osm-way-1',paths:[[[1,0,1],[4,0,4]]],
            geometry:{type:'LineString',coordinates:[[55.73,24.2],[55.74,24.21]]},
            properties:{sourceId:'osm-way-1',edition:'fixture-edition',product:'Owned road',dataTime:'Fixture date',
              producer:'Fixture producer',tags:{name:'Reviewed fixture road'},height:{kind:'unknown'}}}]}}))
      const roadSha = createHash('sha256').update(roadBytes).digest('hex')
      await page.route('**/api/v1/environment-releases/release/displays', route => json(route,{items:[
        display,{...display,id:'road-preview',sha256:roadSha,bytes:roadBytes.length,recipe:'roads-display-v1'},
      ]}))
      await page.route('**/api/v1/environment-displays/road-preview/content', route =>
        route.fulfill({body:roadBytes,contentType:'application/json',headers:{etag:`"${roadSha}"`}}))
      await page.reload()
      await panel.getByLabel('Available releases').selectOption('release')
      const access = panel.getByText('Riyadh reports and road access replay',{exact:true}).locator('..')
      await access.getByText('Riyadh reports and road access replay',{exact:true}).click()
      await access.getByLabel('Source ID',{exact:true}).fill('fixture-source')
      await access.getByLabel('Reviewed admission ID').fill('fixture-admission')
      await access.getByLabel('Exercise file SHA-256').fill('b'.repeat(64))
      await access.getByLabel('Evidence cutoff in UTC').fill('2026-11-21T07:05')
      await access.getByLabel('Reviewed road exercise JSON').setInputFiles({name:'exercise.json',mimeType:'application/json',buffer:Buffer.from('{}')})
      await access.getByRole('button',{name:'Replay access'}).click()
      await access.getByText('Road findings and inspections',{exact:true}).click()
      await access.getByRole('button',{name:'Inspect retained road in 3D'}).click()
      await expect(preview.getByLabel('Inspect feature')).toHaveValue('fixture-edition:osm-way-1')
    }
  })
}
