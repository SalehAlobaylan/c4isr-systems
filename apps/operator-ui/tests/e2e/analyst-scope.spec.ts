import { testEvidencePath } from './evidence-path'
import {expect,test,type Page} from '@playwright/test'
import {json,list,mockOperationalWorkflow} from './fixtures-operational'
import {mockSession} from './session'
import {chooseWorkbench} from '../navigation'
import type {AreaSelection,ScopeInput} from '../../src/lib/api'

const rectangle=(w:number,s:number,e:number,n:number)=>({type:'Polygon',coordinates:[[[w,s],[e,s],[e,n],[w,n],[w,s]]]})
const country={type:'Feature',id:'edition-a:country',geometry:rectangle(2,48,3,49),properties:{name:'Fixture country',kind:'country',regionKey:'country:X',sourceId:'geo-source',editionId:'edition-a',releaseId:'release-a'}}
const subdivision={...country,id:'edition-a:subdivision',geometry:rectangle(2.2,48.7,2.5,48.9),properties:{...country.properties,name:'Fixture subdivision',kind:'region',regionKey:'region:X1',parentKey:'country:X'}}
const local=rectangle(2.3,48.8,2.32,48.82)
const period={mode:'historical' as const,interval:{from:'2026-01-01T00:00:00Z',to:'2026-01-02T00:00:00Z'},revisionView:'latest' as const,timezone:'UTC',backgroundImagery:false}
const scene={id:'scene-jan',sourceId:'source-imagery',collection:'sentinel-2-c1-l2a',acquiredAt:'2026-01-01T10:00:00Z',platform:'Sentinel-2',providerId:'fixture-jan-scene',tile:'fixture-tile',orbit:1,processingVersion:'1',rights:{attribution:'Synthetic protocol fixture'},bbox:[2,48,3,49]}
const scopeURL=(area:AreaSelection,tool='systems')=>'/map?scope='+encodeURIComponent(JSON.stringify({area,temporal:period}))+'&tool='+tool
const scopeInURL=(url:string):ScopeInput=>{let value:any=new URL(url).searchParams.get('scope');for(let i=0;i<2&&typeof value==='string';i++)value=JSON.parse(value);return value}
async function openViews(page:Page){const button=page.getByRole('button',{name:'Map views',exact:true});if(!await button.isVisible())await page.locator('summary[aria-label="Map controls"]').click();await button.click()}
async function setup(page:Page){
 await mockOperationalWorkflow(page)
 const resource={workspaceId:'default',environmentId:'local',partition:'PUBLIC_EVIDENCE'}
 await page.route('**/api/v1/auth/me',r=>json(r,{...mockSession(),resource,resources:[resource],productProfile:'map',permissions:['operational.read','sources.read','observations.read','tracks.read','assessments.read','imagery.jobs.submit','context.export']}))
 await page.route('**/api/v1/geography/regions?*',r=>{const q=new URL(r.request().url()).searchParams;return json(r,{type:'FeatureCollection',features:q.has('id')?[country,subdivision].filter(f=>f.id===q.get('id')):q.get('kind')==='country'?[country]:q.has('parent')?[subdivision]:[country,subdivision]})})
 await page.route('**/api/v1/map/features?*',r=>json(r,{type:'FeatureCollection',features:[],total:0,state:'latest_retained'}))
 await page.route('**/api/v1/public-events*',r=>json(r,list([])))
 await page.route('**/api/v1/reference-datasets',r=>json(r,list([{id:'ref-fixture',label:'Fixture reference',eligibleForSimulation:true,featureCount:1,kind:'vector',datasetKey:'fixture-reference',aoi:{west:2,south:48,east:3,north:49},rights:{attribution:'Synthetic fixture'},resolution:{value:1,unit:'m'},quality:'Synthetic protocol fixture',version:'1',sha256:'fixture-reference-hash'}])))
 await page.route('**/api/v1/imagery/jobs*',r=>json(r,list([])))
 await page.route('**/api/v1/imagery/scenes*',r=>json(r,list([scene])))
 await page.route('**/api/v1/imagery/captures*',r=>json(r,list([])))
 await page.route('**/api/v1/assessments/intelligence*',r=>json(r,list([])))
 await page.route('**/api/v1/public-context/records*',r=>json(r,list([])))
 await page.route('**/api/v1/external-tracks*',r=>json(r,list([])))
 await page.route('**/api/v1/context-packages',r=>json(r,list([])))
}
test.beforeEach(async({page},info)=>{test.skip(info.project.name!=='no-token','One deterministic scope matrix');await setup(page)})

