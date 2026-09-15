/**
 * Typed REST client for the C4ISR backend.
 *
 * Field names mirror the Go transport responses exactly (camelCase JSON),
 * derived from internal/*\/http.go. Timestamps are RFC3339 strings.
 */

export interface Point {
  lat: number
  lng: number
}

export interface ListResponse<T> {
  items: T[]
  total: number
}

export interface ErrorEnvelope {
  error: {
    code: string
    message: string
  }
}

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
/* Domain types                                                        */
/* ------------------------------------------------------------------ */

export interface Source {
  id: string
  name: string
  type: string
  status: string
  metadata: Record<string, unknown>
  createdAt: string
  updatedAt: string
}

export interface Track {
  id: string
  externalRef: string
  status: string
  firstSeenAt: string
  lastSeenAt: string
  position: Point | null
  speed: number | null
  heading: number | null
  metadata: Record<string, unknown>
  createdAt: string
  updatedAt: string
  closedAt: string | null
  observationCount: number
}

export interface TrackHistoryPoint {
  id: string
  observedAt: string
  position: Point | null
  speed: number | null
  heading: number | null
}

export interface Classification {
  id: string
  trackId: string
  label: string
  confidence: number | null
  method: string
  sourceReference: string
  createdBy: string
  createdAt: string
}

export interface Observation {
  id: string
  sourceId: string
  type: string
  observedAt: string
  receivedAt: string
  processedAt?: string | null
  position?: Point | null
  payload: Record<string, unknown>
  quality?: Record<string, unknown> | null
  trackHint?: string
  createdAt: string
  duplicate?: boolean
}

export interface Asset {
  id: string
  name: string
  type: string
  status: string
  capabilities: string[]
  metadata: Record<string, unknown>
  position?: Point | null
  speed: number | null
  heading: number | null
  health: string
  connectionState: string
  lastSeenAt: string | null
  createdAt: string
  updatedAt: string
}

export interface TelemetrySample {
  id?: string
  messageId?: string
  assetId?: string
  sourceId?: string
  observedAt: string
  receivedAt: string
  position?: Point | null
  speed?: number | null
  heading?: number | null
  health?: string
  connectionState?: string
  payload?: Record<string, unknown>
  stale?: boolean
  duplicate?: boolean
}

export interface Geofence {
  id: string
  name: string
  type: string
  severity: string
  active: boolean
  geojson: string
  metadata: Record<string, unknown>
  createdAt: string
  updatedAt: string
}

export interface Alert {
  id: string
  type: string
  severity: string
  state: string
  title: string
  message: string
  sourceReference: Record<string, unknown>
  trackId: string
  assetId: string
  geofenceId: string
  incidentId: string
  createdAt: string
  updatedAt: string
  acknowledgedAt: string | null
  acknowledgedBy: string
  resolvedAt: string | null
  resolvedBy: string
}

export interface Incident {
  id: string
  title: string
  description: string
  priority: string
  status: string
  assignedOperator?: string
  createdAt: string
  updatedAt: string
  resolvedAt?: string | null
  closedAt?: string | null
}

export interface RelatedAlert {
  id: string
  type: string
  severity: string
  state: string
  title: string
  createdAt: string
}

export interface RelatedTrack {
  id: string
  externalRef?: string
  status: string
  position?: Point | null
  lastSeenAt: string
}

export interface RelatedAsset {
  id: string
  name: string
  type: string
  status: string
  position?: Point | null
  connectionState?: string
}

export interface RelatedObservation {
  id: string
  sourceId: string
  type: string
  observedAt: string
  position?: Point | null
}

export interface RelatedAssessment {
  id: string
  subjectType: string
  subjectId: string
  type: string
  conclusion: string
  method: string
  confidence?: number | null
  createdAt: string
}

export interface IncidentDetail extends Incident {
  alerts: RelatedAlert[]
  tracks: RelatedTrack[]
  assets: RelatedAsset[]
  observations: RelatedObservation[]
  assessments: RelatedAssessment[]
}

export interface MissionTask {
  id: string
  missionId: string
  type: string
  description: string
  status: string
  target?: Point | null
  createdAt: string
  updatedAt: string
}

export interface Mission {
  id: string
  name: string
  objective: string
  priority: string
  status: string
  incidentId?: string
  createdAt: string
  updatedAt: string
  startedAt?: string | null
  endedAt?: string | null
  assets: RelatedAsset[]
  tasks: MissionTask[]
}

export interface Command {
  id: string
  assetId: string
  missionId?: string
  incidentId?: string
  type: string
  payload: Record<string, unknown>
  state: string
  createdBy?: string
  correlationId?: string
  createdAt: string
  updatedAt: string
  queuedAt?: string | null
  sentAt?: string | null
  acknowledgedAt?: string | null
  completedAt?: string | null
  failureReason?: string
}

