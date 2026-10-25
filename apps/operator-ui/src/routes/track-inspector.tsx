import { useMutation, useQuery } from '@/lib/scopedQueries'
import { useQueryClient } from '@tanstack/react-query'
import { Link, useParams } from '@tanstack/react-router'
import { useMemo, useState } from 'react'

import { Page } from '@/components/layout/Page'
import {
  AlertStateBadge,
  ActorBadge,
  FreshnessBadge,
  KindBadge,
  SeverityBadge,
  StateBadge,
  TrackStatusBadge,
} from '@/components/shared/badges'
import { EmptyState, ErrorState, SectionTitle } from '@/components/shared/states'
import { useAPI } from '@/lib/api'
import {
  formatConfidence,
  formatCoord,
  formatDateTime,
  formatHeading,
  formatRelative,
  formatSpeed,
  formatTimestamp,
  truncateId,
} from '@/lib/format'
import type { SemanticTone } from '@/lib/operationalPresentation'
import { useQueryKeys } from '@/lib/queryKeys'
import { toast } from '@/stores/toasts'

const HISTORY_LIMIT = 60
const OBSERVATION_LIMIT = 200
const LEDGER_LIMIT = 60

/**
 * Classification confidence is not a quality score and must not borrow a status
 * colour (DESIGN.md §7.4). The only judgement on offer is whether an operator
 * still has to make the call, so a *low* confidence is a review item, not a
 * problem — which is exactly what the `review` tone means.
 */
function confidenceTone(confidence: number | null | undefined): SemanticTone {
  if (confidence === null || confidence === undefined) return 'neutral'
  return confidence < 0.5 ? 'review' : 'info'
}

/**
 * A stage of the evidence spine.
 *
 * The spine is the thing this page exists to show: that a track is an
 * *interpretation* built from observations, and that every step from raw source
 * to recorded action can be walked back to. Each stage therefore reports a count
 * the API actually returned rather than a reassuring headline.
 */
interface ChainStage {
  index: string
  label: string
  value: string
  note: string
  /** Whether the link into the next stage is fully accounted for. */
  complete: boolean | null
}

function ChainBand({ stages }: { stages: ChainStage[] }) {
  return (
    <div className="aegis-chain" role="list" aria-label="Evidence chain, source to recorded action">
      {stages.map((stage) => (
        <div
          key={stage.index}
          role="listitem"
          className="aegis-chain-cell"
          data-complete={stage.complete === null ? 'na' : stage.complete ? 'yes' : 'partial'}
        >
          <p className="k">
            <span className="aegis-chain-idx mono">{stage.index}</span>
            {stage.label}
          </p>
          <p className="v mono">{stage.value}</p>
          <p className="n mono" title={stage.note}>
            {stage.note}
          </p>
        </div>
      ))}
    </div>
  )
}

function ConfidenceMeter({ value }: { value: number | null | undefined }) {
  if (value === null || value === undefined) {
    return <span className="mono text-[12px] text-ink-muted">Not stated</span>
  }
  return (
    <span className="flex items-center gap-2">
      <span className="mono text-[12px] font-semibold text-ink">{formatConfidence(value)}</span>
      <span
        className="aegis-confbar"
        role="img"
        aria-label={`Confidence ${formatConfidence(value)}`}
      >
        <span style={{ width: `${Math.round(Math.min(1, Math.max(0, value)) * 100)}%` }} />
      </span>
    </span>
  )
}

function Row({
  label,
  children,
  stack = false,
}: {
  label: string
  children: React.ReactNode
  /** Long machine values (a rule's JSON, a raw provenance reference) cannot be
   *  right-aligned against their label: they wrap into a ragged column that
   *  reads as corruption. They take the full width beneath the label instead. */
  stack?: boolean
}) {
  return (
    <div className={stack ? 'aegis-xrow aegis-xrow--stack' : 'aegis-xrow'}>
      <span className="k cap">{label}</span>
      <span className="v">{children}</span>
    </div>
  )
}

