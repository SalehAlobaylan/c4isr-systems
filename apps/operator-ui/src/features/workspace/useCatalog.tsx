import {useEffect,useRef,useState} from 'react'
import {useInfiniteQuery} from '@tanstack/react-query'
import {fixedScopeInput,useAPI,type ScopeSnapshot} from '@/lib/api'
import type {components} from '@/lib/openapi.generated'
import {useWorkspace} from '@/lib/workspaceContext'
type Input=components['schemas']['CatalogInput']
// Catalogs remain fixed until explicit refresh. Monitoring a selected active job
// uses its shared detail query; it never allocates another catalog per tick.
export function useCatalog<T>(kind:Input['kind'],scope?:ScopeSnapshot,enabled=true,filters:Input['filters']={},limit=100){
 const api=useAPI(),{prefix,session}=useWorkspace(),[expired,setExpired]=useState(false)
 const previous=useRef<string|undefined>(undefined)
 const permission=kind==='investigations'?'investigations.read':kind==='findings'?'assessments.read':'sources.read'
 const query=useInfiniteQuery({queryKey:[...prefix,'catalog',kind,scope?.scopeHash,filters,limit],enabled:enabled&&session.permissions.includes(permission),initialPageParam:undefined as {snapshotId:string;cursor:string}|undefined,
  queryFn:async({pageParam,signal})=>{const page=await api.readCatalog({kind,scope:scope?fixedScopeInput(scope):undefined,filters,limit,...pageParam,...(!pageParam?{replaceSnapshotId:previous.current}:{})},signal,scope);previous.current=page.snapshotId;return page},
  getNextPageParam:last=>last.hasMore&&last.nextCursor?{snapshotId:last.snapshotId,cursor:last.nextCursor}:undefined,
  retry:false,staleTime:Infinity,gcTime:300_000,refetchOnWindowFocus:false,refetchOnReconnect:false})
 const first=query.data?.pages[0]
 useEffect(()=>{setExpired(false);if(!first)return;previous.current=first.snapshotId;const left=new Date(first.expiresAt).getTime()-Date.now();if(left<=0){setExpired(true);return}const timer=setTimeout(()=>setExpired(true),left);return()=>clearTimeout(timer)},[first?.snapshotId,first?.expiresAt])
 return {...query,expired,hasNextPage:query.hasNextPage&&!expired&&!query.isError,data:first&&!expired&&!query.isError?{...first,truncated:query.hasNextPage,features:query.data!.pages.flatMap(p=>p.items),items:query.data!.pages.flatMap(p=>p.items) as unknown as T[]}:undefined,
  restart:()=>query.refetch()}
}
const readFailureMessage=(error:unknown)=>{
 const message=error instanceof Error?error.message:''
 if(message.includes('active read capacity exhausted'))return 'Too many retained views are open. Close other EMAD tabs and retry after their views expire. Saved work is unaffected.'
 return message||'This saved read is unavailable. Refresh or choose a smaller area, period or source.'
}
export function CatalogPaging({catalog,label}:{label:string;catalog:{data?:{items:unknown[];total:number|null;state:string};hasNextPage:boolean;isError?:boolean;error?:unknown;expired?:boolean;isFetching:boolean;fetchNextPage:()=>unknown;refetch:()=>unknown}}){
 return <div className="catalog-paging"><p role="status">{catalog.isError?(readFailureMessage(catalog.error)):catalog.expired?'This saved read expired. Refresh to continue.':catalog.data?`${catalog.data.items.length} shown · ${catalog.data.total===null?'count unavailable':`${catalog.data.total} in saved read`}`:'Loading saved read…'}</p>{catalog.hasNextPage&&!catalog.expired&&<button className="ux-button" disabled={catalog.isFetching} onClick={()=>void catalog.fetchNextPage()}>More {label}</button>}<button className="ux-button" disabled={catalog.isFetching} onClick={()=>void catalog.refetch()}>Refresh {label}</button></div>
}
