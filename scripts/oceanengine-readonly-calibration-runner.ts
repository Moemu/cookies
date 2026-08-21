import { createHash } from 'node:crypto'
import { appendFile, mkdir, readFile, rename, writeFile } from 'node:fs/promises'
import { basename, join, resolve } from 'node:path'
import { chromium, type Locator, type Page } from '@playwright/test'

export const planSchemaVersion = 'oceanengine-readonly-calibration-plan/v1' as const
export const resultSchemaVersion = 'oceanengine-readonly-calibration-result/v1' as const

const allowedHost = 'ad.oceanengine.com'
const accountQueryKey = 'aadvid'
const safeID = /^[a-z][a-z0-9._-]{0,63}$/
const hashPattern = /^[0-9a-f]{64}$/
const pagePathPrefixes = { promotion_list: ['/promotion/promote-manage/project'] } as const
const textVocabulary = {
  create_project: '新建项目',
  project_budget_column: '项目预算',
  project_section: '项目',
} as const
const attributeVocabulary = {
  promotion_operational_state: 'promotion_promotion_on-off',
} as const

type TextKey = keyof typeof textVocabulary
type AttributeKey = keyof typeof attributeVocabulary
type PageKind = keyof typeof pagePathPrefixes
type StepKind = 'identify_page' | 'scope_check' | 'locator_unique' | 'presence_check' | 'readback'
type ValueSource = 'text' | 'input_value' | 'aria_checked' | 'aria_value_now'
type ErrorCode = 'ok' | 'invalid_plan' | 'cdp_unavailable' | 'host_not_allowed' | 'account_context_missing' | 'account_mismatch' | 'page_type_mismatch' | 'scope_missing' | 'locator_missing' | 'locator_not_unique' | 'readback_failed' | 'internal'
type FailureStage = 'session_metadata' | 'playwright_connect' | 'browser_context' | 'allowed_page' | 'target_info'

export type SemanticLocator =
  | { kind: 'visible_text'; name_key: TextKey }
  | { kind: 'role_name'; role: 'button' | 'heading' | 'spinbutton' | 'textbox'; name_key: TextKey }
  | { kind: 'attribute'; attribute_key: 'data-e2e'; value_key: AttributeKey }

type IdentifyStep = { id: string; kind: 'identify_page'; page_kind: PageKind }
type LocatorStep = { id: string; kind: 'scope_check' | 'locator_unique' | 'presence_check'; locator: SemanticLocator }
type ReadbackStep = { id: string; kind: 'readback'; locator: SemanticLocator; value_source: ValueSource }
export type CalibrationStep = IdentifyStep | LocatorStep | ReadbackStep

export type CalibrationPlan = {
  schema_version: typeof planSchemaVersion
  plan_id: string
  platform: 'ocean_engine'
  browser: 'msedge'
  allowed_hosts: [typeof allowedHost]
  account_context: { source: 'url_query_sha256'; query_key: typeof accountQueryKey; expected_sha256: string }
  steps: CalibrationStep[]
}

type SafeLocator = { kind: SemanticLocator['kind']; key: TextKey | AttributeKey }
type StepResult = {
  id: string
  kind: StepKind
  status: 'passed' | 'failed'
  locator?: SafeLocator
  element_count?: number
  visible?: boolean
  accessible_name_state?: 'empty' | 'present'
  accessible_name_sha256?: string
  value_state?: 'empty' | 'present'
  value_kind?: 'text' | 'boolean' | 'number'
  value_sha256?: string
}

export type CalibrationResult = {
  schema_version: typeof resultSchemaVersion
  plan_sha256: string
  observed_at: string
  outcome: 'success' | 'failed' | 'rejected'
  error_code: ErrorCode
  browser_detached: true
  page?: {
    host: typeof allowedHost
    page_kind: 'promotion_list'
    path_sha256: string
    account_context_sha256: string
    account_context_state: 'matched'
    session_context_sha256: string
  }
  structure_summary?: {
    main_count: number
    heading_count: number
    button_count: number
    textbox_count: number
    table_count: number
    row_count: number
  }
  steps: StepResult[]
}

