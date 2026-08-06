import assert from 'node:assert/strict'
import test from 'node:test'
import * as React from 'react'
import { renderToStaticMarkup } from 'react-dom/server'
import { RequirementMediaGallery } from '../src/features/ai-native-ad/RequirementStage'
import { StoryboardStage } from '../src/features/ai-native-ad/StoryboardStage'
import { aiNativeReducer, initialAINativeState } from '../src/features/ai-native-ad/reducer'
import type { AdScriptDraft, AINativeRequirement, AINativeRequirementWorkspace, StoryboardDraft } from '../src/features/ai-native-ad/types'
import { aiNativeWorkspaceLocation, readAINativeWorkspaceLocation } from '../src/features/ai-native-ad/navigation'
import { readAINativeStageDraft, readAINativeWorkspacePointer, rememberAINativeStageDraft, rememberAINativeWorkspace } from '../src/features/ai-native-ad/storage'
import { createSerialAutosave } from '../src/features/ai-native-ad/autosave'

const requirement: AINativeRequirement = {
  contract_version: 'creative.ai-native.requirement/v1',
  revision: 1,
  status: 'draft',
  product: {
    source: 'douyin_mall',
    product_id: 'product-1',
    name: '通勤背包',
    description: '一款适合日常通勤的背包',
    images: [{ url: 'https://example.com/product.jpg', role: 'main' }],
    price: { min_raw: 100, max_raw: 200, currency: 'CNY', display_unconfirmed: true },
    sales: 0,
    source_url: 'https://v.douyin.com/example/',
  },
  product_name: '通勤背包',
  product_description: '一款适合日常通勤的背包',
  target_audiences: [{ id: 'audience-1', text: '城市通勤人群' }],
  media: [{ id: 'media-1', url: 'https://example.com/product.jpg', role: 'main', source: 'douyin_mall' }],
  core_selling_points: [{ id: 'point-1', text: '可扩容设计' }, { id: 'point-2', text: '透气背板' }],
  supplemental_requirement: '真实自然',
  channel: 'douyin',
  aspect_ratio: '9:16',
  duration_seconds: 20,
  language: 'zh-CN',
  needs_confirmation: [],
  generation: { mode: 'model', model_alias: 'cookies.text.standard', model_version: 'test', prompt_version: 'ai-native-requirement/douyin-v1' },
}

test('AI native ad remembers the latest workspace pointer per project', () => {
  const values = new Map<string, string>()
  const storage = {
    getItem: (key: string) => values.get(key) ?? null,
    setItem: (key: string, value: string) => { values.set(key, value) },
    removeItem: (key: string) => { values.delete(key) },
  }

  rememberAINativeWorkspace('project-1', { workspaceId: 'workspace-1', stage: 'storyboard' }, storage)

  assert.deepEqual(readAINativeWorkspacePointer('project-1', storage), { workspaceId: 'workspace-1', stage: 'storyboard' })
  assert.equal(readAINativeWorkspacePointer('project-2', storage), null)
})

test('AI native ad ignores corrupt or unsupported workspace pointers', () => {
  const values = new Map<string, string>([
    ['cookies.ai-native.current.v1:project-1', '{bad json'],
    ['cookies.ai-native.current.v1:project-2', JSON.stringify({ version: 2, workspaceId: 'workspace-2', stage: 'script' })],
  ])
  const storage = {
    getItem: (key: string) => values.get(key) ?? null,
    setItem: (key: string, value: string) => { values.set(key, value) },
    removeItem: (key: string) => { values.delete(key) },
  }

  assert.equal(readAINativeWorkspacePointer('project-1', storage), null)
  assert.equal(readAINativeWorkspacePointer('project-2', storage), null)
})

