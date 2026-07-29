import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  ArrowRight, BookOpenCheck, CircleAlert, CircleCheck, Database, FileInput,
  Layers3, Lightbulb, Link2, RefreshCw, Search, ShieldCheck, Target,
} from 'lucide-react'
import { useProject } from '../context/ProjectContext'
import {
  api,
  type ApiConfidenceLevel,
  type ApiExperienceReference,
  type ApiFeaturePattern,
  type ApiInsightCard,
  type ApiInsightCardType,
  type ApiPreLaunchInsight,
} from '../data/api'
import type { DataState, SystemKey } from '../types'
import { StateBoundary } from './StateBoundary'

/**
 * 投前洞察（03-material-insight-system.md §8，导航见 19 §5.2）。
 *
 * 这一页回答的是「开工之前，历史上已经知道了什么」。它不是一张新表：洞察卡是
 * 经验库的投影，九个字段（结论/类型/适用范围/数据依据/内容依据/置信提示/
 * 风险与反例/建议动作/状态）全部来自 insight_experiences，后端权威定义在
 * internal/systems/insights/prelaunch.go。
 *
 * 五个视图查的确实是不同的数据（22 §8.3）：
 *   策略证据 —— 只有「事实」和「统计观察」两类卡，因为只有它们能被当证据引用；
 *   创意建议 —— 只有「建议」类卡，它们必须带建议动作；
 *   历史模式 —— 查的不是卡而是特征出现次数，是一次聚合；
 *   风险与反例 —— 带反例的卡，加上置信偏弱（样本不足/存在混杂）的卡；
 *   引用记录 —— 查的是 experience_references，即下游到底用没用。
 *
 * 三条边界必须在界面上看得见，否则这一页会变成「拿历史数据编故事」：
 *   1. 跨渠道比较默认关闭（03 §10.3②）。不同渠道的数字不能直接比大小，
 *      要比必须是人主动打开，并且知道自己在比什么。
 *   2. 数据质量不过关时不许下强结论（03 §10.3③）。后端算不出来时会返回
 *      quality_checked=false，那是「不知道」，不是「通过」，界面要照实说。
 *   3. 九字段没填全的卡照常显示，只标出缺哪几项。藏起来会让人以为经验库是空的。
 */
type ViewTarget = 'evidence' | 'recommendation' | 'pattern' | 'risk' | 'reference'

const viewTargets: Record<string, ViewTarget> = {
  策略证据: 'evidence',
  创意建议: 'recommendation',
  历史模式: 'pattern',
  风险与反例: 'risk',
  引用记录: 'reference',
  // 旧导航只有一个视图「策略与创意证据」，保留映射避免存量链接落到空页。
  '策略与创意证据': 'evidence',
}

const cardTypeLabels: Record<ApiInsightCardType, string> = {
  fact: '事实',
  statistic: '统计观察',
  hypothesis: '假设',
  recommendation: '建议',
}

// 类型不是标签，是这条结论能被怎么用。写在旁边，避免有人拿假设当证据。
const cardTypeMeaning: Record<ApiInsightCardType, string> = {
  fact: '数据里直接读到的，没有加工。可以直接引用。',
  statistic: '在数据上算出来的观察，口径写在数据依据里。可以引用，但要连口径一起引。',
  hypothesis: '还没被验证的猜测。可以拿去做实验，不能拿去当证据。',
  recommendation: '人给出的行动建议。它的说服力来自它引用的证据，不来自它自己。',
}

const confidenceLabels: Record<ApiConfidenceLevel, string> = {
  sufficient: '充分',
  directional: '方向性',
  low_sample: '样本不足',
  confounded: '存在混杂',
}

const headings: Record<ViewTarget, { title: string; blurb: string }> = {
  evidence: {
    title: '可以拿去支撑决策的证据',
    blurb: '只有「事实」和「统计观察」两类卡在这里。假设和建议不在——拿假设当证据是循环论证，Brief 里写一句「根据经验」而背后是猜测，比没有证据更糟。',
  },
  recommendation: {
    title: '可以直接写进 Brief 的动作',
    blurb: '每条建议都必须带一个具体动作，否则它只是一句感想。建议的说服力来自它引用的数据依据和内容依据，所以这两项也一并列出。',
  },
  pattern: {
    title: '反复出现的特征',
    blurb: '这是一次按内容特征的聚合，不是卡的列表：某个特征在多少条已确认的结论里出现过。它是计数，不是排名——出现次数多不等于效果好，只等于「这件事被反复观察到」。',
  },
  risk: {
    title: '别照抄的那些',
    blurb: '两类卡在这里：写了反例的，和置信偏弱的（样本不足、存在混杂）。它们不是错的结论，是有边界的结论。这一页存在的意义是让边界和结论一起被看到。',
  },
  reference: {
    title: '这些结论后来被谁用了',
    blurb: '引用记录反过来检验经验库：一条从来没人引用的结论，要么没人看得见，要么没人觉得有用。被引用后标成「没采纳」的，比没被引用的更有价值——那说明有人认真读过。',
  },
}

