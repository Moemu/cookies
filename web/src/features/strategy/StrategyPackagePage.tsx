import { useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { creativePreRollPath, creativeTasksPath } from '../../app/routes'
import { createCreativeIntakeFromStrategy, listCreativeIntakes } from '../creative/api'
import { createStrategyFeedback, listStrategyPackages } from './api'
import type { PackageVersion } from './types'
import './strategy.css'

export function StrategyPackagePage() {
  const { projectId = '', packageId = '' } = useParams()
  const navigate = useNavigate()
  const [value, setValue] = useState<PackageVersion | null>(null)
  const [rating, setRating] = useState<'useful' | 'partly_useful' | 'not_useful'>('useful')
  const [comment, setComment] = useState('')
  const [message, setMessage] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    const controller = new AbortController()
    listStrategyPackages(projectId, controller.signal).then(({ items }) => {
      const found = items.filter((item) => item.package_id === packageId).sort((a, b) => b.version - a.version)[0]
      if (!found) throw new Error('StrategyPackage 不存在。')
      setValue(found)
    }).catch((cause: unknown) => {
      if (!(cause instanceof DOMException && cause.name === 'AbortError')) setError(cause instanceof Error ? cause.message : '策略包加载失败。')
    })
    return () => controller.abort()
  }, [packageId, projectId])

  if (!value) return <section className="strategy-empty"><h1>StrategyPackage</h1><p>{error || '正在验证不可变版本…'}</p></section>
  const document = value.snapshot.strategy
  const preRollRoute = value.snapshot.creative_routes?.find((route) => route.route_type === 'pre_roll')
  return <section className="strategy-page strategy-object-page">
    <header className="strategy-header"><div><nav><Link to={`/projects/${projectId}/strategy/workspaces`}>策略</Link><span>/</span><span>{value.package_id}</span></nav><h1>StrategyPackage v{value.version}</h1><p>这是已批准的不可变交付版本；后续策略更新不会覆盖已经创建的 CreativeTask。</p></div><span className="strategy-status">已发布</span></header>
    {error ? <div className="strategy-alert" role="alert">{error}</div> : null}
    {message ? <div className="strategy-success" role="status">{message}</div> : null}
    <div className="package-layout">
      <main className="package-document">
        <section><span>内容哈希</span><code>{value.content_hash}</code></section>
        <section><h2>目标</h2><p>{document?.objective}</p></section>
        <section><h2>受众</h2><p>{document?.audience.primary}</p></section>
        <section><h2>核心主张</h2><p>{document?.proposition}</p></section>
        {document?.platform_plans?.map((plan) => <section key={plan.platform}><h2>{plan.platform} 执行方案</h2><p>{plan.role}</p><p><strong>转化路径：</strong>{plan.conversion_path}</p><ul>{plan.creative_ideas.map((idea) => <li key={idea}>{idea}</li>)}</ul></section>)}
        {preRollRoute ? <section><h2>Creative 路线 · 短剧前贴</h2><p>{preRollRoute.reason}</p><dl><div><dt>渠道</dt><dd>{preRollRoute.channels.join(' / ')}</dd></div><div><dt>规格</dt><dd>{preRollRoute.target_duration_seconds} 秒 · {preRollRoute.aspect_ratio}</dd></div><div><dt>确认</dt><dd>{preRollRoute.requires_human_confirmation ? '创建任务前必须人工确认' : '无需人工确认'}</dd></div></dl></section> : null}
      </main>
      <aside className="package-sidebar">
        <section><h2>交付证据</h2><dl><div><dt>Revision</dt><dd>{value.snapshot.strategy_revision}</dd></div><div><dt>BriefVersion</dt><dd>{value.snapshot.brief?.version || '—'}</dd></div><div><dt>Creative readiness</dt><dd>{value.snapshot.readiness.creative_ready ? '已满足' : '未满足'}</dd></div></dl></section>
        <section><h2>显式交接</h2><p>{preRollRoute ? '点击后创建只读 CreativeIntake，并进入短剧前贴工作台人工确认路线与主视频。' : '点击后创建 CreativeIntake；策略包仍是不可变事实来源。'}</p><button className="button button--primary" disabled={busy || !value.snapshot.readiness.creative_ready} onClick={async () => {
          setBusy(true)
          setError('')
          try {
            const existing = await listCreativeIntakes(projectId)
            const found = existing.items.some((intake) => intake.request.strategy_package?.package_id === value.package_id && intake.request.strategy_package.package_version === value.version && intake.request.strategy_package.expected_content_hash === value.content_hash)
            if (!found) await createCreativeIntakeFromStrategy(projectId, value.package_id, value.version, value.content_hash)
            navigate(preRollRoute ? creativePreRollPath(projectId) : creativeTasksPath(projectId))
          } catch (cause) {
            setError(cause instanceof Error ? cause.message : '交接失败。')
          } finally {
            setBusy(false)
          }
        }} type="button">发送到 Creative</button>{!value.snapshot.readiness.creative_ready ? <small>策略包还没有可交接的小红书或短剧前贴方案。</small> : null}</section>
        <section><h2>效果反馈</h2><select disabled={busy} onChange={(event) => setRating(event.target.value as typeof rating)} value={rating}><option value="useful">有用</option><option value="partly_useful">部分有用</option><option value="not_useful">无用</option></select><textarea disabled={busy} onChange={(event) => setComment(event.target.value)} placeholder="可选：说明具体原因" rows={3} value={comment} /><button className="button" disabled={busy} onClick={async () => {
          setBusy(true)
          setError('')
          try {
            await createStrategyFeedback(projectId, { target_type: 'strategy_package', target_id: value.package_id, target_version: value.version, rating, comment })
            setMessage('反馈已记录并进入策略评测闭环。')
          } catch (cause) {
            setError(cause instanceof Error ? cause.message : '反馈提交失败。')
          } finally {
            setBusy(false)
          }
        }} type="button">提交反馈</button></section>
      </aside>
    </div>
  </section>
}
