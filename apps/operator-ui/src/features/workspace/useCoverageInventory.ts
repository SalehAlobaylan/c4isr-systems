import { useQuery } from '@tanstack/react-query'
import {
  useAPI,
  type AreaSelection,
  type CoverageInventoryResult,
  type InventoryBounds,
  type InventoryFootprint,
  type InventorySource,
  type InventoryTimeBucket,
  type ScopeInput,
  type TemporalScope,
} from '@/lib/api'
import { useWorkspace } from '@/lib/workspaceContext'
import { monitoringPeriod, rectangleGeometry } from './AnalystScopeProvider'

const LIVE_ONLY_FAMILIES = new Set(['weather', 'bulletins', 'aircraft', 'vessels'])
const STATIC_FAMILIES = new Set(['reference', 'geography', 'elevation', 'coverage'])
const MAX_JUMP_SPAN_MS = 365 * 86400_000

function activeTimezone(): string {
  return Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC'
}

export function inventoryAreaForBounds(bounds: InventoryBounds, name: string): AreaSelection {
  const spanLng = bounds.east - bounds.west
  const spanLat = bounds.north - bounds.south
  if (!Number.isFinite(spanLng) || !Number.isFinite(spanLat) || spanLng <= 0 || spanLat <= 0 || spanLng >= 359 || spanLat >= 179) {
    return { kind: 'world' }
  }
  return {
    kind: 'viewport',
    name: name.trim() || 'Approved footprint',
    geometry: rectangleGeometry(bounds),
  }
}

export function scopeForFootprint(
  footprint: InventoryFootprint,
  currentTemporal?: TemporalScope,
): ScopeInput {
  const area = inventoryAreaForBounds(footprint.bounds, footprint.name)
  if (currentTemporal) {
    return { area, temporal: currentTemporal }
  }
  return { area, temporal: monitoringPeriod(1) }
}

export function temporalForBucket(bucket: InventoryTimeBucket, timezone = activeTimezone()): TemporalScope {
  const families = Object.keys(bucket.byFamily ?? {})
  const onlyLive = families.length > 0 && families.every((f) => LIVE_ONLY_FAMILIES.has(f))
  const toMs = Date.parse(bucket.to)
  const isCurrentWindow = Number.isFinite(toMs) && Date.now() - toMs <= 30 * 86400_000
  if (onlyLive && isCurrentWindow) {
    const spanDays = Math.max(1, Math.min(30, Math.ceil((Date.now() - Date.parse(bucket.from)) / 86400_000)))
    return {
      mode: 'monitoring',
      rollingSeconds: spanDays * 86400,
      revisionView: 'latest',
      timezone,
      backgroundImagery: false,
    }
  }
  return {
    mode: 'historical',
    interval: { from: bucket.from, to: bucket.to },
    revisionView: 'latest',
    timezone,
    backgroundImagery: false,
  }
}

export function scopeForBucket(
  bucket: InventoryTimeBucket,
  currentArea?: AreaSelection,
  timezone = activeTimezone(),
): ScopeInput {
  const area = bucket.bounds
    ? inventoryAreaForBounds(bucket.bounds, bucket.suggestedAreaName || `${bucket.label} retained footprint`)
    : currentArea ?? { kind: 'world' }
  return {
    area,
    temporal: temporalForBucket(bucket, timezone),
  }
}

export function scopeForInventorySource(
  source: InventorySource,
  currentScope?: ScopeInput,
  timezone = activeTimezone(),
): ScopeInput {
  const area = source.bounds
    ? inventoryAreaForBounds(source.bounds, source.name)
    : currentScope?.area ?? { kind: 'world' }

  if (STATIC_FAMILIES.has(source.family)) {
    return {
      area,
      temporal: currentScope?.temporal ?? monitoringPeriod(1),
    }
  }

  if (LIVE_ONLY_FAMILIES.has(source.family)) {
    const latestMs = source.latestAt ? Date.parse(source.latestAt) : NaN
    const ageDays = Number.isFinite(latestMs) ? Math.max(1, Math.ceil((Date.now() - latestMs) / 86400_000)) : 1
    const rollingDays = ageDays <= 1 ? 1 : ageDays <= 7 ? 7 : Math.min(30, ageDays + 1)
    return {
      area,
      temporal: {
        mode: 'monitoring',
        rollingSeconds: rollingDays * 86400,
        revisionView: 'latest',
        timezone,
        backgroundImagery: false,
      },
    }
  }

  const earliestMs = source.earliestAt ? Date.parse(source.earliestAt) : NaN
  const latestMs = source.latestAt ? Date.parse(source.latestAt) : NaN
  if (!Number.isFinite(latestMs)) {
    return { area, temporal: currentScope?.temporal ?? monitoringPeriod(7) }
  }

  const now = Date.now()
  if (now - latestMs <= 86400_000 && (!Number.isFinite(earliestMs) || now - earliestMs <= 7 * 86400_000)) {
    const days = Number.isFinite(earliestMs) && now - earliestMs > 86400_000 ? 7 : 1
    return {
      area,
      temporal: {
        mode: 'monitoring',
        rollingSeconds: days * 86400,
        revisionView: 'latest',
        timezone,
        backgroundImagery: false,
      },
    }
  }

  // Build an exact start-inclusive, end-exclusive historical interval strictly containing latestAt.
  const toMs = Math.min(now + 60_000, latestMs + 3600_000)
  const rawFromMs = Number.isFinite(earliestMs) ? Math.min(earliestMs, toMs - 3600_000) : toMs - 7 * 86400_000
  const fromMs = Math.max(rawFromMs, toMs - MAX_JUMP_SPAN_MS)
  return {
    area,
    temporal: {
      mode: 'historical',
      interval: {
        from: new Date(fromMs).toISOString(),
        to: new Date(toMs).toISOString(),
      },
      revisionView: 'latest',
      timezone,
      backgroundImagery: false,
    },
  }
}

export function formatInventoryDateRange(earliestAt?: string, latestAt?: string): string {
  if (!earliestAt && !latestAt) return 'Reference · undated'
  const fmt = (iso: string) => {
    const d = new Date(iso)
    return Number.isNaN(d.getTime())
      ? iso
      : d.toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: '2-digit' })
  }
  if (earliestAt && latestAt) {
    const a = fmt(earliestAt)
    const b = fmt(latestAt)
    return a === b ? a : `${a} → ${b}`
  }
  return fmt(latestAt ?? earliestAt!)
}

export function useCoverageInventory(enabled = true) {
  const api = useAPI()
  const { prefix, session } = useWorkspace()
  const allowed =
    enabled &&
    session.resource.partition === 'PUBLIC_EVIDENCE' &&
    session.permissions.includes('sources.read')

  return useQuery<CoverageInventoryResult>({
    queryKey: [...prefix, 'coverage-inventory', session.accessRevision],
    enabled: allowed,
    retry: false,
    staleTime: 30_000,
    refetchOnWindowFocus: false,
    queryFn: ({ signal }) => api.getCoverageInventory(signal),
  })
}
