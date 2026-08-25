import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { join, resolve } from 'node:path'
import test from 'node:test'
import Ajv2020 from 'ajv/dist/2020.js'
import addFormats from 'ajv-formats'
import { buildCoverageLedger, classifyCoveragePage, coverageLedgerEvidenceHash, coveragePaths, coverageSchemaVersionV2, mergeCoverageLedgers, runDirectory, upgradeLegacyLedger, type CoverageLedger, type CoverageObservation } from '../scripts/oceanengine-readonly-coverage-ledger.js'

type Validator = { compile(schema: Record<string, unknown>): ((value: unknown) => boolean) & { errors?: unknown } }
const AjvConstructor = Ajv2020 as unknown as new (options: { allErrors: boolean; strict: boolean }) => Validator
const installFormats = addFormats as unknown as (validator: Validator) => void
const root = resolve(import.meta.dirname, '..')
const hash = 'a'.repeat(64)
function locator(key: CoverageObservation['locator_observations'][number]['key'], count = 1) { return { key, kind: 'visible_text' as const, count, visible: count === 1, ...(count === 1 ? { accessible_name_sha256: hash } : {}) } }
function observation(phase: 'before' | 'after'): CoverageObservation {
  return { schema_version: 'oceanengine-readonly-coverage-observation/v1', phase, observed_at: phase === 'before' ? '2026-08-21T08:00:00.000Z' : '2026-08-21T08:01:00.000Z', https: true, host_allowed: true, edit_marker_present: false, create_marker_present: false, page_kind: 'report_overview', path_sha256: hash, account_context_sha256: 'b'.repeat(64), account_context_state: 'matched', session_context_sha256: 'c'.repeat(64), locator_observations: [locator('data_center'), locator('account_report', 2), locator('project_report'), locator('promotion_report'), locator('material_report')], structure: { button_count: 4, textbox_count: 0, table_count: 4, row_count: 12, observable_object_count: 8 }, state_summary: { population_state: 'populated', enabled_count: 0, disabled_count: 0, draft_marker_count: 0, loading_marker_count: 0, empty_marker_count: 0 }, page_state_sha256: 'd'.repeat(64) }
}

test('classifies semantic page fingerprints and create/edit markers', () => {
  assert.equal(classifyCoveragePage(observation('before').locator_observations), 'report_overview')
  assert.equal(classifyCoveragePage([locator('create_project'), locator('project_budget_column')]), 'project_list')
  assert.equal(classifyCoveragePage([locator('promotion_name'), locator('save_and_close')], true), 'promotion_edit')
  assert.equal(classifyCoveragePage([locator('promotion_name'), locator('save_and_close')], false, true), 'promotion_create')
  assert.equal(classifyCoveragePage([locator('promotion_name'), locator('save_and_close')]), 'promotion_detail_or_edit')
  assert.equal(classifyCoveragePage([], false, false), 'unknown')
})

test('builds nine target states and bounded no-write proof', () => {
  const ledger = buildCoverageLedger(observation('before'), observation('after'), 51)
  assert.equal(ledger.schema_version, coverageSchemaVersionV2)
  assert.equal(ledger.targets.length, 9)
  assert.deepEqual(new Set(ledger.targets.map(item => item.status)), new Set(['confirmed_shell', 'not_accessible']))
  assert.equal(ledger.targets.find(item => item.target === 'report_overview')?.status, 'confirmed_shell')
  assert.equal(ledger.no_write_proof.proof_state, 'confirmed')
  assert.equal(ledger.no_write_proof.total_new_object_count, 0)
  assert.equal(ledger.rejected_locators[0]?.reason, 'unscoped_duplicate')
})

test('marks changed observable state as verification_pending', () => {
  const after = observation('after'); after.page_state_sha256 = 'e'.repeat(64); after.structure.observable_object_count = 9
  const ledger = buildCoverageLedger(observation('before'), after, 51)
  assert.equal(ledger.no_write_proof.proof_state, 'verification_pending')
  assert.equal(ledger.no_write_proof.total_new_object_count, 1)
  assert.equal(ledger.targets.find(item => item.target === 'report_overview')?.status, 'blocked')
})

test('accepts explicit empty report state', () => {
  const before = observation('before'); const after = observation('after')
  before.structure.table_count = 0; after.structure.table_count = 0; before.state_summary.empty_marker_count = 4; after.state_summary.empty_marker_count = 4; before.state_summary.population_state = 'empty'; after.state_summary.population_state = 'empty'; before.page_state_sha256 = 'f'.repeat(64); after.page_state_sha256 = 'f'.repeat(64)
  assert.equal(buildCoverageLedger(before, after, 51).targets.find(item => item.target === 'report_overview')?.status, 'confirmed_shell')
})

