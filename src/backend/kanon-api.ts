import type {
  ApiAgencyWorkbench,
  ApiArtifact,
  ApiBusinessTask,
  ApiGenerationJob,
  ApiProject,
  ApiProviderCapabilities,
} from '../data/api'
import {
  apiRequest,
  buildAgencyWorkbench,
  createBackendProject,
  enrichProjectRecord,
  loadWorkspaceBootstrap,
  toProjectRecord,
  type BackendProject,
} from './platform'

type ListResponse<T> = { items: T[] }

type StrategyPackage = {
  package_id: string
  version: number
  status: string
  content_hash: string
  published_at?: string
  snapshot?: Record<string, unknown>
}

type ProjectAsset = {
  asset: {
    id: string
    status: string
    created_at?: string
    updated_at: string
  }
  version: {
    version: number
    mime_type: string
    source_type: string
    provider_job_id?: string
    created_at: string
  }
  created_at?: string
}

type ProviderJob = {
  id: string
  kind: string
  project_id: string
  execution_status: 'queued' | 'running' | 'succeeded' | 'failed' | 'cancelled'
  provider_status: string
  progress: number
  project_asset_refs: Array<{
    project_id: string
    asset_version: {
      asset_id: string
      version: number
    }
  }>
  error: null | {
    code: string
    message: string
    retryable: boolean
  }
  version: number
  created_at: string
  updated_at: string
}

const jobProjects = new Map<string, string>()

export async function loadKanonAgencyWorkbench(): Promise<ApiAgencyWorkbench> {
  const bootstrap = await loadWorkspaceBootstrap()
  const projects = await Promise.all(
    bootstrap.projects.map(async project => {
      const base = toProjectRecord(project, bootstrap.identity)
      return enrichProjectRecord(base).catch(() => base)
    }),
  )
  return buildAgencyWorkbench(bootstrap.identity, bootstrap.projects, projects)
}

export async function listKanonProjects(): Promise<ApiProject[]> {
  const bootstrap = await loadWorkspaceBootstrap()
  const projects = await Promise.all(
    bootstrap.projects.map(async project => {
      const base = toProjectRecord(project, bootstrap.identity)
      return enrichProjectRecord(base).catch(() => base)
    }),
  )
  return projects.map((project, index) => {
    const source = bootstrap.projects[index]
    return {
      id: project.id,
      name: project.name,
      brand: project.brand,
      objective: project.goal,
      runtime: {
        code: project.code,
        product: project.product,
        stage: project.stage,
        progress: project.progress,
        status: project.status === '已完成' ? 'completed' : 'active',
        owner: project.owner,
        budget: project.budget,
        currency: project.currency,
        timezone: project.timezone,
      },
      version: source.project_context_version,
      createdAt: source.created_at,
      updatedAt: source.updated_at,
    }
  })
}

export async function createKanonProject(
  input: Pick<ApiProject, 'name' | 'brand' | 'objective'>,
): Promise<ApiProject> {
  const created = await createBackendProject(input)
  return mapBackendProject(created, input.objective)
}

