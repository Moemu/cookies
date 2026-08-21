import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { join, resolve } from 'node:path'
import test from 'node:test'
import Ajv2020 from 'ajv/dist/2020.js'
import addFormats from 'ajv-formats'

const root = resolve(import.meta.dirname, '..')
const schema = JSON.parse(readFileSync(join(root, 'docs', 'delivery', 'schemas', 'delivery-calibration-case-v1.json'), 'utf8'))
const fixture = JSON.parse(readFileSync(join(root, 'docs', 'delivery', 'fixtures', 'delivery-calibration-case-v1-valid.json'), 'utf8'))

function validator() {
  const ajv = new Ajv2020({ allErrors: true, strict: true })
  addFormats(ajv)
  return ajv.compile(schema)
}

test('historical calibration case is Plan-independent and schema-valid', () => {
  const validate = validator()
  assert.equal(validate(fixture), true, JSON.stringify(validate.errors))
  assert.deepEqual(fixture.source_binding.cookies_plan_binding, { state: 'unbound_historical', plan_id: null, plan_version: null })
})

test('calibration export rejects raw platform identity and post-launch diagnosis', () => {
  const validate = validator()
  const rawIdentity = structuredClone(fixture)
  rawIdentity.source_binding.account_ref = 'raw-account-id'
  assert.equal(validate(rawIdentity), false)

  const diagnosis = structuredClone(fixture)
  diagnosis.platform_diagnosis = { label: 'winner' }
  assert.equal(validate(diagnosis), false)
})

test('operational disposition cannot become a material quality label', () => {
  const validate = validator()
  const candidate = structuredClone(fixture)
  candidate.labels.operational_outcome.is_material_quality_label = true
  assert.equal(validate(candidate), false)
})
