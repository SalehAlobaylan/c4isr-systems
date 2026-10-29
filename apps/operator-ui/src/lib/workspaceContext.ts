import { createContext, useContext } from 'react'

import type { CurrentOperatorResponse, createAPI } from '@/lib/api'

export type Resource = CurrentOperatorResponse['resource']
export const resourceKey = (r: Resource) => JSON.stringify([r.workspaceId, r.environmentId, r.partition, r.runId ?? ''])
export const isCOPResource = (r?: Resource): r is Resource => Boolean(r &&
  r.workspaceId && r.environmentId && (r.partition === 'PUBLIC_EVIDENCE' ||
    (r.partition === 'EXERCISE_OBSERVATION' && r.runId)))

export interface WorkspaceSession {
  session: CurrentOperatorResponse
  api: ReturnType<typeof createAPI>
  prefix: readonly unknown[]
  signal: AbortSignal
  select: (resource: Resource, preserveRoute?:boolean) => Promise<void>
  revalidate: () => Promise<void>
  invalidate: () => void
  connectionWarning?: string
}

export const WorkspaceContext = createContext<WorkspaceSession | null>(null)
export function useWorkspace() {
  const value = useContext(WorkspaceContext)
  if (!value) throw new Error('An authorized workspace is required')
  return value
}
