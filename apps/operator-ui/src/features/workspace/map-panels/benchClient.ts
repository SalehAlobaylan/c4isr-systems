import type { Feature, FeatureCollection } from 'geojson'
import { simPlatformJson, startUnifiedLocalSimHub } from './simPlatformGateway'

export interface BenchEnvironment {
  id: string
  title: string
  biome: string
  boundsWgs84: [number, number, number, number]
  centerLonLat: [number, number]
  buildingCount: number
}

export interface BenchHealth {
  status: string
  service: string
  engineVersion: string
  surrogateLoaded: boolean
  aiPolicyLoaded: boolean
  environmentCount: number
  benchmarkRunning: boolean
}

export interface BenchModelsStatus {
  debrisSurrogate: {
    available: boolean
    path: string | null
    metadata: {
      architecture?: string
      nSamples?: number
      valR2DriftE?: number
      valR2DriftN?: number
      valR2Urban?: number
      valDriftMaeM?: number
      valUrbanMae?: number
      inferenceMs?: number
    } | null
  }
  aiWtaPolicy: {
    available: boolean
    path: string | null
    metadata: {
      generations?: number
      population?: number
      trainElapsedS?: number
      validation?: Record<
        string,
        { meanScore: number; meanCostKUsd: number; meanLeakers: number; meanDecoyWaste: number; airspaceViolations: number }
      >
    } | null
  }
}

export interface MonteCarloResult {
  derivation: string
  trials: number
  fragmentsPerTrial: number
  totalTrajectories: number
  elapsedS: number
  centroidLonLat: [number, number]
  meanDriftM: number
  maxCellHitProbability: number
  riskArea5PctKm2: number
  riskArea20PctKm2: number
  buildingHits: { mean: number; p90: number; p95: number; max: number }
  populationExposed: { mean: number; p95: number }
  hazardousPerTrialMean: number
  surrogatePreview?: {
    derivation: string
    driftDistanceM: number
    urbanExposureFraction: number
    estimatedPopulationExposed: number
    riskScore: number
    inferenceMs: number
  }
  contoursGeoJson: FeatureCollection
}

export interface LeaderboardEntry {
  policy: string
  episodes: number
  meanCompositeScore: number
  meanDefenseSuccessPct: number
  meanLeakers: number
  meanCostKUsd: number
  meanDecoyWaste: number
  totalAirspaceViolations: number
  meanPopExposed: number
}

export interface BenchmarkReport {
  schema: string
  generatedAt: string
  elapsedS: number
  biomes: string[]
  seeds: number[]
  surrogateActive: boolean
  aiPolicyActive: boolean
  leaderboard: LeaderboardEntry[]
}

const benchJson = <T>(path: string, init?: RequestInit): Promise<T> =>
  simPlatformJson<T>('bench', path, init, 'Benchmark service')

export const fetchBenchHealth = () => benchJson<BenchHealth>('/health')
export const fetchBenchEnvironments = async () =>
  (await benchJson<{ environments: BenchEnvironment[] }>('/environments')).environments
export const fetchBenchModels = () => benchJson<BenchModelsStatus>('/models')
export const fetchBenchLeaderboard = async () =>
  (await benchJson<{ leaderboard: BenchmarkReport | null }>('/leaderboard')).leaderboard
export const runMonteCarloApi = (payload: {
  environmentId: string
  lon: number
  lat: number
  interceptAglM: number
  threatSpeedMps: number
  threatHeadingDeg: number
  trials: number
  windSigmaMps: number
  windAddEnuMps: [number, number, number]
}) =>
  benchJson<MonteCarloResult>('/monte-carlo', {
    method: 'POST',
    body: JSON.stringify(payload),
  })
export const runBenchmarkApi = (payload: { biomes?: string[]; seeds?: number[] }) =>
  benchJson<BenchmarkReport>('/benchmark', {
    method: 'POST',
    body: JSON.stringify(payload),
  })

export const startLocalBenchService = () => startUnifiedLocalSimHub('/__local/bench/start', 'task dev:sim-hub')

export function buildBenchOverlay(
  env: BenchEnvironment | undefined,
  point: [number, number] | null,
  mc: MonteCarloResult | null,
): FeatureCollection {
  const features: Feature[] = []
  if (!env) return { type: 'FeatureCollection', features }
  const [w, s, e, n] = env.boundsWgs84
  features.push({
    type: 'Feature',
    geometry: { type: 'Polygon', coordinates: [[[w, s], [e, s], [e, n], [w, n], [w, s]]] },
    properties: { simKind: 'env-bounds', label: env.title },
  })
  if (mc?.contoursGeoJson?.features?.length) {
    features.push(...mc.contoursGeoJson.features)
  }
  if (point) {
    features.push({
      type: 'Feature',
      geometry: { type: 'Point', coordinates: point },
      properties: { simKind: 'result-intercept-safe_window', label: 'MC Intercept Point' },
    })
  }
  return { type: 'FeatureCollection', features }
}
