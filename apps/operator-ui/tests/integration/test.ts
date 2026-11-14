import { test as base, expect } from '@playwright/test'

/** Selection is typed inside the map host, and flat on standalone routes. */
export function workspaceSelection(url: string, key: string): string | null {
  const params = new URL(url).searchParams
  if (params.has(key)) return params.get(key)
  const raw = JSON.parse(params.get('workbench') ?? '{}')
  return (typeof raw === 'string' ? JSON.parse(raw) : raw)[key] ?? null
}

export function scopedAddress(origin: string, path: string, scope: object, selection: Record<string, string> = {}) {
  const address = new URL(path, origin)
  address.search = new URLSearchParams({ ...selection, scope: JSON.stringify(scope) }).toString()
  return address.toString()
}

export function studyArea(west: number, south: number, east: number, north: number) {
  return { kind: 'drawn', geometry: { type: 'Polygon', coordinates: [[[west, south], [east, south], [east, north], [west, north], [west, south]]] } }
}

// Workspace changes intentionally cancel obsolete requests. A server response
// that actually reaches the browser must still never be an unnoticed 5xx.
export const test = base.extend<{ apiHealth: void }>({
  apiHealth: [async ({ page, request }, use) => {
    const fixture = JSON.parse(process.env.EMAD_BROWSER_FIXTURE ?? '{}')
    const reset = await request.post(`${fixture.url}/__fixture/reset-retained-reads`, {
      headers: { Authorization: `Bearer ${fixture.token}` },
    })
    expect(reset.status(), 'Disposable fixture retained-read reset').toBe(204)
    const failures: string[] = []
    page.on('response', (response) => {
      if (response.status() >= 500 && new URL(response.url()).pathname.startsWith('/api/')) failures.push(`${response.status()} ${new URL(response.url()).pathname}`)
    })
    await use()
    expect(failures, 'Unexpected API server errors reached the browser').toEqual([])
  }, { auto: true }],
})
export { expect }
