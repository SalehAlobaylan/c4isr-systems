import { testEvidencePath } from './evidence-path'
import {expect,test,type Page} from '@playwright/test'
import {json,list,mockOperationalWorkflow} from './fixtures-operational'
import {readFileSync} from 'node:fs'
import {resolve} from 'node:path'
import {mockSession} from './session'

const geometry={type:'Polygon',coordinates:[[[2,48],[3,48],[3,49],[2,49],[2,48]]]}
const country={type:'Feature',id:'geo-country',geometry,properties:{name:'Fixture country',kind:'country',regionKey:'country:AAA',sourceId:'geography-source',editionId:'edition-a',releaseId:'release-a'}}
const period={mode:'historical',interval:{from:'2026-01-01T00:00:00Z',to:'2026-01-02T00:00:00Z'},revisionView:'latest',timezone:'UTC',backgroundImagery:false}
const scope={area:{kind:'drawn',name:'Portable study',geometry},temporal:period}
const url=(tool='systems',geographic=false)=>'/map?scope='+encodeURIComponent(JSON.stringify(geographic?{...scope,area:{kind:'administrative',regionId:country.id,sourceId:'geography-source',releaseId:'release-a'}}:scope))+'&tool='+tool
type Server={cases:Map<string,any>;intents:Map<string,any>;conflict:boolean;denied:boolean;loseNextSave:boolean}
const server=():Server=>({cases:new Map(),intents:new Map(),conflict:false,denied:false,loseNextSave:false})
async function setup(page:Page,state=server(),human='operator-01'){
 await mockOperationalWorkflow(page)
 const resource={workspaceId:'default',environmentId:'local',partition:'PUBLIC_EVIDENCE'}
 await page.route('**/api/v1/auth/me',r=>json(r,{...mockSession(),principal:{id:'human:'+human,kind:'human'},operator:{id:human,name:human,role:'operator'},resource,resources:[resource],productProfile:'map',permissions:[...mockSession().permissions,'sources.manage','geography.request','imagery.jobs.submit','context.export','investigations.read','investigations.create','investigations.edit','investigations.manage','investigations.review','outputs.read','outputs.create','outputs.distribute']}))
 await page.route('**/api/v1/geography/regions?*',r=>{const q=new URL(r.request().url()).searchParams;return json(r,{type:'FeatureCollection',features:q.has('id')&&q.get('id')!==country.id?[]:[country],matched:1,truncated:false})})
 await page.route('**/api/v1/map/features?*',r=>json(r,{type:'FeatureCollection',features:[],total:0,state:'latest_retained'}))
 await page.route('**/api/v1/public-events*',r=>json(r,list([])))
 await page.route('**/api/v1/analyst-scope/resolve',async r=>{const body=r.request().postDataJSON();if(!body.area.revisionId)return r.fallback();const found=[...state.cases.values()].find(c=>c.revision.scope.area.revisionId===body.area.revisionId);return found?json(r,found.revision.scope):r.fulfill({status:404,json:{error:{code:'NOT_FOUND',message:'Exact scope unavailable'}}})})
 await page.route('**/api/v1/investigations**',async r=>{
  const path=new URL(r.request().url()).pathname,parts=path.split('/'),id=parts[4],body=r.request().method()==='POST'?r.request().postDataJSON():undefined
  if(path.endsWith('/delivery-jobs'))return json(r,{items:[],limit:100})
  if(path.endsWith('/history'))return json(r,{items:state.cases.has(id)?[state.cases.get(id).revision]:[],limit:200})
  if(path.endsWith('/reviews'))return json(r,{items:state.cases.get(id)?.workflow?[state.cases.get(id).workflow]:[],limit:200})
  if(path.endsWith('/review-inbox'))return json(r,{items:[...state.cases.values()].flatMap(c=>c.workflow?.action==='REVIEW_REQUESTED'&&c.workflow.reviewerHumanId===human?[c.workflow]:[]),limit:100})
  if(path.endsWith('/review')){const c=state.cases.get(id);const event={...body,id:'review-'+body.action,caseId:id,revisionId:c.revision.id,reviewerHumanId:body.action==='REVIEW_REQUESTED'?body.reviewerHumanId:c.workflow.reviewerHumanId,actorHumanId:human,contentDigest:'fixture-content-digest',contributors:['operator-01'],recordedAt:new Date().toISOString()};state.cases.set(id,{...c,workflow:event});return json(r,event,201)}
  if(body?.requestKey&&state.intents.has(body.requestKey))return json(r,state.intents.get(body.requestKey),201)
  if(r.request().method()==='GET')return id?state.denied||!state.cases.has(id)?r.fulfill({status:404,json:{error:{code:'NOT_FOUND',message:'Investigation unavailable'}}}):json(r,state.cases.get(id)):json(r,{items:[...state.cases.values()],total:state.cases.size,offset:0,hasMore:false})
  const old=id?state.cases.get(id):undefined
  if(id&&path.endsWith('/members')){const updated={...old,members:body.members,ownerHumanId:body.ownerHumanId,membershipVersion:old.membershipVersion+1};state.cases.set(id,updated);return json(r,updated)}
  if(state.conflict)return r.fulfill({status:409,json:{error:{code:'CONFLICT',message:'Investigation changed; reload before saving your draft'}}})
  const resolved=await page.evaluate(async input=>{const response=await fetch('/api/v1/analyst-scope/resolve',{method:'POST',headers:{'content-type':'application/json'},body:JSON.stringify(input)});return response.json()},body.scope)
  resolved.area.persistent=true
  const caseId=id??'case-'+(state.cases.size+1),version=(old?.revision.version??0)+1
  const c={id:caseId,ownerHumanId:old?.ownerHumanId??human,members:old?.members??[human],membershipVersion:old?.membershipVersion??1,revision:{...body,id:'revision-'+version,caseId,version,previousId:old?.revision.id,scope:resolved,actorHumanId:human,recordedAt:new Date().toISOString()}}
  state.cases.set(caseId,c);if(body.requestKey)state.intents.set(body.requestKey,c);if(state.loseNextSave){state.loseNextSave=false;return r.fulfill({status:503,json:{error:{code:'UNAVAILABLE',message:'Response lost after durable save; retry original intent'}}})};return json(r,c,201)
 })
 return state
}
const casesTab=async(page:Page)=>page.getByRole('group',{name:'Systems sections'}).getByRole('button',{name:'Investigations',exact:true}).click()
async function saveFirst(page:Page){await page.goto(url());await casesTab(page);await page.getByRole('textbox',{name:'Investigation title',exact:true}).fill('Portable retained study');await page.getByRole('textbox',{name:'Analytical question',exact:true}).fill('What changed in this exact area and interval?');await page.getByRole('textbox',{name:'Investigation notes',exact:true}).fill('A note retained on the server.');await page.getByRole('button',{name:'Save investigation',exact:true}).click();await expect(page.getByRole('status').filter({hasText:'Saved revision 1'})).toBeVisible()}
test.beforeEach(({},info)=>test.skip(info.project.name!=='no-token','One deterministic contract matrix'))

