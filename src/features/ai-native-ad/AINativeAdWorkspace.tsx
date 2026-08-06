import { useCallback, useEffect, useReducer, useState } from 'react'
import { AlertTriangle } from 'lucide-react'
import { analyzeRequirement, cancelProduction, confirmRequirement, confirmScript, confirmStoryboard, fitStoryboardVoiceover, generateScript, generateStoryboard, getAssetPreview, getLatestRequirementWorkspace, getRequirementReopenImpact, getRequirementWorkspace, listAINativeAdWorkspaces, regenerateScript as regenerateScriptRequest, regenerateStoryboardAsset, renameAINativeAdWorkspace, reopenRequirement, reopenScript, reopenStoryboard, retryProductionUnit, startProduction, updateStoryboard } from './api'
import { aiNativeReducer, initialAINativeState, productionFailureMessage, productionReferenceFailure } from './reducer'
import { AINativeStageStepper } from './AINativeStageStepper'
import { RequirementStage } from './RequirementStage'
import { ScriptStage } from './ScriptStage'
import { StoryboardStage } from './StoryboardStage'
import { VideoStage } from './VideoStage'
import type { AdScriptDraft, AINativeAdWorkspaceSummary, AINativeRequirement, AINativeRequirementWorkspace, AINativeStageId, StoryboardDraft } from './types'
import { aiNativeWorkspaceLocation, clearAINativeWorkspaceLocation, readAINativeWorkspaceLocation } from './navigation'
import { forgetAINativeWorkspace, readAINativeStageDraft, readAINativeWorkspacePointer, rememberAINativeStageDraft, rememberAINativeWorkspace } from './storage'
import { useAINativeAutosave } from './useAINativeAutosave'
import { AINativeAdCatalog } from './AINativeAdCatalog'
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
  const [records, setRecords] = useState<AINativeAdWorkspaceSummary[]>([])
  const [catalogBusy, setCatalogBusy] = useState(false)
  const [newDialogOpen, setNewDialogOpen] = useState(false)
  const [recordName, setRecordName] = useState('')
  const [newDialogError, setNewDialogError] = useState('')
  const [pendingStoryboardAssetId, setPendingStoryboardAssetId] = useState('')
  const [voiceoverFitBusy, setVoiceoverFitBusy] = useState(false)
  const autosave = useAINativeAutosave(projectId, state, dispatch)

  const refreshCatalog = useCallback(async () => {
    const value = await listAINativeAdWorkspaces(projectId)
    setRecords(value)
    return value
  }, [projectId])

  useEffect(() => {
	let active = true
    dispatch({ type: 'reset' })
    setProductLink('')
    setSupplementalRequirement('')
	const restore = async () => {
	  const recordsPromise = listAINativeAdWorkspaces(projectId).catch(() => [] as AINativeAdWorkspaceSummary[])
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
	  const restoredRecords = await recordsPromise
	  if (!active) return
	  setRecords(restoredRecords)
	  if (!workspace) return
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
    const timeout = window.setTimeout(() => {
      if (!active) return
      active = false
      window.clearInterval(timer)
      dispatch({ type: 'operation-failed', stage: 'script', message: '脚本生成已超过 3 分钟，任务可能停滞。请重新生成脚本。' })
    }, 180_000)
    return () => {
      active = false
      window.clearInterval(timer)
      window.clearTimeout(timeout)
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
	  await refreshCatalog()
	  window.history.replaceState(null, '', `${window.location.pathname}${aiNativeWorkspaceLocation(window.location.search, workspace.workspace_id, 'requirement')}${window.location.hash}`)
      onNotice('商品信息和生成需求已完成分析，请核对并编辑结果。')
    } catch (cause) {
      dispatch({ type: 'operation-failed', stage: 'requirement', message: cause instanceof Error ? cause.message : '商品链接分析失败，请稍后重试。' })
    }
  }

  const switchWorkspace = async (workspaceId: string) => {
    if (!workspaceId || workspaceId === state.workspace?.workspace_id) return
    setCatalogBusy(true)
    try {
      const saved = await autosave.flushAll()
      if (!saved.succeeded) throw new Error('当前广告自动保存失败，暂时不能切换记录。')
      const workspace = await getRequirementWorkspace(projectId, workspaceId)
      const stage: AINativeStageId = workspace.current_stage === 'production' ? 'video' : workspace.current_stage
      dispatch({ type: 'reset' })
      dispatch({ type: 'requirement-loaded', workspace })
      dispatch({ type: 'open-stage', stage })
      setProductLink(workspace.requirement.product.source_url)
      setSupplementalRequirement(workspace.requirement.supplemental_requirement)
      rememberAINativeWorkspace(projectId, { workspaceId, stage })
      window.history.replaceState(null, '', `${window.location.pathname}${aiNativeWorkspaceLocation(window.location.search, workspaceId, stage)}${window.location.hash}`)
      onNotice(`已切换到「${workspace.display_name || workspace.requirement.product_name}」。`)
    } catch (cause) {
      dispatch({ type: 'operation-failed', stage: state.active_stage, message: cause instanceof Error ? cause.message : '广告记录切换失败。' })
    } finally {
      setCatalogBusy(false)
    }
  }

  const suggestedRecordName = () => {
    const workspace = state.workspace
    if (!workspace) return ''
    if (workspace.display_name.trim()) return workspace.display_name
    const date = new Date(workspace.created_at).toLocaleDateString('zh-CN', { month: '2-digit', day: '2-digit' }).replaceAll('/', '-')
    return `${workspace.requirement.product_name || 'AI 效果广告'} ${date}`.slice(0, 80)
  }

  const beginNewWorkspace = () => {
    dispatch({ type: 'reset' })
    setProductLink('')
    setSupplementalRequirement('')
    forgetAINativeWorkspace(projectId)
    window.history.replaceState(null, '', `${window.location.pathname}${clearAINativeWorkspaceLocation(window.location.search)}${window.location.hash}`)
  }

  const requestNewWorkspace = () => {
    if (!state.workspace) {
      beginNewWorkspace()
      return
    }
    setRecordName(suggestedRecordName())
    setNewDialogError('')
    setNewDialogOpen(true)
  }

  const confirmNewWorkspace = async () => {
    if (!state.workspace) {
      setNewDialogOpen(false)
      beginNewWorkspace()
      return
    }
    const name = recordName.trim()
    if (!name) {
      setNewDialogError('请先为当前广告填写名称。')
      return
    }
    if ([...name].length > 80) {
      setNewDialogError('广告名称不能超过 80 个字符。')
      return
    }
    setCatalogBusy(true)
    setNewDialogError('')
    try {
      const saved = await autosave.flushAll()
      if (!saved.succeeded) throw new Error('当前广告自动保存失败，请重试后再新建。')
      await renameAINativeAdWorkspace(projectId, state.workspace.workspace_id, name)
      await refreshCatalog()
      setNewDialogOpen(false)
      beginNewWorkspace()
      onNotice(`「${name}」已保存，现在可以创建新的广告。`)
    } catch (cause) {
      setNewDialogError(cause instanceof Error ? cause.message : '当前广告保存失败。')
    } finally {
      setCatalogBusy(false)
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

  const retryScriptGeneration = async () => {
    if (!state.workspace) return
    dispatch({ type: 'operation-started', stage: 'script' })
    try {
      const latest = await getRequirementWorkspace(projectId, state.workspace.workspace_id)
      if (latest.script_status === 'generating') {
        dispatch({ type: 'operation-failed', stage: 'script', message: '后台任务仍处于生成中，请稍后刷新状态；若持续超过 3 分钟，请联系管理员处理停滞任务。' })
        return
      }
      if (latest.script_status === 'draft' || latest.script_status === 'confirmed') {
        dispatch({ type: 'requirement-loaded', workspace: latest })
        return
      }
      await startScriptGeneration(latest)
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

  const startStoryboardAssetRegeneration = async (workspace: AINativeRequirementWorkspace, assetId: string) => {
    dispatch({ type: 'operation-started', stage: 'storyboard' })
    try {
      const generating = await regenerateStoryboardAsset(projectId, workspace.workspace_id, assetId, workspace.workspace_version)
      dispatch({ type: 'requirement-loaded', workspace: generating })
      dispatch({ type: 'open-stage', stage: 'storyboard' })
      onNotice(`故事板素材 ${assetId} 已开始重新生成，其他素材保持不变。`)
    } catch (cause) {
      dispatch({ type: 'operation-failed', stage: 'storyboard', message: cause instanceof Error ? cause.message : '故事板素材重新生成失败。' })
    }
  }

  const requestStoryboardAssetRegeneration = async (assetId: string) => {
    if (!state.workspace) return
    if (state.workspace.storyboard_status === 'confirmed') {
      setPendingStoryboardAssetId(assetId)
      await requestEdit('storyboard')
      return
    }
    await startStoryboardAssetRegeneration(state.workspace, assetId)
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

  const fitFailedVoiceover = async () => {
    const current = state.workspace
    const speechUnitId = state.video?.failure_code === 'SPEECH_DURATION_EXCEEDED' ? state.video.failed_unit_id : ''
    if (!current || !speechUnitId) return
    setVoiceoverFitBusy(true)
    try {
      const suggestion = await fitStoryboardVoiceover(projectId, current.workspace_id, speechUnitId, current.workspace_version)
      const reopened = await reopenStoryboard(projectId, current.workspace_id, current.workspace_version)
      if (!reopened.storyboard) throw new Error('故事板重新打开后未返回可编辑版本。')
      const adjusted: StoryboardDraft = {
        ...reopened.storyboard,
        shots: reopened.storyboard.shots.map(shot => shot.id === suggestion.shot_id ? { ...shot, voiceover: suggestion.suggested_voiceover } : shot),
      }
      if (!adjusted.shots.some(shot => shot.id === suggestion.shot_id)) throw new Error('未找到需要压缩旁白的对应分镜。')
      const saved = await updateStoryboard(projectId, reopened.workspace_id, adjusted)
      dispatch({ type: 'requirement-loaded', workspace: saved })
      dispatch({ type: 'open-stage', stage: 'storyboard' })
      window.history.replaceState(null, '', `${window.location.pathname}${aiNativeWorkspaceLocation(window.location.search, saved.workspace_id, 'storyboard')}${window.location.hash}`)
      onNotice(`已将 ${suggestion.shot_id} 旁白压缩到 ${suggestion.max_characters} 字以内，请确认后重新一键成片。`)
    } catch (cause) {
      dispatch({ type: 'operation-failed', stage: 'video', message: cause instanceof Error ? cause.message : '智能压缩旁白失败。' })
    } finally {
      setVoiceoverFitBusy(false)
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
	    if (pendingStoryboardAssetId) {
	      const assetId = pendingStoryboardAssetId
	      setPendingStoryboardAssetId('')
	      await startStoryboardAssetRegeneration(workspace, assetId)
	    } else {
	      onNotice(`故事板已重新打开为 revision ${workspace.current_storyboard_revision}。`)
	    }
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
	  setPendingStoryboardAssetId('')
	  dispatch({ type: 'operation-failed', stage, message: cause instanceof Error ? cause.message : '重新编辑需求失败。' })
	}
  }

  const autosaveCopy = autosave.status === 'saving' || autosave.status === 'waiting'
    ? '正在自动保存…'
    : autosave.status === 'failed' ? '自动保存失败，本地草稿已保留'
    : autosave.savedAt ? `已自动保存 ${autosave.savedAt.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })}` : ''

  const referenceFailure = state.workspace ? productionReferenceFailure(state.workspace) : null

  return <section className="ai-native-workspace" aria-label="AI 效果广告生成工作区">
    <header className="ai-native-workspace-header"><div><h3>AI 效果广告生成</h3><p>从商品链接开始，依次完成需求、脚本、故事板和完整视频。</p></div><div className="ai-native-workspace-header-side"><AINativeAdCatalog records={records} currentId={state.workspace?.workspace_id ?? ''} busy={catalogBusy} onSelect={workspaceId => { void switchWorkspace(workspaceId) }} onNew={requestNewWorkspace}/><div className="ai-native-support-copy"><span>实际支持</span><b>抖音 · 9:16 · 15–30 秒</b>{autosaveCopy ? <small role="status">{autosaveCopy}</small> : null}</div></div></header>
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
      error={state.workspace?.script_error_message || state.error}
      onChange={script => {
        if (state.workspace) rememberAINativeStageDraft(projectId, state.workspace.workspace_id, 'script', script.revision, script)
        autosave.schedule('script', script)
        dispatch({ type: 'script-edited', script })
      }}
      onRegenerate={() => { void regenerateScript() }}
      onRetry={() => { void retryScriptGeneration() }}
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
      onRegenerateAsset={assetId => { void requestStoryboardAssetRegeneration(assetId) }}
      onReplaceSourceAsset={() => { void requestEdit('requirement') }}
    /> : <VideoStage
      status={state.stage_status.video}
      video={state.video}
	  referenceFailure={referenceFailure}
	  onRetry={() => { void retryVideo() }}
	  onFitVoiceover={() => { void fitFailedVoiceover() }}
	  voiceoverFitBusy={voiceoverFitBusy}
	  onCancel={() => { void cancelVideo() }}
	  onReviewReference={() => {
	    const stage = referenceFailure?.asset_source === 'ai_generated' ? 'storyboard' : 'requirement'
	    dispatch({ type: 'open-stage', stage })
	    if (state.workspace) window.history.replaceState(null, '', `${window.location.pathname}${aiNativeWorkspaceLocation(window.location.search, state.workspace.workspace_id, stage)}${window.location.hash}`)
	  }}
    />}
	{state.pending_reopen ? <div className="ai-native-dialog-backdrop" role="presentation"><section className="ai-native-dialog" role="alertdialog" aria-modal="true" aria-labelledby="ai-native-reopen-title"><span><AlertTriangle size={18}/></span><h3 id="ai-native-reopen-title">确认重新编辑？</h3><p>{pendingStoryboardAssetId ? '重新生成这张参考图片需要先解冻故事板，当前视频结果会作废；确认后系统会自动开始重新生成所选图片，其他故事板素材保持不变。' : invalidationCopy[state.pending_reopen]}</p><div><button className="secondary-button" onClick={() => { setPendingStoryboardAssetId(''); dispatch({ type: 'reopen-cancelled' }) }}>取消</button><button className="primary-button" onClick={() => { void confirmReopen() }}>{pendingStoryboardAssetId ? '作废视频并重新生成图片' : '继续编辑并作废下游'}</button></div></section></div> : null}
	{newDialogOpen ? <div className="ai-native-dialog-backdrop" role="presentation"><section className="ai-native-dialog ai-native-name-dialog" role="dialog" aria-modal="true" aria-labelledby="ai-native-name-title"><h3 id="ai-native-name-title">保存当前广告并新建</h3><p>当前完整或未完成的生成进度都会保留。请先为它起一个方便查找的名称。</p><label>广告名称<input autoFocus maxLength={80} value={recordName} onChange={event => setRecordName(event.target.value)} placeholder="例如：施美乐钛杯通勤版"/></label>{newDialogError ? <div className="ai-native-error" role="alert">{newDialogError}</div> : null}<div><button className="secondary-button" disabled={catalogBusy} onClick={() => setNewDialogOpen(false)}>取消</button><button className="primary-button" disabled={catalogBusy || !recordName.trim()} onClick={() => { void confirmNewWorkspace() }}>{catalogBusy ? '正在保存…' : '保存并新建'}</button></div></section></div> : null}
  </section>
}
