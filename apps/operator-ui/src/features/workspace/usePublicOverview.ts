import { useQuery } from '@tanstack/react-query'
import { useAPI } from '@/lib/api'
import { useWorkspace } from '@/lib/workspaceContext'

export function usePublicOverview(bbox?: string, family?:string,region_id?:string,geometry?:string) {
  const api = useAPI(), { prefix, session } = useWorkspace()
  const can = (p: string) => session.resource.partition === 'PUBLIC_EVIDENCE' && session.permissions.includes(p)
  const selected=(name:string)=>!family||family===name
  const sources = useQuery({ queryKey: [...prefix, 'overview', 'sources'], queryFn: () => api.listSources({ limit: 100 }), enabled: can('sources.read'), refetchInterval: 30_000 })
  const references = useQuery({ queryKey: [...prefix, 'reference-datasets'], queryFn: () => api.listReferenceDatasets(), enabled: can('sources.read') })
  const captures = useQuery({ queryKey: [...prefix, 'imagery-captures',region_id,geometry], queryFn: () => api.listImageryCaptures(region_id,geometry), enabled: can('sources.read')&&selected('imagery'), refetchInterval: 15_000 })
  const imagery = useQuery({ queryKey: [...prefix, 'imagery-scenes', bbox,region_id,geometry], queryFn: () => api.listImageryScenes(bbox,region_id,geometry), enabled: can('sources.read')&&!family })
  const assessments = useQuery({ queryKey: [...prefix, 'imagery-assessments'], queryFn: () => api.listIntelligenceAssessments(), enabled: can('assessments.read')&&selected('findings'), refetchInterval: 15_000 })
  const context = useQuery({ queryKey: [...prefix, 'overview', 'records', bbox,region_id,geometry], queryFn: () => api.listContextRecords(undefined, bbox,region_id,geometry), enabled: can('sources.read')&&selected('weather'), refetchInterval: 60_000 })
  const tracks = useQuery({ queryKey: [...prefix, 'external-tracks', bbox,region_id,geometry], queryFn: () => api.listExternalTracks(bbox,region_id,geometry), enabled: can('tracks.read')&&selected('tracks'), refetchInterval: 30_000 })
  const jobs = useQuery({ queryKey: [...prefix, 'imagery-jobs'], queryFn: () => api.listImageryJobs(), enabled: can('sources.read')&&!family, refetchInterval: 15_000 })
  const retained = (captures.data?.items ?? []).filter(c => c.status === 'SUCCEEDED' && c.result).filter(c => { if (!bbox) return true; const b=bbox.split(',').map(Number), a=c.input.aoi; return a.east>=b[0]&&a.west<=b[2]&&a.north>=b[1]&&a.south<=b[3] }).sort((a,b) => a.input.scene.acquiredAt.localeCompare(b.input.scene.acquiredAt))
  return { sources, references, captures, imagery, assessments, context, tracks, jobs, retained }
}
