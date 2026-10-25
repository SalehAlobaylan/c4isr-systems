import { defineConfig, devices } from '@playwright/test'

// Allow the full suite beside a running persistent installation.
const noTokenPort=Number(process.env.EMAD_E2E_PORT??5173)
const tokenPort=Number(process.env.EMAD_E2E_TOKEN_PORT??5174)

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
      use: { ...devices['Desktop Chrome'], baseURL: `http://127.0.0.1:${noTokenPort}` },
    },
    {
      name: 'explicit-token',
      use: { ...devices['Desktop Chrome'], baseURL: `http://127.0.0.1:${tokenPort}` },
    },
  ],
  webServer: [
    {
      command: `pnpm dev --host 127.0.0.1 --port ${noTokenPort}`,
      port: noTokenPort,
      // The token is passed through `env` rather than a `VAR=value` prefix in
      // the command: that prefix is POSIX shell syntax and cmd.exe cannot parse
      // it, so the server never started on Windows at all. The no-token server
      // sets the variable to an empty string explicitly, because an inherited
      // value from the parent shell would invalidate the isolation assertions.
      env: { VITE_API_TOKEN: '', EMAD_VITE_CACHE_DIR: `node_modules/.vite-e2e-${noTokenPort}` },
      // Token-isolation tests must never reuse a manually configured Vite
      // process; its environment could invalidate the no-token assertions.
      reuseExistingServer: false,
    },
    {
      command: `pnpm dev --host 127.0.0.1 --port ${tokenPort}`,
      port: tokenPort,
      env: { VITE_API_TOKEN: 'e2e-explicit-token', EMAD_VITE_CACHE_DIR: `node_modules/.vite-e2e-${tokenPort}` },
      reuseExistingServer: false,
    },
  ],
})