test('readable brief queue retry preserves its original durable intent',async({page})=>{
 await setup(page);await saveFirst(page)
 const bodies:any[]=[];let jobs:any[]=[]
 await page.route('**/api/v1/investigations/*/delivery-jobs',r=>{
  if(r.request().method()==='GET')return json(r,{items:jobs,limit:100})
  bodies.push(r.request().postDataJSON());jobs=[{id:'delivery-fixture',caseId:'case-1',outputId:'output-frozen',creatorHumanId:'operator-01',kind:'brief',state:'QUEUED',attempt:0,workerBuild:'',failureReason:'',createdAt:'2026-10-27T00:00:00Z'}]
  return bodies.length===1?r.fulfill({status:503,json:{error:{code:'UNAVAILABLE',message:'Response lost after enqueue; retry original intent'}}}):json(r,jobs[0],202)
 })
 await page.getByRole('button',{name:'Queue readable brief',exact:true}).click()
 await expect(page.getByRole('status').filter({hasText:'Response lost after enqueue'})).toBeVisible()
 await page.reload()
 await page.getByRole('button',{name:'Retry original delivery request',exact:true}).click()
 await expect(page.getByRole('status').filter({hasText:'Original delivery request recovered.'})).toBeVisible()
 expect(bodies).toHaveLength(2);expect(bodies[0]).toEqual(bodies[1]);expect(bodies[0].output.expectedRevisionId).toBe('revision-1')
 await expect(page.getByRole('list',{name:'Durable delivery jobs'})).toContainText('QUEUED')
})

