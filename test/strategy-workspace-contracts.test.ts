import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { join, resolve } from 'node:path'
import test from 'node:test'
import Ajv2020, { type ValidateFunction } from 'ajv/dist/2020.js'
import addFormats from 'ajv-formats'

const repositoryRoot = resolve(import.meta.dirname, '..')
const contractsDirectory = join(repositoryRoot, 'api', 'contracts')
const fixturesDirectory = join(repositoryRoot, 'api', 'fixtures')

const fixtureContracts = [
  ['strategy-project-context-manifest-v1.json', 'strategy-project-context-manifest-v1.schema.json'],
  ['strategy-task-activity-v1-running.json', 'strategy-task-activity-v1.schema.json'],
  ['strategy-task-activity-snapshot-v1.json', 'strategy-task-activity-snapshot-v1.schema.json'],
  ['strategy-research-run-v2-partial.json', 'strategy-research-run-v2.schema.json'],
  ['strategy-research-finding-v1-conflicting.json', 'strategy-research-finding-v1.schema.json'],
  ['strategy-research-adoption-proposal-v1-stale.json', 'strategy-research-adoption-proposal-v1.schema.json'],
  ['platform-document-parse-v2-partial.json', 'platform-document-parse-v2.schema.json'],
  ['platform-document-vision-fallback-v1-unavailable.json', 'platform-document-vision-fallback-v1.schema.json'],
  ['platform-document-vision-reconciliation-v1-proposed.json', 'platform-document-vision-reconciliation-v1.schema.json'],
  ['platform-document-vision-reconciliation-candidate-v1.json', 'platform-document-vision-reconciliation-candidate-v1.schema.json'],
  ['strategy-draft-v3.json', 'strategy-draft-v3.schema.json'],
  ['strategy-product-event-v1.json', 'strategy-product-event-v1.schema.json'],
  ['strategy-model-capabilities-v1.json', 'strategy-model-capabilities-v1.schema.json'],
  ['strategy-conversation-memory-v2.json', 'strategy-conversation-memory-v2.schema.json'],
  ['strategy-workspace-ux-metrics-v1.json', 'strategy-workspace-ux-metrics-v1.schema.json'],
] as const

const ajv = new Ajv2020({ allErrors: true, allowUnionTypes: true, strict: true, strictRequired: false })
addFormats(ajv)

function readJSON(path: string): Record<string, unknown> {
  return JSON.parse(readFileSync(path, 'utf8')) as Record<string, unknown>
}

const validators = new Map<string, ValidateFunction>()
const schemaFilenames = new Set([
  ...fixtureContracts.map(([, schema]) => schema),
  'document-vision-evaluation-dataset-v1.schema.json',
  'document-vision-evaluation-report-v1.schema.json',
  'strategy-package-v3.schema.json',
  'strategy-section-patch-v1.schema.json',
])
for (const schemaFilename of schemaFilenames) {
  const schema = readJSON(join(contractsDirectory, schemaFilename))
  ajv.addSchema(schema)
}
for (const schemaFilename of schemaFilenames) {
  const schema = readJSON(join(contractsDirectory, schemaFilename))
  const validate = ajv.getSchema(String(schema.$id))
  assert.ok(validate, `schema ${schemaFilename} was not registered`)
  validators.set(schemaFilename, validate)
}

function validator(schemaFilename: string): ValidateFunction {
  const validate = validators.get(schemaFilename)
  assert.ok(validate, `schema ${schemaFilename} was not registered`)
  return validate
}

test('Strategy workspace v2 boundary fixtures satisfy their frozen schemas', () => {
  for (const [fixtureFilename, schemaFilename] of fixtureContracts) {
    const validate = validator(schemaFilename)
    const fixture = readJSON(join(fixturesDirectory, fixtureFilename))
    assert.equal(validate(fixture), true, `${fixtureFilename}: ${ajv.errorsText(validate.errors)}`)
  }
})

