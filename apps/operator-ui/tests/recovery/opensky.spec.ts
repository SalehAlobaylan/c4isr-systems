import {test,expect} from '@playwright/test'
import {readFileSync,mkdirSync} from 'node:fs'
import path from 'node:path'

test.skip(process.env.EMAD_OPENSKY_ACCEPTANCE!=='1','Requires the explicitly activated persistent OpenSky installation')

test('OpenSky source displays its actual last-reported balance separately from the daily allowance',async({page,request})=>{
 const root=path.resolve('../..'),token=readFileSync(path.join(root,'private/local/credentials/local-analyst.token'),'utf8').trim()
 const proof=JSON.parse(readFileSync(path.join(root,'private/local/data/opensky-activation.json'),'utf8'))
 const response=await request.get('http://127.0.0.1:8081/api/v1/sources/'+proof.run.sourceId+'/activation',{headers:{Authorization:'Bearer '+token,'X-EMAD-Workspace-ID':'default','X-EMAD-Environment-ID':'local','X-EMAD-Partition':'PUBLIC_EVIDENCE'}})
 expect(response.ok()).toBeTruthy()
 const activation=await response.json()
 expect(activation.state).toBe('CONFIGURED')
 expect(activation.providerCredits.remaining).toBe(proof.run.retrieval.creditsRemaining)
 const pageErrors:string[]=[];page.on('pageerror',e=>pageErrors.push(e.message))
 await page.addInitScript(value=>sessionStorage.setItem('emad.session-token',value),token)
 await page.goto('/systems')
 await page.getByRole('button',{name:'Sources & access',exact:true}).click()
 const source=page.getByRole('article').filter({has:page.getByRole('heading',{name:'External aircraft observations · OpenSky',exact:true})})
 await expect(source).toContainText('CONFIGURED')
 await expect(source).toContainText('OpenSky states credits remaining (last response): '+activation.providerCredits.remaining)
 await expect(source).toContainText('The daily allowance is separate from this balance.')
 await source.scrollIntoViewIfNeeded()
 const out=path.join(root,'docs/implementation/evidence/opensky');mkdirSync(out,{recursive:true})
 await source.screenshot({path:path.join(out,'source-status.png')})
 expect(pageErrors).toEqual([])
})
