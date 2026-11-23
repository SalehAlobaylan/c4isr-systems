import {useState, type FormEvent} from 'react'
import {useAPI} from '@/lib/api'
import type {components} from '@/lib/openapi.generated'
import {useWorkspace} from '@/lib/workspaceContext'

type Release = components['schemas']['EnvironmentRelease']
type Review = components['schemas']['CivilianAerialReview']
type Scene = components['schemas']['AerialReviewScene']
const utcLabel = (value: string) => value.replace('T', ' ').replace(/Z$/, ' UTC')
const percentage = (number: number | null) => number === null ? 'Unknown' : `${(number * 100).toFixed(1)}%`

export function CivilianAerialReview({release}: {release: Release}) {
  const api = useAPI()
  const {signal} = useWorkspace()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [review, setReview] = useState<Review>()

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (busy) return
    setReview(undefined)
    setError('')
    const fields = new FormData(event.currentTarget)
    const value = (name: string) => String(fields.get(name) ?? '').trim()
    const file = fields.get('file')
    if (!(file instanceof File) || !file.size || file.size > 3 * 1024 * 1024) {
      setError('Choose a reviewed aerial exercise JSON file under 3 MiB.')
      return
    }
    const inputSha256 = value('inputSha256').toLowerCase()
    setBusy(true)
    try {
      const result = await api.reviewCivilianAerial(release.id, {
        sourceId: value('sourceId'), admissionId: value('admissionId'), inputSha256,
        manifestSha256: release.manifestSha256, knownAt: new Date(value('knownAt') + 'Z').toISOString(),
      }, file, signal)
      if (!signal.aborted && result.releaseId === release.id && result.manifestSha256 === release.manifestSha256 &&
          result.inputSha256 === inputSha256) setReview(result)
    } catch (err) {
      if (!signal.aborted) setError(err instanceof Error ? err.message : 'Aerial evidence review failed.')
    } finally {
      if (!signal.aborted) setBusy(false)
    }
  }

  return <details>
    <summary>Civilian aerial inspection replay</summary>
    <form onSubmit={event => void submit(event)} onChange={() => {setReview(undefined); setError('')}}>
      <fieldset disabled={busy} className="space-y-2">
        <label className="block">Source ID<input name="sourceId" className="atlas-field" maxLength={128} required /></label>
        <label className="block">Reviewed admission ID<input name="admissionId" className="atlas-field" maxLength={128} required /></label>
        <label className="block">Aerial package SHA-256<input name="inputSha256" className="atlas-field" pattern="[A-Fa-f0-9]{64}" required /></label>
        <label className="block">Evidence cutoff in UTC<input name="knownAt" className="atlas-field" type="datetime-local" step={1} required /></label>
        <label className="block">Reviewed aerial JSON<input name="file" type="file" className="atlas-field" accept=".json,application/json" required /></label>
        <button className="ux-button" type="submit">{busy ? 'Reviewing…' : 'Review aerial evidence'}</button>
      </fieldset>
    </form>
    {error && <p role="alert">{error}</p>}
    {review && <AerialResults key={`${review.inputSha256}:${review.review.cutoff}`} value={review} />}
  </details>
}

