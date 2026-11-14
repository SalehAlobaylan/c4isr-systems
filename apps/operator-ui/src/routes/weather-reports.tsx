import { useEffect, useState } from 'react'
import { WorkbenchLink as Link, useWorkbenchRoute } from '@/features/workspace/WorkbenchContext'
import { useQueryClient } from '@tanstack/react-query'
import { useQuery } from '@/lib/scopedQueries'
import { useAPI } from '@/lib/api'
import { useWorkspace } from '@/lib/workspaceContext'

const control = 'min-h-10 rounded border border-edge bg-panel px-3 py-2 disabled:opacity-50'
const field = `${control} block w-full min-w-0`
const number = (n: number | null | undefined, unit: string) => n == null ? 'Unknown' : `${Number(n.toFixed(2))} ${unit}`
export function WeatherReportsPage() {
  const api = useAPI(), { prefix, session, signal } = useWorkspace(), cache = useQueryClient()
  const {search,navigate,host,scope,scopeError,active} = useWorkbenchRoute('/weather-reports')
  const [bbox, setBBox] = useState(search.bbox ?? ''), [error, setError] = useState(''), [busy, setBusy] = useState(false)
  const [conclusion, setConclusion] = useState(''), [editing, setEditing] = useState(false)
  const [method, setMethod] = useState<'OPERATOR' | 'ALGORITHM' | 'AI'>('OPERATOR'), [score, setScore] = useState('')
  const [rationale, setRationale] = useState(''), [confidence, setConfidence] = useState(''), [confidenceReason, setConfidenceReason] = useState('')
  const allowed = (p: string) => session.permissions.includes(p)
  const enabled = session.resource.partition === 'PUBLIC_EVIDENCE'&&active&&!!scope&&!scopeError
  const records = useQuery({ queryKey: [...prefix, 'context-records', search.kind,scope?.scopeHash], queryFn: ({signal}) => api.listContextRecords(search.kind,undefined,undefined,undefined,scope,signal), enabled, refetchInterval: 60_000 })
  const selected = useQuery({ queryKey: [...prefix, 'context-record', search.id], queryFn: ({signal}) => api.getContextRecord(search.id!,signal), enabled: enabled && !!search.id, refetchInterval: 60_000 })
  const revisions = useQuery({ queryKey: [...prefix, 'context-history', search.id, records.dataUpdatedAt], queryFn: ({signal}) => api.listContextHistory(search.id!,signal), enabled: enabled && !!search.id })
  const assessments = useQuery({ queryKey: [...prefix, 'report-assessments',scope?.scopeHash], queryFn: ({signal}) => api.listReportAssessments(scope,signal), enabled, refetchInterval: 60_000 })
  const history = useQuery({ queryKey: [...prefix, 'assessment-history', search.assessment, assessments.dataUpdatedAt], queryFn: ({signal}) => api.listAssessmentRevisions(search.assessment!,signal), enabled: enabled && !!search.assessment })
  useEffect(() => { setBBox(search.bbox ?? '') }, [search.bbox])
  useEffect(() => { setEditing(false); setRationale(''); setConfidence(''); setConfidenceReason('') }, [search.assessment])
  const current = history.data?.items[0]?.reportEvidence ? history.data.items[0] : undefined
  const v = selected.data, c = v?.evidence.context, w = c?.weather, report = c?.report
  const act = async (fn: () => Promise<unknown>) => {
    setBusy(true); setError('')
    try { await fn(); if (!signal.aborted) await cache.invalidateQueries({ queryKey: prefix }) }
    catch (e) { if (!signal.aborted) setError(e instanceof Error ? e.message : 'Request failed; reload the current revision before retrying.') }
    finally { if (!signal.aborted) setBusy(false) }
  }
  const original = (source: string, id: string) => act(async () => {
    const raw = await api.getSourceOriginal(source, id); if (signal.aborted) return
    const url = URL.createObjectURL(new Blob([raw], { type: 'application/json' })), a = document.createElement('a'); a.href = url; a.download = `${id}.json`; a.click(); setTimeout(() => URL.revokeObjectURL(url), 1000)
  })
  const propose = () => act(async () => {
    if (!v || !report) return
    const body = { reportEvidenceId: v.evidence.id, conclusion, method, modelScore: method === 'AI' && score !== '' ? Number(score) : null, confidence: null, confidenceRationale: '', expectedRevisionId: editing ? current?.id : undefined }
    const result = editing && current ? await api.reviseReportAssessment(current.assessmentId, body) : await api.proposeReportAssessment(body)
    if (!signal.aborted) { setEditing(false); await navigate({ search: (old) => ({ ...old, assessment: result.assessmentId }) }) }
  })
  const review = (action: 'submit' | 'review' | 'publish' | 'reject') => act(async () => {
    if (!current) return
    const body = { expectedRevisionId: current.id, action, rationale, confidence: confidence === '' ? null : Number(confidence), confidenceRationale: confidenceReason }
    if (action === 'submit') await api.submitAssessmentDraft(current.assessmentId, body)
    else await api.reviewImageryAssessment(current.assessmentId, body)
  })
  const update = (id: string) => { setEditing(false); void navigate({ search: (old) => ({ ...old, id }) }) }
  if(!scope||scopeError)return <p role="status">{scopeError??'Resolving map scope…'} <Link to="/map" returnToMap>Choose a valid map scope</Link></p>
  return <div className="ux-page records-workspace space-y-4" aria-label="Weather and source reports">
    <header className="ux-page-heading"><div><p className="ux-kicker">Public Context · Source evidence</p><h1>Weather & official reports</h1><p>Inspect station conditions and official bulletins, with the source and time behind each record.</p></div><Link to="/map" returnToMap className="ux-button">Return to map</Link></header>
    <form className="workspace-filterbar" onSubmit={(e) => { e.preventDefault(); if(host)host.close();else void navigate({to:'/map'}) }}>
      <label className="min-w-0 grow">Context AOI (west,south,east,north)<input className={field} readOnly value={bbox} onChange={(e) => setBBox(e.target.value)} /></label>
      <label>Evidence kind<select className={field} value={search.kind ?? ''} onChange={(e) => void navigate({ search: (old) => ({ ...old, kind: e.target.value === 'WEATHER' ? 'WEATHER' : e.target.value === 'REPORT' ? 'REPORT' : undefined }) })}><option value="">Weather and reports</option><option value="WEATHER">Station weather</option><option value="REPORT">Source reports</option></select></label><button className={control}>Choose scope on map</button>
    </form>
    {search.kind !== 'REPORT' && scope.capabilities['weather']?.state==='unsupported'&&<p role="status" className="ux-notice">{scope.capabilities['weather'].reason} Select Source reports to inspect available historical flood reports.</p>}<div className="evidence-list-heading"><h2>{records.data?`${records.data.total} retained records`:'Retained records'}</h2><span>Station points & cited event locations · no interpolation</span></div>
    {(error || records.isError || selected.isError || revisions.isError || assessments.isError || history.isError) && <p role="alert">{error || 'Some records are unavailable. Check the AOI, source access and connection.'}</p>}
    {records.isLoading && <p role="status">Loading retained context…</p>}
    {records.data?.total === 0 && <p>No retained context in this view. Sources may be unadmitted, unqueried or outside the selection.</p>}
    {records.data?.truncated && <p>Showing at most 100 current records. Narrow the AOI or evidence kind.</p>}
    <div className="records-layout"><div className="record-list" aria-label="Context choices">{records.data?.items.map((item) => <button key={item.evidence.id} className={`${control} min-w-0 text-left`} aria-pressed={item.evidence.id === search.id} onClick={() => update(item.evidence.id)}><strong>{item.evidence.context?.title}</strong><p>{item.evidence.context?.kind} · {item.status} · {Math.round(item.ageSeconds / 60)} minutes old</p><p>{item.sourceState.replaceAll('_', ' ')} · {item.evidence.upstreamProvider}</p></button>)}</div>
    <div className="record-detail">{!v&&<div className="studio-panel"><h2 className="font-semibold">Select a record</h2><p className="ux-muted mt-3">Select a station sample or bulletin to inspect its source, observation time and retained revisions.</p></div>}
    {v && c && <article className="min-w-0 space-y-3 rounded border border-edge p-3" aria-label="Pinned context evidence">
      <header className="evidence-heading"><div><p className="ux-kicker">{c.kind==='WEATHER'?'Station observation':'Official bulletin'}</p><h2 className="text-lg font-semibold">{c.title}</h2></div><span className="ux-status">{v.status}</span></header>
      <dl className="evidence-meta"><div><dt>Observed / cited · UTC</dt><dd>{new Date(c.from).toLocaleString('en-GB',{day:'numeric',month:'short',year:'numeric',hour:'2-digit',minute:'2-digit',timeZone:'UTC',hour12:false})}</dd></div><div><dt>Location · WGS84</dt><dd>{c.position.lat}, {c.position.lng}</dd></div><div><dt>Retrieval</dt><dd>{v.sourceState.replaceAll('_',' ')}</dd></div></dl>
      <p className="ux-muted">{c.uncertainty}</p>
      {w && <section aria-label="Station weather sample" className="station-sample"><h3 className="font-semibold">{w.stationId} · {w.reportType}</h3><div className="weather-metrics"><div><span>Temperature: </span><strong>{number(w.temperatureC, '°C')}</strong><small>dewpoint: {number(w.dewpointC, '°C')}</small></div><div><span>Wind: </span><strong>{number(w.windSpeedMps, 'm/s')}</strong><small>gust: {number(w.windGustMps, 'm/s')} · direction: {w.variableWind ? 'Variable; no fixed bearing' : number(w.windDirectionDegrees, '°')}</small></div><div><span>Visibility: </span><strong>{w.visibilityLowerBound ? 'At least ' : ''}{number(w.visibilityStatuteMiles, 'statute miles')}</strong><small>altimeter: {number(w.altimeterHPa, 'hPa')}</small></div></div><p className="raw-weather">{w.rawMetar}</p></section>}
      {report && <section className="space-y-2" aria-label="Provider report claim"><h3 className="font-semibold">Unreviewed provider claim</h3><p>{report.claim}</p><p>{report.country} · provider alert level: {report.providerAlertLevel || 'Unknown'} · cited source: {report.citedSource || 'Unknown'} · source identifier: {report.citedSourceId || 'Unavailable'}</p><p>{report.corroborationMeaning}</p><p>Shared lineage: {report.lineageKeys.join(' · ')}</p><a href={report.documentUrl} target="_blank" rel="noopener noreferrer" className="underline">Provider bulletin (external; may change)</a></section>}
      {report?.eventHistory && <details className="evidence-disclosure">
        <summary>Provider event history and citations</summary>
        <p>{report.eventHistory.category} · {report.eventHistory.closedAt ? `Provider closed at ${report.eventHistory.closedAt}` : 'No provider closure reported'}</p>
        <ul>{report.eventHistory.citations.map(citation => <li key={citation.url}><a href={citation.url} target="_blank" rel="noopener noreferrer">{citation.id} source report</a></li>)}</ul>
        <ol>{report.eventHistory.geometries.map((geometry,i) => <li key={i}><time dateTime={geometry.date}>{geometry.date}</time> · {geometry.type}
          <pre className="max-h-32 overflow-auto break-all whitespace-pre-wrap">{JSON.stringify(geometry.coordinates)}</pre></li>)}</ol>
      </details>}
      <button className={control} onClick={() => void original(v.evidence.sourceId, v.evidence.id)}>Download exact provider record</button>
      <details className="evidence-disclosure"><summary>Evidence provenance and extraction</summary><p>Analyst question: {v.analystQuestion}</p><p>Cited {c.from} to {c.to} · retained {v.evidence.retrievedAt}</p><p>{c.timeBasis}</p><p>{c.locationBasis}</p><p>{v.evidence.originalProducer} via {v.evidence.upstreamProvider} · {v.attribution}</p><p>Source: {v.sourceState.replaceAll('_', ' ')} · last attempt {v.lastAttempt ?? 'Unknown'} · last completed {v.lastSuccess ?? 'None'}{v.failureCode ? ` · ${v.failureCode}` : ''}</p><p>{c.extractionMethod}</p><pre className="max-h-60 overflow-auto whitespace-pre-wrap break-all">{JSON.stringify(v.evidence, null, 2)}</pre></details>
      <details><summary className="min-h-10 cursor-pointer font-semibold">Retained samples and revisions</summary>{revisions.data?.truncated && <p>Showing the latest 100 revisions.</p>}
      <ol className="space-y-2">{revisions.data?.items.map((r) => <li key={r.evidence.id}><button className={`${control} w-full break-words text-left`} onClick={() => update(r.evidence.id)}>Inspect cited {r.evidence.context?.from} · provider update {r.evidence.providerUpdatedAt ?? 'Not supplied'} · retained {r.evidence.retrievedAt}</button></li>)}</ol></details>
      {report && allowed('assessments.create') && <section className="space-y-2" aria-label="Source report proposal"><h3 className="font-semibold">{editing ? 'Revise the assessment draft' : 'Propose a report interpretation'}</h3><p>The proposal pins this exact bulletin. Independent human review is required for publication.</p><label>Report assessment conclusion<textarea className={field} maxLength={8000} value={conclusion} onChange={(e) => setConclusion(e.target.value)} /></label><label>Interpretation method<select className={field} value={method} onChange={(e) => setMethod(e.target.value as typeof method)}><option value="OPERATOR">Analyst</option><option value="ALGORITHM">Algorithm assisted</option><option value="AI">AI assisted</option></select></label>{method === 'AI' && <label>Model score (separate from analyst confidence)<input className={field} type="number" min={0} max={1} step={.01} value={score} onChange={(e) => setScore(e.target.value)} /></label>}<button className={control} disabled={busy} onClick={() => void propose()}>{editing ? 'Record report draft revision' : 'Record unreviewed report proposal'}</button></section>}
    </article>}
    <section aria-label="Report assessment reviews" className="space-y-3"><h2 className="text-lg font-semibold">Analyst review</h2>{assessments.data?.items.length===0&&<p className="ux-muted">No report assessments yet. Select an official bulletin to propose an interpretation. Station weather remains a source observation.</p>}
      {assessments.data?.truncated && <p>Showing the latest 100 assessments.</p>}
      {assessments.data?.items.map((a) => <button className={`${control} block w-full text-left`} key={a.id} onClick={() => { setEditing(false); void navigate({ search: (old) => ({ ...old, assessment: a.assessmentId, id: a.reportEvidence?.id }) }) }}>{a.status} · {a.conclusion}</button>)}
      {current && <article className="space-y-2 rounded border border-edge p-3" aria-label="Selected report assessment"><h3 className="font-semibold">{current.status} · revision {current.version}</h3><p>{current.conclusion}</p><p>Method: {current.method} · model score: {current.modelScore ?? 'Not supplied'} · analyst confidence: {current.confidence ?? 'Not assigned'}</p><p>{current.confidenceRationale}</p><p>Author {current.authorHumanId??'Unresolved registered identity'} · recorded by {current.actorHumanId??'Unresolved registered identity'} · review identity {current.reviewIdentityStatus??'UNRESOLVED'} · author principal {current.authorId} · actor principal {current.actorId} · {current.recordedAt}</p><p>{current.reviewRationale}</p><button className={control} onClick={() => update(current.reportEvidence!.id)}>Inspect pinned report evidence</button>
        {allowed('assessments.create') && ['PROPOSED', 'PUBLISHED', 'REJECTED', 'DRAFT'].includes(current.status) && <button className={control} onClick={() => { setEditing(true); setConclusion(current.conclusion); setMethod(current.method as typeof method); setScore(current.modelScore?.toString() ?? ''); void navigate({ search: (old) => ({ ...old, id: current.reportEvidence!.id }) }) }}>Create revised report draft</button>}
        <ol>{history.data?.items.map((a) => <li key={a.id}>{a.status} · revision {a.version} · {a.actorId} · {a.recordedAt}<p>{a.reviewRationale || a.conclusion}</p><details><summary>Immutable review snapshot</summary><pre className="max-h-48 overflow-auto whitespace-pre-wrap break-all">{JSON.stringify(a, null, 2)}</pre></details></li>)}</ol>
        {((allowed('assessments.review') && session.principal.kind === 'human' && !!current.authorHumanId && !!session.operator.id && current.authorHumanId !== session.operator.id && ['PROPOSED', 'IN_REVIEW'].includes(current.status)) || (allowed('assessments.create') && current.status === 'DRAFT')) && <div className="space-y-2"><label>Report review rationale<textarea className={field} maxLength={4000} value={rationale} onChange={(e) => setRationale(e.target.value)} /></label>{current.status === 'IN_REVIEW' && <><label>Report analyst confidence<input className={field} type="number" min={0} max={1} step={.01} value={confidence} onChange={(e) => setConfidence(e.target.value)} /></label><label>Report confidence rationale<textarea className={field} maxLength={4000} value={confidenceReason} onChange={(e) => setConfidenceReason(e.target.value)} /></label></>}{current.status === 'DRAFT' && <button className={control} disabled={busy} onClick={() => void review('submit')}>Submit report draft</button>}{current.status === 'PROPOSED' && <button className={control} disabled={busy} onClick={() => void review('review')}>Begin report review</button>}{current.status === 'IN_REVIEW' && <div className="flex flex-wrap gap-2"><button className={control} disabled={busy} onClick={() => void review('publish')}>Publish reviewed report assessment</button><button className={control} disabled={busy} onClick={() => void review('reject')}>Reject report proposal</button></div>}</div>}
      </article>}
    </section>
    </div></div>
    <p className="ux-muted">Missing reports do not establish clear weather or an absence of hazards. Provider claims and alert levels remain separate from independently reviewed assessments.</p>
  </div>
}
