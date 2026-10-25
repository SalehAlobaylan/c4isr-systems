import { useQueries } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { createAPI } from '@/lib/api'
import { resourceKey, useWorkspace } from '@/lib/workspaceContext'
import { dateLabel } from './preferences'

/** Ordinary COP grants only. No authoring scenes or truth are requested for discovery. */
export function RecordingLibrary({onOpen}:{onOpen?:()=>void}) {
  const {session,prefix,signal}=useWorkspace()
  const resources=[...(session.resources??[])].filter(r=>r.partition==='EXERCISE_OBSERVATION'&&r.runId&&r.runId!=='legacy').reverse().slice(0,24)
  const queries=useQueries({queries:resources.map(resource=>({queryKey:[...prefix,'recording-library',resourceKey(resource)],queryFn:async()=>{const api=createAPI(resource,signal);const auth=await api.getCurrentOperator();if(!auth.permissions.includes('sim.recordings.read'))return null;try{return await api.getRecording(resource.runId!)}catch(e){if(e&&typeof e==='object'&&'status'in e&&e.status===404)return null;throw e}},retry:false,staleTime:15_000}))})
  const entries=queries.flatMap((q,i)=>q.data?[{index:q.data,resource:resources[i]}]:[])
  return <section className="space-y-4" aria-label="Recording library"><div><h2 className="text-lg font-semibold">Recording library</h2><p className="ux-muted">Up to 24 authorized exercises from your current grants. Only permitted recorded output is used for these previews.</p></div>{queries.some(q=>q.isPending)&&<p role="status">Checking retained recordings…</p>}
    <div className="grid gap-3 sm:grid-cols-2">{entries.map(({index,resource})=><article key={resource.runId} className="studio-panel"><span className="ux-status">{index.status}</span><h3 className="mt-3 font-semibold">Exercise · {resource.runId!.slice(-8)}</h3><p className="ux-muted">{index.manifest?`${(index.manifest.elapsedMs/1000).toFixed(1)} virtual seconds · ${index.manifest.events.toLocaleString()} events`:`Archive attempt ${index.attempts} / ${index.maxAttempts}`}</p>{index.manifest&&<p className="ux-muted">Retained through {dateLabel(index.manifest.retainUntil)}</p>}<Link className="ux-button mt-3" to="/recordings" search={{run:resource.runId,ms:0,sequence:0}} onClick={onOpen}>Open recorded exercise →</Link></article>)}</div>
    {!queries.some(q=>q.isPending)&&!entries.length&&<p>No retained recording was found among your current exercise grants. Complete an admitted run in Studio; the recording worker archives closed runs automatically.</p>}
    {queries.some(q=>q.isError)&&<p role="alert">Some recording status could not be checked. Verify the connection and current run grants.</p>}
    {resources.length===24&&<p className="ux-muted">The library is bounded to 24 current grants. Other permitted runs can be opened from their Exercise workspace.</p>}
  </section>
}