const emptyHints: Record<ViewTarget, string> = {
  evidence: '当前筛选下没有「事实」或「统计观察」类的已确认结论。经验库里可能有假设和建议，切到别的视图看看；也可能是这个项目还没有沉淀过复盘。',
  recommendation: '当前筛选下没有建议类结论。建议通常在复盘报告确认时沉淀，可以先去报告中心看看有没有待确认的复盘。',
  pattern: '还没有足够的已确认结论来统计特征。特征来自内容依据字段，如果结论里没写清依据了哪些内容特征，这里就是空的。',
  risk: '当前筛选下没有带反例的结论，置信也都不弱。这不代表没有风险，只代表还没有人把风险写下来。',
  reference: '这个项目还没有任何引用记录。从左边的证据或建议视图里引用一条，这里就会出现。',
}

// consumer_kind 后端没有受控词表（自由文本，≤32 字），这里只翻译已知的几种，
// 其余原样显示——猜错比显示英文更糟。
const consumerLabels: Record<string, string> = {
  brief: 'Brief',
  strategy: '策略',
  creative_task: '创意任务',
  creative: '创意',
  report: '报告',
}

const outcomeLabels: Record<string, string> = {
  referenced: '已引用',
  adopted: '照做了',
  modified: '改了之后用的',
  rejected: '没采纳',
}

type OpenProject = (id: string, system?: SystemKey, navId?: string, objectId?: string, view?: string) => void

