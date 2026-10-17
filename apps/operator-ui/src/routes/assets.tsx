import { useQuery } from '@tanstack/react-query'
import { useNavigate, useSearch } from '@tanstack/react-router'
import type { ColumnDef } from '@tanstack/react-table'
import { useMemo } from 'react'

import { Page } from '@/components/layout/Page'
import { AssetStatusBadge, ConnectionBadge, FreshnessBadge, HealthBadge } from '@/components/shared/badges'
import { DataTable } from '@/components/shared/data-table'
import { ErrorState } from '@/components/shared/states'
import { Select } from '@/components/ui/select'
import { api, type Asset } from '@/lib/api'
import { isActionable } from '@/lib/freshness'
import { formatCoord, formatRelative, formatTimestamp } from '@/lib/format'
import { queryKeys } from '@/lib/queryKeys'

const ASSET_FILTERS = { limit: 200 }

const STATUS_OPTIONS = [
  { value: 'all', label: 'All statuses' },
  { value: 'available', label: 'Available' },
  { value: 'assigned', label: 'Assigned' },
  { value: 'unavailable', label: 'Unavailable' },
  { value: 'offline', label: 'Offline' },
  { value: 'maintenance', label: 'Maintenance' },
]

export function AssetsPage() {
  const search = useSearch({ from: '/assets' })
  const navigate = useNavigate()

  const assetsQuery = useQuery({
    queryKey: queryKeys.assets.list(ASSET_FILTERS),
    queryFn: () => api.listAssets(ASSET_FILTERS),
  })

  const assets = useMemo(() => {
    const items = assetsQuery.data?.items ?? []
    if (!search.status) return items
    return items.filter((asset) => asset.status === search.status)
  }, [assetsQuery.data, search.status])

  const columns = useMemo<Array<ColumnDef<Asset, any>>>(
    () => [
      {
        accessorKey: 'id',
        header: 'Asset ID',
        cell: ({ row }) => (
          <span className="font-mono text-xs text-ink-muted" title={row.original.id}>
            {row.original.id}
          </span>
        ),
      },
      {
        accessorKey: 'name',
        header: 'Name',
        cell: ({ row }) => <span className="text-xs text-ink">{row.original.name}</span>,
      },
      {
        accessorKey: 'type',
        header: 'Type',
        cell: ({ row }) => (
          <span className="font-mono text-xs text-ink-muted">{row.original.type}</span>
        ),
      },
      {
        accessorKey: 'status',
        header: 'Status',
        cell: ({ row }) => <AssetStatusBadge value={row.original.status} lastSeenAt={row.original.lastSeenAt} />,
      },
      {
        accessorKey: 'connectionState',
        header: 'Connection',
        cell: ({ row }) => <ConnectionBadge value={row.original.connectionState || 'unknown'} lastSeenAt={row.original.lastSeenAt} />,
      },
      {
        id: 'freshness',
        header: 'Freshness',
        accessorFn: (asset) => asset.freshness ?? 'unknown',
        cell: ({ row }) => (
          <FreshnessBadge
            freshness={row.original.freshness}
            lastSeenAt={row.original.lastSeenAt}
          />
        ),
      },
      {
        id: 'health',
        header: 'Health',
        cell: ({ row }) => <HealthBadge value={row.original.health || 'unknown'} />,
      },
      {
        id: 'position',
        header: 'Position',
        enableSorting: false,
        cell: ({ row }) => (
          <span
            className="font-mono text-xs text-ink-muted"
            // A position from a stale feed is still the last known position,
            // but presenting it identically to a current one invites acting on
            // where something was rather than where it is.
            title={isActionable(row.original.freshness) ? undefined : 'Last known position'}
          >
            {formatCoord(row.original.position)}
          </span>
        ),
      },
      {
        id: 'lastSeenAt',
        header: 'Last seen',
        accessorFn: (asset) => asset.lastSeenAt ?? '',
        cell: ({ row }) => (
          <span className="text-xs text-ink-muted" title={formatTimestamp(row.original.lastSeenAt)}>
            {formatRelative(row.original.lastSeenAt)}
          </span>
        ),
      },
    ],
    [],
  )

  return (
    <Page
      title="Assets"
      description="Controlled platforms and their connectivity state"
      actions={
        <Select
          ariaLabel="Filter by status"
          className="w-44"
          value={search.status ?? 'all'}
          onValueChange={(value) =>
            void navigate({
              to: '/assets',
              search: { status: value === 'all' ? undefined : value },
            })
          }
          options={STATUS_OPTIONS}
        />
      }
    >
      <div className="p-4">
        {assetsQuery.isError ? (
          <ErrorState error={assetsQuery.error} onRetry={() => void assetsQuery.refetch()} />
        ) : (
          <div className="overflow-x-auto">
            <DataTable
              columns={columns}
              data={assets}
              isLoading={assetsQuery.isLoading}
              emptyMessage="No assets match the current filter."
              initialSorting={[{ id: 'lastSeenAt', desc: true }]}
              onRowClick={(asset) =>
                void navigate({ to: '/map', search: { selected: `asset:${asset.id}` } })
              }
              getRowId={(asset) => asset.id}
            />
          </div>
        )}
      </div>
    </Page>
  )
}