export interface AssessmentEvidence {
  type: string
  id: string
  addedAt: string
}

export interface Assessment {
  id: string
  subjectType: string
  subjectId: string
  type: string
  conclusion: string
  confidence: number | null
  method: string
  createdBy: string
  createdAt: string
  evidence: AssessmentEvidence[]
}

export interface AuditEntry {
  id: string
  occurredAt: string
  actorType: string
  actorId: string
  action: string
  subjectType: string
  subjectId: string
  correlationId: string
  data: Record<string, unknown>
}

export interface Operator {
  id: string
  name: string
  role: string
  createdAt: string
}

export interface ScenarioSummary {
  name: string
  description: string
  seed: number
  sources: number
  assets: number
  tracks: number
  geofences: number
  events: number
}

export interface ScenarioRun {
  id: string
  scenarioName: string
  seed: number
  status: string
  playbackSpeed: number
  virtualTimeMs: number
  startedAt: string
  endedAt?: string | null
  error?: string
}

export interface Health {
  status: string
  database: string
  time: string
}

export interface RealtimeEnvelope {
  id: string
  type: string
  version: number
  occurredAt: string
  data: Record<string, unknown>
}

/* ------------------------------------------------------------------ */
/* Request payloads                                                    */
/* ------------------------------------------------------------------ */

export interface CreateIncidentInput {
  title: string
  description: string
  priority: string
  alertIds: string[]
  trackIds: string[]
  assetIds: string[]
  observationIds: string[]
  assessmentIds: string[]
}

export interface CreateMissionInput {
  name: string
  objective: string
  priority: string
  incidentId?: string
  assets: string[]
  tasks?: Array<{ type: string; description: string; target?: Point }>
}

export interface IssueCommandInput {
  assetId: string
  missionId?: string
  incidentId?: string
  type: string
  payload: Record<string, unknown>
}

export interface CreateClassificationInput {
  trackId: string
  label: string
  confidence?: number
  method?: string
  sourceReference?: string
}

export interface CreateObservationInput {
  sourceId: string
  type: string
  position?: Point
  payload?: Record<string, unknown>
  trackHint?: string
}

export interface StartScenarioInput {
  speed: number
  seed?: number
}

export interface TrackFilters {
  limit?: number
  offset?: number
}

export interface AssetFilters {
  limit?: number
  offset?: number
}

export interface AlertFilters {
  state?: string
  severity?: string
  track_id?: string
  incident_id?: string
  limit?: number
  offset?: number
}

export interface IncidentFilters {
  status?: string
  limit?: number
  offset?: number
}

export interface MissionFilters {
  status?: string
  limit?: number
  offset?: number
}

export interface CommandFilters {
  asset_id?: string
  state?: string
  mission_id?: string
  limit?: number
  offset?: number
}

export interface AssessmentFilters {
  subject_type?: string
  subject_id?: string
  limit?: number
  offset?: number
}

export interface AuditFilters {
  subject_type?: string
  subject_id?: string
  action?: string
  since?: string
  limit?: number
  offset?: number
}

/* ------------------------------------------------------------------ */
/* Transport                                                           */
/* ------------------------------------------------------------------ */

export const OPERATOR_ID = import.meta.env.VITE_OPERATOR_ID ?? 'operator-01'

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
      'X-Operator-ID': OPERATOR_ID,
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
    headers: { Accept: 'application/json', 'X-Operator-ID': OPERATOR_ID },
    signal,
  })
  if (!response.ok) throw await parseError(response)
  return (await response.json()) as Health
}

const segment = (value: string) => encodeURIComponent(value)

