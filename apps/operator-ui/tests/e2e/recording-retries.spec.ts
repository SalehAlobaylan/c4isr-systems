import {expect, test} from '@playwright/test'
import {mockOperationalWorkflow} from './fixtures-operational'
import {mockSession} from './session'

test('recording retries stay visible and stop at the attempt limit', async ({page}) => {
  await mockOperationalWorkflow(page)
  await page.route('**/api/v1/auth/me', async route => {
    const session = mockSession(route.request().headers())
    const resource={...session.resource,runId:'retry-run'}
    await route.fulfill({json: {...session,resource,resources:[resource], permissions: [...session.permissions, 'sim.recordings.read']}})
  })
  let reads = 0
  await page.route('**/api/v1/simulation-recordings/retry-run', async route => {
    reads++
    const attempts = reads < 3 ? 1 : 3
    await route.fulfill({json: {
      status: reads === 2 ? 'PROCESSING' : 'FAILED',
      errorCode: reads === 2 ? '' : 'ARCHIVE_FAILED', attempts, maxAttempts: 3,
      ...(attempts === 1 ? {retryAt: new Date(Date.now() + 20_000).toISOString()} : {}),
    }})
  })
  await page.goto('/recordings?run=retry-run&ms=0&sequence=0')
  const status = page.getByRole('group', {name: 'Recording status'})
  await expect(status).toContainText('The worker will retry automatically')
  await expect(status).toContainText('Archive attempts: 1 / 3')
  await expect(page.getByRole('button', {name: 'Archive closed run', exact: true})).toBeDisabled()
  await expect(status).toContainText('PROCESSING', {timeout: 6000})
  await expect(status).toContainText('Archive failed after the maximum attempts', {timeout: 6000})
  await expect(status).toContainText('Archive attempts: 3 / 3')
  // There must be no further polling once the durable retry budget is spent.
  await page.waitForTimeout(2500)
  expect(reads).toBe(3)
})
