import assert from 'node:assert/strict'
import { createHash } from 'node:crypto'
import { readFileSync } from 'node:fs'
import { join, resolve } from 'node:path'
import test from 'node:test'
import Ajv2020 from 'ajv/dist/2020.js'
import addFormats from 'ajv-formats'
import { buildDefaultPlan, conditionContradicted, detectPageKind, executeCaseObservation, loadCalibrationManifest, normalizeLocator, observationEvidenceHash, parsePersistentSessionRequest, unmetConditionDimensions, validateFieldCapturePlan, type CalibrationManifest, type FieldCaptureLocator, type FieldCapturePage, type ManifestCase, type ManifestField, type NormalizedLocator } from '../scripts/oceanengine-field-capture-runner.js'

type Validator = { compile(schema: Record<string, unknown>): ((value: unknown) => boolean) & { errors?: unknown } }
const AjvConstructor = Ajv2020 as unknown as new (options: { allErrors: boolean; strict: boolean }) => Validator
const installFormats = addFormats as unknown as (validator: Validator) => void
const root = resolve(import.meta.dirname, '..')
const hash = 'a'.repeat(64)
// The runner hashes the raw aadvid query value; the fixture pages carry
// `aadvid=account-1`, so the expected hash must be derived from that value.
const accountHash = createHash('sha256').update('account-1').digest('hex')

type FakeElement = { count: number; visible?: boolean; snapshot?: string; text?: string | null; input?: string; attributes?: Record<string, string> }

class FakePage implements FieldCapturePage {
  constructor(readonly urlValue: string, private readonly elements: Map<string, FakeElement>, private readonly roleCounts: Partial<Record<'main' | 'heading' | 'button' | 'textbox' | 'table' | 'row', number>> = {}) {}

  url() {
    return this.urlValue
  }

  locate(locator: NormalizedLocator): FieldCaptureLocator {
    const key = `${locator.kind}:${locator.kind === 'role' ? `${locator.role}:${locator.name}` : locator.kind === 'attribute' ? `${locator.name}=${locator.value}` : locator.value}`
    const element = this.elements.get(key) ?? { count: 0 }
    return {
      count: async () => element.count,
      isVisible: async () => element.visible ?? false,
      ariaSnapshot: async () => {
        if (element.snapshot === undefined) throw new Error('no snapshot')
        return element.snapshot
      },
      textContent: async () => (element.text === undefined ? Promise.reject(new Error('no text')) : element.text),
      inputValue: async () => (element.input === undefined ? Promise.reject(new Error('no input')) : element.input),
      getAttribute: async (name: string) => element.attributes?.[name] ?? null,
    }
  }

  countRole(role: 'main' | 'heading' | 'button' | 'textbox' | 'table' | 'row') {
    return Promise.resolve(this.roleCounts[role] ?? 0)
  }
}

function manifestFixture(fields: ManifestField[], cases: ManifestCase[]): CalibrationManifest {
  return {
    schema_version: 'oceanengine-calibration-manifest/v1',
    manifest_id: 'test-manifest',
    platform: 'ocean_engine',
    observation_boundary: { remote_write_authorized: false },
    page_families: [
      { id: 'project_create', page_kind: 'project_create', page_fingerprint: [{ kind: 'visible_text', value: 'demo-purpose' }, { kind: 'visible_text', value: 'demo-title' }] },
      { id: 'project_edit', page_kind: 'project_edit', page_fingerprint: [{ kind: 'visible_text', value: 'demo-edit-marker' }] },
      { id: 'promotion_create', page_kind: 'promotion_create', page_fingerprint: [{ kind: 'domain_key', value: 'promotion_create' }] },
      { id: 'promotion_edit', page_kind: 'promotion_edit', page_fingerprint: [{ kind: 'domain_key', value: 'promotion_edit' }] },
      { id: 'project_list', page_kind: 'project_list', page_fingerprint: [{ kind: 'visible_text', value: 'demo-list' }] },
    ],
    path_dimensions: [{ key: 'marketing_purpose', observed_values: ['ecommerce', 'product_catalog'] }],
    fields,
    coverage_cases: cases,
  }
}

