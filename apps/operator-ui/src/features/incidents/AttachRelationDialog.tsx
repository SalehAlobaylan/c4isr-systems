import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useForm, useStore } from '@tanstack/react-form'
import { useEffect } from 'react'

import { Field, fieldError } from '@/components/shared/form'
import { Button } from '@/components/ui/button'
import { Dialog } from '@/components/ui/dialog'
import { Select, type SelectOption } from '@/components/ui/select'
import { api } from '@/lib/api'
import { queryKeys } from '@/lib/queryKeys'
import { toast } from '@/stores/toasts'

export type RelationKind = 'asset' | 'track' | 'alert' | 'observation' | 'assessment'

const KIND_OPTIONS: SelectOption[] = [
  { value: 'asset', label: 'Asset' },
  { value: 'track', label: 'Track' },
  { value: 'alert', label: 'Alert' },
  { value: 'observation', label: 'Observation' },
  { value: 'assessment', label: 'Assessment' },
]

export function AttachRelationDialog({
  incidentId,
  open,
  onOpenChange,
  initialKind = 'asset',
}: {
  incidentId: string
  open: boolean
  onOpenChange: (open: boolean) => void
  initialKind?: RelationKind
}) {
  const queryClient = useQueryClient()

  const assetsQuery = useQuery({
    queryKey: queryKeys.assets.list({ limit: 200 }),
    queryFn: () => api.listAssets({ limit: 200 }),
    enabled: open,
  })
  const tracksQuery = useQuery({
    queryKey: queryKeys.tracks.list({ limit: 200 }),
    queryFn: () => api.listTracks({ limit: 200 }),
    enabled: open,
  })
  const alertsQuery = useQuery({
    queryKey: queryKeys.alerts.list({ limit: 100 }),
    queryFn: () => api.listAlerts({ limit: 100 }),
    enabled: open,
  })
  const observationsQuery = useQuery({
    queryKey: queryKeys.observations.list({ limit: 50 }),
    queryFn: () => api.listObservations({ limit: 50 }),
    enabled: open,
  })
  const assessmentsQuery = useQuery({
    queryKey: queryKeys.assessments.list({ limit: 50 }),
    queryFn: () => api.listAssessments({ limit: 50 }),
    enabled: open,
  })

  const form = useForm({
    defaultValues: {
      kind: initialKind as string,
      entityId: '',
    },
    onSubmit: async ({ value }) => {
      if (!value.entityId) return
      await attachMutation.mutateAsync({ kind: value.kind, id: value.entityId })
    },
  })
  const selectedRelationKind = useStore(form.store, (state) => state.values.kind)
  const selectedEntityId = useStore(form.store, (state) => state.values.entityId)

  useEffect(() => {
    if (open) {
      form.reset({ kind: initialKind, entityId: '' })
    }
  }, [form, initialKind, open])

  const attachMutation = useMutation({
    mutationFn: ({ kind, id }: { kind: string; id: string }) =>
      api.attachIncidentRelation(incidentId, kind, id),
    onSuccess: () => {
      toast({ title: 'Relation attached', variant: 'success' })
      void queryClient.invalidateQueries({ queryKey: queryKeys.incidents.detail(incidentId) })
      void queryClient.invalidateQueries({
        queryKey: queryKeys.audit.list({ subject_type: 'incident', subject_id: incidentId, limit: 100 }),
      })
      onOpenChange(false)
    },
    onError: (error) => {
      toast({
        title: 'Attach failed',
        description: error instanceof Error ? error.message : undefined,
        variant: 'error',
      })
    },
  })

  const optionsForKind = (kind: string): SelectOption[] => {
    switch (kind) {
      case 'asset':
        return (assetsQuery.data?.items ?? []).map((asset) => ({
          value: asset.id,
          label: `${asset.name} (${asset.id})`,
        }))
      case 'track':
        return (tracksQuery.data?.items ?? []).map((track) => ({
          value: track.id,
          label: `${track.externalRef || 'track'} (${track.id})`,
        }))
      case 'alert':
        return (alertsQuery.data?.items ?? []).map((alert) => ({
          value: alert.id,
          label: `${alert.title} (${alert.id})`,
        }))
      case 'observation':
        return (observationsQuery.data?.items ?? []).map((observation) => ({
          value: observation.id,
          label: `${observation.type} · ${observation.sourceId} (${observation.id})`,
        }))
      case 'assessment':
        return (assessmentsQuery.data?.items ?? []).map((assessment) => ({
          value: assessment.id,
          label: `${assessment.conclusion} (${assessment.id})`,
        }))
      default:
        return []
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title="Attach relation"
      description="Link an existing record to this incident."
      footer={
        <>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button
            onClick={() => void form.handleSubmit()}
            disabled={attachMutation.isPending || !selectedEntityId}
          >
            {attachMutation.isPending ? 'Attaching…' : 'Attach'}
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
        <form.Field name="kind">
          {(field) => (
            <Field label="Relation kind" htmlFor="relation-kind">
              <Select
                id="relation-kind"
                value={field.state.value}
                onValueChange={(value) => {
                  field.handleChange(value)
                  form.setFieldValue('entityId', '')
                }}
                options={KIND_OPTIONS}
              />
            </Field>
          )}
        </form.Field>

        <form.Field
          name="entityId"
          validators={{
            onSubmit: ({ value }) => (value ? undefined : 'Select a record to attach'),
          }}
        >
          {(field) => (
            <Field label="Record" htmlFor="relation-entity" required error={fieldError(field.state.meta.errors)}>
              <Select
                id="relation-entity"
                value={field.state.value}
                onValueChange={(value) => field.handleChange(value)}
                options={optionsForKind(selectedRelationKind)}
                placeholder="Select record…"
              />
            </Field>
          )}
        </form.Field>
      </form>
    </Dialog>
  )
}
