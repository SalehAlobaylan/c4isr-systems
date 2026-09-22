/**
 * Typed REST client for the C4ISR backend.
 *
 * Field names mirror the Go transport responses exactly (camelCase JSON),
 * derived from internal/*\/http.go. Timestamps are RFC3339 strings.
 */

import createClient from 'openapi-fetch'

import type { components, operations, paths } from '@/lib/openapi.generated'

// The REST client functions below are the ergonomic application layer over
// the generated OpenAPI contract. Keeping the generated file separate means
// contract regeneration never overwrites UI-specific query helpers.
export type Point = components['schemas']['Point']

type ErrorEnvelope = components['schemas']['Error']

export class ApiError extends Error {
  readonly status: number
  readonly code: string

  constructor(status: number, code: string, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
  }
}

/* ------------------------------------------------------------------ */
/* Contract-backed domain aliases                                      */
/* ------------------------------------------------------------------ */

export type Source = components['schemas']['Source']
export type Track = components['schemas']['Track']
export type TrackHistoryPoint = components['schemas']['TrackHistoryPoint']
export type Classification = components['schemas']['Classification']
export type Observation = components['schemas']['Observation']
export type Asset = components['schemas']['Asset']
export type TelemetrySample = components['schemas']['Telemetry']
export type Geofence = components['schemas']['Geofence']
export type Alert = components['schemas']['Alert']
export type Incident = components['schemas']['Incident']
export type IncidentDetail = components['schemas']['IncidentDetail']
export type RelatedAlert = components['schemas']['RelatedAlert']
export type RelatedTrack = components['schemas']['RelatedTrack']
export type RelatedAsset = components['schemas']['RelatedAsset']
export type RelatedObservation = components['schemas']['RelatedObservation']
export type RelatedAssessment = components['schemas']['RelatedAssessment']
export type MissionTask = components['schemas']['MissionTask']
export type Mission = components['schemas']['Mission']
export type Command = components['schemas']['Command']
export type AssessmentEvidence = components['schemas']['AssessmentEvidence']
export type Assessment = components['schemas']['Assessment']
export type AuditEntry = components['schemas']['AuditEntry']
export type Operator = components['schemas']['Operator']
export type CurrentOperator = components['schemas']['CurrentOperator']
export type Scenario = components['schemas']['Scenario']
export type ScenarioSummary = components['schemas']['ScenarioSummary']
export type ScenarioRun = components['schemas']['ScenarioRun']
export type ScenarioEvent = components['schemas']['ScenarioEvent']
export type Health = components['schemas']['Health']

export type SourceList = components['schemas']['SourceList']
export type ObservationList = components['schemas']['ObservationList']
export type AssetList = components['schemas']['AssetList']
export type TelemetryList = components['schemas']['TelemetryList']
export type TrackList = components['schemas']['TrackList']
export type TrackHistoryList = components['schemas']['TrackHistoryList']
export type ClassificationList = components['schemas']['ClassificationList']
export type GeofenceList = components['schemas']['GeofenceList']
export type AlertList = components['schemas']['AlertList']
export type IncidentList = components['schemas']['IncidentList']
export type MissionList = components['schemas']['MissionList']
export type CommandList = components['schemas']['CommandList']
export type AssessmentList = components['schemas']['AssessmentList']
export type AuditList = components['schemas']['AuditList']
export type OperatorList = components['schemas']['OperatorList']
export type ScenarioSummaryList = components['schemas']['ScenarioSummaryList']
export type ScenarioRunList = components['schemas']['ScenarioRunList']
export type ScenarioEventList = components['schemas']['ScenarioEventList']

export type CreateIncidentInput = components['requestBodies']['Incident']['content']['application/json']
export type CreateMissionInput = components['requestBodies']['Mission']['content']['application/json']
export type IssueCommandInput = components['requestBodies']['Command']['content']['application/json']
export type CreateClassificationInput = NonNullable<
  operations['createClassification']['requestBody']
>['content']['application/json']
export type CreateObservationInput = components['requestBodies']['ObservationInput']['content']['application/json']
export type UpdateAssetStatusInput = operations['updateAssetStatus']['requestBody']['content']['application/json']
export type StartScenarioInput = NonNullable<
  operations['startScenario']['requestBody']
