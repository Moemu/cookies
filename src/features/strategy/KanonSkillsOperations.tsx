import { Bot, CheckCircle2, CircleAlert, Fingerprint, Target } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { strategyApi } from './api'
import type { SkillDescriptor } from './types'

export function KanonSkillsOperations({ activeView }: { activeView: string }) {
  const [skills, setSkills] = useState<SkillDescriptor[]>([])
  const [selected, setSelected] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    const controller = new AbortController()
    void strategyApi.listSkills(controller.signal).then(result => {
      setSkills(result.items)
      setSelected(result.items[0]?.name ?? '')
    }).catch(cause => {
      if (!(cause instanceof DOMException && cause.name === 'AbortError')) {
        setError(cause instanceof Error ? cause.message : '读取 Skills 失败。')
      }
    }).finally(() => setLoading(false))
    return () => controller.abort()
  }, [])

  const visible = useMemo(() => {
    if (activeView === '领域 Skills') return skills
    return []
  }, [activeView, skills])
  const current = skills.find(skill => skill.name === selected) ?? skills[0]

  if (activeView !== '领域 Skills') {
    return <div className="kanon-strategy-state">
      <CircleAlert size={24}/>
      <h2>{activeView}尚未接入治理后端</h2>
      <p>当前只开放真实的只读 Skill Registry，不使用 Project activity 冒充模板、评测集或质量结果。</p>
    </div>
  }
  if (loading) return <div className="kanon-strategy-state"><Bot size={24}/><h2>正在读取 Skill Registry</h2></div>
  if (error) return <div className="kanon-strategy-state"><CircleAlert size={24}/><h2>Skill Registry 不可用</h2><p>{error}</p></div>

  return <div className="kanon-skills-operations">
    <section>
      <div className="kanon-strategy-heading">
        <div><span className="section-label">STRATEGY SKILL REGISTRY</span><h2>平台与目标 Skills</h2><p>读取后端 embed Registry；运行时固定版本与内容哈希。</p></div>
        <span className="source-chip">{skills.length} 个已注册 Skill</span>
      </div>
      <div className="kanon-skill-list">
        {visible.map(skill => <button className={skill.name === current?.name ? 'active' : ''} key={`${skill.name}:${skill.version}`} onClick={() => setSelected(skill.name)}>
          <span>{skill.kind === 'platform' ? <Bot size={17}/> : <Target size={17}/>}</span>
          <div><b>{skillName(skill.name)}</b><small>{skill.name} · {skill.version}</small></div>
          <em>{skill.kind === 'platform' ? '平台' : '目标'}</em>
        </button>)}
      </div>
    </section>
    <aside>
      {current ? <>
        <div className="surface-toolbar"><h3>{skillName(current.name)}</h3><span>{current.version}</span></div>
        <dl>
          <div><dt>类型</dt><dd>{current.kind === 'platform' ? '平台 Skill' : '目标 Skill'}</dd></div>
          <div><dt>匹配条件</dt><dd>{current.match.join('、')}</dd></div>
        </dl>
        <div className="kanon-skill-hash"><Fingerprint size={15}/><code>{current.content_hash}</code></div>
        <h4>质量检查</h4>
        {current.quality_checks.map(check => <p className="kanon-skill-check" key={check}><CheckCircle2 size={15}/><span>{check}</span></p>)}
        <div className="kanon-strategy-note"><CircleAlert size={16}/><div><b>当前为只读 Registry</b><p>发布、评测、灰度和回滚将在数据库 Registry 与权限模型完成后开放。</p></div></div>
      </> : null}
    </aside>
  </div>
}

function skillName(value: string) {
  const labels: Record<string, string> = {
    'channel.douyin': '抖音策略',
    'channel.xiaohongshu': '小红书策略',
    'channel.wechat_ecosystem': '微信生态策略',
    'channel.taobao_tmall': '淘宝 / 天猫策略',
    'objective.awareness': '品牌认知',
    'objective.conversion': '转化增长',
    'objective.lead_generation': '线索获取',
  }
  return labels[value] ?? value
}
