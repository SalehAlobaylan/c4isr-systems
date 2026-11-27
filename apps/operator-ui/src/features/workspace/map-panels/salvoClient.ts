import type { Feature, FeatureCollection } from 'geojson'
import {
  buildSimViewerHref,
  resolveSimViewerOrigin,
  simPlatformJson,
  startUnifiedLocalSimHub,
  subscribeSimPlatformRun,
} from './simPlatformGateway'

export type SalvoPlacementTool = null | 'add-sam' | 'add-shorad' | 'add-pd' | 'move-asset'
export type SalvoBatteryTier = 'tier1_sam' | 'tier2_shorad' | 'tier3_point_defense'
export type SalvoPresetId = 'mixed_swarm_decoys' | 'saturation_salvo' | 'civilian_airspace' | 'dust_storm_salvo'

export interface SalvoPresetMeta {
  id: SalvoPresetId
  label: string
  summary: string
}

export const SALVO_PRESETS: SalvoPresetMeta[] = [
  {
    id: 'mixed_swarm_decoys',
    label: 'Mixed Swarm + Decoys',
    summary: '3 RF decoys + 5 UAVs + 2 cruise + 1 ballistic across 2 waves',
  },
  {
    id: 'saturation_salvo',
    label: 'Ballistic Saturation',
    summary: 'Heavy 2-wave saturation raid: 3 ballistic + 4 cruise + 6 UAVs + 3 decoys',
  },
  {
    id: 'civilian_airspace',
    label: 'Civil Airspace Stress',
    summary: 'Mixed salvo under active civil airliners (strict 1.5 km HOLD_FIRE)',
  },
  {
    id: 'dust_storm_salvo',
    label: 'CAMS Dust (EO Blind)',
    summary: '1.5 km dust visibility degrading EO/IR decoy discrimination + Shamal wind',
  },
]

export interface SalvoAssetDraft {
  id: string
  label: string
  lon: number
  lat: number
  radiusM: number
  value: number
}

export interface SalvoBatteryDraft {
  id: string
  tier: SalvoBatteryTier
  label: string
  lon: number
  lat: number
  magazine: number
}

export interface SalvoSensorDraft {
  id: string
  type: 'radar' | 'eo_ir' | 'acoustic'
  label: string
  lon: number
  lat: number
  mastAglM: number
  maxRangeM: number
}

export interface SalvoSetupDraft {
  environmentId: string
  title: string
  presetId: SalvoPresetId
  seed: number
  durationS: number
  enableCivilianTraffic: boolean
  visibilityKm: number
  windScale: number
  windAddEnuMps: [number, number, number]
  raid: {
    decoys: number
    uavs: number
    cruiseMissiles: number
    ballisticMissiles: number
    waves: number
    ingressSectorDeg: number
    ingressSpreadDeg: number
  }
  assets: SalvoAssetDraft[]
  batteries: SalvoBatteryDraft[]
  sensors: SalvoSensorDraft[]
  sensorNetworkId: string | null
}

export interface SalvoEnvironment {
  id: string
  title: string
  biome: string
  description: string
  manifestSha256: string
  buildingCount: number
  roadCount: number
  hasGlb: boolean
  hasImagery: boolean
  boundsWgs84: [number, number, number, number]
  origin: [number, number, number]
  defaults: SalvoSetupDraft
}

export interface SalvoPolicyRow {
  compositeScore: number
  leakers: number
  lethalIntercepted: number
  lethalThreats: number
  defenseSuccessRate: number
  shotsFired: number
  totalCostKUsd: number
  decoyShotsWasted: number
  airspaceViolations: number
  debrisBuildingHits: number
  estimatedPopulationExposed: number
  emergencyDetourDelayS: number
}

export interface SalvoRunJob {
  jobId: string
  environmentId: string
  environmentTitle: string
  title: string
  status: 'queued' | 'running' | 'completed' | 'failed' | 'interrupted' | 'cancelled'
  stage: string
  createdAt: string
  completedAt: string | null
  viewerUrl: string
  threatCount: number
  batteryCount: number
  elapsedS?: number
  policies?: string[]
  error: string | null
  comparison: {
    policies: Record<string, SalvoPolicyRow>
    wtaVsGreedy?: {
      scoreDelta: number
      leakersDelta: number
      costSavedKUsd: number
      costSavedPct: number
      decoyWasteDelta: number
      airspaceViolationsDelta: number
    }
  } | null
  resultOverlay: FeatureCollection | null
}

export interface SalvoHealth {
  status: string
  service: string
  engineVersion: string
  aiPolicyAvailable: boolean
  viewerOrigin: string
  environmentCount: number
  runningJobs: number
  totalRuns: number
}

export interface SalvoNetworkItem {
  layoutId: string
  name: string
  environmentId: string
  sensorCount: number
  sensors: SalvoSensorDraft[]
}

const salvoJson = <T>(path: string, init?: RequestInit): Promise<T> =>
  simPlatformJson<T>('salvo', path, init, 'Salvo Defense service')

export const fetchSalvoHealth = async (): Promise<SalvoHealth> => {
  const h = await salvoJson<SalvoHealth>('/health')
  return { ...h, viewerOrigin: await resolveSimViewerOrigin('salvo', h.viewerOrigin) }
}
export const fetchSalvoEnvironments = async () =>
  (await salvoJson<{ environments: SalvoEnvironment[] }>('/environments')).environments
export const fetchSalvoRuns = async () => (await salvoJson<{ runs: SalvoRunJob[] }>('/runs')).runs
export const fetchSalvoNetworks = async () =>
  (await salvoJson<{ networks: SalvoNetworkItem[] }>('/networks')).networks