export function PreLaunchInsightPage({ state, activeView, onOpenProject }: {
  state: DataState
  activeView: string
  onOpenProject: OpenProject
}) {
  const { currentProject, reloadProjects } = useProject()
  const target = viewTargets[activeView] ?? 'evidence'
  const [channel, setChannel] = useState('')
  const [creativeType, setCreativeType] = useState('')
  const [objective, setObjective] = useState('')
  const [query, setQuery] = useState('')
  const [crossChannel, setCrossChannel] = useState(false)
  const [insight, setInsight] = useState<ApiPreLaunchInsight | null>(null)
  const [references, setReferences] = useState<ApiExperienceReference[]>([])
  const [selectedId, setSelectedId] = useState('')
  const [notice, setNotice] = useState('')
  const [busyTarget, setBusyTarget] = useState<'brief' | 'creative' | ''>('')
  const [listState, setListState] = useState<'loading' | 'ready' | 'error'>('loading')

  const load = useCallback(async () => {
    if (!currentProject.id) return
    setListState('loading')
    try {
      const [next, referenceList] = await Promise.all([
        api.getPreLaunch(currentProject.id, {
          channel, creative_type: creativeType, objective, q: query.trim(),
          cross_channel: crossChannel,
        }),
        api.listProjectExperienceReferences(currentProject.id).then(page => page.items).catch(() => []),
      ])
      setInsight(next)
      setReferences(referenceList)
      setListState('ready')
    } catch (cause) {
      setInsight(null)
      setListState('error')
      setNotice(cause instanceof Error ? cause.message : '投前洞察读取失败。')
    }
  }, [currentProject.id, channel, creativeType, objective, query, crossChannel])

  useEffect(() => { void load() }, [load])

  // 切视图时清掉上一次引用的回执：那句话是对上一个动作的回应，
  // 留在新视图里会被读成「这个视图刚发生了什么」。
  useEffect(() => { setNotice('') }, [target])

  const cards = useMemo(() => {
    const all = insight?.cards ?? []
    if (target === 'evidence') return all.filter(card => card.type === 'fact' || card.type === 'statistic')
    if (target === 'recommendation') return all.filter(card => card.type === 'recommendation')
    if (target === 'risk') {
      return all.filter(card =>
        card.counterexamples.length > 0 || card.confidence === 'low_sample' || card.confidence === 'confounded')
    }
    return all
  }, [insight, target])

  useEffect(() => {
    const ids = cards.map(card => card.experience_id)
    setSelectedId(current => ids.includes(current) ? current : ids[0] ?? '')
  }, [cards])

  const selected = cards.find(card => card.experience_id === selectedId)

  const cite = useCallback(async (card: ApiInsightCard, consumer: 'brief' | 'creative') => {
    setBusyTarget(consumer)
    setNotice('')
    try {
      // 引用记在经验自己身上（AM-014），不是只在 Brief 里留一句话：
      // 只有这样「这条结论后来被谁用了」才答得出来。
      const consumerId = consumer === 'brief'
        ? currentProject.artifacts.brief.id || currentProject.id
        : ([...currentProject.tasks].reverse().find(task => task.type !== 'strategy')?.id ?? currentProject.id)
      await api.recordExperienceReference(currentProject.id, card.experience_id, {
        consumer_kind: consumer === 'brief' ? 'brief' : 'creative_task',
        consumer_id: consumerId,
        outcome: 'referenced',
        note: `投前洞察引用：${card.conclusion}`,
      })
      if (consumer === 'brief' && currentProject.artifacts.brief.id) {
        const artifacts = await api.listArtifacts(currentProject.id)
        const brief = artifacts.find(artifact => artifact.id === currentProject.artifacts.brief.id)
        // 已经引用过就不再追加：重复引用要留在引用记录里，不该把 Brief 越写越长。
        if (brief && !brief.content.includes(card.experience_id)) {
          await api.updateArtifact(brief.id, {
            content: `${brief.content}\n\n投前洞察引用 ${card.experience_id}（${cardTypeLabels[card.type]}·置信${confidenceLabels[card.confidence]}）：${card.recommended_action || card.conclusion}`,
          })
        }
      }
      await reloadProjects()
      await load()
      setNotice(consumer === 'brief'
        ? '已写入当前 Brief，并在这条经验上记下了引用。'
        : '已在这条经验上记下引用，关联到最近一个创意任务。')
    } catch (cause) {
      setNotice(cause instanceof Error ? cause.message : '引用失败，请稍后重试。')
    } finally {
      setBusyTarget('')
    }
  }, [currentProject, reloadProjects, load])

  const facets = insight?.facets
  const patterns = insight?.patterns ?? []

  return <StateBoundary state={state} onRetry={() => { void load() }}>
    <div className="prelaunch-workspace">
      <section className="prelaunch-main">
        <div className="core-flow-toolbar">
          <div>
            <span className="section-label">PRE-LAUNCH INSIGHT</span>
            <h2>{headings[target].title}</h2>
            <p>当前 Project：{currentProject.name}。{headings[target].blurb}</p>
          </div>
          <div className="core-flow-actions">
            <div className="prelaunch-context"><Target size={16}/><span><small>项目目标</small><b>{currentProject.goal}</b></span></div>
            <button className="secondary-button" disabled={listState === 'loading'} onClick={() => { void load() }}>
              <RefreshCw size={15}/>刷新
            </button>
          </div>
        </div>

        {/* 能不能下强结论排在所有内容前面（03 §10.3③）。算不出来要说「不知道」，不能默认成通过。 */}
        {insight && !insight.quality_checked ? <div className="feature-stack">
          <span>这一次没能校验数据质量</span>
          <b>下面的结论仍然照常显示，但它们背后的数据是否新鲜、是否缺失、口径是否一致，这次没查出来。这是「不知道」，不是「没问题」。</b>
        </div> : null}
        {insight?.quality_checked && !insight.strong_conclusions_allowed ? <div className="feature-stack">
          <span>这个窗口的数据不支持强结论</span>
          {insight.quality_blockers.map(blocker => <b key={blocker}>{blocker}</b>)}
          <b>下面的结论可以看，可以作为做实验的方向，但不该写成 Brief 里的硬要求，也不该触发自动优化动作。</b>
        </div> : null}

        <div className="prelaunch-filterbar">
          <div className="search-field">
            <Search size={15}/>
            <input aria-label="搜索投前洞察" value={query} onChange={event => setQuery(event.target.value)}
              placeholder="搜索结论、建议动作或特征"/>
          </div>
          <label>渠道<select aria-label="渠道" value={channel} onChange={event => setChannel(event.target.value)}>
            <option value="">全部渠道</option>
            {facets?.channels.map(value => <option key={value} value={value}>{value}</option>)}
          </select></label>
          <label>形式<select aria-label="创意形式" value={creativeType} onChange={event => setCreativeType(event.target.value)}>
            <option value="">全部形式</option>
            {facets?.creative_types.map(value => <option key={value} value={value}>{value}</option>)}
          </select></label>
          <label>目标<select aria-label="投放目标" value={objective} onChange={event => setObjective(event.target.value)}>
            <option value="">全部目标</option>
            {facets?.objectives.map(value => <option key={value} value={value}>{value}</option>)}
          </select></label>
          <span>{target === 'pattern' ? `${patterns.length} 个特征` : target === 'reference' ? `${references.length} 条引用` : `${cards.length} 条结论`}</span>
        </div>

        {/* 跨渠道比较默认关闭（03 §10.3②）：混了渠道的数字直接比大小是错的，要比得人主动打开。 */}
        {insight && insight.mixed_channels.length > 1 ? <div className="prelaunch-boundary">
          <CircleAlert size={16}/>
          <span>
            <small>当前结果跨了 {insight.mixed_channels.length} 个渠道</small>
            {insight.mixed_channels.join(' / ')}。不同渠道的曝光口径、计费方式和受众都不一样，
            把它们的数字直接比大小得不出可信结论。
            <label className="prelaunch-crosschannel">
              <input type="checkbox" checked={crossChannel} onChange={event => setCrossChannel(event.target.checked)}/>
              我知道口径不同，仍要横向比较
            </label>
          </span>
        </div> : null}

        {insight && insight.unscoped_excluded > 0 ? <div className="prelaunch-boundary">
          <CircleAlert size={16}/>
          <span>
            <small>有 {insight.unscoped_excluded} 条结论被筛选排除了</small>
            它们没有写适用范围，所以说不清适不适用于当前筛选条件。清空筛选可以看到它们。
          </span>
        </div> : null}

        {listState === 'loading' ? <div className="panel-empty">正在读取当前 Project 的历史结论…</div> : null}
        {listState === 'error' ? <div className="panel-empty">读取失败，请重试。</div> : null}

        {listState === 'ready' && target === 'pattern'
          ? <PatternTable patterns={patterns}/>
          : listState === 'ready' && target === 'reference'
            ? <ReferenceTable references={references} cards={insight?.cards ?? []}/>
            : listState === 'ready'
              ? <CardTable cards={cards} selectedId={selectedId} emptyHint={emptyHints[target]} onSelect={setSelectedId}/>
              : null}
      </section>

      <aside className="prelaunch-detail">
        {target === 'pattern' || target === 'reference'
          ? <div className="panel-empty">{target === 'pattern'
            ? '这个视图统计的是特征出现次数，没有单条结论可选。要看某个特征背后的结论，切到「策略证据」按关键词搜。'
            : '引用记录是只读的：它记录已经发生过的引用，不能在这里补记。'}</div>
          : selected
            ? <CardDetail card={selected} busy={busyTarget} onCite={cite}/>
            : <div className="panel-empty">左边选一条结论，查看它的九个字段和适用边界。</div>}
        {selected && (target === 'evidence' || target === 'recommendation' || target === 'risk')
          ? <button className="text-button prelaunch-deeplink"
            onClick={() => onOpenProject(currentProject.id, 'strategy', 'briefs')}>
            进入需求中心检查 Brief<ArrowRight size={14}/>
          </button>
          : null}
        <div className="reference-count">
          <BookOpenCheck size={15}/>
          <span><b>{references.length} 条引用记录</b><small>记在经验自己身上，可跨项目追溯</small></span>
        </div>
        {notice ? <div className="inline-notice" role="status">{notice}</div> : null}
        {insight?.disclosure ? <p className="prelaunch-disclosure">{insight.disclosure}</p> : null}
      </aside>
    </div>
  </StateBoundary>
}

