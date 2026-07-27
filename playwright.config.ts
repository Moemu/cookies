import { defineConfig, devices } from '@playwright/test'
import { tmpdir } from 'node:os'
import { join } from 'node:path'

const dataDirectory = join(tmpdir(), 'cookies-e2e-store')
const isGitHubActions = process.env.GITHUB_ACTIONS === 'true'
const reuseExistingServer = !isGitHubActions
const apiPort = process.env.E2E_API_PORT ?? (isGitHubActions ? '8787' : '18787')
const apiBaseURL = `http://127.0.0.1:${apiPort}`
process.env.E2E_API_BASE_URL = apiBaseURL

export default defineConfig({
  testDir: './e2e',
  fullyParallel: false,
  globalTeardown: './e2e/global-teardown.ts',
  metadata: { e2eDataDirectory: dataDirectory },
  use: {
    baseURL: 'http://127.0.0.1:4173',
    trace: 'retain-on-failure',
  },
  webServer: [
    {
      command: 'tsx e2e/mock-ark.ts',
      url: 'http://127.0.0.1:8791/test/mode',
      reuseExistingServer,
    },
    {
      command: 'npm run server',
      url: `${apiBaseURL}/health`,
      env: {
        ...process.env,
        NODE_ENV: 'test',
        ARK_API_KEY: 'e2e-test-credential',
        ARK_BASE_URL: 'http://127.0.0.1:8791/api/v3',
        ARK_ALLOW_INSECURE_LOCAL_PROVIDER: 'true',
        DATA_FILE: join(dataDirectory, 'mvp-store.json'),
        PORT: apiPort,
        RESET_DATA_FILE: 'true',
        SKIP_DEMO_SEED: 'true',
      },
      reuseExistingServer,
    },
    {
      command: 'npm run dev -- --port 4173',
      url: 'http://127.0.0.1:4173',
      env: {
        ...process.env,
        VITE_API_BASE_URL: apiBaseURL,
      },
      reuseExistingServer,
    },
  ],
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
})