const forms:[string,AreaSelection][]=[
 ['country',{kind:'administrative',regionId:country.id,sourceId:'geo-source',releaseId:'release-a'}],
 ['subdivision',{kind:'administrative',regionId:subdivision.id,sourceId:'geo-source',releaseId:'release-a'}],
 ['named study',{kind:'named',name:'Coastal study fixture',geometry:local}],
 ['drawn polygon with a hole',{kind:'drawn',name:'Holed study fixture',geometry:{type:'Polygon',coordinates:[(local.coordinates as number[][][])[0],[[2.305,48.805],[2.31,48.805],[2.31,48.81],[2.305,48.81],[2.305,48.805]]]}}],
 ['explicit viewport',{kind:'viewport',name:'Explicit viewport fixture',geometry:local}],
 ['date-line components',{kind:'composite',name:'Date-line fixture',geometry:{type:'MultiPolygon',coordinates:[[[[179,10],[180,10],[180,11],[179,11],[179,10]]],[[[-180,10],[-179,10],[-179,11],[-180,11],[-180,10]]]]}}],
]
for(const [name,area]of forms)test(`${name} and its period survive Systems, imagery, review and export`,async({page})=>{
 const calls:{path:string;scope:any;hash:string|null}[]=[]
 page.on('request',r=>{const u=new URL(r.url());if(u.pathname.startsWith('/api/')&&u.searchParams.has('scope'))calls.push({path:u.pathname,scope:JSON.parse(u.searchParams.get('scope')!),hash:u.searchParams.get('scope_hash')})})
 page.on('response',response=>{const r=response.request();if(new URL(r.url()).pathname==='/api/v1/catalog/reads'&&r.method()==='POST'){const input=r.postDataJSON();if(input.scope)calls.push({path:'/api/v1/catalog/reads#'+input.kind,scope:input.scope,hash:response.headers()['x-analyst-scope-hash']??null})}})
 await page.goto(scopeURL(area))
 await expect(page.locator('[aria-label="Public Context systems"]')).toBeVisible()
 await expect(page.getByText(/Historical study.*2026.*UTC/, {exact:false}).first()).toBeVisible()
 for(const tool of ['imagery','review','context']){await chooseWorkbench(page,tool);await expect(page.getByRole('heading',{name:tool==='imagery'?'Imagery intelligence':tool==='review'?'Assessment review':'Context packages',exact:true})).toBeVisible()}
 await expect.poll(()=>calls.some(c=>c.path.endsWith('#scenes'))).toBe(true)
 for(const call of calls){expect(call.scope.temporal.interval).toEqual(period.interval);expect(call.hash).toBeTruthy();expect(call.scope.temporal.timezone).toBe('UTC');if(area.regionId)expect(call.scope.area.regionId).toBe(area.regionId);else expect(call.scope.area.geometry).toEqual(area.geometry)}
 await page.getByRole('link',{name:'Return to map',exact:true}).click()
 expect(scopeInURL(page.url()).temporal.interval).toEqual(period.interval)
})

