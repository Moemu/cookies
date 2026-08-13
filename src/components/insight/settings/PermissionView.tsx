import { useCallback, useEffect, useState } from 'react'
import { CircleAlert, FileCode2, Lock } from 'lucide-react'
import { useProject } from '../../../context/ProjectContext'
import { api, type ApiSettingGroup } from '../../../data/api'

/**
 * 确认权限。
 *
 * 这一屏**只读**，而且没有禁用态的输入框：画一个改不动的框，比直接说「这里改不了」
 * 更让人恼火，而且会让人以为是权限不够而不是功能没做。
 *
 * 三个权限已经在每个接口上生效，但目前只能按账号授予、不能按角色配——这一点写在
 * 后端给的 summary 里，不在前端另写一句。它和判定阈值摆在同一个入口下，是因为
 * 两者管的是同一件事的两半：阈值定「机器什么时候敢下结论」，权限定「谁能把机器
 * 说的变成我们认的」。
 */
export function PermissionView() {
  const { currentProject } = useProject()
  const [group, setGroup] = useState<ApiSettingGroup | null>(null)
  const [notice, setNotice] = useState('')
  const [listState, setListState] = useState<'loading' | 'ready' | 'error'>('loading')

  const load = useCallback(async () => {
    if (!currentProject.id) return
    setListState('loading')
    try {
      const settings = await api.getInsightSettings(currentProject.id)
      setGroup(settings.groups.find(item => item.key === 'confirmation') ?? null)
      setListState('ready')
    } catch (cause) {
      setGroup(null)
      setListState('error')
      setNotice(cause instanceof Error ? cause.message : '确认权限读取失败。')
    }
  }, [currentProject.id])

  useEffect(() => { void load() }, [load])

  return <div className="settings-page">
    {listState === 'loading' ? <p className="settings-status">读取中…</p> : null}
    {listState === 'error' ? <p className="settings-status error">{notice}</p> : null}

    {group ? <>
      <div className="settings-lock">
        <Lock size={16}/>
        <span>
          <b>这一段只读</b>
          权限现在只能按账号授予，界面上改不了。判定阈值那一段可以改，这里不行——
          谁能确认结论是账号体系的事，不是洞察模块自己能决定的。
        </span>
      </div>

      <section className="settings-group">
        <span className="section-label">{group.label}</span>
        <p className="settings-summary">{group.summary}</p>
        {group.state === 'not_built' ? <div className="settings-missing">
          <div className="settings-missing-head"><CircleAlert size={16}/><b>这一组还没有建设</b></div>
          {group.missing.map(line => <p key={line}>{line}</p>)}
        </div> : <div className="setting-list">
          {group.items.map(item => <div className="setting-row" key={item.key}>
            <div className="setting-head">
              <b>{item.label}</b>
              <span className="setting-value">{item.value}</span>
            </div>
            <p className="setting-effect">{item.effect}</p>
            {/* 这一组的「当前值」是管到哪些操作、「推荐」是该发给谁，本来就是两件事，
                所以不比对、也不提示偏离——后端的 deviates 在这一组恒为 false。 */}
            <p className="setting-recommended">建议授予：{item.recommended}</p>
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
}