test('AI native ad only restores a local stage draft against the same server revision', () => {
  const values = new Map<string, string>()
  const storage = {
    getItem: (key: string) => values.get(key) ?? null,
    setItem: (key: string, value: string) => { values.set(key, value) },
    removeItem: (key: string) => { values.delete(key) },
  }
  const draft = { product_name: 'edited product' }

  rememberAINativeStageDraft('project-1', 'workspace-1', 'requirement', 3, draft, storage)

  assert.deepEqual(readAINativeStageDraft('project-1', 'workspace-1', 'requirement', 3, storage), draft)
  assert.equal(readAINativeStageDraft('project-1', 'workspace-1', 'requirement', 4, storage), null)
  assert.equal(readAINativeStageDraft('project-1', 'workspace-2', 'requirement', 3, storage), null)
})

test('AI native autosave serializes writes and keeps only the latest pending edit', async () => {
  const releases: Array<() => void> = []
  const started: string[] = []
  const autosave = createSerialAutosave<string>({
    delayMs: 0,
    fingerprint: value => value,
    save: async value => {
      started.push(value)
      await new Promise<void>(resolve => { releases.push(resolve) })
    },
  })

  autosave.schedule('first')
  await new Promise(resolve => setTimeout(resolve, 0))
  autosave.schedule('second')
  autosave.schedule('latest')
  assert.deepEqual(started, ['first'])
  releases.shift()?.()
  await new Promise(resolve => setTimeout(resolve, 0))
  assert.deepEqual(started, ['first', 'latest'])
  releases.shift()?.()
  await autosave.flush()
  autosave.dispose()
})

test('AI native autosave skips content whose fingerprint is already saved', async () => {
  const started: string[] = []
  const autosave = createSerialAutosave<string>({ delayMs: 0, fingerprint: value => value, save: async value => { started.push(value) } })

  autosave.schedule('same')
  await autosave.flush()
  autosave.schedule('same')
  await autosave.flush()

  assert.deepEqual(started, ['same'])
  autosave.dispose()
})

Object.assign(globalThis, { React })

const workspace: AINativeRequirementWorkspace = {
  workspace_id: 'workspace-1',
  creative_intake_id: 'intake-1',
  creative_task_id: 'task-1',
  organization_id: 'org-1',
  project_id: 'project-1',
  status: 'confirmed',
  current_stage: 'requirement',
  workspace_version: 2,
  current_revision: 1,
  confirmed_revision: 1,
  requirement,
  created_by: 'user-1',
  confirmed_by: 'user-1',
  created_at: '2026-08-04T00:00:00Z',
  updated_at: '2026-08-04T00:00:00Z',
}

const script: AdScriptDraft = {
  contract_version: 'creative.ai-native.script/v1',
  revision: 1,
  status: 'draft',
  title: '通勤背包完整脚本',
  creative_summary: '从通勤痛点切入，用动作证明卖点并完成行动引导。',
  channel_profile_id: 'douyin.performance.v1',
  channel_profile_hash: 'a'.repeat(64),
  duration_seconds: 20,
  segments: [
    { id: 'segment-1', start_ms: 0, end_ms: 4000, purpose: 'hook', visual_intent: '展示通勤痛点', voiceover: '通勤包总是不够装？', subtitle: '通勤包不够装', selling_point_ids: [] },
    { id: 'segment-2', start_ms: 4000, end_ms: 15000, purpose: 'proof', visual_intent: '动作展示扩容结构', voiceover: '可扩容设计，收纳更从容。', subtitle: '可扩容设计', selling_point_ids: ['point-1'] },
    { id: 'segment-3', start_ms: 15000, end_ms: 20000, purpose: 'cta', visual_intent: '商品定格收束', voiceover: '点击了解这款通勤背包。', subtitle: '点击了解更多', selling_point_ids: [], conversion_action: '点击了解商品' },
  ],
  based_on_requirement_revision: 1,
  based_on_requirement_hash: 'b'.repeat(64),
  generation: { model_alias: 'cookies.text.standard', model_version: 'test', prompt_version: 'ai-ad-script/douyin/v1', profile_hash: 'a'.repeat(64) },
}

