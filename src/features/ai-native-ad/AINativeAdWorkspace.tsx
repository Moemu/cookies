import { useEffect, useReducer, useState } from 'react'
import { AlertTriangle } from 'lucide-react'
import { analyzeRequirement, cancelProduction, confirmRequirement, confirmScript, confirmStoryboard, generateScript, generateStoryboard, getAssetPreview, getLatestRequirementWorkspace, getRequirementReopenImpact, getRequirementWorkspace, regenerateScript as regenerateScriptRequest, reopenRequirement, reopenScript, reopenStoryboard, retryProductionUnit, startProduction } from './api'
import { aiNativeReducer, initialAINativeState, productionFailureMessage } from './reducer'
import { AINativeStageStepper } from './AINativeStageStepper'
import { RequirementStage } from './RequirementStage'
import { ScriptStage } from './ScriptStage'
import { StoryboardStage } from './StoryboardStage'
import { VideoStage } from './VideoStage'
import type { AdScriptDraft, AINativeRequirement, AINativeRequirementWorkspace, AINativeStageId, StoryboardDraft } from './types'
import { aiNativeWorkspaceLocation, readAINativeWorkspaceLocation } from './navigation'
import { forgetAINativeWorkspace, readAINativeStageDraft, readAINativeWorkspacePointer, rememberAINativeStageDraft, rememberAINativeWorkspace } from './storage'
import { useAINativeAutosave } from './useAINativeAutosave'
import './ai-native-ad.css'

const invalidationCopy: Record<AINativeStageId, string> = {
  requirement: '重新编辑需求后，当前脚本、故事板和视频结果都会作废。',
  script: '重新编辑脚本后，当前故事板和视频结果都会作废。',
  storyboard: '重新编辑故事板后，当前视频结果会作废。',
  video: '重新生成视频不会修改已经确认的需求、脚本和故事板。',
}