const demoField: ManifestField = {
  key: 'demo.choice_field',
  semantic_label: 'demo-label',
  page_family: 'project_create',
  value_type: 'dynamic_enum',
  condition_state: 'evaluable',
  condition_rule: { all: [{ dimension: 'marketing_purpose', operator: 'not_in', values: ['product_catalog'] }] },
  locator: { kind: 'visible_text', value: 'demo-choice-scope' },
  playwright_rpa: { operation: 'choose_exact_visible_option', scope: { kind: 'visible_text', value: 'demo-choice-scope' }, target: { kind: 'visible_text', value: 'demo-choice-target' }, expected_target_count: 1, readback: { kind: 'placeholder', value: 'demo-readback-input' } },
  evidence_state: 'observed',
}

const toggleField: ManifestField = {
  key: 'demo.toggle_field',
  semantic_label: 'demo-toggle',
  page_family: 'project_create',
  value_type: 'boolean',
  locator: { kind: 'attribute', value: 'data-e2e=demo-toggle' },
  playwright_rpa: { operation: 'toggle', scope: { kind: 'visible_text', value: 'demo-toggle-section' }, target: { kind: 'attribute', value: 'data-e2e=demo-toggle' }, expected_target_count: 1 },
  evidence_state: 'observed',
}

const pickerField: ManifestField = {
  key: 'demo.picker_field',
  page_family: 'project_create',
  value_type: 'dynamic_reference',
  locator: { kind: 'domain_key', value: 'unresolvable_entry' },
  playwright_rpa: { operation: 'open_reference_picker', target: { kind: 'domain_key', value: 'unresolvable_target' } },
  evidence_state: 'platform_pending',
}

const lazyField: ManifestField = {
  key: 'demo.lazy_field',
  page_family: 'project_create',
  value_type: 'dynamic_enum',
  locator: { kind: 'visible_text', value: 'demo-lazy-scope' },
  playwright_rpa: { operation: 'fill_text', scope: { kind: 'visible_text', value: 'demo-lazy-scope' }, target: { kind: 'role_name', value: 'textbox:demo-lazy-input' } },
  evidence_state: 'platform_pending',
}

function createUrl(markers = ''): string {
  return `https://ad.oceanengine.com/demo/form?aadvid=account-1${markers}`
}

function livePlan(manifest: CalibrationManifest, declared: Record<string, string> = {}) {
  return { ...buildDefaultPlan(manifest, accountHash), case_ids: manifest.coverage_cases.map(item => item.id), declared_conditions: declared }
}

function fullElements(): Map<string, FakeElement> {
  return new Map<string, FakeElement>([
    ['text:demo-purpose', { count: 1, visible: true }],
    ['text:demo-title', { count: 1, visible: true }],
    ['text:demo-edit-marker', { count: 1, visible: true }],
    ['text:demo-list', { count: 1, visible: true }],
    ['text:demo-choice-scope', { count: 1, visible: true }],
    ['text:demo-choice-target', { count: 1, visible: true, snapshot: 'target-snapshot' }],
    ['placeholder:demo-readback-input', { count: 1, visible: true, input: 'chosen-value' }],
    ['text:demo-toggle-section', { count: 1, visible: true }],
    ['attribute:data-e2e=demo-toggle', { count: 1, visible: true, snapshot: '', attributes: { 'aria-checked': 'false' } }],
    ['text:demo-lazy-scope', { count: 1, visible: true }],
    ['role:textbox:demo-lazy-input', { count: 1, visible: true, snapshot: 'lazy-snapshot', input: '' }],
  ])
}

function baseManifest(): CalibrationManifest {
  return manifestFixture([demoField, toggleField, pickerField, lazyField], [
    { id: 'OE-DEMO-CREATE', path: ['project_create', 'ecommerce'], field_keys: ['demo.choice_field', 'demo.toggle_field', 'demo.picker_field', 'demo.lazy_field'], status: 'covered' },
    { id: 'OE-DEMO-BLOCKED', path: ['project_create', 'other'], field_keys: ['demo.choice_field'], status: 'blocked_by_account_capability', reason: 'the branch stays closed for this account' },
    { id: 'OE-DEMO-EDIT', path: ['project_edit', 'existing'], field_keys: ['demo.choice_field'], status: 'covered' },
  ])
}

