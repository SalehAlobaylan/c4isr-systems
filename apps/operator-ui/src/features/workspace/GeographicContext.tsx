import {useEffect, useRef, useState} from 'react'
import {useQuery} from '@tanstack/react-query'
import type {FeatureCollection, Point} from 'geojson'
import {ApiError, fixedScopeInput, type ScopeSnapshot} from '@/lib/api'
import {useWorkspace} from '@/lib/workspaceContext'
import {CoveragePanel, useCoverage} from './CoveragePanel'
import {ContextLayerDetails} from './ContextLayerDetails'
import {projectContextStatus} from './contextLayerStatus'
import {layerInfo, type AtlasLayer} from './LayerCatalog'
import {MovingHistoryLookup, type MovingRead} from './MovingHistory'

export const contextFamilies = ['earthquakes', 'thermal', 'weather', 'bulletins'] as const satisfies readonly AtlasLayer[]
export type ContextMarker = {id: string; title: string; date: string; position: [number, number]; lineage?: string[]}
const rejected = (error: unknown) => error instanceof ApiError && error.status >= 400 && error.status < 500

export function GeographicContext({releaseArea, atlasScope, onMarkers, drawn}: {
  releaseArea: string; atlasScope?: ScopeSnapshot; onMarkers: (markers: ContextMarker[]) => void; drawn?: number;
}) {
  const {api, prefix, session} = useWorkspace()
  const [family, setFamily] = useState<(typeof contextFamilies)[number]>('earthquakes')
  const selected = contextFamilies.find(item => item === family && session.permissions.includes(layerInfo[item].permission)) ??
    contextFamilies.find(item => session.permissions.includes(layerInfo[item].permission))
  const [acquisition, setAcquisition] = useState(false)
  const [draw, setDraw] = useState(true)
  const [moving, setMoving] = useState<MovingRead>()
  const key = [...prefix, 'geographic-context', releaseArea, atlasScope?.scopeHash, selected]
  const read = useQuery({
    queryKey: key, enabled: !!selected && !!atlasScope && !!releaseArea, gcTime: 0,
    retry: (attempt, error) => attempt < 1 && !rejected(error),
    refetchInterval: 60_000,
    queryFn: ({signal}) => api.readAnalystScope({area: {revisionId: releaseArea}, temporal: fixedScopeInput(atlasScope!).temporal,
      families: [selected!], filters: {}, zoom: 14, includeEvents: false, purpose: 'geographic_preview'}, signal),
  })
  const denied = read.isError && rejected(read.error)
  const scope = !denied && read.data?.scope.area.revisionId === releaseArea ? read.data.scope : undefined
  const coverage = useCoverage(scope, !!scope)
  const layer = selected && scope ? read.data?.layers[selected] as unknown as (FeatureCollection & {total?: number; state?: string; truncated?: boolean}) : undefined
  const callback = useRef(onMarkers)
  callback.current = onMarkers
  const usable = !denied && layer && layer.state !== 'unavailable' && layer.state !== 'historical_unsupported'
  const markers = usable ? layer.features.flatMap(feature => {
    const p = feature.properties
    if (feature.geometry.type !== 'Point' || !p?.recordId || p.aggregate || !p.date) return []
    return [{id: String(p.recordId), title: String(p.title ?? p.recordId), date: String(p.date), position: (feature.geometry as Point).coordinates as [number, number],
      lineage: Array.isArray(p.lineageKeys) ? p.lineageKeys.filter((key: unknown): key is string => typeof key === 'string') : undefined}]
  }).slice(0, 150) : []
  const applicableMoving=moving?.areaRevisionId===releaseArea && moving.from && atlasScope &&
    Date.parse(moving.from)===Date.parse(atlasScope.temporal.interval.from) && Date.parse(moving.to)===Date.parse(atlasScope.temporal.interval.to) &&
    (moving.cutoff?Date.parse(moving.cutoff):undefined)===(atlasScope.temporal.asOf?Date.parse(atlasScope.temporal.asOf):undefined) ? moving : undefined
  useEffect(() => {
    const reported: ContextMarker[]=(applicableMoving?.items??[]).filter(item=>item.decision!=='QUARANTINED').map(item=>({id:item.evidenceId,
      title:`Reported ${item.reported.kind} · surface location marker`,date:item.observedAt,
      position:[item.reported.position.lng,item.reported.position.lat],lineage:[item.sourceId]}))
    callback.current(draw ? [...reported,...markers].slice(0,150) : [])
    return () => callback.current([])
  }, [layer, draw, denied, applicableMoving])
  if (!atlasScope) return <p role="status">Select an Atlas time window to inspect retained context.</p>
  const status = projectContextStatus({layer, sources: !denied && !coverage.isError ? coverage.data?.sources.filter(source => source.family === selected) ?? [] : [],
    receipts: denied ? [] : read.data?.coverageReceipts, now: Date.now(), failed: read.isError, retained: !!usable && read.isError,
    cap: draw ? drawn ?? markers.length : 0})
  return <section aria-label="Retained context overlays" className="space-y-2">
    {selected&&<label>Context source family <select className="atlas-field" value={selected} onChange={event => setFamily(event.target.value as typeof family)}>
      {contextFamilies.filter(item => session.permissions.includes(layerInfo[item].permission)).map(item => <option key={item} value={item}>{layerInfo[item].name}</option>)}
    </select></label>}
    <label><input type="checkbox" checked={draw} onChange={event => setDraw(event.target.checked)} /> Show located context markers</label>
    <ContextLayerDetails status={status} />
    <div className="ux-actions">
      <button className="ux-button" disabled={read.isFetching} onClick={() => {void read.refetch(); void coverage.refetch()}}>Refresh retained context</button>
      <button className="ux-button" aria-expanded={acquisition} onClick={() => setAcquisition(value => !value)}>Coverage and acquisition</button>
    </div>
    {acquisition && scope && <CoveragePanel scope={scope} />}
    {session.permissions.includes('tracks.read')&&session.permissions.includes('observations.read')&&<details><summary>Historical aircraft or vessel reports</summary>
      <MovingHistoryLookup key={`${releaseArea}:${atlasScope.scopeHash}`} areaRevisionId={releaseArea} from={atlasScope.temporal.interval.from}
        to={atlasScope.temporal.interval.to} cutoff={atlasScope.temporal.asOf} onRead={setMoving} />
      {applicableMoving&&<button className="ux-button" onClick={()=>setMoving(undefined)}>Remove moving location markers</button>}
    </details>}
    {!!markers.length && <details><summary>Located records</summary><ul>{markers.slice(0, 20).map((marker, index) => <li key={`${marker.id}:${index}`}>
      {marker.title} · <time dateTime={marker.date}>{marker.date}</time>
      {!!marker.lineage?.length && <p>Source lineage: {marker.lineage.join(' · ')}. Reports that cite the same event remain separate records.</p>}
    </li>)}</ul></details>}
  </section>
}
