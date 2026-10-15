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
      command: 'pnpm dev --host 127.0.0.1 --port 5173',
      port: 5173,
      // The token is passed through `env` rather than a `VAR=value` prefix in
      // the command: that prefix is POSIX shell syntax and cmd.exe cannot parse
      // it, so the server never started on Windows at all. The no-token server
      // sets the variable to an empty string explicitly, because an inherited
      // value from the parent shell would invalidate the isolation assertions.
      env: { VITE_API_TOKEN: '' },
      // Token-isolation tests must never reuse a manually configured Vite
      // process; its environment could invalidate the no-token assertions.
      reuseExistingServer: false,
    },
    {
      command: 'pnpm dev --host 127.0.0.1 --port 5174',
      port: 5174,
      env: { VITE_API_TOKEN: 'e2e-explicit-token' },
      reuseExistingServer: false,
    },
  ],
})
