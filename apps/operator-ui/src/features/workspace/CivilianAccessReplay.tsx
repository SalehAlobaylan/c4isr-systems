import {useState, type FormEvent} from 'react'
import {useAPI} from '@/lib/api'
import type {components} from '@/lib/openapi.generated'
import {useWorkspace} from '@/lib/workspaceContext'

type Release = components['schemas']['EnvironmentRelease']
type Review = components['schemas']['CivilianAccessReview']
type Path = components['schemas']['CivilianAccessPath']

const outcomes = {
  reachable: 'Reachable',
  unknown: 'Access uncertain',
  'outside-known-network': 'Outside reviewed network',
  'unreachable-in-graph': 'Unreachable on this graph',
}

const dateTime = (value: string) => value.replace('T', ' ').replace(/Z$/, ' UTC')

export function CivilianAccessReplay({release, onLocate}: {release: Release; onLocate?: (feature: string) => void}) {
  const api = useAPI()
  const {signal} = useWorkspace()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [result, setResult] = useState<Review>()
  const [reference, setReference] = useState<Review>()
  const [decisions, setDecisions] = useState<Record<string, 'linked' | 'separate'>>({})

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (busy) return
    const fields = new FormData(event.currentTarget)
    const value = (key: string) => String(fields.get(key) ?? '').trim()
    const file = fields.get('file')
    setError('')
    setResult(undefined)
    if (!(file instanceof File) || !file.size || file.size > 3 * 1024 * 1024) {
      setError('Choose a reviewed civilian road exercise JSON file under 3 MiB.')
      return
    }
    const query = {sourceId: value('sourceId'), admissionId: value('admissionId'),
      inputSha256: value('inputSha256').toLowerCase(), manifestSha256: release.manifestSha256}
    setBusy(true)
    try {
      const cut = new Date(value('knownAt') + 'Z').toISOString()
      const review = await api.reviewCivilianAccess(release.id, {...query, knownAt: cut}, file, signal)
      if (!signal.aborted && review.releaseId === release.id && review.manifestSha256 === release.manifestSha256 &&
          review.inputSha256 === query.inputSha256) {
        setResult(review)
      }
    } catch (error) {
      if (!signal.aborted) setError(error instanceof Error ? error.message : 'The exercise could not be replayed.')
    } finally {
      if (!signal.aborted) setBusy(false)
    }
  }

  const pinned = reference && reference.inputSha256 === result?.inputSha256 &&
    reference.currentAdmissionId === result.currentAdmissionId ? reference.replay : undefined

  return <details>
    <summary>Riyadh reports and road access replay</summary>
    <form onSubmit={event => void submit(event)} onChange={() => {setResult(undefined); setError('')}}>
      <fieldset disabled={busy} className="space-y-2">
        <label className="block">Source ID<input name="sourceId" className="atlas-field" maxLength={128} required /></label>
        <label className="block">Reviewed admission ID<input name="admissionId" className="atlas-field" maxLength={128} required /></label>
        <label className="block">Exercise file SHA-256<input name="inputSha256" className="atlas-field" pattern="[A-Fa-f0-9]{64}" required /></label>
        <label className="block">Evidence cutoff in UTC<input name="knownAt" className="atlas-field" type="datetime-local" step={1} required /></label>
        <label className="block">Reviewed road exercise JSON<input name="file" type="file" className="atlas-field" accept=".json,application/json" required /></label>
        <button className="ux-button" type="submit">{busy ? 'Replaying…' : 'Replay access'}</button>
      </fieldset>
    </form>
    {error && <p role="alert">{error}</p>}
    {result && <section aria-label="Civilian access replay results" className="space-y-3">
      <p role="status"><strong>{result.replay.title}</strong> · {result.replay.kind === 'authored-exercise' ? 'Authored exercise' : 'Reviewed history'} · {dateTime(result.replay.cutoff)}</p>
      <p>{result.replay.graphEdition} · From {result.replay.origin} · {result.replay.serviceClass}</p>
      <button type="button" className="ux-button" onClick={() => setReference(result)}>Use this cutoff as comparison</button>
      {pinned && <p>Comparing with {dateTime(pinned.cutoff)} <button type="button" className="ux-button" onClick={() => setReference(undefined)}>Clear comparison</button></p>}
      <div className="max-h-96 overflow-auto" tabIndex={0} aria-label="Destination access by evidence and planned works">
        <table className="w-full text-left">
          <thead><tr><th scope="col">Destination</th>{pinned && <th scope="col">Earlier selected cutoff</th>}<th scope="col">Observed access</th><th scope="col">With scheduled works</th><th scope="col">Known resident estimate</th></tr></thead>
          <tbody>{result.replay.destinations.map(zone => <tr key={zone.id}>
            <th scope="row">{zone.label}</th>
            {pinned && <td><RouteReading path={pinned.destinations.find(item => item.id === zone.id)?.observed} /></td>}
            <td><RouteReading path={zone.observed} /></td><td><RouteReading path={zone.withPlannedWork} /></td>
            <td>{zone.knownPeople?.toLocaleString(undefined, {maximumFractionDigits: 2}) ?? 'Unavailable'}{zone.unknownCells > 0 && ` · ${zone.unknownCells} unknown cells`}</td>
          </tr>)}</tbody>
        </table>
      </div>
      {result.replay.populationYear && <p>Model reference year {result.replay.populationYear}{result.replay.populationAllocation && ` · ${result.replay.populationAllocation}`} · {result.replay.unallocatedPeople?.toLocaleString()} known residents outside allocated zones</p>}
      <details><summary>Reports and associations</summary>
        {result.replay.associationChecks.reviewedPairs > 0 && <p>
          {result.replay.associationChecks.reviewedPairs} reviewed pairs · {result.replay.associationChecks.falseCandidates} incorrect suggestions · {result.replay.associationChecks.missedLinks} missed links
        </p>}
        <ul>{result.replay.reports.map(report => <li key={report.id} className="space-y-1">
          <p>{report.id} · <bdi lang="ar">{report.category}</bdi> · {report.placeLabel ? <bdi lang="ar">{report.placeLabel}</bdi> : 'Unresolved place'}</p>
          {report.localText && <p dir="auto">{report.localText}</p>}
          <p>Available {dateTime(report.availableAt)}{report.ticketClosedAt && ` · Ticket closed ${dateTime(report.ticketClosedAt)}`}</p>
        </li>)}</ul>
        <ul>{result.replay.associations.map(item => {
          const pair = JSON.stringify([item.reportId, item.otherId])
          const key = JSON.stringify([result.inputSha256, result.currentAdmissionId, result.replay.cutoff, pair])
          return <li key={pair}>
            {item.reportId} and {item.otherId} · {item.decision === 'suggested' ? 'Candidate for review' : item.decision}{item.reviewer && ` · ${item.reviewer}`}
            {item.decision === 'suggested' && <span className="ux-actions">
              <button className="ux-button" type="button" aria-pressed={decisions[key] === 'linked'}
                onClick={() => setDecisions({...decisions, [key]: 'linked'})}>Accept in this review</button>
              <button className="ux-button" type="button" aria-pressed={decisions[key] === 'separate'}
                onClick={() => setDecisions({...decisions, [key]: 'separate'})}>Keep separate in this review</button>
            </span>}
          </li>
        })}</ul>
      </details>
      <details><summary>Road findings and inspections</summary>
        <ul>{result.replay.assignments.map(assignment => <li key={assignment.id}>
          {assignment.crewLabel} · {assignment.reportId} · dispatched {dateTime(assignment.dispatchedAt)} ·
          {assignment.arrivedAt ? ` arrived ${dateTime(assignment.arrivedAt)}` : ' arrival unverified'}
        </li>)}</ul>
        <ul>{result.replay.edges.map(edge => <li key={edge.id}>{edge.id} · {edge.from} → {edge.to} · {edge.state}
          {edge.evidenceIds.length > 0 && ` · ${edge.evidenceIds.join(', ')}`}{edge.plannedPermits.length > 0 && ` · Scheduled ${edge.plannedPermits.join(', ')}`}
          <small> · {edge.sourceId} · {edge.featureId}</small>
          {edge.retainedOSMFeatureId && onLocate && <button className="ux-button" type="button"
            onClick={() => onLocate(edge.retainedOSMFeatureId!)}>Inspect retained road in 3D</button>}
        </li>)}</ul>
        <ul>{result.replay.inspections.map(inspection => <li key={inspection.id}>
          {inspection.id} · {inspection.reviewer} · observed {dateTime(inspection.observedAt)} · available {dateTime(inspection.availableAt)}
        </li>)}</ul>
      </details>
      <details><summary>Planned works and amendments</summary>
        <ul>{result.replay.works.map(permit => <li key={permit.id}>
          {permit.roadLabel} · {permit.status} · revision {permit.revision} · {permit.edgeId ?? 'Unresolved road match'}
          <br />{dateTime(permit.from)} to {dateTime(permit.toExclusive)}
        </li>)}</ul>
      </details>
      {result.replay.restoration.length > 0 && <details><summary>Facility restoration</summary>
        <ul>{result.replay.restoration.map(task => <li key={task.id}>
          {task.label} · {task.destinationId}
          {task.expectedCompleteAt && <p>Expected {dateTime(task.expectedCompleteAt)}</p>}
          <p>{task.completedAt ? `Completed ${dateTime(task.completedAt)}` : 'Completion unverified'}</p>
          {task.cost !== undefined && <p>Reviewed cost {task.cost.toLocaleString()} {task.currency} · {task.pricingYear} prices</p>}
        </li>)}</ul>
      </details>}
      <details><summary>Source and replay evidence</summary>
        <p className="break-all">Package SHA-256 <code>{result.inputSha256}</code></p>
        <p className="break-all">Release SHA-256 <code>{result.manifestSha256}</code></p>
        <ul>{result.replay.sources.map(source => <li key={source.id}>{source.edition} · {source.attribution} · <code className="break-all">{source.sha256}</code></li>)}</ul>
      </details>
    </section>}
  </details>
}

function RouteReading({path}: {path?: Path}) {
  if (!path) return <>Unavailable</>
  return <>
    {outcomes[path.outcome]}{path.verifiedDistanceM !== null && ` · ${path.verifiedDistanceM.toLocaleString(undefined, {maximumFractionDigits: 1})} m`}
    {path.edgeIds.length > 0 && <small className="block">{path.edgeIds.join(' → ')}</small>}
  </>
}
