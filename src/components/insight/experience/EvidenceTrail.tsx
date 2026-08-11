import { useEffect, useState } from 'react'
import { useProject } from '../../../context/ProjectContext'
import { api, type ApiExperience, type ApiExperienceReference } from '../../../data/api'
import { formatCount, formatDate } from '../analysis/format'
import { projectPath } from '../../../lib/router'
import { shortId } from '../../../data/shortId'

/**
 * 「凭什么」展开层：从一句结论一路走回原始数据。
 *
 * 顺序是固定的：数据依据 → 内容依据 → 来源复盘 → 什么情况下不成立 → 引用记录。
 * 这条链上最要紧的一环是「来源复盘」——人看完结论问的第一个问题是「这是哪次
 * 投放得出来的」，答不上来的话，后面几段说得再细都只是自说自话。
 *
 * 引用记录并到这里，是因为它本来就是每条经验自己的属性。原来把它做成投前洞察
 * 的第五个视图，人得先在那一页里找到这条经验，才能看它被谁用过——找的过程比看
 * 的过程还长。
 */
export function EvidenceTrail({ experience }: { experience: ApiExperience }) {
  const { currentProject } = useProject()
  const [references, setReferences] = useState<ApiExperienceReference[]>([])
  const [referenceState, setReferenceState] = useState<'loading' | 'ready' | 'error'>('loading')

  useEffect(() => {
    let alive = true
    setReferenceState('loading')
    api.listExperienceReferences(currentProject.id, experience.id)
      .then(page => {
        if (!alive) return
        setReferences(page.items ?? [])
        setReferenceState('ready')
      })
      .catch(() => { if (alive) setReferenceState('error') })
    return () => { alive = false }
  }, [currentProject.id, experience.id])

  const basis = experience.data_basis
  // 窗口按本地日历天显示。库里存的是带时区的时间戳，原样摆出来是
  // 「2026-07-12T00:00:00Z」——人要在脑子里做一次时区换算才知道是哪天。
  const window = [basis.window_start, basis.window_end].filter(Boolean).map(value => formatDate(value!)).join(' ~ ')

  return <div className="evidence-trail">
    <section>
      <h5>数据依据</h5>
      <ul>
        {window ? <li>观察窗口：{window}</li> : null}
        {basis.sample_size ? <li>样本量：{formatCount(basis.sample_size)}</li> : null}
        {basis.asset_count ? <li>涉及素材：{basis.asset_count} 个</li> : null}
        {basis.metrics?.length ? <li>看的指标：{basis.metrics.join('、')}</li> : null}
        {basis.baseline ? <li>对照基准：{basis.baseline}</li> : null}
        {experience.note ? <li>档位说明：{experience.note}</li> : null}
      </ul>
      {/* 空着不能不说话。一条经验连窗口和样本量都没有，那它的档位是怎么定的，
          看的人有权知道系统答不上来。 */}
      {!window && !basis.sample_size && !basis.asset_count
        ? <p className="panel-empty">这条经验没记数据依据，它的档位没有数据支撑。</p>
        : null}
    </section>

    <section>
      <h5>内容依据</h5>
      {experience.content_basis.features?.length
        ? <p>涉及变量：{experience.content_basis.features.join('、')}</p>
        : <p className="panel-empty">没记涉及哪些变量，无法判断它能迁移到哪些素材上。</p>}
      {experience.content_basis.example_asset_versions?.length
        ? <p>例子：{experience.content_basis.example_asset_versions.map(shortId).join('、')}</p>
        : null}
      {experience.content_basis.note ? <p>{experience.content_basis.note}</p> : null}
    </section>

    <section>
      <h5>来源</h5>
      {experience.report_id
        ? <p>
          来自
          {' '}
          <a href={projectPath(currentProject.id, 'insight', 'review', experience.report_id, '全部复盘')}>
            {shortId(experience.report_id)}
          </a>
          {' '}
          这份复盘。
        </p>
        : <p className="panel-empty">这条经验没挂到任何复盘上，查不到它是哪一轮投放得出来的。</p>}
      {experience.recommended_action ? <p>建议动作：{experience.recommended_action}</p> : null}
    </section>

    {/* 反例单独一段并且显眼：填了这一格的经验才是真被人推敲过的。
        混在依据里的话，最能说明一条结论边界在哪的东西会被当成补充说明扫过去。 */}
    <section className="evidence-counterexamples">
      <h5>什么情况下不成立</h5>
      {experience.counterexamples.length
        ? <ul>{experience.counterexamples.map(item => <li key={item}>{item}</li>)}</ul>
        : <p className="panel-empty">还没人给它找到反例。这不代表没有——只代表还没人试过推翻它。</p>}
    </section>

    <section>
      <h5>引用记录</h5>
      {referenceState === 'loading' ? <p className="panel-empty">读取中…</p> : null}
      {referenceState === 'error' ? <p className="panel-empty">引用记录读不出来。</p> : null}
      {referenceState === 'ready' && references.length === 0
        ? <p className="panel-empty">还没有人引用过这条经验。</p>
        : null}
      {references.length ? <ul className="evidence-references">
        {references.map(reference => <li key={reference.id}>
          {reference.consumer_kind} · {shortId(reference.consumer_id)}
          <span className={`reference-outcome outcome-${reference.outcome}`}>
            {referenceOutcomeLabel[reference.outcome] ?? reference.outcome}
          </span>
          {reference.note ? <small>{reference.note}</small> : null}
        </li>)}
      </ul> : null}
    </section>
  </div>
}

// 四挡是有序的：只是引用 → 照做 → 改了之后用 → 没采纳。
// 「没采纳」也要显示出来，它比前三挡更有信息量——一条老被拒的经验该退休了。
const referenceOutcomeLabel: Record<string, string> = {
  referenced: '引用了',
  adopted: '照做了',
  modified: '改了之后用的',
  rejected: '没采纳',
}
