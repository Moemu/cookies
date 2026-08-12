import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import test from 'node:test'

test('cross-stage Assistant messages identify the proposal surface without changing the frozen body contract', () => {
  const api = readFileSync(join(process.cwd(), 'src/features/strategy/api.ts'), 'utf8')
  const workspace = readFileSync(join(process.cwd(), 'src/features/strategy/KanonStrategyWorkspace.tsx'), 'utf8')
  assert.match(api, /'X-Strategy-Surface': contextSurface/)
  assert.match(api, /contextSurface === 'assistant'[\s\S]{0,180}strategy-conversation-message-create\/v2/)
  assert.match(workspace, /activeStage, 'assistant'/)
})

test('Assistant source removal is scoped to the next accepted turn', () => {
  const api = readFileSync(join(process.cwd(), 'src/features/strategy/api.ts'), 'utf8')
  const workspace = readFileSync(join(process.cwd(), 'src/features/strategy/KanonStrategyWorkspace.tsx'), 'utf8')
  const dock = readFileSync(join(process.cwd(), 'src/features/strategy/assistant/ProjectAssistantDock.tsx'), 'utf8')
  assert.match(api, /'X-Strategy-Excluded-Source-Ids': JSON\.stringify\(excludedSourceIds\)/)
  assert.match(workspace, /if \(accepted\) setAssistantExcludedSourceIds\(\[\]\)/)
  assert.match(dock, /来源只针对下一轮调用；取消后不会删除资料或改变 Brief。/)
  assert.match(dock, /onToggleSource\(ref\.id\)/)
})

test('Assistant proposals expose explicit adopt, edit, and ignore controls', () => {
  const dock = readFileSync(join(process.cwd(), 'src/features/strategy/assistant/ProjectAssistantDock.tsx'), 'utf8')
  assert.match(dock, /不会自动修改 Brief/)
  assert.match(dock, /采用编辑稿/)
  assert.match(dock, />忽略</)
  assert.match(dock, /当前：/)
})

test('Assistant offers three contextual candidate requests instead of a template wall', () => {
  const dock = readFileSync(join(process.cwd(), 'src/features/strategy/assistant/ProjectAssistantDock.tsx'), 'utf8')
  assert.match(dock, /brief\?\.completeness\.blockers\[0\]\?\.field/)
  assert.match(dock, /给出 2—3 个差异明确的候选/)
  assert.match(dock, /不要提供通用模板/)
  assert.match(dock, /starterRequests\.map/)
})

test('proposal mutation APIs bind idempotency and expected proposal versions', () => {
  const api = readFileSync(join(process.cwd(), 'src/features/strategy/api.ts'), 'utf8')
  assert.match(api, /assistant-proposals\/\$\{encodeURIComponent\(proposal\.id\)\}:apply/)
  assert.match(api, /expected_version: proposal\.version/)
  assert.match(api, /assistant-proposals\/\$\{encodeURIComponent\(proposal\.id\)\}:ignore/)
})
