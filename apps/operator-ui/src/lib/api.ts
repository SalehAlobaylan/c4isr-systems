import { useWorkspace, type Resource } from '@/lib/workspaceContext'
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

export type ImageryScene = components['schemas']['ImageryScene']
export type ImageryJob = components['schemas']['ImageryJob']
export type ImageryResult = components['schemas']['ImageryResultArtifact']
export type ImageryRevision = components['schemas']['AssessmentRevision']
export type ImagerySubmit = components['schemas']['ImagerySubmit']
export type ImageryProposal = components['schemas']['ImageryProposal']
export type ImageryReviewAction = components['schemas']['AssessmentReviewAction']
export type ArtifactInput = components['schemas']['ArtifactInput']
export type FeatureReadiness = components['schemas']['FeatureReadiness']
export type ImageryCapture = components['schemas']['ImageryCapture']
export type ExerciseEnvironmentView = components['schemas']['ExerciseEnvironmentView']
export type PublicEvent = components['schemas']['PublicEvent']
export type PublicEventQuery = NonNullable<paths['/api/v1/public-events']['get']['parameters']['query']>
export type ContextPreview = components['schemas']['ContextPreview']
export type ExternalTrack = components['schemas']['ExternalTrack']
export type ReferenceDataset = components['schemas']['ReferenceDataset']
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
export type Picture = components['schemas']['Picture']
export type PictureTrack = components['schemas']['PictureTrack']
export type PictureAsset = components['schemas']['PictureAsset']
export type PictureAlert = components['schemas']['PictureAlert']
export type PictureWindow = components['schemas']['PictureWindow']

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
  resource: { workspaceId: string; environmentId: string; partition: 'PUBLIC_EVIDENCE' | 'EXERCISE_OBSERVATION'; runId?: string }
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
const sessionToken = (() => { try { return sessionStorage.getItem('emad.session-token') } catch { return null } })()
export const API_TOKEN = (sessionToken ?? runtimeConfig.apiToken ?? import.meta.env.VITE_API_TOKEN ?? '').trim()
export function switchSession(token: string) {
  const value = token.trim()
  if (!value || value.length > 4096 || /[\r\n]/.test(value)) throw new Error('Enter a valid credential.')
  sessionStorage.setItem('emad.session-token', value)
  // Reload closes sockets and discards every cache before another identity is
  // authenticated. Credentials never travel in a route or query parameter.
  window.location.assign('/')
}

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
  // `openapi-fetch` has already read the response body: `payload` is its
  // parsed value, the documented envelope for a JSON error and the raw text
  // otherwise. Cloning the response here is guaranteed to throw ("Response
  // body is already used") for any body that is not the envelope, and the
  // throw replaced the server's message with a JavaScript TypeError in the
  // operator's error state.
  const parsed = errorEnvelope(payload)
  if (parsed) {
    code = parsed.error.code
    message = parsed.error.message
  } else if (typeof payload === 'string') {
    // A short plain-text failure (a proxy's "upstream connect error") is more
    // truthful than the status alone. An HTML error page is not shown.
    const text = payload.trim()
    if (text.length > 0 && text.length <= 200 && !text.startsWith('<')) message = text
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

// Each instance captures one resource and cancellation lifetime.
export function createAPI(resource?: Resource, signal?: AbortSignal) {
  const apiClient = createClient<paths>({
    // Generated OpenAPI paths already include /api/v1. Keep any configured
    // gateway prefix in the base URL, while avoiding a duplicated /api/v1.
    baseUrl: apiClientBaseUrl(API_BASE),
    fetch: (request) => fetch(new Request(request, { cache:'no-store', signal: signal ? AbortSignal.any([request.signal, signal]) : request.signal })),
    headers: {
      Accept: 'application/json',
      ...(resource ? {
        'X-EMAD-Workspace-ID': resource.workspaceId,
        'X-EMAD-Environment-ID': resource.environmentId,
        'X-EMAD-Partition': resource.partition,
        ...(resource.runId ? { 'X-EMAD-Run-ID': resource.runId } : {}),
      } : {}),
      ...(API_TOKEN ? { Authorization: `Bearer ${API_TOKEN}` } : {}),
    },
  })

  async function fetchHealth(signal?: AbortSignal): Promise<Health> {
    return unwrap(apiClient.GET('/health', signal ? { signal } : undefined))
  }

  const api = {
    fetchHealth,
    /* identity */
    getCurrentOperator: () => unwrap(apiClient.GET('/api/v1/auth/me')),
    getReadiness: () => unwrap(apiClient.GET('/api/v1/platform/readiness')),
    createSurroundingArea: (body:{longitude:number;latitude:number;radiusM:number}) => unwrap(apiClient.POST('/api/v1/map/surrounding-area',{body})),
    measureMapGeometry: (geometry:Record<string,unknown>) => unwrap(apiClient.POST('/api/v1/map/measure',{body:{geometry}})),
    listGeographicRegions: (query?: {q?:string;parent?:string;kind?:'country'|'region'|'governorate'|'place'|'study';id?:string;offset?:number}) => unwrap(apiClient.GET('/api/v1/geography/regions',{params:{query}})),
    getMapFeatures: (query: {geometry?:string;family:'events'|'earthquakes'|'thermal'|'weather'|'bulletins'|'aircraft'|'vessels'|'imagery'|'coverage'|'findings';region_id?:string;bbox?:string;view?:'latest'|'known_at'|'effective_at';as_of?:string;observed_from?:string;observed_to?:string;source_id?:string;quality?:string;zoom?:number},signal?:AbortSignal) => unwrap(apiClient.GET('/api/v1/map/features',{params:{query},signal})),
    getReferenceTile: (z:number,x:number,y:number,datasets:string[],signal?:AbortSignal) => unwrap(apiClient.GET('/api/v1/map/reference/{z}/{x}/{y}.pbf',{params:{path:{z,x,y},query:{datasets:datasets.join(',')}},signal,parseAs:'arrayBuffer'})),
    getSourceActivation: (id: string) => unwrap(apiClient.GET('/api/v1/sources/{id}/activation', {params:{path:{id}}})),
    getReferenceViewport: (bbox: string, datasets: string[], offset = 0) => unwrap(apiClient.GET('/api/v1/reference-features', {params:{query:{bbox,datasets:datasets.join(','),offset}}})),
    listRuntimeCapabilities: () => unwrap(apiClient.GET('/api/v1/simulation-runtime/capabilities')),
    listImageryCaptures: (region_id?:string,geometry?:string) => unwrap(apiClient.GET('/api/v1/imagery/captures',{params:{query:{region_id,geometry}}})),
    createImageryCapture: (body: components['schemas']['ImageryCaptureInput']) => unwrap(apiClient.POST('/api/v1/imagery/captures', {body})),
    retryImageryCapture: (id: string) => unwrap(apiClient.POST('/api/v1/imagery/captures/{id}/retry',{params:{path:{id}}})),
    cancelImageryCapture: (id: string) => unwrap(apiClient.POST('/api/v1/imagery/captures/{id}/cancel',{params:{path:{id}}})),
    getExerciseEnvironment: (id: string) => unwrap(apiClient.GET('/api/v1/exercise-view/{id}',{params:{path:{id}}})),
    downloadExerciseEnvironment: (id: string) => unwrap(apiClient.GET('/api/v1/exercise-view/{id}/content',{params:{path:{id}},parseAs:'blob'})),
    getSpatialReplay: (id: string, elapsedMs: number, reason: string) => unwrap(apiClient.GET('/api/v1/simulation-recordings/{id}/spatial',{params:{path:{id},query:{elapsedMs,reason}}})),

    listSensorPresets: () => unwrap(apiClient.GET('/api/v1/simulation-studio/sensor-presets')),
    createSensorPreset: (body: components['schemas']['SensorPresetInput']) => unwrap(apiClient.POST('/api/v1/simulation-studio/sensor-presets', {body})),
    getRecording: (id: string) => unwrap(apiClient.GET('/api/v1/simulation-recordings/{id}', {params:{path:{id}}})),
    archiveRecording: (id: string) => unwrap(apiClient.POST('/api/v1/simulation-recordings/{id}/archive', {params:{path:{id}}})),
    getRecordingFrame: (id: string, elapsedMs: number, sequence = 0) => unwrap(apiClient.GET('/api/v1/simulation-recordings/{id}/frame', {params:{path:{id},query:{elapsedMs,sequence}}})),
    getRecordingEvents: (id: string, after = 0) => unwrap(apiClient.GET('/api/v1/simulation-recordings/{id}/events', {params:{path:{id},query:{after}}})),
    getRecordingCheckpoint: (id: string) => unwrap(apiClient.GET('/api/v1/simulation-recordings/{id}/checkpoint', {params:{path:{id}}})),
    getEvaluation: (id: string) => unwrap(apiClient.GET('/api/v1/simulation-recordings/{id}/evaluation', {params:{path:{id}}})),
    releaseEvaluation: (id: string, body: components['schemas']['EvaluationRelease']) => unwrap(apiClient.POST('/api/v1/simulation-recordings/{id}/release', {params:{path:{id}},body})),
    getReleasedReport: (id: string) => unwrap(apiClient.GET('/api/v1/simulation-recordings/{id}/report', {params:{path:{id}}})),
    listStudioPackages: () => unwrap(apiClient.GET('/api/v1/simulation-studio/context-packages')),
    listStudioScenes: () => unwrap(apiClient.GET('/api/v1/simulation-studio/scenes')),
    getStudioScene: (id: string) => unwrap(apiClient.GET('/api/v1/simulation-studio/scenes/{id}', { params: { path: { id } } })),
    downloadStudioScene: (id: string) => unwrap(apiClient.GET('/api/v1/simulation-studio/scenes/{id}', { params: { path: { id } }, parseAs: 'blob' })),
    freezeStudioScene: (body: components['schemas']['SceneInput']) => unwrap(apiClient.POST('/api/v1/simulation-studio/scenes', { body })),
    listNativeRuns: () => unwrap(apiClient.GET('/api/v1/simulation-runtime/runs')),
    getNativeRun: (id: string) => unwrap(apiClient.GET('/api/v1/simulation-runtime/runs/{id}', { params: { path: { id } } })),
    getNativeTruth: (id: string) => unwrap(apiClient.GET('/api/v1/simulation-runtime/runs/{id}/truth', {params:{path:{id}}})),
    startNativeRun: (body: components['schemas']['RuntimeStart']) => unwrap(apiClient.POST('/api/v1/simulation-runtime/runs', { body })),
    pauseNativeRun: (id: string) => unwrap(apiClient.POST('/api/v1/simulation-runtime/runs/{id}/pause', { params: { path: { id } } })),
    resumeNativeRun: (id: string) => unwrap(apiClient.POST('/api/v1/simulation-runtime/runs/{id}/resume', { params: { path: { id } } })),
    stopNativeRun: (id: string) => unwrap(apiClient.POST('/api/v1/simulation-runtime/runs/{id}/stop', { params: { path: { id } } })),

    listSimulationUploads: () => unwrap(apiClient.GET('/api/v1/simulation-assets/uploads')),
    listSimulationModels: () => unwrap(apiClient.GET('/api/v1/simulation-assets/models')),
    listSimulationTemplates: () => unwrap(apiClient.GET('/api/v1/simulation-assets/templates')),
    listSimulationEnvironments: () => unwrap(apiClient.GET('/api/v1/simulation-assets/environments')),
    getSimulationUpload: (id: string) => unwrap(apiClient.GET('/api/v1/simulation-assets/uploads/{id}', { params: { path: { id } } })),
    getSimulationModel: (id: string) => unwrap(apiClient.GET('/api/v1/simulation-assets/models/{id}', { params: { path: { id } } })),
    listSimulationImportAttempts: (id: string) => unwrap(apiClient.GET('/api/v1/simulation-assets/uploads/{id}/attempts', { params: { path: { id } } })),
    reserveSimulationAsset: (body: components['schemas']['ModelAssetUploadInput']) => unwrap(apiClient.POST('/api/v1/simulation-assets/uploads', { body })),
    uploadSimulationAsset: (id: string, bytes: ArrayBuffer) => unwrap(apiClient.PUT('/api/v1/simulation-assets/uploads/{id}/content', { params: { path: { id } }, body: '', bodySerializer: () => bytes, headers: { 'Content-Type': 'application/octet-stream' } })),
    downloadSimulationAsset: (id: string) => unwrap(apiClient.GET('/api/v1/simulation-assets/uploads/{id}/content', { params: { path: { id } }, parseAs: 'blob' })),
    getSimulationModelManifest: (id: string) => unwrap(apiClient.GET('/api/v1/simulation-assets/models/{id}/manifest', { params: { path: { id } }, parseAs: 'blob' })),
    createSimulationTemplate: (body: components['schemas']['ModelActorTemplateInput']) => unwrap(apiClient.POST('/api/v1/simulation-assets/templates', { body })),
    createSimulationEnvironment: (body: components['schemas']['ModelEnvironmentInput']) => unwrap(apiClient.POST('/api/v1/simulation-assets/environments', { body })),
    listContextRecords: (kind?: 'WEATHER' | 'REPORT', bbox?: string,region_id?:string,geometry?:string) => unwrap(apiClient.GET('/api/v1/public-context/records', { params: { query: { kind, bbox, region_id, geometry } } })),
    getContextRecord: (id: string) => unwrap(apiClient.GET('/api/v1/public-context/records/{id}', { params: { path: { id } } })),
    listContextHistory: (id: string) => unwrap(apiClient.GET('/api/v1/public-context/records/{id}/history', { params: { path: { id } } })),
    listReportAssessments: () => unwrap(apiClient.GET('/api/v1/assessments/reports')),
    proposeReportAssessment: (body: components['schemas']['ReportProposal']) => unwrap(apiClient.POST('/api/v1/assessments/report-proposals', { body })),
    reviseReportAssessment: (id: string, body: components['schemas']['ReportProposal']) => unwrap(apiClient.POST('/api/v1/assessments/{id}/revisions', { params: { path: { id } }, body })),
    listExternalTracks: (bbox?: string,region_id?:string,geometry?:string) => unwrap(apiClient.GET('/api/v1/external-tracks', { params: { query: { bbox, region_id, geometry } } })),
    getExternalTrack: (id: string) => unwrap(apiClient.GET('/api/v1/external-tracks/{id}', { params: { path: { id } } })),
    getExternalAssociations: (id: string) => unwrap(apiClient.GET('/api/v1/external-tracks/{id}/associations', { params: { path: { id } } })),
    previewContextPackage: (body: components['schemas']['ContextPackageSelection']) => unwrap(apiClient.POST('/api/v1/context-packages/preview', { body })),
    listImageryScenes: (bbox?: string,region_id?:string,geometry?:string) => unwrap(apiClient.GET('/api/v1/imagery/scenes', { params: { query: { bbox, region_id, geometry } } })),
    getImageryScene: (id: string) => unwrap(apiClient.GET('/api/v1/imagery/scenes/{id}', { params: { path: { id } } })),
    listImageryJobs: () => unwrap(apiClient.GET('/api/v1/imagery/jobs')),
    getImageryJob: (id: string) => unwrap(apiClient.GET('/api/v1/imagery/jobs/{id}', { params: { path: { id } } })),
    getImageryResult: (id: string) => unwrap(apiClient.GET('/api/v1/imagery/jobs/{id}/result', { params: { path: { id } } })),
    submitImageryJob: (body: ImagerySubmit) => unwrap(apiClient.POST('/api/v1/imagery/jobs', { body })),
    cancelImageryJob: (id: string) => unwrap(apiClient.POST('/api/v1/imagery/jobs/{id}/cancel', { params: { path: { id } } })),
    retryImageryJob: (id: string) => unwrap(apiClient.POST('/api/v1/imagery/jobs/{id}/retry', { params: { path: { id } } })),
    listImageryAttempts: (id: string) => unwrap(apiClient.GET('/api/v1/imagery/jobs/{id}/attempts', { params: { path: { id } } })),
    reserveArtifact: (body: ArtifactInput) => unwrap(apiClient.POST('/api/v1/artifacts', { body })),
    getArtifact: (id: string) => unwrap(apiClient.GET('/api/v1/artifacts/{id}', { params: { path: { id } } })),
    uploadArtifact: (id: string, bytes: ArrayBuffer) => unwrap(apiClient.PUT('/api/v1/artifacts/{id}/content', { params: { path: { id } }, body: '', bodySerializer: () => bytes, headers: { 'Content-Type': 'application/octet-stream' } })),
    downloadArtifact: (id: string) => unwrap(apiClient.GET('/api/v1/artifacts/{id}/content', { params: { path: { id } }, parseAs: 'blob' })),
    listIntelligenceAssessments: () => unwrap(apiClient.GET('/api/v1/assessments/intelligence')),
    proposeImageryAssessment: (body: ImageryProposal) => unwrap(apiClient.POST('/api/v1/assessments/imagery-proposals', { body })),
    reviseImageryAssessment: (id: string, body: ImageryProposal) => unwrap(apiClient.POST('/api/v1/assessments/{id}/revisions', { params: { path: { id } }, body })),
    listAssessmentRevisions: (id: string) => unwrap(apiClient.GET('/api/v1/assessments/{id}/revisions', { params: { path: { id } } })),
    reviewImageryAssessment: (id: string, body: ImageryReviewAction) => unwrap(apiClient.POST('/api/v1/assessments/{id}/review', { params: { path: { id } }, body })),
    submitAssessmentDraft: (id: string, body: ImageryReviewAction) => unwrap(apiClient.POST('/api/v1/assessments/{id}/submit', { params: { path: { id } }, body })),
    /* Public evidence and reference snapshots. Instances retain scope and cancellation. */
    listPublicEvents: (params?: PublicEventQuery) => unwrap(apiClient.GET('/api/v1/public-events', { params: { query: { limit: 200, ...params } } })),
    getPublicEvent: (id: string, query?: PublicEventQuery) => unwrap(apiClient.GET('/api/v1/public-events/{id}', { params: { path: { id }, query } })),
    listPublicEventRevisions: (id: string, query?: PublicEventQuery) => unwrap(apiClient.GET('/api/v1/public-events/{id}/revisions', { params: { path: { id }, query: { limit: 200, ...query } } })),
    listReferenceDatasets: () => unwrap(apiClient.GET('/api/v1/reference-datasets')),
    getReferenceArtifact: (id: string) => unwrap(apiClient.GET('/api/v1/reference-datasets/{id}/artifact', { params: { path: { id } }, parseAs: 'text' })),
    getReferenceFeatures: (id: string) => unwrap(apiClient.GET('/api/v1/reference-datasets/{id}/features', { params: { path: { id } } })),
    getSourceEvidence: (id: string, evidenceID: string) => unwrap(apiClient.GET('/api/v1/sources/{id}/evidence/{evidenceID}', { params: { path: { id, evidenceID } } })),
    getSourceOriginal: (id: string, evidenceID: string) => unwrap(apiClient.GET('/api/v1/sources/{id}/evidence/{evidenceID}/original', { params: { path: { id, evidenceID } }, parseAs: 'text' })),
    getSourceAdmissionVersion: (id: string, admissionID: string) => unwrap(apiClient.GET('/api/v1/sources/{id}/admissions/{admissionID}', { params: { path: { id, admissionID } } })),
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

    /* picture (replay) */
    // Reconstructs the picture at an instant. Omitting `at` returns the live
    // picture, so the interface has one code path for live and replayed views.
    getPicture: (params?: { at?: string; live?: boolean; scope?: string; limit?: number }) =>
      unwrap(apiClient.GET('/api/v1/picture', { params: { query: params } })),
    getPictureWindow: (params?: { scope?: string }) =>
      unwrap(apiClient.GET('/api/v1/picture/window', { params: { query: params } })),

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
    listContextPackages: () => unwrap(apiClient.GET('/api/v1/context-packages')),
    buildContextPackage: (body: components['schemas']['ContextPackageBuild']) => unwrap(apiClient.POST('/api/v1/context-packages', { body })),
    getContextPackage: (id: string) => unwrap(apiClient.GET('/api/v1/context-packages/{id}', { params: { path: { id } } })),
    downloadContextPackage: (id: string) => unwrap(apiClient.GET('/api/v1/context-packages/{id}/bundle', { params: { path: { id } } })),
    listSyntheticMessages: (id: string) => unwrap(apiClient.GET('/api/v1/scenarios/runs/{id}/synthetic-messages', { params: { path: { id } } })),
    getScenarioRunManifest: (id: string) => unwrap(apiClient.GET('/api/v1/scenarios/runs/{id}/manifest', { params: { path: { id } } })),
    getScenarioRunVersion: (id: string) => unwrap(apiClient.GET('/api/v1/scenarios/runs/{id}/version', { params: { path: { id } } })),
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

  return api
}

export function useAPI() { return useWorkspace().api }

export function realtimeUrl(resource?: Resource): string {
  // Reuse the configured API path so a gateway prefix such as
  // /proxy/api/v1 also routes the WebSocket upgrade to the backend.
  const url = new URL(API_BASE, window.location.origin)
  url.pathname = `${url.pathname.replace(/\/+$/, '')}/realtime`
  if (/^https?:\/\//.test(API_BASE)) {
    url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:'
  } else {
    url.protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  }
  if (resource) {
    url.searchParams.set('workspaceId', resource.workspaceId)
    url.searchParams.set('environmentId', resource.environmentId)
    url.searchParams.set('partition', resource.partition)
    if (resource.runId) url.searchParams.set('runId', resource.runId)
  }
  return url.toString()
}

export function realtimeProtocols(): string[] {
  return API_TOKEN ? ['emad.realtime.v2', `emad.auth.${encodeURIComponent(API_TOKEN)}`] : ['emad.realtime.v2']
}