test('需求图片媒介不直接渲染商品源站外链且为旧数据提供重新提取入口', () => {
  const markup = renderToStaticMarkup(React.createElement(RequirementMediaGallery, {
    media: requirement.media,
    previews: {},
    status: 'draft',
    onReanalyze: () => undefined,
  }))

  assert.doesNotMatch(markup, /src="https:\/\/example\.com\/product\.jpg"/)
  assert.match(markup, /旧版链接素材，需重新提取/)
  assert.match(markup, /重新提取商品素材/)
})

test('需求图片媒介只把项目素材预览地址写入 img', () => {
  const media = [{ ...requirement.media[0], asset_ref: { asset_id: 'asset-product', version: 1 } }]
  const markup = renderToStaticMarkup(React.createElement(RequirementMediaGallery, {
    media,
    previews: { 'media-1': '/platform/assets/asset-product/preview' },
    status: 'draft',
    onReanalyze: () => undefined,
  }))

  assert.match(markup, /src="\/platform\/assets\/asset-product\/preview"/)
  assert.doesNotMatch(markup, /src="https:\/\/example\.com\/product\.jpg"/)
  assert.doesNotMatch(markup, /重新提取商品素材/)
})

test('项目素材预览失败时显示可理解的占位状态而不是破图', () => {
  const media = [{ ...requirement.media[0], asset_ref: { asset_id: 'asset-product', version: 1 } }]
  const markup = renderToStaticMarkup(React.createElement(RequirementMediaGallery, {
    media,
    previews: { 'media-1': null },
    status: 'draft',
    onReanalyze: () => undefined,
  }))

  assert.match(markup, /素材预览暂不可用/)
  assert.doesNotMatch(markup, /<img/)
})

test('AI 原生广告工作区地址可恢复 workspace 与阶段且保留项目查询参数', () => {
  const next = aiNativeWorkspaceLocation('?view=效果广告&section=ai-native', 'workspace / 1', 'requirement')
  assert.equal(next, '?view=%E6%95%88%E6%9E%9C%E5%B9%BF%E5%91%8A&section=ai-native&workspace=workspace+%2F+1&stage=requirement')
  assert.deepEqual(readAINativeWorkspaceLocation(next), { workspaceId: 'workspace / 1', stage: 'requirement' })
})

test('AI 效果广告前端恢复后端返回的一个完整闭合脚本', () => {
  assert.equal(script.duration_seconds, 20)
  assert.equal(script.segments[0].start_ms, 0)
  assert.equal(script.segments.at(-1)?.end_ms, 20000)
  script.segments.forEach((segment, index) => {
    assert.ok(segment.end_ms > segment.start_ms)
    if (index > 0) assert.equal(segment.start_ms, script.segments[index - 1].end_ms)
  })
})

test('确认需求后进入单脚本生成，确认脚本后进入故事板生成', () => {
  const afterRequirement = aiNativeReducer(initialAINativeState, { type: 'requirement-confirmed', workspace })
  assert.equal(afterRequirement.active_stage, 'script')
  assert.equal(afterRequirement.stage_status.requirement, 'confirmed')
  assert.equal(afterRequirement.stage_status.script, 'generating')

  const afterScript = aiNativeReducer(afterRequirement, { type: 'script-generated', script })
  const afterConfirm = aiNativeReducer(afterScript, { type: 'script-confirmed' })
  assert.equal(afterConfirm.active_stage, 'storyboard')
  assert.equal(afterConfirm.stage_status.script, 'confirmed')
  assert.equal(afterConfirm.stage_status.storyboard, 'generating')
})

