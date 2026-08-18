import { readFile } from 'node:fs/promises'
import { chromium, type Locator, type Page } from '@playwright/test'

type LocatorSpec = { kind: 'role_name' | 'label' | 'visible_text' | 'placeholder' | 'attribute'; value: string }
type RpaField = { key: string; value: unknown; locator: LocatorSpec; expectedReadback?: unknown }
type RpaStep = { id: string; page: string; action: string; fields: RpaField[]; remoteWrite: boolean }
type RpaPlan = {
  schemaVersion: 'oceanengine-playwright-rpa/v1'
  browser: 'msedge'
  accountId: string
  steps: RpaStep[]
  allowRemoteWrite: boolean
}

function semanticLocator(page: Page, spec: LocatorSpec): Locator {
  if (spec.kind === 'label') return page.getByLabel(spec.value, { exact: true })
  if (spec.kind === 'visible_text') return page.getByText(spec.value, { exact: true })
  if (spec.kind === 'placeholder') return page.getByPlaceholder(spec.value, { exact: true })
  if (spec.kind === 'attribute') {
    const [name, value] = spec.value.split('=', 2)
    if (!name || !value) throw new Error(`Invalid attribute locator: ${spec.value}`)
    return page.locator(`[${name}=${JSON.stringify(value)}]`)
  }
  const [role, name] = spec.value.split(':', 2)
  if (!role || !name) throw new Error(`Invalid role locator: ${spec.value}`)
  return page.getByRole(role as Parameters<Page['getByRole']>[0], { name, exact: true })
}

async function applyField(page: Page, field: RpaField) {
  const target = semanticLocator(page, field.locator)
  if (await target.count() !== 1) throw new Error(`${field.key}: semantic locator did not match exactly one element`)
  if (typeof field.value === 'boolean') await target.setChecked(field.value)
  else if (Array.isArray(field.value)) {
    for (const value of field.value) await target.selectOption({ label: String(value) })
  } else if (typeof field.value === 'string' || typeof field.value === 'number') {
    const tag = await target.evaluate(element => element.tagName)
    if (tag === 'SELECT') await target.selectOption({ label: String(field.value) })
    else await target.fill(String(field.value))
  } else throw new Error(`${field.key}: unsupported deterministic value`)
}

export async function runOceanEngineRpa(plan: RpaPlan, cdpURL: string) {
  const browser = await chromium.connectOverCDP(cdpURL)
  const context = browser.contexts()[0]
  if (!context) throw new Error('The external Edge session is not available')
  const page = context.pages()[0] ?? await context.newPage()
  for (const step of plan.steps) {
    if (step.remoteWrite && !plan.allowRemoteWrite) throw new Error(`${step.id}: remote write is not authorized`)
    await page.goto(step.page, { waitUntil: 'domcontentloaded' })
    for (const field of step.fields) await applyField(page, field)
    if (step.remoteWrite) throw new Error(`${step.id}: write actions require the production authority adapter`)
  }
}

async function main() {
  const [planPath, cdpURL] = process.argv.slice(2)
  if (!planPath || !cdpURL) throw new Error('Usage: tsx scripts/oceanengine-playwright-rpa.ts PLAN.json CDP_URL')
  const plan = JSON.parse(await readFile(planPath, 'utf8')) as RpaPlan
  if (plan.schemaVersion !== 'oceanengine-playwright-rpa/v1' || plan.browser !== 'msedge') throw new Error('Invalid RPA plan')
  await runOceanEngineRpa(plan, cdpURL)
}

if (import.meta.url === `file://${process.argv[1]?.replaceAll('\\', '/')}`) void main()