test('core workspace contracts cover valid, invalid, stale and partial scenario semantics', () => {
  const coreContracts = [
    ['strategy-project-context-manifest-v1.json', 'strategy-project-context-manifest-v1.schema.json'],
    ['strategy-task-activity-v1-running.json', 'strategy-task-activity-v1.schema.json'],
    ['strategy-research-run-v2-partial.json', 'strategy-research-run-v2.schema.json'],
    ['strategy-research-finding-v1-conflicting.json', 'strategy-research-finding-v1.schema.json'],
    ['strategy-research-adoption-proposal-v1-stale.json', 'strategy-research-adoption-proposal-v1.schema.json'],
    ['platform-document-parse-v2-partial.json', 'platform-document-parse-v2.schema.json'],
    ['strategy-draft-v3.json', 'strategy-draft-v3.schema.json'],
  ] as const
  for (const [fixtureFilename, schemaFilename] of coreContracts) {
    const validate = validator(schemaFilename)
    const fixture = readJSON(join(fixturesDirectory, fixtureFilename))
    assert.equal(validate(fixture), true, `${fixtureFilename} valid: ${ajv.errorsText(validate.errors)}`)
    const invalid = structuredClone(fixture)
    delete invalid.contract_version
    assert.equal(validate(invalid), false, `${fixtureFilename} accepted a missing required contract version`)
  }

  const manifest = readJSON(join(fixturesDirectory, 'strategy-project-context-manifest-v1.json'))
  const validateManifest = validator('strategy-project-context-manifest-v1.schema.json')
  assert.equal(validateManifest({ ...manifest, stage: 'intake', brief_ref: null, strategy_ref: null, selected_source_refs: [] }), true,
    'an early-stage partial context manifest must remain representable')

  const activity = readJSON(join(fixturesDirectory, 'strategy-task-activity-v1-running.json'))
  const validateActivity = validator('strategy-task-activity-v1.schema.json')
  assert.equal(validateActivity({ ...activity, status: 'stalled', phase: 'lease_expired' }), true,
    'stale runtime work must have an explicit stalled representation')
  assert.equal(validateActivity({ ...activity, status: 'partially_completed', phase: 'completed_with_gaps' }), true,
    'partial runtime work must remain visible rather than becoming success')

  const finding = readJSON(join(fixturesDirectory, 'strategy-research-finding-v1-conflicting.json'))
  const validateFinding = validator('strategy-research-finding-v1.schema.json')
  assert.equal(validateFinding({ ...finding, status: 'tentative', conflicting_source_ids: [] }), true,
    'a partial-evidence finding must remain tentative')

  const proposal = readJSON(join(fixturesDirectory, 'strategy-research-adoption-proposal-v1-stale.json'))
  const validateProposal = validator('strategy-research-adoption-proposal-v1.schema.json')
  assert.equal(validateProposal(proposal), true, ajv.errorsText(validateProposal.errors))
  assert.equal(validateProposal({ ...proposal, status: 'partially_completed' }), false,
    'partial execution state must not leak into the proposal lifecycle')

  const draft = readJSON(join(fixturesDirectory, 'strategy-draft-v3.json'))
  const validateDraft = validator('strategy-draft-v3.schema.json')
  const partialDraft = structuredClone(draft)
  delete partialDraft.creative_strategy
  assert.equal(validateDraft(partialDraft), false, 'a partial Strategy v3 must not be publishable')
})

test('workspace contracts reject hidden reasoning, fake verification, and unbounded writes', () => {
  const activity = readJSON(join(fixturesDirectory, 'strategy-task-activity-v1-running.json'))
  const validateActivity = validator('strategy-task-activity-v1.schema.json')
  assert.equal(validateActivity({ ...activity, chain_of_thought: 'must never leave the model boundary' }), false)

  const finding = readJSON(join(fixturesDirectory, 'strategy-research-finding-v1-conflicting.json'))
  const validateFinding = validator('strategy-research-finding-v1.schema.json')
  assert.equal(validateFinding({ ...finding, status: 'verified', supporting_source_ids: [] }), false)

  const proposal = readJSON(join(fixturesDirectory, 'strategy-research-adoption-proposal-v1-stale.json'))
  const validateProposal = validator('strategy-research-adoption-proposal-v1.schema.json')
  assert.equal(validateProposal({ ...proposal, status: 'proposed' }), false, 'non-stale proposal retained stale_reason')
  const operations = proposal.operations as Array<Record<string, unknown>>
  assert.equal(validateProposal({ ...proposal, operations: [...operations, { op: 'copy', field_path: 'constraints', value: [] }] }), false)
})

test('partial and page progress semantics stay explicit', () => {
  const run = readJSON(join(fixturesDirectory, 'strategy-research-run-v2-partial.json'))
  const validateRun = validator('strategy-research-run-v2.schema.json')
  assert.equal(validateRun({ ...run, status: 'succeeded' }), false, 'unknown terminal status was accepted')

  const parse = readJSON(join(fixturesDirectory, 'platform-document-parse-v2-partial.json'))
  const validateParse = validator('platform-document-parse-v2.schema.json')
  assert.equal(validateParse({ ...parse, total_pages: null }), false, 'page progress accepted an unknown page total')
  assert.equal(validateParse({ ...parse, parse_progress: 101 }), false, 'parse progress exceeded 100')
})

