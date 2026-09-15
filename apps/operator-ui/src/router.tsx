import { createRootRoute, createRoute, createRouter, redirect } from '@tanstack/react-router'

import { AppShell } from '@/components/layout/AppShell'
import { AlertsPage } from '@/routes/alerts'
import { AssetsPage } from '@/routes/assets'
import { IncidentDetailPage } from '@/routes/incident-detail'
import { IncidentsPage } from '@/routes/incidents'
import { MapPage } from '@/routes/map'
import { ScenariosPage } from '@/routes/scenarios'
import { TimelinePage } from '@/routes/timeline'
import { TracksPage } from '@/routes/tracks'

const rootRoute = createRootRoute({ component: AppShell })

const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/',
  beforeLoad: () => {
    throw redirect({ to: '/map' })
  },
})

const mapRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/map',
  validateSearch: (search: Record<string, unknown>): { selected?: string } => ({
    selected:
      typeof search.selected === 'string' && search.selected.length > 0
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

const routeTree = rootRoute.addChildren([
  indexRoute,
  mapRoute,
  tracksRoute,
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
