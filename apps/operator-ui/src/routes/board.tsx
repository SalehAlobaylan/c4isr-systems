import { useQuery } from '@tanstack/react-query'
import { useEffect, useMemo, useState } from 'react'

import { ErrorState } from '@/components/shared/states'
import { api, type Asset, type Track } from '@/lib/api'
import {
  FRESHNESS_LABELS,
  freshnessWarning,
  isActionable,
  toFreshness,
  type Freshness,
} from '@/lib/freshness'
import { formatCoord, formatHeading, formatRelative, truncateId } from '@/lib/format'
import { queryKeys } from '@/lib/queryKeys'
import { useLayoutStore } from '@/stores/layout'

import '@/components/board/board.css'

const TRACK_FILTERS = { limit: 500 }
const ASSET_FILTERS = { limit: 500 }
const OBSERVATION_FILTERS = { limit: 2000 }
const ALERT_FILTERS = { limit: 500 }
const SOURCE_FILTERS = { limit: 100 }

const EMPTY: never[] = []

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

function columnFreshness(column: Column): Freshness {
  return toFreshness(column.track?.freshness ?? column.asset?.freshness)
}

/** Scenario runs namespace their entities, so the run id disambiguates repeats. */
function runToken(id: string): string {
  const namespace = id.includes('__') ? id.split('__')[0] : ''
  return namespace ? namespace.slice(0, 11) : ''
}

