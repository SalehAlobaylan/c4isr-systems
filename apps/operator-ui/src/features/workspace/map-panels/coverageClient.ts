import type { Feature, FeatureCollection } from 'geojson'

export type CovPlacementTool = null | 'add-radar' | 'add-eo_ir' | 'add-acoustic' | 'add-jammer' | 'asset' | 'route-pt'
export type CovSensorType = 'radar' | 'eo_ir' | 'acoustic'
export type CovPresetId = 'baseline_multimodal' | 'dust_storm' | 'gap_filler_radar' | 'stealth_uav' | 'ew_jammed'

export interface CovJammerDraft {
  id: string
  label: string
  lon: number
  lat: number
  aglM: number
  erpRatio: number
  beamWidthDeg: number
}

export interface CovAdversarialRoute {
  routeId: string
  label: string
  aglM: number
  speedMps: number
  stealthFraction: number
  trackedFraction: number
  lengthM: number
  warningTimeS: number
  warningDistanceM: number
  pointsLonLat: [number, number, number][]
}

export interface CovPresetMeta {
  id: CovPresetId
  label: string
  summary: string
}

export const COV_TACTICAL_PRESETS: CovPresetMeta[] = [
  {
    id: 'baseline_multimodal',
    label: 'Multi-Modal Baseline',
    summary: 'Radar + EO/IR + Acoustic at defended site, clear air (10 km vis)',
  },
  {
    id: 'dust_storm',
    label: 'CAMS Dust Storm',
    summary: '1.5 km CAMS dust visibility + 14 m/s wind stressing EO/IR & acoustic range',
  },
  {
    id: 'gap_filler_radar',
    label: 'Gap-Filler (+1 Radar)',
    summary: 'Lock existing sites and let optimizer place 1 new gap-filler radar',
  },
  {
    id: 'stealth_uav',
    label: 'Low-RCS Micro-UAV',
    summary: 'Tiny 0.02 m² RCS target at 20–120 m AGL with rooftop siting',
  },
  {
    id: 'ew_jammed',
    label: 'Stand-Off EW Jammer',
    summary: 'Active stand-off RF jammer degrading radar burn-through along NW ingress',
  },
]

export interface CovSensorDraft {
  id: string
  type: CovSensorType
  label: string
  lon: number
  lat: number
  mastAglM: number
  maxRangeM: number
  minElevationDeg: number
  clutterFloorAglM: number
  sectorCenterDeg: number
  sectorWidthDeg: number
  locked: boolean
}

export interface CovAssetDraft {
  id: string
  label: string
  lon: number
  lat: number
  warningRadiusM: number
  priority: number
}

export interface CovRouteDraft {
  id: string
  label: string
  speedMps: number
  points: [number, number, number][] // [lon, lat, aglM]
}

export interface CovLayerDraft {
  aglM: number
  weight: number
}

export interface CovSetupDraft {
  environmentId: string
  title: string
  sensors: CovSensorDraft[]
  assets: CovAssetDraft[]
  routes: CovRouteDraft[]
  layers: CovLayerDraft[]
  threat: {
    rcsM2: number
    label: string
  }
  weather: {
    visibilityKm: number
    useEra5Wind: boolean
    surfaceWindMps: number
  }
  optimizer: {
    enabled: boolean
    extraSensors: number
    extraSensorType: CovSensorType
    minSeparationM: number
    allowRooftops: boolean
    swapPasses: number
    redundancyWeight: number
    routeWeight?: number
    optimizeOrientation?: boolean
  }
  jammers?: CovJammerDraft[]
  includeBuildings: boolean
  mapLayerAglM: number
}

export interface CovSensorDefaults {
  mastAglM: number
  maxRangeM: number
  minElevationDeg: number
  clutterFloorAglM: number
  requiresLos: boolean
  buildingsBlock: boolean
}

export interface CovEnvironment {
  id: string
  title: string
  description: string
  manifestSha256: string
  buildingCount: number
  roadCount: number
  hasGlb: boolean
  hasImagery: boolean
  boundsWgs84: [number, number, number, number]
  origin: [number, number, number]
  terrainRangeM: [number, number]
  obstructionGrid: { cellM: number; nx: number; ny: number; buildingCells: number; validFraction: number }
  weather: { surfaceWindMps: number; source: string }
  sensorDefaults: Record<CovSensorType, CovSensorDefaults>
  defaults: CovSetupDraft
}

