import {mapControl} from '../recovery/atlas-helpers'
import { readFile } from 'node:fs/promises'

import { expect, test, type Page } from '@playwright/test'

import { json, list, mockOperationalWorkflow } from './fixtures-operational'
import { mockSession } from './session'
import { resolveScopeFixture } from './fixtures-analyst-scope'
import type { ScopeReadInput } from '../../src/lib/api'

async function openEvidence(page:Page) {
  const btn = page.getByRole('button',{name:'Evidence inspector',exact:true})
  const popover = page.getByRole('region',{name:'Investigation and time'})
  if ((await btn.getAttribute('aria-pressed')) !== 'true' || (await popover.isVisible())) {
    await btn.click()
  }
  await expect(page.getByLabel('Public event and reference data')).toBeVisible();await expect(page.getByRole('dialog')).toHaveCount(0)
}

test('public events retain source revisions and reference attribution on the map', async ({ page }) => {
  await mockOperationalWorkflow(page)
  const session = mockSession({ 'x-emad-partition': 'PUBLIC_EVIDENCE', 'x-emad-run-id': '' })
  await page.route('**/api/v1/auth/me', (route) => json(route, session))
  const sample = JSON.parse(await readFile('../../testdata/reference/jeddah-makkah/places.json', 'utf8'))
  const collection = JSON.parse(Buffer.from(sample.artifact, 'base64').toString('utf8'))
  const dataset = { ...sample, artifact: undefined, id: 'ref-places', approvedBy: 'reviewer', approvedAt: sample.retrievedAt, bytes: 1436, featureCount: 2, eligibleForSimulation: true }
  const report = { evidenceId: 'evi-quake', sourceId: 'public-usgs', providerRecordId: 'usgs-record', providerRevision: '2', admissionId: 'admission-2', observationId: 'obs-quake', supersedesId: 'evi-old', revisionAt: '2026-10-21T05:00:00Z', receivedAt: '2026-10-21T05:01:00Z', reported: { semanticKind: 'point_event', type: 'event.earthquake', title: 'Provider earthquake report', observedAt: '2026-10-21T04:00:00Z', position: { lat: 21.5, lng: 39.2 }, geometryRole: 'reported_location', withdrawn: false, properties: { depthKm: 12, magnitude: 3.5 }, quality: { reviewStatus: 'reviewed' } } }
  const older = { ...report, evidenceId: 'evi-old', providerRevision: '1', admissionId: 'admission-1', supersedesId: undefined }
  const original = '{ "type": "Feature", "id": "usgs-record" }'
  await page.route('**/api/v1/public-events?*', (route) => json(route, list([report])))
  await page.route('**/api/v1/public-events/evi-*', (route) => json(route, new URL(route.request().url()).pathname.endsWith('evi-old') ? older : report))
  await page.route('**/api/v1/public-events/*/revisions?*', (route) => json(route, { ...list([report, older]), truncated: false }))
  await page.route('**/api/v1/reference-datasets', (route) => json(route, list([dataset])))
  await page.route('**/api/v1/reference-datasets/ref-places/features', (route) => json(route, collection))
  await page.route('**/api/v1/reference-features?*',route=>json(route,{...collection,matched:collection.features.length,truncated:false}))
  await page.route('**/api/v1/sources/public-usgs/activation',route=>json(route,{sourceId:'public-usgs',state:'CONFIGURED',reason:'Configured retrieval with retained evidence',nextAction:'Inspect coverage',health:{state:'stale',pendingPublications:0,meaning:'Coverage only'}}))
  await page.route('**/api/v1/reference-datasets/ref-places/artifact', (route) => json(route, collection))
  await page.route('**/api/v1/sources?*', (route) => json(route, list([{ id: 'public-usgs', name: 'USGS regional query', type: 'external', status: 'active', ingestion: { state: 'stale', pendingPublications: 0, meaning: 'Coverage only' } }])))
  await page.route('**/api/v1/sources/public-usgs/evidence/*', (route) => json(route, { contentHash: 'evidence-hash', originalRecord: JSON.parse(original) }))
  await page.route('**/api/v1/sources/public-usgs/evidence/*/original', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: original }))
  await page.route('**/api/v1/sources/public-usgs/admissions/*', (route) => json(route, { originalProducer: 'USGS', upstreamProvider: 'ComCat', rights: { attribution: 'U.S. Geological Survey' } }))
  await page.goto('/map')
  await openEvidence(page)
  // Native details elements expose a group; all source facts remain available
  // without requiring WebGL hover or precise point selection.
  await expect(page.getByText('Made with Natural Earth, v5.1.2, 1:50 million', { exact: true })).toBeVisible()
  await page.getByText('Provider coverage and activation · current, historical and gated sources',{exact:true}).click()
  await expect(page.getByLabel('Provider coverage')).toContainText('USGS regional query')
  await expect(page.getByLabel('Provider coverage')).toContainText('stale')
  await page.getByRole('button', { name: /^Provider earthquake report · event\.earthquake/ }).click()
  const evidence = page.getByRole('region', { name: 'Reported event evidence' })
  await expect(evidence.getByText('USGS via ComCat', { exact: false })).toBeVisible()
  await evidence.getByText('Source revisions (2)', { exact: true }).click()
  await evidence.getByRole('button', { name: /^1 ·/ }).click()
  await expect(evidence.getByText('revision 1', { exact: false })).toBeVisible()
  await evidence.getByText('Retained original provider record', { exact: true }).click()
  const download = page.waitForEvent('download')
  await evidence.getByRole('button', { name: 'Download exact original' }).click()
  await expect((await download).suggestedFilename()).toBe('evi-old.json')
  await expect(page.locator('[data-map-ready="true"]')).toBeVisible()
  await expect(page.locator('[data-map-layers]')).toHaveAttribute('data-map-layers', /earthquakes-symbol/)
  await page.screenshot({ path: '../../tmp/p1-2-4-public-context-desktop.png', fullPage: true })
  await page.setViewportSize({ width: 390, height: 844 })
  await expect(page.getByLabel('Reference version')).toBeVisible()
  await page.getByLabel('Reference version').click({ trial: true })
  await mapControl(page,'Map layers');await expect(page.getByRole('region',{name:'Map layers'}).getByRole('checkbox',{name:'Earthquake reports',exact:false})).toBeVisible()
  await page.screenshot({ path: '../../tmp/p1-2-4-public-context-mobile.png', fullPage: true })
})