export interface ReadonlyLocator {
  count(): Promise<number>
  isVisible(): Promise<boolean>
  ariaSnapshot(): Promise<string>
  textContent(): Promise<string | null>
  inputValue(): Promise<string>
  getAttribute(name: string): Promise<string | null>
}

export interface ReadonlyPage {
  url(): string
  locate(spec: SemanticLocator): ReadonlyLocator
  countRole(role: 'main' | 'heading' | 'button' | 'textbox' | 'table' | 'row'): Promise<number>
}

class CalibrationError extends Error {
  constructor(readonly code: ErrorCode, readonly stage?: FailureStage) {
    super(code)
  }
}

function sha256(value: string | Buffer) {
  return createHash('sha256').update(value).digest('hex')
}

function canonicalJSON(value: unknown): string {
  if (Array.isArray(value)) return `[${value.map(canonicalJSON).join(',')}]`
  if (value && typeof value === 'object') {
    return `{${Object.entries(value as Record<string, unknown>).sort(([left], [right]) => left.localeCompare(right)).map(([key, item]) => `${JSON.stringify(key)}:${canonicalJSON(item)}`).join(',')}}`
  }
  return JSON.stringify(value)
}

function objectValue(value: unknown, label: string): Record<string, unknown> {
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw new CalibrationError('invalid_plan')
  return value as Record<string, unknown>
}

function exactKeys(value: Record<string, unknown>, required: string[], optional: string[] = []) {
  const allowed = new Set([...required, ...optional])
  if (required.some(key => !(key in value)) || Object.keys(value).some(key => !allowed.has(key))) throw new CalibrationError('invalid_plan')
}

function stringValue(value: unknown, expected?: string) {
  if (typeof value !== 'string' || (expected !== undefined && value !== expected)) throw new CalibrationError('invalid_plan')
  return value
}

function validateLocator(input: unknown): SemanticLocator {
  const value = objectValue(input, 'locator')
  const kind = stringValue(value.kind)
  if (kind === 'visible_text') {
    exactKeys(value, ['kind', 'name_key'])
    const nameKey = stringValue(value.name_key) as TextKey
    if (!(nameKey in textVocabulary)) throw new CalibrationError('invalid_plan')
    return { kind, name_key: nameKey }
  }
  if (kind === 'role_name') {
    exactKeys(value, ['kind', 'role', 'name_key'])
    const role = stringValue(value.role) as 'button' | 'heading' | 'spinbutton' | 'textbox'
    const nameKey = stringValue(value.name_key) as TextKey
    if (!['button', 'heading', 'spinbutton', 'textbox'].includes(role) || !(nameKey in textVocabulary)) throw new CalibrationError('invalid_plan')
    return { kind, role, name_key: nameKey }
  }
  if (kind === 'attribute') {
    exactKeys(value, ['kind', 'attribute_key', 'value_key'])
    stringValue(value.attribute_key, 'data-e2e')
    const valueKey = stringValue(value.value_key) as AttributeKey
    if (!(valueKey in attributeVocabulary)) throw new CalibrationError('invalid_plan')
    return { kind, attribute_key: 'data-e2e', value_key: valueKey }
  }
  throw new CalibrationError('invalid_plan')
}