test('adopting a different map period clears scene choices and export previews',async({page})=>{
 await page.goto(scopeURL({kind:'drawn',name:'Local fixture',geometry:local},'imagery'))
 const capture=page.getByRole('region',{name:'Real satellite imagery'})
 await capture.getByLabel('Collection 1 scene').selectOption('scene-jan')
 await expect(capture.getByRole('button',{name:'Capture real band windows'})).toBeEnabled()
 await page.getByRole('button',{name:'Time & filters',exact:true}).click()
 await page.getByRole('button',{name:'7 days',exact:true}).click()
 await page.getByRole('button',{name:'Close map tool',exact:true}).click()
 await expect(capture.getByLabel('Collection 1 scene')).toHaveValue('scene-jan')
 await page.getByRole('button',{name:'Use current map scope',exact:true}).click()
 await expect(capture.getByLabel('Collection 1 scene')).toHaveValue('')
 await expect(capture.getByRole('button',{name:'Capture real band windows'})).toBeDisabled()
 expect(scopeInURL(page.url()).temporal.mode).toBe('monitoring')
 await chooseWorkbench(page,'context')
 await expect(page.getByRole('region',{name:'Approved context export'}).getByLabel('Export interval from (UTC)')).not.toHaveValue('2026-01-01T00:00')
})

test('bookmark and browser history restore exact geography and period',async({page})=>{
 await page.goto(scopeURL({kind:'administrative',regionId:country.id,sourceId:'geo-source',releaseId:'release-a'},'geography'))
 await expect(page.getByRole('navigation',{name:'Selected geographic area'})).toContainText('Fixture country')
 await openViews(page)
 await page.getByLabel('Name this map view').fill('Pinned January geography')
 await page.getByRole('button',{name:'Save current view',exact:true}).click()
 await page.getByRole('button',{name:'Close map tool',exact:true}).click()
 await page.getByRole('searchbox',{name:'Find a country, region or place'}).fill('subdivision')
 await page.getByRole('button',{name:/Fixture subdivision.*region/}).click()
 await expect.poll(()=>scopeInURL(page.url()).area.regionId).toBe(subdivision.id)
 await openViews(page)
 await page.getByRole('button',{name:'Pinned January geography',exact:true}).click()
 await expect.poll(()=>scopeInURL(page.url()).area.regionId).toBe(country.id)
 await page.goBack();await expect.poll(()=>scopeInURL(page.url()).area.regionId).toBe(subdivision.id)
 await page.goForward();await expect.poll(()=>scopeInURL(page.url()).area.releaseId).toBe('release-a')
 expect(scopeInURL(page.url()).temporal.interval).toEqual(period.interval)
 await expect(page.getByRole('button',{name:'Time & filters',exact:true})).toContainText(/Historical study.*2026/)
 await expect(page.getByText('Resolving the selected area and period…', {exact:true})).toHaveCount(0)
 await page.screenshot({path:testEvidencePath('bookmarked-scope.png'),fullPage:true})
})

test('conflicting legacy scopes recover without a catalog or mutation fallback',async({page})=>{
 const submissions:string[]=[];page.on('request',r=>{if(r.method()==='POST'&&/imagery\/captures|context-packages/.test(r.url()))submissions.push(r.url())})
 await page.goto('/map?region='+country.id+'&area='+encodeURIComponent(JSON.stringify(local))+'&tool=context')
 await expect(page.getByRole('alert').filter({hasText:/both an administrative area and a drawn area/}).first()).toBeVisible()
 await expect(page.getByRole('region',{name:'Approved context export'})).toHaveCount(0)
 await page.getByRole('button',{name:'Start a new view',exact:true}).click()
 await expect.poll(()=>scopeInURL(page.url()).area.kind).toBe('world')
 expect(submissions).toEqual([])
})

test('a hidden retained workbench stops catalog polling',async({page})=>{
 await page.clock.install()
 let captures=0;page.on('request',r=>{if(new URL(r.url()).pathname==='/api/v1/imagery/captures')captures++})
 await page.goto(scopeURL({kind:'drawn',geometry:local},'imagery'))
 await expect(page.getByRole('region',{name:'Real satellite imagery'})).toBeVisible()
 await expect.poll(()=>captures).toBeGreaterThan(0)
 await chooseWorkbench(page,'geography')
 await expect(page.getByRole('region',{name:'Real satellite imagery'})).toBeHidden()
 const before=captures
 await page.clock.fastForward(12_000)
 expect(captures).toBe(before)
})