function AerialResults({value}: {value: Review}) {
  const data = value.review
  const [mask, setMask] = useState(false)
  const [floodOverlay, setFloodOverlay] = useState(false)
  const [decision, setDecision] = useState('')
  const [owner, setOwner] = useState('')
  const [acceptedOwner, setAcceptedOwner] = useState('')
  const [acknowledged, setAcknowledged] = useState(false)
  const [paused, setPaused] = useState(false)
  const hasPair = data.before && data.after

  function downloadHandoff() {
    const events = []
    if (acknowledged) events.push({kind: 'acknowledged', at: data.cutoff})
    if (paused) events.push({kind: 'paused', at: data.cutoff})
    if (acceptedOwner) events.push({kind: 'accepted-follow-up', at: data.cutoff, owner: acceptedOwner})
    const file = new Blob([JSON.stringify({reviewSha256: value.inputSha256, events}, null, 2)], {type: 'application/json'})
    const url = URL.createObjectURL(file)
    const link = document.createElement('a')
    link.href = url
    link.download = 'civilian-handoff.json'
    link.click()
    setTimeout(() => URL.revokeObjectURL(url), 1000)
  }
  return <section aria-label="Aerial inspection results" className="space-y-2">
    <p role="status"><strong>{data.title}</strong> · {data.kind === 'authored-exercise' ? 'Authored exercise' : 'Reviewed history'} · {utcLabel(data.cutoff)}</p>
    <p>Study area {data.areaRevision} · incident {utcLabel(data.incidentAt)}</p>
    {hasPair ? <>
      <label><input type="checkbox" checked={mask} onChange={event => setMask(event.target.checked)} /> Show local quality mask</label>
      {hasPair.assets.some(asset => asset.role === 'flood-overlay') && <label className="block">
        <input type="checkbox" checked={floodOverlay} onChange={event => setFloodOverlay(event.target.checked)} /> Show suggested flooded-road pixels
      </label>}
      <div className="grid gap-3 sm:grid-cols-2">
        {data.before && <ScenePreview title="Before" scene={data.before} mask={mask} />}
        <ScenePreview title="After" scene={hasPair} mask={mask} floodOverlay={floodOverlay} />
      </div>
    </> : <p role="status">Another observation is needed. Review the available candidates and their limitations.</p>}
    <details><summary>Scene selection evidence</summary>
      <ul>{data.candidates.map(scene => <li key={scene.id}>
        {scene.id} · {scene.state.replaceAll('-', ' ')} · {percentage(scene.coveredFraction)} covered · {percentage(scene.usableFraction)} usable
        {scene.catalogCoverage === 'partial-page' && ' · catalog query is partial'}
      </li>)}</ul>
    </details>
    <fieldset className="ux-actions"><legend>Exercise inspection decision</legend>
      <button className="ux-button" type="button" aria-pressed={decision === 'field'} onClick={() => setDecision('field')}>Refer for field inspection</button>
      <button className="ux-button" type="button" aria-pressed={decision === 'reobserve'} onClick={() => setDecision('reobserve')}>Request a clearer view</button>
      <button className="ux-button" type="button" disabled={!hasPair} aria-pressed={decision === 'noted'} onClick={() => setDecision('noted')}>Record imagery evidence</button>
    </fieldset>
    {decision && <p role="status">Exercise decision recorded at this cutoff. Field findings determine access.</p>}
    {data.assessments.length > 0 && <details><summary>Flood mapping assessments</summary>
      <ul>{data.assessments.map(item => <li key={`${item.id}:${item.version}`}>
        {item.label} · {item.kind} · version {item.version} · {utcLabel(item.availableAt)} · {item.coverage} assessment coverage
        {item.affected !== null && ` · ${item.affected ? 'Intersecting mapped effect' : 'No mapped effect in the reviewed area'}`}
        {item.sharesScene && ' · Shares a selected source scene'}
        {!!item.classes?.length && <p>{item.classes.join(', ')}</p>}
        {!!item.sourceImageTimes?.length && <p>Source images acquired {item.sourceImageTimes.map(utcLabel).join(', ')}</p>}
      </li>)}</ul>
    </details>}
    {data.briefing.length > 0 && <details><summary>Civilian handoff exercise</summary>
      <ol>{data.briefing.map(cue => <li key={cue.id}>{utcLabel(cue.availableAt)} · {cue.briefing}</li>)}</ol>
      <div className="ux-actions">
        <button type="button" className="ux-button" aria-pressed={acknowledged} onClick={() => setAcknowledged(true)}>Acknowledge briefing</button>
        <button type="button" className="ux-button" aria-pressed={paused} onClick={() => setPaused(true)}>Pause exercise inspection</button>
      </div>
      <label className="block">Follow-up owner<input className="atlas-field" value={owner} maxLength={128} onChange={event => setOwner(event.target.value)} /></label>
      <button type="button" className="ux-button" disabled={!owner.trim()} onClick={() => setAcceptedOwner(owner.trim())}>Record handoff</button>
      {acceptedOwner && <p role="status">Follow-up assigned in this review to {acceptedOwner}.</p>}
      <button type="button" className="ux-button" onClick={downloadHandoff}>Download handoff for scoring</button>
    </details>}
    {data.traffic.length > 0 && <details><summary>Historical civilian traffic coverage</summary>
      <p>Completed days in the supplied record. Hours without observations remain unknown.</p>
      <ul>{data.traffic.map(day => <li key={`${day.sourceId}:${day.day}`}>
        {day.day} · {day.sourceId} · {day.usableHours}/24 usable hours · {day.civilCount ?? 'Unavailable'} civilian observations · {day.unclassified ?? 'Unavailable'} unclassified
        {day.oldestAt && ` · Earliest ${utcLabel(day.oldestAt)}`}
      </li>)}</ul>
    </details>}
    {data.notices.length > 0 && <details><summary>Airport publications and service notices</summary>
      <ul>{data.notices.map(item => <li key={`${item.publisher}:${item.id}:${item.revision}`}>
        {item.publisher} · {item.title} · {item.status} · {item.deliveryIds.length} delivery records
        <p>Published {utcLabel(item.publishedAt)} · effective {utcLabel(item.effectiveAt)}</p>
      </li>)}</ul>
    </details>}
    {data.survey && <p role="status">{data.survey.coveredCells}/{data.survey.requiredCells} required survey cells found · {data.survey.duplicateVisits} overlap visits · {data.survey.aborted}/{data.survey.assignments} assignments aborted</p>}
    {data.energy.length > 0 && <details><summary>Completed research flight energy comparison</summary>
      <ul>{data.energy.map(row => <li key={`${row.sourceId}:${row.platform}:${row.testType}`}>
        {row.platform} · {row.testType} · {row.trainingFlights} training flights · {row.scoredFlights}/{row.heldOutFlights} held-out flights scored
        <p>Absolute error in Wh · median {row.medianAbsoluteErrorWh?.toFixed(2) ?? 'Unavailable'} · 90th percentile {row.p90AbsoluteErrorWh?.toFixed(2) ?? 'Unavailable'} · {row.excludedHeldOutSeconds.toFixed(1)} held-out seconds excluded</p>
      </li>)}</ul>
    </details>}
    <details><summary>Package and source evidence</summary>
      <p className="break-all">Package SHA-256 <code>{value.inputSha256}</code></p>
      <ul>{data.sources.map(source => <li key={source.id}>{source.edition} · {source.attribution} · <code className="break-all">{source.sha256}</code></li>)}</ul>
    </details>
  </section>
}

