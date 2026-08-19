import { mkdir } from 'node:fs/promises'
import { join } from 'node:path'
import { chromium, type Locator, type Page } from '@playwright/test'

type LocatorSpec = { kind: 'role_name' | 'label' | 'visible_text' | 'placeholder' | 'attribute'; value: string }
type RpaField = { key: string; value?: unknown; locator: LocatorSpec }
type RpaStep = {
  id: string
  kind: 'identify_page' | 'readback' | 'fill_money' | 'final_click'
  page?: string
  page_kind?: string
  fields?: RpaField[]
  locator?: LocatorSpec
  scope_checks?: LocatorSpec[]
  presence_checks?: LocatorSpec[]
  remoteWrite?: boolean
}
type RpaPlan = {
  schemaVersion: 'oceanengine-playwright-rpa/v2'
  browser: 'msedge'
  mode: 'prepare' | 'submit'
  accountId: string
  steps: RpaStep[]
  allowRemoteWrite: boolean
  evidenceRoot?: string
  runId?: string
  allowedProtocols?: string[]
  allowedHosts?: string[]
  expectedObjectId?: string
  objectIdQueryKey?: string
  accountIdQueryKey?: string
}
type StepResult = {
  id: string
  status: 'succeeded' | 'failed' | 'result_unknown'
  before_facts?: Record<string, string>
  readback?: Record<string, string>
  diff_keys?: string[]
  page_reference?: string
  screenshot_path?: string
}
type RpaResult = {
  schema_version: 'oceanengine-playwright-rpa-result/v1'
  outcome: 'success' | 'failed' | 'partial' | 'result_unknown'
  error_code: string
  error_message?: string
  final_click_performed: boolean
  steps: StepResult[]
}

class RpaError extends Error {
  constructor(readonly code: string, message: string) {
    super(message)
  }
}

function semanticLocator(page: Page, spec: LocatorSpec): Locator {
  if (spec.kind === 'label') return page.getByLabel(spec.value, { exact: true })
  if (spec.kind === 'visible_text') return page.getByText(spec.value, { exact: true })
  if (spec.kind === 'placeholder') return page.getByPlaceholder(spec.value, { exact: true })
  if (spec.kind === 'attribute') {
    const [name, value] = spec.value.split('=', 2)
    if (!name || !value) throw new RpaError('page_drift', `Invalid attribute locator: ${spec.value}`)
    return page.locator(`[${name}=${JSON.stringify(value)}]`)
  }
  const [role, name] = spec.value.split(':', 2)
  if (!role || !name) throw new RpaError('page_drift', `Invalid role locator: ${spec.value}`)
  return page.getByRole(role as Parameters<Page['getByRole']>[0], { name, exact: true })
}

async function requireScope(page: Page, step: RpaStep) {
  for (const spec of step.scope_checks ?? []) {
    const target = semanticLocator(page, spec)
    if ((await target.count()) < 1) {
      throw new RpaError('page_drift', `${step.id}: scope locator ${spec.kind}:${spec.value} is not present`)
    }
  }
}

async function recordPresence(page: Page, step: RpaStep, readback: Record<string, string>) {
  for (const spec of step.presence_checks ?? []) {
    const target = semanticLocator(page, spec)
    if ((await target.count()) < 1) {
      throw new RpaError('page_drift', `${step.id}: expected page fact ${spec.kind}:${spec.value} is missing`)
    }
    readback[`presence:${spec.value}`] = 'true'
  }
}

async function readFieldValue(page: Page, field: RpaField): Promise<string> {
  const target = semanticLocator(page, field.locator)
  const count = await target.count()
  if (count !== 1) {
    throw new RpaError('locator_not_unique', `${field.key}: locator matched ${count} elements`)
  }
  const tagName = String(await target.evaluate(element => element.tagName)).toLowerCase()
  if (tagName === 'input' || tagName === 'textarea') {
    const handle = await target.elementHandle()
    const value = handle ? await handle.inputValue() : ''
    return value
  }
  const role = await target.getAttribute('role')
  if (role === 'spinbutton') {
    const value = await target.getAttribute('aria-valuenow') ?? await target.innerText()
    return value.trim()
  }
  return (await target.innerText()).trim()
}

