import { useQuery } from '@tanstack/react-query'

import { ErrorState } from '@/components/shared/states'
import { FreshnessBadge } from '@/components/shared/badges'
import { api, type Asset } from '@/lib/api'
import { freshnessWarning, isActionable } from '@/lib/freshness'
import {
  formatCoord,
  formatHeading,
  formatRelative,
  formatSpeed,
  formatTimestamp,
} from '@/lib/format'
import { queryKeys } from '@/lib/queryKeys'
import { selectReplayAt, useScrubStore } from '@/stores/scrub'

export function AssetDetailPanel({ assetId }: { assetId: string }) {
  // While the operator is scrubbed back in time, the panel must describe that
  // moment. Fetching the current asset here would put a present-day position
  // next to a replayed map marker and produce a detail view that never existed.
  const replayAt = useScrubStore(selectReplayAt)
  const replayQuery = useQuery({
    queryKey: ['picture', replayAt ?? 'live'],
    queryFn: () =>
      replayAt ? api.getPicture({ at: replayAt }) : api.getPicture({ live: true }),
    enabled: Boolean(replayAt),
  })

  const assetQuery = useQuery({
    queryKey: queryKeys.assets.detail(assetId),
    queryFn: () => api.getAsset(assetId),
    enabled: Boolean(assetId) && !replayAt,
    refetchInterval: 15_000,
  })
  const telemetryQuery = useQuery({
    queryKey: queryKeys.assets.telemetry(assetId),
    queryFn: () => api.listAssetTelemetry(assetId, { limit: 30 }),
    // Suppressed during replay: telemetry history is present-day evidence and
    // does not belong beside a reconstructed picture.
    enabled: Boolean(assetId) && !replayAt,
  })
  const commandsQuery = useQuery({
    queryKey: queryKeys.commands.list({ asset_id: assetId, limit: 10 }),
    queryFn: () => api.listCommands({ asset_id: assetId, limit: 10 }),
    enabled: Boolean(assetId),
  })

  // A replayed asset is projected onto the same shape the panel already renders,
// so there is one rendering path rather than a live one and a replay one that
// could drift apart. Capabilities are absent from a reconstruction because they
// are registration facts, not observed state.
  const replayedRaw = replayAt ? replayQuery.data?.assets.find((a) => a.id === assetId) : undefined
  const replayed: Asset | undefined = replayedRaw
    ? {
        id: replayedRaw.id,
        name: replayedRaw.name,
        type: replayedRaw.type,
        status: replayedRaw.status,
        capabilities: [],
        metadata: {},
        position: replayedRaw.position ?? undefined,
        speed: replayedRaw.speed,
        heading: replayedRaw.heading,
        health: replayedRaw.health,
        connectionState: replayedRaw.connectionState,
        lastSeenAt: replayedRaw.observedAt,
        createdAt: replayedRaw.observedAt,
        updatedAt: replayedRaw.observedAt,
        freshness: replayedRaw.freshness,
        freshnessLabel: replayedRaw.freshnessLabel,
      }
    : undefined
  const asset = replayed ?? assetQuery.data

  if (assetQuery.isError) {
    return (
      <div className="p-4">
        <ErrorState error={assetQuery.error} onRetry={() => void assetQuery.refetch()} />
      </div>
    )
  }

  if (!asset) {
    return (
      <div className="aegis-panel" aria-hidden="true">
        <span className="block h-3 w-2/3 rounded" style={{ background: 'var(--surface-2)' }} />
      </div>
    )
  }

  return (
    <>
      <section className="aegis-panel">
        <h2>Asset state</h2>
        <p className="sub">Controlled units · telemetry and orders</p>
        {!isActionable(asset.freshness) ? (
          <p className="mt-2 border-l-2 border-[color:var(--st-warn)] bg-[color:var(--st-warn)]/10 px-2 py-1.5 text-[11px] leading-snug text-[color:var(--st-warn)]">
            {freshnessWarning(asset.freshness, asset.lastSeenAt)}
          </p>
        ) : null}
        <dl className="aegis-kv">
          <dt>Status</dt>
          <dd className="word">{asset.status}</dd>
          <dt>Freshness</dt>
          <dd>
            <FreshnessBadge freshness={asset.freshness} lastSeenAt={asset.lastSeenAt} />
          </dd>
          <dt>Link</dt>
          <dd className="word">{asset.connectionState}</dd>
          <dt>Health</dt>
          <dd className="word">{asset.health || 'unknown'}</dd>
          <dt>Name</dt>
          <dd>{asset.name}</dd>
          <dt>Type</dt>
          <dd>{asset.type}</dd>
          <dt>Position</dt>
          <dd>
            {formatCoord(asset.position)}
            {!isActionable(asset.freshness) ? (
              <span className="ml-1 text-[10px] uppercase text-[color:var(--faint)]">
                last known
              </span>
            ) : null}
          </dd>
          <dt>Speed</dt>
          <dd>{formatSpeed(asset.speed)}</dd>
          <dt>Heading</dt>
          <dd>{formatHeading(asset.heading)}</dd>
          <dt>Last seen</dt>
          <dd>{formatTimestamp(asset.lastSeenAt)}</dd>
        </dl>
        {asset.capabilities.length > 0 ? (
          <div className="mt-3 flex flex-wrap gap-1">
            {asset.capabilities.map((capability) => (
              <span className="aegis-src" key={capability}>
                {capability}
              </span>
            ))}
          </div>
        ) : null}
      </section>

      <section className="aegis-panel">
        <h2>Telemetry history ({telemetryQuery.data?.total ?? 0})</h2>
        <p className="sub">Latest samples first</p>
        {telemetryQuery.isLoading ? (
          <span className="block h-3 w-2/3 rounded" style={{ background: 'var(--surface-2)' }} />
        ) : (telemetryQuery.data?.items.length ?? 0) === 0 ? (
          <p className="aegis-empty">No telemetry samples received.</p>
        ) : (
          <ul className="m-0 flex list-none flex-col gap-2 p-0">
            {telemetryQuery.data?.items.map((sample) => (
              <li
                key={sample.id ?? sample.observedAt}
                className="flex items-center justify-between gap-3 border-b border-[color:var(--line)] py-2 last:border-b-0"
              >
                <span className="mono text-[10px] muted">{formatTimestamp(sample.observedAt)}</span>
                <span className="mono text-[10px] faint">{formatCoord(sample.position)}</span>
                <span className="text-[10px] tracking-[.12em] uppercase muted">
                  {sample.connectionState || sample.health || '—'}
                  {sample.freshness && !isActionable(sample.freshness) ? (
                    <span className="ml-1 text-[color:var(--st-warn)]">
                      {sample.freshnessLabel?.toLowerCase() ?? 'stale'}
                    </span>
                  ) : null}
                </span>
              </li>
            ))}
          </ul>
        )}
      </section>

      <section className="aegis-panel">
        <h2>Recent commands ({commandsQuery.data?.total ?? 0})</h2>
        <p className="sub">Human decision stays central</p>
        {(commandsQuery.data?.items.length ?? 0) === 0 ? (
          <p className="aegis-empty">No commands issued to this asset.</p>
        ) : (
          <ul className="m-0 flex list-none flex-col gap-2 p-0">
            {commandsQuery.data?.items.map((command) => (
              <li key={command.id} className="flex items-center justify-between gap-3">
                <span className="mono text-[11px]">{command.type}</span>
                <span className="aegis-sev">{command.state}</span>
              </li>
            ))}
          </ul>
        )}
      </section>

      <p className="mono m-0 px-4 py-3 text-[9px] faint">Updated {formatRelative(asset.updatedAt)}</p>
    </>
  )
}