import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import test from 'node:test'

const componentPath = new URL('../src/components/SpecializedPages.tsx', import.meta.url)
const stylesPath = new URL('../src/styles.css', import.meta.url)

test('video editor keeps the full inspector reachable above the fixed status bar', async () => {
  const styles = await readFile(stylesPath, 'utf8')

  assert.match(styles, /\.video-editing-workspace \.editing-shell \{[^}]*min-height:\s*560px/s)
  assert.match(styles, /\.video-editing-workspace \.editing-inspector \{[^}]*overflow-y:\s*auto/s)
  assert.doesNotMatch(styles, /\.video-editing-workspace \.editing-inspector \{[^}]*overflow:\s*hidden/s)
})

test('video editor explains prerequisites and does not advertise completed M2 tracks as unopened', async () => {
  const component = await readFile(componentPath, 'utf8')

  assert.match(component, /操作顺序：选择素材 → 创建 EditTask → 创建低清预览或正式导出/)
  assert.match(component, /渲染引擎已支持，MVP 暂无可视化编辑/)
  assert.match(component, /已完成，成片已回流素材库/)
  assert.match(component, /选择此素材加入时间线/)
  assert.match(component, /已选择，点击移出时间线/)
  assert.doesNotMatch(component, /轨道将在 M2 开放/)
})
