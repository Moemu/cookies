import { useEffect, useMemo, useRef, useState } from 'react'
import { ArrowRight, Check, CircleAlert, Plus, RefreshCw, Save, ShieldCheck, Trash2 } from 'lucide-react'
import {
  DeliveryApiError,
  deliveryPlanApi,
  type DeliveryControlChangeSet,
  type DeliveryPlan,
  type PlatformConfiguration,
} from '../api/delivery'
import { useProject } from '../context/ProjectContext'
import { oceanEngineCalibrationDispositions, visibleOceanEngineManifestFields, type CalibrationDisposition, type VisibleManifestField } from '../lib/oceanengineCalibrationManifest'
import { projectPath } from '../lib/router'
import type { DataState } from '../types'
import { StateBoundary } from './StateBoundary'

function formatTime(value?: string) {
  return value ? new Date(value).toLocaleString('zh-CN', { dateStyle: 'medium', timeStyle: 'short' }) : '暂无记录'
}

function errorMessage(error: unknown, fallback: string) {
  if (error instanceof DeliveryApiError) {
    if (error.code === 'LEGACY_CONFIGURATION_UNSUPPORTED') return '这份历史配置仅供查看，不能继续提交或修改。请新建投放计划。'
    if (error.code === 'VERSION_CONFLICT') return '计划版本已更新，请刷新后重试。'
    return error.message
  }
  return error instanceof Error ? error.message : fallback
}

function formatManifestValue(value: unknown, unit?: string, valueLabels: Record<string, string> = {}, propertyLabels: Record<string, string> = {}): string {
  if (value === null || value === undefined) return ''
  if (typeof value === 'number' && unit === 'CNY_fen') return (value / 100).toLocaleString('zh-CN', { style: 'currency', currency: 'CNY' })
  if (typeof value === 'boolean') return value ? '开启' : '关闭'
  if (typeof value === 'string' || typeof value === 'number') return valueLabels[String(value)] ?? String(value)
  if (Array.isArray(value)) return value.map(item => formatManifestValue(item, unit, valueLabels, propertyLabels)).filter(Boolean).join('、')
  if (typeof value === 'object') {
    const record = value as Record<string, unknown>
    if (typeof record.display_name_snapshot === 'string' && record.display_name_snapshot) return record.display_name_snapshot
    if (typeof record.text === 'string') return record.text
    if (typeof record.id === 'string' && record.id) return '已选择平台对象'
    if (typeof record.start_at === 'string' && typeof record.end_at === 'string') {
      const start = new Date(record.start_at).toLocaleDateString('zh-CN')
      const end = new Date(record.end_at).toLocaleDateString('zh-CN')
      return `${start} — ${end}${typeof record.timezone === 'string' ? ` · ${record.timezone}` : ''}`
    }
    return Object.entries(record).flatMap(([key, entry]) => {
      if (typeof entry !== 'string' && typeof entry !== 'number' && typeof entry !== 'boolean') return []
      const label = propertyLabels[key] ?? key
      return [`${label}：${formatManifestValue(entry, undefined, valueLabels, propertyLabels)}`]
    }).join(' · ')
  }
  return ''
}

function ManifestFieldList({ fields }: { fields: VisibleManifestField[] }) {
  if (!fields.length) return null
  return <dl className="delivery-config-project-facts">{fields.map(field => <div key={field.key}><dt>{field.label}</dt><dd>{formatManifestValue(field.value, field.unit, field.valueLabels, field.propertyLabels)}</dd></div>)}</dl>
}

const dispositionLabels: Record<CalibrationDisposition['state'], string> = {
  ready: '可手动配置',
  evidence_only: '仅校准证据',
  blocked: '已阻断',
  platform_pending: '等待平台条件',
  condition_unmet: '当前条件未满足',
  missing_value: '缺少当前值',
}

function CalibrationDispositionList({ title, items }: { title: string; items: CalibrationDisposition[] }) {
  return <section className="delivery-config-disposition-group"><h4>{title}</h4><ol>{items.map(item => <li key={item.key}>
    <div><b>{item.label}</b><code>{item.key}</code></div>
    <strong data-state={item.state}>{dispositionLabels[item.state]}</strong>
    <p>{item.reason}</p>
  </li>)}</ol></section>
}

