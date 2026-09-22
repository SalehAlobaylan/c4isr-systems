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
  metadata: {
    sourceId: 'scn_run_demo__source__radar-01',
    scenarioRunId: 'scn_run_demo',
    resourceNamespace: 'scn_run_demo__',
  },
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

  await page.route('**/health', (route) =>
    json(route, { status: 'ok', database: 'up', time: now }),
  )
  await page.route('**/api/v1/**', async (route) => {
    const request = route.request()
    const method = request.method()
    const path = new URL(request.url()).pathname

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
      await json(route, list([track]))
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
      await json(route, list([{ id: 'assessment-1', subjectType: 'track', subjectId: track.id, type: 'identification', conclusion: 'Likely patrol vehicle', method: 'analyst', confidence: 0.8, createdBy: 'operator-01', createdAt: now, evidence: [] }]))
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