test('public filters bookmark historical meaning and retain exact FIRMS CSV', async ({ page }) => {
  await mockOperationalWorkflow(page)
  await page.route('**/api/v1/auth/me', (route) => json(route, mockSession({ 'x-emad-partition': 'PUBLIC_EVIDENCE', 'x-emad-run-id': '' })))
  const report = { evidenceId: 'evi-firms', sourceId: 'public-firms', providerRecordId: 'derived-pixel', providerRevision: 'content-revision', admissionId: 'firms-admission', observationId: 'obs-firms', revisionAt: '2026-10-20T05:00:00Z', receivedAt: '2026-10-20T05:00:00Z', knownAt: '2026-10-20T05:00:01Z', effectiveAt: '2026-10-20T04:00:00Z', effectiveBasis: 'reported_observation_no_revision_time', qualityClass: 'nominal', reported: { semanticKind: 'point_event', type: 'thermal.detection', title: 'Reported VIIRS thermal detection', observedAt: '2026-10-20T04:00:00Z', position: { lat: 21.5, lng: 39.2 }, geometryRole: 'reported_location', withdrawn: false, properties: { scanKm: .4, trackKm: .37 }, quality: { providerConfidence: 'nominal' } } }
  const original = 'latitude,longitude,confidence\n21.5,39.2,n\n'
  const reads: ScopeReadInput[] = []
  await page.route(/\/api\/v1\/public-events(?:[/?].*)?$/, (route) => {
    const u = new URL(route.request().url())
    if (u.pathname.endsWith('/revisions')) return json(route, { ...list([report]), truncated: false })
    if (u.pathname.endsWith('/evi-firms')) return json(route, report)
    return json(route, list(u.searchParams.get('quality') === 'low' ? [] : [report]))
  })
  await page.route('**/api/v1/analyst-scope/read', async route => {
    const input = route.request().postDataJSON() as ScopeReadInput
    if (!input.includeEvents) return route.fallback()
    reads.push(input)
    const scope = await resolveScopeFixture(page, input)
    const items = input.filters?.quality === 'low' ? [] : [report]
    return json(route, {id: 'event-read-' + scope.scopeHash, scope, layers: {}, errors: {},
      expiresAt: new Date(Date.now()+300000).toISOString(), events: {...list(items), truncated: false}})
  })
  await page.route('**/api/v1/reference-datasets', (route) => json(route, list([])))
  await page.route('**/api/v1/sources?*', (route) => json(route, list([{ id: 'public-firms', name: 'NASA FIRMS NOAA-20', ingestion: { state: 'unavailable' } }])))
  await page.route('**/api/v1/sources/public-firms/evidence/evi-firms', (route) => json(route, { contentHash: 'parsed-hash', rawHash: 'exact-csv-hash', originalMediaType: 'text/csv' }))
  await page.route('**/api/v1/sources/public-firms/evidence/evi-firms/original', (route) => route.fulfill({ status: 200, contentType: 'text/csv', body: original }))
  await page.route('**/api/v1/sources/public-firms/admissions/firms-admission', (route) => json(route, { originalProducer: 'NASA LANCE', upstreamProvider: 'FIRMS', rights: { retainRaw: true, attribution: 'NASA FIRMS', legalBasis: 'Open data with attribution' } }))
  await page.goto('/map')
  await openEvidence(page)
  await page.getByRole('button',{name:'Time & filters',exact:true}).click()
  const filters = page.getByLabel('Filter public evidence')
  await filters.getByLabel('AOI: west,south,east,north').fill('NaN,21,40,22')
  await filters.getByRole('button', { name: 'Apply public filters' }).click()
  await expect(filters.getByRole('alert')).toContainText('AOI must be')
  await filters.getByLabel('AOI: west,south,east,north').fill('39,21,40,22')
  await filters.getByLabel('Report source').selectOption('public-firms')
  await filters.getByRole('combobox', { name: 'Phenomenon', exact: true }).selectOption('thermal.detection')
  await filters.getByLabel('Provider quality').selectOption('nominal')
  await filters.getByLabel('Historical interpretation').selectOption('known_at')
  await filters.getByLabel('As-of time (local)').fill('2026-10-20T12:00')
  await filters.getByRole('button', { name: 'Apply public filters' }).click()
  await expect(page).toHaveURL(/publicView=/)
  await expect(page.getByLabel('Public view meaning')).toContainText('Later arrivals and publications are excluded')
  await expect(page.getByLabel('Operations rail')).toHaveCount(0)
  await expect.poll(() => reads.some((input) => {
    if (input.filters?.sourceId !== 'public-firms') return false
    const ring = (input.area.geometry?.coordinates as number[][][] | undefined)?.[0]
    return input.temporal.revisionView === 'known_at' && !!input.temporal.asOf &&
      !!ring && Math.min(...ring.map(p => p[0])) === 39 && Math.max(...ring.map(p => p[0])) === 40 &&
      Math.min(...ring.map(p => p[1])) === 21 && Math.max(...ring.map(p => p[1])) === 22
  })).toBe(true)
  const bookmark = page.url()
  await page.reload()
  await openEvidence(page)
  await expect(page.getByLabel('Evidence time meaning')).toContainText('Later arrivals')
  await page.getByRole('button', { name: /^Reported VIIRS thermal detection ·/ }).click()
  const evidence = page.getByRole('region', { name: 'Reported event evidence' })
  await expect(evidence.getByText('Reported thermal anomaly, not a confirmed fire or perimeter.', { exact: false })).toBeVisible()
  await expect(evidence.getByText('Published by EMAD: 2026-10-20 05:00:01', { exact: false })).toBeVisible()
  await evidence.getByText('Retained original provider record', { exact: true }).click()
  await evidence.getByRole('button', { name: 'View exact original' }).click()
  await expect(evidence.locator('pre').last()).toContainText('21.5,39.2,n')
  const download = page.waitForEvent('download')
  await evidence.getByRole('button', { name: 'Download exact original' }).click()
  expect((await download).suggestedFilename()).toBe('evi-firms.csv')
  expect(page.url()).toBe(bookmark)
  await page.getByRole('button',{name:'Time & filters',exact:true}).click()
  await filters.getByLabel('Historical interpretation').selectOption('effective_at')
  await filters.getByRole('button', { name: 'Apply public filters' }).click()
  await expect(page.getByLabel('Public view meaning')).toContainText('Late arrivals may appear')
  await filters.getByLabel('Provider quality').selectOption('low')
  await filters.getByRole('button', { name: 'Apply public filters' }).click()
  await openEvidence(page)
  await expect(page.getByText('No published reports in this bounded view.', { exact: false })).toBeVisible()
  await expect(page.getByLabel('Provider coverage')).toContainText('unavailable')
  await page.setViewportSize({ width: 390, height: 844 })
  await page.getByRole('button',{name:'Time & filters',exact:true}).click()
  await expect(filters.getByRole('button', { name: 'Reset public filters' })).toBeVisible()
  await page.screenshot({ path: '../../tmp/p16-filters-mobile.png', fullPage: true })
  await filters.getByRole('button', { name: 'Reset public filters' }).click()
  await expect(page).toHaveURL(url => {
    const raw = url.searchParams.get('publicView')
    if (!raw) return true
    const parsed = JSON.parse(raw)
    const value = typeof parsed === 'string' ? JSON.parse(parsed) : parsed
    return value.view === 'latest' && !value.asOf && !value.quality && !value.sourceId
  })
  await expect(page.getByLabel('Public view meaning')).toContainText('Latest published')
})

test('invalid saved public view is visible and cannot silently change history', async ({ page }) => {
  await mockOperationalWorkflow(page)
  await page.route('**/api/v1/auth/me', (route) => json(route, mockSession({ 'x-emad-partition': 'PUBLIC_EVIDENCE', 'x-emad-run-id': '' })))
  await page.route('**/api/v1/public-events?*', (route) => json(route, list([])))
  await page.route('**/api/v1/reference-datasets', (route) => json(route, list([])))
  await page.goto(`/map?publicView=${encodeURIComponent(JSON.stringify({ view: 'known_at', asOf: 'invalid' }))}`)
  await expect(page.getByRole('alert').filter({ hasText: 'The saved public view is invalid' })).toBeVisible()
  await page.getByRole('button',{name:'Time & filters',exact:true}).click()
  await expect(page.getByLabel('Public view meaning')).toContainText('Latest published')
})
