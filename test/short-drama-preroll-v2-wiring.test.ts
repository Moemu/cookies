import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import test from 'node:test'

const workspacePath = new URL('../src/features/short-drama-preroll-v2/ShortDramaPrerollWorkspace.tsx', import.meta.url)
const apiPath = new URL('../src/data/api.ts', import.meta.url)

test('short drama preroll V2 uses the real workspace API and one direction action', async () => {
  const [workspace, api] = await Promise.all([
    readFile(workspacePath, 'utf8'),
    readFile(apiPath, 'utf8'),
  ])

  assert.doesNotMatch(workspace, /fixtureAnalysis|fixtureHooks|fixtureImages/)
  assert.match(workspace, /api\.uploadProjectAsset/)
  assert.match(workspace, /api\.analyzeShortDramaV2Source/)
  assert.match(workspace, /api\.generateShortDramaV2Directions/)
  assert.match(workspace, /direction_batch\?\.selected_direction_id/)
  assert.match(workspace, /batch\.prompt_revision/)
  assert.match(workspace, /version: 3/)
  assert.match(workspace, /restoreState\(currentProject\.id, restored, source, session\)/)
  assert.match(workspace, /api\.bindShortDramaV2TrustedMaterials/)
  assert.equal(workspace.match(/onClick=\{\(\) => void generateHooks\(\)\}/g)?.length, 1)

  assert.match(api, /short-drama-preroll-v2:\$\{action\}/)
  assert.match(api, /route_manual_short_drama_preroll_v2/)
  assert.match(api, /bind-trusted-materials/)
})
