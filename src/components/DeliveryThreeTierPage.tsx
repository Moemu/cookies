import { useEffect, useMemo, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { Check, CircleAlert, FilePlus, RefreshCw, Send, ShieldCheck, SlidersHorizontal } from 'lucide-react'
import {
  DeliveryApiError,
  deliveryConfigurationApi,
  deliveryPlanApi,
  type DeliveryControlChangeSet,
  type DeliveryFieldValue,
  type DeliveryPlan,
  type DeliveryPlanDraft,
  type DeliveryRecommendation,
  type DeliveryThreeTierField,
  type ManualActionPackage,
} from '../api/delivery'
import { useProject } from '../context/ProjectContext'
import type { DataState, ProjectRecord } from '../types'
import { StateBoundary } from './StateBoundary'

type TierObjectType = 'group' | 'plan' | 'creative'

type OverrideTarget = { groupId: string; planId: string; creativeId: string; field: DeliveryThreeTierField } | undefined

function formatCny(value: number) {
  return (value / 100).toLocaleString('zh-CN', { style: 'currency', currency: 'CNY' })
}

function formatTime(value?: string) {
  return value ? new Date(value).toLocaleString('zh-CN', { dateStyle: 'medium', timeStyle: 'short' }) : '尚无记录'
}

function valueText(value: DeliveryFieldValue | undefined) {
  if (value === undefined || value === null || value === '') return '未设置'
  if (Array.isArray(value)) return value.join('、')
  if (typeof value === 'boolean') return value ? '是' : '否'
  return String(value)
}

function fieldValueText(key: string, value: DeliveryFieldValue | undefined) {
  if ((key === 'budget' || key === 'bid') && typeof value === 'number') return formatCny(value)
  if (key === 'landing_page' && value) return '已配置落地页（地址见技术披露）'
  if (key === 'tracking' && value) return '已配置追踪标识（值见技术披露）'
  return ({
    platform_pending: '待平台人工填写',
    current_project_only: '仅限当前 Project',
    not_submitted: '未提交',
    manual_review_required: '需人工复核',
    mock_image_text: '模拟图文',
    project_mock_audience: '当前 Project 模拟受众',
    lead_submit: '销售线索提交',
  } as Record<string, string>)[String(value)] ?? valueText(value)
}

function fieldSourceLabel(value: string) {
  return ({ mock_fixture: '模拟夹具', recommendation: '建议', manual_override: '人工覆盖', recommended: '推荐值' } as Record<string, string>)[value] ?? value
}

function expectedResultLabel(value: string) {
  return value === 'set the reviewed mock value manually without submitting or enabling delivery'
    ? '在不提交、不启用投放的前提下，按人工复核值填写'
    : value
}

function forbiddenActionLabel(value: string) {
  return ({ submit: '禁止提交', enable: '禁止启用投放', budget_expansion: '禁止扩大预算', credentials: '禁止填写或收集凭据', unknown_pages: '禁止操作未知页面', platform_api_call: '禁止调用平台 API', automatic_execution: '禁止自动执行' } as Record<string, string>)[value] ?? value
}

function fieldLabel(key: string) {
  return ({
    group_name: '广告组名称', group_objective: '营销目标', advertiser: '广告主', business_asset_boundary: '业务资产边界',
    plan_name: '广告计划名称', placement: '版位', optimization: '优化目标', audience: '受众', budget: '计划预算', bid: '出价', schedule: '投放排期', conversion: '转化目标', tracking: '追踪标识',
    creative_name: '创意名称', asset_version: '素材版本', title: '标题', format: '创意格式', landing_page: '落地页', call_to_action: '行动按钮', review_status: '审核状态', disclosure: '披露信息',
  } as Record<string, string>)[key] ?? key
}

function recommendationLabel(value: string) {
  return ({
    reduce_mock_budget: '降低模拟计划预算',
    'reduces only the mock budget by 10%': '仅将模拟计划预算下调 10%，不扩大花费。',
    mock_budget_reduction_only: '仅限模拟预算下调',
    'observe mock conversion cost for 24 hours after manual application': '人工应用后观察 24 小时模拟转化成本。',
  } as Record<string, string>)[value] ?? value
}

function readOverrideValue(raw: string, exemplar: DeliveryFieldValue | undefined): DeliveryFieldValue {
  if (typeof exemplar === 'number') return Number(raw)
  if (typeof exemplar === 'boolean') return raw === 'true'
  return raw
}

function threeTierDraft(project: ProjectRecord): DeliveryPlanDraft {
  const projectCode = project.code || 'LOCAL'
  return {
    name: `${project.brand || 'Cookies'} 三层投放配置`,
    objective: project.goal || '获取高质量销售线索',
    advertiser: { id: 'mock-advertiser-001', name: 'Cookies 模拟广告主', platform: 'ocean_engine' },
    budget: { totalMinor: Math.max(project.budget || 3000, 0) * 100, currency: 'CNY' },
    schedule: { startAt: '2026-08-01T00:00:00.000Z', endAt: '2026-08-31T00:00:00.000Z', timezone: project.timezone || 'Asia/Shanghai' },
    tracking: { landingPage: `https://demo.cookies.local/lead/${projectCode.toLowerCase()}`, pixelId: `PX-${projectCode}-LEAD`, conversionEvent: 'lead_submit' },
    creativeReferences: [{ assetId: project.artifacts.creative.id || `asset-${projectCode.toLowerCase()}-mock`, version: 1, confirmed: true }],
    sourceStrategyVersion: project.artifacts.strategy.version || 'strategy-v1',
  }
}

function errorMessage(error: unknown, fallback: string) {
  if (error instanceof DeliveryApiError) {
    if (error.code === 'VERSION_CONFLICT') return '版本已更新：请刷新当前 Project 后重新操作。'
    if (error.status === 403 || error.status === 401) return '当前身份无权执行此受控操作。'
    return error.message
  }
  return error instanceof Error ? error.message : fallback
}

function OverrideDialog({
  target,
  value,
  confirmation,
  busy,
  onValueChange,
  onConfirmationChange,
  onClose,
  onSave,
}: {
  target: NonNullable<OverrideTarget>
  value: string
  confirmation: boolean
  busy: boolean
  onValueChange: (value: string) => void
  onConfirmationChange: (confirmed: boolean) => void
  onClose: () => void
  onSave: () => void
}) {
  const inputRef = useRef<HTMLInputElement>(null)
  const onCloseRef = useRef(onClose)
  onCloseRef.current = onClose

  useEffect(() => {
    const previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null
    const previousOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    inputRef.current?.focus()
    return () => {
      document.body.style.overflow = previousOverflow
      previousFocus?.focus()
    }
  }, [])

  useEffect(() => {
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === 'Escape' && !busy) onCloseRef.current()
    }
    document.addEventListener('keydown', closeOnEscape)
    return () => document.removeEventListener('keydown', closeOnEscape)
  }, [busy])

  return createPortal(<div
    className="delivery-config-override-backdrop"
    onMouseDown={event => {
      if (event.target === event.currentTarget && !busy) onClose()
    }}
  >
    <section className="delivery-config-override" role="dialog" aria-modal="true" aria-label={`人工覆盖：${target.field.label}`}>
      <header>
        <div><span>人工覆盖</span><h3>{target.field.label}</h3><p>只会生成新的模拟计划版本；不会向平台写入。</p></div>
        <button type="button" onClick={onClose} disabled={busy}>关闭</button>
      </header>
      <form onSubmit={event => { event.preventDefault(); onSave() }}>
        <label>覆盖值<input ref={inputRef} value={value} onChange={event => onValueChange(event.target.value)} disabled={busy}/></label>
        {target.field.confirmation?.required ? <label className="delivery-config-confirm"><input type="checkbox" checked={confirmation} onChange={event => onConfirmationChange(event.target.checked)} disabled={busy}/>我已确认此项覆盖：{target.field.confirmation.label || '需要人工确认'}</label> : null}
        <footer><button type="submit" className="primary-button" disabled={busy}>保存人工覆盖</button></footer>
      </form>
    </section>
  </div>, document.body)
}

