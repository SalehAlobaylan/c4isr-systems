import {test,expect} from '@playwright/test'
import {readFileSync,mkdirSync} from 'node:fs'
import path from 'node:path'

test.skip(process.env.EMAD_FIRMS_ACCEPTANCE!=='1','Requires the explicitly activated persistent FIRMS source')

test('FIRMS source explains partial day-page backfill without claiming complete coverage',async({page,request})=>{
 const root=path.resolve('../..'),token=readFileSync(path.join(root,'private/local/credentials/local-analyst.token'),'utf8').trim()
 const key=readFileSync(path.join(root,'private/local/firms.key'),'utf8').trim()
 const proof=JSON.parse(readFileSync(path.join(root,'private/local/data/firms-activation.json'),'utf8'))
 const response=await request.get('http://127.0.0.1:8081/api/v1/sources/'+proof.run.sourceId+'/activation',{headers:{Authorization:'Bearer '+token,'X-EMAD-Workspace-ID':'default','X-EMAD-Environment-ID':'local','X-EMAD-Partition':'PUBLIC_EVIDENCE'}})
 expect(response.ok()).toBeTruthy()
 const activation=await response.json()
 expect(activation.state).toBe('CONFIGURED')
 const pageErrors:string[]=[];page.on('pageerror',e=>pageErrors.push(e.message))
 await page.addInitScript(value=>sessionStorage.setItem('emad.session-token',value),token)
 await page.goto('/systems')
 await page.getByRole('button',{name:'Sources & access',exact:true}).click()
 const source=page.getByRole('article').filter({has:page.getByRole('heading',{name:'NASA FIRMS · NOAA-20 thermal detections',exact:true})})
 await expect(source).toContainText('CONFIGURED')
 await expect(source).toContainText('Next retrieval attempt:')
 if(activation.health.failureCode==='page_budget'&&!activation.health.lastSuccessAt){
  await expect(source).toContainText('being backfilled')
  await expect(source).toContainText('complete query coverage has not been established yet')
 }
 if((await page.locator('body').innerText()).includes(key))throw new Error('A private provider credential was exposed to the browser')
 await source.scrollIntoViewIfNeeded()
 const out=path.join(root,'docs/implementation/evidence/firms');mkdirSync(out,{recursive:true})
 await source.screenshot({path:path.join(out,'source-status.png')})
 expect(pageErrors).toEqual([])
})
