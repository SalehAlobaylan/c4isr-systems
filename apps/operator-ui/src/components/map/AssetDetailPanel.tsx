import { useQuery } from '@tanstack/react-query'

import { ErrorState } from '@/components/shared/states'
import { api } from '@/lib/api'
import {
  formatCoord,
  formatHeading,
  formatRelative,
  formatSpeed,
  formatTimestamp,
} from '@/lib/format'
import { queryKeys } from '@/lib/queryKeys'

export function AssetDetailPanel({ assetId }: { assetId: string }) {
  const assetQuery = useQuery({
    queryKey: queryKeys.assets.detail(assetId),
    queryFn: () => api.getAsset(assetId),
    enabled: Boolean(assetId),
    refetchInterval: 15_000,
  })
  const telemetryQuery = useQuery({
    queryKey: queryKeys.assets.telemetry(assetId),
    queryFn: () => api.listAssetTelemetry(assetId, { limit: 30 }),
    enabled: Boolean(assetId),
  })
  const commandsQuery = useQuery({
    queryKey: queryKeys.commands.list({ asset_id: assetId, limit: 10 }),
    queryFn: () => api.listCommands({ asset_id: assetId, limit: 10 }),
    enabled: Boolean(assetId),
  })

  const asset = assetQuery.data

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
        <dl className="aegis-kv">
          <dt>Status</dt>
          <dd className="word">{asset.status}</dd>
          <dt>Link</dt>
          <dd className="word">{asset.connectionState}</dd>
          <dt>Health</dt>
          <dd className="word">{asset.health || 'unknown'}</dd>
          <dt>Name</dt>
          <dd>{asset.name}</dd>
          <dt>Type</dt>
          <dd>{asset.type}</dd>
          <dt>Position</dt>
          <dd>{formatCoord(asset.position)}</dd>
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
                  {sample.stale ? ' · stale' : ''}
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