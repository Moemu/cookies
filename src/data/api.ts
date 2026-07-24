export type ApiProject = {
  id: string
  name: string
  brand: string
  objective: string
  runtime: {
    code: string
    product: string
    stage: string
    progress: number
    status: 'active' | 'completed'
    owner: string
    budget: number
    currency: 'CNY'
    timezone: 'Asia/Shanghai'
  }
  version: number
  createdAt: string
  updatedAt: string
}

export type ApiArtifact = {
  id: string
  projectId: string
  kind: 'brief' | 'image' | 'video' | 'document'
  purpose?: ApiVideoPurpose
  prerollType?: ApiPrerollType
  status: 'draft' | 'ready' | 'archived'
  content: string
  sourceJobId?: string
  version: number
  createdAt: string
  updatedAt: string
}

export type ApiGenerationJob = {
  id: string
  projectId: string
  artifactKind: ApiArtifact['kind']
  purpose?: ApiVideoPurpose
  prerollType?: ApiPrerollType
  briefArtifactId?: string
  status: 'queued' | 'running' | 'succeeded' | 'failed' | 'cancelled'
  model?: string
  diagnostic?: string
  artifactId?: string
  version: number
  createdAt: string
  updatedAt: string
}

export type ApiVideoPurpose = 'preroll'
export type ApiPrerollType = 'short_drama' | 'game' | 'commerce'

export type ApiPrerollScope = {
  projectId: string
  purpose: 'preroll'
  prerollType: ApiPrerollType
}

export type ApiBusinessTaskType =
  | 'strategy'
  | 'creative'
  | 'video'
  | 'brand_video'
  | 'short_drama_preroll'
  | 'game_preroll'
  | 'commerce_preroll'
  | 'viral_remake'
  | 'video_edit'

export type ApiBusinessTask = {
  id: string
  projectId: string
  type: ApiBusinessTaskType
  name: string
  objective: string
  status: 'draft' | 'in_progress' | 'ready' | 'completed' | 'failed'
  sourceTaskIds: string[]
  sourceArtifactIds: string[]
  outputArtifactIds: string[]
  version: number
  createdAt: string
  updatedAt: string
}

export type ApiAuditEvent = {
  id: string
  projectId: string
  actor: string
  action: string
  entityType: 'project' | 'business_task' | 'artifact' | 'generation_job' | 'change_set'
  entityId: string
  metadata: Record<string, unknown>
  createdAt: string
}

export type ApiOperationalRecordKind =
  | 'work_item'
  | 'evidence'
  | 'activity'
  | 'metric'
  | 'performance_ad'
  | 'audience_mix'
  | 'method'
  | 'delivery_diagnostic'
  | 'delivery_action'
  | 'unified_record'

export type ApiOperationalRecord = {
  id: string
  projectId: string
  kind: ApiOperationalRecordKind
  title: string
  status: string
  occurredAt: string
  fields: Record<string, string | number>
  createdAt: string
  updatedAt: string
}

export type ApiProviderCapabilities = {
  provider: string
  status: 'configured' | 'not_configured'
  capabilities: Array<{
    capability: string
    model: string
    available: boolean
  }>
  checkedAt: string
}

const apiBase = `${import.meta.env.VITE_API_BASE_URL ?? 'http://127.0.0.1:8787'}/api`

