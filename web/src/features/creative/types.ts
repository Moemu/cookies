import type { ProviderJob } from '../platform/types'

export type CreativePlan = {
  id: string
  project_id: string
  strategy_output_id: string
  status: 'planned' | 'ready' | 'failed'
  model_alias: string
  image_prompt: string
  video_prompt: string
  created_at: string
  updated_at: string
}

export type CreateCreativePlanInput = {
  strategy_output_id: string
}

export type CreateCreativeImageJobInput = {
  project_context_version: number
  width: number
  height: number
}

export type CreativeImageJob = ProviderJob
