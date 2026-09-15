import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useForm } from '@tanstack/react-form'

import { Field, Textarea, fieldError } from '@/components/shared/form'
import { Button } from '@/components/ui/button'
import { Dialog } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Select } from '@/components/ui/select'
import { api, type Mission } from '@/lib/api'
import { queryKeys } from '@/lib/queryKeys'
import { toast } from '@/stores/toasts'

const PRIORITY_OPTIONS = [
  { value: 'low', label: 'Low' },
  { value: 'medium', label: 'Medium' },
  { value: 'high', label: 'High' },
  { value: 'critical', label: 'Critical' },
]

export function MissionFormDialog({
  incidentId,
  open,
  onOpenChange,
  onCreated,
}: {
  incidentId: string
  open: boolean
  onOpenChange: (open: boolean) => void
  onCreated?: (mission: Mission) => void
}) {
  const queryClient = useQueryClient()

  const assetsQuery = useQuery({
    queryKey: queryKeys.assets.list({ limit: 200 }),
    queryFn: () => api.listAssets({ limit: 200 }),
    enabled: open,
  })

  const form = useForm({
    defaultValues: {
      name: '',
      objective: '',
      priority: 'high',
      assetIds: [] as string[],
    },
    onSubmit: async ({ value }) => {
      const mission = await createMutation.mutateAsync({
        name: value.name.trim(),
        objective: value.objective.trim(),
        priority: value.priority,
        incidentId,
        assets: value.assetIds,
      })
      onCreated?.(mission)
      onOpenChange(false)
      form.reset()
    },
  })

  const createMutation = useMutation({
    mutationFn: api.createMission,
    onSuccess: (mission) => {
      toast({ title: 'Mission created', description: mission.name, variant: 'success' })
      void queryClient.invalidateQueries({ queryKey: queryKeys.missions.all })
      void queryClient.invalidateQueries({ queryKey: queryKeys.incidents.detail(incidentId) })
    },
    onError: (error) => {
      toast({
        title: 'Mission creation failed',
        description: error instanceof Error ? error.message : undefined,
        variant: 'error',
      })
    },
  })

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title="Plan mission"
      description="Create a mission linked to this incident."
      wide
      footer={
        <>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button
            onClick={() => void form.handleSubmit()}
            disabled={createMutation.isPending}
          >
            {createMutation.isPending ? 'Creating…' : 'Create mission'}
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
        <form.Field
          name="name"
          validators={{
            onSubmit: ({ value }) => (value.trim() ? undefined : 'Name is required'),
          }}
        >
          {(field) => (
            <Field label="Name" htmlFor="mission-name" required error={fieldError(field.state.meta.errors)}>
              <Input
                id="mission-name"
                value={field.state.value}
                onBlur={field.handleBlur}
                onChange={(event) => field.handleChange(event.target.value)}
                placeholder="Respond to Restricted Zone A"
              />
            </Field>
          )}
        </form.Field>

        <form.Field name="objective">
          {(field) => (
            <Field label="Objective" htmlFor="mission-objective">
              <Textarea
                id="mission-objective"
                value={field.state.value}
                onBlur={field.handleBlur}
                onChange={(event) => field.handleChange(event.target.value)}
                placeholder="Intercept and identify the unknown vehicle"
              />
            </Field>
          )}
        </form.Field>

        <form.Field name="priority">
          {(field) => (
            <Field label="Priority" htmlFor="mission-priority">
              <Select
                id="mission-priority"
                value={field.state.value}
                onValueChange={(value) => field.handleChange(value)}
                options={PRIORITY_OPTIONS}
              />
            </Field>
          )}
        </form.Field>

        <form.Field name="assetIds">
          {(field) => (
            <Field label="Assigned assets" hint="Select one or more assets">
              <div className="flex max-h-44 flex-col gap-1 overflow-y-auto rounded-md border border-edge bg-bg p-2">
                {(assetsQuery.data?.items ?? []).length === 0 ? (
                  <p className="px-1 py-2 text-xs text-ink-faint">No assets available.</p>
                ) : (
                  assetsQuery.data?.items.map((asset) => {
                    const checked = field.state.value.includes(asset.id)
                    return (
                      <label
                        key={asset.id}
                        className="flex cursor-pointer items-center gap-2 rounded px-1.5 py-1 hover:bg-panel-raised"
                      >
                        <input
                          type="checkbox"
                          className="accent-cyan-400"
                          checked={checked}
                          onChange={(event) => {
                            field.handleChange(
                              event.target.checked
                                ? [...field.state.value, asset.id]
                                : field.state.value.filter((id) => id !== asset.id),
                            )
                          }}
                        />
                        <span className="text-xs text-ink">{asset.name}</span>
                        <span className="font-mono text-[10px] text-ink-faint">{asset.id}</span>
                        <span className="ml-auto font-mono text-[10px] text-ink-faint">
                          {asset.status}
                        </span>
                      </label>
                    )
                  })
                )}
              </div>
            </Field>
          )}
        </form.Field>
      </form>
    </Dialog>
  )
}
