import type { ApiProjectMediaAsset } from '../../data/api'

export type ShortDramaStep = 'understanding' | 'direction' | 'first-frame' | 'video'
export type AsyncStatus = 'idle' | 'loading' | 'ready' | 'error'
export type HookCategory = 'curiosity' | 'summary'
export type PrerollDuration = 5 | 6 | 10 | 12 | 15

export type StoryAnalysis = {
  title: string
  episode: string
  synopsis: string
  openingBeat: string
  characters: string[]
  visualKeywords: string[]
}

export type HookDirection = {
  id: string
  category: HookCategory
  eyebrow: string
  title: string
  description: string
  hookCopy: string
  rationale: string
}

export type FirstFrameCandidate = {
  id: string
  label: string
  imageUrl: string
  composition: string
  variantKey?: string
  visualMechanism?: string
  styleProfile?: string
}

export type GeneratedPreroll = {
  id: string
  videoUrl: string
  duration: PrerollDuration
  createdAt: string
}

export type ShortDramaPrerollState = {
  activeStep: ShortDramaStep
  source: ApiProjectMediaAsset | null
  analysisStatus: AsyncStatus
  analysis: StoryAnalysis | null
  summaryDraft: string
  hooksStatus: AsyncStatus
  hooks: HookDirection[]
  selectedHookId: string
  selectingHookId: string
  duration: PrerollDuration
  imagePrompt: string
  videoDescription: string
  videoPrompt: string
  imagesStatus: AsyncStatus
  images: FirstFrameCandidate[]
  selectedImageId: string
  selectingImageId: string
  videoStatus: AsyncStatus
  output: GeneratedPreroll | null
  error: string
}
