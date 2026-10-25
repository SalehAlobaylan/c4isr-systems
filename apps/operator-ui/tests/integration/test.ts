import { test as base, expect } from '@playwright/test'

/** Selection is typed inside the map host, and flat on standalone routes. */
export function workspaceSelection(url: string, key: string): string | null {
  const params = new URL(url).searchParams
  if (params.has(key)) return params.get(key)
  const raw = JSON.parse(params.get('workbench') ?? '{}')
  return (typeof raw === 'string' ? JSON.parse(raw) : raw)[key] ?? null
}

// Workspace changes intentionally cancel obsolete requests. A server response
// that actually reaches the browser must still never be an unnoticed 5xx.
export const test = base.extend<{ apiHealth: void }>({
  apiHealth: [async ({ page }, use) => {
    const failures: string[] = []
    page.on('response', (response) => {
      if (response.status() >= 500 && new URL(response.url()).pathname.startsWith('/api/')) failures.push(`${response.status()} ${new URL(response.url()).pathname}`)
    })
    await use()
    expect(failures, 'Unexpected API server errors reached the browser').toEqual([])
  }, { auto: true }],
})
export { expect }
