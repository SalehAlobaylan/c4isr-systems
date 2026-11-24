import { create } from 'zustand'
import type { InvestigationInput } from '@/lib/api'

export type CaseCitationKind = InvestigationInput['evidence'][number]['kind']

export interface PendingCaseCitation {
  kind: CaseCitationKind
  id: string
  label?: string
}

interface CaseCitationBridgeState {
  scopeKey: string
  pending: PendingCaseCitation | null
  attachedKeys: string[]
  queueAttachment: (scopeKey: string, citation: PendingCaseCitation) => void
  consumePending: (scopeKey: string) => PendingCaseCitation | null
  syncAttached: (scopeKey: string, keys: string[]) => void
  reset: (scopeKey: string) => void
}

export const citationKey = (kind: CaseCitationKind, id: string) => `${kind}:${id}`

/**
 * Lightweight bridge allowing map-canvas object selection cards to attach
 * exact retained references into the active Investigation draft without tight
 * component coupling. Scoped by principal/resource key so switching accounts
 * or workspaces clears any queued draft state immediately.
 */
export const useCaseCitationBridge = create<CaseCitationBridgeState>((set, get) => ({
  scopeKey: '',
  pending: null,
  attachedKeys: [],
  queueAttachment: (scopeKey, citation) =>
    set((state) => ({
      scopeKey,
      pending: citation,
      attachedKeys: state.scopeKey === scopeKey ? state.attachedKeys : [],
    })),
  consumePending: (scopeKey) => {
    const state = get()
    if (!state.pending || state.scopeKey !== scopeKey) return null
    const next = state.pending
    set({ pending: null })
    return next
  },
  syncAttached: (scopeKey, keys) =>
    set((state) => ({
      scopeKey,
      pending: state.scopeKey === scopeKey ? state.pending : null,
      attachedKeys: keys,
    })),
  reset: (scopeKey) => set({ scopeKey, pending: null, attachedKeys: [] }),
}))
