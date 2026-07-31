import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { InsightSection } from '../../app/routes'
import { insightEntry } from '../../app/routes'
import { InsightsWorkspacePage } from './InsightsWorkspacePage'
import type { Experience, ExperienceStatus } from './types'

const report = {
  id: 'report_1',
  organization_id: 'org_1',
  project_id: 'project_1',
  execution_id: 'execution_1',
  delivery_mode: 'local_simulation' as const,
  evidence_id: 'evidence_1',
  evidence_summary: '本地模拟投放证据：3 条小红书图文素材，投放 7 天。',
  metric_snapshot_id: 'metric_1',
  creative_package_id: 'package_1',
  is_simulated: true,
  dataset_version: 'preroll-demo/v1',
  status: 'confirmed' as const,
  summary: '标题控制在 22 字内的素材转化更好。',
  findings: ['CTR 提升 18%'],
  version: 1,
  created_by: 'user_1',
  created_at: '2026-07-28T00:00:00Z',
  updated_at: '2026-07-28T00:00:00Z',
}

function experience(id: string, status: ExperienceStatus, revision: number, conclusion: string): Experience {
  return {
    id,
    organization_id: 'org_1',
    project_id: 'project_1',
    lineage_id: 'lineage_1',
    revision,
    report_id: 'report_1',
    source_execution_id: 'execution_1',
    source_evidence_id: 'evidence_1',
    source_metric_snapshot_id: 'metric_1',
    conclusion,
    conditions: ['小红书图文'],
    counterexamples: ['仅有模拟指标'],
    status,
    status_reason: '',
    status_changed_by: 'user_1',
    version: 1,
    created_by: 'user_1',
    created_at: '2026-07-28T00:00:00Z',
    updated_at: '2026-07-28T00:00:00Z',
  }
}

const pending = experience('experience_2', 'pending', 2, '第二版待确认结论')
const confirmed = experience('experience_1', 'confirmed', 1, '已确认可复用结论')

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })
}

function stubFetch() {
  const fetchMock = vi.fn(async (input: string | URL | Request) => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url
    if (url.includes('/experiences?')) return jsonResponse({ items: [pending, confirmed] })
    if (url.includes('/lineage')) return jsonResponse({ items: [confirmed, pending] })
    if (url.includes('/audits')) {
      return jsonResponse({ items: [{
        id: 'audit_1',
        organization_id: 'org_1',
        project_id: 'project_1',
        experience_id: 'experience_2',
        from_status: '',
        to_status: 'pending',
        reason: '从复盘报告沉淀。',
        actor_id: 'user_1',
        created_at: '2026-07-28T00:00:00Z',
      }] })
    }
    // 项目级引用记录要排在单条经验的 /references 之前判断，两者路径互相包含。
    if (url.includes('/experience-references')) {
      return jsonResponse({ items: [{
        id: 'reference_1',
        organization_id: 'org_1',
        project_id: 'project_1',
        experience_id: 'experience_1',
        consumer_kind: 'strategy',
        consumer_id: 'strategy_9',
        outcome: 'modified',
        note: '改成 22 字标题后使用。',
        version: 1,
        created_by: 'user_1',
        created_at: '2026-07-28T00:00:00Z',
        updated_at: '2026-07-28T00:00:00Z',
      }] })
    }
    if (url.includes('/references')) return jsonResponse({ items: [] })
    if (url.includes('/reports')) return jsonResponse({ items: [report] })
    if (url.includes('/prelaunch')) return jsonResponse({ project_id: 'project_1', experience_references: [confirmed], disclosure: '披露' })
    if (url.includes('/performance')) return jsonResponse({ project_id: 'project_1', executions: [], disclosure: '披露' })
    if (url.includes(':confirm')) return jsonResponse({ ...pending, status: 'confirmed', version: 2 })
    // 记录引用走的是 :record-reference，和读取用的 /references 不是同一个路径。
    if (url.includes(':record-reference')) {
      return jsonResponse({
        id: 'reference_2',
        organization_id: 'org_1',
        project_id: 'project_1',
        experience_id: 'experience_1',
        consumer_kind: 'creative',
        consumer_id: 'task_7',
        outcome: 'adopted',
        note: '',
        version: 1,
        created_by: 'user_1',
        created_at: '2026-07-28T00:00:00Z',
        updated_at: '2026-07-28T00:00:00Z',
      })
    }
    return jsonResponse({}, 404)
  })
  vi.stubGlobal('fetch', fetchMock)
  return fetchMock
}

function renderInsight(section: InsightSection, view: string) {
  render(<MemoryRouter initialEntries={[`/projects/project_1/insight/${section}/${view}`]}>
    <Routes>
      <Route
        element={<InsightsWorkspacePage entry={insightEntry(section)!} view={view} />}
        path="/projects/:projectId/insight/:insightSection/:insightView"
      />
    </Routes>
  </MemoryRouter>)
}

function renderExperiences(view = 'pending') {
  renderInsight('experiences', view)
}

