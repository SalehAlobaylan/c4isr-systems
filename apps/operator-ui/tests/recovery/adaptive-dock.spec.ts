import {chooseFeed,chooseWorkbench} from '../navigation'
import {test,expect,type Page} from '@playwright/test'
import {readFileSync,mkdirSync,writeFileSync} from 'node:fs'
import path from 'node:path'
import {mapControl} from './atlas-helpers'

const root=path.resolve('../..'),out=path.join(root,'docs/design/map-dock')
test.skip(process.env.EMAD_RECOVERY_ACCEPTANCE!=='1','Requires the persistent installation and supported task dev')
test.beforeEach(async({page})=>{mkdirSync(out,{recursive:true});await page.addInitScript(token=>sessionStorage.setItem('emad.session-token',token),readFileSync(path.join(root,'private/local/credentials/local-analyst.token'),'utf8').trim())})
async function ready(page:Page){await page.goto('/map');await expect(page.locator('[data-map-ready=true]')).toBeVisible();await expect.poll(async()=>Number(await page.locator('.atlas-canvas').getAttribute('data-public-feature-count'))).toBeGreaterThan(0)}
async function layout(page:Page,name:string){await page.getByLabel('Workbench layout',{exact:true}).click();await page.getByRole('button',{name,exact:true}).click()}

test('a 44px toolbar and shallow previews preserve map space; detailed desktop work uses the side',async({page})=>{
 await ready(page)
 const map=page.locator('.atlas-map-region'),dock=page.getByRole('region',{name:'Map preview dock'}),initial=(await map.boundingBox())!,workspace=(await page.locator('#workspace-content').boundingBox())!
 expect((await dock.boundingBox())!.height).toBeLessThanOrEqual(44)
 expect(initial.height/workspace.height).toBeGreaterThan(.87)
 await page.screenshot({path:path.join(out,'01-toolbar.png')})
 await chooseFeed(page,'imagery')
 await expect(page.locator('.preview-record').first()).toBeVisible();expect((await dock.boundingBox())!.height).toBeLessThanOrEqual(130)
 await page.screenshot({path:path.join(out,'06-preview-tray.png')})
 await mapControl(page,'Map layers')
 await expect(page.locator('.atlas-stage')).toHaveAttribute('data-orientation','vertical')
 const side=(await dock.boundingBox())!,area=(await map.boundingBox())!
 expect(area.height).toBeGreaterThanOrEqual(initial.height);expect(area.width).toBeGreaterThanOrEqual(workspace.width*.7)
 expect(side.width).toBeLessThanOrEqual(400);expect(area.x+area.width).toBeLessThanOrEqual(side.x+1)
 await page.screenshot({path:path.join(out,'02-side-layers.png')})
 writeFileSync(path.join(out,'layout-acceptance.json'),JSON.stringify({capturedAt:new Date().toISOString(),viewport:page.viewportSize(),toolbarHeight:44,previewHeight:124,workspaceHeight:workspace.height,initialMapHeight:initial.height,initialMapHeightFraction:initial.height/workspace.height,desktopPanelWidth:side.width,desktopMapHeightWithPanel:area.height,desktopMapWidthFraction:area.width/workspace.width},null,2))
})

