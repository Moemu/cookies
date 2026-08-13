export type UploadedProjectAssetRef = {
  asset_id: string
  version: number
}

export class ProjectAssetUploadError extends Error {
  constructor(message: string, readonly status: number) {
    super(message)
    this.name = 'ProjectAssetUploadError'
  }
}

type UploadSessionResponse = {
  session: {
    id: string
    project_asset_ref: null | { asset_version: UploadedProjectAssetRef }
  }
  upload: null | {
    url: string
    headers: Record<string, string>
  }
}

function responseMessage(payload: unknown, fallback: string) {
  const value = payload as { error?: { message?: string } } | null
  return value?.error?.message || fallback
}

async function parseResponse(response: Response) {
  const text = await response.text()
  if (!text) return null
  try {
    return JSON.parse(text) as unknown
  } catch {
    return null
  }
}

export async function uploadProjectAssetFile(
  backendOrigin: string,
  projectId: string,
  file: File,
  idempotencyPrefix: string,
): Promise<UploadedProjectAssetRef> {
  const path = `/platform/v1/projects/${encodeURIComponent(projectId)}/assets/uploads`
  const createResponse = await fetch(`${backendOrigin}${path}`, {
    method: 'POST',
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
      'Idempotency-Key': `${idempotencyPrefix}-${Date.now()}-${Math.random().toString(36).slice(2)}`,
    },
    body: JSON.stringify({
      filename: file.name,
      declared_mime_type: file.type,
      declared_size_bytes: file.size,
      declared_sha256: null,
    }),
  })
  const createPayload = await parseResponse(createResponse)
  if (!createResponse.ok) {
    throw new ProjectAssetUploadError(responseMessage(createPayload, '无法创建素材上传任务。'), createResponse.status)
  }
  const created = createPayload as UploadSessionResponse
  const existing = created.session.project_asset_ref?.asset_version
  if (existing) return existing
  if (!created.upload) throw new ProjectAssetUploadError('素材上传任务没有返回上传地址。', 502)

  const requestHeaders = new Headers()
  for (const [name, value] of Object.entries(created.upload.headers)) {
    const normalized = name.toLowerCase()
    if (normalized !== 'host' && normalized !== 'content-length') requestHeaders.set(name, value)
  }
  if (!requestHeaders.has('Content-Type')) requestHeaders.set('Content-Type', file.type)
  const uploadURL = created.upload.url.startsWith('/') ? `${backendOrigin}${created.upload.url}` : created.upload.url
  const uploadResponse = await fetch(uploadURL, {
    method: 'PUT',
    headers: requestHeaders,
    body: file,
    credentials: created.upload.url.startsWith('/') ? 'include' : 'omit',
  })
  if (!uploadResponse.ok) throw new ProjectAssetUploadError(`素材上传失败（HTTP ${uploadResponse.status}）`, uploadResponse.status)

  const finalizeResponse = await fetch(`${backendOrigin}${path}/${encodeURIComponent(created.session.id)}:finalize`, {
    method: 'POST',
    credentials: 'include',
  })
  const finalizePayload = await parseResponse(finalizeResponse)
  if (!finalizeResponse.ok) {
    throw new ProjectAssetUploadError(responseMessage(finalizePayload, '素材已上传，但入库失败。'), finalizeResponse.status)
  }
  const result = (finalizePayload as { project_asset_ref: null | { asset_version: UploadedProjectAssetRef } })
    .project_asset_ref?.asset_version
  if (!result) throw new ProjectAssetUploadError('素材已上传，但没有生成可用的项目素材。', 502)
  return result
}
