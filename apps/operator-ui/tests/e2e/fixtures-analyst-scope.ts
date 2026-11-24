import {createHash} from 'node:crypto'
import type {Page,Route} from '@playwright/test'
import type {ScopeInput,ScopeSnapshot} from '../../src/lib/api'

// Protocol fixtures only. Topology, history, RLS and publication races are
// verified separately against real PostGIS; this does not emulate PostGIS.
const hash=(v:unknown)=>createHash('sha256').update(JSON.stringify(v??null)).digest('hex')
const families=['earthquakes','thermal','weather','bulletins','aircraft','vessels','imagery','findings','coverage','events','scenes','captures']
const get=async(page:Page,path:string)=>page.evaluate(async path=>{const r=await fetch(path);return r.json()},path)
async function resolve(page:Page,input:ScopeInput):Promise<ScopeSnapshot>{
 let geometry=input.area.geometry as any,name=input.area.name??'Drawn study area',kind=input.area.kind??'drawn',geography:any
 if(input.area.regionId){const found=await get(page,'/api/v1/geography/regions?id='+encodeURIComponent(input.area.regionId));const f=found.features?.find((f:any)=>f.id===input.area.regionId);if(!f)throw new Error('Selected geographic area is unavailable.');geometry=f.geometry;name=f.properties.name;kind=geometry.type==='Point'?'point':'administrative';geography={sourceId:f.properties.sourceId??'fixture-geography',regionId:f.id,logicalKey:f.properties.regionKey??f.id,editionId:f.properties.editionId??'fixture-edition',releaseId:f.properties.releaseId,editionSha256:'fixture-edition-hash',attribution:'Synthetic browser fixture',meaning:'Isolated protocol fixture'}}
 if(kind==='world'){name='World · configured evidence';geometry=undefined}
 const polygons=geometry?.type==='Polygon'?[geometry.coordinates]:geometry?.type==='MultiPolygon'?geometry.coordinates:[]
 const components=polygons.map((p:any)=>{const positions=p.flat(),x=positions.map((v:number[])=>v[0]),y=positions.map((v:number[])=>v[1]);return {geometry:{type:'Polygon',coordinates:p},bounds:[Math.min(...x),Math.min(...y),Math.max(...x),Math.max(...y)],areaM2:1}})
 const now=new Date().toISOString(),temporal={...input.temporal,rollingSeconds:undefined,interval:input.temporal.interval??{from:new Date(Date.now()-(input.temporal.rollingSeconds??86400)*1000).toISOString(),to:now},evaluatedAt:now,bounds:'[from,to)',frozen:true}
 const area={schema:'emad.area-revision.v1',normalizationVersion:'fixture-only',areaId:'fixture-area',revisionId:'arev_'+hash({geometry,name,kind,geography}),resource:{workspaceId:'default',environmentId:'local',partition:'PUBLIC_EVIDENCE' as const},name,kind,geometry,originalGeometry:geometry,geometrySha256:hash(geometry),normalized:false,components,areaM2:1,display:{state:components.some((c:any)=>c.bounds[1]<-85||c.bounds[3]>85)?'partial':'supported',latitudeLimit:85,message:'Part of this area lies beyond the map display limit. Exact WGS84 geometry is retained.'},geography,requiredSources:geography?[geography.sourceId]:[],persistent:false}
 const scopeHash=hash({area:area.revisionId,mode:temporal.mode,interval:temporal.interval,revisionView:temporal.revisionView,asOf:temporal.asOf,timezone:temporal.timezone,backgroundImagery:temporal.backgroundImagery})
 const capabilities=Object.fromEntries(families.map(family=>[family,{state:temporal.mode==='historical'&&['weather','bulletins','aircraft','vessels'].includes(family)||temporal.revisionView!=='latest'&&!['earthquakes','thermal','events'].includes(family)?'unsupported':'supported',clock:['imagery','scenes','captures','findings'].includes(family)?'acquisition':'observation',reason:'This feed does not retain a complete historical reconstruction.'}]))
 return {schema:'emad.scope-snapshot.v1',id:'scope_'+scopeHash,scopeHash,area,temporal,authorizationRevision:'fixture-access-v1',requiredSources:area.requiredSources,capabilities}
}
const json=(route:Route,body:unknown,status=200)=>route.fulfill({status,json:body})
export async function mockAnalystScope(page:Page){
 const catalogs=new Map<string,{key:string;items:any[]}>(),cursors=new Map<string,{id:string;offset:number}>()
 await page.route('**/api/v1/catalog/reads',async route=>{try{
  const input=route.request().postDataJSON(),key=hash({...input,snapshotId:undefined,cursor:undefined,replaceSnapshotId:undefined}),limit=input.limit??100
  let id=input.snapshotId,offset=0,stored=catalogs.get(id)
  if(input.cursor){const position=cursors.get(input.cursor);if(!stored||stored.key!==key||position?.id!==id)return json(route,{error:{message:'Cursor mismatch'}},409);offset=position.offset}
  else{const paths:Record<string,string>={geography:'/geography/regions',scenes:'/imagery/scenes',captures:'/imagery/captures',jobs:'/imagery/jobs',findings:'/assessments/intelligence',investigations:'/investigations'};const q=new URLSearchParams();const f=input.filters??{};for(const [from,to]of Object.entries({query:'q',parent:'parent',kind:'kind',sourceId:'source_id',releaseId:'release_id',editionId:'edition_id'}))if(f[from])q.set(to,f[from]);if(input.scope){const scope=await resolve(page,input.scope);if(scope.area.geometry)q.set('geometry',JSON.stringify(scope.area.geometry));q.set('observed_from',scope.temporal.interval.from);q.set('observed_to',scope.temporal.interval.to)}const response=await get(page,'/api/v1'+paths[input.kind]+(q.size?'?'+q:''));if(response.error)return json(route,response,503);stored={key,items:response.items??response.features??[]};id=crypto.randomUUID();catalogs.set(id,stored)}
  const items=stored!.items.slice(offset,offset+limit),hasMore=offset+limit<stored!.items.length,nextCursor=hasMore?crypto.randomUUID():undefined;if(nextCursor)cursors.set(nextCursor,{id,offset:offset+limit})
  return route.fulfill({status:200,headers:{'Content-Type':'application/json',...(input.scope?{'X-Analyst-Scope-Hash':(await resolve(page,input.scope)).scopeHash}:{})},json:{snapshotId:id,kind:input.kind,items,total:stored!.items.length,returned:items.length,offset,representation:'retained_records',state:'available',countMeaning:'Synthetic protocol fixture; exact PostGIS behavior tested separately',evaluatedAt:new Date().toISOString(),expiresAt:new Date(Date.now()+300000).toISOString(),hasMore,nextCursor}})
 }catch{if(!page.isClosed())await json(route,{error:{message:'Catalog fixture unavailable'}},503).catch(()=>{})}})
 await page.route('**/api/v1/imagery/capture-proposal',async route=>{const input=route.request().postDataJSON(),scope=await resolve(page,input.scope),scenes=await get(page,'/api/v1/imagery/scenes'),scene=scenes.items?.find((v:any)=>v.id===input.sceneId),bounds=scene?.bbox??[-180,-90,180,90],a=input.aoi;return json(route,{scope,sceneId:input.sceneId,sceneGeometry:{type:'Polygon',coordinates:[[[bounds[0],bounds[1]],[bounds[2],bounds[1]],[bounds[2],bounds[3]],[bounds[0],bounds[3]],[bounds[0],bounds[1]]]]},supported:!!scene,valid:!!a&&!!scene,reasons:['Synthetic protocol footprint check'],meaning:'Synthetic browser fixture'})})
 await page.route('**/api/v1/imagery/comparison-check',route=>json(route,{compatible:false,reasons:['Choose two completed compatible retained preparations.'],validCells:0,totalCells:0,clearFraction:0,limitations:['Synthetic browser fixture'],evaluatedAt:new Date().toISOString()}))
 await page.route('**/api/v1/coverage/query',async route=>{try{const scope=await resolve(page,route.request().postDataJSON().scope);return await json(route,{schema:'emad.coverage.v1',scope,evaluatedAt:scope.temporal.evaluatedAt,sources:[],storage:'available',meaning:'Synthetic protocol fixture: no coverage evaluation available for a source.',recoveryOwner:'source administrator'})}catch(e){if(!page.isClosed())await json(route,{error:{code:'validation_error',message:'Coverage fixture unavailable'}},400).catch(()=>{})}})
 await page.route('**/api/v1/coverage/inventory',route=>json(route,{schema:'emad.coverage-inventory.v1',evaluatedAt:new Date().toISOString(),meaning:'Synthetic protocol fixture workspace inventory.',totalRetained:0,sources:[],timeBuckets:[],footprints:[]}))
 await page.route('**/api/v1/acquisitions',route=>route.request().method()==='GET'?json(route,{items:[],returned:0,limit:100,historyMeaning:'Synthetic fixture'}):route.fallback())

 await page.route('**/api/v1/analyst-scope/**',async route=>{
  try{
   const input=route.request().postDataJSON() as ScopeInput&{families?:string[];includeEvents?:boolean;filters?:Record<string,string>;zoom?:number}
   const scope=await resolve(page,input)
   if(!route.request().url().endsWith('/read'))return json(route,scope)
   const layers:Record<string,unknown>={}
   for(const family of input.families??[]){const q=new URLSearchParams({family,zoom:String(input.zoom??0),view:scope.temporal.revisionView,observed_from:scope.temporal.interval.from,observed_to:scope.temporal.interval.to});if(scope.area.geometry)q.set('geometry',JSON.stringify(scope.area.geometry));if(input.filters?.sourceId)q.set('source_id',input.filters.sourceId);const supported=scope.capabilities[family].state==='supported',result=supported?await get(page,'/api/v1/map/features?'+q):{};layers[family]={type:'FeatureCollection',features:result.features??[],total:result.total??0,truncated:result.truncated??false,state:supported?result.state??'latest_retained':'historical_unsupported',scopeHash:scope.scopeHash,evaluatedAt:scope.temporal.evaluatedAt,support:scope.capabilities[family],countMeaning:result.countMeaning}}
   const rawEvents=input.includeEvents?await get(page,'/api/v1/public-events?limit=200'):undefined
   const events=input.includeEvents?{items:Array.isArray(rawEvents?.items)?rawEvents.items:[],total:rawEvents?.total??0,truncated:rawEvents?.truncated??false,filters:rawEvents?.filters??{},view:rawEvents?.view??{mode:'latest',meaning:'Synthetic browser protocol fixture'}}:undefined
   return json(route,{id:'read_'+scope.scopeHash,queryFingerprint:hash(input),scope,layers,events,expiresAt:new Date(Date.now()+300000).toISOString(),coverageReceipts:[],receiptMeaning:'Synthetic protocol fixture'})
  }catch(e){if(!page.isClosed())await json(route,{error:{code:'validation_error',message:e instanceof Error?e.message:'Invalid fixture scope'}},400).catch(()=>{})}
 })
 await page.route('**/api/v1/areas/resolve',async route=>{const a=route.request().postDataJSON();const scope=await resolve(page,{area:a,temporal:{mode:'monitoring',revisionView:'latest',timezone:'UTC',backgroundImagery:false}});return json(route,scope.area)})
}

export const resolveScopeFixture=resolve
