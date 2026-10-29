import '@/styles/scope-workflows.css'
import {useRef,useState} from 'react'
import {useQuery,useQueryClient} from '@tanstack/react-query'
import {useAPI} from '@/lib/api'
import {useWorkspace} from '@/lib/workspaceContext'
import {useAnalystScope} from './AnalystScopeProvider'

export function ReferenceDepthPanel(){
 const api=useAPI(),{prefix,session}=useWorkspace(),scope=useAnalystScope()?.snapshot,geo=scope?.area.geography
 const availability=useQuery({queryKey:[...prefix,'geographic-depth',geo?.regionId,geo?.releaseId],queryFn:({signal})=>api.getGeographicAvailability(geo!.regionId,geo?.releaseId,signal),enabled:!!geo})
 const [reason,setReason]=useState(''),[message,setMessage]=useState(''),[pending,setPending]=useState(false),intent=useRef({input:'',key:''})
 if(!geo)return <p>Choose an admitted country or subdivision to inspect its reference depth. Drawn areas retain their own geometry.</p>
 return <section aria-label="Reference depth" className="scope-workflow"><h3>Reference depth</h3>{availability.isError?<p role="status">Reference availability could not be checked.</p>:availability.data&&typeof availability.data.regions==='number'?<><p>{availability.data.regions} retained source subdivisions · {availability.data.places} retained place points</p><p>{availability.data.state==='reference_depth_retained'?'Subdivision and place navigation is retained.':'Additional depth is unavailable or not release-qualified. Detailed roads, buildings and elevation require separate admitted inputs.'}</p><p className="ux-muted">{availability.data.meaning}</p></>:<p>Checking retained reference depth…</p>}
 {session.permissions.includes('geography.request')&&<form onSubmit={async e=>{e.preventDefault();const input=JSON.stringify([geo.regionId,reason]);if(intent.current.input!==input)intent.current={input,key:crypto.randomUUID()};setPending(true);try{const result=await api.requestReferenceDepth({regionId:geo.regionId,reason,requestKey:intent.current.key});setMessage(`Reference request saved: ${result.id}. A source administrator must prepare and admit the requested detail.`)}catch(e){setMessage(e instanceof Error?e.message:'Request failed; retry preserves this request identity.')}finally{setPending(false)}}}><label>Reason for reference depth<textarea required minLength={10} maxLength={2000} value={reason} onChange={e=>setReason(e.target.value)}/></label><button disabled={pending||reason.trim().length<10}>Request reference import</button></form>}
 {message&&<p role="status">{message}</p>}</section>
}

export function ReferenceRequestsPanel({active}:{active:boolean}){
 const api=useAPI(),{prefix,session}=useWorkspace(),client=useQueryClient(),[message,setMessage]=useState(''),[pending,setPending]=useState('')
 const requests=useQuery({queryKey:[...prefix,'reference-depth-requests'],queryFn:({signal})=>api.listReferenceDepthRequests(signal),enabled:active&&session.permissions.includes('sources.manage')})
 if(!session.permissions.includes('sources.manage'))return null
 return <section aria-label="Reference import requests" className="scope-workflow"><h2>Reference import requests</h2><p>Prepare a source-backed regional profile with the geography installer, admit all chunks, then activate its complete release. Requests do not grant retrieval authority. Verification below requires the requested subdivision and place depth in the activated release.</p>{requests.isError&&<p role="alert">Reference requests unavailable.</p>}{requests.data?.items?.map(r=><article key={r.id}><h3>{r.logicalKey}</h3><p>{r.input.reason}</p><p>{r.resolution?'Installed reference verified':'Awaiting source preparation and admission'} · {r.id}</p>{!r.resolution&&<button disabled={!!pending} onClick={async()=>{setPending(r.id);try{const heads=await api.listGeographicReleaseHeads();const release=heads.items.find(h=>h.sourceId===r.sourceId)?.releaseId;if(!release)throw new Error('No complete release is active for this source.');await api.resolveReferenceDepthRequest(r.id,release);await client.invalidateQueries({queryKey:[...prefix,'reference-depth-requests']});setMessage('Installed source release verified.')}catch(e){setMessage(e instanceof Error?e.message:'Verification failed')}finally{setPending('')}}}>Verify installed release</button>}</article>)}{message&&<p role="status">{message}</p>}</section>
}
