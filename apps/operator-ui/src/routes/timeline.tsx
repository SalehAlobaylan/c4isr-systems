import { useQuery } from '@tanstack/react-query'
import { useNavigate, useSearch } from '@tanstack/react-router'
import type { ColumnDef } from '@tanstack/react-table'
import { useMemo } from 'react'

import { RefreshIcon } from '@/components/icons'
import { Page } from '@/components/layout/Page'
import { ActorBadge } from '@/components/shared/badges'
import { DataTable } from '@/components/shared/data-table'
import { ErrorState } from '@/components/shared/states'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select } from '@/components/ui/select'
import { api, type AuditEntry } from '@/lib/api'
import { formatRelative, formatTimestamp, truncateId } from '@/lib/format'
import { queryKeys } from '@/lib/queryKeys'

const SUBJECT_OPTIONS = [
  { value: 'all', label: 'All subjects' },
  { value: 'observation', label: 'Observation' },
  { value: 'track', label: 'Track' },
  { value: 'asset', label: 'Asset' },
  { value: 'alert', label: 'Alert' },
  { value: 'incident', label: 'Incident' },
  { value: 'mission', label: 'Mission' },
  { value: 'command', label: 'Command' },
  { value: 'assessment', label: 'Assessment' },
  { value: 'geofence', label: 'Geofence' },
  { value: 'source', label: 'Source' },
]

const AUDIT_FILTERS = { limit: 200 }

export function TimelinePage() {
  const search = useSearch({ from: '/timeline' })
  const navigate = useNavigate()

  const filters = {
    limit: AUDIT_FILTERS.limit,
    subject_type: search.subject_type,
  }

  const auditQuery = useQuery({
    queryKey: queryKeys.audit.list(filters),
    queryFn: () => api.listAudit(filters),
  })

  const entries = useMemo(() => {
    const items = auditQuery.data?.items ?? []
    const needle = search.action?.trim().toLowerCase()
    if (!needle) return items
    return items.filter((entry) => entry.action.toLowerCase().includes(needle))
  }, [auditQuery.data, search.action])

  const columns = useMemo<Array<ColumnDef<AuditEntry, any>>>(
    () => [
      {
        accessorKey: 'occurredAt',
        header: 'Occurred',
        cell: ({ row }) => (
          <span className="text-xs text-ink-muted" title={formatTimestamp(row.original.occurredAt)}>
            {formatRelative(row.original.occurredAt)}
          </span>
        ),
      },
      {
        accessorKey: 'actorType',
        header: 'Actor',
        cell: ({ row }) => (
          <div className="flex items-center gap-2">
            <ActorBadge value={row.original.actorType} />
            <span className="font-mono text-[10px] text-ink-faint">
              {row.original.actorId || '—'}
            </span>
          </div>
        ),
      },
      {
        accessorKey: 'action',
        header: 'Action',
        cell: ({ row }) => (
          <span className="font-mono text-xs text-accent">{row.original.action}</span>
        ),
      },
      {
        accessorKey: 'subjectType',
        header: 'Subject',
        cell: ({ row }) => (
          <span className="font-mono text-[10px] text-ink-muted">
            {row.original.subjectType}
            {row.original.subjectId ? ` · ${truncateId(row.original.subjectId, 24)}` : ''}
          </span>
        ),
      },
      {
        accessorKey: 'correlationId',
        header: 'Correlation',
        cell: ({ row }) => (
          <span className="font-mono text-[10px] text-ink-faint">
            {row.original.correlationId || '—'}
          </span>
        ),
      },
    ],
    [],
  )

  return (
    <Page
      title="Timeline"
      description="Audit event stream across the operational picture"
      actions={
        <div className="flex items-center gap-2">
          <Input
            value={search.action ?? ''}
            onChange={(event) =>
              void navigate({
                to: '/timeline',
                search: {
                  subject_type: search.subject_type,
                  action: event.target.value || undefined,
                },
                replace: true,
              })
            }
            placeholder="Filter action contains…"
            className="w-56"
          />
          <Select
            ariaLabel="Filter by subject type"
            className="w-44"
            value={search.subject_type ?? 'all'}
            onValueChange={(value) =>
              void navigate({
                to: '/timeline',
                search: {
                  action: search.action,
                  subject_type: value === 'all' ? undefined : value,
                },
              })
            }
            options={SUBJECT_OPTIONS}
          />
          <Button
            variant="outline"
            size="sm"
            onClick={() => void auditQuery.refetch()}
            title="Refresh"
          >
            <RefreshIcon className="size-3.5" />
          </Button>
        </div>
      }
    >
      <div className="p-4">
        {auditQuery.isError ? (
          <ErrorState error={auditQuery.error} onRetry={() => void auditQuery.refetch()} />
        ) : (
          <div className="overflow-hidden rounded-lg border border-edge bg-panel">
            <DataTable
              columns={columns}
              data={entries}
              isLoading={auditQuery.isLoading}
              emptyMessage="No audit events match the current filter."
              initialSorting={[{ id: 'occurredAt', desc: true }]}
              getRowId={(entry) => entry.id}
            />
          </div>
        )}
      </div>
    </Page>
  )
}
