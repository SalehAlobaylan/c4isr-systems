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

export function retryConcurrentRead(attempt: number, error: Error): boolean {
  return attempt < 3 && error instanceof ApiError && error.status === 409 &&
    (error.message === 'concurrent read capacity claim; restart this view' ||
     error.message === 'concurrent read budget claim; retry the view')
}

export function isReadCapacityExhausted(error: unknown): boolean {
  return error instanceof ApiError && error.status === 409 && error.message.includes('active read capacity exhausted')
}

/* ------------------------------------------------------------------ */
/* Contract-backed domain aliases                                      */
/* ------------------------------------------------------------------ */

export type ContentPurposeEntry = components['schemas']['ContentPurposeEntry']
export type ContentPurposeInput = components['schemas']['ContentPurposeInput']
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
export type AreaSelection = components['schemas']['AreaSelection']
export type AreaRevision = components['schemas']['AreaRevision']
export type TemporalScope = components['schemas']['TemporalScope']
export type RetrievalProfileInput = components['schemas']['RetrievalProfileInput']
export type RetrievalProfile = components['schemas']['RetrievalProfile']
export type ReferenceDepthRequestInput = components['schemas']['ReferenceDepthRequestInput']
export type Investigation = components['schemas']['Investigation']
export type InvestigationInput = components['schemas']['InvestigationInput']
export type InvestigationMembership = components['schemas']['InvestigationMembership']
export type ScopeInput = components['schemas']['ScopeInput']
export type AcquisitionInput=components['schemas']['AcquisitionInput']
export type AcquisitionRequest=components['schemas']['AcquisitionRequest']
export type CoverageResult=components['schemas']['CoverageResult']
export type CoverageAction=components['schemas']['CoverageAction']
export type ScopeSnapshot = components['schemas']['ScopeSnapshot']
export type ScopeReadInput = components['schemas']['ScopeReadInput']
export type ScopeReadSnapshot = components['schemas']['ScopeReadSnapshot']
export const fixedScopeInput = (s:ScopeSnapshot):ScopeInput => ({area:s.area.persistent?{revisionId:s.area.revisionId}:s.area.geography?{kind:s.area.kind,regionId:s.area.geography.regionId,sourceId:s.area.geography.sourceId,releaseId:s.area.geography.releaseId}:{kind:s.area.kind,name:s.area.name,geometry:s.area.originalGeometry,parentRevisionId:s.area.parentRevisionId},temporal:{mode:s.temporal.mode,interval:s.temporal.interval,revisionView:s.temporal.revisionView,asOf:s.temporal.asOf,timezone:s.temporal.timezone,backgroundImagery:s.temporal.backgroundImagery}})
const scopeQuery = (s:ScopeSnapshot) => ({scope:JSON.stringify(fixedScopeInput(s)),scope_hash:s.scopeHash})
async function unwrapScoped<T>(result:Promise<{data?:T;error?:unknown;response:Response}>,scope?:ScopeSnapshot):Promise<T>{
 const resolved=await result
 if(scope&&resolved.response.ok&&resolved.response.headers.get('X-Analyst-Scope-Hash')!==scope.scopeHash)throw new Error('The response belongs to a different scope. Refresh this view.')
 if(resolved.response.status===401||resolved.response.status===403)window.dispatchEvent(new Event('emad:access-denied'))
 return unwrap(Promise.resolve(resolved))
}
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

const localEntryToken = (runtimeConfig.apiToken ?? import.meta.env.VITE_API_TOKEN ?? '').trim()
export const LOCAL_AUTO_ENTRY = runtimeConfig.localAutoEntry === true || import.meta.env.VITE_LOCAL_AUTO_ENTRY === 'true'
export const LOCAL_BASEMAP_ONLY = (runtimeConfig.localBasemapOnly || import.meta.env.VITE_LOCAL_BASEMAP_ONLY) === 'true'
export const GEOGRAPHIC_PREVIEW_ENABLED = (runtimeConfig.geographicPreviewEnabled || import.meta.env.VITE_GEOGRAPHIC_PREVIEW_ENABLED) !== 'false'

