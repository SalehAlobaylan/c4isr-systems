import { useEffect, useState } from 'react'
import { WorkbenchLink as Link, useWorkbenchRoute } from '@/features/workspace/WorkbenchContext'
import { useQuery } from '@/lib/scopedQueries'
import { useAPI } from '@/lib/api'
import { useWorkspace } from '@/lib/workspaceContext'
import {MovingHistoryLookup} from '@/features/workspace/MovingHistory'
const control = 'min-h-10 rounded border border-edge bg-panel px-3 py-1'
const coverageLabel = (state: string) => ({
  admitted_bounded_capture: 'Bounded capture completed',
  capture_outdated_or_not_completed: 'No recent completed capture',
  partial_capture_or_source_failure: 'Partial capture or source failure',
  source_disabled_or_rights_expired: 'Capture disabled or rights expired',
  source_marked_degraded: 'Source marked degraded',
  source_marked_offline: 'Source marked offline',
}[state] ?? 'Coverage status unavailable')

export function ExternalTracksPage() {
  const api = useAPI(), { prefix, session, signal } = useWorkspace()
  const {search,navigate,host,scope,scopeError,active} = useWorkbenchRoute('/external-tracks')
  const [bbox, setBBox] = useState(search.bbox ?? ''), [error, setError] = useState('')
  useEffect(() => { setBBox(search.bbox ?? '') }, [search.bbox])
  const enabled = session.resource.partition === 'PUBLIC_EVIDENCE'&&active&&!!scope&&!scopeError
  const tracks = useQuery({ queryKey: [...prefix, 'external-tracks',scope?.scopeHash], queryFn: ({signal}) => api.listExternalTracks(undefined,undefined,undefined,scope,signal), enabled, refetchInterval: 15_000 })
  const sources = useQuery({queryKey:[...prefix,'external-track-sources'],queryFn:()=>api.listSources(),enabled:enabled&&session.permissions.includes('sources.read'),refetchInterval:60_000})
  const selected = useQuery({ queryKey: [...prefix, 'external-track', search.id], queryFn: ({signal}) => api.getExternalTrack(search.id!,signal), enabled: enabled && !!search.id, refetchInterval: 15_000 })
  const history = useQuery({ queryKey: [...prefix, 'external-associations', search.id, selected.data?.latestEvidenceId], queryFn: () => api.getExternalAssociations(search.id!), enabled: enabled && !!search.id && session.permissions.includes('observations.read') })
  const value = selected.data
  const original = async (source: string, evidence: string) => {
    setError(''); try { const text = await api.getSourceOriginal(source, evidence); if (signal.aborted) return; const url = URL.createObjectURL(new Blob([text], { type: 'application/json' })), a = document.createElement('a'); a.href = url; a.download = `${evidence}.json`; a.click(); setTimeout(() => URL.revokeObjectURL(url), 1000) } catch (e) { if (!signal.aborted) setError(e instanceof Error ? e.message : 'Original unavailable') }
  }
  if(!scope||scopeError)return <p role="status">{scopeError??'Resolving map scope…'} <Link to="/map" returnToMap>Choose a valid map scope</Link></p>
  return <div className="ux-page records-workspace space-y-4" aria-label="Public external tracks">
    <header className="ux-page-heading"><div><p className="ux-kicker">Public Context · Source evidence</p><h1>Aircraft & vessels</h1><p>Reported positions with source identity, age and uncertainty. Public tracks have no asset or command controls.</p></div><Link to="/map" returnToMap className="ux-button">Return to map</Link></header>
    <form className="workspace-filterbar" onSubmit={(e) => { e.preventDefault(); if(host)host.close();else void navigate({to:'/map'}) }}><label className="min-w-0 grow">Moving-track AOI (west,south,east,north)<input className={`${control} block w-full`} readOnly value={bbox} onChange={(e) => setBBox(e.target.value)} placeholder="All authorized coverage, or enter a bounded region" /></label><button className={control}>Choose scope on map</button></form>
    {scope.capabilities['aircraft']?.state==='unsupported'&&<p role="status" className="ux-notice">{scope.capabilities['aircraft'].reason}</p>}<div className="evidence-list-heading"><h2>{tracks.data?`${tracks.data.total} reported tracks`:'Reported tracks'}</h2><span>Source-bound hypotheses · no independent identity verification</span></div>
    {(tracks.isError || selected.isError || history.isError || error) && <p role="alert">{error || 'Some selected public-track records are unavailable. Check the AOI, source authorization or connection.'}</p>}
    {tracks.isLoading && <p role="status">Loading reported public positions…</p>}
    {tracks.data?.total === 0 && !search.id && <section className="workspace-empty" aria-label="External track availability"><span className="empty-symbol" aria-hidden="true">◎</span><h2>No positions returned in this view</h2><p>No external moving hypotheses in this bounded view. This does not establish an absence of aircraft or vessels. Check provider access and retrieval freshness, or adjust the region.</p><div className="empty-source-status">{sources.data?.items.filter(s=>/opensky|ais|ads-b|aviation/i.test(`${s.name} ${s.type}`)).map(s=><div key={s.id}><strong>{s.name}</strong><span className="ux-status">{s.ingestion?.state??s.status}</span></div>)}</div>{sources.isError&&<p role="alert">Source status could not be loaded. <button className="underline" onClick={()=>void sources.refetch()}>Retry source status</button></p>}<div className="flex gap-2 justify-center flex-wrap"><Link className="ux-button" to="/systems">Check sources in Systems</Link><Link className="ux-button" to="/map">Explore Public Context</Link></div></section>}
    {tracks.data?.truncated && <p>Showing at most 200 hypotheses; narrow the AOI.</p>}
    {(tracks.data?.total!==0||!!search.id)&&<div className="records-layout"><div className="record-list" aria-label="External track choices">{tracks.data?.items.map((t) => <button className={`${control} min-w-0 text-left`} key={t.id} aria-label={`Inspect ${t.kind} claimed ${t.identityClaim}`} aria-pressed={search.id === t.id} onClick={() => void navigate({ search: (old) => ({ ...old, id: t.id }) })}><strong>{t.kind} · claimed {t.identityClaim}</strong><p>{t.status} · {Math.round(t.ageSeconds)} seconds old · {t.originalProducer}</p><p>{coverageLabel(t.coverageState)}</p></button>)}</div>
    <div className="record-detail">{!value&&<div className="studio-panel"><h2 className="font-semibold">Select a record</h2><p className="ux-muted mt-3">Choose a reported track to inspect source identity, age and association history. An empty result does not establish absence.</p></div>}
    {value && <article aria-label="Selected external hypothesis" className="space-y-3 rounded border border-edge p-3">
      <h2 className="text-lg font-semibold">{value.kind} · claimed {value.identityClaim}</h2><p>{value.status} · observed {value.lastSeenAt} · age {Math.round(value.ageSeconds)} seconds</p><p>{value.meaning}</p><p>Source capture: {coverageLabel(value.coverageState)} · last attempt {value.sourceLastAttemptAt ?? 'Unavailable'} · last completed {value.sourceLastSuccessAt ?? 'None'}{value.sourceFailureCode ? ` · ${value.sourceFailureCode}` : ''}</p><p>{value.originalProducer} via {value.upstreamProvider} · {value.attribution}</p>
      <p className="break-all">Track ID: {value.id}</p>
      {scope.area.persistent && session.permissions.includes('observations.read') && <MovingHistoryLookup key={`${value.id}:${scope.scopeHash}`} trackId={value.id} areaRevisionId={scope.area.revisionId} from={scope.temporal.interval.from} to={scope.temporal.interval.to} cutoff={scope.temporal.asOf} />}
      <p>Reported position (WGS84): {value.reported.position.lat}, {value.reported.position.lng}. Reported speed: {value.reported.speedMps ?? 'Unavailable'} m/s; course: {value.reported.course ?? 'Unavailable'}°.</p>
      <Link to="/map" search={{ selected: `track:${value.id}` }} className="underline">Inspect position on the Public Context map</Link>
      <details><summary className="min-h-10 cursor-pointer">Provider claims & quality fields</summary><div className="grid grid-cols-1 gap-3 md:grid-cols-2"><section><h3 className="font-semibold">Provider identity and classification claims</h3><pre className="max-h-60 overflow-auto whitespace-pre-wrap">{JSON.stringify(value.reported.claims, null, 2)}</pre></section><section><h3 className="font-semibold">Reported quality and lineage</h3><pre className="max-h-60 overflow-auto whitespace-pre-wrap">{JSON.stringify(value.reported.quality, null, 2)}</pre></section></div></details>
      <h3 className="font-semibold">Evidence and association decisions</h3>
      <p>source-bound-motion-v1: 30-minute segment gap; PostGIS motion gate; late reports never rewind state. Candidate jumps are quarantined, without asserting which report is correct.</p>
      {history.data?.truncated && <p>Showing the latest 100 retained decisions.</p>}
      <ol className="space-y-3">{history.data?.items.map((a) => <li className="break-words border-l border-edge pl-3" key={a.evidenceId}><strong>{a.decision}</strong> · observed {a.observedAt} · retained {a.knownAt}<p>{a.rationale}</p><p className="break-all">Evidence {a.evidenceId}</p>{session.permissions.includes('sources.read') && <button className={`${control} mt-1`} onClick={() => void original(a.sourceId, a.evidenceId)}>Download exact provider record</button>}<details><summary>Inspect this immutable decision and reported claims</summary><pre className="max-h-60 overflow-auto whitespace-pre-wrap">{JSON.stringify(a, null, 2)}</pre></details></li>)}</ol>
    </article>}
    </div></div>}
    <details className="workspace-explainer"><summary>Coverage & identity limitations</summary><p>Aircraft addresses, MMSIs, callsigns and vessel names are unverified claims. Registration country does not establish affiliation. Bounded captures can miss messages between connections. AIS has no durable replay; source outages, stale positions and absent reports are different conditions. Repeated reports from the same upstream network do not provide independent corroboration.</p></details>
  </div>
}
