import { useEffect, useMemo, useRef, useState } from 'react'
import { Check, CircleAlert, Plus, RefreshCw, Save, Trash2 } from 'lucide-react'
import {
  DeliveryApiError,
  deliveryPlanApi,
  deliveryExecutionApi,
  type DeliveryControlChangeSet,
  type DeliveryPlan,
  type PlatformConfiguration,
  type StableReference,
} from '../api/delivery'
import { useProject } from '../context/ProjectContext'
import { api, type ApiAssetVersionPointer, type ApiConnectorPlatformObject } from '../data/api'
import { oceanEngineCalibrationDispositions, visibleOceanEngineManifestFields, type CalibrationDisposition, type VisibleManifestField } from '../lib/oceanengineCalibrationManifest'
import { fromShanghaiEndDate, fromShanghaiStartDate, toShanghaiDateInput } from '../lib/deliverySchedule'
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

const optimizationTargetOptions = [
  { value: 'in_app_order', label: 'App 内下单' },
] as const

function updateReference(current: StableReference | undefined, id: string, objectKind: string): StableReference | undefined {
  const value = id.trim()
  if (!value) return undefined
  return { namespace: 'cookies', object_kind: objectKind, scope: current?.scope ?? 'current_project', state: 'resolved', ...current, id: value }
}

function MaterialObjectPicker({ label, assets, platformObjects = [], value, objectKind, onChange }: { label: string; assets: ApiAssetVersionPointer[]; platformObjects?: ApiConnectorPlatformObject[]; value: StableReference[]; objectKind: string; onChange: (value: StableReference[]) => void }) {
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState('')
  const [preview, setPreview] = useState<ApiAssetVersionPointer>()
  const selectionID = (reference: StableReference) => reference.audit_attributes?.connector_platform_object_id ? `connector:${reference.audit_attributes.connector_platform_object_id}` : reference.id ?? ''
  const [draftIds, setDraftIds] = useState<string[]>(value.map(selectionID))
  const draftSelected = new Set(draftIds)
  const filteredAssets = assets.filter(asset => `${asset.assetId} ${asset.oceanEngineMaterialId ?? ''}`.toLowerCase().includes(query.trim().toLowerCase()))
  const filteredPlatformObjects = platformObjects.filter(item => `${item.display_name} ${item.platform_object_id}`.toLowerCase().includes(query.trim().toLowerCase()))
  const toggle = (asset: ApiAssetVersionPointer) => setDraftIds(current => current.includes(asset.assetId) ? current.filter(id => id !== asset.assetId) : [...current, asset.assetId])
  const togglePlatformObject = (item: ApiConnectorPlatformObject) => {
    const id = `connector:${item.id}`
    setDraftIds(current => current.includes(id) ? current.filter(value => value !== id) : [...current, id])
  }
  const confirm = () => {
    onChange(draftIds.map((id): StableReference => {
      const platformObject = platformObjects.find(item => `connector:${item.id}` === id)
      if (platformObject) return { namespace: 'oceanengine', object_kind: platformObject.object_kind, scope: `account:${platformObject.account_id}`, id: platformObject.platform_object_id, version: String(platformObject.version), state: 'resolved' as const, display_name_snapshot: platformObject.display_name || platformObject.platform_object_id, audit_attributes: { connector_platform_object_id: platformObject.id, ocean_engine_material_id: platformObject.platform_object_id } } satisfies StableReference
      const asset = assets.find(item => item.assetId === id)
      const existing = value.find(reference => selectionID(reference) === id)
      if (!asset && existing) return existing
      return { namespace: 'cookies', object_kind: objectKind, scope: 'current_project', id, version: String(asset?.humanConfirmedVersion ?? asset?.workingVersion ?? 0), state: 'resolved' as const, display_name_snapshot: id, audit_attributes: { ocean_engine_material_id: asset?.oceanEngineMaterialId ?? '' } } satisfies StableReference
    }))
    setOpen(false)
  }
  return <fieldset className="delivery-config-object-picker"><legend>{label}</legend><div className="delivery-config-object-summary"><span>{value.length ? `${value.length} 个素材已选` : '尚未选择素材'}</span><button className="secondary-button" type="button" onClick={() => { setDraftIds(value.map(selectionID)); setOpen(true) }}>选择素材</button></div>{value.length ? <div className="delivery-config-selected-chips">{value.map(reference => <span key={`${reference.namespace}-${reference.id}`}>{reference.display_name_snapshot ?? reference.id}</span>)}</div> : null}{!assets.length && !platformObjects.length ? <p>当前 Project 没有可选择素材。</p> : null}
    {open ? <div className="delivery-material-modal-backdrop" role="presentation" onClick={() => setOpen(false)}><section className="delivery-material-modal" style={{ position: 'fixed', left: '50%', top: '50%', transform: 'translate(-50%, -50%)' }} role="dialog" aria-modal="true" aria-label={`${label}选择器`} onClick={event => event.stopPropagation()}><header><div><span className="section-label">MATERIAL PICKER</span><h3>{label}</h3><p>选择 Cookies 素材，或 Connector 已导入素材。</p></div><button className="text-button" type="button" onClick={() => setOpen(false)}>关闭</button></header><div className="delivery-material-modal-toolbar"><input autoFocus placeholder="搜索素材名称或巨量素材 ID" value={query} onChange={event => setQuery(event.target.value)}/><span>已选 {draftIds.length} 个</span><button className="text-button" type="button" onClick={() => setDraftIds([])}>清空</button></div><div className="delivery-material-modal-grid">{filteredPlatformObjects.map(item => { const selection = `connector:${item.id}`; return <label key={item.id} className={draftSelected.has(selection) ? 'selected' : ''}><input type="checkbox" checked={draftSelected.has(selection)} onChange={() => togglePlatformObject(item)}/><span className="delivery-material-platform-object">Connector</span><b>{item.display_name || item.platform_object_id}</b><small>{item.object_kind === 'video_material' ? '已有巨量视频' : '已有巨量图片'}</small></label> })}{filteredAssets.map(asset => <label key={asset.id} className={draftSelected.has(asset.assetId) ? 'selected' : ''}><input type="checkbox" checked={draftSelected.has(asset.assetId)} onChange={() => toggle(asset)}/><button type="button" className="delivery-material-preview-button" onClick={() => setPreview(asset)}>{asset.contentUrl ? asset.mediaKind === 'video' ? <video src={asset.contentUrl} preload="metadata"/> : <img src={asset.contentUrl} alt=""/> : <span>无预览</span>}</button><b>{asset.assetId}</b><small>{asset.oceanEngineMaterialId ? '已录入巨量' : '待 RPA 录入'}</small></label>)}{!filteredAssets.length && !filteredPlatformObjects.length ? <div className="delivery-material-empty">没有匹配的素材。</div> : null}</div><footer><button className="secondary-button" type="button" onClick={() => setOpen(false)}>取消</button><button className="primary-button" type="button" onClick={confirm}>确认选择</button></footer>{preview ? <div className="delivery-material-preview-overlay" role="dialog" aria-label="素材预览" onClick={() => setPreview(undefined)}>{preview.contentUrl ? preview.mediaKind === 'video' ? <video src={preview.contentUrl} controls autoPlay onClick={event => event.stopPropagation()}/> : <img src={preview.contentUrl} alt={preview.assetId} onClick={event => event.stopPropagation()}/> : <span>无预览</span>}</div> : null}</section></div> : null}
  </fieldset>
}

