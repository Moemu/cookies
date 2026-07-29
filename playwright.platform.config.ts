import { defineConfig, devices } from '@playwright/test'

const apiBaseURL = 'http://127.0.0.1:18080'

const localGoEnv = {
  COOKIES_ENV: 'local',
  COOKIES_HTTP_ADDR: ':18080',
  COOKIES_MYSQL_DSN: 'cookies:cookies_local_development_only@tcp(127.0.0.1:3306)/cookies?parseTime=true&multiStatements=true',
  COOKIES_LOCAL_ORGANIZATION_ID: 'org_local',
  COOKIES_LOCAL_PRINCIPAL_KIND: 'user',
  COOKIES_LOCAL_PRINCIPAL_ID: 'user_local',
  COOKIES_LOCAL_PROJECT_ID: 'project_local',
  COOKIES_LOCAL_SCOPES: [
    'project.read',
    'project.write',
    'assets.read',
    'assets.write',
    'provider.job.create',
    'provider.text.generate',
    'provider.vision.understand',
  ].join(','),
  COOKIES_BLOB_PROVIDER: 'filesystem',
  COOKIES_FILESYSTEM_BLOB_ROOT: '.data/e2e-platform-blobs',
  COOKIES_SCANNER_MODE: 'noop',
}

export default defineConfig({
  testDir: './e2e',
  testMatch: /platform-go-demo\.spec\.ts/,
  fullyParallel: false,
  use: {
    baseURL: 'http://127.0.0.1:4174',
    trace: 'retain-on-failure',
  },
  webServer: [
    {
      command: 'bash -lc "docker compose up -d --wait mysql && go run ./cmd/cookies-seed && go run ./cmd/cookies-api"',
      url: `${apiBaseURL}/healthz`,
      env: {
        ...process.env,
        ...localGoEnv,
      },
      reuseExistingServer: false,
      timeout: 120_000,
    },
    {
      command: 'npm run dev -- --host 127.0.0.1 --port 4174',
      url: 'http://127.0.0.1:4174',
      env: {
        ...process.env,
        VITE_PLATFORM_PROXY_TARGET: apiBaseURL,
      },
      reuseExistingServer: false,
      timeout: 60_000,
    },
  ],
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
})
