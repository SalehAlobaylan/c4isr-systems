import { expect, test, type Page } from '@playwright/test'
import { json, mockOperationalWorkflow } from './fixtures-operational'
import { mockSession } from './session'
import { resolveScopeFixture } from './fixtures-analyst-scope'
import type { ScopeReadInput } from '../../src/lib/api'

const geometry = {
  type: 'Polygon',
  coordinates: [
    [
      [39.0, 21.0],
      [39.5, 21.0],
      [39.5, 21.5],
      [39.0, 21.5],
      [39.0, 21.0],
    ],
  ],
}
const period = {
  mode: 'historical' as const,
  interval: { from: '2026-01-01T00:00:00Z', to: '2026-01-02T00:00:00Z' },
  revisionView: 'latest' as const,
  timezone: 'UTC',
  backgroundImagery: false,
}
const studyScope = { area: { kind: 'drawn', name: 'Red Sea corridor', geometry }, temporal: period }
const mapUrl = '/map?scope=' + encodeURIComponent(JSON.stringify(studyScope))

async function setupWorkspace(page: Page) {
  await mockOperationalWorkflow(page)
  const resource = { workspaceId: 'default', environmentId: 'local', partition: 'PUBLIC_EVIDENCE' }
  await page.route('**/api/v1/auth/me', (r) =>
    json(r, {
      ...mockSession(),
      principal: { id: 'human:operator-01', kind: 'human' },
      operator: { id: 'operator-01', name: 'Fixture analyst', role: 'operator' },
      resource,
      resources: [resource],
      productProfile: 'map',
      permissions: [
        ...mockSession().permissions,
        'acquisitions.read',
        'acquisitions.request',
        'investigations.read',
        'investigations.create',
        'investigations.edit',
      ],
    }),
  )

  const quakeFeature = {
    type: 'Feature',
    id: 'quake-01',
    geometry: { type: 'Point', coordinates: [39.2, 21.3] },
    properties: {
      kind: 'earthquakes',
      recordId: 'evi-quake-01',
      evidenceId: 'evi-quake-01',
      title: 'M4.2 Red Sea tremor',
      date: '2026-01-01T12:00:00Z',
      source: 'USGS regional',
    },
  }

  await page.route('**/api/v1/analyst-scope/read', async (route) => {
    const input = route.request().postDataJSON() as ScopeReadInput
    const scope = await resolveScopeFixture(page, input)
    return json(route, {
      id: 'read-' + scope.scopeHash,
      scope,
      layers: {
        earthquakes: {
          type: 'FeatureCollection',
          features: [quakeFeature],
          total: 1,
          state: 'latest_retained',
        },
      },
      errors: {},
      expiresAt: new Date(Date.now() + 300_000).toISOString(),
    })
  })

  await page.route('**/api/v1/coverage/query', async (route) => {
    const scope = await resolveScopeFixture(page, route.request().postDataJSON().scope)
    return json(route, {
      schema: 'emad.coverage.v1',
      scope,
      evaluatedAt: scope.temporal.evaluatedAt,
      storage: 'available',
      meaning: 'Fixture readiness verdict.',
      recoveryOwner: 'source administrator',
      sources: [
        {
          id: 'usgs-quakes',
          name: 'USGS Earthquakes',
          family: 'earthquakes',
          support: 'admitted',
          supportReason: '',
          capability: { adapter: 'usgs-earthquakes-v1', historical: true },
          profiles: [],
          spatialRelation: 'full',
          retrieval: 'complete_query_footprint',
          dataResult: 'retained_records',
          usableRetainedRecords: 1,
          completedWindows: [period.interval],
          missingWindows: [],
          freshness: 'fresh',
          observationCompleteness: 'complete',
          receipts: [],
          receiptsTruncated: false,
          actions: [],
        },
        {
          id: 'firms-thermal',
          name: 'NASA FIRMS Thermal',
          family: 'thermal',
          support: 'admitted',
          supportReason: '',
          capability: { adapter: 'firms-v1', historical: true },
          profiles: [],
          spatialRelation: 'full',
          retrieval: 'no_retrieval',
          dataResult: 'no_retrieval',
          usableRetainedRecords: 0,
          completedWindows: [],
          missingWindows: [period.interval],
          freshness: 'unknown',
          observationCompleteness: 'unknown',
          receipts: [],
          receiptsTruncated: false,
          actions: [],
        },
        {
          id: 'local-naturalearth',
          name: 'Saudi geographic reference · Natural Earth',
          family: 'reference',
          support: 'retained_reference',
          supportReason: 'Versioned geographic reference artifacts are installed; static edition, not a live polling feed.',
          capability: { adapter: 'reference-import-v1', historical: true },
          profiles: [],
          spatialRelation: 'full',
          retrieval: 'retained_reference',
          dataResult: 'usable_retained_records',
          usableRetainedRecords: 120,
          completedWindows: [],
          missingWindows: [],
          freshness: 'retained_edition',
          observationCompleteness: 'unknown',
          receipts: [],
          receiptsTruncated: false,
          actions: [],
        },
        {
          id: 'local-dem',
          name: 'Jeddah surface elevation · Copernicus DEM',
          family: 'terrain',
          support: 'retained_reference',
          supportReason: 'Versioned geographic reference artifacts are installed for another footprint and do not intersect the selected area.',
          capability: { adapter: 'dem-import-v1', historical: true },
          profiles: [],
          spatialRelation: 'none',
          retrieval: 'outside_area',
          dataResult: 'outside_retained_area',
          usableRetainedRecords: 0,
          completedWindows: [],
          missingWindows: [],
          freshness: 'retained_edition',
          observationCompleteness: 'unknown',
          receipts: [],
          receiptsTruncated: false,
          actions: [],
        },
        {
          id: 'local-ais',
          name: 'External vessel observations · AISStream',
          family: 'vessels',
          support: 'not_admitted',
          supportReason: 'Provider access is not admitted yet (requires reviewed use/retention permission).',
          capability: { adapter: 'aisstream-position-v1', historical: false },
          profiles: [],
          spatialRelation: 'unknown',
          retrieval: 'not_admitted',
          dataResult: 'not_admitted',
          usableRetainedRecords: null,
          completedWindows: [],
          missingWindows: [],
          freshness: 'unknown',
          observationCompleteness: 'unknown',
          receipts: [],
          receiptsTruncated: false,
          actions: [
            {
              kind: 'inspect_source',
              sourceId: 'local-ais',
              owner: 'source administrator',
              reason: 'Record AIS use/retention permission and a private API key file.',
            },
          ],
        },
      ],
    })
  })

  await page.route('**/api/v1/investigations**', (route) =>
    json(route, { items: [], total: 0, offset: 0, hasMore: false }),
  )
}

