import type { ApiAgencyWorkbench, ApiOperationalRecord } from '../data/api.js'
import type {
  ArtifactKey,
  BusinessTaskRecord,
  ProjectArtifact,
  ProjectRecord,
} from '../types.js'

type Problem = {
  error?: {
    code?: string
    message?: string
  }
}

export class BackendApiError extends Error {
  readonly code: string
  readonly status: number

  constructor(message: string, code: string, status: number) {
    super(message)
    this.name = 'BackendApiError'
    this.code = code
    this.status = status
  }
}

export type BackendIdentity = {
  actor: {
    organization_id: string
    principal: { kind: 'user' | 'service'; id: string }
    scopes: string[]
  }
  organization: {
    id: string
    name: string
    status: string
  }
  user: null | {
    id: string
    display_name: string
  }
  membership: null | {
    organization_id: string
    user_id: string
    role: 'owner' | 'admin' | 'member' | 'auditor'
    status: string
    updated_at: string
  }
}

export type BackendOrganizationAccess = {
  organization: BackendIdentity['organization']
  membership: NonNullable<BackendIdentity['membership']>
}

export type BackendOrganizationMember = {
  user: NonNullable<BackendIdentity['user']> & { status: string; updated_at: string }
  membership: NonNullable<BackendIdentity['membership']>
}

export type BackendProjectMembership = {
  organization_id: string
  project_id: string
  principal_kind: 'user' | 'service'
  principal_id: string
  display_name: string
  role: 'owner' | 'editor' | 'viewer' | 'worker'
  status: 'active' | 'suspended' | 'removed'
  created_at: string
  updated_at: string
}

export const accountApi = {
  listOrganizations: () => apiRequest<ListResponse<BackendOrganizationAccess>>('/platform/v1/organizations'),
  updateProfile: (displayName: string) => apiRequest<NonNullable<BackendIdentity['user']>>('/platform/v1/me', {
    method: 'PATCH',
    body: JSON.stringify({ display_name: displayName }),
  }),
  listOrganizationMembers: (organizationId: string) =>
    apiRequest<ListResponse<BackendOrganizationMember>>(`/platform/v1/organizations/${encodeURIComponent(organizationId)}/members`),
  addOrganizationMember: (organizationId: string, userId: string, role: string) =>
    apiRequest<BackendOrganizationMember>(`/platform/v1/organizations/${encodeURIComponent(organizationId)}/members`, {
      method: 'POST',
      body: JSON.stringify({ user_id: userId, role }),
    }),
  updateOrganizationMember: (organizationId: string, userId: string, input: { role: string; status: string; expected_updated_at: string }) =>
    apiRequest<BackendOrganizationMember>(`/platform/v1/organizations/${encodeURIComponent(organizationId)}/members/${encodeURIComponent(userId)}`, {
      method: 'PATCH',
      body: JSON.stringify(input),
    }),
  listProjectMembers: (projectId: string) =>
    apiRequest<ListResponse<BackendProjectMembership>>(`/platform/v1/projects/${encodeURIComponent(projectId)}/members`),
  addProjectMember: (projectId: string, input: { principal_kind: 'user' | 'service'; principal_id: string; role: string }) =>
    apiRequest<BackendProjectMembership>(`/platform/v1/projects/${encodeURIComponent(projectId)}/members`, {
      method: 'POST',
      body: JSON.stringify(input),
    }),
  updateProjectMember: (projectId: string, member: BackendProjectMembership, input: { role: string; status: string }) =>
    apiRequest<BackendProjectMembership>(`/platform/v1/projects/${encodeURIComponent(projectId)}/members/${member.principal_kind}/${encodeURIComponent(member.principal_id)}`, {
      method: 'PATCH',
      body: JSON.stringify({ ...input, expected_updated_at: member.updated_at }),
    }),
}

export type BackendProject = {
  id: string
  organization_id: string
  name: string
  status: 'draft' | 'active' | 'archived'
  industry?: 'short_drama' | 'game' | 'ecommerce' | 'automotive_brand'
  primary_brand_id: string | null
  project_context_version: number
  created_at: string
  updated_at: string
}

type ListResponse<T> = { items: T[] }

type StrategyPackage = {
  package_id: string
  version: number
  status: string
  content_hash: string
  snapshot?: Record<string, unknown>
}

