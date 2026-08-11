import type { DataState } from '../../../types'
import { StateBoundary } from '../../StateBoundary'
import { LookupView } from './LookupView'
import { ManageView } from './ManageView'

/**
 * 「经验」入口。原来的两个入口——投前洞察、经验库——合成这一个。
 *
 * 合并的理由是：它们是同一批数据的两种读法。投前洞察那五个视图里，前四个只是同一批
 * 经验按不同条件筛，第五个（引用记录）是每条经验自己的属性；经验库那一页则是同一批
 * 经验的另一种排法加上几个按钮。分成两个入口，同一条经验在两处长成两个样子，人会
 * 以为那是两条。
 *
 * 剩下的两个视图分的是**这个人现在要干什么**，不是数据的两种切片：
 *   查 —— 下一轮要做素材，看以前什么有效。高频，每轮开工都进。
 *   管 —— 给这些结论背书：确认、驳回、标复审、停用、修订。低频，值班的人才进。
 */
export type ExperienceView = 'lookup' | 'manage'

export function ExperiencePage({ state, view }: {
  state: DataState
  view: ExperienceView
}) {
  // 两个视图各自管自己的取数和空态，壳只负责挑一个。壳再包一层标题的话，
  // 「查」那一屏会顶着两条工具条，人分不清哪条说的是当前这一屏。
  return <StateBoundary state={state} onRetry={() => {}}>
    {view === 'manage' ? <ManageView/> : <LookupView/>}
  </StateBoundary>
}
