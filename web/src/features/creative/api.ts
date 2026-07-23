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

// The browser sends only the immutable package identity and hash. Creative
// reads the authorized package server-side, preventing a client from changing
// strategy content or organization scope during the handoff.
export function createCreativeIntakeFromStrategy(projectId: string, packageId: string, packageVersion: number, expectedContentHash: string) {
  return apiRequest<CreativeIntake>(`${base(projectId)}/creative-intakes`, {
    method: 'POST',
    headers: { 'Idempotency-Key': `creative-from-strategy-${crypto.randomUUID()}` },
    body: JSON.stringify({
      source: 'strategy_package',
      strategy_package: { package_id: packageId, package_version: packageVersion, expected_content_hash: expectedContentHash },
    }),
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