test.beforeEach(({}, info) => test.skip(info.project.name !== 'no-token', 'Deterministic UI interaction contract'))

test('rail, resizable drawer, anchored time popover, contextual legend and focus map stay non-competing', async ({
  page,
}) => {
  await page.setViewportSize({ width: 1440, height: 900 })
  await setupWorkspace(page)
  await page.goto(mapUrl)

  // 1. Unified Scope & Readiness Bar counts only in-scope applicable sources (2 ready: USGS + Natural Earth, 1 gap: FIRMS; excludes outside-area DEM and not-admitted AIS)
  const coveragePill = page.locator('.map-scope-bar').getByRole('button', { name: 'Coverage & readiness', exact: true })
  await expect(coveragePill).toContainText('2/3 ready')
  await expect(coveragePill).toContainText('1 gap')

  // 2. Anchored Time & filters popover opens directly with filters visible (no nested collapsed accordion)
  await page.getByRole('button', { name: 'Time & filters', exact: true }).click()
  const popover = page.getByRole('region', { name: 'Investigation and time' })
  await expect(popover).toBeVisible()
  await expect(popover.getByLabel('Filter public evidence')).toBeVisible()
  await expect(popover.getByRole('button', { name: 'Apply public filters' })).toBeVisible()
  await page.keyboard.press('Escape')
  await expect(popover).toHaveCount(0)

  // 3. Fixed Left Rail switches panels cleanly inside one single resizable drawer
  const rail = page.getByRole('navigation', { name: 'Map panels' })
  await rail.getByRole('button', { name: 'Map layers', exact: true }).click()
  const drawer = page.getByRole('region', { name: 'Map layers' })
  await expect(drawer).toBeVisible()

  // Keyboard resize via separator widens the drawer
  const initialBox = await drawer.boundingBox()
  const resizer = page.getByRole('separator', { name: 'Resize map support panel' })
  await resizer.focus()
  await page.keyboard.press('Shift+ArrowRight')
  await page.keyboard.press('Shift+ArrowRight')
  const resizedBox = await drawer.boundingBox()
  expect(resizedBox!.width).toBeGreaterThan(initialBox!.width + 40)

  // Direct 1-click Maximize and Return
  await page.getByRole('button', { name: 'Maximize map workbench', exact: true }).click()
  await expect(drawer).toHaveAttribute('data-size', 'full')
  await page.getByRole('button', { name: 'Return tool to map', exact: true }).click()
  await expect(drawer).not.toHaveAttribute('data-size', 'full')

  // Switching to Coverage opens task-first verdict card and honest source groups
  await coveragePill.click()
  const coverageRegion = page.getByRole('region', { name: 'Coverage and acquisition', exact: true })
  await expect(coverageRegion).toBeVisible()
  await expect(coverageRegion.getByText('Partial readiness · 2 of 3 sources ready')).toBeVisible()
  await expect(coverageRegion.getByText(/Retained reference/)).toBeVisible()
  await expect(coverageRegion.getByText('Outside selected area (1)')).toBeVisible()
  await expect(coverageRegion.getByText('Not admitted · awaiting activation (1)')).toBeVisible()

  // 4. Focus map hides rail, drawer and scope bar; Restore workspace brings them back
  await page.getByRole('button', { name: 'Focus map', exact: true }).click()
  await expect(rail).toBeHidden()
  await expect(coverageRegion).toBeHidden()
  await page.getByRole('button', { name: 'Restore workspace', exact: true }).click()
  await expect(rail).toBeVisible()
  await expect(coverageRegion).toBeVisible()
})

