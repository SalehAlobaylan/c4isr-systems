import {createContext,useContext,useEffect,useMemo,useRef,type ReactNode} from 'react'
import {useQuery} from '@tanstack/react-query'
import {useLocation,useNavigate,useSearch} from '@tanstack/react-router'
import {fixedScopeInput,useAPI,type AreaSelection,type ScopeInput,type ScopeSnapshot,type TemporalScope} from '@/lib/api'
import {useWorkspace} from '@/lib/workspaceContext'
import {DEFAULT_PUBLIC_FILTERS,queryFor,readPublicFilters,type PublicFilters} from '@/features/public-context/filters'
import {parseRectangle} from './operationArea'

const timezone=()=>Intl.DateTimeFormat().resolvedOptions().timeZone||'UTC'
export const monitoringPeriod=(days=1):TemporalScope=>({mode:'monitoring',rollingSeconds:days*86400,revisionView:'latest',timezone:timezone(),backgroundImagery:false})
export function rectangleGeometry(b:{west:number;south:number;east:number;north:number}){return {type:'Polygon',coordinates:[[[b.west,b.south],[b.east,b.south],[b.east,b.north],[b.west,b.north],[b.west,b.south]]]}}
type Search=Record<string,unknown>
export function importScope(search:Search,fallback:ScopeInput):{input?:ScopeInput;error?:string}{
 try{
  const legacy=readPublicFilters(typeof search.publicView==='string'?search.publicView:undefined)
  if(legacy.warning)throw new Error('The saved filters are invalid. Choose a new area and period to recover this view.')
  const f=legacy.filters,raw=search.scope
  if(raw!==undefined){
   const input=typeof raw==='string'?JSON.parse(raw):raw
   if(!input||typeof input!=='object'||!input.area||!input.temporal||Object.keys(input).some(k=>!['area','temporal','readSnapshotId'].includes(k)))throw new Error('The saved scope is invalid.')
   if(search.area||search.region&&search.region!=='world'||search.bbox||f.bbox||f.observedFrom||f.observedTo||f.asOf||f.view!=='latest')throw new Error('The link contains conflicting area or period selections. Open one exact scope at a time.')
   return {input:input as ScopeInput}
  }
  let area:AreaSelection|undefined
  if(search.region!==undefined&&typeof search.region!=='string')throw new Error('Invalid geographic identity.')
  if(search.region&&search.region!=='world')area={kind:'administrative',regionId:String(search.region)}
  if(search.area!==undefined){if(area)throw new Error('The link contains both an administrative area and a drawn area.');const geometry=typeof search.area==='string'?JSON.parse(search.area):search.area;if(!geometry||typeof geometry!=='object')throw new Error('Invalid shared geometry.');area={kind:'drawn',name:'Drawn study area',geometry:geometry as Record<string,unknown>}}
  const bbox=search.bbox??f.bbox
  if(bbox){if(area)throw new Error('The link contains conflicting polygon and rectangle scopes.');const b=parseRectangle(String(bbox));if(!b)throw new Error('The saved rectangle is invalid.');area={kind:'viewport',name:'Explicit rectangle',geometry:rectangleGeometry(b)}}
  let temporal=fallback.temporal
  if(f.observedFrom||f.observedTo||f.view!=='latest'){
   queryFor(f)
   if(!f.observedFrom||!f.observedTo)throw new Error('This legacy historical link has an open interval. Set both dates before analytical work; the legacy API remains available with its original interval semantics.')
   temporal={mode:'historical',legacyEndInclusive:true,interval:{from:f.observedFrom,to:f.observedTo},revisionView:f.view,asOf:f.view==='latest'?undefined:f.asOf,timezone:timezone(),backgroundImagery:false}
  }
  const explicitLegacy=['region','area','bbox','publicView'].some(k=>search[k]!==undefined)
  return {input:{area:area??(explicitLegacy?{kind:'world'}:fallback.area),temporal}}
 }catch(e){return {error:e instanceof Error?e.message:'The saved scope is invalid.'}}
}
const nonSpatialFilters=(f:PublicFilters)=>({...f,bbox:'',observedFrom:'',observedTo:'',view:'latest' as const,asOf:'',offset:0})
type ScopeOwner={input?:ScopeInput;snapshot?:ScopeSnapshot;error?:string;resolving:boolean;filters:PublicFilters;searchFor:(input?:ScopeInput,filters?:PublicFilters)=>Record<string,string|undefined>;setScope:(input:ScopeInput,filters?:PublicFilters)=>void;changeArea:(area:AreaSelection)=>void;changePeriod:(temporal:TemporalScope)=>void;refresh:()=>void}
const ScopeContext=createContext<ScopeOwner|null>(null)
export const useAnalystScope=()=>useContext(ScopeContext)
export function AnalystScopeProvider({children}:{children:ReactNode}){
 const api=useAPI(),{prefix,session}=useWorkspace(),search=useSearch({strict:false}) as Search,navigate=useNavigate(),path=useLocation().pathname
 const initial=useRef<ScopeInput>({area:{kind:'world'},temporal:monitoringPeriod()})
 const last=useRef(initial.current)
 const relevant=['/map','/systems','/intelligence','/weather-reports','/external-tracks'].includes(path)
 const rawKey=JSON.stringify([search.scope,search.region,search.area,search.bbox,search.publicView])
 const parsed=useMemo(()=>importScope(search,last.current),[rawKey])
 const input=parsed.input
 useEffect(()=>{if(input)last.current=input},[input])
 const filters=readPublicFilters(typeof search.publicView==='string'?search.publicView:undefined).filters
 const enabled=relevant&&session.resource.partition==='PUBLIC_EVIDENCE'&&session.permissions.includes('sources.read')&&!!input&&!parsed.error
 const resolved=useQuery({queryKey:[...prefix,'analyst-scope',input],queryFn:({signal})=>api.resolveAnalystScope(input!,signal),enabled,retry:false,gcTime:0,staleTime:30_000,refetchInterval:input?.temporal.rollingSeconds?30_000:false})
 const snapshot=enabled&&resolved.data?.authorizationRevision===session.accessRevision?resolved.data:enabled&&!session.accessRevision?resolved.data:undefined
 const searchFor=(next=input,nextFilters=filters)=>({scope:next?JSON.stringify(next):undefined,region:undefined,area:undefined,bbox:undefined,publicView:JSON.stringify(nonSpatialFilters(nextFilters))})
 const setScope=(next:ScopeInput,nextFilters=filters)=>{void navigate({search:old=>({...old,...searchFor(next,nextFilters)}) as never})}
 const error=parsed.error??(resolved.isError?resolved.error.message:resolved.data&&!snapshot?'Scope authorization changed. Reconnect to refresh access.':snapshot?.area.kind==='point'?'This place is a navigation point. Draw an area or create a deliberate buffer before querying evidence.':undefined)
 // Canonicalize ordinary navigation so browser back/forward can restore its
 // area and period even when a header link did not supply search parameters.
 useEffect(()=>{if(enabled&&!['scope','region','area','bbox','publicView'].some(k=>search[k]!==undefined)&&input)void navigate({search:old=>({...old,...searchFor(input)}) as never,replace:true})},[path,rawKey,enabled])
 useEffect(()=>{if(enabled&&snapshot&&!search.scope&&snapshot.area.kind!=='point'&&input){const fixed=fixedScopeInput(snapshot),canonical={area:fixed.area,temporal:input.temporal.rollingSeconds?input.temporal:fixed.temporal};void navigate({search:old=>({...old,...searchFor(canonical)}) as never,replace:true})}},[snapshot?.scopeHash,rawKey,enabled])
 const value:ScopeOwner={input,snapshot,error,resolving:enabled&&resolved.isPending,filters,searchFor,setScope,changeArea:area=>setScope({area,temporal:input?.temporal??monitoringPeriod()}),changePeriod:temporal=>setScope({area:input?.area??{kind:'world'},temporal}),refresh:()=>void resolved.refetch()}
 return <ScopeContext.Provider value={value}>{children}</ScopeContext.Provider>
}
export const scopePeriodLabel=(s?:ScopeSnapshot)=>s?`${s.temporal.mode==='monitoring'?'Monitoring':'Historical study'} · ${new Date(s.temporal.interval.from).toLocaleString(undefined,{calendar:'gregory',timeZone:s.temporal.timezone,timeZoneName:'shortOffset',year:'numeric',month:'short',day:'2-digit',hour:'2-digit',minute:'2-digit',hour12:false})} → ${new Date(s.temporal.interval.to).toLocaleString(undefined,{calendar:'gregory',timeZone:s.temporal.timezone,timeZoneName:'shortOffset',year:'numeric',month:'short',day:'2-digit',hour:'2-digit',minute:'2-digit',hour12:false})} · ${s.temporal.timezone} · Gregorian · end exclusive`:'Resolving period…'
export const emptyPublicFilters=()=>({...DEFAULT_PUBLIC_FILTERS})
export const frozenInput=fixedScopeInput

export const historicalReadOptions=(scope?:ScopeSnapshot)=>({staleTime:scope?.temporal.mode==='historical'?Infinity:0,refetchOnWindowFocus:scope?.temporal.mode!=='historical',refetchOnReconnect:scope?.temporal.mode!=='historical',refetchOnMount:scope?.temporal.mode!=='historical'})
