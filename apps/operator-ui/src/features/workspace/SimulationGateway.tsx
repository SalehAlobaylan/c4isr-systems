import {useState} from 'react'
import {useNavigate} from '@tanstack/react-router'
import {createAPI} from '@/lib/api'
import {resourceKey,useWorkspace} from '@/lib/workspaceContext'
import {WorkspaceChooser} from '@/components/navigation/WorkspaceChooser'

/** Only probes explicit grants; never preloads private scenes or truth. */
export function SimulationGateway(){
 const {session,select,signal}=useWorkspace(),navigate=useNavigate(),[error,setError]=useState(''),[busy,setBusy]=useState(false)
 const exercises=(session.resources??[]).filter(resource=>resource.partition==='EXERCISE_OBSERVATION')
 const open=async(target:'studio'|'assets')=>{setError('');setBusy(true);try{
     const resource={workspaceId:session.resource.workspaceId,environmentId:session.resource.environmentId,partition:'EXERCISE_INPUT' as const}
     const auth=await createAPI(resource,signal).getCurrentOperator();if(!auth.permissions.includes('sim.scenarios.author'))throw new Error('A separate authoring grant is required. No scene or actor data was loaded.')
     await select(resource,true);await navigate({to:target==='studio'?'/simulation-studio':'/simulation-assets'})
 }catch(e){if(!signal.aborted)setError(e instanceof Error?e.message:'Workspace could not be opened.')}finally{if(!signal.aborted)setBusy(false)}}
 return <div className="ux-page"><header className="ux-page-heading"><div><p className="ux-kicker">Simulation · separate workspace & virtual clock</p><h1>Build, exercise & review</h1><p>Enter an authorized workspace deliberately. Public tracks never become controllable actors.</p></div></header><div className="systems-grid">{([['studio','Studio & environments','Place actors in retained real geography, freeze a reproducible scene and start its independent runtime.'],['assets','Models & templates','Inspect validated models, dimensions, licensing and reusable synthetic sensor presets.'],['exercise','Exercise operations','Operate permitted virtual assets against sanitized perceived contacts.'],['replay','Recording, replay & evaluation','Reopen retained virtual-time journals. Privileged spatial comparison requires evaluator access.']] as const).map(([id,title,description])=><section className="system-domain" key={id}><header><h2>{title}</h2></header><div><p>{description}</p></div><footer>{id==='exercise'||id==='replay'?exercises.length?<WorkspaceChooser exerciseOnly resources={exercises} trigger={<span>Choose exercise for {id==='replay'?'replay':'operations'}</span>} onChoose={async key=>{const resource=exercises.find(value=>resourceKey(value)===key);if(!resource)throw new Error('This exercise is no longer available.');if(key!==resourceKey(session.resource))await select(resource,true);await navigate({to:id==='replay'?'/recordings':'/exercise',search:{run:resource.runId}})}}/>:<p className="ux-muted">An exercise observation grant is required.</p>:<button className="ux-button" disabled={busy} onClick={()=>void open(id)}>Open {id==='assets'?'library':id} →</button>}</footer></section>)}</div>{error&&<p role="alert" className="ux-notice mt-4">{error}</p>}</div>
}
