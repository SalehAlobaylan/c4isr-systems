import { expect, test, type Page, type Route } from '@playwright/test'

const scenario = {
  name: 'restricted-area-intrusion',
  description: 'Synthetic replay',
  seed: 1007,
  sources: 1,
  assets: 1,
  tracks: 1,
  geofences: 1,
  events: 8,
}

const runs = [
  {
    id: 'scn_run_1',
    resourceNamespace: 'scn_run_1__',
    scenarioName: scenario.name,
    seed: 1007,
    status: 'RUNNING',
    playbackSpeed: 1,
    virtualTimeMs: 12_000,
    lastAction: 'observe.unknown-01',
    lastActionAtMs: 12_000,
    eventsRun: 3,
    eventsTotal: 8,
    startedAt: '2026-09-19T10:00:00Z',
  },
  {
    id: 'scn_run_2',
    resourceNamespace: 'scn_run_2__',
    scenarioName: scenario.name,
    seed: 1007,
    status: 'STOPPED',
    playbackSpeed: 1,
    virtualTimeMs: 5_000,
    lastAction: 'observe.unknown-01',
    lastActionAtMs: 5_000,
    eventsRun: 2,
    eventsTotal: 8,
    startedAt: '2026-09-19T09:00:00Z',
    endedAt: '2026-09-19T09:05:00Z',
  },
]

const events = {
  items: [
    { sequence: 1, atMs: 0, name: 'telemetry.patrol-01', status: 'completed' },
    { sequence: 2, atMs: 5_000, name: 'observe.unknown-01', status: 'failed', error: 'invalid coordinate' },
    { sequence: 3, atMs: 10_000, name: 'command.complete', status: 'skipped', error: 'scenario run stopped' },
  ],
  total: 3,
}

async function json(route: Route, body: unknown, status = 200): Promise<void> {
  await route.fulfill({
    status,
    contentType: 'application/json',
    body: JSON.stringify(body),
  })
}

async function mockApi(page: Page, options: { unauthorized?: boolean } = {}): Promise<void> {
  await page.route('**/health', (route) =>
    json(route, { status: 'ok', database: 'up', time: '2026-09-19T10:00:00Z' }),
  )
  await page.route('**/api/v1/**', async (route) => {
    const request = route.request()
    const url = new URL(request.url())
    if (options.unauthorized && url.pathname.endsWith('/scenarios')) {
      await json(route, { error: { code: 'unauthorized', message: 'a valid bearer token is required' } }, 401)
      return
    }
    if (url.pathname.endsWith('/scenarios/runs')) {
      await json(route, { items: runs, total: runs.length })
      return
    }
    if (url.pathname.endsWith('/scenarios/runs/scn_run_1/events')) {
      await json(route, events)
      return
    }
    if (url.pathname.endsWith('/scenarios/runs/scn_run_2/events')) {
      await json(route, events)
      return
    }
    if (url.pathname.endsWith('/scenarios/runs/scn_run_1/restart')) {
      await json(route, { ...runs[0], id: 'scn_run_3', resourceNamespace: 'scn_run_3__' }, 201)
      return
    }
    if (url.pathname.endsWith('/scenarios')) {
      await json(route, { items: [scenario], total: 1 })
      return
    }
    await json(route, {})
  })
}

test.describe('scenario console', () => {
  test('does not invent a production token and renders failed/skipped evidence', async ({ page }, testInfo) => {
    test.skip(testInfo.project.name !== 'no-token', 'Runs once against the no-token build')
    const websocketUrls: string[] = []
    const authorizationHeaders: string[] = []
    page.on('websocket', (socket) => websocketUrls.push(socket.url()))
    page.on('request', (request) => {
      if (request.url().includes('/api/v1/')) {
        authorizationHeaders.push(request.headers().authorization ?? '')
      }
    })
    await mockApi(page)
    await page.goto('/scenarios')

    await expect(page.getByRole('heading', { name: 'Scenarios', exact: true })).toBeVisible()
    await expect(page.getByText(scenario.name).first()).toBeVisible()
    expect(authorizationHeaders.every((value) => value === '')).toBe(true)

    await page.getByRole('button', { name: 'Events' }).first().click()
    await expect(page.getByText('FAILED')).toBeVisible()
    await expect(page.getByText('SKIPPED')).toBeVisible()
    await expect(page.getByText('invalid coordinate')).toBeVisible()

    for (const url of websocketUrls) {
      expect(url).not.toContain('access_token=')
    }
  })

  test('restart targets the selected run and injects the configured token', async ({ page }, testInfo) => {
    test.skip(testInfo.project.name !== 'explicit-token', 'Runs once against the explicit-token build')
    const authorizationHeaders: string[] = []
    const restartRequests: string[] = []
    page.on('request', (request) => {
      if (request.url().includes('/api/v1/')) {
        authorizationHeaders.push(request.headers().authorization ?? '')
        if (request.method() === 'POST' && request.url().includes('/restart')) {
          restartRequests.push(request.url())
        }
      }
    })
    await mockApi(page)
    await page.goto('/scenarios')
    await expect(page.getByText('scn_run_1')).toBeVisible()
    await page.getByRole('button', { name: 'Restart' }).first().click()
    await expect.poll(() => restartRequests.length).toBe(1)
    expect(restartRequests[0]).toContain('/scenarios/runs/scn_run_1/restart')
    await expect.poll(() => authorizationHeaders.length).toBeGreaterThan(0)
    expect(authorizationHeaders.every((value) => value === 'Bearer e2e-explicit-token')).toBe(true)
  })

  test('presents a useful authentication configuration error', async ({ page }, testInfo) => {
    test.skip(testInfo.project.name !== 'no-token', 'Runs once against the no-token build')
    await mockApi(page, { unauthorized: true })
    await page.goto('/scenarios')
    await expect(page.getByText('Authentication failed. Configure VITE_API_TOKEN for the operator UI.').first()).toBeVisible()
  })
})
