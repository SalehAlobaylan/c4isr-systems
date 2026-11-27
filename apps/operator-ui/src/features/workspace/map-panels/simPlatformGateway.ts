/**
 * Unified Go + Python Simulation Platform Gateway client.
 *
 * Automatically prefers the authenticated Go Simulation Gateway
 * (`/api/v1/sim-platform/*` with RBAC, bounded concurrency, PostGIS/S3
 * persistence, SSE streaming, and wire-level truth isolation) whenever
 * `c4isr-server` is reachable and connected to the Python compute hub,
 * and seamlessly falls back to Vite's zero-Docker local hub proxy
 * (`/sim-api`, `/cov-api`, `/salvo-api`, `/bench-api`) otherwise.
 */

export type SimPlatformEngine = 'sim' | 'coverage' | 'salvo' | 'bench'

interface EmadRuntimeConfig {
  apiBaseUrl?: string
  apiToken?: string
  localAutoEntry?: boolean
}

const LEGACY_PROXY_PREFIX: Record<SimPlatformEngine, string> = {
  sim: '/sim-api',
  coverage: '/cov-api',
  salvo: '/salvo-api',
  bench: '/bench-api',
}

let gatewayProbeCache: { ok: boolean; checkedAt: number } | null = null
let gatewayProbeInflight: Promise<boolean> | null = null

export function getEmadApiToken(): string | undefined {
  if (typeof window === 'undefined') return undefined
  const cfg = (window as unknown as { __C4ISR_CONFIG__?: EmadRuntimeConfig }).__C4ISR_CONFIG__
  return cfg?.apiToken || undefined
}

export function resetSimGatewayProbe(): void {
  gatewayProbeCache = null
}

export async function isSimGatewayActive(forceRefresh = false): Promise<boolean> {
  const now = Date.now()
  if (!forceRefresh && gatewayProbeCache && now - gatewayProbeCache.checkedAt < 4000) {
    return gatewayProbeCache.ok
  }
  if (gatewayProbeInflight) return gatewayProbeInflight

  gatewayProbeInflight = (async () => {
    const token = getEmadApiToken()
    const ctrl = new AbortController()
    const timer = window.setTimeout(() => ctrl.abort(), 1200)
    try {
      const headers: Record<string, string> = {}
      if (token) headers.Authorization = `Bearer ${token}`
      const res = await fetch('/api/v1/sim-platform/health', {
        headers,
        cache: 'no-store',
        signal: ctrl.signal,
      })
      if (!res.ok) {
        gatewayProbeCache = { ok: false, checkedAt: Date.now() }
        return false
      }
      const data = (await res.json()) as { gateway?: string; workerReachable?: boolean }
      const ok = data.gateway === 'emad-go-simgateway' && Boolean(data.workerReachable)
      gatewayProbeCache = { ok, checkedAt: Date.now() }
      return ok
    } catch {
      gatewayProbeCache = { ok: false, checkedAt: Date.now() }
      return false
    } finally {
      window.clearTimeout(timer)
      gatewayProbeInflight = null
    }
  })()

  return gatewayProbeInflight
}

export async function simPlatformJson<T>(
  engine: SimPlatformEngine,
  subPath: string,
  init?: RequestInit,
  serviceLabel = 'Simulation service',
): Promise<T> {
  const useGateway = await isSimGatewayActive()
  const url = useGateway
    ? `/api/v1/sim-platform/${engine}${subPath}`
    : `${LEGACY_PROXY_PREFIX[engine]}/api${subPath}`

  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...((init?.headers as Record<string, string> | undefined) ?? {}),
  }
  if (useGateway) {
    const token = getEmadApiToken()
    if (token) headers.Authorization = `Bearer ${token}`
  }

  const res = await fetch(url, { ...init, headers })
  const text = await res.text()
  let parsed: unknown = null
  try {
    parsed = text ? JSON.parse(text) : null
  } catch {
    // Non-JSON proxy error
  }
  if (!res.ok) {
    const errField =
      parsed !== null && typeof parsed === 'object' && 'error' in parsed
        ? String((parsed as { error: unknown }).error)
        : parsed !== null && typeof parsed === 'object' && 'message' in parsed
          ? String((parsed as { message: unknown }).message)
          : ''
    throw new Error(errField || `${serviceLabel} returned HTTP ${res.status}`)
  }
  return parsed as T
}

export async function resolveSimViewerOrigin(
  engine: 'sim' | 'coverage' | 'salvo',
  reportedOrigin: string,
): Promise<string> {
  if (await isSimGatewayActive()) {
    return `${window.location.origin}/sim-viewers/${engine}`
  }
  return reportedOrigin.replace(/\/$/, '')
}

