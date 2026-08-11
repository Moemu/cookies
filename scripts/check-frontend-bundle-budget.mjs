import { readdir, stat } from 'node:fs/promises'
import { join } from 'node:path'

const assetsDirectory = join(process.cwd(), 'dist', 'assets')
const budgets = [
  { label: 'entry', prefix: 'index-', maximumBytes: 450_000 },
  { label: 'module shell', prefix: 'Pages-', maximumBytes: 140_000 },
  { label: 'Strategy workspace', prefix: 'StrategyWorkspaceRoute-', maximumBytes: 230_000 },
]
const maximumChunkBytes = 500_000

const files = (await readdir(assetsDirectory)).filter(name => name.endsWith('.js'))
const sizes = new Map(await Promise.all(files.map(async name => [name, (await stat(join(assetsDirectory, name))).size])))
const failures = []

for (const budget of budgets) {
  const matches = [...sizes].filter(([name]) => name.startsWith(budget.prefix))
  if (matches.length !== 1) {
    failures.push(`${budget.label}: expected exactly one ${budget.prefix}*.js asset, found ${matches.length}`)
    continue
  }
  const [name, size] = matches[0]
  console.log(`${budget.label}: ${name} ${formatBytes(size)} / ${formatBytes(budget.maximumBytes)}`)
  if (size > budget.maximumBytes) failures.push(`${budget.label}: ${formatBytes(size)} exceeds ${formatBytes(budget.maximumBytes)}`)
}

for (const [name, size] of sizes) {
  if (size > maximumChunkBytes) failures.push(`${name}: ${formatBytes(size)} exceeds the ${formatBytes(maximumChunkBytes)} per-chunk ceiling`)
}

if (failures.length) {
  console.error(`Frontend bundle budget failed:\n- ${failures.join('\n- ')}`)
  process.exitCode = 1
} else {
  console.log(`Frontend bundle budget passed across ${sizes.size} JavaScript assets.`)
}

function formatBytes(value) {
  return `${(value / 1000).toFixed(2)} kB`
}
