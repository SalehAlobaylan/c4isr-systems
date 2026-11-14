import type {FeatureCollection} from 'geojson'
import { createContext, useContext, useCallback, type ReactNode } from 'react'
import { Link, useNavigate, useSearch } from '@tanstack/react-router'
import type {ScopeInput,ScopeSnapshot} from '@/lib/api'
import {useAnalystScope,frozenInput} from './AnalystScopeProvider'

export type WorkbenchSearch = {scope?:string;selected?:string; view?:string;job?:string;assessment?:string;bbox?:string;before?:string;after?:string;id?:string;kind?:'WEATHER'|'REPORT';model?:string;upload?:string;display?:string;manifest?:string;feature?:string;layers?:string;basemap?:string;atlasArea?:string;terrainPoint?:string }
export type MapTool = 'systems'|'investigations'|'coverage'|'geography'|'imagery'|'review'|'context'|'weather'|'tracks'|'sources'|'jobs'|'simulation'|'environments'
export const toolNames:Record<MapTool,string> = {investigations:'Investigations',coverage:'Coverage & readiness',systems:'Systems',geography:'Geography',imagery:'Imagery & analysis',review:'Review desk',context:'Context packages',weather:'Weather & reports',tracks:'Aircraft & vessels',sources:'Sources & access',jobs:'Processing',simulation:'Simulation & replay',environments:'Environment Library'}
export const toolPermissions:Partial<Record<MapTool,string>>={investigations:'investigations.read',coverage:'sources.read',geography:'sources.read',imagery:'sources.read',review:'assessments.read',context:'context.export',weather:'sources.read',tracks:'tracks.read',sources:'sources.read',jobs:'sources.read',environments:'environments.read'}
export type WorkbenchHost = {previewFootprints?:(features:FeatureCollection)=>void;scope?:ScopeSnapshot;scopeInput?:ScopeInput;selectionKey?:string;visible?:boolean;search:WorkbenchSearch;regionId?:string;geometry?:string;operationGeometry?:string;areaError?:string;regionName:string;open:(tool:MapTool,search?:WorkbenchSearch)=>void;update:(search:WorkbenchSearch,replace?:boolean)=>void;close:()=>void;authoring?:boolean;showOnMap?:(selected:string)=>void;showAtlas?:()=>void}
export const WorkbenchContext=createContext<WorkbenchHost|null>(null)
export const useWorkbenchHost=()=>useContext(WorkbenchContext)
export function useWorkbenchRoute(route:'/intelligence'|'/weather-reports'|'/external-tracks'|'/simulation-assets') {
  const host=useWorkbenchHost(), analyst=useAnalystScope(), routeSearch=useSearch({strict:false}) as WorkbenchSearch, routeNavigate=useNavigate()
  const search=host?.search??routeSearch
  type Options={search?:WorkbenchSearch|((old:WorkbenchSearch)=>WorkbenchSearch);replace?:boolean;to?:string}
  const navigate=useCallback(async(options:Options)=>{
    if(host&&!options.to){host.update(typeof options.search==='function'?options.search(search):options.search??search,options.replace);return}
    await routeNavigate({to:options.to??route,search:options.search as never,replace:options.replace})
  },[host,search,routeNavigate,route])
  return {search,navigate,host,scope:host?host.scope:analyst?.snapshot,scopeInput:host?.scope?frozenInput(host.scope):analyst?.input,scopeError:host?host.areaError:analyst?.error,active:host?.visible!==false}
}
export function WorkbenchLink({to,search,children,className,returnToMap,...props}:{to:string;search?:Record<string,unknown>|((old:WorkbenchSearch)=>WorkbenchSearch);children:ReactNode;className?:string;'data-primary'?:boolean;returnToMap?:boolean}){
  const host=useWorkbenchHost(),analyst=useAnalystScope()
  const values=typeof search==='function'?search(host?.search??{}):search
  const target:MapTool|undefined=to==='/systems'?'systems':to==='/weather-reports'?'weather':to==='/external-tracks'?'tracks':to==='/intelligence'?values?.view==='review'?'review':values?.view==='context'?'context':'imagery':to==='/map'?'geography':undefined
  const carried=host?.scope?frozenInput(host.scope):analyst?.input
  const linkSearch=target||to==='/map'?{...(analyst?.searchFor(carried)??{}),...values,bbox:undefined}:values
  return <Link to={to} search={linkSearch as never} className={className} {...props} onClick={e=>{if(host&&to==='/map'&&!e.ctrlKey&&!e.metaKey&&!e.shiftKey&&!e.altKey){if(typeof values?.selected==='string'&&host.showOnMap){e.preventDefault();host.showOnMap(values.selected);return}if(returnToMap){e.preventDefault();host.close();return}}if(host?.authoring&&to==='/simulation-studio'&&!e.ctrlKey&&!e.metaKey&&!e.shiftKey&&!e.altKey){e.preventDefault();host.close();return}if(host&&target&&!e.ctrlKey&&!e.metaKey&&!e.shiftKey&&!e.altKey){e.preventDefault();host.open(target,values as WorkbenchSearch)}}}>{children}</Link>
}