function ManualPackageDetails({ value }: { value: ManualActionPackage }) {
  return <div className="delivery-config-package">
    <header className="delivery-config-package-heading">
      <div>
        <b>操作包已就绪</b>
        <span title={`source=${value.source} · scenario=${value.scenario}`}>生成于 {formatTime(value.generatedAt)} · 模拟环境 · 人工执行包</span>
      </div>
      <strong>仅供人工执行</strong>
    </header>
    <section className="delivery-config-package-safety" aria-label="人工执行安全边界">
      <ShieldCheck size={18}/>
      <div>
        <b>人工执行安全边界</b>
        <p>本操作包只授权人工复核与填写。以下操作不在授权范围内，请勿执行：</p>
        <ul>{value.forbiddenActions.map(action => <li key={action}><Check size={12}/><span>{forbiddenActionLabel(action)}</span></li>)}</ul>
      </div>
    </section>
    <section className="delivery-config-package-instructions" aria-label="待人工填写清单">
      <header><b>待人工填写清单</b><span>{value.instructions.length} 项 · 按广告组、计划、创意顺序</span></header>
      <ul>{value.instructions.map((instruction, index) => <li key={`${instruction.fieldKey}-${index}`}><b>{fieldLabel(instruction.fieldKey)}：{fieldValueText(instruction.fieldKey, instruction.effectiveValue)}</b><span>来源：{fieldSourceLabel(instruction.source)} · {instruction.confirmationRequired ? '待人工确认' : '已确认'} · 预期：{expectedResultLabel(instruction.expectedResult)}</span></li>)}</ul>
    </section>
  </div>
}

