import {fixture} from '../engine-evaluation/fixture.mjs'
import { build } from 'vite'
import { chromium } from '@playwright/test'
import { readFile,writeFile,mkdir,readdir,stat } from 'node:fs/promises'
import { createServer } from 'node:http'
import { resolve,dirname,join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { createHash } from 'node:crypto'
import os from 'node:os'
import {execFile} from 'node:child_process'
import {promisify} from 'node:util'
const ui=resolve(dirname(fileURLToPath(import.meta.url)),'..'),root=resolve(ui,'../..'),out=join(root,'tmp/s1-s2/engine-web')
await build({configFile:false,root:join(ui,'engine-evaluation'),build:{outDir:out,emptyOutDir:true,chunkSizeWarningLimit:6000}})
const allowed=new Map([['/fixture.json',Buffer.from(JSON.stringify(fixture))]])
const assets=[];for(const a of fixture.assets){const b=await readFile(join(root,'testdata/models',a.file));allowed.set('/models/'+a.file,b);assets.push({file:a.file,sha256:createHash('sha256').update(b).digest('hex'),bytes:b.length})}
async function load(dir,prefix=''){for(const name of await readdir(dir)){const path=join(dir,name),url=prefix+'/'+name;if((await stat(path)).isDirectory())await load(path,url);else allowed.set(url,await readFile(path))}}
await load(out)
const server=createServer((req,res)=>{const p=new URL(req.url,'http://localhost').pathname,b=allowed.get(p==='/'?'/index.html':p);if(!b){res.writeHead(404).end();return}res.setHeader('Content-Type',p.endsWith('.js')?'text/javascript':p.endsWith('.json')?'application/json':p==='/'?'text/html':'application/octet-stream');res.end(b)})
await new Promise(r=>server.listen(0,'127.0.0.1',r));const origin=`http://127.0.0.1:${server.address().port}`
const hardware=process.env.EMAD_BENCHMARK_GPU==='1'
const browser=await chromium.launch(hardware?{channel:'chromium',headless:true,args:['--use-angle=d3d11','--enable-gpu']}:{headless:true,args:['--enable-unsafe-swiftshader']})
const browserVersion=browser.version()
const trials=[]
try{for(let trial=0;trial<5;trial++)for(const candidate of trial%2?['babylon','three']:['three','babylon']){
 const context=await browser.newContext(),page=await context.newPage(),cdp=await context.newCDPSession(page);page.on('pageerror',e=>console.error(candidate,e));await cdp.send('Performance.enable')
 await page.goto(origin+'/?engine='+candidate);await page.waitForFunction(()=>!!window.evaluation,null,{timeout:60000});const result=await page.evaluate(()=>window.evaluation)
 const metrics=await cdp.send('Performance.getMetrics');result.trial=trial;result.rendererTaskSeconds=metrics.metrics.find(m=>m.name==='TaskDuration')?.value??null
 if(result.maxFloat32PositionErrorM>.002||result.triangles!==13||result.drawCalls!==4||!Number.isFinite(result.loadMs))throw new Error('precision or loading gate failed: '+JSON.stringify(result))
 trials.push(result);await context.close()
}}finally{await browser.close();await new Promise(r=>server.close(r))}
const headless=[]
for(let trial=0;trial<5;trial++)for(const candidate of trial%2?['babylon','three']:['three','babylon']){const {stdout}=await promisify(execFile)(process.execPath,[join(ui,'scripts/evaluate-headless.mjs'),candidate],{windowsHide:true,timeout:15000,maxBuffer:1048576});headless.push({...JSON.parse(stdout.split('\n').find(line=>line.startsWith('EMAD_RESULT ')).slice(12)),trial})}
const result={at:new Date().toISOString(),versions:{three:'0.180.0',babylon:'8.32.0',chromium:browserVersion},machine:{platform:os.platform(),release:os.release(),arch:os.arch(),cpu:os.cpus()[0].model,logicalCPUs:os.cpus().length,memoryBytes:os.totalmem(),node:process.version},method:'5 alternating cold browser contexts per candidate in one browser; same 960x640 DPR1 scene, files and transforms; shader-ready first frame included in load. 120 identical kinematic steps. 13 triangles and 4 draws required. GPU timer null is unavailable, never zero. Five fresh no-GPU Node processes per candidate also load the terrain and run 20000 identical 50-ms waypoint steps; RSS is end-of-test resident memory, not peak. CPU is process CPU milliseconds. Browser heap is privacy-rounded and excludes native/GPU allocations.',assets,fixture,trials,headless}
await mkdir(join(root,'docs/implementation/evidence'),{recursive:true});await writeFile(join(root,'docs/implementation/evidence/S1_2_ENGINE_BENCHMARK'+(hardware?'_HARDWARE':'')+'.json'),JSON.stringify(result,null,2)+'\n')
console.log(JSON.stringify(trials.map(({candidate,loadMs,renderBatchMs,gpuBatchMs,rendererTaskSeconds,maxFloat32PositionErrorM})=>({candidate,loadMs,renderBatchMs,gpuBatchMs,rendererTaskSeconds,maxFloat32PositionErrorM})),null,2))