type StrategyBriefVersion = {
  brief_id: string
  version: number
  content_hash: string
  confirmed_at: string
  snapshot?: Record<string, unknown>
}

type CreativeTask = {
  id: string
  project_id: string
  format: 'image_text' | 'video'
  status: string
  direction?: {
    focus?: string
    concept?: string
    core_message?: string
  }
  version: number
  created_at: string
  updated_at: string
}

type CreativePackage = {
  id: string
  creative_version_id: string
  content_hash: string
  format?: 'image_text' | 'video'
  created_at: string
}

type ProjectAsset = {
  asset: {
    id: string
    status: string
    updated_at: string
  }
  version: {
    version: number
    mime_type: string
    source_type: string
    provider_job_id?: string
    created_at: string
  }
}

type DeliveryPlan = {
  id: string
  name: string
  status: string
  version: number
  created_at: string
  updated_at: string
}

type DeliveryExecution = {
  execution: {
    id: string
    status: string
    created_at: string
  }
}

type InsightReport = {
  id: string
  status: string
  summary: string
  version: number
  created_at: string
  updated_at: string
}

type Experience = {
  id: string
  conclusion: string
  created_at: string
}

export type WorkspaceBootstrap = {
  identity: BackendIdentity
  projects: BackendProject[]
}

export async function apiRequest<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers)
  if (!headers.has('Accept')) headers.set('Accept', 'application/json')
  if (typeof init.body === 'string' && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json')
  }

  const response = await fetch(path, { ...init, credentials: 'include', headers })
  if (!response.ok) {
    const problem = await response.json().catch(() => null) as Problem | null
    throw new BackendApiError(
      problem?.error?.message ?? `请求失败（HTTP ${response.status}）`,
      problem?.error?.code ?? 'HTTP_ERROR',
      response.status,
    )
  }
  if (response.status === 204) return undefined as T
  return response.json() as Promise<T>
}

export async function loadWorkspaceBootstrap(signal?: AbortSignal): Promise<WorkspaceBootstrap> {
  const [identity, projectList] = await Promise.all([
    apiRequest<BackendIdentity>('/platform/v1/me', { signal }),
    apiRequest<ListResponse<BackendProject>>('/platform/v1/projects', { signal }),
  ])
  return { identity, projects: projectList.items }
}

export async function createBackendProject(input: {
  name: string
  brand: string
}): Promise<BackendProject> {
  const brand = await apiRequest<{ id: string }>('/platform/v1/brands', {
    method: 'POST',
    body: JSON.stringify({ name: input.brand || `${input.name} 品牌` }),
  })
  return apiRequest<BackendProject>('/platform/v1/projects', {
    method: 'POST',
    body: JSON.stringify({
      name: input.name,
      primary_brand_id: brand.id,
      product_ids: [],
      activate: true,
    }),
  })
}

export function toProjectRecord(project: BackendProject, identity: BackendIdentity): ProjectRecord {
  return {
    id: project.id,
    version: project.project_context_version,
    code: project.id,
    name: project.name,
    brand: project.primary_brand_id ?? '尚未绑定品牌',
    product: '项目产品',
    goal: '在同一 ProjectContext 中完成策略、创意、投放和洞察闭环。',
    industry: project.industry ?? 'ecommerce',
    stage: project.status === 'active' ? '进行中' : project.status === 'archived' ? '已归档' : '准备中',
    progress: 0,
    status: project.status === 'archived' ? '已完成' : '进行中',
    owner: identity.user?.display_name ?? identity.actor.organization_id,
    updatedAt: formatDate(project.updated_at),
    budget: 0,
    currency: 'CNY',
    timezone: 'Asia/Shanghai',
    artifacts: emptyArtifacts(project.updated_at),
    tasks: [],
    changeSets: [],
    operations: [],
    knowledgeCount: 0,
  }
}

