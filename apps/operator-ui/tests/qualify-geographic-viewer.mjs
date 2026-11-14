import assert from 'node:assert/strict'
import {writeFile} from 'node:fs/promises'
import {chromium} from '@playwright/test'

const [address, output, previewProof] = process.argv.slice(2)
if (!address || !output || !process.env.EMAD_VIEWER_HARDWARE) {
  throw new Error('Usage: node tests/qualify-geographic-viewer.mjs <exact preview URL> <result JSON>. Set EMAD_VIEWER_HARDWARE first.')
}
const target = new URL(address)
if (!['localhost', '127.0.0.1', '[::1]'].includes(target.hostname)) throw new Error('Use the private loopback staging viewer.')
const descriptor = JSON.parse(target.searchParams.get('workbench'))
const expectedPreview = previewProof ? JSON.parse(previewProof) : undefined
assert.equal(descriptor.view, 'environment-3d')
assert.match(descriptor.display, /^geodisplay_[a-f0-9]{64}$/)

const baselines = [
  {name:'desktop', viewport:{width:1440,height:900}, deviceScaleFactor:1, cpu:1, startupMs:3000, frameMs:33.3},
  {name:'compact portrait', viewport:{width:390,height:844}, deviceScaleFactor:2, cpu:1, startupMs:5000, frameMs:50},
  {name:'compact landscape', viewport:{width:740,height:360}, deviceScaleFactor:2, cpu:1, startupMs:5000, frameMs:50},
]
const percentile = (values, n) => [...values].sort((a,b)=>a-b)[Math.ceil(values.length*n)-1]
const headed = process.env.EMAD_VIEWER_HEADED === 'true'
const browser = await chromium.launch({headless:!headed, channel: headed ? 'chromium' : undefined})
const results = []
let gpu
const browserVersion = browser.version()
try {
  const session = await browser.newBrowserCDPSession()
  gpu = (await session.send('SystemInfo.getInfo')).gpu.devices
  for (const baseline of baselines) {
    const context = await browser.newContext({viewport:baseline.viewport, deviceScaleFactor:baseline.deviceScaleFactor, serviceWorkers:'block', reducedMotion:'reduce'})
    const page = await context.newPage()
    const cdp = await context.newCDPSession(page)
    await cdp.send('Emulation.setCPUThrottlingRate',{rate:baseline.cpu})
    const external = [], failures = [], responseBytes = new Map(), seen = []
    const payload = {metadata:0,display:0,font:0,failed:0}
    const responseTypes = new Map()
    // Route only other hosts. Routing local requests would disable the HTTP
    // cache and turn the warm sample into a second cold load.
    const origin = target.origin.replace(/[.*+?^${}()|[\]\\]/g,'\\$&')
    await cdp.send('Network.enable')
    cdp.on('Network.responseReceived', ({requestId,type,response}) => {
      const url = new URL(response.url)
      if (url.origin !== target.origin) return
      const category = type === 'Font' ? 'font' : /\/environment-displays\/[^/]+\/content$/.test(url.pathname)
        ? 'display' : /\/environment-(?:releases|displays)\//.test(url.pathname) ? 'metadata' : undefined
      if (category) responseTypes.set(requestId,response.status >= 400 ? 'failed' : category)
    })
    cdp.on('Network.loadingFinished', ({requestId,encodedDataLength}) => {
      const category = responseTypes.get(requestId)
      if (category) payload[category] += encodedDataLength
      responseTypes.delete(requestId)
    })
    await cdp.send('Network.setBlockedURLs',{urlPatterns:[
      {urlPattern: `${target.origin}/*`,block:false},{urlPattern:'http://*',block:true},{urlPattern:'https://*',block:true},
    ]})
    const local = new RegExp(`^${origin}(?:/|$)`)
    page.on('request', request=>{if (/^https?:\/\//.test(request.url()) && !local.test(request.url())) external.push(new URL(request.url()).origin)})
    page.on('response', response => {
      const path = new URL(response.url()).pathname
      if (!/\/environment-displays\/[^/]+\/content$/.test(path)) return
      const id = path.split('/').at(-2)
      const item = {id, status: response.status(), bytes: Number(response.headers()['content-length'] ?? 0)}
      responseBytes.set(id, item)
      seen.push(item)
    })
    page.on('pageerror', error=>failures.push(error.message))
    const panel = page.getByRole('region',{name:'Environment Library'})
    const canvas = panel.getByLabel('Interactive reference terrain')
    const readReady = async () => {
      await panel.getByText('Verified reference terrain',{exact:true}).waitFor({timeout:30000})
      await canvas.waitFor({state:'visible'})
    }
    const measure = async (cold) => {
      await cdp.send('Network.setCacheDisabled',{cacheDisabled:cold})
      const start = performance.now()
      await page.goto(address,{waitUntil:'domcontentloaded'})
      await readReady()
      return performance.now()-start
    }
    const cold = await measure(true)
    assert.equal(seen[0]?.id, descriptor.display)
    assert.equal(seen.length, 1, 'Terrain must be the only asset requested before layer opt-in')
    let terrainPoint
    if (expectedPreview) {
      await canvas.focus()
      await page.keyboard.press('Enter')
      const point = await panel.getByText(/^Terrain point -?\d+\.\d{6}°,/).textContent({timeout:10000})
      await panel.getByRole('button',{name:'Locate terrain point in Atlas'}).click()
      const atlasPoint = await page.getByRole('complementary',{name:'Environment selection'}).getByText(/^Terrain point ·/).textContent()
      const [lng,lat] = atlasPoint.match(/-?\d+\.\d{6}/g).map(Number)
      const [west,south,east,north] = expectedPreview.bounds
      assert.ok(lng>=west&&lng<=east&&lat>=south&&lat<=north)
      assert.ok(point.includes(`${lng.toFixed(6)}°, ${lat.toFixed(6)}°`))
      terrainPoint = [lng,lat]
    }
    await measure(false) // Prime local HTTP asset cache for the warm sample.
    const warm = await measure(false)
    for (const [label, summary] of [['Optical imagery',/^Optical imagery ·/],['Roads',/^Roads ·/],['Building footprints',/^Building footprints ·/],['Water',/^Water ·/]]) {
      await panel.getByRole('checkbox',{name:label,exact:true}).check()
      await panel.locator('summary').filter({hasText:summary}).waitFor({timeout:15000})
    }
    if (expectedPreview) {
      const roadSummary=panel.locator('summary').filter({hasText:/^Roads ·/})
      const roadDetails=roadSummary.locator('..')
      await roadSummary.click()
      const selector = roadDetails.getByLabel('Inspect feature')
      const featureID = await selector.locator('option').nth(1).getAttribute('value')
      assert.ok(featureID)
      await selector.selectOption(featureID)
      await canvas.focus()
      await page.keyboard.press('ArrowLeft')
      assert.equal(await selector.inputValue(),featureID)
      await roadDetails.getByRole('button',{name:'Locate feature in Atlas'}).click()
      const [edition,...sourceID] = featureID.split(':')
      assert.match(edition,/^[0-9a-f]{64}$/)
      await page.getByRole('complementary',{name:'Environment selection'}).getByText(sourceID.join(':'),{exact:false}).first().waitFor()
      await page.getByRole('button',{name:'Reopen 3D preview',exact:true}).click()
      await readReady()
      await selector.waitFor({state:'visible'})
      assert.equal(await selector.inputValue(),featureID)
    }
    const renderedResources = () => canvas.evaluate(element=>new Promise(resolve => {
      requestAnimationFrame(()=>requestAnimationFrame(()=>resolve({...element.dataset})))
    }))
    const initialResources = await renderedResources()
    const rendererInfo = await canvas.evaluate(element => {
      const gl = element.getContext('webgl2')
      const debug = gl?.getExtension('WEBGL_debug_renderer_info')
      return debug ? gl.getParameter(debug.UNMASKED_RENDERER_WEBGL) : 'Unavailable'
    })
    const activeFrames = await canvas.evaluate(element=>new Promise(resolve => {
      const samples = []
      const bounds = element.getBoundingClientRect()
      const x = bounds.x + bounds.width/2, y = bounds.y + bounds.height/2
      let previous = 0
      const step = time => {
        element.dispatchEvent(new WheelEvent('wheel',{deltaY:samples.length%2 ? -10 : 10,clientX:x,clientY:y,cancelable:true,bubbles:true}))
        if (previous) samples.push(time-previous)
        previous = time
        if (samples.length >= 100) resolve(samples)
        else requestAnimationFrame(step)
      }
      requestAnimationFrame(step)
    }))
    const repetitions = baseline.name === 'desktop' ? 30 : 0
    const cycles = []
    for (let index=0; index<repetitions; index++) {
      await panel.getByRole('button',{name:'Return to Atlas',exact:true}).click()
      await canvas.waitFor({state:'detached'})
      await page.getByRole('button',{name:'Reopen 3D preview',exact:true}).click()
      await readReady()
      await panel.locator('summary').filter({hasText:/^Optical imagery ·/}).waitFor()
      await panel.locator('summary').filter({hasText:/^Water ·/}).waitFor()
      cycles.push(await renderedResources())
    }
    await panel.getByRole('button',{name:'Return to Atlas',exact:true}).focus()
    await page.keyboard.press('Enter')
    await canvas.waitFor({state:'detached'})
    await cdp.send('Performance.enable')
    const heap = (await cdp.send('Performance.getMetrics')).metrics.find(metric=>metric.name==='JSHeapUsedSize')?.value
    // Chromium reports frame timestamps with 0.1 ms precision. Compare at
    // that resolution to avoid treating 33.30000000000018 as over 33.3 ms.
    const p95 = Number(percentile(activeFrames,.95).toFixed(1))
    const peak = key => Math.max(Number(initialResources[key]),...cycles.map(cycle=>Number(cycle[key])))
    results.push({baseline,rendererInfo,coldMs:cold,warmMs:warm,p95FrameMs:p95,activeFrames,initialResources,
      peakResources:{geometries:peak('geometryCount'),textures:peak('textureCount'),drawCalls:peak('drawCalls')},
      cycles, bytes:[...responseBytes.values()], requests:seen.length, retries:seen.filter(item=>item.status>=400).length, transferBytes:payload, terrainPoint,
      jsHeapUsedBytes:heap,external,failures,budgetsMet:cold<=baseline.startupMs&&warm<=baseline.startupMs&&p95<=baseline.frameMs&&
      external.length===0&&failures.length===0&&cycles.every(cycle=>Number(cycle.geometryCount)<=Number(initialResources.geometryCount)&&Number(cycle.textureCount)<=Number(initialResources.textureCount))})
    await context.close()
  }
} finally {
  await browser.close()
  await writeFile(output, JSON.stringify({schema:'emad.viewer-qualification.v1',checkedAt:new Date().toISOString(),
    hardware:process.env.EMAD_VIEWER_HARDWARE, gpu, browser:browserVersion, headed, network:'Loopback API and assets, external HTTP blocked',
    release:descriptor.id, manifest:descriptor.manifest, results},null,2)+'\n')
}
console.log(JSON.stringify(results.map(({baseline,coldMs,warmMs,p95FrameMs,budgetsMet})=>({baseline:baseline.name,coldMs,warmMs,p95FrameMs,budgetsMet}))))
if (results.some(result=>!result.budgetsMet)) process.exitCode=1