test('mismatched response fingerprints cannot supply a scene or enable capture',async({page})=>{
 await page.route('**/api/v1/catalog/reads',r=>r.request().postDataJSON().kind==='scenes'?r.fulfill({json:{items:[scene],total:1,hasMore:false},headers:{'X-Analyst-Scope-Hash':'obsolete-scope'}}):r.fallback())
 await page.goto(scopeURL({kind:'drawn',geometry:local},'imagery'))
 const capture=page.getByRole('region',{name:'Real satellite imagery'})
 await expect(capture.getByLabel('Collection 1 scene').locator('option[value=scene-jan]')).toHaveCount(0)
 await expect(capture.getByRole('button',{name:'Capture real band windows'})).toBeDisabled()
 await expect(page.getByRole('alert').filter({hasText:'Some authorized imagery records are unavailable.'})).toBeVisible()
})

test('high-latitude scope remains available with an explicit display restriction',async({page})=>{
 await page.goto(scopeURL({kind:'drawn',name:'Polar geometry fixture',geometry:rectangle(10,86,12,88)},'context'))
 await expect(page.getByRole('status').filter({hasText:/beyond the map display limit/})).toBeVisible()
 await expect(page.getByRole('region',{name:'Approved context export'}).getByText('Pinned area: Polar geometry fixture',{exact:true})).toBeVisible()
 expect(scopeInURL(page.url()).area.geometry).toEqual(rectangle(10,86,12,88))
})

for(const endpoint of ['resolve','read'])test(`a denied scope ${endpoint} clears protected work until reconnect`,async({page})=>{
 await page.goto(scopeURL({kind:'drawn',name:'Private draft fixture',geometry:local},'imagery'))
 await expect(page.getByRole('region',{name:'Real satellite imagery'})).toBeVisible()
 await page.route('**/api/v1/analyst-scope/'+endpoint,r=>r.fulfill({status:403,json:{error:{code:'FORBIDDEN',message:'Scope access revoked'}}}))
 await page.getByRole('searchbox',{name:'Find a country, region or place'}).fill('Fixture country')
 await page.getByRole('button',{name:/Fixture country.*country/}).click()
 await expect(page.getByText('Workspace access changed. Reconnect to refresh your authorization.',{exact:true})).toBeVisible()
 await expect(page.getByRole('button',{name:'Reconnect',exact:true})).toBeVisible()
 await expect(page.getByRole('region',{name:'Map preview dock'})).toHaveCount(0)
 await expect(page.getByRole('region',{name:'Real satellite imagery'})).toHaveCount(0)
})

test('a delayed old map response is cancelled and cannot replace a newer scope',async({page})=>{
 let release!:()=>void,entered=false,lateFinished=false
 const gate=new Promise<void>(resolve=>{release=resolve})
 await page.route('**/api/v1/analyst-scope/read',async route=>{
  const input=route.request().postDataJSON()
  if(input.area.name!=='Slow old scope'||!input.families?.length)return route.fallback()
  entered=true;await gate
  await route.fulfill({json:{scope:{scopeHash:'obsolete'},layers:{imagery:{type:'FeatureCollection',features:[{type:'Feature',geometry:{type:'Point',coordinates:[2.3,48.8]},properties:{recordId:'obsolete',title:'Obsolete late feature'}}],total:1}}}}).catch(()=>{})
  lateFinished=true
 })
 let cancelled=0;page.on('requestfailed',r=>{if(r.url().endsWith('/analyst-scope/read'))cancelled++})
 try{
  await page.goto(scopeURL({kind:'drawn',name:'Slow old scope',geometry:local},'geography'))
  await expect.poll(()=>entered).toBe(true)
  await page.getByRole('searchbox',{name:'Find a country, region or place'}).fill('Fixture country')
  await page.getByRole('button',{name:/Fixture country.*country/}).click()
  await expect.poll(()=>scopeInURL(page.url()).area.regionId).toBe(country.id)
  await expect(page.getByRole('navigation',{name:'Selected geographic area'})).toContainText('Fixture country')
  await expect.poll(()=>cancelled).toBeGreaterThan(0)
  release();await expect.poll(()=>lateFinished).toBe(true)
  await expect(page.getByText('Obsolete late feature',{exact:true})).toHaveCount(0)
 }finally{release()}
})
