import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import test from 'node:test'

test('commerce preroll V2 read, create, version, and command routes stay registered', async () => {
  const source = await readFile(new URL('../internal/platform/httpserver/server.go', import.meta.url), 'utf8')

  for (const route of [
    'GET /api/creative/v1/projects/{project_id}/commerce-preroll-v2:latest',
    'POST /api/creative/v1/projects/{project_id}/commerce-preroll-v2',
    'GET /api/creative/v1/projects/{project_id}/creative-tasks/{task_id}/commerce-preroll-v2',
    'GET /api/creative/v1/projects/{project_id}/creative-tasks/{task_id}/commerce-preroll-v2/versions',
  ]) {
    assert.match(source, new RegExp(route.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')))
  }

  for (const action of ['analyze-source', 'prepare-references', 'select-first-frame', 'generate-video', 'adopt-output']) {
    assert.match(source, new RegExp(`"${action}"`))
  }
})
