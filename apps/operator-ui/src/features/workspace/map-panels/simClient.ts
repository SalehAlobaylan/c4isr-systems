import type { Feature, FeatureCollection } from 'geojson'

export type SimPlacementTool = null | 'radar' | 'launcher' | 'protected' | 'threat'
export type SimRunMode = 'compare' | 'earliest' | 'safe_window'
export type SimThreatKind = 'one_way_uav' | 'cruise_missile' | 'ballistic_missile'
export type SimTacticalPresetId = 'default_uav' | 'cruise_skim' | 'ballistic_arc' | 'shamal_stress'

export interface SimTacticalPresetMeta {
  id: SimTacticalPresetId
  label: string
  summary: string
}

export const SIM_TACTICAL_PRESETS: SimTacticalPresetMeta[] = [
  {
    id: 'default_uav',
    label: 'SW UAV Ingress',
    summary: 'Low-altitude one-way UAV with exact ERA5 winds',
  },
  {
    id: 'cruise_skim',
    label: 'Cruise Skim',
    summary: 'Fast terrain-hugging cruise missile (230 m/s, 55 m AGL)',
  },
  {
    id: 'ballistic_arc',
    label: 'Ballistic Arc',
    summary: 'High-arc ballistic trajectory (1.5 km apogee) + Shamal wind',
  },
  {
    id: 'shamal_stress',
    label: 'Shamal Crosswind',
    summary: 'UAV under strong +14E / -5N m/s crosswind drift',
  },
]

export interface SimWindLevel {
  altitudeAglM: number
  upM: number
  windEnuMps: [number, number, number]
  speedMps: number
}

export interface SimSetupDraft {
  environmentId: string
  title: string
  mode: SimRunMode
  seed: number
  radar: {
    lon: number
    lat: number
    heightAglM: number
    maxRangeM: number
    includeBuildingsInLos: boolean
  }
  launcher: {
    lon: number
    lat: number
    heightAglM: number
    speedMps: number
    maxRangeM: number
  }
  protectedZone: {
    lon: number
    lat: number
    radiusM: number
  }
  threat: {
    kind: string
    speedMps: number
    rcsM2: number
    ballisticApogeeM?: number
    waypoints: [number, number, number][] // [lon, lat, altAglM]
  }
  shrapnel: {
    fragmentCount: number
  }
  atmosphere: {
    windScale: number
    windAddEnuMps: [number, number, number]
  }
}

export interface SimEnvironment {
  id: string
  title: string
  description: string
  manifestSha256: string
  profileId: string
  buildingCount: number
  roadCount: number
  hasGlb: boolean
  hasImagery: boolean
  boundsWgs84: [number, number, number, number]
  origin: [number, number, number]
  weather: {
    edition?: string
    validTime?: string
    profileAtOrigin: SimWindLevel[]
  }
  defaults: Omit<SimSetupDraft, 'environmentId'>
}

export interface SimPolicyRow {
  intercepted: boolean
  launchTimeS?: number
  interceptTimeS?: number
  interceptAglM?: number
  protectedMarginM?: number
  buildingHits?: number
  roadHits?: number
  protectedZoneHits?: number
  openHits?: number
  urbanExposureFraction?: number
  hazardousHits?: number
  hazardousBuildingHits?: number
  maxImpactEnergyJ?: number
  driftDistanceM?: number
}

export interface SimRunJob {
  jobId: string
  environmentId: string
  environmentTitle: string
  title: string
  mode: SimRunMode
  seed: number
  status: 'running' | 'completed' | 'failed' | 'interrupted'
  stage: string
  createdAt: string
  completedAt: string | null
  viewerUrl: string
  hasGlb: boolean
  hasImagery: boolean
  error: string | null
  comparison: {
    policies: Record<string, SimPolicyRow>
    safeWindowVsEarliest?: {
      buildingHitsDelta: number
      roadHitsDelta: number
      urbanExposureDelta: number
      urbanExposureReductionPct: number
    }
  } | null
  summary: {
    policy: string
    intercepted: boolean
    interceptTimeS?: number
    exposure?: {
      total: number
      buildingHits: number
      roadHits: number
      protectedZoneHits: number
      openHits: number
      urbanExposureFraction: number
      hazardousHits?: number
      hazardousBuildingHits?: number
      hazardousRoadHits?: number
      hazardousProtectedHits?: number
      threatDebrisCount?: number
      interceptorDebrisCount?: number
      maxImpactEnergyJ?: number
      meanImpactEnergyJ?: number
    }
    footprint?: {
      driftDistanceM: number
    }
  } | null
  resultOverlay: FeatureCollection | null
}

