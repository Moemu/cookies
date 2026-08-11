import { useCallback, useEffect, useMemo, useState } from 'react'
import { Check, PencilLine, RefreshCw } from 'lucide-react'
import { useProject } from '../../../context/ProjectContext'
import { api, type ApiExperience, type ApiExperienceStatus, type ReviseExperienceBody } from '../../../data/api'
import { ExperienceReviseForm } from '../../ExperienceReviseForm'
import { shortId } from '../../../data/shortId'
import { ExperienceCard } from './ExperienceCard'

/**
 * 「管」：谁来给这些结论背书。
 *
 * 低频模式。人进来第一件事是看有没有事要做，所以列表按「要不要人做事」排：
 * 待定的没人认可过，标了复审的有人提出质疑，这两批排前面；其余是存档。
 * 按时间排的话，一条躺了两周的待定经验会被今天新沉淀的一堆挤到看不见。
 */
export function ManageView() {
  const { currentProject } = useProject()
  const [experiences, setExperiences] = useState<ApiExperience[]>([])
  const [listState, setListState] = useState<'loading' | 'ready' | 'error'>('loading')
  const [reasons, setReasons] = useState<Record<string, string>>({})
  const [revisingId, setRevisingId] = useState('')
  const [busyId, setBusyId] = useState('')
  const [notice, setNotice] = useState('')

  const load = useCallback(async () => {
    if (!currentProject.id) return
    setListState('loading')
    try {
      // 不带 status：三态一起看。「管」这一屏的问题是「有没有事要做」，
      // 分三栏的话人得点三次才知道答案。
      const page = await api.listExperiences(currentProject.id)
      setExperiences(page.items ?? [])
      setListState('ready')
    } catch (cause) {
      setExperiences([])
      setListState('error')
      setNotice(cause instanceof Error ? cause.message : '经验读取失败。')
    }
  }, [currentProject.id])

  useEffect(() => { void load() }, [load])

  const sorted = useMemo(() => [...experiences].sort((left, right) =>
    rank(left) - rank(right) || right.updated_at.localeCompare(left.updated_at)), [experiences])
  const pendingCount = experiences.filter(item => item.status === 'pending').length
  const flaggedCount = experiences.filter(item => item.needs_review).length

  const runAction = async (experience: ApiExperience, action: Action) => {
    const reason = (reasons[experience.id] ?? '').trim()
    // 确认不需要理由：它是「我看过了，成立」，动作本身就是理由。
    // 其余三个都会让一条经验对下游失效或降级，得留下一句话说明凭什么。
    if (action !== 'confirm' && !reason) {
      setNotice(`${shortId(experience.id)}：${actionLabels[action]}要先填理由，它会写进审计记录。`)
      return
    }
    setBusyId(experience.id)
    try {
      if (action === 'confirm') await api.confirmExperience(currentProject.id, experience.id, experience.version)
      if (action === 'reject') await api.rejectExperience(currentProject.id, experience.id, experience.version, reason)
      if (action === 'request-review') await api.requestExperienceReview(currentProject.id, experience.id, experience.version, reason)
      if (action === 'retire') await api.retireExperience(currentProject.id, experience.id, experience.version, reason)
      setReasons(current => ({ ...current, [experience.id]: '' }))
      await load()
      setNotice(`${shortId(experience.id)} 已${actionLabels[action]}。`)
    } catch (cause) {
      setNotice(cause instanceof Error ? cause.message : '操作失败，请稍后重试。')
    } finally {
      setBusyId('')
    }
  }

  const runRevise = async (experience: ApiExperience, body: ReviseExperienceBody) => {
    setBusyId(experience.id)
    try {
      const wasPending = experience.status === 'pending'
      const next = await api.reviseExperience(currentProject.id, experience.id, body)
      setRevisingId('')
      await load()
      // 前身的去向必须写清楚。退休和「还在用」是两回事，含糊过去，
      // 下次有人找不到旧那条会以为被删了。
      setNotice(`已保存为第 ${next.revision} 次修订，它回到「待定」等人确认。${wasPending
        ? '原来那一版没确认过，已经停用。'
        : '原来那一版继续在用，等这一版被确认后才交班。'}`)
    } catch (cause) {
      setNotice(cause instanceof Error ? cause.message : '修订失败，请稍后重试。')
    } finally {
      setBusyId('')
    }
  }

  return <div className="experience-manage">
    <div className="core-flow-toolbar">
      <div>
        <span className="section-label">EXPERIENCE · MANAGE</span>
        <h2>谁来给这些结论背书</h2>
        <p>确认过的才进下游的默认引用集。三态一起列，要人做事的排在最前面。</p>
      </div>
      <button className="secondary-button" disabled={listState === 'loading'} onClick={() => { void load() }}>
        <RefreshCw size={15}/>刷新
      </button>
    </div>

    <p className="manage-context">
      {pendingCount} 条待定、{flaggedCount} 条该看一眼了。
      {!pendingCount && !flaggedCount ? '现在没有要处理的。' : ''}
    </p>
    {notice ? <div className="inline-notice" role="status">{notice}</div> : null}

    {listState === 'loading' ? <p className="panel-empty">正在读取当前 Project 的经验…</p> : null}
    {listState === 'error' ? <p className="panel-empty">经验读取失败，请重试。</p> : null}
    {listState === 'ready' && !sorted.length
      ? <p className="panel-empty">这个 Project 还没有经验。经验来自复盘——投完一轮、提交复盘，值得留的会沉淀到这里等人确认。</p>
      : null}

    {sorted.map(experience => {
      const actions = actionsFor(experience)
      const busy = busyId === experience.id
      return <div key={experience.id} className="manage-item">
        <ExperienceCard
          experience={experience}
          status={<span className={`experience-status status-${experience.status}`}>{statusLabels[experience.status]}</span>}
          actions={revisingId === experience.id ? null : <>
            {/* 理由框只在有动作可做时出现。停用的经验没有动作，摆一个填不出所以然的
                输入框在那里，人会以为自己漏了一步。 */}
            {actions.some(action => action !== 'confirm') ? <textarea
              className="manage-reason" rows={2}
              value={reasons[experience.id] ?? ''}
              placeholder="理由：驳回 / 该看一眼了 / 停用必填，会写进审计记录。"
              onChange={event => setReasons(current => ({ ...current, [experience.id]: event.target.value }))}
            /> : null}
            {actions.map(action => <button key={action} disabled={busy}
              className={action === 'confirm' ? 'primary-button' : 'secondary-button'}
              onClick={() => { void runAction(experience, action) }}>
              {action === 'confirm' ? <Check size={15}/> : null}
              {busy ? '处理中…' : actionLabels[action]}
            </button>)}
            {canRevise(experience) ? <button className="secondary-button" disabled={busy}
              onClick={() => { setNotice(''); setRevisingId(experience.id) }}>
              <PencilLine size={15}/>修订，补齐依据
            </button> : null}
            {!actions.length && !canRevise(experience)
              ? <small className="manage-no-action">已停用的经验是逻辑删除，保留可读但不再参与流转。</small>
              : null}
          </>}
        />
        {revisingId === experience.id ? <ExperienceReviseForm
          // key 挂在经验 ID 上：换一条就重新挂载一份表单，
          // 否则 useState 的初值只在第一次挂载时取，下一条打开的还是上一条的内容。
          key={experience.id}
          experience={experience}
          busy={busy}
          onCancel={() => setRevisingId('')}
          onSubmit={body => { void runRevise(experience, body) }}
        /> : null}
      </div>
    })}
  </div>
}

