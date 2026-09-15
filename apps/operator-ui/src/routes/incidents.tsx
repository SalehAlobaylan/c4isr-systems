import { useQuery } from '@tanstack/react-query'
import { useNavigate, useSearch } from '@tanstack/react-router'
import type { ColumnDef } from '@tanstack/react-table'
import { useEffect, useMemo, useState } from 'react'

import { Page } from '@/components/layout/Page'
import { StateBadge } from '@/components/shared/badges'
import { DataTable } from '@/components/shared/data-table'
import { ErrorState } from '@/components/shared/states'
import { Button } from '@/components/ui/button'
import { Select } from '@/components/ui/select'
import { IncidentFormDialog } from '@/features/incidents/IncidentFormDialog'
import { PlusIcon } from '@/components/icons'
import { api, type Incident } from '@/lib/api'
import { formatRelative, formatTimestamp } from '@/lib/format'
import { queryKeys } from '@/lib/queryKeys'

const STATUS_OPTIONS = [
  { value: 'all', label: 'All statuses' },
  { value: 'OPEN', label: 'Open' },
  { value: 'ACKNOWLEDGED', label: 'Acknowledged' },
  { value: 'INVESTIGATING', label: 'Investigating' },
  { value: 'RESPONDING', label: 'Responding' },
  { value: 'RESOLVED', label: 'Resolved' },
  { value: 'CLOSED', label: 'Closed' },
]

export function IncidentsPage() {
  const search = useSearch({ from: '/incidents' })
  const navigate = useNavigate()
  const [dialogOpen, setDialogOpen] = useState(false)

  useEffect(() => {
    if (search.new) setDialogOpen(true)
  }, [search.new])

  const filters = { limit: 200, status: search.status }
  const incidentsQuery = useQuery({
    queryKey: queryKeys.incidents.list(filters),
    queryFn: () => api.listIncidents(filters),
  })

  const columns = useMemo<Array<ColumnDef<Incident, any>>>(
    () => [
      {
        accessorKey: 'id',
        header: 'Incident ID',
        cell: ({ row }) => (
          <span className="font-mono text-[11px] text-ink-muted" title={row.original.id}>
            {row.original.id}
          </span>
        ),
      },
      {
        accessorKey: 'title',
        header: 'Title',
        cell: ({ row }) => (
          <span className="block max-w-[360px] truncate text-xs text-ink" title={row.original.title}>
            {row.original.title}
          </span>
        ),
      },
      {
        accessorKey: 'status',
        header: 'Status',
        cell: ({ row }) => <StateBadge value={row.original.status} />,
      },
      {
        accessorKey: 'priority',
        header: 'Priority',
        cell: ({ row }) => <StateBadge value={row.original.priority} />,
      },
      {
        accessorKey: 'assignedOperator',
        header: 'Operator',
        cell: ({ row }) => (
          <span className="font-mono text-xs text-ink-muted">
            {row.original.assignedOperator || '—'}
          </span>
        ),
      },
      {
        accessorKey: 'createdAt',
        header: 'Created',
        cell: ({ row }) => (
          <span className="text-xs text-ink-muted" title={formatTimestamp(row.original.createdAt)}>
            {formatRelative(row.original.createdAt)}
          </span>
        ),
      },
      {
        accessorKey: 'updatedAt',
        header: 'Updated',
        cell: ({ row }) => (
          <span className="text-xs text-ink-muted" title={formatTimestamp(row.original.updatedAt)}>
            {formatRelative(row.original.updatedAt)}
          </span>
        ),
      },
    ],
    [],
  )

  const closeDialog = (open: boolean) => {
    setDialogOpen(open)
    if (!open && (search.new || search.alertId)) {
      void navigate({
        to: '/incidents',
        search: { status: search.status },
      })
    }
  }

  return (
    <Page
      title="Incidents"
      description="Coordinated response workspaces"
      actions={
        <div className="flex items-center gap-2">
          <Select
            ariaLabel="Filter by status"
            className="w-44"
            value={search.status ?? 'all'}
            onValueChange={(value) =>
              void navigate({
                to: '/incidents',
                search: { status: value === 'all' ? undefined : value },
              })
            }
            options={STATUS_OPTIONS}
          />
          <Button onClick={() => setDialogOpen(true)}>
            <PlusIcon className="size-3.5" />
            New incident
          </Button>
        </div>
      }
    >
      <div className="p-4">
        {incidentsQuery.isError ? (
          <ErrorState error={incidentsQuery.error} onRetry={() => void incidentsQuery.refetch()} />
        ) : (
          <div className="overflow-hidden rounded-lg border border-edge bg-panel">
            <DataTable
              columns={columns}
              data={incidentsQuery.data?.items ?? []}
              isLoading={incidentsQuery.isLoading}
              emptyMessage="No incidents match the current filter."
              initialSorting={[{ id: 'updatedAt', desc: true }]}
              onRowClick={(incident) =>
                void navigate({ to: '/incidents/$id', params: { id: incident.id } })
              }
              getRowId={(incident) => incident.id}
            />
          </div>
        )}
      </div>

      <IncidentFormDialog
        open={dialogOpen}
        onOpenChange={closeDialog}
        defaultAlertId={search.alertId}
        onCreated={(incident) => {
          closeDialog(false)
          void navigate({ to: '/incidents/$id', params: { id: incident.id } })
        }}
      />
    </Page>
  )
}
