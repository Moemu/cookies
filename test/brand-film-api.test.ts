import assert from 'node:assert/strict'
import test from 'node:test'
import { api, type ApiBrandBriefAnalysis } from '../src/data/api.ts'

test('brand film fixture creation is project-scoped and idempotent', async () => {
  const originalFetch = globalThis.fetch
  const calls: Array<{ url: string; init: RequestInit }> = []
  globalThis.fetch = async (input, init = {}) => {
    calls.push({ url: String(input), init })
    return jsonResponse({})
  }
  try {
    await api.ensureBrandFilmFixtureWorkspace('project_demo')
  } finally {
    globalThis.fetch = originalFetch
  }
  assert.equal(calls[0].url, '/api/creative/v1/projects/project_demo/creative-workspaces/brand-film:ensure-fixture')
  assert.equal(calls[0].init.method, 'POST')
  assert.equal(new Headers(calls[0].init.headers).get('Idempotency-Key'), 'brand-film-fixture-project_demo-guerlain-v1')
  assert.equal(calls[0].init.body, undefined)
})

test('brand Brief edits preserve confirmed uploaded asset lineage', async () => {
  const originalFetch = globalThis.fetch
  const calls: Array<{ url: string; init: RequestInit }> = []
  globalThis.fetch = async (input, init = {}) => {
    calls.push({ url: String(input), init })
    return jsonResponse({})
  }
  const analysis = {
    asset_candidates: [{
      id: 'product', role: 'product_front', label: '商品正面图', source_locator: 'manual-upload',
      asset_ref: { asset_id: 'asset_1', version: 2 }, rights_status: 'user_confirmed', user_confirmed: true,
    }],
  } as ApiBrandBriefAnalysis
  try {
    await api.updateBrandFilmBrief('project_demo', 'task_1', 4, analysis)
  } finally {
    globalThis.fetch = originalFetch
  }
  assert.equal(calls[0].url, '/api/creative/v1/projects/project_demo/creative-tasks/task_1/brand-film/brief')
  assert.equal(calls[0].init.method, 'PATCH')
  assert.deepEqual(JSON.parse(String(calls[0].init.body)), { expected_revision: 4, analysis })
})

test('brand-film commands bind selection and plan generation to the current revision', async () => {
  const originalFetch = globalThis.fetch
  const calls: Array<{ url: string; init: RequestInit }> = []
  globalThis.fetch = async (input, init = {}) => {
    calls.push({ url: String(input), init })
    return jsonResponse({})
  }
  try {
    await api.selectBrandFilmConcept('project_demo', 'task_1', 6, 'concept_01')
    await api.generateBrandFilmPlan('project_demo', 'task_1', 7)
  } finally {
    globalThis.fetch = originalFetch
  }
  assert.deepEqual(JSON.parse(String(calls[0].init.body)), { expected_revision: 6, concept_id: 'concept_01' })
  assert.equal(new Headers(calls[0].init.headers).get('Idempotency-Key'), 'brand-film-select-task_1-6-concept_01')
  assert.deepEqual(JSON.parse(String(calls[1].init.body)), { expected_revision: 7 })
  assert.equal(new Headers(calls[1].init.headers).get('Idempotency-Key'), 'brand-film-plan-task_1-7')
})

