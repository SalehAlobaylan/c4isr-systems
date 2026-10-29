import {useEffect,useRef,useState} from 'react'
import {useQuery} from '@tanstack/react-query'
import {fixedScopeInput,useAPI,type ScopeSnapshot,type AcquisitionInput,type CoverageAction,type CoverageResult} from '@/lib/api'
import {useWorkspace} from '@/lib/workspaceContext'
import {useAnalystScope,scopePeriodLabel} from './AnalystScopeProvider'
import '@/styles/scope-workflows.css'

export function useCoverage(scope?:ScopeSnapshot,enabled=true){
 const api=useAPI(),{prefix,session}=useWorkspace(),historical=scope?.temporal.mode==='historical'
 return useQuery({queryKey:[...prefix,'scope-coverage',scope?.scopeHash,scope?.authorizationRevision],queryFn:({signal})=>api.queryCoverage({scope:fixedScopeInput(scope!)},signal),enabled:enabled&&!!scope&&session.permissions.includes('sources.read'),retry:false,staleTime:historical?Infinity:30_000,refetchOnWindowFocus:false,refetchOnReconnect:false,refetchOnMount:!historical,refetchInterval:historical?false:60_000})
}
export function quietMeaning(result:CoverageResult|undefined,family:string,unavailable=false){
 if(unavailable)return 'This query is unavailable. The record count is unknown; other layers remain usable.'
 const sources=result?.sources.filter(s=>s.family===family)??[]
 if(!sources.length)return 'No coverage evaluation is available for this family. Missing records do not establish absence.'
 if(sources.every(s=>s.support==='unsupported'))return 'This family cannot reconstruct the selected historical period. Choose compatible dates or inspect exact retained evidence.'
 if(sources.some(s=>s.dataResult==='unavailable_query'))return 'A source query is unavailable. Its count is unknown; inspect source access and recovery status.'
 if(sources.every(s=>s.dataResult==='successful_empty_retrieval'))return 'Completed provider queries returned no matching retained records for this area and interval. This does not establish factual absence.'
 if(sources.some(s=>s.retrieval==='partial_retrieval'))return 'Retrieval is partial: pages, normalization or publication may be incomplete. Inspect receipts and queued work.'
 if(sources.every(s=>s.retrieval==='no_retrieval'))return 'No completed retrieval is recorded for this area and period. Inspect approved coverage or request acquisition.'
 return 'No matching retained records are displayed. Review retrieval coverage, dates, source freshness and filters.'
}
export function CoveragePanel({scope:provided,active=true,compact=false}:{scope?:ScopeSnapshot;active?:boolean;compact?:boolean}){
 const analyst=useAnalystScope(),scope=provided??analyst?.snapshot,api=useAPI(),{prefix,session,signal}=useWorkspace(),coverage=useCoverage(scope,active)
 const [selected,setSelected]=useState(''),[status,setStatus]=useState(''),[busy,setBusy]=useState(false),[last,setLast]=useState('')
 const intent=useRef<AcquisitionInput|undefined>(undefined)
 const canRequest=session.permissions.includes('acquisitions.request'),canRead=session.permissions.includes('acquisitions.read')
 const requests=useQuery({queryKey:[...prefix,'acquisition-requests'],queryFn:({signal})=>api.listAcquisitions(undefined,signal),enabled:active&&canRead,refetchInterval:active?15_000:false})
 const request=useQuery({queryKey:[...prefix,'acquisition-request',last],queryFn:({signal})=>api.getAcquisition(last,signal),enabled:active&&canRead&&!!last,refetchInterval:active?5_000:false,retry:false})
 useEffect(()=>{intent.current=undefined;setLast('');setSelected('');setStatus('');setBusy(false)},[JSON.stringify(prefix)])
 const actions=coverage.data?.sources.flatMap(source=>source.actions.filter(a=>['request_acquisition','request_coverage_review'].includes(a.kind)))??[]
 const key=(a:CoverageAction)=>`${a.sourceId}/${a.profileKey}/${a.profileRevisionId}`
 const submit=async()=>{
  if(!scope)return
  const action=actions.find(a=>key(a)===selected)
  if(!intent.current){if(!action?.sourceId||!action.profileKey||!action.profileRevisionId)return;intent.current={requestKey:crypto.randomUUID(),sourceId:action.sourceId,profileKey:action.profileKey,profileRevisionId:action.profileRevisionId,purpose:'discovery',scope:fixedScopeInput(scope)}}
  setBusy(true)
  try{const saved=await api.createAcquisition(intent.current);if(signal.aborted)return;setLast(saved.id);setStatus(`${saved.kind==='COVERAGE_REVIEW'?'Coverage review saved':'Acquisition saved'} · ${saved.status}. ${saved.reason}`);void requests.refetch();void coverage.refetch()}
  catch(e){if(signal.aborted)return;setStatus(`${e instanceof Error?e.message:'Submission unavailable'}. Retry preserves this intent, its original area, interval and request key.`)}finally{if(!signal.aborted)setBusy(false)}
 }
 const details=<div className="scope-workflow coverage-details">
  <p>{scopePeriodLabel(coverage.data?.scope??scope)}</p>
  <p className="ux-muted">Provider coverage, retained records, rights and reference detail are separate. Worker health remains in Sources & access.</p>
  {coverage.isPending&&<p role="status">Evaluating selected area and period…</p>}
  {coverage.isError&&<p role="alert">Coverage query unavailable. Counts and readiness are unknown. <button onClick={()=>void coverage.refetch()}>Retry coverage</button></p>}
  {coverage.data&&<><p role="status">Storage: {coverage.data.storage} · evaluated {new Date(coverage.data.evaluatedAt).toLocaleString()}.</p>{coverage.data.storage==='blocked'&&<p role="alert">New acquisition is paused. A source administrator must reconcile storage or the approved retention policy. Retained history stays available.</p>}
   {coverage.data.sources.length===0&&<p>No authorized sources are configured for evaluation. Ask a source administrator about coverage.</p>}
   {coverage.data.sources.map(source=><details key={source.id}><summary>{source.name} · {source.dataResult.replaceAll('_',' ')} · {source.freshness}</summary>
    <p>{source.support} · approved footprint: {source.spatialRelation} · retrieval: {source.retrieval.replaceAll('_',' ')} · retained count: {source.usableRetainedRecords??'unknown'}</p>
    {source.supportReason&&<p>{source.supportReason}</p>}
    <p>Missing query windows: {source.missingWindows.length}. Observation completeness: {source.observationCompleteness}. Provider reports are not independently verified facts.</p>
    {source.profiles.map(profile=><p key={profile.key}>{profile.key} · {profile.gate} · {profile.spatialRelation} overlap · execution {profile.executionGranted?'granted':'requires approval'}</p>)}
    {source.receipts.map(receipt=><p key={receipt.runId}>Query receipt {receipt.runId} · {receipt.queryComplete?'completed query footprint':'partial / unavailable'} · pages {receipt.pages} · rejected {receipt.rejectedRecords} · unpublished {receipt.unpublishedRecords}{receipt.providerTruncated?' · provider response capped':''}</p>)}
    {source.actions.map((action,i)=><p key={i}>{action.reason} Owner: {action.owner}.{action.requestId&&<button onClick={()=>setLast(action.requestId!)}>Inspect request {action.requestId}</button>}</p>)}
   </details>)}<p className="ux-muted">{coverage.data.meaning}</p></>}
  <button disabled={!active||coverage.isFetching} onClick={()=>void coverage.refetch()}>Refresh coverage evaluation</button>
  {canRequest&&<fieldset disabled={busy||!scope||!active||coverage.data?.storage==='blocked'}><legend>Deliberate acquisition or coverage review</legend>
   <label>Approved source profile<select value={selected} disabled={!!intent.current} onChange={e=>setSelected(e.target.value)}><option value="">Choose a source and profile</option>{actions.map(action=><option key={key(action)} value={key(action)}>{coverage.data?.sources.find(s=>s.id===action.sourceId)?.name} · {action.profileKey} · {action.kind==='request_coverage_review'?'review needed':'within approved coverage'}</option>)}</select></label>
   <p>The server saves the exact area, interval, purpose and revision before worker execution. A coverage review does not authorize retrieval.</p>
   <button disabled={!selected&&!intent.current} onClick={()=>void submit()}>{busy?'Saving…':intent.current?'Retry or reopen same acquisition intent':'Submit acquisition intent'}</button>
   {intent.current&&<><p>Pinned intent: {new Date(intent.current.scope.temporal.interval!.from).toLocaleString(undefined,{timeZone:intent.current.scope.temporal.timezone,timeZoneName:'shortOffset'})} → {new Date(intent.current.scope.temporal.interval!.to).toLocaleString(undefined,{timeZone:intent.current.scope.temporal.timezone,timeZoneName:'shortOffset'})}.</p><button onClick={()=>{intent.current=undefined;setSelected('');setStatus('New intent: choose the current map scope and approved profile.')}}>Start a new acquisition intent</button></>}
  </fieldset>}
  {status&&<p role="status">{status}</p>}
  {canRead&&<details open={!!last}><summary>Acquisition and coverage requests</summary>{requests.data?.items.map(item=><button key={item.id} onClick={()=>setLast(item.id)}>{item.scope.area.name} · {item.status} · {new Date(item.createdAt).toLocaleString()}</button>)}{requests.isError&&<p role="alert">Request list unavailable.</p>}
   {request.data&&<section aria-label="Acquisition request detail"><p>{request.data.id} · {request.data.status} · owner: {request.data.recoveryOwner}</p><p>{scopePeriodLabel(request.data.scope)}</p><p>{request.data.reason}</p>{request.data.parts?.map(part=><p key={part.id}>{part.status} · attempt {part.attempts} · {part.reason??''} {part.nextAttemptAt?`Next attempt ${new Date(part.nextAttemptAt).toLocaleString()}`:''} · recovery: {part.recoveryOwner}</p>)}{canRequest&&request.data.status!=='SUCCEEDED'&&request.data.status!=='CANCELLED'&&(request.data.requesterId===session.operator.id||session.permissions.includes('sources.manage'))&&<button onClick={()=>{setBusy(true);void api.cancelAcquisition(last).then(()=>{if(signal.aborted)return;setStatus('Cancellation saved. Already retained evidence remains traceable.');void request.refetch();void requests.refetch()}).catch(e=>{if(!signal.aborted)setStatus(e.message)}).finally(()=>{if(!signal.aborted)setBusy(false)})}}>Cancel unstarted and active request work</button>}</section>}
   {request.isError&&<p role="alert">This request is unavailable under current source access.</p>}
  </details>}
 </div>
 return compact?<details className="coverage-summary"><summary>Coverage & acquisition · {coverage.isError?'unavailable':coverage.data?`${coverage.data.sources.length} authorized sources evaluated`:'evaluating'}</summary>{details}</details>:<section aria-label="Coverage and acquisition"><h2>Coverage for this area and period</h2>{details}</section>
}