function CardTable({ cards, selectedId, emptyHint, onSelect }: {
  cards: ApiInsightCard[]
  selectedId: string
  emptyHint: string
  onSelect: (id: string) => void
}) {
  if (!cards.length) return <div className="panel-empty">{emptyHint}</div>
  return <div className="prelaunch-table" role="list" aria-label="洞察卡列表">
    <div className="prelaunch-row header"><span>结论</span><span>类型</span><span>数据依据</span><span>置信</span></div>
    {cards.map(card => <button role="listitem" key={card.experience_id}
      className={card.experience_id === selectedId ? 'prelaunch-row active' : 'prelaunch-row'}
      onClick={() => onSelect(card.experience_id)}>
      <span>
        <b>{card.conclusion}</b>
        <small>
          {describeApplicability(card)}
          {card.missing_fields.length ? ` · 缺 ${card.missing_fields.join('、')}` : ''}
        </small>
      </span>
      <span>{cardTypeLabels[card.type]}</span>
      <span>{describeDataBasis(card)}</span>
      <span>
        {card.confidence === 'sufficient' ? <CircleCheck size={14}/> : <CircleAlert size={14}/>}
        {confidenceLabels[card.confidence]}
      </span>
    </button>)}
  </div>
}

function PatternTable({ patterns }: { patterns: ApiFeaturePattern[] }) {
  if (!patterns.length) return <div className="panel-empty">{emptyHints.pattern}</div>
  return <div className="prelaunch-table" role="list" aria-label="历史模式列表">
    <div className="prelaunch-row header"><span>特征</span><span>出现次数</span><span>出现渠道</span><span>最强置信</span></div>
    {patterns.map(pattern => <div role="listitem" key={pattern.feature} className="prelaunch-row">
      <span><b>{pattern.feature}</b><small>{pattern.conclusions.slice(0, 2).join('；')}</small></span>
      <span>{pattern.card_count} 条结论提到</span>
      <span>{pattern.channels.length ? pattern.channels.join(' / ') : '未写明渠道'}</span>
      <span>{confidenceLabels[pattern.best_confidence]}</span>
    </div>)}
  </div>
}

