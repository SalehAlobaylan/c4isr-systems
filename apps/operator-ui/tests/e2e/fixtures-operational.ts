/**
 * Deterministic operational fixture shared by the end-to-end suites.
 *
 * It lives here rather than inside one spec so the workflow suite and the
 * design-system suite exercise the *same* picture, board, alert and incident
 * data. A visual regression is only meaningful against data that also keeps the
 * operational assertions honest, and two drifting copies of this mock would
 * quietly break that.
 *
 * Nothing here is invented to make a screenshot look good: the states are the
 * ones the product actually produces — a live track, a track whose feed went
 * quiet, an assigned asset, a restricted geofence, a hazard footprint, and a
 * high-severity active alert.
 */

import type { Page, Route } from '@playwright/test'
export const point = { lat: 24.712, lng: 46.676 }
export const now = '2026-09-20T12:00:00Z'

export const track = {
  id: 'trk-demo',
  externalRef: 'patrol-01',
  status: 'active',
  observationCount: 1,
  position: point,
  speed: 8,
  heading: 90,
  lastSeenAt: now,
  firstSeenAt: now,
  updatedAt: now,
  freshness: 'live',
  freshnessLabel: 'Live',
  metadata: {
    sourceId: 'scn_run_demo__source__radar-01',
    scenarioRunId: 'scn_run_demo',
    resourceNamespace: 'scn_run_demo__',
  },
}

/** A track whose feed went quiet: the case the picture must not hide. */
export const staleTrack = {
  ...track,
  id: 'trk-quiet',
  externalRef: 'quiet-01',
  position: { lat: 24.7, lng: 46.69 },
  speed: 0,
  lastSeenAt: '2026-09-20T11:20:00Z',
  freshness: 'stale',
  freshnessLabel: 'Stale',
}

export const asset = {
  id: 'scn_run_demo__asset__patrol-01',
  name: 'Patrol 01',
  type: 'uav',
  status: 'available',
  capabilities: ['observe', 'move_to'],
  metadata: { scenarioRunId: 'scn_run_demo' },
  position: { lat: 24.713, lng: 46.677 },
  speed: 0,
  heading: 0,
  health: 'nominal',
  connectionState: 'connected',
  lastSeenAt: now,
  freshness: 'live',
  freshnessLabel: 'Live',
  createdAt: now,
  updatedAt: now,
}

export const geofence = {
  id: 'scn_run_demo__geofence__restricted-zone',
  name: 'Restricted Zone A',
  type: 'restricted',
  severity: 'high',
  active: true,
  geojson: JSON.stringify({
    type: 'Polygon',
    coordinates: [[[46.67, 24.70], [46.69, 24.70], [46.69, 24.72], [46.67, 24.72], [46.67, 24.70]]],
  }),
  metadata: { scenarioRunId: 'scn_run_demo' },
  createdAt: now,
  updatedAt: now,
}

export const observation = {
  id: 'scn_run_demo__observation__obs-01',
  sourceId: 'scn_run_demo__source__radar-01',
  type: 'radar',
  observedAt: now,
  receivedAt: now,
  position: point,
  payload: { logicalId: 'obs-01' },
  quality: { confidence: 0.91 },
  trackHint: 'scn_run_demo__track__patrol-01',
  createdAt: now,
  processedAt: now,
}

// An area observation: a wildfire footprint that straddles a restricted zone.
// It has no position and no track hint, because it is never correlated.
export const areaObservation = {
  id: 'scn_run_demo__observation__obs-fire-01',
  sourceId: 'scn_run_demo__source__fire-monitor',
  type: 'hazard.wildfire',
  observedAt: now,
  receivedAt: now,
  area: {
    kind: 'hazard',
    geojson:
      '{"type":"Polygon","coordinates":[[[46.664,24.718],[46.664,24.730],[46.682,24.730],[46.682,24.718],[46.664,24.718]]]}',
    center: { lat: 24.724, lng: 46.673 },
    areaM2: 3_800_000,
  },
  payload: { detection: 'VIIRS' },
  createdAt: now,
}

