import { useQuery } from '@tanstack/react-query'
import { useNavigate, useSearch } from '@tanstack/react-router'
import type { ColumnDef } from '@tanstack/react-table'
import { useMemo } from 'react'

import { Page } from '@/components/layout/Page'
import { StateBadge } from '@/components/shared/badges'
import { DataTable } from '@/components/shared/data-table'
import { ErrorState } from '@/components/shared/states'
import { Select } from '@/components/ui/select'
import { api, type Asset } from '@/lib/api'
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
        cell: ({ row }) => <StateBadge value={row.original.status} />,
      },
      {
        accessorKey: 'connectionState',
        header: 'Connection',
        cell: ({ row }) => <StateBadge value={row.original.connectionState || 'unknown'} />,
      },
      {
        id: 'health',
        header: 'Health',
        cell: ({ row }) => (
          <span className="font-mono text-xs text-ink-muted">{row.original.health || '—'}</span>
        ),
      },
      {
        id: 'position',
        header: 'Position',
        enableSorting: false,
        cell: ({ row }) => (
          <span className="font-mono text-xs text-ink-muted">
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
          <div className="overflow-hidden rounded-lg border border-edge bg-panel">
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