export function validateCalibrationPlan(input: unknown): CalibrationPlan {
  const value = objectValue(input, 'plan')
  exactKeys(value, ['schema_version', 'plan_id', 'platform', 'browser', 'allowed_hosts', 'account_context', 'steps'])
  stringValue(value.schema_version, planSchemaVersion)
  const planID = stringValue(value.plan_id)
  if (!safeID.test(planID)) throw new CalibrationError('invalid_plan')
  stringValue(value.platform, 'ocean_engine')
  stringValue(value.browser, 'msedge')
  if (!Array.isArray(value.allowed_hosts) || value.allowed_hosts.length !== 1 || value.allowed_hosts[0] !== allowedHost) throw new CalibrationError('invalid_plan')
  const account = objectValue(value.account_context, 'account_context')
  exactKeys(account, ['source', 'query_key', 'expected_sha256'])
  stringValue(account.source, 'url_query_sha256')
  stringValue(account.query_key, accountQueryKey)
  const expectedHash = stringValue(account.expected_sha256)
  if (!hashPattern.test(expectedHash)) throw new CalibrationError('invalid_plan')
  if (!Array.isArray(value.steps) || value.steps.length < 2 || value.steps.length > 32) throw new CalibrationError('invalid_plan')

  const ids = new Set<string>()
  const steps = value.steps.map((inputStep, index): CalibrationStep => {
    const step = objectValue(inputStep, 'step')
    const kind = stringValue(step.kind) as StepKind
    const id = stringValue(step.id)
    if (!safeID.test(id) || ids.has(id)) throw new CalibrationError('invalid_plan')
    ids.add(id)
    if (kind === 'identify_page') {
      exactKeys(step, ['id', 'kind', 'page_kind'])
      stringValue(step.page_kind, 'promotion_list')
      if (index !== 0) throw new CalibrationError('invalid_plan')
      return { id, kind, page_kind: 'promotion_list' }
    }
    if (kind === 'scope_check' || kind === 'locator_unique' || kind === 'presence_check') {
      exactKeys(step, ['id', 'kind', 'locator'])
      return { id, kind, locator: validateLocator(step.locator) }
    }
    if (kind === 'readback') {
      exactKeys(step, ['id', 'kind', 'locator', 'value_source'])
      const source = stringValue(step.value_source) as ValueSource
      if (!['text', 'input_value', 'aria_checked', 'aria_value_now'].includes(source)) throw new CalibrationError('invalid_plan')
      return { id, kind, locator: validateLocator(step.locator), value_source: source }
    }
    throw new CalibrationError('invalid_plan')
  })
  if (steps[0]?.kind !== 'identify_page' || !steps.some(step => step.kind === 'readback')) throw new CalibrationError('invalid_plan')
  return {
    schema_version: planSchemaVersion,
    plan_id: planID,
    platform: 'ocean_engine',
    browser: 'msedge',
    allowed_hosts: [allowedHost],
    account_context: { source: 'url_query_sha256', query_key: accountQueryKey, expected_sha256: expectedHash },
    steps,
  }
}

function safeLocator(spec: SemanticLocator): SafeLocator {
  return { kind: spec.kind, key: spec.kind === 'attribute' ? spec.value_key : spec.name_key }
}

async function structureSummary(page: ReadonlyPage) {
  const [main_count, heading_count, button_count, textbox_count, table_count, row_count] = await Promise.all([
    page.countRole('main'), page.countRole('heading'), page.countRole('button'), page.countRole('textbox'), page.countRole('table'), page.countRole('row'),
  ])
  return { main_count, heading_count, button_count, textbox_count, table_count, row_count }
}

async function locatorFacts(locator: ReadonlyLocator) {
  const count = await locator.count()
  const visible = count > 0 ? await locator.isVisible() : false
  return { count, visible }
}

async function readbackValue(locator: ReadonlyLocator, source: ValueSource) {
  if (source === 'input_value') return { value: await locator.inputValue(), kind: 'text' as const }
  if (source === 'aria_checked') return { value: await locator.getAttribute('aria-checked') ?? '', kind: 'boolean' as const }
  if (source === 'aria_value_now') return { value: await locator.getAttribute('aria-valuenow') ?? '', kind: 'number' as const }
  return { value: await locator.textContent() ?? '', kind: 'text' as const }
}