test('retained Go-rendered brief stays readable offline and binds review selection',async({page})=>{
 await setup(page);await saveFirst(page)
 const html=readFileSync(resolve('../../docs/implementation/evidence/map-first/t19-t21/analyst-brief.html'),'utf-8')
 const source=JSON.parse(readFileSync(resolve('../../docs/implementation/evidence/map-first/t19-t21/brief-output.json'),'utf-8'))
 const output={...source,id:'output-readable',caseId:'case-1',revisionId:'revision-1',superseded:false}
 await page.route('**/api/v1/outputs/output-readable',r=>json(r,output))
 await page.route('**/api/v1/outputs/output-readable/content*',r=>r.fulfill({status:200,contentType:'text/html',body:html}))
 const external:string[]=[];page.on('request',r=>{if(r.frame().url()==='about:srcdoc')external.push(r.url())})
 await page.getByLabel('Retained output identifier',{exact:true}).fill(output.id)
 await page.getByRole('button',{name:'Open retained delivery',exact:true}).click()
 const frame=page.frameLocator('iframe[title="Frozen analytical brief reader"]')
 await expect(frame.getByRole('heading',{name:'Readable brief · تحليل',exact:true})).toBeVisible()
 expect(await frame.locator('script,form,iframe').count()).toBe(0)
 await expect(frame.locator('p').filter({hasText:/Authored uncertainty remains visible/})).toContainText("<script>alert('not executable')</script>")
 await page.context().setOffline(true)
 await expect(frame.getByRole('heading',{name:'Coverage at the frozen cutoff',exact:true})).toBeVisible()
 expect(external).toEqual([])
 await page.context().setOffline(false)
 await page.getByRole('button',{name:'Use exact rendered output for review',exact:true}).click()
 await expect(page.getByLabel('Rendered output identifier (optional)',{exact:true})).toHaveValue(output.id)
 await page.setViewportSize({width:390,height:844})
 const box=await page.locator('iframe[title="Frozen analytical brief reader"]').boundingBox();expect(box?.width).toBeLessThanOrEqual(390)
 const recipient=await page.context().newPage();try{await recipient.route('**/retained-brief.html',r=>r.fulfill({status:200,contentType:'text/html',body:html}));await recipient.goto('/retained-brief.html');await recipient.setViewportSize({width:390,height:844});await page.context().setOffline(true);await expect(recipient.getByRole('heading',{name:'Readable brief · تحليل',exact:true})).toBeVisible();await recipient.screenshot({path:testEvidencePath('readable-brief-mobile.png'),fullPage:true});await recipient.setViewportSize({width:1280,height:900});await recipient.screenshot({path:testEvidencePath('readable-brief-desktop.png'),fullPage:true});await recipient.emulateMedia({media:'print'});await expect(recipient.locator('.reproduction details')).toBeHidden()}finally{await page.context().setOffline(false);await recipient.close()}
})

