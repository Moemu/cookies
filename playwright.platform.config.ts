import { defineConfig, devices } from '@playwright/test'

const apiBaseURL = 'http://127.0.0.1:18080'
const mysqlBootstrap = `docker compose -f deployments/docker-compose.yml up -d --wait mysql && docker compose -f deployments/docker-compose.yml exec -T mysql mysql -uroot -proot_local_development_only -e 'CREATE DATABASE IF NOT EXISTS cookies_e2e CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci'`
const mysqlCommand = process.env.COOKIES_E2E_SKIP_MYSQL_BOOTSTRAP === 'true'
  ? 'node -e ""'
  : process.platform === 'win32'
    ? `wsl.exe -d Ubuntu-24.04 --cd "${process.cwd()}" bash -lc "${mysqlBootstrap}"`
    : mysqlBootstrap
const localChromiumExecutable = process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH
const reuseE2EServers = process.env.COOKIES_E2E_REUSE_SERVERS === 'true'
const apiExecutable = process.platform === 'win32'
  ? '.cache\\runtime\\cookies-api-e2e.exe'
  : '.cache/runtime/cookies-api-e2e'
const runApiExecutable = process.platform === 'win32' ? `"${apiExecutable}"` : `./${apiExecutable}`

const localGoEnv = {
  COOKIES_ENV: 'local',
  COOKIES_PASSWORD_AUTH_ENABLED: 'false',
  COOKIES_HTTP_ADDR: ':18080',
  COOKIES_MYSQL_DSN: 'root:root_local_development_only@tcp(127.0.0.1:3307)/cookies_e2e?parseTime=true&multiStatements=true',
  COOKIES_LOCAL_ORGANIZATION_ID: 'org_local',
  COOKIES_LOCAL_PRINCIPAL_KIND: 'user',
  COOKIES_LOCAL_PRINCIPAL_ID: 'user_local',
  COOKIES_LOCAL_PROJECT_ID: 'project_local',
  COOKIES_LOCAL_SCOPES: [
    'project.read',
    'project.write',
    'assets.read',
    'assets.write',
    'delivery.read',
    'delivery.write',
    'delivery.approve',
    'delivery.execute',
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
  testMatch: /(platform-go-demo|delivery-plan-preflight|delivery-approval-content-hash|delivery-execution-scenarios|delivery-monitoring-alerts)\.spec\.ts/,
  fullyParallel: false,
  workers: 1,
  use: {
    baseURL: 'http://127.0.0.1:4174',
    trace: 'retain-on-failure',
  },
  webServer: [
    {
      command: `${mysqlCommand} && go run ./cmd/cookies-migrate && go run ./cmd/cookies-seed && node -e "require('fs').mkdirSync('.cache/runtime',{recursive:true})" && go build -o "${apiExecutable}" ./cmd/cookies-api && ${runApiExecutable}`,
      url: `${apiBaseURL}/healthz`,
      env: {
        ...process.env,
        ...localGoEnv,
      },
      reuseExistingServer: reuseE2EServers,
      timeout: 120_000,
    },
    {
      command: 'node node_modules/vite/bin/vite.js --host 127.0.0.1 --port 4174',
      url: 'http://127.0.0.1:4174',
      env: {
        ...process.env,
        VITE_PLATFORM_PROXY_TARGET: apiBaseURL,
      },
      reuseExistingServer: reuseE2EServers,
      timeout: 60_000,
    },
  ],
  projects: [{
    name: 'chromium',
    use: {
      ...devices['Desktop Chrome'],
      launchOptions: localChromiumExecutable ? { executablePath: localChromiumExecutable } : undefined,
    },
  }],
})