type Action = 'confirm' | 'reject' | 'request-review' | 'retire'

const actionLabels: Record<Action, string> = {
  confirm: '确认',
  reject: '驳回',
  'request-review': '标为该看一眼',
  retire: '停用',
}

export const statusLabels: Record<ApiExperienceStatus, string> = {
  pending: '待定',
  confirmed: '在用',
  retired: '停用',
}

// 要人做事的排前面：待定 → 标了复审的 → 在用 → 停用。
function rank(experience: ApiExperience): number {
  if (experience.status === 'pending') return 0
  if (experience.needs_review) return 1
  if (experience.status === 'confirmed') return 2
  return 3
}

/**
 * 一条经验此刻能做什么。
 *
 * 在用且被标了「该看一眼了」的，多一个「确认」：那是「重新看过，还成立」，
 * 按下去把标记摘掉。没标记的在用经验不给「确认」——它已经在用了，再确认一次
 * 什么也不改变，摆在那里只会让人以为自己漏做了一步。
 */
function actionsFor(experience: ApiExperience): Action[] {
  if (experience.status === 'pending') return ['confirm', 'reject']
  if (experience.status === 'confirmed') {
    return experience.needs_review ? ['confirm', 'retire'] : ['request-review', 'retire']
  }
  return []
}

/**
 * 能不能修订。后端的门槛是：已停用的不能改，已经被新版本取代的也不能改
 * （一条血缘同时只允许有一个待确认的版本）。前端提前拦掉，免得人填完一整屏
 * 才在提交时被退回来。
 */
function canRevise(experience: ApiExperience): boolean {
  return experience.status !== 'retired' && !experience.superseded_by_id
}