function CalibrationDispositionView({ value }: { value: PlatformConfiguration }) {
  if (value.platform !== 'ocean_engine' || !value.payload.ocean_engine) return <div className="delivery-config-empty-inline"><CircleAlert size={18}/>当前平台没有可读取的字段校准记录。</div>
  const configuration = value.payload.ocean_engine
  return <section className="delivery-config-calibration-card">
    <header><div><span className="section-label">只读</span><h3>字段校准与处置</h3><p>状态和原因直接来自冻结 Manifest。此视图不填写、不保存、不提交平台表单。</p></div></header>
    <CalibrationDispositionList title="项目字段" items={oceanEngineCalibrationDispositions(configuration, 'project')}/>
    <CalibrationDispositionList title="推广单元字段" items={oceanEngineCalibrationDispositions(configuration, 'promotion')}/>
  </section>
}

function PlatformConfigurationDetails({ value }: { value: PlatformConfiguration }) {
  if (value.platform === 'magnetic_engine') {
    return <div className="delivery-config-empty-inline"><CircleAlert size={18}/><div><b>磁力引擎能力尚未开放</b><p>{value.payload.magnetic_engine?.reason}</p></div></div>
  }
  const ocean = value.payload.ocean_engine
  if (!ocean?.project) return <div className="delivery-config-empty-inline"><CircleAlert size={18}/>平台配置缺少主投放项目。</div>
  const project = ocean.project
  const projectFields = visibleOceanEngineManifestFields(ocean, 'project')
  return <div className="delivery-config-business-map">
    <section className="delivery-config-project-card">
      <header><div><span className="delivery-config-eyebrow">主投放项目</span><h4>{project.project_name}</h4><p>预算、排期和营销目标在此项目下统一生效。</p></div><strong className="delivery-config-ready-state">配置已就绪</strong></header>
      <ManifestFieldList fields={projectFields}/>
    </section>
    <section className="delivery-config-promotion-section">
      <header><div><span className="delivery-config-eyebrow">推广单元</span><h4>素材与文案组合</h4><p>所有推广单元均归属于上方主投放项目。</p></div><strong>{ocean.promotions.length} 个</strong></header>
      {ocean.promotions.length ? <div className="delivery-config-promotion-grid">{ocean.promotions.map((promotion, index) => <article key={promotion.promotion_draft_id}>
        <header><span>推广单元 {index + 1}</span><h5>{promotion.promotion_name}</h5></header>
        <ManifestFieldList fields={visibleOceanEngineManifestFields(ocean, 'promotion', promotion)}/>
      </article>)}</div> : <div className="delivery-config-empty-inline">暂未添加推广单元，可稍后从投放计划补充素材与文案。</div>}
    </section>
  </div>
}

type OceanConfiguration = NonNullable<PlatformConfiguration['payload']['ocean_engine']>
type OceanPromotion = OceanConfiguration['promotions'][number]

