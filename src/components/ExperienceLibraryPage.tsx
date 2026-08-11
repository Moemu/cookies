import { useCallback, useEffect, useMemo, useState } from 'react'
import { ArrowRight, BookOpenCheck, Check, CircleAlert, CircleCheck, Database, Film, Layers3, Lightbulb, Link2, PencilLine, RefreshCw, Search, ShieldCheck, Target } from 'lucide-react'
import { useProject } from '../context/ProjectContext'
import { api, type ApiExperience, type ApiExperienceAudit, type ApiExperienceReference, type ApiExperienceStatus, type ReviseExperienceBody } from '../data/api'
import { ExperienceReviseForm } from './ExperienceReviseForm'
import {
  cardTypeLabels, cardTypeMeaning, confidenceLabels,
  describeApplicability, describeContentBasis, describeDataBasis,
} from '../data/insightCard'
import type { DataState } from '../types'
import { StateBoundary } from './StateBoundary'
import { shortId } from '../data/shortId'
import { consumerLabel, outcomeLabel } from '../data/experienceReference'
import { useInsightAssets } from '../data/useInsightAssets'

type ViewTarget = ApiExperienceStatus | 'references'

const viewTargets: Record<string, ViewTarget> = {
  候选经验: 'pending',
  已确认: 'confirmed',
  待复审: 'needs_review',
  已失效: 'retired',
  引用记录: 'references',
}

const statusLabels: Record<ApiExperienceStatus, string> = {
  pending: '待确认',
  confirmed: '已确认',
  needs_review: '待复审',
  retired: '已失效',
}

const emptyHints: Record<ViewTarget, string> = {
  pending: '当前 Project 暂无待确认的候选经验。经验在「复盘」里提交一轮复盘之后沉淀产生。',
  confirmed: '当前 Project 暂无已确认经验。只有已确认的结论才允许被下游引用。',
  needs_review: '当前 Project 暂无待复审经验。已确认结论出现反例时可申请复审。',
  retired: '当前 Project 暂无已失效经验。失效是逻辑删除，记录仍可追溯。',
  references: '当前 Project 暂无引用记录。经验被 Brief、创意任务或实验引用后会出现在这里。',
}