function ReferenceTable({ references, cards }: { references: ApiExperienceReference[]; cards: ApiInsightCard[] }) {
  if (!references.length) return <div className="panel-empty">{emptyHints.reference}</div>
  const conclusions = new Map(cards.map(card => [card.experience_id, card.conclusion]))
  return <div className="prelaunch-table" role="list" aria-label="引用记录列表">
    <div className="prelaunch-row header"><span>被引用的结论</span><span>用在哪</span><span>后来怎么样</span><span>时间</span></div>
    {references.map(reference => <div role="listitem" key={reference.id} className="prelaunch-row">
      <span>
        {/* 引用记录会比当前筛选出的卡更多：被引用的可能是旧版本，也可能已失效或被筛掉。
            这些记录必须留着——引用发生在当时，不因为结论后来变了就当没发生过。 */}
        <b>{conclusions.get(reference.experience_id) ?? '（引用的是旧版本或已失效的结论，当前列表里看不到它）'}</b>
        <small>{reference.experience_id}</small>
      </span>
      <span>{consumerLabels[reference.consumer_kind] ?? reference.consumer_kind}</span>
      <span>{outcomeLabels[reference.outcome] ?? reference.outcome}</span>
      <span>{formatTime(reference.created_at)}</span>
    </div>)}
  </div>
}

