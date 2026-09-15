import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useForm } from '@tanstack/react-form'
import { useEffect } from 'react'

import { Field, Textarea, fieldError } from '@/components/shared/form'
import { Button } from '@/components/ui/button'
import { Dialog } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Select } from '@/components/ui/select'
import { api, type CreateIncidentInput, type Incident } from '@/lib/api'
import { parseCommaList } from '@/lib/format'
import { queryKeys } from '@/lib/queryKeys'
import { toast } from '@/stores/toasts'

const PRIORITY_OPTIONS = [
  { value: 'low', label: 'Low' },
  { value: 'medium', label: 'Medium' },
  { value: 'high', label: 'High' },
  { value: 'critical', label: 'Critical' },
]

interface IncidentFormValues {
  title: string
  description: string
  priority: string
  alertIds: string
  trackIds: string
  assetIds: string
}

export function IncidentFormDialog({
  open,
  onOpenChange,
  defaultAlertId,
  onCreated,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  defaultAlertId?: string
  onCreated: (incident: Incident) => void
}) {
  const queryClient = useQueryClient()

  const form = useForm({
    defaultValues: {
      title: '',
      description: '',
      priority: 'medium',
      alertIds: defaultAlertId ?? '',
      trackIds: '',
      assetIds: '',
    } satisfies IncidentFormValues,
    onSubmit: async ({ value }) => {
      const input: CreateIncidentInput = {
        title: value.title.trim(),
        description: value.description.trim(),
        priority: value.priority,
        alertIds: parseCommaList(value.alertIds),
        trackIds: parseCommaList(value.trackIds),
        assetIds: parseCommaList(value.assetIds),
        observationIds: [],
        assessmentIds: [],
      }
      const incident = await createMutation.mutateAsync(input)
      onCreated(incident)
    },
  })

  useEffect(() => {
    if (open) {
      form.reset()
      form.setFieldValue('alertIds', defaultAlertId ?? '')
    }
    // Reset only when the dialog opens with a new prefill.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, defaultAlertId])

  const createMutation = useMutation({
    mutationFn: (input: CreateIncidentInput) => api.createIncident(input),
    onSuccess: (incident) => {
      toast({ title: 'Incident created', description: incident.title, variant: 'success' })
      void queryClient.invalidateQueries({ queryKey: queryKeys.incidents.all })
    },
    onError: (error) => {
      toast({
        title: 'Incident creation failed',
        description: error instanceof Error ? error.message : undefined,
        variant: 'error',
      })
    },
  })

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title="Create incident"
      description="Open a coordinated workspace for an operational situation."
      wide
      footer={
        <>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <form.Subscribe selector={(state) => state.isSubmitting}>
            {(isSubmitting) => (
              <Button
                onClick={() => void form.handleSubmit()}
                disabled={isSubmitting || createMutation.isPending}
              >
                {createMutation.isPending ? 'Creating…' : 'Create incident'}
              </Button>
            )}
          </form.Subscribe>
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
          name="title"
          validators={{
            onSubmit: ({ value }) => (value.trim().length === 0 ? 'Title is required' : undefined),
          }}
        >
          {(field) => (
            <Field
              label="Title"
              htmlFor="incident-title"
              required
              error={fieldError(field.state.meta.errors)}
            >
              <Input
                id="incident-title"
                value={field.state.value}
                onBlur={field.handleBlur}
                onChange={(event) => field.handleChange(event.target.value)}
                placeholder="Unknown vehicle in restricted zone"
              />
            </Field>
          )}
        </form.Field>

        <form.Field name="description">
          {(field) => (
            <Field label="Description" htmlFor="incident-description">
              <Textarea
                id="incident-description"
                value={field.state.value}
                onBlur={field.handleBlur}
                onChange={(event) => field.handleChange(event.target.value)}
                placeholder="Operational context, intent, and constraints"
              />
            </Field>
          )}
        </form.Field>

        <form.Field name="priority">
          {(field) => (
            <Field label="Priority" htmlFor="incident-priority">
              <Select
                id="incident-priority"
                value={field.state.value}
                onValueChange={(value) => field.handleChange(value)}
                options={PRIORITY_OPTIONS}
              />
            </Field>
          )}
        </form.Field>

        <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
          <form.Field name="alertIds">
            {(field) => (
              <Field
                label="Alert IDs"
                htmlFor="incident-alerts"
                hint="Comma separated; pre-filled from the alert center."
              >
                <Input
                  id="incident-alerts"
                  className="font-mono text-xs"
                  value={field.state.value}
                  onBlur={field.handleBlur}
                  onChange={(event) => field.handleChange(event.target.value)}
                  placeholder="alr_…"
                />
              </Field>
            )}
          </form.Field>
          <form.Field name="trackIds">
            {(field) => (
              <Field label="Track IDs" htmlFor="incident-tracks" hint="Comma separated">
                <Input
                  id="incident-tracks"
                  className="font-mono text-xs"
                  value={field.state.value}
                  onBlur={field.handleBlur}
                  onChange={(event) => field.handleChange(event.target.value)}
                  placeholder="trk_…"
                />
              </Field>
            )}
          </form.Field>
          <form.Field name="assetIds">
            {(field) => (
              <Field label="Asset IDs" htmlFor="incident-assets" hint="Comma separated">
                <Input
                  id="incident-assets"
                  className="font-mono text-xs"
                  value={field.state.value}
                  onBlur={field.handleBlur}
                  onChange={(event) => field.handleChange(event.target.value)}
                  placeholder="patrol-01, …"
                />
              </Field>
            )}
          </form.Field>
        </div>
      </form>
    </Dialog>
  )
}
