import { useCallback, useEffect, useMemo, useState } from 'react'
import { RefreshCw } from 'lucide-react'
import { useProject } from '../../../context/ProjectContext'
import { api, type ApiExperience, type ApiExperienceLookup, type ApiExperienceMatch } from '../../../data/api'
import { ExperienceCard, applicabilityGroups } from './ExperienceCard'

/**
 * 「查」：下一轮要做素材，先看以前什么有效。
 *
 * 这一屏替代了原来的「投前洞察」页。那一页的五个视图——策略证据、创意建议、
 * 历史模式、风险与反例、引用记录——前四个只是同一批经验按不同条件筛，第五个是
 * 每条经验自己的引用历史（现在在卡片的展开层里）。它们不是五个功能，是一个功能
 * 的五种切法。
 *
 * 进来先自动按当前项目的条件筛一遍，不是给一张空表单等人填：查经验的人手上就是
 * 当前这个项目，让他把品牌产品再敲一遍是白费功夫。
 */
export function LookupView() {
  const { currentProject } = useProject()
  // 预填只填项目上真有的两格。渠道 / 广告类型 / 受众项目档案里没有，
  // 拿「目标」那种自由文本去卡 objective 会把大部分经验误筛掉——
  // 匹配是整值相等，一句「提升 30 天复购」对不上任何一条经验写的目标。
  const [lookup, setLookup] = useState<ApiExperienceLookup>({})
  const [matches, setMatches] = useState<ApiExperienceMatch[]>([])
  const [confirmed, setConfirmed] = useState<ApiExperience[]>([])
  const [listState, setListState] = useState<'loading' | 'ready' | 'error'>('loading')
  const [notice, setNotice] = useState('')

  useEffect(() => {
    setLookup({ brand: currentProject.brand || '', product: currentProject.product || '' })
  }, [currentProject.id, currentProject.brand, currentProject.product])

  // 下拉的选项从这个 Project 已有的经验里长出来，不写死一张渠道表。
  // 写死的话，人能选到「视频号」，但这里一条关于视频号的经验都没有，
  // 他会以为是筛坏了，而不是「本来就没有」。
  const options = useMemo(() => {
    const buckets: Record<string, Set<string>> = {}
    confirmed.forEach(experience => applicabilityGroups(experience.applicability).forEach(group => {
      buckets[group.key] ??= new Set()
      group.values.forEach(value => buckets[group.key].add(value))
    }))
    return buckets
  }, [confirmed])

  const runLookup = useCallback(async (next: ApiExperienceLookup) => {
    if (!currentProject.id) return
    setListState('loading')
    try {
      const [result, all] = await Promise.all([
        api.lookupExperiences(currentProject.id, clean(next)),
        api.listExperiences(currentProject.id, 'confirmed'),
      ])
      setMatches(result.items ?? [])
      setConfirmed(all.items ?? [])
      setListState('ready')
      setNotice('')
    } catch (cause) {
      setMatches([])
      setListState('error')
      setNotice(cause instanceof Error ? cause.message : '经验读取失败。')
    }
  }, [currentProject.id])

  useEffect(() => { void runLookup(lookup) }, [runLookup, lookup])

  const update = (patch: Partial<ApiExperienceLookup>) => setLookup(current => ({ ...current, ...patch }))
  const scope = [lookup.brand, lookup.product, lookup.channel, lookup.ad_type, lookup.objective, lookup.audience]
    .filter(Boolean).join(' · ')

  // 空结果要分清两种空。第二种在项目刚起步时是常态，说成「暂无数据」的话，
  // 人会以为是系统坏了，而实际该做的事完全不同——一个是放宽条件，一个是去复盘。
  const emptyHint = confirmed.length
    ? '这些条件下还没有能用的经验。放宽条件再看看，或者去「复盘」里沉淀一条。'
    : '这个 Project 还没有在用的经验。经验来自复盘——投完一轮、提交复盘、有人确认，它才会出现在这里。'

  return <div className="experience-lookup">
    <div className="core-flow-toolbar">
      <div>
        <span className="section-label">EXPERIENCE · LOOKUP</span>
        <h2>这一轮的条件下，以前什么有效</h2>
        <p>只列这个 Project 里在用的经验。默认不给「👁 只是观察」的——那些没排除掉别的变量，不能照着做。</p>
      </div>
      <button className="secondary-button" disabled={listState === 'loading'} onClick={() => { void runLookup(lookup) }}>
        <RefreshCw size={15}/>刷新
      </button>
    </div>

    <p className="lookup-context">
      按 {scope || '（没设条件）'} 筛出 {matches.length} 条。
      {lookup.include_observed ? '含只是观察的。' : ''}
    </p>

    <div className="lookup-filters">
      <Field label="品牌" value={lookup.brand} options={options.brand} onChange={value => update({ brand: value })}/>
      <Field label="产品" value={lookup.product} options={options.product} onChange={value => update({ product: value })}/>
      <Field label="渠道" value={lookup.channel} options={options.channel} onChange={value => update({ channel: value })}/>
      <Field label="广告类型" value={lookup.ad_type} options={options.ad_type} onChange={value => update({ ad_type: value })}/>
      <Field label="目标" value={lookup.objective} options={options.objective} onChange={value => update({ objective: value })}/>
      <Field label="受众" value={lookup.audience} options={options.audience} onChange={value => update({ audience: value })}/>
      <label className="lookup-field">
        <small>内容特征</small>
        <input value={lookup.feature ?? ''} placeholder="例如：开场"
          onChange={event => update({ feature: event.target.value })}/>
      </label>
      <label className="lookup-toggle">
        <input type="checkbox" checked={Boolean(lookup.include_observed)}
          onChange={event => update({ include_observed: event.target.checked })}/>
        连只是观察的也看
      </label>
      <button type="button" className="text-button"
        onClick={() => setLookup({})}>清空条件</button>
    </div>

    {listState === 'loading' ? <p className="panel-empty">正在按条件筛…</p> : null}
    {listState === 'error' ? <p className="panel-empty">{notice || '经验读取失败，请重试。'}</p> : null}
    {listState === 'ready' && !matches.length ? <p className="panel-empty">{emptyHint}</p> : null}

    {matches.map(match => <ExperienceCard key={match.experience.id}
      experience={match.experience} matched={match.matched} citation={match.citation_text}/>)}
  </div>
}

