import { useWorkspacePreference } from '@/features/workspace/preferences'
import type { Feature, FeatureCollection, Geometry } from 'geojson'
import { useEffect, useMemo, useState } from 'react'
import { useNavigate, useSearch } from '@tanstack/react-router'
import { DEFAULT_PUBLIC_FILTERS, queryFor, readPublicFilters } from './filters'

import { useAPI } from '@/lib/api'
import { useQuery } from '@/lib/scopedQueries'
import { useWorkspace } from '@/lib/workspaceContext'

export function usePublicContext({cartography=true,loadEvents=true,regionId,geometry}: {cartography?:boolean;loadEvents?:boolean;regionId?:string;geometry?:string}={}) {
  const api = useAPI()
  const { prefix, session } = useWorkspace()
  const enabled = session.resource.partition === 'PUBLIC_EVIDENCE'
  const [datasetId, selectDataset] = useState('')
  const [layerIds,setLayerIds] = useWorkspacePreference<string[]|null>('map-layers',null,(v):v is string[]|null=>v===null||Array.isArray(v)&&v.length<=32&&v.every(x=>typeof x==='string'))
  const [viewport,setViewport] = useState('34,16,56,33')
  const [viewportLimited,setViewportLimited]=useState(false)
  const [featureOffset,setFeatureOffset] = useState(0)
  const [evidenceId, selectEvidence] = useWorkspacePreference('map-evidence','',(v):v is string=>typeof v==='string'&&v.length<=512)
  const search = useSearch({ from: '/map' })
  const navigate = useNavigate({ from: '/map' })
  const [savedView,setSavedView]=useWorkspacePreference('investigation','',(v):v is string=>typeof v==='string'&&v.length<=4096)
  const effectiveView=search.publicView??savedView
  const saved = useMemo(() => readPublicFilters(effectiveView), [effectiveView])
  useEffect(()=>{if(search.publicView!==undefined)setSavedView(search.publicView)},[search.publicView])
  const [draft, setDraft] = useState(saved.filters)
  const [filterError, setFilterError] = useState('')
  useEffect(() => { setDraft(saved.filters); selectEvidence(''); setFilterError('') }, [effectiveView])
  const params = useMemo(() => ({...queryFor(saved.filters),bbox:regionId||geometry?undefined:queryFor(saved.filters).bbox,region_id:regionId||undefined,geometry}), [saved.filters,regionId,geometry])
  const temporal = useMemo(() => ({ view: params.view, as_of: params.as_of }), [params.view, params.as_of])
  const save = (filters: typeof draft) => { queryFor(filters); selectEvidence(''); void navigate({ search: (previous) => ({ ...previous, publicView: JSON.stringify(filters) }), replace: false }) }
  const applyFilters = () => { try { save({ ...draft, offset: 0 }); setFilterError('') } catch (err) { setFilterError(err instanceof Error ? err.message : 'Invalid public filters.') } }
  const resetFilters = () => { setSavedView(''); setDraft({ ...DEFAULT_PUBLIC_FILTERS }); selectEvidence(''); setFilterError(''); void navigate({ search: (previous) => ({ ...previous, publicView: undefined }) }) }
  const setPage = (offset: number) => save({ ...saved.filters, offset })
  const [showEvents, setShowEvents] = useWorkspacePreference('show-events',true,(v):v is boolean=>typeof v==='boolean')
  const [showReference, setShowReference] = useWorkspacePreference('show-reference',true,(v):v is boolean=>typeof v==='boolean')
  const events = useQuery({ queryKey: [...prefix, 'public-events', params], queryFn: () => api.listPublicEvents(params), enabled:enabled&&loadEvents&&session.permissions.includes('observations.read'), refetchInterval: saved.filters.view === 'known_at' ? false : 15_000 })
  const datasets = useQuery({ queryKey: [...prefix, 'reference-datasets'], queryFn: () => api.listReferenceDatasets(), enabled:enabled&&session.permissions.includes('sources.read') })
  const sources = useQuery({ queryKey: [...prefix, 'sources', 'public-coverage'], queryFn: () => api.listSources({ limit: 100 }), enabled:enabled&&session.permissions.includes('sources.read'), refetchInterval: 15_000 })
  const initialDataset = datasets.data?.items.find((d) => d.kind === 'vector') ?? datasets.data?.items[0]
  const activeDataset = datasets.data?.items.find((d) => d.id === (datasetId || initialDataset?.id))
  const authorizedLayers=datasets.data?.items.filter(d=>d.kind==='vector').map(d=>d.id)??[]
  const selectedLayers=(layerIds??authorizedLayers.slice(0,32)).filter(id=>authorizedLayers.includes(id))
  const features = useQuery({ queryKey: [...prefix, 'reference-datasets', 'viewport', viewport,selectedLayers,featureOffset], queryFn: () => api.getReferenceViewport(viewport,selectedLayers,featureOffset), enabled: enabled && cartography && showReference && selectedLayers.length>0 })
  const setBounds=(b:string)=>{const p=b.split(',').map(Number);const limited=(p[2]-p[0])*(p[3]-p[1])>400;setViewportLimited(limited);if(!limited){setViewport(b);setFeatureOffset(0)}}
  const toggleLayers=(ids:string[])=>{setLayerIds(previous=>{const current=(previous??authorizedLayers.slice(0,32)).filter(id=>authorizedLayers.includes(id));return ids.every(id=>current.includes(id))?current.filter(id=>!ids.includes(id)):[...new Set([...current,...ids.filter(id=>authorizedLayers.includes(id))])].slice(0,32)});setFeatureOffset(0)}
  const toggleLayer=(id:string)=>toggleLayers([id])
  const detail = useQuery({ queryKey: [...prefix, 'public-events', 'detail', evidenceId, temporal], enabled: enabled && Boolean(evidenceId) && session.permissions.includes('sources.read'), queryFn: async () => {
    const event = await api.getPublicEvent(evidenceId, temporal)
    const [revisions, evidence, admission] = await Promise.all([
      api.listPublicEventRevisions(event.evidenceId, temporal),
      api.getSourceEvidence(event.sourceId, event.evidenceId),
      api.getSourceAdmissionVersion(event.sourceId, event.admissionId),
    ])
    return { event, revisions, evidence, admission }
  } })
  const mapFeatures = useMemo<FeatureCollection>(() => {
    const result: Feature[] = []
    if (showReference) {
      for (const feature of features.data?.features ?? []) result.push({ type: 'Feature', id: String(feature.id), geometry: feature.geometry as unknown as Geometry, properties: { ...(feature.properties as Record<string,unknown>), layerKind: 'reference' } })
      if (activeDataset?.kind === 'elevation') {
        const b = activeDataset.aoi
        result.push({ type: 'Feature', geometry: { type: 'Polygon', coordinates: [[[b.west, b.south], [b.east, b.south], [b.east, b.north], [b.west, b.north], [b.west, b.south]]] }, properties: { layerKind: 'reference', datasetId: activeDataset.id, meaning: 'Elevation dataset coverage, not terrain geometry' } })
      }
    }
    if (showEvents) for (const event of events.data?.items ?? []) {
      const o = event.reported
      if (o.withdrawn) continue
      if (o.position) result.push({ type: 'Feature', geometry: { type: 'Point', coordinates: [o.position.lng, o.position.lat] }, properties: { layerKind: 'event', evidenceId: event.evidenceId, semanticKind: o.semanticKind, reportedType: o.type } })
      if (o.area) {
        const ring = o.area.polygon.map((p) => [p.lng, p.lat]); if (ring.length) ring.push(ring[0])
        result.push({ type: 'Feature', geometry: { type: 'Polygon', coordinates: [ring] }, properties: { layerKind: 'event', evidenceId: event.evidenceId, geometryRole: o.geometryRole } })
      }
    }
    return { type: 'FeatureCollection', features: result }
  }, [showReference, activeDataset, features.data, showEvents, events.data])
  return { draft, setDraft, applyFilters, resetFilters, setPage, filterError, savedFilters: saved.filters, filterWarning: saved.warning, params, enabled, events, datasets, sources, features, detail, activeDataset, datasetId, selectDataset, evidenceId, selectEvidence, showEvents, setShowEvents, showReference, setShowReference, mapFeatures,viewportLimited,selectedLayers,toggleLayer,toggleLayers,setBounds,featureOffset,setFeatureOffset }
}
export type PublicContextState = ReturnType<typeof usePublicContext>
