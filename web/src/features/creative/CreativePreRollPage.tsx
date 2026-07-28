import { useCallback, useEffect, useMemo, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { deliveryPath, strategyPath } from '../../app/routes'
import { ApiProblem } from '../../shared/api/client'
import { listProjectAssets } from '../assets/api'
import { UploadDrawer } from '../assets/UploadDrawer'
import type { ProjectAsset } from '../assets/types'
import { getProviderJob } from '../platform/api'
import type { ProviderJob } from '../platform/types'
import {
  approveCreativeVersion,
  checkCreativeVersion,
  createPreRollRenderJob,
  createVideoJob,
  createVideoTask,
  deliverCreativeVersion,
  freezeCreativeVersion,
  getCreativeRenderJob,
  getCreativeTask,
  listCreativeIntakes,
  listCreativePackages,
  listCreativeTasks,
  listCreativeVersions,
} from './api'
import type {
  CreativeIntake,
  CreativePackage,
  CreativeRenderJob,
  CreativeTask,
  CreativeTaskDetail,
  CreativeVersion,
} from './types'
import './pre-roll-workbench.css'

const providerTerminal = new Set<ProviderJob['provider_status']>(['succeeded', 'partially_succeeded', 'failed', 'cancelled', 'expired'])

export function CreativePreRollPage() {
  const { projectId = '' } = useParams()
  const [intakes, setIntakes] = useState<CreativeIntake[]>([])
  const [assets, setAssets] = useState<ProjectAsset[]>([])
  const [tasks, setTasks] = useState<CreativeTask[]>([])
  const [detail, setDetail] = useState<CreativeTaskDetail | null>(null)
  const [providerJob, setProviderJob] = useState<ProviderJob | null>(null)
  const [renderJob, setRenderJob] = useState<CreativeRenderJob | null>(null)
  const [version, setVersion] = useState<CreativeVersion | null>(null)
  const [pkg, setPackage] = useState<CreativePackage | null>(null)
  const [selectedIntakeId, setSelectedIntakeId] = useState('')
  const [selectedAssetKey, setSelectedAssetKey] = useState('')
  const [channel, setChannel] = useState<'douyin' | 'kuaishou'>('douyin')
  const [concept, setConcept] = useState('')
  const [prompt, setPrompt] = useState('为短剧主片生成一个 5 秒竖屏广告前贴，开场快速建立利益点，结尾自然衔接主片。')
  const [cta, setCTA] = useState('立即了解')
  const [busy, setBusy] = useState('')
  const [error, setError] = useState('')
  const [uploadOpen, setUploadOpen] = useState(false)

  const load = useCallback(async (signal?: AbortSignal) => {
    try {
      const [intakeResult, assetResult, taskResult, packageResult] = await Promise.all([
        listCreativeIntakes(projectId, signal),
        listProjectAssets(projectId, signal),
        listCreativeTasks(projectId, signal),
        listCreativePackages(projectId, signal),
      ])
      const videoTasks = taskResult.items.filter((item) => item.format === 'video' && item.performance_mode === 'pre_roll')
      const strategyIntakes = intakeResult.items.filter((item) => item.source === 'strategy_package' && item.request.creative_routes?.some((route) => route.route_type === 'pre_roll'))
      const videoAssets = assetResult.items.filter((item) => item.version.mime_type === 'video/mp4' && item.version.status === 'ready')
      setIntakes(strategyIntakes)
      setAssets(videoAssets)
      setTasks(videoTasks)
      setSelectedIntakeId((current) => current || strategyIntakes[0]?.id || '')
      setSelectedAssetKey((current) => current || assetKey(videoAssets[0]))

      const activeTask = videoTasks[0]
      if (activeTask) {
        const nextDetail = await getCreativeTask(projectId, activeTask.id, signal)
        setDetail(nextDetail)
        setConcept((current) => current || nextDetail.video_draft?.concept || '')
        const production = [...nextDetail.production_jobs].reverse().find((item) => item.kind === 'video_generate')
        if (production) setProviderJob(await getProviderJob(projectId, production.provider_job_id, signal))
        const versions = await listCreativeVersions(projectId, activeTask.id, signal)
        setVersion(versions.items[0] || null)
        setPackage(packageResult.items.find((item) => item.creative_version_id === versions.items[0]?.id) || null)
      } else {
        setDetail(null)
        setProviderJob(null)
        setVersion(null)
        setPackage(null)
      }
    } catch (caught) {
      if (caught instanceof DOMException && caught.name === 'AbortError') return
      setError(messageFor(caught))
    }
  }, [projectId])

  useEffect(() => {
    const controller = new AbortController()
    const timer = window.setTimeout(() => void load(controller.signal), 0)
    return () => { window.clearTimeout(timer); controller.abort() }
  }, [load])

  useEffect(() => {
    if (!providerJob || providerTerminal.has(providerJob.provider_status)) return
    const controller = new AbortController()
    const timer = window.setInterval(() => {
      void getProviderJob(projectId, providerJob.id, controller.signal).then(setProviderJob).catch((caught) => {
        if (!(caught instanceof DOMException && caught.name === 'AbortError')) setError(messageFor(caught))
      })
    }, 2000)
    return () => { window.clearInterval(timer); controller.abort() }
  }, [projectId, providerJob])

  useEffect(() => {
    if (!renderJob || renderJob.status === 'succeeded' || renderJob.status === 'failed') return
    const controller = new AbortController()
    const timer = window.setInterval(() => {
      void getCreativeRenderJob(projectId, renderJob.id, controller.signal).then(setRenderJob).catch((caught) => {
        if (!(caught instanceof DOMException && caught.name === 'AbortError')) setError(messageFor(caught))
      })
    }, 1500)
    return () => { window.clearInterval(timer); controller.abort() }
  }, [projectId, renderJob])

  const intake = intakes.find((item) => item.id === selectedIntakeId)
  const routeIndex = intake?.request.creative_routes?.findIndex((route) => route.route_type === 'pre_roll') ?? -1
  const route = routeIndex >= 0 ? intake?.request.creative_routes?.[routeIndex] : undefined
  const sourceAsset = useMemo(() => assets.find((item) => assetKey(item) === selectedAssetKey), [assets, selectedAssetKey])
  const task = detail?.task || tasks[0]
  const canGenerate = Boolean(task && !providerJob)
  const canRender = providerJob?.provider_status === 'succeeded' && providerJob.project_asset_refs.length === 1 && !renderJob
  const canFreeze = renderJob?.status === 'succeeded' && !version

  const act = async (name: string, operation: () => Promise<void>) => {
    if (busy) return
    setBusy(name)
    setError('')
    try {
      await operation()
    } catch (caught) {
      setError(messageFor(caught))
    } finally {
      setBusy('')
    }
  }

  const createTask = () => act('task', async () => {
    if (!intake || !route || !sourceAsset || routeIndex < 0) throw new Error('请先选择策略交接和已入库的 MP4 主视频。')
    const next = await createVideoTask(projectId, intake.id, {
      route_index: routeIndex,
      channel,
      source_video: sourceAsset.ref.asset_version,
      concept: concept.trim() || intake.request.concept || route.reason,
      prompt: prompt.trim(),
      call_to_action: cta.trim(),
      mandatory_elements: intake.request.mandatory_elements || [],
      prohibited_claims: intake.request.prohibited_claims || [],
      confirm_route: true,
    })
    setDetail(await getCreativeTask(projectId, next.id))
    setTasks([next, ...tasks])
  })

  const generate = () => act('generate', async () => {
    if (!task) return
    setProviderJob(await createVideoJob(projectId, task.id))
  })

  const render = () => act('render', async () => {
    if (!task) return
    setRenderJob(await createPreRollRenderJob(projectId, task.id))
  })

  const freeze = () => act('freeze', async () => {
    if (!task || !detail?.video_draft || !renderJob) return
    setVersion(await freezeCreativeVersion(projectId, task.id, detail.video_draft.revision, renderJob.id))
  })

  const check = () => act('check', async () => {
    if (!version) return
    setVersion(await checkCreativeVersion(projectId, version.id))
  })

  const approve = () => act('approve', async () => {
    if (!version) return
    setVersion(await approveCreativeVersion(projectId, version.id))
  })

  const deliver = () => act('deliver', async () => {
    if (!version) return
    setPackage(await deliverCreativeVersion(projectId, version.id))
  })

  return <section className="preroll-page">
    <header className="preroll-header">
      <div><span className="page-eyebrow">CREATIVE · PERFORMANCE VIDEO</span><h1>短剧前贴创作</h1><p>从已批准策略选择前贴路线，生成 5 秒素材并拼接主视频，交付可投放的 CreativePackage。</p></div>
      <Link className="button button--secondary button--compact" to={`/projects/${encodeURIComponent(projectId)}/assets`}>项目素材库</Link>
    </header>

    <ol className="preroll-progress" aria-label="短剧前贴闭环">
      {['策略路线', '生成前贴', '拼接主片', '评审交付', '模拟投放'].map((label, index) => <li className={stageIndex({ task, providerJob, renderJob, version, pkg }) >= index ? 'is-done' : ''} key={label}><b>{index + 1}</b><span>{label}</span></li>)}
    </ol>

    {error ? <div className="workspace-alert" role="alert"><span>{error}</span><button className="text-action" onClick={() => setError('')} type="button">关闭</button></div> : null}

    <div className="preroll-grid">
      <section className="preroll-panel">
        <div className="preroll-panel__heading"><div><span>01 · STRATEGY HANDOFF</span><h2>策略路线与主视频</h2></div><span className="status-chip status-chip--active">{route ? '可创建' : '等待策略'}</span></div>
        {intakes.length ? <>
          <label>已批准策略交接<select value={selectedIntakeId} onChange={(event) => setSelectedIntakeId(event.target.value)}>{intakes.map((item) => <option key={item.id} value={item.id}>{item.request.strategy_package?.package_id} · {item.id}</option>)}</select></label>
          {route ? <div className="route-card"><strong>推荐：5 秒短剧前贴</strong><p>{route.reason}</p><small>{route.channels.join(' / ')} · {route.aspect_ratio} · 需要人工确认</small></div> : null}
        </> : <div className="preroll-empty"><p>还没有包含前贴路线的已批准 StrategyPackage。</p><Link to={strategyPath(projectId)}>前往需求与策略</Link></div>}

        <label>渠道<select value={channel} onChange={(event) => setChannel(event.target.value as 'douyin' | 'kuaishou')}><option value="douyin">抖音</option><option value="kuaishou">快手</option></select></label>
        <label>已授权 MP4 主视频<select value={selectedAssetKey} onChange={(event) => setSelectedAssetKey(event.target.value)}><option value="">请选择</option>{assets.map((item) => <option key={assetKey(item)} value={assetKey(item)}>{item.asset.id} · v{item.version.version} · {formatDuration(item.version.duration_ms)}</option>)}</select></label>
        <button className="text-action" onClick={() => setUploadOpen(true)} type="button">上传新的 MP4 主视频</button>
        <label>前贴概念<input value={concept} onChange={(event) => setConcept(event.target.value)} placeholder={intake?.request.concept || '一句话概念'} /></label>
        <label>Seedance 提示词<textarea value={prompt} onChange={(event) => setPrompt(event.target.value)} /></label>
        <label>行动号召<input value={cta} onChange={(event) => setCTA(event.target.value)} /></label>
        <button className="button button--primary" disabled={Boolean(task) || !route || !sourceAsset || busy === 'task'} onClick={() => void createTask()} type="button">{task ? '视频任务已创建' : busy === 'task' ? '创建中…' : '确认路线并创建任务'}</button>
      </section>

      <section className="preroll-panel preroll-production">
        <div className="preroll-panel__heading"><div><span>02—04 · PRODUCTION</span><h2>生成、拼接与交付</h2></div><span className="status-chip">{task?.status || '未开始'}</span></div>
        <WorkflowStep index="1" title="生成 5 秒前贴" description={providerJob ? `${providerJob.provider_status} · ${providerJob.progress}%` : '通过 Provider Gateway 调用 cookies.video.standard'}>
          <button className="button button--primary button--compact" disabled={!canGenerate || busy === 'generate'} onClick={() => void generate()} type="button">生成前贴</button>
        </WorkflowStep>
        <WorkflowStep index="2" title="FFmpeg 拼接" description={renderJob ? `${renderJob.status}${renderJob.output_asset ? ` · ${renderJob.output_asset.asset_version.asset_id}` : ''}` : '前贴在前，已授权主视频在后；输出重新入库'}>
          <button className="button button--secondary button--compact" disabled={!canRender || busy === 'render'} onClick={() => void render()} type="button">拼接主片</button>
        </WorkflowStep>
        <WorkflowStep index="3" title="冻结与评审" description={version ? `${version.status} · ${version.id}` : '冻结不可变视频版本，并执行确定性检查'}>
          <div className="preroll-actions">
            <button className="button button--secondary button--compact" disabled={!canFreeze || busy === 'freeze'} onClick={() => void freeze()} type="button">冻结版本</button>
            <button className="button button--secondary button--compact" disabled={version?.status !== 'created' || busy === 'check'} onClick={() => void check()} type="button">执行检查</button>
            <button className="button button--secondary button--compact" disabled={version?.status !== 'checked' || busy === 'approve'} onClick={() => void approve()} type="button">人工批准</button>
          </div>
        </WorkflowStep>
        <WorkflowStep index="4" title="交付 CreativePackage" description={pkg ? `${pkg.id} · ${pkg.content_hash}` : '只有批准后的不可变版本可以进入 Delivery'}>
          {pkg ? <Link className="button button--primary button--compact" to={deliveryPath(projectId)}>进入模拟投放</Link> : <button className="button button--primary button--compact" disabled={version?.status !== 'approved' || busy === 'deliver'} onClick={() => void deliver()} type="button">生成交付包</button>}
        </WorkflowStep>
      </section>
    </div>

    <UploadDrawer acceptVideoOnly open={uploadOpen} projectId={projectId} onClose={() => setUploadOpen(false)} onComplete={() => {
      setUploadOpen(false)
      void load()
    }} />
  </section>
}

function WorkflowStep({ children, description, index, title }: { children: React.ReactNode, description: string, index: string, title: string }) {
  return <article className="preroll-step"><b>{index}</b><div><strong>{title}</strong><p>{description}</p></div><div>{children}</div></article>
}

function assetKey(asset?: ProjectAsset) {
  return asset ? `${asset.ref.asset_version.asset_id}:${asset.ref.asset_version.version}` : ''
}

function formatDuration(duration?: number) {
  return duration ? `${(duration / 1000).toFixed(1)} 秒` : '待探测'
}

function stageIndex(value: { task?: CreativeTask, providerJob: ProviderJob | null, renderJob: CreativeRenderJob | null, version: CreativeVersion | null, pkg: CreativePackage | null }) {
  if (value.pkg) return 4
  if (value.version) return 3
  if (value.renderJob?.status === 'succeeded') return 2
  if (value.providerJob?.provider_status === 'succeeded') return 1
  return value.task ? 0 : -1
}

function messageFor(caught: unknown) {
  if (caught instanceof ApiProblem) return `${caught.problem.error.message}（${caught.problem.error.code}）`
  return caught instanceof Error ? caught.message : '操作未完成，请稍后重试。'
}