export async function listKanonArtifacts(projectId: string): Promise<ApiArtifact[]> {
  const encodedProjectId = encodeURIComponent(projectId)
  const [packageResult, assetResult] = await Promise.allSettled([
    apiRequest<ListResponse<StrategyPackage>>(
      `/api/strategy/v1/projects/${encodedProjectId}/strategy-packages`,
    ),
    apiRequest<ListResponse<ProjectAsset>>(
      `/platform/v1/projects/${encodedProjectId}/assets?limit=100`,
    ),
  ])

  const packages = packageResult.status === 'fulfilled' ? packageResult.value.items : []
  const assets = assetResult.status === 'fulfilled' ? assetResult.value.items : []
  const strategyArtifacts = packages.map<ApiArtifact>(item => {
    const timestamp = item.published_at ?? new Date(0).toISOString()
    return {
      id: item.package_id,
      projectId,
      kind: 'brief',
      status: item.status === 'archived' ? 'archived' : 'ready',
      content: summarizeStrategyPackage(item),
      version: item.version,
      createdAt: timestamp,
      updatedAt: timestamp,
    }
  })
  const assetArtifacts = assets.map<ApiArtifact>(item => {
    const kind = artifactKindFromMime(item.version.mime_type)
    const timestamp = item.version.created_at || item.asset.updated_at
    if (item.version.provider_job_id) {
      jobProjects.set(item.version.provider_job_id, projectId)
    }
    return {
      id: item.asset.id,
      projectId,
      kind,
      status: item.asset.status === 'ready' ? 'ready' : 'draft',
      content: assetContentUrl(projectId, item.asset.id, item.version.version),
      sourceJobId: item.version.provider_job_id,
      version: item.version.version,
      createdAt: item.asset.created_at ?? item.created_at ?? timestamp,
      updatedAt: item.asset.updated_at || timestamp,
    }
  })

  return [...strategyArtifacts, ...assetArtifacts]
    .sort((left, right) => left.createdAt.localeCompare(right.createdAt))
}

export async function listKanonJobs(projectId: string): Promise<ApiGenerationJob[]> {
  const artifacts = await listKanonArtifacts(projectId)
  return artifacts
    .filter((artifact): artifact is ApiArtifact & { sourceJobId: string } => Boolean(artifact.sourceJobId))
    .map(artifact => {
      jobProjects.set(artifact.sourceJobId, projectId)
      return {
        id: artifact.sourceJobId,
        projectId,
        artifactKind: artifact.kind,
        status: 'succeeded',
        model: artifact.kind === 'video' ? 'cookies.video.standard' : 'cookies.image.standard',
        artifactId: artifact.id,
        version: artifact.version,
        createdAt: artifact.createdAt,
        updatedAt: artifact.updatedAt,
      }
    })
}

export async function getKanonJob(jobId: string): Promise<ApiGenerationJob> {
  const projectId = jobProjects.get(jobId)
  if (!projectId) {
    throw new Error('当前页面没有该模型作业所属的 Project，无法读取作业状态。')
  }
  const job = await apiRequest<ProviderJob>(
    `/platform/v1/projects/${encodeURIComponent(projectId)}/model/jobs/${encodeURIComponent(jobId)}`,
  )
  return mapProviderJob(job)
}

export async function createKanonMedia(
  projectId: string,
  kind: 'image' | 'video',
  prompt: string,
  briefId: string,
): Promise<ApiGenerationJob> {
  const bootstrap = await loadWorkspaceBootstrap()
  const project = bootstrap.projects.find(item => item.id === projectId)
  if (!project) {
    throw new Error(`当前身份无法访问 Project ${projectId}。`)
  }
  const capability = kind === 'video' ? 'video.generate' : 'image.generate'
  const modelAlias = kind === 'video' ? 'cookies.video.standard' : 'cookies.image.standard'
  const input = kind === 'video'
    ? {
        prompt,
        duration_seconds: 6,
        aspect_ratio: '9:16',
        resolution: '720p',
      }
    : {
        prompt,
        width: 1024,
        height: 1024,
      }
  const job = await apiRequest<ProviderJob>(
    `/platform/v1/projects/${encodeURIComponent(projectId)}/model/jobs`,
    {
      method: 'POST',
      headers: {
        'Idempotency-Key': browserIdempotencyKey(kind),
      },
      body: JSON.stringify({
        capability,
        model_alias: modelAlias,
        input,
        project_context_version: project.project_context_version,
        source_system: 'kanon-frontend',
        source_task_id: briefId,
      }),
    },
  )
  jobProjects.set(job.id, projectId)
  return mapProviderJob(job, kind)
}

export async function getKanonCapabilities(): Promise<ApiProviderCapabilities> {
  await apiRequest<{ status: string }>('/readyz')
  return {
    provider: 'cookies-provider-gateway',
    status: 'configured',
    capabilities: [
      { capability: 'image.generate', model: 'cookies.image.standard', available: true },
      { capability: 'video.generate', model: 'cookies.video.standard', available: true },
    ],
    checkedAt: new Date().toISOString(),
  }
}