async function request<T>(path: string, method = 'GET', body?: unknown): Promise<T> {
  const response = await fetch(`${apiBase}${path}`, {
    method,
    headers: body === undefined ? undefined : { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  const payload = await response.json() as T | { error?: { message?: string } }
  if (!response.ok) {
    const error = payload as { error?: { message?: string } }
    throw new Error(error.error?.message ?? 'API 请求失败')
  }
  return payload as T
}

function projectQuery(projectId?: string): string {
  return projectId ? `?projectId=${encodeURIComponent(projectId)}` : ''
}

function prerollQuery(scope: ApiPrerollScope): string {
  const search = new URLSearchParams({
    projectId: scope.projectId,
    purpose: scope.purpose,
    prerollType: scope.prerollType,
  })
  return `?${search.toString()}`
}

export const api = {
  getCapabilities: () => request<ApiProviderCapabilities>('/provider/capabilities'),
  listProjects: () => request<ApiProject[]>('/projects'),
  createProject: (input: Pick<ApiProject, 'name' | 'brand' | 'objective'>) =>
    request<ApiProject>('/projects', 'POST', input),
  updateProject: (id: string, input: Partial<Pick<ApiProject, 'name' | 'brand' | 'objective'>>) =>
    request<ApiProject>(`/projects/${encodeURIComponent(id)}`, 'PATCH', input),
  listArtifacts: (projectId?: string) =>
    request<ApiArtifact[]>(`/artifacts${projectQuery(projectId)}`),
  listPrerollArtifacts: (scope: ApiPrerollScope) =>
    request<ApiArtifact[]>(`/artifacts${prerollQuery(scope)}`),
  listTasks: (projectId?: string) =>
    request<ApiBusinessTask[]>(`/tasks${projectQuery(projectId)}`),
  getTask: (id: string) =>
    request<ApiBusinessTask>(`/tasks/${encodeURIComponent(id)}`),
  createTask: (input: {
    projectId: string
    type: ApiBusinessTaskType
    name: string
    objective: string
    sourceTaskIds?: string[]
    sourceArtifactIds?: string[]
  }) => request<ApiBusinessTask>('/tasks', 'POST', input),
  updateTask: (
    id: string,
    input: Partial<Pick<ApiBusinessTask, 'name' | 'objective' | 'status' | 'sourceTaskIds' | 'sourceArtifactIds' | 'outputArtifactIds'>>,
  ) => request<ApiBusinessTask>(`/tasks/${encodeURIComponent(id)}`, 'PATCH', input),
  createArtifact: (input: {
    projectId: string
    kind: ApiArtifact['kind']
    content: string
    status?: ApiArtifact['status']
    sourceJobId?: string
  }) => request<ApiArtifact>('/artifacts', 'POST', input),
  updateArtifact: (
    id: string,
    input: Partial<Pick<ApiArtifact, 'content' | 'status' | 'sourceJobId'>>,
  ) => request<ApiArtifact>(`/artifacts/${encodeURIComponent(id)}`, 'PATCH', input),
  listJobs: (projectId?: string) =>
    request<ApiGenerationJob[]>(`/generation-jobs${projectQuery(projectId)}`),
  listPrerollJobs: (scope: ApiPrerollScope) =>
    request<ApiGenerationJob[]>(`/generation-jobs${prerollQuery(scope)}`),
  getJob: (id: string) =>
    request<ApiGenerationJob>(`/generation-jobs/${encodeURIComponent(id)}`),
  getPrerollJob: (id: string, scope: ApiPrerollScope) =>
    request<ApiGenerationJob>(`/generation-jobs/${encodeURIComponent(id)}${prerollQuery(scope)}`),
  cancelJob: (id: string, scope?: ApiPrerollScope) =>
    request<ApiGenerationJob>(
      `/generation-jobs/${encodeURIComponent(id)}/cancel${scope ? prerollQuery(scope) : ''}`,
      'POST',
    ),
  generateBrief: (projectId: string, prompt: string) =>
    request<{ job: ApiGenerationJob; artifact: ApiArtifact }>('/generation/text', 'POST', {
      projectId,
      prompt,
    }),
  createMedia: (
    projectId: string,
    kind: 'image' | 'video',
    prompt: string,
    briefId: string,
  ) => request<ApiGenerationJob>('/generation/media', 'POST', {
    projectId,
    kind,
    prompt,
    briefId,
  }),
  createPrerollVideo: (
    scope: ApiPrerollScope,
    prompt: string,
    briefId: string,
  ) => request<ApiGenerationJob>('/generation/media', 'POST', {
    projectId: scope.projectId,
    kind: 'video',
    purpose: scope.purpose,
    prerollType: scope.prerollType,
    prompt,
    briefId,
  }),
  listAuditEvents: (projectId?: string) =>
    request<ApiAuditEvent[]>(`/audit-events${projectQuery(projectId)}`),
  listOperations: (projectId: string) =>
    request<ApiOperationalRecord[]>(`/projects/${encodeURIComponent(projectId)}/operations`),
}
