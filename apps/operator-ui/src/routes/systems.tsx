import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { useMemo } from 'react'

import { Page } from '@/components/layout/Page'
import {
  AlertStateBadge,
  CommandStateBadge,
  IncidentStatusBadge,
  SeverityBadge,
} from '@/components/shared/badges'
import { EmptyState, ErrorState } from '@/components/shared/states'
import { api, fetchHealth } from '@/lib/api'
import { formatRelative, formatTimestamp, truncateId } from '@/lib/format'
import {
  TONE_GLYPH,
  presentHealth,
  presentValue,
  type SemanticTone,
} from '@/lib/operationalPresentation'
import { queryKeys } from '@/lib/queryKeys'

const LIST_LIMIT = 500

/**
 * A subsystem cell on the grid.
 *
 * Every number here is a count of something the API returned. `attention` is
 * the one derived value: it is a list of things a commander would want to look
 * at, not a score, and it is always rendered as labelled badges — the grid
 * never reduces a subsystem to a colour (DESIGN.md §7.1).
 *
 * `headline` and `verdict` exist so the same numbers can drive both the
 * six-across strip and the cards without the two disagreeing. `verdict` is a
 * word, not a colour: a commander asks "is this healthy?" and gets an answer
 * they can read aloud.
 */
interface SystemCell {
  index: string
  title: string
  /** The figure the strip leads with. */
  headline: string
  /** The word beside it. Never a bare tone. */
  verdict: string
  verdictTone: SemanticTone
  /** The one-line context printed in the card's foot. */
  context: string
  metrics: Array<{ key: string; value: string }>
  attention: Array<{ label: string; tone: SemanticTone; count: number }>
  /** An optional inventory printed under the cell's own figures. */
  inventory?: React.ReactNode
  /** The route that holds the detail behind these numbers. */
  href?: string
  isError: boolean
  isLoading: boolean
}

/** A six-across summary cell: name, figure, verdict, detail.
 *
 *  The verdict sits beside the figure, not beside the name. A badge in the
 *  name row competes with the name for the same 200px and truncates it to
 *  "03 PO…" — and a subsystem whose name is unreadable is worse than no strip at
 *  all. The name row therefore carries only a small tone dot, which is safe
 *  because the verdict word is two lines below it (DESIGN.md §7.1). */
function Kpi({ cell }: { cell: SystemCell }) {
  return (
    <div>
      <p className="k">
        <span className="aegis-xsystem-idx">{cell.index}</span>
        <span className="aegis-kpi-name">{cell.title}</span>
        <i
          className="tone-glyph spacer"
          data-glyph={TONE_GLYPH[cell.verdictTone]}
          aria-hidden="true"
        />
      </p>
      <p className="v">
        <span>{cell.isError ? '—' : cell.headline}</span>
        <span className="aegis-sev" data-tone={cell.verdictTone}>
          <i className="tone-glyph" data-glyph={TONE_GLYPH[cell.verdictTone]} aria-hidden="true" />
          <span>{cell.verdict}</span>
        </span>
      </p>
      <span className="n">{cell.context}</span>
    </div>
  )
}