test('geographic preparation submits explicit qualification and preserves preflight rejection',async({page})=>{
 const state=await setup(page);await saveFirst(page)
 const c=state.cases.get('case-1');state.cases.set(c.id,{...c,revision:{...c.revision,evidence:[{kind:'reference',id:'portable-dem',sourceId:'geographic-source',sha256:'fixture'}]}})
 await page.reload()
 await page.getByRole('combobox',{name:'Deliverable',exact:true}).selectOption('environment')
 await page.getByRole('checkbox',{name:/portable-dem/}).check()
 await page.getByLabel('Local origin · longitude, latitude, ellipsoid height in metres').fill('2.5,48.5,0')
 const qualification={horizontalToleranceM:10,verticalToleranceM:10,method:'Independent reference controls from admitted profile',controls:[{longitude:2.5,latitude:48.5,ellipsoidHeight:100,east:0,north:0,up:100,provenance:'Protocol fixture control'}],maxPixels:1024,maxFeatures:100,maxTriangles:2048,maxBytes:2097152,maxRamBytes:33554432,maxTempBytes:0,maxSeconds:10}
 await page.getByRole('textbox',{name:'Qualification profile JSON',exact:true}).fill(JSON.stringify(qualification))
 let body:any
 await page.route('**/api/v1/investigations/*/delivery-jobs',r=>{if(r.request().method()==='GET')return json(r,{items:[],limit:100});body=r.request().postDataJSON();return r.fulfill({status:400,json:{error:{code:'VALIDATION',message:'Missing qualified transformation grid; no processing job was consumed'}}})})
 await page.getByRole('button',{name:'Queue geographic preparation',exact:true}).click()
 await expect(page.getByRole('status').filter({hasText:'Missing qualified transformation grid'})).toBeVisible()
 expect(body.environment.datasetIds).toEqual(['portable-dem']);expect(body.environment.origin).toEqual({longitude:2.5,latitude:48.5,height:0});expect(body.output.expectedRevisionId).toBe('revision-1');expect(body.environment.qualification).toEqual(qualification)
 await expect(page.getByRole('list',{name:'Durable delivery jobs'})).not.toContainText('SUCCEEDED')
})

test('case autosave persists authored notes and finding fields without an explicit save',async({page})=>{
 const state=await setup(page);await saveFirst(page)
 await page.getByRole('textbox',{name:'Investigation notes',exact:true}).fill('Autosaved after a deliberate pause')
 await expect(page.getByRole('status').filter({hasText:'Saved revision 2'})).toBeVisible()
 expect([...state.cases.values()][0].revision.notes).toBe('Autosaved after a deliberate pause')
 await page.getByRole('button',{name:'Add finding',exact:true}).click()
 await page.getByLabel('Finding title',{exact:true}).fill('Authored conclusion')
 await page.getByLabel('Finding conclusion',{exact:true}).fill('No inference without cited evidence')
 await page.getByLabel('Confidence and rationale',{exact:true}).fill('Uncertain synthetic fixture')
 await page.getByLabel('Uncertainty and competing explanations',{exact:true}).fill('The backend must validate exact supporting revisions')
 await expect(page.getByRole('button',{name:'Use selected case evidence for finding 1'})).toBeDisabled()
})

test('an in-flight case save prevents edits from being overwritten by its response',async({page})=>{
 await setup(page);await saveFirst(page)
 let release:()=>void=()=>{};const pending=new Promise<void>(resolve=>{release=resolve})
 await page.route('**/api/v1/investigations/*/revisions',async route=>{await pending;return route.fallback()})
 const notes=page.getByRole('textbox',{name:'Investigation notes',exact:true})
 await notes.fill('Save this exact note before accepting more edits')
 await page.getByRole('button',{name:'Save investigation',exact:true}).click()
 await expect(notes).toBeDisabled()
 release()
 await expect(page.getByRole('status').filter({hasText:'Saved revision 2'})).toBeVisible()
 await expect(notes).toBeEnabled()
 await expect(notes).toHaveValue('Save this exact note before accepting more edits')
})