export const fetchSalvoRun = (jobId: string) => salvoJson<SalvoRunJob>(`/runs/${encodeURIComponent(jobId)}`)
export const startSalvoRun = (draft: SalvoSetupDraft, jobId?: string) =>
  salvoJson<SalvoRunJob>('/runs', {
    method: 'POST',
    headers: jobId ? { 'Idempotency-Key': jobId } : undefined,
    body: JSON.stringify(jobId ? { ...draft, jobId } : draft),
  })
export const cancelSalvoRun = (jobId: string) =>
  salvoJson<SalvoRunJob>(`/runs/${encodeURIComponent(jobId)}/cancel`, { method: 'POST' })
export const subscribeSalvoRun = (
  jobId: string,
  onUpdate: (job: SalvoRunJob) => void,
  onTerminal?: (job: SalvoRunJob) => void,
) => subscribeSimPlatformRun('salvo', jobId, fetchSalvoRun, onUpdate, onTerminal)
export { buildSimViewerHref }

export const startLocalSalvoService = () => startUnifiedLocalSimHub('/__local/salvo/start', 'task dev:sim-hub')

export function cloneSalvoDraft(d: SalvoSetupDraft): SalvoSetupDraft {
  return JSON.parse(JSON.stringify(d)) as SalvoSetupDraft
}

export function applySalvoPreset(env: SalvoEnvironment, presetId: SalvoPresetId): SalvoSetupDraft {
  const base = cloneSalvoDraft(env.defaults)
  const shortTitle = env.title.split('(')[0].trim()
  base.presetId = presetId
  if (presetId === 'mixed_swarm_decoys') return base
  if (presetId === 'saturation_salvo') {
    return {
      ...base,
      title: `${shortTitle} — Ballistic + Cruise Saturation`,
      raid: { ...base.raid, decoys: 3, uavs: 6, cruiseMissiles: 4, ballisticMissiles: 3, waves: 2, ingressSpreadDeg: 130 },
    }
  }
  if (presetId === 'civilian_airspace') {
    return {
      ...base,
      title: `${shortTitle} — Civil Airspace Deconfliction`,
      enableCivilianTraffic: true,
      raid: { ...base.raid, decoys: 2, uavs: 5, cruiseMissiles: 3, ballisticMissiles: 1, waves: 2 },
    }
  }
  return {
    ...base,
    title: `${shortTitle} — CAMS Dust Storm Salvo`,
    visibilityKm: 1.5,
    windAddEnuMps: [14, -5, 0],
    raid: { ...base.raid, decoys: 4, uavs: 5, cruiseMissiles: 2, ballisticMissiles: 1, waves: 2 },
  }
}

export function isPointInSalvoBounds(
  lng: number,
  lat: number,
  bounds: [number, number, number, number],
  padDeg = 0.002,
): boolean {
  const [w, s, e, n] = bounds
  return lng >= w - padDeg && lng <= e + padDeg && lat >= s - padDeg && lat <= n + padDeg
}

function circleRing(lon: number, lat: number, radiusM: number, steps = 42): [number, number][] {
  const mPerLat = 111132.92
  const mPerLon = Math.max(1000, 111319.49 * Math.cos((lat * Math.PI) / 180))
  const out: [number, number][] = []
  for (let i = 0; i <= steps; i++) {
    const a = (i / steps) * Math.PI * 2
    out.push([lon + (Math.cos(a) * radiusM) / mPerLon, lat + (Math.sin(a) * radiusM) / mPerLat])
  }
  return out
}

const TIER_RANGE_M: Record<SalvoBatteryTier, number> = {
  tier1_sam: 14000,
  tier2_shorad: 7200,
  tier3_point_defense: 2900,
}

export function buildSalvoOverlay(
  env: SalvoEnvironment | undefined,
  draft: SalvoSetupDraft | undefined,
  resultOverlay: FeatureCollection | null,
): FeatureCollection {
  const features: Feature[] = []
  if (!env || !draft) return { type: 'FeatureCollection', features }

  const [w, s, e, n] = env.boundsWgs84
  features.push({
    type: 'Feature',
    geometry: { type: 'Polygon', coordinates: [[[w, s], [e, s], [e, n], [w, n], [w, s]]] },
    properties: { simKind: 'env-bounds', label: env.title },
  })

  for (const a of draft.assets) {
    features.push({
      type: 'Feature',
      geometry: { type: 'Polygon', coordinates: [circleRing(a.lon, a.lat, a.radiusM)] },
      properties: { simKind: 'protected-zone', label: a.label },
    })
  }

  for (const b of draft.batteries) {
    const r = Math.min(TIER_RANGE_M[b.tier] ?? 6000, 12000)
    features.push({
      type: 'Feature',
      geometry: { type: 'Polygon', coordinates: [circleRing(b.lon, b.lat, r)] },
      properties: { simKind: `salvo-ring-${b.tier}`, label: b.label },
    })
  }

  if (resultOverlay?.features?.length) {
    features.push(...resultOverlay.features)
  }

  for (const a of draft.assets) {
    features.push({
      type: 'Feature',
      geometry: { type: 'Point', coordinates: [a.lon, a.lat] },
      properties: { simKind: 'protected-center', label: a.label },
    })
  }

  for (const b of draft.batteries) {
    features.push({
      type: 'Feature',
      geometry: { type: 'Point', coordinates: [b.lon, b.lat] },
      properties: { simKind: `salvo-batt-${b.tier}`, label: `${b.label} (${b.magazine})` },
    })
  }

  return { type: 'FeatureCollection', features }
}
