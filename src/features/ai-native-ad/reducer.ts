import type {
  AdScriptDraft,
  AINativeFrontendState,
  AINativeRequirementWorkspace,
  AINativeStageId,
  AINativeStageStatus,
  ProductionReferenceFailure,
  StoryboardDraft,
  VideoRenderState,
} from './types'

export type AINativeAction =
  | { type: 'reset' }
  | { type: 'open-stage'; stage: AINativeStageId }
  | { type: 'operation-started'; stage: AINativeStageId }
  | { type: 'operation-failed'; stage: AINativeStageId; message: string }
  | { type: 'requirement-loaded'; workspace: AINativeRequirementWorkspace }
  | { type: 'requirement-edited'; workspace: AINativeRequirementWorkspace }
  | { type: 'requirement-confirmed'; workspace: AINativeRequirementWorkspace }
  | { type: 'requirement-reopened'; workspace: AINativeRequirementWorkspace }
  | { type: 'script-generated'; script: AdScriptDraft }
  | { type: 'script-edited'; script: AdScriptDraft }
  | { type: 'script-confirmed' }
  | { type: 'storyboard-generated'; storyboard: StoryboardDraft }
  | { type: 'storyboard-edited'; storyboard: StoryboardDraft }
  | { type: 'storyboard-confirmed'; video: VideoRenderState }
  | { type: 'video-progressed'; video: VideoRenderState }
  | { type: 'reopen-requested'; stage: AINativeStageId }
  | { type: 'reopen-cancelled' }
  | { type: 'reopen-confirmed' }

export const initialAINativeState: AINativeFrontendState = {
  active_stage: 'requirement',
  stage_status: {
    requirement: 'empty',
    script: 'empty',
    storyboard: 'empty',
    video: 'empty',
  },
  workspace: null,
  script: null,
  storyboard: null,
  video: null,
  pending_reopen: null,
  error: '',
}

const downstream: Record<AINativeStageId, AINativeStageId[]> = {
  requirement: ['script', 'storyboard', 'video'],
  script: ['storyboard', 'video'],
  storyboard: ['video'],
  video: [],
}

function videoFromWorkspace(workspace: AINativeRequirementWorkspace): VideoRenderState | null {
  const progress = workspace.production_progress
  if (!workspace.production_status || !progress) return null
  const failedVideo = workspace.production_plan?.units?.find(unit => unit.attempts.at(-1)?.status === 'failed')
  const failedSpeech = workspace.production_plan?.speech_units?.find(unit => unit.attempts.at(-1)?.status === 'failed')
  const failedUnit = failedVideo ?? failedSpeech
  const failedAttempt = failedUnit?.attempts.at(-1)
  return {
    status: workspace.production_status,
    progress: progress.progress_percent,
    current_step: progress.current_step,
    completed_shots: progress.completed_video_units,
    total_shots: progress.total_video_units,
    completed_speech_units: progress.completed_speech_units,
    total_speech_units: progress.total_speech_units,
    failed_unit_id: failedUnit?.id,
    failed_shot_id: failedVideo?.shot_ids[0] ?? failedSpeech?.shot_id,
    failure_code: failedAttempt?.error_code,
    failure_reason: failedAttempt?.error_message,
    eta_seconds: progress.eta_seconds,
  }
}

function videoStatusFromWorkspace(workspace: AINativeRequirementWorkspace, current: AINativeStageStatus): AINativeStageStatus {
  switch (workspace.production_status) {
    case 'running':
    case 'rendering': return 'generating'
    case 'assets_ready': return 'draft'
    case 'completed': return 'confirmed'
    case 'render_failed':
    case 'failed':
    case 'cancelled': return 'failed'
    default: return current
  }
}

export function productionFailureMessage(workspace: AINativeRequirementWorkspace): string {
  const failedVideo = workspace.production_plan?.units.find(unit => unit.attempts.at(-1)?.status === 'failed')
  const failedSpeech = workspace.production_plan?.speech_units.find(unit => unit.attempts.at(-1)?.status === 'failed')
  const attempt = failedVideo?.attempts.at(-1) ?? failedSpeech?.attempts.at(-1)
  if (attempt?.error_message) return attempt.error_message
  if (workspace.production_status === 'render_failed') return workspace.production_plan?.render?.error_message || '最终视频渲染失败，已生成的片段与旁白仍会保留。'
  return '部分视频片段或旁白生成失败，可只重试失败片段。'
}