function ToggleField({ label, checked, onChange }: { label: string; checked: boolean; onChange: (checked: boolean) => void }) {
  return <label className="delivery-config-toggle-field"><span>{label}</span><span className="delivery-config-toggle-control"><span>{checked ? '已开启' : '未开启'}</span><input type="checkbox" role="switch" checked={checked} onChange={event => onChange(event.target.checked)}/></span></label>
}

function PlatformConfigurationEditor({ value, onChange, products, assets, platformObjects }: { value: PlatformConfiguration; onChange: (value: PlatformConfiguration) => void; products: Array<{ id: string; name: string; oceanEngineProductId?: string }>; assets: ApiAssetVersionPointer[]; platformObjects: ApiConnectorPlatformObject[] }) {
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
      <div className="delivery-config-subheading"><div><span>01</span><div><h4>投放项目</h4><p>设置营销路径、预算、竞价、排期和定向。</p></div></div></div>
      <div className="delivery-config-editor-fields delivery-config-editor-fields--wide">
        <label><span>项目名称</span><input name="oceanengine_project_name" autoComplete="off" value={ocean.project.project_name} onChange={event => updateProject({ project_name: event.target.value })}/></label>
        <label><span>营销目的</span><select value={ocean.project.marketing_purpose} onChange={event => updateProject({ marketing_purpose: event.target.value })}><option value="ecommerce">电商</option><option value="lead_generation">销售线索</option><option value="application">应用</option><option value="product_catalog">商品</option><option value="content_marketing">内容营销</option></select></label>
        {ocean.project.marketing_purpose !== 'product_catalog' ? <label><span>营销场景</span><select value={ocean.project.marketing_scenario} onChange={event => updateProject({ marketing_scenario: event.target.value })}><option value="manual_delivery">短视频与图文</option><option value="live_stream" disabled>直播（平台暂不可用）</option></select></label> : null}
        <label><span>cookies 产品</span><select value={ocean.project.marketing_product_reference?.id ?? ''} onChange={event => { const product = products.find(item => item.id === event.target.value); updateProject({ marketing_product_reference: product ? { namespace: 'cookies', object_kind: 'product', scope: 'current_project', id: product.id, state: 'resolved', display_name_snapshot: product.name, audit_attributes: { ocean_engine_product_id: product.oceanEngineProductId ?? '' } } : undefined }) }}><option value="">请选择 cookies 产品</option>{products.map(product => <option key={product.id} value={product.id}>{product.name}{product.oceanEngineProductId ? ' · 已录入巨量' : ' · 待 RPA 录入'}</option>)}</select><small>Playwright 自动选择匹配商品。用户不设置选择方式。</small></label>
        {ocean.project.marketing_purpose === 'application' ? <>
          <label><span>应用引用</span><input value={ocean.project.application_reference?.id ?? ''} placeholder="应用链接或应用对象 ID" onChange={event => updateProject({ application_reference: updateReference(ocean.project.application_reference, event.target.value, 'application') })}/></label>
          <label><span>应用场景</span><input value={ocean.project.application_scenario ?? ''} onChange={event => updateProject({ application_scenario: event.target.value })}/></label>
          <label><span>操作系统</span><select value={ocean.project.operating_system ?? ''} onChange={event => updateProject({ operating_system: event.target.value })}><option value="">请选择</option><option value="android">安卓</option><option value="ios">iOS</option><option value="harmonyos">鸿蒙</option></select></label>
          <label><span>下载方式</span><select value={ocean.project.application_download_mode ?? ''} onChange={event => updateProject({ application_download_mode: event.target.value })}><option value="">请选择</option><option value="direct_download">直接下载</option><option value="reservation_download">预约下载</option></select></label>
          <label><span>调起方式</span><input value={ocean.project.application_launch_mode ?? ''} onChange={event => updateProject({ application_launch_mode: event.target.value })}/></label>
        </> : null}
        {ocean.project.marketing_purpose === 'lead_generation' ? <label><span>获取线索方式</span><select value={ocean.project.lead_capture_mode ?? 'smart_lead'} onChange={event => updateProject({ lead_capture_mode: event.target.value, carrier: event.target.value === 'smart_lead' && ocean.project.carrier === 'owned_landing_page' ? 'orange_landing_page' : ocean.project.carrier })}><option value="smart_lead">智能优选</option><option value="custom_lead">自定义</option></select></label> : null}
        <label><span>投放模式</span><select value={ocean.project.delivery_mode} onChange={event => updateProject({ delivery_mode: event.target.value })}><option value="manual">手动投放</option><option value="ubmax">UBMax</option></select></label>
        <label><span>深度优化方式</span><select value={ocean.project.deep_optimization_mode ?? 'disabled'} onChange={event => updateProject({ deep_optimization_mode: event.target.value })}><option value="disabled">不启用</option><option value="conversion_roi">成交 ROI</option><option value="net_conversion_order">净成交下单</option><option value="net_conversion_roi">净成交 ROI</option></select><small>平台会按当前场景限制可用选项。</small></label>
        {['lead_generation', 'ecommerce'].includes(ocean.project.marketing_purpose) ? <ToggleField label="AIGC 动态创意" checked={ocean.project.aigc_dynamic_creative ?? false} onChange={aigc_dynamic_creative => updateProject({ aigc_dynamic_creative })}/> : null}
        <label><span>竞价策略</span><select value={ocean.project.budget_and_bidding.bidding_strategy} onChange={event => updateProject({ budget_and_bidding: { ...ocean.project.budget_and_bidding, bidding_strategy: event.target.value } })}><option value="stable_cost">稳定成本 · 成本稳定在出价附近</option><option value="cost_cap">最优成本 · 均匀消耗预算，成本不超过出价</option><option value="maximum_conversion">最大转化 · 花完预算，拿到最大转化（价值）</option></select></label>
        <label><span>付费方式</span><select value={ocean.project.budget_and_bidding.charging_mode} onChange={event => updateProject({ budget_and_bidding: { ...ocean.project.budget_and_bidding, charging_mode: event.target.value } })}><option value="CPC">按点击付费</option><option value="CPM">按展示付费</option><option value="OCPM">按目标转化出价</option></select></label>
        <label><span>项目日预算</span><select value={ocean.project.budget_and_bidding.budget_mode ?? (ocean.project.budget_and_bidding.daily_budget_minor === 0 ? 'unlimited' : 'daily')} onChange={event => updateProject({ budget_and_bidding: { ...ocean.project.budget_and_bidding, budget_mode: event.target.value as 'daily' | 'unlimited', daily_budget_minor: event.target.value === 'unlimited' ? 0 : Math.max(ocean.project.budget_and_bidding.daily_budget_minor, 100) } })}><option value="daily">设置日预算</option><option value="unlimited">不限</option></select>{(ocean.project.budget_and_bidding.budget_mode ?? 'daily') !== 'unlimited' ? <div className="delivery-config-money-input"><input type="number" inputMode="decimal" min="0" value={ocean.project.budget_and_bidding.daily_budget_minor / 100} onChange={event => updateProject({ budget_and_bidding: { ...ocean.project.budget_and_bidding, budget_mode: 'daily', daily_budget_minor: Math.round(Number(event.target.value) * 100) } })}/><small>元 / 天</small></div> : <small>预算不设上限</small>}</label>
        <label><span>项目出价</span><div className="delivery-config-money-input"><input type="number" inputMode="decimal" min="0" step="0.01" value={(ocean.project.budget_and_bidding.bid_minor ?? 0) / 100} onChange={event => updateProject({ budget_and_bidding: { ...ocean.project.budget_and_bidding, bid_minor: Math.round(Number(event.target.value) * 100) } })}/><small>元</small></div></label>
        <label><span>投放周期</span><select value={ocean.project.schedule.mode ?? 'fixed_range'} onChange={event => updateProject({ schedule: { ...ocean.project.schedule, mode: event.target.value as 'long_term' | 'fixed_range' } })}><option value="long_term">从今天起长期投放</option><option value="fixed_range">设置开始和结束日期</option></select></label>
        <label><span>开始日期</span><input type="date" value={toShanghaiDateInput(ocean.project.schedule.start_at)} onChange={event => updateProject({ schedule: { ...ocean.project.schedule, start_at: fromShanghaiStartDate(event.target.value) } })}/></label>
        {ocean.project.schedule.mode !== 'long_term' ? <label><span>结束日期</span><input type="date" value={toShanghaiDateInput(ocean.project.schedule.end_at)} onChange={event => updateProject({ schedule: { ...ocean.project.schedule, end_at: fromShanghaiEndDate(event.target.value) } })}/></label> : null}
        {ocean.project.marketing_purpose === 'product_catalog' ? <fieldset className="delivery-config-inline-fieldset"><legend>投放版位</legend><label><span>投放位置</span><select value={ocean.project.placement_strategy ?? 'smart'} onChange={event => updateProject({ placement_strategy: event.target.value, placement_media: event.target.value === 'smart' ? undefined : ocean.project.placement_media })}><option value="smart">通投智选</option><option value="preferred_media">首选媒体</option></select></label>{ocean.project.placement_strategy === 'preferred_media' ? <div className="delivery-config-check-grid">{[{ key: 'all', label: '全选' }, { key: 'toutiao', label: '今日头条' }, { key: 'xigua', label: '西瓜视频' }, { key: 'douyin', label: '抖音' }, { key: 'fanqie', label: '番茄系媒体' }, { key: 'pangolin', label: '穿山甲' }].map(option => { const media = ['toutiao', 'xigua', 'douyin', 'fanqie', 'pangolin']; const checked = option.key === 'all' ? media.every(item => ocean.project.placement_media?.includes(item)) : ocean.project.placement_media?.includes(option.key) ?? false; return <label key={option.key}><input type="checkbox" checked={checked} onChange={event => updateProject({ placement_media: option.key === 'all' ? event.target.checked ? media : [] : event.target.checked ? [...new Set([...(ocean.project.placement_media ?? []), option.key])] : (ocean.project.placement_media ?? []).filter(item => item !== option.key) })}/><span>{option.label}</span></label> })}</div> : null}</fieldset> : null}
        <label><span>地域</span><select multiple value={ocean.project.targeting.regions ?? []} onChange={event => updateProject({ targeting: { ...ocean.project.targeting, regions: Array.from(event.currentTarget.selectedOptions, option => option.value) } })}>{['不限', '北京市', '上海市', '广东省', '浙江省', '江苏省', '四川省'].map(region => <option key={region} value={region === '不限' ? 'all' : region}>{region}</option>)}</select><small>按住 Ctrl 可多选。</small></label>
        <label><span>年龄</span><select multiple value={ocean.project.targeting.age_ranges ?? []} onChange={event => { const values = Array.from(event.currentTarget.selectedOptions, option => option.value); updateProject({ targeting: { ...ocean.project.targeting, age_ranges: values.includes('all') ? ['all'] : values } }) }}>{[['all', '不限'], ['18-23', '18-23'], ['24-30', '24-30'], ['31-40', '31-40'], ['41-49', '41-49'], ['50+', '50+']].map(([value, label]) => <option key={value} value={value}>{label}</option>)}</select><small>按住 Ctrl 可多选；选择“不限”会清除其他年龄。</small></label>
        <label><span>性别</span><select value={ocean.project.targeting.gender ?? ''} onChange={event => updateProject({ targeting: { ...ocean.project.targeting, gender: event.target.value } })}><option value="">不限</option><option value="male">男</option><option value="female">女</option></select></label>
        <ToggleField label="智能定向扩展" checked={ocean.project.targeting.smart_expansion} onChange={smart_expansion => updateProject({ targeting: { ...ocean.project.targeting, smart_expansion } })}/>
      </div>
    </div>
    <div className="delivery-config-project-editor">
      <div className="delivery-config-subheading"><div><span>02</span><div><h4>投放载体和监测</h4><p>设置落地页、优化目标、搜索快投和第三方监测。</p></div></div></div>
      <div className="delivery-config-editor-fields delivery-config-editor-fields--wide">
        <label><span>投放载体</span><select value={ocean.project.carrier} onChange={event => updateProject({ carrier: event.target.value })}><option value="orange_landing_page">橙子落地页</option>{ocean.project.marketing_purpose === 'lead_generation' && ocean.project.lead_capture_mode === 'smart_lead' ? <option value="orange_landing_page_and_im">橙子落地页 + 抖音私信页</option> : null}{ocean.project.marketing_purpose !== 'lead_generation' || ocean.project.lead_capture_mode === 'custom_lead' ? <><option value="owned_landing_page">自研落地页</option><option value="im">抖音私信页（原抖音主页）</option></> : null}<option value="byte_miniapp" disabled>字节小程序（暂不支持）</option><option value="wechat_miniapp" disabled>微信小程序（暂不支持）</option></select></label>
        <label><span>优化目标</span><select value={ocean.project.optimization_target_reference?.semantic_key ?? ocean.project.optimization_target_reference?.id ?? ''} onChange={event => { const option = optimizationTargetOptions.find(item => item.value === event.target.value); updateProject({ optimization_target_reference: option ? { namespace: 'cookies', object_kind: 'optimization_target', scope: 'current_project', id: option.value, semantic_key: option.value, state: 'resolved', display_name_snapshot: option.label } : undefined }) }}><option value="">请选择优化目标</option>{optimizationTargetOptions.map(option => <option key={option.value} value={option.value}>{option.label}</option>)}</select></label>

        <ToggleField label="商品定向 · RTA 跳转" checked={ocean.project.product_targeting?.rta_redirect ?? false} onChange={rta_redirect => updateProject({ product_targeting: { ...ocean.project.product_targeting, rta_redirect } })}/>
        <ToggleField label="商品定向 · 地域匹配" checked={ocean.project.product_targeting?.region_match ?? false} onChange={region_match => updateProject({ product_targeting: { ...ocean.project.product_targeting, region_match } })}/>
        <label><span>搜索关键词</span><textarea rows={2} value={ocean.project.search_boost?.keywords?.join(', ') ?? ''} placeholder="多个关键词用逗号分隔" onChange={event => updateProject({ search_boost: { ...ocean.project.search_boost, keywords: event.target.value.split(/[,，\n]/).map(item => item.trim()).filter(Boolean) } })}/></label>
        <label><span>搜索出价系数</span><input type="number" min="1" step="0.1" value={ocean.project.search_boost?.bid_coefficient ?? 1.1} onChange={event => updateProject({ search_boost: { ...ocean.project.search_boost, bid_coefficient: Number(event.target.value) } })}/></label>
        <ToggleField label="搜索定向扩展" checked={ocean.project.search_boost?.targeting_expansion ?? false} onChange={targeting_expansion => updateProject({ search_boost: { ...ocean.project.search_boost, targeting_expansion } })}/>
        {['impression', 'valid_touch', 'video_play', 'video_complete', 'valid_video_play'].map((kind, index) => { const labels = ['展示监测链接', '有效触点监测链接', '视频播放监测链接', '视频播完监测链接', '视频有效播放监测链接']; const reference = ocean.project.monitoring_references?.find(item => item.object_kind === `monitoring_link_${kind}`); return <label key={kind}><span>{labels[index]}</span><input type="url" value={reference?.id ?? ''} onChange={event => { const remaining = ocean.project.monitoring_references?.filter(item => item.object_kind !== `monitoring_link_${kind}`) ?? []; const next = updateReference(reference, event.target.value, `monitoring_link_${kind}`); updateProject({ monitoring_references: next ? [...remaining, next] : remaining }) }}/></label> })}
      </div>
    </div>
    <div className="delivery-config-unit-editor">
      <div className="delivery-config-subheading"><div><span>03</span><div><h4>推广单元</h4><p>每个单元使用独立身份、预算、出价、落地页和设置。</p></div></div><button className="secondary-button" type="button" onClick={addPromotion}><Plus size={15} aria-hidden="true"/>增加推广单元</button></div>
      <div className="delivery-config-unit-list">{ocean.promotions.map((promotion, index) => <article key={promotion.promotion_draft_id} className="delivery-config-unit-card">
        <header><div><span>推广单元 {String(index + 1).padStart(2, '0')}</span><strong>{promotion.promotion_name || '未命名单元'}</strong></div><button className="delivery-config-delete-unit" type="button" aria-label={`删除推广单元 ${index + 1}`} onClick={() => removePromotion(index)}><Trash2 size={15} aria-hidden="true"/></button></header>
        <div className="delivery-config-unit-fields delivery-config-unit-fields--wide">
          <label><span>单元名称</span><input name={`promotion_${index}_name`} autoComplete="off" value={promotion.promotion_name} onChange={event => updatePromotion(index, { promotion_name: event.target.value })}/></label>
          <label><span>投放身份</span><select value={promotion.delivery_identity.mode} onChange={event => updatePromotion(index, { delivery_identity: { ...promotion.delivery_identity, mode: event.target.value } })}><option value="account_info">账户信息</option><option value="authorized_identity">授权身份</option></select></label>
          {promotion.delivery_identity.mode === 'authorized_identity' ? <label><span>授权身份 ID</span><input value={promotion.delivery_identity.authorized_identity?.id ?? ''} onChange={event => updatePromotion(index, { delivery_identity: { ...promotion.delivery_identity, authorized_identity: updateReference(promotion.delivery_identity.authorized_identity, event.target.value, 'delivery_identity') } })}/></label> : null}
          <label><span>单元日预算</span><div className="delivery-config-money-input"><input name={`promotion_${index}_daily_budget`} autoComplete="off" type="number" inputMode="decimal" min="0" value={(promotion.budget_and_bidding?.daily_budget_minor ?? 0) / 100} onChange={event => updatePromotion(index, { budget_and_bidding: { currency: 'CNY', bidding_strategy: promotion.budget_and_bidding?.bidding_strategy ?? 'stable_cost', charging_mode: promotion.budget_and_bidding?.charging_mode ?? 'CPC', ...promotion.budget_and_bidding, daily_budget_minor: Math.round(Number(event.target.value) * 100) } })}/><small>元 / 天</small></div></label>
          <label><span>单元出价</span><div className="delivery-config-money-input"><input name={`promotion_${index}_bid`} autoComplete="off" type="number" inputMode="decimal" min="0" step="0.01" value={(promotion.budget_and_bidding?.bid_minor ?? 0) / 100} onChange={event => updatePromotion(index, { budget_and_bidding: { currency: 'CNY', daily_budget_minor: promotion.budget_and_bidding?.daily_budget_minor ?? 0, bidding_strategy: promotion.budget_and_bidding?.bidding_strategy ?? 'stable_cost', charging_mode: promotion.budget_and_bidding?.charging_mode ?? 'CPC', ...promotion.budget_and_bidding, bid_minor: Math.round(Number(event.target.value) * 100) } })}/><small>元</small></div></label>
          <label><span>橙子落地页</span><select value={promotion.landing_page_reference?.id ?? ''} onChange={event => { const item = platformObjects.find(value => value.object_kind === 'orange_landing_page' && value.platform_object_id === event.target.value); updatePromotion(index, { landing_page_reference: item ? { namespace: 'oceanengine', object_kind: 'orange_landing_page', scope: `account:${item.account_id}`, id: item.platform_object_id, version: String(item.version), state: 'resolved', display_name_snapshot: item.display_name || item.platform_object_id, audit_attributes: { connector_platform_object_id: item.id } } : undefined }) }}><option value="">请选择已导入落地页</option>{platformObjects.filter(item => item.object_kind === 'orange_landing_page').map(item => <option key={item.id} value={item.platform_object_id}>{item.display_name || item.platform_object_id}</option>)}</select></label>
          <label><span>直达链接引用</span><input value={promotion.direct_link_reference?.id ?? ''} onChange={event => updatePromotion(index, { direct_link_reference: updateReference(promotion.direct_link_reference, event.target.value, 'direct_link') })}/></label>
          <label><span>产品引用</span><input value={promotion.product_reference?.id ?? ''} onChange={event => updatePromotion(index, { product_reference: updateReference(promotion.product_reference, event.target.value, 'product') })}/></label>
          <label><span>原生锚点 ID</span><input value={promotion.native_anchor_reference?.id ?? ''} onChange={event => updatePromotion(index, { native_anchor_reference: updateReference(promotion.native_anchor_reference, event.target.value, 'native_anchor') })}/></label>
          <label><span>所属类别 ID</span><input value={promotion.settings.category_reference?.id ?? ''} onChange={event => updatePromotion(index, { settings: { ...promotion.settings, category_reference: updateReference(promotion.settings.category_reference, event.target.value, 'category') } })}/></label>
          <label><span>品牌 ID</span><input value={promotion.settings.brand_reference?.id ?? ''} onChange={event => updatePromotion(index, { settings: { ...promotion.settings, brand_reference: updateReference(promotion.settings.brand_reference, event.target.value, 'brand') } })}/></label>
          <label><span>行动号召（每行一个，最多 10 个）</span><textarea rows={3} value={promotion.settings.call_to_action?.join('\n') ?? ''} onChange={event => updatePromotion(index, { settings: { ...promotion.settings, call_to_action: event.target.value.split('\n').map(value => value.trim()).filter(Boolean).filter((value, itemIndex, values) => values.indexOf(value) === itemIndex).slice(0, 10) } })}/></label>
          <label><span>来源说明</span><input value={promotion.settings.source_label ?? ''} onChange={event => updatePromotion(index, { settings: { ...promotion.settings, source_label: event.target.value } })}/></label>
          <label><span>直达链接方式</span><select value={promotion.settings.direct_link_mode ?? 'automatic'} onChange={event => updatePromotion(index, { settings: { ...promotion.settings, direct_link_mode: event.target.value as 'automatic' | 'manual' } })}><option value="automatic">自动</option><option value="manual">手动</option></select></label>
          <ToggleField label="开启评论" checked={promotion.settings.comments_enabled ?? false} onChange={comments_enabled => updatePromotion(index, { settings: { ...promotion.settings, comments_enabled } })}/>
          <ToggleField label="智能生成" checked={promotion.settings.smart_generation_enabled ?? false} onChange={smart_generation_enabled => updatePromotion(index, { settings: { ...promotion.settings, smart_generation_enabled } })}/>
          <ToggleField label="允许客户端下载" checked={promotion.settings.client_download_enabled ?? false} onChange={client_download_enabled => updatePromotion(index, { settings: { ...promotion.settings, client_download_enabled } })}/>
        </div>
        <section className="delivery-config-material-editor"><h5>04 素材与文案</h5><div className="delivery-config-unit-fields delivery-config-unit-fields--wide">
          <MaterialObjectPicker label="基础素材" assets={assets} platformObjects={platformObjects.filter(item => item.object_kind === 'image_material' || item.object_kind === 'video_material')} value={promotion.base_material_references} objectKind="material" onChange={base_material_references => updatePromotion(index, { base_material_references })}/>
          <MaterialObjectPicker label="产品主图" assets={assets.filter(asset => asset.mediaKind === 'image')} value={promotion.product_image_references ?? []} objectKind="product_image" onChange={product_image_references => updatePromotion(index, { product_image_references })}/>
          <label><span>广告文案</span><textarea rows={2} value={promotion.copy_items.map(item => item.text).join('\n')} placeholder="每行一条文案" onChange={event => updatePromotion(index, { copy_items: event.target.value.split('\n').map(text => text.trim()).filter(Boolean).map(text => ({ text })) })}/></label>
          <label><span>产品卖点</span><textarea rows={2} value={promotion.product_selling_points?.join('\n') ?? ''} placeholder="每行一个卖点" onChange={event => updatePromotion(index, { product_selling_points: event.target.value.split('\n').map(text => text.trim()).filter(Boolean) })}/></label>
        </div></section>
        <footer><span>{promotion.base_material_references.length} 个素材</span><small>{promotion.base_material_references.length ? '素材已关联' : '尚未关联素材'}</small></footer>
      </article>)}</div>
      {!ocean.promotions.length ? <div className="delivery-config-empty-units"><b>还没有推广单元</b><p>增加一个推广单元，然后设置预算、出价和素材。</p><button className="secondary-button" type="button" onClick={addPromotion}><Plus size={15} aria-hidden="true"/>增加推广单元</button></div> : null}
    </div>
  </section>
}

