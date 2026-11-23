import {useState, type FormEvent} from 'react'
import {useAPI} from '@/lib/api'
import type {components} from '@/lib/openapi.generated'
import {useWorkspace} from '@/lib/workspaceContext'

type Release = components['schemas']['EnvironmentRelease']
type Result = components['schemas']['RegionalResponseReview']
type Service = components['schemas']['RegionalServiceReview']
type Coast = components['schemas']['RegionalCoastReview']
type Path = components['schemas']['CivilianAccessPath']

const utcLabel = (value: string) => value.replace('T', ' ').replace(/Z$/, ' UTC')
const number = (value: number | null) => value === null ? 'Unavailable' : value.toLocaleString()
const stateLabel = (value: string) => value.replaceAll('-', ' ')

export function RegionalResponseReview({release}: {release: Release}) {
  const api = useAPI()
  const {signal} = useWorkspace()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [result, setResult] = useState<Result>()

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (busy) return
    const fields = new FormData(event.currentTarget)
    const value = (key: string) => String(fields.get(key) ?? '').trim()
    const file = fields.get('file')
    setResult(undefined)
    setError('')
    if (!(file instanceof File) || !file.size || file.size > 3 * 1024 * 1024) {
      setError('Choose a reviewed regional JSON file under 3 MiB.')
      return
    }
    const inputSha256 = value('inputSha256').toLowerCase()
    setBusy(true)
    try {
      const response = await api.reviewRegionalResponse(release.id, {
        sourceId: value('sourceId'), admissionId: value('admissionId'), inputSha256,
        manifestSha256: release.manifestSha256, knownAt: new Date(value('knownAt') + 'Z').toISOString(),
        staffedSlots: value('staffedSlots') === '' ? undefined : Number(value('staffedSlots')),
      }, file, signal)
      if (!signal.aborted && response.releaseId === release.id && response.manifestSha256 === release.manifestSha256 &&
        response.inputSha256 === inputSha256) setResult(response)
    } catch (err) {
      if (!signal.aborted) setError(err instanceof Error ? err.message : 'The regional review could not be read.')
    } finally {
      if (!signal.aborted) setBusy(false)
    }
  }

  return <details>
    <summary>Regional response and coastal evidence</summary>
    <form onSubmit={event => void submit(event)} onChange={() => {setResult(undefined); setError('')}}>
      <fieldset disabled={busy} className="space-y-2">
        <label className="block">Source ID<input name="sourceId" className="atlas-field" maxLength={128} required /></label>
        <label className="block">Reviewed admission ID<input name="admissionId" className="atlas-field" maxLength={128} required /></label>
        <label className="block">Regional package SHA-256<input name="inputSha256" className="atlas-field" pattern="[A-Fa-f0-9]{64}" required /></label>
        <label className="block">Evidence cutoff in UTC<input name="knownAt" className="atlas-field" type="datetime-local" step={1} required /></label>
        <label className="block">Additional staffing assumption, slots<input name="staffedSlots" className="atlas-field" type="number" min={0} max={128} step={1} /></label>
        <label className="block">Reviewed regional JSON<input name="file" type="file" className="atlas-field" accept=".json,application/json" required /></label>
        <button className="ux-button" type="submit">{busy ? 'Reviewing…' : 'Compare regional evidence'}</button>
      </fieldset>
    </form>
    {error && <p role="alert">{error}</p>}
    {result && <RegionalResults value={result} />}
  </details>
}

function RegionalResults({value}: {value: Result}) {
  const {review} = value
  return <section aria-label="Regional comparison results" className="space-y-3">
    <p role="status"><strong>{review.title}</strong> · {utcLabel(review.cutoff)}</p>
    {review.service && <ServiceComparison service={review.service} />}
    {review.indicators.length > 0 && <details>
      <summary>Published regional statistics and scope</summary>
      <ul className="space-y-2">{review.indicators.map(row => <li key={row.id}>
        <strong>{row.measure} · {number(row.value)} {row.unit}</strong>
        {row.nativeLabel !== row.measure && <p><bdi>{row.nativeLabel}</bdi></p>}
        <p>{row.region} ({row.regionCode}) · {row.geography} · {row.populationScope} · {row.channel}</p>
        <p>{row.periodFrom} to {row.periodToExclusive}, end exclusive · {row.sourceId}</p>
      </li>)}</ul>
      {review.reconciliations.map(item => <p key={item.totalId}>
        {review.indicators.find(row => row.id === item.totalId)?.measure} · component sum {number(item.componentSum)} · unallocated difference {number(item.unallocatedResidual)}
      </p>)}
    </details>}
    {review.coast && <CoastalEvidence coast={review.coast} />}
    <details><summary>Package and sources</summary>
      <p className="break-all">Package SHA-256 <code>{value.inputSha256}</code></p>
      <ul>{review.sources.map(source => <li key={source.id}>{source.id} · {source.edition} · {source.attribution} · <code className="break-all">{source.sha256}</code></li>)}</ul>
    </details>
  </section>
}

function accessLabel(path: Path) {
  return path.outcome === 'reachable' ? `Reachable · ${number(path.verifiedDistanceM)} m` : stateLabel(path.outcome)
}

