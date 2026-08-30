import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import test from 'node:test'

const component = readFileSync(resolve(import.meta.dirname, '../src/components/DeliveryConfigurationPage.tsx'), 'utf8')
const styles = readFileSync(resolve(import.meta.dirname, '../src/styles.css'), 'utf8')

test('delivery line-list fields preserve the editing draft until blur', () => {
  assert.match(component, /function LineListTextarea/)
  assert.match(component, /setDraft\(nextDraft\)/)
  assert.match(component, /onValuesChange\(lineListValues\(nextDraft, limit, unique\)\)/)
  assert.match(component, /setDraft\(normalized\.join\('\\n'\)\)/)
  assert.match(component, /placeholder="每行一个行动号召"/)
  assert.match(component, /placeholder="每行一个卖点"/)
  assert.match(component, /placeholder="每行一条文案"/)
})

test('preferred-media checkboxes keep a fixed native control size', () => {
  assert.match(styles, /\.delivery-config-check-grid input\[type="checkbox"\]/)
  assert.match(styles, /flex:\s*0 0 16px/)
  assert.match(styles, /min-height:\s*16px/)
})

test('manual direct links do not require an OceanEngine object binding', () => {
  assert.match(component, /direct_link_mode === 'manual'/)
  assert.match(component, /placeholder="请输入 tbopen:\/\//)
  assert.match(component, /手动链接不需要绑定巨量平台 ID/)
  assert.match(component, /missingRequiredFields\.has\('direct_link'\)/)
})

test('unsupported Runner paths and CPM bid limits are visible before execution', () => {
  assert.match(component, /当前项目路径不能生成 Runner 计划/)
  assert.match(component, /销售线索（Runner 暂不支持）/)
  assert.match(component, /value="short_video_image_text"/)
  assert.match(component, /CPM 项目出价必须是 4 至 100 元/)
})
