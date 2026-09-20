import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useNavigate, useSearch } from '@tanstack/react-router'
import type { ColumnDef } from '@tanstack/react-table'
import { useMemo } from 'react'

import { Page } from '@/components/layout/Page'
import { DataTable } from '@/components/shared/data-table'
import { SeverityBadge, StateBadge } from '@/components/shared/badges'
import { ErrorState } from '@/components/shared/states'
import { Button } from '@/components/ui/button'
import { Select } from '@/components/ui/select'
import { api, type Alert } from '@/lib/api'
import { formatRelative, formatTimestamp, truncateId } from '@/lib/format'
import { queryKeys } from '@/lib/queryKeys'
import { toast } from '@/stores/toasts'

const STATE_OPTIONS = [
  { value: 'all', label: 'All states' },
  { value: 'ACTIVE', label: 'Active' },
  { value: 'ACKNOWLEDGED', label: 'Acknowledged' },
  { value: 'RESOLVED', label: 'Resolved' },
]

const SEVERITY_OPTIONS = [
  { value: 'all', label: 'All severities' },
  { value: 'critical', label: 'Critical' },
  { value: 'high', label: 'High' },
  { value: 'medium', label: 'Medium' },
  { value: 'low', label: 'Low' },
]

function sourceReferenceValue(alert: Alert, key: string): string | undefined {
  const value = alert.sourceReference?.[key]
  return typeof value === 'string' && value.trim() ? value : undefined
}

function alertProvenance(alert: Alert) {
  const geofenceId = alert.geofenceId || sourceReferenceValue(alert, 'geofenceId') || ''
  const qualifiedGeofenceMarker = '__geofence__'
  const qualifiedGeofenceIndex = geofenceId.indexOf(qualifiedGeofenceMarker)
  const inferredRunId =
    qualifiedGeofenceIndex > 0 ? geofenceId.slice(0, qualifiedGeofenceIndex) : undefined
  const runId = sourceReferenceValue(alert, 'scenarioRunId') || inferredRunId
  const geofenceName = sourceReferenceValue(alert, 'geofenceName')
  return {
    label: runId ? `Scenario run ${truncateId(runId, 18)}` : 'External rule',
    detail: geofenceName || (geofenceId ? `geofence ${truncateId(geofenceId, 18)}` : 'rule source unavailable'),
    title: [runId ? `scenario run ${runId}` : 'external rule', geofenceId ? `geofence ${geofenceId}` : ''].filter(Boolean).join(' · '),
  }
}

