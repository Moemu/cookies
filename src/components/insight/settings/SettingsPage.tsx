import type { DataState } from '../../../types'
import { StateBoundary } from '../../StateBoundary'
import { DictionaryView } from './DictionaryView'
import { HealthView } from './HealthView'
import { PermissionView } from './PermissionView'
import { ThresholdView } from './ThresholdView'

/**
 * 「设置」入口。原来的三个入口——数据质量、能力运营、系统设置——合成这一个。
 *
 * 合并的理由是：这三页回答的是同一个问题「这套系统凭什么这么判」。判定阈值是标准，
 * 数据体检是按这个标准量出来的数据够不够干净，变量字典是这些数和词各自指什么，
 * 确认权限是谁能把机器说的变成我们认的。摆成三个平级入口，人得先猜「我这个疑问
 * 归哪一页管」才找得到。
 *
 * 四组里只有第一组能改。改判定阈值是这一版真正新增的能力：以前那七个数字散在三个
 * Go 文件里，看不见也调不了——而一个看不见的阈值和一个错的阈值，在使用者那里是
 * 同一种东西。
 */
export type SettingsView = 'thresholds' | 'health' | 'dictionary' | 'permission'

export function SettingsPage({ state, view }: {
  state: DataState
  view: SettingsView
}) {
  // 体检和字典委托给两个既有页面，它们自己包了 StateBoundary、自己管取数和空态。
  // 这里再包一层的话，同一屏会出现两条「暂无数据」。
  if (view === 'health') return <HealthView state={state}/>
  if (view === 'dictionary') return <DictionaryView state={state}/>

  return <StateBoundary state={state} onRetry={() => {}}>
    {view === 'permission' ? <PermissionView/> : <ThresholdView/>}
  </StateBoundary>
}
