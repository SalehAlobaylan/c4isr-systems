import { expect, test } from '@playwright/test'

import { asset, geofence, mockOperationalWorkflow, staleTrack, track } from '../e2e/fixtures-operational'

/**
 * Reference-screen captures.
 *
 * These exist to be looked at, not asserted against. They cover the four
 * workspaces DESIGN.md §11 names, at 1440px and at one compact width, plus the
 * state variations that a single happy-path screenshot hides: replay, a stale
 * entity, a selected entity, and a validation error.
 */

/**
 * Captures land in the repository's `docs/design-review`, so the review lives
 * with the design contract rather than inside the application's build tree.
 * Paths resolve from the package directory this script is run in.
 */
const OUT = '../../docs/design-review'

test.beforeEach(async ({ page }) => {
  await mockOperationalWorkflow(page)
  // The layout store persists to local storage; a review capture must start
  // from the shipped defaults, not from whatever a previous run left behind.
  await page.addInitScript(() => window.localStorage.clear())
})

test('A — operational picture, live', async ({ page }) => {
  await page.goto('/map')
  await expect(page.locator('[data-map-ready="true"]')).toBeVisible()
  await page.waitForTimeout(1_200)
  await page.screenshot({ path: `${OUT}/01-picture-live.png` })
})

test('A — operational picture, selected asset and a stale track', async ({ page }) => {
  await page.goto('/map?selected=track%3Atrk-quiet')
  await expect(page.locator('[data-map-ready="true"]')).toBeVisible()
  await page.waitForTimeout(1_200)
  await page.screenshot({ path: `${OUT}/02-picture-stale-selected.png` })
})

test('A — operational picture, replay', async ({ page }) => {
  await page.goto('/map')
  await expect(page.locator('[data-map-ready="true"]')).toBeVisible()
  const timeline = page.getByLabel('Picture timeline position')
  await expect(timeline).toBeEnabled()
  const min = Number(await timeline.getAttribute('min'))
  await timeline.fill(String(min + 60_000))
  await expect(page.getByTestId('scrub-mode')).toHaveText('Replay')
  await page.waitForTimeout(1_200)
  await page.screenshot({ path: `${OUT}/03-picture-replay.png` })
})

test('A — operational picture, compact width', async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 800 })
  await page.goto('/map')
  await expect(page.locator('[data-map-ready="true"]')).toBeVisible()
  await page.waitForTimeout(1_200)
  await page.screenshot({ path: `${OUT}/04-picture-1280.png` })
})

test('B — entity board, default density', async ({ page }) => {
  await page.goto('/board')
  await expect(page.locator('.aegis-board')).toBeVisible()
  await page.screenshot({ path: `${OUT}/05-board-default.png` })
})

test('B — entity board, compact density', async ({ page }) => {
  await page.goto('/board')
  await page.getByRole('button', { name: 'Compact', exact: true }).click()
  await expect(page.locator('.aegis-board[data-density="compact"]')).toBeVisible()
  await page.screenshot({ path: `${OUT}/06-board-compact.png` })
})

test('C — alert center', async ({ page }) => {
  await page.goto('/alerts')
  await expect(page.getByRole('row', { name: /Restricted Zone A/ })).toBeVisible()
  await page.screenshot({ path: `${OUT}/07-alerts.png` })
})

test('C — alert center, empty filter', async ({ page }) => {
  await page.goto('/alerts?severity=low')
  await expect(page.getByText('No alerts match the current filter.')).toBeVisible()
  await page.screenshot({ path: `${OUT}/08-alerts-empty.png` })
})

test('D — incident detail, overview', async ({ page }) => {
  await page.goto('/alerts')
  await page.getByRole('button', { name: 'New incident', exact: true }).click()
  const dialog = page.getByRole('dialog')
  await dialog.getByLabel('Title').fill('Unknown vehicle in restricted zone')
  await dialog.getByLabel('Description').fill('Review synthetic evidence before dispatch.')
  await dialog.getByRole('button', { name: 'Create incident', exact: true }).click()
  await expect(page).toHaveURL(/\/incidents\/incident-1$/)
  await page.screenshot({ path: `${OUT}/09-incident-overview.png` })
})

