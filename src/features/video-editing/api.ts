const viteEnv = import.meta.env as Record<string, string | undefined>
const backendOrigin = viteEnv.VITE_API_BASE_URL ?? ''

export type EditingAssetRef = { asset_id: string; version: number }

export type EditingTimeline = {
  schema_version: 'editing-timeline/v1'
  output_profile: { id: 'cookies-editing-vertical-v1'; width: 720; height: 1280; frame_rate: 30; sample_rate: 48000 }
  duration_ms: number
  tracks: Array<{
    id: string
    role: 'primary_video' | 'caption' | 'voiceover' | 'music' | 'sfx'
    clips: Array<{
      id: string
      asset_ref?: EditingAssetRef
      timeline_start_ms: number
      timeline_end_ms: number
      source_in_ms?: number
      source_out_ms?: number
      text?: string
      gain_db?: number
      loop?: boolean
    }>
  }>
}

export type ApiEditTask = {
  id: string
  display_name: string
  status: 'draft'
  entry_source: 'manual' | 'short_drama_preroll_v2' | 'creative_version'
  source_creative_task_id?: string
  current_timeline: { version: number; timeline: EditingTimeline; content_hash: string; created_at: string }
  created_at: string
  updated_at: string
}

export type ApiEditingRenderJob = {
  id: string
  edit_task_id: string
  timeline: { version: number; content_hash: string }
  kind: 'preview' | 'export'
  status: 'queued' | 'running' | 'succeeded' | 'failed' | 'cancelled'
  progress_percent: number
  output_asset?: { asset_version: EditingAssetRef }
  error_code?: string
  error_message?: string
}

async function editingRequest<T>(path: string, method = 'GET', body?: unknown): Promise<T> {
  const response = await fetch(`${backendOrigin}/api/creative/v1${path}`, {
    method,
    credentials: 'include',
    headers: body === undefined ? undefined : { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  const text = await response.text()
  let payload: T | { error?: { message?: string } } = {}
  try { payload = text ? JSON.parse(text) as T | { error?: { message?: string } } : {} } catch { throw new Error(`素材剪辑服务返回了无效响应（HTTP ${response.status}）`) }
  if (!response.ok) throw new Error((payload as { error?: { message?: string } }).error?.message ?? `素材剪辑请求失败（HTTP ${response.status}）`)
  return payload as T
}

export const editingApi = {
  create: (projectId: string, input: { display_name: string; timeline: EditingTimeline }) => editingRequest<ApiEditTask>(`/projects/${encodeURIComponent(projectId)}/edit-tasks`, 'POST', input),
  get: (projectId: string, editTaskId: string) => editingRequest<ApiEditTask>(`/projects/${encodeURIComponent(projectId)}/edit-tasks/${encodeURIComponent(editTaskId)}`),
  saveTimeline: (projectId: string, editTaskId: string, expectedVersion: number, timeline: EditingTimeline) => editingRequest<ApiEditTask>(`/projects/${encodeURIComponent(projectId)}/edit-tasks/${encodeURIComponent(editTaskId)}/timeline`, 'PATCH', { expected_version: expectedVersion, timeline }),
  openShortDramaV2: (projectId: string, creativeTaskId: string) => editingRequest<ApiEditTask>(`/projects/${encodeURIComponent(projectId)}/creative-tasks/${encodeURIComponent(creativeTaskId)}/short-drama-preroll-v2:open-editor`, 'POST'),
  openCreativeVersion: (projectId: string, creativeTaskId: string) => editingRequest<ApiEditTask>(`/projects/${encodeURIComponent(projectId)}/creative-tasks/${encodeURIComponent(creativeTaskId)}/open-editor`, 'POST'),
  createRender: (projectId: string, editTaskId: string, kind: 'preview' | 'export') => editingRequest<ApiEditingRenderJob>(`/projects/${encodeURIComponent(projectId)}/edit-tasks/${encodeURIComponent(editTaskId)}/renders`, 'POST', { kind }),
  getRender: (projectId: string, renderJobId: string) => editingRequest<ApiEditingRenderJob>(`/projects/${encodeURIComponent(projectId)}/edit-renders/${encodeURIComponent(renderJobId)}`),
  cancelRender: (projectId: string, renderJobId: string) => editingRequest<ApiEditingRenderJob>(`/projects/${encodeURIComponent(projectId)}/edit-renders/${encodeURIComponent(renderJobId)}/cancel`, 'POST'),
  retryRender: (projectId: string, renderJobId: string) => editingRequest<ApiEditingRenderJob>(`/projects/${encodeURIComponent(projectId)}/edit-renders/${encodeURIComponent(renderJobId)}/retry`, 'POST'),
}
