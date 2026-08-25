import { createHash } from 'node:crypto'
import { mkdir, readFile, rename, rm, writeFile } from 'node:fs/promises'
import { basename, join, resolve } from 'node:path'
import { chromium, type Locator, type Page } from '@playwright/test'

// v1 is frozen history: it is only read to upgrade stored ledgers. All new
// ledgers are written as v2, whose contract separates create/edit pages and
// confirms shells and fields at different levels.
export const coverageSchemaVersion = 'oceanengine-readonly-coverage-ledger/v1' as const
export const coverageSchemaVersionV2 = 'oceanengine-readonly-coverage-ledger/v2' as const
export const runPointerSchema = 'oceanengine-readonly-coverage-run/v1' as const
const observationSchemaVersion = 'oceanengine-readonly-coverage-observation/v1' as const
const allowedHost = 'ad.oceanengine.com' as const
const accountQueryKey = 'aadvid'
const hashPattern = /^[0-9a-f]{64}$/

const textLocators = {
  data_center: '数据中心',
  account_report: '账户报表',
  project_report: '项目报表',
  promotion_report: '单元报表',
  material_report: '基础素材报表',
  create_project: '新建项目',
  project_budget_column: '项目预算',
  create_promotion: '新建单元',
  promotion_budget_column: '单元预算',
  edit_project: '编辑项目',
  project_marketing_purpose: '营销目的',
  save_and_close: '保存并关闭',
  material_overview: '素材概览',
} as const

const attributeLocators = {
  project_deep_optimization: '[data-e2e=createproject_deepbidtype]',
  project_name: '[data-e2e=createproject_projectname] textarea',
  promotion_budget: '[data-e2e=createad_adBudget_input_component] input',
  promotion_name: '[data-e2e=createad_adName] textarea',
  promotion_operational_state: '[data-e2e=promotion_promotion_on-off]',
} as const

type TextLocatorKey = keyof typeof textLocators
type AttributeLocatorKey = keyof typeof attributeLocators
type LocatorKey = TextLocatorKey | AttributeLocatorKey
export type AccountContextTarget = 'account_context'
export type PageTarget = 'project_list' | 'project_create' | 'project_edit' | 'promotion_list' | 'promotion_create' | 'promotion_edit' | 'material_relation_summary' | 'report_overview'
export type LegacyFormTarget = 'project_detail_or_edit' | 'promotion_detail_or_edit'
export type PageKind = PageTarget | LegacyFormTarget | 'unknown'
type CheckState = 'passed' | 'failed' | 'not_run' | 'not_applicable'
export type CoverageStatus = 'confirmed_shell' | 'confirmed_fields' | 'blocked' | 'not_accessible'
type DriftState = 'stable' | 'page_drift' | 'verification_pending'

type LocatorObservation = {
  key: LocatorKey
  kind: 'visible_text' | 'stable_test_attribute'
  count: number
  visible: boolean
  accessible_name_sha256?: string
}

export type CoverageObservation = {
  schema_version: typeof observationSchemaVersion
  phase: 'before' | 'after'
  observed_at: string
  https: boolean
  host_allowed: boolean
  edit_marker_present: boolean
  create_marker_present: boolean
  page_kind: PageKind
  path_sha256: string
  account_context_sha256: string
  account_context_state: 'matched' | 'mismatch' | 'missing'
  session_context_sha256: string
  locator_observations: LocatorObservation[]
  structure: {
    button_count: number
    textbox_count: number
    table_count: number
    row_count: number
    observable_object_count: number
  }
  state_summary: {
    population_state: 'populated' | 'empty' | 'loading' | 'unknown'
    enabled_count: number
    disabled_count: number
    draft_marker_count: number
    loading_marker_count: number
    empty_marker_count: number
  }
  page_state_sha256: string
}

type LedgerLocator = {
  key: LocatorKey
  kind: LocatorObservation['kind']
  element_count: 1
  visible: true
  unique: true
  accessible_name_sha256: string
  source: 'real_current_page'
}

type EvidenceSnapshot = {
  evidence_sha256: string
  observable_object_count: number
  population_state: CoverageObservation['state_summary']['population_state']
  page_state_sha256: string
}

type PageNoWriteProof = {
  proof_state: 'confirmed' | 'verification_pending'
  comparison_scope: 'current_page_observable_counts_and_status'
  write_actions_executed: 0
  mutation_api_surface: 'absent'
  new_object_count: number
  observable_state_change_count: 0 | 1
  draft_residual_count: number
  draft_residual_delta: number
  same_page_target: boolean
  same_account_context: boolean
}

type PageEvidence = {
  target: PageTarget
  before: EvidenceSnapshot
  after: EvidenceSnapshot
  no_write_proof: PageNoWriteProof
  evidence_sha256: string
}