function TierObject({
  type,
  object,
  location,
  onOverride,
}: {
  type: TierObjectType
  object: { id: string; label: string; fields: DeliveryThreeTierField[] }
  location?: Omit<NonNullable<OverrideTarget>, 'field'>
  onOverride: (target: NonNullable<OverrideTarget>) => void
}) {
  return <section className={`delivery-config-tier-object delivery-config-tier-object--${type}`}>
    <header><span>{type === 'group' ? '广告组' : type === 'plan' ? '广告计划' : '广告创意'}</span><h4>{object.label}</h4></header>
    {object.fields.length ? <div className="delivery-config-field-list">
      {object.fields.map(field => <article key={field.key} className="delivery-config-field">
        <div><b>{field.label}</b><span>当前采用：{fieldValueText(field.key, field.effectiveValue)}</span></div>
        <div className="delivery-config-field-state">
          <span className={field.platformRequired ? 'delivery-config-required' : ''}>{field.platformRequired ? '需在平台填写' : '模拟配置'}</span>
          <span>{field.mockRequired ? '模拟必填' : '模拟可选'}</span>
          <span className={field.confirmation?.required ? 'delivery-config-required' : ''}>{field.confirmation?.required ? '待人工确认' : '已确认'}</span>
          <span>{field.editable ? '可人工覆盖' : '已锁定'}</span>
          {field.editable && location ? <button onClick={() => onOverride({ ...location, field })}><SlidersHorizontal size={14}/>人工覆盖</button> : null}
        </div>
        <details><summary>来源、依赖与风险</summary><dl>
          <div><dt>建议值</dt><dd>{valueText(field.recommendedValue)}</dd></div>
          <div><dt>人工值</dt><dd>{valueText(field.manualValue)}</dd></div>
          <div><dt>生效来源</dt><dd>{fieldSourceLabel(field.effectiveSource)}</dd></div>
          <div><dt>平台状态</dt><dd>{field.platformStatus}</dd></div>
          <div><dt>来源引用</dt><dd>{field.sourceRefs.join('、') || '无'}</dd></div>
          <div><dt>依赖</dt><dd>{field.dependencyRefs.join('、') || '无'}</dd></div>
          <div><dt>风险</dt><dd>{field.riskRefs.join('、') || '无'}</dd></div>
          <div><dt>证据</dt><dd>{field.evidenceRefs.join('、') || '无'}</dd></div>
        </dl></details>
      </article>)}
    </div> : <p className="delivery-config-empty-inline">该层暂无可显示字段。</p>}
  </section>
}

