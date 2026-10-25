import {lazy,Suspense,useCallback,useMemo} from 'react'
import type {components} from '@/lib/openapi.generated'
import {createAPI,type ExerciseEnvironmentView} from '@/lib/api'
import {useWorkspace} from '@/lib/workspaceContext'
import {localFromPosition} from './spatial'
import type {DisplayContact} from './ExerciseViewport'
const Viewport=lazy(()=>import('./ExerciseViewport'))
export function RecordedWorld({runId,frame,spatial}:{runId:string;frame:components['schemas']['RecordingFrame'];spatial?:components['schemas']['SpatialReplayFrame']}){
 const {session,signal}=useWorkspace(),api=useMemo(()=>createAPI({...session.resource,partition:'EXERCISE_OBSERVATION',runId},signal),[session.resource,runId,signal])
 // The manifest is reconstructed from the operator journal. The content route
 // serves its immutable released geometry; no authoring scene is requested.
 const environment=frame.state.environment?.base as ExerciseEnvironmentView|undefined
 const load=useCallback(()=>api.downloadExerciseEnvironment(runId),[api,runId])
 const contacts=useMemo<DisplayContact[]>(()=>{
  if(!environment)return []
  if(spatial)return [...(spatial.truthSnapshot?.actors??[]).filter(a=>a.active).map(a=>({id:'Truth · '+a.id,local:a.local,altitudeKnown:true,kind:'truth' as const})),...spatial.contacts.map(c=>({id:'Perceived · '+c.contact,local:c.local,altitudeKnown:true,kind:'contact' as const,confidence:c.confidence,sampleMs:c.sampleMs,errorTo:c.truthAtSample}))]
  const origin=environment.environment.frame.origin
  return ['track_state','asset_state'].flatMap(kind=>Object.entries(frame.state[kind]??{}).flatMap(([id,row])=>{const point=row.position as {coordinates?:number[]}|undefined,coords=point?.coordinates;if(!coords||coords.length<2)return [];return [{id:kind+': '+id,local:localFromPosition(origin,{longitude:coords[0],latitude:coords[1],height:origin.height,verticalDatum:'WGS84_ELLIPSOID'}),altitudeKnown:false,kind:kind==='asset_state'?'controlled' as const:'contact' as const}]}))
 },[environment,frame,spatial])
 if(!environment||environment.schema!=='emad.exercise-environment.v1')return <p>No released prepared environment was recorded for this run. Recorded positions and events remain available below.</p>
 return <Suspense fallback={<p role="status">Loading recorded spatial view…</p>}><Viewport compact environment={environment} load={load} contacts={contacts} meaning={spatial?'Privileged recorded truth and perception':'Operator replay · recorded perception'}/></Suspense>
}
