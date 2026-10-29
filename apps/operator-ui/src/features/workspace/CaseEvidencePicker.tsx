import {useState} from 'react'
import {useQuery} from '@tanstack/react-query'
import {useAPI,type InvestigationInput,type ScopeInput} from '@/lib/api'
import {useWorkspace} from '@/lib/workspaceContext'
import {useCatalog,CatalogPaging} from './useCatalog'

type Evidence=InvestigationInput['evidence'][number]
export function CaseEvidencePicker({scope,active,selected=[],onPick}:{scope?:ScopeInput;active:boolean;selected?:Evidence[];onPick:(r:Evidence)=>void}){
 const api=useAPI(),{prefix}=useWorkspace(),[open,setOpen]=useState(false),[kind,setKind]=useState<'observation'|'analysis'|'capture'|'assessment'|'reference'>('observation'),[search,setSearch]=useState('')
 const resolved=useQuery({queryKey:[...prefix,'case-picker-scope',scope],queryFn:({signal})=>api.resolveAnalystScope(scope!,signal),enabled:active&&open&&!!scope,staleTime:60000,retry:false})
 const enabled=active&&open&&!!resolved.data
 const records=useQuery({queryKey:[...prefix,'case-picker-reports',resolved.data?.scopeHash],queryFn:({signal})=>api.listPublicEvents({limit:100,revisions:'all'},signal,resolved.data),enabled:enabled&&kind==='observation',retry:false})
 const references=useQuery({queryKey:[...prefix,'reference-datasets'],queryFn:({signal})=>api.listReferenceDatasets(signal),enabled:enabled&&kind==='reference'})
 const catalog=useCatalog<Record<string,any>>(kind==='capture'?'captures':kind==='assessment'?'findings':'jobs',resolved.data,enabled&&['analysis','capture','assessment'].includes(kind),kind==='assessment'?{}:{status:'SUCCEEDED'})
 const query=kind==='observation'?records:kind==='reference'?references:catalog
 const unavailable=resolved.isError||query.isError
 const loading=!!scope&&(resolved.isPending||enabled&&query.isPending)&&!unavailable
 const items=kind==='observation'?(records.data?.items??[]).map(r=>({id:r.evidenceId,label:r.reported.title||r.providerRecordId,source:r.sourceId})):kind==='reference'?(references.data?.items??[]).map(r=>({id:r.id,label:r.label,source:r.sourceId})):(catalog.data?.items??[]).map(r=>({id:String(r.id),label:String(r.conclusion??r.input?.scene?.providerId??r.input?.kind??r.id),source:String(r.sourceId??r.input?.scene?.sourceId??'Retained input')}))
 const matching=items.filter(v=>`${v.label} ${v.source} ${v.id}`.toLocaleLowerCase().includes(search.toLocaleLowerCase()))
 return <fieldset disabled={!active}><details open={open} onToggle={e=>setOpen(e.currentTarget.open)}>
  <summary>Select exact retained evidence</summary>
  <p>Reports and completed imagery or analysis use this draft's area and period. Reference editions come from the workspace catalog. Access and exact revisions are checked again when saving.</p>
  <label>Retained evidence category<select value={kind} onChange={e=>{setKind(e.target.value as typeof kind);setSearch('')}}><option value="observation">Provider reports · exact retained revision</option><option value="analysis">Completed analysis · frozen result</option><option value="capture">Completed imagery capture</option><option value="assessment">Assessment · exact revision</option><option value="reference">Reference edition · workspace catalog</option></select></label>
  {!scope&&<p role="status">Choose the investigation's area and period before browsing evidence.</p>}
  {loading&&<p role="status">Loading authorized retained evidence…</p>}
  {unavailable&&<p role="alert">Evidence discovery is unavailable for this area, period or access. No alternate scope is used. <button type="button" onClick={()=>void (resolved.isError?resolved.refetch():query.refetch())}>Retry evidence</button></p>}
  {!loading&&!unavailable&&enabled&&<>
   {items.length>0?<><label>Search loaded evidence<input type="search" value={search} onChange={e=>setSearch(e.target.value)} placeholder="Title, source or identifier"/></label><p className="ux-muted">Search covers the records loaded below.</p></>:<p role="status">No matching retained evidence was returned. Check coverage or adjust the case's area and period; this does not establish that nothing happened.</p>}
   {items.length>0&&!matching.length&&<p role="status">No loaded records match this search. Try another title or source.</p>}
   <ul className="case-evidence-results">{matching.map(v=>{const attached=selected.some(r=>r.kind===kind&&r.id===v.id);return <li key={v.id}><div><strong><bdi dir="auto">{v.label}</bdi></strong><small>{v.source}</small><details><summary>Exact identifier</summary><code>{v.id}</code></details></div><button type="button" disabled={attached} onClick={()=>onPick({kind,id:v.id})} aria-label={`${attached?'Already attached':'Attach'} ${v.label}`}>{attached?'Attached':'Attach'}</button></li>})}</ul>
   {['analysis','capture','assessment'].includes(kind)&&<CatalogPaging catalog={catalog} label="retained inputs"/>}
   {kind==='observation'&&records.data?.truncated&&<p>First 100 revisions shown. Narrow the draft's area or period to find another exact report.</p>}
  </>}
 </details></fieldset>
}
