/**
 * Centralized TanStack Query key factory.
 *
 * Every key starts with a stable resource segment so WebSocket handlers can
 * invalidate a whole resource with a single prefix match.
 */
export const queryKeys = {
  health: () => ['health'] as const,
  operators: () => ['operators'] as const,

  sources: {
    all: ['sources'] as const,
    list: (filters?: Record<string, unknown>) => ['sources', 'list', filters ?? {}] as const,
  },

  assets: {
    all: ['assets'] as const,
    list: (filters?: Record<string, unknown>) => ['assets', 'list', filters ?? {}] as const,
    detail: (id: string) => ['assets', 'detail', id] as const,
    telemetry: (id: string) => ['assets', 'telemetry', id] as const,
  },

  tracks: {
    all: ['tracks'] as const,
    list: (filters?: Record<string, unknown>) => ['tracks', 'list', filters ?? {}] as const,
    detail: (id: string) => ['tracks', 'detail', id] as const,
    history: (id: string) => ['tracks', 'history', id] as const,
    classifications: (id: string) => ['tracks', 'classifications', id] as const,
  },

  observations: {
    all: ['observations'] as const,
    list: (filters?: Record<string, unknown>) => ['observations', 'list', filters ?? {}] as const,
    detail: (id: string) => ['observations', 'detail', id] as const,
  },

  geofences: {
    all: ['geofences'] as const,
    list: () => ['geofences', 'list'] as const,
  },

  alerts: {
    all: ['alerts'] as const,
    list: (filters?: Record<string, unknown>) => ['alerts', 'list', filters ?? {}] as const,
    detail: (id: string) => ['alerts', 'detail', id] as const,
  },

  incidents: {
    all: ['incidents'] as const,
    list: (filters?: Record<string, unknown>) => ['incidents', 'list', filters ?? {}] as const,
    detail: (id: string) => ['incidents', 'detail', id] as const,
  },

  missions: {
    all: ['missions'] as const,
    list: (filters?: Record<string, unknown>) => ['missions', 'list', filters ?? {}] as const,
    detail: (id: string) => ['missions', 'detail', id] as const,
  },

  commands: {
    all: ['commands'] as const,
    list: (filters?: Record<string, unknown>) => ['commands', 'list', filters ?? {}] as const,
    detail: (id: string) => ['commands', 'detail', id] as const,
  },

  assessments: {
    all: ['assessments'] as const,
    list: (filters?: Record<string, unknown>) => ['assessments', 'list', filters ?? {}] as const,
  },

  audit: {
    all: ['audit'] as const,
    list: (filters?: Record<string, unknown>) => ['audit', 'list', filters ?? {}] as const,
  },

  scenarios: {
    all: ['scenarios'] as const,
    list: () => ['scenarios', 'list'] as const,
    detail: (name: string) => ['scenarios', 'detail', name] as const,
    runs: (filters?: Record<string, unknown>) => ['scenarios', 'runs', filters ?? {}] as const,
  },
} as const
