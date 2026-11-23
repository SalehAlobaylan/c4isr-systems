import {useState, type FormEvent} from 'react'
import {useAPI} from '@/lib/api'
import type {components} from '@/lib/openapi.generated'
import {useWorkspace} from '@/lib/workspaceContext'

type Release = components['schemas']['EnvironmentRelease']
type Result = components['schemas']['SupportingSourceReview']
const words = (value: string) => value.replaceAll('-', ' ')
const utc = (value: string) => value.replace('T', ' ').replace(/Z$/, ' UTC')
const v = (value: number | null | undefined, unit = '') => value == null ? 'Unavailable' : `${value.toLocaleString(undefined, {maximumFractionDigits: 7})}${unit ? ` ${unit}` : ''}`

export function SupportingSourceReview({release}: {release: Release}) {
  const api = useAPI()
  const {signal} = useWorkspace()
  const [result, setResult] = useState<Result>()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  async function review(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (busy) return
    const fields = new FormData(event.currentTarget)
    const input = (key: string) => String(fields.get(key) ?? '').trim()
    const file = fields.get('file')
    setResult(undefined)
    setError('')
    if (!(file instanceof File) || !file.size || file.size > 3 * 1024 * 1024) {
      setError('Choose a reviewed supporting evidence JSON file under 3 MiB.')
      return
    }
    setBusy(true)
    const inputSha256 = input('inputSha256').toLowerCase()
    try {
      const next = await api.reviewSupportingSources(release.id, {
        sourceId: input('sourceId'), admissionId: input('admissionId'), inputSha256,
        manifestSha256: release.manifestSha256, knownAt: new Date(input('knownAt') + 'Z').toISOString(),
      }, file, signal)
      if (!signal.aborted && next.releaseId === release.id && next.manifestSha256 === release.manifestSha256 && next.inputSha256 === inputSha256) {
        setResult(next)
      }
    } catch (err) {
      if (!signal.aborted) setError(err instanceof Error ? err.message : 'The supporting evidence could not be read.')
    } finally {
      if (!signal.aborted) setBusy(false)
    }
  }

  return <details>
    <summary>Supporting evidence replay</summary>
    <form onSubmit={event => void review(event)} onChange={() => {setResult(undefined); setError('')}}>
      <fieldset disabled={busy} className="space-y-2">
        <label className="block">Source ID<input name="sourceId" className="atlas-field" maxLength={128} required /></label>
        <label className="block">Reviewed admission ID<input name="admissionId" className="atlas-field" maxLength={128} required /></label>
        <label className="block">Supporting package SHA-256<input name="inputSha256" className="atlas-field" pattern="[A-Fa-f0-9]{64}" required /></label>
        <label className="block">Evidence cutoff in UTC<input name="knownAt" className="atlas-field" type="datetime-local" step={1} required /></label>
        <label className="block">Reviewed supporting JSON<input name="file" type="file" className="atlas-field" accept=".json,application/json" required /></label>
        <button className="ux-button" type="submit">{busy ? 'Reviewing…' : 'Replay supporting evidence'}</button>
      </fieldset>
    </form>
    {error && <p role="alert">{error}</p>}
    {result && <SupportingResults result={result} />}
  </details>
}