function Cell({ cell, onRetry }: { cell: SystemCell; onRetry: () => void }) {
  return (
    <section className="aegis-xsystem" aria-labelledby={`sys-${cell.index}`}>
      <header className="aegis-xsystem-head">
        <span className="aegis-xsystem-idx">{cell.index}</span>
        <h3 className="aegis-xsystem-title" id={`sys-${cell.index}`}>
          {cell.title}
        </h3>
        <span className="spacer">
          {cell.isError ? (
            <span className="aegis-sev" data-tone="critical">
              <i className="tone-glyph" data-glyph={TONE_GLYPH.critical} aria-hidden="true" />
              <span>UNRESPONSIVE</span>
            </span>
          ) : cell.isLoading ? (
            <span className="mono text-[12px] text-ink-muted">—</span>
          ) : (
            <span className="aegis-sev" data-tone={cell.verdictTone}>
              <i className="tone-glyph" data-glyph={TONE_GLYPH[cell.verdictTone]} aria-hidden="true" />
              <span>{cell.verdict}</span>
            </span>
          )}
        </span>
      </header>

      <div className="aegis-xsystem-body">
        {cell.isError ? (
          <ErrorState error={new Error('This subsystem did not respond.')} onRetry={onRetry} />
        ) : (
          <>
            <div className="aegis-xsystem-metrics">
              {cell.metrics.map((metric) => (
                <div key={metric.key} className="aegis-xsystem-metric">
                  <p className="k">{metric.key}</p>
                  <p className="v">{cell.isLoading ? '—' : metric.value}</p>
                </div>
              ))}
            </div>

            {cell.attention.length > 0 ? (
              <div className="aegis-xsystem-attn">
                <p className="cap">Standing attention</p>
                <div className="aegis-xsystem-badges">
                  {cell.attention.map((item) => (
                    <span key={item.label} className="aegis-sev" data-tone={item.tone}>
                      <i className="tone-glyph" data-glyph={TONE_GLYPH[item.tone]} aria-hidden="true" />
                      <span>{item.label}</span>
                      <span className="opacity-70">{item.count}</span>
                    </span>
                  ))}
                </div>
              </div>
            ) : (
              <p className="aegis-xnote">Nothing currently needs attention in this subsystem.</p>
            )}

            {cell.inventory ? <div className="aegis-xsystem-inventory">{cell.inventory}</div> : null}
          </>
        )}
      </div>

      <div className="aegis-xsystem-foot">
        <span className="ctx" title={cell.context}>
          {cell.context}
        </span>
        {cell.href ? (
          <Link
            to={cell.href}
            className="aegis-xsystem-link mono text-[12px] text-[color:var(--emad-link)]"
          >
            Open →
          </Link>
        ) : null}
      </div>
    </section>
  )
}