test('故事板启动失败不会把已确认脚本标记为失败或继续无限转圈', () => {
  const afterRequirement = aiNativeReducer(initialAINativeState, { type: 'requirement-confirmed', workspace })
  const afterScript = aiNativeReducer(afterRequirement, { type: 'script-generated', script })
  const afterConfirm = aiNativeReducer(afterScript, { type: 'script-confirmed' })
  const failed = aiNativeReducer(afterConfirm, { type: 'operation-failed', stage: 'storyboard', message: '故事板启动失败' })

  assert.equal(failed.active_stage, 'storyboard')
  assert.equal(failed.stage_status.script, 'confirmed')
  assert.equal(failed.stage_status.storyboard, 'failed')
  assert.equal(failed.error, '故事板启动失败')
})

test('故事板启动失败后展示错误和重新生成入口', () => {
  const props = {
    projectId: 'project-1',
    storyboard: null,
    canGenerate: true,
    onChange: () => undefined,
    onSave: () => undefined,
    onConfirm: () => undefined,
    onEdit: () => undefined,
    onRetry: () => undefined,
  } as const
  const markup = renderToStaticMarkup(React.createElement(StoryboardStage, {
    ...props,
    status: 'failed',
    error: '故事板启动失败，请稍后重试。',
  }))
  const refreshedMarkup = renderToStaticMarkup(React.createElement(StoryboardStage, {
    ...props,
    status: 'empty',
    error: '',
  }))

  assert.match(markup, /故事板启动失败，请稍后重试。/)
  assert.match(markup, /重新生成故事板/)
  assert.doesNotMatch(markup, /正在生成故事板/)
  assert.match(refreshedMarkup, /重新生成故事板/)
})

test('故事板部分素材失败时保留成功素材并提供仅重试失败素材入口', () => {
  const partialStoryboard: StoryboardDraft = {
    contract_version: 'creative.ai-native.storyboard/v1', revision: 1, status: 'draft', duration_seconds: 20,
    assets: [
      { id: 'person_1', role: 'person_identity', name: '通勤男士', source: 'ai_generated', status: 'ready', asset_ref: { asset_id: 'asset_person_1', version: 1 } },
      { id: 'scene_1', role: 'scene_reference', name: '地铁通勤场景', source: 'ai_generated', status: 'failed', generation_brief: '明亮地铁站', generation_attempt: 1, error_code: 'AI_NATIVE_STORYBOARD_ASSET_FAILED', error_message: '图片服务暂时不可用' },
    ],
    shots: [], channel_profile_id: 'douyin.performance.v1', channel_profile_hash: 'a'.repeat(64),
    based_on_requirement_revision: 1, based_on_requirement_hash: 'b'.repeat(64), based_on_script_revision: 1, based_on_script_hash: 'c'.repeat(64),
    generation: { model_alias: 'cookies.text.standard', model_version: 'test', prompt_version: 'ai-ad-storyboard/douyin/v1', profile_hash: 'a'.repeat(64) },
  }
  const markup = renderToStaticMarkup(React.createElement(StoryboardStage, {
    projectId: 'project-1', status: 'failed', storyboard: partialStoryboard, canGenerate: true, error: '部分图片生成失败',
    onChange: () => undefined, onSave: () => undefined, onConfirm: () => undefined, onEdit: () => undefined, onRetry: () => undefined,
  }))

  assert.match(markup, /Asset asset_person_1/)
  assert.match(markup, /图片服务暂时不可用/)
  assert.match(markup, /仅重试失败素材/)
})

test('恢复失败的故事板工作区时展示服务端具体原因', () => {
  const restored = aiNativeReducer(initialAINativeState, {
    type: 'requirement-loaded',
    workspace: {
      ...workspace,
      current_stage: 'storyboard',
      storyboard_status: 'failed',
      storyboard_error_code: 'AI_NATIVE_STORYBOARD_GENERATION_FAILED',
      storyboard_error_message: '素材 product_1 与固定商品素材 ID 重复',
    },
  })

  assert.equal(restored.stage_status.storyboard, 'failed')
  assert.equal(restored.error, '素材 product_1 与固定商品素材 ID 重复')
})

