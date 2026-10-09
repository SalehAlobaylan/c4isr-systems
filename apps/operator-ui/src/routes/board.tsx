import { useQuery } from '@tanstack/react-query'

import { ErrorState } from '@/components/shared/states'
import { api, type Asset, type Track } from '@/lib/api'
import { formatCoord, formatHeading, formatRelative, truncateId } from '@/lib/format'
import { queryKeys } from '@/lib/queryKeys'
import { useLayoutStore } from '@/stores/layout'

import '@/components/board/board.css'

const TRACK_FILTERS = { limit: 500 }
const ASSET_FILTERS = { limit: 500 }
const OBSERVATION_FILTERS = { limit: 500 }
const ALERT_FILTERS = { limit: 500 }
const SOURCE_FILTERS = { limit: 100 }

const TRACK_STATE: Record<string, string> = {
  active: 'track',
  lost: 'warn',
  closed: 'unknown',
}

const ASSET_STATE: Record<string, string> = {
  available: 'ok',
  assigned: 'track',
  maintenance: 'warn',
  unavailable: 'warn',
  offline: 'unknown',
}

interface Column {
  key: string
  kind: 'track' | 'asset'
  id: string
  label: string
  subtitle: string
  state: string
  status: string
  statusSeverity?: 'ok' | 'warn' | 'crit'
  act: string
  track?: Track
  asset?: Asset
  observations: number
  sources: number
  alerts: number
}

interface Metric {
  key: string
  label: string
  sub?: string
  value: (column: Column) => {
    text: string
    kind?: 'word' | 'na' | 'coord'
    sev?: 'ok' | 'warn' | 'crit'
  } | null
}

function shortRef(value: string | undefined | null, fallbackId: string): string {
  const raw = value?.trim()
  if (!raw) return truncateId(fallbackId, 10)
  const tail = raw.split('__').filter(Boolean).pop()
  return (tail ?? raw).toUpperCase()
}

function trackSeverity(status: string): Column['statusSeverity'] {
  if (status === 'active') return undefined
  if (status === 'lost') return 'warn'
  return undefined
}

