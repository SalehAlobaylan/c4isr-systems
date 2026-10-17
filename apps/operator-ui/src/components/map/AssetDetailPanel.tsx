import { useQuery } from '@tanstack/react-query'

import { AlertTriangleIcon } from '@/components/icons'
import { ErrorState } from '@/components/shared/states'
import { AssetStatusBadge, CommandStateBadge, ConnectionBadge, FreshnessBadge, HealthBadge, KindBadge } from '@/components/shared/badges'
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
        <span className="block h-3 w-2/3 rounded" style={{ background: 'var(--emad-surface-raised)' }} />
      </div>
    )
  }

  return (
    <>
      <section className="aegis-panel">
        <h2>Asset state</h2>
        <p className="sub">Controlled units · telemetry and orders</p>
{!isActionable(asset.freshness) ? (
          <p className="mt-3 flex items-start gap-2 rounded-[4px] border border-[color:var(--emad-status-caution)] bg-[color:var(--emad-wash-caution)] px-3 py-2 text-[13px] leading-[18px] text-[color:var(--emad-status-caution)]">
            <AlertTriangleIcon className="mt-px size-4 shrink-0" />
            {freshnessWarning(asset.freshness, asset.lastSeenAt)}
          </p>
        ) : null}
        <dl className="aegis-kv">
          <dt>Status</dt>
          <dd className="word">
            <AssetStatusBadge value={asset.status} lastSeenAt={asset.lastSeenAt} />
          </dd>
          <dt>Freshness</dt>
          <dd>
            <FreshnessBadge freshness={asset.freshness} lastSeenAt={asset.lastSeenAt} />
          </dd>
          <dt>Link</dt>
          <dd className="word">
            <ConnectionBadge value={asset.connectionState || 'unknown'} lastSeenAt={asset.lastSeenAt} />
          </dd>
          <dt>Health</dt>
          <dd className="word">
            <HealthBadge value={asset.health || 'unknown'} />
          </dd>
          <dt>Name</dt>
          <dd>{asset.name}</dd>
          <dt>Type</dt>
          <dd>{asset.type}</dd>
          <dt>Position</dt>
          <dd>
            {formatCoord(asset.position)}
            {!isActionable(asset.freshness) ? (
              <span className="ml-1 text-[12px] uppercase text-[color:var(--emad-text-tertiary)]">
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
              <KindBadge key={capability} value={capability} />
            ))}
          </div>
        ) : null}
      </section>

      <section className="aegis-panel">
        <h2>Telemetry history ({telemetryQuery.data?.total ?? 0})</h2>
        <p className="sub">Latest samples first</p>
        {telemetryQuery.isLoading ? (
          <span className="block h-3 w-2/3 rounded" style={{ background: 'var(--emad-surface-raised)' }} />
        ) : (telemetryQuery.data?.items?.length ?? 0) === 0 ? (
          <p className="aegis-empty">No telemetry samples received.</p>
        ) : (
          <ul className="m-0 flex list-none flex-col gap-2 p-0">
            {telemetryQuery.data?.items.map((sample) => (
              <li
                key={sample.id ?? sample.observedAt}
                className="flex items-center justify-between gap-3 border-b border-[color:var(--emad-border-subtle-soft)] py-2 last:border-b-0"
              >
                <span className="mono text-[12px] muted">{formatTimestamp(sample.observedAt)}</span>
                <span className="mono text-[12px] faint">{formatCoord(sample.position)}</span>
                <span className="text-[12px] tracking-[.08em] uppercase muted">
                  {sample.connectionState || sample.health || '—'}
                  {sample.freshness && !isActionable(sample.freshness) ? (
                    <span className="ml-1 text-[color:var(--emad-status-caution)]">
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
        {(commandsQuery.data?.items?.length ?? 0) === 0 ? (
          <p className="aegis-empty">No commands issued to this asset.</p>
        ) : (
          <ul className="m-0 flex list-none flex-col gap-2 p-0">
            {commandsQuery.data?.items.map((command) => (
              <li key={command.id} className="flex items-center justify-between gap-3">
                <span className="mono text-[13px]">{command.type}</span>
                <CommandStateBadge value={command.state} />
              </li>
            ))}
          </ul>
        )}
      </section>

      <p className="mono m-0 px-4 py-3 text-[12px] faint">Updated {formatRelative(asset.updatedAt)}</p>
    </>
  )
}