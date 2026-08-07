import { useEffect, useMemo, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { ArrowRight, Check, CircleAlert, FilePlus, RefreshCw, Send, ShieldCheck, SlidersHorizontal } from 'lucide-react'
import {
  DeliveryApiError,
  deliveryConfigurationApi,
  deliveryPlanApi,
  type DeliveryControlChangeSet,
  type DeliveryFieldValue,
  type DeliveryPlan,
  type DeliveryThreeTierField,
  type ManualActionPackage,
} from '../api/delivery'
import { useProject } from '../context/ProjectContext'
import { projectPath } from '../lib/router'
import type { DataState } from '../types'
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

function readOverrideValue(raw: string, exemplar: DeliveryFieldValue | undefined): DeliveryFieldValue {
  if (typeof exemplar === 'number') return Number(raw)
  if (typeof exemplar === 'boolean') return raw === 'true'
  return raw
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
        <div><span>人工覆盖</span><h3>{target.field.label}</h3><p>保存后生成新的不可变计划版本。</p></div>
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
        <span title={`source=${value.source} · scenario=${value.scenario} · hash=${value.optimizedPlanHash}`}>生成于 {formatTime(value.generatedAt)} · 已物化优化计划 V{value.optimizedPlanVersion}</span>
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
          <span className={field.platformRequired ? 'delivery-config-required' : ''}>{field.platformRequired ? '需在平台填写' : '内部配置'}</span>
          <span>{field.mockRequired ? '当前必填' : '当前可选'}</span>
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

export function DeliveryThreeTierPage({ state, activeView, tourRunId, tourCase }: { state: DataState; activeView: string; tourRunId?: string; tourCase?: string }) {
  const { currentProject } = useProject()
  const projectId = currentProject.id
  const [plans, setPlans] = useState<DeliveryPlan[]>([])
  const [selectedId, setSelectedId] = useState(() => new URLSearchParams(window.location.search).get('plan_id') ?? '')
  const [changeSet, setChangeSet] = useState<DeliveryControlChangeSet>()
  const [manualPackage, setManualPackage] = useState<ManualActionPackage>()
  const [target, setTarget] = useState<OverrideTarget>()
  const [overrideValue, setOverrideValue] = useState('')
  const [confirmation, setConfirmation] = useState(false)
  const [preflightMessage, setPreflightMessage] = useState('尚未运行计划预检。')
  const [notice, setNotice] = useState('')
  const [busy, setBusy] = useState(false)
  const refreshGenerationRef = useRef(0)

  const selectedPlan = useMemo(() => plans.find(plan => plan.id === selectedId), [plans, selectedId])
  const configuration = selectedPlan?.currentVersion.threeTierConfiguration
  const showConfiguration = activeView === '配置映射'
  const showPreflight = activeView === '检查与提交'
  const showManualPackage = activeView === '人工操作包'
  const approvalURL = changeSet ? projectPath(projectId, 'delivery', 'approvals', changeSet.id, '待我审批', undefined, tourRunId, tourCase) : undefined
  const planEditorBaseURL = projectPath(projectId, 'delivery', 'plans', undefined, '计划列表', undefined, tourRunId, tourCase)
  const selectedPlanEditorURL = selectedPlan
    ? `${planEditorBaseURL}${planEditorBaseURL.includes('?') ? '&' : '?'}plan_id=${encodeURIComponent(selectedPlan.id)}`
    : planEditorBaseURL
  const canSubmitChangeSet = !changeSet || changeSet.status === 'draft' || changeSet.status === 'preflight_failed' || changeSet.status === 'rejected'

  const restoreWorkflow = async (planId: string) => {
    if (!planId) return
    try {
      const changeSets = await deliveryPlanApi.listChangeSets(projectId)
      const requestedChangeSetId = new URLSearchParams(window.location.search).get('change_set_id')
      const restored = changeSets.find(item => item.planId === planId && item.id === requestedChangeSetId)
        ?? changeSets.filter(item => item.planId === planId).sort((left, right) => right.updatedAt.localeCompare(left.updatedAt))[0]
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
      setNotice(errorMessage(error, '恢复当前计划的变更申请与人工操作包失败。'))
    }
  }

  const refresh = async () => {
    if (!projectId) return
    const generation = ++refreshGenerationRef.current
    setBusy(true)
    try {
      const nextPlans = await deliveryPlanApi.list(projectId)
      if (generation !== refreshGenerationRef.current) return
      setPlans(nextPlans)
      setSelectedId(current => nextPlans.some(plan => plan.id === current) ? current : nextPlans[0]?.id ?? '')
      setNotice(nextPlans.length ? '已刷新当前 Project 的内部配置。' : '当前 Project 暂无投放计划，可创建计划。')
    } catch (error) {
      if (generation !== refreshGenerationRef.current) return
      setNotice(errorMessage(error, '读取三层配置失败。'))
    } finally {
      if (generation === refreshGenerationRef.current) setBusy(false)
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

  const compile = async () => {
    if (!selectedPlan) return
    refreshGenerationRef.current += 1
    setBusy(true)
    try {
      const compiled = await deliveryConfigurationApi.compile(projectId, selectedPlan.id, selectedPlan.currentVersionNumber, 'golden_path')
      updatePlan(compiled)
      setNotice('三层配置已由服务端编译为新的不可变版本。')
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
      setNotice('人工覆盖已生成新的不可变计划版本。')
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
        ? '变更申请已提交审批中心，等待批准或打回。'
        : '变更申请未通过最终检查，请查看服务端门禁。')
    } catch (error) { setNotice(errorMessage(error, '提交变更申请失败。')) } finally { setBusy(false) }
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

  return <StateBoundary state={state} contextLabel="智能投放 / 内部配置编排" errorDetail="当前 Project 的内部投放配置无法读取，请确认 Delivery 服务可用后刷新。">
    <div className="delivery-config-workspace">
      <header className="delivery-config-heading">
        <div><span className="section-label">投放配置</span><h2>广告组、广告计划与广告创意配置</h2><p>三个对象并列配置；通常按广告组、广告计划、广告创意的顺序填写。</p></div>
        <div className="delivery-config-source"><button onClick={() => void refresh()} disabled={busy}><RefreshCw size={14}/>刷新当前 Project</button></div>
      </header>

      <section className="delivery-config-toolbar">
        <label>投放计划<select value={selectedId} onChange={event => setSelectedId(event.target.value)}>{plans.map(plan => <option value={plan.id} key={plan.id}>{plan.currentVersion.name} · V{plan.currentVersionNumber}</option>)}</select></label>
        {showConfiguration ? <>
          <a className="secondary-button" href={selectedPlanEditorURL}>在投放计划中查看</a>
          <button className="primary-button" onClick={() => void compile()} disabled={busy || !selectedPlan}><Send size={15}/>编译三层配置</button>
        </> : <span className="delivery-config-view-context">当前视图沿用所选计划，不会重复创建任务。</span>}
      </section>

      <dl className="delivery-config-runtime" aria-label="投放平台与配置编译器">
        <div><dt>投放平台</dt><dd>巨量引擎</dd></div>
        <div><dt>配置编译器</dt><dd>巨量内部配置 v1 <small>当前固定；平台字段校准后再开放适配器选择</small></dd></div>
      </dl>

      {!selectedPlan ? <div className="panel-empty">当前 Project 尚无投放计划。<a href={planEditorBaseURL}>前往投放计划创建</a>，保存后再返回编译配置。</div> : <>
        {showConfiguration ? <section className="delivery-config-config-card">
          <header><div><span>当前计划 V{selectedPlan.currentVersionNumber}</span><h3>{selectedPlan.currentVersion.name}</h3><p>预算 {formatCny(selectedPlan.currentVersion.budget.totalMinor)} · 更新时间 {formatTime(selectedPlan.updatedAt)}</p></div><div className="delivery-config-contract"><b>{configuration ? '已编译' : '待编译'}</b><span>{configuration ? `生成于 ${formatTime(configuration.generatedAt)}` : '请先服务端编译配置'}</span></div></header>
          {configuration ? <><div className="delivery-config-config-meta"><span>schema={configuration.schema}</span><span>source={configuration.source}</span><span>scenario={configuration.scenario}</span><span>证据 {configuration.evidenceRefs.length} 条</span></div>
            <div className="delivery-config-tier-tree">{configuration.groups.map(group => <div key={group.id}><TierObject type="group" object={group} onOverride={selectOverride}/>{group.plans.map(plan => <div className="delivery-config-tier-indent" key={plan.id}><TierObject type="plan" object={plan} onOverride={selectOverride}/>{plan.creatives.map(creative => <div className="delivery-config-tier-indent" key={creative.id}><TierObject type="creative" object={creative} location={{ groupId: group.id, planId: plan.id, creativeId: creative.id }} onOverride={selectOverride}/></div>)}</div>)}</div>)}</div>
          </> : <div className="delivery-config-empty-inline"><CircleAlert size={18}/>服务端尚未生成配置。请先完成策略与素材引用，再编译内部配置。</div>}
        </section> : null}

        {showPreflight ? <section className="delivery-config-flow-grid delivery-config-flow-grid--preflight">
          <article className="delivery-config-preflight-card">
            <header><div><span className="section-label">当前计划门禁</span><h3>检查草稿并提交变更申请</h3></div><strong className="delivery-config-preflight-state">{changeSet ? `变更申请 ${changeSet.status}` : '尚未提交变更申请'}</strong></header>
            <div className="delivery-config-preflight-summary"><b>校验结果</b><p>{preflightMessage}</p>{changeSet ? <small>申请版本 {changeSet.version}{changeSet.status === 'draft' ? ' · 当前仅为草稿，提交时服务端会重新校验同一内容快照' : ''}</small> : <small>先检查当前草稿，再提交需要审批的变更申请。</small>}</div>
            <div className="delivery-config-actions delivery-config-preflight-actions">
              <button onClick={() => void preflightPlan()} disabled={busy}><ShieldCheck size={14}/>检查当前草稿</button>
              <button onClick={() => void createAndPreflightChangeSet()} disabled={busy || !canSubmitChangeSet}><Check size={14}/>{changeSet?.recommendationId ? '提交优化变更申请' : '提交变更申请'}</button>
              {changeSet && changeSet.status !== 'draft' ? <div className={`delivery-config-approval-handoff ${changeSet.status}`}>
                <span><b>{changeSet.status === 'preflight_passed' ? '等待审批' : changeSet.status === 'approved' ? '审批中心已批准' : changeSet.status === 'rejected' ? '审批中心已打回' : '变更申请状态已更新'}</b><small>{changeSet.status === 'preflight_passed' ? '当前投手只可查看审批进度，批准或打回由具备权限的审批角色完成。' : changeSet.status === 'rejected' ? '根据打回原因修改计划后，可重新检查并提交。' : '审批记录与当前内容快照已持久化。'}</small></span>
                {approvalURL ? <a className="primary-button" href={approvalURL}>{changeSet.status === 'preflight_passed' ? '查看审批进度' : '查看审批记录'}<ArrowRight size={14}/></a> : null}
              </div> : null}
            </div>
            {changeSet?.status === 'rejected' ? <div className="inline-notice danger"><CircleAlert size={16}/>已打回：{changeSet.rejectionReason}</div> : null}
          </article>
        </section> : null}

        {showManualPackage ? <section className="delivery-config-package-page">
          <article><header><div><span className="section-label">获批后的交付物</span><h3>人工操作包</h3></div></header><p>仅在优化变更申请审批后可编译。它是供授权人员逐项核对的操作步骤，不是平台执行指令。</p><button className="primary-button" onClick={() => void compileManualPackage()} disabled={busy || changeSet?.status !== 'approved'}><FilePlus size={14}/>编译人工操作包</button>{manualPackage ? <ManualPackageDetails value={manualPackage}/> : <div className="panel-empty">等待优化变更申请在审批中心获批。</div>}</article>
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
