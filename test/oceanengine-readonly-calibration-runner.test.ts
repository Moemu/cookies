import assert from 'node:assert/strict'
import { createHash } from 'node:crypto'
import { readFileSync } from 'node:fs'
import { join, resolve } from 'node:path'
import test from 'node:test'
import Ajv2020 from 'ajv/dist/2020.js'
import addFormats from 'ajv-formats'
import {
  calibrationPaths,
  executeReadonlyPlan,
  planSchemaVersion,
  resultSchemaVersion,
  sessionContextKey,
  validateCalibrationPlan,
  type CalibrationPlan,
  type ReadonlyLocator,
  type ReadonlyPage,
  type SemanticLocator,
} from '../scripts/oceanengine-readonly-calibration-runner.js'

type Validator = {
  compile(schema: Record<string, unknown>): ((value: unknown) => boolean) & { errors?: unknown }
  errorsText(errors?: unknown): string
}
const AjvConstructor = Ajv2020 as unknown as new (options: { allErrors: boolean; strict: boolean }) => Validator
const installFormats = addFormats as unknown as (validator: Validator) => void

const root = resolve(import.meta.dirname, '..')
const contracts = join(root, 'api', 'contracts')
const fixtures = join(root, 'api', 'fixtures')

function readJSON(path: string): Record<string, unknown> {
  return JSON.parse(readFileSync(path, 'utf8')) as Record<string, unknown>
}

function hash(value: string) {
  return createHash('sha256').update(value).digest('hex')
}

function plan(overrides: Partial<CalibrationPlan> = {}): CalibrationPlan {
  return {
    schema_version: planSchemaVersion,
    plan_id: 'readonly-test',
    platform: 'ocean_engine',
    browser: 'msedge',
    allowed_hosts: ['ad.oceanengine.com'],
    account_context: { source: 'url_query_sha256', query_key: 'aadvid', expected_sha256: hash('account_test') },
    steps: [
      { id: 'identify-page', kind: 'identify_page', page_kind: 'promotion_list' },
      { id: 'scope-budget', kind: 'scope_check', locator: { kind: 'visible_text', name_key: 'project_budget_column' } },
      { id: 'unique-budget', kind: 'locator_unique', locator: { kind: 'visible_text', name_key: 'project_budget_column' } },
      { id: 'presence-create', kind: 'presence_check', locator: { kind: 'visible_text', name_key: 'create_project' } },
      { id: 'read-budget', kind: 'readback', locator: { kind: 'visible_text', name_key: 'project_budget_column' }, value_source: 'text' },
    ],
    ...overrides,
  }
}

class FakeLocator implements ReadonlyLocator {
  constructor(private readonly matches: number, private readonly text = 'sensitive field text') {}
  async count() { return this.matches }
  async isVisible() { return this.matches > 0 }
  async ariaSnapshot() { return this.matches > 0 ? `- columnheader "${this.text}"` : '' }
  async textContent() { return this.matches > 0 ? this.text : null }
  async inputValue() { return this.matches > 0 ? this.text : '' }
  async getAttribute(name: string) {
    if (this.matches < 1) return null
    if (name === 'aria-checked') return 'false'
    if (name === 'aria-valuenow') return '300'
    return null
  }
}

class FakePage implements ReadonlyPage {
  constructor(
    private readonly pageURL = 'https://ad.oceanengine.com/promotion/promote-manage/project/ads?aadvid=account_test',
    private readonly counts: Partial<Record<string, number>> = {},
  ) {}

  url() { return this.pageURL }
  locate(spec: SemanticLocator) {
    const key = spec.kind === 'attribute' ? spec.value_key : spec.name_key
    return new FakeLocator(this.counts[key] ?? 1, key === 'project_budget_column' ? '项目预算' : '新建项目')
  }
  async countRole(role: 'main' | 'heading' | 'button' | 'textbox' | 'table' | 'row') {
    return { main: 1, heading: 2, button: 4, textbox: 1, table: 1, row: 2 }[role]
  }
}

test('versioned Plan and Result fixtures satisfy independent contracts', () => {
  const ajv = new AjvConstructor({ allErrors: true, strict: true })
  installFormats(ajv)
  for (const [schemaName, fixtureName] of [
    ['oceanengine-readonly-calibration-plan-v1.schema.json', 'oceanengine-readonly-calibration-plan-v1.json'],
    ['oceanengine-readonly-calibration-result-v1.schema.json', 'oceanengine-readonly-calibration-result-v1.json'],
  ] as const) {
    const schema = readJSON(join(contracts, schemaName))
    const validate = ajv.compile(schema)
    assert.equal(validate(readJSON(join(fixtures, fixtureName))), true, ajv.errorsText(validate.errors))
  }
})

test('protocol rejects malicious actions, unknown steps, and remoteWrite', () => {
  for (const mutation of [
    (value: Record<string, unknown>) => { value.remoteWrite = true },
    (value: Record<string, unknown>) => { (value.steps as Array<Record<string, unknown>>)[1] = { id: 'attack', kind: 'click', locator: { kind: 'visible_text', name_key: 'create_project' } } },
    (value: Record<string, unknown>) => { (value.steps as Array<Record<string, unknown>>)[1].press = 'Enter' },
  ]) {
    const value = structuredClone(plan()) as unknown as Record<string, unknown>
    mutation(value)
    assert.throws(() => validateCalibrationPlan(value), /invalid_plan/)
  }
  const prototypePlan = JSON.parse(JSON.stringify(plan()).replace('{', '{"__proto__":{"remoteWrite":true},')) as unknown
  assert.throws(() => validateCalibrationPlan(prototypePlan), /invalid_plan/)
})