test('document vision reconciliation contract preserves two-operator evidence state', () => {
  const proposal = readJSON(join(fixturesDirectory, 'platform-document-vision-reconciliation-v1-proposed.json'))
  const validate = validator('platform-document-vision-reconciliation-v1.schema.json')
  assert.equal(validate(proposal), true, ajv.errorsText(validate.errors))
  assert.equal(validate({ ...proposal, confirmed_by: proposal.proposed_by, confirmed_at: proposal.proposed_at }), false)
  const notAccepted = { ...proposal, decision: 'not_accepted' }
  delete notAccepted.external_task_id
  assert.equal(validate(notAccepted), true, ajv.errorsText(validate.errors))
  assert.equal(validate({ ...proposal, decision: 'not_accepted' }), false)
})

test('document vision evaluation contracts require blinded timing evidence and immutable lineage', () => {
  const validateDataset = validator('document-vision-evaluation-dataset-v1.schema.json')
  const evaluationCase = {
    id: 'synthetic-scan-1',
    category: 'scanned_pdf',
    source_sha256: 'a'.repeat(64),
    source_mime_type: 'application/pdf',
    page_numbers: [1],
    gold_markdown: '# 标题\n结论',
    text_baseline_markdown: '乱码',
    hybrid_markdown: '# 标题\n结论',
    baseline_parser_code: 'tika',
    baseline_parser_version: '3.2.3',
    hybrid_parser_code: 'cookies-hybrid',
    hybrid_parser_version: 'v1',
    hybrid_model_alias: 'cookies.document.vision.standard',
    hybrid_route_revision_id: 'route-test-v1',
    hybrid_prompt_version: 'vision-prompt-test-v1',
    outputs_blinded: true,
    reviews: [
      {
        blind_label_id: 'blind-synthetic-scan-1-a',
        reviewer_id: 'reviewer-a',
        review_order: 'baseline_first',
        baseline_corrections: 8,
        hybrid_corrections: 2,
        baseline_review_and_correction_ms: 120_000,
        hybrid_review_and_correction_ms: 30_000,
      },
      {
        blind_label_id: 'blind-synthetic-scan-1-b',
        reviewer_id: 'reviewer-b',
        review_order: 'hybrid_first',
        baseline_corrections: 8,
        hybrid_corrections: 2,
        baseline_review_and_correction_ms: 120_000,
        hybrid_review_and_correction_ms: 30_000,
      },
    ],
    adjudicated: true,
    adjudicator_id: 'adjudicator-c',
    baseline_latency_ms: 100,
    hybrid_latency_ms: 900,
    hybrid_billable_pages: 1,
    hybrid_cost_millicny: 5,
  }
  const dataset = {
    contract_version: 'document-vision-evaluation-dataset/v1',
    dataset_id: 'synthetic-contract-test-v1',
    label_policy_version: 'label-policy-test-v1',
    redaction_policy_version: 'redaction-policy-test-v1',
    cost_policy_version: 'las-pricing-test-v1',
    collected_at: '2026-08-11T10:00:00+08:00',
    deidentified: true,
    cases: [evaluationCase],
  }
  assert.equal(validateDataset(dataset), true, ajv.errorsText(validateDataset.errors))
  assert.equal(validateDataset({ ...dataset, cases: [{ ...evaluationCase, outputs_blinded: false }] }), false)
  assert.equal(validateDataset({ ...dataset, cases: [{ ...evaluationCase, reviews: evaluationCase.reviews.slice(0, 1) }] }), false)
  assert.equal(validateDataset({ ...dataset, cases: [{ ...evaluationCase, reviews: [{ ...evaluationCase.reviews[0], hybrid_review_and_correction_ms: 0 }, evaluationCase.reviews[1]] }] }), false)
  assert.equal(validateDataset({ ...dataset, cases: [{ ...evaluationCase, raw_provider_response: 'must not be retained' }] }), false)

  const presentation = {
    ...evaluationCase,
    id: 'synthetic-ppt-1',
    category: 'chinese_ppt',
    source_mime_type: 'application/vnd.openxmlformats-officedocument.presentationml.presentation',
  }
  assert.equal(validateDataset({ ...dataset, cases: [presentation] }), false, 'presentation lineage omitted converter')
  assert.equal(validateDataset({ ...dataset, cases: [{ ...presentation, converter_code: 'gotenberg', converter_version: '8.34.0' }] }), true, ajv.errorsText(validateDataset.errors))

  const validateReport = validator('document-vision-evaluation-report-v1.schema.json')
  const report = {
    contract_version: 'document-vision-evaluation-report/v1',
    dataset_id: 'synthetic-contract-test-v1',
    dataset_sha256: 'b'.repeat(64),
    label_policy_version: 'label-policy-test-v1',
    cost_policy_version: 'las-pricing-test-v1',
    case_count: 1,
    review_count: 2,
    category_count: 1,
    cases_by_category: { scanned_pdf: 1 },
    review_orders_by_category: { scanned_pdf: { baseline_first: 1 } },
    mean_baseline_quality: 0.1,
    mean_hybrid_quality: 1,
    mean_quality_gain: 0.9,
    worst_case_regression: 0,
    correction_count_reduction: 0.75,
    total_baseline_correction_time_ms: 120_000,
    total_hybrid_correction_time_ms: 30_000,
    total_correction_time_saved_ms: 90_000,
    mean_baseline_correction_time_ms: 120_000,
    mean_hybrid_correction_time_ms: 30_000,
    median_baseline_correction_time_ms: 120_000,
    median_hybrid_correction_time_ms: 30_000,
    correction_time_reduction: 0.75,
    mean_baseline_latency_ms: 100,
    mean_hybrid_latency_ms: 900,
    total_hybrid_billable_pages: 1,
    total_hybrid_cost_millicny: 5,
    auto_enable_allowed: false,
    blockers: ['LABELLED_CATEGORY_COVERAGE_INSUFFICIENT'],
  }
  assert.equal(validateReport(report), true, ajv.errorsText(validateReport.errors))
  assert.equal(validateReport({ ...report, dataset_sha256: 'untraceable' }), false)
})

