import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'

import { ErrorState } from '@/components/shared/states'
import { FreshnessBadge } from '@/components/shared/badges'
import { AssessmentFormDialog } from '@/features/incidents/AssessmentFormDialog'
import { api, type Track } from '@/lib/api'
import { freshnessWarning, isActionable } from '@/lib/freshness'
import {
  formatConfidence,
  formatCoord,
  formatHeading,
  formatRelative,
  formatSpeed,
  formatTimestamp,
} from '@/lib/format'
import { queryKeys } from '@/lib/queryKeys'
import { selectReplayAt, useScrubStore } from '@/stores/scrub'
import { useUiStore } from '@/stores/ui'

export function TrackDetailPanel({ trackId }: { trackId: string }) {
  const showHistory = useUiStore((state) => state.showTrackHistory)
  const toggleHistory = useUiStore((state) => state.toggleTrackHistory)
  const [assessmentOpen, setAssessmentOpen] = useState(false)

  // While the operator is scrubbed back in time, the panel must describe that
  // moment. Fetching the current track here would put a present-day position
  // next to a replayed map marker.
  const replayAt = useScrubStore(selectReplayAt)
  const replayQuery = useQuery({
    queryKey: ['picture', replayAt ?? 'live'],
    queryFn: () =>
      replayAt ? api.getPicture({ at: replayAt }) : api.getPicture({ live: true }),
    enabled: Boolean(replayAt),
  })
  const replayedTrack = replayAt ? replayQuery.data?.tracks.find((t) => t.id === trackId) : undefined

  const trackQuery = useQuery({
    queryKey: queryKeys.tracks.detail(trackId),
    queryFn: () => api.getTrack(trackId),
    enabled: Boolean(trackId) && !replayAt,
    refetchInterval: 15_000,
  })
  // Evidence and judgement panels are suppressed during replay. They describe
  // present-day records: an observation logged after the replayed instant would
  // not have existed yet, and an assessment written later could not have
  // informed a decision the operator is reviewing as though it was made then.
  const evidenceEnabled = Boolean(trackId) && !replayAt
  const classificationsQuery = useQuery({
    queryKey: queryKeys.tracks.classifications(trackId),
    queryFn: () => api.listTrackClassifications(trackId, { limit: 20 }),
    enabled: evidenceEnabled,
  })
  const observationsQuery = useQuery({
    queryKey: queryKeys.observations.list({ track_id: trackId, limit: 20 }),
    queryFn: () => api.listObservations({ track_id: trackId, limit: 20 }),
    enabled: evidenceEnabled,
  })
  const alertsQuery = useQuery({
    queryKey: queryKeys.alerts.list({ track_id: trackId, limit: 10 }),
    queryFn: () => api.listAlerts({ track_id: trackId, limit: 10 }),
    enabled: evidenceEnabled,
  })
  const assessmentsQuery = useQuery({
    queryKey: queryKeys.assessments.list({ subject_type: 'track', subject_id: trackId }),
    queryFn: () => api.listAssessments({ subject_type: 'track', subject_id: trackId, limit: 10 }),
    enabled: evidenceEnabled,
  })

  // A replayed track is projected onto the same shape the panel already renders.
  // Observation counts are absent from a reconstruction because counting
  // present-day evidence beside a past position would be misleading.
  const replayed: Track | undefined = replayedTrack
    ? {
        id: replayedTrack.id,
        externalRef: replayedTrack.externalRef,
        status: replayedTrack.status,
        position: replayedTrack.position ?? null,
        speed: replayedTrack.speed ?? null,
        heading: replayedTrack.heading ?? null,
        firstSeenAt: replayedTrack.firstSeenAt,
        lastSeenAt: replayedTrack.lastSeenAt,
        createdAt: replayedTrack.firstSeenAt,
        updatedAt: replayedTrack.observedAt,
        closedAt: null,
        observationCount: 0,
        metadata: {},
        freshness: replayedTrack.freshness,
        freshnessLabel: replayedTrack.freshnessLabel,
      } as Track
    : undefined
  const track = replayed ?? trackQuery.data

  if (trackQuery.isError && !replayed) {
    return (
      <div className="p-4">
        <ErrorState error={trackQuery.error} onRetry={() => void trackQuery.refetch()} />
      </div>
    )
  }

  if (!track) {
    return <PanelSkeleton />
  }

  return (
    <>
      <section className="aegis-panel">
        <h2>Track state</h2>
        <p className="sub">Fused from observations · provenance preserved</p>
        {!isActionable(track.freshness) ? (
          <p className="mt-2 border-l-2 border-[color:var(--st-warn)] bg-[color:var(--st-warn)]/10 px-2 py-1.5 text-[11px] leading-snug text-[color:var(--st-warn)]">
            {freshnessWarning(track.freshness, track.lastSeenAt)}
          </p>
        ) : null}
        <dl className="aegis-kv">
          <dt>Status</dt>
          <dd className="word">{track.status}</dd>
          <dt>Freshness</dt>
          <dd>
            <FreshnessBadge freshness={track.freshness} lastSeenAt={track.lastSeenAt} />
          </dd>
          <dt>External ref</dt>
          <dd title={track.externalRef || undefined}>
            {(track.externalRef ?? '').split('__').filter(Boolean).pop() || '—'}
          </dd>
          <dt>Observations</dt>
          <dd>{track.observationCount}</dd>
          <dt>Position</dt>
          <dd>
            {formatCoord(track.position)}
            {!isActionable(track.freshness) ? (
              <span className="ml-1 text-[10px] uppercase text-[color:var(--faint)]">
                last known
              </span>
            ) : null}
          </dd>
          <dt>Speed</dt>
          <dd>{formatSpeed(track.speed)}</dd>
          <dt>Heading</dt>
          <dd>{formatHeading(track.heading)}</dd>
          <dt>First seen</dt>
          <dd>{formatTimestamp(track.firstSeenAt)}</dd>
          <dt>Last seen</dt>
          <dd>{formatTimestamp(track.lastSeenAt)}</dd>
        </dl>
        <button
          type="button"
          className="aegis-btn mt-4"
          aria-pressed={showHistory}
          onClick={toggleHistory}
        >
          {showHistory ? 'Hide history polyline' : 'Show history polyline'}
        </button>
      </section>

      <section className="aegis-panel">
        <h2>Classification confidence</h2>
        <p className="sub">Interpretation stays separate from evidence</p>
        {classificationsQuery.isLoading ? (
          <PanelSkeleton rows={2} />
        ) : (classificationsQuery.data?.items?.length ?? 0) === 0 ? (
          <p className="aegis-empty">No classifications recorded.</p>
        ) : (
          <ul className="m-0 flex list-none flex-col gap-3 p-0">
            {classificationsQuery.data?.items.map((classification) => (
              <li key={classification.id}>
                <div className="flex items-center gap-3">
                  <span className="cap" style={{ minWidth: 132 }}>
                    {classification.label}
                  </span>
                  <div className="aegis-confbar" style={{ flex: 1 }}>
                    <span style={{ width: `${Math.round((classification.confidence ?? 0) * 100)}%` }} />
                  </div>
                  <span className="mono text-[10px] faint">{formatConfidence(classification.confidence)}</span>
                </div>
                <p className="mono mt-1 mb-0 text-[10px] faint">
                  {classification.method} · {classification.sourceReference || classification.createdBy} ·{' '}
                  {formatTimestamp(classification.createdAt)}
                </p>
              </li>
            ))}
          </ul>
        )}
      </section>

      <section className="aegis-panel">
        <h2>Supporting observations ({observationsQuery.data?.total ?? 0})</h2>
        <p className="sub">Evidence before interpretation</p>
        {observationsQuery.isLoading ? (
          <PanelSkeleton rows={2} />
        ) : (observationsQuery.data?.items?.length ?? 0) === 0 ? (
          <p className="aegis-empty">No observations linked to this track.</p>
        ) : (
          <ul className="m-0 flex list-none flex-col gap-2 p-0">
            {observationsQuery.data?.items.map((observation) => (
              <li className="flex items-center justify-between gap-3 border-b border-[color:var(--line)] py-2 last:border-b-0" key={observation.id}>
                <span className="mono text-[10px] muted">{formatTimestamp(observation.observedAt)}</span>
                <span className="text-[10px] tracking-[.12em] uppercase muted">
                  {observation.sourceId.split('__').pop()}
                </span>
                <span className="mono text-[10px] faint">{formatCoord(observation.position)}</span>
              </li>
            ))}
          </ul>
        )}
      </section>

      <section className="aegis-panel">
        <h2>Related alerts ({alertsQuery.data?.total ?? 0})</h2>
        <p className="sub">Alerting stays auditable</p>
        {(alertsQuery.data?.items?.length ?? 0) === 0 ? (
          <p className="aegis-empty">No alerts reference this track.</p>
        ) : (
          <ul className="m-0 flex list-none flex-col gap-2 p-0">
            {alertsQuery.data?.items.map((alert) => (
              <li key={alert.id} className="flex items-center justify-between gap-2">
                <span className="truncate text-[12px] tracking-[.03em]">{alert.title}</span>
                <span className="aegis-sev">{alert.severity}</span>
              </li>
            ))}
          </ul>
        )}
      </section>

      <section className="aegis-panel">
        <div className="flex items-start justify-between gap-2">
          <div>
            <h2>Assessments ({assessmentsQuery.data?.total ?? 0})</h2>
            <p className="sub">Human or machine judgement, with confidence</p>
          </div>
          <button
            type="button"
            className="aegis-btn shrink-0"
            onClick={() => setAssessmentOpen(true)}
            // Recording a judgement about a reconstructed past moment would
            // attach present-day reasoning to a historical picture.
            disabled={Boolean(replayAt)}
            title={
              replayAt
                ? 'Return to the live picture to record an assessment'
                : 'State what you conclude about this track, and what it rests on'
            }
          >
            Record
          </button>
        </div>
        {replayAt ? (
          <p className="aegis-empty">
            Assessments are not shown while replaying. They are recorded against what the system
            knows now, which would misdescribe the moment on screen.
          </p>
        ) : (assessmentsQuery.data?.items?.length ?? 0) === 0 ? (
          <p className="aegis-empty">
            No assessments for this track. An assessment records your conclusion about what this
            track is, separately from the observations that support it.
          </p>
        ) : (
          <ul className="m-0 flex list-none flex-col gap-2 p-0">
            {assessmentsQuery.data?.items.map((assessment) => (
              <li key={assessment.id}>
                <p className="m-0 text-[12px] tracking-[.03em]">{assessment.conclusion}</p>
                <p className="mono mt-1 mb-0 text-[10px] faint">
                  {assessment.method} · {formatConfidence(assessment.confidence)} ·{' '}
                  {formatTimestamp(assessment.createdAt)}
                </p>
              </li>
            ))}
          </ul>
        )}
      </section>

      <p className="mono m-0 px-4 py-3 text-[9px] faint">
        Updated {formatRelative(track.updatedAt)}
      </p>

      <AssessmentFormDialog
        open={assessmentOpen}
        onOpenChange={setAssessmentOpen}
        subjectType="track"
        subjectId={trackId}
      />
    </>
  )
}

function PanelSkeleton({ rows = 1 }: { rows?: number }) {
  return (
    <div className="flex flex-col gap-2" aria-hidden="true">
      {Array.from({ length: rows }, (_, index) => (
        <span
          key={index}
          className="block h-3 w-full rounded"
          style={{ background: 'var(--surface-2)' }}
        />
      ))}
    </div>
  )
}