import {expect,test,type Page} from '@playwright/test'
import {json,list,mockOperationalWorkflow} from './fixtures-operational'
import {mockSession} from './session'

const pub={workspaceId:'default',environmentId:'local',partition:'PUBLIC_EVIDENCE'}
const rectangle=(w:number,s:number,e:number,n:number)=>({type:'Polygon',coordinates:[[[w,s],[e,s],[e,n],[w,n],[w,s]]]})
const country={type:'Feature',id:'country-sa',geometry:rectangle(34,16,56,33),properties:{name:'Saudi Arabia',kind:'country',regionKey:'country:SAU'}}
const scene={id:'scene-paris',sourceId:'source-imagery',collection:'sentinel-2-c1-l2a',acquiredAt:'2026-01-01T10:00:00Z',platform:'Sentinel-2',providerId:'retained-paris',tile:'tile',orbit:1,processingVersion:'1',cloudCover:0,rights:{attribution:'Isolated test'},bbox:[2,48,3,49]}

async function setup(page:Page){
 await mockOperationalWorkflow(page)
 await page.route('**/api/v1/auth/me',r=>json(r,{...mockSession(),resource:pub,resources:[pub],productProfile:'map',permissions:['operational.read','sources.read','sources.manage','observations.read','tracks.read','assessments.read','imagery.jobs.submit','context.export']}))
 await page.route('**/api/v1/platform/readiness',r=>json(r,{features:[],workers:[],resource:pub,checkedAt:'2026-10-25T00:00:00Z'}))
 await page.route('**/api/v1/geography/regions?*',r=>json(r,{type:'FeatureCollection',features:[country]}))
 await page.route('**/api/v1/map/features?*',r=>json(r,{type:'FeatureCollection',features:[],total:0,truncated:false}))
 await page.route('**/api/v1/reference-datasets',r=>json(r,list([])))
 await page.route('**/api/v1/imagery/jobs',r=>json(r,list([])))
 await page.route('**/api/v1/imagery/scenes*',r=>json(r,list([scene])))
 await page.route('**/api/v1/imagery/captures*',r=>json(r,list([])))
 await page.route('**/api/v1/assessments/intelligence',r=>json(r,list([])))
 await page.route('**/api/v1/context-packages',r=>json(r,list([])))
}
test.beforeEach(async({page},info)=>{test.skip(info.project.name!=='no-token','One deterministic map-first matrix');await setup(page)})

test('map entry ignores a saved exercise and never requests simulation onboarding',async({page})=>{
 await page.addInitScript(()=>localStorage.setItem('emad.resource.human:operator-01',JSON.stringify(['default','local','EXERCISE_OBSERVATION','legacy'])))
 const calls:string[]=[]
 page.on('request',r=>{if(/\/api\/v1\/(simulation|scenarios)/.test(r.url()))calls.push(r.url())})
 await page.goto('/')
 await expect(page).toHaveURL(url=>url.pathname==='/map')
 await expect(page.getByRole('searchbox',{name:'Find a country, region or place'})).toBeVisible()
 await page.screenshot({path:'test-results/map-entry.png',fullPage:true})
 await expect(page.getByRole('button',{name:'Simulation',exact:true})).toHaveCount(0)
 await expect(page.getByTestId('workspace-basis')).toContainText('Public sources')
 expect(calls).toEqual([])
})

test('country handoff copies the actual extent and never supplies Jeddah export coordinates',async({page})=>{
 await page.goto('/map?region=country-sa&tool=context')
 const region=page.getByRole('region',{name:'Approved context export'})
 await expect(region.getByText('Pinned area: Saudi Arabia',{exact:true})).toBeVisible()
 await region.getByRole('button',{name:'Use investigation region'}).click()
 await expect(region.getByLabel('Export AOI (west,south,east,north)')).toHaveValue('34,16,56,33')
 await expect(region.getByLabel('Export AOI (west,south,east,north)')).not.toHaveValue('39.144,21.464,39.216,21.536')
 // An irregular country cannot become its bounding rectangle implicitly.
 await page.route('**/api/v1/geography/regions?*',r=>json(r,{type:'FeatureCollection',features:[{...country,id:'irregular',geometry:{type:'Polygon',coordinates:[[[34,16],[56,16],[50,33],[34,16]]]}}]}))
 await page.goto('/map?region=irregular&tool=context')
 await expect(region.getByRole('button',{name:'Use investigation region'})).toBeDisabled()
 await expect(region.getByRole('button',{name:'Preview selected context'})).toBeDisabled()
 await expect(region.getByLabel('Export AOI (west,south,east,north)')).toHaveValue('')
})

test('changing a mounted capture workbench to a large country clears the old AOI and scene',async({page})=>{
 const scope={area:{kind:'drawn',geometry:rectangle(2.3,48.8,2.32,48.82)},temporal:{mode:'historical',interval:{from:'2026-01-01T00:00:00Z',to:'2026-01-02T00:00:00Z'},revisionView:'latest',timezone:'UTC',backgroundImagery:false}}
 await page.goto('/map?scope='+encodeURIComponent(JSON.stringify(scope))+'&tool=imagery')
 const capture=page.getByRole('region',{name:'Real satellite imagery'})
 await expect(capture.getByLabel('Capture AOI (west,south,east,north)')).toHaveValue('2.3,48.8,2.32,48.82')
 await capture.getByLabel('Collection 1 scene').selectOption('scene-paris')
 await expect(capture.getByRole('button',{name:'Capture real band windows'})).toBeEnabled()
 await page.getByLabel('Find a country, region or place').fill('Saudi')
 await page.getByRole('button',{name:/Saudi Arabia.*country/}).click()
 // Until explicitly changed, the draft remains visibly pinned to its old area.
 await expect(capture.getByLabel('Capture AOI (west,south,east,north)')).toHaveValue('2.3,48.8,2.32,48.82')
 await page.getByRole('button',{name:'Use current map scope',exact:true}).click()
 await expect(capture.getByText('Pinned area: Saudi Arabia.',{exact:false})).toBeVisible()
 await expect(capture.getByLabel('Capture AOI (west,south,east,north)')).toHaveValue('')
 await expect(capture.getByLabel('Collection 1 scene')).toHaveValue('')
 await expect(capture.getByRole('button',{name:'Capture real band windows'})).toBeDisabled()
 await page.screenshot({path:'test-results/capture-scope-cleared.png',fullPage:true})
})