test('D — incident detail, missions and commands', async ({ page }) => {
  await page.goto('/alerts')
  await page.getByRole('button', { name: 'New incident', exact: true }).click()
  const dialog = page.getByRole('dialog')
  await dialog.getByLabel('Title').fill('Unknown vehicle in restricted zone')
  await dialog.getByRole('button', { name: 'Create incident', exact: true }).click()
  await expect(page).toHaveURL(/\/incidents\/incident-1$/)
  await page.getByRole('tab', { name: 'Missions & Commands' }).click()
  await page.waitForTimeout(400)
  await page.screenshot({ path: `${OUT}/10-incident-operations.png` })
})

test('D — incident detail, form validation state', async ({ page }) => {
  await page.goto('/incidents')
  await page.getByRole('button', { name: 'New incident', exact: true }).click()
  const dialog = page.getByRole('dialog')
  await dialog.getByRole('button', { name: 'Create incident', exact: true }).click()
  await expect(dialog.getByRole('alert').first()).toBeVisible()
  await page.screenshot({ path: `${OUT}/11-incident-validation.png` })
})

test('E — remaining routes', async ({ page }) => {
  for (const [name, path] of [
    ['12-assets', '/assets'],
    ['13-tracks', '/tracks'],
    ['14-timeline', '/timeline'],
    ['15-scenarios', '/scenarios'],
    ['16-incidents', '/incidents'],
  ] as const) {
    await page.goto(path)
    await page.waitForTimeout(300)
    await page.screenshot({ path: `${OUT}/${name}.png` })
  }
})

test('F — keyboard focus on a form', async ({ page }) => {
  await page.goto('/incidents')
  await page.getByRole('button', { name: 'New incident', exact: true }).click()
  await page.getByLabel('Title').focus()
  await page.screenshot({ path: `${OUT}/17-focus.png` })
})

test('G — narrow layout', async ({ page }) => {
  await page.setViewportSize({ width: 900, height: 800 })
  await page.goto('/board')
  await expect(page.locator('.aegis-board')).toBeVisible()
  await page.screenshot({ path: `${OUT}/18-board-narrow.png` })
})

test('H — wide layout', async ({ page }) => {
  await page.setViewportSize({ width: 1920, height: 1080 })
  await page.goto('/map')
  await expect(page.locator('[data-map-ready="true"]')).toBeVisible()
  await page.waitForTimeout(1_200)
  await page.screenshot({ path: `${OUT}/19-picture-1920.png` })
})

test('H — menu and tooltip', async ({ page }) => {
  await page.goto('/map')
  await expect(page.locator('[data-map-ready="true"]')).toBeVisible()

  // A menu is one of the few surfaces that floats over the map and is not a
  // panel; it has to hold the same surface and border language as everything
  // else (DESIGN.md §7.4).
  await page.getByRole('button', { name: /Signed in as/ }).click()
  await expect(page.getByRole('menu')).toBeVisible()
  await page.screenshot({ path: `${OUT}/20-menu.png` })
  await page.keyboard.press('Escape')

  // The health control's accessible name is its own visible text ("SYS OK"), so
  // it is targeted by its title rather than by a name that changes with state.
  await page.locator('button[title^="Backend health"]').hover()
  await page.waitForTimeout(400)
  await page.screenshot({ path: `${OUT}/21-tooltip.png` })
})

test('I — 200% browser zoom keeps the console usable', async ({ page }) => {
  // DESIGN.md §10 requires zoom without loss of essential content.
  //
  // Halving the viewport is exactly what browser zoom does to layout: CSS
  // pixels get bigger, so the same window reports half as many of them, and
  // every media and container query fires as if the window had shrunk. Setting
  // a CSS `zoom` factor would scale the pixels without touching the viewport,
  // which is the one thing zoom does *not* do — and would have quietly proved
  // nothing.
  await page.setViewportSize({ width: 720, height: 450 })
  await page.goto('/map')
  await expect(page.locator('[data-map-ready="true"]')).toBeVisible()
  await page.waitForTimeout(1_200)
  await page.screenshot({ path: `${OUT}/22-zoom-200.png` })
})