test('selection peek card coexists with drawer and attaches citations to active case in one click', async ({
  page,
}) => {
  await page.setViewportSize({ width: 1440, height: 900 })
  await setupWorkspace(page)
  await page.goto(mapUrl)

  // Expand contextual legend and jump to Earthquake records
  await page.getByRole('button', { name: 'Show area situation', exact: true }).click()
  const quakeLegendRow = page.locator('.map-legend-row').filter({ hasText: 'Earthquake reports' })
  await expect(quakeLegendRow).toBeVisible()
  await quakeLegendRow.click()

  const recordsDrawer = page.getByRole('region', { name: 'Map records' })
  await expect(recordsDrawer).toBeVisible()

  // Select a record — Floating Selection Peek Card appears WITHOUT closing or hijacking Map records!
  await recordsDrawer.getByRole('button', { name: /M4\.2 Red Sea tremor/ }).click()
  await expect(recordsDrawer).toBeVisible()
  const peekCard = page.getByRole('region', { name: 'Selected map object' })
  await expect(peekCard).toBeVisible()
  await expect(peekCard.getByRole('heading', { name: 'M4.2 Red Sea tremor' })).toBeVisible()

  // One-click Attach to case opens Investigations and pre-populates the attached evidence item
  await peekCard.getByRole('button', { name: 'Attach to case', exact: true }).click()
  await expect(page.getByRole('textbox', { name: 'Investigation title', exact: true })).toBeVisible()
  await expect(page.getByText(/observation · evi-quake-01/)).toBeVisible()
  await expect(peekCard.getByRole('button', { name: 'In case', exact: true })).toBeDisabled()
})

