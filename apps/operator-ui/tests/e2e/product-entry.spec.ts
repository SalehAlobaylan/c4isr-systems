import { expect, test } from '@playwright/test'
import { mockOperationalWorkflow } from './fixtures-operational'
import { mockSession } from './session'

const exercise = { workspaceId: 'default', environmentId: 'local', partition: 'EXERCISE_OBSERVATION', runId: 'legacy' }
const publicResource = { workspaceId: 'default', environmentId: 'local', partition: 'PUBLIC_EVIDENCE' }
const input = { workspaceId: 'default', environmentId: 'local', partition: 'EXERCISE_INPUT' }

test.beforeEach(async ({ page }, info) => {
  test.skip(info.project.name !== 'no-token', 'One product entry and startup matrix')
  await mockOperationalWorkflow(page)
})

test('ordinary startup explains missing access and exposes Studio without granting it', async ({ page }) => {
  await page.route('**/api/v1/auth/me', async route => {
    if (route.request().headers()['x-emad-partition'] === 'EXERCISE_INPUT') {
      await route.fulfill({ status: 403, json: { error: { code: 'FORBIDDEN', message: 'resource is not authorized' } } })
    } else await route.fulfill({ json: mockSession() })
  })
  await page.goto('/')
  await expect(page.locator('main').getByRole('heading', { level: 1 })).toHaveText('Where will you work today?')
  await expect(page.getByText('This account needs an explicit Public Context workspace grant.', { exact: true })).toBeVisible()
  await expect(page.getByText('Authoring requires a separate EXERCISE_INPUT grant.', { exact: true })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Open Public Context' })).toHaveCount(0)
  await page.goto('/simulation-studio')
  await expect(page.getByRole('alert')).toContainText('explicit EXERCISE_INPUT author grant')
  await page.getByRole('link', { name: 'View EMAD workspaces' }).click()
  await expect(page).toHaveURL(/\/$/)
})

test('Public Context entry changes scope and makes intelligence modules directly reachable', async ({ page }) => {
  await page.route('**/api/v1/auth/me', async route => {
    const headers = route.request().headers()
    const resource = headers['x-emad-partition'] === 'PUBLIC_EVIDENCE' ? publicResource : headers['x-emad-partition'] === 'EXERCISE_INPUT' ? input : exercise
    await route.fulfill({ json: { ...mockSession(headers), resource, resources: [exercise, publicResource] } })
  })
  await page.goto('/')
  await page.getByRole('button', { name: 'Open Public Context' }).click()
  await expect(page.getByTestId('workspace-basis')).toContainText('Public sources')
  const nav = page.getByRole('navigation', { name: 'Workspace navigation' })
  for (const label of ['Map','Systems','Imagery']) {
    await expect(nav.getByRole('link', { name: label, exact: true })).toBeVisible()
  }
  await expect(nav.getByRole('link', { name: 'Recording & replay', exact: true })).toHaveCount(0)
  await nav.getByRole('link', { name: 'Imagery', exact: true }).click()
  await expect.poll(()=>new URL(page.url()).searchParams.get('tool')).toBe('imagery')
  await expect(page.getByRole('region',{name:'Map preview dock'}).getByRole('heading',{name:'Imagery intelligence',exact:true})).toBeVisible()
})

test('input-only authors can use Home and return to Studio without opening a COP socket', async ({ page }) => {
  const copRequests: string[] = []
  page.on('request', request => { if (/\/api\/v1\/(picture|tracks|assets|realtime)(\/|\?|$)/.test(request.url())) copRequests.push(request.url()) })
  page.on('websocket', socket => { if (socket.url().includes('/api/v1/realtime')) copRequests.push(socket.url()) })
  await page.route('**/api/v1/auth/me', async route => {
    await route.fulfill({ json: { ...mockSession(), resource: input, resources: [] } })
  })
  await page.goto('/')
  await expect(page.getByText('Protected authoring available',{exact:true})).toBeVisible()
  await page.getByRole('link', { name: 'Open Simulation Studio' }).click()
  await expect(page.getByRole('heading', { name: 'Simulation Studio', exact: true })).toBeVisible()
  await page.getByRole('link',{name:'EMAD Home',exact:true}).click()
  await expect(page.getByRole('heading', { level: 1 })).toContainText('Where will you work today?')
  expect(copRequests).toEqual([])
})

test('development effect startup opens one COP stream and keeps it across routes', async ({ page }) => {
  let streams = 0
  await page.routeWebSocket('**/api/v1/realtime**', socket => {
    streams++
    socket.send(JSON.stringify({ type: 'realtime.ready', version: 2, resource: exercise, data: {} }))
  })
  await page.goto('/')
  await expect(page.locator('.workspace-connection')).toHaveText('Events connected')
  await page.getByRole('link', { name: 'Map', exact: true }).click()
  await expect(page.locator('[data-map-ready="true"]')).toBeVisible()
  await page.getByRole('link',{name:'EMAD Home',exact:true}).click()
  await expect(page.locator('main').getByRole('heading', { level: 1 })).toContainText('Where will you work today?')
  expect(streams).toBe(1)
})