export async function listKanonTasks(projectId: string): Promise<ApiBusinessTask[]> {
  const bootstrap = await loadWorkspaceBootstrap()
  const backendProject = bootstrap.projects.find(project => project.id === projectId)
  if (!backendProject) return []
  const project = await enrichProjectRecord(toProjectRecord(backendProject, bootstrap.identity))
  return project.tasks
}

export function unsupportedKanonWrite(action: string): Error {
  return new Error(`${action}尚未接入当前 Go 后端；页面入口已保留，待对应领域契约实现后启用。`)
}

function mapBackendProject(project: BackendProject, objective: string): ApiProject {
  return {
    id: project.id,
    name: project.name,
    brand: project.primary_brand_id ?? '尚未绑定品牌',
    objective,
    runtime: {
      code: project.id,
      product: '项目产品',
      stage: project.status === 'active' ? '进行中' : '准备中',
      progress: 0,
      status: project.status === 'archived' ? 'completed' : 'active',
      owner: project.organization_id,
      budget: 0,
      currency: 'CNY',
      timezone: 'Asia/Shanghai',
    },
    version: project.project_context_version,
    createdAt: project.created_at,
    updatedAt: project.updated_at,
  }
}

function mapProviderJob(job: ProviderJob, requestedKind?: 'image' | 'video'): ApiGenerationJob {
  const asset = job.project_asset_refs.at(-1)?.asset_version
  const kind = requestedKind ?? (job.kind.includes('video') ? 'video' : 'image')
  const status = normalizeJobStatus(job.execution_status, job.provider_status)
  return {
    id: job.id,
    projectId: job.project_id,
    artifactKind: kind,
    status,
    model: kind === 'video' ? 'cookies.video.standard' : 'cookies.image.standard',
    diagnostic: job.error?.message,
    artifactId: asset?.asset_id,
    version: job.version,
    createdAt: job.created_at,
    updatedAt: job.updated_at,
  }
}

function normalizeJobStatus(
  executionStatus: ProviderJob['execution_status'],
  providerStatus: string,
): ApiGenerationJob['status'] {
  if (executionStatus === 'failed' || providerStatus === 'failed' || providerStatus === 'expired') {
    return 'failed'
  }
  if (executionStatus === 'cancelled' || providerStatus === 'cancelled') return 'cancelled'
  if (executionStatus === 'succeeded' || providerStatus === 'succeeded' || providerStatus === 'partially_succeeded') {
    return 'succeeded'
  }
  if (executionStatus === 'running' || providerStatus !== 'submitted') return 'running'
  return 'queued'
}

function summarizeStrategyPackage(item: StrategyPackage): string {
  const strategy = asRecord(item.snapshot?.strategy)
  const brief = asRecord(item.snapshot?.brief)
  const candidates = [
    strategy?.objective,
    strategy?.core_message,
    brief?.objective,
    brief?.summary,
  ].filter((value): value is string => typeof value === 'string' && value.trim().length > 0)
  return candidates[0] ?? `已批准策略包 ${item.package_id} v${item.version}`
}

function asRecord(value: unknown): Record<string, unknown> | undefined {
  return value !== null && typeof value === 'object' && !Array.isArray(value)
    ? value as Record<string, unknown>
    : undefined
}

function artifactKindFromMime(mimeType: string): ApiArtifact['kind'] {
  if (mimeType.startsWith('video/')) return 'video'
  if (mimeType.startsWith('image/')) return 'image'
  return 'document'
}

function assetContentUrl(projectId: string, assetId: string, version: number): string {
  return `/platform/v1/projects/${encodeURIComponent(projectId)}/assets/${encodeURIComponent(assetId)}/versions/${version}/content`
}

function browserIdempotencyKey(kind: 'image' | 'video'): string {
  const random = globalThis.crypto?.randomUUID?.().replaceAll('-', '') ?? `${Date.now()}`
  return `kanon-${kind}-${random}`.slice(0, 120)
}
