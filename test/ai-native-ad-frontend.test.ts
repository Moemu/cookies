import assert from 'node:assert/strict'
import test from 'node:test'
import * as React from 'react'
import { renderToStaticMarkup } from 'react-dom/server'
import { RequirementMediaGallery } from '../src/features/ai-native-ad/RequirementStage'
import { aiNativeReducer, initialAINativeState } from '../src/features/ai-native-ad/reducer'
import type { AdScriptDraft, AINativeRequirement, AINativeRequirementWorkspace, StoryboardDraft } from '../src/features/ai-native-ad/types'
import { aiNativeWorkspaceLocation, readAINativeWorkspaceLocation } from '../src/features/ai-native-ad/navigation'

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
