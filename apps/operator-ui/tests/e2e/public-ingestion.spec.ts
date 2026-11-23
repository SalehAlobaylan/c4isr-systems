import { expect, test } from '@playwright/test'

import { json, list, mockOperationalWorkflow, sources } from './fixtures-operational'
import { mockSession } from './session'

test('source coverage exposes outages and pending evidence independently of registration', async ({ page }) => {
  await mockOperationalWorkflow(page)
  await page.route('**/api/v1/sources?*', (route) => json(route, list([
    {
      ...sources[0], type: 'external', status: 'active', name: 'Public event feed',
      ingestion: {
        state: 'unavailable', pendingPublications: 2,
        lastSuccessAt: '2026-09-20T11:41:00Z',
        meaning: 'Provider query coverage only; missing records are not evidence of absence.',
      },
    },
  ])))
  await page.goto('/systems')
    await page.getByRole('button',{name:'Operations diagnostics',exact:true}).click()
  const inventory = page.getByRole('table', { name: 'Registered sources and the state each is in' })
  await expect(inventory.getByRole('columnheader', { name: 'Provider coverage' })).toBeVisible()
  const row = inventory.getByRole('row').filter({ hasText: 'Public event feed' })
  await expect(row.getByText('ACTIVE', { exact: true })).toBeVisible()
  await expect(row.getByText('UNAVAILABLE', { exact: true })).toBeVisible()
  await expect(row.getByText('2 awaiting publication', { exact: false })).toBeVisible()
  await expect(page.getByTitle('Provider query coverage; missing records do not prove absence.', { exact: true })).toBeVisible()
})

test('retained geographic sources display their inputs and staged release', async ({page}, info) => {
  test.skip(info.project.name !== 'no-token', 'One retained source presentation check')
  await mockOperationalWorkflow(page)
  const session = mockSession({'x-emad-partition': 'PUBLIC_EVIDENCE', 'x-emad-run-id': ''})
  session.permissions.push('environments.read', 'environments.qualify')
  await page.route('**/api/v1/auth/me', route => json(route, session))
  const inventories = [
    {id: 'fixture-geoid', name: 'Riyadh urban reference · NGA EGM2008 geoid', layer: 'geoid', editions: ['EGM2008']},
    {id: 'fixture-dsm', name: 'Riyadh urban reference · Copernicus GLO-30 Public DSM', layer: 'surface', editions: ['Copernicus GLO-30']},
    {id: 'fixture-sentinel', name: 'Riyadh urban reference · Sentinel-2 L2A window', layer: 'imagery', editions: ['B02', 'B03', 'B04', 'B08', 'SCL']},
    {id: 'fixture-osm', name: 'Riyadh urban reference · OpenStreetMap source vectors', layer: 'features', editions: ['Buildings', 'Roads', 'Water']},
  ]
  const time = '2025-10-23T07:34:42Z'
  const release = {id: `georelease_${'1'.repeat(26)}`, name: 'Riyadh urban reference', state: 'STAGED', manifestSha256: 'a'.repeat(64), createdAt: time}
  await page.route('**/api/v1/sources?*', route => json(route, list(inventories.map(source => ({...source, type: 'external', status: 'active'})))))
  await page.route('**/api/v1/sources/*/activation', route => json(route, {
    state: 'RETAINED', reason: 'Retained geographic inputs are available in the environment library.', nextAction: '',
    geographicRetention: {readable: true, assetsAvailable: true}, health: {state: 'not_admitted'},
  }))
  await page.route('**/api/v1/environment-assets?*', route => {
    const query = new URL(route.request().url()).searchParams
    const source = inventories.find(source => source.id === query.get('source'))!
    const items = source.editions.map(productEdition => ({id: productEdition, bytes: 512, sha256: 'b'.repeat(64), retainUntil: time,
      representation: {productEdition, layer: source.layer, nativeResolution: '30 m', dataTime: time, retrievedAt: time, verticalDatum: 'EGM2008', crs: 'EPSG:4326', producer: 'Fixture producer', format: 'geotiff'}}))
    if (source.layer === 'imagery') return json(route, query.has('after') ? {items: items.slice(2)} : {items: items.slice(0, 2), next: items[1].id})
    return json(route, {items})
  })
  await page.route('**/api/v1/environment-releases?*', route => {
    expect(inventories.some(source => source.id === new URL(route.request().url()).searchParams.get('source'))).toBe(true)
    return json(route, {items: [release]})
  })
  await page.goto('/map?tool=sources')
  for (const source of inventories) {
    const card = page.locator('article.source-card').filter({has: page.getByRole('heading', {name: source.name, exact: true})})
    await expect(card.getByText('RETAINED', {exact: true})).toBeVisible()
    await expect(card.getByText(/^STAGED/)).toBeVisible()
    await expect(card.getByText('No successful retrieval recorded')).toHaveCount(0)
    await expect(card.getByText('Configure and admit a source binding on the server.')).toHaveCount(0)
    const link = card.getByRole('link', {name: release.name})
    const href = new URL((await link.getAttribute('href'))!, 'http://example.test')
    expect(href.searchParams.get('tool')).toBe('environments')
    const workbench = JSON.parse(href.searchParams.get('workbench')!)
    expect(typeof workbench === 'string' ? JSON.parse(workbench) : workbench).toEqual({id: release.id, manifest: release.manifestSha256})
    if (source.layer === 'imagery') await card.getByRole('button', {name: 'Show more inputs'}).click()
    const inputs = card.getByRole('region', {name: 'Retained environment inputs'})
    await expect(inputs.getByRole('listitem')).toHaveCount(source.editions.length)
    await expect(inputs.getByRole('strong')).toHaveText(source.editions)
    await inputs.getByText('Asset provenance').first().click()
    await expect(inputs.getByText('Fixture producer', {exact: true}).first()).toBeVisible()
  }
})
