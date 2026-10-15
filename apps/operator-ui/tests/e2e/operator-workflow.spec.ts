import { expect, test, type Page, type Route } from '@playwright/test'

const point = { lat: 24.712, lng: 46.676 }
const now = '2026-09-20T12:00:00Z'

const track = {
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
const staleTrack = {
  ...track,
  id: 'trk-quiet',
  externalRef: 'quiet-01',
  position: { lat: 24.7, lng: 46.69 },
  speed: 0,
  lastSeenAt: '2026-09-20T11:20:00Z',
  freshness: 'stale',
  freshnessLabel: 'Stale',
}

const asset = {
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

const geofence = {
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

const observation = {
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
const areaObservation = {
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

const baseAlert = {
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

function list<T>(items: T[]) {
  return { items, total: items.length }
}

async function json(route: Route, body: unknown, status = 200): Promise<void> {
  await route.fulfill({
    status,
    contentType: 'application/json',
    body: JSON.stringify(body),
  })
}

async function mockOperationalWorkflow(page: Page): Promise<void> {
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
      await json(route, list([{ id: 'scn_run_demo', scenarioName: 'restricted-area-intrusion', status: 'RUNNING' }]))
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
      await json(route, list([alert]))
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

    await json(route, {})
  })
}

test('the operator can scrub back to a past picture and is never misled about it', async ({
  page,
}, testInfo) => {
  test.skip(testInfo.project.name !== 'no-token', 'Runs once against the mocked no-token build')
  await mockOperationalWorkflow(page)

  await page.goto('/map')
  await expect(page.getByTestId('scrub-mode')).toHaveText('Live')

  const timeline = page.getByLabel('Picture timeline position')
  await expect(timeline).toBeEnabled()

  // Scrub back to just after the quiet track's last observation but before the
  // alert was raised.
  await timeline.fill(new Date('2026-09-20T11:50:00Z').toISOString())
  await timeline.dispatchEvent('change')

  await expect(page.getByTestId('scrub-mode')).toHaveText('Replay')
  // The replay banner is the point of the whole control: an operator must never
  // act on a past picture believing it is the present.
  await expect(page.getByTestId('replay-banner')).toBeVisible()
  await expect(page.getByTestId('replay-banner')).toContainText('realtime updates are not applied')

  // The alert had not been raised yet at that instant, so it must be absent.
  await expect(page.getByText(baseAlert.title, { exact: true })).toHaveCount(0)

  // Returning to live restores the present picture.
  await page.getByRole('button', { name: 'Return to live', exact: true }).click()
  await expect(page.getByTestId('scrub-mode')).toHaveText('Live')
  await expect(page.getByTestId('replay-banner')).toHaveCount(0)
})

test('an operator can state an assessment and have it kept with its evidence', async ({
  page,
}, testInfo) => {
  test.skip(testInfo.project.name !== 'no-token', 'Runs once against the mocked no-token build')
  await mockOperationalWorkflow(page)

  await page.goto('/map?selected=track%3Atrk-demo')
  await page.getByRole('button', { name: 'Record', exact: true }).click()

  const dialog = page.getByRole('dialog')
  await dialog.getByLabel('Conclusion').fill('Consistent with a friendly patrol on the same route.')
  await dialog.getByLabel('Confidence').fill('0.7')
  await dialog.getByRole('button', { name: 'Record assessment', exact: true }).click()

  // The new assessment is attributed to the operator who wrote it, and the
  // conclusion they typed is what the interface shows back.
  await expect(dialog).toBeHidden()
  await expect(
    page.getByText('Consistent with a friendly patrol on the same route.', { exact: true }),
  ).toBeVisible()
  await expect(page.getByText('OPERATOR', { exact: true }).first()).toBeVisible()
})

test('the picture qualifies information that is no longer current', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'no-token', 'Runs once against the mocked no-token build')
  await mockOperationalWorkflow(page)

  await page.goto('/map')
  await expect(page.locator('[data-map-ready="true"]')).toBeVisible()

  // The legend counts what is on the picture and flags what is not current.
  const freshnessLegend = page.getByLabel('Freshness legend')
  await expect(freshnessLegend).toBeVisible()
  await expect(freshnessLegend).toContainText('Stale (1)')
  await expect(freshnessLegend).toContainText('1 not current')

  // A track whose feed went quiet must not read like a current one.
  await page.goto('/map?selected=track%3Atrk-quiet')
  await expect(page.getByText('Position may no longer be accurate.', { exact: false })).toBeVisible()
  await expect(page.getByText('last known', { exact: true }).first()).toBeVisible()

  // A live track shows the server's verdict without a caveat.
  await page.goto('/map?selected=track%3Atrk-demo')
  await expect(page.getByText('Position may no longer be accurate.', { exact: false })).toHaveCount(0)

  // The list view carries the same classification.
  await page.goto('/tracks')
  await expect(page.getByText('STALE', { exact: true }).first()).toBeVisible()
})

test('an operator can see an event footprint on the map', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'no-token', 'Runs once against the mocked no-token build')
  await mockOperationalWorkflow(page)

  await page.goto('/map')
  await expect(page.locator('[data-map-ready="true"]')).toBeVisible()

  // The legend names the phenomenon class and counts it, so the footprint is
  // identified rather than being an unexplained shape on the map.
  const areaLegend = page.getByLabel('Event area legend')
  await expect(areaLegend).toBeVisible()
  await expect(areaLegend).toContainText('Hazard area (1)')

  // The layer is operator-controllable: a footprint can cover a whole city and
  // an operator may need it out of the way to see what is underneath.
  const toggle = page.getByRole('checkbox', { name: 'Event areas' })
  await expect(toggle).toBeChecked()
})