export const api = {
  /* sources */
  listSources: (params?: { limit?: number; offset?: number }) =>
    get<ListResponse<Source>>('/sources', params),

  /* assets */
  listAssets: (params?: AssetFilters) => get<ListResponse<Asset>>('/assets', params),
  getAsset: (id: string) => get<Asset>(`/assets/${segment(id)}`),
  updateAssetStatus: (id: string, status: string) =>
    post<Asset>(`/assets/${segment(id)}/status`, { status }),
  listAssetTelemetry: (id: string, params?: { limit?: number; offset?: number }) =>
    get<ListResponse<TelemetrySample>>(`/assets/${segment(id)}/telemetry`, params),

  /* tracks */
  listTracks: (params?: TrackFilters) => get<ListResponse<Track>>('/tracks', params),
  getTrack: (id: string) => get<Track>(`/tracks/${segment(id)}`),
  getTrackHistory: (id: string, params?: { limit?: number }) =>
    get<ListResponse<TrackHistoryPoint>>(`/tracks/${segment(id)}/history`, params),
  listTrackClassifications: (id: string, params?: { limit?: number; offset?: number }) =>
    get<ListResponse<Classification>>(`/tracks/${segment(id)}/classifications`, params),

  /* classifications */
  createClassification: (input: CreateClassificationInput) =>
    post<Classification>('/classifications', input),

  /* observations */
  listObservations: (params?: { track_id?: string; limit?: number; offset?: number }) =>
    get<ListResponse<Observation>>('/observations', params),
  getObservation: (id: string) => get<Observation>(`/observations/${segment(id)}`),
  createObservation: (input: CreateObservationInput) => post<Observation>('/observations', input),

  /* geofences */
  listGeofences: (params?: { limit?: number; offset?: number }) =>
    get<ListResponse<Geofence>>('/geofences', params),

  /* alerts */
  listAlerts: (params?: AlertFilters) => get<ListResponse<Alert>>('/alerts', params),
  getAlert: (id: string) => get<Alert>(`/alerts/${segment(id)}`),
  acknowledgeAlert: (id: string) => post<Alert>(`/alerts/${segment(id)}/acknowledge`),
  resolveAlert: (id: string) => post<Alert>(`/alerts/${segment(id)}/resolve`),

  /* incidents */
  listIncidents: (params?: IncidentFilters) => get<ListResponse<Incident>>('/incidents', params),
  getIncident: (id: string) => get<IncidentDetail>(`/incidents/${segment(id)}`),
  createIncident: (input: CreateIncidentInput) => post<Incident>('/incidents', input),
  updateIncidentStatus: (id: string, status: string) =>
    post<Incident>(`/incidents/${segment(id)}/status`, { status }),
  attachIncidentRelation: (id: string, kind: string, relationId: string) =>
    post<Incident>(`/incidents/${segment(id)}/relations`, { kind, id: relationId }),

  /* missions */
  listMissions: (params?: MissionFilters) => get<ListResponse<Mission>>('/missions', params),
  getMission: (id: string) => get<Mission>(`/missions/${segment(id)}`),
  createMission: (input: CreateMissionInput) => post<Mission>('/missions', input),
  updateMissionStatus: (id: string, status: string) =>
    post<Mission>(`/missions/${segment(id)}/status`, { status }),
  assignMissionAsset: (id: string, assetId: string) =>
    post<Mission>(`/missions/${segment(id)}/assets`, { assetId }),

  /* commands */
  listCommands: (params?: CommandFilters) => get<ListResponse<Command>>('/commands', params),
  getCommand: (id: string) => get<Command>(`/commands/${segment(id)}`),
  issueCommand: (input: IssueCommandInput) => post<Command>('/commands', input),
  transitionCommand: (id: string, state: string, reason?: string) =>
    post<Command>(`/commands/${segment(id)}/transition`, { state, reason: reason ?? '' }),

  /* assessments */
  listAssessments: (params?: AssessmentFilters) =>
    get<ListResponse<Assessment>>('/assessments', params),
  createAssessment: (input: {
    subjectType: string
    subjectId: string
    type: string
    conclusion: string
    confidence?: number
    method?: string
    evidence?: Array<{ type: string; id: string }>
  }) => post<Assessment>('/assessments', input),

  /* audit */
  listAudit: (params?: AuditFilters) => get<ListResponse<AuditEntry>>('/audit', params),

  /* operators */
  listOperators: (params?: { limit?: number; offset?: number }) =>
    get<ListResponse<Operator>>('/operators', params),

  /* scenarios */
  listScenarios: () => get<ListResponse<ScenarioSummary>>('/scenarios'),
  getScenario: (name: string) => get<Record<string, unknown>>(`/scenarios/${segment(name)}`),
  startScenario: (name: string, input: StartScenarioInput) =>
    post<ScenarioRun>(`/scenarios/${segment(name)}/start`, input),
  listScenarioRuns: (params?: { limit?: number; offset?: number }) =>
    get<ListResponse<ScenarioRun>>('/scenarios/runs', params),
  getScenarioRun: (id: string) => get<ScenarioRun>(`/scenarios/runs/${segment(id)}`),
  pauseScenarioRun: (id: string) => post<ScenarioRun>(`/scenarios/runs/${segment(id)}/pause`),
  resumeScenarioRun: (id: string) => post<ScenarioRun>(`/scenarios/runs/${segment(id)}/resume`),
  stopScenarioRun: (id: string) => post<ScenarioRun>(`/scenarios/runs/${segment(id)}/stop`),
  setScenarioRunSpeed: (id: string, speed: number) =>
    post<ScenarioRun>(`/scenarios/runs/${segment(id)}/speed`, { speed }),
}

export function realtimeUrl(): string {
  const base = import.meta.env.VITE_API_BASE_URL
  if (base && /^https?:\/\//.test(base)) {
    const url = new URL(base)
    url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:'
    url.pathname = `${url.pathname.replace(/\/+$/, '')}/realtime`
    return url.toString()
  }
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${protocol}//${window.location.host}/api/v1/realtime`
}