function SupportingResults({result}: {result: Result}) {
  const {review} = result
  const training = review.events.filter(row => row.kind === 'navigation-training')
  return <section aria-label="Supporting evidence results" className="space-y-3">
    <p role="status"><strong>{review.title}</strong> · {utc(review.cutoff)}</p>
    {review.delivery.length > 0 && <details open><summary>Authored report delivery comparison</summary>
      <ul className="space-y-2">{review.delivery.map(row => <li key={row.mode}>
        <strong>{words(row.mode)}</strong> · {row.received} received · {row.pending} pending · peak backlog {row.peakBacklog}
        <p>Median lag {v(row.p50LagSeconds, 's')} · 90th percentile {v(row.p90LagSeconds, 's')}</p>
        {row.mode === 'authored-delay' && <p>Recovery {row.recoveryStatus} · {v(row.recoverySeconds, 's after resumption')}</p>}
        <details><summary>Receipts and acknowledgments</summary><ul>{row.receipts.map(receipt => <li key={receipt.id}>
          {receipt.id} · {receipt.recipient} · sent {utc(receipt.sentAt)} · {receipt.receivedAt ? `received ${utc(receipt.receivedAt)}` : 'awaiting delivery'} · {receipt.acknowledgedAt ? `acknowledged ${utc(receipt.acknowledgedAt)}` : 'acknowledgment unavailable'}
        </li>)}</ul></details>
      </li>)}</ul>
    </details>}
    {review.events.length > 0 && <details><summary>Event and publication history</summary>
      <p>{review.uniqueOrigins} distinct evidence origins</p>
      <ol className="space-y-2">{review.events.map(row => <li key={`${row.sourceId}:${row.id}:${row.revision}`}>
        <strong>{row.label}</strong> · {words(row.kind)} · {words(row.status)} · edition {row.revision}
        {row.value != null && ` · ${v(row.value, row.unit)}`}
        <p>{row.area} · {row.origin} · {row.sourceId} · available {utc(row.availableAt)}</p>
        {row.effectiveAt && <p>Effective {utc(row.effectiveAt)}{row.expiresAt && ` · expires ${utc(row.expiresAt)}`}</p>}
        {row.details && <p>{row.details}</p>}
        {row.languages && <ul>{Object.entries(row.languages).map(([lang, text]) => <li key={lang}><bdi lang={lang}>{text}</bdi></li>)}</ul>}
      </li>)}</ol>
    </details>}
    {(['decision', 'debrief'] as const).map(lane => {
      const fields = review.fields.filter(row => row.lane === lane)
      return fields.length === 0 ? null : <details key={lane}><summary>{lane === 'decision' ? 'Decision-time conditions' : 'Retrospective measurements and models'}</summary>
        <ul className="space-y-2">{fields.map(row => <li key={row.id}>
          <strong>{row.variable}</strong> · {row.status === 'available' ? v(row.value, row.units) : words(row.status)} · {row.area}
          <p>{words(row.kind)} · {words(row.method)} · {row.support} · {row.reference} · {row.nativeRef}</p>
          <p>Valid {utc(row.validFrom)}{row.validTo && ` through ${utc(row.validTo)}`} · available {utc(row.availableAt)}{row.initializedAt && ` · run ${utc(row.initializedAt)}`}</p>
          <p>{row.sourceId}{row.samples != null && ` · ${v(row.samples)} samples`}{row.coverage != null && ` · ${v(100 * row.coverage)}% coverage`}</p>
        </li>)}</ul>
      </details>
    })}
    {review.indicators.length > 0 && <details><summary>Humanitarian indicator coverage</summary>
      <ul className="space-y-2">{review.indicators.map(row => <li key={row.id}>
        <strong>{row.definition}</strong> · {row.status === 'available' ? v(row.value, row.units) : words(row.status)} · {row.kind}
        <p>{row.area} · {row.level} · {row.from} to {row.toExclusive}, end exclusive</p>
        {(row.origin || row.asylum) && <p>Origin {row.origin || 'Unavailable'} · asylum {row.asylum || 'Unavailable'}</p>}
        <p>{row.method} · {row.boundary} · upstream {row.upstream.startsWith('https://')
          ? <a href={row.upstream} target="_blank" rel="noreferrer">original release</a> : row.upstream}</p>
      </li>)}</ul>
    </details>}
    {review.comparisons.length > 0 && <details><summary>Comparable values and breaks in coverage</summary>
      <ul>{review.comparisons.map(row => <li key={`${row.before}:${row.after}`}>
        {row.before} → {row.after} · {row.status === 'comparable' ? `change ${v(row.difference)}` : words(row.status)}
      </li>)}</ul>
    </details>}
    {training.length > 0 && <details><summary>Navigation uncertainty exercise</summary>
      <form className="space-y-3" onSubmit={event => {
        event.preventDefault()
        const fields = new FormData(event.currentTarget)
        const cases = training.map(({id}) => ({id, state: fields.get(`${id}-state`), cause: fields.get(`${id}-cause`),
          requestObservation: fields.has(`${id}-reobserve`)}))
        const url = URL.createObjectURL(new Blob([JSON.stringify({exerciseSha256: result.inputSha256,cases}, null, 2)], {type: 'application/json'}))
        const anchor = document.createElement('a')
        anchor.href = url
        anchor.download = 'civilian-navigation-decisions.json'
        anchor.click()
        setTimeout(() => URL.revokeObjectURL(url), 1000)
      }}>
        {training.map(row => <fieldset key={row.id}>
          <legend>{row.id}</legend>
          <label className="block" htmlFor={`support-${row.id}-state`}>Coverage state</label>
          <select id={`support-${row.id}-state`} name={`${row.id}-state`} className="atlas-field" defaultValue="" required>
            <option disabled value="">Select state</option><option value="missing">Observation missing</option><option value="degraded">Degraded summary</option>
          </select>
          <label className="block" htmlFor={`support-${row.id}-cause`}>Cause</label>
          <select id={`support-${row.id}-cause`} name={`${row.id}-cause`} className="atlas-field" defaultValue="" required>
            <option disabled value="">Select confidence</option><option value="unknown">Cause unknown</option><option value="asserted">Cause asserted</option>
          </select>
          <label><input type="checkbox" name={`${row.id}-reobserve`} /> Request an independent observation</label>
        </fieldset>)}
        <button className="ux-button" type="submit">Download training decisions</button>
      </form>
    </details>}
    <details><summary>Package and source lineage</summary>
      <p className="break-all">Package SHA-256 <code>{result.inputSha256}</code></p>
      <ul>{review.sources.map(source => <li key={source.id}>{source.id} · {source.edition} · {source.attribution} · <code className="break-all">{source.sha256}</code></li>)}</ul>
    </details>
  </section>
}