function PlatformConfigurationEditor({ value, onChange }: { value: PlatformConfiguration; onChange: (value: PlatformConfiguration) => void }) {
  const ocean = value.payload.ocean_engine
  if (!ocean?.project) return null
  const updateOcean = (next: OceanConfiguration) => onChange({ ...value, payload: { ...value.payload, ocean_engine: next } })
  const updateProject = (patch: Partial<OceanConfiguration['project']>) => updateOcean({ ...ocean, project: { ...ocean.project, ...patch } })
  const updatePromotion = (index: number, patch: Partial<OceanPromotion>) => updateOcean({ ...ocean, promotions: ocean.promotions.map((promotion, itemIndex) => itemIndex === index ? { ...promotion, ...patch } : promotion) })
  const addPromotion = () => updateOcean({ ...ocean, promotions: [...ocean.promotions, {
    draft_schema_version: 'oceanengine-configuration/v1',
    promotion_draft_id: `promotion-local-${Date.now()}`,
    delivery_identity: { mode: 'account_info' }, base_material_references: [], copy_items: [], settings: {},
    promotion_name: `${ocean.project.project_name}-${ocean.promotions.length + 1}`,
  }] })
  const removePromotion = (index: number) => {
    const promotionName = ocean.promotions[index]?.promotion_name || `推广单元 ${index + 1}`
    if (!window.confirm(`删除“${promotionName}”？此操作只修改当前本地草稿。`)) return
    updateOcean({ ...ocean, promotions: ocean.promotions.filter((_, itemIndex) => itemIndex !== index) })
  }
  return <section className="delivery-config-editor" aria-labelledby="platform-config-editor-title">
    <header className="delivery-config-editor-intro"><div><span className="section-label">本地配置</span><h3 id="platform-config-editor-title">编辑投放项目和推广单元</h3><p>保存后生成 cookies 计划版本。Playwright RPA 在执行阶段读取该版本。</p></div><span className="delivery-config-local-badge">不会写入巨量</span></header>
    <div className="delivery-config-project-editor">
      <div className="delivery-config-subheading"><div><span>01</span><div><h4>投放项目</h4><p>设置项目名称、预算和投放周期。</p></div></div></div>
      <div className="delivery-config-editor-fields">
      <label><span>项目名称</span><input name="oceanengine_project_name" autoComplete="off" value={ocean.project.project_name} onChange={event => updateProject({ project_name: event.target.value })}/></label>
      <label><span>项目日预算</span><div className="delivery-config-money-input"><input name="oceanengine_project_daily_budget" autoComplete="off" type="number" inputMode="decimal" min="0" value={ocean.project.budget_and_bidding.daily_budget_minor / 100} onChange={event => updateProject({ budget_and_bidding: { ...ocean.project.budget_and_bidding, daily_budget_minor: Math.round(Number(event.target.value) * 100) } })}/><small>元 / 天</small></div></label>
      <label><span>投放周期</span><select name="oceanengine_project_schedule_mode" autoComplete="off" value={ocean.project.schedule.mode ?? 'fixed_range'} onChange={event => updateProject({ schedule: { ...ocean.project.schedule, mode: event.target.value as 'long_term' | 'fixed_range' } })}><option value="long_term">从今天起长期投放</option><option value="fixed_range">设置开始和结束日期</option></select></label>
      </div>
    </div>
    <div className="delivery-config-unit-editor">
      <div className="delivery-config-subheading"><div><span>02</span><div><h4>推广单元</h4><p>每个单元可使用独立名称、预算、出价和素材组合。</p></div></div><button className="secondary-button" type="button" onClick={addPromotion}><Plus size={15} aria-hidden="true"/>增加推广单元</button></div>
      <div className="delivery-config-unit-list">{ocean.promotions.map((promotion, index) => <article key={promotion.promotion_draft_id} className="delivery-config-unit-card">
        <header><div><span>推广单元 {String(index + 1).padStart(2, '0')}</span><strong>{promotion.promotion_name || '未命名单元'}</strong></div><button className="delivery-config-delete-unit" type="button" aria-label={`删除推广单元 ${index + 1}`} onClick={() => removePromotion(index)}><Trash2 size={15} aria-hidden="true"/></button></header>
        <div className="delivery-config-unit-fields">
          <label><span>单元名称</span><input name={`promotion_${index}_name`} autoComplete="off" value={promotion.promotion_name} onChange={event => updatePromotion(index, { promotion_name: event.target.value })}/></label>
          <label><span>单元日预算</span><div className="delivery-config-money-input"><input name={`promotion_${index}_daily_budget`} autoComplete="off" type="number" inputMode="decimal" min="0" value={(promotion.budget_and_bidding?.daily_budget_minor ?? 0) / 100} onChange={event => updatePromotion(index, { budget_and_bidding: { currency: 'CNY', bidding_strategy: promotion.budget_and_bidding?.bidding_strategy ?? 'stable_cost', charging_mode: promotion.budget_and_bidding?.charging_mode ?? 'CPC', ...promotion.budget_and_bidding, daily_budget_minor: Math.round(Number(event.target.value) * 100) } })}/><small>元 / 天</small></div></label>
          <label><span>单元出价</span><div className="delivery-config-money-input"><input name={`promotion_${index}_bid`} autoComplete="off" type="number" inputMode="decimal" min="0" step="0.01" value={(promotion.budget_and_bidding?.bid_minor ?? 0) / 100} onChange={event => updatePromotion(index, { budget_and_bidding: { currency: 'CNY', daily_budget_minor: promotion.budget_and_bidding?.daily_budget_minor ?? 0, bidding_strategy: promotion.budget_and_bidding?.bidding_strategy ?? 'stable_cost', charging_mode: promotion.budget_and_bidding?.charging_mode ?? 'CPC', ...promotion.budget_and_bidding, bid_minor: Math.round(Number(event.target.value) * 100) } })}/><small>元</small></div></label>
        </div>
        <footer><span>{promotion.base_material_references.length} 个素材</span><small>{promotion.base_material_references.length ? '素材已关联' : '尚未关联素材'}</small></footer>
      </article>)}</div>
      {!ocean.promotions.length ? <div className="delivery-config-empty-units"><b>还没有推广单元</b><p>增加一个推广单元，然后设置预算、出价和素材。</p><button className="secondary-button" type="button" onClick={addPromotion}><Plus size={15} aria-hidden="true"/>增加推广单元</button></div> : null}
    </div>
  </section>
}