export function buildSimViewerHref(viewerUrl: string): string {
  const url = new URL(viewerUrl, window.location.origin)
  url.searchParams.set('mapOrigin', window.location.origin)
  return url.toString()
}

export async function startUnifiedLocalSimHub(
  controlPath = '/__local/sim/start',
  manualHint = 'task dev:sim-hub',
): Promise<{ ok: boolean; origin?: string; error?: string }> {
  resetSimGatewayProbe()
  const res = await fetch(controlPath, { method: 'POST' })
  const text = await res.text()
  try {
    const parsed = JSON.parse(text) as { ok?: boolean; origin?: string; error?: string }
    resetSimGatewayProbe()
    return { ok: Boolean(res.ok && parsed.ok), origin: parsed.origin, error: parsed.error }
  } catch {
    return {
      ok: false,
      error: `Auto-start endpoint unavailable (HTTP ${res.status}). Start manually with "${manualHint}".`,
    }
  }
}

export async function restartUnifiedLocalSimHub(
  controlPath = '/__local/sim/restart',
): Promise<{ ok: boolean; origin?: string; error?: string }> {
  resetSimGatewayProbe()
  const res = await fetch(controlPath, { method: 'POST' })
  const text = await res.text()
  try {
    const parsed = JSON.parse(text) as { ok?: boolean; origin?: string; error?: string }
    resetSimGatewayProbe()
    return { ok: Boolean(res.ok && parsed.ok), origin: parsed.origin, error: parsed.error }
  } catch {
    return { ok: false, error: `Restart endpoint unavailable (HTTP ${res.status}).` }
  }
}

/**
 * Subscribe to a running simulation job using Go SSE when connected through
 * `c4isr-server`, falling back smoothly to 350ms polling in zero-Docker mode.
 */
export function subscribeSimPlatformRun<T extends { status: string }>(
  engine: 'sim' | 'coverage' | 'salvo',
  jobId: string,
  fetchOne: (id: string) => Promise<T>,
  onUpdate: (job: T) => void,
  onTerminal?: (job: T) => void,
): () => void {
  let cancelled = false
  let pollTimer: number | null = null
  const abort = new AbortController()

  const startPolling = () => {
    if (cancelled || pollTimer !== null) return
    pollTimer = window.setInterval(() => {
      void fetchOne(jobId)
        .then((updated) => {
          if (cancelled) return
          onUpdate(updated)
          if (updated.status !== 'running' && updated.status !== 'queued') {
            if (pollTimer !== null) {
              window.clearInterval(pollTimer)
              pollTimer = null
            }
            onTerminal?.(updated)
          }
        })
        .catch(() => {
          /* retry next tick */
        })
    }, 350)
  }

  void (async () => {
    const useGateway = await isSimGatewayActive()
    if (cancelled) return
    if (!useGateway) {
      startPolling()
      return
    }
    try {
      const headers: Record<string, string> = { Accept: 'text/event-stream' }
      const token = getEmadApiToken()
      if (token) headers.Authorization = `Bearer ${token}`
      const res = await fetch(
        `/api/v1/sim-platform/${engine}/runs/${encodeURIComponent(jobId)}/events`,
        { headers, signal: abort.signal },
      )
      if (!res.ok || !res.body) {
        startPolling()
        return
      }
      const reader = res.body.getReader()
      const dec = new TextDecoder()
      let buf = ''
      while (!cancelled) {
        const { value, done } = await reader.read()
        if (done) break
        buf += dec.decode(value, { stream: true })
        const frames = buf.split('\n\n')
        buf = frames.pop() ?? ''
        for (const frame of frames) {
          const dataLine = frame.split('\n').find((l) => l.startsWith('data:'))
          if (!dataLine) continue
          try {
            const parsed = JSON.parse(dataLine.slice(5).trim()) as { job?: T }
            if (!parsed.job) continue
            onUpdate(parsed.job)
            if (parsed.job.status !== 'running' && parsed.job.status !== 'queued') {
              // Fetch authoritative full record (including resultOverlay) on completion
              const full = await fetchOne(jobId).catch(() => parsed.job as T)
              if (!cancelled) {
                onUpdate(full)
                onTerminal?.(full)
              }
              return
            }
          } catch {
            // Ignore malformed keepalive frames
          }
        }
      }
      if (!cancelled) startPolling()
    } catch {
      if (!cancelled) startPolling()
    }
  })()

  return () => {
    cancelled = true
    abort.abort()
    if (pollTimer !== null) {
      window.clearInterval(pollTimer)
      pollTimer = null
    }
  }
}