const fixtureInventory = {
  schema: 'emad.coverage-inventory.v1',
  evaluatedAt: '2026-01-02T00:00:00Z',
  meaning: 'Synthetic read-only workspace inventory.',
  totalRetained: 31,
  sources: [
    {
      sourceId: 'usgs-quakes',
      name: 'USGS Earthquakes · Saudi region',
      family: 'earthquakes',
      support: 'admitted',
      gate: 'ready',
      retainedCount: 25,
      earliestAt: '2025-03-04T02:00:00Z',
      latestAt: '2025-03-27T18:00:00Z',
      bounds: { west: 34, south: 16, east: 56, north: 33 },
      dailyBudget: 50,
      attemptsToday: 1,
    },
    {
      sourceId: 'firms-thermal',
      name: 'NASA FIRMS Thermal',
      family: 'thermal',
      support: 'admitted',
      gate: 'ready',
      retainedCount: 0,
      bounds: { west: 39.0, south: 21.0, east: 39.5, north: 21.5 },
      dailyBudget: 25,
      attemptsToday: 2,
    },
    {
      sourceId: 'earth-search-s2',
      name: 'Sentinel-2 Collection 1 · Earth Search',
      family: 'imagery',
      support: 'admitted',
      gate: 'ready',
      retainedCount: 6,
      earliestAt: '2025-03-10T08:00:00Z',
      latestAt: '2025-03-22T08:00:00Z',
      bounds: { west: 39.0, south: 21.3, east: 39.35, north: 21.7 },
      dailyBudget: 40,
      attemptsToday: 0,
    },
  ],
  timeBuckets: [
    {
      month: '2025-03',
      label: 'Mar 2025',
      from: '2025-03-01T00:00:00Z',
      to: '2025-04-01T00:00:00Z',
      total: 31,
      byFamily: { earthquakes: 25, imagery: 6 },
      bounds: { west: 39.0, south: 21.3, east: 39.35, north: 21.7 },
      suggestedAreaName: 'Jeddah corridor',
    },
  ],
  footprints: [
    {
      key: '39.00,21.30,39.35,21.70',
      name: 'Jeddah corridor',
      bounds: { west: 39.0, south: 21.3, east: 39.35, north: 21.7 },
      sourceIds: ['earth-search-s2', 'firms-thermal'],
      sourceNames: ['Sentinel-2 Collection 1 · Earth Search', 'NASA FIRMS Thermal'],
      families: ['imagery', 'thermal'],
      retainedCount: 6,
    },
    {
      key: '34.00,16.00,56.00,33.00',
      name: 'Saudi region',
      bounds: { west: 34, south: 16, east: 56, north: 33 },
      sourceIds: ['usgs-quakes'],
      sourceNames: ['USGS Earthquakes · Saudi region'],
      families: ['earthquakes'],
      retainedCount: 25,
    },
  ],
}

test('data radar exposes retained timeline, all-retained records and 1-click approved footprints', async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 900 })
  await setupWorkspace(page)
  await page.route('**/api/v1/coverage/inventory', (route) => json(route, fixtureInventory))
  await page.goto(mapUrl)

  // 1. Time & filters popover exposes "Jump to retained dates" chips
  await page.getByRole('button', { name: 'Time & filters', exact: true }).click()
  const popover = page.getByRole('region', { name: 'Investigation and time' })
  await expect(popover.getByText('Jump to retained dates')).toBeVisible()
  await popover.getByRole('button', { name: /Mar 2025/ }).click()
  await expect(popover).toHaveCount(0)
  await expect(page.getByRole('navigation', { name: 'Selected geographic area' })).toContainText('Jeddah corridor')

  // 2. Records drawer exposes "This view" vs "All retained (31)" with Jump to data
  await page.getByRole('navigation', { name: 'Map panels' }).getByRole('button', { name: 'Map records', exact: true }).click()
  const recordsDrawer = page.getByRole('region', { name: 'Map records' })
  const allRetainedTab = recordsDrawer.getByRole('tab', { name: /All retained \(31\)/ })
  await expect(allRetainedTab).toBeVisible()
  await allRetainedTab.click()
  await expect(recordsDrawer.getByText('USGS Earthquakes · Saudi region')).toBeVisible()
  await expect(recordsDrawer.getByText('Approved regional footprints')).toBeVisible()
  await recordsDrawer.getByRole('button', { name: 'Jump to data' }).first().click()
  await expect(recordsDrawer.getByRole('tab', { name: /This view/ })).toHaveAttribute('aria-selected', 'true')

  // 3. Contextual Legend provides 1-click Approved footprints overlay toggle
  await page.getByRole('button', { name: 'Show area situation', exact: true }).click()
  const footprintsToggle = page.getByRole('button', { name: /Approved footprints \(2\)/ })
  await expect(footprintsToggle).toHaveAttribute('aria-pressed', 'false')
  await footprintsToggle.click()
  await expect(footprintsToggle).toHaveAttribute('aria-pressed', 'true')
})

