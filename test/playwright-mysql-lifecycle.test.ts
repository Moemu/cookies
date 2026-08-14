import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const config = readFileSync(new URL('../playwright.platform.config.ts', import.meta.url), 'utf8')

test('platform Playwright isolates and resets its bounded test database', () => {
  assert.match(config, /cookies_e2e_\$\{apiPort\}/)
  assert.match(config, /DROP DATABASE IF EXISTS \$\{mysqlDatabase\}/)
  assert.match(config, /CREATE DATABASE \$\{mysqlDatabase\}/)
  assert.match(config, /COOKIES_E2E_SKIP_MYSQL_BOOTSTRAP/)
  assert.match(config, /COOKIES_E2E_REUSE_SERVERS/)
})