test('lost case save response recovers the original intent after reopening',async({page})=>{
 const state=await setup(page);await saveFirst(page)
 await page.getByLabel('Autosave valid changes after a pause').uncheck()
 state.loseNextSave=true
 await page.getByRole('textbox',{name:'Investigation notes',exact:true}).fill('Durably saved despite a lost response')
 await page.getByRole('button',{name:'Save investigation',exact:true}).click()
 await expect(page.getByRole('alert').filter({hasText:'Response lost after durable save'})).toBeVisible()
 const keys=await page.evaluate(()=>Object.keys(localStorage).filter(k=>k.startsWith('emad.unsent-case.v1:')))
 expect(keys).toHaveLength(1)
 await page.reload()
 await expect(page.getByRole('button',{name:'Retry recovered save',exact:true})).toBeVisible()
 await page.getByRole('button',{name:'Retry recovered save',exact:true}).click()
 await expect(page.getByRole('status').filter({hasText:'Recovered save acknowledged by server.'})).toBeVisible()
 expect([...state.cases.values()][0].revision.version).toBe(2)
 await expect(page.getByRole('textbox',{name:'Investigation notes',exact:true})).toHaveValue('Durably saved despite a lost response')
 expect(await page.evaluate(()=>Object.keys(localStorage).filter(k=>k.startsWith('emad.unsent-case.v1:')))).toEqual([])
})

test('assigned reviewer opens exact saved content and requests changes',async({page,browser})=>{
 const state=await setup(page);await saveFirst(page)
 await page.getByLabel('Member human identifiers, comma separated').fill('operator-01, operator-02')
 await page.getByRole('button',{name:'Save membership',exact:true}).click()
 await expect(page.getByRole('status').filter({hasText:'Membership saved'})).toBeVisible()
 await page.getByLabel('Assigned reviewer human identifier').selectOption('operator-02')
 await page.getByLabel('Review reason / disagreement').fill('Please check the uncertainty in this exact saved revision')
 await page.getByRole('button',{name:'Request independent review',exact:true}).click()
 await expect(page.getByText(/Current revision 1 · REVIEW REQUESTED/)).toBeVisible()
 const context=await browser.newContext({baseURL:new URL(page.url()).origin})
 try{const reviewer=await context.newPage();await setup(reviewer,state,'operator-02');await reviewer.goto(page.url())
 await expect(reviewer.getByRole('textbox',{name:'Investigation notes',exact:true})).toHaveValue('A note retained on the server.')
 await expect(reviewer.getByRole('region',{name:'Assigned case reviews'})).toContainText('revision-1')
 await reviewer.getByLabel('Review reason / disagreement').fill('Clarify the evidence limitations before approving this revision')
 await reviewer.getByRole('button',{name:'Request changes',exact:true}).click()
 await expect(reviewer.getByText(/Current revision 1 · CHANGES REQUESTED/)).toBeVisible()
 expect([...state.cases.values()][0].workflow.actorHumanId).toBe('operator-02')
 await reviewer.screenshot({path:testEvidencePath('independent-review-fixture.png'),fullPage:true})
 }finally{await context.close()}
})

