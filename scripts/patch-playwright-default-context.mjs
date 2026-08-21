import { readFile, writeFile } from 'node:fs/promises'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const repositoryRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const packageRoot = resolve(repositoryRoot, 'node_modules', 'playwright-core')
const packageMetadata = JSON.parse(await readFile(resolve(packageRoot, 'package.json'), 'utf8'))

if (packageMetadata.version !== '1.62.1') {
  throw new Error(`Unsupported playwright-core version: ${packageMetadata.version}`)
}

const bundlePath = resolve(packageRoot, 'lib', 'coreBundle.js')
let source = await readFile(bundlePath, 'utf8')
const replacements = [
  [
    '        assert(targetInfo.browserContextId, "targetInfo: " + JSON.stringify(targetInfo, null, 2));\n',
    '',
  ],
  [
    '        await browser._waitForAllPagesToBeInitialized();\n        return browser;\n',
    '        return browser;\n',
  ],
]

let changed = false
for (const [before, after] of replacements) {
  if (source.includes(before)) {
    source = source.replace(before, after)
    changed = true
  } else if (!source.includes(after)) {
    throw new Error('playwright-core default Context patch target is missing')
  }
}

if (changed) await writeFile(bundlePath, source, 'utf8')
