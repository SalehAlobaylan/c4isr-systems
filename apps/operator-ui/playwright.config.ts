import { defineConfig, devices } from '@playwright/test'

export default defineConfig({
  testDir: './tests/e2e',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  reporter: process.env.CI ? 'github' : 'list',
  use: {
    ...devices['Desktop Chrome'],
    trace: 'retain-on-failure',
  },
  projects: [
    {
      name: 'no-token',
      use: { ...devices['Desktop Chrome'], baseURL: 'http://127.0.0.1:5173' },
    },
    {
      name: 'explicit-token',
      use: { ...devices['Desktop Chrome'], baseURL: 'http://127.0.0.1:5174' },
    },
  ],
  webServer: [
    {
      command: 'VITE_API_TOKEN= pnpm dev --host 127.0.0.1 --port 5173',
      port: 5173,
      // Token-isolation tests must never reuse a manually configured Vite
      // process; its environment could invalidate the no-token assertions.
      reuseExistingServer: false,
    },
    {
      command: 'VITE_API_TOKEN=e2e-explicit-token pnpm dev --host 127.0.0.1 --port 5174',
      port: 5174,
      reuseExistingServer: false,
    },
  ],
})
