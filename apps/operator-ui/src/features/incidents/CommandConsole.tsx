import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useForm } from '@tanstack/react-form'
import type { ColumnDef } from '@tanstack/react-table'
import { useMemo } from 'react'

import { Field } from '@/components/shared/form'
import { StateBadge } from '@/components/shared/badges'
import { DataTable } from '@/components/shared/data-table'
import { SectionTitle } from '@/components/shared/states'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select } from '@/components/ui/select'
import { api, type Command } from '@/lib/api'
import { formatRelative, formatTimestamp } from '@/lib/format'
import { queryKeys } from '@/lib/queryKeys'
import { toast } from '@/stores/toasts'

const COMMAND_TRANSITIONS: Record<string, string[]> = {
  CREATED: ['QUEUED', 'SENT', 'CANCELLED'],
  QUEUED: ['SENT', 'CANCELLED'],
  SENT: ['ACKNOWLEDGED', 'REJECTED', 'FAILED', 'TIMED_OUT', 'CANCELLED'],
  ACKNOWLEDGED: ['COMPLETED', 'FAILED', 'CANCELLED'],
}

const COMMAND_TYPES = [
  { value: 'move_to', label: 'move_to' },
  { value: 'hold', label: 'hold' },
  { value: 'return_to_base', label: 'return_to_base' },
  { value: 'observe', label: 'observe' },
]