>['content']['application/json']
export type CurrentOperatorResponse = operations['getCurrentOperator']['responses'][200]['content']['application/json']

export interface RealtimeEnvelope {
  id: string
  type: string
  version: number
  occurredAt: string
  data: Record<string, unknown>
}

type QueryOf<Operation extends keyof operations> = NonNullable<operations[Operation]['parameters']['query']>
export type SourceFilters = QueryOf<'listSources'>
export type ObservationFilters = QueryOf<'listObservations'>
export type TrackFilters = QueryOf<'listTracks'>
export type AssetFilters = QueryOf<'listAssets'>
export type AlertFilters = QueryOf<'listAlerts'>
export type IncidentFilters = QueryOf<'listIncidents'>
export type MissionFilters = QueryOf<'listMissions'>
export type CommandFilters = QueryOf<'listCommands'>
export type AssessmentFilters = QueryOf<'listAssessments'>
export type AuditFilters = QueryOf<'listAudit'>

/* ------------------------------------------------------------------ */
/* Transport                                                           */
/* ------------------------------------------------------------------ */

const runtimeConfig = typeof window === 'undefined' ? {} : (window.__C4ISR_CONFIG__ ?? {})

// Runtime configuration keeps staging/production tokens out of the immutable
// UI image. Vite values remain available for local development and tests.
export const API_TOKEN = (runtimeConfig.apiToken ?? import.meta.env.VITE_API_TOKEN ?? '').trim()

const configuredApiBase = runtimeConfig.apiBaseUrl ?? import.meta.env.VITE_API_BASE_URL ?? '/api/v1'
const API_BASE = configuredApiBase.trim().replace(/\/+$/, '') || '/api/v1'
export const MAP_STYLE_URL = (runtimeConfig.mapStyleUrl ?? import.meta.env.VITE_MAP_STYLE_URL ?? '').trim()

function apiClientBaseUrl(baseUrl: string): string {
  const origin = typeof window === 'undefined' ? 'http://localhost' : window.location.origin
  const resolved = new URL(baseUrl, origin)
  const pathname = resolved.pathname.replace(/\/+$/, '')
  const apiPath = '/api/v1'
  const prefix = pathname.endsWith(apiPath)
    ? pathname.slice(0, -apiPath.length)
    : pathname
  return `${resolved.origin}${prefix}`
}

export const apiClient = createClient<paths>({
  // Generated OpenAPI paths already include /api/v1. Keep any configured
  // gateway prefix in the base URL, while avoiding a duplicated /api/v1.
  baseUrl: apiClientBaseUrl(API_BASE),
  headers: {
    Accept: 'application/json',
    ...(API_TOKEN ? { Authorization: `Bearer ${API_TOKEN}` } : {}),
  },
})

function errorEnvelope(value: unknown): ErrorEnvelope | undefined {
  if (!value || typeof value !== 'object') return undefined
  const error = (value as { error?: unknown }).error
  if (!error || typeof error !== 'object') return undefined
  const code = (error as { code?: unknown }).code
  const message = (error as { message?: unknown }).message
  if (typeof code !== 'string' || typeof message !== 'string') return undefined
  return { error: { code, message } }
}

async function parseError(response: Response, payload?: unknown): Promise<ApiError> {
  let code = 'http_error'
  let message = `Request failed with status ${response.status}`
  const body = errorEnvelope(payload) ?? (await response.clone().json().catch(() => undefined))
  const parsed = errorEnvelope(body)
  if (parsed) {
    code = parsed.error.code
    message = parsed.error.message
  }
  if (response.status === 401) {
    message = 'Authentication failed. Configure the operator UI API token.'
  } else if (response.status === 403) {
    message = 'You are authenticated but not authorized for this operation.'
  }
  return new ApiError(response.status, code, message)
}

async function unwrap<T>(result: Promise<{
  data?: T
  error?: unknown
  response: Response
}>): Promise<T> {
  const resolved = await result
  if (!resolved.response.ok) throw await parseError(resolved.response, resolved.error)
  return resolved.data as T
}

/* ------------------------------------------------------------------ */
/* Endpoints                                                           */
/* ------------------------------------------------------------------ */

export async function fetchHealth(signal?: AbortSignal): Promise<Health> {
  return unwrap(apiClient.GET('/health', signal ? { signal } : undefined))
}

