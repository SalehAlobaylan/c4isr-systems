import { expect, test } from '@playwright/test'

import { json, list, mockOperationalWorkflow, sources } from './fixtures-operational'

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
