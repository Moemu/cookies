import type { AdScriptDraft, AINativeAdWorkspaceSummary, AINativeProductPreview, AINativeReopenImpact, AINativeRequirement, AINativeRequirementWorkspace, AINativeStageId, StoryboardDraft, VoiceoverFitSuggestion } from './types'

const viteEnv = (import.meta as unknown as { env?: { VITE_API_BASE_URL?: string } }).env
const backendOrigin = viteEnv?.VITE_API_BASE_URL ?? ''

export class AINativeApiError extends Error {
  constructor(message: string, readonly status: number, readonly code = '') {
    super(message)
    this.name = 'AINativeApiError'
  }
}

async function request<T>(path: string, method = 'GET', body?: unknown, timeoutMs = 0): Promise<T> {
  const controller = new AbortController()
  const timeout = timeoutMs > 0 ? setTimeout(() => controller.abort(), timeoutMs) : undefined
  let response: Response
  try {
    response = await fetch(`${backendOrigin}/api/creative/v1${path}`, {
      method,
      credentials: 'include',
      headers: body === undefined ? undefined : { 'Content-Type': 'application/json' },
      body: body === undefined ? undefined : JSON.stringify(body),
      signal: controller.signal,
    })
  } catch (cause) {
    if (controller.signal.aborted) throw new AINativeApiError('AI 效果广告接口请求超时，请重试。', 408, 'CLIENT_TIMEOUT')
    throw cause
  } finally {
    if (timeout !== undefined) clearTimeout(timeout)
  }
  const responseText = await response.text()
  let payload: T | { error?: { message?: string; code?: string; request_id?: string } }
  try {
    payload = responseText ? JSON.parse(responseText) as T | { error?: { message?: string; code?: string; request_id?: string } } : {}
  } catch {
    throw new AINativeApiError(`AI 效果广告接口返回了无法解析的响应（HTTP ${response.status}）`, response.status)
  }
  if (!response.ok) {
    const error = payload as { error?: { message?: string; code?: string; request_id?: string } }
    const requestId = error.error?.request_id ? `（request_id: ${error.error.request_id}）` : ''
    throw new AINativeApiError(`${error.error?.message ?? `AI 效果广告接口请求失败（HTTP ${response.status}）`}${requestId}`, response.status, error.error?.code)
  }
  return payload as T
}

export function analyzeRequirement(projectId: string, productLink: string, supplementalRequirement: string) {
  return request<AINativeRequirementWorkspace>(
    `/projects/${encodeURIComponent(projectId)}/ai-native-ads/requirements:analyze`,
    'POST',
    {
      product_link: productLink,
      supplemental_requirement: supplementalRequirement,
      channel: 'douyin',
      aspect_ratio: '9:16',
      duration_seconds: 20,
      language: 'zh-CN',
    },
  )
}

export function resolveProductPreview(projectId: string, productLink: string) {
  return request<AINativeProductPreview>(
    `/projects/${encodeURIComponent(projectId)}/ai-native-ads/products:resolve`,
    'POST',
    { product_link: productLink },
    15000,
  )
}

export function getRequirementWorkspace(projectId: string, workspaceId: string) {
  return request<AINativeRequirementWorkspace>(
    `/projects/${encodeURIComponent(projectId)}/ai-native-ads/${encodeURIComponent(workspaceId)}`,
  )
}

export async function getLatestRequirementWorkspace(projectId: string) {
  try {
    return await request<AINativeRequirementWorkspace>(
      `/projects/${encodeURIComponent(projectId)}/ai-native-ads:latest`,
    )
  } catch (cause) {
    if (cause instanceof AINativeApiError && cause.status === 404) return null
    throw cause
  }
}

export function listAINativeAdWorkspaces(projectId: string) {
  return request<AINativeAdWorkspaceSummary[]>(
    `/projects/${encodeURIComponent(projectId)}/ai-native-ads`,
  )
}

export function renameAINativeAdWorkspace(projectId: string, workspaceId: string, displayName: string) {
  return request<AINativeRequirementWorkspace>(
    `/projects/${encodeURIComponent(projectId)}/ai-native-ads/${encodeURIComponent(workspaceId)}/metadata`,
    'PATCH',
    { display_name: displayName.trim() },
  )
}