export async function enrichProjectRecord(
  base: ProjectRecord,
  signal?: AbortSignal,
): Promise<ProjectRecord> {
  const projectId = encodeURIComponent(base.id)
  const results = await Promise.allSettled([
    apiRequest<ListResponse<StrategyPackage>>(`/api/strategy/v1/projects/${projectId}/strategy-packages`, { signal }),
    apiRequest<ListResponse<CreativeTask>>(`/api/creative/v1/projects/${projectId}/creative-tasks?limit=100`, { signal }),
    apiRequest<ListResponse<CreativePackage>>(`/api/creative/v1/projects/${projectId}/creative-packages`, { signal }),
    apiRequest<ListResponse<ProjectAsset>>(`/platform/v1/projects/${projectId}/assets?limit=100`, { signal }),
    apiRequest<ListResponse<DeliveryPlan>>(`/api/delivery/v1/projects/${projectId}/plans`, { signal }),
    apiRequest<ListResponse<DeliveryExecution>>(`/api/delivery/v1/projects/${projectId}/executions`, { signal }),
    apiRequest<ListResponse<InsightReport>>(`/api/insights/v1/projects/${projectId}/reports`, { signal }),
    apiRequest<ListResponse<Experience>>(`/api/insights/v1/projects/${projectId}/experiences`, { signal }),
    apiRequest<ListResponse<StrategyBriefVersion>>(`/api/strategy/v1/projects/${projectId}/brief-versions`, { signal }),
  ])

  const packages = fulfilled(results[0])?.items ?? []
  const creativeTasks = fulfilled(results[1])?.items ?? []
  const creativePackages = fulfilled(results[2])?.items ?? []
  const assets = fulfilled(results[3])?.items ?? []
  const deliveryPlans = fulfilled(results[4])?.items ?? []
  const executions = fulfilled(results[5])?.items ?? []
  const reports = fulfilled(results[6])?.items ?? []
  const experiences = fulfilled(results[7])?.items ?? []
  const briefVersions = fulfilled(results[8])?.items ?? []

  const latestStrategy = packages[0]
  const latestBrief = briefVersions[0]
  const latestCreativePackage = creativePackages[0]
  const latestAsset = assets[0]
  const latestPlan = deliveryPlans[0]
  const latestReport = reports[0]
  const tasks = creativeTasks.map(toBusinessTask)
  const progress = calculateProgress({
    packages,
    creativeTasks,
    creativePackages,
    assets,
    deliveryPlans,
    executions,
    reports,
    experiences,
  })

  return {
    ...base,
    stage: progress.stage,
    progress: progress.percent,
    updatedAt: newestDate([
      base.updatedAt,
      latestStrategy ? base.updatedAt : '',
      latestAsset?.asset.updated_at,
      latestPlan?.updated_at,
      latestReport?.updated_at,
    ]),
    artifacts: {
      brief: artifact('brief', latestBrief?.brief_id, latestBrief ? `Brief ${latestBrief.brief_id} v${latestBrief.version} · ${latestBrief.content_hash}` : '尚未确认 Brief', latestBrief ? '已确认' : '草稿', latestBrief?.confirmed_at ?? base.updatedAt),
      strategy: artifact('strategy', latestStrategy?.package_id, latestStrategy ? `StrategyPackage · ${latestStrategy.content_hash}` : '尚未发布策略版本', latestStrategy ? '已确认' : '待确认', base.updatedAt),
      creative: artifact('creative', latestCreativePackage?.id ?? latestAsset?.asset.id, latestCreativePackage ? `CreativePackage · ${latestCreativePackage.id}` : latestAsset ? `${latestAsset.version.mime_type} · ${latestAsset.asset.id}` : '尚未生成创意资产', latestCreativePackage || latestAsset ? '已完成' : creativeTasks.length ? '制作中' : '待确认', latestCreativePackage?.created_at ?? latestAsset?.asset.updated_at ?? base.updatedAt),
      delivery: artifact('delivery', latestPlan?.id, latestPlan ? `${latestPlan.name} · ${latestPlan.status}` : '尚未创建投放计划', executions.length ? '已完成' : latestPlan ? '执行中' : '待确认', latestPlan?.updated_at ?? base.updatedAt),
      insight: artifact('insight', latestReport?.id, latestReport?.summary ?? (experiences[0]?.conclusion || '尚未形成复盘洞察'), experiences.length ? '已确认' : latestReport ? '草稿' : '待确认', latestReport?.updated_at ?? experiences[0]?.created_at ?? base.updatedAt),
    },
    tasks,
    operations: buildOperations(base.id, creativeTasks, deliveryPlans, reports),
    knowledgeCount: experiences.length,
  }
}

