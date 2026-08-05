import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import test from 'node:test'
import Ajv2020 from 'ajv/dist/2020.js'
import addFormats from 'ajv-formats'

test('partial media artifacts remain explicit, attributable, and schema-valid', () => {
  const schema = JSON.parse(readFileSync(join(
    process.cwd(),
    'api',
    'contracts',
    'platform-media-understanding-artifact-v1.schema.json',
  ), 'utf8'))
  const ajv = new Ajv2020({ allErrors: true, strict: false })
  addFormats(ajv)
  const validate = ajv.compile(schema)
  const ref = {
    project_id: 'project_1',
    asset_version: { asset_id: 'asset_1', version: 1 },
  }
  const value = {
    contract_version: 'platform-media-understanding-artifact/v1',
    id: 'media_1',
    organization_id: 'org_1',
    project_id: 'project_1',
    asset_ref: ref,
    asset_kind: 'image',
    asset_sha256: 'a'.repeat(64),
    profile: 'strategy.multimodal.p0',
    profile_version: 'v1',
    input_identity_hash: 'b'.repeat(64),
    status: 'partial',
    summary: '素材已完成技术校验；当前没有可采信的语义理解。',
    visible_text: [],
    observations: [{
      id: 'observation_01',
      text: '素材技术校验通过：image/png，960 × 540',
      confidence: 1,
      locator: { kind: 'image', asset_ref: ref },
    }],
    inferences: [],
    risks: [],
    unknowns: [],
    keyframes: [],
    transcript: [],
    warnings: ['vision_route_unavailable'],
    model_lineage: {
      model_alias: 'cookies.vision.standard',
      prompt_version: 'media.understand.v1',
      schema_version: 'media-understanding-output/v1',
    },
    content_hash: 'c'.repeat(64),
    created_by: { kind: 'user', id: 'user_1' },
    created_at: '2026-08-04T17:00:00Z',
    updated_at: '2026-08-04T17:00:01Z',
  }

  assert.equal(validate(value), true, JSON.stringify(validate.errors))
  assert.equal(validate({ ...value, surprise_claim: 'not allowed' }), false)
})
