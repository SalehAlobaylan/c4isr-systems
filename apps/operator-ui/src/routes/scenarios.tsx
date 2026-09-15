import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useForm } from '@tanstack/react-form'
import type { ColumnDef } from '@tanstack/react-table'
import { useMemo, useState } from 'react'

import {
  PauseIcon,
  PlayIcon,
  RefreshIcon,
  StopIcon,
} from '@/components/icons'
import { Page } from '@/components/layout/Page'
import { StateBadge } from '@/components/shared/badges'
import { DataTable } from '@/components/shared/data-table'
import { ErrorState, SectionTitle } from '@/components/shared/states'
import { Field } from '@/components/shared/form'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Dialog } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Select } from '@/components/ui/select'
import { api, type ScenarioRun, type ScenarioSummary } from '@/lib/api'
import { formatRelative, formatTimestamp, formatVirtualTime } from '@/lib/format'
import { queryKeys } from '@/lib/queryKeys'
import { toast } from '@/stores/toasts'

const SPEED_OPTIONS = [
  { value: '1', label: '1×' },
  { value: '2', label: '2×' },
  { value: '5', label: '5×' },
  { value: '10', label: '10×' },
  { value: '50', label: '50×' },
]

export function ScenariosPage() {
  const queryClient = useQueryClient()
  const [startTarget, setStartTarget] = useState<ScenarioSummary | null>(null)

  const scenariosQuery = useQuery({
    queryKey: queryKeys.scenarios.list(),
    queryFn: () => api.listScenarios(),
  })

  const runsFilters = { limit: 50 }
  const runsQuery = useQuery({
    queryKey: queryKeys.scenarios.runs(runsFilters),
    queryFn: () => api.listScenarioRuns(runsFilters),
    refetchInterval: 10_000,
  })

  const invalidate = () => {
    void queryClient.invalidateQueries({ queryKey: queryKeys.scenarios.all })
  }

  const startMutation = useMutation({
    mutationFn: ({
      name,
      speed,
      seed,
    }: {
      name: string
      speed: number
      seed?: number
    }) => api.startScenario(name, { speed, seed }),
    onSuccess: (run) => {
      toast({
        title: 'Scenario started',
        description: `${run.scenarioName} @ ${run.playbackSpeed}× · seed ${run.seed}`,
        variant: 'success',
      })
      invalidate()
    },
    onError: (error) => {
      toast({
        title: 'Scenario start failed',
        description: error instanceof Error ? error.message : undefined,
        variant: 'error',
      })
    },
  })

  const runActionMutation = useMutation({
    mutationFn: ({ id, action }: { id: string; action: 'pause' | 'resume' | 'stop' }) => {
      if (action === 'pause') return api.pauseScenarioRun(id)
      if (action === 'resume') return api.resumeScenarioRun(id)
      return api.stopScenarioRun(id)
    },
    onSuccess: (run, variables) => {
      toast({ title: `Run ${variables.action}d`, description: run.scenarioName, variant: 'success' })
      invalidate()
    },
    onError: (error) => {
      toast({
        title: 'Run control failed',
        description: error instanceof Error ? error.message : undefined,
        variant: 'error',
      })
    },
  })

  const speedMutation = useMutation({
    mutationFn: ({ id, speed }: { id: string; speed: number }) =>
      api.setScenarioRunSpeed(id, speed),
    onSuccess: (run) => {
      toast({ title: `Playback speed set to ${run.playbackSpeed}×`, variant: 'success' })
      invalidate()
    },
    onError: (error) => {
      toast({
        title: 'Speed change failed',
        description: error instanceof Error ? error.message : undefined,
        variant: 'error',
      })
    },
  })

  const runs = runsQuery.data?.items ?? []

  const columns = useMemo<Array<ColumnDef<ScenarioRun, any>>>(
    () => [
      {
        accessorKey: 'scenarioName',
        header: 'Scenario',
        cell: ({ row }) => (
          <div className="min-w-0">
            <p className="font-mono text-xs text-ink">{row.original.scenarioName}</p>
            <p className="font-mono text-[10px] text-ink-faint">{row.original.id}</p>
          </div>
        ),
      },
      {
        accessorKey: 'seed',
        header: 'Seed',
        cell: ({ row }) => (
          <span className="font-mono text-xs text-ink-muted">{row.original.seed}</span>
        ),
      },
      {
        accessorKey: 'status',
        header: 'Status',
        cell: ({ row }) => <StateBadge value={row.original.status} />,
      },
      {
        id: 'virtualTimeMs',
        header: 'Virtual time',
        cell: ({ row }) => (
          <span className="font-mono text-xs text-ink-muted">
            {formatVirtualTime(row.original.virtualTimeMs)}
          </span>
        ),
      },
      {
        id: 'playbackSpeed',
        header: 'Speed',
        cell: ({ row }) => {
          const run = row.original
          const active = run.status === 'RUNNING' || run.status === 'PAUSED'
          if (!active) {
            return (
              <span className="font-mono text-xs text-ink-muted">{run.playbackSpeed}×</span>
            )
          }
          return (
            <Select
              ariaLabel={`Playback speed for ${run.id}`}
              className="w-24"
              value={String(run.playbackSpeed)}
              onValueChange={(value) =>
                speedMutation.mutate({ id: run.id, speed: Number(value) })
              }
              options={SPEED_OPTIONS}
            />
          )
        },
      },
      {
        accessorKey: 'startedAt',
        header: 'Started',
        cell: ({ row }) => (
          <span className="text-xs text-ink-muted" title={formatTimestamp(row.original.startedAt)}>
            {formatRelative(row.original.startedAt)}
          </span>
        ),
      },
      {
        id: 'endedAt',
        header: 'Ended',
        cell: ({ row }) => (
          <span className="text-xs text-ink-muted" title={formatTimestamp(row.original.endedAt)}>
            {row.original.endedAt ? formatRelative(row.original.endedAt) : '—'}
          </span>
        ),
      },
      {
        id: 'controls',
        header: 'Controls',
        enableSorting: false,
        cell: ({ row }) => {
          const run = row.original
          const isRunning = run.status === 'RUNNING'
          const isPaused = run.status === 'PAUSED'
          const isActive = isRunning || isPaused
          return (
            <div className="flex items-center gap-1.5">
              {isRunning ? (
                <Button
                  variant="secondary"
                  size="sm"
                  title="Pause"
                  onClick={() => runActionMutation.mutate({ id: run.id, action: 'pause' })}
                >
                  <PauseIcon className="size-3.5" />
                </Button>
              ) : null}
              {isPaused ? (
                <Button
                  variant="secondary"
                  size="sm"
                  title="Resume"
                  onClick={() => runActionMutation.mutate({ id: run.id, action: 'resume' })}
                >
                  <PlayIcon className="size-3.5" />
                </Button>
              ) : null}
              {isActive ? (
                <Button
                  variant="destructive"
                  size="sm"
                  title="Stop"
                  onClick={() => runActionMutation.mutate({ id: run.id, action: 'stop' })}
                >
                  <StopIcon className="size-3.5" />
                </Button>
              ) : null}
              <Button
                variant="outline"
                size="sm"
                title="Restart with same seed"
                onClick={() =>
                  startMutation.mutate({
                    name: run.scenarioName,
                    speed: run.playbackSpeed || 1,
                    seed: run.seed,
                  })
                }
              >
                <RefreshIcon className="size-3.5" />
                Restart
              </Button>
            </div>
          )
        },
      },
    ],
    [runActionMutation, speedMutation, startMutation],
  )

  return (
    <Page
      title="Scenarios"
      description="Deterministic replay of synthetic operational activity"
    >
      <div className="flex flex-col gap-6 p-4">
        <section className="flex flex-col gap-3">
          <SectionTitle>Available scenarios</SectionTitle>
          {scenariosQuery.isError ? (
            <ErrorState
              error={scenariosQuery.error}
              onRetry={() => void scenariosQuery.refetch()}
            />
          ) : (
            <div className="grid grid-cols-1 gap-3 lg:grid-cols-2 xl:grid-cols-3">
              {(scenariosQuery.data?.items ?? []).map((scenario) => (
                <div
                  key={scenario.name}
                  className="flex flex-col gap-3 rounded-lg border border-edge bg-panel p-4"
                >
                  <div className="flex items-start justify-between gap-3">
                    <div className="min-w-0">
                      <h3 className="truncate font-mono text-sm font-semibold text-ink">
                        {scenario.name}
                      </h3>
                      <p className="mt-1 line-clamp-3 text-xs text-ink-muted">
                        {scenario.description.trim()}
                      </p>
                    </div>
                    <Badge variant="outline" className="shrink-0 font-mono">
                      seed {scenario.seed}
                    </Badge>
                  </div>
                  <div className="flex flex-wrap gap-1.5">
                    <Badge variant="muted">{scenario.sources} sources</Badge>
                    <Badge variant="muted">{scenario.assets} assets</Badge>
                    <Badge variant="muted">{scenario.tracks} tracks</Badge>
                    <Badge variant="muted">{scenario.geofences} geofences</Badge>
                    <Badge variant="muted">{scenario.events} events</Badge>
                  </div>
                  <Button size="sm" className="mt-auto self-start" onClick={() => setStartTarget(scenario)}>
                    <PlayIcon className="size-3.5" />
                    Start run
                  </Button>
                </div>
              ))}
              {scenariosQuery.isLoading
                ? Array.from({ length: 3 }).map((_, index) => (
                    <div
                      key={index}
                      className="h-40 animate-pulse rounded-lg border border-edge bg-panel"
                    />
                  ))
                : null}
            </div>
          )}
        </section>

        <section className="flex flex-col gap-3">
          <SectionTitle>Runs</SectionTitle>
          {runsQuery.isError ? (
            <ErrorState error={runsQuery.error} onRetry={() => void runsQuery.refetch()} />
          ) : (
            <div className="overflow-hidden rounded-lg border border-edge bg-panel">
              <DataTable
                columns={columns}
                data={runs}
                isLoading={runsQuery.isLoading}
                emptyMessage="No scenario runs recorded yet."
                initialSorting={[{ id: 'startedAt', desc: true }]}
                getRowId={(run) => run.id}
              />
            </div>
          )}
        </section>
      </div>

      {startTarget ? (
        <StartScenarioDialog
          scenario={startTarget}
          open={startTarget !== null}
          onOpenChange={(open) => {
            if (!open) setStartTarget(null)
          }}
          onStart={(speed, seed) => {
            startMutation.mutate({ name: startTarget.name, speed, seed })
            setStartTarget(null)
          }}
          pending={startMutation.isPending}
        />
      ) : null}
    </Page>
  )
}