function ServiceComparison({service}: {service: Service}) {
  return <section aria-label="Service comparison" className="space-y-2">
    <h4>{service.region} service comparison</h4>
    {service.zones.length > 0 && <details open>
      <summary>Road access by zone</summary>
      {service.referenceAt && <p>Reference {utcLabel(service.referenceAt)} · {service.graphEdition}</p>}
      <ul>{service.zones.map(zone => <li key={zone.id}>
        <strong>{zone.label}</strong> · {accessLabel(zone.reference)} → {accessLabel(zone.current)}
      </li>)}</ul>
      <p>The two road states use the same arrivals, staffing and service cutoff. Each state is fixed for the run. Arrivals from disconnected or unknown zones remain outside the queue.</p>
    </details>}
    <div className="grid gap-3 md:grid-cols-2">{service.workloads.map(workload => <section key={workload.id} aria-label={workload.label}>
      <h5>{workload.label}</h5>
      <p>{workload.kind === 'authored' ? 'Authored arrivals' : 'Reviewed historical arrivals'} · {workload.arrivals} by cutoff{workload.plannedArrivals !== null && ` of ${workload.plannedArrivals} planned`}</p>
      {service.zones.length > 0 && <p>{workload.reachable} reach the service point · {workload.disconnected} disconnected · {workload.accessUnknown} unknown access</p>}
      {workload.reference && <p>Reference road state · {workload.reference.reachable} reachable · {workload.reference.disconnected} disconnected · {workload.reference.accessUnknown} unknown access</p>}
      {service.zones.length > 0 && <details><summary>Arrivals by zone</summary>
        <ul>{service.zones.map(zone => <li key={zone.id}>{zone.label} · {workload.byZone[zone.id] ?? 0}</li>)}</ul>
      </details>}
      <ul className="space-y-2">{service.capacity.map(capacity => {
        const queue = workload.queues[capacity.id]
        const reference = workload.reference?.queues[capacity.id]
        return <li key={capacity.id}>
          <strong>{capacity.label}</strong> · {capacity.staffedSlots === null ? 'Capacity unknown' : `${capacity.staffedSlots} slots · ${capacity.kind}`}
          {queue && <>
            <p>{queue.completed} completed · {queue.inServiceAtCutoff} in service · {queue.awaitingService} waiting</p>
            {workload.reachable > 0 && <meter min={0} max={workload.reachable} value={queue.awaitingService} aria-label={`${queue.awaitingService} of ${workload.reachable} waiting`} />}
            <p>Wait among started cases · median {number(queue.medianStartedWaitSeconds)} s · 90th percentile {number(queue.p90StartedWaitSeconds)} s</p>
          </>}
          {reference && <p>Reference roads · {reference.completed} completed · {reference.inServiceAtCutoff} in service · {reference.awaitingService} waiting</p>}
        </li>
      })}</ul>
    </section>)}</div>
  </section>
}

function CoastalEvidence({coast}: {coast: Coast}) {
  return <details>
    <summary>{coast.port} coastal context</summary>
    {coast.portWatchId && <p>PortWatch {coast.portWatchId}{coast.inventory && ` · inventory ${coast.inventory.identifier} · ${coast.inventory.locality} · ${stateLabel(coast.inventory.match)}`}</p>}
    {coast.depth && <details><summary>Coastal grid and shoreline</summary>
      <p>{coast.depth.gridSourceId} · {coast.depth.shorelineSourceId} · {coast.depth.horizontalReference} · {coast.depth.verticalReference}</p>
      <ul>{coast.depth.samples.map(sample => <li key={sample.cell}>
        {sample.cell} · {sample.state} · {sample.elevationM === null ? 'Elevation unavailable' : `${sample.elevationM} m signed elevation`} · {sample.sourceType}
      </li>)}</ul>
    </details>}
    {coast.coverage.length > 0 && <details><summary>Historical reception and port calls</summary>
      <ul className="space-y-2">{coast.coverage.map(row => <li key={`${row.day}:${row.sourceId}`}>
        <strong>{row.day}</strong> · {row.sourceId} · {row.usableMinutes}/{row.windowMinutes} minutes covered
        <p><meter min={0} max={row.windowMinutes} value={row.usableMinutes} aria-label={`${row.usableMinutes} of ${row.windowMinutes} minutes covered`} /></p>
        <p>{number(row.uniqueReports)} unique messages · {number(row.duplicates)} duplicates · {number(row.portWatchCalls)} estimated calls{row.portWatchSourceId && ` · ${row.portWatchSourceId}`}</p>
        {row.portWatchAvailableAt && <p>PortWatch available {utcLabel(row.portWatchAvailableAt)}</p>}
      </li>)}</ul>
      <p>Message counts measure reception. PortWatch estimates daily port calls from AIS.</p>
    </details>}
    {coast.notices.length > 0 && <details><summary>Dated civilian notices and debriefs</summary>
      <ul className="space-y-2">{coast.notices.map(row => <li key={`${row.publisher}:${row.id}`}>
        <strong>{row.publisher} · {row.id} · revision {row.revision}</strong> · {row.status}
        <p>{row.warningArea} · {row.summary}</p>
        <p>Service impact {stateLabel(row.serviceImpact)}{row.impactSourceId && ` · ${row.impactSourceId}`}</p>
        <p>Issued {utcLabel(row.issuedAt)} · effective {utcLabel(row.effectiveAt)} · available {utcLabel(row.availableAt)}</p>
        <p className="break-all">Issuer reference {row.issuerReference}</p>
      </li>)}</ul>
    </details>}
  </details>
}
