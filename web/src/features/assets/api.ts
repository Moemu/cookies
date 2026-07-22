import { apiRequest } from '../../shared/api/client'
import type { CreateUploadResponse, ProjectAsset, SignedRequest, UploadSession } from './types'

export function listProjectAssets(projectId: string, signal?: AbortSignal) {
  return apiRequest<{ items: ProjectAsset[] }>(`/platform/v1/projects/${encodeURIComponent(projectId)}/assets?limit=100`, { signal })
}

export function getAssetPreview(projectId: string, assetId: string, version: number, signal?: AbortSignal) {
  return apiRequest<SignedRequest>(`/platform/v1/projects/${encodeURIComponent(projectId)}/assets/${encodeURIComponent(assetId)}/versions/${version}/preview`, { signal })
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