export async function executeReadonlyPlan(plan: CalibrationPlan, page: ReadonlyPage, sessionContext: string): Promise<CalibrationResult> {
  const result: CalibrationResult = {
    schema_version: resultSchemaVersion,
    plan_sha256: sha256(canonicalJSON(plan)),
    observed_at: new Date().toISOString(),
    outcome: 'failed',
    error_code: 'internal',
    browser_detached: true,
    steps: [],
  }
  try {
    for (const step of plan.steps) {
      if (step.kind === 'identify_page') {
        const url = new URL(page.url())
        if (url.protocol !== 'https:' || !plan.allowed_hosts.includes(url.hostname as typeof allowedHost)) throw new CalibrationError('host_not_allowed')
        if (!pagePathPrefixes[step.page_kind].some(prefix => url.pathname.startsWith(prefix))) throw new CalibrationError('page_type_mismatch')
        const accountValue = url.searchParams.get(plan.account_context.query_key)
        if (!accountValue) throw new CalibrationError('account_context_missing')
        const accountHash = sha256(accountValue)
        if (accountHash !== plan.account_context.expected_sha256) throw new CalibrationError('account_mismatch')
        result.page = {
          host: allowedHost,
          page_kind: step.page_kind,
          path_sha256: sha256(url.pathname),
          account_context_sha256: accountHash,
          account_context_state: 'matched',
          session_context_sha256: sha256(sessionContext),
        }
        result.structure_summary = await structureSummary(page)
        result.steps.push({ id: step.id, kind: step.kind, status: 'passed' })
        continue
      }

      const locator = page.locate(step.locator)
      const facts = await locatorFacts(locator)
      const stepResult: StepResult = { id: step.id, kind: step.kind, status: 'passed', locator: safeLocator(step.locator), element_count: facts.count, visible: facts.visible }
      if (step.kind === 'scope_check' && (facts.count < 1 || !facts.visible)) throw new CalibrationError('scope_missing')
      if (step.kind === 'presence_check' && facts.count < 1) throw new CalibrationError('locator_missing')
      if (step.kind === 'locator_unique' && facts.count !== 1) throw new CalibrationError('locator_not_unique')
      if (step.kind === 'readback') {
        if (facts.count < 1) throw new CalibrationError('locator_missing')
        if (facts.count !== 1) throw new CalibrationError('locator_not_unique')
        let snapshot: string
        let readback: { value: string; kind: 'text' | 'boolean' | 'number' }
        try {
          snapshot = await locator.ariaSnapshot()
          readback = await readbackValue(locator, step.value_source)
        } catch {
          throw new CalibrationError('readback_failed')
        }
        stepResult.accessible_name_state = snapshot.trim() ? 'present' : 'empty'
        if (snapshot.trim()) stepResult.accessible_name_sha256 = sha256(snapshot)
        stepResult.value_state = readback.value ? 'present' : 'empty'
        stepResult.value_kind = readback.kind
        if (readback.value) stepResult.value_sha256 = sha256(readback.value)
      }
      result.steps.push(stepResult)
    }
    result.outcome = 'success'
    result.error_code = 'ok'
    return result
  } catch (error) {
    result.outcome = 'failed'
    result.error_code = error instanceof CalibrationError ? error.code : 'internal'
    const failedStep = plan.steps[result.steps.length]
    if (failedStep) result.steps.push({ id: failedStep.id, kind: failedStep.kind, status: 'failed', ...('locator' in failedStep ? { locator: safeLocator(failedStep.locator) } : {}) })
    return result
  }
}

class PlaywrightReadonlyPage implements ReadonlyPage {
  constructor(private readonly page: Page) {}

  url() { return this.page.url() }

  locate(spec: SemanticLocator): ReadonlyLocator {
    let locator: Locator
    if (spec.kind === 'visible_text') locator = this.page.getByText(textVocabulary[spec.name_key], { exact: true })
    else if (spec.kind === 'role_name') locator = this.page.getByRole(spec.role, { name: textVocabulary[spec.name_key], exact: true })
    else locator = this.page.locator(`[data-e2e=${JSON.stringify(attributeVocabulary[spec.value_key])}]`)
    return locator
  }

  countRole(role: 'main' | 'heading' | 'button' | 'textbox' | 'table' | 'row') {
    return this.page.getByRole(role).count()
  }
}

export function calibrationPaths(localAppData = process.env.LOCALAPPDATA) {
  if (!localAppData) throw new Error('LOCALAPPDATA is not set')
  const root = resolve(localAppData, 'cookies', 'browser-rpa', 'calibration')
  return { root, plan: join(root, 'live-plan.json'), results: join(root, 'results'), diagnostics: join(root, 'diagnostics.jsonl') }
}

type SessionMetadata = { state?: string; mode?: string; cdp_endpoint?: string; profile_path?: string }