export function DeliveryConfigurationPage({ state, activeView, tourRunId, tourCase }: { state: DataState; activeView: string; tourRunId?: string; tourCase?: string }) {
  const { currentProject, agencyWorkbench } = useProject()
  const projectId = currentProject.id
  const confirmedAssets = useMemo(() => (agencyWorkbench?.assetVersionPointers ?? []).filter(asset => asset.projectId === projectId && asset.humanConfirmedVersion), [agencyWorkbench, projectId])
  const [plans, setPlans] = useState<DeliveryPlan[]>([])
  const [selectedId, setSelectedId] = useState(() => new URLSearchParams(window.location.search).get('plan_id') ?? '')
  const [changeSet, setChangeSet] = useState<DeliveryControlChangeSet>()
  const [notice, setNotice] = useState('')
  const [busy, setBusy] = useState(false)
  const [editableConfiguration, setEditableConfiguration] = useState<PlatformConfiguration>()
  const [platformObjects, setPlatformObjects] = useState<ApiConnectorPlatformObject[]>([])
  const refreshGenerationRef = useRef(0)

  const selectedPlan = useMemo(() => plans.find(plan => plan.id === selectedId), [plans, selectedId])
  const platformConfiguration = selectedPlan?.currentVersion.platformConfiguration
  const legacyReadOnly = Boolean(selectedPlan?.currentVersion.readOnly || (selectedPlan && !platformConfiguration))
  const showConfiguration = activeView === '配置映射'
  const showCalibration = activeView === '字段校准与处置'
  const showPreflight = activeView === '检查与提交'
  const planEditorURL = projectPath(projectId, 'delivery', 'plans', undefined, '计划列表', undefined, tourRunId, tourCase)

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
    const accountID = editableConfiguration?.payload.ocean_engine?.project?.account_reference?.id
    if (!accountID) {
      setPlatformObjects([])
      return
    }
    let active = true
    void Promise.all((['image_material', 'video_material', 'orange_landing_page'] as const).map(objectKind => api.listProjectConnectorPlatformObjects(projectId, accountID, { objectKind, status: 'active', limit: 100 })))
      .then(values => { if (active) setPlatformObjects(values.flatMap(value => value.items)) })
      .catch(() => { if (active) setPlatformObjects([]) })
    return () => { active = false }
  }, [editableConfiguration?.payload.ocean_engine?.project?.account_reference?.id, projectId])
  useEffect(() => {
    if (!selectedId) return
    const url = new URL(window.location.href)
    url.searchParams.set('plan_id', selectedId)
    window.history.replaceState(window.history.state, '', url)
  }, [selectedId])

  const saveConfiguration = async () => {
    if (!selectedPlan || !editableConfiguration) return
    setBusy(true)
    try {
      const updated = await deliveryPlanApi.updatePlatformConfiguration(projectId, selectedPlan, editableConfiguration)
      setPlans(current => current.map(plan => plan.id === updated.id ? updated : plan))
      setNotice(`平台配置已保存为 V${updated.currentVersionNumber}。未执行巨量远端操作。`)
    } catch (error) { setNotice(errorMessage(error, '保存平台配置失败。')) } finally { setBusy(false) }
  }

  const confirmLaunch = async () => {
    if (!selectedPlan) return
    setBusy(true)
    try {
      const idempotencyKey = `launch-${selectedPlan.id}-${selectedPlan.currentVersionNumber}-${Date.now()}`
      const result = await deliveryExecutionApi.executePlan(projectId, selectedPlan.id, selectedPlan.currentVersionNumber, idempotencyKey)
      setNotice(
        result.execution.status === 'succeeded'
          ? '已确认投放并完成平台写入（本地模拟）。'
          : `投放执行结果为 ${result.execution.status}：${result.execution.recoveryReason || '详见执行记录'}。`,
      )
      setChangeSet(result.changeSet)
    } catch (error) {
      if (error instanceof DeliveryApiError && error.code === 'VALIDATION_FAILED' && error.violations.length) {
        setNotice(`当前配置无法投放：${error.violations.map(item => item.reason).join('；')}`)
      } else {
        setNotice(errorMessage(error, '确认投放失败。'))
      }
    } finally { setBusy(false) }
  }

  return <StateBoundary state={state} contextLabel="智能投放 / 平台配置" errorDetail="当前 Project 的平台配置无法读取。">
    <div className="delivery-config-workspace">
      <section className="delivery-config-toolbar"><label><span>投放计划</span><select name="delivery_plan" autoComplete="off" value={selectedId} onChange={event => setSelectedId(event.target.value)}>{plans.map(plan => <option value={plan.id} key={plan.id}>{plan.currentVersion.name} · V{plan.currentVersionNumber}</option>)}</select></label><a className="secondary-button" href={planEditorURL}>查看投放计划</a></section>

      {!selectedPlan ? <div className="panel-empty">当前 Project 暂无投放计划。<a href={planEditorURL}>前往创建</a></div> : legacyReadOnly ? <section className="delivery-config-config-card">
        <div className="delivery-config-empty-inline"><CircleAlert size={20}/><div><b>历史配置，仅供查看</b><p>这份计划不能继续修改、检查或提交。若要继续投放，请新建计划并选择目标广告平台。</p></div></div>
      </section> : <>
        {showConfiguration && platformConfiguration && editableConfiguration ? <section className="delivery-config-config-card"><header><div><span>当前计划 · V{selectedPlan.currentVersionNumber}</span><h3>{selectedPlan.currentVersion.name}</h3><p>更新于 {formatTime(selectedPlan.updatedAt)}</p></div><div className="delivery-config-contract"><span>配置草稿</span><button className="primary-button" type="button" onClick={() => void saveConfiguration()} disabled={busy}><Save size={15} aria-hidden="true"/>{busy ? '保存中…' : '保存'}</button></div></header><PlatformConfigurationEditor value={editableConfiguration} onChange={setEditableConfiguration} products={currentProject.products ?? []} assets={confirmedAssets} platformObjects={platformObjects}/><details className="delivery-config-mapping-details"><summary>查看 Manifest 字段映射</summary><PlatformConfigurationDetails value={editableConfiguration}/></details></section> : null}
        {showCalibration && platformConfiguration ? <CalibrationDispositionView value={platformConfiguration}/> : null}
        {showPreflight ? <section className="delivery-config-flow-grid delivery-config-flow-grid--preflight"><article className="delivery-config-preflight-card">
          <header><div><span className="section-label">确认投放</span><h3>检查并确认投放</h3></div><strong className="delivery-config-preflight-state">{changeSet ? `最近执行 ${changeSet.status}` : '尚未投放'}</strong></header>
          <div className="delivery-config-preflight-summary"><b>服务端校验</b><p>保存时会校验结构；确认投放时要求执行所需引用与平台证据均已解析。</p></div>
          <div className="delivery-config-actions delivery-config-preflight-actions">
            <button className="primary-button" onClick={() => void confirmLaunch()} disabled={busy || legacyReadOnly}><Check size={14}/>确认投放</button>
          </div>
        </article></section> : null}
      </>}
      {notice ? <div className="inline-notice" role="status">{notice}</div> : null}
    </div>
  </StateBoundary>
}
