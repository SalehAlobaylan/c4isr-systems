import { useQuery } from '@tanstack/react-query'
import { useNavigate, useSearch } from '@tanstack/react-router'
import type { ColumnDef } from '@tanstack/react-table'
import { useMemo } from 'react'

import { Page } from '@/components/layout/Page'
import { DataTable } from '@/components/shared/data-table'
import { StateBadge } from '@/components/shared/badges'
import { ErrorState } from '@/components/shared/states'
import { Select } from '@/components/ui/select'
import { api, type Track } from '@/lib/api'
import { formatCoord, formatRelative, formatSpeed, formatTimestamp } from '@/lib/format'
import { queryKeys } from '@/lib/queryKeys'

const TRACK_FILTERS = { limit: 200 }

const STATUS_OPTIONS = [
  { value: 'all', label: 'All statuses' },
  { value: 'active', label: 'Active' },
  { value: 'lost', label: 'Lost' },
  { value: 'closed', label: 'Closed' },
]

export function TracksPage() {
  const search = useSearch({ from: '/tracks' })
  const navigate = useNavigate()

  const tracksQuery = useQuery({
    queryKey: queryKeys.tracks.list(TRACK_FILTERS),
    queryFn: () => api.listTracks(TRACK_FILTERS),
  })

  const tracks = useMemo(() => {
    const items = tracksQuery.data?.items ?? []
    if (!search.status) return items
    return items.filter((track) => track.status === search.status)
  }, [tracksQuery.data, search.status])

  const columns = useMemo<Array<ColumnDef<Track, any>>>(
    () => [
      {
        accessorKey: 'id',
        header: 'Track ID',
        cell: ({ row }) => (
          <span className="font-mono text-xs text-ink-muted" title={row.original.id}>
            {row.original.id}
          </span>
        ),
      },
      {
        accessorKey: 'externalRef',
        header: 'External ref',
        cell: ({ row }) => (
          <span className="font-mono text-xs text-ink">{row.original.externalRef || '—'}</span>
        ),
      },
      {
        accessorKey: 'status',
        header: 'Status',
        cell: ({ row }) => <StateBadge value={row.original.status} />,
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
        id: 'speed',
        header: 'Speed',
        accessorFn: (track) => track.speed ?? -1,
        cell: ({ row }) => <span className="text-xs">{formatSpeed(row.original.speed)}</span>,
      },
      {
        accessorKey: 'observationCount',
        header: 'Obs',
        cell: ({ row }) => (
          <span className="font-mono text-xs text-ink-muted">{row.original.observationCount}</span>
        ),
      },
      {
        accessorKey: 'lastSeenAt',
        header: 'Last seen',
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
      title="Tracks"
      description="Operational interpretations built from observations"
      actions={
        <div className="flex items-center gap-2">
          <Select
            ariaLabel="Filter by status"
            className="w-40"
            value={search.status ?? 'all'}
            onValueChange={(value) =>
              void navigate({
                to: '/tracks',
                search: { status: value === 'all' ? undefined : value },
              })
            }
            options={STATUS_OPTIONS}
          />
        </div>
      }
    >
      <div className="p-4">
        {tracksQuery.isError ? (
          <ErrorState error={tracksQuery.error} onRetry={() => void tracksQuery.refetch()} />
        ) : (
          <div className="overflow-hidden rounded-lg border border-edge bg-panel">
            <DataTable
              columns={columns}
              data={tracks}
              isLoading={tracksQuery.isLoading}
              emptyMessage="No tracks match the current filter."
              initialSorting={[{ id: 'lastSeenAt', desc: true }]}
              onRowClick={(track) =>
                void navigate({ to: '/map', search: { selected: `track:${track.id}` } })
              }
              getRowId={(track) => track.id}
            />
          </div>
        )}
      </div>
    </Page>
  )
}