export const baseAlert = {
  id: 'alert-1',
  type: 'geofence.breach',
  severity: 'high',
  state: 'ACTIVE',
  title: 'Track patrol-01 entered Restricted Zone A',
  message: 'Synthetic track entered a restricted geofence.',
  sourceReference: {
    rule: 'geofence.breach',
    scenarioRunId: 'scn_run_demo',
    resourceNamespace: 'scn_run_demo__',
    geofenceId: geofence.id,
    geofenceName: geofence.name,
  },
  trackId: track.id,
  assetId: asset.id,
  geofenceId: geofence.id,
  incidentId: '',
  createdAt: now,
  updatedAt: now,
  acknowledgedAt: null,
  acknowledgedBy: '',
  resolvedAt: null,
  resolvedBy: '',
}

export function list<T>(items: T[]) {
  return { items, total: items.length }
}

export async function json(route: Route, body: unknown, status = 200): Promise<void> {
  await route.fulfill({
    status,
    contentType: 'application/json',
    body: JSON.stringify(body),
  })
}

export async function mockOperationalWorkflow(page: Page): Promise<void> {
  let alert = { ...baseAlert }
  let incident: Record<string, any> | null = null
  let mission: Record<string, any> | null = null
  let command: Record<string, any> | null = null
  let assessments: Array<Record<string, any>> = [
    {
      id: 'assessment-1',
      subjectType: 'track',
      subjectId: track.id,
      type: 'identification',
      conclusion: 'Likely patrol vehicle',
      method: 'OPERATOR',
      confidence: 0.8,
      createdBy: 'operator-01',
      createdAt: now,
      evidence: [],
    },
  ]

  await page.route('**/health', (route) =>
    json(route, { status: 'ok', database: 'up', time: now }),
  )
  const windowStart = new Date('2026-09-20T11:55:00Z')
  const windowEnd = new Date('2026-09-20T12:05:00Z')

  await page.route('**/api/v1/**', async (route) => {
    const request = route.request()
    const method = request.method()
    const path = new URL(request.url()).pathname
    const query = new URL(request.url()).searchParams

    if (path === '/api/v1/picture/window') {
      await json(route, { earliest: windowStart.toISOString(), latest: windowEnd.toISOString() })
      return
    }
    if (path === '/api/v1/picture') {
      // The reconstructed picture is served from recorded evidence: entities
      // that had been observed by the requested instant, and no later.
      const at = query.get('at') ? new Date(query.get('at')!) : windowEnd
      const cutoff = at.getTime()
      const isLive = query.get('live') === 'true' || !query.get('at')
      const observedAt = (iso: string) => new Date(iso).getTime() <= cutoff
      const freshAt = (iso: string) => {
        const ageSeconds = (cutoff - new Date(iso).getTime()) / 1000
        if (ageSeconds < 60) return { freshness: 'live', freshnessLabel: 'Live' }
        if (ageSeconds < 300) return { freshness: 'delayed', freshnessLabel: 'Delayed' }
        return { freshness: 'stale', freshnessLabel: 'Stale' }
      }

      await json(route, {
        at: at.toISOString(),
        live: isLive,
        tracks: [track, staleTrack]
          .filter((t) => observedAt(t.lastSeenAt))
          .map((t) => ({
            id: t.id,
            externalRef: t.externalRef,
            status: t.status,
            position: t.position,
            speed: t.speed,
            heading: t.heading,
            observedAt: t.lastSeenAt,
            firstSeenAt: t.firstSeenAt,
            lastSeenAt: t.lastSeenAt,
            ...freshAt(t.lastSeenAt),
          })),
        assets: [asset].filter((a) => observedAt(a.lastSeenAt)).map((a) => ({
          id: a.id,
          name: a.name,
          type: a.type,
          status: a.status,
          position: a.position,
          speed: a.speed,
          heading: a.heading,
          observedAt: a.lastSeenAt,
          connectionState: a.connectionState,
          health: a.health,
          ...freshAt(a.lastSeenAt),
        })),
        // An alert only exists in a replay once it has actually been raised.
        alerts: new Date(baseAlert.createdAt).getTime() <= cutoff ? [baseAlert] : [],
        geofences: [geofence],
        window: { earliest: windowStart.toISOString(), latest: windowEnd.toISOString() },
        truncatedAlerts: false,
      })
      return
    }

    if (path === '/api/v1/auth/me') {
      await json(route, {
        operator: { id: 'operator-01', name: 'Operator 01', role: 'operator' },
        permissions: ['read', 'write'],
      })
      return
    }
    if (path === '/api/v1/scenarios/runs' && method === 'GET') {
      await json(
        route,
        list([
          {
            id: 'scn_run_demo',
            scenarioName: 'restricted-area-intrusion',
            status: 'RUNNING',
            seed: 1007,
            playbackSpeed: 1,
            virtualTimeMs: 300_000,
          },
        ]),
      )
      return
    }
    if (path === '/api/v1/scenarios' && method === 'GET') {
      await json(route, list([{ name: 'restricted-area-intrusion', description: 'Synthetic replay' }]))
      return
    }
    if (path === '/api/v1/geofences' && method === 'GET') {
      await json(route, list([geofence]))
      return
    }
    if (path === '/api/v1/tracks' && method === 'GET') {
      await json(route, list([track, staleTrack]))
      return
    }
    if (path === `/api/v1/tracks/${track.id}` && method === 'GET') {
      await json(route, track)
      return
    }
    if (path === `/api/v1/tracks/${track.id}/history` && method === 'GET') {
      await json(route, list([
        { id: 'history-1', observedAt: '2026-09-20T11:59:00Z', position: { lat: 24.710, lng: 46.674 }, speed: 7, heading: 88 },
        { id: 'history-2', observedAt: now, position: point, speed: 8, heading: 90 },
      ]))
      return
    }
    if (path === `/api/v1/tracks/${track.id}/classifications` && method === 'GET') {
      await json(route, list([{ id: 'classification-1', label: 'vehicle', confidence: 0.91, method: 'radar', sourceReference: 'radar-01', createdBy: 'system', createdAt: now }]))
      return
    }
    if (path === `/api/v1/tracks/${staleTrack.id}` && method === 'GET') {
      await json(route, staleTrack)
      return
    }
    // The stale track's sub-resources must be served too. Selecting it fetches
    // its classifications, observations, alerts and assessments; leaving them
    // unhandled would fall through to the catch-all below and answer with an
    // object that is not a list, which is not a shape the API can return.
    if (path === `/api/v1/tracks/${staleTrack.id}/classifications` && method === 'GET') {
      await json(route, list([]))
      return
    }
    if (path === `/api/v1/tracks/${staleTrack.id}/history` && method === 'GET') {
      await json(route, list([]))
      return
    }
    if (path === '/api/v1/assets' && method === 'GET') {
      await json(route, list([asset]))
      return
    }
    if (path === `/api/v1/assets/${asset.id}` && method === 'GET') {
      await json(route, asset)
      return
    }
    if (path === `/api/v1/assets/${asset.id}/telemetry` && method === 'GET') {
      await json(route, list([{ id: 'telemetry-1', assetId: asset.id, sourceId: 'radar-01', observedAt: now, receivedAt: now, position: asset.position, health: 'nominal', connectionState: 'connected' }]))
      return
    }
    if (path === `/api/v1/observations` && method === 'GET') {
      // The map asks for areas with a dedicated filter rather than paging the
      // general list, so the mock has to honour it or the area layer would be
      // fed point observations.
      const url = new URL(route.request().url())
      if (url.searchParams.get('area') === 'true') {
        await json(route, list([areaObservation]))
        return
      }
      await json(route, list([observation]))
      return
    }
    if (path === `/api/v1/alerts` && method === 'GET') {
      // Filtering is the server's job, so the mock honours it. A mock that
      // ignored `severity` or `state` would make every filtered view look
      // populated and hide the empty state entirely.
      const severity = query.get('severity')
      const state = query.get('state')
      const matches =
        (!severity || alert.severity === severity) && (!state || alert.state === state)
      await json(route, matches ? list([alert]) : list([]))
      return
    }
    if (path === `/api/v1/alerts/${alert.id}/acknowledge` && method === 'POST') {
      alert = { ...alert, state: 'ACKNOWLEDGED', acknowledgedAt: now, acknowledgedBy: 'operator-01', updatedAt: now }
      await json(route, alert)
      return
    }
    if (path === `/api/v1/alerts/${alert.id}/resolve` && method === 'POST') {
      alert = { ...alert, state: 'RESOLVED', resolvedAt: now, resolvedBy: 'operator-01', updatedAt: now }
      await json(route, alert)
      return
    }
    if (path === '/api/v1/assessments' && method === 'GET') {
      await json(route, list(assessments))
      return
    }
    if (path === '/api/v1/assessments' && method === 'POST') {
      const body = request.postDataJSON() as {
        subjectType: string
        subjectId: string
        type: string
        conclusion: string
        method?: string
        confidence?: number
        evidence?: Array<{ type: string; id: string }>
      }
      const created = {
        id: `assessment-${assessments.length + 1}`,
        subjectType: body.subjectType,
        subjectId: body.subjectId,
        type: body.type,
        conclusion: body.conclusion,
        method: body.method ?? 'OPERATOR',
        confidence: body.confidence ?? null,
        createdBy: 'operator-01',
        createdAt: now,
        evidence: body.evidence ?? [],
      }
      assessments = [...assessments, created]
      await json(route, created, 201)
      return
    }
    if (path === '/api/v1/incidents' && method === 'GET') {
      await json(route, list(incident ? [incident] : []))
      return
    }
    if (path === '/api/v1/incidents' && method === 'POST') {
      const body = request.postDataJSON() as { title: string; description?: string; priority?: string }
      incident = {
        id: 'incident-1',
        title: body.title,
        description: body.description ?? '',
        priority: body.priority ?? 'medium',
        status: 'OPEN',
        assignedOperator: 'operator-01',
        createdAt: now,
        updatedAt: now,
        resolvedAt: null,
        closedAt: null,
      }
      alert = { ...alert, incidentId: incident.id }
      await json(route, incident, 201)
      return
    }
    if (path === '/api/v1/incidents/incident-1' && method === 'GET') {
      await json(route, {
        ...incident,
        id: 'incident-1',
        title: incident?.title ?? 'Unknown vehicle in restricted zone',
        alerts: [{ id: alert.id, type: alert.type, severity: alert.severity, state: alert.state, title: alert.title, createdAt: alert.createdAt }],
        tracks: [{ id: track.id, externalRef: track.externalRef, status: track.status, position: track.position, lastSeenAt: track.lastSeenAt }],
        assets: [{ id: asset.id, name: asset.name, type: asset.type, status: asset.status, position: asset.position, connectionState: asset.connectionState }],
        observations: [{ id: observation.id, sourceId: observation.sourceId, type: observation.type, observedAt: observation.observedAt, position: observation.position }],
        assessments: [],
      })
      return
    }
    if (path === '/api/v1/audit' && method === 'GET') {
      await json(route, list([]))
      return
    }
    if (path === '/api/v1/missions' && method === 'GET') {
      await json(route, list(mission ? [mission] : []))
      return
    }
    if (path === '/api/v1/missions' && method === 'POST') {
      const body = request.postDataJSON() as { name: string; objective?: string; priority?: string; incidentId?: string }
      mission = {
        id: 'mission-1',
        name: body.name,
        objective: body.objective ?? '',
        priority: body.priority ?? 'high',
        status: 'PLANNED',
        incidentId: body.incidentId,
        createdAt: now,
        updatedAt: now,
        assets: [{ id: asset.id, name: asset.name, type: asset.type, status: asset.status, position: asset.position, connectionState: asset.connectionState }],
        tasks: [],
      }
      await json(route, mission, 201)
      return
    }
    if (path === '/api/v1/commands' && method === 'GET') {
      await json(route, list(command ? [command] : []))
      return
    }
    if (path === '/api/v1/commands' && method === 'POST') {
      const body = request.postDataJSON() as { assetId: string; missionId?: string; incidentId?: string; type: string; payload?: Record<string, unknown> }
      command = {
        id: 'command-1',
        assetId: body.assetId,
        missionId: body.missionId,
        incidentId: body.incidentId,
        type: body.type,
        payload: body.payload ?? {},
        state: 'CREATED',
        createdBy: 'operator-01',
        correlationId: 'incident-1',
        createdAt: now,
        updatedAt: now,
      }
      await json(route, command, 201)
      return
    }
    if (path === '/api/v1/commands/command-1/transition' && method === 'POST') {
      const body = request.postDataJSON() as { state: string }
      command = { ...command, state: body.state, updatedAt: now }
      await json(route, command)
      return
    }

    // An unhandled route answers 501 rather than an empty object. The API always
// returns a list for a collection, so an empty object is a shape the client
// cannot receive: it surfaces much later as an unrelated-looking crash deep in
// a component. Failing here names the route instead.
await json(route, { error: { code: 'not_mocked', message: `no mock for ${method} ${path}` } }, 501)
  })
}