test('StrategyDraft v3 owns structured creative strategy and rejects the removed v2 write field', () => {
  const draft = readJSON(join(fixturesDirectory, 'strategy-draft-v3.json'))
  const validate = validator('strategy-draft-v3.schema.json')
  assert.equal(validate(draft), true, ajv.errorsText(validate.errors))
  assert.equal(validate({ ...draft, creative_recommendations: ['legacy writer must stay closed'] }), false)
  const withoutCreativeStrategy = { ...draft }
  delete withoutCreativeStrategy.creative_strategy
  assert.equal(validate(withoutCreativeStrategy), false)
})

test('StrategyPackage v3 freezes a v3 strategy snapshot and section writes stay v3-only', () => {
  const draft = readJSON(join(fixturesDirectory, 'strategy-draft-v3.json'))
  const validatePackage = validator('strategy-package-v3.schema.json')
  const strategyPackage = {
    contract_version: 'strategy-package/v3',
    package_id: 'strategypackage_03',
    package_version: 1,
    organization_id: 'org_01',
    project_id: 'project_01',
    strategy_id: 'strategy_03',
    strategy_revision: 2,
    brief: { brief_id: 'brief_01', version: 1 },
    strategy: draft,
    readiness: { publish_blockers: [], creative_ready: true, delivery_ready: true, insights_ready: true },
    approval: {
      review_id: 'strategyreview_03',
      approved_by: 'user_01',
      approved_at: '2026-08-10T08:10:00Z',
      content_hash: `sha256:${'a'.repeat(64)}`,
    },
  }
  assert.equal(validatePackage(strategyPackage), true, ajv.errorsText(validatePackage.errors))
  assert.equal(validatePackage({ ...strategyPackage, contract_version: 'strategy-package/v2' }), false)

  const validatePatch = validator('strategy-section-patch-v1.schema.json')
  assert.equal(validatePatch({ expected_version: 2, base_revision: 1, section: 'creative_strategy', value: draft.creative_strategy }), true, ajv.errorsText(validatePatch.errors))
  assert.equal(validatePatch({ expected_version: 2, base_revision: 1, section: 'creative_recommendations', value: ['legacy'] }), false)
})

test('product event contract cannot carry prompts or nested arbitrary payloads', () => {
  const event = readJSON(join(fixturesDirectory, 'strategy-product-event-v1.json'))
  const validate = validator('strategy-product-event-v1.schema.json')
  assert.equal(validate({ ...event, attributes: { prompt: 'sensitive' } }), false)
  assert.equal(validate({ ...event, attributes: { status: { raw: 'payload' } } }), false)
})

test('fixed model capability contract rejects aliases selected by a dynamic score', () => {
  const manifest = readJSON(join(fixturesDirectory, 'strategy-model-capabilities-v1.json'))
  const validate = validator('strategy-model-capabilities-v1.schema.json')
  const items = manifest.items as Array<Record<string, unknown>>
  const mutated = { ...manifest, items: items.map((item, index) => index === 0 ? { ...item, score: 0.94 } : item) }
  assert.equal(validate(mutated), false)
  const duplicate = { ...manifest, items: items.map((item, index) => index === 5 ? { ...item, capability: 'research.web' } : item) }
  assert.equal(validate(duplicate), false, 'manifest accepted a missing and duplicated capability')
})