test('smart fetch checklist queues durable batch intents, retries failed items with pinned key and separates out-of-area review', async ({
  page,
}) => {
  await page.setViewportSize({ width: 1440, height: 900 })
  await setupWorkspace(page)
  await page.route('**/api/v1/coverage/inventory', (route) => json(route, fixtureInventory))
  await page.route('**/api/v1/coverage/query', async (route) => {
    const scope = await resolveScopeFixture(page, route.request().postDataJSON().scope)
    return json(route, {
      schema: 'emad.coverage.v1',
      scope,
      evaluatedAt: scope.temporal.evaluatedAt,
      storage: 'available',
      meaning: 'Smart fetch fixture readiness verdict.',
      recoveryOwner: 'source administrator',
      sources: [
        {
          id: 'firms-thermal',
          name: 'NASA FIRMS Thermal',
          family: 'thermal',
          support: 'admitted',
          supportReason: '',
          capability: { adapter: 'firms-v1', historical: true },
          profiles: [
            {
              key: 'saudi-daily',
              revisionId: 'rev-firms-1',
              gate: 'ready',
              spatialRelation: 'full',
              executionGranted: true,
              rights: {},
            },
          ],
          spatialRelation: 'full',
          retrieval: 'no_retrieval',
          dataResult: 'no_retrieval',
          usableRetainedRecords: 0,
          completedWindows: [],
          missingWindows: [period.interval],
          freshness: 'unknown',
          observationCompleteness: 'unknown',
          receipts: [],
          receiptsTruncated: false,
          actions: [
            {
              kind: 'request_acquisition',
              sourceId: 'firms-thermal',
              profileKey: 'saudi-daily',
              profileRevisionId: 'rev-firms-1',
              owner: 'analyst',
              reason: 'Within approved profile.',
            },
          ],
        },
        {
          id: 'earth-search-s2',
          name: 'Sentinel-2 Collection 1 · Earth Search',
          family: 'imagery',
          support: 'admitted',
          supportReason: '',
          capability: { adapter: 'earth-search-v1', historical: true },
          profiles: [
            {
              key: 'jeddah-optical',
              revisionId: 'rev-s2-1',
              gate: 'ready',
              spatialRelation: 'partial',
              executionGranted: true,
              rights: {},
            },
          ],
          spatialRelation: 'partial',
          retrieval: 'no_retrieval',
          dataResult: 'no_retrieval',
          usableRetainedRecords: 0,
          completedWindows: [],
          missingWindows: [period.interval],
          freshness: 'unknown',
          observationCompleteness: 'unknown',
          receipts: [],
          receiptsTruncated: false,
          actions: [
            {
              kind: 'request_acquisition',
              sourceId: 'earth-search-s2',
              profileKey: 'jeddah-optical',
              profileRevisionId: 'rev-s2-1',
              owner: 'analyst',
              reason: 'Within approved profile.',
            },
          ],
        },
        {
          id: 'jeddah-dem',
          name: 'Jeddah surface elevation · Copernicus DEM',
          family: 'reference',
          support: 'admitted',
          supportReason: '',
          capability: { adapter: 'copernicus-dem-v1', historical: true },
          profiles: [],
          spatialRelation: 'none',
          retrieval: 'outside_area',
          dataResult: 'outside_retained_area',
          usableRetainedRecords: 0,
          completedWindows: [],
          missingWindows: [],
          freshness: 'unknown',
          observationCompleteness: 'unknown',
          receipts: [],
          receiptsTruncated: false,
          actions: [
            {
              kind: 'request_coverage_review',
              sourceId: 'jeddah-dem',
              profileKey: 'jeddah-dem',
              profileRevisionId: 'rev-dem-1',
              owner: 'source administrator',
              reason: 'The selected area exceeds this approved profile.',
            },
          ],
        },
      ],
    })
  })

  const postedBodies: Array<{ sourceId: string; requestKey: string; scope: unknown }> = []
  let earthSearchAttempts = 0
  await page.route('**/api/v1/acquisitions', async (route) => {
    if (route.request().method() === 'GET') return route.fallback()
    const body = route.request().postDataJSON()
    postedBodies.push(body)
    if (body.sourceId === 'earth-search-s2' && earthSearchAttempts === 0) {
      earthSearchAttempts++
      return json(route, { error: { code: 'internal_error', message: 'Response lost after durable save' } }, 500)
    }
    if (body.sourceId === 'earth-search-s2') earthSearchAttempts++
    return json(
      route,
      {
        id: `acq-${postedBodies.length}`,
        input: body,
        scope: await resolveScopeFixture(page, body.scope),
        kind: body.sourceId === 'jeddah-dem' ? 'COVERAGE_REVIEW' : 'ACQUISITION',
        status: 'QUEUED',
        reason: 'Durable intent saved.',
        recoveryOwner: 'connector operator',
        requesterId: 'operator-01',
        createdAt: '2026-01-02T00:00:00Z',
        parts: [],
      },
      201,
    )
  })

  await page.goto(mapUrl)
  await page.locator('.map-scope-bar').getByRole('button', { name: 'Coverage & readiness', exact: true }).click()
  const panel = page.getByRole('region', { name: 'Coverage and acquisition', exact: true })

  // Smart Fetch checklist is prominent with both eligible sources pre-checked and quota readouts visible
  const smartSection = panel.getByRole('region', { name: 'Smart fetch eligible sources' })
  await expect(smartSection).toBeVisible()
  await expect(smartSection.getByText('Today: 2/25')).toBeVisible()
  await expect(smartSection.getByText('Today: 0/40')).toBeVisible()
  // Raw single-profile dropdown stays collapsed by default
  await expect(panel.getByLabel('Approved source profile')).toBeHidden()

  // Submit batch: FIRMS succeeds, Earth Search simulates lost response -> pinned retry reuses exact requestKey & scope
  await smartSection.getByRole('button', { name: 'Fetch 2 selected sources', exact: true }).click()
  await expect(smartSection.getByText('QUEUED')).toBeVisible()
  const retryBtn = smartSection.getByRole('button', { name: 'Retry 1 failed with same intent', exact: true })
  await expect(retryBtn).toBeVisible()
  await retryBtn.click()
  await expect(smartSection.getByText(/Queued 1 acquisition intent/)).toBeVisible()

  expect(postedBodies).toHaveLength(3)
  expect(postedBodies[1].sourceId).toBe('earth-search-s2')
  expect(postedBodies[2]).toEqual(postedBodies[1])

  // Out-of-area section separates covered-footprint jump from admin coverage expansion
  const outsideSection = panel.getByRole('region', { name: 'Outside approved area and coverage expansion' })
  await expect(outsideSection).toBeVisible()
  await expect(outsideSection.getByRole('button', { name: /Jump to Jeddah corridor/ })).toBeVisible()
  const reviewBtn = outsideSection.getByRole('button', { name: 'Request admin coverage expansion', exact: true })
  await expect(reviewBtn).toBeDisabled()
})