export function DeliveryThreeTierPage({ state, activeView }: { state: DataState; activeView: string }) {
  const { currentProject } = useProject()
  const projectId = currentProject.id
  const [plans, setPlans] = useState<DeliveryPlan[]>([])
  const [recommendations, setRecommendations] = useState<DeliveryRecommendation[]>([])
  const [selectedId, setSelectedId] = useState(() => new URLSearchParams(window.location.search).get('plan_id') ?? '')
  const [changeSet, setChangeSet] = useState<DeliveryControlChangeSet>()
  const [manualPackage, setManualPackage] = useState<ManualActionPackage>()
  const [target, setTarget] = useState<OverrideTarget>()
  const [overrideValue, setOverrideValue] = useState('')
  const [confirmation, setConfirmation] = useState(false)
  const [preflightMessage, setPreflightMessage] = useState('尚未运行计划预检。')
  const [notice, setNotice] = useState('')
  const [busy, setBusy] = useState(false)

  const selectedPlan = useMemo(() => plans.find(plan => plan.id === selectedId), [plans, selectedId])
  const configuration = selectedPlan?.currentVersion.threeTierConfiguration
  const showConfiguration = activeView === '模拟配置'
  const showPreflight = activeView === '预检与审批'
  const showRecommendations = activeView === '建议与人工操作包'
  const selectedRecommendations = useMemo(() => recommendations.filter(item => item.planId === selectedId), [recommendations, selectedId])

  const restoreWorkflow = async (planId: string) => {
    if (!planId) return
    try {
      const changeSets = await deliveryPlanApi.listChangeSets(projectId)
      const restored = changeSets.find(item => item.planId === planId)
      setChangeSet(restored)
      setManualPackage(undefined)
      if (restored?.status === 'approved') {
        try {
          setManualPackage(await deliveryConfigurationApi.getManualActionPackage(projectId, restored.id))
        } catch (error) {
          if (!(error instanceof DeliveryApiError) || error.status !== 404) throw error
        }
      }
    } catch (error) {
      setNotice(errorMessage(error, '恢复当前计划的 ChangeSet 与人工操作包失败。'))
    }
  }

  const refresh = async () => {
    if (!projectId) return
    setBusy(true)
    try {
      const [nextPlans, nextRecommendations] = await Promise.all([
        deliveryPlanApi.list(projectId),
        deliveryConfigurationApi.listRecommendations(projectId),
      ])
      setPlans(nextPlans)
      setRecommendations(nextRecommendations)
      setSelectedId(current => nextPlans.some(plan => plan.id === current) ? current : nextPlans[0]?.id ?? '')
      setNotice(nextPlans.length ? '已刷新当前 Project 的模拟配置与建议。' : '当前 Project 暂无投放计划，可创建模拟配置。')
    } catch (error) {
      setNotice(errorMessage(error, '读取三层配置失败。'))
    } finally {
      setBusy(false)
    }
  }

  useEffect(() => { void refresh() }, [projectId])
  useEffect(() => {
    if (!selectedId) return
    const url = new URL(window.location.href)
    url.searchParams.set('plan_id', selectedId)
    window.history.replaceState(window.history.state, '', url)
  }, [activeView, selectedId])
  useEffect(() => {
    if (selectedId) void restoreWorkflow(selectedId)
  }, [projectId, selectedId])

  const updatePlan = (next: DeliveryPlan) => {
    setPlans(current => [...current.filter(item => item.id !== next.id), next])
    setSelectedId(next.id)
  }

  const createPlan = async () => {
    setBusy(true)
    try {
      const created = await deliveryPlanApi.create(projectId, threeTierDraft(currentProject))
      updatePlan(created)
      setNotice('已创建模拟投放计划；下一步请由服务端编译三层配置。')
    } catch (error) {
      setNotice(errorMessage(error, '创建模拟计划失败。'))
    } finally { setBusy(false) }
  }

  const compile = async () => {
    if (!selectedPlan) return
    setBusy(true)
    try {
      const compiled = await deliveryConfigurationApi.compile(projectId, selectedPlan.id, selectedPlan.currentVersionNumber, 'golden_path')
      updatePlan(compiled)
      setNotice('三层配置已由服务端编译为新不可变版本；仅含模拟数据，不会写入广告平台。')
    } catch (error) {
      setNotice(errorMessage(error, '编译三层配置失败。'))
    } finally { setBusy(false) }
  }

  const selectOverride = (next: NonNullable<OverrideTarget>) => {
    setTarget(next)
    setOverrideValue(valueText(next.field.effectiveValue))
    setConfirmation(Boolean(next.field.confirmation?.confirmed))
  }

  const applyOverride = async () => {
    if (!selectedPlan || !target) return
    if (target.field.confirmation?.required && !confirmation) {
      setNotice('该字段需要人工确认后才能覆盖。')
      return
    }
    setBusy(true)
    try {
      const overridden = await deliveryConfigurationApi.override(projectId, selectedPlan.id, {
        expectedVersion: selectedPlan.currentVersionNumber,
        groupId: target.groupId,
        planId: target.planId,
        creativeId: target.creativeId,
        fieldKey: target.field.key,
        value: { type: target.field.valueType, value: readOverrideValue(overrideValue, target.field.effectiveValue) },
        confirmed: confirmation,
      })
      updatePlan(overridden)
      setTarget(undefined)
      setNotice('人工覆盖已生成新的模拟计划版本，尚未向平台写入任何内容。')
    } catch (error) {
      setNotice(errorMessage(error, '保存人工覆盖失败。'))
    } finally { setBusy(false) }
  }

  const preflightPlan = async () => {
    if (!selectedPlan) return
    setBusy(true)
    try {
      const result = await deliveryPlanApi.preflight(projectId, selectedPlan.id)
      setPreflightMessage(result.passed ? `计划预检通过（${formatTime(result.checkedAt)}）。` : `计划预检被阻断：${result.checks.filter(check => !check.passed).map(check => check.message).join('；')}`)
    } catch (error) { setNotice(errorMessage(error, '计划预检失败。')) } finally { setBusy(false) }
  }

  const createAndPreflightChangeSet = async () => {
    if (!selectedPlan) return
    setBusy(true)
    try {
      const draft = changeSet?.status === 'draft'
        ? changeSet
        : await deliveryPlanApi.createChangeSet(projectId, selectedPlan.id, selectedPlan.currentVersionNumber)
      const checked = await deliveryPlanApi.preflightChangeSet(projectId, draft.id, draft.version)
      setChangeSet(checked)
      setNotice(checked.status === 'preflight_passed'
        ? 'ChangeSet 预检通过，仍需人工审批；不会执行平台写入。'
        : 'ChangeSet 预检未通过，请查看服务端门禁。')
    } catch (error) { setNotice(errorMessage(error, '创建或预检 ChangeSet 失败。')) } finally { setBusy(false) }
  }

  const approveChangeSet = async () => {
    if (!changeSet) return
    setBusy(true)
    try {
      const approved = await deliveryPlanApi.approveChangeSet(projectId, changeSet.id, changeSet.version)
      setChangeSet(approved)
      setNotice('ChangeSet 已获人工审批。接下来只能编译人工操作包，页面不会发起平台写入。')
    } catch (error) { setNotice(errorMessage(error, '审批 ChangeSet 失败。')) } finally { setBusy(false) }
  }

  const generateRecommendations = async () => {
    if (!selectedPlan) return
    setBusy(true)
    try {
      const generated = await deliveryConfigurationApi.generateRecommendations(projectId, selectedPlan.id, selectedPlan.currentVersionNumber)
      setRecommendations(current => [...current.filter(item => item.id !== generated.id), generated])
      setNotice('已生成 1 条建议；生成只记录建议，不会执行。')
    } catch (error) { setNotice(errorMessage(error, '生成建议失败。')) } finally { setBusy(false) }
  }

  const acceptRecommendation = async (recommendation: DeliveryRecommendation) => {
    setBusy(true)
    try {
      const accepted = await deliveryConfigurationApi.acceptRecommendation(projectId, recommendation.id, recommendation.version, `three-tier-${recommendation.id}-${recommendation.version}`)
      setRecommendations(current => current.map(item => item.id === accepted.recommendation.id ? accepted.recommendation : item))
      setChangeSet(accepted.changeSet)
      setNotice('建议已采纳为一个新的草稿 ChangeSet；它尚未预检、审批或执行。')
    } catch (error) { setNotice(errorMessage(error, '采纳建议失败。')) } finally { setBusy(false) }
  }

  const rejectRecommendation = async (recommendation: DeliveryRecommendation) => {
    setBusy(true)
    try {
      const rejected = await deliveryConfigurationApi.rejectRecommendation(projectId, recommendation.id, recommendation.version)
      setRecommendations(current => current.map(item => item.id === rejected.id ? rejected : item))
      setNotice('建议已拒绝，未创建任何 ChangeSet。')
    } catch (error) { setNotice(errorMessage(error, '拒绝建议失败。')) } finally { setBusy(false) }
  }

  const compileManualPackage = async () => {
    if (!changeSet || changeSet.status !== 'approved') return
    setBusy(true)
    try {
      const compiled = await deliveryConfigurationApi.compileManualActionPackage(projectId, changeSet.id, changeSet.version)
      setManualPackage(compiled)
      setNotice('已编译人工操作包；请由授权人员在平台中手工执行，系统没有写入平台。')
    } catch (error) { setNotice(errorMessage(error, '编译人工操作包失败。')) } finally { setBusy(false) }
  }

  return <StateBoundary state={state} contextLabel="智能投放 / 三级配置编排" errorDetail="当前 Project 的三层投放配置无法读取，请确认 Delivery 服务可用后刷新。">
    <div className="delivery-config-workspace">
      <header className="delivery-config-heading">
        <div><span className="section-label">模拟投放编排</span><h2>广告组 → 广告计划 → 广告创意</h2><p>所有结果都留在模拟环境、计划版本、ChangeSet 或人工操作包中；本页从不写入广告平台。</p></div>
        <div className="delivery-config-source"><b>模拟环境</b><span>source=mock · scenario=golden_path</span><button onClick={() => void refresh()} disabled={busy}><RefreshCw size={14}/>刷新当前 Project</button></div>
      </header>

      <section className="delivery-config-toolbar">
        <label>投放计划<select value={selectedId} onChange={event => setSelectedId(event.target.value)}>{plans.map(plan => <option value={plan.id} key={plan.id}>{plan.currentVersion.name} · V{plan.currentVersionNumber}</option>)}</select></label>
        {showConfiguration ? <>
          <button className="secondary-button" onClick={() => void createPlan()} disabled={busy}><FilePlus size={15}/>创建模拟计划</button>
          <button className="primary-button" onClick={() => void compile()} disabled={busy || !selectedPlan}><Send size={15}/>编译三层配置</button>
        </> : <span className="delivery-config-view-context">当前视图沿用所选计划，不会重复创建任务。</span>}
      </section>

      {!selectedPlan ? <div className="panel-empty">{showConfiguration ? '尚无模拟计划。创建后可由服务端编译三级配置。' : '当前 Project 尚无模拟计划，请先切换到“模拟配置”视图创建计划。'}</div> : <>
        {showConfiguration ? <section className="delivery-config-config-card">
          <header><div><span>当前计划 V{selectedPlan.currentVersionNumber}</span><h3>{selectedPlan.currentVersion.name}</h3><p>预算 {formatCny(selectedPlan.currentVersion.budget.totalMinor)} · 更新时间 {formatTime(selectedPlan.updatedAt)}</p></div><div className="delivery-config-contract"><b>{configuration ? '已编译' : '待编译'}</b><span>{configuration ? `模拟数据 · ${formatTime(configuration.generatedAt)}` : '请先服务端编译配置'}</span></div></header>
          {configuration ? <><div className="delivery-config-config-meta"><span>schema={configuration.schema}</span><span>source={configuration.source}</span><span>scenario={configuration.scenario}</span><span>证据 {configuration.evidenceRefs.length} 条</span></div>
            <div className="delivery-config-tier-tree">{configuration.groups.map(group => <div key={group.id}><TierObject type="group" object={group} onOverride={selectOverride}/>{group.plans.map(plan => <div className="delivery-config-tier-indent" key={plan.id}><TierObject type="plan" object={plan} onOverride={selectOverride}/>{plan.creatives.map(creative => <div className="delivery-config-tier-indent" key={creative.id}><TierObject type="creative" object={creative} location={{ groupId: group.id, planId: plan.id, creativeId: creative.id }} onOverride={selectOverride}/></div>)}</div>)}</div>)}</div>
          </> : <div className="delivery-config-empty-inline"><CircleAlert size={18}/>服务端尚未生成配置。编译使用固定 mock fixture，客户端不上传配置内容。</div>}
        </section> : null}

        {showPreflight ? <section className="delivery-config-flow-grid delivery-config-flow-grid--preflight">
          <article className="delivery-config-preflight-card">
            <header><div><span className="section-label">当前计划门禁</span><h3>预检与审批</h3></div><strong className="delivery-config-preflight-state">{changeSet ? `ChangeSet ${changeSet.status}` : '尚未创建 ChangeSet'}</strong></header>
            <div className="delivery-config-preflight-summary"><b>校验结果</b><p>{preflightMessage}</p>{changeSet ? <small>版本 {changeSet.version}{changeSet.status === 'draft' ? ' · 采纳仅创建草稿，仍需预检与审批' : ''}</small> : <small>先运行计划预检，再创建需要审批的变更。</small>}</div>
            <div className="delivery-config-actions delivery-config-preflight-actions"><button onClick={() => void preflightPlan()} disabled={busy}><ShieldCheck size={14}/>运行计划预检</button><button onClick={() => void createAndPreflightChangeSet()} disabled={busy}><Check size={14}/>{changeSet?.status === 'draft' ? '预检已采纳的草稿 ChangeSet' : '创建并预检 ChangeSet'}</button><button onClick={() => void approveChangeSet()} disabled={busy || changeSet?.status !== 'preflight_passed'}>人工审批 ChangeSet</button></div>
          </article>
        </section> : null}

        {showRecommendations ? <section className="delivery-config-flow-grid delivery-config-flow-grid--decision">
          <article>
            <header><h3>建议只待决策</h3></header>
            <p>生成建议只保留证据、影响、风险、观察期与冷却期，不会执行投放。</p>
            <button onClick={() => void generateRecommendations()} disabled={busy || !configuration}><Send size={14}/>生成建议</button>
            <div className="delivery-config-recommendations">
              {selectedRecommendations.map(item => <div key={item.id}>
                <b>{recommendationLabel(item.action)}</b><span>{recommendationLabel(item.impact)}</span>
                <small>风险：{item.risks.map(recommendationLabel).join('、') || '无'} · 观察：{recommendationLabel(item.observation)} · 冷却至：{formatTime(item.cooldown)}</small>
                <footer><em>{item.status}</em>{item.status === 'proposed' ? <>
                  <button onClick={() => void acceptRecommendation(item)} disabled={busy}>采纳为草稿</button>
                  <button onClick={() => void rejectRecommendation(item)} disabled={busy}>拒绝</button>
                </> : null}</footer>
              </div>)}
              {!selectedRecommendations.length ? <small>尚无建议。</small> : null}
            </div>
          </article>
          <article><header><h3>人工操作包</h3></header><p>仅在 ChangeSet 审批后可编译。它是给人工投手的步骤包，不是平台执行指令。</p><button onClick={() => void compileManualPackage()} disabled={busy || changeSet?.status !== 'approved'}><FilePlus size={14}/>编译人工操作包</button>{manualPackage ? <ManualPackageDetails value={manualPackage}/> : <small>等待审批完成。</small>}</article>
        </section> : null}
      </>}

      {target ? <OverrideDialog
        target={target}
        value={overrideValue}
        confirmation={confirmation}
        busy={busy}
        onValueChange={setOverrideValue}
        onConfirmationChange={setConfirmation}
        onClose={() => setTarget(undefined)}
        onSave={() => void applyOverride()}
      /> : null}
      {notice ? <div className="inline-notice" role="status">{notice}</div> : null}
    </div>
  </StateBoundary>
}
