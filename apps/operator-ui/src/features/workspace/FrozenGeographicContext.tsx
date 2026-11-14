import {useEffect, useRef, useState} from 'react'
import {useQuery} from '@tanstack/react-query'
import {fixedScopeInput, type ScopeReadInput, type ScopeSnapshot} from '@/lib/api'
import type {operations} from '@/lib/openapi.generated'
import {useWorkspace} from '@/lib/workspaceContext'
import type {Display} from './displayBytes'
import {useReadSnapshotRelease} from './useReadSnapshotRelease'
import {MovingHistory, useMovingHistory} from './MovingHistory'

type Intent = operations['freezeGeographicContext']['requestBody']['content']['application/json']

export function FrozenGeographicContext({terrain, displays, scope}: {terrain: Display; displays: Display[]; scope: ScopeSnapshot}) {
  const {api, prefix, signal, session} = useWorkspace()
  const [query, setQuery] = useState<ScopeReadInput>()
  const [cutoff, setCutoff] = useState('')
  const [selected, setSelected] = useState<string[]>([])
  const [movingTrack, setMovingTrack] = useState('')
  const [pinnedTrack, setPinnedTrack] = useState('')
  const [selectedMoving, setSelectedMoving] = useState<string[]>([])
  const [confirmation, setConfirmation] = useState(false)
  const [lookup, setLookup] = useState('')
  const [handoff, setHandoff] = useState('')
  const [failure, setFailure] = useState('')
  const [busy, setBusy] = useState(false)
  const intent = useRef<Intent>(undefined)
  const key = [...prefix, 'geographic-handoff-read', query]
  const read = useQuery({queryKey: key, enabled: !!query, gcTime: 0, staleTime: Infinity, retry: false,
    queryFn: ({signal}) => api.readAnalystScope(query!, signal)})
  useReadSnapshotRelease(read.data?.id, key)
  const movingRead=useMovingHistory(pinnedTrack,read.data&&{areaRevisionId:terrain.areaRevisionId,from:read.data.scope.temporal.interval.from,
    to:read.data.scope.temporal.interval.to,cutoff:read.data.scope.temporal.asOf})
  const saved = useQuery({queryKey: [...prefix, 'geographic-handoff', handoff], enabled: !!handoff, gcTime: 0, retry: false,
    queryFn: ({signal}) => api.geographicContext(handoff, signal)})
  const [offset, setOffset] = useState(0)
  const [download, setDownload] = useState('')
  const eventItems = read.data?.eventState === 'available' && !read.data.events?.truncated ? read.data.events?.items : undefined
  const reports = read.data?.layers.bulletins
  const members = eventItems && [
    ...eventItems.map(event => ({id: event.evidenceId, title: event.reported.title, at: event.reported.observedAt, revision: event.providerRevision})),
    ...(reports && !reports.truncated && reports.state !== 'unavailable' ? reports.features.flatMap(item => {
      const props = item.properties as Record<string,unknown>
      return props.reportHistory && typeof item.id === 'string'
        ? [{id:item.id,title:String(props.title),at:String(props.date),revision:String(props.providerRevision)}] : []
    }) : []),
  ]
  useEffect(() => {
    if (!saved.isSuccess) {setDownload(''); return}
    const url = URL.createObjectURL(new Blob([JSON.stringify(saved.data, null, 2)], {type: 'application/json'}))
    setDownload(url)
    return () => URL.revokeObjectURL(url)
  }, [saved.isSuccess, saved.data])

  function preview() {
    const fixed = fixedScopeInput(scope)
    const date = cutoff ? new Date(cutoff) : undefined
    if (date && (!Number.isFinite(date.getTime()) || !/T.*(?:Z|[+-]\d{2}:\d{2})$/i.test(cutoff))) {setFailure('Enter an ISO date and time with its time zone.'); return}
    setFailure(''); setSelected([]); setSelectedMoving([]); setPinnedTrack(movingTrack.trim()); setConfirmation(false); setOffset(0); intent.current = undefined
    const input: ScopeReadInput = {area: {revisionId: terrain.areaRevisionId}, temporal: {...fixed.temporal,
      mode: 'historical', revisionView: date ? 'known_at' : 'latest', asOf: date?.toISOString()},
      families: session.permissions.includes('sources.read') ? ['bulletins'] : [], filters: {}, zoom: 14, includeEvents: true, purpose: 'geographic_handoff'}
    if (JSON.stringify(input) === JSON.stringify(query)) void read.refetch()
    else setQuery(input)
  }

  async function freeze() {
    if (!read.data || !members || !confirmation || (selectedMoving.length>0 && (!movingRead.data || movingRead.data.truncated))) return
    intent.current ??= {requestKey: crypto.randomUUID(), readId: read.data.id,
      geography: {releaseId: terrain.releaseId, manifestSha256: terrain.manifestSha256,
        displays: [terrain, ...displays].map(display => ({id: display.id, sha256: display.sha256}))},
      evidenceIds: selected, movingTrackId:selectedMoving.length?pinnedTrack:undefined,
      movingEvidenceIds:selectedMoving.length?selectedMoving:undefined, missingData: 'explicit_unavailable'}
    setBusy(true); setFailure('')
    try {
      const result = await api.freezeGeographicContext(intent.current, signal)
      if (!signal.aborted) {setHandoff(result.id); setLookup(result.id)}
    } catch (error) {
      if (!signal.aborted) setFailure(error instanceof Error ? error.message : 'The handoff could not be frozen.')
    } finally {
      if (!signal.aborted) setBusy(false)
    }
  }

  return <section aria-label="Frozen geographic handoff" className="space-y-2">
    <p>Freeze this release, its enabled display editions, and exact reported revisions within the selected Atlas interval.</p>
    <label>Known-by cutoff (optional ISO timestamp)<input className="atlas-field" value={cutoff} onChange={event => setCutoff(event.target.value)} placeholder="2026-11-12T15:00:00Z" /></label>
    {session.permissions.includes('tracks.read')&&<label>Moving track ID (optional; up to six hours)<input className="atlas-field" value={movingTrack} maxLength={128} onChange={event=>setMovingTrack(event.target.value)} /></label>}
    <button className="ux-button" disabled={busy || read.isFetching} onClick={preview}>Read observation revisions</button>
    {read.isError && <p role="alert">Observation membership could not be read. {read.error.message}</p>}
    {read.isSuccess && !members && <p role="status">Complete observation membership is unavailable. Narrow the interval and read again.</p>}
    {reports && (reports.truncated || reports.state === 'unavailable') && <p role="status">Report membership is incomplete. Narrow the interval to select provider reports.</p>}
    {movingRead.isError&&<p role="alert">Moving revisions could not be read. {movingRead.error.message}</p>}
    {members && <fieldset disabled={busy}>
      <legend>{members.length} reported revisions · {selected.length} selected</legend>
      <p>Read expires at <time dateTime={read.data!.expiresAt}>{read.data!.expiresAt}</time>.</p>
      <ul>{members.slice(offset, offset + 20).map(event => <li key={event.id}>
        <label><input type="checkbox" checked={selected.includes(event.id)} disabled={!selected.includes(event.id) && selected.length+selectedMoving.length >= 200}
          onChange={change => {intent.current = undefined; setConfirmation(false); setSelected(current => change.target.checked ? [...current, event.id] : current.filter(id => id !== event.id))}} />
          {event.title} · {event.at} · {event.revision}</label>
      </li>)}</ul>
      {offset > 0 && <button className="ux-button" onClick={() => setOffset(offset - 20)}>Previous observations</button>}
      {offset + 20 < members.length && <button className="ux-button" onClick={() => setOffset(offset + 20)}>Next observations</button>}
      {movingRead.data&&<><MovingHistory data={movingRead.data} />{!movingRead.data.truncated&&<details><summary>Select moving revisions ({selectedMoving.length} selected)</summary>
        <ul>{movingRead.data.items.map(item=><li key={item.evidenceId}><label><input type="checkbox" checked={selectedMoving.includes(item.evidenceId)}
          disabled={!selectedMoving.includes(item.evidenceId)&&selected.length+selectedMoving.length>=200}
          onChange={event=>{intent.current=undefined;setConfirmation(false);setSelectedMoving(current=>event.target.checked?[...current,item.evidenceId]:current.filter(id=>id!==item.evidenceId))}} />
          {item.decision} · {item.observedAt} · {item.providerRevision}</label></li>)}</ul></details>}</>}
      <label><input type="checkbox" checked={confirmation} onChange={event => setConfirmation(event.target.checked)} /> Freeze this selection with explicit unavailable handling for gaps</label>
      <button className="ux-button" disabled={!confirmation} onClick={() => void freeze()}>Freeze geographic handoff</button>
    </fieldset>}
    {failure && <p role="alert">{failure}</p>}
    <label>Saved handoff ID<input className="atlas-field" value={lookup} onChange={event => setLookup(event.target.value)} /></label>
    <button className="ux-button" disabled={!/^geoctx_[0-9a-f]{64}$/.test(lookup)} onClick={() => lookup === handoff ? void saved.refetch() : setHandoff(lookup)}>Reopen exact handoff</button>
    {saved.isError ? <p role="alert">{saved.error.message}</p> : saved.data && <>
      <p>{saved.data.id} · {saved.data.observations.length} frozen observations · Expires {saved.data.expiresAt}</p>
      {saved.data.moving&&<p>{saved.data.moving.items.length} frozen moving revisions. {saved.data.moving.meaning}</p>}
      <p>{saved.data.selectionMeaning}</p>
      {download && <a className="ux-button" href={download} download={`${saved.data.id}.json`}>Save handoff manifest</a>}
    </>}
  </section>
}