export function updateRequirement(projectId: string, workspaceId: string, requirement: AINativeRequirement) {
  return request<AINativeRequirementWorkspace>(
    `/projects/${encodeURIComponent(projectId)}/ai-native-ads/${encodeURIComponent(workspaceId)}/requirement`,
    'PATCH',
    {
      expected_revision: requirement.revision,
      product_name: requirement.product_name,
      product_description: requirement.product_description,
      target_audiences: requirement.target_audiences,
      media: requirement.media,
      core_selling_points: requirement.core_selling_points,
      supplemental_requirement: requirement.supplemental_requirement,
      aspect_ratio: requirement.aspect_ratio,
      duration_seconds: requirement.duration_seconds,
      language: requirement.language,
    },
  )
}

export function confirmRequirement(projectId: string, workspaceId: string, expectedRevision: number) {
  return request<AINativeRequirementWorkspace>(
    `/projects/${encodeURIComponent(projectId)}/ai-native-ads/${encodeURIComponent(workspaceId)}/requirement:confirm`,
    'POST',
    { expected_revision: expectedRevision },
  )
}

export function getRequirementReopenImpact(projectId: string, workspaceId: string, stage: Extract<AINativeStageId, 'requirement' | 'script' | 'storyboard'> = 'requirement') {
  return request<AINativeReopenImpact>(
    `/projects/${encodeURIComponent(projectId)}/ai-native-ads/${encodeURIComponent(workspaceId)}/reopen-impact?stage=${stage}`,
  )
}

export function reopenStoryboard(projectId: string, workspaceId: string, expectedWorkspaceVersion: number) {
  return request<AINativeRequirementWorkspace>(`/projects/${encodeURIComponent(projectId)}/ai-native-ads/${encodeURIComponent(workspaceId)}/storyboard:reopen`, 'POST', { expected_workspace_version: expectedWorkspaceVersion, invalidate_downstream: true })
}

export function generateScript(projectId: string, workspaceId: string, expectedWorkspaceVersion: number, regenerationNote = '') {
  return request<AINativeRequirementWorkspace>(
    `/projects/${encodeURIComponent(projectId)}/ai-native-ads/${encodeURIComponent(workspaceId)}/script:generate`,
    'POST',
    { expected_workspace_version: expectedWorkspaceVersion, regeneration_note: regenerationNote.trim() },
  )
}

export function regenerateScript(projectId: string, workspaceId: string, expectedWorkspaceVersion: number, regenerationNote = '') {
  return request<AINativeRequirementWorkspace>(
    `/projects/${encodeURIComponent(projectId)}/ai-native-ads/${encodeURIComponent(workspaceId)}/script:regenerate`,
    'POST',
    { expected_workspace_version: expectedWorkspaceVersion, regeneration_note: regenerationNote.trim() },
  )
}

export function updateScript(projectId: string, workspaceId: string, script: AdScriptDraft) {
  return request<AINativeRequirementWorkspace>(
    `/projects/${encodeURIComponent(projectId)}/ai-native-ads/${encodeURIComponent(workspaceId)}/script`,
    'PATCH',
    { expected_revision: script.revision, script },
  )
}

export function confirmScript(projectId: string, workspaceId: string, expectedRevision: number, expectedWorkspaceVersion: number) {
  return request<AINativeRequirementWorkspace>(
    `/projects/${encodeURIComponent(projectId)}/ai-native-ads/${encodeURIComponent(workspaceId)}/script:confirm`,
    'POST',
    { expected_revision: expectedRevision, expected_workspace_version: expectedWorkspaceVersion },
  )
}

export function reopenScript(projectId: string, workspaceId: string, expectedWorkspaceVersion: number) {
  return request<AINativeRequirementWorkspace>(
    `/projects/${encodeURIComponent(projectId)}/ai-native-ads/${encodeURIComponent(workspaceId)}/script:reopen`,
    'POST',
    { expected_workspace_version: expectedWorkspaceVersion, invalidate_downstream: true },
  )
}

