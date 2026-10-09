import { useQuery } from '@tanstack/react-query'

import { ErrorState } from '@/components/shared/states'
import { api } from '@/lib/api'
import {
  formatConfidence,
  formatCoord,
  formatHeading,
  formatRelative,
  formatSpeed,
  formatTimestamp,
} from '@/lib/format'
import { queryKeys } from '@/lib/queryKeys'
import { useUiStore } from '@/stores/ui'

export function TrackDetailPanel({ trackId }: { trackId: string }) {
  const showHistory = useUiStore((state) => state.showTrackHistory)
  const toggleHistory = useUiStore((state) => state.toggleTrackHistory)

  const trackQuery = useQuery({
    queryKey: queryKeys.tracks.detail(trackId),
    queryFn: () => api.getTrack(trackId),
    enabled: Boolean(trackId),
    refetchInterval: 15_000,
  })
  const classificationsQuery = useQuery({
    queryKey: queryKeys.tracks.classifications(trackId),
    queryFn: () => api.listTrackClassifications(trackId, { limit: 20 }),
    enabled: Boolean(trackId),
  })
  const observationsQuery = useQuery({
    queryKey: queryKeys.observations.list({ track_id: trackId, limit: 20 }),
    queryFn: () => api.listObservations({ track_id: trackId, limit: 20 }),
    enabled: Boolean(trackId),
  })
  const alertsQuery = useQuery({
    queryKey: queryKeys.alerts.list({ track_id: trackId, limit: 10 }),
    queryFn: () => api.listAlerts({ track_id: trackId, limit: 10 }),
    enabled: Boolean(trackId),
  })
  const assessmentsQuery = useQuery({
    queryKey: queryKeys.assessments.list({ subject_type: 'track', subject_id: trackId }),
    queryFn: () => api.listAssessments({ subject_type: 'track', subject_id: trackId, limit: 10 }),
    enabled: Boolean(trackId),
  })

  const track = trackQuery.data

  if (trackQuery.isError) {
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
        <dl className="aegis-kv">
          <dt>Status</dt>
          <dd className="word">{track.status}</dd>
          <dt>External ref</dt>
          <dd>{track.externalRef || '—'}</dd>
          <dt>Observations</dt>
          <dd>{track.observationCount}</dd>
          <dt>Position</dt>
          <dd>{formatCoord(track.position)}</dd>
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
        ) : (classificationsQuery.data?.items.length ?? 0) === 0 ? (
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
        ) : (observationsQuery.data?.items.length ?? 0) === 0 ? (
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
        {(alertsQuery.data?.items.length ?? 0) === 0 ? (
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
        <h2>Assessments ({assessmentsQuery.data?.total ?? 0})</h2>
        <p className="sub">Human or machine judgement, with confidence</p>
        {(assessmentsQuery.data?.items.length ?? 0) === 0 ? (
          <p className="aegis-empty">No assessments for this track.</p>
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