export function SystemsPage() {
  const healthQuery = useQuery({
    queryKey: queryKeys.health(),
    queryFn: () => fetchHealth(),
    refetchInterval: 30_000,
  })

  const sourceQuery = useQuery({
    queryKey: queryKeys.sources.list({ limit: LIST_LIMIT }),
    queryFn: () => api.listSources({ limit: LIST_LIMIT }),
  })

  const observationQuery = useQuery({
    queryKey: queryKeys.observations.list({ limit: LIST_LIMIT }),
    queryFn: () => api.listObservations({ limit: LIST_LIMIT }),
  })

  const trackQuery = useQuery({
    queryKey: queryKeys.tracks.list({ limit: LIST_LIMIT }),
    queryFn: () => api.listTracks({ limit: LIST_LIMIT }),
  })

  const assetQuery = useQuery({
    queryKey: queryKeys.assets.list({ limit: LIST_LIMIT }),
    queryFn: () => api.listAssets({ limit: LIST_LIMIT }),
  })

  const alertQuery = useQuery({
    queryKey: queryKeys.alerts.list({ limit: LIST_LIMIT }),
    queryFn: () => api.listAlerts({ limit: LIST_LIMIT }),
  })

  const incidentQuery = useQuery({
    queryKey: queryKeys.incidents.list({ limit: LIST_LIMIT }),
    queryFn: () => api.listIncidents({ limit: LIST_LIMIT }),
  })

  const commandQuery = useQuery({
    queryKey: queryKeys.commands.list({ limit: LIST_LIMIT }),
    queryFn: () => api.listCommands({ limit: LIST_LIMIT }),
  })

  /* Each cell is computed independently so one subsystem failing cannot blank
     the grid — a partial picture of a live system is more useful to an operator
     than an error page that tells them nothing about the other seven. */
  const cells = useMemo<SystemCell[]>(() => {
    const sources = sourceQuery.data?.items ?? []
    const observations = observationQuery.data?.items ?? []
    const tracks = trackQuery.data?.items ?? []
    const assets = assetQuery.data?.items ?? []
    const alerts = alertQuery.data?.items ?? []
    const incidents = incidentQuery.data?.items ?? []
    const commands = commandQuery.data?.items ?? []

    const countBy = <T,>(items: T[], key: (item: T) => string | undefined | null) => {
      const tally = new Map<string, number>()
      for (const item of items) {
        const value = key(item)
        if (!value) continue
        tally.set(value, (tally.get(value) ?? 0) + 1)
      }
      return tally
    }

    /* ── 01 Sources ─────────────────────────────────────────────────────── */
    const sourceStatus = countBy(sources, (item) => item.status)
    const sourceFreshest = sources
      .map((item) => item.updatedAt)
      .filter(Boolean)
      .sort()
      .at(-1)
    const degradedSources = sources.filter(
      (item) => item.status === 'degraded' || item.status === 'offline',
    )

    /* ── 02 Observations ───────────────────────────────────────────────── */
    const duplicates = observations.filter((item) => item.duplicate).length
    // Ingest latency is measured between the two timestamps the API actually
    // returns. When `processedAt` is absent the sample is still in flight and
    // is counted rather than silently dropped.
    const latencies = observations
      .map((item) => {
        if (!item.receivedAt) return null
        const received = new Date(item.receivedAt).getTime()
        const processed = item.processedAt ? new Date(item.processedAt).getTime() : null
        if (processed === null || Number.isNaN(received)) return null
        return processed - received
      })
      .filter((value): value is number => value !== null && value >= 0)
      .sort((a, b) => a - b)
    const medianLatency = latencies.length > 0 ? latencies[Math.floor(latencies.length / 2)] : null

    /* ── 03 Tracks ──────────────────────────────────────────────────────── */
    const activeTracks = tracks.filter((track) => track.status === 'active')
    // `lost` and `closed` are distinct lifecycle states and are counted
    // separately. Collapsing them into one "not active" figure would hide a
    // track that still needs looking for behind one that has been dealt with
    // (DESIGN.md §7.1).
    const lostTracks = tracks.filter((track) => track.status === 'lost')
    const untrustworthyTracks = tracks.filter(
      (track) => track.freshness === 'stale' || track.freshness === 'unavailable',
    )
    const trackFreshest = tracks
      .map((track) => track.lastSeenAt)
      .filter(Boolean)
      .sort()
      .at(-1)

    /* ── 04 Assets ──────────────────────────────────────────────────────── */
    const unavailableAssets = assets.filter(
      (asset) => asset.status === 'unavailable' || asset.status === 'maintenance',
    )
    const disconnectedAssets = assets.filter((asset) => asset.connectionState === 'disconnected')
    const taskedAssets = assets.filter((asset) => asset.status === 'assigned')

    /* ── 05 Alerts & incidents ──────────────────────────────────────────── */
    const activeAlerts = alerts.filter((alert) => alert.state !== 'RESOLVED')
    const severityTally = countBy(activeAlerts, (alert) => alert.severity)
    const openIncidents = incidents.filter(
      (incident) => incident.status !== 'CLOSED' && incident.status !== 'RESOLVED',
    )

    /* ── 06 Command dispatch ────────────────────────────────────────────── */
    const unsettled = commands.filter(
      (command) =>
        command.state !== 'COMPLETED' &&
        command.state !== 'REJECTED' &&
        command.state !== 'FAILED' &&
        command.state !== 'CANCELLED' &&
        command.state !== 'TIMED_OUT',
    )
    const failedCommands = commands.filter(
      (command) =>
        command.state === 'FAILED' ||
        command.state === 'REJECTED' ||
        command.state === 'TIMED_OUT',
    )
    const acknowledged = commands.filter((command) => command.state === 'ACKNOWLEDGED').length
    const completed = commands.filter((command) => command.state === 'COMPLETED').length
    const commandFreshest = commands
      .map((command) => command.createdAt)
      .filter(Boolean)
      .sort()
      .at(-1)

    const trackCell: SystemCell = {
      index: '01',
      title: 'Ingest & sources',
      headline: `${sources.length}`,
      verdict:
        degradedSources.length > 0
          ? degradedSources.some((s) => s.status === 'offline')
            ? 'Offline'
            : 'Degraded'
          : sources.length > 0
            ? 'Nominal'
            : 'None registered',
      verdictTone:
        degradedSources.some((s) => s.status === 'offline')
          ? 'critical'
          : degradedSources.length > 0
            ? 'caution'
            : sources.length > 0
              ? 'positive'
              : 'neutral',
      context: sourceFreshest
        ? `last source update ${formatRelative(sourceFreshest)}`
        : 'no sources registered',
      metrics: [
        { key: 'Sources', value: String(sources.length) },
        { key: 'Active', value: String(sourceStatus.get('active') ?? 0) },
        {
          key: 'Degraded',
          value: String((sourceStatus.get('degraded') ?? 0) + (sourceStatus.get('offline') ?? 0)),
        },
      ],
      attention: degradedSources.slice(0, 6).map((source) => ({
        label: truncateId(source.name || source.id, 18),
        tone: presentValue(source.status, { active: 'info', degraded: 'caution' }).tone,
        count: 1,
      })),
      /* The devices themselves, not just their count. The reference grid names
         every sensor and what state it is in, and it is right to: a commander
         asking "which feed is down" cannot be answered by "one of three is
         degraded" — the name is the whole point. */
      inventory: sources.length === 0 ? null : (
        <table className="aegis-xtable">
          <caption className="sr-only">Registered sources and the state each is in</caption>
          <thead>
            <tr>
              <th scope="col">Device</th>
              <th scope="col">Type</th>
              <th scope="col">Status</th>
            </tr>
          </thead>
          <tbody>
            {sources.slice(0, 8).map((source) => {
              const tone = presentValue(source.status, { active: 'info', degraded: 'caution' }).tone
              return (
                <tr key={source.id}>
                  <td className="truncate" title={source.name || source.id}>
                    {truncateId(source.name || source.id, 22)}
                  </td>
                  <td>{source.type}</td>
                  <td>
                    <span className="aegis-sev" data-tone={tone}>
                      <i className="tone-glyph" data-glyph={TONE_GLYPH[tone]} aria-hidden="true" />
                      <span>{source.status.toUpperCase()}</span>
                    </span>
                  </td>
                </tr>
              )
            })}
          </tbody>
        </table>
      ),
      isError: sourceQuery.isError,
      isLoading: sourceQuery.isLoading,
    }

    const observationCell: SystemCell = {
      index: '02',
      title: 'Observation intake',
      headline: String(observationQuery.data?.total ?? observations.length),
      verdict: observationQuery.isError
        ? 'Unreachable'
        : (observationQuery.data?.total ?? 0) === 0
          ? 'Empty'
          : duplicates > 0
            ? 'Rejects'
            : 'Accepting',
      verdictTone: observationQuery.isError
        ? 'critical'
        : (observationQuery.data?.total ?? 0) === 0
          ? 'neutral'
          : duplicates > 0
            ? 'caution'
            : 'positive',
      context: (observationQuery.data?.total ?? 0) === 0
        ? 'nothing ingested'
        : medianLatency === null
          ? 'lag not measurable yet'
          : `median ingest lag ${medianLatency} ms`,
      metrics: [
        { key: 'Received', value: String(observationQuery.data?.total ?? observations.length) },
        { key: 'Duplicates', value: String(duplicates) },
        {
          key: 'Median lag',
          value: medianLatency === null ? '—' : `${medianLatency} ms`,
        },
      ],
      attention:
        duplicates > 0
          ? [{ label: 'Duplicate samples rejected downstream', tone: 'caution', count: duplicates }]
          : [],
      isError: observationQuery.isError,
      isLoading: observationQuery.isLoading,
    }

    const trackCellOut: SystemCell = {
      index: '03',
      title: 'Track correlation',
      headline: String(activeTracks.length),
      verdict: trackQuery.isError
        ? 'Unreachable'
        : tracks.length === 0
          ? 'No tracks'
          : untrustworthyTracks.length > 0
            ? 'Uncertain'
            : 'Tracking',
      verdictTone: trackQuery.isError
        ? 'critical'
        : tracks.length === 0
          ? 'neutral'
          : untrustworthyTracks.length > 0
            ? 'caution'
            : 'positive',
      context: trackFreshest ? `latest fix ${formatRelative(trackFreshest)}` : 'no tracks',
      metrics: [
        { key: 'Active', value: String(activeTracks.length) },
        { key: 'Lost', value: String(lostTracks.length) },
        { key: 'Not current', value: String(untrustworthyTracks.length) },
      ],
      attention: untrustworthyTracks.slice(0, 6).map((track) => ({
        label: truncateId(track.externalRef || track.id, 18),
        tone: track.freshness === 'stale' ? 'critical' : 'neutral',
        count: 1,
      })),
      href: '/tracks',
      isError: trackQuery.isError,
      isLoading: trackQuery.isLoading,
    }

    const assetCell: SystemCell = {
      index: '04',
      title: 'Asset readiness',
      headline: String(assets.length),
      verdict: assetQuery.isError
        ? 'Unreachable'
        : assets.length === 0
          ? 'None registered'
          : disconnectedAssets.length > 0
            ? 'Links down'
            : unavailableAssets.length > 0
              ? 'Partial'
              : taskedAssets.length > 0
                ? 'Tasked'
                : 'Available',
      verdictTone: assetQuery.isError
        ? 'critical'
        : assets.length === 0
          ? 'neutral'
          : disconnectedAssets.length > 0
            ? 'critical'
            : unavailableAssets.length > 0
              ? 'caution'
              : 'positive',
      context: `${taskedAssets.length} tasked of ${assets.length}`,
      metrics: [
        { key: 'Registered', value: String(assets.length) },
        { key: 'Available', value: String(assets.filter((a) => a.status === 'available').length) },
        { key: 'Unavailable', value: String(unavailableAssets.length) },
      ],
      attention: [
        ...unavailableAssets.slice(0, 4).map((asset) => ({
          label: truncateId(asset.name || asset.id, 16),
          tone: 'caution' as SemanticTone,
          count: 1,
        })),
        ...(disconnectedAssets.length > 0
          ? [
              {
                label: 'Disconnected links',
                tone: 'neutral' as SemanticTone,
                count: disconnectedAssets.length,
              },
            ]
          : []),
      ],
      href: '/assets',
      isError: assetQuery.isError,
      isLoading: assetQuery.isLoading,
    }

    const alertCell: SystemCell = {
      index: '05',
      title: 'Alerts & incidents',
      headline: String(activeAlerts.length),
      verdict:
        (severityTally.get('critical') ?? 0) > 0
          ? 'Critical raised'
          : (severityTally.get('high') ?? 0) > 0
            ? 'High raised'
            : activeAlerts.length > 0
              ? 'Advisory only'
              : openIncidents.length > 0
                ? 'Incidents'
                : 'Quiet',
      verdictTone: (severityTally.get('critical') ?? 0) > 0
        ? 'critical'
        : (severityTally.get('high') ?? 0) > 0
          ? 'high'
          : activeAlerts.length > 0
            ? 'caution'
            : openIncidents.length > 0
              ? 'info'
              : 'positive',
      context: `${openIncidents.length} open incident${openIncidents.length === 1 ? '' : 's'}`,
      metrics: [
        { key: 'Active alerts', value: String(activeAlerts.length) },
        { key: 'Critical', value: String(severityTally.get('critical') ?? 0) },
        { key: 'Open incidents', value: String(openIncidents.length) },
      ],
      attention: [
        ...(['critical', 'high', 'medium', 'low'] as const)
          .filter((severity) => (severityTally.get(severity) ?? 0) > 0)
          .map((severity) => ({
            label: severity.toUpperCase(),
            tone: presentValue(severity, {
              critical: 'critical',
              high: 'high',
              medium: 'caution',
              low: 'info',
            }).tone,
            count: severityTally.get(severity) ?? 0,
          })),
        ...openIncidents.slice(0, 3).map((incident) => ({
          label: `${incident.status} · ${truncateId(incident.title, 14)}`,
          tone: 'info' as SemanticTone,
          count: 1,
        })),
      ],
      href: '/alerts',
      isError: alertQuery.isError || incidentQuery.isError,
      isLoading: alertQuery.isLoading || incidentQuery.isLoading,
    }

    const commandCell: SystemCell = {
      index: '06',
      title: 'Command dispatch',
      headline: String(commands.length),
      verdict: commandQuery.isError
        ? 'Unreachable'
        : failedCommands.length > 0
          ? 'Failed'
          : unsettled.length > 0
            ? 'In flight'
            : commands.length > 0
              ? 'All settled'
              : 'None issued',
      verdictTone: commandQuery.isError
        ? 'critical'
        : failedCommands.length > 0
          ? 'critical'
          : unsettled.length > 0
            ? 'info'
            : commands.length > 0
              ? 'positive'
              : 'neutral',
      context: unsettled.length
        ? `${unsettled.length} unsettled · last issued ${commandFreshest ? formatRelative(commandFreshest) : '—'}`
        : commands.length
          ? `all settled · last issued ${formatRelative(commandFreshest)}`
          : 'no commands issued',
      metrics: [
        { key: 'Issued', value: String(commands.length) },
        { key: 'Acknowledged', value: String(acknowledged) },
        { key: 'Completed', value: String(completed) },
      ],
      attention: failedCommands.slice(0, 6).map((command) => ({
        label: `${command.state} · ${truncateId(command.type, 12)}`,
        tone: presentValue(command.state, {
          FAILED: 'critical',
          REJECTED: 'critical',
          TIMED_OUT: 'high',
        }).tone,
        count: 1,
      })),
      href: '/incidents',
      isError: commandQuery.isError,
      isLoading: commandQuery.isLoading,
    }

    return [
      trackCell,
      observationCell,
      trackCellOut,
      assetCell,
      alertCell,
      commandCell,
    ]
  }, [
    sourceQuery.data,
    sourceQuery.isError,
    sourceQuery.isLoading,
    observationQuery.data,
    observationQuery.isError,
    observationQuery.isLoading,
    trackQuery.data,
    trackQuery.isError,
    trackQuery.isLoading,
    assetQuery.data,
    assetQuery.isError,
    assetQuery.isLoading,
    alertQuery.data,
    alertQuery.isError,
    alertQuery.isLoading,
    incidentQuery.data,
    incidentQuery.isError,
    incidentQuery.isLoading,
    commandQuery.data,
    commandQuery.isError,
    commandQuery.isLoading,
  ])

  const health = healthQuery.data
  const serviceTone = presentHealth(health?.status).tone
  const databaseTone = presentHealth(health?.database).tone

  // The one number an operator would want before scrolling: how much is
  // currently asking for a decision. Severity, not freshness or lifecycle — an
  // old resolved critical alert is not on this count.
  const attentionCount = useMemo(() => {
    const items = alertQuery.data?.items ?? []
    return items.filter(
      (alert) =>
        alert.state !== 'RESOLVED' && (alert.severity === 'critical' || alert.severity === 'high'),
    ).length
  }, [alertQuery.data])

  return (
    <Page
      title="Systems"
      description="Every subsystem the picture depends on, and what each is currently asking of an operator"
      actions={
        <div className="flex flex-wrap items-center gap-2">
          {attentionCount > 0 ? (
            <span className="aegis-sev" data-tone="high">
              <i className="tone-glyph" data-glyph={TONE_GLYPH.high} aria-hidden="true" />
              <span>Needs attention</span>
              <span className="opacity-70">{attentionCount}</span>
            </span>
          ) : null}
          <span
            className="aegis-sev"
            data-tone={healthQuery.isError ? 'critical' : databaseTone}
            title={
              healthQuery.isError
                ? 'The platform did not answer /health'
                : `service ${health?.status ?? 'unknown'} · database ${health?.database ?? 'unknown'}`
            }
          >
            <i
              className="tone-glyph"
              data-glyph={TONE_GLYPH[healthQuery.isError ? 'critical' : serviceTone]}
              aria-hidden="true"
            />
            <span>
              {healthQuery.isLoading
                ? 'Checking'
                : healthQuery.isError
                  ? 'Unreachable'
                  : `Service ${(health?.status ?? 'unknown').toUpperCase()} · DB ${(health?.database ?? 'unknown').toUpperCase()}`}
            </span>
          </span>
        </div>
      }
    >
      {/* Platform health rides in the page actions rather than taking a slot in
          the strip: the strip's six cells are the six subsystems an operator can
          act on, and the service/database verdict is a property of the platform
          rather than of any one of them. */}
      <div className="aegis-kpi6" role="group" aria-label="Subsystem summary">
        {cells.map((cell) => (
          <Kpi key={cell.index} cell={cell} />
        ))}
      </div>

      {healthQuery.isError ? (
        <div className="p-4">
          <ErrorState error={healthQuery.error} onRetry={() => void healthQuery.refetch()} />
        </div>
      ) : null}

      <div className="aegis-xsystems">
        {cells.map((cell) => (
          <Cell
            key={cell.index}
            cell={cell}
            onRetry={() => {
              switch (cell.index) {
                case '01':
                  void sourceQuery.refetch()
                  break
                case '02':
                  void observationQuery.refetch()
                  break
                case '03':
                  void trackQuery.refetch()
                  break
                case '04':
                  void assetQuery.refetch()
                  break
                case '05':
                  void alertQuery.refetch()
                  void incidentQuery.refetch()
                  break
                case '06':
                  void commandQuery.refetch()
                  break
              }
            }}
          />
        ))}
      </div>

      {/* The most recent cross-system events, kept below the grid so the grid is
          read first. This is the audit trail for the numbers above. */}
      <div className="aegis-xgrid">
        <div className="aegis-xcol">
          <section className="aegis-panel" aria-labelledby="sys-alerts-detail">
            <h2 id="sys-alerts-detail">Active alerts requiring a decision</h2>
            <p className="sub">Severity and lifecycle are separate facts and are shown separately</p>
            {alertQuery.isError ? (
              <ErrorState error={alertQuery.error} onRetry={() => void alertQuery.refetch()} />
            ) : alertQuery.isLoading ? (
              <EmptyState title="Loading alerts…" />
            ) : (alertQuery.data?.items ?? []).filter((alert) => alert.state !== 'RESOLVED').length ===
              0 ? (
              <EmptyState
                title="No active alerts"
                description="Nothing is currently requesting operator attention."
              />
            ) : (
              <div className="aegis-xlist">
                {(alertQuery.data?.items ?? [])
                  .filter((alert) => alert.state !== 'RESOLVED')
                  .slice(0, 12)
                  .map((alert) => (
                    <div key={alert.id} className="aegis-xitem">
                      <SeverityBadge value={alert.severity} />
                      <span className="min-w-0 flex-1 truncate text-[13px] text-ink">
                        {alert.title || alert.type}
                      </span>
                      <AlertStateBadge value={alert.state} />
                      <span className="mono text-[12px] text-ink-muted">
                        {formatRelative(alert.createdAt)}
                      </span>
                    </div>
                  ))}
              </div>
            )}
          </section>
        </div>

        <div className="aegis-xcol">
          <section className="aegis-panel" aria-labelledby="sys-command-detail">
            <h2 id="sys-command-detail">Command state</h2>
            <p className="sub">
              Acknowledged means received, not carried out — only completed says the latter
            </p>
            {commandQuery.isError ? (
              <ErrorState error={commandQuery.error} onRetry={() => void commandQuery.refetch()} />
            ) : commandQuery.isLoading ? (
              <EmptyState title="Loading commands…" />
            ) : (commandQuery.data?.items ?? []).length === 0 ? (
              <EmptyState
                title="No commands issued"
                description="Nothing has been ordered in this session."
              />
            ) : (
              <div className="aegis-xlist">
                {(commandQuery.data?.items ?? [])
                  .slice()
                  .sort((a, b) => b.createdAt.localeCompare(a.createdAt))
                  .slice(0, 12)
                  .map((command) => (
                    <div key={command.id} className="aegis-xitem">
                      <CommandStateBadge value={command.state} />
                      <span className="min-w-0 flex-1 truncate text-[13px] text-ink">
                        {command.type}
                      </span>
                      <span className="mono text-[12px] text-ink-muted">
                        {formatTimestamp(command.createdAt)}
                      </span>
                    </div>
                  ))}
              </div>
            )}
          </section>

          <section className="aegis-panel" aria-labelledby="sys-incident-detail">
            <h2 id="sys-incident-detail">Open incidents</h2>
            <p className="sub">Workflow state, distinct from priority</p>
            {incidentQuery.isError ? (
              <ErrorState error={incidentQuery.error} onRetry={() => void incidentQuery.refetch()} />
            ) : incidentQuery.isLoading ? (
              <EmptyState title="Loading incidents…" />
            ) : (incidentQuery.data?.items ?? [])
                .filter((incident) => incident.status !== 'CLOSED' && incident.status !== 'RESOLVED')
                .length === 0 ? (
              <EmptyState title="No open incidents" />
            ) : (
              <div className="aegis-xlist">
                {(incidentQuery.data?.items ?? [])
                  .filter(
                    (incident) => incident.status !== 'CLOSED' && incident.status !== 'RESOLVED',
                  )
                  .slice(0, 10)
                  .map((incident) => (
                    <Link
                      key={incident.id}
                      to="/incidents/$id"
                      params={{ id: incident.id }}
                      className="aegis-xitem aegis-xitem-link"
                    >
                      <IncidentStatusBadge value={incident.status} />
                      <span className="min-w-0 flex-1 truncate text-[13px] text-ink">
                        {incident.title}
                      </span>
                      <span className="mono text-[12px] text-ink-muted">
                        {formatRelative(incident.createdAt)}
                      </span>
                    </Link>
                  ))}
              </div>
            )}
          </section>
        </div>
      </div>

    </Page>
  )
}
