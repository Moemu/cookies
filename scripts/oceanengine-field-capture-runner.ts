import { createHash } from 'node:crypto'
import { appendFile, mkdir, readFile, rename, writeFile } from 'node:fs/promises'
import { basename, join, resolve } from 'node:path'
import { chromium, type Locator, type Page } from '@playwright/test'

// Read-only field-level capture: walks the frozen calibration manifest's
// coverage cases and records per-field presence/readback evidence for the
// current page. No click, fill, select, upload, navigation, or submit exists
// on this surface; conditional branches are observed by re-running after the
// operator manually changes the upstream selection.
export const planSchemaVersion = 'oceanengine-field-capture-plan/v1' as const
export const observationSchemaVersion = 'oceanengine-field-observation/v1' as const
const manifestFixture = 'oceanengine-calibration-manifest-v1.json'

const allowedHost = 'ad.oceanengine.com'
const accountQueryKey = 'aadvid'
const safeID = /^[a-z][a-z0-9._-]{0,63}$/
const caseIDPattern = /^OE-[A-Z0-9-]{1,64}$/
const hashPattern = /^[0-9a-f]{64}$/
const dimensionKeyPattern = /^[a-z][a-z0-9_-]{0,47}$/

export type PageFamilyId = 'project_list' | 'project_create' | 'project_edit' | 'promotion_list' | 'promotion_create' | 'promotion_edit'
type CandidateFamily = PageFamilyId
const candidateOrder: CandidateFamily[] = ['project_list', 'promotion_list', 'project_create', 'project_edit', 'promotion_create', 'promotion_edit']

type ErrorCode = 'ok' | 'invalid_plan' | 'invalid_manifest' | 'cdp_unavailable' | 'host_not_allowed' | 'account_context_missing' | 'account_mismatch' | 'page_type_mismatch' | 'case_not_found' | 'case_blocked' | 'internal'