/** 一格条件。有历史取值就给下拉，没有就只给输入框——下拉里一个选项都没有更让人困惑。 */
function Field({ label, value, options, onChange }: {
  label: string
  value: string | undefined
  options: Set<string> | undefined
  onChange: (value: string) => void
}) {
  const list = options ? [...options].sort() : []
  return <label className="lookup-field">
    <small>{label}</small>
    {list.length
      ? <select value={value ?? ''} onChange={event => onChange(event.target.value)}>
        <option value="">不限</option>
        {list.map(item => <option key={item} value={item}>{item}</option>)}
        {/* 预填的值可能不在历史取值里（项目品牌还没沉淀过经验）。
            不补这一项，select 会显示成「不限」，而 state 里其实卡着这个值。 */}
        {value && !list.includes(value) ? <option value={value}>{value}</option> : null}
      </select>
      : <input value={value ?? ''} onChange={event => onChange(event.target.value)} placeholder="不限"/>}
  </label>
}

/** 空字符串不发出去。后端把空串当「不限」，但发一堆空字段会让请求体读起来像设了条件。 */
function clean(lookup: ApiExperienceLookup): ApiExperienceLookup {
  const next: ApiExperienceLookup = {}
  if (lookup.brand?.trim()) next.brand = lookup.brand.trim()
  if (lookup.product?.trim()) next.product = lookup.product.trim()
  if (lookup.channel?.trim()) next.channel = lookup.channel.trim()
  if (lookup.ad_type?.trim()) next.ad_type = lookup.ad_type.trim()
  if (lookup.objective?.trim()) next.objective = lookup.objective.trim()
  if (lookup.audience?.trim()) next.audience = lookup.audience.trim()
  if (lookup.feature?.trim()) next.feature = lookup.feature.trim()
  if (lookup.include_observed) next.include_observed = true
  return next
}