export type LedgerTarget = {
  target: AccountContextTarget | PageTarget
  attempted: true
  status: CoverageStatus
  reason: 'current_page_confirmed' | 'current_page_different' | 'verification_failed'
  checks: {
    https_host: CheckState
    page_type: CheckState
    account_context: CheckState
    necessary_structure: CheckState
    locator_uniqueness: CheckState
  }
  drift_state: DriftState
  locators: LedgerLocator[]
  page_evidence_sha256?: string
  field_evidence_sha256?: string
}

export type CoverageLedger = {
  schema_version: typeof coverageSchemaVersionV2
  observed_at: string
  platform: 'ocean_engine'
  allowed_host: typeof allowedHost
  scope: 'current_page_only'
  completion_state: 'partial_current_page_only'
  source_inventory: {
    manifest_page_family_count: 6
    manifest_field_count: number
    locator_fixture_count: 4
    coordinate_fallback_allowed: false
    remote_write_authorized: false
  }
  account_context_sha256: string
  targets: LedgerTarget[]
  rejected_locators: Array<{
    key: LocatorKey
    reason: 'unscoped_duplicate'
    element_count: number
    source: 'real_current_page'
  }>
  page_evidence: PageEvidence[]
  no_write_proof: {
    proof_state: 'confirmed' | 'verification_pending'
    comparison_scope: 'current_page_observable_counts_and_status'
    write_actions_executed: 0
    mutation_api_surface: 'absent'
    confirmed_page_count: number
    total_new_object_count: number
    total_observable_state_change_count: number
    total_draft_residual_count: number
  }
  evidence_sha256: string
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

export function coverageLedgerEvidenceHash(value: Omit<CoverageLedger, 'evidence_sha256'>) {
  return sha256(canonicalJSON(value))
}

export function coveragePaths(localAppData = process.env.LOCALAPPDATA) {
  if (!localAppData) throw new Error('LOCALAPPDATA is not set')
  const root = resolve(localAppData, 'cookies', 'browser-rpa', 'calibration')
  return {
    root,
    runs: join(root, 'runs'),
    ledger: join(root, 'coverage-ledger.json'),
    plan: join(root, 'live-plan.json'),
    activeRun: join(root, 'active-run.json'),
    legacyBefore: join(root, 'coverage-before.json'),
    legacyAfter: join(root, 'coverage-after.json'),
  }
}

export function runIdFromTimestamp(timestamp: string): string {
  return timestamp.replaceAll(':', '-').replaceAll('.', '-')
}

export function runDirectory(paths: ReturnType<typeof coveragePaths>, runId: string): string {
  if (!/^[0-9T-Z-]{10,40}$/.test(runId)) throw new Error('run id is invalid')
  return join(paths.runs, runId)
}

export type RunPointer = {
  schema_version: typeof runPointerSchema
  run_id: string
  phase: 'before' | 'after'
  started_at: string
}

function locatorMap(observations: LocatorObservation[]) {
  return new Map(observations.map(item => [item.key, item]))
}

export function classifyCoveragePage(observations: LocatorObservation[], editMarkerPresent = false, createMarkerPresent = false): PageKind {
  const locators = locatorMap(observations)
  const present = (key: LocatorKey) => (locators.get(key)?.count ?? 0) > 0
  if (present('data_center') && present('project_report') && present('promotion_report') && present('material_report')) return 'report_overview'
  if (present('material_overview')) return 'material_relation_summary'
  if (present('promotion_name') && present('save_and_close')) {
    if (editMarkerPresent) return 'promotion_edit'
    if (createMarkerPresent) return 'promotion_create'
    return 'promotion_detail_or_edit'
  }
  if (present('project_marketing_purpose') && present('project_name') && present('project_deep_optimization')) {
    if (editMarkerPresent) return 'project_edit'
    if (createMarkerPresent) return 'project_create'
    return 'project_detail_or_edit'
  }
  if (present('create_promotion') && present('promotion_budget_column')) return 'promotion_list'
  if (present('create_project') && present('project_budget_column')) return 'project_list'
  return 'unknown'
}

async function inspectLocator(key: LocatorKey, locator: Locator, kind: LocatorObservation['kind']): Promise<LocatorObservation> {
  const count = await locator.count()
  const visible = count === 1 ? await locator.isVisible() : false
  const accessibleNameHash = count === 1 ? sha256(await locator.ariaSnapshot()) : undefined
  return { key, kind, count, visible, ...(accessibleNameHash ? { accessible_name_sha256: accessibleNameHash } : {}) }
}

async function inspectPage(page: Page, phase: CoverageObservation['phase'], expectedAccountHash: string): Promise<CoverageObservation> {
  const url = new URL(page.url())
  const accountValue = url.searchParams.get(accountQueryKey)
  const accountHash = accountValue ? sha256(accountValue) : ''
  const observations: LocatorObservation[] = []
  for (const [key, text] of Object.entries(textLocators) as Array<[TextLocatorKey, string]>) {
    observations.push(await inspectLocator(key, page.getByText(text, { exact: true }), 'visible_text'))
  }
  for (const [key, selector] of Object.entries(attributeLocators) as Array<[AttributeLocatorKey, string]>) {
    observations.push(await inspectLocator(key, page.locator(selector), 'stable_test_attribute'))
  }
  const operationalStates = page.locator(attributeLocators.promotion_operational_state)
  let enabledCount = 0
  let disabledCount = 0
  for (const operationalState of await operationalStates.all()) {
    const state = await operationalState.getAttribute('aria-checked')
    if (state === 'true') enabledCount += 1
    if (state === 'false') disabledCount += 1
  }
  const draftMarkerCount = await page.getByText('草稿', { exact: true }).count()
  const loadingMarkerCount = await page.getByText('加载中', { exact: true }).count()
  const emptyMarkerCount = await page.getByText('暂无数据', { exact: true }).count()
  const structure = {
    button_count: await page.getByRole('button').count(),
    textbox_count: await page.getByRole('textbox').count(),
    table_count: await page.getByRole('table').count(),
    row_count: await page.getByRole('row').count(),
    observable_object_count: await page.locator('table tbody tr').count(),
  }
  const stateSummary = {
    population_state: (loadingMarkerCount > 0 ? 'loading' : structure.observable_object_count > 0 ? 'populated' : emptyMarkerCount > 0 ? 'empty' : 'unknown') as CoverageObservation['state_summary']['population_state'],
    enabled_count: enabledCount,
    disabled_count: disabledCount,
    draft_marker_count: draftMarkerCount,
    loading_marker_count: loadingMarkerCount,
    empty_marker_count: emptyMarkerCount,
  }
  const pageCDP = await page.context().newCDPSession(page)
  const target = await pageCDP.send('Target.getTargetInfo') as { targetInfo?: { targetId?: string; browserContextId?: string } }
  if (!target.targetInfo?.targetId) throw new Error('target_info_unavailable')
  const editMarkerPresent = url.searchParams.has('is_update')
  const createMarkerPresent = url.searchParams.has('is_create')
  const pageKind = classifyCoveragePage(observations, editMarkerPresent, createMarkerPresent)
  const pageState = {
    pageKind,
    observations,
    observable_object_count: structure.observable_object_count,
    stateSummary,
  }
  return {
    schema_version: observationSchemaVersion,
    phase,
    observed_at: new Date().toISOString(),
    https: url.protocol === 'https:',
    host_allowed: url.hostname === allowedHost,
    edit_marker_present: editMarkerPresent,
    create_marker_present: createMarkerPresent,
    page_kind: pageKind,
    path_sha256: sha256(url.pathname),
    account_context_sha256: accountHash || '0'.repeat(64),
    account_context_state: !accountValue ? 'missing' : accountHash === expectedAccountHash ? 'matched' : 'mismatch',
    session_context_sha256: sha256(`${target.targetInfo.browserContextId ?? 'default'}:${target.targetInfo.targetId}`),
    locator_observations: observations,
    structure,
    state_summary: stateSummary,
    page_state_sha256: sha256(canonicalJSON(pageState)),
  }
}

async function observeCommand(localAppData: string, phase: CoverageObservation['phase']) {
  const paths = coveragePaths(localAppData)
  await mkdir(paths.runs, { recursive: true })
  const plan = JSON.parse(await readFile(paths.plan, 'utf8')) as { account_context?: { expected_sha256?: string } }
  const expectedAccountHash = plan.account_context?.expected_sha256
  if (!expectedAccountHash || !hashPattern.test(expectedAccountHash)) throw new Error('account_context_unavailable')
  const browser = await chromium.connectOverCDP(await sessionConnection(localAppData), { timeout: 120000 })
  const deadline = Date.now() + 30000
  let page: Page | undefined
  do {
    page = browser.contexts().flatMap(context => context.pages()).filter(item => {
      try { return new URL(item.url()).hostname === allowedHost } catch { return false }
    }).at(-1)
    if (!page) await new Promise(resolveTimer => setTimeout(resolveTimer, 250))
  } while (!page && Date.now() < deadline)
  if (!page) throw new Error('host_not_allowed')
  const observation = await inspectPage(page, phase, expectedAccountHash)
  if (phase === 'before') {
    const runId = runIdFromTimestamp(observation.observed_at)
    await mkdir(runDirectory(paths, runId), { recursive: true })
    await atomicJSON(join(runDirectory(paths, runId), 'coverage-before.json'), observation)
    const pointer: RunPointer = { schema_version: runPointerSchema, run_id: runId, phase: 'before', started_at: observation.observed_at }
    await atomicJSON(paths.activeRun, pointer)
    return { outcome: 'observed' as const, phase, run_id: runId, page_kind: observation.page_kind, evidence_sha256: sha256(canonicalJSON(observation)) }
  }
  const pointer = await readRunPointer(paths)
  await mkdir(runDirectory(paths, pointer.run_id), { recursive: true })
  await atomicJSON(join(runDirectory(paths, pointer.run_id), 'coverage-after.json'), observation)
  await atomicJSON(paths.activeRun, { ...pointer, phase: 'after' })
  return { outcome: 'observed' as const, phase, run_id: pointer.run_id, page_kind: observation.page_kind, evidence_sha256: sha256(canonicalJSON(observation)) }
}

async function readRunPointer(paths: ReturnType<typeof coveragePaths>): Promise<RunPointer> {
  let raw: string
  try {
    raw = await readFile(paths.activeRun, 'utf8')
  } catch {
    throw new Error('run_before_missing: run "browser-rpa:coverage -- observe before" first')
  }
  const pointer = JSON.parse(raw) as RunPointer
  const runIdValid = typeof pointer.run_id === 'string' && /^[0-9T-Z-]{10,40}$/.test(pointer.run_id)
  if (pointer.schema_version !== runPointerSchema || !runIdValid || (pointer.phase !== 'before' && pointer.phase !== 'after')) throw new Error('active_run_pointer_invalid')
  return pointer
}

const requirements: Record<PageTarget, { locators: LocatorKey[]; structure: (observation: CoverageObservation) => boolean }> = {
  project_list: { locators: ['create_project', 'project_budget_column'], structure: item => item.structure.table_count > 0 },
  project_create: { locators: ['project_marketing_purpose', 'project_name', 'project_deep_optimization'], structure: item => item.structure.textbox_count > 0 },
  project_edit: { locators: ['project_marketing_purpose', 'project_name', 'project_deep_optimization'], structure: item => item.structure.textbox_count > 0 },
  promotion_list: { locators: ['create_promotion', 'promotion_budget_column'], structure: item => item.structure.table_count > 0 },
  promotion_create: { locators: ['promotion_name', 'save_and_close'], structure: item => item.structure.textbox_count > 0 },
  promotion_edit: { locators: ['promotion_name', 'save_and_close'], structure: item => item.structure.textbox_count > 0 },
  material_relation_summary: { locators: ['material_overview'], structure: item => item.structure.table_count > 0 || item.state_summary.empty_marker_count > 0 },
  report_overview: { locators: ['data_center', 'project_report', 'promotion_report', 'material_report'], structure: item => item.structure.table_count > 0 || item.state_summary.empty_marker_count > 0 },
}

function evidenceHash(observation: CoverageObservation) {
  return sha256(canonicalJSON(observation))
}

function ledgerLocators(observation: CoverageObservation, keys: LocatorKey[]): LedgerLocator[] {
  const observations = locatorMap(observation.locator_observations)
  return keys.flatMap(key => {
    const item = observations.get(key)
    if (!item || item.count !== 1 || !item.visible || !item.accessible_name_sha256) return []
    return [{ key, kind: item.kind, element_count: 1 as const, visible: true as const, unique: true as const, accessible_name_sha256: item.accessible_name_sha256, source: 'real_current_page' as const }]
  })
}

export function buildCoverageLedger(before: CoverageObservation, after: CoverageObservation, manifestFieldCount: number): CoverageLedger {
  const samePage = before.path_sha256 === after.path_sha256 && before.session_context_sha256 === after.session_context_sha256 && before.page_kind === after.page_kind
  const sameAccount = before.account_context_sha256 === after.account_context_sha256 && before.account_context_state === 'matched' && after.account_context_state === 'matched'
  const sameState = before.page_state_sha256 === after.page_state_sha256
  const targets: LedgerTarget[] = [{
    target: 'account_context', attempted: true,
    status: sameAccount ? 'confirmed_shell' : 'blocked',
    reason: sameAccount ? 'current_page_confirmed' : 'verification_failed',
    checks: { https_host: 'not_applicable', page_type: 'not_applicable', account_context: sameAccount ? 'passed' : 'failed', necessary_structure: 'not_applicable', locator_uniqueness: 'not_applicable' },
    drift_state: sameAccount ? 'stable' : 'page_drift',
    locators: [],
  }]
  for (const target of Object.keys(requirements) as PageTarget[]) {
    const current = before.page_kind === target && after.page_kind === target
    if (!current) {
      targets.push({ target, attempted: true, status: 'not_accessible', reason: 'current_page_different', checks: { https_host: 'not_run', page_type: 'not_run', account_context: 'not_run', necessary_structure: 'not_run', locator_uniqueness: 'not_run' }, drift_state: 'verification_pending', locators: [] })
      continue
    }
    const requirement = requirements[target]
    const locators = ledgerLocators(after, requirement.locators)
    const httpsPassed = before.https && after.https && before.host_allowed && after.host_allowed
    const pageTypePassed = samePage
    const structurePassed = requirement.structure(before) && requirement.structure(after)
    const uniquePassed = locators.length === requirement.locators.length
    const shellConfirmed = httpsPassed && pageTypePassed && sameAccount && structurePassed && uniquePassed && sameState
    targets.push({
      target, attempted: true, status: shellConfirmed ? 'confirmed_shell' : 'blocked', reason: shellConfirmed ? 'current_page_confirmed' : 'verification_failed',
      checks: { https_host: httpsPassed ? 'passed' : 'failed', page_type: pageTypePassed ? 'passed' : 'failed', account_context: sameAccount ? 'passed' : 'failed', necessary_structure: structurePassed ? 'passed' : 'failed', locator_uniqueness: uniquePassed ? 'passed' : 'failed' },
      drift_state: shellConfirmed ? 'stable' : 'page_drift', locators,
    })
  }
  const afterLocators = locatorMap(after.locator_observations)
  const rejectedLocators = (['account_report'] as LocatorKey[]).flatMap(key => {
    const item = afterLocators.get(key)
    return item && item.count > 1 ? [{ key, reason: 'unscoped_duplicate' as const, element_count: item.count, source: 'real_current_page' as const }] : []
  })
  const objectDelta = after.structure.observable_object_count - before.structure.observable_object_count
  const draftDelta = after.state_summary.draft_marker_count - before.state_summary.draft_marker_count
  const proofConfirmed = samePage && sameAccount && sameState && objectDelta === 0 && draftDelta === 0 && after.state_summary.draft_marker_count === 0
  const observableKinds = Object.keys(requirements) as PageTarget[]
  const currentPageTarget = observableKinds.includes(before.page_kind as PageTarget) && before.page_kind === after.page_kind ? before.page_kind as PageTarget : undefined
  const beforeSnapshot = { evidence_sha256: evidenceHash(before), observable_object_count: before.structure.observable_object_count, population_state: before.state_summary.population_state, page_state_sha256: before.page_state_sha256 }
  const afterSnapshot = { evidence_sha256: evidenceHash(after), observable_object_count: after.structure.observable_object_count, population_state: after.state_summary.population_state, page_state_sha256: after.page_state_sha256 }
  const pageNoWriteProof: PageNoWriteProof = { proof_state: proofConfirmed ? 'confirmed' : 'verification_pending', comparison_scope: 'current_page_observable_counts_and_status', write_actions_executed: 0, mutation_api_surface: 'absent', new_object_count: Math.max(0, objectDelta), observable_state_change_count: sameState ? 0 : 1, draft_residual_count: after.state_summary.draft_marker_count, draft_residual_delta: draftDelta, same_page_target: samePage, same_account_context: sameAccount }
  const pageEvidenceWithoutHash = currentPageTarget ? { target: currentPageTarget, before: beforeSnapshot, after: afterSnapshot, no_write_proof: pageNoWriteProof } : undefined
  const pageEvidence: PageEvidence[] = pageEvidenceWithoutHash ? [{ ...pageEvidenceWithoutHash, evidence_sha256: sha256(canonicalJSON(pageEvidenceWithoutHash)) }] : []
  if (pageEvidence[0]) {
    const currentTarget = targets.find(item => item.target === pageEvidence[0].target)
    if (currentTarget) currentTarget.page_evidence_sha256 = pageEvidence[0].evidence_sha256
  }
  const ledgerWithoutHash = {
    schema_version: coverageSchemaVersionV2,
    observed_at: after.observed_at,
    platform: 'ocean_engine' as const,
    allowed_host: allowedHost,
    scope: 'current_page_only' as const,
    completion_state: 'partial_current_page_only' as const,
    source_inventory: { manifest_page_family_count: 6 as const, manifest_field_count: manifestFieldCount, locator_fixture_count: 4 as const, coordinate_fallback_allowed: false as const, remote_write_authorized: false as const },
    account_context_sha256: after.account_context_sha256,
    targets,
    rejected_locators: rejectedLocators,
    page_evidence: pageEvidence,
    no_write_proof: { proof_state: proofConfirmed ? 'confirmed' as const : 'verification_pending' as const, comparison_scope: 'current_page_observable_counts_and_status' as const, write_actions_executed: 0 as const, mutation_api_surface: 'absent' as const, confirmed_page_count: proofConfirmed ? 1 : 0, total_new_object_count: Math.max(0, objectDelta), total_observable_state_change_count: sameState ? 0 : 1, total_draft_residual_count: after.state_summary.draft_marker_count },
  }
  return { ...ledgerWithoutHash, evidence_sha256: coverageLedgerEvidenceHash(ledgerWithoutHash) }
}

function aggregateNoWriteProof(pageEvidence: PageEvidence[]): CoverageLedger['no_write_proof'] {
  const confirmed = pageEvidence.every(item => item.no_write_proof.proof_state === 'confirmed')
  return {
    proof_state: confirmed ? 'confirmed' : 'verification_pending',
    comparison_scope: 'current_page_observable_counts_and_status',
    write_actions_executed: 0,
    mutation_api_surface: 'absent',
    confirmed_page_count: pageEvidence.filter(item => item.no_write_proof.proof_state === 'confirmed').length,
    total_new_object_count: pageEvidence.reduce((total, item) => total + item.no_write_proof.new_object_count, 0),
    total_observable_state_change_count: pageEvidence.reduce((total, item) => total + item.no_write_proof.observable_state_change_count, 0),
    total_draft_residual_count: pageEvidence.reduce((total, item) => total + item.no_write_proof.draft_residual_count, 0),
  }
}

type LegacyTargetName = LedgerTarget['target'] | LegacyFormTarget
type LegacyStoredLedger = Omit<CoverageLedger, 'schema_version' | 'targets' | 'page_evidence'> & {
  schema_version: typeof coverageSchemaVersion
  targets: Array<Omit<LedgerTarget, 'target' | 'status'> & { target: LegacyTargetName; status: 'confirmed' | 'blocked' | 'not_accessible' }>
  before?: EvidenceSnapshot
  after?: EvidenceSnapshot
  no_write_proof?: PageNoWriteProof
  page_evidence?: Array<Omit<PageEvidence, 'target' | 'evidence_sha256'> & { target: PageTarget | LegacyFormTarget }>
}

const legacyTargetNames: Record<LegacyFormTarget, PageTarget> = {
  project_detail_or_edit: 'project_edit',
  promotion_detail_or_edit: 'promotion_edit',
}

function mapLegacyTargetName(name: LegacyTargetName): LedgerTarget['target'] {
  return name in legacyTargetNames ? legacyTargetNames[name as LegacyFormTarget] : name as LedgerTarget['target']
}

function mapLegacyStatus(status: 'confirmed' | CoverageStatus): CoverageStatus {
  return status === 'confirmed' ? 'confirmed_shell' : status
}

// Stored ledgers arrive in three historical shapes: pre-page-evidence v1
// (root before/after snapshots), complete v1, and current v2. Everything is
// normalized to v2 before merging so accumulated evidence survives upgrades.
export function upgradeLegacyLedger(legacy: unknown): CoverageLedger {
  const stored = legacy as LegacyStoredLedger
  let pageEvidence: Array<Omit<PageEvidence, 'target' | 'evidence_sha256'> & { target: PageTarget }> = []
  if (stored.page_evidence) {
    pageEvidence = stored.page_evidence.map(item => ({ ...item, target: mapLegacyTargetName(item.target) as PageTarget }))
  } else if (stored.before && stored.after && stored.no_write_proof) {
    const pageTarget = stored.targets.find(item => item.target !== 'account_context' && item.status === 'confirmed')?.target
    if (pageTarget && pageTarget !== 'account_context') {
      pageEvidence = [{ target: mapLegacyTargetName(pageTarget) as PageTarget, before: stored.before, after: stored.after, no_write_proof: stored.no_write_proof }]
    }
  }
  const evidenceWithHash = pageEvidence.map(item => ({ ...item, evidence_sha256: sha256(canonicalJSON(item)) }))
  const mappedTargets = new Map(stored.targets.map(item => {
    const target = mapLegacyTargetName(item.target)
    const evidence = target === 'account_context' ? undefined : evidenceWithHash.find(candidate => candidate.target === target)
    return [target, { ...item, target, status: mapLegacyStatus(item.status), ...(evidence ? { page_evidence_sha256: evidence.evidence_sha256 } : {}) } as LedgerTarget]
  }))
  // The v2 contract requires all nine targets. Legacy runs predate the
  // create/edit split, so any target they never carried is recorded as
  // not-accessible on that page rather than dropped.
  const targets: LedgerTarget[] = []
  if (!mappedTargets.has('account_context')) throw new Error('legacy ledger lacks account_context target')
  for (const key of ['account_context', ...Object.keys(requirements)] as const) {
    const existing = mappedTargets.get(key)
    targets.push(existing ?? {
      target: key as PageTarget,
      attempted: true,
      status: 'not_accessible',
      reason: 'current_page_different',
      checks: { https_host: 'not_run', page_type: 'not_run', account_context: 'not_run', necessary_structure: 'not_run', locator_uniqueness: 'not_run' },
      drift_state: 'verification_pending',
      locators: [],
    })
    mappedTargets.delete(key)
  }
  if (mappedTargets.size > 0) throw new Error(`legacy ledger carries unknown targets: ${[...mappedTargets.keys()].join(', ')}`)
  const ledgerWithoutHash = {
    schema_version: coverageSchemaVersionV2,
    observed_at: stored.observed_at,
    platform: stored.platform,
    allowed_host: stored.allowed_host,
    scope: stored.scope,
    completion_state: stored.completion_state,
    source_inventory: stored.source_inventory,
    account_context_sha256: stored.account_context_sha256,
    targets,
    rejected_locators: stored.rejected_locators,
    page_evidence: evidenceWithHash,
    no_write_proof: aggregateNoWriteProof(evidenceWithHash),
  }
  return { ...ledgerWithoutHash, evidence_sha256: coverageLedgerEvidenceHash(ledgerWithoutHash) }
}

function statusRank(status: CoverageStatus): number {
  if (status === 'confirmed_fields') return 3
  if (status === 'confirmed_shell') return 2
  if (status === 'blocked') return 1
  return 0
}

export function mergeCoverageLedgers(previous: CoverageLedger, current: CoverageLedger): CoverageLedger {
  if (previous.account_context_sha256 !== current.account_context_sha256) return current
  const evidenceByTarget = new Map<PageTarget, PageEvidence>()
  for (const item of [...previous.page_evidence, ...current.page_evidence]) evidenceByTarget.set(item.target, item)
  const pageEvidence = [...evidenceByTarget.values()]
  const previousTargets = new Map(previous.targets.map(item => [item.target, item]))
  const targets = current.targets.map(item => {
    const previousTarget = previousTargets.get(item.target)
    const selected = item.status === 'not_accessible' && previousTarget && previousTarget.status !== 'not_accessible' ? previousTarget : item
    // A stronger prior confirmation level survives a weaker fresh pass; the
    // fresh run can never silently downgrade accumulated field evidence.
    const strongest = previousTarget && statusRank(previousTarget.status) > statusRank(selected.status) ? previousTarget : selected
    const evidence = item.target === 'account_context' ? undefined : evidenceByTarget.get(item.target)
    return { ...strongest, ...(evidence ? { page_evidence_sha256: evidence.evidence_sha256 } : {}) }
  })
  const rejectedByKey = new Map<string, CoverageLedger['rejected_locators'][number]>()
  for (const item of [...previous.rejected_locators, ...current.rejected_locators]) rejectedByKey.set(`${item.key}:${item.reason}`, item)
  const ledgerWithoutHash = {
    schema_version: coverageSchemaVersionV2,
    observed_at: current.observed_at,
    platform: current.platform,
    allowed_host: current.allowed_host,
    scope: current.scope,
    completion_state: current.completion_state,
    source_inventory: current.source_inventory,
    account_context_sha256: current.account_context_sha256,
    targets,
    rejected_locators: [...rejectedByKey.values()],
    page_evidence: pageEvidence,
    no_write_proof: aggregateNoWriteProof(pageEvidence),
  }
  return { ...ledgerWithoutHash, evidence_sha256: coverageLedgerEvidenceHash(ledgerWithoutHash) }
}

async function loadObservationPair(paths: ReturnType<typeof coveragePaths>): Promise<{ before: CoverageObservation; after: CoverageObservation; runId: string }> {
  try {
    const pointer = await readRunPointer(paths)
    if (pointer.phase !== 'after') throw new Error('observe_after_missing: run "browser-rpa:coverage -- observe after" first')
    const directory = runDirectory(paths, pointer.run_id)
    const before = JSON.parse(await readFile(join(directory, 'coverage-before.json'), 'utf8')) as CoverageObservation
    const after = JSON.parse(await readFile(join(directory, 'coverage-after.json'), 'utf8')) as CoverageObservation
    return { before, after, runId: pointer.run_id }
  } catch (error) {
    if (!(error instanceof Error) || !/run_before_missing|observe_after_missing/.test(error.message)) throw error
    // Fall back to the legacy fixed-name layout so pre-run-dir history stays usable.
    const before = JSON.parse(await readFile(paths.legacyBefore, 'utf8')) as CoverageObservation
    const after = JSON.parse(await readFile(paths.legacyAfter, 'utf8')) as CoverageObservation
    return { before, after, runId: 'legacy-root-layout' }
  }
}

async function finalizeCommand(localAppData: string) {
  const paths = coveragePaths(localAppData)
  const { before, after } = await loadObservationPair(paths)
  if (before.schema_version !== observationSchemaVersion || before.phase !== 'before' || after.schema_version !== observationSchemaVersion || after.phase !== 'after') throw new Error('invalid_observation')
  const fixtureRoot = resolve('docs', 'delivery', 'fixtures')
  const manifest = JSON.parse(await readFile(resolve(fixtureRoot, 'oceanengine-calibration-manifest-v1.json'), 'utf8')) as { fields?: unknown[]; page_families?: unknown[]; observation_boundary?: { remote_write_authorized?: boolean } }
  const locatorFixturePaths = [
    'oceanengine-existing-object-live-locators-v0.1.json',
    'oceanengine-promotion-live-locators-v0.1.json',
    'oceanengine-ecommerce-manual-live-locators-v0.1.json',
    'oceanengine-promotion-live-locator-capture-v0.1-template.json',
  ]
  const locatorFixtures = await Promise.all(locatorFixturePaths.map(async path => JSON.parse(await readFile(resolve(fixtureRoot, path), 'utf8')) as { coordinate_fallback_allowed?: boolean }))
  if (manifest.page_families?.length !== 6 || manifest.observation_boundary?.remote_write_authorized !== false || locatorFixtures.length !== 4 || locatorFixtures.some(item => item.coordinate_fallback_allowed !== false)) throw new Error('invalid_source_inventory')
  const currentLedger = buildCoverageLedger(before, after, manifest.fields?.length ?? 0)
  let ledger = currentLedger
  try {
    const stored = JSON.parse(await readFile(paths.ledger, 'utf8'))
    const previous = stored.schema_version === coverageSchemaVersionV2 ? stored as CoverageLedger : upgradeLegacyLedger(stored)
    ledger = mergeCoverageLedgers(previous, currentLedger)
  } catch (error) {
    if ((error as NodeJS.ErrnoException).code !== 'ENOENT') throw error
  }
  await atomicJSON(paths.ledger, ledger)
  await rm(paths.activeRun, { force: true })
  return ledger
}

type SessionMetadata = { state?: string; mode?: string; cdp_endpoint?: string; profile_path?: string }

async function sessionConnection(localAppData: string) {
  const metadataPath = resolve(localAppData, 'cookies', 'browser-rpa', 'session.json')
  const metadata = JSON.parse(await readFile(metadataPath, 'utf8')) as SessionMetadata
  if (metadata.state !== 'running' || !metadata.cdp_endpoint) throw new Error('cdp_unavailable')
  const endpoint = new URL(metadata.cdp_endpoint)
  if (endpoint.protocol !== 'http:' || endpoint.hostname !== '127.0.0.1') throw new Error('cdp_unavailable')
  if (metadata.mode !== 'current_user') return metadata.cdp_endpoint
  if (!metadata.profile_path) throw new Error('cdp_unavailable')
  const lines = (await readFile(join(resolve(metadata.profile_path, '..'), 'DevToolsActivePort'), 'utf8')).split(/\r?\n/)
  const port = Number.parseInt(lines[0] ?? '', 10)
  const browserPath = lines[1] ?? ''
  if (port !== Number.parseInt(endpoint.port, 10) || !/^\/devtools\/browser\/[A-Za-z0-9-]+$/.test(browserPath)) throw new Error('cdp_unavailable')
  return `ws://127.0.0.1:${port}${browserPath}`
}

async function atomicJSON(path: string, value: unknown) {
  const temporary = `${path}.${process.pid}.tmp`
  await writeFile(temporary, `${JSON.stringify(value, null, 2)}\n`, { encoding: 'utf8', mode: 0o600 })
  await rename(temporary, path)
}

async function main() {
  const localAppData = process.env.LOCALAPPDATA
  if (!localAppData) throw new Error('LOCALAPPDATA is not set')
  const [command, phase] = process.argv.slice(2)
  if (command === 'observe' && (phase === 'before' || phase === 'after')) return await observeCommand(localAppData, phase)
  if (command === 'finalize') return await finalizeCommand(localAppData)
  throw new Error('Usage: npm run browser-rpa:coverage -- <observe before|observe after|finalize>')
}

if (basename(process.argv[1] ?? '') === 'oceanengine-readonly-coverage-ledger.ts') {
  main().then(result => process.stdout.write(`${JSON.stringify(result, null, 2)}\n`, () => process.exit(0))).catch(() => {
    process.stderr.write(`${JSON.stringify({ outcome: 'failed', error_code: 'coverage_unavailable' })}\n`, () => process.exit(1))
  })
}
