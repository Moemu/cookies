import { apiRequest } from '../../shared/api/client'
import type { CreativeImageJob, CreativePlan, CreateCreativeImageJobInput, CreateCreativePlanInput } from './types'

const projectPath = (projectId: string) => `/api/creative/v1/projects/${encodeURIComponent(projectId)}`

export function createCreativePlan(projectId: string, input: CreateCreativePlanInput, signal?: AbortSignal) {
  return apiRequest<CreativePlan>(`${projectPath(projectId)}/plans`, {
    method: 'POST',
    headers: { 'Idempotency-Key': `web-creative-plan-${crypto.randomUUID()}` },
    body: JSON.stringify(input),
    signal,
  })
}

export function createCreativeImageJob(projectId: string, planId: string, input: CreateCreativeImageJobInput, signal?: AbortSignal) {
  return apiRequest<CreativeImageJob>(`${projectPath(projectId)}/plans/${encodeURIComponent(planId)}/image-jobs`, {
    method: 'POST',
    headers: { 'Idempotency-Key': `web-creative-image-${crypto.randomUUID()}` },
    body: JSON.stringify(input),
    signal,
  })
}
