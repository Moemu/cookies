import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import test from 'node:test'

test('Strategy workspaces use the host application router', async () => {
  const [workspaceSource, plannerSource] = await Promise.all([
    readFile(new URL('../src/features/strategy/KanonStrategyWorkspace.tsx', import.meta.url), 'utf8'),
    readFile(new URL('../src/features/strategy/CreativeTaskPlanner.tsx', import.meta.url), 'utf8'),
  ])

  assert.doesNotMatch(workspaceSource, /from ['"]react-router-dom['"]/)
  assert.match(workspaceSource, /onOpenWorkspace/)
  assert.doesNotMatch(plannerSource, /from ['"]react-router-dom['"]/)
  assert.match(plannerSource, /onOpenCreative/)
})
