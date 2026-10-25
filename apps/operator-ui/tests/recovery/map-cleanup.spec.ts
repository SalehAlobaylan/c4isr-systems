import {chooseWorkbench} from '../navigation'
import {mapControl,expectMapControlFocus} from './atlas-helpers'
import {test,expect,type Page} from '@playwright/test'
import {readFileSync,mkdirSync,writeFileSync} from 'node:fs'
import path from 'node:path'

const root=path.resolve('../..'),out=path.join(root,'docs/design/map-cleanup')
test.skip(process.env.EMAD_RECOVERY_ACCEPTANCE!=='1','Requires the persistent installation and supported task dev')
test.beforeEach(async({page})=>{mkdirSync(out,{recursive:true});await page.addInitScript(token=>sessionStorage.setItem('emad.session-token',token),readFileSync(path.join(root,'private/local/credentials/local-analyst.token'),'utf8').trim())})
async function ready(page:Page){await page.goto('/map');await expect(page.locator('[data-map-ready=true]')).toBeVisible();await expect.poll(async()=>Number(await page.locator('.atlas-canvas').getAttribute('data-public-feature-count'))).toBeGreaterThan(0)}

test('initial map reserves its canvas for geography and puts summaries and tools in one compact dock',async({page})=>{
 const errors:string[]=[];page.on('pageerror',e=>errors.push(e.message));await ready(page)
 const dock=page.getByRole('region',{name:'Map preview dock'}),map=page.locator('.atlas-map-region'),main=page.locator('#workspace-content')
 await expect(dock).toHaveAttribute('data-size','collapsed')
 await expect(map.locator('.atlas-situation,.atlas-peek,.atlas-flyout,.atlas-rail,.atlas-map-actions')).toHaveCount(0)
 await expect(dock.getByLabel('Show area situation',{exact:true})).toBeVisible();await expect(page.locator('.atlas-capabilities')).toHaveCount(0)
 const area=(await map.boundingBox())!,workspace=(await main.boundingBox())!,ratio=area.height/workspace.height
 expect(ratio).toBeGreaterThan(.8)
 await page.screenshot({path:path.join(out,'01-clean-map.png')})
 writeFileSync(path.join(out,'layout-acceptance.json'),JSON.stringify({capturedAt:new Date().toISOString(),viewport:page.viewportSize(),workspaceHeight:workspace.height,mapHeight:area.height,mapHeightFraction:ratio,defaultDockHeight:(await dock.boundingBox())!.height,persistentPanelsOnCanvas:0},null,2))
 expect(errors).toEqual([])
})

test('controls and evidence reuse the dock while the underlying workbench retains its selected record',async({page})=>{
 const errors:string[]=[];page.on('pageerror',e=>errors.push(e.message));await ready(page)
 const dock=page.getByRole('region',{name:'Map preview dock'})
 await chooseWorkbench(page,'weather');await dock.locator('.record-list>button').first().click()
 const evidence=dock.getByRole('article',{name:'Pinned context evidence'})
 await expect(evidence).toBeVisible();const pinned=await evidence.getByRole('heading').first().innerText()
 await mapControl(page,'Map layers')
 const layers=page.getByRole('region',{name:'Map layers'})
 await expect(evidence).toBeHidden();await expect(layers).toBeVisible();await expect(page.getByRole('dialog')).toHaveCount(0)
 const panel=(await layers.boundingBox())!,frame=(await dock.boundingBox())!
 expect(panel.y).toBeGreaterThanOrEqual(frame.y)
 await page.getByRole('button',{name:'Collapse previews',exact:true}).click()
 await mapControl(page,'Map layers');await expect(layers).toBeVisible()
 await page.screenshot({path:path.join(out,'02-shared-layers.png')})
 await page.getByRole('button',{name:'Time & filters',exact:true}).click()
 await expect(layers).toHaveCount(0);await expect(page.getByLabel('Filter public evidence')).toHaveCount(1)
 await page.keyboard.press('Escape');await expect(evidence).toBeVisible();await expect(evidence.getByRole('heading').first()).toHaveText(pinned)
 await chooseWorkbench(page,'systems');await expect(dock.locator('.system-domain')).toHaveCount(6)
 await page.screenshot({path:path.join(out,'03-shared-systems.png')})
 await chooseWorkbench(page,'previews');await page.locator('.preview-record').first().click()
 await expect(dock.getByRole('region',{name:'Selected map object'})).toBeVisible();await expect(page.locator('.atlas-map-region .atlas-peek')).toHaveCount(0)
 await page.screenshot({path:path.join(out,'04-docked-selection.png')})
 await mapControl(page,'Evidence inspector')
 await expect(dock.getByLabel('Public event and reference data')).toBeVisible();await expect(page.getByLabel('Filter public evidence')).toHaveCount(0)
 expect(errors).toEqual([])
})

test('compact controls stay inside phone and short landscape screens, with keyboard return to the map',async({page})=>{
 const errors:string[]=[];page.on('pageerror',e=>errors.push(e.message));await page.setViewportSize({width:390,height:844});await ready(page)
 for(const viewport of [{width:390,height:844},{width:320,height:700},{width:768,height:390}]){
  await page.setViewportSize(viewport)
  await page.getByLabel('Map controls',{exact:true}).click()
  for(const name of ['Map layers','Map tools','Map views','Evidence inspector']){
   const control=(await page.getByRole('button',{name,exact:true}).boundingBox())!
   expect(control.x).toBeGreaterThanOrEqual(0);expect(control.x+control.width).toBeLessThanOrEqual(viewport.width)
   expect(control.y+control.height).toBeLessThanOrEqual(viewport.height)
  }
  await page.keyboard.press('Escape')
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBeTruthy()
  await mapControl(page,'Map layers')
  await expect(page.getByRole('button',{name:'Close map tool',exact:true})).toBeFocused()
  await page.keyboard.press('Escape');await expectMapControlFocus(page,'Map layers')
 }
 await page.setViewportSize({width:390,height:844});await page.screenshot({path:path.join(out,'05-mobile-clean.png')})
 expect(errors).toEqual([])
})