// A previous manual account can outlive the local installation that issued it.
// Only explicitly configured local launchers may recover to their default account.
export function restoreLocalSession(): boolean {
  if (!LOCAL_AUTO_ENTRY || !localEntryToken || sessionToken === null) return false
  try { sessionStorage.removeItem('emad.session-token') } catch { return false }
  window.location.reload()
  return true
}
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
  const analystReads = new Set<string>()
  const catalogReads = new Set<string>()
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

  // Release requests must survive page navigation and workspace cancellation.
  // The same scoped headers and credentials still go through the API client.
  const releaseFetch = (request: Request) => fetch(request)
  async function releaseCatalog(id: string) {
    await unwrap(apiClient.DELETE('/api/v1/catalog/reads/{id}', {params:{path:{id}}, keepalive:true, fetch:releaseFetch}))
    catalogReads.delete(id)
  }
  async function releaseAnalystRead(id: string) {
    await unwrap(apiClient.POST('/api/v1/analyst-scope/reads/{id}/release', {params:{path:{id}}, keepalive:true, fetch:releaseFetch}))
    analystReads.delete(id)
  }

  async function fetchHealth(signal?: AbortSignal): Promise<Health> {
    return unwrap(apiClient.GET('/health', signal ? { signal } : undefined))
  }

  const api = {
    fetchHealth,
    getGeographicAvailability:(region_id:string,release_id?:string,requestSignal?:AbortSignal)=>unwrapScoped(apiClient.GET('/api/v1/geography/availability',{params:{query:{region_id,release_id}},signal:requestSignal})),
    requestReferenceDepth:(body:ReferenceDepthRequestInput)=>unwrapScoped(apiClient.POST('/api/v1/geography/reference-requests',{body})),
    listReferenceDepthRequests:(requestSignal?:AbortSignal)=>unwrapScoped(apiClient.GET('/api/v1/geography/reference-requests',{signal:requestSignal})),
    listGeographicReleaseHeads:(requestSignal?:AbortSignal)=>unwrap(apiClient.GET('/api/v1/geography/releases',{signal:requestSignal})),
    resolveReferenceDepthRequest:(id:string,releaseId:string)=>unwrapScoped(apiClient.POST('/api/v1/geography/reference-requests/{id}/resolve',{params:{path:{id}},body:{releaseId}})),
    listRetrievalProfiles:(id:string,requestSignal?:AbortSignal)=>unwrap(apiClient.GET('/api/v1/sources/{id}/profiles',{params:{path:{id}},signal:requestSignal})),
    approveRetrievalProfile:(id:string,key:string,body:RetrievalProfileInput)=>unwrap(apiClient.POST('/api/v1/sources/{id}/profiles/{key}',{params:{path:{id,key}},body})),
    grantRetrievalExecution:(id:string,key:string,body:components['schemas']['RetrievalExecutionGrant'])=>unwrap(apiClient.POST('/api/v1/sources/{id}/profiles/{key}/execution',{params:{path:{id,key}},body})),
    listInvestigations:(offset=0,requestSignal?:AbortSignal)=>unwrapScoped(apiClient.GET('/api/v1/investigations',{params:{query:{offset}},signal:requestSignal})),
    getInvestigation:(id:string,revision?:string,requestSignal?:AbortSignal)=>unwrapScoped(apiClient.GET('/api/v1/investigations/{id}',{params:{path:{id},query:{revision}},signal:requestSignal})),
    saveInvestigation:(body:InvestigationInput,id?:string)=>id?unwrapScoped(apiClient.POST('/api/v1/investigations/{id}/revisions',{params:{path:{id}},body})):unwrapScoped(apiClient.POST('/api/v1/investigations',{body})),
    changeInvestigationMembers:(id:string,body:InvestigationMembership)=>unwrapScoped(apiClient.POST('/api/v1/investigations/{id}/members',{params:{path:{id}},body})),
    /* identity */
    getCurrentOperator: () => unwrap(apiClient.GET('/api/v1/auth/me')),
    getReadiness: () => unwrap(apiClient.GET('/api/v1/platform/readiness')),
    resolveArea:(body:AreaSelection,requestSignal?:AbortSignal)=>unwrapScoped(apiClient.POST('/api/v1/areas/resolve',{body,signal:requestSignal})),
    persistArea:(body:AreaSelection,signal?:AbortSignal)=>unwrapScoped(apiClient.POST('/api/v1/areas/revisions',{body,signal})),
    environmentProfiles:(signal?:AbortSignal)=>unwrapScoped(apiClient.GET('/api/v1/environment-profiles',{signal})),
    environmentAssets:(after?:string,signal?:AbortSignal,source?:string)=>unwrapScoped(apiClient.GET('/api/v1/environment-assets',{params:{query:{after,source}},signal})),
    environmentDefinitions:(after?:string,signal?:AbortSignal)=>unwrapScoped(apiClient.GET('/api/v1/geographic-environments',{params:{query:{after}},signal})),
    saveEnvironment:(body:components['schemas']['EnvironmentDefinitionInput'],id?:string,signal?:AbortSignal)=>id
      ? unwrapScoped(apiClient.PUT('/api/v1/geographic-environments/{id}',{params:{path:{id}},body,signal}))
      : unwrapScoped(apiClient.POST('/api/v1/geographic-environments',{body,signal})),
    environmentPreflight:(id:string,signal?:AbortSignal)=>unwrapScoped(apiClient.GET('/api/v1/geographic-environments/{id}/preflight',{params:{path:{id}},signal})),
    submitEnvironment:(id:string,body:components['schemas']['EnvironmentBuildInput'],signal?:AbortSignal)=>unwrapScoped(apiClient.POST('/api/v1/geographic-environments/{id}/builds',{params:{path:{id}},body,signal})),
    environmentBuild:(id:string,signal?:AbortSignal)=>unwrapScoped(apiClient.GET('/api/v1/environment-builds/{id}',{params:{path:{id}},signal})),
    submitEnvironmentDisplay:(id:string,body:components['schemas']['EnvironmentDisplayBuildInput'],signal?:AbortSignal)=>unwrapScoped(apiClient.POST('/api/v1/environment-releases/{id}/display-builds',{params:{path:{id}},body,signal})),
    cancelEnvironmentDisplay:(id:string,signal?:AbortSignal)=>unwrapScoped(apiClient.POST('/api/v1/environment-display-builds/{id}/cancel',{params:{path:{id}},signal})),
    cancelEnvironment:(id:string,signal?:AbortSignal)=>unwrapScoped(apiClient.POST('/api/v1/environment-builds/{id}/cancel',{params:{path:{id}},signal})),
    environmentReleases:(build?:string,after?:string,signal?:AbortSignal,source?:string)=>unwrapScoped(apiClient.GET('/api/v1/environment-releases',{params:{query:{build,after,source}},signal})),
    environmentRelease:(id:string,signal?:AbortSignal)=>unwrapScoped(apiClient.GET('/api/v1/environment-releases/{id}',{params:{path:{id}},signal})),
    reviewMunicipalWorkload:(
      id: string, query: {manifestSha256:string; sourceId:string; admissionId:string}, file: File, signal?: AbortSignal,
    ) => unwrapScoped(apiClient.POST('/api/v1/environment-releases/{id}/municipal-workload', {
      params: {path: {id}, query},
      body: '',
      bodySerializer: () => file,
      headers: {'Content-Type': 'application/octet-stream'},
      signal,
    })),
    reviewCivilianAccess:(
      id: string, query: operations['reviewCivilianAccess']['parameters']['query'], file: File, signal?: AbortSignal,
    ) => unwrapScoped(apiClient.POST('/api/v1/environment-releases/{id}/civilian-access-replay', {
      params: {path: {id}, query},
      body: '',
      bodySerializer: () => file,
      headers: {'Content-Type': 'application/octet-stream'},
      signal,
    })),
    reviewCivilianAerial:(
      id: string, query: operations['reviewCivilianAerial']['parameters']['query'], file: File, signal?: AbortSignal,
    ) => unwrapScoped(apiClient.POST('/api/v1/environment-releases/{id}/civilian-aerial-review', {
      params: {path: {id}, query}, body: '', bodySerializer: () => file,
      headers: {'Content-Type': 'application/octet-stream'}, signal,
    })),
    reviewSupportingSources:(
      id: string, query: operations['reviewSupportingSources']['parameters']['query'], file: File, signal?: AbortSignal,
    ) => unwrapScoped(apiClient.POST('/api/v1/environment-releases/{id}/supporting-source-review', {
      params: {path: {id}, query}, body: '', bodySerializer: () => file,
      headers: {'Content-Type': 'application/octet-stream'}, signal,
    })),
    reviewRegionalResponse:(
      id: string, query: operations['reviewRegionalResponse']['parameters']['query'], file: File, signal?: AbortSignal,
    ) => unwrapScoped(apiClient.POST('/api/v1/environment-releases/{id}/regional-response-review', {
      params: {path: {id}, query}, body: '', bodySerializer: () => file,
      headers: {'Content-Type': 'application/octet-stream'}, signal,
    })),
    reviewPortActivity:(
      id: string, query: operations['reviewPortActivity']['parameters']['query'], file: File, signal?: AbortSignal,
    ) => unwrapScoped(apiClient.POST('/api/v1/environment-releases/{id}/port-activity', {
      params: {path: {id}, query},
      body: '',
      bodySerializer: () => file,
      headers: {'Content-Type': 'application/octet-stream'},
      signal,
    })),
    reviewFloodMapping:(
      id: string, query: operations['reviewFloodMapping']['parameters']['query'], file: File, signal?: AbortSignal,
    ) => unwrapScoped(apiClient.POST('/api/v1/environment-releases/{id}/flood-mapping', {
      params: {path: {id}, query},
      body: '',
      bodySerializer: () => file,
      headers: {'Content-Type': 'application/octet-stream'},
      signal,
    })),
    reviewRadarOutages:(
      id: string, query: operations['reviewRadarOutages']['parameters']['query'], file: File, signal?: AbortSignal,
    ) => unwrapScoped(apiClient.POST('/api/v1/environment-releases/{id}/radar-outages', {
      params: {path: {id}, query},
      body: '',
      bodySerializer: () => file,
      headers: {'Content-Type': 'application/octet-stream'},
      signal,
    })),
    environmentLineage:(id:string,signal?:AbortSignal)=>unwrapScoped(apiClient.GET('/api/v1/environment-releases/{id}/lineage',{params:{path:{id}},signal})),
    environmentSourceImpact:(id:string,after?:string,signal?:AbortSignal)=>unwrapScoped(apiClient.GET('/api/v1/environment-sources/{id}/impact',{params:{path:{id},query:{after}},signal})),
    environmentQualityReport:(id:string,signal?:AbortSignal)=>unwrapScoped(apiClient.GET('/api/v1/environment-releases/{id}/quality-report',{params:{path:{id}},signal})),
    environmentStorageBacklog:(after?:string,signal?:AbortSignal)=>unwrapScoped(apiClient.GET('/api/v1/environment-operations/storage',{params:{query:{after}},signal})),
    environmentDecisions:(id:string,signal?:AbortSignal)=>unwrapScoped(apiClient.GET('/api/v1/environment-releases/{id}/decisions',{params:{path:{id}},signal})),
    environmentDisplays:(id:string,after?:string,signal?:AbortSignal)=>unwrapScoped(apiClient.GET('/api/v1/environment-releases/{id}/displays',{params:{path:{id},query:{after}},signal})),
    environmentDisplay:(id:string,signal?:AbortSignal)=>unwrapScoped(apiClient.GET('/api/v1/environment-displays/{id}/manifest',{params:{path:{id}},signal})),
    environmentAssessments:(id:string,after?:string,signal?:AbortSignal)=>unwrapScoped(apiClient.GET('/api/v1/environment-releases/{id}/assessments',{params:{path:{id},query:{after}},signal})),
    environmentAssessment:(id:string,signal?:AbortSignal)=>unwrapScoped(apiClient.GET('/api/v1/environment-assessments/{id}',{params:{path:{id}},signal})),
    recordEnvironmentAssessment:(id:string,body:components['schemas']['EnvironmentAssessmentInput'],signal?:AbortSignal)=>unwrapScoped(apiClient.POST('/api/v1/environment-releases/{id}/assessments',{params:{path:{id}},body,signal})),
    supersedeEnvironmentAssessment:(id:string,body:components['schemas']['EnvironmentAssessmentCorrection'],signal?:AbortSignal)=>unwrapScoped(apiClient.POST('/api/v1/environment-assessments/{id}/supersede',{params:{path:{id}},body,signal})),
    environmentSuitability:(id:string,body:components['schemas']['EnvironmentSuitabilityRequest'],signal?:AbortSignal)=>unwrapScoped(apiClient.POST('/api/v1/environment-releases/{id}/suitability',{params:{path:{id}},body,signal})),
    freezeGeographicContext:(body:operations['freezeGeographicContext']['requestBody']['content']['application/json'],signal?:AbortSignal)=>
      unwrapScoped(apiClient.POST('/api/v1/context-packages/geographic',{body,signal})),
    geographicContext:(id:string,signal?:AbortSignal)=>unwrapScoped(apiClient.GET('/api/v1/context-packages/{id}/geographic',{params:{path:{id}},signal})),
    environmentDisplayContent:async(id:string,signal?:AbortSignal)=>{
      const result=await apiClient.GET('/api/v1/environment-displays/{id}/content',{params:{path:{id}},parseAs:'stream',signal})
      const body=await unwrapScoped(Promise.resolve(result))
      if(!body)throw new Error('The display stream is unavailable.')
      return {body,etag:result.response.headers.get('ETag')}
    },
    reviewEnvironment:(id:string,action:'verify'|'release'|'withdraw',body:components['schemas']['EnvironmentReview'],signal?:AbortSignal)=> {
      const params={path:{id}}
      if(action==='verify')return unwrapScoped(apiClient.POST('/api/v1/environment-releases/{id}/verify',{params,body,signal}))
      if(action==='release')return unwrapScoped(apiClient.POST('/api/v1/environment-releases/{id}/release',{params,body,signal}))
      return unwrapScoped(apiClient.POST('/api/v1/environment-releases/{id}/withdraw',{params,body,signal}))
    },
    downloadEnvironment:async(id:string,rebuild:boolean,signal?:AbortSignal)=>{
      const path=rebuild?'/api/v1/environment-releases/{id}/rebuild':'/api/v1/environment-releases/{id}/prepared'
      const result=await apiClient.GET(path,{params:{path:{id}},parseAs:'stream',signal})
      const body=await unwrapScoped(Promise.resolve(result))
      if(!body)throw new Error('The package stream is unavailable.')
      return {body,length:Number(result.response.headers.get('Content-Length'))}
    },
    resolveAnalystScope:(body:ScopeInput,requestSignal?:AbortSignal)=>unwrapScoped(apiClient.POST('/api/v1/analyst-scope/resolve',{body,signal:requestSignal})),
    freezeAnalystScope:(body:ScopeInput,requestSignal?:AbortSignal)=>unwrapScoped(apiClient.POST('/api/v1/analyst-scope/freeze',{body,signal:requestSignal})),
    queryCoverage:(body:{scope:ScopeInput;sourceId?:string},requestSignal?:AbortSignal)=>unwrap(apiClient.POST('/api/v1/coverage/query',{body,signal:requestSignal})),
    createAcquisition:(body:AcquisitionInput,requestSignal?:AbortSignal)=>unwrap(apiClient.POST('/api/v1/acquisitions',{body,signal:requestSignal})),
    listAcquisitions:(source_id?:string,requestSignal?:AbortSignal)=>unwrap(apiClient.GET('/api/v1/acquisitions',{params:{query:{source_id}},signal:requestSignal})),
    getAcquisition:(id:string,requestSignal?:AbortSignal)=>unwrap(apiClient.GET('/api/v1/acquisitions/{id}',{params:{path:{id}},signal:requestSignal})),
    cancelAcquisition:(id:string)=>unwrap(apiClient.POST('/api/v1/acquisitions/{id}/cancel',{params:{path:{id}}})),
    listInvestigationHistory:(id:string,requestSignal?:AbortSignal)=>unwrap(apiClient.GET('/api/v1/investigations/{id}/history',{params:{path:{id}},signal:requestSignal})),
    listInvestigationReviews:(id:string,requestSignal?:AbortSignal)=>unwrap(apiClient.GET('/api/v1/investigations/{id}/reviews',{params:{path:{id}},signal:requestSignal})),
    reviewInvestigation:(id:string,body:components['schemas']['CaseReviewInput'])=>unwrap(apiClient.POST('/api/v1/investigations/{id}/review',{params:{path:{id}},body})),
    caseReviewInbox:(requestSignal?:AbortSignal)=>unwrap(apiClient.GET('/api/v1/investigations/review-inbox',{signal:requestSignal})),
    submitDelivery:(id:string,body:components['schemas']['DeliveryInput'])=>unwrap(apiClient.POST('/api/v1/investigations/{id}/delivery-jobs',{params:{path:{id}},body})),
    listDeliveries:(id:string,requestSignal?:AbortSignal)=>unwrap(apiClient.GET('/api/v1/investigations/{id}/delivery-jobs',{params:{path:{id}},signal:requestSignal})),
    changeDelivery:(id:string,action:'cancel'|'retry',expectedAttempt:number)=>action==='cancel'?unwrap(apiClient.POST('/api/v1/delivery-jobs/{id}/cancel',{params:{path:{id}},body:{expectedAttempt}})):unwrap(apiClient.POST('/api/v1/delivery-jobs/{id}/retry',{params:{path:{id}},body:{expectedAttempt}})),
    downloadBriefBundle:(id:string)=>unwrap(apiClient.GET('/api/v1/outputs/{id}/bundle',{params:{path:{id}},parseAs:'blob'})),
    outputAttestation:(id:string)=>unwrap(apiClient.GET('/api/v1/outputs/{id}/attestation',{params:{path:{id}}})),
    createCompositeOutput:(id:string,body:components['schemas']['CompositeOutputInput'])=>unwrap(apiClient.POST('/api/v1/investigations/{id}/outputs',{params:{path:{id}},body})),
    getCompositeOutput:(id:string,requestSignal?:AbortSignal)=>unwrap(apiClient.GET('/api/v1/outputs/{id}',{params:{path:{id}},signal:requestSignal})),
    stageCompositeOutput:(id:string,bytes:ArrayBuffer,media:string)=>unwrap(apiClient.PUT('/api/v1/outputs/{id}/content',{params:{path:{id}},body:'',bodySerializer:()=>bytes,headers:{'Content-Type':media}})),
    downloadCompositeOutput:(id:string,distribution=false)=>unwrap(apiClient.GET('/api/v1/outputs/{id}/content',{params:{path:{id},query:{distribution}},parseAs:'blob'})),
    expireCompositeOutput:(id:string)=>unwrap(apiClient.POST('/api/v1/outputs/{id}/expire',{params:{path:{id}}})),
    async readCatalog(body:components['schemas']['CatalogInput'],requestSignal?:AbortSignal,scope?:ScopeSnapshot) {
      const page = await unwrapScoped(apiClient.POST('/api/v1/catalog/reads', {body, signal:requestSignal}), scope)
      if (body.replaceSnapshotId) catalogReads.delete(body.replaceSnapshotId)
      if (page.snapshotId) {
        catalogReads.add(page.snapshotId)
        if (signal?.aborted || requestSignal?.aborted && !body.cursor) void releaseCatalog(page.snapshotId).catch(() => {})
      }
      return page
    },
    releaseCatalog,
    readIndividualRecords:(id:string,family:string,aggregate?:string,cursor?:string,requestSignal?:AbortSignal,scope?:ScopeSnapshot)=>unwrapScoped(apiClient.GET('/api/v1/analyst-scope/reads/{id}/records',{params:{path:{id},query:{family,aggregate_id:aggregate,cursor,limit:100}},signal:requestSignal}),scope),
    captureProposal:(body:components['schemas']['CaptureProposalInput'],requestSignal?:AbortSignal)=>unwrap(apiClient.POST('/api/v1/imagery/capture-proposal',{body,signal:requestSignal})),
    checkImageryPair:(body:components['schemas']['PairInput'],requestSignal?:AbortSignal)=>unwrap(apiClient.POST('/api/v1/imagery/comparison-check',{body,signal:requestSignal})),
    async readAnalystScope(body:ScopeReadInput,requestSignal?:AbortSignal) {
      const read = await unwrapScoped(apiClient.POST('/api/v1/analyst-scope/read', {body, signal:requestSignal}))
      if (read.id) {
        analystReads.add(read.id)
        if (signal?.aborted || requestSignal?.aborted) void releaseAnalystRead(read.id).catch(() => {})
      }
      return read
    },
    getAnalystReadPage:(id:string,family:string,offset=0,requestSignal?:AbortSignal)=>unwrapScoped(apiClient.GET('/api/v1/analyst-scope/reads/{id}',{params:{path:{id},query:{family,offset,limit:100}},signal:requestSignal})),
    releaseAnalystRead,
    releaseRetainedReads() {
      for (const id of analystReads) void releaseAnalystRead(id).catch(() => {})
      for (const id of catalogReads) void releaseCatalog(id).catch(() => {})
    },
    createSurroundingArea: (body:{longitude:number;latitude:number;radiusM:number},requestSignal?:AbortSignal) => unwrap(apiClient.POST('/api/v1/map/surrounding-area',{body,signal:requestSignal})),
    measureMapGeometry: (geometry:Record<string,unknown>,requestSignal?:AbortSignal) => unwrap(apiClient.POST('/api/v1/map/measure',{body:{geometry},signal:requestSignal})),
    listGeographicRegions: (query?: {q?:string;parent?:string;kind?:'country'|'region'|'governorate'|'place'|'study';id?:string;offset?:number;source_id?:string;release_id?:string;edition_id?:string},requestSignal?:AbortSignal) => unwrap(apiClient.GET('/api/v1/geography/regions',{params:{query},signal:requestSignal})),
    getMapFeatures: (query: {geometry?:string;family:'events'|'earthquakes'|'thermal'|'weather'|'bulletins'|'aircraft'|'vessels'|'imagery'|'coverage'|'findings';region_id?:string;bbox?:string;view?:'latest'|'known_at'|'effective_at';as_of?:string;observed_from?:string;observed_to?:string;source_id?:string;quality?:string;zoom?:number},signal?:AbortSignal) => unwrap(apiClient.GET('/api/v1/map/features',{params:{query},signal})),
    getReferenceTile: (z:number,x:number,y:number,datasets:string[],signal?:AbortSignal) => unwrap(apiClient.GET('/api/v1/map/reference/{z}/{x}/{y}.pbf',{params:{path:{z,x,y},query:{datasets:datasets.join(',')}},signal,parseAs:'arrayBuffer'})),
    getSourceActivation: (id: string, signal?: AbortSignal) => unwrap(apiClient.GET('/api/v1/sources/{id}/activation', {params:{path:{id}},signal})),
    getReferenceViewport: (bbox: string, datasets: string[], offset = 0,requestSignal?:AbortSignal) => unwrap(apiClient.GET('/api/v1/reference-features', {params:{query:{bbox,datasets:datasets.join(','),offset}},signal:requestSignal})),
    listRuntimeCapabilities: () => unwrap(apiClient.GET('/api/v1/simulation-runtime/capabilities')),
    listContentPurpose: (view:'all'|'unclassified'='unclassified',offset=0) => unwrap(apiClient.GET('/api/v1/content-purpose',{params:{query:{view,offset}}})),
    classifyContentPurpose: (body:ContentPurposeInput) => unwrap(apiClient.POST('/api/v1/content-purpose',{body})),
    listImageryCaptures: (region_id?:string,geometry?:string,scope?:ScopeSnapshot,requestSignal?:AbortSignal) => unwrapScoped(apiClient.GET('/api/v1/imagery/captures',{params:{query:scope?scopeQuery(scope):{region_id,geometry}},signal:requestSignal}),scope),
    createImageryCapture: (body: components['schemas']['ImageryCaptureInput']) => unwrap(apiClient.POST('/api/v1/imagery/captures', {body})),
    getImageryCapture:(id:string,requestSignal?:AbortSignal)=>unwrap(apiClient.GET('/api/v1/imagery/captures/{id}',{params:{path:{id}},signal:requestSignal})),
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
    listContextRecords: (kind?: 'WEATHER' | 'REPORT', bbox?: string,region_id?:string,geometry?:string,scope?:ScopeSnapshot,requestSignal?:AbortSignal) => unwrapScoped(apiClient.GET('/api/v1/public-context/records', { params: { query: scope?{kind,...scopeQuery(scope)}:{ kind, bbox, region_id, geometry } },signal:requestSignal }),scope),
    getContextRecord: (id: string,requestSignal?:AbortSignal) => unwrap(apiClient.GET('/api/v1/public-context/records/{id}', { params: { path: { id } },signal:requestSignal })),
    listContextHistory: (id: string,requestSignal?:AbortSignal) => unwrap(apiClient.GET('/api/v1/public-context/records/{id}/history', { params: { path: { id } },signal:requestSignal })),
    listReportAssessments: (scope?:ScopeSnapshot,requestSignal?:AbortSignal) => unwrapScoped(apiClient.GET('/api/v1/assessments/reports',{params:{query:scope?scopeQuery(scope):undefined},signal:requestSignal}),scope),
    proposeReportAssessment: (body: components['schemas']['ReportProposal']) => unwrap(apiClient.POST('/api/v1/assessments/report-proposals', { body })),
    reviseReportAssessment: (id: string, body: components['schemas']['ReportProposal']) => unwrap(apiClient.POST('/api/v1/assessments/{id}/revisions', { params: { path: { id } }, body })),
    listExternalTracks: (bbox?: string,region_id?:string,geometry?:string,scope?:ScopeSnapshot,requestSignal?:AbortSignal) => unwrapScoped(apiClient.GET('/api/v1/external-tracks', { params: { query: scope?scopeQuery(scope):{ bbox, region_id, geometry } },signal:requestSignal }),scope),
    getExternalTrack: (id: string,requestSignal?:AbortSignal) => unwrap(apiClient.GET('/api/v1/external-tracks/{id}', { params: { path: { id } },signal:requestSignal })),
    getExternalAssociations: (id: string) => unwrap(apiClient.GET('/api/v1/external-tracks/{id}/associations', { params: { path: { id } } })),
    readExternalHistory: (id: string, query: operations['readExternalTrackHistory']['parameters']['query'], signal?: AbortSignal) =>
      unwrap(apiClient.GET('/api/v1/external-tracks/{id}/history', {params: {path: {id}, query}, signal})),
    previewContextPackage: (body: components['schemas']['ContextPackageSelection'],requestSignal?:AbortSignal) => unwrap(apiClient.POST('/api/v1/context-packages/preview', { body,signal:requestSignal })),
    listImageryScenes: (bbox?: string,region_id?:string,geometry?:string,scope?:ScopeSnapshot,requestSignal?:AbortSignal) => unwrapScoped(apiClient.GET('/api/v1/imagery/scenes', { params: { query: scope?scopeQuery(scope):{ bbox, region_id, geometry } },signal:requestSignal }),scope),
    getImageryScene: (id: string) => unwrap(apiClient.GET('/api/v1/imagery/scenes/{id}', { params: { path: { id } } })),
    listImageryJobs: (scope?:ScopeSnapshot,requestSignal?:AbortSignal) => unwrapScoped(apiClient.GET('/api/v1/imagery/jobs',{params:{query:scope?scopeQuery(scope):undefined},signal:requestSignal}),scope),
    getImageryJob: (id: string,requestSignal?:AbortSignal) => unwrap(apiClient.GET('/api/v1/imagery/jobs/{id}', { params: { path: { id } },signal:requestSignal })),
    getImageryResult: (id: string,requestSignal?:AbortSignal) => unwrap(apiClient.GET('/api/v1/imagery/jobs/{id}/result', { params: { path: { id } },signal:requestSignal })),
    submitImageryJob: (body: ImagerySubmit) => unwrap(apiClient.POST('/api/v1/imagery/jobs', { body })),
    cancelImageryJob: (id: string) => unwrap(apiClient.POST('/api/v1/imagery/jobs/{id}/cancel', { params: { path: { id } } })),
    retryImageryJob: (id: string) => unwrap(apiClient.POST('/api/v1/imagery/jobs/{id}/retry', { params: { path: { id } } })),
    listImageryAttempts: (id: string,requestSignal?:AbortSignal) => unwrap(apiClient.GET('/api/v1/imagery/jobs/{id}/attempts', { params: { path: { id } },signal:requestSignal })),
    reserveArtifact: (body: ArtifactInput) => unwrap(apiClient.POST('/api/v1/artifacts', { body })),
    getArtifact: (id: string) => unwrap(apiClient.GET('/api/v1/artifacts/{id}', { params: { path: { id } } })),
    uploadArtifact: (id: string, bytes: ArrayBuffer) => unwrap(apiClient.PUT('/api/v1/artifacts/{id}/content', { params: { path: { id } }, body: '', bodySerializer: () => bytes, headers: { 'Content-Type': 'application/octet-stream' } })),
    downloadArtifact: (id: string,requestSignal?:AbortSignal) => unwrap(apiClient.GET('/api/v1/artifacts/{id}/content', { params: { path: { id } }, parseAs: 'blob',signal:requestSignal })),
    listIntelligenceAssessments: (scope?:ScopeSnapshot,requestSignal?:AbortSignal) => unwrapScoped(apiClient.GET('/api/v1/assessments/intelligence',{params:{query:scope?scopeQuery(scope):undefined},signal:requestSignal}),scope),
    proposeImageryAssessment: (body: ImageryProposal) => unwrap(apiClient.POST('/api/v1/assessments/imagery-proposals', { body })),
    reviseImageryAssessment: (id: string, body: ImageryProposal) => unwrap(apiClient.POST('/api/v1/assessments/{id}/revisions', { params: { path: { id } }, body })),
    listAssessmentRevisions: (id: string,requestSignal?:AbortSignal) => unwrap(apiClient.GET('/api/v1/assessments/{id}/revisions', { params: { path: { id } },signal:requestSignal })),
    reviewImageryAssessment: (id: string, body: ImageryReviewAction) => unwrap(apiClient.POST('/api/v1/assessments/{id}/review', { params: { path: { id } }, body })),
    submitAssessmentDraft: (id: string, body: ImageryReviewAction) => unwrap(apiClient.POST('/api/v1/assessments/{id}/submit', { params: { path: { id } }, body })),
    /* Public evidence and reference snapshots. Instances retain scope and cancellation. */
    listPublicEvents: (params?: PublicEventQuery,requestSignal?:AbortSignal,scope?:ScopeSnapshot) => unwrapScoped(apiClient.GET('/api/v1/public-events', { params: { query: scope?{limit:params?.limit??100,offset:params?.offset??0,source_id:params?.source_id,quality:params?.quality,phenomenon:params?.phenomenon,revisions:params?.revisions,...scopeQuery(scope)}:{ limit: 200, ...params } },signal:requestSignal }),scope),
    getPublicEvent: (id: string, query?: PublicEventQuery,requestSignal?:AbortSignal) => unwrap(apiClient.GET('/api/v1/public-events/{id}', { params: { path: { id }, query },signal:requestSignal })),
    listPublicEventRevisions: (id: string, query?: PublicEventQuery,requestSignal?:AbortSignal) => unwrap(apiClient.GET('/api/v1/public-events/{id}/revisions', { params: { path: { id }, query: { limit: 200, ...query } },signal:requestSignal })),
    listReferenceDatasets: (requestSignal?:AbortSignal) => unwrap(apiClient.GET('/api/v1/reference-datasets',{signal:requestSignal})),
    getReferenceArtifact: (id: string) => unwrap(apiClient.GET('/api/v1/reference-datasets/{id}/artifact', { params: { path: { id } }, parseAs: 'text' })),
    getReferenceFeatures: (id: string) => unwrap(apiClient.GET('/api/v1/reference-datasets/{id}/features', { params: { path: { id } } })),
    getSourceEvidence: (id: string, evidenceID: string,requestSignal?:AbortSignal) => unwrap(apiClient.GET('/api/v1/sources/{id}/evidence/{evidenceID}', { params: { path: { id, evidenceID } },signal:requestSignal })),
    getSourceOriginal: (id: string, evidenceID: string,requestSignal?:AbortSignal) => unwrap(apiClient.GET('/api/v1/sources/{id}/evidence/{evidenceID}/original', { params: { path: { id, evidenceID } }, parseAs: 'text',signal:requestSignal })),
    getSourceAdmissionVersion: (id: string, admissionID: string,requestSignal?:AbortSignal) => unwrap(apiClient.GET('/api/v1/sources/{id}/admissions/{admissionID}', { params: { path: { id, admissionID } },signal:requestSignal })),
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