test('a non-fixture export submits exactly the explicitly selected rectangle',async({page})=>{
 await page.route('**/api/v1/reference-datasets',r=>json(r,list([{id:'ref-paris',label:'Paris retained reference',eligibleForSimulation:true,featureCount:1}])))
 let submitted:Record<string,unknown>|undefined
 await page.route('**/api/v1/context-packages/preview',r=>{submitted=r.request().postDataJSON();return json(r,{id:'preview-paris',sha256:'test',bytes:20,expiresAt:'2099-01-01T00:00:00Z',manifest:{reproduction:'reference_only',assets:[],omissions:[]}})})
 await page.goto('/map?area='+encodeURIComponent(JSON.stringify(rectangle(2.3,48.8,2.32,48.82)))+'&tool=context')
 const region=page.getByRole('region',{name:'Approved context export'})
 await region.getByRole('button',{name:'Use investigation region'}).click()
 await expect(region.getByLabel('Export interval from (UTC)')).not.toHaveValue('')
 await expect(region.getByLabel('Export interval to (UTC)')).not.toHaveValue('')
 await region.getByRole('checkbox',{name:/Paris retained reference/}).check()
 await region.getByLabel('Accepted accuracy and imagery limitations').fill('Reference-only isolated browser test.')
 await region.getByRole('button',{name:'Preview selected context'}).click()
 await expect.poll(()=>submitted?.aoi).toEqual({west:2.3,south:48.8,east:2.32,north:48.82})
})

test('source administration records purpose separately from original evidence',async({page})=>{
 let classified=false,submitted:Record<string,unknown>|undefined
 await page.route('**/api/v1/content-purpose*',r=>{
  if(r.request().method()==='POST'){
   submitted=r.request().postDataJSON();classified=true
   return json(r,{...submitted,version:1,actorId:'fixture-administrator'})
  }
  const all=new URL(r.request().url()).searchParams.get('view')==='all'
  const items=!classified||all?[{kind:'reference',entityId:'ref-unknown',purpose:classified?'working':'unclassified',version:classified?1:0,reason:'',receiptReference:'',actorId:''}]:[]
  return json(r,{items,total:items.length,offset:0,hasMore:false})
 })
 await page.goto('/map?tool=sources')
 const panel=page.getByRole('region',{name:'Content purpose review'})
 await panel.getByRole('button',{name:'reference · unclassified · ref-unknown',exact:true}).click()
 await panel.getByRole('combobox',{name:'Purpose',exact:true}).selectOption('working')
 await panel.getByLabel('Verified receipt or evidence reference').fill('isolated-installation-receipt#references')
 await panel.getByLabel('Classification reason').fill('Exact retained hash and source receipt verified for this fixture.')
 await panel.getByRole('button',{name:'Record classification',exact:true}).click()
 await expect.poll(()=>submitted?.expectedVersion).toBe(0)
 expect(submitted).toMatchObject({kind:'reference',entityId:'ref-unknown',purpose:'working'})
 await panel.getByLabel('Catalog purpose view').selectOption('all')
 await expect(panel.getByRole('button',{name:'reference · working · ref-unknown',exact:true})).toBeVisible()
})

test('an invalid shared polygon cannot borrow a saved investigation rectangle',async({page})=>{
 await page.addInitScript(()=>{
  const key='emad.ux.v2.human:operator-01.'+JSON.stringify(['default','local','PUBLIC_EVIDENCE',''])+'.investigation'
  localStorage.setItem(key,JSON.stringify({bbox:'39.144,21.464,39.216,21.536'}))
 })
 await page.goto('/map?area='+encodeURIComponent('{invalid')+'&tool=context')
 const region=page.getByRole('region',{name:'Approved context export'})
 await expect(region).toHaveCount(0)
 await expect(page.getByRole('button',{name:'Start a new view',exact:true})).toBeVisible()
 await expect(page.getByRole('link',{name:'Choose a valid map scope'})).toBeVisible()
})

test('external basemap failure leaves selected geometry and export controls usable',async({page})=>{
 await page.route('https://tile.openstreetmap.org/**',r=>r.abort('failed'))
 await page.goto('/map?area='+encodeURIComponent(JSON.stringify(rectangle(2.3,48.8,2.32,48.82)))+'&tool=context')
 await expect(page.locator('[data-map-ready="true"]')).toBeVisible()
 await expect(page.getByRole('status').filter({hasText:/basemap unavailable|map source could not be loaded/})).toBeVisible()
 const region=page.getByRole('region',{name:'Approved context export'})
 await region.getByRole('button',{name:'Use investigation region'}).click()
 await expect(region.getByLabel('Export AOI (west,south,east,north)')).toHaveValue('2.3,48.8,2.32,48.82')
 await expect(page.getByRole('button',{name:'Fit area',exact:true})).toBeEnabled()
})
