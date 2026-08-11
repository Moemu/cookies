import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import test from 'node:test'

const root = resolve(import.meta.dirname, '..')
const shell = readFileSync(resolve(root, 'src/components/Shell.tsx'), 'utf8')
const styles = readFileSync(resolve(root, 'src/styles.css'), 'utf8')

test('skip link lands on a focusable main region with a visible keyboard outline', () => {
  assert.match(shell, /href="#main-content"/)
  assert.match(shell, /setTimeout\(\(\) => document\.getElementById\('main-content'\)\?\.focus\(\), 0\)/)
  assert.match(shell, /id="main-content"[^>]*tabIndex=\{-1\}/)
  assert.match(styles, /\.main-content:focus-visible/)
})
