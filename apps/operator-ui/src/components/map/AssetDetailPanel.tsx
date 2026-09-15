import { useQuery } from '@tanstack/react-query'

import { StateBadge } from '@/components/shared/badges'
import { DetailList, ErrorState, SectionTitle } from '@/components/shared/states'
import { Separator } from '@/components/ui/separator'
import { Skeleton } from '@/components/ui/skeleton'
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
      <div className="flex flex-col gap-3 p-4">
        <Skeleton className="h-4 w-2/3" />
        <Skeleton className="h-3 w-1/2" />
        <Skeleton className="h-24 w-full" />
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-4 p-4">
      <div className="flex items-center justify-between gap-2">
        <div className="flex items-center gap-2">
          <StateBadge value={asset.status} />
          <StateBadge value={asset.connectionState} />
        </div>
        <span className="font-mono text-[10px] text-ink-faint">{asset.id}</span>
      </div>

      <DetailList
        columns={2}
        items={[
          { label: 'Name', value: asset.name },
          { label: 'Type', value: asset.type },
          { label: 'Health', value: asset.health || 'unknown' },
          { label: 'Last seen', value: formatTimestamp(asset.lastSeenAt) },
          { label: 'Position', value: formatCoord(asset.position), mono: true },
          { label: 'Speed', value: formatSpeed(asset.speed) },
          { label: 'Heading', value: formatHeading(asset.heading) },
          { label: 'Updated', value: formatRelative(asset.updatedAt) },
        ]}
      />

      {asset.capabilities.length > 0 ? (
        <div className="flex flex-wrap gap-1">
          {asset.capabilities.map((capability) => (
            <span
              key={capability}
              className="rounded border border-edge-strong bg-panel-raised px-1.5 py-0.5 font-mono text-[10px] text-ink-muted"
            >
              {capability}
            </span>
          ))}
        </div>
      ) : null}

      <Separator />

      <section className="flex flex-col gap-2">
        <SectionTitle>Telemetry history ({telemetryQuery.data?.total ?? 0})</SectionTitle>
        {telemetryQuery.isLoading ? (
          <Skeleton className="h-10 w-full" />
        ) : (telemetryQuery.data?.items.length ?? 0) === 0 ? (
          <p className="text-xs text-ink-faint">No telemetry samples received.</p>
        ) : (
          <div className="max-h-72 overflow-y-auto rounded-md border border-edge">
            <table className="w-full text-left">
              <thead className="sticky top-0 bg-panel-raised">
                <tr className="text-[10px] tracking-wider text-ink-faint uppercase">
                  <th className="px-2 py-1.5 font-medium">Observed</th>
                  <th className="px-2 py-1.5 font-medium">Position</th>
                  <th className="px-2 py-1.5 font-medium">Link</th>
                </tr>
              </thead>
              <tbody>
                {telemetryQuery.data?.items.map((sample) => (
                  <tr key={sample.id ?? sample.observedAt} className="border-t border-edge/70">
                    <td className="px-2 py-1.5 font-mono text-[10px] text-ink-muted">
                      {formatTimestamp(sample.observedAt)}
                    </td>
                    <td className="px-2 py-1.5 font-mono text-[10px] text-ink-faint">
                      {formatCoord(sample.position)}
                    </td>
                    <td className="px-2 py-1.5">
                      <span className="font-mono text-[10px] text-ink-muted">
                        {sample.connectionState || sample.health || '—'}
                        {sample.stale ? ' · stale' : ''}
                      </span>
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
        <SectionTitle>Recent commands ({commandsQuery.data?.total ?? 0})</SectionTitle>
        {(commandsQuery.data?.items.length ?? 0) === 0 ? (
          <p className="text-xs text-ink-faint">No commands issued to this asset.</p>
        ) : (
          <ul className="flex flex-col gap-2">
            {commandsQuery.data?.items.map((command) => (
              <li
                key={command.id}
                className="flex items-center justify-between gap-2 rounded-md border border-edge px-3 py-2"
              >
                <div className="min-w-0">
                  <p className="font-mono text-xs text-ink">{command.type}</p>
                  <p className="font-mono text-[10px] text-ink-faint">
                    {formatTimestamp(command.createdAt)}
                  </p>
                </div>
                <StateBadge value={command.state} />
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  )
}
