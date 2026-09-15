import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate, useParams } from '@tanstack/react-router'
import type { ColumnDef } from '@tanstack/react-table'
import { useMemo, useState } from 'react'

import {
  ArrowLeftIcon,
  ExternalLinkIcon,
  PlusIcon,
} from '@/components/icons'
import { Page } from '@/components/layout/Page'
import { ActorBadge, SeverityBadge, StateBadge } from '@/components/shared/badges'
import { DataTable } from '@/components/shared/data-table'
import { DetailList, ErrorState, SectionTitle } from '@/components/shared/states'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsList, TabsPanel, TabsTab } from '@/components/ui/tabs'
import { AttachRelationDialog, type RelationKind } from '@/features/incidents/AttachRelationDialog'
import { CommandConsole } from '@/features/incidents/CommandConsole'
import { MissionFormDialog } from '@/features/incidents/MissionFormDialog'
import { api, type AuditEntry } from '@/lib/api'
import {
  formatConfidence,
  formatCoord,
  formatRelative,
  formatTimestamp,
  truncateId,
} from '@/lib/format'
import { queryKeys } from '@/lib/queryKeys'
import { toast } from '@/stores/toasts'

const INCIDENT_TRANSITIONS: Record<string, string[]> = {
  OPEN: ['ACKNOWLEDGED', 'INVESTIGATING', 'CLOSED'],
  ACKNOWLEDGED: ['INVESTIGATING', 'RESPONDING', 'CLOSED'],
  INVESTIGATING: ['RESPONDING', 'RESOLVED', 'CLOSED'],
  RESPONDING: ['INVESTIGATING', 'RESOLVED', 'CLOSED'],
  RESOLVED: ['INVESTIGATING', 'CLOSED'],
  CLOSED: [],
}

