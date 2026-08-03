import { useCallback, useEffect, useState } from 'react'
import { CircleAlert, FileCode2, Lock } from 'lucide-react'
import { useProject } from '../context/ProjectContext'
import { api, type ApiInsightSettings, type ApiSettingGroup } from '../data/api'
import type { DataState } from '../types'
import { StateBoundary } from './StateBoundary'

/**
 * 系统设置（03 §78 与 19 §289 的五个视图；20 §121）。
 *
 * 20 §121 给 MVP 的定位是「可采用保守默认值」，并要求「单列分组表单，重要阈值显示
 * 影响说明和默认推荐；**不使用仪表盘布局**」。22 §239 记的问题是「缺少实际阈值
 * 影响说明」——所以这一页要解决的不是「让人改阈值」，而是**让人知道现在生效的阈值
 * 是多少、调它会动到什么**。
 *
 * 整页只读，也没有禁用态的输入框：画一个改不动的框，比直接说「这里改不了」更让人
 * 恼火，而且会让人以为是权限不够而不是功能没做。
 *
 * 数字全部由后端从代码常量现算，前端一个都不写死。前端抄一份，改了 Go 忘了改这里，
 * 这一页就从说明变成误导——那比不做更糟。
 */
const groupByView: Record<string, string> = {
  样本门槛: 'sample',
  观察窗口: 'window',
  通知: 'notification',
  确认权限: 'confirmation',
  报告模板: 'report_template',
}

export function InsightSettingsPage({ state, activeView }: { state: DataState; activeView: string }) {
  const { currentProject } = useProject()
  const [settings, setSettings] = useState<ApiInsightSettings | null>(null)
  const [notice, setNotice] = useState('')
  const [listState, setListState] = useState<'loading' | 'ready' | 'error'>('loading')

  const load = useCallback(async () => {
    if (!currentProject.id) return
    setListState('loading')
    try {
      setSettings(await api.getInsightSettings(currentProject.id))
      setListState('ready')
    } catch (cause) {
      setSettings(null)
      setListState('error')
      setNotice(cause instanceof Error ? cause.message : '设置读取失败。')
    }
  }, [currentProject.id])

  useEffect(() => { void load() }, [load])

  const wantedKey = groupByView[activeView] ?? 'sample'
  const group = settings?.groups.find(item => item.key === wantedKey) ?? settings?.groups[0]

  return <StateBoundary state={state} onRetry={() => { void load() }}>
    <div className="settings-page">
      {listState === 'loading' ? <p className="settings-status">读取中…</p> : null}
      {listState === 'error' ? <p className="settings-status error">{notice}</p> : null}

      {settings && group ? <>
        <div className="settings-lock">
          <Lock size={16}/>
          <span>
            <b>这一页只读</b>
            {settings.editable_note}
          </span>
        </div>

        <section className="settings-group">
          <span className="section-label">{group.label}</span>
          <p className="settings-summary">{group.summary}</p>
          {group.state === 'not_built' ? <NotBuilt group={group}/> : <div className="setting-list">
            {group.items.map(item => <div className="setting-row" key={item.key}>
              <div className="setting-head">
                <b>{item.label}</b>
                <span className="setting-value">{item.value}</span>
              </div>
              <p className="setting-effect">{item.effect}</p>
              <p className="setting-recommended">
                默认推荐：{item.recommended}
                {/* 一致是常态，不一致才要提醒——说明有人调过它。是否算「不一致」由后端给：
                    确认权限那组两边说的是不同的事，前端比字符串会全部误报。 */}
                {item.deviates ? <em>（当前值与推荐不一致）</em> : null}
              </p>
              <p className="setting-meta">
                <FileCode2 size={12}/>
                <code>{item.source}</code>
                <span>依据：{item.basis}</span>
              </p>
            </div>)}
          </div>}
        </section>
      </> : null}
    </div>
  </StateBoundary>
}

/**
 * 「还没做」和「做了但当前为空」必须长得不一样。通知这一组尤其容易被误读成
 * 「在后台默默生效」——实际上代码里没有一处会主动往外发消息。
 */
function NotBuilt({ group }: { group: ApiSettingGroup }) {
  return <div className="settings-missing">
    <div className="settings-missing-head"><CircleAlert size={16}/><b>这一组还没有建设</b></div>
    {group.missing.map(line => <p key={line}>{line}</p>)}
  </div>
}
