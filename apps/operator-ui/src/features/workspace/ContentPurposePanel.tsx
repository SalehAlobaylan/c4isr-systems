import {useState} from 'react'
import {useQuery,useQueryClient} from '@tanstack/react-query'
import {useAPI,type ContentPurposeEntry,type ContentPurposeInput} from '@/lib/api'
import {useWorkspace} from '@/lib/workspaceContext'

export function ContentPurposePanel(){
 const api=useAPI(),{prefix}=useWorkspace(),cache=useQueryClient()
 const [view,setView]=useState<'all'|'unclassified'>('unclassified'),[offset,setOffset]=useState(0)
 const [selected,setSelected]=useState<ContentPurposeEntry>(),[purpose,setPurpose]=useState<ContentPurposeInput['purpose']>('unclassified')
 const [reason,setReason]=useState(''),[receipt,setReceipt]=useState(''),[error,setError]=useState(''),[busy,setBusy]=useState(false)
 const rows=useQuery({queryKey:[...prefix,'content-purpose',view,offset],queryFn:()=>api.listContentPurpose(view,offset)})
 const choose=(entry:ContentPurposeEntry)=>{setSelected(entry);setPurpose(entry.purpose);setReason('');setReceipt('');setError('')}
 const save=async()=>{if(!selected)return;setBusy(true);setError('');try{
  await api.classifyContentPurpose({kind:selected.kind as ContentPurposeInput['kind'],entityId:selected.entityId,purpose,reason,receiptReference:receipt,expectedVersion:selected.version})
  setSelected(undefined);await cache.invalidateQueries({queryKey:prefix})
 }catch(e){setError(e instanceof Error?e.message:'Classification could not be saved.')}finally{setBusy(false)}}
 return <section className="studio-panel mb-4" aria-label="Content purpose review">
  <h2>Content purpose review</h2><p>Classify from verified receipts and exact record identities. Purpose is separate from real/synthetic provenance and human review. Classification preserves the original evidence.</p>
  <label>Catalog purpose view <select className="ux-button" value={view} onChange={e=>{setView(e.target.value as typeof view);setOffset(0);setSelected(undefined)}}><option value="unclassified">Needs classification</option><option value="all">All purposes · support history</option></select></label>
  {rows.isError&&<p role="alert">Classification inventory unavailable. <button onClick={()=>void rows.refetch()}>Retry</button></p>}
  <p>{rows.data?.total??'—'} authorized records match.</p>
  <ul>{rows.data?.items.map(entry=><li key={entry.kind+entry.entityId}><button className="ux-button my-1 break-all text-left" onClick={()=>choose(entry)}>{entry.kind} · {entry.purpose} · {entry.entityId}</button></li>)}</ul>
  <div className="ux-actions"><button className="ux-button" disabled={offset===0} onClick={()=>setOffset(v=>Math.max(0,v-100))}>Previous classification page</button><button className="ux-button" disabled={!rows.data?.hasMore} onClick={()=>setOffset(v=>v+100)}>Next classification page</button></div>
  {selected&&<form className="space-y-3 mt-4" onSubmit={e=>{e.preventDefault();void save()}}><h3>Classify {selected.kind}</h3><p className="break-all">{selected.entityId} · expected purpose revision {selected.version}</p>
   <label className="block">Purpose <select className="ux-button" value={purpose} onChange={e=>setPurpose(e.target.value as typeof purpose)}>{['working','development','acceptance','unclassified'].map(v=><option key={v}>{v}</option>)}</select></label>
   <label className="block">Verified receipt or evidence reference<input className="block w-full rounded border border-edge bg-panel p-2" required maxLength={1024} value={receipt} onChange={e=>setReceipt(e.target.value)}/></label>
   <label className="block">Classification reason<textarea className="block w-full rounded border border-edge bg-panel p-2" required minLength={10} maxLength={2000} value={reason} onChange={e=>setReason(e.target.value)}/></label>
   <button className="ux-button" disabled={busy||reason.trim().length<10||!receipt.trim()}>Record classification</button><button type="button" className="ux-button" onClick={()=>setSelected(undefined)}>Cancel classification</button>
  </form>}{error&&<p role="alert">{error}</p>}
 </section>
}