async function sessionConnection(localAppData: string) {
  const metadataPath = resolve(localAppData, 'cookies', 'browser-rpa', 'session.json')
  const metadata = JSON.parse(await readFile(metadataPath, 'utf8')) as SessionMetadata
  if (metadata.state !== 'running' || !metadata.cdp_endpoint) throw new CalibrationError('cdp_unavailable')
  const endpoint = new URL(metadata.cdp_endpoint)
  if (endpoint.protocol !== 'http:' || endpoint.hostname !== '127.0.0.1') throw new CalibrationError('cdp_unavailable')
  if (metadata.mode !== 'current_user') return metadata.cdp_endpoint
  if (!metadata.profile_path) throw new CalibrationError('cdp_unavailable')
  const userDataRoot = resolve(metadata.profile_path, '..')
  const lines = (await readFile(join(userDataRoot, 'DevToolsActivePort'), 'utf8')).split(/\r?\n/)
  const port = Number.parseInt(lines[0] ?? '', 10)
  const browserPath = lines[1] ?? ''
  if (port !== Number.parseInt(endpoint.port, 10) || !/^\/devtools\/browser\/[A-Za-z0-9-]+$/.test(browserPath)) throw new CalibrationError('cdp_unavailable')
  return `ws://127.0.0.1:${port}${browserPath}`
}

async function atomicJSON(path: string, value: unknown) {
  const temporary = `${path}.${process.pid}.tmp`
  await writeFile(temporary, `${JSON.stringify(value, null, 2)}\n`, { encoding: 'utf8', mode: 0o600 })
  await rename(temporary, path)
}

async function appendDiagnostic(path: string, command: string, outcome: string, errorCode: ErrorCode | 'ok', failureStage?: FailureStage) {
  await appendFile(path, `${JSON.stringify({ schema_version: 'oceanengine-readonly-calibration-diagnostic/v1', observed_at: new Date().toISOString(), command, outcome, error_code: errorCode, ...(failureStage ? { failure_stage: failureStage } : {}) })}\n`, { encoding: 'utf8', mode: 0o600 })
}

function defaultLivePlan(accountHash: string): CalibrationPlan {
  return {
    schema_version: planSchemaVersion,
    plan_id: 'promotion-list-readonly-live',
    platform: 'ocean_engine',
    browser: 'msedge',
    allowed_hosts: [allowedHost],
    account_context: { source: 'url_query_sha256', query_key: accountQueryKey, expected_sha256: accountHash },
    steps: [
      { id: 'identify-page', kind: 'identify_page', page_kind: 'promotion_list' },
      { id: 'scope-budget', kind: 'scope_check', locator: { kind: 'visible_text', name_key: 'project_budget_column' } },
      { id: 'unique-budget', kind: 'locator_unique', locator: { kind: 'visible_text', name_key: 'project_budget_column' } },
      { id: 'presence-create', kind: 'presence_check', locator: { kind: 'visible_text', name_key: 'create_project' } },
      { id: 'read-budget', kind: 'readback', locator: { kind: 'visible_text', name_key: 'project_budget_column' }, value_source: 'text' },
    ],
  }
}

async function openLivePage(localAppData: string) {
  let endpoint: string
  try {
    endpoint = await sessionConnection(localAppData)
  } catch {
    throw new CalibrationError('cdp_unavailable', 'session_metadata')
  }
  let browser
  try {
    browser = await chromium.connectOverCDP(endpoint, { timeout: 120000 })
  } catch {
    throw new CalibrationError('cdp_unavailable', 'playwright_connect')
  }
  const contexts = browser.contexts()
  if (contexts.length === 0) throw new CalibrationError('cdp_unavailable', 'browser_context')
  const deadline = Date.now() + 30000
  let page: Page | undefined
  do {
    page = contexts.flatMap(context => context.pages()).filter(item => {
      try { return new URL(item.url()).hostname === allowedHost } catch { return false }
    }).at(-1)
    if (!page) await new Promise(resolveTimer => setTimeout(resolveTimer, 250))
  } while (!page && Date.now() < deadline)
  if (!page) throw new CalibrationError('host_not_allowed', 'allowed_page')
  try {
    const pageCDP = await page.context().newCDPSession(page)
    const target = await pageCDP.send('Target.getTargetInfo') as { targetInfo?: { targetId?: string; browserContextId?: string } }
    return { page, sessionContext: sessionContextKey(target.targetInfo) }
  } catch {
    throw new CalibrationError('cdp_unavailable', 'target_info')
  }
}

