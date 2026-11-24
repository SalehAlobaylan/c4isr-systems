import {scopePeriodLabel} from '@/features/workspace/AnalystScopeProvider'
import { Link } from '@tanstack/react-router'
import { useEffect, useRef, useState } from 'react'
import { useWorkspace } from '@/lib/workspaceContext'
import { PublicFiltersForm } from './PublicFiltersForm'
import { meaningFor } from './filters'
import {SourceStatus} from './SourceStatus'

import { useAPI } from '@/lib/api'
import { formatTimestamp } from '@/lib/format'
import type { PublicContextState } from './usePublicContext'

export function PublicContextPanel({ data, focus, replaying, embedded=false }: { data: PublicContextState; focus: (lng: number, lat: number) => void; replaying: boolean; embedded?:boolean }) {
  const api = useAPI()
  const { signal, session } = useWorkspace()
  const [original, setOriginal] = useState('')
  const [loadingOriginal, setLoadingOriginal] = useState(false)
  const selectionKey = `${data.evidenceId}|${data.savedFilters.view}|${data.savedFilters.asOf}`
  const currentSelection = useRef(selectionKey)
  currentSelection.current = selectionKey
  useEffect(() => { setOriginal(''); setDownloadError(''); setLoadingOriginal(false) }, [selectionKey])
  const [downloadError, setDownloadError] = useState('')
  const download = async (load: () => Promise<string>, name: string, mediaType = 'application/json') => {
    const key = selectionKey; try { const text = await load(); if (signal.aborted || currentSelection.current !== key) return; const url = URL.createObjectURL(new Blob([text], { type: mediaType })); const link = document.createElement('a'); link.href = url; link.download = name; link.click(); URL.revokeObjectURL(url); setDownloadError('') } catch { if (!signal.aborted && currentSelection.current === key) setDownloadError('The retained artifact could not be downloaded.') }
  }
  const viewOriginal = async () => { const selected = data.detail.data; if (!selected) return; const key = selectionKey; setLoadingOriginal(true); try { const raw = await api.getSourceOriginal(selected.event.sourceId, selected.event.evidenceId); if (!signal.aborted && currentSelection.current === key) setOriginal(raw) } catch { if (!signal.aborted && currentSelection.current === key) setDownloadError('The retained original could not be loaded.') } finally { if (!signal.aborted && currentSelection.current === key) setLoadingOriginal(false) } }
  if (!data.enabled) return null
  const selected = data.detail.data
  const dataset = data.activeDataset
  const temporalMeaning = data.events.data?.view?.meaning ?? meaningFor(data.savedFilters.view)
  const exactTime = (iso?: string) => iso && !iso.startsWith('0001') ? new Date(iso).toISOString().replace('T', ' ').replace('Z', ' UTC') : 'Not provided'
  const isCSV = selected?.evidence.originalMediaType === 'text/csv'
  const hasOriginal = selected?.evidence.originalRecord != null || (isCSV && selected?.admission.rights.retainRaw)

  return <details className="shrink-0 border-b border-edge bg-panel px-4 py-2 text-sm" open={embedded || undefined} aria-label="Public event and reference data">
    <summary className="cursor-pointer font-semibold">Reported events and reference geography · {data.events.data?.total ?? '—'} reports</summary>
    <div className="max-h-64 space-y-3 overflow-auto py-2">
      {!embedded&&<><Link className="inline-block min-h-9 underline" to="/intelligence">Imagery comparison and assessment review</Link>
      {session.permissions.includes('sources.read') && <Link className="ml-4 inline-block min-h-9 underline" to="/weather-reports">Weather and source reports</Link>}
      {session.permissions.includes('tracks.read') && <Link className="ml-4 inline-block min-h-9 underline" to="/external-tracks">External moving tracks</Link>}
      <p>Provider reports retain uncertainty. Missing records do not prove absence. Elevation outlines show dataset coverage.</p></>}
      <p role="status" aria-label={embedded?"Evidence time meaning":"Public view meaning"}>{temporalMeaning}{data.savedFilters.asOf && data.savedFilters.view !== 'latest' ? ` As of ${exactTime(data.savedFilters.asOf)}.` : ''}</p>
      {data.filterWarning && <p role="alert">{data.filterWarning}</p>}
      {data.savedFilters.revisions === 'all' && <p>All retained revisions are shown. Revisions of the same provider record are not independent reports.</p>}
      {!embedded&&<PublicFiltersForm data={data} />}
      {data.events.data?.items.some((e) => e.reported.area) && <p>Footprints: measurement dotted, derived buffer dashed, provider-reported extent or boundary solid. Points do not depict impact perimeters.</p>}
      {replaying && <p role="status">Reported-event history follows the public time filters above. The track scrubber has its own replay clock.</p>}
      <div className="flex flex-wrap items-center gap-4">
        {!embedded&&<><label><input type="checkbox" checked={data.showEvents} onChange={(e) => data.setShowEvents(e.target.checked)} /> Reported events</label>
        <label><input type="checkbox" checked={data.showReference} onChange={(e) => data.setShowReference(e.target.checked)} /> Reference geography</label></>}
        <label className="block min-w-0 max-w-full">Reference version <select aria-label="Reference version" className="block w-full min-w-0 max-w-full border border-edge bg-panel p-1" value={data.datasetId || dataset?.id || 'none'} onChange={(e) => data.selectDataset(e.target.value)}>
          <option value="none">None</option>
          {(data.datasets.data?.items ?? []).map((d) => <option key={d.id} value={d.id}>{d.label} · {d.version}</option>)}
        </select></label>
      </div>
      {!embedded&&<details className="rounded border border-edge p-2"><summary className="min-h-10 cursor-pointer">Visible geographic layers · {data.selectedLayers.length}</summary><div className="grid gap-2 sm:grid-cols-2">{data.datasets.data?.items.filter(d=>d.kind==='vector').map(d=><label key={d.id} className="flex min-h-10 items-center gap-2"><input type="checkbox" checked={data.selectedLayers.includes(d.id)} onChange={()=>data.toggleLayer(d.id)}/>{d.label}</label>)}</div><p>Geography is queried in PostGIS for the current map viewport. Blue lines: reference geometry. Pale polygons: buildings and boundaries; these are geographic inputs, not evidence of activity.</p></details>}
      {data.features.data?.truncated&&<div role="status" className="space-x-3"><span>Showing up to 2,000 of {data.features.data.matched} matching features. Zoom in or page through the viewport.</span><button className="min-h-10 underline" disabled={data.featureOffset===0} onClick={()=>data.setFeatureOffset(Math.max(0,data.featureOffset-2000))}>Previous geography</button><button className="min-h-10 underline" disabled={data.featureOffset+2000>=data.features.data.matched} onClick={()=>data.setFeatureOffset(data.featureOffset+2000)}>Next geography</button></div>}
      {(data.events.isError || data.datasets.isError || data.features.isError) && <p role="alert">{data.events.isError?(data.events.error?.message??'Event query unavailable; count unknown'):'Some public layers are unavailable. Retained data may be incomplete.'} <button className="underline" onClick={() => { void data.events.refetch(); void data.datasets.refetch(); if (dataset?.kind === 'vector') void data.features.refetch() }}>Retry public layers</button></p>}
      {(data.events.isLoading || data.datasets.isLoading) && <p role="status">Loading authorized public layers…</p>}
      {dataset && <section aria-label="Reference provenance" className="space-y-1">
        <strong>{dataset.label} · {dataset.version}</strong>
        <p>{dataset.rights.attribution}</p>
        <p>{dataset.crs} · {dataset.kind === 'vector' ? 'Geographic reference, no height values' : dataset.verticalDatum} · {dataset.resolution.unit === 'map_scale' ? `1:${dataset.resolution.value.toLocaleString()} scale` : `${dataset.resolution.value} ${dataset.resolution.unit.replace('_', ' ')}`} · {dataset.eligibleForSimulation ? 'Eligible for reviewed simulation input' : 'Simulation export not eligible'}</p>
        <p>{dataset.quality}</p><p>{dataset.validity}</p><p>Reference versions keep their own validity; report-time filters do not reconstruct historical geography.</p>
        <p className="break-all">Artifact SHA-256: {dataset.sha256}</p>
        <button className="underline mr-3" onClick={() => void download(() => api.getReferenceArtifact(dataset.id), `${dataset.id}.json`)}>Download frozen reference</button>
        <button className="underline" onClick={() => focus((dataset.aoi.west + dataset.aoi.east) / 2, (dataset.aoi.south + dataset.aoi.north) / 2)}>Show reference region</button>
      </section>}
      <p className="text-xs text-ink-muted">Current retrieval health; historical provider availability is not reconstructed.</p>
      <details><summary className="min-h-11 cursor-pointer">Provider coverage and activation · current, historical and gated sources</summary><div aria-label="Provider coverage" className="grid gap-3 sm:grid-cols-2">{(data.sources.data?.items??[]).slice(0,20).map(s=><SourceStatus key={s.id} source={s}/>)}</div></details>
      {data.events.data?.total === 0 && <p>No published reports in this bounded view. Check provider coverage before interpreting an empty map.</p>}
      {data.events.data && data.events.data.total > data.events.data.items.length && <div className="flex flex-wrap items-center gap-2" aria-label="Public report pages"><span>{data.events.data.items.length ? `Showing ${data.params.offset! + 1}–${data.params.offset! + data.events.data.items.length} of ${data.events.data.total}` : `No reports on this page of ${data.events.data.total}`}; {embedded?'inspector page; map aggregates independently':'map shows this page'}.</span><button className="min-h-9 rounded border border-edge px-2 disabled:opacity-50" disabled={!data.params.offset} onClick={() => data.setPage(Math.max(0, data.params.offset! - 100))}>Previous reports</button><button className="min-h-9 rounded border border-edge px-2 disabled:opacity-50" disabled={data.params.offset! + data.events.data.items.length >= data.events.data.total || data.params.offset! >= 10000} onClick={() => data.setPage(data.params.offset! + 100)}>Next reports</button></div>}
      <div className="flex flex-wrap gap-2" aria-label="Inspect reported events">
        {(data.events.data?.items ?? []).map((event) => <button key={event.evidenceId} className="rounded border border-edge px-2 py-1 text-left" aria-pressed={data.evidenceId === event.evidenceId} onClick={() => { data.selectEvidence(event.evidenceId) }}>
          {event.reported.title || event.providerRecordId} · {event.reported.withdrawn ? 'Withdrawn by provider' : event.reported.type}{event.reported.area ? ` · ${event.reported.geometryRole}` : ''} · {event.reported.observedAt.startsWith('0001') ? 'Origin time not provided' : formatTimestamp(event.reported.observedAt)}
        </button>)}
      </div>
      {data.detail.isError && <p role="alert">Source revision details could not be loaded.</p>}
      {selected && <section aria-label="Reported event evidence" className="space-y-2 border-t border-edge pt-2">
        <h2 className="font-semibold">{selected.event.reported.title || selected.event.providerRecordId}</h2>{selected.event.reported.position&&<button className="ux-button" onClick={()=>focus(selected.event.reported.position!.lng,selected.event.reported.position!.lat)}>Locate reported position</button>}
        <p>{selected.admission.originalProducer} via {selected.admission.upstreamProvider} · {selected.event.providerRecordId} · revision {selected.event.providerRevision}</p>
        <p>Reported origin: {exactTime(selected.event.reported.observedAt)} · provider revision: {exactTime(selected.event.revisionAt)} · received: {exactTime(selected.event.receivedAt)}</p>
        <p>Published by EMAD: {exactTime(selected.event.knownAt)} · effective basis: {selected.event.effectiveBasis?.replaceAll('_', ' ') ?? 'See provider metadata'}. Retrospective effective time: {exactTime(selected.event.effectiveAt)}.</p>
        <p>{scopePeriodLabel(data.evaluatedScope)}</p>{selected.event.currentValidity&&<p role="status">Current retained status: {selected.event.currentValidity.state.replaceAll('_',' ')}. {selected.event.currentValidity.meaning}</p>}
        <p>Geometry: {selected.event.reported.geometryRole}.{selected.event.reported.type === 'event.earthquake' && ' Depth is the provider’s hypocentral depth in kilometres, positive down; it is not terrain height or scene altitude.'}</p>
        {selected.event.reported.type === 'thermal.detection' && <p>Reported thermal anomaly, not a confirmed fire or perimeter. Pixel dimensions: {String(selected.event.reported.properties?.scanKm ?? 'unknown')} × {String(selected.event.reported.properties?.trackKm ?? 'unknown')} km. A measurement footprint orientation is not supplied; the marker shows the reported location. Provider confidence: {String(selected.event.reported.quality?.providerConfidence ?? 'unknown')}.</p>}
        <p>{selected.admission.rights.attribution}</p>
        <p>Rights: {selected.admission.rights.legalBasis} · {selected.admission.rights.retainRaw ? 'Original retention permitted' : 'Reference-only retention'} · {selected.admission.rights.allowAnalysis ? 'Analysis permitted' : 'Analysis not permitted'}.</p>
        <p className="break-all">Parsed content SHA-256: {selected.evidence.contentHash}</p><p className="break-all">Original bytes SHA-256: {selected.evidence.rawHash}</p>
        <details><summary>Normalized provider fields and reported quality</summary><pre className="max-h-40 overflow-auto whitespace-pre-wrap">{JSON.stringify({ properties: selected.event.reported.properties, quality: selected.event.reported.quality }, null, 2)}</pre></details>
        <p className="text-xs text-ink-muted">Linked revisions respect the selected historical cutoff; source, geometry and quality filters apply to the report list.</p>
        <details><summary>Source revisions ({selected.revisions.total}{selected.revisions.truncated ? '+' : ''})</summary><ol>{selected.revisions.items.map((r) => <li key={r.evidenceId}><button className="underline" onClick={() => { data.selectEvidence(r.evidenceId) }}>{r.providerRevision} · {formatTimestamp(r.revisionAt)}{r.reported.withdrawn ? ' · withdrawn' : ''}</button></li>)}</ol></details>
        <details><summary>Retained original provider record</summary><pre className="max-h-48 overflow-auto whitespace-pre-wrap">{original || (selected.evidence.originalRecord ? JSON.stringify(selected.evidence.originalRecord, null, 2) : isCSV && hasOriginal ? 'Retained CSV header and selected original row; load or download below. The full response body is identified separately by its retrieval hash.' : 'Reference-only retention; no original bytes retained.')}</pre>
          {hasOriginal && <div className="flex flex-wrap gap-3"><button className="underline" disabled={loadingOriginal} onClick={() => void viewOriginal()}>{loadingOriginal ? 'Loading original…' : 'View exact original'}</button><button className="underline" onClick={() => void download(() => api.getSourceOriginal(selected.event.sourceId, selected.event.evidenceId), `${selected.event.evidenceId}.${isCSV ? 'csv' : 'json'}`, isCSV ? 'text/csv' : 'application/json')}>Download exact original</button></div>}
          {downloadError && <p role="alert">{downloadError}</p>}
        </details>
      </section>}
    </div>
  </details>
}
