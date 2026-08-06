import type { FirstFrameCandidate, GeneratedPreroll, HookDirection, PrerollDuration, ShortDramaPrerollState, ShortDramaStep, StoryAnalysis } from './types'
import type { ApiProjectMediaAsset } from '../../data/api'

export const initialShortDramaPrerollState: ShortDramaPrerollState = {
  activeStep: 'understanding', source: null,
  analysisStatus: 'idle', analysis: null, summaryDraft: '',
  hooksStatus: 'idle', hooks: [], selectedHookId: '', duration: 6,
  imagePrompt: '', videoDescription: '', videoPrompt: '',
  imagesStatus: 'idle', images: [], selectedImageId: '',
  trustedFirstFrameAssetId: '', trustedLastFrameAssetId: '', trustedMaterialsBound: false,
  videoStatus: 'idle', output: null, error: '',
}

export type ShortDramaPrerollAction =
  | { type: 'restore'; state: ShortDramaPrerollState }
  | { type: 'open-step'; step: ShortDramaStep }
  | { type: 'source-selected'; source: ApiProjectMediaAsset }
  | { type: 'analysis-started' }
  | { type: 'analysis-ready'; analysis: StoryAnalysis }
  | { type: 'summary-changed'; value: string }
  | { type: 'hooks-started' }
  | { type: 'hooks-ready'; hooks: HookDirection[] }
  | { type: 'hook-selected'; id: string; imagePrompt: string; videoDescription: string; videoPrompt: string; duration: PrerollDuration }
  | { type: 'duration-changed'; duration: PrerollDuration }
  | { type: 'image-prompt-changed'; value: string }
  | { type: 'video-description-changed'; value: string }
  | { type: 'video-prompt-changed'; value: string }
  | { type: 'images-started' }
  | { type: 'images-ready'; images: FirstFrameCandidate[] }
  | { type: 'image-selected'; id: string }
  | { type: 'trusted-material-changed'; role: 'first' | 'last'; value: string }
  | { type: 'trusted-materials-bound'; firstFrameAssetId: string; lastFrameAssetId: string }
  | { type: 'video-started' }
  | { type: 'video-ready'; output: GeneratedPreroll }
  | { type: 'operation-failed'; message: string }

const clearVideo = { videoStatus: 'idle' as const, output: null }
const clearImagesAndVideo = {
  imagesStatus: 'idle' as const, images: [], selectedImageId: '',
  trustedFirstFrameAssetId: '', trustedLastFrameAssetId: '', trustedMaterialsBound: false,
  ...clearVideo,
}

export function shortDramaPrerollReducer(state: ShortDramaPrerollState, action: ShortDramaPrerollAction): ShortDramaPrerollState {
  switch (action.type) {
    case 'restore': return action.state
    case 'open-step': return { ...state, activeStep: action.step, error: '' }
    case 'source-selected': return { ...initialShortDramaPrerollState, source: action.source }
    case 'analysis-started': return { ...state, analysisStatus: 'loading', error: '' }
    case 'analysis-ready': return { ...state, analysisStatus: 'ready', analysis: action.analysis, summaryDraft: action.analysis.synopsis, activeStep: 'direction', error: '' }
    case 'summary-changed': return { ...state, summaryDraft: action.value, hooksStatus: 'idle', hooks: [], selectedHookId: '', imagePrompt: '', videoDescription: '', videoPrompt: '', ...clearImagesAndVideo }
    case 'hooks-started': return { ...state, hooksStatus: 'loading', error: '' }
    case 'hooks-ready': return { ...state, hooksStatus: 'ready', hooks: action.hooks, selectedHookId: '', error: '' }
    case 'hook-selected': {
      const hook = state.hooks.find(item => item.id === action.id)
      if (!hook) return state
      return { ...state, selectedHookId: action.id, imagePrompt: action.imagePrompt, videoDescription: action.videoDescription, videoPrompt: action.videoPrompt, duration: action.duration, activeStep: 'first-frame', ...clearImagesAndVideo }
    }
    case 'duration-changed': return { ...state, duration: action.duration, ...clearVideo }
    case 'image-prompt-changed': return { ...state, imagePrompt: action.value, ...clearImagesAndVideo }
    case 'video-description-changed': return { ...state, videoDescription: action.value, ...clearVideo }
    case 'video-prompt-changed': return { ...state, videoPrompt: action.value, ...clearVideo }
    case 'images-started': return { ...state, imagesStatus: 'loading', error: '' }
    case 'images-ready': return { ...state, imagesStatus: 'ready', images: action.images, selectedImageId: '', error: '' }
    case 'image-selected': return { ...state, selectedImageId: action.id, activeStep: 'video', trustedFirstFrameAssetId: '', trustedLastFrameAssetId: '', trustedMaterialsBound: false, ...clearVideo }
    case 'trusted-material-changed': return action.role === 'first'
      ? { ...state, trustedFirstFrameAssetId: action.value, trustedMaterialsBound: false, ...clearVideo }
      : { ...state, trustedLastFrameAssetId: action.value, trustedMaterialsBound: false, ...clearVideo }
    case 'trusted-materials-bound': return { ...state, trustedFirstFrameAssetId: action.firstFrameAssetId, trustedLastFrameAssetId: action.lastFrameAssetId, trustedMaterialsBound: true, ...clearVideo }
    case 'video-started': return { ...state, videoStatus: 'loading', error: '' }
    case 'video-ready': return { ...state, videoStatus: 'ready', output: action.output, error: '' }
    case 'operation-failed': return { ...state, analysisStatus: state.analysisStatus === 'loading' ? 'error' : state.analysisStatus, hooksStatus: state.hooksStatus === 'loading' ? 'error' : state.hooksStatus, imagesStatus: state.imagesStatus === 'loading' ? 'error' : state.imagesStatus, videoStatus: state.videoStatus === 'loading' ? 'error' : state.videoStatus, error: action.message }
  }
}

export function canOpenShortDramaStep(state: ShortDramaPrerollState, step: ShortDramaStep): boolean {
  if (step === 'understanding') return true
  if (step === 'direction') return state.analysisStatus === 'ready'
  if (step === 'first-frame') return Boolean(state.selectedHookId)
  return Boolean(state.selectedImageId)
}