test('saved investigation and membership resume in a different browser context',async({page,browser})=>{
 const state=await setup(page);await saveFirst(page)
 await page.getByLabel('Member human identifiers, comma separated').fill('operator-01, operator-02')
 await page.getByRole('button',{name:'Save membership',exact:true}).click()
 await expect(page.getByRole('status').filter({hasText:'Membership saved'})).toBeVisible()
 const savedURL=page.url(),second=await browser.newContext({baseURL:new URL(savedURL).origin})
 try{const other=await second.newPage();await setup(other,state,'operator-02');await other.goto(savedURL);await expect(other.getByRole('textbox',{name:'Investigation notes',exact:true})).toHaveValue('A note retained on the server.');await other.getByRole('button',{name:'Resume saved area and period on map'}).click();await expect(other.getByRole('navigation',{name:'Selected geographic area'})).toContainText('Portable study');expect(await other.evaluate(()=>localStorage.getItem('investigation'))).toBeNull();await other.setViewportSize({width:1440,height:1100});await other.getByRole('button',{name:'Maximize map workbench',exact:true}).click();await other.screenshot({path:testEvidencePath('saved-investigation.png'),fullPage:true})}finally{await second.close()}
})
test('a conflicting case save preserves the unsaved draft until deliberate reload',async({page})=>{
 const state=await setup(page);await saveFirst(page);state.conflict=true
 await page.getByRole('textbox',{name:'Investigation notes',exact:true}).fill('My unsaved conflicting note')
 await page.getByRole('button',{name:'Save investigation',exact:true}).click()
 await expect(page.getByRole('alert').filter({hasText:'Investigation changed'})).toBeVisible()
 await expect(page.getByRole('textbox',{name:'Investigation notes',exact:true})).toHaveValue('My unsaved conflicting note')
 page.once('dialog',dialog=>dialog.accept())
 await page.getByRole('button',{name:'Reload saved revision · discard unsaved edits'}).click()
 await expect(page.getByRole('textbox',{name:'Investigation notes',exact:true})).toHaveValue('A note retained on the server.')
})
test('membership loss clears an already displayed protected case on access refresh',async({page})=>{
 const state=await setup(page);await page.clock.install();await saveFirst(page);state.denied=true
 await page.clock.fastForward(10_001)
 await expect(page.getByText('Workspace access changed. Reconnect to refresh your authorization.',{exact:true})).toBeVisible()
 await expect(page.getByRole('textbox',{name:'Investigation notes',exact:true})).toHaveCount(0)
})
test('a reference import retry preserves its durable request identity and area',async({page})=>{
 await setup(page)
 await page.route('**/api/v1/geography/availability?*',r=>json(r,{regionId:country.id,sourceId:'geography-source',releaseId:'release-a',logicalKey:'country:AAA',regions:0,places:0,state:'partial_reference_depth',meaning:'Source navigation only; not local detail.',nextAction:'request_reference_import'}))
 const bodies:any[]=[]
 await page.route('**/api/v1/geography/reference-requests',r=>{bodies.push(r.request().postDataJSON());return bodies.length===1?r.fulfill({status:500,json:{error:{code:'CONFLICT',message:'Response lost after saving; retry the same request.'}}}):json(r,{id:'georeq-retained',state:'REQUESTED'},201)})
 await page.goto(url('geography',true));await expect(page.getByText('0 retained source subdivisions · 0 retained place points')).toBeVisible()
 await page.getByLabel('Reason for reference depth').fill('Need subdivision and populated-place navigation for this study.')
 await page.getByRole('button',{name:'Request reference import',exact:true}).click();await expect(page.getByRole('status').filter({hasText:'Response lost after saving'})).toBeVisible()
 await page.getByRole('button',{name:'Request reference import',exact:true}).click();await expect(page.getByRole('status').filter({hasText:'georeq-retained'})).toBeVisible()
 expect(bodies).toHaveLength(2);expect(bodies[0]).toEqual(bodies[1]);expect(bodies[0].regionId).toBe(country.id)
})
test('geographic search follows explicit continuation without dropping prior results',async({page})=>{
 await setup(page);const second={...country,id:'geo-country-2',properties:{...country.properties,name:'Second fixture country',regionKey:'country:BBB'}}
 await page.route('**/api/v1/catalog/reads',r=>{const body=r.request().postDataJSON();if(body.kind!=='geography'||!body.filters?.query)return r.fallback();const more=!!body.cursor;return json(r,{snapshotId:'saved-geographic-read',kind:'geography',items:[more?second:country],total:2,returned:1,offset:more?1:0,state:'available',representation:'retained_records',hasMore:!more,nextCursor:more?undefined:'opaque-second-page',evaluatedAt:'2026-01-01T00:00:00Z',expiresAt:'2099-01-01T00:00:00Z'})})
 await page.goto(url('geography'));await page.getByRole('searchbox',{name:'Find a country, region or place'}).fill('fixture')
 await page.getByRole('button',{name:'More geographic results',exact:true}).click()
 await expect(page.getByRole('button',{name:/Second fixture country.*country/})).toBeVisible()
 await expect(page.getByRole('button',{name:/^Fixture country.*country/})).toBeVisible()
})
test('profile approval and execution grants stay separate and name the exact revision',async({page})=>{
 await setup(page)
 const admission={id:'admission-default',sourceId:'source-profiles',version:1,approvedBy:'operator-01',approvedAt:'2026-01-01T00:00:00Z',originalProducer:'Synthetic test producer',upstreamProvider:'Synthetic test provider',connectorId:'configured-fixture',connectorPrincipalId:'worker-fixture',dataset:'fixture-product',productVersion:'v1',coverageDescription:'Existing reviewed fixture product',analystQuestion:'Default reviewed question',revisionPolicy:'provider_version',enabled:true,rights:{id:'fixture-terms',legalBasis:'Owned synthetic fixture',termsUrl:'https://fixture.invalid/terms',attribution:'Synthetic protocol fixture',retainRaw:true,allowAnalysis:true,allowSimulationExport:false},policy:{aoi:{west:2,south:48,east:3,north:49},lookbackSeconds:86400,freshForSeconds:60,unavailableAfterSeconds:120,minIntervalSeconds:5,timeoutSeconds:10,maxPages:1,maxRecordsPerPage:10,maxResponseBytes:4096,maxRequestsPerDay:20}}
 const profiles:any[]=[{id:'rpr_admission-default',key:'default',sourceId:admission.sourceId,admissionId:admission.id,version:1,admission}];let approved:any,execution:any;const providerCalls:string[]=[]
 page.on('request',r=>{if(r.url().includes('/connector-runs'))providerCalls.push(r.url())})
 await page.route('**/api/v1/sources?*',r=>json(r,list([{id:admission.sourceId,name:'Synthetic profile source',type:'external',status:'active',metadata:{}}])))
 await page.route('**/api/v1/sources/source-profiles/profiles',r=>json(r,{items:profiles,limit:64}))
 await page.route('**/api/v1/sources/source-profiles/profiles/study',r=>{approved=r.request().postDataJSON();const a={...approved.admission,id:'admission-study',sourceId:admission.sourceId,version:2,approvedBy:'operator-01',approvedAt:'2026-01-02T00:00:00Z'};profiles.push({id:'rpr_admission-study',key:'study',sourceId:admission.sourceId,admissionId:a.id,version:2,admission:a});return json(r,a,201)})
 await page.route('**/api/v1/sources/source-profiles/profiles/study/execution',r=>{execution=r.request().postDataJSON();return json(r,{profileRevisionId:execution.profileRevisionId,enabled:execution.enabled})})
 await page.goto(url('sources'));await page.getByRole('combobox',{name:'Profile source',exact:true}).selectOption(admission.sourceId)
 await page.getByRole('textbox',{name:'Profile key',exact:true}).fill('study');await page.getByLabel('Reviewed analytical purpose').fill('Inspect observations within the approved local study area.')
 await page.getByLabel('Approved bounds · west,south,east,north').fill('2,48,3,49');await page.getByRole('button',{name:'Approve profile revision',exact:true}).click()
 await expect(page.getByRole('status').filter({hasText:'Profile revision approved'})).toBeVisible()
 expect(approved.expectedRevisionId).toBe('');expect(approved.admission.id).toBeUndefined();expect(approved.admission.sourceId).toBeUndefined();expect(providerCalls).toEqual([])
 await page.getByRole('textbox',{name:'Service principal identifier',exact:true}).fill('worker-fixture');await page.getByRole('button',{name:'Grant service execution',exact:true}).click()
 await expect(page.getByRole('status').filter({hasText:'Execution approved for this revision and service.'})).toBeVisible()
 expect(execution).toEqual({profileRevisionId:'rpr_admission-study',principalId:'worker-fixture',enabled:true});expect(providerCalls).toEqual([])
})
