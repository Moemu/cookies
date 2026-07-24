import { act, cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { BriefDraft } from './types'
import { BriefCompanion, ConversationPane } from './StrategyWorkspacePage'

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
})
