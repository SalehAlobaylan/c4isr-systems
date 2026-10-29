import {useInfiniteQuery} from '@tanstack/react-query'
import {useAPI,type ScopeSnapshot} from '@/lib/api'
import {useWorkspace} from '@/lib/workspaceContext'
export function ReadRecords({readId,scope,family,aggregate,onChoose}:{readId:string;scope:ScopeSnapshot;family:string;aggregate?:string;onChoose:(value:Record<string,unknown>)=>void}){
 const api=useAPI(),{prefix}=useWorkspace()
 const q=useInfiniteQuery({queryKey:[...prefix,'read-records',readId,family,aggregate],initialPageParam:undefined as string|undefined,
  queryFn:({signal,pageParam})=>api.readIndividualRecords(readId,family,aggregate,pageParam,signal,scope),getNextPageParam:p=>p.nextCursor||undefined,
  staleTime:Infinity,retry:false,refetchOnWindowFocus:false,refetchOnReconnect:false,gcTime:0})
 return <section aria-label="Individual retained reports"><h3>Individual retained reports</h3><p>Exact revisions in this saved map read. Area representatives are locators; inspect each reported location and uncertainty.</p>{q.isError?<p role="alert">This saved read is unavailable or expired. Refresh the map before reading records.</p>:<><p role="status">{q.data?.pages[0].total??'…'} reports in this selection</p><ul>{q.data?.pages.flatMap(p=>p.items).map(v=><li key={String(v.evidenceId)}><button className="ux-button" onClick={()=>onChoose(v)}><bdi dir="auto">{String((v.reported as {title?:string})?.title??v.providerRecordId??v.evidenceId)}</bdi> · {(v.reported as {observedAt?:string})?.observedAt?new Date(String((v.reported as {observedAt:string}).observedAt)).toISOString().replace('T',' ').replace('Z',' UTC'):'Reported time unavailable'}</button></li>)}</ul>{q.hasNextPage&&<button className="ux-button" disabled={q.isFetching} onClick={()=>void q.fetchNextPage()}>More individual reports</button>}</>}</section>
}
