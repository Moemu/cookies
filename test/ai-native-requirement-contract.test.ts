import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import test from 'node:test'
import Ajv2020 from 'ajv/dist/2020.js'
import addFormats from 'ajv-formats'

const schema = JSON.parse(readFileSync(resolve('api/contracts/creative-ai-native-requirement-v2.schema.json'), 'utf8'))
const ajv = new Ajv2020({ allErrors: true, strict: true, strictRequired: false })
addFormats(ajv)
const validate = ajv.compile(schema)

function incompleteDraft() {
  return {
    contract_version: 'creative.ai-native.requirement/v2', revision: 1, status: 'draft',
    product: {
      source: 'taobao', product_id: '123456789', name: '', description: '', images: [],
      price: { min_raw: 0, max_raw: 0, currency: 'CNY', display_unconfirmed: true },
      sales: 0, source_url: 'https://item.taobao.com/item.htm?id=123456789',
    },
    product_resolution: {
      status: 'manual_required', source: 'taobao', resource_type: 'product', external_id: '123456789',
      source_url: 'https://item.taobao.com/item.htm?id=123456789', missing_fields: ['product_name', 'images'],
    },
    product_name: '', product_description: '', target_audiences: [], media: [], core_selling_points: [], supplemental_requirement: '',
    channel: 'douyin', aspect_ratio: '9:16', duration_seconds: 20, language: 'zh-CN',
    output_preset: {
      id: 'douyin_feed_9x16_v1', label: '抖音信息流 · 9:16', channel: 'douyin', placement: 'feed', aspect_ratio: '9:16',
      width: 720, height: 1280, resolution: '720p', profile_id: 'douyin.performance.v1', profile_version: 'v1',
      profile_hash: 'a'.repeat(64), safe_zone: { top: 96, right: 48, bottom: 240, left: 48 },
    },
    delivery_treatment: {
      preset: 'full_ad', voiceover_mode: 'generated', caption_mode: 'from_voiceover', sales_overlay_mode: 'key_points', music_sfx_mode: 'auto',
    },
    needs_confirmation: [],
    generation: { mode: 'deterministic_fallback', model_alias: 'fixture.deterministic', model_version: 'partial-v1', prompt_version: 'ai-native-requirement/douyin-v1' },
  }
}

test('AI native requirement v2 allows an incomplete persisted draft', () => {
  const draft = incompleteDraft()
  assert.equal(validate(draft), true, ajv.errorsText(validate.errors))
})

test('AI native requirement v2 rejects voiceover captions when voiceover is disabled', () => {
  const draft = incompleteDraft()
  draft.delivery_treatment.voiceover_mode = 'none'
  assert.equal(validate(draft), false)
})

test('AI native requirement v2 rejects a contradictory fixed delivery preset', () => {
  const draft = incompleteDraft()
  draft.delivery_treatment.caption_mode = 'editorial'
  assert.equal(validate(draft), false)
})
