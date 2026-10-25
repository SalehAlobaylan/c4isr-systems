import { useState, type Dispatch, type SetStateAction } from 'react'
import { resourceKey, useWorkspace } from '@/lib/workspaceContext'

/** Versioned, identity/resource-scoped UI preferences contain no evidence bytes. */
export function useWorkspacePreference<T>(name: string, fallback: T, valid: (value: unknown) => value is T): [T, Dispatch<SetStateAction<T>>] {
  const { session } = useWorkspace()
  const key = `emad.ux.v2.${session.principal.id}.${resourceKey(session.resource)}.${name}`
  const [value, setValue] = useState<T>(() => { try { const saved: unknown = JSON.parse(localStorage.getItem(key) ?? 'null'); return valid(saved) ? saved : fallback } catch { return fallback } })
  const save: Dispatch<SetStateAction<T>> = next => setValue(previous => {
    const result = typeof next === 'function' ? (next as (v:T)=>T)(previous) : next
    try { localStorage.setItem(key, JSON.stringify(result)) } catch { /* A full/disabled store must not block work. */ }
    return result
  })
  return [value, save]
}

export const dateLabel = (value?: string) => !value || value.startsWith('0001') ? 'Time not supplied' : new Date(value).toLocaleDateString(undefined, { day: 'numeric', month: 'short', year: 'numeric' })
