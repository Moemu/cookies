import assert from 'node:assert/strict'
import { existsSync, readFileSync } from 'node:fs'
import { join, resolve } from 'node:path'
import test from 'node:test'

const root = resolve(import.meta.dirname, '..')
const manifest = JSON.parse(readFileSync(join(root, 'docs/delivery/fixtures/oceanengine-calibration-manifest-v1.json'), 'utf8')) as {
  session_evidence_ref: string
  fields: Array<{ key: string; consumers: string[] }>
  coverage_cases: Array<{ id: string; status: string }>
  consumer_mappings: Array<{ field_key: string; destination: string; treatment: string }>
}

test('OceanEngine calibration manifest drives consumer and coverage checks', () => {
  assert.ok(existsSync(join(root, manifest.session_evidence_ref)))
  const fieldKeys = new Set(manifest.fields.map(field => field.key))
  const consumers = new Set<string>()
  for (const mapping of manifest.consumer_mappings) {
    assert.ok(fieldKeys.has(mapping.field_key), `unknown manifest field: ${mapping.field_key}`)
    consumers.add(mapping.destination)
  }
  for (const destination of ['DeliveryIntent', 'OceanEngineConfiguration', 'DeliveryDecisionCandidate', 'CompiledDeliveryWorkflow', 'PlatformSkill']) {
    assert.ok(consumers.has(destination), `missing manifest consumer: ${destination}`)
  }
  assert.ok(manifest.coverage_cases.some(item => item.status === 'blocked_by_event_asset'))
  assert.ok(manifest.coverage_cases.every(item => item.id.startsWith('OE-')))
})
