/**
 * Typed REST client for the C4ISR backend.
 *
 * Field names mirror the Go transport responses exactly (camelCase JSON),
 * derived from internal/*\/http.go. Timestamps are RFC3339 strings.
 */

import type { components, operations } from '@/lib/openapi.generated'

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

export const API_TOKEN = (import.meta.env.VITE_API_TOKEN ?? '').trim()

const API_BASE = (import.meta.env.VITE_API_BASE_URL ?? '/api/v1').replace(/\/+$/, '')

type QueryValue = string | number | boolean | null | undefined

type QueryParams = Record<string, QueryValue> | object

function buildUrl(path: string, params?: QueryParams): string {
  const url = new URL(`${API_BASE}${path}`, window.location.origin)
  if (params) {
    for (const [key, value] of Object.entries(params as Record<string, unknown>)) {
      if (value === undefined || value === null || value === '') continue
      url.searchParams.set(key, String(value))
    }
  }
  return url.toString()
}

async function parseError(response: Response): Promise<ApiError> {
  let code = 'http_error'
  let message = `Request failed with status ${response.status}`
  try {
    const body = (await response.json()) as Partial<ErrorEnvelope>
    if (body.error) {
      code = body.error.code ?? code
      message = body.error.message ?? message
    }
  } catch {
    // Non-JSON error body; keep the generic message.
  }
  if (response.status === 401) {
    message = 'Authentication failed. Configure VITE_API_TOKEN for the operator UI.'
  } else if (response.status === 403) {
    message = 'You are authenticated but not authorized for this operation.'
  }
  return new ApiError(response.status, code, message)
}

interface RequestOptions {
  params?: QueryParams
  body?: unknown
  signal?: AbortSignal
}

async function request<T>(method: string, path: string, options: RequestOptions = {}): Promise<T> {
  const response = await fetch(buildUrl(path, options.params), {
    method,
    headers: {
      Accept: 'application/json',
      ...(API_TOKEN ? { Authorization: `Bearer ${API_TOKEN}` } : {}),
      ...(options.body !== undefined ? { 'Content-Type': 'application/json' } : {}),
    },
    body: options.body !== undefined ? JSON.stringify(options.body) : undefined,
    signal: options.signal,
  })

  if (!response.ok) throw await parseError(response)
  if (response.status === 204) return undefined as T
  return (await response.json()) as T
}

const get = <T>(path: string, params?: QueryParams, signal?: AbortSignal) =>
  request<T>('GET', path, { params, signal })

const post = <T>(path: string, body?: unknown, params?: QueryParams) =>
  request<T>('POST', path, { body, params })

/* ------------------------------------------------------------------ */
/* Endpoints                                                           */
/* ------------------------------------------------------------------ */

export async function fetchHealth(signal?: AbortSignal): Promise<Health> {
  const response = await fetch('/health', {
    headers: { Accept: 'application/json' },
    signal,
  })
  if (!response.ok) throw await parseError(response)
  return (await response.json()) as Health
}

const segment = (value: string) => encodeURIComponent(value)

