import { testEvidencePath } from './evidence-path'
import {expect,test,type Page} from '@playwright/test'
import {json,mockOperationalWorkflow} from './fixtures-operational'
import {mockSession} from './session'
import {resolveScopeFixture} from './fixtures-analyst-scope'

const geometry={type:'Polygon',coordinates:[[[2.3,48.8],[2.32,48.8],[2.32,48.82],[2.3,48.82],[2.3,48.8]]]}
const period={mode:'historical' as const,interval:{from:'2026-01-01T00:00:00Z',to:'2026-01-02T00:00:00Z'},revisionView:'latest' as const,timezone:'UTC',backgroundImagery:false}
const input={area:{kind:'drawn',name:'Coverage study',geometry},temporal:period}
const url=(tool='')=>'/map?scope='+encodeURIComponent(JSON.stringify(input))+(tool?'&tool='+tool:'')
async function setup(page:Page){
 await mockOperationalWorkflow(page)
 const resource={workspaceId:'default',environmentId:'local',partition:'PUBLIC_EVIDENCE'}
 await page.route('**/api/v1/auth/me',r=>json(r,{...mockSession(),principal:{id:'human:operator-01',kind:'human'},operator:{id:'operator-01',name:'Fixture analyst',role:'operator'},resource,resources:[resource],productProfile:'map',permissions:[...mockSession().permissions,'acquisitions.read','acquisitions.request','imagery.jobs.submit','context.export']}))
 await page.route('**/api/v1/map/features?*',r=>json(r,{type:'FeatureCollection',features:[],total:0,state:'latest_retained'}))
}
const source=(dataResult='no_retrieval')=>({id:'approved-source',name:'Approved fixture source',family:'earthquakes',support:'admitted',supportReason:'',capability:{adapter:'usgs-earthquakes-v1',historical:true},profiles:[{key:'regional',revisionId:'profile-revision-1',gate:'ready',spatialRelation:'full',executionGranted:true,rights:{}}],spatialRelation:'full',retrieval:dataResult==='successful_empty_retrieval'?'complete_query_footprint':'no_retrieval',dataResult,usableRetainedRecords:0,completedWindows:[],missingWindows:[period.interval],freshness:'unknown',observationCompleteness:'unknown',receipts:[],receiptsTruncated:false,actions:[{kind:'request_acquisition',sourceId:'approved-source',profileKey:'regional',profileRevisionId:'profile-revision-1',owner:'analyst',reason:'Within approved coverage.'}]})
async function coverage(page:Page,dataResult='no_retrieval'){
 await page.route('**/api/v1/coverage/query',async r=>{const scope=await resolveScopeFixture(page,r.request().postDataJSON().scope);return json(r,{schema:'emad.coverage.v1',scope,evaluatedAt:scope.temporal.evaluatedAt,sources:[source(dataResult)],storage:'available',meaning:'Synthetic protocol fixture; missing records do not establish absence.',recoveryOwner:'source administrator'})})
}
test('lost acquisition response retries the same frozen intent without provider execution',async({page})=>{
 await setup(page);await coverage(page)
 const bodies:any[]=[],providerCalls:string[]=[];let saved:any
 page.on('request',r=>{if(r.url().includes('/connector-runs')||r.url().includes('/scheduler/run'))providerCalls.push(r.url())})
 await page.route('**/api/v1/acquisitions',async r=>{
  if(r.request().method()==='GET')return r.fallback()
  const body=r.request().postDataJSON();bodies.push(body)
  saved={id:'acq-retained-intent',input:body,scope:await resolveScopeFixture(page,body.scope),kind:'ACQUISITION',status:'QUEUED',reason:'Approved frozen scope saved.',recoveryOwner:'connector operator',requesterId:'operator-01',createdAt:'2026-10-26T10:00:00Z',parts:[]}
  return bodies.length===1?json(r,{error:{code:'internal_error',message:'Response lost after durable save'}},500):json(r,saved,201)
 })
 await page.route('**/api/v1/acquisitions/acq-retained-intent',r=>json(r,saved))
 await page.goto(url('systems'));await page.getByRole('button',{name:'Coverage & acquisition',exact:true}).click()
 const panel=page.getByRole('region',{name:'Coverage and acquisition',exact:true})
 await panel.getByText('Manual single-profile intent (advanced)').click()
 await panel.getByLabel('Approved source profile').selectOption('approved-source/regional/profile-revision-1')
 await panel.getByRole('button',{name:'Submit acquisition intent',exact:true}).click()
 await expect(panel.getByRole('status').filter({hasText:'Response lost'})).toBeVisible()
 await panel.getByRole('button',{name:'Retry or reopen same acquisition intent',exact:true}).click()
 await expect(panel.getByRole('status').filter({hasText:'Acquisition saved'})).toBeVisible()
 expect(bodies).toHaveLength(2);expect(bodies[0]).toEqual(bodies[1]);expect(bodies[0].scope.temporal.interval).toEqual(period.interval);expect(providerCalls).toEqual([])
 await page.setViewportSize({width:1440,height:1100})
 await page.getByRole('button',{name:'Maximize map workbench',exact:true}).click()
 await panel.scrollIntoViewIfNeeded();await page.screenshot({path:testEvidencePath('coverage-request.png')})
})
test('historical map reads stay fixed across focus and reconnect until explicit refresh',async({page})=>{
 await setup(page);await coverage(page,'successful_empty_retrieval')
 let reads=0
 page.on('request',r=>{if(r.url().endsWith('/analyst-scope/read')&&r.postDataJSON().families?.length)reads++})
 await page.goto(url());await expect(page.getByRole('navigation',{name:'Selected geographic area'})).toContainText('Coverage study')
 await expect.poll(()=>reads).toBeGreaterThan(0);await page.waitForTimeout(500);const initial=reads
 await page.evaluate(()=>{window.dispatchEvent(new Event('focus'));window.dispatchEvent(new Event('online'));document.dispatchEvent(new Event('visibilitychange'))})
 await page.waitForTimeout(800);expect(reads).toBe(initial)
 await page.getByRole('button',{name:'Refresh display',exact:true}).click();await expect.poll(()=>reads).toBe(initial+1)
 await page.getByRole('button',{name:'Show area situation',exact:true}).click()
 await page.locator('.map-legend-row').filter({hasText:'Earthquake reports'}).click()
 await expect(page.getByText(/Completed provider queries returned no matching retained records/)).toBeVisible()
})
test('a failed family displays an unknown count while successful siblings remain available',async({page})=>{
 await setup(page);await coverage(page)
 await page.route('**/api/v1/analyst-scope/read',async r=>{
  const body=r.request().postDataJSON();if(!body.families?.length)return r.fallback()
  const scope=await resolveScopeFixture(page,body),layers:Record<string,any>={}
  for(const family of body.families)layers[family]={type:'FeatureCollection',features:[],total:family==='weather'?null:family==='earthquakes'?1:0,state:family==='weather'?'unavailable':'latest_retained',scopeHash:scope.scopeHash,evaluatedAt:scope.temporal.evaluatedAt,countMeaning:'Synthetic provider reports'}
  return json(r,{id:'read-sibling-fixture',scope,layers,errors:{weather:{code:'query_unavailable',meaning:'Count unknown',recoveryOwner:'source administrator'}},expiresAt:'2026-10-26T12:00:00Z',coverageReceipts:[],receiptMeaning:'Synthetic protocol fixture'})
 })
 await page.goto(url())
 await page.getByRole('button',{name:'Show area situation',exact:true}).click()
 await expect(page.locator('.map-legend-row').filter({hasText:'Weather'})).toContainText('!')
 await expect(page.locator('.map-legend-row').filter({hasText:'Earthquake reports'})).toContainText('1')
 await page.locator('.map-legend-row').filter({hasText:'Weather'}).click()
 await expect(page.getByText('This query is unavailable. The record count is unknown; other layers remain usable.')).toBeVisible()
})