export function reopenRequirement(projectId: string, workspaceId: string, expectedWorkspaceVersion: number) {
  return request<AINativeRequirementWorkspace>(
    `/projects/${encodeURIComponent(projectId)}/ai-native-ads/${encodeURIComponent(workspaceId)}/requirement:reopen`,
    'POST',
    { expected_workspace_version: expectedWorkspaceVersion, invalidate_downstream: true },
  )
}

export function generateStoryboard(projectId: string, workspaceId: string, expectedWorkspaceVersion: number) {
  return request<AINativeRequirementWorkspace>(
    `/projects/${encodeURIComponent(projectId)}/ai-native-ads/${encodeURIComponent(workspaceId)}/storyboard:generate`,
    'POST',
    { expected_workspace_version: expectedWorkspaceVersion },
    30_000,
  )
}

export function regenerateStoryboardAsset(projectId: string, workspaceId: string, assetId: string, expectedWorkspaceVersion: number, feedback = '') {
  return request<AINativeRequirementWorkspace>(
    `/projects/${encodeURIComponent(projectId)}/ai-native-ads/${encodeURIComponent(workspaceId)}/storyboard/assets/${encodeURIComponent(assetId)}/regenerate`,
    'POST',
    { expected_workspace_version: expectedWorkspaceVersion, ...(feedback.trim() ? { feedback: feedback.trim() } : {}) },
  )
}

export function updateStoryboard(projectId: string, workspaceId: string, storyboard: StoryboardDraft) {
  return request<AINativeRequirementWorkspace>(
    `/projects/${encodeURIComponent(projectId)}/ai-native-ads/${encodeURIComponent(workspaceId)}/storyboard`,
    'PATCH',
    { expected_revision: storyboard.revision, storyboard },
  )
}

export function confirmStoryboard(projectId: string, workspaceId: string, expectedRevision: number, expectedWorkspaceVersion: number) {
  return request<AINativeRequirementWorkspace>(
    `/projects/${encodeURIComponent(projectId)}/ai-native-ads/${encodeURIComponent(workspaceId)}/storyboard:confirm`,
    'POST',
    { expected_revision: expectedRevision, expected_workspace_version: expectedWorkspaceVersion },
  )
}

export function startProduction(projectId: string, workspaceId: string, expectedWorkspaceVersion: number) {
  return request<AINativeRequirementWorkspace>(`/projects/${encodeURIComponent(projectId)}/ai-native-ads/${encodeURIComponent(workspaceId)}/production:start`, 'POST', { expected_workspace_version: expectedWorkspaceVersion })
}

export function retryProductionUnit(projectId: string, workspaceId: string, unitId: string, expectedWorkspaceVersion: number) {
  return request<AINativeRequirementWorkspace>(`/projects/${encodeURIComponent(projectId)}/ai-native-ads/${encodeURIComponent(workspaceId)}/production/units/${encodeURIComponent(unitId)}/retry`, 'POST', { expected_workspace_version: expectedWorkspaceVersion })
}

export function cancelProduction(projectId: string, workspaceId: string, expectedWorkspaceVersion: number) {
  return request<AINativeRequirementWorkspace>(`/projects/${encodeURIComponent(projectId)}/ai-native-ads/${encodeURIComponent(workspaceId)}/production:cancel`, 'POST', { expected_workspace_version: expectedWorkspaceVersion })
}

export function fitStoryboardVoiceover(projectId: string, workspaceId: string, speechUnitId: string, expectedWorkspaceVersion: number) {
  return request<VoiceoverFitSuggestion>(
    `/projects/${encodeURIComponent(projectId)}/ai-native-ads/${encodeURIComponent(workspaceId)}/storyboard/voiceover:fit`,
    'POST',
    { expected_workspace_version: expectedWorkspaceVersion, speech_unit_id: speechUnitId },
    45_000,
  )
}

export async function getAssetPreview(projectId: string, ref: { asset_id: string; version: number }) {
  const response = await fetch(`${backendOrigin}/platform/v1/projects/${encodeURIComponent(projectId)}/assets/${encodeURIComponent(ref.asset_id)}/versions/${ref.version}/preview`, { credentials: 'include' })
  if (!response.ok) throw new AINativeApiError('项目素材预览读取失败。', response.status)
  const value = await response.json() as { url: string }
  return value.url.startsWith('/') ? `${backendOrigin}${value.url}` : value.url
}
