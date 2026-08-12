import { useState } from 'react'
import type { DataState } from '../../../types'
import { DataQualityPage } from '../../DataQualityPage'

/**
 * 数据体检。原来的「数据质量」入口整个搬到这里。
 *
 * 它进设置而不是自成一个入口，理由是：**它和判定阈值看的是同一个东西的两头**。
 * 上面那一屏定的是标准，这一屏看的是按这个标准量出来的数据够不够干净。分成两个
 * 一级入口，人会以为「数据有没有问题」和「结论按什么判」是两回事，而实际上
 * 一个阻断级问题会让所有结论都不作数。
 *
 * 原来的六个侧栏视图在这里降级成一排小标题。它们之间不是六件事，是同一批数据的
 * 六种问法：前五个按问题类型切（包括已处置的），第六个跨类型只留还要人管的。
 */
const segments = ['新鲜度', '缺失', '异常', '口径', '对账', '修复队列'] as const

export function HealthView({ state }: { state: DataState }) {
  // 分段状态留在这一层，不往上抛给导航：抛上去的话，侧栏的二级视图名会被分段名
  // 覆盖掉，人切一下「对账」，整个页面会退回「设置 · 判定阈值」。
  const [active, setActive] = useState<string>(segments[segments.length - 1])

  return <div className="assets-delegate">
    <p className="settings-intro">
      数据体检是「配置对不对」的报告。上面判定阈值定的是标准，这里看的是
      按这个标准量出来的数据够不够干净——两件事看的是同一个东西的两头。
    </p>
    <div className="assets-subtabs" role="tablist" aria-label="数据体检分段">
      {segments.map(item => <button key={item} type="button" role="tab" aria-selected={active === item}
        className={active === item ? 'assets-subtab active' : 'assets-subtab'}
        onClick={() => setActive(item)}>{item}</button>)}
    </div>
    <DataQualityPage state={state} activeView={active}/>
  </div>
}
