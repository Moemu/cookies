import { defineConfig, devices } from '@playwright/test'
import { tmpdir } from 'node:os'
import { join } from 'node:path'

const dataDirectory = join(tmpdir(), 'cookies-e2e-store')

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
      reuseExistingServer: false,
    },
    {
      command: 'npm run server',
      url: 'http://127.0.0.1:8787/health',
      env: {
        ...process.env,
        NODE_ENV: 'test',
        ARK_API_KEY: 'e2e-test-credential',
        ARK_BASE_URL: 'http://127.0.0.1:8791/api/v3',
        ARK_ALLOW_INSECURE_LOCAL_PROVIDER: 'true',
        DATA_FILE: join(dataDirectory, 'mvp-store.json'),
        PORT: '8787',
        RESET_DATA_FILE: 'true',
        SKIP_DEMO_SEED: 'true',
      },
      reuseExistingServer: false,
    },
    {
      command: 'npm run dev -- --port 4173',
      url: 'http://127.0.0.1:4173',
      reuseExistingServer: false,
    },
  ],
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
})