export function sessionContextKey(targetInfo?: { targetId?: string; browserContextId?: string }) {
  if (!targetInfo?.targetId) throw new CalibrationError('cdp_unavailable')
  return `${targetInfo.browserContextId ?? 'default'}:${targetInfo.targetId}`
}

function rejectedResult(planHash: string, code: ErrorCode): CalibrationResult {
  return { schema_version: resultSchemaVersion, plan_sha256: planHash, observed_at: new Date().toISOString(), outcome: 'rejected', error_code: code, browser_detached: true, steps: [] }
}

async function writeRawResult(paths: ReturnType<typeof calibrationPaths>, result: CalibrationResult) {
  await mkdir(paths.results, { recursive: true })
  const timestamp = result.observed_at.replaceAll(':', '-').replaceAll('.', '-')
  await atomicJSON(join(paths.results, `${timestamp}.json`), result)
  await appendDiagnostic(paths.diagnostics, 'run', result.outcome, result.error_code)
}

async function initCommand(localAppData: string) {
  const paths = calibrationPaths(localAppData)
  await mkdir(paths.root, { recursive: true })
  const { page } = await openLivePage(localAppData)
  const url = new URL(page.url())
  if (url.hostname !== allowedHost || !pagePathPrefixes.promotion_list.some(prefix => url.pathname.startsWith(prefix))) throw new CalibrationError('page_type_mismatch')
  const accountValue = url.searchParams.get(accountQueryKey)
  if (!accountValue) throw new CalibrationError('account_context_missing')
  const plan = defaultLivePlan(sha256(accountValue))
  await atomicJSON(paths.plan, plan)
  await appendDiagnostic(paths.diagnostics, 'init', 'success', 'ok')
  return { outcome: 'initialized' as const, plan_sha256: sha256(canonicalJSON(plan)) }
}

async function runCommand(localAppData: string, planPath?: string) {
  const paths = calibrationPaths(localAppData)
  await mkdir(paths.root, { recursive: true })
  let input: unknown
  let inputHash = '0'.repeat(64)
  try {
    const raw = await readFile(planPath ? resolve(planPath) : paths.plan, 'utf8')
    inputHash = sha256(raw)
    input = JSON.parse(raw)
    const plan = validateCalibrationPlan(input)
    const live = await openLivePage(localAppData)
    const result = await executeReadonlyPlan(plan, new PlaywrightReadonlyPage(live.page), live.sessionContext)
    await writeRawResult(paths, result)
    return result
  } catch (error) {
    const code = error instanceof CalibrationError ? error.code : 'invalid_plan'
    const result = rejectedResult(inputHash, code)
    await writeRawResult(paths, result)
    return result
  }
}

async function main() {
  const localAppData = process.env.LOCALAPPDATA
  if (!localAppData) throw new Error('LOCALAPPDATA is not set')
  const command = process.argv[2]
  if (command === 'init') return await initCommand(localAppData)
  if (command === 'run') return await runCommand(localAppData, process.argv[3])
  throw new Error('Usage: npm run browser-rpa:calibrate -- <init|run> [PLAN_PATH]')
}

if (basename(process.argv[1] ?? '') === 'oceanengine-readonly-calibration-runner.ts') {
  main().then(result => {
    process.stdout.write(`${JSON.stringify(result, null, 2)}\n`, () => process.exit('outcome' in result && result.outcome !== 'success' && result.outcome !== 'initialized' ? 2 : 0))
  }).catch(async error => {
    const code: ErrorCode = error instanceof CalibrationError ? error.code : 'internal'
    const stage = error instanceof CalibrationError ? error.stage : undefined
    try {
      const paths = calibrationPaths()
      await mkdir(paths.root, { recursive: true })
      await appendDiagnostic(paths.diagnostics, process.argv[2] ?? 'unknown', 'failed', code, stage)
    } catch {
      // Keep the primary controlled failure when diagnostics cannot be written.
    }
    process.stderr.write(`${JSON.stringify({ outcome: 'failed', error_code: code, ...(stage ? { failure_stage: stage } : {}) })}\n`, () => process.exit(1))
  })
}