export function AINativeAdWorkspace({ projectId, onNotice }: { projectId: string; onNotice: (message: string) => void }) {
  const [state, dispatch] = useReducer(aiNativeReducer, initialAINativeState)
  const [productLink, setProductLink] = useState('')
  const [supplementalRequirement, setSupplementalRequirement] = useState('')
  const autosave = useAINativeAutosave(projectId, state, dispatch)

  useEffect(() => {
	let active = true
    dispatch({ type: 'reset' })
    setProductLink('')
    setSupplementalRequirement('')
	const restore = async () => {
	  const urlLocation = readAINativeWorkspaceLocation(window.location.search)
	  const savedLocation = readAINativeWorkspacePointer(projectId)
	  const location = urlLocation ?? savedLocation
	  let workspace: AINativeRequirementWorkspace | null = null
	  if (location) {
	    try {
	      workspace = await getRequirementWorkspace(projectId, location.workspaceId)
	    } catch {
	      forgetAINativeWorkspace(projectId)
	    }
	  }
	  if (!workspace) workspace = await getLatestRequirementWorkspace(projectId)
	  if (!active || !workspace) return
	  const requirementDraft = readAINativeStageDraft<AINativeRequirement>(projectId, workspace.workspace_id, 'requirement', workspace.requirement.revision)
	  const scriptDraft = workspace.script ? readAINativeStageDraft<AdScriptDraft>(projectId, workspace.workspace_id, 'script', workspace.script.revision) : null
	  const storyboardSource = workspace.storyboard ?? workspace.storyboard_plan
	  const storyboardDraft = storyboardSource ? readAINativeStageDraft<StoryboardDraft>(projectId, workspace.workspace_id, 'storyboard', storyboardSource.revision) : null
	  workspace = {
	    ...workspace,
	    requirement: requirementDraft ?? workspace.requirement,
	    script: scriptDraft ?? workspace.script,
	    storyboard: storyboardDraft ?? workspace.storyboard,
	    storyboard_plan: storyboardDraft ?? workspace.storyboard_plan,
	  }
	  if (requirementDraft) autosave.schedule('requirement', requirementDraft)
	  if (scriptDraft) autosave.schedule('script', scriptDraft)
	  if (storyboardDraft) autosave.schedule('storyboard', storyboardDraft)
	  const restoredStage: AINativeStageId = location?.workspaceId === workspace.workspace_id
	    ? location.stage
	    : workspace.current_stage === 'production' ? 'video' : workspace.current_stage
	  dispatch({ type: 'requirement-loaded', workspace })
	  dispatch({ type: 'open-stage', stage: restoredStage })
	  if (workspace.storyboard_status === 'failed') {
	    dispatch({
	      type: 'operation-failed',
	      stage: 'storyboard',
	      message: workspace.storyboard_error_message || '故事板或参考图片生成失败，请重试。',
	    })
	  }
	  if (workspace.production_status === 'failed' || workspace.production_status === 'render_failed') {
	    dispatch({ type: 'operation-failed', stage: 'video', message: productionFailureMessage(workspace) })
	  }
	  setProductLink(workspace.requirement.product.source_url)
	  setSupplementalRequirement(workspace.requirement.supplemental_requirement)
	  rememberAINativeWorkspace(projectId, { workspaceId: workspace.workspace_id, stage: restoredStage })
	  window.history.replaceState(null, '', `${window.location.pathname}${aiNativeWorkspaceLocation(window.location.search, workspace.workspace_id, restoredStage)}${window.location.hash}`)
	}
	void restore().catch(cause => {
	  if (!active) return
	  dispatch({ type: 'operation-failed', stage: 'requirement', message: cause instanceof Error ? cause.message : '工作区恢复失败。' })
	})
	return () => { active = false }
  }, [projectId])

  useEffect(() => {
    const workspaceId = state.workspace?.workspace_id
    if (!workspaceId) return
    rememberAINativeWorkspace(projectId, { workspaceId, stage: state.active_stage })
  }, [projectId, state.active_stage, state.workspace?.workspace_id])

  useEffect(() => {
    const workspace = state.workspace
    if (!workspace || workspace.script_status !== 'generating') return
    let active = true
    const refresh = async () => {
      try {
        const latest = await getRequirementWorkspace(projectId, workspace.workspace_id)
        if (!active) return
        dispatch({ type: 'requirement-loaded', workspace: latest })
        if (latest.script_status === 'failed') {
          dispatch({ type: 'operation-failed', stage: 'script', message: '脚本生成失败，请调整要求后重试。' })
        }
      } catch (cause) {
        if (!active) return
        dispatch({ type: 'operation-failed', stage: 'script', message: cause instanceof Error ? cause.message : '脚本生成状态读取失败。' })
      }
    }
    const timer = window.setInterval(() => { void refresh() }, 1200)
    return () => {
      active = false
      window.clearInterval(timer)
    }
  }, [projectId, state.workspace?.script_status, state.workspace?.workspace_id])

  useEffect(() => {
    const workspace = state.workspace
    if (!workspace || workspace.storyboard_status !== 'generating') return
    let active = true
    const refresh = async () => {
      try {
        const latest = await getRequirementWorkspace(projectId, workspace.workspace_id)
        if (!active) return
        dispatch({ type: 'requirement-loaded', workspace: latest })
        if (latest.storyboard_status === 'failed') dispatch({ type: 'operation-failed', stage: 'storyboard', message: latest.storyboard_error_message || '故事板或参考图片生成失败，请重试。' })
      } catch (cause) {
        if (active) dispatch({ type: 'operation-failed', stage: 'storyboard', message: cause instanceof Error ? cause.message : '故事板生成状态读取失败。' })
      }
    }
    const timer = window.setInterval(() => { void refresh() }, 1500)
    return () => { active = false; window.clearInterval(timer) }
  }, [projectId, state.workspace?.storyboard_status, state.workspace?.workspace_id])

  useEffect(() => {
    const workspace = state.workspace
    if (!workspace || (workspace.production_status !== 'running' && workspace.production_status !== 'rendering')) return
    let active = true
    const refresh = async () => {
      try {
        const latest = await getRequirementWorkspace(projectId, workspace.workspace_id)
        if (!active) return
        dispatch({ type: 'requirement-loaded', workspace: latest })
        if (latest.production_status === 'failed' || latest.production_status === 'render_failed') dispatch({ type: 'operation-failed', stage: 'video', message: productionFailureMessage(latest) })
      } catch (cause) {
        if (active) dispatch({ type: 'operation-failed', stage: 'video', message: cause instanceof Error ? cause.message : '视频生产状态读取失败。' })
      }
    }
    const timer = window.setInterval(() => { void refresh() }, 1800)
    return () => { active = false; window.clearInterval(timer) }
  }, [projectId, state.workspace?.production_status, state.workspace?.workspace_id])

  useEffect(() => {
    const workspace = state.workspace
    const output = workspace?.production_plan?.render?.output_asset_ref
    if (workspace?.production_status !== 'completed' || !output || state.video?.output_url) return
    let active = true
    void getAssetPreview(projectId, output).then(url => {
      if (active && state.video) dispatch({ type: 'video-progressed', video: { ...state.video, status: 'completed', progress: 100, eta_seconds: 0, output_url: url } })
    }).catch(cause => {
      if (active) dispatch({ type: 'operation-failed', stage: 'video', message: cause instanceof Error ? cause.message : '最终视频预览地址读取失败。' })
    })
    return () => { active = false }
  }, [projectId, state.workspace?.production_plan?.render?.output_asset_ref?.asset_id, state.workspace?.production_plan?.render?.output_asset_ref?.version, state.workspace?.production_status, state.video])

  const analyze = async () => {
    dispatch({ type: 'operation-started', stage: 'requirement' })
    try {
      const workspace = await analyzeRequirement(projectId, productLink.trim(), supplementalRequirement.trim())
      dispatch({ type: 'requirement-loaded', workspace })
	  window.history.replaceState(null, '', `${window.location.pathname}${aiNativeWorkspaceLocation(window.location.search, workspace.workspace_id, 'requirement')}${window.location.hash}`)
      onNotice('商品信息和生成需求已完成分析，请核对并编辑结果。')
    } catch (cause) {
      dispatch({ type: 'operation-failed', stage: 'requirement', message: cause instanceof Error ? cause.message : '商品链接分析失败，请稍后重试。' })
    }
  }

  const editLocalRequirement = (workspace: AINativeRequirementWorkspace) => {
    rememberAINativeStageDraft(projectId, workspace.workspace_id, 'requirement', workspace.requirement.revision, workspace.requirement)
    autosave.schedule('requirement', workspace.requirement)
    dispatch({ type: 'requirement-edited', workspace })
  }

  const saveRequirement = async () => {
    if (!state.workspace) return
	if (state.workspace.status === 'confirmed') return
    autosave.schedule('requirement', state.workspace.requirement)
    const result = await autosave.flush('requirement')
    if (result.succeeded && result.workspace) onNotice(`需求修改已保存为 revision ${result.workspace.current_revision}。`)
  }

  const startScriptGeneration = async (workspace: AINativeRequirementWorkspace) => {
    dispatch({ type: 'requirement-confirmed', workspace })
    const generating = await generateScript(projectId, workspace.workspace_id, workspace.workspace_version)
    dispatch({ type: 'requirement-loaded', workspace: generating })
    dispatch({ type: 'open-stage', stage: 'script' })
    window.history.replaceState(null, '', `${window.location.pathname}${aiNativeWorkspaceLocation(window.location.search, workspace.workspace_id, 'script')}${window.location.hash}`)
    onNotice('完整营销脚本生成任务已启动，离开页面后也可以恢复。')
  }

  const confirmRequirementStage = async () => {
    if (!state.workspace) return
    if (state.workspace.status === 'confirmed') {
      if (!state.workspace.script_status || state.workspace.script_status === 'failed') {
        try {
          await startScriptGeneration(state.workspace)
        } catch (cause) {
          dispatch({ type: 'operation-failed', stage: 'script', message: cause instanceof Error ? cause.message : '脚本生成启动失败。' })
        }
      } else {
        dispatch({ type: 'open-stage', stage: 'script' })
      }
      return
    }
    dispatch({ type: 'operation-started', stage: 'requirement' })
    try {
      autosave.schedule('requirement', state.workspace.requirement)
      const saved = await autosave.flush('requirement')
      if (!saved.succeeded || !saved.workspace) throw new Error('需求自动保存失败，请重试后再进入下一步。')
      const workspace = await confirmRequirement(projectId, saved.workspace.workspace_id, saved.workspace.requirement.revision)
      await startScriptGeneration(workspace)
    } catch (cause) {
      dispatch({ type: 'operation-failed', stage: 'requirement', message: cause instanceof Error ? cause.message : '需求确认失败。' })
    }
  }

  const regenerateScript = async () => {
    if (!state.workspace || !state.script) return
    dispatch({ type: 'operation-started', stage: 'script' })
    try {
      const workspace = await regenerateScriptRequest(projectId, state.workspace.workspace_id, state.workspace.workspace_version, state.script.regeneration_note ?? '')
      dispatch({ type: 'requirement-loaded', workspace })
      onNotice('整版脚本重新生成任务已启动，旧 revision 会保留。')
    } catch (cause) {
      dispatch({ type: 'operation-failed', stage: 'script', message: cause instanceof Error ? cause.message : '脚本重新生成失败。' })
    }
  }

  const startStoryboardGeneration = async (workspace: AINativeRequirementWorkspace) => {
    dispatch({ type: 'operation-started', stage: 'storyboard' })
    window.history.replaceState(null, '', `${window.location.pathname}${aiNativeWorkspaceLocation(window.location.search, workspace.workspace_id, 'storyboard')}${window.location.hash}`)
    try {
      const generating = await generateStoryboard(projectId, workspace.workspace_id, workspace.workspace_version)
      dispatch({ type: 'requirement-loaded', workspace: generating })
      onNotice('真实故事板与参考图片生成任务已启动。')
    } catch (cause) {
      dispatch({ type: 'operation-failed', stage: 'storyboard', message: cause instanceof Error ? cause.message : '故事板生成启动失败，请重试。' })
    }
  }

  const confirmScriptStage = async () => {
    if (!state.script || !state.workspace) return
    dispatch({ type: 'operation-started', stage: 'script' })
    let confirmedWorkspace: AINativeRequirementWorkspace
    try {
      autosave.schedule('script', state.script)
      const saved = await autosave.flush('script')
      const updated = saved.workspace
      if (!saved.succeeded || !updated) throw new Error('脚本自动保存失败，请重试后再进入下一步。')
      if (!updated.script) throw new Error('脚本保存后未返回当前 revision。')
      confirmedWorkspace = await confirmScript(projectId, updated.workspace_id, updated.script.revision, updated.workspace_version)
      dispatch({ type: 'requirement-loaded', workspace: confirmedWorkspace })
      dispatch({ type: 'script-confirmed' })
    } catch (cause) {
      dispatch({ type: 'operation-failed', stage: 'script', message: cause instanceof Error ? cause.message : '脚本确认失败。' })
      return
    }
    await startStoryboardGeneration(confirmedWorkspace)
  }

  const retryStoryboardGeneration = async () => {
    if (!state.workspace || state.workspace.script_status !== 'confirmed') return
    await startStoryboardGeneration(state.workspace)
  }

  const saveStoryboard = async () => {
    if (!state.storyboard || !state.workspace) return null
    autosave.schedule('storyboard', state.storyboard)
    const saved = await autosave.flush('storyboard')
    if (!saved.succeeded || !saved.workspace) return null
    onNotice(`故事板修改已保存为 revision ${saved.workspace.current_storyboard_revision}。`)
    return saved.workspace
  }

  const confirmStoryboardStage = async () => {
    const updated = await saveStoryboard()
    if (!updated?.storyboard || !updated.current_storyboard_revision) return
    try {
      const workspace = await confirmStoryboard(projectId, updated.workspace_id, updated.current_storyboard_revision, updated.workspace_version)
	  const producing = await startProduction(projectId, workspace.workspace_id, workspace.workspace_version)
      dispatch({ type: 'requirement-loaded', workspace: producing })
	  dispatch({ type: 'open-stage', stage: 'video' })
      onNotice('故事板已冻结，视频片段与旁白生产任务已启动；离开页面后仍可恢复。')
    } catch (cause) {
      dispatch({ type: 'operation-failed', stage: 'storyboard', message: cause instanceof Error ? cause.message : '故事板确认失败。' })
    }
  }

  const retryVideo = async () => {
    if (!state.workspace?.production_plan) return
	const failed = state.workspace.production_plan.units.find(unit => unit.attempts.at(-1)?.status === 'failed')
	  ?? state.workspace.production_plan.speech_units.find(unit => unit.attempts.at(-1)?.status === 'failed')
	  ?? (state.workspace.production_status === 'render_failed' && state.workspace.production_plan.render ? { id: state.workspace.production_plan.render.id } : undefined)
	if (!failed) return
    dispatch({ type: 'operation-started', stage: 'video' })
	try {
	  const workspace = await retryProductionUnit(projectId, state.workspace.workspace_id, failed.id, state.workspace.workspace_version)
	  dispatch({ type: 'requirement-loaded', workspace })
	  onNotice(`已只重试失败片段 ${failed.id}，其他成功片段保持不变。`)
	} catch (cause) {
	  dispatch({ type: 'operation-failed', stage: 'video', message: cause instanceof Error ? cause.message : '片段重试失败。' })
	}
  }

  const cancelVideo = async () => {
	if (!state.workspace || (state.workspace.production_status !== 'running' && state.workspace.production_status !== 'rendering')) return
	try {
	  const workspace = await cancelProduction(projectId, state.workspace.workspace_id, state.workspace.workspace_version)
	  dispatch({ type: 'requirement-loaded', workspace })
	  onNotice('已请求取消视频生产，已完成素材仍会保留。')
	} catch (cause) {
	  dispatch({ type: 'operation-failed', stage: 'video', message: cause instanceof Error ? cause.message : '取消生产失败。' })
	}
  }

  const requestEdit = async (stage: AINativeStageId) => {
	if ((stage === 'requirement' || stage === 'script' || stage === 'storyboard') && state.workspace) {
	  try {
		await getRequirementReopenImpact(projectId, state.workspace.workspace_id, stage)
	  } catch (cause) {
		dispatch({ type: 'operation-failed', stage, message: cause instanceof Error ? cause.message : '无法读取重新编辑影响。' })
		return
	  }
	}
	dispatch({ type: 'reopen-requested', stage })
  }

  const confirmReopen = async () => {
	const stage = state.pending_reopen
	if (!stage) return
	if ((stage !== 'requirement' && stage !== 'script' && stage !== 'storyboard') || !state.workspace) {
	  dispatch({ type: 'reopen-confirmed' })
	  return
	}
	try {
	  if (stage === 'storyboard') {
	    const workspace = await reopenStoryboard(projectId, state.workspace.workspace_id, state.workspace.workspace_version)
	    dispatch({ type: 'requirement-loaded', workspace })
	    dispatch({ type: 'reopen-confirmed' })
	    onNotice(`故事板已重新打开为 revision ${workspace.current_storyboard_revision}。`)
	  } else if (stage === 'script') {
	    const workspace = await reopenScript(projectId, state.workspace.workspace_id, state.workspace.workspace_version)
	    dispatch({ type: 'requirement-loaded', workspace })
	    dispatch({ type: 'reopen-confirmed' })
	    window.history.replaceState(null, '', `${window.location.pathname}${aiNativeWorkspaceLocation(window.location.search, workspace.workspace_id, 'script')}${window.location.hash}`)
	    onNotice(`脚本已重新打开为 revision ${workspace.current_script_revision}，旧版本已保留并作废。`)
	  } else {
	    const workspace = await reopenRequirement(projectId, state.workspace.workspace_id, state.workspace.workspace_version)
	    dispatch({ type: 'requirement-reopened', workspace })
	    window.history.replaceState(null, '', `${window.location.pathname}${aiNativeWorkspaceLocation(window.location.search, workspace.workspace_id, 'requirement')}${window.location.hash}`)
	    onNotice(`需求已重新打开为 revision ${workspace.current_revision}，旧版本已保留并作废。`)
	  }
	} catch (cause) {
	  dispatch({ type: 'operation-failed', stage, message: cause instanceof Error ? cause.message : '重新编辑需求失败。' })
	}
  }

  const autosaveCopy = autosave.status === 'saving' || autosave.status === 'waiting'
    ? '正在自动保存…'
    : autosave.status === 'failed' ? '自动保存失败，本地草稿已保留'
    : autosave.savedAt ? `已自动保存 ${autosave.savedAt.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })}` : ''

  return <section className="ai-native-workspace" aria-label="AI 效果广告生成工作区">
    <header className="ai-native-workspace-header"><div><h3>AI 效果广告生成</h3><p>从商品链接开始，依次完成需求、脚本、故事板和完整视频。</p></div><div><span>实际支持</span><b>抖音 · 9:16 · 15–30 秒</b>{autosaveCopy ? <small role="status">{autosaveCopy}</small> : null}</div></header>
    <AINativeStageStepper active={state.active_stage} status={state.stage_status} onSelect={stage => {
      dispatch({ type: 'open-stage', stage })
      if (state.workspace) window.history.replaceState(null, '', `${window.location.pathname}${aiNativeWorkspaceLocation(window.location.search, state.workspace.workspace_id, stage)}${window.location.hash}`)
    }}/>
    {state.active_stage === 'requirement' ? <RequirementStage
      projectId={projectId}
      status={state.stage_status.requirement}
      productLink={productLink}
      supplementalRequirement={supplementalRequirement}
      requirement={state.workspace?.requirement ?? null}
      error={state.error}
      onProductLinkChange={setProductLink}
      onSupplementalRequirementChange={setSupplementalRequirement}
      onAnalyze={() => { void analyze() }}
      onReanalyze={() => { void analyze() }}
      onChange={requirement => state.workspace && editLocalRequirement({ ...state.workspace, requirement })}
      onSave={() => { void saveRequirement() }}
      onConfirm={() => { void confirmRequirementStage() }}
	  onEdit={() => { void requestEdit('requirement') }}
    /> : state.active_stage === 'script' ? <ScriptStage
      status={state.stage_status.script}
      script={state.script}
      onChange={script => {
        if (state.workspace) rememberAINativeStageDraft(projectId, state.workspace.workspace_id, 'script', script.revision, script)
        autosave.schedule('script', script)
        dispatch({ type: 'script-edited', script })
      }}
      onRegenerate={() => { void regenerateScript() }}
      onConfirm={() => { void confirmScriptStage() }}
	  onEdit={() => { void requestEdit('script') }}
    /> : state.active_stage === 'storyboard' ? <StoryboardStage
      projectId={projectId}
      status={state.stage_status.storyboard}
      storyboard={state.storyboard}
      canGenerate={state.workspace?.script_status === 'confirmed'}
      error={state.error}
      onChange={storyboard => {
        if (state.workspace) rememberAINativeStageDraft(projectId, state.workspace.workspace_id, 'storyboard', storyboard.revision, storyboard)
        autosave.schedule('storyboard', storyboard)
        dispatch({ type: 'storyboard-edited', storyboard })
      }}
      onSave={() => { void saveStoryboard() }}
      onConfirm={() => { void confirmStoryboardStage() }}
	  onEdit={() => { void requestEdit('storyboard') }}
      onRetry={() => { void retryStoryboardGeneration() }}
    /> : <VideoStage
      status={state.stage_status.video}
      video={state.video}
	  onRetry={() => { void retryVideo() }}
	  onCancel={() => { void cancelVideo() }}
    />}
	{state.pending_reopen ? <div className="ai-native-dialog-backdrop" role="presentation"><section className="ai-native-dialog" role="alertdialog" aria-modal="true" aria-labelledby="ai-native-reopen-title"><span><AlertTriangle size={18}/></span><h3 id="ai-native-reopen-title">确认重新编辑？</h3><p>{invalidationCopy[state.pending_reopen]}</p><div><button className="secondary-button" onClick={() => dispatch({ type: 'reopen-cancelled' })}>取消</button><button className="primary-button" onClick={() => { void confirmReopen() }}>继续编辑并作废下游</button></div></section></div> : null}
  </section>
}
