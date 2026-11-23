import {useState, type FormEvent} from 'react'
import {useAPI} from '@/lib/api'
import type {components} from '@/lib/openapi.generated'
import {useWorkspace} from '@/lib/workspaceContext'

type Release = components['schemas']['EnvironmentRelease']
type Review = Awaited<ReturnType<ReturnType<typeof useAPI>['reviewPortActivity']>>

const comparisons = {
  'missing-day': 'No record',
  'insufficient-history': 'Insufficient baseline',
  'below-history': 'Below baseline range',
  'above-history': 'Above baseline range',
  'within-history': 'Within baseline range',
}

export function PortActivityReview({release}: {release: Release}) {
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
    const file = fields.get('file')
    const value = (key: string) => String(fields.get(key) ?? '').trim()
    if (!(file instanceof File) || !file.size || file.size > 3 * 1024 * 1024) {
      setError('Choose a PortWatch JSON file under 3 MiB.')
      return
    }
    setBusy(true)
    try {
      const result = await api.reviewPortActivity(release.id, {
        manifestSha256: release.manifestSha256,
        sourceId: value('sourceId'), admissionId: value('admissionId'), inputSha256: value('inputSha256').toLowerCase(),
        portId: value('portId') === 'port2074' ? 'port2074' : 'port24',
        baselineFrom: value('baselineFrom'), from: value('from'), toExclusive: value('toExclusive'),
        minimumWeekdays: Number(value('minimumWeekdays')), consecutiveDays: Number(value('consecutiveDays')),
      }, file, signal)
      if (!signal.aborted && result.releaseId === release.id && result.manifestSha256 === release.manifestSha256) {
        setReview(result)
      }
    } catch (error) {
      if (!signal.aborted) setError(error instanceof Error ? error.message : 'The port activity could not be read.')
    } finally {
      if (!signal.aborted) setBusy(false)
    }
  }

  return <details>
    <summary>Historical port activity</summary>
    <p>Review a PortWatch daily export for one port. The baseline uses matching weekdays before the study period.</p>
    <form onSubmit={event => void submit(event)} onChange={() => setReview(undefined)}>
      <fieldset disabled={busy} className="space-y-2">
        <label className="block">Source ID<input name="sourceId" className="atlas-field" maxLength={128} required /></label>
        <label className="block">Reviewed admission ID<input name="admissionId" className="atlas-field" maxLength={128} required /></label>
        <label className="block">Reviewed file SHA-256<input name="inputSha256" className="atlas-field" pattern="[A-Fa-f0-9]{64}" required /></label>
        <label className="block">Port<select name="portId" className="atlas-field"><option value="port24">Jubail</option><option value="port2074">Jazan</option></select></label>
        <label className="block">Baseline start<input type="date" name="baselineFrom" className="atlas-field" defaultValue="2023-01-01" required /></label>
        <label className="block">Study start<input type="date" name="from" className="atlas-field" defaultValue="2024-01-01" required /></label>
        <label className="block">Study end, exclusive<input type="date" name="toExclusive" className="atlas-field" defaultValue="2025-01-01" required /></label>
        <label className="block">Minimum matching weekdays<input type="number" name="minimumWeekdays" className="atlas-field" defaultValue={4} min={2} max={30} required /></label>
        <label className="block">Consecutive days to confirm return<input type="number" name="consecutiveDays" className="atlas-field" defaultValue={3} min={1} max={30} required /></label>
        <label className="block">PortWatch daily JSON<input name="file" type="file" className="atlas-field" accept=".json,application/json" required /></label>
        <button className="ux-button" type="submit">{busy ? 'Reading activity…' : 'Review activity'}</button>
      </fieldset>
    </form>
    {error && <p role="alert">{error}</p>}
    {review && <section aria-label="Port activity results" className="space-y-2">
      <p role="status">{review.activity.portId === 'port2074' ? 'Jazan' : 'Jubail'} · {review.activity.days.length} study days · {review.activity.missingDays} without a record</p>
      {review.activity.recovery && <div>
        <p>Retrospective weekday reference from {review.activity.baselineFrom} through the day before {review.activity.from}.</p>
        {review.activity.recovery.firstBelowDate && <p>First day below range {review.activity.recovery.firstBelowDate}.</p>}
        <p role="status">{review.activity.recovery.returnConfirmedOn
          ? `Return confirmed ${review.activity.recovery.returnConfirmedOn}, ${review.activity.recovery.daysToConfirmation} days after the first below-range day.`
          : review.activity.recovery.status === 'unresolved' ? 'Sustained return remains unresolved at the end of the study.'
          : review.activity.recovery.status === 'insufficient-coverage' ? 'No study day has both a recorded count and enough weekday history.'
          : 'No comparable study day was below its weekday range.'}</p>
        <p>Return requires {review.activity.recovery.consecutiveDays} consecutive in-range days. Missing days interrupt the count.</p>
      </div>}
      {review.activity.baselineValidation && <details><summary>Held-out baseline comparison</summary>
        <p>{review.activity.baselineValidation.comparedDays} days scored by both baselines · {review.activity.baselineValidation.withinWeekdayRange} within the preceding weekday range.</p>
        <p>Mean absolute error in calls · rolling weekday {review.activity.baselineValidation.weekdayMeanAbsoluteError?.toFixed(2) ?? 'Unavailable'} · trailing median {review.activity.baselineValidation.trailingMeanAbsoluteError?.toFixed(2) ?? 'Unavailable'}</p>
      </details>}
      <div className="max-h-80 overflow-auto" tabIndex={0} aria-label="Daily activity and historical weekday baseline">
        <table className="w-full text-left">
          <thead><tr><th scope="col">Date</th><th scope="col">Calls</th><th scope="col">Baseline median</th><th scope="col">Comparison</th></tr></thead>
          <tbody>{review.activity.days.map(day => <tr key={day.date}>
            <th scope="row"><time dateTime={day.date}>{day.date}</time></th>
            <td>{day.portCalls ?? 'Unavailable'}</td>
            <td>{day.baselineMedian ?? 'Unavailable'}</td>
            <td>{comparisons[day.comparison]}{day.baselineRange && <small className="block">Range {day.baselineRange.join('–')} · Δ {day.departureFromMedian ?? 'unavailable'}</small>}</td>
          </tr>)}</tbody>
        </table>
      </div>
      <p>{review.attribution} · {review.productVersion}</p>
      <p className="break-all">Input SHA-256 <code>{review.inputSha256}</code></p>
    </section>}
  </details>
}