export function DeliveryConfigurationPage({ state, activeView, tourRunId, tourCase }: { state: DataState; activeView: string; tourRunId?: string; tourCase?: string }) {
  const { currentProject } = useProject()
  const projectId = currentProject.id
  const [plans, setPlans] = useState<DeliveryPlan[]>([])
  const [selectedId, setSelectedId] = useState(() => new URLSearchParams(window.location.search).get('plan_id') ?? '')
  const [changeSet, setChangeSet] = useState<DeliveryControlChangeSet>()
  const [preflightMessage, setPreflightMessage] = useState('尚未检查当前计划。')
  const [notice, setNotice] = useState('')
  const [busy, setBusy] = useState(false)
  const [editableConfiguration, setEditableConfiguration] = useState<PlatformConfiguration>()
  const refreshGenerationRef = useRef(0)

  const selectedPlan = useMemo(() => plans.find(plan => plan.id === selectedId), [plans, selectedId])
  const platformConfiguration = selectedPlan?.currentVersion.platformConfiguration
  const legacyReadOnly = Boolean(selectedPlan?.currentVersion.readOnly || (selectedPlan && !platformConfiguration))
  const showConfiguration = activeView === '配置映射'
  const showCalibration = activeView === '字段校准与处置'
  const showPreflight = activeView === '检查与提交'
  const approvalURL = changeSet ? projectPath(projectId, 'delivery', 'approvals', changeSet.id, '待我审批', undefined, tourRunId, tourCase) : undefined
  const planEditorURL = projectPath(projectId, 'delivery', 'plans', undefined, '计划列表', undefined, tourRunId, tourCase)
  const canSubmit = !legacyReadOnly && (!changeSet || changeSet.status === 'draft' || changeSet.status === 'preflight_failed' || changeSet.status === 'rejected')

  const restoreWorkflow = async (planId: string) => {
    if (!planId) return
    try {
      const changeSets = await deliveryPlanApi.listChangeSets(projectId)
      setChangeSet(changeSets.filter(item => item.planId === planId).sort((left, right) => right.updatedAt.localeCompare(left.updatedAt))[0])
    } catch (error) {
      setNotice(errorMessage(error, '恢复计划的变更申请失败。'))
    }
  }

  const refresh = async () => {
    const generation = ++refreshGenerationRef.current
    setBusy(true)
    try {
      const nextPlans = await deliveryPlanApi.list(projectId)
      if (generation !== refreshGenerationRef.current) return
      setPlans(nextPlans)
      setSelectedId(current => nextPlans.some(plan => plan.id === current) ? current : nextPlans[0]?.id ?? '')
      setNotice(nextPlans.length ? '已刷新当前 Project 的平台配置。' : '当前 Project 暂无投放计划。')
    } catch (error) {
      if (generation === refreshGenerationRef.current) setNotice(errorMessage(error, '读取平台配置失败。'))
    } finally {
      if (generation === refreshGenerationRef.current) setBusy(false)
    }
  }

  useEffect(() => { void refresh() }, [projectId])
  useEffect(() => { if (selectedId) void restoreWorkflow(selectedId) }, [projectId, selectedId])
  useEffect(() => { setEditableConfiguration(platformConfiguration ? structuredClone(platformConfiguration) : undefined) }, [selectedId, selectedPlan?.currentVersionNumber])
  useEffect(() => {
    if (!selectedId) return
    const url = new URL(window.location.href)
    url.searchParams.set('plan_id', selectedId)
    window.history.replaceState(window.history.state, '', url)
  }, [selectedId])

  const preflightPlan = async () => {
    if (!selectedPlan || legacyReadOnly) return
    setBusy(true)
    try {
      const result = await deliveryPlanApi.preflight(projectId, selectedPlan.id)
      setPreflightMessage(result.passed ? `检查通过（${formatTime(result.checkedAt)}）。` : `检查未通过：${result.checks.filter(check => !check.passed).map(check => check.message).join('；')}`)
    } catch (error) { setNotice(errorMessage(error, '检查计划失败。')) } finally { setBusy(false) }
  }

  const createAndPreflightChangeSet = async () => {
    if (!selectedPlan || legacyReadOnly) return
    setBusy(true)
    try {
      const draft = changeSet?.status === 'draft' ? changeSet : await deliveryPlanApi.createChangeSet(projectId, selectedPlan.id, selectedPlan.currentVersionNumber)
      const checked = await deliveryPlanApi.preflightChangeSet(projectId, draft.id, draft.version)
      setChangeSet(checked)
      setNotice(checked.status === 'preflight_passed' ? '变更申请已提交审批中心。' : '变更申请未通过最终检查。')
    } catch (error) { setNotice(errorMessage(error, '提交变更申请失败。')) } finally { setBusy(false) }
  }

  const saveConfiguration = async () => {
    if (!selectedPlan || !editableConfiguration) return
    setBusy(true)
    try {
      const updated = await deliveryPlanApi.updatePlatformConfiguration(projectId, selectedPlan, editableConfiguration)
      setPlans(current => current.map(plan => plan.id === updated.id ? updated : plan))
      setNotice(`平台配置已保存为 V${updated.currentVersionNumber}。未执行巨量远端操作。`)
    } catch (error) { setNotice(errorMessage(error, '保存平台配置失败。')) } finally { setBusy(false) }
  }

  return <StateBoundary state={state} contextLabel="智能投放 / 平台配置" errorDetail="当前 Project 的平台配置无法读取。">
    <div className="delivery-config-workspace">
      <section className="delivery-config-toolbar"><label><span>投放计划</span><select name="delivery_plan" autoComplete="off" value={selectedId} onChange={event => setSelectedId(event.target.value)}>{plans.map(plan => <option value={plan.id} key={plan.id}>{plan.currentVersion.name} · V{plan.currentVersionNumber}</option>)}</select></label><a className="secondary-button" href={planEditorURL}>查看投放计划</a></section>

      {!selectedPlan ? <div className="panel-empty">当前 Project 暂无投放计划。<a href={planEditorURL}>前往创建</a></div> : legacyReadOnly ? <section className="delivery-config-config-card">
        <div className="delivery-config-empty-inline"><CircleAlert size={20}/><div><b>历史配置，仅供查看</b><p>这份计划不能继续修改、检查或提交。若要继续投放，请新建计划并选择目标广告平台。</p></div></div>
      </section> : <>
        {showConfiguration && platformConfiguration && editableConfiguration ? <section className="delivery-config-config-card"><header><div><span>当前计划 · V{selectedPlan.currentVersionNumber}</span><h3>{selectedPlan.currentVersion.name}</h3><p>更新于 {formatTime(selectedPlan.updatedAt)}</p></div><div className="delivery-config-contract"><span>配置草稿</span><button className="primary-button" type="button" onClick={() => void saveConfiguration()} disabled={busy}><Save size={15} aria-hidden="true"/>{busy ? '保存中…' : '保存新版本'}</button></div></header><PlatformConfigurationEditor value={editableConfiguration} onChange={setEditableConfiguration}/><details className="delivery-config-mapping-details"><summary>查看 Manifest 字段映射</summary><PlatformConfigurationDetails value={editableConfiguration}/></details></section> : null}
        {showCalibration && platformConfiguration ? <CalibrationDispositionView value={platformConfiguration}/> : null}
        {showPreflight ? <section className="delivery-config-flow-grid delivery-config-flow-grid--preflight"><article className="delivery-config-preflight-card">
          <header><div><span className="section-label">提交前检查</span><h3>检查并提交变更申请</h3></div><strong className="delivery-config-preflight-state">{changeSet ? `变更申请 ${changeSet.status}` : '尚未提交'}</strong></header>
          <div className="delivery-config-preflight-summary"><b>检查结果</b><p>{preflightMessage}</p></div>
          <div className="delivery-config-actions delivery-config-preflight-actions"><button onClick={() => void preflightPlan()} disabled={busy}><ShieldCheck size={14}/>检查当前计划</button><button onClick={() => void createAndPreflightChangeSet()} disabled={busy || !canSubmit}><Check size={14}/>提交变更申请</button>{approvalURL && changeSet?.status !== 'draft' ? <a className="primary-button" href={approvalURL}>查看审批记录<ArrowRight size={14}/></a> : null}</div>
        </article></section> : null}
      </>}
      {notice ? <div className="inline-notice" role="status">{notice}</div> : null}
    </div>
  </StateBoundary>
}