export function IncidentDetailPage() {
  const { id } = useParams({ from: '/incidents/$id' })
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [tab, setTab] = useState('overview')
  const [attachKind, setAttachKind] = useState<RelationKind | null>(null)
  const [missionOpen, setMissionOpen] = useState(false)

  const incidentQuery = useQuery({
    queryKey: queryKeys.incidents.detail(id),
    queryFn: () => api.getIncident(id),
    refetchInterval: 15_000,
  })

  const auditFilters = { subject_type: 'incident', subject_id: id, limit: 100 }
  const auditQuery = useQuery({
    queryKey: queryKeys.audit.list(auditFilters),
    queryFn: () => api.listAudit(auditFilters),
  })

  const statusMutation = useMutation({
    mutationFn: (status: string) => api.updateIncidentStatus(id, status),
    onSuccess: (incident) => {
      toast({ title: `Incident ${incident.status.toLowerCase()}`, variant: 'success' })
      void queryClient.invalidateQueries({ queryKey: queryKeys.incidents.all })
    },
    onError: (error) => {
      toast({
        title: 'Status update failed',
        description: error instanceof Error ? error.message : undefined,
        variant: 'error',
      })
    },
  })

  const auditColumns = useMemo<Array<ColumnDef<AuditEntry, any>>>(
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
            <span className="font-mono text-[10px] text-ink-faint">{row.original.actorId || '—'}</span>
          </div>
        ),
      },
      {
        accessorKey: 'action',
        header: 'Action',
        cell: ({ row }) => (
          <span className="font-mono text-xs text-ink">{row.original.action}</span>
        ),
      },
      {
        accessorKey: 'subjectType',
        header: 'Subject',
        cell: ({ row }) => (
          <span className="font-mono text-[10px] text-ink-muted">
            {row.original.subjectType}
            {row.original.subjectId ? ` · ${truncateId(row.original.subjectId, 22)}` : ''}
          </span>
        ),
      },
    ],
    [],
  )

  if (incidentQuery.isError) {
    return (
      <Page title="Incident">
        <div className="p-4">
          <ErrorState
            error={incidentQuery.error}
            onRetry={() => void incidentQuery.refetch()}
          />
        </div>
      </Page>
    )
  }

  const incident = incidentQuery.data

  return (
    <Page
      title={incident ? incident.title : 'Incident'}
      description={incident ? incident.id : undefined}
      scroll={false}
      actions={
        <Button variant="ghost" size="sm" onClick={() => void navigate({ to: '/incidents' })}>
          <ArrowLeftIcon className="size-3.5" />
          Back
        </Button>
      }
    >
      {!incident ? (
        <div className="flex flex-col gap-3 p-4">
          <Skeleton className="h-8 w-64" />
          <Skeleton className="h-32 w-full" />
        </div>
      ) : (
        <Tabs value={tab} onValueChange={setTab} className="h-full">
          <TabsList>
            <TabsTab value="overview">Overview</TabsTab>
            <TabsTab value="relations">
              Relations (
              {incident.alerts.length +
                incident.tracks.length +
                incident.assets.length +
                incident.observations.length +
                incident.assessments.length}
              )
            </TabsTab>
            <TabsTab value="operations">Missions &amp; Commands</TabsTab>
            <TabsTab value="audit">Audit ({auditQuery.data?.total ?? 0})</TabsTab>
          </TabsList>

          <TabsPanel value="overview" keepMounted>
            <div className="flex max-w-4xl flex-col gap-5">
              <div className="flex flex-wrap items-center gap-2">
                <StateBadge value={incident.status} />
                <StateBadge value={incident.priority} />
                <span className="font-mono text-[10px] text-ink-faint">{incident.id}</span>
              </div>

              <DetailList
                columns={2}
                items={[
                  {
                    label: 'Assigned operator',
                    value: incident.assignedOperator || '—',
                    mono: true,
                  },
                  { label: 'Created', value: formatTimestamp(incident.createdAt) },
                  { label: 'Updated', value: formatTimestamp(incident.updatedAt) },
                  { label: 'Resolved', value: formatTimestamp(incident.resolvedAt) },
                  { label: 'Closed', value: formatTimestamp(incident.closedAt) },
                ]}
              />

              <section className="flex flex-col gap-2">
                <SectionTitle>Description</SectionTitle>
                <p className="rounded-lg border border-edge bg-panel px-4 py-3 text-sm whitespace-pre-wrap text-ink-muted">
                  {incident.description || 'No description recorded.'}
                </p>
              </section>

              <section className="flex flex-col gap-2">
                <SectionTitle>Status transitions</SectionTitle>
                {(INCIDENT_TRANSITIONS[incident.status] ?? []).length === 0 ? (
                  <p className="text-xs text-ink-faint">This incident is closed to further transitions.</p>
                ) : (
                  <div className="flex flex-wrap gap-2">
                    {(INCIDENT_TRANSITIONS[incident.status] ?? []).map((status) => (
                      <Button
                        key={status}
                        variant={status === 'CLOSED' ? 'destructive' : 'secondary'}
                        size="sm"
                        disabled={statusMutation.isPending}
                        onClick={() => statusMutation.mutate(status)}
                      >
                        {status}
                      </Button>
                    ))}
                  </div>
                )}
              </section>
            </div>
          </TabsPanel>

          <TabsPanel value="relations" keepMounted>
            <div className="flex max-w-5xl flex-col gap-6">
              <div className="flex flex-wrap items-center gap-2">
                <Button size="sm" onClick={() => setAttachKind('asset')}>
                  <PlusIcon className="size-3.5" />
                  Attach asset
                </Button>
                <Button variant="secondary" size="sm" onClick={() => setAttachKind('track')}>
                  Attach track
                </Button>
                <Button variant="secondary" size="sm" onClick={() => setAttachKind('alert')}>
                  Attach alert
                </Button>
                <Button variant="secondary" size="sm" onClick={() => setAttachKind('observation')}>
                  Attach observation
                </Button>
                <Button variant="secondary" size="sm" onClick={() => setAttachKind('assessment')}>
                  Attach assessment
                </Button>
              </div>

              <section className="flex flex-col gap-3">
                <SectionTitle>Alerts ({incident.alerts.length})</SectionTitle>
                {incident.alerts.length === 0 ? (
                  <p className="text-xs text-ink-faint">No alerts attached.</p>
                ) : (
                  <ul className="flex flex-col gap-2">
                    {incident.alerts.map((alert) => (
                      <li
                        key={alert.id}
                        className="flex items-center justify-between gap-3 rounded-lg border border-edge bg-panel px-4 py-2.5"
                      >
                        <div className="min-w-0">
                          <p className="truncate text-xs text-ink">{alert.title}</p>
                          <p className="font-mono text-[10px] text-ink-faint">
                            {alert.id} · {formatTimestamp(alert.createdAt)}
                          </p>
                        </div>
                        <div className="flex shrink-0 items-center gap-2">
                          <SeverityBadge value={alert.severity} />
                          <StateBadge value={alert.state} />
                        </div>
                      </li>
                    ))}
                  </ul>
                )}
              </section>

              <section className="flex flex-col gap-3">
                <SectionTitle>Tracks ({incident.tracks.length})</SectionTitle>
                {incident.tracks.length === 0 ? (
                  <p className="text-xs text-ink-faint">No tracks attached.</p>
                ) : (
                  <ul className="flex flex-col gap-2">
                    {incident.tracks.map((track) => (
                      <li
                        key={track.id}
                        className="flex items-center justify-between gap-3 rounded-lg border border-edge bg-panel px-4 py-2.5"
                      >
                        <div className="min-w-0">
                          <p className="font-mono text-xs text-ink">
                            {track.externalRef || track.id}
                          </p>
                          <p className="font-mono text-[10px] text-ink-faint">
                            {track.id} · {formatCoord(track.position)}
                          </p>
                        </div>
                        <Link
                          to="/map"
                          search={{ selected: `track:${track.id}` }}
                          className="inline-flex items-center gap-1 text-xs text-accent hover:underline"
                        >
                          Inspect
                          <ExternalLinkIcon className="size-3" />
                        </Link>
                      </li>
                    ))}
                  </ul>
                )}
              </section>

              <section className="flex flex-col gap-3">
                <SectionTitle>Assets ({incident.assets.length})</SectionTitle>
                {incident.assets.length === 0 ? (
                  <p className="text-xs text-ink-faint">No assets attached.</p>
                ) : (
                  <ul className="flex flex-col gap-2">
                    {incident.assets.map((asset) => (
                      <li
                        key={asset.id}
                        className="flex items-center justify-between gap-3 rounded-lg border border-edge bg-panel px-4 py-2.5"
                      >
                        <div className="min-w-0">
                          <p className="text-xs text-ink">{asset.name}</p>
                          <p className="font-mono text-[10px] text-ink-faint">
                            {asset.id} · {formatCoord(asset.position)}
                          </p>
                        </div>
                        <div className="flex shrink-0 items-center gap-2">
                          <StateBadge value={asset.status} />
                          <Link
                            to="/map"
                            search={{ selected: `asset:${asset.id}` }}
                            className="inline-flex items-center gap-1 text-xs text-accent hover:underline"
                          >
                            Inspect
                            <ExternalLinkIcon className="size-3" />
                          </Link>
                        </div>
                      </li>
                    ))}
                  </ul>
                )}
              </section>

              <section className="flex flex-col gap-3">
                <SectionTitle>Observations ({incident.observations.length})</SectionTitle>
                {incident.observations.length === 0 ? (
                  <p className="text-xs text-ink-faint">No observations attached.</p>
                ) : (
                  <ul className="flex flex-col gap-1.5">
                    {incident.observations.map((observation) => (
                      <li
                        key={observation.id}
                        className="flex items-center justify-between gap-3 rounded border border-edge px-3 py-1.5"
                      >
                        <span className="font-mono text-[10px] text-ink-muted">
                          {observation.id}
                        </span>
                        <span className="font-mono text-[10px] text-ink-faint">
                          {observation.sourceId} · {formatCoord(observation.position)} ·{' '}
                          {formatTimestamp(observation.observedAt)}
                        </span>
                      </li>
                    ))}
                  </ul>
                )}
              </section>

              <section className="flex flex-col gap-3">
                <SectionTitle>Assessments ({incident.assessments.length})</SectionTitle>
                {incident.assessments.length === 0 ? (
                  <p className="text-xs text-ink-faint">No assessments attached.</p>
                ) : (
                  <ul className="flex flex-col gap-2">
                    {incident.assessments.map((assessment) => (
                      <li
                        key={assessment.id}
                        className="rounded-lg border border-edge bg-panel px-4 py-2.5"
                      >
                        <p className="text-xs text-ink">{assessment.conclusion}</p>
                        <p className="mt-0.5 font-mono text-[10px] text-ink-faint">
                          {assessment.method} · {formatConfidence(assessment.confidence)} ·{' '}
                          {formatTimestamp(assessment.createdAt)}
                        </p>
                      </li>
                    ))}
                  </ul>
                )}
              </section>
            </div>
          </TabsPanel>

          <TabsPanel value="operations" keepMounted>
            <div className="flex max-w-6xl flex-col gap-5">
              <div className="flex items-center justify-between gap-3">
                <SectionTitle>Mission planning</SectionTitle>
                <Button size="sm" onClick={() => setMissionOpen(true)}>
                  <PlusIcon className="size-3.5" />
                  Plan mission
                </Button>
              </div>
              <CommandConsole incidentId={id} />
            </div>
          </TabsPanel>

          <TabsPanel value="audit" keepMounted>
            <div className="max-w-6xl">
              {auditQuery.isError ? (
                <ErrorState error={auditQuery.error} onRetry={() => void auditQuery.refetch()} />
              ) : (
                <div className="overflow-hidden rounded-lg border border-edge bg-panel">
                  <DataTable
                    columns={auditColumns}
                    data={auditQuery.data?.items ?? []}
                    isLoading={auditQuery.isLoading}
                    emptyMessage="No audit entries recorded for this incident."
                    initialSorting={[{ id: 'occurredAt', desc: true }]}
                    getRowId={(entry) => entry.id}
                  />
                </div>
              )}
            </div>
          </TabsPanel>
        </Tabs>
      )}

      <AttachRelationDialog
        incidentId={id}
        open={attachKind !== null}
        onOpenChange={(open) => {
          if (!open) setAttachKind(null)
        }}
        initialKind={attachKind ?? 'asset'}
      />

      <MissionFormDialog
        incidentId={id}
        open={missionOpen}
        onOpenChange={setMissionOpen}
        onCreated={() => setTab('operations')}
      />
    </Page>
  )
}
