import {test,expect,type Page} from '@playwright/test'
import {mockOperationalWorkflow} from './fixtures-operational'
import {mockSession} from './session'
import {chooseNavigation,chooseWorkbench,chooseFeed} from '../navigation'

async function setup(page:Page){
 await mockOperationalWorkflow(page)
 const publicResource={workspaceId:'default',environmentId:'local',partition:'PUBLIC_EVIDENCE'}
 const exercises=Array.from({length:50},(_,i)=>({workspaceId:'default',environmentId:'local',partition:'EXERCISE_OBSERVATION',runId:`exercise-${i}`}))
 let resources=[publicResource,...exercises]
 let details=resources.map((resource,i)=>({resource,label:i===0?'Public Context':`Harbor patrol ${String(i-1).padStart(2,'0')}`,kind:i===0?'public':'exercise',...(i===0?{}:{status:i<3?'RUNNING':'STOPPED',startedAt:new Date(Date.UTC(2026,9,20,0,i)).toISOString()})}))
 await page.route('**/api/v1/auth/me',route=>{
  const headers=route.request().headers(),resource=headers['x-emad-partition']==='EXERCISE_OBSERVATION'?exercises.find(r=>r.runId===headers['x-emad-run-id'])??exercises[0]:publicResource
  return route.fulfill({json:{...mockSession(headers),resource,resources,resourceDetails:details}})
 })
 return {publicResource,exercises,rename:(label:string)=>{details=details.map((detail,i)=>i===50?{...detail,label}:detail)},removeLast:()=>{resources=resources.slice(0,-1);details=details.slice(0,-1)}}
}
test.beforeEach(async({},info)=>test.skip(info.project.name!=='no-token','One presentation profile'))

test('large exercise catalogs remain bounded, searchable and pinnable before deliberate switching',async({page})=>{
 const fixture=await setup(page);await page.goto('/map')
 const context=page.getByRole('region',{name:'Workspace context'}),current=await context.getAttribute('data-resource')
 await page.getByRole('button',{name:'Workspace environment and run',exact:true}).click()
 const dialog=page.getByRole('dialog',{name:'Choose a workspace'}),search=dialog.getByRole('searchbox')
 await expect(search).toBeFocused();expect(await dialog.locator('[data-choice]').count()).toBeLessThan(15)
 await search.fill('Harbor patrol 47');await expect(dialog.locator('[data-choice]')).toHaveCount(1)
 await dialog.getByRole('button',{name:/^Pin Harbor patrol 47/}).click()
 await expect(dialog.getByRole('button',{name:/^Unpin Harbor patrol 47/})).toBeFocused()
 await search.fill('');await expect(dialog.getByRole('region',{name:'Pinned'})).toBeVisible()
 await expect(dialog.locator('section[aria-label=Pinned] [data-choice]')).toHaveCount(1)
 await page.keyboard.press('Escape');await expect(context).toHaveAttribute('data-resource',current!)
 await expect(page.getByRole('button',{name:'Workspace environment and run',exact:true})).toBeFocused()
 await chooseNavigation(page,'Workspace environment and run',JSON.stringify(['default','local','EXERCISE_OBSERVATION',fixture.exercises[47].runId]))
 await expect(context).toHaveAttribute('data-resource',JSON.stringify(['default','local','EXERCISE_OBSERVATION','exercise-47']))
})

test('catalog refresh updates labels and removes inaccessible pins without remounting the map',async({page})=>{
 const fixture=await setup(page);await page.goto('/map')
 const country=page.getByRole('searchbox',{name:'Find a country, region or place'})
 await country.fill('Unsubmitted area search')
 await page.getByRole('button',{name:'Workspace environment and run',exact:true}).click()
 const dialog=page.getByRole('dialog'),search=dialog.getByRole('searchbox')
 await search.fill('Harbor patrol 49');await dialog.getByRole('button',{name:/^Pin Harbor patrol 49/}).click()
 fixture.rename('Updated coastal exercise');await search.fill('Updated coastal')
 await page.evaluate(()=>window.dispatchEvent(new Event('focus')))
 await expect(dialog.getByRole('button',{name:/^Open Updated coastal exercise/})).toBeVisible()
 await expect(search).toHaveValue('Updated coastal')
 fixture.removeLast();await page.evaluate(()=>window.dispatchEvent(new Event('focus')))
 await expect(dialog.getByRole('heading',{name:'No matching choices'})).toBeVisible()
 await page.keyboard.press('Escape');await expect(country).toHaveValue('Unsubmitted area search')
})

test('workbenches use purpose and search while unavailable actions stay explicit',async({page})=>{
 await setup(page);await page.goto('/map')
 await page.getByRole('button',{name:'Map workspace',exact:true}).click()
 const dialog=page.getByRole('dialog',{name:'Open a workbench'}),search=dialog.getByRole('searchbox')
 await search.fill('export');await expect(dialog.getByRole('button',{name:'Open Context packages',exact:true})).toBeDisabled()
 await expect(dialog.getByText('Your account needs access to this capability.')).toBeVisible()
 await search.fill('no-such-capability');await expect(dialog.getByRole('heading',{name:'No matching choices'})).toBeVisible()
 await dialog.getByRole('button',{name:'Clear filters',exact:true}).click();await search.fill('satellite')
 await expect(dialog.locator('[data-choice]')).toHaveCount(1)
 await page.keyboard.press('ArrowDown');await page.keyboard.press('Enter')
 await expect(page).toHaveURL(/tool=imagery/)
 await chooseWorkbench(page,'previews');await chooseFeed(page,'weather')
 await expect(page.getByRole('combobox',{name:'Preview category',exact:true})).toContainText('Weather stations')
})

test('phone choosers fit the viewport and Escape preserves the underlying map panel',async({page})=>{
 await setup(page);await page.setViewportSize({width:320,height:700});await page.goto('/map')
 await page.getByLabel('Map controls',{exact:true}).click();await page.getByRole('button',{name:'Map layers',exact:true}).click()
 await page.getByRole('button',{name:'Map workspace',exact:true}).click()
 const dialog=page.getByRole('dialog'),bounds=(await dialog.boundingBox())!
 expect(bounds.x).toBeGreaterThanOrEqual(0);expect(bounds.x+bounds.width).toBeLessThanOrEqual(320)
 expect(bounds.y).toBeGreaterThanOrEqual(0);expect(bounds.y+bounds.height).toBeLessThanOrEqual(700)
 await page.keyboard.press('Escape');await expect(dialog).toBeHidden()
 await expect(page.getByRole('region',{name:'Map layers'})).toBeVisible()
 await expect(page.getByRole('button',{name:'Map workspace',exact:true})).toBeFocused()
 await page.keyboard.press('Escape');await expect(page.getByRole('region',{name:'Map layers'})).toHaveCount(0)
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBeTruthy()
})