async function fillField(page: Page, field: RpaField) {
  const target = semanticLocator(page, field.locator)
  if ((await target.count()) !== 1) {
    throw new RpaError('locator_not_unique', `${field.key}: fill locator did not match exactly one element`)
  }
  const value = field.value
  if (typeof value === 'boolean') await target.setChecked(value)
  else if (typeof value === 'string' || typeof value === 'number') {
    const tagName = String(await target.evaluate(element => element.tagName)).toLowerCase()
    if (tagName === 'select') await target.selectOption({ label: String(value) })
    else await target.fill(String(value))
  } else throw new RpaError('page_drift', `${field.key}: unsupported fill value`)
}

async function screenshot(page: Page, plan: RpaPlan, stepId: string, results: StepResult[]): Promise<string | undefined> {
  if (!plan.evidenceRoot) return undefined
  try {
    await mkdir(plan.evidenceRoot, { recursive: true })
    const path = join(plan.evidenceRoot, `${plan.runId ?? 'run'}-${stepId}-${results.length}.png`)
    await page.screenshot({ path })
    return path
  } catch {
    return undefined
  }
}

async function executeStep(page: Page, plan: RpaPlan, step: RpaStep, results: StepResult[]): Promise<StepResult> {
  const result: StepResult = { id: step.id, status: 'succeeded', readback: {} }
  const url = new URL(page.url())
  result.page_reference = `${url.protocol}//${url.host}${url.pathname}`

  if (step.kind === 'identify_page') {
    const protocols = plan.allowedProtocols ?? ['https:']
    const hosts = (plan.allowedHosts ?? []).map(host => host.toLowerCase())
    const normalizedProtocol = url.protocol.endsWith(':') ? url.protocol : `${url.protocol}:`
    if (!protocols.some(allowed => allowed === url.protocol || `${allowed}:` === normalizedProtocol)) {
      throw new RpaError('page_drift', `page protocol ${url.protocol} is not allowed`)
    }
    if (hosts.length > 0 && !hosts.includes(url.hostname.toLowerCase())) {
      throw new RpaError('page_drift', `page host ${url.hostname} is not allowed`)
    }
    const accountKey = plan.accountIdQueryKey ?? 'aadvid'
    const observedAccount = url.searchParams.get(accountKey) ?? ''
    result.before_facts = { page_kind: step.page_kind ?? '', url_host: url.hostname, [accountKey]: observedAccount }
    if (observedAccount && observedAccount !== plan.accountId) {
      throw new RpaError('account_mismatch', `observed account ${observedAccount} does not match the authorized account ${plan.accountId}`)
    }
    if (!observedAccount && plan.mode === 'submit') {
      throw new RpaError('account_mismatch', `page URL does not expose the ${accountKey} account identifier; refusing to enter the write path`)
    }
    const objectKey = plan.objectIdQueryKey ?? ''
    if (objectKey) {
      const observedObject = url.searchParams.get(objectKey) ?? ''
      if (observedObject) result.readback['object_id'] = observedObject
      if (observedObject && plan.expectedObjectId && observedObject !== plan.expectedObjectId) {
        throw new RpaError('page_drift', `observed object ${observedObject} does not match the authorized object ${plan.expectedObjectId}`)
      }
    }
  } else if (step.kind === 'readback') {
    await requireScope(page, step)
    for (const field of step.fields ?? []) {
      result.readback![field.key] = await readFieldValue(page, field)
    }
    await recordPresence(page, step, result.readback!)
  } else if (step.kind === 'fill_money') {
    await requireScope(page, step)
    for (const field of step.fields ?? []) await fillField(page, field)
    for (const field of step.fields ?? []) {
      result.readback![field.key] = await readFieldValue(page, field)
    }
  } else if (step.kind === 'final_click') {
    if (!step.remoteWrite || !plan.allowRemoteWrite) {
      throw new RpaError('write_blocked', `${step.id}: remote write is not authorized`)
    }
    if (!step.locator) throw new RpaError('page_drift', `${step.id}: final click needs a locator`)
    const currentHost = new URL(page.url()).hostname.toLowerCase()
    const allowedHosts = (plan.allowedHosts ?? []).map(host => host.toLowerCase())
    if (allowedHosts.length > 0 && !allowedHosts.includes(currentHost)) {
      throw new RpaError('page_drift', `${step.id}: page drifted to ${currentHost} before the write boundary`)
    }
    const target = semanticLocator(page, step.locator)
    if ((await target.count()) !== 1) {
      throw new RpaError('locator_not_unique', `${step.id}: final write boundary did not match exactly one element`)
    }
    // Exactly one click, never retried. Once the click is attempted, any
    // error (including timeout) is result uncertainty: the dispatch may have
    // reached the platform even though the acknowledgment was lost.
    try {
      await target.click({ timeout: 15000 })
    } catch (error) {
      throw new RpaError('click_uncertain', `${step.id}: click outcome is uncertain: ${error instanceof Error ? error.message : error}`)
    }
    result.readback!['final_click'] = 'performed'
    await page.waitForLoadState('domcontentloaded', { timeout: 15000 }).catch(() => undefined)
    const after = new URL(page.url())
    result.readback!['after_url_host'] = after.hostname
    result.page_reference = `${after.protocol}//${after.host}${after.pathname}`
  } else {
    throw new RpaError('page_drift', `unsupported step kind ${step.kind}`)
  }

  result.screenshot_path = await screenshot(page, plan, step.id, results)
  return result
}

