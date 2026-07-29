import { describe, expect, it } from 'vitest'
import { buildBulkRemixPlan, estimateAssetDurationSeconds, isVideoAsset } from './aiRemixPlanner'
import type { AssetFeature, ProjectAsset } from './types'

function asset(id: string, overrides: Partial<ProjectAsset['version']> & { kind?: string; createdAt?: string } = {}): ProjectAsset {
  const createdAt = overrides.createdAt ?? '2026-07-20T00:00:00Z'
  return {
    ref: { project_id: 'project_1', asset_version: { asset_id: id, version: 1 } },
    asset: {
      id,
      organization_id: 'org_1',
      asset_kind: overrides.kind ?? 'video',
      status: 'ready',
      owner_system: 'assets',
      latest_version: 1,
      created_at: createdAt,
      updated_at: createdAt,
    },
    version: {
      organization_id: 'org_1',
      asset_id: id,
      version: 1,
      status: 'ready',
      source_type: overrides.source_type ?? 'upload',
      mime_type: overrides.mime_type ?? 'video/mp4',
      size_bytes: overrides.size_bytes ?? 8 * 1024 * 1024,
      sha256: `${id}sha`,
      width_pixels: overrides.width_pixels,
      height_pixels: overrides.height_pixels,
      media: {
        duration_seconds: overrides.media?.duration_seconds,
        fps: overrides.media?.fps,
        codec: overrides.media?.codec,
        bitrate_bps: overrides.media?.bitrate_bps,
        audio_codec: overrides.media?.audio_codec,
        audio_channels: overrides.media?.audio_channels,
        audio_sample_rate: overrides.media?.audio_sample_rate,
        poster_frame_ref: overrides.media?.poster_frame_ref,
        probe_status: overrides.media?.probe_status ?? 'not_required',
        probe_error: overrides.media?.probe_error,
      },
      created_at: createdAt,
    },
    created_at: createdAt,
  }
}

