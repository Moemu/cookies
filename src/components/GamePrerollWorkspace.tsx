import { useEffect, useMemo, useRef, useState } from 'react'
import { Check, ChevronDown, ChevronRight, CircleAlert, Film, Play, Sparkles, Upload, WandSparkles } from 'lucide-react'
import { useModelConfig } from '../context/ModelConfigContext'
import { useProject } from '../context/ProjectContext'
import {
  api,
  type ApiGamePrerollCandidate,
  type ApiGamePrerollWorkspace,
  type ApiGenerationJob,
} from '../data/api'
import { buildDefendSunflowerInput, defendSunflowerFixture } from '../data/gamePrerollFixture'
import {
  buildGameShotViews,
  compareGamePrerollCandidates,
  type GameShotView,
} from '../features/game-preroll/presentation'

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
  const [sourceVideoUrl, setSourceVideoUrl] = useState('')
	const [evidenceFrameUrls, setEvidenceFrameUrls] = useState<Record<string, string>>({})
  const [isPlanning, setIsPlanning] = useState(false)
  const [selectedShot, setSelectedShot] = useState(0)
  const generatedVideoRef = useRef<HTMLVideoElement>(null)
  const [feedback, setFeedback] = useState('上传授权游戏实录后，系统会先规划 3 套方案，不会立即生成视频。')
  const configuredProvider = providers.find(provider => provider.status === '已配置')
  const draft = workspace?.video_draft.game_preroll
  const selectedCandidate = useMemo(
    () => draft?.candidates.find(candidate => candidate.id === draft.selected_candidate_id),
    [draft],
  )
  const isGenerating = job?.status === 'queued' || job?.status === 'running'
  const generated = job?.status === 'succeeded' && Boolean(generatedVideoUrl)
  const shotViews = useMemo(
    () => buildGameShotViews(selectedCandidate, draft?.input_snapshot.evidence_moments ?? []),
    [draft?.input_snapshot.evidence_moments, selectedCandidate],
  )
  const comparedCandidates = useMemo(
    () => compareGamePrerollCandidates(draft?.candidates ?? []),
    [draft?.candidates],
  )
  const activeShot = shotViews[selectedShot]

  useEffect(() => {
    let active = true
    void Promise.all([
      api.getLatestGamePrerollWorkspace(currentProject.id),
      api.listProjectMediaAssets(currentProject.id),
    ]).then(async ([restored, media]) => {
      if (!active || !restored) return
      setWorkspace(restored)
      const restoredDraft = restored.video_draft.game_preroll
      const sourceAsset = media.find(asset => asset.id === restoredDraft.input_snapshot.source_video.asset_id)
      setSourceVideoUrl(sourceAsset?.contentUrl ?? '')
	  setEvidenceFrameUrls(Object.fromEntries((restoredDraft.evidence_assets?.frames ?? []).map(frame => [
	    frame.evidence_moment_id,
	    media.find(asset => asset.id === frame.frame_asset.asset_version.asset_id)?.contentUrl ?? '',
	  ])))
      const latestAttempt = restored.game_preroll_generation_attempts
        ?.filter(attempt => attempt.draft_revision === restored.video_draft.revision
          && attempt.candidate_id === restoredDraft.selected_candidate_id)
        .at(-1)
      if (!latestAttempt) {
        setFeedback(restoredDraft.selected_candidate_id
          ? '已恢复固定样例和人工选择，可以继续生成视频。'
          : '已恢复 3 套创意方案；它们目前只是规划，请人工选择一套。')
        return
      }
      const restoredJob = await api.getGamePrerollVideoJob(currentProject.id, latestAttempt.provider_job_id)
      if (!active) return
      setJob(restoredJob)
      if (restoredJob.status === 'succeeded' && restoredJob.artifactId) {
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
	  setFeedback('已规划 3 套创意方案，正在从三个证据时间段抽取真实关键帧。')
	  const prepared = await api.prepareGamePrerollEvidence(
	    currentProject.id,
	    created.task.id,
	    created.video_draft.revision,
	  )
      const media = await api.listProjectMediaAssets(currentProject.id)
      setWorkspace(prepared)
      setSourceVideoUrl(media.find(asset => asset.id === sourceVideo.asset_id)?.contentUrl ?? '')
	  setEvidenceFrameUrls(Object.fromEntries((prepared.video_draft.game_preroll.evidence_assets?.frames ?? []).map(frame => [
	    frame.evidence_moment_id,
	    media.find(asset => asset.id === frame.frame_asset.asset_version.asset_id)?.contentUrl ?? '',
	  ])))
      setSelectedShot(0)
	  setFeedback('已完成 3 套 6 秒创意方案和真实证据帧准备；请选择一套，系统才会封存首尾帧生成规格。')
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
        ? 'Seed-2-pro 已重新规划 3 套方案；尚未生成视频，请人工选择一套。'
        : '规划模型暂不可用，已生成可继续验证链路的确定性候选。')
    } catch (cause) {
      setFeedback(cause instanceof Error ? cause.message : '候选重新规划失败。')
    } finally {
      setIsPlanning(false)
    }
  }

	const prepareEvidence = async () => {
	  if (!workspace) return
	  setIsPlanning(true)
	  try {
	    setFeedback('正在从三个证据时间段抽取关键帧并保存素材血缘…')
	    const prepared = await api.prepareGamePrerollEvidence(currentProject.id, workspace.task.id, workspace.video_draft.revision)
	    const media = await api.listProjectMediaAssets(currentProject.id)
	    setWorkspace(prepared)
	    setEvidenceFrameUrls(Object.fromEntries((prepared.video_draft.game_preroll.evidence_assets?.frames ?? []).map(frame => [
	      frame.evidence_moment_id,
	      media.find(asset => asset.id === frame.frame_asset.asset_version.asset_id)?.contentUrl ?? '',
	    ])))
	    setFeedback('三个真实证据帧已准备，可人工选择方案并封存 Seedance 首尾帧生成规格。')
	  } catch (cause) {
	    setFeedback(cause instanceof Error ? cause.message : '证据帧准备失败。')
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
      setFeedback(`已选择“${mechanismLabels[candidate.hook_mechanism]}”方案；左侧显示它的三个分镜，现在可以生成一条完整 6 秒视频。`)
    } catch (cause) {
      setFeedback(cause instanceof Error ? cause.message : '候选选择失败。')
    }
  }

  const generateVideo = async () => {
	if (!workspace || !selectedCandidate) {
	  setFeedback('请先人工选择一个候选，系统不会自动替你决定。')
	  return
	}
	if (!draft?.readiness.generation_ready || !draft.generation_spec || draft.generation_spec.input_mode !== 'first_last_frame') {
	  setFeedback('请先准备真实证据帧并封存首尾帧 GenerationSpec，系统不会退回 text-only 生成。')
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

  const selectShot = (index: number, shot: GameShotView) => {
    setSelectedShot(index)
    const video = generatedVideoRef.current
    if (!video || !generated) return
    video.pause()
    video.currentTime = shot.seekSeconds
    setFeedback(`已定位到镜头 ${String(index + 1).padStart(2, '0')}（${shot.outputRangeLabel}），不会创建新的生成任务。`)
  }

  return <div className="preroll-workspace game-preroll-workspace">
    <aside className="preroll-candidate-panel game-preroll-rail" aria-label="所选方案分镜与创意方案">
      <div className="preroll-storyboard game-preroll-storyboard" aria-label="所选方案分镜">
        <div className="surface-toolbar"><h3>所选方案分镜</h3><span>{selectedCandidate ? '03 个镜头' : '待选择'}</span></div>
        <p className="preroll-keyboard-hint">三个镜头共同组成一条 6 秒视频；点击只切换预览或定位成片。</p>
        {!selectedCandidate ? <div className="game-shot-empty">
          <Film size={18}/>
          <b>先选择一套创意方案</b>
          <small>选择后，这里才显示该方案的三个连续分镜。</small>
        </div> : shotViews.map((shot, index) => <button
          type="button"
          key={shot.id}
          className={selectedShot === index ? 'active' : ''}
          aria-pressed={selectedShot === index}
          onClick={() => selectShot(index, shot)}
        >
          <EvidenceThumbnail
            sourceUrl={sourceVideoUrl}
			imageUrl={evidenceFrameUrls[shot.beat.evidence_moment_id] ?? ''}
            timestampSeconds={shot.thumbnailTimestampSeconds}
            label={`镜头 ${index + 1} 的源实录画面`}
          />
          <span className="game-shot-index">0{index + 1}</span>
          <span className="game-shot-copy">
            <b>{gameShotLabels[index] ?? `镜头 ${index + 1}`}</b>
            <small>成片 {shot.outputRangeLabel}</small>
            <small>来源 {shot.sourceRangeLabel}</small>
            <small>{shot.evidenceDescription}</small>
			<small>{index === 0 ? '模型条件：Seedance 首帧' : index === 2 ? '模型条件：Seedance 尾帧' : '用途：证据预览 / Prompt 约束'}</small>
          </span>
          <ChevronRight size={14}/>
        </button>)}
      </div>
      <details className="game-candidate-section" open>
        <summary><span className="section-label">3 套 6 秒创意方案</span><b>选择一套后才生成视频</b><ChevronDown size={15}/></summary>
        <p>这里生成的是方案和分镜，不是 3 条视频。推荐仅表示玩法证据匹配度，不是 CTR 或转化预测，也不会替你自动选择。</p>
        {workspace ? <div className="game-candidate-toolbar">
          <small>规划器：{draft?.active_candidate_batch.planner_version}</small>
          <button
            type="button"
            className="secondary-button"
            disabled={isPlanning || isGenerating}
            onClick={() => void regenerateCandidates()}
          >
            <Sparkles size={14}/>{isPlanning ? 'Seed-2-pro 规划中…' : '重新规划 3 套方案'}
          </button>
        </div> : null}
        <div className="game-candidate-grid">
          {!draft ? <div className="preroll-candidate-empty">上传授权实录并加载固定样例后，这里会规划 3 套完整的 6 秒方案，但不会调用视频模型。</div> : comparedCandidates.map(({ candidate, recommended, recommendationLabel }, index) =>
            <article className={`preroll-candidate-card${recommended ? ' recommended' : ''}`} key={candidate.id}>
            <button
              type="button"
              className={draft.selected_candidate_id === candidate.id ? 'active' : ''}
              aria-pressed={draft.selected_candidate_id === candidate.id}
              onClick={() => void selectCandidate(candidate)}
            >
              <span><b>方案 {String(index + 1).padStart(2, '0')} · {mechanismLabels[candidate.hook_mechanism]}</b><small>{recommended ? '推荐' : '备选'}</small></span>
              <strong>{candidate.hook_line}</strong>
              <small>{recommendationLabel} · {candidate.score}</small>
            </button>
            <div className="preroll-candidate-detail">
              <b>唯一测试变量</b><p>{candidate.primary_test_variable}</p>
              <b>方案假设</b><p>{candidate.variant_hypothesis}</p>
              <b>证据覆盖</b><p>{candidate.evidence_moment_ids.join(' · ')}</p>
              <details><summary>查看完整分镜与 Prompt</summary>
                <ol>{candidate.storyboard.map(beat =>
                  <li key={`${candidate.id}-${beat.start_milliseconds}`}>
                    <small>{(beat.start_milliseconds / 1000).toFixed(0)}–{(beat.end_milliseconds / 1000).toFixed(0)} 秒</small>
                    <span>{beat.copy}</span>
                  </li>,
                )}</ol>
                <small>{candidate.prompt_package.prompt_compiler_version}</small>
                <pre>{candidate.prompt_package.compiled_prompt}</pre>
              </details>
            </div>
            </article>,
          )}
        </div>
      </details>
    </aside>

    <section className="preroll-preview game game-preroll-main" aria-label="游戏前贴预览">
      <div className="preroll-preview-header">
        <span className="section-label">所选方案预览</span>
        <b>{selectedCandidate ? `0${selectedShot + 1} / 03 · ${mechanismLabels[selectedCandidate.hook_mechanism]}` : '尚未选择方案'}</b>
        <span>{generated ? '视频已生成' : isGenerating ? 'Seedance 生成中' : '待生成'}</span>
      </div>
      <div className="preroll-screen">
        {generatedVideoUrl
          ? <video ref={generatedVideoRef} controls playsInline preload="metadata" src={generatedVideoUrl} aria-label="生成好的游戏前贴视频"/>
          : <>
            <span>GAMEPLAY · EVIDENCE GROUNDED</span>
            <h3>{activeShot?.beat.copy ?? selectedCandidate?.hook_line ?? '请从下方选择一套创意方案'}</h3>
            <p>{activeShot?.beat.visual ?? (selectedCandidate
              ? selectedCandidate.storyboard.map(beat => beat.copy).join(' → ')
              : '三套方案目前都只是规划；人工选择后才会生成一条 6 秒视频。')}</p>
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
        <small>固定约束：CTA“立即下载” · 禁止虚构奖励、数值和游戏 UI</small>
      </div>
      <p className="preroll-feedback" role="status" aria-live="polite">{feedback}</p>
    </section>

    <aside className="preroll-config game-preroll-config">
      <span className="section-label">生产控制</span>
      <h3>从方案到一条 6 秒视频</h3>

      <section className="game-config-section">
        <div className="game-config-heading"><span>01</span><b>素材与固定约束</b></div>
        {workspace
          ? <>
            <div className="game-source-ready"><Check size={15}/><span><b>授权实录已确认</b><small>固定样例与三个证据时间段已冻结</small></span></div>
            <dl className="game-fixed-constraints">
              <div><dt>游戏</dt><dd>{draft?.input_snapshot.game_name}</dd></div>
              <div><dt>CTA</dt><dd>{draft?.input_snapshot.call_to_action}</dd></div>
              <div><dt>证据</dt><dd>{draft?.input_snapshot.evidence_moments.length ?? 0} 个时间段</dd></div>
			  <div><dt>关键帧</dt><dd>{draft?.evidence_assets?.status === 'ready' ? '3 张已落库' : '尚未准备'}</dd></div>
            </dl>
			{draft?.evidence_assets?.status !== 'ready' ? <button className="secondary-button full" disabled={isPlanning} onClick={() => void prepareEvidence()}><Film size={14}/>{isPlanning ? '正在准备…' : '准备 3 张证据帧'}</button> : null}
          </>
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
              {isPlanning ? '正在冻结证据并规划…' : '上传并规划 3 套方案'}
            </button>
          </>}
      </section>

      <section className="game-config-section">
        <div className="game-config-heading"><span>02</span><b>候选规划参数</b></div>
        <label>钩子强度 <small>{draft?.active_candidate_batch.generation_config.hook_strength ?? 4} / 5</small><input aria-label="钩子强度" type="range" min="1" max="5" value={draft?.active_candidate_batch.generation_config.hook_strength ?? 4} readOnly/></label>
        <label>节奏倾向<select disabled value={draft?.active_candidate_batch.generation_config.pace_profile ?? 'punchy'}><option value="punchy">快速有冲击</option><option value="balanced">平衡可读</option></select></label>
        <small className="game-config-copy">改变这一组参数需要重新规划方案，不会调用 Seedance。</small>
        <button className="secondary-button full" disabled={!workspace || isPlanning || isGenerating} onClick={() => void regenerateCandidates()}>
          <Sparkles size={14}/>{isPlanning ? '正在规划…' : '重新规划 3 套方案'}
        </button>
      </section>

      <section className="game-config-section">
        <div className="game-config-heading"><span>03</span><b>成片参数</b></div>
        <label>字幕样式<select disabled value={draft?.active_candidate_batch.generation_config.subtitle_style ?? 'high_contrast_dynamic'}><option value="high_contrast_dynamic">高对比动态字幕</option><option value="brand_minimal">品牌极简字幕</option></select></label>
        <div className="game-production-facts"><span>时长 <b>6 秒</b></span><span>画幅 <b>9:16</b></span><span>清晰度 <b>720p</b></span></div>
        <small className="game-config-copy">成片参数决定最终 GenerationSpec，不代表重新规划创意方向。</small>
      </section>

      <section className="game-config-section">
        <div className="game-config-heading"><span>04</span><b>生成前检查</b></div>
		{['玩法与 UI 事实已校验', '3 张证据帧已准备', '候选已经人工选择', '静音可理解', 'CTA 为立即下载'].map(item => {
		  const pending = (item === '候选已经人工选择' && !selectedCandidate) || (item === '3 张证据帧已准备' && draft?.evidence_assets?.status !== 'ready')
		  return <span className={`analysis-check${pending ? ' pending' : ''}`} key={item}>
			{pending ? <CircleAlert size={14}/> : <Check size={14}/>} {item}
          </span>
		})}
        <details className="game-policy-details">
          <summary><CircleAlert size={14}/>查看禁用规则</summary>
          <small>禁止失败反转、合成升级、奖励揭示；禁止广告刷新植物弹窗。</small>
        </details>
      </section>

      <section className="game-config-section game-output-section">
        <div className="game-config-heading"><span>05</span><b>生成与输出</b></div>
        {!configuredProvider ? <div className="model-required"><CircleAlert size={15}/><span>服务端尚未配置视频 Provider。</span></div> : null}
		<div className="game-production-facts"><span>输入 <b>{draft?.generation_spec?.input_mode ?? '待封存'}</b></span><span>条件图 <b>{draft?.generation_spec?.conditioning_assets.length ?? 0} 张</b></span></div>
        <button
          className="primary-button full"
		  disabled={!configuredProvider || !draft?.readiness.generation_ready || draft?.generation_spec?.input_mode !== 'first_last_frame' || isGenerating}
          aria-busy={isGenerating}
          onClick={() => void generateVideo()}
        >
          <WandSparkles size={15}/>{isGenerating ? 'Seedance 生成中…' : generated ? '按当前方案重新生成视频' : '生成所选方案的 6 秒视频'}
        </button>
        <button className="secondary-button full" disabled={!generated} onClick={() => onNotice('视频已在当前 Project 的稳定素材中。')}>加入混剪素材箱</button>
        {job ? <div className="inline-notice">任务 {job.id.slice(0, 8)} · {job.status} · {job.model ?? '模型待分配'}{job.diagnostic ? ` · ${job.diagnostic}` : ''}</div> : null}
      </section>
    </aside>
  </div>
}

function EvidenceThumbnail({
  sourceUrl,
	imageUrl,
  timestampSeconds,
  label,
}: {
  sourceUrl: string
	imageUrl: string
  timestampSeconds: number
  label: string
}) {
  const videoRef = useRef<HTMLVideoElement>(null)

  useEffect(() => {
    seekEvidenceFrame(videoRef.current, timestampSeconds)
  }, [sourceUrl, timestampSeconds])

	if (imageUrl) {
	  return <span className="game-shot-thumbnail"><img src={imageUrl} alt={label}/></span>
	}

  if (!sourceUrl) {
    return <span className="game-shot-thumbnail empty"><Film size={16}/><small>实录帧</small></span>
  }

  return <span className="game-shot-thumbnail">
    <video
      ref={videoRef}
      muted
      playsInline
      preload="metadata"
      src={sourceUrl}
      aria-label={label}
      onLoadedMetadata={event => seekEvidenceFrame(event.currentTarget, timestampSeconds)}
      onSeeked={event => event.currentTarget.pause()}
    />
  </span>
}

function seekEvidenceFrame(video: HTMLVideoElement | null, timestampSeconds: number) {
  if (!video || video.readyState < HTMLMediaElement.HAVE_METADATA || !Number.isFinite(video.duration)) return
  const latestSafeFrame = Math.max(0, video.duration - 0.05)
  video.currentTime = Math.min(Math.max(0, timestampSeconds), latestSafeFrame)
}
