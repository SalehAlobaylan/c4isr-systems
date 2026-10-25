import { useWorkspace } from '@/lib/workspaceContext'

/**
 * Centralized TanStack Query key factory.
 *
 * Every key starts with a stable resource segment so WebSocket handlers can
 * invalidate a whole resource with a single prefix match.
 */
export function createQueryKeys(prefix: readonly unknown[]) {
  return {
    picture: (at?: string) => [...prefix, 'picture', at ?? 'live'] as const,
    pictureWindow: () => [...prefix, 'picture-window'] as const,
    health: () => [...prefix, 'health'] as const,
    currentOperator: () => [...prefix, 'current-operator'] as const,
    operators: () => [...prefix, 'operators'] as const,

    sources: {
      all: [...prefix, 'sources'] as const,
      list: (filters?: Record<string, unknown>) => [...prefix, 'sources', 'list', filters ?? {}] as const,
    },

    assets: {
      all: [...prefix, 'assets'] as const,
      list: (filters?: Record<string, unknown>) => [...prefix, 'assets', 'list', filters ?? {}] as const,
      detail: (id: string) => [...prefix, 'assets', 'detail', id] as const,
      telemetry: (id: string) => [...prefix, 'assets', 'telemetry', id] as const,
    },

    tracks: {
      all: [...prefix, 'tracks'] as const,
      list: (filters?: Record<string, unknown>) => [...prefix, 'tracks', 'list', filters ?? {}] as const,
      detail: (id: string) => [...prefix, 'tracks', 'detail', id] as const,
      history: (id: string) => [...prefix, 'tracks', 'history', id] as const,
      classifications: (id: string) => [...prefix, 'tracks', 'classifications', id] as const,
    },

    observations: {
      all: [...prefix, 'observations'] as const,
      list: (filters?: Record<string, unknown>) => [...prefix, 'observations', 'list', filters ?? {}] as const,
      detail: (id: string) => [...prefix, 'observations', 'detail', id] as const,
    },

    geofences: {
      all: [...prefix, 'geofences'] as const,
      list: () => [...prefix, 'geofences', 'list'] as const,
    },

    alerts: {
      all: [...prefix, 'alerts'] as const,
      list: (filters?: Record<string, unknown>) => [...prefix, 'alerts', 'list', filters ?? {}] as const,
      detail: (id: string) => [...prefix, 'alerts', 'detail', id] as const,
    },

    incidents: {
      all: [...prefix, 'incidents'] as const,
      list: (filters?: Record<string, unknown>) => [...prefix, 'incidents', 'list', filters ?? {}] as const,
      detail: (id: string) => [...prefix, 'incidents', 'detail', id] as const,
    },

    missions: {
      all: [...prefix, 'missions'] as const,
      list: (filters?: Record<string, unknown>) => [...prefix, 'missions', 'list', filters ?? {}] as const,
      detail: (id: string) => [...prefix, 'missions', 'detail', id] as const,
    },

    commands: {
      all: [...prefix, 'commands'] as const,
      list: (filters?: Record<string, unknown>) => [...prefix, 'commands', 'list', filters ?? {}] as const,
      detail: (id: string) => [...prefix, 'commands', 'detail', id] as const,
    },

    assessments: {
      all: [...prefix, 'assessments'] as const,
      list: (filters?: Record<string, unknown>) => [...prefix, 'assessments', 'list', filters ?? {}] as const,
    },

    audit: {
      all: [...prefix, 'audit'] as const,
      list: (filters?: Record<string, unknown>) => [...prefix, 'audit', 'list', filters ?? {}] as const,
    },

    scenarios: {
      all: [...prefix, 'scenarios'] as const,
      list: () => [...prefix, 'scenarios', 'list'] as const,
      detail: (name: string) => [...prefix, 'scenarios', 'detail', name] as const,
      runs: (filters?: Record<string, unknown>) => [...prefix, 'scenarios', 'runs', filters ?? {}] as const,
      run: (id: string) => [...prefix, 'scenarios', 'run', id] as const,
      events: (id: string) => [...prefix, 'scenarios', 'run', id, 'events'] as const,
    },
  } as const
}

export function useQueryKeys() { return createQueryKeys(useWorkspace().prefix) }