export function AlertsPage() {
  const search = useSearch({ from: '/alerts' })
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const filters = {
    limit: 200,
    state: search.state,
    severity: search.severity,
  }

  const alertsQuery = useQuery({
    queryKey: queryKeys.alerts.list(filters),
    queryFn: () => api.listAlerts(filters),
  })

  const invalidateAlerts = () => queryClient.invalidateQueries({ queryKey: queryKeys.alerts.all })

  const acknowledgeMutation = useMutation({
    mutationFn: (id: string) => api.acknowledgeAlert(id),
    onSuccess: (alert) => {
      toast({ title: 'Alert acknowledged', description: alert.title, variant: 'success' })
      void invalidateAlerts()
    },
    onError: (error) => {
      toast({
        title: 'Acknowledge failed',
        description: error instanceof Error ? error.message : undefined,
        variant: 'error',
      })
    },
  })

  const resolveMutation = useMutation({
    mutationFn: (id: string) => api.resolveAlert(id),
    onSuccess: (alert) => {
      toast({ title: 'Alert resolved', description: alert.title, variant: 'success' })
      void invalidateAlerts()
    },
    onError: (error) => {
      toast({
        title: 'Resolve failed',
        description: error instanceof Error ? error.message : undefined,
        variant: 'error',
      })
    },
  })

  const columns = useMemo<Array<ColumnDef<Alert, any>>>(
    () => [
      {
        accessorKey: 'severity',
        header: 'Severity',
        cell: ({ row }) => <SeverityBadge value={row.original.severity} />,
      },
      {
        accessorKey: 'state',
        header: 'State',
        cell: ({ row }) => <StateBadge value={row.original.state} />,
      },
      {
        accessorKey: 'title',
        header: 'Alert',
        cell: ({ row }) => (
          <div className="max-w-[380px] min-w-0">
            <p className="truncate text-xs text-ink" title={row.original.title}>
              {row.original.title}
            </p>
            <p className="mt-0.5 truncate font-mono text-[10px] text-ink-faint">
              {row.original.type} · {row.original.message}
            </p>
          </div>
        ),
      },
      {
        id: 'provenance',
        header: 'Provenance',
        enableSorting: false,
        cell: ({ row }) => {
          const provenance = alertProvenance(row.original)
          return (
            <div className="max-w-[220px] min-w-0" title={provenance.title}>
              <p className="truncate text-xs text-ink">{provenance.label}</p>
              <p className="mt-0.5 truncate font-mono text-[10px] text-ink-faint">
                {provenance.detail}
              </p>
            </div>
          )
        },
      },
      {
        id: 'entities',
        header: 'Related',
        enableSorting: false,
        cell: ({ row }) => (
          <div className="flex items-center gap-1.5">
            {row.original.trackId ? (
              <Button
                variant="outline"
                size="sm"
                className="h-6 px-2 font-mono text-[10px]"
                onClick={(event) => {
                  event.stopPropagation()
                  void navigate({
                    to: '/map',
                    search: { selected: `track:${row.original.trackId}` },
                  })
                }}
              >
                TRACK
              </Button>
            ) : null}
            {row.original.incidentId ? (
              <Button
                variant="outline"
                size="sm"
                className="h-6 px-2 font-mono text-[10px]"
                onClick={(event) => {
                  event.stopPropagation()
                  void navigate({
                    to: '/incidents/$id',
                    params: { id: row.original.incidentId },
                  })
                }}
              >
                INCIDENT
              </Button>
            ) : null}
            {!row.original.trackId && !row.original.incidentId ? (
              <span className="text-[10px] text-ink-faint">—</span>
            ) : null}
          </div>
        ),
      },
      {
        accessorKey: 'createdAt',
        header: 'Raised',
        cell: ({ row }) => (
          <span className="text-xs text-ink-muted" title={formatTimestamp(row.original.createdAt)}>
            {formatRelative(row.original.createdAt)}
          </span>
        ),
      },
      {
        id: 'actions',
        header: 'Actions',
        enableSorting: false,
        cell: ({ row }) => {
          const alert = row.original
          const resolved = alert.state === 'RESOLVED'
          return (
            <div className="flex items-center gap-1.5">
              <Button
                variant="secondary"
                size="sm"
                disabled={resolved || alert.state === 'ACKNOWLEDGED'}
                onClick={(event) => {
                  event.stopPropagation()
                  acknowledgeMutation.mutate(alert.id)
                }}
              >
                Ack
              </Button>
              <Button
                variant="success"
                size="sm"
                disabled={resolved}
                onClick={(event) => {
                  event.stopPropagation()
                  resolveMutation.mutate(alert.id)
                }}
              >
                Resolve
              </Button>
              <Button
                variant="default"
                size="sm"
                disabled={Boolean(alert.incidentId)}
                onClick={(event) => {
                  event.stopPropagation()
                  void navigate({
                    to: '/incidents',
                    search: { new: true, alertId: alert.id },
                  })
                }}
              >
                Create incident
              </Button>
            </div>
          )
        },
      },
    ],
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [acknowledgeMutation, resolveMutation, navigate],
  )

  return (
    <Page
      title="Alert Center"
      description="Geofence breaches and rule-generated attention items"
      actions={
        <div className="flex items-center gap-2">
          <Select
            ariaLabel="Filter by state"
            className="w-40"
            value={search.state ?? 'all'}
            onValueChange={(value) =>
              void navigate({
                to: '/alerts',
                search: {
                  state: value === 'all' ? undefined : value,
                  severity: search.severity,
                },
              })
            }
            options={STATE_OPTIONS}
          />
          <Select
            ariaLabel="Filter by severity"
            className="w-40"
            value={search.severity ?? 'all'}
            onValueChange={(value) =>
              void navigate({
                to: '/alerts',
                search: {
                  state: search.state,
                  severity: value === 'all' ? undefined : value,
                },
              })
            }
            options={SEVERITY_OPTIONS}
          />
        </div>
      }
    >
      <div className="p-4">
        {alertsQuery.isError ? (
          <ErrorState error={alertsQuery.error} onRetry={() => void alertsQuery.refetch()} />
        ) : (
          <div className="overflow-hidden rounded-lg border border-edge bg-panel">
            <DataTable
              columns={columns}
              data={alertsQuery.data?.items ?? []}
              isLoading={alertsQuery.isLoading}
              emptyMessage="No alerts match the current filter."
              initialSorting={[{ id: 'createdAt', desc: true }]}
              getRowId={(alert) => alert.id}
            />
          </div>
        )}
      </div>
    </Page>
  )
}
