import {lazy,Suspense,useCallback,useMemo} from 'react'
import {useQuery} from '@tanstack/react-query'
import {createAPI} from '@/lib/api'
import {useWorkspace} from '@/lib/workspaceContext'
import {localFromPosition} from './spatial'
import type {DisplayContact} from './ExerciseViewport'
const Viewport=lazy(()=>import('./ExerciseViewport'))
export function LiveExerciseView({runId}:{runId:string}){
 const {session,prefix,signal}=useWorkspace()
 const api=useMemo(()=>createAPI({...session.resource,partition:'EXERCISE_OBSERVATION',runId},signal),[session.resource,runId,signal])
 const key=[...prefix,'perceived-world',runId]
 const env=useQuery({queryKey:[...key,'environment'],queryFn:()=>api.getExerciseEnvironment(runId),retry:false})
 const tracks=useQuery({queryKey:[...key,'tracks'],queryFn:()=>api.listTracks({limit:100}),refetchInterval:1000,retry:false})
 const assets=useQuery({queryKey:[...key,'assets'],queryFn:()=>api.listAssets({limit:100}),refetchInterval:1000,retry:false})
 const load=useCallback(()=>api.downloadExerciseEnvironment(runId),[api,runId])
 const contacts=useMemo<DisplayContact[]>(()=>{if(!env.data)return [];const origin=env.data.environment.frame.origin;return [...(tracks.data?.items??[]).flatMap(t=>t.position?[{id:t.id,local:localFromPosition(origin,{longitude:t.position.lng,latitude:t.position.lat,height:origin.height,verticalDatum:'WGS84_ELLIPSOID'}),altitudeKnown:false,kind:'contact' as const}]:[]),...(assets.data?.items??[]).flatMap(a=>a.position?[{id:a.name||a.id,local:localFromPosition(origin,{longitude:a.position.lng,latitude:a.position.lat,height:origin.height,verticalDatum:'WGS84_ELLIPSOID'}),altitudeKnown:false,kind:'controlled' as const}]:[])]},[env.data,tracks.data,assets.data])
 if(env.isLoading)return <p role="status">Loading permitted exercise geography…</p>
 if(env.isError||!env.data)return <p role="alert">This run has no available released 3D environment, or this account lacks access. The 2D Exercise COP remains available.</p>
 return <><p className="text-sm">Live perception · last position response {tracks.dataUpdatedAt?new Date(tracks.dataUpdatedAt).toLocaleTimeString():'pending'} · observations retain sensor delay and uncertainty.</p>{(tracks.isError||assets.isError)&&<p role="alert">Some positions could not refresh. Displayed readings may be stale.</p>}<Suspense fallback={<p role="status">Loading spatial renderer…</p>}><Viewport environment={env.data} load={load} contacts={contacts} meaning="Operator view · perceived world"/></Suspense></>
}
