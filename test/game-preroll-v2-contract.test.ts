import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import test from 'node:test'

const schema = JSON.parse(readFileSync(resolve('api/contracts/creative-game-preroll-workspace-v2.schema.json'), 'utf8'))

test('game preroll V2 freezes task-scoped workflow and human selection stages', () => {
  assert.equal(schema.properties.contract_version.const, 'creative-game-preroll-workspace/v2')
  for (const stage of ['source_ready', 'brief_confirmed', 'candidate_selected', 'video_ready']) {
    assert.ok(schema.properties.stage.enum.includes(stage))
  }
  assert.equal(schema.$defs.candidateBatch.properties.generated_candidate_count.const, 3)
})

test('game preroll V2 freezes Douyin 9:16 and the supported 6-10 second window', () => {
  const config = schema.$defs.generationConfig.properties
  assert.equal(config.channel.const, 'douyin')
  assert.equal(config.aspect_ratio.const, '9:16')
  assert.deepEqual({ minimum: config.duration_seconds.minimum, maximum: config.duration_seconds.maximum }, { minimum: 6, maximum: 10 })
  assert.equal(schema.$defs.generationSpec.properties.input_mode.const, 'first_last_frame')
})