test('重新编辑脚本会作废故事板和视频但保留需求', () => {
  const storyboard: StoryboardDraft = {
    contract_version: 'creative.ai-native.storyboard/v1', revision: 1, status: 'confirmed', duration_seconds: 20,
    assets: [], shots: [], channel_profile_id: 'douyin.performance.v1', channel_profile_hash: 'a'.repeat(64),
    based_on_requirement_revision: 1, based_on_requirement_hash: 'b'.repeat(64), based_on_script_revision: 1, based_on_script_hash: 'c'.repeat(64),
    generation: { model_alias: 'cookies.text.standard', model_version: 'test', prompt_version: 'ai-ad-storyboard/douyin/v1', profile_hash: 'a'.repeat(64) },
  }
  const readyState = {
    ...initialAINativeState,
    workspace,
    script,
    storyboard,
    video: { progress: 100, current_step: '完成', completed_shots: storyboard.shots.length, total_shots: storyboard.shots.length, eta_seconds: 0 },
    stage_status: { requirement: 'confirmed', script: 'confirmed', storyboard: 'confirmed', video: 'confirmed' } as const,
  }
  const requested = aiNativeReducer(readyState, { type: 'reopen-requested', stage: 'script' })
  const reopened = aiNativeReducer(requested, { type: 'reopen-confirmed' })
  assert.equal(reopened.stage_status.requirement, 'confirmed')
  assert.equal(reopened.stage_status.script, 'draft')
  assert.equal(reopened.stage_status.storyboard, 'invalidated')
  assert.equal(reopened.stage_status.video, 'invalidated')
  assert.equal(reopened.storyboard, null)
  assert.equal(reopened.video, null)

  const edited = aiNativeReducer(reopened, {
    type: 'requirement-edited',
    workspace: { ...workspace, requirement: { ...requirement, product_name: '编辑后的商品名称' } },
  })
  assert.equal(edited.stage_status.requirement, 'confirmed')
})

test('重新编辑需求时本地修改不会把表单意外重新锁定', () => {
  const confirmedState = aiNativeReducer(initialAINativeState, { type: 'requirement-confirmed', workspace })
  const requested = aiNativeReducer(confirmedState, { type: 'reopen-requested', stage: 'requirement' })
  const reopened = aiNativeReducer(requested, { type: 'reopen-confirmed' })
  const edited = aiNativeReducer(reopened, {
    type: 'requirement-edited',
    workspace: { ...workspace, requirement: { ...requirement, product_name: '编辑后的商品名称' } },
  })
  assert.equal(edited.stage_status.requirement, 'draft')
  assert.equal(edited.workspace?.requirement.product_name, '编辑后的商品名称')
})

test('视频阶段只采用服务端 ProductionProgress 且素材就绪不冒充最终成片', () => {
  const productionWorkspace: AINativeRequirementWorkspace = {
    ...workspace,
    current_stage: 'production',
    production_status: 'running',
    production_progress: {
      status: 'running', progress_percent: 43, current_step: '正在生成视频片段',
      completed_video_units: 2, total_video_units: 4, completed_video_duration_ms: 10000,
      completed_speech_units: 3, total_speech_units: 3, eta_seconds: 120, available_actions: ['cancel_production'],
    },
  }
  const running = aiNativeReducer(initialAINativeState, { type: 'requirement-loaded', workspace: productionWorkspace })
  assert.equal(running.video?.progress, 43)
  assert.equal(running.video?.completed_shots, 2)
  assert.equal(running.stage_status.video, 'generating')

  const ready = aiNativeReducer(running, { type: 'requirement-loaded', workspace: {
    ...productionWorkspace,
    production_status: 'assets_ready',
    production_progress: { ...productionWorkspace.production_progress!, status: 'assets_ready', progress_percent: 70, current_step: '视频片段与旁白已就绪', available_actions: [] },
  } })
  assert.equal(ready.stage_status.video, 'draft')
  assert.equal(ready.video?.status, 'assets_ready')
  assert.notEqual(ready.stage_status.video, 'confirmed')

  const completedWorkspace: AINativeRequirementWorkspace = {
    ...productionWorkspace,
    production_status: 'completed',
    production_plan: { ...productionWorkspace.production_plan!, status: 'completed', render: { id: 'render-1', status: 'completed', progress_percent: 100, eta_seconds: 0, renderer_version: 'ffmpeg-ai-ad-timeline/v1', output_asset_ref: { asset_id: 'final-video', version: 1 } } },
    production_progress: { ...productionWorkspace.production_progress!, status: 'completed', progress_percent: 100, current_step: '最终广告视频已生成', eta_seconds: 0, available_actions: ['preview', 'download'] },
  }
  const completed = aiNativeReducer(ready, { type: 'requirement-loaded', workspace: completedWorkspace })
  assert.equal(completed.video?.status, 'completed')
  assert.equal(completed.video?.progress, 100)
  assert.equal(completed.stage_status.video, 'confirmed')
})