test('bottom placement is bounded and larger views require explicit user actions',async({page})=>{
 await ready(page);await chooseWorkbench(page,'systems')
 const dock=page.getByRole('region',{name:'Map preview dock'}),map=page.locator('.atlas-map-region')
 await expect(dock.locator('.system-domain')).toHaveCount(6)
 await expect(dock.locator('.domain-image-pair img')).toHaveCount(2)
 await expect.poll(()=>dock.locator('.domain-image-pair img').evaluateAll(images=>images.every(image=>(image as HTMLImageElement).complete&&(image as HTMLImageElement).naturalWidth>0))).toBeTruthy()
 await page.screenshot({path:path.join(out,'03-side-systems.png')})
 await layout(page,'Bottom dock');await expect(page.locator('.atlas-stage')).toHaveAttribute('data-orientation','horizontal')
 const bottom=(await dock.boundingBox())!,area=(await map.boundingBox())!,stage=(await page.locator('.atlas-stage').boundingBox())!
 expect(bottom.height).toBeLessThanOrEqual(250);expect(area.height/stage.height).toBeGreaterThan(.65)
 await layout(page,'Automatic layout');await expect(page.locator('.atlas-stage')).toHaveAttribute('data-orientation','vertical')
 const compact=(await dock.boundingBox())!.width
 await layout(page,'Expand preview dock');await expect(dock).toHaveAttribute('data-size','expanded');expect((await dock.boundingBox())!.width).toBeGreaterThan(compact)
 await layout(page,'Maximize map workbench');await expect(dock).toHaveAttribute('data-size','full')
 await page.screenshot({path:path.join(out,'07-full-grid.png')})
 await page.reload();await expect(dock).toHaveAttribute('data-size','preview');await expect(page.locator('.atlas-stage')).toHaveAttribute('data-orientation','vertical')
})

test('phone sheets are bounded and short-landscape menus do not overlap the scope bar',async({page})=>{
 await page.setViewportSize({width:390,height:844});await ready(page)
 const dock=page.getByRole('region',{name:'Map preview dock'}),map=page.locator('.atlas-map-region')
 expect((await dock.boundingBox())!.height).toBeLessThanOrEqual(44)
 await page.screenshot({path:path.join(out,'04-mobile-toolbar.png')})
 await mapControl(page,'Map layers');await expect(page.locator('.atlas-stage')).toHaveAttribute('data-orientation','horizontal')
 expect((await dock.boundingBox())!.height).toBeLessThanOrEqual(248)
 const stage=(await page.locator('.atlas-stage').boundingBox())!;expect((await map.boundingBox())!.height/stage.height).toBeGreaterThan(.6)
 await page.screenshot({path:path.join(out,'05-mobile-panel.png')})
 await page.keyboard.press('Escape');await expect(page.getByLabel('Map controls',{exact:true})).toBeFocused()
 await page.setViewportSize({width:768,height:390})
 await page.getByLabel('Map controls',{exact:true}).click()
 const popup=(await page.locator('.atlas-controls-menu > .atlas-toolbar-menu-body').boundingBox())!,scope=(await page.getByLabel('Public map scope',{exact:true}).boundingBox())!
 expect(popup.y).toBeGreaterThanOrEqual(scope.y+scope.height)
 await page.getByRole('button',{name:'Map layers',exact:true}).click();await expect(page.locator('.atlas-stage')).toHaveAttribute('data-orientation','vertical')
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBeTruthy()
 await page.setViewportSize({width:700,height:390})
 const compactWidth=(await dock.boundingBox())!.width
 await layout(page,'Expand preview dock')
 expect((await dock.boundingBox())!.width).toBeGreaterThanOrEqual(compactWidth)
})

test('imagery results retain readable width in side, maximized and phone panels',async({page})=>{
 await ready(page)
 await chooseWorkbench(page,'imagery')
 await page.getByRole('navigation',{name:'Intelligence workspaces'}).getByRole('link',{name:'Analysis & processing',exact:true}).click()
 const result=page.getByRole('region',{name:'Selected imagery job'}),heading=result.getByRole('heading').first()
 await expect(heading).toBeVisible()
 for(const mode of ['side','full','phone']){
  if(mode==='full')await layout(page,'Maximize map workbench')
  if(mode==='phone'){await layout(page,'Return tool to map');await page.setViewportSize({width:390,height:844})}
  await expect(heading).toBeVisible()
  await expect.poll(async()=>(await heading.boundingBox())?.width??0).toBeGreaterThan(200)
  await result.scrollIntoViewIfNeeded()
  if(mode==='side')await page.screenshot({path:path.join(out,'08-side-analysis.png')})
  await expect(page.getByLabel('Before prepared subset')).toBeAttached()
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBeTruthy()
 }
})