export class CaptureError extends Error {
  constructor(readonly code: ErrorCode) {
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

export type NormalizedLocator =
  | { kind: 'text'; value: string }
  | { kind: 'label'; value: string }
  | { kind: 'placeholder'; value: string }
  | { kind: 'role'; role: RoleName; name: string }
  | { kind: 'attribute'; name: string; value: string }

type RoleName = 'button' | 'textbox' | 'spinbutton' | 'checkbox' | 'switch' | 'radio' | 'combobox' | 'searchbox' | 'link' | 'tab' | 'heading' | 'dialog' | 'table' | 'row' | 'cell' | 'menuitem' | 'option'
const roleNames: readonly RoleName[] = ['button', 'textbox', 'spinbutton', 'checkbox', 'switch', 'radio', 'combobox', 'searchbox', 'link', 'tab', 'heading', 'dialog', 'table', 'row', 'cell', 'menuitem', 'option']

type ManifestLocatorSpec = { kind?: unknown; value?: unknown }
export type ManifestLocatorKind = 'visible_text' | 'label' | 'placeholder' | 'role_name' | 'attribute' | 'domain_key'

export function normalizeLocator(spec: ManifestLocatorSpec): NormalizedLocator | null {
  const kind = typeof spec.kind === 'string' ? spec.kind : ''
  const value = typeof spec.value === 'string' ? spec.value : ''
  if (!value) return null
  if (kind === 'visible_text') return { kind: 'text', value }
  if (kind === 'label') return { kind: 'label', value }
  if (kind === 'placeholder') return { kind: 'placeholder', value }
  if (kind === 'role_name') {
    const separator = value.indexOf(':')
    if (separator <= 0) return null
    const role = value.slice(0, separator) as RoleName
    const name = value.slice(separator + 1)
    if (!roleNames.includes(role) || !name) return null
    return { kind: 'role', role, name }
  }
  if (kind === 'attribute') {
    const separator = value.indexOf('=')
    if (separator <= 0) return null
    const name = value.slice(0, separator)
    const attributeValue = value.slice(separator + 1)
    if (!/^data-[a-z][a-z0-9_-]*$/.test(name) || !attributeValue) return null
    return { kind: 'attribute', name, value: attributeValue }
  }
  return null
}

export type CalibrationManifest = {
  schema_version: string
  manifest_id: string
  platform: string
  observation_boundary: { remote_write_authorized?: boolean }
  page_families: Array<{ id: string; page_kind: string; page_fingerprint?: ManifestLocatorSpec[] }>
  path_dimensions: Array<{ key: string; observed_values: string[] }>
  fields: ManifestField[]
  coverage_cases: ManifestCase[]
}

type ConditionRule = { all?: Array<{ dimension?: unknown; operator?: unknown; values?: unknown }> }

export type ManifestField = {
  key: string
  semantic_label?: string
  owner?: string
  page_family: string
  value_type: string
  required_state?: string
  editable_state?: string
  condition_state?: string
  condition_dimensions?: string[]
  condition_rule?: ConditionRule
  locator: ManifestLocatorSpec
  playwright_rpa: {
    operation: string
    scope?: ManifestLocatorSpec
    target?: ManifestLocatorSpec
    expected_target_count?: number
    readback?: ManifestLocatorSpec
    blocked_state?: string
  }
  evidence_state: string
}

export type ManifestCase = { id: string; path: string[]; field_keys: string[]; status: string; reason?: string }

export async function loadCalibrationManifest(baseDirectory = process.cwd()): Promise<CalibrationManifest> {
  let raw: string
  try {
    raw = await readFile(resolve(baseDirectory, 'docs', 'delivery', 'fixtures', manifestFixture), 'utf8')
  } catch {
    throw new CaptureError('invalid_manifest')
  }
  const manifest = JSON.parse(raw) as CalibrationManifest
  if (manifest.schema_version !== 'oceanengine-calibration-manifest/v1') throw new CaptureError('invalid_manifest')
  if (manifest.platform !== 'ocean_engine' || manifest.observation_boundary?.remote_write_authorized !== false) throw new CaptureError('invalid_manifest')
  const familyIDs = new Set(manifest.page_families.map(item => item.id))
  if (familyIDs.size !== manifest.page_families.length || manifest.page_families.length === 0) throw new CaptureError('invalid_manifest')
  const fieldKeys = new Set(manifest.fields.map(item => item.key))
  if (fieldKeys.size !== manifest.fields.length || manifest.fields.length === 0) throw new CaptureError('invalid_manifest')
  for (const field of manifest.fields) {
    if (!fieldKeys.has(field.key) || !familyIDs.has(field.page_family)) throw new CaptureError('invalid_manifest')
    const operation = field.playwright_rpa?.operation
    if (!['choose_exact_visible_option', 'open_reference_picker', 'fill_text', 'fill_money', 'toggle', 'configure_object', 'no_action'].includes(operation)) throw new CaptureError('invalid_manifest')
  }
  const caseIDs = new Set(manifest.coverage_cases.map(item => item.id))
  if (caseIDs.size !== manifest.coverage_cases.length || manifest.coverage_cases.length === 0) throw new CaptureError('invalid_manifest')
  for (const coverageCase of manifest.coverage_cases) {
    if (!Array.isArray(coverageCase.path) || coverageCase.path.length === 0 || !familyIDs.has(coverageCase.path[0])) throw new CaptureError('invalid_manifest')
    if (!Array.isArray(coverageCase.field_keys) || coverageCase.field_keys.some(key => !fieldKeys.has(key))) throw new CaptureError('invalid_manifest')
  }
  return manifest
}

export type FieldCapturePlan = {
  schema_version: typeof planSchemaVersion
  plan_id: string
  platform: 'ocean_engine'
  browser: 'msedge'
  allowed_hosts: [typeof allowedHost]
  account_context: { source: 'url_query_sha256'; query_key: typeof accountQueryKey; expected_sha256: string }
  manifest_ref: { fixture: typeof manifestFixture }
  case_ids: string[]
  declared_conditions: Record<string, string>
}

export function buildDefaultPlan(manifest: CalibrationManifest, accountHash: string): FieldCapturePlan {
  return {
    schema_version: planSchemaVersion,
    plan_id: 'oceanengine-field-capture-live',
    platform: 'ocean_engine',
    browser: 'msedge',
    allowed_hosts: [allowedHost],
    account_context: { source: 'url_query_sha256', query_key: accountQueryKey, expected_sha256: accountHash },
    manifest_ref: { fixture: manifestFixture },
    case_ids: manifest.coverage_cases.map(item => item.id),
    declared_conditions: {},
  }
}

export function validateFieldCapturePlan(input: unknown, manifest: CalibrationManifest): FieldCapturePlan {
  const value = input as Record<string, unknown>
  const requiredKeys = ['schema_version', 'plan_id', 'platform', 'browser', 'allowed_hosts', 'account_context', 'manifest_ref', 'case_ids', 'declared_conditions']
  if (!value || typeof value !== 'object' || requiredKeys.some(key => !(key in value))) throw new CaptureError('invalid_plan')
  if (value.schema_version !== planSchemaVersion || value.platform !== 'ocean_engine' || value.browser !== 'msedge') throw new CaptureError('invalid_plan')
  if (!safeID.test(String(value.plan_id))) throw new CaptureError('invalid_plan')
  const hosts = value.allowed_hosts
  if (!Array.isArray(hosts) || hosts.length !== 1 || hosts[0] !== allowedHost) throw new CaptureError('invalid_plan')
  const account = value.account_context as Record<string, unknown>
  if (!account || account.source !== 'url_query_sha256' || account.query_key !== accountQueryKey || !hashPattern.test(String(account.expected_sha256))) throw new CaptureError('invalid_plan')
  const reference = value.manifest_ref as Record<string, unknown>
  if (!reference || reference.fixture !== manifestFixture) throw new CaptureError('invalid_plan')
  const caseIDs = value.case_ids
  const knownCases = new Set(manifest.coverage_cases.map(item => item.id))
  if (!Array.isArray(caseIDs) || caseIDs.length < 1 || caseIDs.length > manifest.coverage_cases.length) throw new CaptureError('invalid_plan')
  if (new Set(caseIDs).size !== caseIDs.length || caseIDs.some(id => typeof id !== 'string' || !knownCases.has(id))) throw new CaptureError('invalid_plan')
  const dimensions = new Map(manifest.path_dimensions.map(item => [item.key, new Set(item.observed_values)]))
  const declared = value.declared_conditions as Record<string, unknown>
  if (!declared || typeof declared !== 'object' || Array.isArray(declared)) throw new CaptureError('invalid_plan')
  const declaredConditions: Record<string, string> = {}
  for (const [key, item] of Object.entries(declared)) {
    if (!dimensionKeyPattern.test(key) || typeof item !== 'string' || !dimensions.get(key)?.has(item)) throw new CaptureError('invalid_plan')
    declaredConditions[key] = item
  }
  return {
    schema_version: planSchemaVersion,
    plan_id: String(value.plan_id),
    platform: 'ocean_engine',
    browser: 'msedge',
    allowed_hosts: [allowedHost],
    account_context: { source: 'url_query_sha256', query_key: accountQueryKey, expected_sha256: String(account.expected_sha256) },
    manifest_ref: { fixture: manifestFixture },
    case_ids: caseIDs as string[],
    declared_conditions: declaredConditions,
  }
}

// A field whose evaluable condition rule is contradicted by the operator's
// declared dimension state cannot appear on the current page variant.
// Undeclared dimensions stay unknown and never contradict.
export function conditionContradicted(field: ManifestField, declared: Record<string, string>): boolean {
  const rule = field.condition_rule?.all
  if (field.condition_state !== 'evaluable' || !Array.isArray(rule)) return false
  return rule.some(clause => {
    const dimension = typeof clause.dimension === 'string' ? clause.dimension : ''
    const operator = typeof clause.operator === 'string' ? clause.operator : ''
    const values = Array.isArray(clause.values) ? clause.values.filter((item): item is string => typeof item === 'string') : []
    const current = declared[dimension]
    if (current === undefined || dimension === '') return false
    if (operator === 'in') return !values.includes(current)
    if (operator === 'not_in') return values.includes(current)
    return false
  })
}

export interface FieldCaptureLocator {
  count(): Promise<number>
  isVisible(): Promise<boolean>
  ariaSnapshot(): Promise<string>
  textContent(): Promise<string | null>
  inputValue(): Promise<string>
  getAttribute(name: string): Promise<string | null>
}

export interface FieldCapturePage {
  url(): string
  locate(locator: NormalizedLocator): FieldCaptureLocator
  countRole(role: 'main' | 'heading' | 'button' | 'textbox' | 'table' | 'row'): Promise<number>
}

async function locatorFacts(locator: FieldCaptureLocator) {
  const count = await locator.count()
  const visible = count > 0 ? await locator.isVisible() : false
  return { count, visible }
}

type NameState = 'present' | 'empty' | 'unreadable' | 'not_attempted'
type ValueState = 'present' | 'empty' | 'unreadable' | 'not_attempted'

async function accessibleNameFacts(locator: FieldCaptureLocator) {
  try {
    const snapshot = await locator.ariaSnapshot()
    return { state: (snapshot.trim() ? 'present' : 'empty') as NameState, sha256: snapshot.trim() ? sha256(snapshot) : undefined }
  } catch {
    return { state: 'unreadable' as NameState, sha256: undefined }
  }
}

async function readbackFacts(locator: FieldCaptureLocator) {
  let valueKind: 'text' | 'boolean' | 'number' = 'text'
  let raw: string | null = null
  try {
    raw = await locator.inputValue()
  } catch {
    raw = null
  }
  if (raw === null) {
    try {
      raw = await locator.textContent()
    } catch {
      raw = null
    }
  }
  if (raw === null) {
    try {
      const checked = await locator.getAttribute('aria-checked')
      if (checked !== null) {
        raw = checked
        valueKind = 'boolean'
      }
    } catch {
      raw = null
    }
  }
  if (raw === null) {
    try {
      const valueNow = await locator.getAttribute('aria-valuenow')
      if (valueNow !== null) {
        raw = valueNow
        valueKind = 'number'
      }
    } catch {
      raw = null
    }
  }
  const state: ValueState = raw === null ? 'unreadable' : raw ? 'present' : 'empty'
  return { value_kind: valueKind, value_state: state, ...(raw ? { value_sha256: sha256(raw) } : {}) }
}

export type FieldObservationStatus = 'observed' | 'scope_missing' | 'scope_ambiguous' | 'target_missing' | 'target_ambiguous' | 'blocked_by_condition' | 'blocked_spec'

type FieldObservation = {
  key: string
  semantic_label_state: 'present' | 'absent'
  operation: string
  scope: { kind: 'resolved' | 'unresolvable'; element_count: number; visible: boolean }
  target: { kind: 'resolved' | 'unresolvable'; element_count: number; visible: boolean; accessible_name_state: NameState; accessible_name_sha256?: string }
  readback: { kind: 'resolved' | 'unresolvable'; resolved: boolean; value_kind?: 'text' | 'boolean' | 'number'; value_state: ValueState; value_sha256?: string }
  status: FieldObservationStatus
}

export type FieldObservationEnvelope = {
  schema_version: typeof observationSchemaVersion
  manifest_id: string
  plan_sha256: string
  case_id: string
  path: string[]
  declared_conditions: Record<string, string>
  reason?: string
  observed_at: string
  page?: {
    host: typeof allowedHost
    page_kind: PageFamilyId | 'unknown'
    path_sha256: string
    account_context_sha256: string
    account_context_state: 'matched'
    create_marker_present: boolean
    edit_marker_present: boolean
    session_context_sha256: string
    structure_summary: { main_count: number; heading_count: number; button_count: number; textbox_count: number; table_count: number; row_count: number }
  }
  fields: FieldObservation[]
  outcome: 'success' | 'partial' | 'blocked_case' | 'failed' | 'rejected'
  error_code: ErrorCode
  evidence_sha256: string
}

export function observationEvidenceHash(observation: Omit<FieldObservationEnvelope, 'evidence_sha256'>): string {
  return sha256(canonicalJSON(observation))
}

export async function detectPageKind(url: URL, manifest: CalibrationManifest, probe: (locator: NormalizedLocator) => Promise<number>): Promise<PageFamilyId | 'unknown'> {
  const createMarker = url.searchParams.has('is_create')
  const updateMarker = url.searchParams.has('is_update')
  const familiesByID = new Map(manifest.page_families.map(item => [item.id, item]))
  // Ranks (higher wins): a URL marker is authoritative — the form family it
  // names beats everything, and list fingerprints on a marker-bearing URL are
  // stale page text, so they drop below the returnable threshold.
  const scored: Array<{ id: CandidateFamily; rank: 0 | 1 | 2 | 3 }> = []
  for (const id of candidateOrder) {
    const family = familiesByID.get(id)
    if (!family) continue
    const fingerprints = (family.page_fingerprint ?? []).map(normalizeLocator)
    if (fingerprints.length === 0 || fingerprints.some(item => item === null)) {
      // Domain-key fingerprints are not resolvable on-page; promotion form
      // variants are identified by their recorded URL shape and markers.
      if (url.pathname.startsWith('/superior/ads')) {
        if (id === 'promotion_create' && createMarker) scored.push({ id, rank: 3 })
        else if (id === 'promotion_edit' && updateMarker) scored.push({ id, rank: 3 })
      }
      continue
    }
    let matched = true
    for (const fingerprint of fingerprints as NormalizedLocator[]) {
      if ((await probe(fingerprint)) < 1) {
        matched = false
        break
      }
    }
    if (!matched) continue
    const wantsCreate = id.endsWith('_create')
    const wantsUpdate = id.endsWith('_edit')
    let rank: 0 | 1 | 2 | 3
    if (wantsCreate || wantsUpdate) {
      const markerMatches = wantsCreate ? createMarker : updateMarker
      const markerContradicts = wantsCreate ? updateMarker : createMarker
      rank = markerMatches ? 3 : markerContradicts ? 0 : 1
    } else {
      rank = createMarker || updateMarker ? 0 : 2
    }
    scored.push({ id, rank })
  }
  scored.sort((left, right) => right.rank - left.rank)
  const best = scored[0]
  if (!best || best.rank < 1) return 'unknown'
  return best.id
}

export type CaseObservationInput = {
  plan: FieldCapturePlan
  manifest: CalibrationManifest
  caseId: string
  page: FieldCapturePage
  sessionContextSha256: string
  observedAt?: string
}

export async function executeCaseObservation(input: CaseObservationInput): Promise<FieldObservationEnvelope> {
  const { plan, manifest, caseId, page } = input
  const base = {
    schema_version: observationSchemaVersion,
    manifest_id: manifest.manifest_id,
    plan_sha256: sha256(canonicalJSON(plan)),
    case_id: caseId,
    path: [] as string[],
    declared_conditions: plan.declared_conditions,
    observed_at: input.observedAt ?? new Date().toISOString(),
    fields: [] as FieldObservation[],
  }
  const finish = (envelope: Omit<FieldObservationEnvelope, 'evidence_sha256'>): FieldObservationEnvelope => ({ ...envelope, evidence_sha256: observationEvidenceHash(envelope) })
  try {
    const coverageCase = manifest.coverage_cases.find(item => item.id === caseId)
    if (!coverageCase) throw new CaptureError('case_not_found')
    const pathCopy = [...coverageCase.path]
    if (coverageCase.status.startsWith('blocked')) {
      return finish({ ...base, path: pathCopy, outcome: 'blocked_case', error_code: 'case_blocked', ...(coverageCase.reason ? { reason: coverageCase.reason.slice(0, 200).replace(/[^\x20-\x7E]/g, '?') } : {}), fields: [] })
    }
    const url = new URL(page.url())
    if (url.protocol !== 'https:' || !plan.allowed_hosts.includes(url.hostname as typeof allowedHost)) throw new CaptureError('host_not_allowed')
    const accountValue = url.searchParams.get(plan.account_context.query_key)
    if (!accountValue) throw new CaptureError('account_context_missing')
    if (sha256(accountValue) !== plan.account_context.expected_sha256) throw new CaptureError('account_mismatch')
    const probe = async (locator: NormalizedLocator) => await page.locate(locator).count()
    const pageKind = await detectPageKind(url, manifest, probe)
    const structureSummary = {
      main_count: await page.countRole('main'),
      heading_count: await page.countRole('heading'),
      button_count: await page.countRole('button'),
      textbox_count: await page.countRole('textbox'),
      table_count: await page.countRole('table'),
      row_count: await page.countRole('row'),
    }
    const pageEvidence: NonNullable<FieldObservationEnvelope['page']> = {
      host: allowedHost,
      page_kind: pageKind,
      path_sha256: sha256(url.pathname),
      account_context_sha256: sha256(accountValue),
      account_context_state: 'matched' as const,
      create_marker_present: url.searchParams.has('is_create'),
      edit_marker_present: url.searchParams.has('is_update'),
      session_context_sha256: input.sessionContextSha256,
      structure_summary: structureSummary,
    }
    const rejectWithPage = (code: ErrorCode) => finish({ ...base, path: pathCopy, page: pageEvidence, outcome: 'rejected', error_code: code, ...(code === 'page_type_mismatch' ? { reason: `required_family=${coverageCase.path[0]} detected=${pageKind}` } : {}), fields: [] })
    if (pageKind !== coverageCase.path[0]) return rejectWithPage('page_type_mismatch')
    const fieldByKey = new Map(manifest.fields.map(item => [item.key, item]))
    const observations: FieldObservation[] = []
    for (const key of coverageCase.field_keys) {
      const field = fieldByKey.get(key)
      if (!field) throw new CaptureError('invalid_plan')
      const operation = field.playwright_rpa.operation
      const labelPresent = typeof field.semantic_label === 'string' && field.semantic_label.length > 0
      const emptyObservation = (): FieldObservation => ({
        key,
        semantic_label_state: labelPresent ? 'present' : 'absent',
        operation,
        scope: { kind: 'unresolvable', element_count: 0, visible: false },
        target: { kind: 'unresolvable', element_count: 0, visible: false, accessible_name_state: 'not_attempted' },
        readback: { kind: 'unresolvable', resolved: false, value_state: 'not_attempted' },
        status: 'blocked_spec',
      })
      if (operation === 'no_action') {
        observations.push(emptyObservation())
        continue
      }
      if (conditionContradicted(field, plan.declared_conditions)) {
        const skipped = emptyObservation()
        skipped.status = 'blocked_by_condition'
        observations.push(skipped)
        continue
      }
      const scopeSpec = normalizeLocator(field.playwright_rpa.scope ?? field.locator)
      if (!scopeSpec) {
        observations.push(emptyObservation())
        continue
      }
      const targetSpec = normalizeLocator(field.playwright_rpa.target ?? { kind: undefined, value: undefined })
      const readbackSpec = normalizeLocator(field.playwright_rpa.readback ?? { kind: undefined, value: undefined })
      const scopeFacts = await locatorFacts(page.locate(scopeSpec))
      const partial = (): FieldObservation => ({
        key,
        semantic_label_state: labelPresent ? 'present' : 'absent',
        operation,
        scope: { kind: 'resolved', element_count: scopeFacts.count, visible: scopeFacts.visible },
        target: { kind: targetSpec ? 'resolved' : 'unresolvable', element_count: 0, visible: false, accessible_name_state: 'not_attempted' },
        readback: { kind: readbackSpec ? 'resolved' : 'unresolvable', resolved: false, value_state: 'not_attempted' },
        status: scopeFacts.count === 0 ? 'scope_missing' : scopeFacts.count > 1 ? 'scope_ambiguous' : 'target_missing',
      })
      if (scopeFacts.count !== 1 || !scopeFacts.visible) {
        observations.push(partial())
        continue
      }
      if (!targetSpec) {
        const missingTarget = partial()
        missingTarget.status = 'blocked_spec'
        observations.push(missingTarget)
        continue
      }
      const targetFacts = await locatorFacts(page.locate(targetSpec))
      if (targetFacts.count !== 1 || !targetFacts.visible) {
        const failed = partial()
        failed.target.element_count = targetFacts.count
        failed.target.visible = targetFacts.visible
        failed.status = targetFacts.count === 0 ? 'target_missing' : 'target_ambiguous'
        observations.push(failed)
        continue
      }
      const targetName = await accessibleNameFacts(page.locate(targetSpec))
      let readback: FieldObservation['readback'] = { kind: readbackSpec ? 'resolved' : 'unresolvable', resolved: false, value_state: 'not_attempted' as ValueState }
      if (readbackSpec) {
        const facts = await readbackFacts(page.locate(readbackSpec))
        readback = { kind: 'resolved', resolved: facts.value_state === 'present' || facts.value_state === 'empty', value_kind: facts.value_kind, value_state: facts.value_state, ...(facts.value_sha256 ? { value_sha256: facts.value_sha256 } : {}) }
      }
      observations.push({
        key,
        semantic_label_state: labelPresent ? 'present' : 'absent',
        operation,
        scope: { kind: 'resolved', element_count: 1, visible: true },
        target: { kind: 'resolved', element_count: 1, visible: true, accessible_name_state: targetName.state, ...(targetName.sha256 ? { accessible_name_sha256: targetName.sha256 } : {}) },
        readback,
        status: 'observed',
      })
    }
    const attempted = observations.filter(item => item.status !== 'blocked_spec' && item.status !== 'blocked_by_condition')
    const confirmed = attempted.every(item => item.status === 'observed')
    return finish({
      ...base,
      path: pathCopy,
      page: pageEvidence,
      fields: observations,
      outcome: attempted.length === 0 ? 'partial' : confirmed ? 'success' : 'partial',
      error_code: 'ok',
    })
  } catch (error) {
    const code = error instanceof CaptureError ? error.code : 'internal'
    return finish({ ...base, outcome: 'rejected', error_code: code })
  }
}

class PlaywrightFieldCapturePage implements FieldCapturePage {
  constructor(private readonly page: Page) {}

  url() {
    return this.page.url()
  }

  locate(locator: NormalizedLocator): FieldCaptureLocator {
    let resolved: Locator
    if (locator.kind === 'text') resolved = this.page.getByText(locator.value, { exact: true })
    else if (locator.kind === 'label') resolved = this.page.getByLabel(locator.value, { exact: true })
    else if (locator.kind === 'placeholder') resolved = this.page.getByPlaceholder(locator.value, { exact: true })
    else if (locator.kind === 'role') resolved = this.page.getByRole(locator.role, { name: locator.name, exact: true })
    else resolved = this.page.locator(`[${locator.name}=${JSON.stringify(locator.value)}]`)
    return resolved
  }

  countRole(role: 'main' | 'heading' | 'button' | 'textbox' | 'table' | 'row') {
    return this.page.getByRole(role).count()
  }
}

export function fieldCapturePaths(localAppData = process.env.LOCALAPPDATA) {
  if (!localAppData) throw new Error('LOCALAPPDATA is not set')
  const root = resolve(localAppData, 'cookies', 'browser-rpa', 'calibration')
  return { root, plan: join(root, 'live-fields-plan.json'), observations: join(root, 'field-observations'), diagnostics: join(root, 'diagnostics.jsonl') }
}

type SessionMetadata = { state?: string; mode?: string; cdp_endpoint?: string; profile_path?: string }

async function sessionConnection(localAppData: string) {
  const metadataPath = resolve(localAppData, 'cookies', 'browser-rpa', 'session.json')
  const metadata = JSON.parse(await readFile(metadataPath, 'utf8')) as SessionMetadata
  if (metadata.state !== 'running' || !metadata.cdp_endpoint) throw new CaptureError('cdp_unavailable')
  const endpoint = new URL(metadata.cdp_endpoint)
  if (endpoint.protocol !== 'http:' || endpoint.hostname !== '127.0.0.1') throw new CaptureError('cdp_unavailable')
  if (metadata.mode !== 'current_user') return metadata.cdp_endpoint
  if (!metadata.profile_path) throw new CaptureError('cdp_unavailable')
  const lines = (await readFile(join(resolve(metadata.profile_path, '..'), 'DevToolsActivePort'), 'utf8')).split(/\r?\n/)
  const port = Number.parseInt(lines[0] ?? '', 10)
  const browserPath = lines[1] ?? ''
  if (port !== Number.parseInt(endpoint.port, 10) || !/^\/devtools\/browser\/[A-Za-z0-9-]+$/.test(browserPath)) throw new CaptureError('cdp_unavailable')
  return `ws://127.0.0.1:${port}${browserPath}`
}

async function openLivePage(localAppData: string) {
  let endpoint: string
  try {
    endpoint = await sessionConnection(localAppData)
  } catch {
    throw new CaptureError('cdp_unavailable')
  }
  const browser = await chromium.connectOverCDP(endpoint, { timeout: 120000 })
  const deadline = Date.now() + 30000
  let page: Page | undefined
  do {
    page = browser.contexts().flatMap(context => context.pages()).filter(item => {
      try { return new URL(item.url()).hostname === allowedHost } catch { return false }
    }).at(-1)
    if (!page) await new Promise(resolveTimer => setTimeout(resolveTimer, 250))
  } while (!page && Date.now() < deadline)
  if (!page) throw new CaptureError('host_not_allowed')
  const pageCDP = await page.context().newCDPSession(page)
  const target = await pageCDP.send('Target.getTargetInfo') as { targetInfo?: { targetId?: string; browserContextId?: string } }
  if (!target.targetInfo?.targetId) throw new CaptureError('cdp_unavailable')
  return { page, sessionContextSha256: sha256(`${target.targetInfo.browserContextId ?? 'default'}:${target.targetInfo.targetId}`) }
}

async function atomicJSON(path: string, value: unknown) {
  const temporary = `${path}.${process.pid}.tmp`
  await writeFile(temporary, `${JSON.stringify(value, null, 2)}\n`, { encoding: 'utf8', mode: 0o600 })
  await rename(temporary, path)
}

async function appendDiagnostic(path: string, command: string, outcome: string, errorCode: ErrorCode) {
  await appendFile(path, `${JSON.stringify({ schema_version: 'oceanengine-field-capture-diagnostic/v1', observed_at: new Date().toISOString(), command, outcome, error_code: errorCode })}\n`, { encoding: 'utf8', mode: 0o600 })
}

async function initCommand(localAppData: string) {
  const paths = fieldCapturePaths(localAppData)
  await mkdir(paths.root, { recursive: true })
  const manifest = await loadCalibrationManifest()
  const { page } = await openLivePage(localAppData)
  const url = new URL(page.url())
  if (url.hostname !== allowedHost) throw new CaptureError('host_not_allowed')
  const accountValue = url.searchParams.get(accountQueryKey)
  if (!accountValue) throw new CaptureError('account_context_missing')
  const plan = buildDefaultPlan(manifest, sha256(accountValue))
  await atomicJSON(paths.plan, plan)
  return { outcome: 'initialized' as const, case_count: plan.case_ids.length, plan_sha256: sha256(canonicalJSON(plan)) }
}

async function runCommand(localAppData: string, caseId: string, planPath?: string) {
  if (!caseIDPattern.test(caseId ?? '')) throw new CaptureError('invalid_plan')
  const paths = fieldCapturePaths(localAppData)
  await mkdir(paths.observations, { recursive: true })
  const manifest = await loadCalibrationManifest()
  const storedPlan = JSON.parse(await readFile(planPath ? resolve(planPath) : paths.plan, 'utf8'))
  const plan = validateFieldCapturePlan(storedPlan, manifest)
  if (!plan.case_ids.includes(caseId)) throw new CaptureError('invalid_plan')
  const live = await openLivePage(localAppData)
  const observation = await executeCaseObservation({ plan, manifest, caseId, page: new PlaywrightFieldCapturePage(live.page), sessionContextSha256: live.sessionContextSha256 })
  const stamp = observation.observed_at.replaceAll(':', '-').replaceAll('.', '-')
  await atomicJSON(join(paths.observations, `${caseId}-${stamp}.json`), observation)
  await appendDiagnostic(paths.diagnostics, `run:${caseId}`, observation.outcome, observation.error_code)
  return observation
}

async function main() {
  const localAppData = process.env.LOCALAPPDATA
  if (!localAppData) throw new Error('LOCALAPPDATA is not set')
  const [command, firstArgument, secondArgument] = process.argv.slice(2)
  if (command === 'init') return await initCommand(localAppData)
  if (command === 'run') return await runCommand(localAppData, firstArgument ?? '', secondArgument)
  throw new Error('Usage: npm run browser-rpa:fields -- <init|run CASE_ID [PLAN_PATH]>')
}

if (basename(process.argv[1] ?? '') === 'oceanengine-field-capture-runner.ts') {
  main().then(result => {
    process.stdout.write(`${JSON.stringify(result, null, 2)}\n`, () => process.exit(0))
  }).catch(async error => {
    const code: ErrorCode = error instanceof CaptureError ? error.code : 'internal'
    try {
      const paths = fieldCapturePaths()
      await mkdir(paths.root, { recursive: true })
      await appendDiagnostic(paths.diagnostics, process.argv[2] ?? 'unknown', 'failed', code)
    } catch {
      // Keep the primary controlled failure when diagnostics cannot be written.
    }
    process.stderr.write(`${JSON.stringify({ outcome: 'failed', error_code: code })}\n`, () => process.exit(1))
  })
}
