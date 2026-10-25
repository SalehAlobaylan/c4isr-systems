import {useEffect,useMemo,useRef,useState} from 'react'
import type {Feature,FeatureCollection} from 'geojson'
import {layerInfo,type AtlasLayer} from './AtlasCanvas'

/** Arrival/revision awareness never reclassifies an old observation as a new
 * incident. Aggregates and changing age counters do not invent record arrivals. */
export function MapUpdates({scopeKey,layers,onInspect}:{scopeKey:string;layers:Partial<Record<AtlasLayer,FeatureCollection>>;onInspect:(kind:AtlasLayer,feature:Feature)=>void}){
 const baseline=useRef(new Map<AtlasLayer,Set<string>>()),scope=useRef(scopeKey),[pending,setPending]=useState<{kind:AtlasLayer;feature:Feature}[]>([])
 const current=useMemo(()=>Object.entries(layers).map(([kind,data])=>({kind:kind as AtlasLayer,features:[...new Map((data?.features??[]).filter(f=>!f.properties?.aggregate).map(f=>[String(f.properties?.recordId),f])).values()]})),[layers])
 const signature=(f:Feature)=>`${f.properties?.recordId}:${f.properties?.recordVersion??''}:${f.properties?.reviewStatus??''}`
 const fingerprint=JSON.stringify(current.map(v=>[v.kind,v.features.map(signature)]))
 useEffect(()=>{
   if(scope.current!==scopeKey){scope.current=scopeKey;baseline.current.clear();setPending([])}
   const next:{kind:AtlasLayer;feature:Feature}[]=[]
   for(const entry of current){
     // An empty initial response is still an established baseline. A layer
     // that is not loaded is omitted by the parent until it has a response.
     const old=baseline.current.get(entry.kind)
     if(!old){baseline.current.set(entry.kind,new Set(entry.features.map(signature)));continue}
     for(const feature of entry.features)if(!old.has(signature(feature)))next.push({kind:entry.kind,feature})
   }
   setPending(next)
 },[scopeKey,fingerprint])
 if(!pending.length)return null
 return <button className="atlas-updates" title="Newly retained historical evidence may appear here. Inspect observation and acquisition dates separately from arrival." onClick={()=>{const first=pending[0];onInspect(first.kind,first.feature);for(const entry of current)baseline.current.set(entry.kind,new Set(entry.features.map(signature)));setPending([])}}>{pending.length} new / revised retained records <span>Inspect {layerInfo[pending[0].kind].name.toLowerCase()} →</span></button>
}