function ScenePreview({title, scene, mask, floodOverlay}: {title: string; scene: Scene; mask: boolean; floodOverlay?: boolean}) {
  const picture = scene.assets.find(asset => asset.role === 'visual')
  const quality = scene.assets.find(asset => asset.role === 'quality-mask')
  const flood = scene.assets.find(asset => asset.role === 'flood-overlay')
  return <figure>
    <div className="relative">
      {picture && <img className="block w-full" src={`data:image/png;base64,${picture.png}`} alt={`${title} reviewed image`} />}
      {floodOverlay && flood && <img className="absolute inset-0 h-full w-full" src={`data:image/png;base64,${flood.png}`} alt="Suggested flooded-road pixels" />}
      {mask && quality && <img className="absolute inset-0 h-full w-full" src={`data:image/png;base64,${quality.png}`} alt={`${title} review quality mask`} />}
    </div>
    <figcaption>{title} · {scene.sensorFamily} · {scene.pixelM} m pixels · {percentage(scene.usableFraction)} usable
      <p>Acquired {utcLabel(scene.acquiredAt)}{scene.availableAt && ` · available ${utcLabel(scene.availableAt)}`}</p>
      <p>{scene.timeBasis}{scene.catalogCoverage === 'partial-page' && ' · catalog query is partial'}</p>
      {mask && <p>Amber shows pixels marked unusable. Dark shading shows pixels outside coverage or the review extent.</p>}
      {floodOverlay && <p>Blue marks suggested flooded-road pixels for review.</p>}
      {scene.bounds?.length === 4 && <p>Requested WGS84 extent {scene.bounds.join(', ')}</p>}
    </figcaption>
  </figure>
}
