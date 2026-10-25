import {chooseFeed} from '../navigation'
import {mapControl,expectMapControlFocus} from './atlas-helpers'
import {test,expect} from '@playwright/test'
import {readFileSync,mkdirSync} from 'node:fs'
import path from 'node:path'

test.skip(process.env.EMAD_RECOVERY_ACCEPTANCE!=='1','Requires the persistent installation and task dev')
const root=path.resolve('../..'),out=path.join(root,'docs/design/ux-implementation')
const credential=(name='local-analyst')=>readFileSync(path.join(root,'private/local/credentials',name+'.token'),'utf8').trim()
test.beforeEach(async({page})=>{mkdirSync(out,{recursive:true});await page.addInitScript(token=>sessionStorage.setItem('emad.session-token',token),credential())})

test('Systems is a bounded six-domain grid with retained previews and separate source diagnostics',async({page})=>{
  await page.goto('/systems')
  const cards=page.locator('.system-domain');await expect(cards).toHaveCount(6)
  await expect(cards.nth(1).locator('img')).toHaveCount(2)
  await expect.poll(()=>cards.nth(1).locator('img').first().evaluate((e:HTMLImageElement)=>e.naturalWidth)).toBeGreaterThan(100)
  await expect(cards.nth(2)).toContainText('PROPOSED')
  const bounds=await cards.last().boundingBox();expect(bounds!.y+bounds!.height).toBeLessThan(950)
  await page.getByRole('button',{name:'Sources & access',exact:true}).click()
  await expect(page.getByRole('heading',{name:'NASA FIRMS · NOAA-20 thermal detections',exact:true})).toBeVisible()
  await page.getByRole('button',{name:'Overview',exact:true}).click()
  await page.screenshot({path:path.join(out,'systems-desktop.png')})
})

test('map selection, locate, dock geometry and route restoration have distinct effects',async({page})=>{
  await page.goto('/map')
  await expect(page.locator('[data-map-ready=true]')).toBeVisible()
  const map=page.locator('.public-map-canvas'),dock=page.getByRole('region',{name:'Map preview dock'})
  await page.getByRole('button',{name:'Restore previews',exact:true}).click();const first=page.locator('.preview-record').first();await expect(first).toBeVisible()
  const camera=()=>page.evaluate(()=>Object.entries(localStorage).find(([k])=>k.endsWith('.atlas-camera'))?.[1])
  const before=await camera();await first.click();expect(await camera()).toBe(before)
  await page.getByRole('region',{name:'Selected map object'}).getByRole('button',{name:'Locate',exact:true}).click();await expect.poll(camera).not.toBe(before)
  const located=await camera()
  await dock.getByLabel('Workbench layout',{exact:true}).click();await dock.getByRole('button',{name:'Expand preview dock'}).click();await expect(dock).toHaveAttribute('data-size','expanded')
  await mapControl(page,'Evidence inspector')
  await expect(dock.getByLabel('Public event and reference data')).toBeVisible();await expect(page.getByRole('dialog')).toHaveCount(0)
  await dock.getByLabel('Workbench layout',{exact:true}).click();await dock.getByRole('button',{name:'Side dock',exact:true}).click();await expect(page.locator('.atlas-stage')).toHaveAttribute('data-orientation','vertical')
  const side=(await dock.boundingBox())!,canvas=(await map.boundingBox())!;expect(canvas.x+canvas.width).toBeLessThanOrEqual(side.x+1)
  await dock.getByRole('button',{name:'Collapse previews'}).click();await expect(dock).toHaveAttribute('data-size','collapsed')
  await dock.getByRole('button',{name:'Restore previews'}).click()
  await page.getByRole('navigation',{name:'Workspace navigation'}).getByRole('link',{name:'Systems',exact:true}).click()
  await page.getByRole('navigation',{name:'Workspace navigation'}).getByRole('link',{name:'Map',exact:true}).click()
  await expect(page.locator('.preview-record[aria-pressed=true]')).toHaveCount(1)
  await expect.poll(camera).toBe(located)
  await page.getByRole('button',{name:'Focus map',exact:true}).click();await expect(dock).toBeHidden()
  await page.getByRole('button',{name:'Restore workspace',exact:true}).click();await expect(dock).toBeVisible()
})

test('historical investigation never presents current weather or tracks as historical results',async({page})=>{
  const filters={view:'known_at',asOf:'2025-03-31T23:59:00Z',bbox:'39.144,21.464,39.216,21.536'}
  await page.goto('/map?publicView='+encodeURIComponent(JSON.stringify(filters)))
  await chooseFeed(page,'weather')
  await expect(page.getByRole('heading',{name:'Historical query unavailable',exact:true})).toBeVisible()
  await mapControl(page,'Map layers');await page.getByRole('region',{name:'Map layers'}).getByRole('checkbox',{name:/Aircraft/}).check();await page.keyboard.press('Escape');await chooseFeed(page,'aircraft')
  await expect(page.getByRole('heading',{name:'Historical query unavailable',exact:true})).toBeVisible()
  await page.reload();await expect(page.getByLabel('Historical report map')).toBeVisible()
})