test('normalizes manifest locator kinds and rejects domain keys', () => {
  assert.deepEqual(normalizeLocator({ kind: 'visible_text', value: 'x' }), { kind: 'text', value: 'x' })
  assert.deepEqual(normalizeLocator({ kind: 'role_name', value: 'button:save-view' }), { kind: 'role', role: 'button', name: 'save-view' })
  assert.deepEqual(normalizeLocator({ kind: 'attribute', value: 'data-e2e=demo' }), { kind: 'attribute', name: 'data-e2e', value: 'demo' })
  assert.equal(normalizeLocator({ kind: 'domain_key', value: 'promotion_create' }), null)
  assert.equal(normalizeLocator({ kind: 'role_name', value: 'wizard:step' }), null)
  assert.equal(normalizeLocator({ kind: 'attribute', value: 'style=red' }), null)
})

test('persistent session accepts safe line requests and explicit exit', () => {
  assert.deepEqual(parsePersistentSessionRequest('OE-DEMO-CREATE'), { case_id: 'OE-DEMO-CREATE', declarations: [] })
  assert.deepEqual(parsePersistentSessionRequest('{"case_id":"OE-DEMO-CREATE","declare":["carrier=orange_landing_page"]}'), { case_id: 'OE-DEMO-CREATE', declarations: ['carrier=orange_landing_page'] })
  assert.equal(parsePersistentSessionRequest('  '), null)
  assert.equal(parsePersistentSessionRequest('exit'), 'exit')
  assert.equal(parsePersistentSessionRequest('quit'), 'exit')
  assert.throws(() => parsePersistentSessionRequest('{"case_id":"OE-DEMO-CREATE","unknown":true}'), /invalid_plan/)
  assert.throws(() => parsePersistentSessionRequest('not-a-case'), /invalid_plan/)
})

test('contradiction only fires on declared conflicting dimensions', () => {
  assert.equal(conditionContradicted(demoField, {}), false)
  assert.equal(conditionContradicted(demoField, { marketing_purpose: 'ecommerce' }), false)
  assert.equal(conditionContradicted(demoField, { marketing_purpose: 'product_catalog' }), true)
})

test('unmet condition dimensions name declared failing clauses including equals rules', () => {
  const deepOptimizationLike = {
    ...demoField,
    condition_rule: { all: [
      { dimension: 'marketing_purpose', operator: 'equals' as const, values: ['ecommerce'] },
      { dimension: 'carrier', operator: 'in' as const, values: ['orange_landing_page', 'owned_landing_page'] },
      { dimension: 'optimization_target_semantic_key', operator: 'equals' as const, values: ['in_app_order'] },
    ] },
  }
  assert.deepEqual(unmetConditionDimensions(deepOptimizationLike, {}), [])
  assert.deepEqual(unmetConditionDimensions(deepOptimizationLike, { marketing_purpose: 'ecommerce' }), [])
  // Declared but failing clauses are named; undeclared ones stay unknown.
  assert.deepEqual(unmetConditionDimensions(deepOptimizationLike, { marketing_purpose: 'application', carrier: 'orange_landing_page' }), ['marketing_purpose'])
  assert.deepEqual(unmetConditionDimensions(deepOptimizationLike, { marketing_purpose: 'ecommerce', carrier: 'independent_private_page', optimization_target_semantic_key: 'in_app_order' }), ['carrier'])
  assert.deepEqual(unmetConditionDimensions(deepOptimizationLike, { marketing_purpose: 'lead_generation', carrier: 'independent_private_page', optimization_target_semantic_key: 'form_submit' }), ['marketing_purpose', 'carrier', 'optimization_target_semantic_key'])
})

test('observations record unmet condition dimensions from declared plan state', async () => {
  const manifest = baseManifest()
  const plan = livePlan(manifest, { marketing_purpose: 'product_catalog' })
  const envelope = await executeCaseObservation({ plan, manifest, caseId: 'OE-DEMO-CREATE', page: new FakePage(createUrl('&is_create=1'), fullElements()), sessionContextSha256: hash })
  const choice = envelope.fields[0]
  assert.equal(choice.status, 'blocked_by_condition')
  assert.deepEqual(choice.unmet_condition_dimensions, ['marketing_purpose'])
  const lazy = envelope.fields[3]
  assert.equal(lazy.status, 'observed')
  assert.equal(lazy.unmet_condition_dimensions, undefined)
})

