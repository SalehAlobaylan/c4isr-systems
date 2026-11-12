import { testEvidencePath } from './evidence-path'
import {expect,test,type Page} from '@playwright/test'
import {mkdirSync} from 'node:fs'
import {resolve} from 'node:path'
import {json,list,mockOperationalWorkflow} from './fixtures-operational'
import {mockSession} from './session'
import {resolveScopeFixture} from './fixtures-analyst-scope'
const pub={workspaceId:'default',environmentId:'local',partition:'PUBLIC_EVIDENCE'}
const geometry={type:'Polygon',coordinates:[[[55.7,24.1],[55.8,24.1],[55.8,24.3],[55.7,24.3],[55.7,24.1]]]}
const country={type:'Feature',id:'test-al-ain',geometry,properties:{name:'العين Al Ain',kind:'country',regionKey:'country:X',sourceId:'fixture-geography',editionId:'fixture-edition'}}
const scope={area:{kind:'drawn',name:'العين Al Ain',geometry},temporal:{mode:'historical',interval:{from:'2025-06-01T00:00:00Z',to:'2025-07-01T00:00:00Z'},revisionView:'latest',timezone:'Asia/Dubai',backgroundImagery:false}}
const url=(tool='geography')=>'/map?tool='+tool+'&scope='+encodeURIComponent(JSON.stringify(scope))
async function setup(page:Page,role='analyst'){
 await mockOperationalWorkflow(page)
 const permissions=['operational.read','sources.read','observations.read',...(role==='analyst'?['investigations.read','investigations.create','investigations.edit','imagery.jobs.submit']:role==='reviewer'?['assessments.read','assessments.review']:role==='source-admin'?['sources.manage']:[])]
 await page.route('**/api/v1/auth/me',r=>json(r,{...mockSession(),operator:{id:'test-'+role,name:role,role:'operator'},principal:{id:'login-'+role,kind:'human'},resource:pub,resources:[pub],productProfile:'map',permissions}))
 await page.route('**/api/v1/geography/regions*',r=>json(r,{type:'FeatureCollection',features:[country]}))
 await page.route('**/api/v1/reference-datasets',r=>json(r,list([])))
 await page.route('**/api/v1/map/features*',r=>json(r,{type:'FeatureCollection',features:[],total:0,state:'latest_retained'}))
 await page.route('**/api/v1/imagery/scenes*',r=>json(r,list([])))
 await page.route('**/api/v1/imagery/captures*',r=>json(r,list([])))
 await page.route('**/api/v1/imagery/jobs*',r=>json(r,list([])))
 await page.route('**/api/v1/investigations*',r=>json(r,list([])))
 await page.route('**/api/v1/assessments/intelligence*',r=>json(r,list([])))
}
test.beforeEach(({},info)=>test.skip(info.project.name!=='no-token','Deterministic presentation matrix; isolated account per test'))
for(const role of ['analyst','reviewer','source-admin','restricted'])test(`separate ${role} account gets permitted role actions`,async({page})=>{
 await setup(page,role);await page.goto(url());await page.getByRole('button',{name:'Map workspace',exact:true}).click()
 const dialog=page.getByRole('dialog');await expect(dialog).toBeVisible()
 const wanted=role==='analyst'?'investigations':role==='reviewer'?'review':role==='source-admin'?'sources':'geography'
 await expect(dialog.locator(`[data-choice="${wanted}"]`)).toBeEnabled()
 if(role==='restricted')await expect(dialog.locator('[data-choice="investigations"]')).toBeDisabled()
 await page.keyboard.press('Escape');await expect(dialog).toBeHidden();await expect(page.getByRole('button',{name:'Map workspace',exact:true})).toBeFocused()
})
test('installed MapLibre workers load local RTL and shape mixed Arabic with packaged fonts',async({page})=>{
 await setup(page);await page.goto(url())
 await expect(page.locator('.atlas-canvas')).toHaveAttribute('data-rtl-status','loaded',{timeout:15000})
 const shaped=await page.evaluate(()=>new Promise<{arabic:string;lines:string[];font:boolean}>(resolve=>{
  (window as any).registerRTLTextPlugin=(plugin:any)=>{const text=plugin.applyArabicShaping('العين UAE 24.2');resolve({arabic:text,lines:plugin.processBidirectionalText(text,[text.length]),font:document.fonts.check('16px "IBM Plex Sans Arabic"','العين')})}
  const script=document.createElement('script');script.src='/vendor/rtl-text-0.3.0/mapbox-gl-rtl-text.js';document.head.append(script)
 }))
 expect(shaped.arabic).not.toBe('العين UAE 24.2');expect(shaped.lines.join(' ')).toContain('UAE');expect(shaped.font).toBe(true)
 await page.getByRole('searchbox',{name:'Find a country, region or place'}).fill('العين')
 await expect(page.getByRole('button',{name:/العين Al Ain.*country/})).toBeVisible()
 await expect(page.locator('.atlas-search-results bdi').first()).toHaveAttribute('dir','auto')
})
for(const size of [{width:320,height:700},{width:390,height:844},{width:740,height:360}])test(`keyboard and scope stay reachable at ${size.width}x${size.height}`,async({page})=>{
 await page.setViewportSize(size);await setup(page);await page.goto(url('imagery'))
 await expect(page.getByRole('region',{name:'Real satellite imagery'})).toBeVisible()
 await expect(page.getByText(/2025.*Asia\/Dubai/).filter({visible:true}).first()).toBeVisible()
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1)).toBe(true)
 // A 200% text enlargement is tested separately from a viewport resize.
 await page.addStyleTag({content:'html{font-size:200%}'});await page.getByRole('button',{name:'Map workspace',exact:true}).click();await expect(page.getByRole('dialog')).toBeVisible();await page.keyboard.press('Escape')
 const captureButton=page.getByRole('button',{name:'Capture real band windows',exact:true});await captureButton.scrollIntoViewIfNeeded();const rect=await captureButton.boundingBox();expect(rect!.height).toBeGreaterThanOrEqual(44);expect(rect!.y).toBeGreaterThanOrEqual(0);expect(rect!.y).toBeLessThan(size.height)
 const evidence=testEvidencePath('');mkdirSync(evidence,{recursive:true});await page.screenshot({path:resolve(evidence,`imagery-${size.width}x${size.height}-text200.png`)})
})
test('Arabic coordinate digits keep signed longitude first and reject unsupported ranges',async({page})=>{
 await setup(page);await page.goto(url())
 const value=await page.evaluate(async()=>{const input=await import('/src/lib/internationalInput.ts');return {point:input.longitudeLatitude('٥٥٫٧٣،٢٤٫٢١'),negative:input.longitudeLatitude('−٤٫٨٢،٤٣٫٩٥'),invalid:input.longitudeLatitude('EPSG:3857 100000,200000'),label:input.pointerLabel([55.73,24.21])}})
 expect(value.point).toEqual([55.73,24.21]);expect(value.negative).toEqual([-4.82,43.95]);expect(value.invalid).toBeUndefined();expect(value.label).toContain('longitude, 24.2100° latitude')
})
test('adopting an inscribed subset is deliberate and a lost capture response keeps its intent',async({page})=>{
 await setup(page)
 const scene={id:'live-path-fixture',sourceId:'test-imagery',collection:'sentinel-2-c1-l2a',acquiredAt:'2025-06-01T06:59:14Z',platform:'sentinel-2c',providerId:'Synthetic protocol scene',bbox:[55.6,24,56,24.5],rights:{attribution:'Synthetic browser fixture'}}
 await page.route('**/api/v1/imagery/scenes*',r=>json(r,list([scene])))
 const aoi={west:55.73,south:24.20,east:55.75,north:24.22}
 await page.route('**/api/v1/imagery/capture-proposal',r=>{const input=r.request().postDataJSON();return json(r,{sceneId:scene.id,sceneGeometry:geometry,supported:true,valid:!!input.aoi,suggestedAoi:aoi,reasons:[],meaning:'Synthetic exact-containment protocol response'})})
 const attempts:any[]=[]
 await page.route('**/api/v1/imagery/captures',r=>{if(r.request().method()!=='POST')return json(r,list([]));attempts.push(r.request().postDataJSON());return attempts.length===1?r.abort('failed'):json(r,{id:'saved-intent',status:'QUEUED',input:{scene,aoi}},201)})
 await page.goto(url('imagery'))
 const region=page.getByRole('region',{name:'Real satellite imagery'}),capture=region.getByRole('button',{name:'Capture real band windows',exact:true})
 await region.getByLabel('Collection 1 scene').selectOption(scene.id)
 await expect(capture).toBeDisabled()
 await region.getByRole('button',{name:'Adopt suggested inscribed subset'}).click()
 await expect(region.getByLabel('Capture AOI (west,south,east,north)')).toHaveValue('55.73,24.2,55.75,24.22')
 await expect(capture).toBeEnabled();await expect(page.locator('.atlas-canvas')).toHaveAttribute('data-proposal-footprint-count','2')
 await capture.click();await expect(region.getByRole('alert').first()).toBeVisible();await capture.click()
 await expect.poll(()=>attempts.length).toBe(2);expect(attempts[1]).toEqual(attempts[0]);expect(attempts[0].aoi).toEqual(aoi)
 expect(attempts[0].scope.temporal.interval).toEqual(scope.temporal.interval)
})
test('an incompatible retained pair explains the problem and cannot advance',async({page})=>{
 await setup(page)
 const make=(id:string,at:string)=>({id,sourceId:'test-imagery',status:'SUCCEEDED',attempt:1,progress:1,errorCode:'',createdAt:at,input:{scene:{id,acquiredAt:at,platform:'sentinel-2c',rights:{attribution:'Synthetic fixture'}},aoi:{west:55.73,south:24.20,east:55.75,north:24.22},method:'Synthetic fixture'},result:{preparationId:'prep-'+id,rgbArtifactId:'rgb-'+id,qualityArtifactId:'mask-'+id}})
 await page.route('**/api/v1/imagery/captures*',r=>json(r,list([make('before','2025-06-01T06:59:14Z'),make('after','2025-06-11T06:59:14Z')])))
 await page.route('**/api/v1/imagery/comparison-check',r=>json(r,{compatible:false,reasons:['The retained grids differ. Choose the same rectangle and grid size.'],validCells:0,totalCells:0,clearFraction:0,limitations:[],evaluatedAt:'2026-10-26T00:00:00Z'}))
 await page.goto(url('imagery'));const region=page.getByRole('region',{name:'Real satellite imagery'})
 await expect(region.getByText('The retained grids differ. Choose the same rectangle and grid size.',{exact:true})).toBeVisible()
 await expect(region.getByRole('button',{name:'Use this pair for reproducible NDVI comparison'})).toBeDisabled()
})
test('a failed WebGL renderer keeps exact scope and draft controls usable',async({page})=>{
 await setup(page)
 await page.addInitScript(()=>{const original=HTMLCanvasElement.prototype.getContext;HTMLCanvasElement.prototype.getContext=function(type:string,...args:any[]){if(type==='webgl'||type==='webgl2'||type==='experimental-webgl')return null;return original.apply(this,[type,...args] as any)} as typeof original})
 await page.goto(url('imagery'))
 await expect(page.getByText(/The map renderer is unavailable on this device/)).toBeVisible()
 const capture=page.getByRole('region',{name:'Real satellite imagery'})
 await capture.getByLabel('Capture AOI (west,south,east,north)').fill('55.73,24.2,55.75,24.22')
 await expect(capture.getByLabel('Capture AOI (west,south,east,north)')).toHaveValue('55.73,24.2,55.75,24.22')
 const raw=JSON.parse(new URL(page.url()).searchParams.get('scope')!);expect((typeof raw==='string'?JSON.parse(raw):raw).area.geometry).toEqual(geometry)
})
test('aggregate drill-down continues within the original read and exposes individual revisions',async({page})=>{
 await setup(page)
 await page.route('**/api/v1/map/features*',r=>{const q=new URL(r.request().url()).searchParams;return json(r,{type:'FeatureCollection',state:'aggregated',total:2110,features:q.get('family')==='earthquakes'?[{type:'Feature',geometry:{type:'Point',coordinates:[55.74,24.21]},properties:{kind:'earthquakes',recordId:'aggregate-a',aggregate:true,aggregateId:'cell-a',cellBounds:[55.7,24.1,55.8,24.3],count:2110,title:'2110 retained reports'}}]:[]})})
 const requests:URL[]=[]
 await page.route('**/api/v1/analyst-scope/reads/*/records?*',async r=>{const u=new URL(r.request().url());requests.push(u);const next=!!u.searchParams.get('cursor'),resolved=await resolveScopeFixture(page,scope as any);return r.fulfill({headers:{'Content-Type':'application/json','X-Analyst-Scope-Hash':resolved.scopeHash},json:{snapshotId:'fixture',family:'earthquakes',total:2110,returned:1,hasMore:!next,nextCursor:next?undefined:'opaque-next-records',items:[{evidenceId:next?'report-b':'report-a',reported:{title:next?'Retained report B':'Retained report A',observedAt:'2025-06-11T00:00:00Z'},displayPosition:{type:'Point',coordinates:[55.74,24.21]}}]}})})
 const mapState={camera:{lng:55.74,lat:24.21,zoom:10,bearing:0,pitch:0},layers:['earthquakes']}
 await page.goto('/map?scope='+encodeURIComponent(JSON.stringify(scope))+'&mapState='+encodeURIComponent(JSON.stringify(mapState)))
 const canvas=page.locator('.atlas-canvas');await expect(canvas).toHaveAttribute('data-map-ready','true');await expect(canvas).toHaveAttribute('data-public-feature-count','1')
 const records=page.getByRole('region',{name:'Individual retained reports'})
 await expect(async()=>{const b=await canvas.boundingBox();await canvas.click({position:{x:b!.width/2,y:b!.height/2}});await expect(records).toBeVisible()}).toPass({timeout:10000})
 await expect(records.getByRole('button',{name:/Retained report A/})).toBeVisible();await records.getByRole('button',{name:'More individual reports'}).click()
 await expect(records.getByRole('button',{name:/Retained report B/})).toBeVisible();await expect(records.getByText('2110 reports in this selection')).toBeVisible()
 const continued=requests.find(u=>u.searchParams.has('cursor'));expect(continued?.pathname).toBe(requests[0].pathname);expect(continued?.searchParams.get('cursor')).toBe('opaque-next-records');expect(continued?.searchParams.get('aggregate_id')).toBe('cell-a')
})

test('a direct exact-area link keeps its camera after development renderer recreation',async({page})=>{
 await setup(page);await page.goto(url('imagery'))
 const canvas=page.locator('.atlas-canvas');await expect(canvas).toHaveAttribute('data-map-ready','true')
 await expect.poll(async()=>JSON.parse(await canvas.getAttribute('data-map-center')??'[0,0]')[0]).toBeGreaterThan(55.6)
 const center=JSON.parse(await canvas.getAttribute('data-map-center')??'[0,0]');expect(center[0]).toBeLessThan(55.9);expect(center[1]).toBeGreaterThan(24);expect(center[1]).toBeLessThan(24.4)
 await page.getByRole('button',{name:'Map workspace',exact:true}).click();await page.keyboard.press('Escape');await expect(page.locator('.atlas-canvas')).toHaveCount(1)
})
