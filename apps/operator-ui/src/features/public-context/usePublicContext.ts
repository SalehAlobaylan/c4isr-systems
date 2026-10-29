import {historicalReadOptions,useAnalystScope,rectangleGeometry,monitoringPeriod} from '@/features/workspace/AnalystScopeProvider'
import {fixedScopeInput} from '@/lib/api'
import {parseRectangle} from '@/features/workspace/operationArea'
import { useWorkspacePreference } from '@/features/workspace/preferences'
import type { Feature, FeatureCollection, Geometry } from 'geojson'
import { useEffect, useMemo, useState } from 'react'
import { useSearch } from '@tanstack/react-router'
import { DEFAULT_PUBLIC_FILTERS, queryFor, readPublicFilters } from './filters'

import { useAPI } from '@/lib/api'
import { useQuery } from '@/lib/scopedQueries'
import { useWorkspace } from '@/lib/workspaceContext'

export function usePublicContext({cartography=true,loadEvents=true,regionId,geometry}: {cartography?:boolean;loadEvents?:boolean;regionId?:string;geometry?:string}={}) {
  const api = useAPI(),analyst=useAnalystScope(),scope=analyst?.snapshot
  const { prefix, session } = useWorkspace()
  const enabled = session.resource.partition === 'PUBLIC_EVIDENCE' && !!scope && !analyst?.error
  const [datasetId, selectDataset] = useState('')
  const [layerIds,setLayerIds] = useWorkspacePreference<string[]|null>('map-layers',null,(v):v is string[]|null=>v===null||Array.isArray(v)&&v.length<=32&&v.every(x=>typeof x==='string'))
  const [viewport,setViewport] = useState('34,16,56,33')
  const [viewportLimited,setViewportLimited]=useState(false)
  const [featureOffset,setFeatureOffset] = useState(0)
  const [evidenceId, selectEvidence] = useWorkspacePreference('map-evidence','',(v):v is string=>typeof v==='string'&&v.length<=512)
  const search = useSearch({ from: '/map' })

  const effectiveView=search.publicView
  const saved=useMemo(()=>{const base=readPublicFilters(effectiveView);if(scope){base.filters={...base.filters,observedFrom:scope.temporal.interval.from,observedTo:scope.temporal.interval.to,view:scope.temporal.revisionView,asOf:scope.temporal.asOf??''}};return base},[effectiveView,scope?.scopeHash])
  const [draft, setDraft] = useState(saved.filters)
  const [filterError, setFilterError] = useState('')
  useEffect(() => { setDraft(saved.filters); selectEvidence(''); setFilterError('') }, [effectiveView])
  const params = useMemo(() => ({...queryFor(saved.filters),bbox:regionId||geometry?undefined:queryFor(saved.filters).bbox,region_id:regionId||undefined,geometry}), [saved.filters,regionId,geometry])
  const temporal = useMemo(() => ({ view: params.view, as_of: params.as_of }), [params.view, params.as_of])
  const save = (filters: typeof draft) => {
    queryFor(filters);selectEvidence('')
    let area=analyst?.input?.area??{kind:'world'}
    if(filters.bbox){const b=parseRectangle(filters.bbox);if(!b)throw new Error('Choose a valid explicit rectangle.');area={kind:'viewport',geometry:rectangleGeometry(b),name:'Explicit map rectangle'}}
    if(!filters.observedFrom||!filters.observedTo)throw new Error('Choose both dates for a fixed historical study, or use a monitoring preset.')
    analyst?.setScope({area,temporal:{mode:'historical',interval:{from:filters.observedFrom,to:filters.observedTo},revisionView:filters.view,asOf:filters.view==='latest'?undefined:filters.asOf,timezone:scope?.temporal.timezone??Intl.DateTimeFormat().resolvedOptions().timeZone,backgroundImagery:scope?.temporal.backgroundImagery??false}},filters)
  }
  const applyFilters = () => { try { save({ ...draft, offset: 0 }); setFilterError('') } catch (err) { setFilterError(err instanceof Error ? err.message : 'Invalid public filters.') } }
  const resetFilters = () => {setDraft({...DEFAULT_PUBLIC_FILTERS});selectEvidence('');setFilterError('');analyst?.setScope({area:analyst.input?.area??{kind:'world'},temporal:monitoringPeriod()},{...DEFAULT_PUBLIC_FILTERS})}
  const [pageOffset,setPage]=useState(0)
  useEffect(()=>setPage(0),[scope?.scopeHash,effectiveView])
  const [showEvents, setShowEvents] = useWorkspacePreference('show-events',true,(v):v is boolean=>typeof v==='boolean')
  const [showReference, setShowReference] = useWorkspacePreference('show-reference',true,(v):v is boolean=>typeof v==='boolean')
  const eventBatch=useQuery({...historicalReadOptions(scope),queryKey:[...prefix,'public-events','read',scope?.scopeHash,scope?.authorizationRevision,saved.filters.sourceId,saved.filters.quality,saved.filters.phenomenon,saved.filters.revisions],queryFn:({signal})=>api.readAnalystScope({...fixedScopeInput(scope!),families:[],includeEvents:true,filters:{sourceId:saved.filters.sourceId,quality:saved.filters.quality,phenomenon:saved.filters.phenomenon,revisions:saved.filters.revisions},zoom:0},signal),enabled:enabled&&loadEvents&&session.permissions.includes('observations.read'),retry:false})
  const events={...eventBatch,isError:eventBatch.isError||eventBatch.data?.eventState==='unavailable',data:eventBatch.data?.events?{...eventBatch.data.events,items:eventBatch.data.events.items.slice(pageOffset,pageOffset+100),truncated:eventBatch.data.events.total>pageOffset+100}:undefined}

  const datasets = useQuery({ queryKey: [...prefix, 'reference-datasets'], queryFn: ({signal}) => api.listReferenceDatasets(signal), enabled:enabled&&session.permissions.includes('sources.read') })
  const sources = useQuery({ queryKey: [...prefix, 'sources', 'public-coverage'], queryFn: () => api.listSources({ limit: 100 }), enabled:enabled&&session.permissions.includes('sources.read'), refetchInterval: 15_000 })
  const initialDataset = datasets.data?.items.find((d) => d.kind === 'vector') ?? datasets.data?.items[0]
  const activeDataset = datasets.data?.items.find((d) => d.id === (datasetId || initialDataset?.id))
  const authorizedLayers=datasets.data?.items.filter(d=>d.kind==='vector').map(d=>d.id)??[]
  const selectedLayers=(layerIds??authorizedLayers.slice(0,32)).filter(id=>authorizedLayers.includes(id))
  const features = useQuery({ queryKey: [...prefix, 'reference-datasets', 'viewport', viewport,selectedLayers,featureOffset], queryFn: ({signal}) => api.getReferenceViewport(viewport,selectedLayers,featureOffset,signal), enabled: enabled && cartography && showReference && selectedLayers.length>0 })
  const setBounds=(b:string)=>{const p=b.split(',').map(Number);const limited=(p[2]-p[0])*(p[3]-p[1])>400;setViewportLimited(limited);if(!limited){setViewport(b);setFeatureOffset(0)}}
  const toggleLayers=(ids:string[])=>{setLayerIds(previous=>{const current=(previous??authorizedLayers.slice(0,32)).filter(id=>authorizedLayers.includes(id));return ids.every(id=>current.includes(id))?current.filter(id=>!ids.includes(id)):[...new Set([...current,...ids.filter(id=>authorizedLayers.includes(id))])].slice(0,32)});setFeatureOffset(0)}
  const toggleLayer=(id:string)=>toggleLayers([id])
  const detail = useQuery({ queryKey: [...prefix, 'public-events', 'detail', evidenceId, temporal], enabled: enabled && Boolean(evidenceId) && session.permissions.includes('sources.read'), queryFn: async ({signal}) => {
    const event = await api.getPublicEvent(evidenceId, temporal,signal)
    const [revisions, evidence, admission] = await Promise.all([
      api.listPublicEventRevisions(event.evidenceId, temporal,signal),
      api.getSourceEvidence(event.sourceId, event.evidenceId,signal),
      api.getSourceAdmissionVersion(event.sourceId, event.admissionId,signal),
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
  return { evaluatedScope:eventBatch.data?.scope??scope,draft, setDraft, applyFilters, resetFilters, setPage, filterError, savedFilters: {...saved.filters,offset:pageOffset}, filterWarning: saved.warning, params, enabled, events,readSnapshotId:eventBatch.data?.id, datasets, sources, features, detail, activeDataset, datasetId, selectDataset, evidenceId, selectEvidence, showEvents, setShowEvents, showReference, setShowReference, mapFeatures,viewportLimited,selectedLayers,toggleLayer,toggleLayers,setBounds,featureOffset,setFeatureOffset }
}
export type PublicContextState = ReturnType<typeof usePublicContext>