test('never-empty map discovery banner guides operators on empty scopes and disappears when data loads', async ({
  page,
}) => {
  await page.setViewportSize({ width: 1440, height: 900 })
  await setupWorkspace(page)
  await page.route('**/api/v1/coverage/inventory', (route) => json(route, fixtureInventory))
  await page.route('**/api/v1/analyst-scope/read', async (route) => {
    const input = route.request().postDataJSON() as ScopeReadInput
    const scope = await resolveScopeFixture(page, input)
    const isMarch2025 = scope.temporal.interval.from.startsWith('2025-03')
    return json(route, {
      id: 'read-' + scope.scopeHash,
      scope,
      layers: {
        earthquakes: {
          type: 'FeatureCollection',
          features: isMarch2025
            ? [
                {
                  type: 'Feature',
                  id: 'q-march-1',
                  geometry: { type: 'Point', coordinates: [39.2, 21.5] },
                  properties: {
                    kind: 'earthquakes',
                    recordId: 'evi-march-1',
                    title: 'March 2025 retained quake',
                    date: '2025-03-14T10:00:00Z',
                    source: 'USGS regional',
                  },
                },
              ]
            : [],
          total: isMarch2025 ? 1 : 0,
          state: 'latest_retained',
        },
      },
      errors: {},
      expiresAt: new Date(Date.now() + 300_000).toISOString(),
    })
  })

  await page.goto(mapUrl)

  const banner = page.getByRole('region', { name: 'Map data discovery guidance' })
  await expect(banner).toBeVisible()
  await expect(banner).toContainText('0 records displayed here · 31 retained in workspace')
  await expect(banner).toContainText('Zero displayed records does not establish absence')
  await expect(banner.getByRole('button', { name: 'Smart Fetch (1 eligible)', exact: true })).toBeVisible()

  // One-click jump to Mar 2025 loads retained data and banner automatically hides
  await banner.getByRole('button', { name: /Jump to Mar 2025 · Jeddah corridor \(31\)/ }).click()
  await expect(banner).toHaveCount(0)
  await expect(page.getByRole('navigation', { name: 'Selected geographic area' })).toContainText('Jeddah corridor')
})
