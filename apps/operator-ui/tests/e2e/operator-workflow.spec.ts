import { expect, test, type Page } from '@playwright/test'

import {
  areaObservation,
  asset,
  baseAlert,
  geofence,
  mockOperationalWorkflow,
  now,
  observation,
  point,
  staleTrack,
  track,
} from './fixtures-operational'


test('the operator can scrub back to a past picture and is never misled about it', async ({
  page,
}, testInfo) => {
  test.skip(testInfo.project.name !== 'no-token', 'Runs once against the mocked no-token build')
  await mockOperationalWorkflow(page)

  await page.goto('/map')
  await expect(page.getByTestId('scrub-mode')).toHaveText('Live')

  const timeline = page.getByLabel('Picture timeline position')
  await expect(timeline).toBeEnabled()

  // The control is a range input in epoch milliseconds, so the value must be a
  // number. The bounds are read from the element rather than hardcoded: this
  // previously aimed at an instant outside the recorded window entirely, and a
  // hardcoded timestamp cannot warn you when the fixture moves.
  const min = Number(await timeline.getAttribute('min'))
  const max = Number(await timeline.getAttribute('max'))
  expect(max).toBeGreaterThan(min)
  // One minute past the window opened. The alert in this fixture was raised
  // later in the window, so at this instant it must not exist yet.
  await timeline.fill(String(min + 60_000))

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
  //
  // Scoped to the operations rail rather than matched globally: the conclusion
  // also appears in a transient confirmation toast, so an unscoped match hits
  // two elements. The rail is the durable record, which is what this asserts.
  await expect(dialog).toBeHidden()
  const rail = page.getByRole('complementary', { name: 'Operations rail' })
  const recorded = rail.getByText('Consistent with a friendly patrol on the same route.', {
    exact: true,
  })
  await expect(recorded).toBeVisible()
  // The method, confidence and timestamp share one metadata line, so this
  // matches the attribution within that line rather than an exact node.
  await expect(rail.getByText('OPERATOR', { exact: false }).first()).toBeVisible()
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

  // The list view carries the same classification. The badge renders its label
  // and age adjacently ("STALE20 Sept, ..."), so this matches within the
  // track's own row rather than asserting an exact text node.
  await page.goto('/tracks')
  const quietRow = page.getByRole('row', { name: /trk-quiet/ })
  await expect(quietRow).toContainText('STALE')
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
  // an operator may need it out of the way to see what is underneath. The
  // controls live behind the Layers popover, so it has to be opened first.
  await page.getByRole('button', { name: 'Layers', exact: true }).click()
  const toggle = page.getByRole('checkbox', { name: 'Event areas' })
  await expect(toggle).toBeChecked()

  // Turning it off must actually remove the footprint's legend entry, so an
  // operator can tell the layer is off rather than assuming there is nothing
  // to see.
  await toggle.uncheck()
  await expect(areaLegend).toHaveCount(0)
})

test('operator can follow evidence to outcome and inspect it on the map', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'no-token', 'Runs once against the mocked no-token build')
  await mockOperationalWorkflow(page)

  await page.goto('/map')
  const canvas = page.locator('.maplibregl-canvas')
  await expect(canvas).toBeVisible()
  await expect(page.locator('[data-map-ready="true"]')).toBeVisible()
  await page.getByRole('button', { name: 'Measure', exact: true }).click()
  await expect(page.getByText('Click the plot to set the start point.', { exact: true })).toBeVisible()

  // The board overlay and the timeline control sit above the plot, and the legend
  // column and mode switch sit over its lower-left and lower-right corners, so
  // the clickable plot is a band in between. Both edges are measured rather
  // than guessed: a hardcoded fraction of the canvas height silently became a
  // click on chrome once the chrome grew, which is exactly the kind of test
  // that stops testing.
  await page.getByRole('button', { name: /Board · live plot/ }).click()

  const wrap = await page.locator('.aegis-mapwrap').boundingBox()
  const board = await page.locator('.aegis-mapboard').boundingBox()
  const legends = await page.locator('.aegis-maplegends').boundingBox()
  if (!wrap) throw new Error('map wrap did not expose a bounding box')

  const plotTop = board ? board.y + board.height : wrap.y
  const plotBottom = legends ? legends.y : wrap.y + wrap.height
  const plotHeight = plotBottom - plotTop
  if (plotHeight < 60) throw new Error('no usable plot area below the map chrome')

  // Mid-height of the free band, horizontally past the legend column.
  const midY = plotTop - wrap.y + plotHeight * 0.5
  await canvas.click({ position: { x: wrap.width * 0.62, y: midY - plotHeight * 0.15 } })
  await canvas.click({ position: { x: wrap.width * 0.78, y: midY + plotHeight * 0.15 } })
  // Matched without exact punctuation: the product's wording uses an em dash and
  // has changed once, which is why this assertion sat stale for so long.
  await expect(page.getByText('Measurement complete', { exact: false })).toBeVisible()

  // Restore the board. Overlay visibility is persisted to local storage, so
  // leaving it collapsed would hide the provenance panel asserted below even
  // across a full page navigation.
  await page.getByRole('button', { name: /Board · live plot/ }).click()

  // The track panel states that provenance is preserved, counts the evidence
  // behind the track, and keeps the classifier's confidence separate from it.
  // These assert the panel's actual wording: it renders headings such as
  // "Supporting observations (1)", not "Provenance" or "Evidence (n)", and it
  // does not surface the scenario run id at all - that is shown on the alert,
  // asserted below, which is where an operator traces provenance.
  await page.goto('/map?selected=track%3Atrk-demo')
  await expect(page.getByText('provenance preserved', { exact: false })).toBeVisible()
  await expect(page.getByText('Supporting observations (1)', { exact: true })).toBeVisible()
  // The classification confidence the classifier recorded reaches the operator
  // rather than being recomputed in the browser. The label and the confidence
  // are separate elements flanking a bar, so the row is scoped by its label
  // and the confidence asserted inside it.
  const classification = page.getByRole('listitem').filter({ hasText: 'vehicle' })
  await expect(classification.getByText('91%', { exact: true })).toBeVisible()

  await page.goto('/alerts')
  await expect(page.getByText(baseAlert.title, { exact: true })).toBeVisible()
  await expect(page.getByText('Scenario run scn_run_demo', { exact: false })).toBeVisible()
  await page.getByRole('button', { name: 'Ack', exact: true }).click()
  await expect(page.getByText('ACKNOWLEDGED', { exact: true })).toBeVisible()

  // The alerts page opens the incident dialog with "New incident"; "Create
  // incident" is the dialog's own submit button.
  await page.getByRole('button', { name: 'New incident', exact: true }).click()
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