export interface CovBlindCauses {
  aglM: number
  totalBlindKm2: number
  buildingBlockedKm2: number
  terrainBlockedKm2: number
  jammedKm2?: number
  outOfRangeOrSectorKm2: number
}

export interface CovConfigMetrics {
  objective: number
  aglM: number
  coverageFraction: number
  weightedCoverageFraction: number
  blindAreaKm2: number
  blindCauses?: CovBlindCauses | null
  priorityZoneCoverageFraction: number | null
  minWarningTimeS: number | null
  routes: { routeId: string; label: string; warningTimeS: number; trackedFraction: number }[]
}

export interface CovRunJob {
  jobId: string
  environmentId: string
  environmentTitle: string
  title: string
  status: 'running' | 'completed' | 'failed' | 'interrupted'
  stage: string
  createdAt: string
  completedAt: string | null
  viewerUrl: string
  hasGlb: boolean
  hasImagery: boolean
  sensorCount: number
  mapLayerAglM: number
  error: string | null
  summary: {
    layers: CovLayerDraft[]
    baseline: CovConfigMetrics | null
    optimized: CovConfigMetrics | null
    comparison: {
      objectiveDelta: number
      objectiveGainPct: number | null
      layers: { aglM: number; coverageDelta: number; weightedCoverageDelta: number; blindAreaDeltaKm2: number }[]
      routes: { routeId: string; warningTimeDeltaS: number; trackedFractionDelta: number }[]
    } | null
    optimizer: { enabled?: boolean; evaluations?: number; elapsedS?: number; note?: string | null }
    optimizedSensors: {
      id: string
      type: CovSensorType
      label: string
      lon: number
      lat: number
      origin: string
      siteOnRoof: boolean
    }[]
    elapsedS: number
  } | null
  resultOverlay: FeatureCollection | null
}

export interface CovHealth {
  status: string
  service: string
  engineVersion: string
  capabilities?: string[]
  viewerOrigin: string
  environmentCount: number
  runningJobs: number
  totalRuns: number
}

const COV_API_BASE = '/cov-api'