export function productionReferenceFailure(workspace: AINativeRequirementWorkspace): ProductionReferenceFailure | null {
  const unit = workspace.production_plan?.units.find(candidate => candidate.attempts.at(-1)?.status === 'failed' && candidate.reference_asset)
  const attempt = unit?.attempts.at(-1)
  if (!unit?.reference_asset || !attempt) return null
  const errorCode = attempt.error_code ?? ''
  const errorMessage = attempt.error_message ?? ''
  const privacyRejected = errorCode.startsWith('InputImageSensitiveContentDetected') || /input image.+real person/i.test(errorMessage)
  const copyrightRejected = /copyright restrictions/i.test(errorMessage)
  if (!privacyRejected && !copyrightRejected) return null
  const asset = (workspace.storyboard ?? workspace.storyboard_plan)?.assets.find(candidate => candidate.asset_ref?.asset_id === unit.reference_asset?.asset_id)
  if (!asset) return null
  return {
    unit_id: unit.id,
    asset_id: asset.id,
    asset_name: asset.name,
    asset_source: asset.source,
    reason: privacyRejected
      ? '参考图片可能包含写实人物，视频模型因隐私保护拒绝使用。'
      : '参考图片包含受版权保护的品牌或角色形象，视频模型拒绝生成。',
  }
}

export function aiNativeReducer(state: AINativeFrontendState, action: AINativeAction): AINativeFrontendState {
  switch (action.type) {
    case 'reset':
      return initialAINativeState
    case 'open-stage':
      return { ...state, active_stage: action.stage, error: '' }
    case 'operation-started':
      return {
        ...state,
        active_stage: action.stage,
        stage_status: { ...state.stage_status, [action.stage]: 'generating' },
        error: '',
      }
    case 'operation-failed':
      return {
        ...state,
        stage_status: { ...state.stage_status, [action.stage]: 'failed' },
        error: action.message,
      }
    case 'requirement-loaded':
    case 'requirement-edited':
      return {
        ...state,
        workspace: action.workspace,
        script: action.workspace.script ?? state.script,
        storyboard: action.workspace.storyboard ?? action.workspace.storyboard_plan ?? state.storyboard,
        video: videoFromWorkspace(action.workspace) ?? state.video,
        stage_status: {
          ...state.stage_status,
          requirement: action.workspace.status === 'confirmed' && state.stage_status.requirement !== 'draft' ? 'confirmed' : 'draft',
          script: action.workspace.script_status ?? state.stage_status.script,
          storyboard: action.workspace.storyboard_status ?? state.stage_status.storyboard,
          video: videoStatusFromWorkspace(action.workspace, state.stage_status.video),
        },
        error: action.workspace.script_status === 'failed'
          ? action.workspace.script_error_message || '脚本生成失败，请重新生成。'
          : action.workspace.storyboard_status === 'failed'
            ? action.workspace.storyboard_error_message || '故事板或参考图片生成失败，请重试。'
            : action.workspace.production_status === 'failed' || action.workspace.production_status === 'render_failed'
              ? productionFailureMessage(action.workspace)
              : '',
      }
    case 'requirement-confirmed':
      return {
        ...state,
        active_stage: 'script',
        workspace: action.workspace,
        stage_status: { ...state.stage_status, requirement: 'confirmed', script: 'generating' },
        error: '',
      }
    case 'requirement-reopened':
      return {
        ...state,
        active_stage: 'requirement',
        workspace: action.workspace,
        stage_status: { requirement: 'draft', script: 'invalidated', storyboard: 'invalidated', video: 'invalidated' },
        script: null,
        storyboard: null,
        video: null,
        pending_reopen: null,
        error: '',
      }
    case 'script-generated':
      return {
        ...state,
        active_stage: 'script',
        script: action.script,
        stage_status: { ...state.stage_status, script: 'draft' },
        error: '',
      }
    case 'script-edited':
      return { ...state, script: action.script, error: '' }
    case 'script-confirmed':
      return {
        ...state,
        active_stage: 'storyboard',
        stage_status: { ...state.stage_status, script: 'confirmed', storyboard: 'generating' },
        error: '',
      }
    case 'storyboard-generated':
      return {
        ...state,
        active_stage: 'storyboard',
        storyboard: action.storyboard,
        stage_status: { ...state.stage_status, storyboard: 'draft' },
        error: '',
      }
    case 'storyboard-edited':
      return { ...state, storyboard: action.storyboard, error: '' }
    case 'storyboard-confirmed':
      return {
        ...state,
        active_stage: 'video',
        video: action.video,
        stage_status: { ...state.stage_status, storyboard: 'confirmed', video: 'generating' },
        error: '',
      }
    case 'video-progressed':
      return {
        ...state,
        video: action.video,
        stage_status: { ...state.stage_status, video: action.video.progress >= 100 ? 'confirmed' : 'generating' },
        error: '',
      }
    case 'reopen-requested':
      return { ...state, pending_reopen: action.stage }
    case 'reopen-cancelled':
      return { ...state, pending_reopen: null }
    case 'reopen-confirmed': {
      const stage = state.pending_reopen
      if (!stage) return state
      const nextStatus = { ...state.stage_status, [stage]: 'draft' as const }
      downstream[stage].forEach(item => { nextStatus[item] = 'invalidated' })
      return {
        ...state,
        active_stage: stage,
        stage_status: nextStatus,
        script: stage === 'requirement' ? null : state.script,
        storyboard: stage === 'requirement' || stage === 'script' ? null : state.storyboard,
        video: stage !== 'video' ? null : state.video,
        pending_reopen: null,
        error: '',
      }
    }
  }
}
