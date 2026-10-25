import {useQuery} from '@tanstack/react-query'
import {useAPI,type Source} from '@/lib/api'
import {useWorkspace} from '@/lib/workspaceContext'
import {formatTimestamp} from '@/lib/format'
export function SourceStatus({source}:{source:Source}){
 const api=useAPI(),{prefix}=useWorkspace()
 const state=useQuery({queryKey:[...prefix,'source-activation',source.id],queryFn:()=>api.getSourceActivation(source.id),retry:false,refetchInterval:30000})
 const s=state.data,h=s?.health as Source['ingestion']
 return <article className="source-card"><header><h3>{source.name}</h3><span className="ux-status">{state.isLoading?'Checking…':state.isError?'Status unavailable':s?.state.replaceAll('_',' ')}</span></header>
  {h&&s?.state!=='RETAINED'?<div className="source-health"><span className="ux-status" data-health={h.state}>{h.state.replaceAll('_',' ')}</span><p>{h.lastSuccessAt?'Last success · '+formatTimestamp(h.lastSuccessAt):'No successful retrieval recorded'}</p></div>:s?.state==='RETAINED'?<p className="ux-muted">Retained source history · separately dated evidence</p>:null}
  {s?.nextAction&&<div className="source-next"><span>Next step</span><p>{s.nextAction}</p></div>}
  {state.isError&&<p role="alert" className="ux-muted">Retrieval status unavailable. <button className="underline" onClick={()=>void state.refetch()}>Retry status</button></p>}
  {s&&<details className="source-diagnostics"><summary>Coverage & retrieval details</summary><p>{s.reason}</p>{h?.coverageFrom&&h.coverageTo&&s.state!=='RETAINED'&&<p>Retained query window: {formatTimestamp(h.coverageFrom)} – {formatTimestamp(h.coverageTo)}. This describes the query, not complete regional coverage.</p>}{h?.nextAttemptAt&&s.state!=='RETAINED'&&<p>Next retrieval attempt: {formatTimestamp(h.nextAttemptAt)}</p>}{s.providerCredits&&<p>OpenSky states credits remaining (last response): {s.providerCredits.remaining??'Not reported'}{s.providerCredits.reportedAt?' · '+formatTimestamp(s.providerCredits.reportedAt):''}. The daily allowance is separate from this balance.</p>}{h?.meaning&&s.state!=='RETAINED'&&<p>{h.meaning}</p>}</details>}
 </article>
}
