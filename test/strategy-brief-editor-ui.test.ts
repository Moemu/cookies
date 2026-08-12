import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import test from 'node:test'

const workspace = readFileSync(join(process.cwd(), 'src/features/strategy/KanonStrategyWorkspace.tsx'), 'utf8')

test('Brief uses decision groups and keeps high-risk confirmation explicit', () => {
  for (const title of ['业务目标', '产品与证据', '受众与情境', '渠道与转化', '资源与约束', '创意边界']) {
    assert.match(workspace, new RegExp(`title: '${title}'`))
  }
  assert.match(workspace, /briefHighRiskFields\.has\(field\.path\)/)
  assert.match(workspace, /确认本组普通信息/)
  assert.doesNotMatch(workspace, /确认全部已填写字段/)
})

test('Brief autosaves after the UX debounce while preserving explicit confirmation', () => {
  assert.match(workspace, /window\.setTimeout\(\(\) => \{[\s\S]*?\}, 750\)/)
  assert.match(workspace, /等待自动保存/)
  assert.match(workspace, /正在自动保存/)
  assert.match(workspace, /自动保存失败 · 输入已保留/)
  assert.match(workspace, /已自动保存 · 需单独确认/)
  assert.match(workspace, /onAutosaveField=\{actions\.autosaveBriefField\}/)
})

test('Strategy v3 editor previews revision impact and keeps editable list keys focus-stable', () => {
  assert.match(workspace, /只更新“\{strategySectionLabel\(section\)\}”章节/)
  assert.match(workspace, /历史 Revision 与已发布 StrategyPackage 保持不可变/)
  assert.match(workspace, /key=\{`territory-\$\{territoryIndex\}`\}/)
  assert.match(workspace, /key=\{`adaptation-\$\{adaptationIndex\}`\}/)
  assert.doesNotMatch(workspace, /key=\{`\$\{territory\.name\}/)
  assert.doesNotMatch(workspace, /key=\{`\$\{adaptation\.platform\}/)
})
