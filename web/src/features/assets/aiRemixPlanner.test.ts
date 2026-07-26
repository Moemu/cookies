import { describe, expect, it } from 'vitest'
import { buildBulkRemixPlan, estimateAssetDurationSeconds, isVideoAsset } from './aiRemixPlanner'
import type { ProjectAsset } from './types'

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

    expect(plan.segments).toHaveLength(3)
    expect(plan.segments[0].clips.map((clip) => clip.assetId)).toEqual(['hook_vertical'])
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
})