test('brand audio preparation and mix edits stay revision-bound', async () => {
  const originalFetch = globalThis.fetch
  const calls: Array<{ url: string; init: RequestInit }> = []
  globalThis.fetch = async (input, init = {}) => {
    calls.push({ url: String(input), init })
    return jsonResponse({})
  }
  try {
    await api.prepareBrandFilmAudio('project_demo', 'task_1', 18)
    await api.materializeBrandFilmAudioAssets('project_demo', 'task_1', 19)
    await api.updateBrandFilmAudioMix('project_demo', 'task_1', 20, [
      { op: 'set_track_gain', track_id: 'track_music', gain_db: -21 },
      { op: 'set_track_muted', track_id: 'track_sfx', muted: true },
      { op: 'replace_clip_asset', clip_id: 'voice_clip_01', asset_ref: { asset_id: 'asset_voice', version: 1 } },
    ])
    await api.renderBrandFilmAudioPreview('project_demo', 'task_1', 21)
    await api.selectBrandFilmAudioVariant('project_demo', 'task_1', 22, 'audio_variant_immersive_water_zh_cn')
  } finally {
    globalThis.fetch = originalFetch
  }
  assert.equal(calls[0].url, '/api/creative/v1/projects/project_demo/creative-tasks/task_1/brand-film:prepare-audio')
  assert.deepEqual(JSON.parse(String(calls[0].init.body)), { expected_revision: 18 })
  assert.equal(new Headers(calls[0].init.headers).get('Idempotency-Key'), 'brand-film-audio-task_1-18')
  assert.equal(calls[1].url, '/api/creative/v1/projects/project_demo/creative-tasks/task_1/brand-film:materialize-audio-assets')
  assert.deepEqual(JSON.parse(String(calls[1].init.body)), { expected_revision: 19 })
  assert.equal(new Headers(calls[1].init.headers).get('Idempotency-Key'), 'brand-film-audio-assets-task_1-19')
  assert.equal(calls[2].url, '/api/creative/v1/projects/project_demo/creative-tasks/task_1/brand-film/audio/mix')
  assert.deepEqual(JSON.parse(String(calls[2].init.body)), {
    expected_revision: 20,
    operations: [
      { op: 'set_track_gain', track_id: 'track_music', gain_db: -21 },
      { op: 'set_track_muted', track_id: 'track_sfx', muted: true },
      { op: 'replace_clip_asset', clip_id: 'voice_clip_01', asset_ref: { asset_id: 'asset_voice', version: 1 } },
    ],
  })
  assert.equal(calls[3].url, '/api/creative/v1/projects/project_demo/creative-tasks/task_1/brand-film:render-audio-preview')
  assert.deepEqual(JSON.parse(String(calls[3].init.body)), { expected_revision: 21 })
  assert.equal(new Headers(calls[3].init.headers).get('Idempotency-Key'), 'brand-film-audio-preview-task_1-21')
  assert.equal(calls[4].url, '/api/creative/v1/projects/project_demo/creative-tasks/task_1/brand-film/audio:select-variant')
  assert.deepEqual(JSON.parse(String(calls[4].init.body)), { expected_revision: 22, variant_id: 'audio_variant_immersive_water_zh_cn' })
  assert.equal(new Headers(calls[4].init.headers).get('Idempotency-Key'), 'brand-film-audio-variant-task_1-22-audio_variant_immersive_water_zh_cn')
})

test('brand narration capability and per-clip generation use the A3 routes', async () => {
  const originalFetch = globalThis.fetch
  const calls: Array<{ url: string; init: RequestInit }> = []
  globalThis.fetch = async (input, init = {}) => {
    calls.push({ url: String(input), init })
    return jsonResponse({ available: false, voice_aliases: [] })
  }
  try {
    await api.probeBrandFilmSpeech('project_demo')
    await api.generateBrandFilmVoiceClip('project_demo', 'task_1', 22, 'voice_clip_01', 'cookies.voice.brand.warm_female')
  } finally {
    globalThis.fetch = originalFetch
  }
  assert.equal(calls[0].url, '/api/creative/v1/projects/project_demo/brand-film/speech-capability')
  assert.equal(calls[0].init.method, 'GET')
  assert.equal(calls[1].url, '/api/creative/v1/projects/project_demo/creative-tasks/task_1/brand-film:generate-voice')
  assert.deepEqual(JSON.parse(String(calls[1].init.body)), {
    expected_revision: 22,
    clip_id: 'voice_clip_01',
    voice_alias: 'cookies.voice.brand.warm_female',
  })
  assert.equal(new Headers(calls[1].init.headers).get('Idempotency-Key'), 'brand-film-voice-task_1-22-voice_clip_01-cookies.voice.brand.warm_female')
})

function jsonResponse(value: unknown, status = 200): Response {
  return new Response(JSON.stringify(value), { status, headers: { 'Content-Type': 'application/json' } })
}