test('detects page families through fingerprints and URL markers', async () => {
  const manifest = baseManifest()
  const detectWith = async (url: string, elements: Map<string, FakeElement>) => {
    const page = new FakePage(url, elements)
    return await detectPageKind(new URL(url), manifest, async locator => await page.locate(locator).count())
  }
  const full = fullElements()
  // A create/edit URL marker is authoritative: it promotes the named form
  // family over coinciding list fingerprints instead of tying with them.
  assert.equal(await detectWith(createUrl('&is_create=1'), full), 'project_create')
  assert.equal(await detectWith(createUrl('&is_update=1'), full), 'project_edit')
  assert.equal(await detectWith(createUrl(), full), 'project_list')
  // Without a marker the matched form fingerprints still identify the form
  // page once no list fingerprint competes.
  const formOnly = new Map([...full].filter(([key]) => key !== 'text:demo-list' && key !== 'text:demo-edit-marker'))
  assert.equal(await detectWith(createUrl(), formOnly), 'project_create')
  // Promotion forms are identified by their recorded URL shape; the fixture
  // page carries none of the project-family fingerprint text.
  const empty = new Map<string, FakeElement>()
  assert.equal(await detectWith('https://ad.oceanengine.com/superior/ads?aadvid=account-1&is_create=1', empty), 'promotion_create')
  assert.equal(await detectWith('https://ad.oceanengine.com/superior/ads?aadvid=account-1&is_update=1', empty), 'promotion_edit')
  // Nothing matches on a page outside every family.
  assert.equal(await detectWith('https://ad.oceanengine.com/unrelated', empty), 'unknown')
})

test('executes a covered case into hashed per-field observations', async () => {
  const manifest = baseManifest()
  const plan = livePlan(manifest)
  const envelope = await executeCaseObservation({ plan, manifest, caseId: 'OE-DEMO-CREATE', page: new FakePage(createUrl('&is_create=1'), fullElements()), sessionContextSha256: hash })
  assert.equal(envelope.outcome, 'success')
  assert.equal(envelope.error_code, 'ok')
  assert.equal(envelope.page?.page_kind, 'project_create')
  assert.equal(envelope.page?.create_marker_present, true)
  assert.deepEqual(envelope.fields.map(item => item.status), ['observed', 'observed', 'blocked_spec', 'observed'])
  const choice = envelope.fields[0]
  assert.equal(choice.readback.value_kind, 'text')
  assert.ok(choice.readback.value_sha256)
  const serialized = JSON.stringify(envelope)
  assert.ok(!serialized.includes('chosen-value'))
  assert.ok(!serialized.includes('account-1'))
  const { evidence_sha256, ...rest } = envelope
  assert.equal(evidence_sha256, observationEvidenceHash(rest))
})

test('uses the first unique visible alternate locator for branch-specific controls', async () => {
  const alternateField: ManifestField = {
    ...demoField,
    locator: { kind: 'attribute', value: 'data-auto-id=primary-scope' },
    playwright_rpa: {
      operation: 'choose_exact_visible_option',
      scope: { kind: 'attribute', value: 'data-auto-id=primary-scope' },
      scope_alternates: [{ kind: 'attribute', value: 'data-e2e=alternate-scope' }],
      target: { kind: 'attribute', value: 'data-auto-id=primary-target' },
      target_alternates: [{ kind: 'attribute', value: 'data-e2e=alternate-target' }],
      expected_target_count: 1,
      readback: { kind: 'attribute', value: 'data-auto-id=primary-readback' },
      readback_alternates: [{ kind: 'attribute', value: 'data-e2e=alternate-readback' }],
    },
  }
  const manifest = manifestFixture([alternateField], [
    { id: 'OE-DEMO-ALTERNATE', path: ['project_create', 'ecommerce'], field_keys: [alternateField.key], status: 'covered' },
  ])
  const elements = fullElements()
  elements.set('attribute:data-auto-id=primary-scope', { count: 1, visible: false })
  elements.set('attribute:data-e2e=alternate-scope', { count: 1, visible: true })
  elements.set('attribute:data-auto-id=primary-target', { count: 1, visible: false })
  elements.set('attribute:data-e2e=alternate-target', { count: 1, visible: true, snapshot: 'alternate-target' })
  elements.set('attribute:data-auto-id=primary-readback', { count: 1, visible: false })
  elements.set('attribute:data-e2e=alternate-readback', { count: 1, visible: true, input: 'alternate-value' })
  const envelope = await executeCaseObservation({
    plan: livePlan(manifest, { marketing_purpose: 'ecommerce' }),
    manifest,
    caseId: 'OE-DEMO-ALTERNATE',
    page: new FakePage(createUrl('&is_create=1'), elements),
    sessionContextSha256: hash,
  })
  assert.equal(envelope.outcome, 'success')
  assert.equal(envelope.fields[0]?.status, 'observed')
})

