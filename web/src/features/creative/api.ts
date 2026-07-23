import { apiRequest } from '../../shared/api/client'
import type { ProviderJob } from '../platform/types'
import type { CreativeIntake, CreativeIntakeInput, CreativeTask, CreativeTaskDetail } from './types'

function base(projectId: string) {
  return `/api/creative/v1/projects/${encodeURIComponent(projectId)}`
}

export function listCreativeIntakes(projectId: string, signal?: AbortSignal) {
  return apiRequest<{ items: CreativeIntake[] }>(`${base(projectId)}/creative-intakes?limit=100`, { signal })
}

export function createCreativeIntake(projectId: string, input: CreativeIntakeInput) {
  return apiRequest<CreativeIntake>(`${base(projectId)}/creative-intakes`, {
    method: 'POST',
    headers: { 'Idempotency-Key': `creative-intake-${crypto.randomUUID()}` },
    body: JSON.stringify(input),
  })
}

export function createCreativeTask(projectId: string, intakeId: string) {
  return apiRequest<CreativeTask>(`${base(projectId)}/creative-intakes/${encodeURIComponent(intakeId)}:create-task`, { method: 'POST' })
}

export function listCreativeTasks(projectId: string, signal?: AbortSignal) {
  return apiRequest<{ items: CreativeTask[] }>(`${base(projectId)}/creative-tasks?limit=100`, { signal })
}

export function getCreativeTask(projectId: string, taskId: string, signal?: AbortSignal) {
  return apiRequest<CreativeTaskDetail>(`${base(projectId)}/creative-tasks/${encodeURIComponent(taskId)}`, { signal })
}

export function createCoverImageJob(projectId: string, taskId: string) {
  return apiRequest<ProviderJob>(`${base(projectId)}/creative-tasks/${encodeURIComponent(taskId)}:cover-image-job`, {
    method: 'POST',
    headers: { 'Idempotency-Key': `creative-cover-${crypto.randomUUID()}` },
    body: JSON.stringify({}),
  })
}
