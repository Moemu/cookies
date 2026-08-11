import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import test from 'node:test'
import { systems } from '../src/data/navigation.js'

test('Strategy cross-workspace centers form one prominent shared-context hub', () => {
  const strategy = systems.find(system => system.key === 'strategy')
  assert.ok(strategy)

  const hubs = strategy.nav.filter(item => item.prominence === 'hub')
  assert.deepEqual(hubs.map(item => item.id), ['briefs', 'research', 'strategies', 'reviews'])
  assert.ok(hubs.every(item => item.group === '策略中枢'))
  assert.ok(hubs.every(item => item.navHint && item.navHint.length >= 6))
})

test('Strategy workspace uses compact workbench chrome without a redundant page header', () => {
  const pages = readFileSync(join(process.cwd(), 'src/components/Pages.tsx'), 'utf8')
  const shell = readFileSync(join(process.cwd(), 'src/features/strategy/workspace/StrategyWorkspaceShell.tsx'), 'utf8')
  const styles = readFileSync(join(process.cwd(), 'src/styles.css'), 'utf8')

  assert.match(pages, /isStrategyWorkspace \? null : <PageHeader/)
  assert.match(shell, /aria-label="策略工作区"/)
  assert.doesNotMatch(styles, /\.strategy-workspace-page > \.page-header/)
  assert.match(styles, /\.strategy-workspace-shell \{[\s\S]*?margin-top: 0;/)
})