test('short-circuits blocked cases without touching the page', async () => {
  const manifest = baseManifest()
  const envelope = await executeCaseObservation({ plan: livePlan(manifest), manifest, caseId: 'OE-DEMO-BLOCKED', page: new FakePage('https://example.com/none', new Map()), sessionContextSha256: hash })
  assert.equal(envelope.outcome, 'blocked_case')
  assert.equal(envelope.error_code, 'case_blocked')
  assert.deepEqual(envelope.fields, [])
})

test('rejects wrong-page and account mismatches with controlled codes', async () => {
  const manifest = baseManifest()
  const plan = livePlan(manifest)
  const wrongPage = await executeCaseObservation({ plan, manifest, caseId: 'OE-DEMO-CREATE', page: new FakePage('https://ad.oceanengine.com/other/page?aadvid=account-1', fullElements()), sessionContextSha256: hash })
  assert.equal(wrongPage.outcome, 'rejected')
  assert.equal(wrongPage.error_code, 'page_type_mismatch')
  assert.match(wrongPage.reason ?? '', /^required_family=project_create detected=/)
  const mismatch = await executeCaseObservation({ plan: { ...plan, account_context: { ...plan.account_context, expected_sha256: 'c'.repeat(64) } }, manifest, caseId: 'OE-DEMO-CREATE', page: new FakePage(createUrl(), fullElements()), sessionContextSha256: hash })
  assert.equal(mismatch.error_code, 'account_mismatch')
  assert.equal(mismatch.outcome, 'rejected')
})

test('marks missing targets as partial instead of failing the run', async () => {
  const manifest = baseManifest()
  const plan = livePlan(manifest)
  const sparse = new Map<string, FakeElement>([
    ['text:demo-purpose', { count: 1, visible: true }],
    ['text:demo-title', { count: 1, visible: true }],
    ['text:demo-choice-scope', { count: 2, visible: true }],
  ])
  const envelope = await executeCaseObservation({ plan, manifest, caseId: 'OE-DEMO-CREATE', page: new FakePage(createUrl('&is_create=1'), sparse), sessionContextSha256: hash })
  assert.equal(envelope.outcome, 'partial')
  assert.equal(envelope.fields[0]?.status, 'scope_ambiguous')
  assert.equal(envelope.fields[1]?.status, 'scope_missing')
  assert.equal(envelope.fields[2]?.status, 'blocked_spec')
  assert.equal(envelope.fields[3]?.status, 'scope_missing')
})