export function emptyAgencyWorkbench(): ApiAgencyWorkbench {
  return {
    organizations: [],
    clients: [],
    brands: [],
    projects: [],
    adAccountBindings: [],
    qualityCheckRuns: [],
    materialConfirmations: [],
    assetVersionPointers: [],
  }
}

export function buildAgencyWorkbench(
  identity: BackendIdentity,
  backendProjects: BackendProject[],
  projects: ProjectRecord[],
): ApiAgencyWorkbench {
  const organizationId = identity.organization.id
  const owner = identity.user?.display_name ?? identity.organization.name
  const updatedAt = newestDate(backendProjects.map((project) => project.updated_at))
  const brandIds = [...new Set(
    backendProjects
      .map((project) => project.primary_brand_id)
      .filter((brandId): brandId is string => Boolean(brandId)),
  )]

  return {
    organizations: [{
      id: organizationId,
      code: organizationId,
      name: identity.organization.name,
      owner,
      currency: 'CNY',
      timezone: 'Asia/Shanghai',
      updatedAt,
    }],
    // Client is not part of the current Go platform contract. Keep it unassigned
    // instead of inventing a second owner for project state.
    clients: [],
    brands: brandIds.map((brandId) => ({
      id: brandId,
      organizationId,
      clientId: '',
      code: brandId,
      name: brandId,
      category: '尚未配置',
      productLines: [],
      owner,
      guidelineStatus: 'missing',
      updatedAt,
    })),
    projects: projects.map((project) => {
      const backendProject = backendProjects.find((item) => item.id === project.id)
      const progress = toAgencyProgress(project)
      return {
        id: project.id,
        organizationId: backendProject?.organization_id ?? organizationId,
        clientId: '',
        brandId: backendProject?.primary_brand_id ?? '',
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
        progressDetail: progress,
        version: backendProject?.project_context_version ?? 1,
        createdAt: backendProject?.created_at ?? '',
        updatedAt: backendProject?.updated_at ?? project.updatedAt,
      }
    }),
    adAccountBindings: [],
    qualityCheckRuns: [],
    materialConfirmations: [],
    assetVersionPointers: [],
  }
}

function toAgencyProgress(project: ProjectRecord): ApiAgencyWorkbench['projects'][number]['progressDetail'] {
  const stage = project.status === '已完成'
    ? 'completed'
    : project.artifacts.delivery.id
      ? 'delivery'
      : project.artifacts.creative.status === '已确认'
        ? 'human_review'
        : project.artifacts.creative.id
          ? 'quality_check'
          : project.tasks.length || project.artifacts.strategy.id
            ? 'creative'
            : project.artifacts.brief.id
              ? 'strategy'
              : 'intake'
  const stageLabels = {
    intake: '需求下达',
    strategy: '策略确认',
    creative: '创意制作',
    quality_check: '素材质检',
    human_review: '人工确认',
    delivery: '投放预检',
    completed: '项目完成',
  } as const
  const hasFailedTask = project.tasks.some((task) => task.status === 'failed')
  const hasActiveTask = project.tasks.some((task) => ['draft', 'in_progress', 'ready'].includes(task.status))

  return {
    stage,
    stageLabel: stageLabels[stage],
    stagePercent: project.progress,
    taskPercent: project.progress,
    riskStatus: hasFailedTask ? 'blocked' : hasActiveTask ? 'watch' : 'healthy',
    blocker: hasFailedTask ? '存在失败的创意生产任务。' : undefined,
    updatedAt: project.updatedAt,
  }
}

function fulfilled<T>(result: PromiseSettledResult<T>): T | undefined {
  return result.status === 'fulfilled' ? result.value : undefined
}

function toBusinessTask(task: CreativeTask): BusinessTaskRecord {
  const type = task.format === 'video' ? 'video' : 'creative'
  return {
    id: task.id,
    projectId: task.project_id,
    type,
    name: task.direction?.focus || task.direction?.concept || task.direction?.core_message || (task.format === 'video' ? '视频创意任务' : '图文创意任务'),
    objective: task.direction?.core_message || '依据已确认策略完成创意生产与交付。',
    status: task.status === 'delivered' || task.status === 'approved'
      ? 'completed'
      : task.status === 'archived'
        ? 'failed'
        : task.status === 'draft'
          ? 'draft'
          : 'in_progress',
    sourceTaskIds: [],
    sourceArtifactIds: [],
    outputArtifactIds: [],
    version: task.version,
    createdAt: task.created_at,
    updatedAt: task.updated_at,
  }
}

