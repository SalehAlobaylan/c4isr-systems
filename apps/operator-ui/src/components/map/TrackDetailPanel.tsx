import { useQuery } from '@tanstack/react-query'

import { StateBadge } from '@/components/shared/badges'
import { DetailList, ErrorState, SectionTitle } from '@/components/shared/states'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'
import { Skeleton } from '@/components/ui/skeleton'
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
    return (
      <div className="flex flex-col gap-3 p-4">
        <Skeleton className="h-4 w-2/3" />
        <Skeleton className="h-3 w-1/2" />
        <Skeleton className="h-24 w-full" />
      </div>
    )
  }

  const scenarioRunId =
    typeof track.metadata?.scenarioRunId === 'string' ? track.metadata.scenarioRunId : undefined
  const resourceNamespace =
    typeof track.metadata?.resourceNamespace === 'string'
      ? track.metadata.resourceNamespace
      : undefined
  const sourceId = typeof track.metadata?.sourceId === 'string' ? track.metadata.sourceId : undefined

  return (
    <div className="flex flex-col gap-4 p-4">
      <div className="flex items-center justify-between gap-2">
        <StateBadge value={track.status} />
        <span className="font-mono text-[10px] text-ink-faint">{track.id}</span>
      </div>

      <DetailList
        columns={2}
        items={[
          { label: 'External ref', value: track.externalRef || '—', mono: true },
          { label: 'Observations', value: String(track.observationCount) },
          { label: 'Position', value: formatCoord(track.position), mono: true },
          { label: 'Speed', value: formatSpeed(track.speed) },
          { label: 'Heading', value: formatHeading(track.heading) },
          { label: 'Last seen', value: formatTimestamp(track.lastSeenAt) },
          { label: 'First seen', value: formatTimestamp(track.firstSeenAt) },
          { label: 'Updated', value: formatRelative(track.updatedAt) },
        ]}
      />

      <section className="flex flex-col gap-2">
        <SectionTitle>Provenance</SectionTitle>
        <DetailList
          columns={1}
          items={[
            { label: 'Source', value: sourceId || '—', mono: true },
            { label: 'Scenario run', value: scenarioRunId || 'External / operator', mono: true },
            ...(resourceNamespace
              ? [{ label: 'Resource namespace', value: resourceNamespace, mono: true }]
              : []),
          ]}
        />
      </section>

      <Button
        variant={showHistory ? 'default' : 'outline'}
        size="sm"
        onClick={toggleHistory}
        className="self-start"
      >
        {showHistory ? 'Hide history polyline' : 'Show history polyline'}
      </Button>

      <Separator />

      <section className="flex flex-col gap-2">
        <SectionTitle>Classification</SectionTitle>
        {classificationsQuery.isLoading ? (
          <Skeleton className="h-10 w-full" />
        ) : (classificationsQuery.data?.items.length ?? 0) === 0 ? (
          <p className="text-xs text-ink-faint">No classifications recorded.</p>
        ) : (
          <ul className="flex flex-col gap-2">
            {classificationsQuery.data?.items.map((classification) => (
              <li
                key={classification.id}
                className="rounded-md border border-edge bg-panel-raised/50 px-3 py-2"
              >
                <div className="flex items-center justify-between gap-2">
                  <span className="text-xs font-medium text-ink">{classification.label}</span>
                  <span className="font-mono text-[11px] text-accent">
                    {formatConfidence(classification.confidence)}
                  </span>
                </div>
                <div className="mt-1 flex items-center gap-2">
                  <div className="h-1 flex-1 overflow-hidden rounded bg-edge">
                    <div
                      className="h-full bg-accent"
                      style={{ width: `${Math.round((classification.confidence ?? 0) * 100)}%` }}
                    />
                  </div>
                  <span className="font-mono text-[10px] text-ink-faint">
                    {classification.method}
                  </span>
                </div>
                <p className="mt-1 text-[10px] text-ink-faint">
                  {classification.sourceReference || classification.createdBy} ·{' '}
                  {formatTimestamp(classification.createdAt)}
                </p>
              </li>
            ))}
          </ul>
        )}
      </section>

      <Separator />

      <section className="flex flex-col gap-2">
        <SectionTitle>Evidence ({observationsQuery.data?.total ?? 0})</SectionTitle>
        {observationsQuery.isLoading ? (
          <Skeleton className="h-10 w-full" />
        ) : (observationsQuery.data?.items.length ?? 0) === 0 ? (
          <p className="text-xs text-ink-faint">No observations linked to this track.</p>
        ) : (
          <div className="overflow-hidden rounded-md border border-edge">
            <table className="w-full text-left">
              <thead className="bg-panel-raised/60">
                <tr className="text-[10px] tracking-wider text-ink-faint uppercase">
                  <th className="px-2 py-1.5 font-medium">Observed</th>
                  <th className="px-2 py-1.5 font-medium">Source</th>
                  <th className="px-2 py-1.5 font-medium">Position</th>
                </tr>
              </thead>
              <tbody>
                {observationsQuery.data?.items.map((observation) => (
                  <tr key={observation.id} className="border-t border-edge/70">
                    <td className="px-2 py-1.5 font-mono text-[10px] text-ink-muted">
                      {formatTimestamp(observation.observedAt)}
                    </td>
                    <td className="px-2 py-1.5 font-mono text-[10px] text-ink-muted">
                      {observation.sourceId}
                    </td>
                    <td className="px-2 py-1.5 font-mono text-[10px] text-ink-faint">
                      {formatCoord(observation.position)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      <Separator />

      <section className="flex flex-col gap-2">
        <SectionTitle>Related alerts ({alertsQuery.data?.total ?? 0})</SectionTitle>
        {(alertsQuery.data?.items.length ?? 0) === 0 ? (
          <p className="text-xs text-ink-faint">No alerts reference this track.</p>
        ) : (
          <ul className="flex flex-col gap-1.5">
            {alertsQuery.data?.items.map((alert) => (
              <li key={alert.id} className="flex items-center justify-between gap-2">
                <span className="truncate text-xs text-ink-muted">{alert.title}</span>
                <StateBadge value={alert.severity} />
              </li>
            ))}
          </ul>
        )}
      </section>

      <Separator />

      <section className="flex flex-col gap-2">
        <SectionTitle>Assessments ({assessmentsQuery.data?.total ?? 0})</SectionTitle>
        {(assessmentsQuery.data?.items.length ?? 0) === 0 ? (
          <p className="text-xs text-ink-faint">No assessments for this track.</p>
        ) : (
          <ul className="flex flex-col gap-2">
            {assessmentsQuery.data?.items.map((assessment) => (
              <li key={assessment.id} className="rounded-md border border-edge px-3 py-2">
                <p className="text-xs text-ink">{assessment.conclusion}</p>
                <p className="mt-0.5 font-mono text-[10px] text-ink-faint">
                  {assessment.method} · {formatConfidence(assessment.confidence)} ·{' '}
                  {formatTimestamp(assessment.createdAt)}
                </p>
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  )
}
