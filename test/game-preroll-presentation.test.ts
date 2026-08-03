import assert from 'node:assert/strict'
import test from 'node:test'
import {
  buildGameShotViews,
  compareGamePrerollCandidates,
} from '../src/features/game-preroll/presentation.ts'
import type { ApiGamePrerollCandidate, ApiGameEvidenceMoment } from '../src/data/api.ts'

const evidence: ApiGameEvidenceMoment[] = [
  {
    id: 'evidence_1',
    kind: 'skill_choice',
    start_milliseconds: 20292,
    end_milliseconds: 22250,
    description: '技能三选一界面',
    verified_copy: ['三项技能可选'],
  },
  {
    id: 'evidence_2',
    kind: 'skill_choice',
    start_milliseconds: 29792,
    end_milliseconds: 31375,
    description: '第二次技能选择',
    verified_copy: ['选择后继续战斗'],
  },
  {
    id: 'evidence_3',
    kind: 'wave_progress',
    start_milliseconds: 34000,
    end_milliseconds: 35500,
    description: '第 2/10 波战斗',
    verified_copy: ['画面显示第 2/10 波'],
  },
]

test('selected six-second plan exposes three shot navigation items with source evidence', () => {
  const views = buildGameShotViews(candidate('candidate_a', 92), evidence)

  assert.equal(views.length, 3)
  assert.deepEqual(views.map(view => view.outputRangeLabel), ['0–2 秒', '2–4 秒', '4–6 秒'])
  assert.deepEqual(views.map(view => view.sourceRangeLabel), [
    '20.292–22.250s',
    '29.792–31.375s',
    '34.000–35.500s',
  ])
  assert.deepEqual(views.map(view => view.seekSeconds), [0, 2, 4])
  assert.equal(views[0].evidenceDescription, '技能三选一界面')
  assert.equal(views[0].thumbnailTimestampSeconds, 21.271)
})

test('candidate comparison recommends but never selects the strongest evidence-grounded plan', () => {
  const compared = compareGamePrerollCandidates([
    candidate('candidate_a', 86),
    candidate('candidate_b', 94),
    candidate('candidate_c', 90),
  ])

  assert.deepEqual(compared.map(item => item.candidate.id), [
    'candidate_b',
    'candidate_c',
    'candidate_a',
  ])
  assert.equal(compared[0].recommended, true)
  assert.equal(compared[0].recommendationLabel, '证据匹配度最高')
  assert.equal(compared[0].candidate.id, 'candidate_b')
})

function candidate(id: string, score: number): ApiGamePrerollCandidate {
  return {
    id,
    hook_mechanism: 'choice_challenge',
    execution_angle: '真实玩法挑战',
    primary_test_variable: '开场问题',
    variant_hypothesis: '明确选择题可提升首秒理解',
    score,
    score_meaning: 'evidence_grounded_hook_relevance',
    hook_line: '你会选哪一个？',
    evidence_moment_ids: evidence.map(item => item.id),
    storyboard: evidence.map((item, index) => ({
      start_milliseconds: index * 2000,
      end_milliseconds: (index + 1) * 2000,
      visual: item.description,
      copy: item.verified_copy[0],
      evidence_moment_id: item.id,
    })),
    prompt_package: {
      prompt_compiler_version: 'game-preroll/v1',
      input_snapshot_hash: 'sha256:input',
      candidate_batch_id: 'batch_1',
      candidate_id: id,
      generation_config: {
        subtitle_style: 'high_contrast_dynamic',
        hook_strength: 4,
        pace_profile: 'punchy',
      },
      director_spec: {},
      negative_constraints: [],
      compiled_prompt: 'prompt',
      content_hash: 'sha256:prompt',
    },
  }
}