function emptyArtifacts(updatedAt: string): Record<ArtifactKey, ProjectArtifact> {
  return {
    brief: artifact('brief', undefined, '尚未确认 Brief', '草稿', updatedAt),
    strategy: artifact('strategy', undefined, '尚未发布策略版本', '待确认', updatedAt),
    creative: artifact('creative', undefined, '尚未生成创意资产', '待确认', updatedAt),
    delivery: artifact('delivery', undefined, '尚未创建投放计划', '待确认', updatedAt),
    insight: artifact('insight', undefined, '尚未形成复盘洞察', '待确认', updatedAt),
  }
}

function artifact(
  key: ArtifactKey,
  id: string | undefined,
  summary: string,
  status: ProjectArtifact['status'],
  updatedAt: string,
): ProjectArtifact {
  const labels: Record<ArtifactKey, string> = {
    brief: '策略 Brief',
    strategy: '策略方案',
    creative: '创意产物',
    delivery: '投放计划',
    insight: '洞察报告',
  }
  return {
    id,
    key,
    label: labels[key],
    version: id ? '已持久化' : 'v0',
    status,
    owner: 'Go 服务端',
    updatedAt: formatDate(updatedAt),
    summary,
  }
}

function calculateProgress(input: {
  packages: StrategyPackage[]
  creativeTasks: CreativeTask[]
  creativePackages: CreativePackage[]
  assets: ProjectAsset[]
  deliveryPlans: DeliveryPlan[]
  executions: DeliveryExecution[]
  reports: InsightReport[]
  experiences: Experience[]
}) {
  if (input.experiences.length) return { stage: '经验沉淀', percent: 100 }
  if (input.reports.length) return { stage: '复盘洞察', percent: 88 }
  if (input.executions.length) return { stage: '模拟投放', percent: 75 }
  if (input.deliveryPlans.length) return { stage: '投放计划', percent: 65 }
  if (input.creativePackages.length) return { stage: '创意交付', percent: 55 }
  if (input.assets.length) return { stage: '素材入库', percent: 45 }
  if (input.creativeTasks.length) return { stage: '创意生产', percent: 35 }
  if (input.packages.length) return { stage: '策略已批准', percent: 25 }
  return { stage: '需求梳理', percent: 10 }
}

function buildOperations(
  projectId: string,
  creativeTasks: CreativeTask[],
  deliveryPlans: DeliveryPlan[],
  reports: InsightReport[],
): ApiOperationalRecord[] {
  const records: ApiOperationalRecord[] = []
  for (const task of creativeTasks.slice(0, 20)) {
    records.push({
      id: task.id,
      projectId,
      kind: 'unified_record',
      title: task.direction?.focus || task.direction?.concept || '创意任务',
      status: task.status,
      occurredAt: task.updated_at,
      fields: { kind: task.format, owner: 'Creative', detail: task.status },
      createdAt: task.created_at,
      updatedAt: task.updated_at,
    })
  }
  for (const plan of deliveryPlans.slice(0, 10)) {
    records.push({
      id: plan.id,
      projectId,
      kind: 'activity',
      title: plan.name,
      status: plan.status,
      occurredAt: plan.updated_at,
      fields: { kind: 'delivery_plan', owner: 'Delivery', detail: plan.status },
      createdAt: plan.created_at,
      updatedAt: plan.updated_at,
    })
  }
  for (const report of reports.slice(0, 10)) {
    records.push({
      id: report.id,
      projectId,
      kind: 'activity',
      title: report.summary,
      status: report.status,
      occurredAt: report.updated_at,
      fields: { kind: 'insight_report', owner: 'Insights', detail: report.status },
      createdAt: report.created_at,
      updatedAt: report.updated_at,
    })
  }
  return records.sort((left, right) => right.updatedAt.localeCompare(left.updatedAt))
}

function newestDate(values: Array<string | undefined>): string {
  const normalized = values.filter((value): value is string => Boolean(value))
  return formatDate(normalized.sort((left, right) => right.localeCompare(left))[0] ?? '')
}

function formatDate(value: string): string {
  const date = new Date(value)
  return Number.isNaN(date.valueOf())
    ? value
    : date.toLocaleString('zh-CN', { hour12: false }).replace(/\//g, '-')
}
