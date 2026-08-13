import { useState } from 'react'
import type { DataState } from '../../../types'
import { DataConnectionsPage } from '../../DataConnectionsPage'

/**
 * 「数据接入」视图。
 *
 * 它进「素材」而不是留在治理里，理由是：一条素材要能进复盘必须齐三样——对上号、
 * 内容变量、投放数据，而数据接入管的正是第三样。把它放在治理里，等于让备料
 * 这件事分两个入口做。
 *
 * 原来的五个侧栏入口在这里降级成一排小标题。它们之间不是五件事，是同一件事的
 * 五段：接源 → 导数 → 对字段 → 对素材 → 看同步。
 */
const segments = ['数据源', '导入任务', '字段映射', '素材映射', '同步记录'] as const

export function IntakeView({ state }: { state: DataState }) {
  // 分段状态留在这一层，不往上抛给导航：抛上去的话，侧栏的二级视图名会被分段名
  // 覆盖掉，人切一下「字段映射」，整个页面会退回「素材 · 总览」。
  const [active, setActive] = useState<string>(segments[0])

  return <div className="assets-delegate">
    <div className="assets-subtabs" role="tablist" aria-label="数据接入分段">
      {segments.map(item => <button key={item} type="button" role="tab" aria-selected={active === item}
        className={active === item ? 'assets-subtab active' : 'assets-subtab'}
        onClick={() => setActive(item)}>{item}</button>)}
    </div>
    <DataConnectionsPage state={state} activeView={active}/>
  </div>
}
