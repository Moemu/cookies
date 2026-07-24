import { apiRequest } from '../../shared/api/client'
import type { ProviderJob } from '../platform/types'
import type { CreateCreativeTaskInput, CreativeIntake, CreativeIntakeInput, CreativeTask, CreativeTaskDetail, CreativeVersion, CreativePackage, ReviseDraftInput, ImageTextDraft } from './types'

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

export function createCreativeTask(projectId: string, intakeId: string, input: CreateCreativeTaskInput) {
  return apiRequest<CreativeTask>(`${base(projectId)}/creative-intakes/${encodeURIComponent(intakeId)}:create-task`, { method: 'POST', body: JSON.stringify(input) })
}

export function listCreativeTasks(projectId: string, signal?: AbortSignal) {
  return apiRequest<{ items: CreativeTask[] }>(`${base(projectId)}/creative-tasks?limit=100`, { signal })
}

export function getCreativeTask(projectId: string, taskId: string, signal?: AbortSignal) {
  return apiRequest<CreativeTaskDetail>(`${base(projectId)}/creative-tasks/${encodeURIComponent(taskId)}`, { signal })
}

// This is intentionally an archive operation. The API uses DELETE for the
// familiar user action, but the server preserves Creative and Provider lineage.
export function archiveCreativeTask(projectId: string, taskId: string) {
  return apiRequest<void>(`${base(projectId)}/creative-tasks/${encodeURIComponent(taskId)}`, { method: 'DELETE' })
}

export function reviseCreativeDraft(projectId: string, taskId: string, input: ReviseDraftInput) {
  return apiRequest<ImageTextDraft>(`${base(projectId)}/creative-tasks/${encodeURIComponent(taskId)}:draft`, {
    method: 'PATCH', body: JSON.stringify(input),
  })
}

export function freezeCreativeVersion(projectId: string, taskId: string, draftVersion: number) {
  return apiRequest<CreativeVersion>(`${base(projectId)}/creative-tasks/${encodeURIComponent(taskId)}:freeze-version`, {
    method: 'POST', headers: { 'Idempotency-Key': `creative-freeze-${crypto.randomUUID()}` }, body: JSON.stringify({ draft_version: draftVersion }),
  })
}

export function listCreativeVersions(projectId: string, taskId = '', signal?: AbortSignal) {
  const query = new URLSearchParams({ limit: '100' })
  if (taskId) query.set('task_id', taskId)
  return apiRequest<{ items: CreativeVersion[] }>(`${base(projectId)}/creative-versions?${query}`, { signal })
}

export function listCreativePackages(projectId: string, signal?: AbortSignal) {
  return apiRequest<{ items: CreativePackage[] }>(`${base(projectId)}/creative-packages?limit=100`, { signal })
}

export function createCoverImageJob(projectId: string, taskId: string) {
  return apiRequest<ProviderJob>(`${base(projectId)}/creative-tasks/${encodeURIComponent(taskId)}:cover-image-job`, {
    method: 'POST',
    headers: { 'Idempotency-Key': `creative-cover-${crypto.randomUUID()}` },
    body: JSON.stringify({}),
  })
}

export function createImagePlanJob(projectId: string, taskId: string, imagePlanOrder: number) {
  return apiRequest<ProviderJob>(`${base(projectId)}/creative-tasks/${encodeURIComponent(taskId)}:image-job`, {
    method: 'POST',
    headers: { 'Idempotency-Key': `creative-image-${imagePlanOrder}-${crypto.randomUUID()}` },
    body: JSON.stringify({ image_plan_order: imagePlanOrder }),
  })
}

export function bindCreativeImageAsset(projectId: string, taskId: string, expectedDraftVersion: number, imagePlanOrder: number, assetId: string, assetVersion: number) {
  return apiRequest<ImageTextDraft>(`${base(projectId)}/creative-tasks/${encodeURIComponent(taskId)}:bind-image-asset`, {
    method: 'POST',
    body: JSON.stringify({ expected_draft_version: expectedDraftVersion, image_plan_order: imagePlanOrder, asset_ref: { asset_id: assetId, version: assetVersion } }),
  })
}

export function checkCreativeVersion(projectId: string, versionId: string) {
  return apiRequest<CreativeVersion>(`${base(projectId)}/creative-versions/${encodeURIComponent(versionId)}:check`, { method: 'POST' })
}

export function approveCreativeVersion(projectId: string, versionId: string) {
  return apiRequest<CreativeVersion>(`${base(projectId)}/creative-versions/${encodeURIComponent(versionId)}:approve`, { method: 'POST' })
}

export function deliverCreativeVersion(projectId: string, versionId: string) {
  return apiRequest<CreativePackage>(`${base(projectId)}/creative-versions/${encodeURIComponent(versionId)}:deliver`, { method: 'POST' })
}