function StartScenarioDialog({
  scenario,
  open,
  onOpenChange,
  onStart,
  pending,
}: {
  scenario: ScenarioSummary
  open: boolean
  onOpenChange: (open: boolean) => void
  onStart: (speed: number, seed?: number) => void
  pending: boolean
}) {
  const form = useForm({
    defaultValues: {
      speed: '10',
      seed: String(scenario.seed),
    },
    onSubmit: ({ value }) => {
      const seed = value.seed.trim() ? Number(value.seed) : undefined
      onStart(Number(value.speed), seed)
    },
  })

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={`Start ${scenario.name}`}
      description="Select the playback speed and an optional seed for deterministic replay."
      footer={
        <>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button onClick={() => void form.handleSubmit()} disabled={pending}>
            {pending ? 'Starting…' : 'Start run'}
          </Button>
        </>
      }
    >
      <form
        className="flex flex-col gap-4"
        onSubmit={(event) => {
          event.preventDefault()
          void form.handleSubmit()
        }}
      >
        <form.Field name="speed">
          {(field) => (
            <Field label="Playback speed" htmlFor="scenario-speed">
              <Select
                id="scenario-speed"
                value={field.state.value}
                onValueChange={(value) => field.handleChange(value)}
                options={SPEED_OPTIONS}
              />
            </Field>
          )}
        </form.Field>
        <form.Field name="seed">
          {(field) => (
            <Field
              label="Seed"
              htmlFor="scenario-seed"
              hint={`Default seed for this scenario is ${scenario.seed}.`}
            >
              <Input
                id="scenario-seed"
                className="font-mono"
                value={field.state.value}
                onBlur={field.handleBlur}
                onChange={(event) => field.handleChange(event.target.value)}
                placeholder={String(scenario.seed)}
              />
            </Field>
          )}
        </form.Field>
      </form>
    </Dialog>
  )
}