export function BoardPage() {
  const density = useLayoutStore((state) => state.density)
  const setDensity = useLayoutStore((state) => state.setDensity)
  const chipsOpen = useLayoutStore((state) => state.chipsOpen)
  const toggle = useLayoutStore((state) => state.toggle)

  const tracksQuery = useQuery({
    queryKey: queryKeys.tracks.list(TRACK_FILTERS),
    queryFn: () => api.listTracks(TRACK_FILTERS),
  })
  const assetsQuery = useQuery({
    queryKey: queryKeys.assets.list(ASSET_FILTERS),
    queryFn: () => api.listAssets(ASSET_FILTERS),
  })
  const observationsQuery = useQuery({
    queryKey: queryKeys.observations.list(OBSERVATION_FILTERS),
    queryFn: () => api.listObservations(OBSERVATION_FILTERS),
  })
  const alertsQuery = useQuery({
    queryKey: queryKeys.alerts.list(ALERT_FILTERS),
    queryFn: () => api.listAlerts(ALERT_FILTERS),
  })
  const sourcesQuery = useQuery({
    queryKey: queryKeys.sources.list(SOURCE_FILTERS),
    queryFn: () => api.listSources(SOURCE_FILTERS),
  })

  const tracks = tracksQuery.data?.items ?? []
  const assets = assetsQuery.data?.items ?? []
  const observations = observationsQuery.data?.items ?? []
  const alerts = alertsQuery.data?.items ?? []
  const sources = sourcesQuery.data?.items ?? []

  const observationsByTrack = new Map<string, Set<string>>()
  for (const observation of observations) {
    const hint = observation.trackHint
    if (!hint) continue
    const set = observationsByTrack.get(hint) ?? new Set<string>()
    set.add(observation.sourceId)
    observationsByTrack.set(hint, set)
  }

  const alertsByTrack = new Map<string, number>()
  const alertsByAsset = new Map<string, number>()
  for (const alert of alerts) {
    if (alert.trackId) alertsByTrack.set(alert.trackId, (alertsByTrack.get(alert.trackId) ?? 0) + 1)
    if (alert.assetId) alertsByAsset.set(alert.assetId, (alertsByAsset.get(alert.assetId) ?? 0) + 1)
  }

  const columns: Column[] = [
    ...tracks.map((track) => {
      const trackSources = observationsByTrack.get(track.externalRef)?.size ?? 0
      const alertCount = alertsByTrack.get(track.id) ?? 0
      return {
        key: `track:${track.id}`,
        kind: 'track' as const,
        id: track.id,
        label: shortRef(track.externalRef, track.id),
        subtitle: 'Track',
        state: TRACK_STATE[track.status] ?? 'unknown',
        status: track.status,
        statusSeverity: trackSeverity(track.status),
        act: formatRelative(track.lastSeenAt),
        track,
        observations: track.observationCount,
        sources: trackSources,
        alerts: alertCount,
      }
    }),
    ...assets.map((asset) => ({
      key: `asset:${asset.id}`,
      kind: 'asset' as const,
      id: asset.id,
      label: asset.name || truncateId(asset.id, 10),
      subtitle: asset.type || 'Asset',
      state: asset.health === 'critical' ? 'crit' : (ASSET_STATE[asset.status] ?? 'unknown'),
      status: asset.status,
      statusSeverity:
        asset.health === 'critical' ? ('crit' as const) : asset.status === 'available' ? undefined : ('warn' as const),
      act: formatRelative(asset.lastSeenAt),
      asset,
      observations: 0,
      sources: 0,
      alerts: alertsByAsset.get(asset.id) ?? 0,
    })),
  ]

  const metrics: Metric[] = [
    { key: 'status', label: 'Status', value: (column) => ({ text: column.status, kind: 'word', sev: column.statusSeverity }) },
    {
      key: 'speed',
      label: 'Speed',
      sub: 'm/s',
      value: (column) => {
        const speed = column.track?.speed ?? column.asset?.speed
        if (speed === undefined || speed === null) return { text: '—', kind: 'na' as const }
        return { text: speed.toFixed(1) }
      },
    },
    {
      key: 'heading',
      label: 'Heading',
      value: (column) => {
        const heading = column.track?.heading ?? column.asset?.heading
        if (heading === undefined || heading === null) return { text: '—', kind: 'na' as const }
        return { text: formatHeading(heading).split(' ')[0] }
      },
    },
    {
      key: 'position',
      label: 'Position',
      sub: 'lat, lng',
      value: (column) => {
        const position = column.track?.position ?? column.asset?.position
        if (!position) return { text: '—', kind: 'na' as const }
        return { text: formatCoord(position), kind: 'coord' as const }
      },
    },
    {
      key: 'observations',
      label: 'Observations',
      value: (column) => (column.kind === 'track' ? { text: String(column.observations) } : null),
    },
    {
      key: 'sources',
      label: 'Sources',
      value: (column) => (column.kind === 'track' ? { text: String(column.sources) } : null),
    },
    {
      key: 'alerts',
      label: 'Alerts',
      value: (column) =>
        column.kind === 'track' || column.alerts > 0
          ? { text: String(column.alerts), sev: column.alerts > 0 ? ('warn' as const) : undefined }
          : null,
    },
    {
      key: 'lastSeen',
      label: 'Last seen',
      value: (column) => ({ text: formatRelative(column.track?.lastSeenAt ?? column.asset?.lastSeenAt) }),
    },
  ]

  return (
    <section className="aegis-view" aria-label="Entity board">
      <div className="aegis-chipline">
        <div className="lead">
          <button
            type="button"
            className="aegis-dsc"
            aria-expanded={chipsOpen}
            aria-controls="pane-chips"
            onClick={() => toggle('chipsOpen')}
            title="Show or hide the asset and source rail"
          >
            Assets &amp; sources
          </button>
          <p className="mono faint m-0 mt-1 text-[9px]">
            {assets.length} ASSETS · {sources.length} SOURCES
          </p>
        </div>

        {chipsOpen ? (
          <div className="aegis-chips-wrap" id="pane-chips">
            <div className="aegis-chips" role="group" aria-label="Filter by entity">
              {sources.map((source) => (
                <span key={source.id} className="aegis-chip">
                  <span className="n">{source.name}</span>
                  <span className="t">{source.type}</span>
                </span>
              ))}
              {sources.length === 0 && assets.length === 0 ? (
                <p className="aegis-empty m-0">No sources reported yet.</p>
              ) : null}
            </div>
          </div>
        ) : null}

        <div className="aegis-chipline-actions">
          <span className="cap faint">Rows</span>
          <span className="aegis-seg" role="group" aria-label="Board row density">
            <button
              type="button"
              aria-pressed={density === 'comfortable'}
              onClick={() => setDensity('comfortable')}
              title="Standard row height"
            >
              Comfortable
            </button>
            <button
              type="button"
              aria-pressed={density === 'compact'}
              onClick={() => setDensity('compact')}
              title="Fit more entities on screen (D)"
            >
              Compact
            </button>
          </span>
        </div>
      </div>

      {tracksQuery.isError ? (
        <div className="p-6">
          <ErrorState error={tracksQuery.error} onRetry={() => void tracksQuery.refetch()} />
        </div>
      ) : (
        <div className="aegis-board-scroll">
          <table className="aegis-board" data-density={density}>
            <colgroup>
              <col className="g" />
              {columns.map((column) => (
                <col className="c" key={column.key} />
              ))}
            </colgroup>
            <thead>
              <tr>
                <th className="gutter">
                  <span className="rlabel">Entities</span>
                </th>
                {columns.map((column) => (
                  <th className="aegis-ent ent-col" data-state={column.state} key={column.key} scope="col">
                    <button type="button" title={`Inspect ${column.label} on the map`}>
                      <span className="inner">
                        <span className="id">{column.label}</span>
                        <span className="ty">{column.subtitle}</span>
                        <span className="st">{column.status}</span>
                        <span className="act">{column.act}</span>
                      </span>
                    </button>
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {metrics.map((metric) => (
                <tr key={metric.key}>
                  <th className="gutter" scope="row">
                    <span className="rlabel">{metric.label}</span>
                    {metric.sub ? <span className="rsub">{metric.sub}</span> : null}
                  </th>
                  {columns.map((column) => {
                    const value = metric.value(column)
                    if (!value) return <td className="cell na" key={column.key}>n/a</td>
                    return (
                      <td
                        className={`cell ${value.kind ?? ''}`}
                        data-sev={value.sev}
                        key={column.key}
                      >
                        {value.text}
                      </td>
                    )
                  })}
                </tr>
              ))}
              {columns.length === 0 ? (
                <tr>
                  <td className="aegis-empty" colSpan={columns.length + 1}>
                    No entities reported yet.
                  </td>
                </tr>
              ) : null}
            </tbody>
            <tfoot>
              <tr>
                <td className="gutter">
                  <span className="rlabel">Sources</span>
                </td>
                <td colSpan={Math.max(1, columns.length)}>
                  <div className="aegis-srcstrip">
                    {sources.length > 0 ? (
                      sources.map((source) => (
                        <span className="src" key={source.id}>
                          {source.name} · {source.type}
                        </span>
                      ))
                    ) : (
                      <span className="src none">none reported</span>
                    )}
                  </div>
                </td>
              </tr>
            </tfoot>
          </table>
        </div>
      )}
    </section>
  )
}