export function CommandConsole({ incidentId }: { incidentId: string }) {
  const queryClient = useQueryClient()

  const missionsQuery = useQuery({
    queryKey: queryKeys.missions.list({ limit: 100 }),
    queryFn: () => api.listMissions({ limit: 100 }),
  })
  const assetsQuery = useQuery({
    queryKey: queryKeys.assets.list({ limit: 200 }),
    queryFn: () => api.listAssets({ limit: 200 }),
  })
  const commandsQuery = useQuery({
    queryKey: queryKeys.commands.list({ limit: 200 }),
    queryFn: () => api.listCommands({ limit: 200 }),
    refetchInterval: 15_000,
  })

  const incidentMissions = useMemo(
    () => (missionsQuery.data?.items ?? []).filter((mission) => mission.incidentId === incidentId),
    [missionsQuery.data, incidentId],
  )

  const incidentCommands = useMemo(
    () => (commandsQuery.data?.items ?? []).filter((command) => command.incidentId === incidentId),
    [commandsQuery.data, incidentId],
  )

  const missionOptions = useMemo(
    () => [
      { value: '', label: 'No mission' },
      ...incidentMissions.map((mission) => ({
        value: mission.id,
        label: `${mission.name} (${mission.status})`,
      })),
    ],
    [incidentMissions],
  )

  const assetOptions = useMemo(
    () =>
      (assetsQuery.data?.items ?? []).map((asset) => ({
        value: asset.id,
        label: `${asset.name} (${asset.status})`,
      })),
    [assetsQuery.data],
  )

  const form = useForm({
    defaultValues: {
      assetId: '',
      missionId: '',
      type: 'move_to',
      lat: '',
      lng: '',
    },
    onSubmit: async ({ value }) => {
      if (!value.assetId) return
      const payload: Record<string, unknown> = {}
      if (value.lat.trim() && value.lng.trim()) {
        payload.lat = Number(value.lat)
        payload.lng = Number(value.lng)
      }
      await issueMutation.mutateAsync({
        assetId: value.assetId,
        missionId: value.missionId || undefined,
        incidentId,
        type: value.type,
        payload,
      })
      form.reset()
    },
  })

  const issueMutation = useMutation({
    mutationFn: api.issueCommand,
    onSuccess: (command) => {
      toast({ title: 'Command issued', description: command.type, variant: 'success' })
      void queryClient.invalidateQueries({ queryKey: queryKeys.commands.all })
    },
    onError: (error) => {
      toast({
        title: 'Command failed',
        description: error instanceof Error ? error.message : undefined,
        variant: 'error',
      })
    },
  })

  const transitionMutation = useMutation({
    mutationFn: ({ id, state }: { id: string; state: string }) =>
      api.transitionCommand(id, state),
    onSuccess: (command) => {
      toast({ title: `Command ${command.state.toLowerCase()}`, variant: 'success' })
      void queryClient.invalidateQueries({ queryKey: queryKeys.commands.all })
    },
    onError: (error) => {
      toast({
        title: 'Transition failed',
        description: error instanceof Error ? error.message : undefined,
        variant: 'error',
      })
    },
  })

  const columns = useMemo<Array<ColumnDef<Command, any>>>(
    () => [
      {
        accessorKey: 'id',
        header: 'Command',
        cell: ({ row }) => (
          <span className="font-mono text-[11px] text-ink-muted" title={row.original.id}>
            {row.original.id}
          </span>
        ),
      },
      {
        accessorKey: 'type',
        header: 'Type',
        cell: ({ row }) => <span className="font-mono text-xs text-ink">{row.original.type}</span>,
      },
      {
        accessorKey: 'assetId',
        header: 'Asset',
        cell: ({ row }) => (
          <span className="font-mono text-xs text-ink-muted">{row.original.assetId}</span>
        ),
      },
      {
        accessorKey: 'state',
        header: 'State',
        cell: ({ row }) => <StateBadge value={row.original.state} />,
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
      {
        id: 'transitions',
        header: 'Transition',
        enableSorting: false,
        cell: ({ row }) => {
          const nextStates = COMMAND_TRANSITIONS[row.original.state] ?? []
          if (nextStates.length === 0) {
            return <span className="text-[10px] text-ink-faint">terminal</span>
          }
          return (
            <div className="flex items-center gap-1">
              {nextStates.map((state) => (
                <Button
                  key={state}
                  variant="outline"
                  size="sm"
                  className="h-6 px-2 font-mono text-[10px]"
                  disabled={transitionMutation.isPending}
                  onClick={() =>
                    transitionMutation.mutate({ id: row.original.id, state })
                  }
                >
                  {state}
                </Button>
              ))}
            </div>
          )
        },
      },
    ],
    [transitionMutation],
  )

  return (
    <div className="flex flex-col gap-6">
      <section className="rounded-lg border border-edge bg-panel p-4">
        <SectionTitle className="mb-3">Issue command</SectionTitle>
        <form
          className="grid grid-cols-1 gap-3 md:grid-cols-6"
          onSubmit={(event) => {
            event.preventDefault()
            void form.handleSubmit()
          }}
        >
          <form.Field
            name="assetId"
            validators={{
              onSubmit: ({ value }) => (value ? undefined : 'Select an asset'),
            }}
          >
            {(field) => (
              <Field
                label="Asset"
                htmlFor="command-asset"
                required
                className="md:col-span-2"
                error={field.state.meta.errors.length ? 'Select an asset' : undefined}
              >
                <Select
                  id="command-asset"
                  value={field.state.value}
                  onValueChange={(value) => field.handleChange(value)}
                  options={assetOptions}
                  placeholder="Select asset…"
                />
              </Field>
            )}
          </form.Field>

          <form.Field name="missionId">
            {(field) => (
              <Field label="Mission" htmlFor="command-mission" className="md:col-span-2">
                <Select
                  id="command-mission"
                  value={field.state.value}
                  onValueChange={(value) => field.handleChange(value)}
                  options={missionOptions}
                  placeholder="No mission"
                />
              </Field>
            )}
          </form.Field>

          <form.Field name="type">
            {(field) => (
              <Field label="Type" htmlFor="command-type" className="md:col-span-2">
                <Select
                  id="command-type"
                  value={field.state.value}
                  onValueChange={(value) => field.handleChange(value)}
                  options={COMMAND_TYPES}
                />
              </Field>
            )}
          </form.Field>

          <form.Field name="lat">
            {(field) => (
              <Field label="Target lat" htmlFor="command-lat" className="md:col-span-2">
                <Input
                  id="command-lat"
                  value={field.state.value}
                  onBlur={field.handleBlur}
                  onChange={(event) => field.handleChange(event.target.value)}
                  placeholder="24.7160"
                />
              </Field>
            )}
          </form.Field>

          <form.Field name="lng">
            {(field) => (
              <Field label="Target lng" htmlFor="command-lng" className="md:col-span-2">
                <Input
                  id="command-lng"
                  value={field.state.value}
                  onBlur={field.handleBlur}
                  onChange={(event) => field.handleChange(event.target.value)}
                  placeholder="46.6790"
                />
              </Field>
            )}
          </form.Field>

          <div className="flex items-end md:col-span-2">
            <Button
              type="submit"
              className="w-full"
              disabled={issueMutation.isPending || !form.state.values.assetId}
            >
              {issueMutation.isPending ? 'Issuing…' : 'Issue command'}
            </Button>
          </div>
        </form>
      </section>

      <section className="flex flex-col gap-3">
        <SectionTitle>Commands ({incidentCommands.length})</SectionTitle>
        <div className="overflow-hidden rounded-lg border border-edge bg-panel">
          <DataTable
            columns={columns}
            data={incidentCommands}
            isLoading={commandsQuery.isLoading}
            emptyMessage="No commands issued for this incident."
            initialSorting={[{ id: 'updatedAt', desc: true }]}
            getRowId={(command) => command.id}
          />
        </div>
      </section>

      <section className="flex flex-col gap-3">
        <SectionTitle>Missions ({incidentMissions.length})</SectionTitle>
        {incidentMissions.length === 0 ? (
          <p className="text-xs text-ink-faint">No missions linked to this incident.</p>
        ) : (
          <ul className="flex flex-col gap-2">
            {incidentMissions.map((mission) => (
              <li
                key={mission.id}
                className="rounded-lg border border-edge bg-panel px-4 py-3"
              >
                <div className="flex items-center justify-between gap-3">
                  <div className="min-w-0">
                    <p className="truncate text-sm text-ink">{mission.name}</p>
                    <p className="mt-0.5 truncate text-xs text-ink-muted">{mission.objective}</p>
                  </div>
                  <div className="flex shrink-0 items-center gap-2">
                    <StateBadge value={mission.priority} />
                    <StateBadge value={mission.status} />
                  </div>
                </div>
                {mission.assets.length > 0 ? (
                  <p className="mt-2 font-mono text-[10px] text-ink-faint">
                    assets: {mission.assets.map((asset) => asset.id).join(', ')}
                  </p>
                ) : null}
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  )
}
