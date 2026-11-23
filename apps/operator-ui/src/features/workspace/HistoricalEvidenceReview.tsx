import {useState, type FormEvent} from 'react'
import {useAPI} from '@/lib/api'
import type {components} from '@/lib/openapi.generated'
import {useWorkspace} from '@/lib/workspaceContext'

type Release = components['schemas']['EnvironmentRelease']
type FloodReview = Awaited<ReturnType<ReturnType<typeof useAPI>['reviewFloodMapping']>>
type RadarReview = Awaited<ReturnType<ReturnType<typeof useAPI>['reviewRadarOutages']>>

function UtcTime({value}: {value: string}) {
  return <time dateTime={value}>{value.replace('T', ' ').replace(/Z$/, ' UTC')}</time>
}

export function HistoricalEvidenceReview({release, source}: {release: Release; source: 'flood' | 'outages'}) {
  const api = useAPI()
  const {signal} = useWorkspace()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [review, setReview] = useState<FloodReview | RadarReview>()
  const flood = source === 'flood'
  const title = flood ? 'Copernicus flood mapping' : 'Historical Internet outage annotations'

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (busy) return
    const fields = new FormData(event.currentTarget)
    const file = fields.get('file')
    const value = (name: string) => String(fields.get(name) ?? '').trim()
    setReview(undefined)
    setError('')
    if (!(file instanceof File) || file.size === 0 || file.size > 3 * 1024 * 1024) {
      setError('Choose a reviewed JSON file under 3 MiB.')
      return
    }
    const common = {
      manifestSha256: release.manifestSha256, sourceId: value('sourceId'),
      admissionId: value('admissionId'), inputSha256: value('inputSha256').toLowerCase(),
    }
    const utcField = (name: string) => new Date(value(name) + 'Z').toISOString()
    setBusy(true)
    try {
      const result = flood
        ? await api.reviewFloodMapping(release.id, {...common, activation: value('activation').toUpperCase(),
            knownAt: utcField('knownAt')}, file, signal)
        : await api.reviewRadarOutages(release.id, {...common, country: value('country').toUpperCase(),
            from: utcField('from'), toExclusive: utcField('toExclusive')}, file, signal)
      if (!signal.aborted && result.releaseId === release.id && result.manifestSha256 === release.manifestSha256) setReview(result)
    } catch (error) {
      if (!signal.aborted) setError(error instanceof Error ? error.message : 'The source file could not be reviewed.')
    } finally {
      if (!signal.aborted) setBusy(false)
    }
  }

  return <details>
    <summary>{title}</summary>
    <form onSubmit={event => void submit(event)} onChange={() => {setReview(undefined); setError('')}}>
      <fieldset disabled={busy} className="space-y-2">
        <label className="block">Source ID<input name="sourceId" className="atlas-field" maxLength={128} required /></label>
        <label className="block">Reviewed admission ID<input name="admissionId" className="atlas-field" maxLength={128} required /></label>
        <label className="block">Reviewed file SHA-256<input name="inputSha256" className="atlas-field" pattern="[A-Fa-f0-9]{64}" required /></label>
        {flood ? <>
          <label className="block">Public activation code<input name="activation" className="atlas-field" placeholder="EMSR761" pattern="[Ee][Mm][Ss][Rr][0-9]{1,8}" required /></label>
          <label className="block">Delivery cutoff in UTC<input name="knownAt" className="atlas-field" type="datetime-local" step={1} required /></label>
        </> : <>
          <label className="block">Country code<input name="country" className="atlas-field" defaultValue="SA" pattern="[A-Za-z]{2}" required /></label>
          <label className="block">Interval start in UTC<input name="from" className="atlas-field" type="datetime-local" step={1} required /></label>
          <label className="block">Interval end in UTC, exclusive<input name="toExclusive" className="atlas-field" type="datetime-local" step={1} required /></label>
        </>}
        <label className="block">{flood ? 'Activation JSON' : 'Radar annotations JSON'}<input name="file" type="file" className="atlas-field" accept=".json,application/json" required /></label>
        <button className="ux-button" type="submit">{busy ? 'Reviewing…' : 'Review evidence'}</button>
      </fieldset>
    </form>
    {error && <p role="alert">{error}</p>}
    {review && <section aria-label={`${title} results`} className="space-y-2">
      {'floodMapping' in review ? <FloodResults value={review.floodMapping} /> : <RadarResults value={review.outages} />}
      <details>
        <summary>Source and release evidence</summary>
        <p>{review.attribution} · {review.productVersion}</p>
        <p className="break-all">Input SHA-256 <code>{review.inputSha256}</code></p>
        <p className="break-all">Release SHA-256 <code>{review.manifestSha256}</code></p>
        <p className="break-all">Reviewed admission <code>{review.admissionId}</code> · Current admission <code>{review.currentAdmissionId}</code></p>
      </details>
    </section>}
  </details>
}

function FloodResults({value}: {value: FloodReview['floodMapping']}) {
  return <>
    <p role="status">{value.activation} · {value.products.length} delivered versions in this file by <UtcTime value={value.cutoff} /></p>
    <p>The history covers versions retained in this file. Use independent inspections for road access decisions.</p>
    <ul className="max-h-80 overflow-auto space-y-2">
      {value.products.map(product => <li key={product.versionId}>
        <details>
          <summary>{product.areaName} · {product.type === 'DEL' ? 'Delineation' : 'Grading'} · version {product.versionNumber}</summary>
          <p>Product {product.productId} · delivered <UtcTime value={product.deliveredAt} /></p>
          <p>Monitoring round {product.monitoringNumber} · area {product.areaNumber}</p>
          <ul>{product.sourceImages.map(image => <li key={image.sourceImageId}>
            {image.sensor || image.sourceImageId} · {image.acquisitionAt ? <UtcTime value={image.acquisitionAt} /> : 'Acquisition time unavailable'}
          </li>)}</ul>
        </details>
      </li>)}
    </ul>
  </>
}

function RadarResults({value}: {value: RadarReview['outages']}) {
  return <>
    <p role="status">{value.country} · {value.annotations.length} outage annotations in this file</p>
    <p>Annotation availability at the time is unknown. Use the reported intervals as retrospective context.</p>
    <ul className="max-h-80 overflow-auto space-y-2">
      {value.annotations.map(item => <li key={item.id}>
        <strong>{item.providerType}</strong> · <UtcTime value={item.startAt} /> · {item.endAt ? <UtcTime value={item.endAt} /> : 'End time unknown'}
      </li>)}
    </ul>
  </>
}
