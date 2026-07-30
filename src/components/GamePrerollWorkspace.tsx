import { useEffect, useMemo, useState } from 'react'
import { Check, ChevronDown, ChevronRight, CircleAlert, Play, Sparkles, Upload, WandSparkles } from 'lucide-react'
import { useModelConfig } from '../context/ModelConfigContext'
import { useProject } from '../context/ProjectContext'
import {
  api,
  type ApiGamePrerollCandidate,
  type ApiGamePrerollWorkspace,
  type ApiGenerationJob,
} from '../data/api'
import { buildDefendSunflowerInput, defendSunflowerFixture } from '../data/gamePrerollFixture'

const mechanismLabels: Record<ApiGamePrerollCandidate['hook_mechanism'], string> = {
  choice_challenge: '选择挑战',
  tactical_tradeoff: '战术取舍',
  wave_escalation: '波次压力',
  failure_reversal: '失败反转',
  merge_upgrade: '合成升级',
  reward_reveal: '奖励揭示',
}

const gameShotLabels = ['建立选择冲突', '放大策略取舍', 'CTA 收束']

export function GamePrerollWorkspace({ onNotice }: { onNotice: (message: string) => void }) {
  const { currentProject, reloadProjects } = useProject()
  const { providers } = useModelConfig()
  const [workspace, setWorkspace] = useState<ApiGamePrerollWorkspace | null>(null)
  const [sourceFile, setSourceFile] = useState<File | null>(null)
  const [job, setJob] = useState<ApiGenerationJob | null>(null)
  const [generatedVideoUrl, setGeneratedVideoUrl] = useState('')
  const [isPlanning, setIsPlanning] = useState(false)
  const [selectedShot, setSelectedShot] = useState(0)
  const [feedback, setFeedback] = useState('上传授权游戏实录后，固定样例会生成 3 个证据约束候选。')
  const configuredProvider = providers.find(provider => provider.status === '已配置')
  const draft = workspace?.video_draft.game_preroll
  const selectedCandidate = useMemo(
    () => draft?.candidates.find(candidate => candidate.id === draft.selected_candidate_id),
    [draft],
  )
  const isGenerating = job?.status === 'queued' || job?.status === 'running'
  const generated = job?.status === 'succeeded' && Boolean(generatedVideoUrl)
  const storyboard = selectedCandidate?.storyboard ?? draft?.candidates[0]?.storyboard ?? []
  const activeBeat = storyboard[selectedShot]

  useEffect(() => {
    let active = true
    void api.getLatestGamePrerollWorkspace(currentProject.id).then(async restored => {
      if (!active || !restored) return
      setWorkspace(restored)
      const restoredDraft = restored.video_draft.game_preroll
      const latestAttempt = restored.game_preroll_generation_attempts
        ?.filter(attempt => attempt.draft_revision === restored.video_draft.revision
          && attempt.candidate_id === restoredDraft.selected_candidate_id)
        .at(-1)
      if (!latestAttempt) {
        setFeedback(restoredDraft.selected_candidate_id
          ? '已恢复固定样例和人工选择，可以继续生成视频。'
          : '已恢复 3 个候选，请人工选择一个完整 6 秒方案。')
        return
      }
      const restoredJob = await api.getGamePrerollVideoJob(currentProject.id, latestAttempt.provider_job_id)
      if (!active) return
      setJob(restoredJob)
      if (restoredJob.status === 'succeeded' && restoredJob.artifactId) {
        const media = await api.listProjectMediaAssets(currentProject.id)
        if (!active) return
        const video = media.find(asset => asset.id === restoredJob.artifactId)
        setGeneratedVideoUrl(video?.contentUrl ?? '')
        setFeedback(video ? '已恢复候选、生成任务和稳定视频资产。' : '任务已成功，视频资产仍在入库。')
      } else {
        setFeedback(restoredJob.status === 'failed' ? '已恢复上一次失败任务，可以重新生成。' : '已恢复正在执行的视频任务。')
      }
    }).catch(cause => {
      if (active) setFeedback(cause instanceof Error ? cause.message : '游戏前贴工作区恢复失败。')
    })
    return () => { active = false }
  }, [currentProject.id])

  useEffect(() => {
    if (!job || !isGenerating) return
    const timer = window.setInterval(() => {
      void api.getGamePrerollVideoJob(currentProject.id, job.id).then(async next => {
        setJob(next)
        if (next.status === 'succeeded' && next.artifactId) {
          const media = await api.listProjectMediaAssets(currentProject.id)
          const video = media.find(asset => asset.id === next.artifactId)
          setGeneratedVideoUrl(video?.contentUrl ?? '')
          if (video) {
            await reloadProjects()
            setFeedback('游戏前贴生成完成，稳定视频资产已关联到当前 Project。')
            onNotice('《保卫向日葵》游戏前贴已生成并进入项目素材。')
          } else {
            setFeedback('任务已成功，视频资产仍在入库。')
          }
        } else if (next.status === 'failed' || next.status === 'cancelled') {
          setGeneratedVideoUrl('')
          setFeedback(next.status === 'failed'
            ? `视频生成失败${next.diagnostic ? `：${next.diagnostic}` : '，请重试。'}`
            : '视频任务已取消。')
        }
      }).catch(cause => setFeedback(cause instanceof Error ? cause.message : '视频任务状态读取失败。'))
    }, 1500)
    return () => window.clearInterval(timer)
  }, [currentProject.id, isGenerating, job, onNotice, reloadProjects])

  const createCandidates = async () => {
    if (!sourceFile) {
      setFeedback('请先选择你提供的授权《保卫向日葵》MP4 实录。')
      return
    }
    if (sourceFile.type !== 'video/mp4') {
      setFeedback('固定样例当前只接受 MP4 游戏实录。')
      return
    }
    setIsPlanning(true)
    setJob(null)
    setGeneratedVideoUrl('')
    try {
      setFeedback('正在上传授权实录并冻结玩法证据…')
      const sourceVideo = await api.uploadProjectAsset(currentProject.id, sourceFile)
      const created = await api.createManualGamePrerollWorkspace(
        currentProject.id,
        buildDefendSunflowerInput(sourceVideo),
      )
      setWorkspace(created)
      setFeedback('已生成 3 个证据约束候选。它们都是完整 6 秒视频方案，请人工选择一个。')
    } catch (cause) {
      setFeedback(cause instanceof Error ? cause.message : '固定样例候选创建失败。')
    } finally {
      setIsPlanning(false)
    }
  }

  const regenerateCandidates = async () => {
    if (!workspace) return
    setIsPlanning(true)
    setJob(null)
    setGeneratedVideoUrl('')
    try {
      setFeedback('正在调用 Seed-2-pro 重新规划 3 个候选…')
      const regenerated = await api.regenerateGamePrerollCandidates(
        currentProject.id,
        workspace.task.id,
        workspace.video_draft.revision,
      )
      setWorkspace(regenerated)
      setSelectedShot(0)
      const plannerVersion = regenerated.video_draft.game_preroll.active_candidate_batch.planner_version
      setFeedback(plannerVersion.startsWith('model:')
        ? 'Seed-2-pro 已完成候选规划，请人工选择一个完整 6 秒方案。'
        : '规划模型暂不可用，已生成可继续验证链路的确定性候选。')
    } catch (cause) {
      setFeedback(cause instanceof Error ? cause.message : '候选重新规划失败。')
    } finally {
      setIsPlanning(false)
    }
  }

  const selectCandidate = async (candidate: ApiGamePrerollCandidate) => {
    if (!workspace) return
    try {
      const selected = await api.selectGamePrerollCandidate(
        currentProject.id,
        workspace.task.id,
        workspace.video_draft.revision,
        candidate.id,
      )
      setWorkspace(selected)
      setSelectedShot(0)
      setJob(null)
      setGeneratedVideoUrl('')
      setFeedback(`已人工选择“${mechanismLabels[candidate.hook_mechanism]}”方案；PromptPackage 已冻结，可以生成视频。`)
    } catch (cause) {
      setFeedback(cause instanceof Error ? cause.message : '候选选择失败。')
    }
  }

  const generateVideo = async () => {
    if (!workspace || !selectedCandidate || !draft?.readiness.generation_ready) {
      setFeedback('请先人工选择一个候选，系统不会自动替你决定。')
      return
    }
    if (!configuredProvider) {
      setFeedback('服务端尚未配置视频 Provider，无法创建 Seedance 任务。')
      return
    }
    setJob(null)
    setGeneratedVideoUrl('')
    try {
      setFeedback('正在创建 Seedance 2.0 游戏前贴任务…')
      const next = await api.createGamePrerollVideoJob(
        currentProject.id,
        workspace.task.id,
        workspace.video_draft.revision,
        selectedCandidate.id,
      )
      setJob(next)
      setFeedback(next.status === 'succeeded' ? '视频已生成，正在确认稳定资产。' : '视频任务已创建，正在轮询生成状态。')
    } catch (cause) {
      setFeedback(cause instanceof Error ? cause.message : '游戏前贴视频任务创建失败。')
    }
  }

  return <div className="preroll-workspace game-preroll-workspace">
    <aside className="preroll-candidate-panel game-preroll-rail" aria-label="游戏前贴镜头与候选">
      <div className="preroll-storyboard game-preroll-storyboard" aria-label="游戏前贴镜头">
        <div className="surface-toolbar"><h3>镜头</h3><span>03 个镜头</span></div>
        <p className="preroll-keyboard-hint">所选候选内部的三个连续时间段</p>
        {storyboard.map((beat, index) => <button
          type="button"
          key={`${beat.start_milliseconds}-${beat.end_milliseconds}`}
          className={selectedShot === index ? 'active' : ''}
          aria-pressed={selectedShot === index}
          onClick={() => setSelectedShot(index)}
        >
          <span>0{index + 1}</span>
          <span>
            <b>{gameShotLabels[index] ?? `镜头 ${index + 1}`}</b>
            <small>
              {(beat.start_milliseconds / 1000).toFixed(0)}–{(beat.end_milliseconds / 1000).toFixed(0)} 秒 · {beat.copy}
            </small>
          </span>
          <ChevronRight size={14}/>
        </button>)}
      </div>
      <details className="game-candidate-section" open>
        <summary><span className="section-label">3 个完整候选</span><b>必须择一</b><ChevronDown size={15}/></summary>
        <p>候选分数表示玩法证据与钩子的匹配度，不是 CTR 或转化预测。</p>
        {workspace ? <div className="game-candidate-toolbar">
          <small>规划器：{draft?.active_candidate_batch.planner_version}</small>
          <button
            type="button"
            className="secondary-button"
            disabled={isPlanning || isGenerating}
            onClick={() => void regenerateCandidates()}
          >
            <Sparkles size={14}/>{isPlanning ? 'Seed-2-pro 规划中…' : '重新规划'}
          </button>
        </div> : null}
        <div className="game-candidate-grid">
          {!draft ? <div className="preroll-candidate-empty">上传授权实录并加载固定样例后，这里会出现 3 个完整的 6 秒方案。</div> : draft.candidates.map(candidate =>
            <article className="preroll-candidate-card" key={candidate.id}>
            <button
              type="button"
              className={draft.selected_candidate_id === candidate.id ? 'active' : ''}
              aria-pressed={draft.selected_candidate_id === candidate.id}
              onClick={() => void selectCandidate(candidate)}
            >
              <span><b>{mechanismLabels[candidate.hook_mechanism]}</b><small>证据匹配 {candidate.score}</small></span>
              <strong>{candidate.hook_line}</strong>
              <small>{candidate.evidence_moment_ids.join(' · ')}</small>
            </button>
            <div className="preroll-candidate-detail">
              <b>差异假设</b><p>{candidate.variant_hypothesis}</p>
              <b>完整 6 秒分镜</b>
              <ol>{candidate.storyboard.map(beat =>
                <li key={`${candidate.id}-${beat.start_milliseconds}`}>
                  <small>{(beat.start_milliseconds / 1000).toFixed(0)}–{(beat.end_milliseconds / 1000).toFixed(0)} 秒</small>
                  <span>{beat.copy}</span>
                </li>,
              )}</ol>
              <details><summary>服务端 PromptPackage</summary><small>{candidate.prompt_package.prompt_compiler_version}</small><pre>{candidate.prompt_package.compiled_prompt}</pre></details>
            </div>
            </article>,
          )}
        </div>
      </details>
    </aside>

    <section className="preroll-preview game game-preroll-main" aria-label="游戏前贴预览">
      <div className="preroll-preview-header">
        <span className="section-label">当前方案</span>
        <b>0{selectedShot + 1} / 03 · {selectedCandidate ? mechanismLabels[selectedCandidate.hook_mechanism] : '候选预览'}</b>
        <span>{generated ? '视频已生成' : isGenerating ? 'Seedance 生成中' : '待生成'}</span>
      </div>
      <div className="preroll-screen">
        {generatedVideoUrl
          ? <video controls playsInline preload="metadata" src={generatedVideoUrl} aria-label="生成好的游戏前贴视频"/>
          : <>
            <span>GAMEPLAY · EVIDENCE GROUNDED</span>
            <h3>{activeBeat?.copy ?? selectedCandidate?.hook_line ?? defendSunflowerFixture.name}</h3>
            <p>{activeBeat?.visual ?? (selectedCandidate
              ? selectedCandidate.storyboard.map(beat => beat.copy).join(' → ')
              : '技能三选一 → 战术取舍 → 第 2/10 波 → 立即下载')}</p>
            <button disabled={!generated}><Play size={20} fill="currentColor"/></button>
            <small>00:00 / 00:06 · 9:16 · CTA 立即下载</small>
          </>}
      </div>
      <div className="preroll-source">
        <span className="section-label">固定策略样例</span>
        <b>{defendSunflowerFixture.name}</b>
        <small>{defendSunflowerFixture.gameplaySummary}</small>
        <small>可用机制：选择挑战 / 战术取舍 / 波次压力</small>
        <small>证据：20.292–22.250s · 29.792–31.375s · 34.000–35.500s</small>
      </div>
      <p className="preroll-feedback" role="status" aria-live="polite">{feedback}</p>
    </section>

    <aside className="preroll-config game-preroll-config">
      <span className="section-label">生成配置</span>
      <h3>真实玩法挑战型</h3>

      <section className="game-config-section">
        <div className="game-config-heading"><span>01</span><b>素材状态</b></div>
        {workspace
          ? <div className="game-source-ready"><Check size={15}/><span><b>授权实录已确认</b><small>固定样例与三个证据时间段已冻结</small></span></div>
          : <>
            <label>授权游戏实录
              <input
                type="file"
                accept="video/mp4"
                onChange={event => setSourceFile(event.target.files?.[0] ?? null)}
              />
            </label>
            <small className="game-config-copy">{sourceFile?.name ?? '请选择已授权的竖屏 MP4 实录'}</small>
            <button className="secondary-button full" disabled={isPlanning} onClick={() => void createCandidates()}>
              {isPlanning ? <Sparkles size={15}/> : <Upload size={15}/>}
              {isPlanning ? '正在冻结证据…' : '上传并生成候选'}
            </button>
          </>}
      </section>

      <section className="game-config-section">
        <div className="game-config-heading"><span>02</span><b>画面参数</b></div>
        <label>字幕样式<select disabled defaultValue="high_contrast_dynamic"><option value="high_contrast_dynamic">高对比动态字幕</option></select></label>
        <label>钩子强度 <small>4 / 5</small><input aria-label="钩子强度" type="range" min="1" max="5" value="4" readOnly/></label>
      </section>

      <section className="game-config-section">
        <div className="game-config-heading"><span>03</span><b>生成前检查</b></div>
        {['玩法与 UI 事实已校验', '候选已经人工选择', '静音可理解', 'CTA 为立即下载'].map(item =>
          <span className="analysis-check" key={item}><Check size={14}/>{item}</span>,
        )}
        <details className="game-policy-details">
          <summary><CircleAlert size={14}/>查看禁用规则</summary>
          <small>禁止失败反转、合成升级、奖励揭示；禁止广告刷新植物弹窗。</small>
        </details>
      </section>

      <section className="game-config-section game-output-section">
        <div className="game-config-heading"><span>04</span><b>生成与输出</b></div>
        {!configuredProvider ? <div className="model-required"><CircleAlert size={15}/><span>服务端尚未配置视频 Provider。</span></div> : null}
        <button
          className="primary-button full"
          disabled={!configuredProvider || !draft?.readiness.generation_ready || isGenerating}
          aria-busy={isGenerating}
          onClick={() => void generateVideo()}
        >
          <WandSparkles size={15}/>{isGenerating ? 'Seedance 生成中…' : generated ? '重新生成游戏前贴' : '生成游戏前贴'}
        </button>
        <button className="secondary-button full" disabled={!generated} onClick={() => onNotice('视频已在当前 Project 的稳定素材中。')}>加入混剪素材箱</button>
        {job ? <div className="inline-notice">任务 {job.id.slice(0, 8)} · {job.status} · {job.model ?? '模型待分配'}{job.diagnostic ? ` · ${job.diagnostic}` : ''}</div> : null}
      </section>
    </aside>
  </div>
}
