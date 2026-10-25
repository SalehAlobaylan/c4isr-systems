import {useRef,useReducer,type ReactNode} from 'react'
import {IntelligencePage} from '@/routes/intelligence'
import {WeatherReportsPage} from '@/routes/weather-reports'
import {ExternalTracksPage} from '@/routes/external-tracks'
import {WorkbenchContext,toolPermissions,type WorkbenchHost,type MapTool} from './WorkbenchContext'
import {useWorkspace} from '@/lib/workspaceContext'
import {PublicSystemsWorkspace} from './SystemsWorkspace'
import {SimulationGateway} from './SimulationGateway'

const groupFor=(tool?:MapTool)=>tool==='review'||tool==='context'?'imagery':tool
type Entry={host:WorkbenchHost;routeKey?:string;tool:MapTool}
/** One mounted instance per visited workbench preserves drafts while the user
 * minimizes it or switches tasks. Its scope is pinned until explicitly changed. */
export function MapWorkbenches({tool,host,routeKey,geography}:{tool?:MapTool;host:WorkbenchHost;routeKey?:string;geography:ReactNode}){
 const {session}=useWorkspace()
 const entries=useRef(new Map<string,Entry>()),active=groupFor(tool),current=useRef(active),[,refresh]=useReducer(v=>v+1,0)
 const entering=current.current!==active
 current.current=active
 if(active&&tool){
  const old=entries.current.get(active)
  const previous=old?.host
  let value=previous??host
  if(!old||old.routeKey!==routeKey||old.tool!==tool){
   // History within a workbench restores its exact typed selection, including
   // cleared fields. Reopening a different tool keeps its pinned draft inputs.
   const nextSearch=old&&!entering&&old.tool===tool&&old.routeKey!==routeKey?host.search:routeKey?{...value.search,...host.search,bbox:old?value.search.bbox:host.search.bbox}:{...value.search,view:tool==='review'?'review':tool==='context'?'context':tool==='imagery'?'compare':value.search.view}
   value={...value,search:nextSearch}
  }
  entries.current.set(active,{tool,routeKey,host:{...value,open:host.open,close:host.close,update:(search,replace)=>{const entry=entries.current.get(active)!;entry.host={...entry.host,search};if(current.current===active)host.update(search,replace);else refresh()}}})
 }
 return <>{[...entries.current].map(([key,entry])=><div key={key} hidden={key!==active} className="atlas-workbench-instance"><WorkbenchContext.Provider value={entry.host}>
   <div className="atlas-workbench-scope"><span>{key==='sources'||key==='jobs'?'Workspace':`Workbench area: ${entry.host.regionName}`} {key==='imagery'?' · catalogue overlap; frozen inputs and review retain their original scope':''}</span>{key===active&&(entry.host.regionName!==host.regionName||entry.host.geometry!==host.geometry||entry.host.regionId!==host.regionId)&&<button className="ux-button" onClick={()=>{entries.current.set(key,{...entry,host:{...entry.host,regionName:host.regionName,regionId:host.regionId,geometry:host.geometry,search:{...entry.host.search,bbox:host.search.bbox}}});host.update({...entry.host.search,bbox:host.search.bbox});refresh()}}>Use current map area</button>}</div>
   {toolPermissions[entry.tool]&&!session.permissions.includes(toolPermissions[entry.tool]!)?<p role="status" className="ux-notice">This capability requires {toolPermissions[entry.tool]}. Other permitted map tools remain available.</p>:key==='systems'?<PublicSystemsWorkspace/>:key==='sources'?<PublicSystemsWorkspace initialTab="sources"/>:key==='jobs'?<PublicSystemsWorkspace initialTab="jobs"/>:key==='imagery'?<IntelligencePage/>:key==='weather'?<WeatherReportsPage/>:key==='tracks'?<ExternalTracksPage/>:key==='simulation'?<SimulationGateway/>:geography}
 </WorkbenchContext.Provider></div>)}</>
}