test('contract rejects remoteWrite even when other Plan fields are valid', () => {
  const ajv = new AjvConstructor({ allErrors: true, strict: true })
  const validate = ajv.compile(readJSON(join(contracts, 'oceanengine-readonly-calibration-plan-v1.schema.json')))
  const value = structuredClone(plan()) as unknown as Record<string, unknown>
  value.remoteWrite = true
  assert.equal(validate(value), false)
})

test('runner rejects a non-allowed host before locator access', async () => {
  const result = await executeReadonlyPlan(plan(), new FakePage('https://evil.example/promotion/promote-manage/project/ads?aadvid=account_test'), 'session')
  assert.equal(result.outcome, 'failed')
  assert.equal(result.error_code, 'host_not_allowed')
})

test('runner rejects an account mismatch without returning the account ID', async () => {
  const result = await executeReadonlyPlan(plan(), new FakePage('https://ad.oceanengine.com/promotion/promote-manage/project/ads?aadvid=another_account'), 'session')
  assert.equal(result.error_code, 'account_mismatch')
  assert.doesNotMatch(JSON.stringify(result), /another_account|aadvid/)
})

test('runner rejects a duplicate locator match', async () => {
  const result = await executeReadonlyPlan(plan(), new FakePage(undefined, { project_budget_column: 2 }), 'session')
  assert.equal(result.error_code, 'locator_not_unique')
  assert.equal(result.steps.at(-1)?.status, 'failed')
})

test('runner reads counts, visibility, accessible name, value, and structure as safe facts', async () => {
  const result = await executeReadonlyPlan(plan(), new FakePage(), 'context:target')
  assert.equal(result.schema_version, resultSchemaVersion)
  assert.equal(result.outcome, 'success')
  assert.equal(result.error_code, 'ok')
  assert.equal(result.page?.account_context_state, 'matched')
  assert.equal(result.structure_summary?.table_count, 1)
  const readback = result.steps.find(step => step.kind === 'readback')
  assert.equal(readback?.element_count, 1)
  assert.equal(readback?.visible, true)
  assert.equal(readback?.accessible_name_state, 'present')
  assert.equal(readback?.value_state, 'present')
  assert.match(readback?.accessible_name_sha256 ?? '', /^[0-9a-f]{64}$/)
  assert.match(readback?.value_sha256 ?? '', /^[0-9a-f]{64}$/)
  const serialized = JSON.stringify(result)
  assert.doesNotMatch(serialized, /account_test|项目预算|新建项目|\?aadvid/)
})

test('default CDP BrowserContext does not require browserContextId', () => {
  assert.equal(sessionContextKey({ targetId: 'target-1' }), 'default:target-1')
  assert.equal(sessionContextKey({ targetId: 'target-1', browserContextId: 'context-1' }), 'context-1:target-1')
  assert.throws(() => sessionContextKey({}), /cdp_unavailable/)
})

test('calibration artifacts always resolve below LOCALAPPDATA browser-rpa calibration', () => {
  const localAppData = resolve('C:/Users/test/AppData/Local')
  const paths = calibrationPaths(localAppData)
  const expected = join(localAppData, 'cookies', 'browser-rpa', 'calibration')
  assert.equal(paths.root, expected)
  for (const path of Object.values(paths)) assert.ok(path.startsWith(expected))
})

test('implementation has no page mutation, script evaluation, clipboard, download, or browser close call', () => {
  const source = readFileSync(join(root, 'scripts', 'oceanengine-readonly-calibration-runner.ts'), 'utf8')
  for (const forbidden of [
    /\.click\s*\(/,
    /\.fill\s*\(/,
    /\.press\s*\(/,
    /\.selectOption\s*\(/,
    /\.setChecked\s*\(/,
    /\.dragTo\s*\(/,
    /\.setInputFiles\s*\(/,
    /\.evaluate\s*\(/,
    /clipboard/i,
    /\.waitForEvent\s*\(\s*['"]download/,
    /browser\.close\s*\(/,
    /Browser\.close/,
  ]) assert.doesNotMatch(source, forbidden)
  assert.match(source, /chromium\.connectOverCDP\s*\(/)
  assert.match(source, /process\.exit\s*\(/)
})

test('installed Playwright permits the default Context and does not wait for unrelated pages', () => {
  const packageJSON = JSON.parse(readFileSync(join(root, 'package.json'), 'utf8')) as { scripts?: Record<string, string> }
  const patchSource = readFileSync(join(root, 'scripts', 'patch-playwright-default-context.mjs'), 'utf8')
  const coreSource = readFileSync(join(root, 'node_modules', 'playwright-core', 'lib', 'coreBundle.js'), 'utf8')
  assert.equal(packageJSON.scripts?.postinstall, 'node scripts/patch-playwright-default-context.mjs')
  assert.match(patchSource, /packageMetadata\.version !== '1\.62\.1'/)
  assert.doesNotMatch(coreSource, /assert\(targetInfo\.browserContextId/)
  assert.doesNotMatch(coreSource, /await browser\._waitForAllPagesToBeInitialized\(\)/)
})
