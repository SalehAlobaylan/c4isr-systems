import { lazy } from 'react'
const ExercisePage=lazy(()=>import('@/routes/exercise').then(m=>({default:m.ExercisePage})))
const RecordingsPage=lazy(()=>import('@/routes/recordings').then(m=>({default:m.RecordingsPage})))
const SimulationStudioPage=lazy(()=>import('@/routes/simulation-studio').then(m=>({default:m.SimulationStudioPage})))
import { createRootRoute, createRoute, createRouter } from '@tanstack/react-router'
import { HomePage } from '@/routes/home'

import { AppShell } from '@/components/layout/AppShell'
import { AlertsPage } from '@/routes/alerts'
import { AssetsPage } from '@/routes/assets'
import { BoardPage } from '@/routes/board'
import { IncidentDetailPage } from '@/routes/incident-detail'
import { IncidentsPage } from '@/routes/incidents'
const MapPage=lazy(()=>import('@/routes/map').then(m=>({default:m.MapPage})))
import { ScenariosPage } from '@/routes/scenarios'
import { TimelinePage } from '@/routes/timeline'
import { TrackInspectorPage } from '@/routes/track-inspector'
import { TracksPage } from '@/routes/tracks'
const SimulationAssetsPage=lazy(()=>import('@/routes/simulation-assets').then(m=>({default:m.SimulationAssetsPage})))
const WeatherReportsPage=lazy(()=>import('@/routes/weather-reports').then(m=>({default:m.WeatherReportsPage})))
const ExternalTracksPage=lazy(()=>import('@/routes/external-tracks').then(m=>({default:m.ExternalTracksPage})))
const IntelligencePage=lazy(()=>import('@/routes/intelligence').then(m=>({default:m.IntelligencePage})))
const SystemsPage=lazy(()=>import('@/routes/systems').then(m=>({default:m.SystemsPage})))

const rootRoute = createRootRoute({ component: AppShell })

const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/',
  component: HomePage,
})

const boardRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/board',
  component: BoardPage,
})

// TanStack decodes JSON-looking query values before validation. Accept either
// its object form or a deliberately preserved JSON string from an app link.
function mapJSONSearch(value:unknown,limit:number):string|undefined {
 if(value===undefined)return
 const raw=typeof value==='string'?value:value&&typeof value==='object'?JSON.stringify(value):'invalid'
 return raw.length<=limit?raw:'invalid'
}

const mapRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/map',
  validateSearch: (search: Record<string, unknown>): { investigation?:string;scope?:string;selected?: string; publicView?: string; region?:string; tool?:string; workbench?:string; area?:string; mapState?:string } => ({
    scope:mapJSONSearch(search.scope,65536),
    mapState:mapJSONSearch(search.mapState,4096),
    area:mapJSONSearch(search.area,8192),
    region:search.region===undefined?undefined:typeof search.region==='string'&&search.region.length<=256?search.region:'invalid',
    tool:typeof search.tool==='string'&&['systems','investigations','coverage','geography','imagery','review','context','weather','tracks','sources','jobs','simulation','environments'].includes(search.tool)?search.tool:undefined,
    investigation:typeof search.investigation==='string'&&search.investigation.length<=160?search.investigation:undefined,
    workbench:mapJSONSearch(search.workbench,2048),
    publicView: search.publicView === undefined ? undefined : (JSON.stringify(search.publicView).length > 4096 ? 'invalid' : typeof search.publicView === 'string' ? search.publicView : JSON.stringify(search.publicView)),
    selected:
      typeof search.selected === 'string' && search.selected.length > 0 && search.selected.length <= 512
        ? search.selected
        : undefined,
  }),
  component: MapPage,
})

const tracksRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/tracks',
  validateSearch: (search: Record<string, unknown>): { status?: string } => ({
    status: typeof search.status === 'string' && search.status ? search.status : undefined,
  }),
  component: TracksPage,
})

// The full evidence and provenance view for a single track. The map's detail
// rail is the same question asked in passing while working; this is the route
// an operator opens when the answer itself is the task.
const trackInspectorRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/tracks/$id',
  component: TrackInspectorPage,
})

// Cross-system health and command state in one view.
const systemsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/systems',
  validateSearch:(s:Record<string,unknown>):{scope?:string}=>({scope:mapJSONSearch(s.scope,65536)}),
  component: SystemsPage,
})

const assetsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/assets',
  validateSearch: (search: Record<string, unknown>): { status?: string } => ({
    status: typeof search.status === 'string' && search.status ? search.status : undefined,
  }),
  component: AssetsPage,
})

const alertsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/alerts',
  validateSearch: (
    search: Record<string, unknown>,
  ): { state?: string; severity?: string } => ({
    state: typeof search.state === 'string' && search.state ? search.state : undefined,
    severity:
      typeof search.severity === 'string' && search.severity ? search.severity : undefined,
  }),
  component: AlertsPage,
})

const incidentsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/incidents',
  validateSearch: (
    search: Record<string, unknown>,
  ): { status?: string; new?: boolean; alertId?: string } => ({
    status: typeof search.status === 'string' && search.status ? search.status : undefined,
    new:
      search.new === true ||
      search.new === 'true' ||
      search.new === 1 ||
      search.new === '1'
        ? true
        : undefined,
    alertId: typeof search.alertId === 'string' && search.alertId ? search.alertId : undefined,
  }),
  component: IncidentsPage,
})

const incidentDetailRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/incidents/$id',
  component: IncidentDetailPage,
})

const timelineRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/timeline',
  validateSearch: (
    search: Record<string, unknown>,
  ): { action?: string; subject_type?: string } => ({
    action: typeof search.action === 'string' && search.action ? search.action : undefined,
    subject_type:
      typeof search.subject_type === 'string' && search.subject_type
        ? search.subject_type
        : undefined,
  }),
  component: TimelinePage,
})

const scenariosRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/scenarios',
  component: ScenariosPage,
})

const intelligenceRoute = createRoute({
  getParentRoute: () => rootRoute, path: '/intelligence', component: IntelligencePage,
  validateSearch: (search: Record<string, unknown>): { scope?:string;job?: string; assessment?: string; bbox?: string; before?: string; after?: string; view?: string } => ({...Object.fromEntries(['job', 'assessment', 'bbox', 'before', 'after', 'view'].filter((key) => typeof search[key] === 'string' && (search[key] as string).length <= 256).map((key) => [key, search[key]])),scope:mapJSONSearch(search.scope,65536)}),
})

const exerciseRoute = createRoute({getParentRoute:()=>rootRoute,path:'/exercise',component:ExercisePage,validateSearch:(s:Record<string,unknown>):{run?:string}=>({run:typeof s.run==='string'&&s.run.length<=160?s.run:undefined})})
const recordingsRoute=createRoute({getParentRoute:()=>rootRoute,path:'/recordings',component:RecordingsPage,validateSearch:(s:Record<string,unknown>)=>({run:typeof s.run==='string'&&s.run.length<=160?s.run:undefined,ms:Number.isSafeInteger(Number(s.ms))&&Number(s.ms)>=0&&Number(s.ms)<=3600000?Number(s.ms):0,sequence:Number.isSafeInteger(Number(s.sequence))&&Number(s.sequence)>=0&&Number(s.sequence)<=100000?Number(s.sequence):0})})
const simulationStudioRoute = createRoute({getParentRoute:()=>rootRoute,path:'/simulation-studio',component:SimulationStudioPage,validateSearch:(s:Record<string,unknown>):{scene?:string;run?:string}=>({scene:typeof s.scene==='string'&&s.scene.length<=160?s.scene:undefined,run:typeof s.run==='string'&&s.run.length<=160?s.run:undefined})})
const simulationAssetsRoute = createRoute({ getParentRoute: () => rootRoute, path: '/simulation-assets', component: SimulationAssetsPage,
 validateSearch: (s: Record<string, unknown>): { model?: string; upload?: string } => ({model: typeof s.model === 'string' && s.model.length <= 160 ? s.model : undefined, upload: typeof s.upload === 'string' && s.upload.length <= 160 ? s.upload : undefined}),
})
const weatherReportsRoute = createRoute({ getParentRoute: () => rootRoute, path: '/weather-reports', component: WeatherReportsPage,
  validateSearch: (s: Record<string, unknown>): { scope?:string;id?: string; bbox?: string; kind?: 'WEATHER' | 'REPORT'; assessment?: string } => ({
    scope:mapJSONSearch(s.scope,65536),
    id: typeof s.id === 'string' && s.id.length <= 160 ? s.id : undefined,
    bbox: s.bbox === undefined ? undefined : typeof s.bbox === 'string' && s.bbox.length <= 200 ? s.bbox : 'invalid',
    kind: s.kind === 'WEATHER' || s.kind === 'REPORT' ? s.kind : undefined,
    assessment: typeof s.assessment === 'string' && s.assessment.length <= 160 ? s.assessment : undefined,
  }),
})
const externalTracksRoute = createRoute({ getParentRoute: () => rootRoute, path: '/external-tracks', component: ExternalTracksPage,
  validateSearch: (search: Record<string, unknown>): { scope?:string;id?: string; bbox?: string } => ({scope:mapJSONSearch(search.scope,65536), id: typeof search.id === 'string' && search.id.length <= 200 ? search.id : undefined, bbox: typeof search.bbox === 'string' && search.bbox.length <= 200 ? search.bbox : undefined }),
})
const routeTree = rootRoute.addChildren([
  externalTracksRoute,
  indexRoute,
  boardRoute,
  mapRoute,
  intelligenceRoute,
  weatherReportsRoute,
  simulationAssetsRoute,
  simulationStudioRoute,
  exerciseRoute,
  recordingsRoute,
  tracksRoute,
  trackInspectorRoute,
  systemsRoute,
  assetsRoute,
  alertsRoute,
  incidentsRoute,
  incidentDetailRoute,
  timelineRoute,
  scenariosRoute,
])

export const router = createRouter({
  routeTree,
  defaultPreload: 'intent',
})

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}
