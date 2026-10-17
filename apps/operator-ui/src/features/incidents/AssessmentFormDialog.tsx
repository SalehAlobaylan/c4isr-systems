import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useForm } from '@tanstack/react-form'
import { useEffect } from 'react'

import { Field, Textarea, fieldError } from '@/components/shared/form'
import { Button } from '@/components/ui/button'
import { Dialog } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Select, type SelectOption } from '@/components/ui/select'
import { api } from '@/lib/api'
import { queryKeys } from '@/lib/queryKeys'
import { toast } from '@/stores/toasts'

/**
 * Authoring surface for operator assessments.
 *
 * The platform stored assessments and granted `assessments.create` to analyst
 * and supervisor, but offered no way to write one - so the most distinctive
 * claim the system makes ("why does it believe what it presents?") could be
 * read for machine-generated classifications and never stated by a human.
 *
 * Method is fixed to OPERATOR rather than offered as a choice. An assessment
 * written in this dialog is by definition one person's judgement; allowing it
 * to claim RULE, ALGORITHM, or AI provenance would let a human record be
 * indistinguishable from a derived one, which is the opposite of what the
 * evidence model exists to guarantee.
 */

const METHOD_OPERATOR = 'OPERATOR'

const TYPE_OPTIONS: SelectOption[] = [
  { value: 'identification', label: 'Identification' },
  { value: 'correlation', label: 'Correlation' },
  { value: 'threat', label: 'Threat' },
  { value: 'intent', label: 'Intent' },
  { value: 'capability', label: 'Capability' },
  { value: 'reliability', label: 'Source reliability' },
  { value: 'other', label: 'Other' },
]

const EVIDENCE_TYPE_OPTIONS: SelectOption[] = [
  { value: 'observation', label: 'Observation' },
  { value: 'track', label: 'Track' },
  { value: 'asset', label: 'Asset' },
  { value: 'alert', label: 'Alert' },
  { value: 'incident', label: 'Incident' },
  { value: 'source', label: 'Source' },
  { value: 'classification', label: 'Classification' },
]

