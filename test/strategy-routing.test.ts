import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import test from 'node:test'

test('Strategy workspaces use the host application router', async () => {
  const [workspaceSource, routeSource, pagesSource, plannerSource] = await Promise.all([
    readFile(new URL('../src/features/strategy/KanonStrategyWorkspace.tsx', import.meta.url), 'utf8'),
    readFile(new URL('../src/features/strategy/workspace/StrategyWorkspaceRoute.tsx', import.meta.url), 'utf8'),
    readFile(new URL('../src/components/Pages.tsx', import.meta.url), 'utf8'),
    readFile(new URL('../src/features/strategy/CreativeTaskPlanner.tsx', import.meta.url), 'utf8'),
  ])

  assert.doesNotMatch(workspaceSource, /from ['"]react-router-dom['"]/)
  assert.match(routeSource, /WorkspaceProvider/)
  assert.match(workspaceSource, /workspaceRoute\.navigate/)
  assert.doesNotMatch(pagesSource, /<ViewTabs item=\{item\} activeView=\{activeView\} onViewChange=\{changeView\}\/>{pageSurface}/)
  assert.doesNotMatch(plannerSource, /from ['"]react-router-dom['"]/)
  assert.match(plannerSource, /onOpenCreative/)
})

test('Strategy stage changes restore only the stage container', async () => {
  const [workspaceSource, routerSource] = await Promise.all([
    readFile(new URL('../src/features/strategy/KanonStrategyWorkspace.tsx', import.meta.url), 'utf8'),
    readFile(new URL('../src/lib/router.ts', import.meta.url), 'utf8'),
  ])
  assert.match(workspaceSource, /const main = mainRef\.current/)
  assert.match(workspaceSource, /stageScrollSessionKey/)
  assert.match(workspaceSource, /main\.scrollTo\(\{ top: lastScrollTop \}\)/)
  assert.doesNotMatch(workspaceSource, /window\.scrollTo/)
  assert.match(routerSource, /preserveWindowScroll/)
  assert.match(routerSource, /if \(!preserveWindowScroll\) window\.scrollTo/)
})