test('J — map preparing state', async ({ page }) => {
  // Captured before the map finishes loading, which is the only moment the
  // loading state is on screen. Held deliberately: this is a loading state, not
  // a failure, and it must not read as an empty picture.
  await page.goto('/map')
  await expect(page.locator('[data-map-ready="true"]')).toBeVisible()
  await page.route('**/api/v1/**', async (route) => {
    await new Promise((resolve) => setTimeout(resolve, 2_500))
    await route.continue()
  })
  await page.reload()
  await page.waitForTimeout(700)
  await page.screenshot({ path: `${OUT}/23-map-loading.png` })
})

test('K — board with long identifiers', async ({ page }) => {
  // The plan asks for long identifiers specifically. The board must absorb
  // them by truncating the displayed cell while the full value stays reachable
  // on hover, not by widening the column and pushing the rest off screen.
  const longTrack = {
    ...track,
    id: 'scn_run_demo__track__convoy-escort-762-reconnaissance-patrol',
    externalRef: 'convoy-escort-762-reconnaissance-patrol',
  }
  const longQuiet = {
    ...staleTrack,
    id: 'scn_run_demo__track__quiet-sensor-991-forward-observation',
    externalRef: 'quiet-sensor-991-forward-observation',
  }
  const longAsset = {
    ...asset,
    id: 'scn_run_demo__asset__patrol-01-reconnaissance-uav-alpha',
    name: 'Patrol 01 Reconnaissance UAV Alpha',
  }
  await page.route('**/api/v1/**', async (route) => {
    const url = new URL(route.request().url())
    if (route.request().method() === 'GET' && url.pathname === '/api/v1/tracks') {
      await route.fulfill({ json: { items: [longTrack, longQuiet], total: 2 } })
      return
    }
    if (route.request().method() === 'GET' && url.pathname === '/api/v1/assets') {
      await route.fulfill({ json: { items: [longAsset], total: 1 } })
      return
    }
    await route.fallback()
  })
  await page.goto('/board')
  await expect(page.locator('.aegis-board')).toBeVisible()
  await page.waitForTimeout(400)
  await page.screenshot({ path: `${OUT}/24-board-long-identifiers.png` })
})

test('L — alert center, request failure', async ({ page }) => {
  // A failed fetch is not an empty list, and the two must never look alike
  // (DESIGN.md §7.7). The error state shows the failure, the server's message
  // and a retry — captured so the review can compare it against the empty
  // state in `08`.
  await page.route('**/api/v1/alerts*', (route) =>
    route.fulfill({
      status: 503,
      json: { error: { code: 'unavailable', message: 'Alerts service unavailable' } },
    }),
  )
  await page.goto('/alerts')
  await expect(page.getByText('Request failed')).toBeVisible()
  await page.screenshot({ path: `${OUT}/25-alerts-error.png` })
})

test('M — picture, measurement over two restricted zones', async ({ page }) => {
  // GIS coverage in the plan: multiple geofences, the measurement tool active,
  // entities over restricted areas. The second zone is medium severity, so
  // severity reads as a scale rather than a synonym for "restricted".
  const secondZone = {
    ...geofence,
    id: 'scn_run_demo__geofence__restricted-zone-b',
    name: 'Restricted Zone B',
    severity: 'medium',
    geojson: JSON.stringify({
      type: 'Polygon',
      coordinates: [[[46.652, 24.695], [46.668, 24.695], [46.668, 24.709], [46.652, 24.709], [46.652, 24.695]]],
    }),
  }
  await page.route('**/api/v1/geofences*', (route) =>
    route.fulfill({ json: { items: [geofence, secondZone], total: 2 } }),
  )
  await page.goto('/map')
  await expect(page.locator('[data-map-ready="true"]')).toBeVisible()
  await page.waitForTimeout(1_200)
  await page.getByRole('button', { name: 'Measure' }).click()
  // Two plot points clear of the legends, the scale bar, the bottom-right
  // measure panel and the mode switch.
  await page.mouse.click(520, 560)
  await page.mouse.click(700, 660)
  await page.waitForTimeout(400)
  await page.screenshot({ path: `${OUT}/26-picture-measure.png` })
})