async function executePlan(plan: RpaPlan, cdpURL: string): Promise<RpaResult> {
  const steps: StepResult[] = []
  let finalClickPerformed = false
  let browser
  try {
    browser = await chromium.connectOverCDP(cdpURL)
  } catch (error) {
    return { schema_version: 'oceanengine-playwright-rpa-result/v1', outcome: 'failed', error_code: 'cdp_unavailable', error_message: String(error), final_click_performed: false, steps }
  }
  try {
    const context = browser.contexts()[0]
    if (!context) throw new RpaError('cdp_unavailable', 'the external Edge session has no browser context')
    const page = context.pages()[0] ?? await context.newPage()
    for (const step of plan.steps) {
      const result = await executeStep(page, plan, step, steps)
      steps.push(result)
      if (step.kind === 'final_click') finalClickPerformed = true
    }
    return { schema_version: 'oceanengine-playwright-rpa-result/v1', outcome: 'success', error_code: 'ok', final_click_performed: finalClickPerformed, steps }
  } catch (error) {
    const code = error instanceof RpaError ? error.code : 'internal'
    const clickUncertain = error instanceof RpaError && error.code === 'click_uncertain'
    const performed = finalClickPerformed || clickUncertain
    const outcome = performed ? 'result_unknown' : 'failed'
    return { schema_version: 'oceanengine-playwright-rpa-result/v1', outcome, error_code: code, error_message: String(error instanceof Error ? error.message : error), final_click_performed: performed, steps }
  } finally {
    // close() on a CDP connection only detaches; the browser session belongs
    // to the operator and keeps running.
    await browser.close().catch(() => undefined)
  }
}

async function main() {
  const cdpURL = process.argv[2]
  if (!cdpURL) throw new Error('Usage: echo PLAN.json | tsx scripts/browser-rpa-runner.ts CDP_URL')
  const raw = await new Promise<string>((resolvePromise, rejectPromise) => {
    let data = ''
    process.stdin.setEncoding('utf8')
    process.stdin.on('data', chunk => { data += chunk })
    process.stdin.on('end', () => resolvePromise(data))
    process.stdin.on('error', rejectPromise)
  })
  const plan = JSON.parse(raw) as RpaPlan
  if (plan.schemaVersion !== 'oceanengine-playwright-rpa/v2' || plan.browser !== 'msedge') {
    const result: RpaResult = { schema_version: 'oceanengine-playwright-rpa-result/v1', outcome: 'failed', error_code: 'internal', error_message: 'invalid plan schema', final_click_performed: false, steps: [] }
    process.stdout.write(JSON.stringify(result))
    return
  }
  const result = await executePlan(plan, cdpURL)
  process.stdout.write(JSON.stringify(result))
}

if (import.meta.url === `file://${process.argv[1]?.replaceAll('\\', '/')}`) {
  main().catch(error => {
    const result: RpaResult = { schema_version: 'oceanengine-playwright-rpa-result/v1', outcome: 'failed', error_code: 'internal', error_message: String(error), final_click_performed: false, steps: [] }
    process.stdout.write(JSON.stringify(result))
    process.exitCode = 1
  })
}