async function covJson<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${COV_API_BASE}${path}`, {
    ...init,
    headers: { 'Content-Type': 'application/json', ...(init?.headers ?? {}) },
  })
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
        : ''
    throw new Error(errField || `Sensor coverage service returned HTTP ${res.status}`)
  }
  return parsed as T
}

export const fetchCovHealth = () => covJson<CovHealth>('/api/health')
export const fetchCovEnvironments = async () =>
  (await covJson<{ environments: CovEnvironment[] }>('/api/environments')).environments
export const fetchCovRuns = async () => (await covJson<{ runs: CovRunJob[] }>('/api/runs')).runs
export const fetchCovRun = (jobId: string) => covJson<CovRunJob>(`/api/runs/${encodeURIComponent(jobId)}`)
export const startCovRun = (draft: CovSetupDraft, jobId?: string) =>
  covJson<CovRunJob>('/api/runs', {
    method: 'POST',
    body: JSON.stringify(jobId ? { ...draft, jobId } : draft),
  })

export const exportCovNetwork = (payload: {
  environmentId: string
  jobId?: string
  name?: string
  sensors?: unknown[]
}) =>
  covJson<{ layoutId: string; name: string; sensorCount: number }>('/api/export-network', {
    method: 'POST',
    body: JSON.stringify(payload),
  })

export const findCovAdversarialRoute = (payload: {
  environmentId: string
  aglM: number
  speedMps?: number
  includeBuildings?: boolean
  rcsM2?: number
  visibilityKm?: number
  surfaceWindMps?: number
  assetLon?: number
  assetLat?: number
  jammers?: CovJammerDraft[]
  sensors: unknown[]
}) =>
  covJson<CovAdversarialRoute>('/api/adversarial-route', {
    method: 'POST',
    body: JSON.stringify(payload),
  })

export async function startLocalCovService(): Promise<{ ok: boolean; origin?: string; error?: string }> {
  const res = await fetch('/__local/cov/start', { method: 'POST' })
  const text = await res.text()
  try {
    const parsed = JSON.parse(text) as { ok?: boolean; origin?: string; error?: string }
    return { ok: Boolean(res.ok && parsed.ok), origin: parsed.origin, error: parsed.error }
  } catch {
    return {
      ok: false,
      error: `Auto-start endpoint unavailable (HTTP ${res.status}). Start manually with "task dev:coverage".`,
    }
  }
}

export async function restartLocalCovService(): Promise<{ ok: boolean; origin?: string; error?: string }> {
  const res = await fetch('/__local/cov/restart', { method: 'POST' })
  const text = await res.text()
  try {
    const parsed = JSON.parse(text) as { ok?: boolean; origin?: string; error?: string }
    return { ok: Boolean(res.ok && parsed.ok), origin: parsed.origin, error: parsed.error }
  } catch {
    return { ok: false, error: `Restart endpoint unavailable (HTTP ${res.status}).` }
  }
}

export function cloneCovDraft(draft: CovSetupDraft): CovSetupDraft {
  return JSON.parse(JSON.stringify(draft)) as CovSetupDraft
}

export function applyCovPreset(env: CovEnvironment, presetId: CovPresetId): CovSetupDraft {
  const base = cloneCovDraft(env.defaults)
  const shortTitle = env.title.split('(')[0].trim()
  if (presetId === 'baseline_multimodal') {
    return base
  }
  if (presetId === 'dust_storm') {
    return {
      ...base,
      title: `${shortTitle} — CAMS Dust Storm EO/Acoustic Stress`,
      weather: {
        visibilityKm: 1.5,
        useEra5Wind: false,
        surfaceWindMps: 14,
      },
      jammers: [],
    }
  }
  if (presetId === 'ew_jammed') {
    const [w, s, e, n] = env.boundsWgs84
    return {
      ...base,
      title: `${shortTitle} — Stand-Off EW Jammed Radar`,
      jammers: [
        {
          id: 'jammer-1',
          label: 'Stand-Off EW Jammer',
          lon: Number((w + (e - w) * 0.16).toFixed(6)),
          lat: Number((s + (n - s) * 0.84).toFixed(6)),
          aglM: 180,
          erpRatio: 16,
          beamWidthDeg: 36,
        },
      ],
    }
  }
  if (presetId === 'gap_filler_radar') {
    return {
      ...base,
      title: `${shortTitle} — Gap-Filler Radar Placement`,
      sensors: base.sensors.map((s) => ({ ...s, locked: true })),
      optimizer: {
        ...base.optimizer,
        enabled: true,
        extraSensors: 1,
        extraSensorType: 'radar',
        minSeparationM: 800,
      },
    }
  }
  // stealth_uav
  return {
    ...base,
    title: `${shortTitle} — Low-RCS Micro-UAV Coverage`,
    threat: { rcsM2: 0.02, label: 'Low-RCS Micro-UAV' },
    layers: [
      { aglM: 20, weight: 0.4 },
      { aglM: 50, weight: 0.3 },
      { aglM: 120, weight: 0.2 },
      { aglM: 250, weight: 0.1 },
    ],
    mapLayerAglM: 20,
  }
}

export function isPointInCovBounds(
  lng: number,
  lat: number,
  bounds: [number, number, number, number],
  padDeg = 0.001,
): boolean {
  const [w, s, e, n] = bounds
  return lng >= w - padDeg && lng <= e + padDeg && lat >= s - padDeg && lat <= n + padDeg
}

function sectorOrCircleRing(
  lon: number,
  lat: number,
  radiusM: number,
  centerDeg = 0,
  widthDeg = 360,
): [number, number][] {
  const radLat = (lat * Math.PI) / 180
  const mPerDegLat = 111132.92
  const mPerDegLon = Math.max(1000, 111319.49 * Math.cos(radLat))
  const full = widthDeg >= 359.5
  const steps = full ? 48 : Math.max(16, Math.ceil(widthDeg / 6))
  const startDeg = full ? 0 : centerDeg - widthDeg / 2
  const spanDeg = full ? 360 : widthDeg
  const ring: [number, number][] = []
  if (!full) ring.push([lon, lat])
  for (let i = 0; i <= steps; i++) {
    const az = ((startDeg + (i / steps) * spanDeg) * Math.PI) / 180
    ring.push([lon + (Math.sin(az) * radiusM) / mPerDegLon, lat + (Math.cos(az) * radiusM) / mPerDegLat])
  }
  if (!full) ring.push([lon, lat])
  return ring
}

export function buildCoverageOverlay(
  env: CovEnvironment | undefined,
  draft: CovSetupDraft | undefined,
  resultOverlay: FeatureCollection | null,
  overlayConfig: 'both' | 'baseline' | 'optimized',
  stealthRoute?: CovAdversarialRoute | null,
): FeatureCollection {
  const features: Feature[] = []
  if (!env || !draft) return { type: 'FeatureCollection', features }

  const [w, s, e, n] = env.boundsWgs84
  features.push({
    type: 'Feature',
    geometry: {
      type: 'Polygon',
      coordinates: [[[w, s], [e, s], [e, n], [w, n], [w, s]]],
    },
    properties: { simKind: 'env-bounds', label: env.title },
  })

  // Completed-run blind-spot polygons, optimized sensors, movement arcs, and first detections
  if (resultOverlay?.features?.length) {
    for (const f of resultOverlay.features) {
      const kind = String(f.properties?.simKind ?? '')
      if (overlayConfig === 'baseline' && (kind.endsWith('-optimized') || kind === 'cov-move')) continue
      if (overlayConfig === 'optimized' && kind.endsWith('-baseline')) continue
      features.push(f)
    }
  }

  // Defended asset priority rings
  for (const a of draft.assets) {
    features.push({
      type: 'Feature',
      geometry: {
        type: 'Polygon',
        coordinates: [sectorOrCircleRing(a.lon, a.lat, a.warningRadiusM)],
      },
      properties: { simKind: 'cov-asset-zone', label: a.label },
    })
  }

  // Sensor nominal range / sector rings
  for (const sensor of draft.sensors) {
    const cappedR = Math.min(sensor.maxRangeM, 12000)
    features.push({
      type: 'Feature',
      geometry: {
        type: 'Polygon',
        coordinates: [sectorOrCircleRing(sensor.lon, sensor.lat, cappedR, sensor.sectorCenterDeg, sensor.sectorWidthDeg)],
      },
      properties: { simKind: `cov-range-${sensor.type}`, label: sensor.label },
    })
  }

  // Ingress routes
  for (const r of draft.routes) {
    if (r.points.length >= 2) {
      features.push({
        type: 'Feature',
        geometry: {
          type: 'LineString',
          coordinates: r.points.map((p) => [p[0], p[1]]),
        },
        properties: { simKind: 'cov-route', label: r.label },
      })
    }
  }

  // Defended asset pins
  for (const a of draft.assets) {
    features.push({
      type: 'Feature',
      geometry: { type: 'Point', coordinates: [a.lon, a.lat] },
      properties: { simKind: 'cov-asset', label: a.label },
    })
  }

  // Draft sensor pins
  for (const sensor of draft.sensors) {
    features.push({
      type: 'Feature',
      geometry: { type: 'Point', coordinates: [sensor.lon, sensor.lat] },
      properties: { simKind: `cov-sensor-${sensor.type}`, label: sensor.label },
    })
  }

  // Stand-off EW jammers
  for (const jam of draft.jammers ?? []) {
    features.push({
      type: 'Feature',
      geometry: { type: 'Point', coordinates: [jam.lon, jam.lat] },
      properties: { simKind: 'cov-detect-baseline', label: `EW: ${jam.label}` },
    })
  }

  // Adversarial A* least-detection stealth route
  if (stealthRoute && stealthRoute.pointsLonLat.length >= 2) {
    features.push({
      type: 'Feature',
      geometry: {
        type: 'LineString',
        coordinates: stealthRoute.pointsLonLat.map((p) => [p[0], p[1]]),
      },
      properties: {
        simKind: 'cov-move',
        label: `Stealth (${Math.round(stealthRoute.stealthFraction * 100)}% unseen · ${stealthRoute.warningTimeS}s warn)`,
      },
    })
  }

  return { type: 'FeatureCollection', features }
}
