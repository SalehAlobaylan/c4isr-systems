import {useState} from 'react'
import {useQuery} from '@tanstack/react-query'
import {useWorkspace} from '@/lib/workspaceContext'
import type {operations} from '@/lib/openapi.generated'

type Read = operations['readExternalTrackHistory']['parameters']['query']
export type MovingRead = operations['readExternalTrackHistory']['responses'][200]['content']['application/json']

export function useMovingHistory(track: string, query: Read | undefined) {
  const {api,prefix,session}=useWorkspace()
  return useQuery({queryKey:[...prefix,'bounded-moving-history',track,query],
    enabled:!!track && !!query && session.permissions.includes('tracks.read') && session.permissions.includes('observations.read'),
    queryFn:({signal})=>api.readExternalHistory(track,query!,signal),retry:false,gcTime:0})
}

export function MovingHistory({data}: {data: MovingRead}) {
  const seconds=(value:string)=>Date.parse(value)
  const elapsed=seconds(data.to)-seconds(data.from)
  const x=(value:string)=>6+288*(seconds(value)-seconds(data.from))/elapsed
  return <div className="space-y-2">
    <p>{data.meaning}</p>
    {data.truncated&&<p role="status">Only the first 200 records are shown. Narrow the window to freeze revisions.</p>}
    {!data.items.length&&<p role="status">No admitted reports in this area and interval. Coverage is unknown.</p>}
    <svg viewBox="0 0 300 30" role="img" aria-label="Timeline with individual observed samples and shaded unavailable intervals" className="w-full max-w-lg">
      <line x1="6" y1="15" x2="294" y2="15" stroke="currentColor" opacity=".55" />
      {data.gaps.map(gap=><rect key={`${gap.from}:${gap.to}:${gap.reason}`} x={x(gap.from)} y="5" width={Math.max(0,x(gap.to)-x(gap.from))} height="20" fill="#b77848" opacity=".6" />)}
      {data.items.map(item=><circle key={item.evidenceId} cx={x(item.observedAt)} cy="15" r="2.5" fill={item.decision==='QUARANTINED'?'#e87462':'#4ac3ef'} />)}
    </svg>
    <details><summary>{data.gaps.length} gaps · {data.items.length} exact reports</summary>
      <ul>{data.gaps.map(gap=><li key={`${gap.from}:${gap.to}:${gap.reason}`}>{gap.from} to {gap.to}: {gap.reason.replaceAll('_',' ')}</li>)}</ul>
      <ol>{data.items.map(item=><li key={item.evidenceId} className="break-words">
        <strong>{item.decision}</strong> · {item.observedAt} · {item.reported.position.lat}, {item.reported.position.lng}
        <details><summary>Revision {item.providerRevision}</summary><pre className="max-h-64 overflow-auto whitespace-pre-wrap">{JSON.stringify(item,null,2)}</pre></details>
      </li>)}</ol>
    </details>
  </div>
}

export function MovingHistoryLookup({areaRevisionId,from,to,cutoff,trackId,onRead}:{areaRevisionId:string;from:string;to:string;cutoff?:string;trackId?:string;onRead?:(data:MovingRead|undefined)=>void}) {
  const [text,setText]=useState(trackId??'')
  const [requested,setRequested]=useState<{id:string;query:Read}>()
  const read=useMovingHistory(requested?.id??'',requested?.query)
  return <section aria-label="Retained moving history" className="space-y-2">
    <p>Read an exact track over a fixed Atlas interval of up to six hours. Copy a track ID from Aircraft &amp; vessels.</p>
    <label>Track ID<input className="atlas-field" value={text} onChange={event=>setText(event.target.value)} maxLength={128} /></label>
    <button className="ux-button" disabled={!areaRevisionId || !text.trim() || read.isFetching} onClick={()=>{onRead?.(undefined);setRequested({id:text.trim(),query:{areaRevisionId,from,to,cutoff}})}}>Read retained moving reports</button>
    {read.isError&&<p role="alert">{read.error.message}</p>}
    {read.data&&<MovingHistory data={read.data} />}
    {onRead&&read.data&&<button className="ux-button" onClick={()=>onRead(read.data)}>Show reported locations in 3D view</button>}
  </section>
}