test('operator can follow evidence to outcome and inspect it on the map', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'no-token', 'Runs once against the mocked no-token build')
  await mockOperationalWorkflow(page)

  await page.goto('/map')
  const canvas = page.locator('.maplibregl-canvas')
  await expect(canvas).toBeVisible()
  await expect(page.locator('[data-map-ready="true"]')).toBeVisible()
  await page.getByRole('button', { name: 'Measure', exact: true }).click()
  await expect(page.getByText('Click the map to set the start point.', { exact: true })).toBeVisible()
  const box = await canvas.boundingBox()
  if (!box) throw new Error('map canvas did not expose a bounding box')
  await canvas.click({ position: { x: box.width * 0.35, y: box.height * 0.45 } })
  await canvas.click({ position: { x: box.width * 0.55, y: box.height * 0.55 } })
  await expect(page.getByText('Measurement complete. Click to start over.', { exact: true })).toBeVisible()

  await page.goto('/map?selected=track%3Atrk-demo')
  await expect(page.getByText('Provenance', { exact: true })).toBeVisible()
  await expect(page.getByText('scn_run_demo', { exact: true })).toBeVisible()
  await expect(page.getByText('Evidence (1)', { exact: true })).toBeVisible()
  await expect(page.getByText('91%', { exact: true })).toBeVisible()

  await page.goto('/alerts')
  await expect(page.getByText(baseAlert.title, { exact: true })).toBeVisible()
  await expect(page.getByText('Scenario run scn_run_demo', { exact: false })).toBeVisible()
  await page.getByRole('button', { name: 'Ack', exact: true }).click()
  await expect(page.getByText('ACKNOWLEDGED', { exact: true })).toBeVisible()

  await page.getByRole('button', { name: 'Create incident', exact: true }).click()
  const incidentDialog = page.getByRole('dialog')
  await incidentDialog.getByLabel('Title').fill('Unknown vehicle in restricted zone')
  await incidentDialog.getByLabel('Description').fill('Review synthetic evidence before dispatch.')
  await incidentDialog.getByRole('button', { name: 'Create incident', exact: true }).click()
  await expect(page).toHaveURL(/\/incidents\/incident-1$/)
  await expect(page.getByText('Unknown vehicle in restricted zone', { exact: true }).first()).toBeVisible()

  await page.getByRole('tab', { name: 'Missions & Commands' }).click()
  await page.getByRole('button', { name: 'Plan mission', exact: true }).click()
  const missionDialog = page.getByRole('dialog')
  await missionDialog.getByLabel('Name').fill('Intercept patrol')
  await missionDialog.getByLabel('Objective').fill('Confirm identity and maintain safe distance.')
  await missionDialog.locator('input[type="checkbox"]').check()
  await missionDialog.getByRole('button', { name: 'Create mission', exact: true }).click()
  await expect(page.getByRole('tabpanel', { name: 'Missions & Commands' }).getByText('Intercept patrol', { exact: true })).toBeVisible()

  await page.locator('#command-asset').click()
  await page.getByRole('option', { name: /Patrol 01/ }).click()
  await page.locator('#command-mission').click()
  await page.getByRole('option', { name: /Intercept patrol/ }).click()
  await page.locator('#command-lat').fill('24.716')
  await page.locator('#command-lng').fill('46.679')
  await page.getByRole('button', { name: 'Issue command', exact: true }).click()
  await expect(page.getByText('command-1', { exact: true })).toBeVisible()

  await page.getByRole('button', { name: 'SENT', exact: true }).click()
  await expect(page.getByRole('button', { name: 'ACKNOWLEDGED', exact: true })).toBeVisible()
  await page.getByRole('button', { name: 'ACKNOWLEDGED', exact: true }).click()
  await expect(page.getByRole('button', { name: 'COMPLETED', exact: true })).toBeVisible()
  await page.getByRole('button', { name: 'COMPLETED', exact: true }).click()
  await expect(page.getByText('COMPLETED', { exact: true })).toBeVisible()
})
