import type {
  AdScriptDraft,
  AINativeFrontendState,
  AINativeRequirementWorkspace,
  AINativeStageId,
  AINativeStageStatus,
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
  return { status: workspace.production_status, progress: progress.progress_percent, current_step: progress.current_step, completed_shots: progress.completed_video_units, total_shots: progress.total_video_units, eta_seconds: progress.eta_seconds }
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
        error: '',
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
