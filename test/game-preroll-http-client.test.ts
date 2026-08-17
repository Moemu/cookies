import assert from 'node:assert/strict'
import test from 'node:test'

import {
  mapGamePrerollWorkspace,
  shouldPauseEvidenceAt,
  type ApiGamePrerollTaskDetail,
} from '../src/features/game-preroll/httpClient.ts'

const detail: ApiGamePrerollTaskDetail = {
  task: { id: 'task_game_real_1', display_name: '真实游戏前贴', status: 'in_progress' },
  video_draft: {
    revision: 7,
    game_preroll: {
      contract_version: 'creative-game-preroll-workspace/v2',
      task_id: 'task_game_real_1',
      revision: 7,
      stage: 'candidates_ready',
      input_snapshot: {
        source_video: { asset_id: 'source_real', version: 2 },
        evidence_moments: [
          {
            id: 'moment_operation',
            kind: 'operation',
            start_milliseconds: 12340,
            end_milliseconds: 14560,
            description: '玩家切换武器并击中敌人',
            verified_copy: ['切换武器', '命中敌人'],
          },
        ],
      },
      source_metadata: { DurationMS: 61510 },
      analysis: {
        status: 'ready',
        facts: [{ id: 'fact_gameplay', label: '核心玩法', value: '射击闯关', provenance: 'video_evidence', evidence_refs: ['moment_operation'] }],
        evidence: [
          {
            id: 'moment_operation',
            kind: 'operation',
            start_milliseconds: 12340,
            end_milliseconds: 14560,
            description: '玩家切换武器并击中敌人',
            verified_copy: ['切换武器', '命中敌人'],
          },
        ],
        suggested_brief: [{ id: 'cta', key: 'cta', label: '行动号召 CTA', value: '立即下载', provenance: 'ai_inference', evidence_refs: [], required: true }],
      },
      confirmed_brief: { fields: [{ id: 'cta', key: 'cta', label: '行动号召 CTA', value: '立即下载', provenance: 'manual', evidence_refs: [], required: true }] },
      evidence_assets: {
        status: 'ready',
        frames: [{
          evidence_moment_id: 'moment_operation',
          source_start_milliseconds: 12340,
          source_end_milliseconds: 14560,
          representative_frame_milliseconds: 13450,
          frame_asset: { project_id: 'project_demo', asset_version: { asset_id: 'frame_real', version: 1 } },
        }],
      },
      generation_config: { subtitle_style: 'high_contrast_dynamic', hook_strength: 4, pace_profile: 'punchy', duration_seconds: 8, channel: 'douyin', aspect_ratio: '9:16', resolution: '720p', audio_policy: 'source_audio', call_to_action: '立即下载' },
      candidates: [{
        id: 'candidate_real', hook_mechanism: 'choice_challenge', execution_angle: '悬念提问', primary_test_variable: '首秒问题', variant_hypothesis: '用选择题提升停留', score: 93, score_meaning: '强推荐', hook_line: '这把武器你会怎么选？', evidence_moment_ids: ['moment_operation'],
        storyboard: [{ start_milliseconds: 0, end_milliseconds: 8000, visual: '真实玩法特写', copy: '立即下载', evidence_moment_id: 'moment_operation' }],
      }],
      selected_candidate_id: 'candidate_real',
      output_asset: { project_id: 'project_demo', asset_version: { asset_id: 'output_real', version: 3 } },
    },
  },
}

test('maps real workspace evidence frames and output assets without fixture values', () => {
  const state = mapGamePrerollWorkspace('project_demo', detail, {
    sourceUrl: '/preview/source-real',
    frameUrls: { moment_operation: '/preview/frame-real' },
    outputUrl: '/preview/output-real',
  })

  assert.equal(state.taskId, 'task_game_real_1')
  assert.equal(state.revision, 7)
  assert.equal(state.source?.assetId, 'source_real')
  assert.equal(state.source?.durationSeconds, 61.51)
  assert.deepEqual(state.evidence[0], {
    id: 'moment_operation',
    kind: 'operation',
    label: '切换武器',
    description: '玩家切换武器并击中敌人',
    startMs: 12340,
    endMs: 14560,
    provenance: 'video_evidence',
    thumbnailUrl: '/preview/frame-real',
    verifiedCopy: ['切换武器', '命中敌人'],
  })
  assert.equal(state.candidates[0].hookLine, '这把武器你会怎么选？')
  assert.equal(state.generation.outputUrl, '/preview/output-real')
  assert.equal(state.generation.status, 'succeeded')
})

test('evidence playback stops only at the real server-provided segment end', () => {
  assert.equal(shouldPauseEvidenceAt(14.559, 14560), false)
  assert.equal(shouldPauseEvidenceAt(14.56, 14560), true)
  assert.equal(shouldPauseEvidenceAt(16, undefined), false)
})
