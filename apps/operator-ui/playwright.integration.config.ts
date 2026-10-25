import { defineConfig, devices } from '@playwright/test'

export default defineConfig({
  testDir: './tests/integration',
  workers: 1,
  outputDir: '../../tmp/f0-6-browser-results',
  retries: 0,
  timeout: 90_000,
  use: { ...devices['Desktop Chrome'], viewport: { width: 1440, height: 1000 }, trace: 'retain-on-failure', actionTimeout: 10_000 },
})
