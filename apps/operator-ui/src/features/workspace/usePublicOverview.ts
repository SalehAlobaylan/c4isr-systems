import {useCatalog} from './useCatalog'
import type {ImageryCapture,ImageryScene,ImageryJob,ImageryRevision} from '@/lib/api'
import {historicalReadOptions} from './AnalystScopeProvider'
import { useQuery } from '@tanstack/react-query'
import { useAPI,type ScopeSnapshot } from '@/lib/api'
import { useWorkspace } from '@/lib/workspaceContext'

export function usePublicOverview(bbox?: string, family?:string,region_id?:string,geometry?:string,enabled=true,scope?:ScopeSnapshot) {
  const api = useAPI(), { prefix, session } = useWorkspace()
  const can = (p: string) => enabled && session.resource.partition === 'PUBLIC_EVIDENCE' && session.permissions.includes(p)
  const selected=(name:string)=>!family||family===name
  const sources = useQuery({ queryKey: [...prefix, 'overview', 'sources'], queryFn: () => api.listSources({ limit: 100 }), enabled: can('sources.read'), refetchInterval: 30_000 })
  const references = useQuery({ queryKey: [...prefix, 'reference-datasets'], queryFn: ({signal}) => api.listReferenceDatasets(signal), enabled: can('sources.read') })
  const captures=useCatalog<ImageryCapture>('captures',scope,!!scope&&can('sources.read')&&selected('imagery'))
  const imagery=useCatalog<ImageryScene>('scenes',scope,!!scope&&can('sources.read')&&!family)
  const assessments=useCatalog<ImageryRevision>('findings',scope,!!scope&&can('assessments.read')&&selected('findings'))
  const context = useQuery({...historicalReadOptions(scope), queryKey: [...prefix, 'overview', 'records',scope?.scopeHash??bbox,region_id,geometry], queryFn: ({signal}) => api.listContextRecords(undefined, bbox,region_id,geometry,scope,signal), enabled: can('sources.read')&&selected('weather'), refetchInterval: scope?.temporal.mode==='historical'?false:60_000 })
  const tracks = useQuery({...historicalReadOptions(scope), queryKey: [...prefix, 'external-tracks',scope?.scopeHash??bbox,region_id,geometry], queryFn: ({signal}) => api.listExternalTracks(bbox,region_id,geometry,scope,signal), enabled: can('tracks.read')&&selected('tracks'), refetchInterval: scope?.temporal.mode==='historical'?false:30_000 })
  const jobs=useCatalog<ImageryJob>('jobs',scope,!!scope&&can('sources.read')&&selected('jobs'))
  const retained = (captures.data?.items ?? []).filter(c => c.status === 'SUCCEEDED' && c.result).filter(c => { if (!bbox) return true; const b=bbox.split(',').map(Number), a=c.input.aoi; return a.east>=b[0]&&a.west<=b[2]&&a.north>=b[1]&&a.south<=b[3] }).sort((a,b) => a.input.scene.acquiredAt.localeCompare(b.input.scene.acquiredAt))
  return { sources, references, captures, imagery, assessments, context, tracks, jobs, retained }
}