describe('经验库', () => {
  afterEach(() => {
    cleanup()
    vi.unstubAllGlobals()
  })

  it('五个视图挂在 URL 上，当前视图只列对应状态的经验', async () => {
    stubFetch()
    renderExperiences()

    expect(await screen.findByRole('link', { name: /^待确认1$/ })).toHaveAttribute(
      'href', '/projects/project_1/insight/experiences/pending')
    expect(screen.getByRole('link', { name: /^已确认1$/ })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /^已失效0$/ })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: '引用记录' })).toHaveAttribute(
      'href', '/projects/project_1/insight/experiences/references')

    expect(await screen.findByRole('heading', { name: '第二版待确认结论' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /已确认可复用结论/ })).not.toBeInTheDocument()
    expect(screen.getByText('未确认或已失效的经验不会出现在投前引用中，但会保留在库内可追溯。')).toBeInTheDocument()
    // 洞察模块面向的是不懂投放术语的人，界面上不该出现英文小标题或 Skills 这类词。
    expect(screen.queryByText(/INSIGHTS|NEW EXPERIENCE|NEW REVISION|DOWNSTREAM FEEDBACK|Skills/)).not.toBeInTheDocument()
    expect(await screen.findByText('从复盘报告沉淀。')).toBeInTheDocument()
  })

  it('确认待确认经验时带上乐观锁版本号', async () => {
    const fetchMock = stubFetch()
    renderExperiences()

    fireEvent.click(await screen.findByRole('button', { name: '确认经验' }))

    await waitFor(() => expect(fetchMock).toHaveBeenCalledWith(
      '/api/insights/v1/projects/project_1/experiences/experience_2:confirm',
      expect.objectContaining({ method: 'POST', body: JSON.stringify({ expected_version: 1 }) }),
    ))
  })

  it('废除经验必须先填写理由', async () => {
    stubFetch()
    renderExperiences('confirmed')

    expect(await screen.findByRole('heading', { name: '已确认可复用结论' })).toBeInTheDocument()
    expect(screen.getByText('已确认且未失效，下游环节可以引用该结论。')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: '废除经验' }))
    expect(screen.getByRole('button', { name: '提交' })).toBeDisabled()

    fireEvent.change(screen.getByRole('textbox', { name: '理由' }), { target: { value: '平台规则变化，结论不再成立。' } })
    expect(screen.getByRole('button', { name: '提交' })).toBeEnabled()
  })

  it('记录下游引用后立刻刷新引用列表', async () => {
    // 记录引用不改经验的版本号，早先版本因此不会重新拉取，写完看不到新记录。
    const fetchMock = stubFetch()
    renderExperiences('confirmed')

    // 先等详情自己那轮引用/轨迹加载落地，否则数不清刷新到底有没有发生。
    await screen.findByText('从复盘报告沉淀。')
    const reads = () => fetchMock.mock.calls.filter(([url]) =>
      String(url).includes('/experiences/experience_1/references?')).length
    expect(reads()).toBe(1)

    fireEvent.click(screen.getByRole('button', { name: '记录下游引用' }))
    fireEvent.change(screen.getByRole('combobox', { name: '用在哪里' }), { target: { value: 'creative' } })
    fireEvent.change(screen.getByRole('textbox', { name: '对象编号' }), { target: { value: 'task_7' } })
    fireEvent.click(screen.getByRole('button', { name: '记录引用' }))

    await waitFor(() => expect(fetchMock).toHaveBeenCalledWith(
      '/api/insights/v1/projects/project_1/experiences/experience_1:record-reference',
      expect.objectContaining({ method: 'POST' })))
    await waitFor(() => expect(reads()).toBe(2))
  })

  it('引用记录视图列出整个项目的下游引用', async () => {
    stubFetch()
    renderExperiences('references')

    // 卡片正面先说被引用的结论，下游对象翻成中文并把编号退到括号里。
    expect(await screen.findByText('已确认可复用结论')).toBeInTheDocument()
    expect(screen.getByText(/使用方：/)).toHaveTextContent('使用方：策略包（编号 strategy_9）')
    expect(screen.getByText('修改后使用')).toBeInTheDocument()
    expect(screen.getByText('改成 22 字标题后使用。')).toBeInTheDocument()
  })
})

describe('投前洞察', () => {
  afterEach(() => {
    cleanup()
    vi.unstubAllGlobals()
  })

  it('二级页签按导航规范列全，未开放的视图也可点进去', async () => {
    stubFetch()
    renderInsight('prelaunch', 'strategy-evidence')

    expect(await screen.findByRole('link', { name: '策略证据' })).toHaveAttribute(
      'href', '/projects/project_1/insight/prelaunch/strategy-evidence')
    expect(screen.getByRole('link', { name: '历史模式' })).toHaveAttribute(
      'href', '/projects/project_1/insight/prelaunch/patterns')
    expect(screen.getByRole('link', { name: '引用记录' })).toBeInTheDocument()
  })

  it('经验卡片展示报告里的证据说明，而不是数据库主键', async () => {
    stubFetch()
    renderInsight('prelaunch', 'strategy-evidence')

    // 证据说明在左边列表和右边详情里各出现一次，两处都不能露出报告主键。
    expect(await screen.findAllByText('本地模拟投放证据：3 条小红书图文素材，投放 7 天。')).toHaveLength(2)
    expect(screen.queryByText(/evidence_1/)).not.toBeInTheDocument()
    expect(screen.queryByText(/experience_1/)).not.toBeInTheDocument()
    expect(screen.getByText('第 1 版')).toBeInTheDocument()
  })
})
