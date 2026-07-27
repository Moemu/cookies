import { apiRequest } from '../../shared/api/client'
import type { BulkRemixPlan } from './aiRemixPlanner'
import type { CreateUploadResponse, ProjectAsset, SignedRequest, UploadSession } from './types'

export function listProjectAssets(projectId: string, signal?: AbortSignal) {
  return apiRequest<{ items: ProjectAsset[] }>(`/platform/v1/projects/${encodeURIComponent(projectId)}/assets?limit=100`, { signal })
}

export function getAssetPreview(projectId: string, assetId: string, version: number, signal?: AbortSignal) {
  return apiRequest<SignedRequest>(`/platform/v1/projects/${encodeURIComponent(projectId)}/assets/${encodeURIComponent(assetId)}/versions/${version}/preview`, { signal })
}

export function removeProjectAsset(projectId: string, assetId: string, version: number, signal?: AbortSignal) {
  return apiRequest<void>(`/platform/v1/projects/${encodeURIComponent(projectId)}/assets/${encodeURIComponent(assetId)}/versions/${version}`, {
    method: 'DELETE',
    signal,
  })
}

export function createAssetUpload(projectId: string, file: File, sha256: string, signal?: AbortSignal) {
  return apiRequest<CreateUploadResponse>(`/platform/v1/projects/${encodeURIComponent(projectId)}/assets/uploads`, {
    method: 'POST',
    signal,
    headers: { 'Idempotency-Key': `web-upload-${crypto.randomUUID()}` },
    body: JSON.stringify({
      filename: file.name,
      declared_mime_type: file.type,
      declared_size_bytes: file.size,
      declared_sha256: sha256,
    }),
  })
}

export async function putAssetContent(request: SignedRequest, file: File, signal?: AbortSignal) {
  if (!request.url) throw new Error('上传地址为空，请重新创建上传会话。')
  const headers = new Headers()
  for (const [name, value] of Object.entries(request.headers)) {
    const normalizedName = name.toLowerCase()
    if (normalizedName !== 'host' && normalizedName !== 'content-length') headers.set(name, value)
  }
  if (!headers.has('Content-Type')) headers.set('Content-Type', file.type)
  const response = await fetch(request.url, { method: request.method, headers, body: file, signal })
  if (!response.ok) throw new Error(`文件上传失败（HTTP ${response.status}）`)
}

export function finalizeAssetUpload(projectId: string, uploadId: string, signal?: AbortSignal) {
  return apiRequest<UploadSession>(`/platform/v1/projects/${encodeURIComponent(projectId)}/assets/uploads/${encodeURIComponent(uploadId)}:finalize`, {
    method: 'POST',
    signal,
  })
}

export async function calculateSHA256(file: File) {
  const digest = await crypto.subtle.digest('SHA-256', await file.arrayBuffer())
  return Array.from(new Uint8Array(digest), (byte) => byte.toString(16).padStart(2, '0')).join('')
}

export type SavedRemixPlan = {
  id: string
  organization_id: string
  project_id: string
  client_plan_id: string
  target_seconds: number
  actual_seconds: number
  pace: BulkRemixPlan['pace']
  segments: Array<{
    segment: string
    label: string
    target_seconds: number
    actual_seconds: number
    clips: Array<{
      id: string
      segment: string
      asset_version: { asset_id: string; version: number }
      label: string
      source_type: string
      mime_type: string
      aspect: string
      start_seconds: number
      duration_seconds: number
      in_point_seconds: number
      out_point_seconds: number
      score: number
      reason: string
    }>
  }>
  warnings: string[]
  summary: {
    selected_assets: number
    used_assets: number
    coverage_percent: number
    strategy: string
  }
  created_at: string
  updated_at: string
}

export type RemixRenderJob = {
  id: string
  organization_id: string
  project_id: string
  plan_id: string
  status: 'queued' | 'running' | 'succeeded' | 'failed'
  target_format: 'mp4'
  target_quality: 'draft' | 'standard' | 'high'
  output_asset?: { project_id: string; asset_version: { asset_id: string; version: number } }
  error_code?: string
  error_message?: string
  created_at: string
  updated_at: string
}

export function createRemixPlan(projectId: string, plan: BulkRemixPlan, signal?: AbortSignal) {
  return apiRequest<SavedRemixPlan>(`/platform/v1/projects/${encodeURIComponent(projectId)}/remix-plans`, {
    method: 'POST',
    signal,
    body: JSON.stringify({
      client_plan_id: plan.id,
      target_seconds: plan.targetSeconds,
      actual_seconds: plan.actualSeconds,
      pace: plan.pace,
      segments: plan.segments.map((segment) => ({
        segment: segment.segment,
        label: segment.label,
        target_seconds: segment.targetSeconds,
        actual_seconds: segment.actualSeconds,
        clips: segment.clips.map((clip) => ({
          id: clip.id,
          segment: clip.segment,
          asset_version: { asset_id: clip.assetId, version: clip.version },
          label: clip.label,
          source_type: clip.sourceType,
          mime_type: clip.mimeType,
          aspect: clip.aspect,
          start_seconds: clip.startSeconds,
          duration_seconds: clip.durationSeconds,
          in_point_seconds: clip.inPointSeconds,
          out_point_seconds: clip.outPointSeconds,
          score: clip.score,
          reason: clip.reason,
        })),
      })),
      warnings: plan.warnings,
      summary: {
        selected_assets: plan.summary.selectedAssets,
        used_assets: plan.summary.usedAssets,
        coverage_percent: plan.summary.coveragePercent,
        strategy: plan.summary.strategy,
      },
    }),
  })
}

export function getRemixPlan(projectId: string, planId: string, signal?: AbortSignal) {
  return apiRequest<SavedRemixPlan>(`/platform/v1/projects/${encodeURIComponent(projectId)}/remix-plans/${encodeURIComponent(planId)}`, { signal })
}

export function listRemixPlans(projectId: string, limit = 20, signal?: AbortSignal) {
  return apiRequest<{ items: SavedRemixPlan[] }>(`/platform/v1/projects/${encodeURIComponent(projectId)}/remix-plans?limit=${limit}`, { signal })
}

export function createRemixRenderJob(projectId: string, planId: string, targetQuality: RemixRenderJob['target_quality'] = 'standard', signal?: AbortSignal) {
  return apiRequest<RemixRenderJob>(`/platform/v1/projects/${encodeURIComponent(projectId)}/remix-render-jobs`, {
    method: 'POST',
    signal,
    body: JSON.stringify({
      plan_id: planId,
      target_format: 'mp4',
      target_quality: targetQuality,
    }),
  })
}

export function getRemixRenderJob(projectId: string, jobId: string, signal?: AbortSignal) {
  return apiRequest<RemixRenderJob>(`/platform/v1/projects/${encodeURIComponent(projectId)}/remix-render-jobs/${encodeURIComponent(jobId)}`, { signal })
}