/**
 * Record an operator's own classification of a track.
 *
 * The system states what it believes and why. This is the other half: a human
 * who disagrees can say so, in the same ledger, attributed to them rather than
 * to the correlation engine. It is a record, not an override — the engine's
 * hypothesis is still standing, and both are listed together, because
 * silently replacing one with the other would destroy the provenance this page
 * exists to show.
 */
function ClassificationOverride({ trackId }: { trackId: string }) {
  const api = useAPI()
  const queryKeys = useQueryKeys()
  const queryClient = useQueryClient()
  const [label, setLabel] = useState('')
  const [confidence, setConfidence] = useState('')
  const [method, setMethod] = useState('OPERATOR')
  const [sourceReference, setSourceReference] = useState('')

  const mutation = useMutation({
    mutationFn: () =>
      api.createClassification({
        trackId,
        label: label.trim(),
        confidence: confidence.trim() === '' ? undefined : Number(confidence),
        method: method as 'OPERATOR' | 'RULE' | 'ALGORITHM',
        sourceReference: sourceReference.trim() || undefined,
      }),
    onSuccess: (created) => {
      toast({
        title: 'Classification recorded',
        description: `${created.label} · attributed to operator-01`,
        variant: 'success',
      })
      setLabel('')
      setConfidence('')
      setSourceReference('')
      void queryClient.invalidateQueries({ queryKey: queryKeys.tracks.classifications(trackId) })
    },
    onError: (error) => {
      toast({
        title: 'Classification not recorded',
        description: error instanceof Error ? error.message : undefined,
        variant: 'error',
      })
    },
  })

  const invalid =
    label.trim() === '' ||
    (confidence.trim() !== '' && (Number(confidence) < 0 || Number(confidence) > 1))

  return (
    <form
      className="aegis-xform"
      onSubmit={(event) => {
        event.preventDefault()
        if (invalid) return
        mutation.mutate()
      }}
    >
      <div className="aegis-xform-row">
        <label className="aegis-xfield">
          <span className="k">Your reading</span>
          <input
            value={label}
            onChange={(event) => setLabel(event.target.value)}
            placeholder="What this track appears to be"
            maxLength={120}
            required
          />
        </label>
        <label className="aegis-xfield">
          <span className="k">Confidence</span>
          <input
            value={confidence}
            onChange={(event) => setConfidence(event.target.value)}
            placeholder="0.91"
            inputMode="decimal"
            aria-describedby="override-confidence-help"
          />
        </label>
      </div>
      <p id="override-confidence-help" className="aegis-xnote">
        A number between 0 and 1, or nothing at all. Leaving it blank records
        that you were not willing to put a figure on it — which is a different
        statement from low confidence, and the one the page prints.
      </p>
      <div className="aegis-xform-row">
        <label className="aegis-xfield">
          <span className="k">Method</span>
          <select value={method} onChange={(event) => setMethod(event.target.value)}>
            <option value="OPERATOR">Operator judgement</option>
            <option value="RULE">Rule</option>
            <option value="ALGORITHM">Algorithm</option>
          </select>
        </label>
        <label className="aegis-xfield">
          <span className="k">Source</span>
          <input
            value={sourceReference}
            onChange={(event) => setSourceReference(event.target.value)}
            placeholder="Report, log or reference this came from"
            maxLength={160}
          />
        </label>
      </div>
      <div className="aegis-xform-actions">
        <button type="submit" className="aegis-dsc" disabled={invalid || mutation.isPending}>
          {mutation.isPending ? 'Recording…' : 'Record classification'}
        </button>
      </div>
    </form>
  )
}

