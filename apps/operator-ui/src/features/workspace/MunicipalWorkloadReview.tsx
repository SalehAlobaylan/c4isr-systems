import {useState, type FormEvent} from 'react'
import {useAPI} from '@/lib/api'
import type {components} from '@/lib/openapi.generated'
import {useWorkspace} from '@/lib/workspaceContext'

type Release = components['schemas']['EnvironmentRelease']
type Review = Awaited<ReturnType<ReturnType<typeof useAPI>['reviewMunicipalWorkload']>>

export function MunicipalWorkloadReview({release}: {release: Release}) {
  const api = useAPI()
  const {signal} = useWorkspace()
  const [sourceId, setSourceId] = useState('')
  const [admissionId, setAdmissionId] = useState('')
  const [file, setFile] = useState<File>()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [review, setReview] = useState<Review>()

  async function submit(event: FormEvent) {
    event.preventDefault()
    if (!file || busy) return
    setReview(undefined)
    setError('')
    if (file.size > 3 * 1024 * 1024) {
      setError('Choose the original Q1 JSON file under 3 MiB.')
      return
    }
    setBusy(true)
    try {
      const result = await api.reviewMunicipalWorkload(release.id, {
        manifestSha256: release.manifestSha256, sourceId: sourceId.trim(), admissionId: admissionId.trim(),
      }, file, signal)
      if (!signal.aborted && result.releaseId === release.id && result.manifestSha256 === release.manifestSha256) {
        setReview(result)
      }
    } catch (error) {
      if (!signal.aborted) setError(error instanceof Error ? error.message : 'The workload could not be read.')
    } finally {
      if (!signal.aborted) setBusy(false)
    }
  }

  return <details>
    <summary>Riyadh 940 workload</summary>
    <p>Review the Q1 2026 municipal JSON as retrospective workload. The file is processed for this request.</p>
    <form onSubmit={event => void submit(event)}>
      <fieldset disabled={busy} className="space-y-2">
        <label className="block">Source ID<input className="atlas-field" value={sourceId} onChange={event => {setSourceId(event.target.value); setReview(undefined)}} maxLength={128} required /></label>
        <label className="block">Reviewed admission ID<input className="atlas-field" value={admissionId} onChange={event => {setAdmissionId(event.target.value); setReview(undefined)}} maxLength={128} required /></label>
        <label className="block">Municipal Q1 JSON<input className="atlas-field" type="file" accept=".json,application/json" onChange={event => {setFile(event.target.files?.[0]); setReview(undefined)}} required /></label>
        <button className="ux-button" type="submit" disabled={!file || !sourceId.trim() || !admissionId.trim()}>
          {busy ? 'Reading workload…' : 'Review workload'}
        </button>
      </fieldset>
    </form>
    {error && <p role="alert">{error}</p>}
    {review && <section aria-label="Municipal workload results" className="space-y-2">
      <p role="status"><strong>{review.workload.rowCount.toLocaleString()} reports</strong> · {review.workload.rowsWithoutPlaceLabel.toLocaleString()} without a usable place label</p>
      <p>Category and UTC receipt counts are from the published snapshot. Inspection outcomes are recorded independently.</p>
      <details>
        <summary>Report categories</summary>
        <ul>{Object.entries(review.workload.categoryCounts)
          .sort(([left, a], [right, b]) => b - a || left.localeCompare(right))
          .map(([category, count]) => <li key={category}><span lang="ar" dir="auto">{category}</span> · {count.toLocaleString()}</li>)}</ul>
      </details>
      <details>
        <summary>Daily receipt counts</summary>
        <ul>{Object.entries(review.workload.utcDayCounts).sort(([a], [b]) => a.localeCompare(b))
          .map(([date, count]) => <li key={date}><time dateTime={date}>{date}</time> · {count.toLocaleString()}</li>)}</ul>
      </details>
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
