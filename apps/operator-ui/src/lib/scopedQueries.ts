import {
  useQuery as useBaseQuery, useMutation as useBaseMutation,
  type DefaultError, type QueryKey, type UseQueryOptions, type UseMutationOptions,
} from '@tanstack/react-query'
import { useWorkspace } from '@/lib/workspaceContext'

// A missing capability must not start a forbidden background request. Server
// authorization remains authoritative for every request and mutation.
export function useQuery<TQueryFnData = unknown, TError = DefaultError, TData = TQueryFnData, TKey extends QueryKey = QueryKey>(
  options: UseQueryOptions<TQueryFnData, TError, TData, TKey>,
) {
  const { session, signal, prefix } = useWorkspace()
  const domain = String(options.queryKey[prefix.length] ?? '')
  let permission = ['health', 'current-operator'].includes(domain) ? undefined : `${domain}.read`
  if (['context-records', 'context-record', 'context-history'].includes(domain)) permission = 'sources.read'
  if (domain === 'report-assessments') permission = 'assessments.read'
  if (domain === 'public-events') permission = 'observations.read'
  if (domain === 'reference-datasets') permission = 'sources.read'
  if (['imagery-scenes', 'imagery-jobs', 'imagery-job', 'imagery-result', 'imagery-attempts'].includes(domain)) permission = 'sources.read'
  if (domain === 'imagery-assessments' || domain === 'assessment-history') permission = 'assessments.read'
  if (['external-tracks', 'external-track'].includes(domain)) permission = 'tracks.read'
  if (domain === 'external-associations') permission = 'observations.read'
  if (domain === 'context-packages') permission = 'context.export'
  if (domain === 'picture' || domain === 'picture-window') permission = 'operational.read'
  if (domain === 'sensor-presets') permission = 'sim.scenarios.author'
  if (domain === 'recordings') permission = session.resource.partition === 'EXERCISE_INPUT' ? 'sim.scenarios.author' : 'sim.recordings.read'
  if (domain === 'scenarios' && ['list', 'detail'].includes(String(options.queryKey[prefix.length + 1]))) permission = 'sim.scenarios.author'
  if (domain === 'scenarios' && options.queryKey.at(-1) === 'events') permission = 'sim.scenarios.author'
  if (domain === 'assets' && options.queryKey[prefix.length + 1] === 'telemetry') permission = 'telemetry.read'
  const exerciseOnly = ['assets', 'missions', 'commands', 'scenarios'].includes(domain)
  const allowed = !signal.aborted && (!permission || session.permissions.includes(permission)) &&
    (!exerciseOnly || session.resource.partition === 'EXERCISE_OBSERVATION')
  return useBaseQuery({ ...options, enabled: allowed ? options.enabled : false })
}

// Mutations already accepted by the server can finish after a workspace has
// unmounted. Their callbacks must never navigate or toast into the next one.
export function useMutation<TData = unknown, TError = DefaultError, TVariables = void, TContext = unknown>(
  options: UseMutationOptions<TData, TError, TVariables, TContext>,
) {
  const { signal } = useWorkspace()
  return useBaseMutation({ ...options,
    onSuccess: (...args) => { if (!signal.aborted) return options.onSuccess?.(...args) },
    onError: (...args) => { if (!signal.aborted) return options.onError?.(...args) },
    onSettled: (...args) => { if (!signal.aborted) return options.onSettled?.(...args) },
  })
}