test('source failure preserves successful sibling domains without displaying an invented zero',async({page})=>{
  await page.route('**/api/v1/imagery/captures',route=>route.fulfill({status:503,contentType:'application/json',body:JSON.stringify({error:{code:'unavailable',message:'Storage temporarily unavailable'}})}))
  await page.goto('/systems')
  await expect(page.locator('.system-domain').nth(1)).toContainText('Unavailable',{timeout:15000})
  await expect(page.locator('.system-domain').first()).toContainText('24')
  await expect(page.locator('.system-domain').nth(2)).toContainText('PROPOSED')
})

test('imagery comparison links to retained analysis, a review-ready proposal and context export',async({page})=>{
  await page.goto('/intelligence')
  await expect(page.getByRole('region',{name:'Real satellite imagery'}).locator('img').first()).toBeVisible()
  await page.getByRole('button',{name:'Use this pair for reproducible NDVI comparison'}).click()
  await expect(page.getByLabel('Before prepared subset')).not.toHaveValue('')
  await expect(page.getByRole('region',{name:'Aligned NDVI comparison'})).toBeVisible({timeout:15000})
  await page.getByRole('navigation',{name:'Intelligence workspaces'}).getByRole('link',{name:'Assessment review',exact:true}).click()
  await expect(page.getByRole('article',{name:'Selected assessment review'})).toContainText('PROPOSED')
  await expect(page.getByRole('button',{name:'Begin independent review',exact:true})).toHaveCount(0)
  await page.getByRole('navigation',{name:'Intelligence workspaces'}).getByRole('link',{name:'Context packages',exact:true}).click()
  await expect(page.getByRole('region',{name:'Approved context export'})).toBeVisible()
})

test('Studio previews actor edits while retaining the frozen runtime contract',async({page})=>{
  const installed=JSON.parse(readFileSync(path.join(root,'private/local/data/installation.json'),'utf8'))
  await page.goto('/simulation-studio?scene='+installed.simulation.sceneId)
  const viewport=page.getByRole('region',{name:'Georeferenced viewport'})
  await expect(viewport.locator('[data-render-ready]')).toHaveAttribute('data-render-ready','true',{timeout:30000})
  await page.getByLabel('Heading (degrees)',{exact:true}).fill('137')
  await expect(page.getByText('Unfrozen edits · Start uses saved version',{exact:true})).toBeVisible()
  await expect(page.getByRole('button',{name:'Freeze scene version',exact:true})).toBeEnabled()
  await page.getByRole('button',{name:'Asset library',exact:true}).click()
  await expect(page.getByRole('region',{name:'Studio asset workbench'})).toBeVisible()
  await page.getByRole('region',{name:'Studio asset workbench'}).getByRole('button',{name:'Close library',exact:true}).click()
  await expect(page.getByLabel('Heading (degrees)',{exact:true})).toHaveValue('137')
  await expect(viewport.locator('[data-render-ready]')).toHaveAttribute('data-render-ready','true',{timeout:30000})
  await viewport.getByRole('button',{name:'Focus selected actor',exact:true}).click()
  await page.screenshot({path:path.join(out,'studio-desktop.png')})
  await page.getByText(/Actor library ·/).click()
  await expect(page.locator('.studio-library img').first()).toBeVisible({timeout:15000})
})

test('desktop and narrow-screen journeys have no overflow or browser exceptions',async({page})=>{
  const errors:string[]=[];page.on('pageerror',e=>errors.push(e.message))
  const accepted=JSON.parse(readFileSync(path.join(root,'private/local/data/last-accepted-run.json'),'utf8'))
  await page.setViewportSize({width:390,height:844})
  for(const [name,url]of [['home','/'],['systems','/systems'],['map','/map'],['imagery','/intelligence'],['weather','/weather-reports'],['tracks','/external-tracks'],['studio','/simulation-studio'],['replay',`/recordings?run=${accepted.id}&ms=${accepted.elapsedMs}`]]){
    await page.goto(url);await expect(page.locator('#workspace-content')).toBeVisible()
    expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),name).toBeTruthy()
    await page.screenshot({path:path.join(out,name+'-mobile.png')})
  }
  await page.getByRole('button',{name:'Public Context',exact:true}).click();await mapControl(page,'Map layers')
  const layers=page.getByRole('region',{name:'Map layers'});await expect(layers).toBeVisible()
  await expect(layers.getByRole('button',{name:'Close map tool'})).toBeFocused()
  await page.keyboard.press('Escape');await expect(layers).toHaveCount(0)
  await expectMapControlFocus(page,'Map layers')
  expect(errors).toEqual([])
})

test('a short landscape map allocates space for its dock and a 320px screen keeps controls reachable',async({page})=>{
  await page.setViewportSize({width:768,height:390});await page.goto('/map')
  await page.getByRole('button',{name:'Restore previews',exact:true}).click();await expect(page.locator('.atlas-stage')).toHaveAttribute('data-orientation','vertical')
  const main=(await page.locator('#workspace-content').boundingBox())!,dock=(await page.getByRole('region',{name:'Map preview dock'}).boundingBox())!,map=(await page.locator('.public-map-canvas').boundingBox())!
  expect(dock.y+dock.height).toBeLessThanOrEqual(main.y+main.height+1)
  expect(map.x+map.width).toBeLessThanOrEqual(dock.x+1)
  await page.setViewportSize({width:320,height:700})
  for(const name of ['Collapse previews']){const box=(await page.getByRole('button',{name,exact:true}).boundingBox())!;expect(box.x).toBeGreaterThanOrEqual(0);expect(box.x+box.width).toBeLessThanOrEqual(320)}
})