export function TrackInspectorPage() {
  const api = useAPI()
  const queryKeys = useQueryKeys()
  const { id } = useParams({ from: '/tracks/$id' })

  const trackQuery = useQuery({
    queryKey: queryKeys.tracks.detail(id),
    queryFn: () => api.getTrack(id),
  })

  const historyQuery = useQuery({
    queryKey: queryKeys.tracks.history(id),
    queryFn: () => api.getTrackHistory(id, { limit: HISTORY_LIMIT }),
  })

  const classificationQuery = useQuery({
    queryKey: queryKeys.tracks.classifications(id),
    queryFn: () => api.listTrackClassifications(id, { limit: 100 }),
  })

  // The observations that fed this track. This is the provenance spine itself:
  // a track's real claim to authority is the evidence behind it, not the badge
  // on its identity.
  const observationQuery = useQuery({
    queryKey: queryKeys.observations.list({ track_id: id, limit: OBSERVATION_LIMIT }),
    queryFn: () => api.listObservations({ track_id: id, limit: OBSERVATION_LIMIT }),
  })

  const assessmentQuery = useQuery({
    queryKey: queryKeys.assessments.list({ subject_type: 'track', subject_id: id, limit: 50 }),
    queryFn: () => api.listAssessments({ subject_type: 'track', subject_id: id, limit: 50 }),
  })

  const alertQuery = useQuery({
    queryKey: queryKeys.alerts.list({ track_id: id, limit: 50 }),
    queryFn: () => api.listAlerts({ track_id: id, limit: 50 }),
  })

  const auditQuery = useQuery({
    queryKey: queryKeys.audit.list({ subject_type: 'track', subject_id: id, limit: LEDGER_LIMIT }),
    queryFn: () => api.listAudit({ subject_type: 'track', subject_id: id, limit: LEDGER_LIMIT }),
  })

  const track = trackQuery.data

  /* ── Provenance aggregation ───────────────────────────────────────────
     Grouped once and reused by the chain band, the integrity panel and the
     evidence table, so the counts a reader sees in the summary are the same
     numbers the table below is showing. */
  const provenance = useMemo(() => {
    const observations = observationQuery.data?.items ?? []
    const classifications = classificationQuery.data?.items ?? []
    const assessments = assessmentQuery.data?.items ?? []
    const alerts = alertQuery.data?.items ?? []
    const ledger = auditQuery.data?.items ?? []

    const bySource = new Map<string, { count: number; oldest: string | null; newest: string | null }>()
    let unsourced = 0
    for (const observation of observations) {
      const key = observation.sourceId?.trim()
      if (!key) {
        unsourced += 1
        continue
      }
      const entry = bySource.get(key) ?? { count: 0, oldest: null, newest: null }
      entry.count += 1
      if (!entry.oldest || observation.observedAt < entry.oldest) entry.oldest = observation.observedAt
      if (!entry.newest || observation.observedAt > entry.newest) entry.newest = observation.observedAt
      bySource.set(key, entry)
    }

    const sourced = [...bySource.entries()].sort((a, b) => b[1].count - a[1].count)

    const topConfidence = classifications.reduce<number | null>((best, item) => {
      if (item.confidence === null || item.confidence === undefined) return best
      return best === null || item.confidence > best ? item.confidence : best
    }, null)

    // A classification that names no source of its own is an interpretation with
    // nothing to walk back to. Saying so is more useful than quietly omitting it.
    const unsupportedClassifications = classifications.filter(
      (item) => !item.sourceReference?.trim(),
    ).length
    const evidencelessAssessments = assessments.filter((item) => item.evidence.length === 0).length

    return {
      observations,
      classifications,
      assessments,
      alerts,
      ledger,
      sourced,
      unsourced,
      topConfidence,
      unsupportedClassifications,
      evidencelessAssessments,
    }
  }, [
    observationQuery.data,
    classificationQuery.data,
    assessmentQuery.data,
    alertQuery.data,
    auditQuery.data,
  ])

  const stages = useMemo<ChainStage[]>(() => {
    const subject = track
    if (!subject) return []
    const activeAlerts = provenance.alerts.filter((alert) => alert.state !== 'RESOLVED').length
    const history = historyQuery.data?.items ?? []
    return [
      {
        index: '01',
        label: 'Sources',
        value: provenance.sourced.length ? String(provenance.sourced.length) : '—',
        note: provenance.unsourced
          ? `${provenance.unsourced} unsourced`
          : provenance.sourced.length
            ? 'all sourced'
            : 'none yet',
        complete: provenance.unsourced === 0 ? provenance.sourced.length > 0 : false,
      },
      {
        index: '02',
        label: 'Observations',
        value: subject.observationCount > 0 ? String(subject.observationCount) : '—',
        note:
          provenance.sourced.length > 0
            ? provenance.sourced
                .slice(0, 2)
                .map(([key, entry]) => `${truncateId(key, 8)}×${entry.count}`)
                .join('  ')
            : 'nothing received',
        complete: provenance.unsourced === 0 && subject.observationCount > 0,
      },
      {
        index: '03',
        label: 'Track history',
        value: history.length ? String(history.length) : '—',
        note: history.length
          ? `latest ${formatRelative(subject.lastSeenAt)}`
          : 'no position samples',
        complete: null,
      },
      {
        index: '04',
        label: 'Classifications',
        value: provenance.classifications.length ? String(provenance.classifications.length) : '—',
        note: provenance.topConfidence === null
          ? 'no hypothesis stated'
          : `top ${formatConfidence(provenance.topConfidence)}`,
        complete:
          provenance.classifications.length === 0
            ? null
            : provenance.unsupportedClassifications === 0,
      },
      {
        index: '05',
        label: 'Assessments',
        value: provenance.assessments.length ? String(provenance.assessments.length) : '—',
        note: provenance.assessments.length
          ? `${provenance.evidencelessAssessments} without evidence`
          : 'operator judgement outstanding',
        complete:
          provenance.assessments.length === 0
            ? null
            : provenance.evidencelessAssessments === 0,
      },
      {
        index: '06',
        label: 'Recorded action',
        value: activeAlerts > 0 ? `${activeAlerts} alert${activeAlerts === 1 ? '' : 's'}` : '—',
        note: `${provenance.ledger.length} ledger entr${provenance.ledger.length === 1 ? 'y' : 'ies'}`,
        complete: null,
      },
    ]
  }, [track, provenance, historyQuery.data])

  // The classification the platform currently stands behind. Picked by stated
  // confidence only; a classification that asserts no confidence never displaces
  // one that does, because an unquantified claim is not a weaker claim, it is an
  // absent one (DESIGN.md §7.4).
  //
  // Declared above the early returns below: a hook that only runs on the loaded
  // path is a hook-order violation, and React reports it as "rendered more hooks
  // than during the previous render" the moment the load completes.
  const topClassification = useMemo(() => {
    let best: { label: string; confidence: number; method: string } | null = null
    for (const item of provenance.classifications) {
      const { confidence } = item
      if (confidence === null || confidence === undefined) continue
      if (best === null || confidence > best.confidence) {
        best = { label: item.label, confidence, method: item.method }
      }
    }
    return best
  }, [provenance.classifications])

  if (trackQuery.isError) {
    return (
      <Page title="Track inspector" description="Evidence and provenance for one track">
        <div className="p-4">
          <ErrorState error={trackQuery.error} onRetry={() => void trackQuery.refetch()} />
        </div>
      </Page>
    )
  }

  if (trackQuery.isLoading || !track) {
    return (
      <Page title="Track inspector" description="Evidence and provenance for one track">
        <div className="p-4">
          <EmptyState title="Loading track…" description={`Fetching ${id}.`} />
        </div>
      </Page>
    )
  }

  return (
    <Page
      title={track.externalRef || track.id}
      description="Evidence, provenance and recorded action for one track"
      actions={
        <div className="flex flex-wrap items-center gap-2">
          <TrackStatusBadge value={track.status} lastSeenAt={track.lastSeenAt} />
          <FreshnessBadge freshness={track.freshness} lastSeenAt={track.lastSeenAt} />
          <Link to="/tracks" className="aegis-btn">
            All tracks
          </Link>
        </div>
      }
    >
      {stages.length > 0 ? <ChainBand stages={stages} /> : null}

      <div className="aegis-xgrid12">
        {/* Row one: 5 / 7. The evidence column is narrow because it is a list of
            sources; the interpretation column is wide because a classification
            has a method, a confidence and a provenance reference to state. */}
        <div className="aegis-span5">
          <section className="aegis-panel" aria-labelledby="x-identity">
            <h2 id="x-identity">Identity and kinematics</h2>
            <p className="sub">Reported by the correlation engine</p>
            <dl className="aegis-kv">
              <dt>Track</dt>
              <dd>{track.id}</dd>
              <dt>External ref</dt>
              <dd>{track.externalRef || '—'}</dd>
              <dt>Status</dt>
              <dd>
                <TrackStatusBadge value={track.status} lastSeenAt={track.lastSeenAt} />
              </dd>
              <dt>Freshness</dt>
              <dd>
                <FreshnessBadge freshness={track.freshness} lastSeenAt={track.lastSeenAt} />
              </dd>
              <dt>Position</dt>
              <dd>{formatCoord(track.position)}</dd>
              <dt>Speed</dt>
              <dd>{formatSpeed(track.speed)}</dd>
              <dt>Heading</dt>
              <dd>{formatHeading(track.heading)}</dd>
              <dt>First seen</dt>
              <dd title={formatDateTime(track.firstSeenAt)}>{formatDateTime(track.firstSeenAt)}</dd>
              <dt>Last seen</dt>
              <dd title={formatDateTime(track.lastSeenAt)}>{formatDateTime(track.lastSeenAt)}</dd>
              <dt>Closed</dt>
              <dd>{track.closedAt ? formatDateTime(track.closedAt) : '—'}</dd>
            </dl>
          </section>

          <section className="aegis-panel" aria-labelledby="x-evidence">
            <h2 id="x-evidence">Evidence by source</h2>
            <p className="sub">
              Observations attributed to this track, grouped by the source that reported them
            </p>
            {observationQuery.isError ? (
              <ErrorState
                error={observationQuery.error}
                onRetry={() => void observationQuery.refetch()}
              />
            ) : observationQuery.isLoading ? (
              <EmptyState title="Loading evidence…" />
            ) : provenance.sourced.length === 0 ? (
              <EmptyState
                title="No attributed observations"
                description={
                  provenance.unsourced > 0
                    ? `${provenance.unsourced} observation${provenance.unsourced === 1 ? '' : 's'} reference this track but name no source, so they cannot be attributed.`
                    : 'The correlation engine has not attributed any raw observation to this track yet.'
                }
              />
            ) : (
              <div className="aegis-xlist">
                {provenance.sourced.map(([sourceId, entry]) => (
                  <div key={sourceId} className="aegis-xitem">
                    <span className="aegis-src">{truncateId(sourceId, 22)}</span>
                    <span className="mono text-[12px] text-ink">
                      {entry.count} observation{entry.count === 1 ? '' : 's'}
                    </span>
                    <span className="spacer mono text-[12px] text-ink-muted">
                      {entry.newest ? formatRelative(entry.newest) : '—'}
                    </span>
                  </div>
                ))}
                {provenance.unsourced > 0 ? (
                  <div className="aegis-xitem" data-flag="partial">
                    <span className="aegis-src">UNATTRIBUTED</span>
                    <span className="mono text-[12px] text-ink">
                      {provenance.unsourced} observation{provenance.unsourced === 1 ? '' : 's'}
                    </span>
                    <span className="spacer mono text-[12px] text-ink-muted">no source id</span>
                  </div>
                ) : null}
              </div>
            )}
          </section>
        </div>

        <div className="aegis-span7">
          <section className="aegis-panel" aria-labelledby="x-classifications">
            <h2 id="x-classifications">Classification hypotheses</h2>
            <p className="sub">
              Interpretations, not facts — each one keeps its method and its own provenance
            </p>
            {classificationQuery.isError ? (
              <ErrorState
                error={classificationQuery.error}
                onRetry={() => void classificationQuery.refetch()}
              />
            ) : classificationQuery.isLoading ? (
              <EmptyState title="Loading classifications…" />
            ) : provenance.classifications.length === 0 ? (
              <EmptyState
                title="No classification stated"
                description="Nothing has been concluded about what this track is. An unclassified track is not a hostile one."
              />
            ) : (
              <ul className="aegis-xcards">
                {provenance.classifications.map((item) => (
                  <li key={item.id} className="aegis-xcard">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="aegis-xcard-title">{item.label}</span>
                      <StateBadge
                        value={confidenceTone(item.confidence)}
                        tone={confidenceTone(item.confidence)}
                        detail={item.confidence !== null && item.confidence < 0.5 ? 'needs review' : null}
                      />
                      <span className="spacer mono text-[12px] text-ink-muted">
                        {formatRelative(item.createdAt)}
                      </span>
                    </div>
                    <div className="aegis-xcard-meta">
                      <Row label="Confidence">
                        <ConfidenceMeter value={item.confidence} />
                      </Row>
                      <Row label="Method">
                        <KindBadge value={item.method} />
                      </Row>
                      <Row label="Attributed">{item.createdBy || '—'}</Row>
                      <Row label="Stated at">{formatDateTime(item.createdAt)}</Row>
                      <Row label="Provenance" stack>
                        {item.sourceReference?.trim() ? (
                          <span className="mono text-[12px] break-all">{item.sourceReference}</span>
                        ) : (
                          <span className="text-[12px] text-ink-muted">
                            No source reference — this hypothesis cannot be traced to evidence
                          </span>
                        )}
                      </Row>
                    </div>
                  </li>
                ))}
              </ul>
            )}
          </section>

          <section className="aegis-panel" aria-labelledby="x-assessments">
            <h2 id="x-assessments">Assessments</h2>
            <p className="sub">Conclusions recorded against this track, with their evidence</p>
            {assessmentQuery.isError ? (
              <ErrorState
                error={assessmentQuery.error}
                onRetry={() => void assessmentQuery.refetch()}
              />
            ) : assessmentQuery.isLoading ? (
              <EmptyState title="Loading assessments…" />
            ) : provenance.assessments.length === 0 ? (
              <EmptyState
                title="No assessment recorded"
                description="The platform holds no conclusion for this track. An operator still has to make that judgement."
              />
            ) : (
              <ul className="aegis-xcards">
                {provenance.assessments.map((item) => (
                  <li key={item.id} className="aegis-xcard">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="aegis-xcard-title">{item.type}</span>
                      <KindBadge value={item.method} />
                      <span className="spacer mono text-[12px] text-ink-muted">
                        {formatRelative(item.createdAt)}
                      </span>
                    </div>
                    <p className="aegis-xcard-body">{item.conclusion}</p>
                    <div className="aegis-xcard-meta">
                      <Row label="Confidence">
                        <ConfidenceMeter value={item.confidence} />
                      </Row>
                      <Row label="Recorded by">{item.createdBy || '—'}</Row>
                      <Row label="Evidence" stack>
                        {item.evidence.length > 0 ? (
                          <span className="mono text-[12px]">
                            {item.evidence.length} reference{item.evidence.length === 1 ? '' : 's'}:{' '}
                            {item.evidence
                              .map((evidence) => `${evidence.type}/${truncateId(evidence.id, 10)}`)
                              .join(', ')}
                          </span>
                        ) : (
                          <span className="text-[12px] text-ink-muted">
                            None — this conclusion rests on no recorded evidence
                          </span>
                        )}
                      </Row>
                    </div>
                  </li>
                ))}
              </ul>
            )}
          </section>

          <section className="aegis-panel" aria-labelledby="x-override">
            <h2 id="x-override">Record your reading</h2>
            <p className="sub">Added to the hypotheses above, attributed to you</p>
            <ClassificationOverride trackId={id} />
          </section>
        </div>

        {/* Row two: 4 / 8. The integrity column is narrow because it is four short
            verdicts; the ledger is wide because it is a table of recorded
            actions. */}
        <div className="aegis-span4">
          <section className="aegis-panel" aria-labelledby="x-integrity">
            <h2 id="x-integrity">Provenance integrity</h2>
            <p className="sub">What the chain above can and cannot account for</p>
            <div className="aegis-xchecks">
              <div className="aegis-xcheck" data-ok={track.observationCount > 0}>
                <span className="cap">Observations attributed</span>
                <span className="mono text-[12px] text-ink">
                  {track.observationCount} of {track.observationCount + provenance.unsourced} resolvable
                </span>
              </div>
              <div className="aegis-xcheck" data-ok={provenance.unsourced === 0}>
                <span className="cap">Every observation names a source</span>
                <span className="mono text-[12px] text-ink">
                  {provenance.unsourced === 0 ? 'Yes' : `${provenance.unsourced} do not`}
                </span>
              </div>
              <div className="aegis-xcheck" data-ok={provenance.unsupportedClassifications === 0}>
                <span className="cap">Classifications traceable to a reference</span>
                <span className="mono text-[12px] text-ink">
                  {provenance.classifications.length === 0
                    ? 'No classifications'
                    : provenance.unsupportedClassifications === 0
                      ? 'All of them'
                      : `${provenance.unsupportedClassifications} of ${provenance.classifications.length} do not`}
                </span>
              </div>
              <div className="aegis-xcheck" data-ok={provenance.evidencelessAssessments === 0}>
                <span className="cap">Assessments cite evidence</span>
                <span className="mono text-[12px] text-ink">
                  {provenance.assessments.length === 0
                    ? 'No assessments'
                    : provenance.evidencelessAssessments === 0
                      ? 'All of them'
                      : `${provenance.evidencelessAssessments} of ${provenance.assessments.length} do not`}
                </span>
              </div>
            </div>
            <p className="aegis-xnote">
              These checks describe only what the API returned for this track. They are not an
              assertion that the observation itself was correct.
            </p>
          </section>

          <section className="aegis-panel" aria-labelledby="x-alerts">
            <h2 id="x-alerts">Alerts raised</h2>
            <p className="sub">Severity and lifecycle are separate facts and are shown separately</p>
            {alertQuery.isError ? (
              <ErrorState error={alertQuery.error} onRetry={() => void alertQuery.refetch()} />
            ) : alertQuery.isLoading ? (
              <EmptyState title="Loading alerts…" />
            ) : provenance.alerts.length === 0 ? (
              <EmptyState title="No alerts reference this track" />
            ) : (
              <ul className="aegis-xcards">
                {provenance.alerts.map((alert) => (
                  <li key={alert.id} className="aegis-xcard">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="aegis-xcard-title">{alert.title || alert.type}</span>
                      <SeverityBadge value={alert.severity} />
                      <AlertStateBadge
                        value={alert.state}
                        acknowledgedBy={alert.acknowledgedBy}
                        acknowledgedAt={alert.acknowledgedAt}
                      />
                      <span className="spacer mono text-[12px] text-ink-muted">
                        {formatRelative(alert.createdAt)}
                      </span>
                    </div>
                    {alert.message ? <p className="aegis-xcard-body">{alert.message}</p> : null}
                    <div className="aegis-xcard-meta">
                      <Row label="Type">
                        <KindBadge value={alert.type} />
                      </Row>
                      <Row label="Raised at">{formatDateTime(alert.createdAt)}</Row>
                      <Row label="Incident">
                        {alert.incidentId ? (
                          <Link
                            to="/incidents/$id"
                            params={{ id: alert.incidentId }}
                            className="mono text-[12px] text-[color:var(--emad-link)]"
                          >
                            {truncateId(alert.incidentId, 22)}
                          </Link>
                        ) : (
                          '—'
                        )}
                      </Row>
                      <Row label="Rule provenance" stack>
                        {Object.keys(alert.sourceReference ?? {}).length > 0 ? (
                          <span className="mono text-[12px] break-all">
                            {JSON.stringify(alert.sourceReference)}
                          </span>
                        ) : (
                          <span className="text-[12px] text-ink-muted">Not recorded</span>
                        )}
                      </Row>
                    </div>
                  </li>
                ))}
              </ul>
)}
             </section>
           </div>

           <div className="aegis-span8">
             <section className="aegis-panel" aria-labelledby="x-ledger">
            <h2 id="x-ledger">Audit ledger</h2>
            <p className="sub">Every recorded action against this track</p>
            <SectionTitle
              action={
                <Link
                  to="/timeline"
                  search={{ subject_type: 'track' }}
                  className="mono text-[12px] text-[color:var(--emad-link)]"
                >
                  Full audit timeline
                </Link>
              }
            >
              {provenance.ledger.length} entr{provenance.ledger.length === 1 ? 'y' : 'ies'}
            </SectionTitle>
            {auditQuery.isError ? (
              <ErrorState error={auditQuery.error} onRetry={() => void auditQuery.refetch()} />
            ) : auditQuery.isLoading ? (
              <EmptyState title="Loading ledger…" />
            ) : provenance.ledger.length === 0 ? (
              <EmptyState
                title="Nothing recorded against this track"
                description="No actor — operator, scenario or system — has acted on it."
              />
            ) : (
              <div className="aegis-xlist">
                {provenance.ledger.map((entry) => (
                  <div key={entry.id} className="aegis-xitem aegis-xitem-ledger">
                    <span className="mono text-[12px] text-ink-muted" title={formatDateTime(entry.occurredAt)}>
                      {formatTimestamp(entry.occurredAt)}
                    </span>
                    <ActorBadge value={entry.actorType} />
                    <span className="mono text-[12px] text-ink">{entry.action}</span>
                    <span className="spacer mono text-[12px] text-ink-muted">
                      {truncateId(entry.actorId, 16)}
                    </span>
                  </div>
                ))}
              </div>
            )}
          </section>

          <section className="aegis-panel" aria-labelledby="x-history">
            <h2 id="x-history">Position history</h2>
            <p className="sub">Reported positions, newest first</p>
            {historyQuery.isError ? (
              <ErrorState error={historyQuery.error} onRetry={() => void historyQuery.refetch()} />
            ) : historyQuery.isLoading ? (
              <EmptyState title="Loading history…" />
            ) : (historyQuery.data?.items ?? []).length === 0 ? (
              <EmptyState title="No position samples" description="This track has no recorded fixes." />
            ) : (
              <table className="aegis-xtable">
                <caption className="sr-only">Reported positions for this track, newest first</caption>
                <thead>
                  <tr>
                    <th scope="col">Observed</th>
                    <th scope="col">Position</th>
                    <th scope="col">Speed</th>
                    <th scope="col">Heading</th>
                  </tr>
                </thead>
                <tbody>
                  {(historyQuery.data?.items ?? []).map((point) => (
                    <tr key={point.id}>
                      <td className="mono" title={formatDateTime(point.observedAt)}>
                        {formatTimestamp(point.observedAt)}
                      </td>
                      <td className="mono">{formatCoord(point.position)}</td>
                      <td className="mono">{formatSpeed(point.speed)}</td>
                      <td className="mono">{formatHeading(point.heading)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </section>

          {topClassification ? (
            <section className="aegis-panel" aria-labelledby="x-summary">
              <h2 id="x-summary">Standing conclusion</h2>
              <p className="sub">Highest-confidence classification held for this track</p>
              <div className="aegis-xsummary">
                <span className="aegis-xcard-title">{topClassification.label}</span>
                <ConfidenceMeter value={topClassification.confidence} />
                <KindBadge value={topClassification.method} />
              </div>
              <p className="aegis-xnote">
                An interpretation with provenance, not an established fact. Corroborating and
                contradicting observations remain listed above rather than being filtered out.
              </p>
            </section>
          ) : null}
        </div>
      </div>
    </Page>
  )
}