export function ExperienceLibraryPage({ state, activeView }: { state: DataState; activeView: string }) {
  const { currentProject } = useProject()
  const target = viewTargets[activeView] ?? 'pending'
  const [experiences, setExperiences] = useState<ApiExperience[]>([])
  const [projectReferences, setProjectReferences] = useState<ApiExperienceReference[]>([])
  const [audits, setAudits] = useState<ApiExperienceAudit[]>([])
  const [references, setReferences] = useState<ApiExperienceReference[]>([])
  const [selectedId, setSelectedId] = useState('')
  const [query, setQuery] = useState('')
  const [reason, setReason] = useState('')
  const [notice, setNotice] = useState('')
  const [busy, setBusy] = useState(false)
  const [revising, setRevising] = useState(false)
  const [listState, setListState] = useState<'loading' | 'ready' | 'error'>('loading')
  // 经验里存的样例素材是一串 ID。要显示成人能读的名字，得有这张对照表。
  const assetIndex = useInsightAssets(currentProject.id)

  const loadList = useCallback(async () => {
    if (!currentProject.id) return
    setListState('loading')
    try {
      if (target === 'references') {
        const next = await api.listProjectExperienceReferences(currentProject.id)
        setProjectReferences(next.items)
        setExperiences([])
      } else {
        const next = await api.listExperiences(currentProject.id, target)
        setExperiences(next.items)
        setProjectReferences([])
      }
      setListState('ready')
    } catch (cause) {
      setExperiences([])
      setProjectReferences([])
      setListState('error')
      setNotice(cause instanceof Error ? cause.message : '经验库读取失败。')
    }
  }, [currentProject.id, target])

  useEffect(() => { void loadList() }, [loadList])
  useEffect(() => { setReason(''); setNotice(''); setRevising(false) }, [target])
  // 换了一条经验就把修订表单收起来。表单里的内容是上一条的，留着它继续开着，
  // 人很容易以为自己在改右边这条，实际提交到的是刚才那条。
  useEffect(() => { setRevising(false) }, [selectedId])

  const filtered = useMemo(() => experiences.filter(experience =>
    `${experience.id} ${experience.conclusion} ${experience.conditions.join(' ')} ${experience.counterexamples.join(' ')}`
      .toLowerCase().includes(query.trim().toLowerCase()),
  ), [experiences, query])

  const filteredReferences = useMemo(() => projectReferences.filter(reference =>
    `${reference.id} ${reference.experience_id} ${reference.consumer_kind} ${reference.consumer_id} ${reference.note}`
      .toLowerCase().includes(query.trim().toLowerCase()),
  ), [projectReferences, query])

  useEffect(() => {
    setSelectedId(current => filtered.some(experience => experience.id === current) ? current : filtered[0]?.id ?? '')
  }, [filtered])

  const selected = filtered.find(experience => experience.id === selectedId)

  useEffect(() => {
    if (!selected) {
      setAudits([])
      setReferences([])
      return
    }
    let active = true
    void Promise.all([
      api.listExperienceAudits(currentProject.id, selected.id),
      api.listExperienceReferences(currentProject.id, selected.id),
    ]).then(([auditPage, referencePage]) => {
      if (!active) return
      setAudits(auditPage.items)
      setReferences(referencePage.items)
    }).catch(() => {
      if (!active) return
      setAudits([])
      setReferences([])
    })
    return () => { active = false }
  }, [currentProject.id, selected])

  const runAction = async (action: 'confirm' | 'reject' | 'request-review' | 'retire') => {
    if (!selected) return
    if (action !== 'confirm' && !reason.trim()) {
      setNotice('请先填写理由，状态变更会写入审计记录。')
      return
    }
    setBusy(true)
    try {
      if (action === 'confirm') await api.confirmExperience(currentProject.id, selected.id, selected.version)
      if (action === 'reject') await api.rejectExperience(currentProject.id, selected.id, selected.version, reason.trim())
      if (action === 'request-review') await api.requestExperienceReview(currentProject.id, selected.id, selected.version, reason.trim())
      if (action === 'retire') await api.retireExperience(currentProject.id, selected.id, selected.version, reason.trim())
      setReason('')
      await loadList()
      setNotice(`${shortId(selected.id)} 已${actionLabels[action]}，可在对应视图中查看。`)
    } catch (cause) {
      setNotice(cause instanceof Error ? cause.message : '状态变更失败，请稍后重试。')
    } finally {
      setBusy(false)
    }
  }

  const runRevise = async (body: ReviseExperienceBody) => {
    if (!selected) return
    setBusy(true)
    try {
      const wasPending = selected.status === 'pending'
      const next = await api.reviseExperience(currentProject.id, selected.id, body)
      setRevising(false)
      await loadList()
      // 修订出来的是一条新记录，旧的那条不一定还在当前这个列表里：它要么已经退休
      // （前身本来就没确认过），要么还挂在「已确认」那一栏等着交班。
      // 不把选中项挪过去，右边会空掉一屏，看着像是刚才那一下把东西弄丢了。
      setSelectedId(next.id)
      // 前身的去向必须写清楚。退休和「还在用」是两回事，含糊过去，
      // 下次有人找不到旧那条会以为被删了。
      const previous = wasPending
        ? '原来那一版没确认过，已经退休，在「已退休」里还能查到。'
        : '原来那一版继续可引用，等这一版被确认后才交班。'
      setNotice(next.status === target
        ? `已保存为第 ${next.revision} 次修订。${previous}`
        : `已保存为第 ${next.revision} 次修订，它回到「${statusLabels[next.status]}」等人确认。${previous}`)
    } catch (cause) {
      setNotice(cause instanceof Error ? cause.message : '修订失败，请稍后重试。')
    } finally {
      setBusy(false)
    }
  }

  if (target === 'references') {
    return <StateBoundary state={state} onRetry={() => { void loadList() }}>
      <div className="prelaunch-workspace">
        <section className="prelaunch-main">
          <div className="core-flow-toolbar">
            <div><span className="section-label">EXPERIENCE REFERENCES</span><h2>经验被谁用过</h2><p>当前 Project：{currentProject.name}。每条记录说明哪条经验被哪个环节引用，以及引用后的结果。</p></div>
            <button className="secondary-button" disabled={listState === 'loading'} onClick={() => { void loadList() }}><RefreshCw size={15}/>刷新</button>
          </div>
          <div className="prelaunch-filterbar">
            <div className="search-field"><Search size={15}/><input aria-label="搜索引用记录" value={query} onChange={event => setQuery(event.target.value)} placeholder="搜索经验、引用方或备注"/></div>
            <span>{filteredReferences.length} 条引用记录</span>
          </div>
          <div className="prelaunch-table" role="list" aria-label="引用记录列表">
            <div className="prelaunch-row header"><span>引用方</span><span>结果</span><span>备注</span><span>版本</span></div>
            {listState === 'loading' ? <div className="panel-empty">正在读取当前 Project 的引用记录…</div> : null}
            {listState === 'error' ? <div className="panel-empty">引用记录读取失败，请重试。</div> : null}
            {listState === 'ready' && !filteredReferences.length ? <div className="panel-empty">{emptyHints.references}</div> : null}
            {filteredReferences.map(reference => <div role="listitem" key={reference.id} className="prelaunch-row">
              <span><b>{consumerLabel(reference.consumer_kind)} · {shortId(reference.consumer_id)}</b><small>经验 {shortId(reference.experience_id)} · {formatTime(reference.created_at)}</small></span>
              <span>{outcomeLabel(reference)}</span>
              <span>{reference.note || '无备注'}</span>
              <span><Link2 size={14}/>v{reference.version}</span>
            </div>)}
          </div>
        </section>
        <aside className="prelaunch-detail">
          <span className="section-label">说明</span><h3>引用记录是经验价值的唯一证据</h3>
          <p>没有引用记录的经验，无法证明它影响过任何一次决策。</p>
          <div className="prelaunch-fact"><Lightbulb size={17}/><span><small>怎么产生</small><b>在投前洞察页把结论「引用到 Brief」或「引用到创意任务」，会在这里留下一条记录。实验中心给一条实验下结论时，判定也会自动记回它验证的那条经验。</b></span></div>
          <div className="prelaunch-fact"><Database size={17}/><span><small>为什么保留失效经验的记录</small><b>经验失效后引用历史仍然可读，便于回溯当时依据的是哪个版本的结论。</b></span></div>
          {notice ? <div className="inline-notice" role="status">{notice}</div> : null}
        </aside>
      </div>
    </StateBoundary>
  }

  return <StateBoundary state={state} onRetry={() => { void loadList() }}>
    <div className="prelaunch-workspace">
      <section className="prelaunch-main">
        <div className="core-flow-toolbar">
          <div><span className="section-label">EXPERIENCE LIBRARY</span><h2>把一次复盘的结论，变成组织能重复使用的经验</h2><p>当前 Project：{currentProject.name}。结论必须带适用条件和反例，确认后才允许被下游引用。</p></div>
          <button className="secondary-button" disabled={listState === 'loading'} onClick={() => { void loadList() }}><RefreshCw size={15}/>刷新</button>
        </div>
        <div className="prelaunch-filterbar">
          <div className="search-field"><Search size={15}/><input aria-label="搜索经验" value={query} onChange={event => setQuery(event.target.value)} placeholder="搜索结论、适用条件或反例"/></div>
          <span>{filtered.length} 条{statusLabels[target]}经验</span>
        </div>
        <div className="prelaunch-table" role="list" aria-label="经验列表">
          <div className="prelaunch-row header"><span>结论</span><span>状态</span><span>适用条件</span><span>修订</span></div>
          {listState === 'loading' ? <div className="panel-empty">正在读取当前 Project 的经验…</div> : null}
          {listState === 'error' ? <div className="panel-empty">经验读取失败，请重试。</div> : null}
          {listState === 'ready' && !filtered.length ? <div className="panel-empty">{emptyHints[target]}</div> : null}
          {filtered.map(experience => <button role="listitem" key={experience.id} className={selectedId === experience.id ? 'prelaunch-row active' : 'prelaunch-row'} onClick={() => setSelectedId(experience.id)}>
            <span><b>{experience.conclusion}</b><small>{cardTypeLabels[experience.card_type]} · 置信{confidenceLabels[experience.confidence]} · {shortId(experience.id)} · 来源报告 {shortId(experience.report_id)} · {formatTime(experience.updated_at)}</small></span>
            <span>{statusLabels[experience.status]}</span>
            <span>{experience.conditions.length ? experience.conditions.join('；') : '未填写适用条件'}</span>
            <span><CircleCheck size={14}/>v{experience.revision}</span>
          </button>)}
        </div>
      </section>
      <aside className="prelaunch-detail">
        {selected ? <>
          {/* 洞察卡九字段全展开（03 §8.1）。少显示一项，负责点「确认」的人就少一样判断依据：
              只看结论一句话，看不到样本量和置信，确认按钮就变成了盖章。 */}
          <span className="section-label">{cardTypeLabels[selected.card_type]} · 置信{confidenceLabels[selected.confidence]}</span>
          <h3>{selected.conclusion}</h3>
          <p>{shortId(selected.id)} · 第 {selected.revision} 次修订 · {statusLabels[selected.status]}</p>
          <div className="prelaunch-fact"><Layers3 size={17}/><span><small>这条能被怎么用</small><b>{cardTypeMeaning[selected.card_type]}</b></span></div>
          <div className="prelaunch-fact"><ShieldCheck size={17}/><span><small>置信提示</small><b>{confidenceLabels[selected.confidence]}</b></span></div>
          <div className="prelaunch-fact"><Target size={17}/><span><small>适用范围</small><b>{describeApplicability(selected.applicability)}</b></span></div>
          <div className="prelaunch-fact"><Database size={17}/><span><small>数据依据</small><b>{describeDataBasis(selected.data_basis)}</b></span></div>
          <div className="prelaunch-fact"><Lightbulb size={17}/><span><small>内容依据</small><b>{describeContentBasis(selected.content_basis)}</b></span></div>
          {/* 样例素材单独一行，而且换成名字。只写「示例素材 3 个」，
              看到这条经验的人还是不知道该去看哪三条片子。 */}
          {selected.content_basis.example_asset_versions?.length ? <div className="prelaunch-fact"><Film size={17}/><span><small>样例素材</small><b>{selected.content_basis.example_asset_versions.map(assetIndex.labelOf).join('、')}</b></span></div> : null}
          {selected.recommended_action ? <div className="prelaunch-fact"><ArrowRight size={17}/><span><small>建议动作</small><b>{selected.recommended_action}</b></span></div> : null}
          <div className="prelaunch-fact"><CircleCheck size={17}/><span><small>适用条件</small><b>{selected.conditions.length ? selected.conditions.join('；') : '未填写。没有适用条件的结论不应被直接套用。'}</b></span></div>
          <div className="prelaunch-boundary"><CircleAlert size={16}/><span><small>风险与反例</small>{selected.counterexamples.length ? selected.counterexamples.join('；') : '未记录反例。'}</span></div>
          <div className="prelaunch-fact"><Database size={17}/><span><small>来源</small><b>报告 {shortId(selected.report_id)} · 执行 {shortId(selected.source_execution_id)} · 指标快照 {shortId(selected.source_metric_snapshot_id)}</b></span></div>
          {selected.status_reason ? <div className="prelaunch-fact"><Lightbulb size={17}/><span><small>最近一次状态说明</small><b>{selected.status_reason}</b></span></div> : null}

          {revising ? <ExperienceReviseForm
            // key 挂在经验 ID 上：换一条就重新挂载一份表单，
            // 否则 useState 的初值只在第一次挂载时取，下一条打开的还是上一条的内容。
            key={selected.id}
            experience={selected}
            busy={busy}
            onCancel={() => setRevising(false)}
            onSubmit={body => { void runRevise(body) }}
          /> : actionsFor(selected.status).length ? <>
            <label className="experience-reason">
              <small>理由（驳回、申请复审、失效必填，会写入审计记录）</small>
              <textarea value={reason} onChange={event => setReason(event.target.value)} rows={3} placeholder="例如：新一批投放出现反例，结论需要重新验证。"/>
            </label>
            <div className="prelaunch-actions">
              {actionsFor(selected.status).map(action => <button key={action} className={action === 'confirm' ? 'primary-button full' : 'secondary-button full'} disabled={busy} onClick={() => void runAction(action)}>
                {action === 'confirm' ? <Check size={15}/> : <ArrowRight size={15}/>}
                {busy ? '处理中…' : actionLabels[action]}
              </button>)}
              {/* 「去经验库补齐依据并确认」——报告页那句话指的就是这个按钮。
                  没有它，上面那三行「没写数据依据」只能一直没写下去。 */}
              {canRevise(selected) ? <button className="secondary-button full" disabled={busy} onClick={() => { setNotice(''); setRevising(true) }}>
                <PencilLine size={15}/>修订，补齐依据
              </button> : null}
            </div>
          </> : <div className="prelaunch-boundary"><CircleAlert size={16}/><span><small>无可用动作</small>已失效经验是逻辑删除，保留可读但不再参与流转。</span></div>}

          <div className="reference-count"><BookOpenCheck size={15}/><span><b>{references.length} 条引用记录</b><small>{references.length ? references.map(reference => consumerLabel(reference.consumer_kind)).join('、') : '尚未被任何环节引用'}</small></span></div>

          <div className="feature-stack">
            <span>状态变更记录</span>
            {audits.length ? audits.map(audit => <b key={audit.id}>
              {formatTime(audit.created_at)} · {audit.from_status ? `${statusLabels[audit.from_status as ApiExperienceStatus] ?? audit.from_status} → ` : '沉淀为 '}{statusLabels[audit.to_status]}
              {audit.reason ? ` · ${audit.reason}` : ''}
            </b>) : <b>暂无变更记录。</b>}
          </div>

          {notice ? <div className="inline-notice" role="status">{notice}</div> : null}
        </> : <div className="panel-empty">选择左侧经验后查看适用条件、反例、来源链路和状态变更记录。</div>}
      </aside>
    </div>
  </StateBoundary>
}

const actionLabels: Record<'confirm' | 'reject' | 'request-review' | 'retire', string> = {
  confirm: '确认',
  reject: '驳回',
  'request-review': '申请复审',
  retire: '失效',
}

/**
 * 能不能修订。后端的门槛是：已失效的不能改，已经被新版本取代的也不能改
 * （一条血缘同时只允许有一个待确认的版本）。前端提前拦掉，免得人填完一整屏
 * 才在提交时被退回来。
 */
function canRevise(experience: ApiExperience): boolean {
  return experience.status !== 'retired' && !experience.superseded_by_id
}

function actionsFor(status: ApiExperienceStatus): Array<'confirm' | 'reject' | 'request-review' | 'retire'> {
  if (status === 'pending') return ['confirm', 'reject']
  if (status === 'confirmed') return ['request-review', 'retire']
  if (status === 'needs_review') return ['confirm', 'retire']
  return []
}

function formatTime(value: string): string {
  const time = new Date(value)
  return Number.isNaN(time.valueOf()) ? value : time.toLocaleString('zh-CN')
}
