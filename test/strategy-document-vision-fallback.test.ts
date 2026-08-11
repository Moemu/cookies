import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import test from 'node:test'

const root = resolve(import.meta.dirname, '..')
const materialsSource = readFileSync(resolve(root, 'src/features/strategy/documents/MaterialsDrawer.tsx'), 'utf8')
const workspaceSource = readFileSync(resolve(root, 'src/features/strategy/useStrategyWorkspace.ts'), 'utf8')
const documentStyles = readFileSync(resolve(root, 'src/features/strategy/styles/documents.css'), 'utf8')

test('document quality explains the routing decision without claiming accuracy', () => {
  assert.match(materialsSource, /解析质量路由信号/)
  assert.match(materialsSource, /不代表内容事实准确率/)
  assert.match(materialsSource, /summary\.characters_per_page/)
  assert.match(materialsSource, /summary\.locator_coverage/)
  assert.match(materialsSource, /summary\.metadata_image_signal_ratio/)
  assert.match(materialsSource, /summary\.metadata_table_signal_ratio/)
  assert.match(materialsSource, /summary\.empty_pages/)
  assert.match(materialsSource, /图片\/表格为元数据信号密度，不等同真实页面占比/)
  assert.match(documentStyles, /\.strategy-materials__quality-metrics/)
  assert.match(documentStyles, /font-variant-numeric: tabular-nums/)
})

test('document visual fallback stays manual, bounded, and non-blocking', () => {
  assert.match(materialsSource, /只有你确认后才会执行/)
  assert.match(materialsSource, /一次最多选择 24 页/)
  assert.match(materialsSource, /可以离开本面板，不会阻塞其他工作/)
  assert.match(materialsSource, /系统不会静默换用其他模型/)
  assert.match(workspaceSource, /runDocumentVisionFallback/)
  assert.doesNotMatch(workspaceSource, /setInterval\([^)]*runDocumentVisionFallback/)
})

test('presentation fallback explains conversion progress and preserves the source', () => {
  assert.match(materialsSource, /正在将演示文稿转换为可追溯 PDF/)
  assert.match(materialsSource, /原始演示文稿和已有文本都会保留/)
  assert.match(materialsSource, /conversion_required/)
  assert.match(materialsSource, /DOCUMENT_VISION_STORAGE_SCOPE_INVALID/)
})

test('visual fallback failure explicitly preserves the prior text result', () => {
  assert.match(materialsSource, /视觉解析未完成，文本结果未丢失/)
  assert.match(materialsSource, /当前已有片段仍可使用/)
})

test('uncertain paid submission requires reconciliation instead of offering retry', () => {
  assert.match(materialsSource, /visionFailureNeedsReconciliation/)
  assert.match(materialsSource, /为避免重复计费/)
  assert.match(materialsSource, /请先由运维人员到 LAS 对账/)
  assert.match(materialsSource, /capability\?\.available && !needsReconciliation/)
})
