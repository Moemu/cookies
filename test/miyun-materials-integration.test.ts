import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import test from 'node:test'

test('米云素材保持为素材洞察下的独立路由页面，冻结四个一级视图', async () => {
  const [navigation, pages] = await Promise.all([
    readFile(new URL('../src/data/navigation.ts', import.meta.url), 'utf8'),
    readFile(new URL('../src/components/Pages.tsx', import.meta.url), 'utf8'),
  ])

  assert.match(navigation, /id: 'miyun-materials', label: '米云素材', icon: Aperture, group: '素材与分析', layout: 'workspace'/)
  assert.match(navigation, /views: \['产品分析', '采集任务', '素材候选', '裂变任务'\]/)
  assert.match(pages, /system\.key === 'insight' && item\.id === 'miyun-materials' \? <MiyunMaterialsPage state=\{dataState\} activeView=\{activeView\}\/>/)
})
