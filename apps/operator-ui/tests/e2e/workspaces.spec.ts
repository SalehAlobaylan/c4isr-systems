import { expect, test } from '@playwright/test'
import { mockOperationalWorkflow, track } from './fixtures-operational'
import { mockSession } from './session'

test('workspace switch discards pending responses, selection and replay state', async ({ page }, info) => {
  test.skip(info.project.name !== 'no-token', 'One deterministic lifecycle check')
  await mockOperationalWorkflow(page)
  const exercise = { workspaceId: 'default', environmentId: 'local', partition: 'EXERCISE_OBSERVATION', runId: 'legacy' }
  const publicContext = { workspaceId: 'public', environmentId: 'world', partition: 'PUBLIC_EVIDENCE' }
  await page.route('**/api/v1/auth/me', async (route) => {
    const headers = route.request().headers()
    const resource = headers['x-emad-partition'] === 'PUBLIC_EVIDENCE' ? publicContext : exercise
    await route.fulfill({ json: { ...mockSession(headers), resource, resources: [exercise, publicContext] } })
  })
  let release: () => void = () => {}
  const delayed = new Promise<void>((resolve) => { release = resolve })
  let pending = 0
  await page.route('**/api/v1/tracks?**', async (route) => {
    const isPublic = route.request().headers()['x-emad-partition'] === 'PUBLIC_EVIDENCE'
    if (!isPublic) { pending++; await delayed }
    try {
      await route.fulfill({ json: { items: [{ ...track, id: isPublic ? 'PUBLIC_ONLY' : 'OLD_EXERCISE', externalRef: isPublic ? 'PUBLIC_ONLY' : 'OLD_EXERCISE', metadata: {} }], total: 1 } })
    } catch { /* A cancelled old request cannot deliver its response. */ }
  })
  await page.goto('/map?selected=track:trk-demo')
  await expect(page.getByRole('button', { name: 'Public Context', exact: true })).toBeEnabled()
  await expect.poll(() => pending).toBeGreaterThan(0)
  const timeline = page.getByLabel('Picture timeline position')
  await expect(timeline).toBeEnabled()
  await timeline.fill(String(Number(await timeline.getAttribute('min')) + 60_000))
  await expect(page.getByTestId('scrub-mode')).toHaveText('Replay')
  await page.getByRole('button', { name: 'Public Context', exact: true }).click()
  await expect(page.getByTestId('workspace-basis')).toContainText('Public sources')
  await expect(page).toHaveURL(/\/map$/)
  await page.screenshot({ path: '../../tmp/f0-6-public-workspace.png' })
  await expect(page.getByRole('region', { name: 'Scenario run' })).toHaveCount(0)
  release()
  await page.getByRole('link', { name: 'Map', exact: true }).click()
  await expect(page.getByText('Supporting observations (1)', { exact: true })).toHaveCount(0)
  await page.goto('/tracks')
  await expect(page.getByRole('row').filter({ hasText: 'PUBLIC_ONLY' })).toBeVisible()
  await expect(page.getByText('OLD_EXERCISE', { exact: true })).toHaveCount(0)
  await expect(page.getByTestId('workspace-basis')).toContainText('Public sources')
  await page.goto('/scenarios')
  await expect(page.getByText('This view requires an authorized exercise workspace.')).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Scenarios', exact: true })).toHaveCount(0)
})