export const api = {
  /* identity */
  getCurrentOperator: () => unwrap(apiClient.GET('/api/v1/auth/me')),

  /* sources */
  listSources: (params?: { limit?: number; offset?: number }) =>
    unwrap(apiClient.GET('/api/v1/sources', { params: { query: params } })),

  /* assets */
  listAssets: (params?: AssetFilters) =>
    unwrap(apiClient.GET('/api/v1/assets', { params: { query: params } })),
  getAsset: (id: string) =>
    unwrap(apiClient.GET('/api/v1/assets/{id}', { params: { path: { id } } })),
  updateAssetStatus: (id: string, status: UpdateAssetStatusInput['status']) =>
    unwrap(apiClient.POST('/api/v1/assets/{id}/status', { params: { path: { id } }, body: { status } })),
  listAssetTelemetry: (id: string, params?: { limit?: number; offset?: number }) =>
    unwrap(
      apiClient.GET('/api/v1/assets/{id}/telemetry', {
        params: { path: { id }, query: params },
      }),
    ),

  /* tracks */
  listTracks: (params?: TrackFilters) =>
    unwrap(apiClient.GET('/api/v1/tracks', { params: { query: params } })),
  getTrack: (id: string) =>
    unwrap(apiClient.GET('/api/v1/tracks/{id}', { params: { path: { id } } })),
  getTrackHistory: (id: string, params?: { limit?: number }) =>
    unwrap(
      apiClient.GET('/api/v1/tracks/{id}/history', {
        params: { path: { id }, query: params },
      }),
    ),
  listTrackClassifications: (id: string, params?: { limit?: number; offset?: number }) =>
    unwrap(
      apiClient.GET('/api/v1/tracks/{id}/classifications', {
        params: { path: { id }, query: params },
      }),
    ),

  /* classifications */
  createClassification: (input: CreateClassificationInput) =>
    unwrap(apiClient.POST('/api/v1/classifications', { body: input })),

  /* observations */
  listObservations: (params?: ObservationFilters) =>
    unwrap(apiClient.GET('/api/v1/observations', { params: { query: params } })),
  getObservation: (id: string) =>
    unwrap(apiClient.GET('/api/v1/observations/{id}', { params: { path: { id } } })),
  createObservation: (input: CreateObservationInput) =>
    unwrap(apiClient.POST('/api/v1/observations', { body: input })),

  /* geofences */
  listGeofences: (params?: { limit?: number; offset?: number }) =>
    unwrap(apiClient.GET('/api/v1/geofences', { params: { query: params } })),

  /* alerts */
  listAlerts: (params?: AlertFilters) =>
    unwrap(apiClient.GET('/api/v1/alerts', { params: { query: params } })),
  getAlert: (id: string) =>
    unwrap(apiClient.GET('/api/v1/alerts/{id}', { params: { path: { id } } })),
  acknowledgeAlert: (id: string) =>
    unwrap(apiClient.POST('/api/v1/alerts/{id}/acknowledge', { params: { path: { id } } })),
  resolveAlert: (id: string) =>
    unwrap(apiClient.POST('/api/v1/alerts/{id}/resolve', { params: { path: { id } } })),

  /* incidents */
  listIncidents: (params?: IncidentFilters) =>
    unwrap(apiClient.GET('/api/v1/incidents', { params: { query: params } })),
  getIncident: (id: string) =>
    unwrap(apiClient.GET('/api/v1/incidents/{id}', { params: { path: { id } } })),
  createIncident: (input: CreateIncidentInput) =>
    unwrap(apiClient.POST('/api/v1/incidents', { body: input })),
  updateIncidentStatus: (id: string, status: string) =>
    unwrap(apiClient.POST('/api/v1/incidents/{id}/status', { params: { path: { id } }, body: { status } })),
  attachIncidentRelation: (id: string, kind: string, relationId: string) =>
    unwrap(
      apiClient.POST('/api/v1/incidents/{id}/relations', {
        params: { path: { id } },
        body: { kind, id: relationId },
      }),
    ),

  /* missions */
  listMissions: (params?: MissionFilters) =>
    unwrap(apiClient.GET('/api/v1/missions', { params: { query: params } })),
  getMission: (id: string) =>
    unwrap(apiClient.GET('/api/v1/missions/{id}', { params: { path: { id } } })),
  createMission: (input: CreateMissionInput) =>
    unwrap(apiClient.POST('/api/v1/missions', { body: input })),
  updateMissionStatus: (id: string, status: string) =>
    unwrap(apiClient.POST('/api/v1/missions/{id}/status', { params: { path: { id } }, body: { status } })),
  assignMissionAsset: (id: string, assetId: string) =>
    unwrap(apiClient.POST('/api/v1/missions/{id}/assets', { params: { path: { id } }, body: { assetId } })),

  /* commands */
  listCommands: (params?: CommandFilters) =>
    unwrap(apiClient.GET('/api/v1/commands', { params: { query: params } })),
  getCommand: (id: string) =>
    unwrap(apiClient.GET('/api/v1/commands/{id}', { params: { path: { id } } })),
  issueCommand: (input: IssueCommandInput) =>
    unwrap(apiClient.POST('/api/v1/commands', { body: input })),
  transitionCommand: (id: string, state: string, reason?: string) =>
    unwrap(
      apiClient.POST('/api/v1/commands/{id}/transition', {
        params: { path: { id } },
        body: { state, reason: reason ?? '' },
      }),
    ),

  /* assessments */
  listAssessments: (params?: AssessmentFilters) =>
    unwrap(apiClient.GET('/api/v1/assessments', { params: { query: params } })),
  createAssessment: (input: components['requestBodies']['Assessment']['content']['application/json']) =>
    unwrap(apiClient.POST('/api/v1/assessments', { body: input })),

  /* audit */
  listAudit: (params?: AuditFilters) =>
    unwrap(apiClient.GET('/api/v1/audit', { params: { query: params } })),

  /* operators */
  listOperators: (params?: { limit?: number; offset?: number }) =>
    unwrap(apiClient.GET('/api/v1/operators', { params: { query: params } })),

  /* scenarios */
  listScenarios: () => unwrap(apiClient.GET('/api/v1/scenarios')),
  getScenario: (name: string) =>
    unwrap(apiClient.GET('/api/v1/scenarios/{name}', { params: { path: { name } } })),
  startScenario: (name: string, input: StartScenarioInput) =>
    unwrap(apiClient.POST('/api/v1/scenarios/definitions/{name}/start', { params: { path: { name } }, body: input })),
  listScenarioRuns: (params?: { limit?: number; offset?: number }) =>
    unwrap(apiClient.GET('/api/v1/scenarios/runs', { params: { query: params } })),
  getScenarioRun: (id: string) =>
    unwrap(apiClient.GET('/api/v1/scenarios/runs/{id}', { params: { path: { id } } })),
  listScenarioRunEvents: (id: string) =>
    unwrap(apiClient.GET('/api/v1/scenarios/runs/{id}/events', { params: { path: { id } } })),
  pauseScenarioRun: (id: string) =>
    unwrap(apiClient.POST('/api/v1/scenarios/runs/{id}/pause', { params: { path: { id } } })),
  resumeScenarioRun: (id: string) =>
    unwrap(apiClient.POST('/api/v1/scenarios/runs/{id}/resume', { params: { path: { id } } })),
  stopScenarioRun: (id: string) =>
    unwrap(apiClient.POST('/api/v1/scenarios/runs/{id}/stop', { params: { path: { id } } })),
  restartScenarioRun: (id: string) =>
    unwrap(apiClient.POST('/api/v1/scenarios/runs/{id}/restart', { params: { path: { id } } })),
  setScenarioRunSpeed: (id: string, speed: number) =>
    unwrap(apiClient.POST('/api/v1/scenarios/runs/{id}/speed', { params: { path: { id } }, body: { speed } })),
}

export function realtimeUrl(): string {
  // Reuse the configured API path so a gateway prefix such as
  // /proxy/api/v1 also routes the WebSocket upgrade to the backend.
  const url = new URL(API_BASE, window.location.origin)
  url.pathname = `${url.pathname.replace(/\/+$/, '')}/realtime`
  if (/^https?:\/\//.test(API_BASE)) {
    url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:'
  } else {
    url.protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  }
  if (API_TOKEN) url.searchParams.set('access_token', API_TOKEN)
  return url.toString()
}
