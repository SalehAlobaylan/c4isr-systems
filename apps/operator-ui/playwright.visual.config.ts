import { defineConfig, devices } from '@playwright/test'

/**
 * Visual capture for the Alloy Pearl design review.
 *
 * Separate from `playwright.config.ts` on purpose: this run is only for
 * producing the before/after artifacts the design review needs, and it must
 * never become part of a gate that fails a build over a pixel. Everything it
 * renders is deterministic — the fixture, the viewport, the fonts and the
 * moment on screen are all fixed — so two runs of the same revision produce
 * comparable images.
 *
 *   pnpm --filter @c4isr/operator-ui capture:visual
 */
export default defineConfig({
  testDir: './tests/visual',
  fullyParallel: false,
  workers: 1,
  retries: 0,
  reporter: 'list',
  outputDir: './test-results/visual',
  use: {
    ...devices['Desktop Chrome'],
    baseURL: 'http://127.0.0.1:5173',
    viewport: { width: 1440, height: 900 },
    deviceScaleFactor: 1,
    // Animation and transition state has to be pinned or two captures of the
    // same revision will differ for reasons that have nothing to do with design.
    reducedMotion: 'reduce',
    // DESIGN.md §1.2: there is exactly one appearance, balanced light. Pinning
    // it here is a guard, not a preference — a capture run under a dark OS
    // preference must not silently produce a second theme.
    colorScheme: 'light',
  },
  projects: [{ name: 'visual' }],
  webServer: [
    {
      command: 'pnpm dev --host 127.0.0.1 --port 5173',
      port: 5173,
      env: { VITE_API_TOKEN: '' },
      reuseExistingServer: false,
    },
  ],
})