export function AssessmentFormDialog({
  open,
  onOpenChange,
  subjectType,
  subjectId,
  incidentId,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** Defaults the subject; the dialog can still be pointed elsewhere. */
  subjectType?: string
  subjectId?: string
  /** When set, the assessment is also attached to this incident. */
  incidentId?: string
}) {
  const queryClient = useQueryClient()

  const tracksQuery = useQuery({
    queryKey: queryKeys.tracks.list({ limit: 200 }),
    queryFn: () => api.listTracks({ limit: 200 }),
    enabled: open && !subjectId,
  })
  const assetsQuery = useQuery({
    queryKey: queryKeys.assets.list({ limit: 200 }),
    queryFn: () => api.listAssets({ limit: 200 }),
    enabled: open && !subjectId,
  })
  const alertsQuery = useQuery({
    queryKey: queryKeys.alerts.list({ limit: 100 }),
    queryFn: () => api.listAlerts({ limit: 100 }),
    enabled: open,
  })

  const createMutation = useMutation({
    mutationFn: api.createAssessment,
    onSuccess: (assessment) => {
      toast({ title: 'Assessment recorded', description: assessment.conclusion, variant: 'success' })
      void queryClient.invalidateQueries({ queryKey: queryKeys.assessments.all })
      void queryClient.invalidateQueries({ queryKey: queryKeys.tracks.all })
      void queryClient.invalidateQueries({ queryKey: queryKeys.audit.all })
      if (incidentId) {
        void queryClient.invalidateQueries({ queryKey: queryKeys.incidents.detail(incidentId) })
      }
      onOpenChange(false)
    },
    onError: (error) => {
      toast({
        title: 'Assessment could not be saved',
        description: error instanceof Error ? error.message : undefined,
        variant: 'error',
      })
    },
  })

  const form = useForm({
    defaultValues: {
      subjectType: subjectType ?? 'track',
      subjectId: subjectId ?? '',
      type: 'identification',
      conclusion: '',
      confidence: '',
      evidenceType: 'observation',
      evidenceId: '',
    },
    onSubmit: async ({ value }) => {
      const confidence = value.confidence.trim()
      const evidenceId = value.evidenceId.trim()
      await createMutation.mutateAsync({
        subjectType: value.subjectType,
        subjectId: value.subjectId.trim(),
        type: value.type,
        conclusion: value.conclusion.trim(),
        method: METHOD_OPERATOR,
        // An empty box means "not stated", which is a different claim from
        // zero confidence, so it is sent as absent rather than 0.
        confidence: confidence === '' ? undefined : Number(confidence),
        evidence: evidenceId ? [{ type: value.evidenceType, id: evidenceId }] : [],
      })
    },
  })

  useEffect(() => {
    if (open) {
      form.reset({
        subjectType: subjectType ?? 'track',
        subjectId: subjectId ?? '',
        type: 'identification',
        conclusion: '',
        confidence: '',
        evidenceType: 'observation',
        evidenceId: '',
      })
    }
    // form.reset is stable across renders; the open flag is the trigger.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, subjectType, subjectId])

  const subjectOptions: SelectOption[] =
    form.state.values.subjectType === 'asset'
      ? (assetsQuery.data?.items ?? []).map((asset) => ({
          value: asset.id,
          label: `${asset.name} (${asset.id})`,
        }))
      : (tracksQuery.data?.items ?? []).map((track) => ({
          value: track.id,
          label: `${track.externalRef || 'track'} (${track.id})`,
        }))

  const evidenceOptions: SelectOption[] =
    form.state.values.evidenceType === 'alert'
      ? (alertsQuery.data?.items ?? []).map((alert) => ({ value: alert.id, label: alert.title }))
      : []

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title="Record assessment"
      description="State what you conclude, and what it rests on."
      wide
      footer={
        <>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button variant="primary" onClick={() => void form.handleSubmit()} disabled={createMutation.isPending}>
            {createMutation.isPending ? 'Saving…' : 'Record assessment'}
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
        <div className="grid grid-cols-2 gap-4">
          <form.Field
            name="subjectType"
            validators={{
              onChange: ({ value }) => (value ? undefined : 'Subject type is required'),
            }}
          >
            {(field) => (
              <Field label="Subject type" htmlFor="assessment-subject-type">
                <Select
                  id="assessment-subject-type"
                  value={field.state.value}
                  onValueChange={(value) => {
                    field.handleChange(value)
                    if (!subjectId) form.setFieldValue('subjectId', '')
                  }}
                  options={[
                    { value: 'track', label: 'Track' },
                    { value: 'asset', label: 'Asset' },
                    { value: 'incident', label: 'Incident' },
                    { value: 'observation', label: 'Observation' },
                    { value: 'source', label: 'Source' },
                  ]}
                />
              </Field>
            )}
          </form.Field>

          <form.Field
            name="subjectId"
            validators={{
              onSubmit: ({ value }) => (value.trim() ? undefined : 'Subject is required'),
            }}
          >
            {(field) => (
              <Field
                label="Subject"
                htmlFor="assessment-subject"
                required
                error={fieldError(field.state.meta.errors)}
              >
                {subjectOptions.length > 0 ? (
                  <Select
                    id="assessment-subject"
                    value={field.state.value}
                    onValueChange={(value) => field.handleChange(value)}
                    options={subjectOptions}
                  />
                ) : (
                  <Input
                    id="assessment-subject"
                    value={field.state.value}
                    onBlur={field.handleBlur}
                    onChange={(event) => field.handleChange(event.target.value)}
                    placeholder="Subject id"
                  />
                )}
              </Field>
            )}
          </form.Field>
        </div>

        <div className="grid grid-cols-2 gap-4">
          <form.Field
            name="type"
            validators={{ onChange: ({ value }) => (value ? undefined : 'Type is required') }}
          >
            {(field) => (
              <Field label="Assessment type" htmlFor="assessment-type" required>
                <Select
                  id="assessment-type"
                  value={field.state.value}
                  onValueChange={(value) => field.handleChange(value)}
                  options={TYPE_OPTIONS}
                />
              </Field>
            )}
          </form.Field>

          <form.Field
            name="confidence"
            validators={{
              onSubmit: ({ value }) => {
                const raw = value.trim()
                if (raw === '') return undefined
                const parsed = Number(raw)
                if (Number.isNaN(parsed) || parsed < 0 || parsed > 1) {
                  return 'Confidence must be between 0 and 1'
                }
                return undefined
              },
            }}
          >
            {(field) => (
              <Field
                label="Confidence"
                htmlFor="assessment-confidence"
                hint="Optional. Leave blank if you cannot quantify it."
                error={fieldError(field.state.meta.errors)}
              >
                <Input
                  id="assessment-confidence"
                  inputMode="decimal"
                  value={field.state.value}
                  onBlur={field.handleBlur}
                  onChange={(event) => field.handleChange(event.target.value)}
                  placeholder="0.7"
                />
              </Field>
            )}
          </form.Field>
        </div>

        <form.Field
          name="conclusion"
          validators={{
            onSubmit: ({ value }) =>
              value.trim() ? undefined : 'A conclusion is required - state what you believe',
          }}
        >
          {(field) => (
            <Field
              label="Conclusion"
              htmlFor="assessment-conclusion"
              required
              error={fieldError(field.state.meta.errors)}
            >
              <Textarea
                id="assessment-conclusion"
                value={field.state.value}
                onBlur={field.handleBlur}
                onChange={(event) => field.handleChange(event.target.value)}
                placeholder="Track-42 likely represents the same entity observed by Source-A and Source-B."
              />
            </Field>
          )}
        </form.Field>

        <div className="grid grid-cols-2 gap-4">
          <form.Field name="evidenceType">
            {(field) => (
              <Field label="Supporting evidence" htmlFor="assessment-evidence-type" hint="Optional">
                <Select
                  id="assessment-evidence-type"
                  value={field.state.value}
                  onValueChange={(value) => {
                    field.handleChange(value)
                    form.setFieldValue('evidenceId', '')
                  }}
                  options={EVIDENCE_TYPE_OPTIONS}
                />
              </Field>
            )}
          </form.Field>

          <form.Field name="evidenceId">
            {(field) => (
              <Field label="Evidence id" htmlFor="assessment-evidence-id">
                {evidenceOptions.length > 0 ? (
                  <Select
                    id="assessment-evidence-id"
                    value={field.state.value}
                    onValueChange={(value) => field.handleChange(value)}
                    options={evidenceOptions}
                  />
                ) : (
                  <Input
                    id="assessment-evidence-id"
                    value={field.state.value}
                    onBlur={field.handleBlur}
                    onChange={(event) => field.handleChange(event.target.value)}
                    placeholder="Evidence id"
                  />
                )}
              </Field>
            )}
          </form.Field>
        </div>
      </form>
    </Dialog>
  )
}