export const api = {
  /* identity */
  getCurrentOperator: () => get<CurrentOperatorResponse>('/auth/me'),

  /* sources */
  listSources: (params?: { limit?: number; offset?: number }) =>
    get<SourceList>('/sources', params),

  /* assets */
  listAssets: (params?: AssetFilters) => get<AssetList>('/assets', params),
  getAsset: (id: string) => get<Asset>(`/assets/${segment(id)}`),
  updateAssetStatus: (id: string, status: string) =>
    post<Asset>(`/assets/${segment(id)}/status`, { status }),
  listAssetTelemetry: (id: string, params?: { limit?: number; offset?: number }) =>
    get<TelemetryList>(`/assets/${segment(id)}/telemetry`, params),

  /* tracks */
  listTracks: (params?: TrackFilters) => get<TrackList>('/tracks', params),
  getTrack: (id: string) => get<Track>(`/tracks/${segment(id)}`),
  getTrackHistory: (id: string, params?: { limit?: number }) =>
    get<TrackHistoryList>(`/tracks/${segment(id)}/history`, params),
  listTrackClassifications: (id: string, params?: { limit?: number; offset?: number }) =>
    get<ClassificationList>(`/tracks/${segment(id)}/classifications`, params),

  /* classifications */
  createClassification: (input: CreateClassificationInput) =>
    post<Classification>('/classifications', input),

  /* observations */
  listObservations: (params?: ObservationFilters) => get<ObservationList>('/observations', params),
  getObservation: (id: string) => get<Observation>(`/observations/${segment(id)}`),
  createObservation: (input: CreateObservationInput) => post<Observation>('/observations', input),

  /* geofences */
  listGeofences: (params?: { limit?: number; offset?: number }) =>
    get<GeofenceList>('/geofences', params),

  /* alerts */
  listAlerts: (params?: AlertFilters) => get<AlertList>('/alerts', params),
  getAlert: (id: string) => get<Alert>(`/alerts/${segment(id)}`),
  acknowledgeAlert: (id: string) => post<Alert>(`/alerts/${segment(id)}/acknowledge`),
  resolveAlert: (id: string) => post<Alert>(`/alerts/${segment(id)}/resolve`),

  /* incidents */
  listIncidents: (params?: IncidentFilters) => get<IncidentList>('/incidents', params),
  getIncident: (id: string) => get<IncidentDetail>(`/incidents/${segment(id)}`),
  createIncident: (input: CreateIncidentInput) => post<Incident>('/incidents', input),
  updateIncidentStatus: (id: string, status: string) =>
    post<Incident>(`/incidents/${segment(id)}/status`, { status }),
  attachIncidentRelation: (id: string, kind: string, relationId: string) =>
    post<Incident>(`/incidents/${segment(id)}/relations`, { kind, id: relationId }),

  /* missions */
  listMissions: (params?: MissionFilters) => get<MissionList>('/missions', params),
  getMission: (id: string) => get<Mission>(`/missions/${segment(id)}`),
  createMission: (input: CreateMissionInput) => post<Mission>('/missions', input),
  updateMissionStatus: (id: string, status: string) =>
    post<Mission>(`/missions/${segment(id)}/status`, { status }),
  assignMissionAsset: (id: string, assetId: string) =>
    post<Mission>(`/missions/${segment(id)}/assets`, { assetId }),

  /* commands */
  listCommands: (params?: CommandFilters) => get<CommandList>('/commands', params),
  getCommand: (id: string) => get<Command>(`/commands/${segment(id)}`),
  issueCommand: (input: IssueCommandInput) => post<Command>('/commands', input),
  transitionCommand: (id: string, state: string, reason?: string) =>
    post<Command>(`/commands/${segment(id)}/transition`, { state, reason: reason ?? '' }),

  /* assessments */
  listAssessments: (params?: AssessmentFilters) =>
    get<AssessmentList>('/assessments', params),
  createAssessment: (input: components['requestBodies']['Assessment']['content']['application/json']) =>
    post<Assessment>('/assessments', input),

  /* audit */
  listAudit: (params?: AuditFilters) => get<AuditList>('/audit', params),

  /* operators */
  listOperators: (params?: { limit?: number; offset?: number }) =>
    get<OperatorList>('/operators', params),

  /* scenarios */
  listScenarios: () => get<ScenarioSummaryList>('/scenarios'),
  getScenario: (name: string) => get<Scenario>(`/scenarios/${segment(name)}`),
  startScenario: (name: string, input: StartScenarioInput) =>
    post<ScenarioRun>(`/scenarios/definitions/${segment(name)}/start`, input),
  listScenarioRuns: (params?: { limit?: number; offset?: number }) =>
    get<ScenarioRunList>('/scenarios/runs', params),
  getScenarioRun: (id: string) => get<ScenarioRun>(`/scenarios/runs/${segment(id)}`),
  listScenarioRunEvents: (id: string) =>
    get<ScenarioEventList>(`/scenarios/runs/${segment(id)}/events`),
  pauseScenarioRun: (id: string) => post<ScenarioRun>(`/scenarios/runs/${segment(id)}/pause`),
  resumeScenarioRun: (id: string) => post<ScenarioRun>(`/scenarios/runs/${segment(id)}/resume`),
  stopScenarioRun: (id: string) => post<ScenarioRun>(`/scenarios/runs/${segment(id)}/stop`),
  restartScenarioRun: (id: string) => post<ScenarioRun>(`/scenarios/runs/${segment(id)}/restart`),
  setScenarioRunSpeed: (id: string, speed: number) =>
    post<ScenarioRun>(`/scenarios/runs/${segment(id)}/speed`, { speed }),
}

export function realtimeUrl(): string {
  const base = import.meta.env.VITE_API_BASE_URL
  if (base && /^https?:\/\//.test(base)) {
    const url = new URL(base)
    url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:'
    url.pathname = `${url.pathname.replace(/\/+$/, '')}/realtime`
    if (API_TOKEN) url.searchParams.set('access_token', API_TOKEN)
    return url.toString()
  }
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const url = new URL(`${protocol}//${window.location.host}/api/v1/realtime`)
  if (API_TOKEN) url.searchParams.set('access_token', API_TOKEN)
  return url.toString()
}