test('observation envelopes satisfy the frozen schema and redaction boundary', () => {
  const ajv = new AjvConstructor({ allErrors: true, strict: false })
  installFormats(ajv)
  const schema = JSON.parse(readFileSync(join(root, 'docs/delivery/schemas/oceanengine-field-observation-v1.json'), 'utf8'))
  const validate = ajv.compile(schema)
  const success = {
    schema_version: 'oceanengine-field-observation/v1', manifest_id: 'test-manifest', plan_sha256: hash, case_id: 'OE-DEMO-CREATE', path: ['project_create', 'ecommerce'], declared_conditions: {}, observed_at: '2026-08-23T08:00:00.000Z',
    page: { host: 'ad.oceanengine.com', page_kind: 'project_create', path_sha256: hash, account_context_sha256: accountHash, account_context_state: 'matched', create_marker_present: false, edit_marker_present: false, session_context_sha256: hash, structure_summary: { main_count: 1, heading_count: 2, button_count: 3, textbox_count: 2, table_count: 1, row_count: 5 } },
    fields: [{ key: 'demo.choice_field', semantic_label_state: 'present', operation: 'choose_exact_visible_option', scope: { kind: 'resolved', element_count: 1, visible: true }, target: { kind: 'resolved', element_count: 1, visible: true, accessible_name_state: 'present', accessible_name_sha256: hash }, readback: { kind: 'resolved', resolved: true, value_kind: 'text', value_state: 'present', value_sha256: hash }, status: 'observed' }],
    outcome: 'success', error_code: 'ok', evidence_sha256: hash,
  }
  assert.equal(validate(success), true, JSON.stringify(validate.errors))
  const serialized = JSON.stringify(success)
  assert.doesNotMatch(serialized, /aadvid|cookie|token|余额|客户名|项目名|单元名|商品名|https?:\/\/|[?&][a-z_]+=/i)
  const { page: _page, ...withoutPage } = success as typeof success & { page?: unknown }
  const rejected = { ...withoutPage, outcome: 'rejected', error_code: 'page_type_mismatch', reason: 'required_family=project_create detected=promotion_list', fields: [] }
  assert.equal(validate(rejected), true, JSON.stringify(validate.errors))
  const poisoned = JSON.parse(serialized) as Record<string, unknown>
  poisoned.remoteWrite = true
  assert.equal(validate(poisoned), false)
})

test('validates plans against the manifest and closes unknown dimensions', async () => {
  const manifest = baseManifest()
  const plan = buildDefaultPlan(manifest, accountHash)
  assert.equal(validateFieldCapturePlan(plan, manifest).case_ids.length, 3)
  assert.throws(() => validateFieldCapturePlan({ ...plan, case_ids: ['OE-MISSING'] }, manifest), /invalid_plan/)
  assert.throws(() => validateFieldCapturePlan({ ...plan, declared_conditions: { marketing_purpose: 'unknown_value' } }, manifest), /invalid_plan/)
  assert.throws(() => validateFieldCapturePlan({ ...plan, declared_conditions: { nonexistent_dimension: 'x' } }, manifest), /invalid_plan/)
  // Condition-rule dimensions accept declarations even though they are not
  // path dimensions with closed value sets.
  const liveManifest = await loadCalibrationManifest(root)
  validateFieldCapturePlan({ ...buildDefaultPlan(liveManifest, accountHash), declared_conditions: { carrier: 'orange_landing_page', optimization_target_semantic_key: 'in_app_order' } }, liveManifest)
  assert.throws(() => validateFieldCapturePlan({ ...plan, allowed_hosts: ['evil.example.com'] }, manifest), /invalid_plan/)
  assert.equal(validateFieldCapturePlan(buildDefaultPlan(manifest, accountHash), manifest).manifest_ref.fixture, 'oceanengine-calibration-manifest-v1.json')
})