test('merges confirmed evidence from separate current-page observations', () => {
  const projectBefore = observation('before'); const projectAfter = observation('after'); projectBefore.page_kind = 'project_list'; projectAfter.page_kind = 'project_list'; projectBefore.locator_observations = [locator('create_project'), locator('project_budget_column')]; projectAfter.locator_observations = projectBefore.locator_observations
  const promotionBefore = observation('before'); const promotionAfter = observation('after'); promotionBefore.page_kind = 'promotion_list'; promotionAfter.page_kind = 'promotion_list'; promotionBefore.path_sha256 = 'f'.repeat(64); promotionAfter.path_sha256 = 'f'.repeat(64); promotionBefore.locator_observations = [locator('create_promotion'), locator('promotion_budget_column')]; promotionAfter.locator_observations = promotionBefore.locator_observations
  const merged = mergeCoverageLedgers(buildCoverageLedger(projectBefore, projectAfter, 51), buildCoverageLedger(promotionBefore, promotionAfter, 51))
  assert.equal(merged.targets.find(item => item.target === 'project_list')?.status, 'confirmed_shell'); assert.equal(merged.targets.find(item => item.target === 'promotion_list')?.status, 'confirmed_shell'); assert.equal(merged.page_evidence.length, 2); assert.equal(merged.no_write_proof.confirmed_page_count, 2)
})

test('v2 contract rejects free text and remote write', () => {
  const ajv = new AjvConstructor({ allErrors: true, strict: false }); installFormats(ajv); const schema = JSON.parse(readFileSync(join(root, 'api/contracts/oceanengine-readonly-coverage-ledger-v2.schema.json'), 'utf8')); const validate = ajv.compile(schema); const ledger = buildCoverageLedger(observation('before'), observation('after'), 51) as unknown as Record<string, unknown>; assert.equal(validate(ledger), true, JSON.stringify(validate.errors)); ledger.remoteWrite = true; ledger.note = 'free text'; assert.equal(validate(ledger), false)
})

test('upgrades v1 detail-or-edit targets to v2 edit shells', () => {
  const upgraded = upgradeLegacyLedger(JSON.parse(readFileSync(join(root, 'api/fixtures/oceanengine-readonly-coverage-ledger-v1.json'), 'utf8'))); assert.equal(upgraded.schema_version, coverageSchemaVersionV2); assert.equal(upgraded.targets.length, 9); assert.equal(upgraded.targets.find(item => item.target === 'project_edit')?.status, 'confirmed_shell'); assert.equal(upgraded.targets.find(item => item.target === 'promotion_edit')?.status, 'confirmed_shell')
})

test('resolves run-scoped paths and rejects traversal', () => {
  const paths = coveragePaths('C:\\local-data'); assert.equal(paths.activeRun, resolve('C:\\local-data', 'cookies/browser-rpa/calibration/active-run.json')); assert.equal(runDirectory(paths, '2026-08-21T08-00-00-000Z'), join(paths.runs, '2026-08-21T08-00-00-000Z')); assert.throws(() => runDirectory(paths, '../escape'), /run id is invalid/)
})

test('v1 fixture remains valid and v2 fixture satisfies contract and redaction boundary', () => {
  const ajv = new AjvConstructor({ allErrors: true, strict: false }); installFormats(ajv)
  const v1Text = readFileSync(join(root, 'api/fixtures/oceanengine-readonly-coverage-ledger-v1.json'), 'utf8'); const v1Schema = ajv.compile(JSON.parse(readFileSync(join(root, 'api/contracts/oceanengine-readonly-coverage-ledger-v1.schema.json'), 'utf8'))); assert.equal(v1Schema(JSON.parse(v1Text)), true); assert.doesNotMatch(v1Text, /aadvid|cookie|token|余额|客户名|项目名|单元名|商品名|https?:\/\/|[?&][a-z_]+=/i)
  const v2Text = readFileSync(join(root, 'api/fixtures/oceanengine-readonly-coverage-ledger-v2.json'), 'utf8'); const fixture = JSON.parse(v2Text) as CoverageLedger; const validate = ajv.compile(JSON.parse(readFileSync(join(root, 'api/contracts/oceanengine-readonly-coverage-ledger-v2.schema.json'), 'utf8'))); assert.equal(validate(fixture), true, JSON.stringify(validate.errors)); assert.doesNotMatch(v2Text, /aadvid|cookie|token|余额|客户名|项目名|单元名|商品名|https?:\/\/|[?&][a-z_]+=/i); const { evidence_sha256: evidenceHash, ...evidence } = fixture; assert.equal(evidenceHash, coverageLedgerEvidenceHash(evidence))
})

test('coverage implementation has no mutation, navigation, screenshot, download, or browser close call', () => {
  const source = readFileSync(join(root, 'scripts/oceanengine-readonly-coverage-ledger.ts'), 'utf8'); for (const forbidden of [/\.click\s*\(/, /\.fill\s*\(/, /\.press\s*\(/, /\.goto\s*\(/, /\.evaluate\s*\(/, /\.screenshot\s*\(/, /\.setInputFiles\s*\(/, /clipboard/i, /browser\.close\s*\(/, /Browser\.close/]) assert.doesNotMatch(source, forbidden); assert.doesNotMatch(source, /nth-child|\.nth\s*\(/); assert.doesNotMatch(source, /class\s*[~*^$|]?=/); assert.match(source, /chromium\.connectOverCDP\s*\(/); assert.match(source, /process\.exit\s*\(/)
})
