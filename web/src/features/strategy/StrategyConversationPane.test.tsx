import { act, cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { BriefDraft } from './types'
import { BriefCompanion, BriefPane, ConversationPane, StrategyPane } from './StrategyWorkspacePage'

const brief: BriefDraft = {
  id: 'brief_draft_1',
  brief_id: 'brief_1',
  status: 'open',
  version: 3,
  document: {
    contract_version: 'strategy-brief-version/v2',
    brand: { name: '灵裁' },
    product: { name: '电商创作' },
    industry: '信息服务业',
    region: '',
    language: '',
    campaign: { objective: '' },
    audience: { primary: '' },
    proposition: '',
    channels: [],
    budget: { total: '' },
    schedule: { window: '' },
    constraints: [],
    measurement: { primary_kpi: '' },
  },
  field_states: {
    'brand.name': {
      field_path: 'brand.name',
      confidence: 'high',
      confirmation: 'unconfirmed',
      source: { type: 'conversation_message', id: 'msg_1' },
    },
    'product.name': {
      field_path: 'product.name',
      confidence: 'high',
      confirmation: 'confirmed',
      source: { type: 'conversation_message', id: 'msg_1' },
    },
    industry: {
      field_path: 'industry',
      confidence: 'high',
      confirmation: 'unconfirmed',
      source: { type: 'conversation_message', id: 'msg_1' },
    },
  },
  completeness: {
    ready: false,
    blockers: [
      { field: 'brand.name', reason: '需要用户确认' },
      { field: 'region', reason: '必填信息缺失' },
    ],
    warnings: [],
  },
}

describe('Strategy conversation experience', () => {
  afterEach(cleanup)

  it('starts from conversational prompts instead of a blank form', () => {
    const onSend = vi.fn()
    render(<ConversationPane brief={brief} busy={false} messages={[]} onSend={onSend} />)
    expect(screen.getByRole('heading', { name: '把模糊想法聊清楚' })).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: '我有一个新品牌，需要从零梳理推广需求' }))
    expect(onSend).toHaveBeenCalledWith('我有一个新品牌，需要从零梳理推广需求')
  })

  it('shows the current model reply and a visible thinking state', () => {
    render(<ConversationPane
      brief={brief}
      busy
      messages={[{
        id: 'msg_2',
        conversation_id: 'conversation_1',
        role: 'assistant',
        content_type: 'text',
        content: '收到，我已经记录了品牌、产品和行业信息。',
        ai_generated: true,
        created_at: '2026-07-24T00:00:00Z',
      }]}
      onSend={vi.fn()}
    />)
    expect(screen.getByText('收到，我已经记录了品牌、产品和行业信息。')).toBeInTheDocument()
    expect(screen.getByText('正在对照会话记忆并更新 Brief')).toBeInTheDocument()
  })

  it('reveals a completed assistant message progressively', () => {
    vi.useFakeTimers()
    try {
      const onStreamComplete = vi.fn()
      const response = '收到，我已经记住品牌和产品信息，接下来确认本次推广目标。'
      render(<ConversationPane
        brief={brief}
        busy={false}
        messages={[{
          id: 'msg_stream',
          conversation_id: 'conversation_1',
          role: 'assistant',
          content_type: 'text',
          content: response,
          ai_generated: true,
          agent_task_id: 'agent_task_1',
          created_at: '2026-07-24T00:00:00Z',
        }]}
        onSend={vi.fn()}
        onStreamComplete={onStreamComplete}
        streamingMessageId="msg_stream"
      />)
      expect(screen.getByTestId('streaming-assistant').textContent).toBe('')
      act(() => vi.advanceTimersByTime(24))
      expect(screen.getByTestId('streaming-assistant').textContent).not.toBe('')
      expect(screen.getByTestId('streaming-assistant')).not.toHaveTextContent(response)
      act(() => vi.runAllTimers())
      expect(screen.getByTestId('streaming-assistant')).toHaveTextContent(response)
      expect(onStreamComplete).toHaveBeenCalledOnce()
    } finally {
      vi.useRealTimers()
    }
  })

  it('separates captured, confirmed, and missing Brief facts', () => {
    const onOpen = vi.fn()
    render(<BriefCompanion
      brief={brief}
      memory={{ summary: '品牌：灵裁；产品：电商创作；行业：信息服务业', open_questions: ['主要投放地区是哪里？'], version: 3 }}
      onOpen={onOpen}
    />)
    expect(screen.getByText('品牌：灵裁；产品：电商创作；行业：信息服务业')).toBeInTheDocument()
    expect(screen.getByText('灵裁')).toBeInTheDocument()
    expect(screen.getAllByText('已记录').length).toBeGreaterThan(0)
    expect(screen.getAllByText('已确认').length).toBeGreaterThan(0)
    expect(screen.getAllByText('待补充').length).toBeGreaterThan(0)
    fireEvent.click(screen.getByRole('button', { name: '查看并确认完整 Brief' }))
    expect(onOpen).toHaveBeenCalledOnce()
  })

  it('groups Brief fields into an editor and readiness rail', () => {
    render(<BriefPane
      brief={brief}
      busy={false}
      documents={[]}
      memory={null}
      onConfirm={vi.fn()}
      onField={vi.fn()}
      onResearch={vi.fn()}
      onUpload={vi.fn()}
      researchRun={null}
    />)
    expect(screen.getByRole('heading', { name: '确认策略输入' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: '品牌与业务' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: '传播任务' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: '投放条件' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: '完成度' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '确认并冻结 Brief' })).toBeDisabled()
  })

  it('separates conversation memory, project documents, and optional external research', () => {
    const onResearch = vi.fn()
    render(<BriefPane
      brief={brief}
      busy={false}
      documents={[]}
      memory={{ summary: '品牌与产品信息已经确认。', open_questions: [], version: 4 }}
      onConfirm={vi.fn()}
      onField={vi.fn()}
      onResearch={onResearch}
      onUpload={vi.fn()}
      researchRun={null}
    />)

    expect(screen.getByRole('heading', { name: '对话共识' })).toBeInTheDocument()
    expect(screen.getByText('仅用于当前 Project 的需求梳理，不会随外部研究自动发送。')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: '项目资料' })).toBeInTheDocument()
    expect(screen.queryByPlaceholderText('例如：近半年小红书护肤新品常用的内容切入点是什么？')).not.toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: '补充外部研究' }))
    const query = screen.getByPlaceholderText('例如：近半年小红书护肤新品常用的内容切入点是什么？')
    const submit = screen.getByRole('button', { name: '确认并开始研究' })
    fireEvent.change(query, { target: { value: '验证小红书护肤新品的内容切入点' } })
    expect(submit).toBeDisabled()
    fireEvent.click(screen.getByRole('checkbox', { name: '同意执行外部研究' }))
    expect(submit).toBeEnabled()
    fireEvent.click(submit)
    expect(onResearch).toHaveBeenCalledWith('web', '验证小红书护肤新品的内容切入点', false)
  })

  it('keeps sources and external research read-only after the Brief is frozen', () => {
    render(<BriefPane
      brief={{ ...brief, status: 'confirmed' }}
      busy={false}
      documents={[]}
      memory={null}
      onConfirm={vi.fn()}
      onField={vi.fn()}
      onResearch={vi.fn()}
      onUpload={vi.fn()}
      researchRun={null}
    />)

    fireEvent.click(screen.getByRole('button', { name: '补充外部研究' }))
    expect(screen.getByText('当前 Brief 已冻结，资料来源随本版本锁定。补充资料或研究时，需要先进入新一轮需求梳理并形成新的 BriefVersion。')).toBeInTheDocument()
    expect(screen.getByLabelText('本次要验证的问题')).toBeDisabled()
    expect(screen.getByLabelText('外部研究方式')).toBeDisabled()
    expect(screen.getByRole('button', { name: '确认并开始研究' })).toBeDisabled()
  })

  it('shows a useful Strategy preflight instead of an empty canvas', () => {
    render(<StrategyPane
      busy={false}
      canGenerate={false}
      draft={null}
      generationMetadata={null}
      onApprove={vi.fn()}
      onCreateCreative={vi.fn()}
      onFeedback={vi.fn(async () => true)}
      onGenerate={vi.fn()}
      onPatch={vi.fn()}
      onRevise={vi.fn(async () => true)}
      onSubmit={vi.fn()}
      packageVersion={null}
      readiness={null}
      review={null}
      skillRuns={[]}
    />)
    expect(screen.getByRole('heading', { name: '生成第一版策略' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: '生成前检查' })).toBeInTheDocument()
    expect(screen.getByText('Brief 已冻结')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '生成第一版策略' })).toBeDisabled()
  })
})
