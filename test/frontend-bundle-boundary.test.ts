import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import test from 'node:test'

test('the application and non-Strategy product areas stay outside the eager entry graph', () => {
  const app = readFileSync(join(process.cwd(), 'src/App.tsx'), 'utf8')
  const pages = readFileSync(join(process.cwd(), 'src/components/Pages.tsx'), 'utf8')

  assert.doesNotMatch(app, /import \{ HomePage, ModulePage \} from ['"]\.\/components\/Pages['"]/)
  assert.match(app, /const ModulePage = lazy\(\(\) => loadPages\(\)/)
  assert.match(app, /void import\(['"]\.\/features\/strategy\/workspace\/StrategyWorkspaceRoute['"]\)/)
  assert.doesNotMatch(pages, /from ['"]\.\/SpecializedPages['"]/)
  assert.doesNotMatch(pages, /from ['"]\.\/DataConnectionsPage['"]/)
  assert.doesNotMatch(pages, /from ['"]\.\/DeliveryThreeTierPage['"]/)
})

test('production build owns explicit entry, module-shell, Strategy, and per-chunk budgets', () => {
  const packageJson = JSON.parse(readFileSync(join(process.cwd(), 'package.json'), 'utf8')) as { scripts: Record<string, string> }
  const budget = readFileSync(join(process.cwd(), 'scripts/check-frontend-bundle-budget.mjs'), 'utf8')

  assert.match(packageJson.scripts.build, /check-frontend-bundle-budget\.mjs/)
  assert.match(budget, /prefix: 'index-'/)
  assert.match(budget, /prefix: 'Pages-'/)
  assert.match(budget, /prefix: 'StrategyWorkspaceRoute-'/)
  assert.match(budget, /maximumChunkBytes = 500_000/)
})