describe('aiRemixPlanner', () => {
  it('只把视频素材纳入混剪候选', () => {
    expect(isVideoAsset(asset('video_1'))).toBe(true)
    expect(isVideoAsset(asset('image_1', { kind: 'image', mime_type: 'image/png' }))).toBe(false)
  })

  it('为前中后三段生成可解释时间线', () => {
    const plan = buildBulkRemixPlan({
      now: new Date('2026-07-25T00:00:00Z'),
      targetSeconds: 30,
      pace: 'balanced',
      selection: {
        opening: [
          asset('hook_vertical', { source_type: 'captured', width_pixels: 720, height_pixels: 1280 }),
          asset('hook_image', { kind: 'image', mime_type: 'image/png', width_pixels: 1024, height_pixels: 1024 }),
        ],
        middle: [
          asset('proof_horizontal', { source_type: 'provider_generated', width_pixels: 1920, height_pixels: 1080, size_bytes: 18 * 1024 * 1024 }),
          asset('demo_square', { source_type: 'upload', width_pixels: 1080, height_pixels: 1080 }),
        ],
        ending: [
          asset('cta_square', { source_type: 'upload', width_pixels: 1080, height_pixels: 1080 }),
        ],
      },
    })

    expect(plan.schemaVersion).toBe('remix_plan_v2')
    expect(plan.segments).toHaveLength(3)
    expect(plan.segments[0].shots[0]).toMatchObject({
      id: 'opening_hook_vertical_1',
      segment: 'opening',
      source: 'existing_asset',
      assetVersion: { asset_id: 'hook_vertical', version: 1 },
      timeline: { startSeconds: 0 },
      creative: { transition: 'cut' },
      planning: { reasonCodes: expect.arrayContaining(['captured', 'vertical']) },
    })
    expect(plan.segments[0].clips.map((clip) => clip.assetId)).toEqual(['hook_vertical'])
    expect(plan.segments[0].clips[0].assetId).toBe(plan.segments[0].shots[0].assetId)
    expect(plan.segments[1].clips.map((clip) => clip.assetId)).toContain('proof_horizontal')
    expect(plan.segments[2].clips[0].reason).toContain('前段'.replace('前', '后'))
    expect(plan.summary.selectedAssets).toBe(4)
    expect(plan.summary.usedAssets).toBeGreaterThan(0)
    expect(plan.warnings[0]).toContain('估算素材时长')
  })

  it('不同节奏会影响单素材估算时长', () => {
    const sample = asset('clip_1', { size_bytes: 80 * 1024 * 1024 })
    expect(estimateAssetDurationSeconds(sample, 'fast')).toBeLessThan(estimateAssetDurationSeconds(sample, 'story'))
  })

  it('真实视频时长优先于文件大小估算', () => {
    const sample = asset('clip_1', {
      size_bytes: 80 * 1024 * 1024,
      media: { duration_seconds: 12.345, probe_status: 'succeeded', codec: 'h264' },
    })
    expect(estimateAssetDurationSeconds(sample, 'fast')).toBe(12.3)

    const plan = buildBulkRemixPlan({
      now: new Date('2026-07-25T00:00:00Z'),
      targetSeconds: 30,
      pace: 'fast',
      selection: {
        opening: [sample],
        middle: [],
        ending: [],
      },
    })
    expect(plan.segments[0].clips[0].durationSeconds).toBe(8)
    expect(plan.warnings.some((warning) => warning.includes('估算素材时长'))).toBe(false)
  })

  it('使用 AssetFeature 信号提升强 hook 素材并降级高相似风险素材', () => {
    const strongHook = asset('strong_hook', { source_type: 'upload', width_pixels: 720, height_pixels: 1280 })
    const riskyCaptured = asset('risky_captured', { source_type: 'captured', width_pixels: 720, height_pixels: 1280 })
    const plan = buildBulkRemixPlan({
      now: new Date('2026-07-25T00:00:00Z'),
      targetSeconds: 12,
      pace: 'fast',
      assetFeatures: [
        feature('strong_hook', { hook_strength: 0.95, product_visibility: 0.72, similarity_risk: 'low', selling_points: ['3 秒利益点'] }),
        feature('risky_captured', { hook_strength: 0.3, product_visibility: 0.2, similarity_risk: 'high', selling_points: ['重复结构'] }),
      ],
      selection: {
        opening: [riskyCaptured, strongHook],
        middle: [],
        ending: [],
      },
    })

    expect(plan.segments[0].clips[0].assetId).toBe('strong_hook')
    expect(plan.segments[0].clips[0].reason).toContain('Hook 95')
    expect(plan.segments[0].clips[0].reason).toContain('卖点：3 秒利益点')
    expect(plan.segments[0].clips.map((clip) => clip.assetId)).not.toContain('risky_captured')
  })

  it('将相似度风险写入 Shot risks 供 UI 展示', () => {
    const plan = buildBulkRemixPlan({
      now: new Date('2026-07-25T00:00:00Z'),
      targetSeconds: 12,
      pace: 'balanced',
      assetFeatures: [
        feature('medium_risk', { hook_strength: 0.8, product_visibility: 0.7, similarity_risk: 'medium', selling_points: ['重复钩子'] }),
      ],
      selection: {
        opening: [asset('medium_risk', { source_type: 'captured', width_pixels: 720, height_pixels: 1280 })],
        middle: [],
        ending: [],
      },
    })

    expect(plan.segments[0].shots[0].risks).toEqual(['similarity_risk:medium'])
  })
})

function feature(id: string, overrides: Partial<AssetFeature>): AssetFeature {
  return {
    organization_id: 'org_1',
    project_id: 'project_1',
    asset_id: id,
    asset_version: 1,
    schema_version: 'asset_feature_v1',
    feature_version: 'vlm-2026-07-26',
    hook_strength: overrides.hook_strength ?? 0.5,
    product_visibility: overrides.product_visibility ?? 0.5,
    scene_tags: overrides.scene_tags ?? [],
    product_tags: overrides.product_tags ?? [],
    person_tags: overrides.person_tags ?? [],
    action_tags: overrides.action_tags ?? [],
    emotion_tags: overrides.emotion_tags ?? [],
    selling_points: overrides.selling_points ?? [],
    cta_presence: overrides.cta_presence ?? false,
    similarity_group: overrides.similarity_group,
    similarity_risk: overrides.similarity_risk ?? 'low',
    evidence: overrides.evidence ?? [],
    created_at: '2026-07-26T10:00:00Z',
    updated_at: '2026-07-26T10:00:00Z',
  }
}