export interface SimHealth {
  status: string
  engineVersion: string
  fidelityProfile: string
  viewerOrigin: string
  environmentCount: number
  runningJobs: number
  totalRuns: number
}

const SIM_API_BASE = '/sim-api'

async function simJson<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${SIM_API_BASE}${path}`, {
    ...init,
    headers: { 'Content-Type': 'application/json', ...(init?.headers ?? {}) },
  })
  const text = await res.text()
  let parsed: unknown = null
  try {
    parsed = text ? JSON.parse(text) : null
  } catch {
    // Non-JSON proxy error (e.g., ECONNREFUSED when service is down)
  }
  if (!res.ok) {
    const errField =
      parsed !== null && typeof parsed === 'object' && 'error' in parsed
        ? String((parsed as { error: unknown }).error)
        : ''
    throw new Error(errField || `Simulation service returned HTTP ${res.status}`)
  }
  return parsed as T
}

export const fetchSimHealth = () => simJson<SimHealth>('/api/health')
export const fetchSimEnvironments = async () => (await simJson<{ environments: SimEnvironment[] }>('/api/environments')).environments
export const fetchSimRuns = async () => (await simJson<{ runs: SimRunJob[] }>('/api/runs')).runs
export const fetchSimRun = (jobId: string) => simJson<SimRunJob>(`/api/runs/${encodeURIComponent(jobId)}`)
export const startSimRun = (draft: SimSetupDraft, jobId?: string) =>
  simJson<SimRunJob>('/api/runs', {
    method: 'POST',
    body: JSON.stringify(jobId ? { ...draft, jobId } : draft),
  })

export async function startLocalSimService(): Promise<{ ok: boolean; origin?: string; error?: string }> {
  const res = await fetch('/__local/sim/start', { method: 'POST' })
  const text = await res.text()
  try {
    const parsed = JSON.parse(text) as { ok?: boolean; origin?: string; error?: string }
    return { ok: Boolean(res.ok && parsed.ok), origin: parsed.origin, error: parsed.error }
  } catch {
    return {
      ok: false,
      error: `Auto-start endpoint unavailable (HTTP ${res.status}). Start manually with "task dev:sim".`,
    }
  }
}

export async function restartLocalSimService(): Promise<{ ok: boolean; origin?: string; error?: string }> {
  const res = await fetch('/__local/sim/restart', { method: 'POST' })
  const text = await res.text()
  try {
    const parsed = JSON.parse(text) as { ok?: boolean; origin?: string; error?: string }
    return { ok: Boolean(res.ok && parsed.ok), origin: parsed.origin, error: parsed.error }
  } catch {
    return { ok: false, error: `Restart endpoint unavailable (HTTP ${res.status}).` }
  }
}

export function draftFromEnvironment(env: SimEnvironment): SimSetupDraft {
  return {
    environmentId: env.id,
    title: env.defaults.title,
    mode: env.defaults.mode,
    seed: env.defaults.seed,
    radar: { ...env.defaults.radar },
    launcher: { ...env.defaults.launcher },
    protectedZone: { ...env.defaults.protectedZone },
    threat: {
      ...env.defaults.threat,
      ballisticApogeeM: env.defaults.threat.ballisticApogeeM ?? 1400,
      waypoints: env.defaults.threat.waypoints.map((w) => [w[0], w[1], w[2]]),
    },
    shrapnel: { ...env.defaults.shrapnel },
    atmosphere: {
      windScale: env.defaults.atmosphere.windScale,
      windAddEnuMps: [
        env.defaults.atmosphere.windAddEnuMps[0],
        env.defaults.atmosphere.windAddEnuMps[1],
        env.defaults.atmosphere.windAddEnuMps[2],
      ],
    },
  }
}

export function applyTacticalPreset(env: SimEnvironment, presetId: SimTacticalPresetId): SimSetupDraft {
  const base = draftFromEnvironment(env)
  if (presetId === 'default_uav') {
    return base
  }
  if (presetId === 'cruise_skim') {
    return {
      ...base,
      title: `${env.title.split('(')[0].trim()} — Cruise Missile Skim`,
      threat: {
        ...base.threat,
        kind: 'cruise_missile',
        speedMps: 230,
        rcsM2: 0.15,
        waypoints: base.threat.waypoints.map((w, idx) => [w[0], w[1], idx === 0 ? 75 : 55]),
      },
      launcher: {
        ...base.launcher,
        speedMps: 380,
      },
      shrapnel: { fragmentCount: 240 },
    }
  }
  if (presetId === 'ballistic_arc') {
    return {
      ...base,
      title: `${env.title.split('(')[0].trim()} — High-Arc Ballistic Rain`,
      threat: {
        ...base.threat,
        kind: 'ballistic_missile',
        speedMps: 340,
        rcsM2: 0.45,
        ballisticApogeeM: 1500,
      },
      launcher: {
        ...base.launcher,
        speedMps: 500,
      },
      shrapnel: { fragmentCount: 260 },
      atmosphere: {
        windScale: 1.0,
        windAddEnuMps: [14, -5, 0],
      },
    }
  }
  // shamal_stress
  return {
    ...base,
    title: `${env.title.split('(')[0].trim()} — Shamal Crosswind Stress`,
    atmosphere: {
      windScale: 1.0,
      windAddEnuMps: [14, -5, 0],
    },
    shrapnel: { fragmentCount: 240 },
  }
}

export function isPointInEnvBounds(
  lng: number,
  lat: number,
  bounds: [number, number, number, number],
  padDeg = 0.0025,
): boolean {
  const [w, s, e, n] = bounds
  return lng >= w - padDeg && lng <= e + padDeg && lat >= s - padDeg && lat <= n + padDeg
}

function circleRing(lon: number, lat: number, radiusM: number, steps = 48): [number, number][] {
  const radLat = (lat * Math.PI) / 180
  const mPerDegLat = 111132.92
  const mPerDegLon = Math.max(1000, 111319.49 * Math.cos(radLat))
  const ring: [number, number][] = []
  for (let i = 0; i <= steps; i++) {
    const a = (i / steps) * Math.PI * 2
    ring.push([lon + (Math.cos(a) * radiusM) / mPerDegLon, lat + (Math.sin(a) * radiusM) / mPerDegLat])
  }
  return ring
}

export function buildSimulationOverlay(
  env: SimEnvironment | undefined,
  draft: SimSetupDraft | undefined,
  resultOverlay?: FeatureCollection | null,
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

  // Radar range ring (capped visually to avoid obscuring world scale)
  features.push({
    type: 'Feature',
    geometry: {
      type: 'Polygon',
      coordinates: [circleRing(draft.radar.lon, draft.radar.lat, draft.radar.maxRangeM)],
    },
    properties: { simKind: 'radar-range' },
  })

  // Protected zone circle
  features.push({
    type: 'Feature',
    geometry: {
      type: 'Polygon',
      coordinates: [circleRing(draft.protectedZone.lon, draft.protectedZone.lat, draft.protectedZone.radiusM)],
    },
    properties: { simKind: 'protected-zone' },
  })

  // Optional completed-run 90% shrapnel dispersion ellipses & intercept markers
  if (resultOverlay?.features?.length) {
    features.push(...resultOverlay.features)
  }

  // Threat route polyline & numbered waypoints
  if (draft.threat.waypoints.length >= 2) {
    features.push({
      type: 'Feature',
      geometry: {
        type: 'LineString',
        coordinates: draft.threat.waypoints.map((wp) => [wp[0], wp[1]]),
      },
      properties: { simKind: 'threat-path' },
    })
  }
  draft.threat.waypoints.forEach((wp, idx) => {
    features.push({
      type: 'Feature',
      geometry: { type: 'Point', coordinates: [wp[0], wp[1]] },
      properties: { simKind: 'threat-wp', label: `T${idx + 1} (${Math.round(wp[2])}m)` },
    })
  })

  // Radar, Launcher, Protected Center pins
  features.push({
    type: 'Feature',
    geometry: { type: 'Point', coordinates: [draft.radar.lon, draft.radar.lat] },
    properties: { simKind: 'radar', label: 'Radar' },
  })
  features.push({
    type: 'Feature',
    geometry: { type: 'Point', coordinates: [draft.launcher.lon, draft.launcher.lat] },
    properties: { simKind: 'launcher', label: 'Launcher' },
  })
  features.push({
    type: 'Feature',
    geometry: { type: 'Point', coordinates: [draft.protectedZone.lon, draft.protectedZone.lat] },
    properties: { simKind: 'protected-center', label: 'Protected' },
  })

  return { type: 'FeatureCollection', features }
}