test('real calibration manifest integrity holds for capture consumption', async () => {
  const manifest = await loadCalibrationManifest(root)
  assert.equal(manifest.observation_boundary.remote_write_authorized, false)
  assert.equal(manifest.fields.length, 51)
  assert.equal(manifest.coverage_cases.length, 26)
  const fieldKeys = new Set(manifest.fields.map(item => item.key))
  for (const coverageCase of manifest.coverage_cases) {
    assert.ok(coverageCase.field_keys.every(key => fieldKeys.has(key)))
    assert.ok(validateFieldCapturePlan({ ...buildDefaultPlan(manifest, accountHash), case_ids: [coverageCase.id] }, manifest))
  }
  const promotionCreate = manifest.coverage_cases.find(item => item.id === 'OE-PROMOTION-CREATE')
  const promotionEdit = manifest.coverage_cases.find(item => item.id === 'OE-PROMOTION-EDIT')
  const projectDefault = manifest.coverage_cases.find(item => item.id === 'OE-PROJECT-E-COMMERCE-DEFAULT')
  const projectManual = manifest.coverage_cases.find(item => item.id === 'OE-PROJECT-E-COMMERCE-MANUAL')
  const projectLeadsSmart = manifest.coverage_cases.find(item => item.id === 'OE-PROJECT-LEADS-SMART')
  const projectLeadsCustom = manifest.coverage_cases.find(item => item.id === 'OE-PROJECT-LEADS-CUSTOM')
  const projectCatalog = manifest.coverage_cases.find(item => item.id === 'OE-PROJECT-CATALOG-DEFAULT')
  const projectContent = manifest.coverage_cases.find(item => item.id === 'OE-PROJECT-CONTENT-MARKETING')
  const projectAppDownload = manifest.coverage_cases.find(item => item.id === 'OE-PROJECT-APP-DOWNLOAD-IOS-HARMONY-PACKAGE')
  const projectAppLaunch = manifest.coverage_cases.find(item => item.id === 'OE-PROJECT-APP-LAUNCH')
  const projectAppAppointment = manifest.coverage_cases.find(item => item.id === 'OE-PROJECT-APP-APPOINTMENT-ANDROID-IOS')
  const projectEdit = manifest.coverage_cases.find(item => item.id === 'OE-PROJECT-EDIT')
  assert.ok(promotionCreate)
  assert.ok(promotionEdit)
  assert.ok(projectDefault)
  assert.ok(projectManual)
  assert.ok(projectLeadsSmart)
  assert.ok(projectLeadsCustom)
  assert.ok(projectCatalog)
  assert.ok(projectContent)
  assert.ok(projectAppDownload)
  assert.ok(projectAppLaunch)
  assert.ok(projectAppAppointment)
  assert.ok(projectEdit)
  assert.equal(promotionCreate.field_keys.length, 19)
  assert.equal(promotionEdit.field_keys.length, 20)
  assert.ok(promotionCreate.field_keys.every(key => promotionEdit.field_keys.includes(key)))
  assert.ok(promotionEdit.field_keys.includes('promotion.editable_surface'))
  assert.equal(projectDefault.field_keys.length, 12)
  assert.ok(projectDefault.field_keys.includes('project.aigc_dynamic_creative'))
  assert.ok(!projectDefault.field_keys.includes('project.placement_strategy'))
  assert.equal(projectManual.field_keys.length, 6)
  assert.ok(projectManual.field_keys.includes('project.placement_strategy'))
  assert.ok(!projectManual.field_keys.includes('project.aigc_dynamic_creative'))
  assert.ok(!projectManual.field_keys.includes('project.bid_minor'))
  assert.equal(projectLeadsSmart.field_keys.length, 13)
  assert.ok(projectLeadsSmart.field_keys.includes('project.lead_capture_mode'))
  assert.ok(projectLeadsSmart.field_keys.includes('project.bid_minor'))
  assert.equal(projectLeadsCustom.field_keys.length, 3)
  assert.deepEqual(projectLeadsCustom.field_keys, [
    'project.lead_capture_mode',
    'project.carrier',
    'project.optimization_target_reference',
  ])
  const fieldsByKey = new Map(manifest.fields.map(item => [item.key, item]))
  assert.deepEqual(fieldsByKey.get('project.lead_capture_mode')?.locator, {
    kind: 'attribute',
    value: 'data-e2e=createproject_assetType_multioption',
  })
  assert.deepEqual(fieldsByKey.get('project.carrier')?.locator, {
    kind: 'attribute',
    value: 'data-auto-id=assetType',
  })
  assert.deepEqual(fieldsByKey.get('project.bid_minor')?.locator, {
    kind: 'attribute',
    value: 'data-e2e=createproject_adBid',
  })
  assert.equal(projectCatalog.field_keys.length, 11)
  assert.ok(projectCatalog.field_keys.includes('project.product_catalog_reference'))
  assert.ok(projectCatalog.field_keys.includes('project.product_targeting'))
  assert.deepEqual(fieldsByKey.get('project.product_catalog_reference')?.locator, {
    kind: 'attribute',
    value: 'data-auto-id=productCatalogue',
  })
  assert.deepEqual(fieldsByKey.get('project.product_targeting')?.locator, {
    kind: 'attribute',
    value: 'data-auto-id=dpaAudienceModule',
  })
  assert.equal(projectContent.field_keys.length, 11)
  assert.ok(projectContent.field_keys.includes('project.carrier'))
  assert.ok(projectContent.field_keys.includes('project.optimization_target_reference'))
  assert.deepEqual(fieldsByKey.get('project.carrier')?.playwright_rpa.scope_alternates, [
    { kind: 'attribute', value: 'data-e2e=createproject_promotionType' },
  ])
  assert.deepEqual(fieldsByKey.get('project.optimization_target_reference')?.playwright_rpa.scope_alternates, [
    { kind: 'attribute', value: 'data-auto-id=optimizeTargetCrowdGrass' },
  ])
  assert.equal(projectAppDownload.field_keys.length, 12)
  assert.equal(projectAppLaunch.field_keys.length, 5)
  assert.equal(projectAppAppointment.field_keys.length, 4)
  assert.deepEqual(fieldsByKey.get('project.operating_system')?.locator, {
    kind: 'attribute',
    value: 'data-e2e=createproject_appTypeShift',
  })
  assert.deepEqual(fieldsByKey.get('project.operating_system')?.playwright_rpa.scope_alternates, [
    { kind: 'attribute', value: 'data-auto-id=subscribeAppTypeSelect' },
  ])
  assert.deepEqual(fieldsByKey.get('project.application_reference')?.locator, {
    kind: 'attribute',
    value: 'data-e2e=createproject_appselect_input__ocInput',
  })
  assert.deepEqual(fieldsByKey.get('project.application_reference')?.playwright_rpa.scope_alternates, [
    { kind: 'attribute', value: 'data-e2e=createproject_subscribeUrlInput_input__ocInput' },
  ])
  assert.deepEqual(fieldsByKey.get('project.bidding_strategy')?.playwright_rpa.scope_alternates, [
    { kind: 'attribute', value: 'data-e2e=createproject_flowcontrolmode' },
  ])
  assert.deepEqual(fieldsByKey.get('project.operational_state')?.playwright_rpa.scope, {
    kind: 'attribute',
    value: 'data-e2e=promotion_project_on-off',
  })
  assert.deepEqual(fieldsByKey.get('promotion.operational_state')?.playwright_rpa.scope, {
    kind: 'attribute',
    value: 'data-e2e=promotion_promotion_on-off',
  })
  assert.deepEqual(fieldsByKey.get('promotion.material_replacement_edit')?.playwright_rpa.readback, {
    kind: 'visible_text',
    value: '共22个视频',
  })
  assert.equal(projectEdit.field_keys.length, 17)
  for (const key of [
    'project.marketing_product_reference',
    'project.optimization_target_reference',
    'project.placement_strategy',
    'project.targeting',
    'project.schedule',
    'project.daily_budget_minor',
    'project.monitoring_references',
    'project.project_name',
    'project.editable_surface',
  ]) assert.ok(projectEdit.field_keys.includes(key))
})

test('capture implementation has no mutation, navigation, screenshot, or browser close call', () => {
  const source = readFileSync(join(root, 'scripts/oceanengine-field-capture-runner.ts'), 'utf8')
  for (const forbidden of [/\.click\s*\(/, /\.fill\s*\(/, /\.press\s*\(/, /\.goto\s*\(/, /\.evaluate\s*\(/, /\.screenshot\s*\(/, /\.setInputFiles\s*\(/, /selectOption|\.check\s*\(|\.uncheck\s*\(/, /clipboard/i, /browser\.close\s*\(/, /Browser\.close/]) assert.doesNotMatch(source, forbidden)
  assert.doesNotMatch(source, /nth-child|\.nth\s*\(/)
  assert.doesNotMatch(source, /class\s*[~*^$|]?=/)
  assert.match(source, /chromium\.connectOverCDP\s*\(/)
  assert.equal(source.match(/chromium\.connectOverCDP\s*\(/g)?.length, 1)
  assert.match(source, /mode: 'persistent', connection_count: 1/)
  assert.match(source, /process\.exit\s*\(/)
})