function CardDetail({ card, busy, onCite }: {
  card: ApiInsightCard
  busy: 'brief' | 'creative' | ''
  onCite: (card: ApiInsightCard, consumer: 'brief' | 'creative') => Promise<void>
}) {
  // 只有事实和统计观察能被当证据引用（03 §2 目标⑥）。假设和建议也允许引用，
  // 但要先告诉引用的人它的分量不一样，否则 Brief 里会出现「根据经验」而背后是猜测。
  const quotableAsEvidence = card.type === 'fact' || card.type === 'statistic'
  return <>
    <span className="section-label">{cardTypeLabels[card.type]} · 置信{confidenceLabels[card.confidence]}</span>
    <h3>{card.conclusion}</h3>
    <p>{card.experience_id} · 第 {card.revision} 版 · 被引用 {card.reference_count} 次 · 更新于 {formatTime(card.updated_at)}</p>

    <div className="prelaunch-fact"><Layers3 size={17}/><span><small>这条能被怎么用</small><b>{cardTypeMeaning[card.type]}</b></span></div>
    <div className="prelaunch-fact"><Target size={17}/><span><small>适用范围</small><b>{describeApplicability(card)}</b></span></div>
    <div className="prelaunch-fact"><Database size={17}/><span><small>数据依据</small><b>{describeDataBasis(card)}</b></span></div>
    <div className="prelaunch-fact"><Lightbulb size={17}/><span><small>内容依据</small><b>{describeContentBasis(card)}</b></span></div>
    {/* 提示文案由后端给（ConfidenceLevel.Hint）：这句话决定别人敢拿它做什么，
        前端再写一份迟早会和后端说得不一样。 */}
    <div className="prelaunch-fact"><ShieldCheck size={17}/><span><small>置信提示</small><b>
      {confidenceLabels[card.confidence]} · {card.confidence_hint}
    </b></span></div>
    {card.recommended_action
      ? <div className="prelaunch-fact"><ArrowRight size={17}/><span><small>建议动作</small><b>{card.recommended_action}</b></span></div>
      : null}

    {card.conditions.length ? <div className="feature-stack">
      <span>适用条件</span>
      {card.conditions.map(condition => <b key={condition}>{condition}</b>)}
    </div> : null}

    {card.counterexamples.length ? <div className="prelaunch-boundary">
      <CircleAlert size={16}/>
      <span>
        <small>风险与反例</small>
        {card.counterexamples.join('；')}
      </span>
    </div> : null}

    {card.missing_fields.length ? <div className="prelaunch-boundary">
      <CircleAlert size={16}/>
      <span>
        <small>这张卡还缺 {card.missing_fields.length} 项</small>
        缺：{card.missing_fields.join('、')}。缺的部分不影响结论本身，但会让引用它的人无法判断边界。
        补齐要回经验库对这条做修订。
      </span>
    </div> : null}

    {!quotableAsEvidence ? <div className="prelaunch-boundary">
      <CircleAlert size={16}/>
      <span>
        <small>这不是一条证据</small>
        {card.type === 'hypothesis'
          ? '这是一条还没被验证的假设。引用它意味着「我们打算这么试」，不是「数据支持这么做」。'
          : '这是人给出的建议。它的说服力来自它引用的数据依据，引用时把依据一起带上。'}
      </span>
    </div> : null}

    <div className="prelaunch-actions">
      <button className="secondary-button full" disabled={Boolean(busy)} onClick={() => void onCite(card, 'brief')}>
        <FileInput size={15}/>{busy === 'brief' ? '正在写入 Brief…' : '引用到 Brief'}
      </button>
      <button className="primary-button full" disabled={Boolean(busy)} onClick={() => void onCite(card, 'creative')}>
        <Link2 size={15}/>{busy === 'creative' ? '正在关联创意…' : '引用到创意任务'}
      </button>
    </div>
  </>
}

function describeApplicability(card: ApiInsightCard): string {
  const scope = card.applicability
  const parts = [
    ...(scope.channels ?? []),
    ...(scope.creative_types ?? []),
    ...(scope.objectives ?? []),
    ...(scope.audiences ?? []),
  ]
  if (!parts.length) return '没写适用范围——这条结论没说自己适用于哪里'
  return parts.join(' · ') + (scope.time_range_note ? ` · ${scope.time_range_note}` : '')
}

function describeDataBasis(card: ApiInsightCard): string {
  const basis = card.data_basis
  const parts: string[] = []
  if (basis.asset_count) parts.push(`${basis.asset_count} 个素材`)
  if (basis.sample_size) parts.push(`样本 ${basis.sample_size.toLocaleString('zh-CN')}`)
  if (basis.metrics?.length) parts.push(basis.metrics.join('、'))
  if (basis.window_start && basis.window_end) parts.push(`${formatDate(basis.window_start)} ~ ${formatDate(basis.window_end)}`)
  if (basis.baseline) parts.push(`对照：${basis.baseline}`)
  return parts.length ? parts.join(' · ') : '没写数据依据'
}

function describeContentBasis(card: ApiInsightCard): string {
  const basis = card.content_basis
  const parts: string[] = []
  if (basis.features?.length) parts.push(basis.features.join('、'))
  if (basis.example_asset_versions?.length) parts.push(`示例素材 ${basis.example_asset_versions.length} 个`)
  if (basis.note) parts.push(basis.note)
  return parts.length ? parts.join(' · ') : '没写内容依据——不知道这条结论指的是素材上的哪个特征'
}

function formatDate(value: string): string {
  const date = new Date(value)
  return Number.isNaN(date.valueOf()) ? value : date.toLocaleDateString('zh-CN')
}

function formatTime(value: string): string {
  const date = new Date(value)
  return Number.isNaN(date.valueOf()) ? value : date.toLocaleString('zh-CN', { hour12: false })
}