test('视频 Unit 提交失败后恢复具体原因而不是继续显示生成中', () => {
  const failedWorkspace: AINativeRequirementWorkspace = {
    ...workspace,
    current_stage: 'production',
    production_status: 'failed',
    production_plan: {
      contract_version: 'creative.ai-native.production-plan/v1', revision: 1, based_on_storyboard_revision: 1,
      based_on_storyboard_hash: 'a'.repeat(64), channel_profile_id: 'douyin.performance.v1', channel_profile_hash: 'b'.repeat(64),
      status: 'failed', total_duration_ms: 20000, aspect_ratio: '9:16', video_model_alias: 'cookies.video.standard', speech_model_alias: 'cookies.speech.standard',
      units: [{ id: 'video-unit-01', order: 1, shot_ids: ['shot-1'], start_ms: 0, end_ms: 4000, duration_seconds: 4, prompt: 'prompt', prompt_hash: 'c'.repeat(64), aspect_ratio: '9:16', resolution: '720p', product_identity_required: false,
        attempts: [{ id: 'attempt-1', ordinal: 1, status: 'failed', error_code: 'AI_NATIVE_VIDEO_SUBMISSION_FAILED', error_message: '视频任务创建失败：来源 ID 过长', created_at: '2026-08-05T08:00:00Z', updated_at: '2026-08-05T08:00:01Z' }] }],
      speech_units: [], created_at: '2026-08-05T08:00:00Z', updated_at: '2026-08-05T08:00:01Z',
    },
    production_progress: {
      status: 'failed', progress_percent: 5, current_step: '视频素材生成失败', completed_video_units: 0, total_video_units: 1,
      completed_video_duration_ms: 0, completed_speech_units: 0, total_speech_units: 0, eta_seconds: 0, available_actions: ['retry_failed_unit'],
    },
  }
  const failed = aiNativeReducer(initialAINativeState, { type: 'requirement-loaded', workspace: failedWorkspace })
  assert.equal(failed.stage_status.video, 'failed')
  assert.equal(failed.video?.status, 'failed')
  assert.match(failed.error, /来源 ID 过长/)
})

test('脚本后台任务失败后恢复具体原因而不是继续显示生成中', () => {
  const failedWorkspace: AINativeRequirementWorkspace = {
    ...workspace,
    current_stage: 'script',
    script_status: 'failed',
    script_error_code: 'AI_NATIVE_SCRIPT_PERSIST_FAILED',
    script_error_message: '脚本保存失败，请重新生成。',
  }

  const failed = aiNativeReducer(initialAINativeState, { type: 'requirement-loaded', workspace: failedWorkspace })
  assert.equal(failed.stage_status.script, 'failed')
  assert.match(failed.error, /脚本保存失败/)
})
