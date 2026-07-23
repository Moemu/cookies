import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { ApiProblem } from '../../shared/api/client'
import { getProjectContext } from '../platform/api'
import type { ProjectContext } from '../platform/types'
import { approveStrategy, createProposal, generateStrategy } from './api'
import type { ProposalInput, StrategyProposal } from './types'

const initialInput: ProposalInput = {
  brand: '极地鲜生',
  product: '深海鳕鱼柳',
  target_audience: '25-40 岁关注品质与效率的城市家庭',
  platform: '抖音',
  budget: '618 整合营销预算',
  compliance: ['禁用绝对化用语', '不得承诺功效'],
}

function message(error: unknown) {
  if (error instanceof ApiProblem) return `${error.message}（${error.problem.error.code}）`
  return error instanceof Error ? error.message : '策略操作失败，请稍后重试。'
}

export function StrategyWorkspacePage() {
  const { projectId = '' } = useParams()
  const [input, setInput] = useState<ProposalInput>(initialInput)
  const [proposal, setProposal] = useState<StrategyProposal | null>(null)
  const [context, setContext] = useState<ProjectContext | null>(null)
  const [busy, setBusy] = useState<'create' | 'generate' | 'approve' | null>(null)
  const [error, setError] = useState('')

  useEffect(() => {
    const controller = new AbortController()
    getProjectContext(projectId, controller.signal).then(setContext).catch((caught: unknown) => {
      if (!(caught instanceof DOMException && caught.name === 'AbortError')) setError(message(caught))
    })
    return () => controller.abort()
  }, [projectId])

  async function create() {
    if (busy) return
    setBusy('create')
    setError('')
    try {
      setProposal(await createProposal(projectId, input))
    } catch (caught) {
      setError(message(caught))
    } finally {
      setBusy(null)
    }
  }

  async function generate() {
    if (!proposal || busy) return
    setBusy('generate')
    setError('')
    try {
      setProposal(await generateStrategy(projectId, proposal.id))
    } catch (caught) {
      setError(message(caught))
    } finally {
      setBusy(null)
    }
  }

  async function approve() {
    if (!proposal?.strategy || busy) return
    setBusy('approve')
    setError('')
    try {
      const strategy = await approveStrategy(projectId, proposal.strategy.id)
      setProposal((current) => current ? { ...current, strategy } : current)
    } catch (caught) {
      setError(message(caught))
    } finally {
      setBusy(null)
    }
  }

  const strategy = proposal?.strategy
  const approved = strategy?.status === 'approved'

  return <section className="workflow-page">
    <header className="page-header">
      <div>
        <h1>项目策略工作区</h1>
        <p>将项目上下文转化为可审阅策略。策略获批前，不能创建创意计划或提交生成任务。</p>
      </div>
      <div className="context-summary"><span className={context?.brand_id ? 'context-dot context-dot--active' : 'context-dot'} /><span>项目上下文</span><strong>{context ? `v${context.project_context_version}` : '加载中'}</strong></div>
    </header>

    <div className="workflow-layout">
      <form className="workflow-card workflow-form" onSubmit={(event) => { event.preventDefault(); void create() }}>
        <div className="workflow-card__heading"><div><h2>提案简报</h2><p>提案输入与合规边界会随策略一起留存。</p></div><span className="workflow-status">{proposal ? proposal.status : '待创建'}</span></div>
        <label>品牌<input disabled={Boolean(proposal)} onChange={(event) => setInput((current) => ({ ...current, brand: event.target.value }))} value={input.brand} /></label>
        <label>产品<input disabled={Boolean(proposal)} onChange={(event) => setInput((current) => ({ ...current, product: event.target.value }))} value={input.product} /></label>
        <label>目标人群<input disabled={Boolean(proposal)} onChange={(event) => setInput((current) => ({ ...current, target_audience: event.target.value }))} value={input.target_audience} /></label>
        <label>投放平台<input disabled={Boolean(proposal)} onChange={(event) => setInput((current) => ({ ...current, platform: event.target.value }))} value={input.platform} /></label>
        <label>预算范围<input disabled={Boolean(proposal)} onChange={(event) => setInput((current) => ({ ...current, budget: event.target.value }))} value={input.budget} /></label>
        <div className="workflow-compliance"><strong>合规边界</strong>{input.compliance.map((item) => <span key={item}>{item}</span>)}</div>
        <button className="button button--primary" disabled={Boolean(proposal) || busy !== null} type="submit">{busy === 'create' ? '正在创建…' : '创建项目提案'}</button>
      </form>

      <article className="workflow-card workflow-output" aria-live="polite">
        <div className="workflow-card__heading"><div><h2>策略输出</h2><p>使用版本化模板生成，审批动作不可由创意页面绕过。</p></div>{proposal ? <span className="workflow-version">{proposal.template_version}</span> : null}</div>
        {!proposal ? <p className="workflow-empty">先创建项目提案，再生成可审阅的策略输出。</p> : <>
          <dl className="workflow-facts"><div><dt>提案 ID</dt><dd>{proposal.id}</dd></div><div><dt>策略状态</dt><dd>{strategy?.status === 'approved' ? '已审批' : strategy ? '待审批' : '待生成'}</dd></div></dl>
          {strategy ? <pre className="strategy-json">{JSON.stringify(strategy.content, null, 2)}</pre> : <p className="workflow-empty">尚未生成策略。生成会使用当前项目上下文与简报中的合规限制。</p>}
          <div className="workflow-actions">
            <button className="button button--secondary" disabled={Boolean(strategy) || busy !== null} onClick={() => void generate()} type="button">{busy === 'generate' ? '正在生成…' : '生成策略'}</button>
            <button className="button button--primary" disabled={!strategy || approved || busy !== null} onClick={() => void approve()} type="button">{busy === 'approve' ? '正在审批…' : approved ? '策略已审批' : '审批策略'}</button>
            {approved ? <Link className="button button--secondary" to={`/projects/${encodeURIComponent(projectId)}/creative?strategy=${encodeURIComponent(strategy.id)}`}>进入创意工作区</Link> : null}
          </div>
        </>}
      </article>
    </div>
    {error ? <div className="library-error" role="alert"><div><strong>策略工作区操作失败</strong><span>{error}</span></div><button className="text-button" onClick={() => setError('')} type="button">关闭</button></div> : null}
  </section>
}