/** Appends the run token only when a label would otherwise repeat on the board. */
function disambiguate(labels: Map<string, number>, label: string, subtitle: string, id: string): string {
  if ((labels.get(label) ?? 0) < 2) return subtitle
  const token = runToken(id)
  return token ? `${subtitle} · ${token}` : subtitle
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
  const [selectedSources, setSelectedSources] = useState<string[]>([])
  const [hiddenAssets, setHiddenAssets] = useState<string[]>([])

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.metaKey || event.ctrlKey || event.altKey) return
      const target = event.target as HTMLElement | null
      const tag = target?.tagName.toLowerCase() ?? ''
      if (
        tag === 'input' ||
        tag === 'select' ||
        tag === 'textarea' ||
        target?.isContentEditable ||
        target?.closest('dialog, [role="dialog"]')
      ) {
        return
      }
      if (event.key.toLowerCase() === 'd') {
        event.preventDefault()
        setDensity(useLayoutStore.getState().density === 'compact' ? 'comfortable' : 'compact')
      }
    }
    document.addEventListener('keydown', onKeyDown)
    return () => document.removeEventListener('keydown', onKeyDown)
  }, [setDensity])

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

  const tracks = tracksQuery.data?.items ?? EMPTY
  const assets = assetsQuery.data?.items ?? EMPTY
  const observations = observationsQuery.data?.items ?? EMPTY
  const alerts = alertsQuery.data?.items ?? EMPTY
  const sources = sourcesQuery.data?.items ?? EMPTY

  const { observationsByTrack, alertsByTrack, alertsByAsset } = useMemo(() => {
    const byTrack = new Map<string, Set<string>>()
    for (const observation of observations) {
      const hint = observation.trackHint
      if (!hint) continue
      const set = byTrack.get(hint) ?? new Set<string>()
      set.add(observation.sourceId)
      byTrack.set(hint, set)
    }

    const alertByTrack = new Map<string, number>()
    const alertByAsset = new Map<string, number>()
    for (const alert of alerts) {
      if (alert.trackId) alertByTrack.set(alert.trackId, (alertByTrack.get(alert.trackId) ?? 0) + 1)
      if (alert.assetId) alertByAsset.set(alert.assetId, (alertByAsset.get(alert.assetId) ?? 0) + 1)
    }

    return { observationsByTrack: byTrack, alertsByTrack: alertByTrack, alertsByAsset: alertByAsset }
  }, [observations, alerts])

  const columns: Column[] = useMemo(() => {
    const assetColumns = assets
      .filter((asset) => !hiddenAssets.includes(asset.id))
      .map((asset) => ({
        key: `asset:${asset.id}`,
        kind: 'asset' as const,
        id: asset.id,
        label: asset.name || truncateId(asset.id, 10),
        subtitle: asset.type || 'Asset',
        state: asset.health === 'critical' ? 'crit' : (ASSET_STATE[asset.status] ?? 'unknown'),
        status: asset.status,
        statusSeverity:
          asset.health === 'critical'
            ? ('crit' as const)
            : asset.status === 'available'
              ? undefined
              : ('warn' as const),
        act: formatRelative(asset.lastSeenAt),
        asset,
        observations: 0,
        sources: 0,
        alerts: alertsByAsset.get(asset.id) ?? 0,
      }))

    const trackColumns = tracks
      .filter((track) => {
        if (selectedSources.length === 0) return true
        const observed = observationsByTrack.get(track.externalRef)
        return observed ? selectedSources.some((sourceId) => observed.has(sourceId)) : false
      })
      .map((track) => ({
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
        sources: observationsByTrack.get(track.externalRef)?.size ?? 0,
        alerts: alertsByTrack.get(track.id) ?? 0,
      }))

    const all: Column[] = [...trackColumns, ...assetColumns]
    const labelCounts = new Map<string, number>()
    for (const column of all) labelCounts.set(column.label, (labelCounts.get(column.label) ?? 0) + 1)
    return all.map((column) => ({
      ...column,
      subtitle: disambiguate(
        labelCounts,
        column.label,
        column.subtitle,
        column.track?.externalRef ?? column.id,
      ),
    }))
  }, [tracks, assets, observationsByTrack, alertsByTrack, alertsByAsset, selectedSources, hiddenAssets])

  const metrics: Metric[] = [
    {
      key: 'freshness',
      label: 'Fresh',
      // Administrative status answers "is it fit to task"; freshness answers
      // "is what I am looking at still true". An available asset that stopped
      // reporting is the exact case the board previously showed as healthy.
      value: (column) => {
        const state = columnFreshness(column)
        return {
          text: FRESHNESS_LABELS[state].toUpperCase(),
          kind: 'word',
          sev: isActionable(state)
            ? undefined
            : state === 'unavailable' || state === 'stale'
              ? ('crit' as const)
              : ('warn' as const),
        }
      },
    },
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
            {selectedSources.length > 0 || hiddenAssets.length > 0 ? ' · FILTERED' : ''}
          </p>
        </div>

        {chipsOpen ? (
          <div className="aegis-chips-wrap" id="pane-chips">
            <div className="aegis-chips" role="group" aria-label="Filter tracks by reporting source">
              <span className="cap faint self-center pr-1">Sources</span>
              {sources.map((source) => {
                const active = selectedSources.includes(source.id)
                return (
                  <button
                    key={source.id}
                    type="button"
                    className="aegis-chip"
                    aria-pressed={active}
                    onClick={() =>
                      setSelectedSources((current) =>
                        active ? current.filter((id) => id !== source.id) : [...current, source.id],
                      )
                    }
                    title={`Show tracks observed by ${source.name}`}
                  >
                    <span className="n">{source.name}</span>
                    <span className="t">{runToken(source.id) || source.type}</span>
                  </button>
                )
              })}
              {sources.length === 0 ? <p className="aegis-empty m-0">No sources reported yet.</p> : null}
            </div>
            <div className="aegis-chips" role="group" aria-label="Show or hide asset columns">
              <span className="cap faint self-center pr-1">Assets</span>
              {assets.map((asset) => {
                const hiddenColumn = hiddenAssets.includes(asset.id)
                return (
                  <button
                    key={asset.id}
                    type="button"
                    className="aegis-chip"
                    aria-pressed={!hiddenColumn}
                    onClick={() =>
                      setHiddenAssets((current) =>
                        hiddenColumn
                          ? current.filter((id) => id !== asset.id)
                          : [...current, asset.id],
                      )
                    }
                    title={`${hiddenColumn ? 'Show' : 'Hide'} ${asset.name}`}
                  >
                    <span className="n">{asset.name || truncateId(asset.id, 10)}</span>
                    <span className="t">
                      {runToken(asset.id) ? `${runToken(asset.id)} · ` : ''}
                      {asset.status}
                    </span>
                  </button>
                )
              })}
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
                  <th
                    className="aegis-ent ent-col"
                    data-state={column.state}
                    data-freshness={columnFreshness(column)}
                    key={column.key}
                    scope="col"
                  >
                    <button
                      type="button"
                      title={
                        isActionable(columnFreshness(column))
                          ? `Inspect ${column.label} on the map`
                          : `Inspect ${column.label} on the map — ${freshnessWarning(columnFreshness(column), column.track?.lastSeenAt ?? column.asset?.lastSeenAt)}`
                      }
                    >
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