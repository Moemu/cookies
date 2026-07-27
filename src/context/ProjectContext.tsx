import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import { api, type ApiAgencyWorkbench, type ApiArtifact, type ApiBusinessTask, type ApiBusinessTaskType, type ApiGenerationJob, type ApiOperationalRecord, type ApiProject } from '../data/api'
import type { ArtifactKey, ArtifactStatus, BusinessTaskRecord, ChangeSetRecord, ProjectArtifact, ProjectRecord } from '../types'
import { deliveryApi, type DeliveryChangeSet } from '../api/delivery'
import { presentCreativeStatus } from '../lib/media-status'

interface ProjectContextValue {
  projects: ProjectRecord[]
  currentProject: ProjectRecord
  agencyWorkbench: ApiAgencyWorkbench | null
  targetProjectId: string
  loadedProjectId: string
  isLoading: boolean
  error: string | null
  routeDiagnostic: string | null
  reloadProjects: (expectedProjectId?: string) => Promise<void>
  selectProject: (id: string) => void
  createProject: (input: Pick<ProjectRecord, 'name' | 'brand' | 'goal'>) => Promise<ProjectRecord>
  updateProject: (input: Pick<ProjectRecord, 'name' | 'brand' | 'goal'>) => Promise<void>
  createTask: (input: { type: ApiBusinessTaskType; name: string; objective: string }) => Promise<BusinessTaskRecord>
  updateTask: (id: string, patch: Partial<Pick<BusinessTaskRecord, 'name' | 'objective' | 'status' | 'sourceTaskIds' | 'sourceArtifactIds' | 'outputArtifactIds'>>) => Promise<BusinessTaskRecord>
  advanceArtifact: (key: ArtifactKey, status: ProjectRecord['artifacts'][ArtifactKey]['status']) => Promise<void>
  updateArtifact: (key: ArtifactKey, patch: Partial<ProjectRecord['artifacts'][ArtifactKey]>) => Promise<void>
  addChangeSet: (budgetLimit?: number) => Promise<DeliveryChangeSet>
  preflightChangeSet: (id: string) => Promise<DeliveryChangeSet>
  approveChangeSet: (id: string) => Promise<DeliveryChangeSet>
  executeChangeSet: (id: string) => Promise<DeliveryChangeSet>
  rollbackChangeSet: (id: string, reason: string) => Promise<DeliveryChangeSet>
}

const ProjectContext = createContext<ProjectContextValue | null>(null)

export function ProjectProvider({ children }: { children: ReactNode }) {
  const [projects, setProjects] = useState<ProjectRecord[]>([])
  const [agencyWorkbench, setAgencyWorkbench] = useState<ApiAgencyWorkbench | null>(null)
  const [targetProjectId, setTargetProjectId] = useState('')
  const [loadedProjectId, setLoadedProjectId] = useState('')
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [routeDiagnostic, setRouteDiagnostic] = useState<string | null>(null)
  const projectsRef = useRef<ProjectRecord[]>([])
  const targetProjectIdRef = useRef('')
  const loadedProjectIdRef = useRef('')
  const reloadRequestRef = useRef(0)
  const currentProject = projects.find(project => project.id === loadedProjectId) ?? emptyProject

  useEffect(() => { projectsRef.current = projects }, [projects])

  const reloadProjects = useCallback(async (expectedProjectId?: string) => {
    const requestId = reloadRequestRef.current + 1
    reloadRequestRef.current = requestId
    setIsLoading(true)
    try {
      const apiProjects = await api.listProjects()
      const workbench = await api.listAgencyWorkbench({ projectIds: apiProjects.map(project => project.id) })
      const nextProjects = await Promise.all(apiProjects.map(async project => {
        const [artifacts, jobs, tasks, changeSets, operations] = await Promise.all([
          api.listArtifacts(project.id),
          api.listJobs(project.id),
          api.listTasks(project.id),
          deliveryApi.listChangeSets(project.id),
          api.listOperations(project.id),
        ])
        return toProjectRecord(project, artifacts, jobs, tasks, changeSets, operations)
      }))
      if (reloadRequestRef.current !== requestId) {
        setRouteDiagnostic(`已忽略过期的 Project 加载响应，当前路由目标为 ${targetProjectIdRef.current || '未选择 Project'}。`)
        return
      }
      if (expectedProjectId && targetProjectIdRef.current !== expectedProjectId) {
        setRouteDiagnostic(`已忽略 Project ${expectedProjectId} 的过期响应，当前路由目标为 ${targetProjectIdRef.current || '未选择 Project'}。`)
        return
      }
      setProjects(nextProjects)
      setAgencyWorkbench(workbench)
      projectsRef.current = nextProjects
      const activeTargetId = targetProjectIdRef.current
      const nextLoadedProjectId = activeTargetId && nextProjects.some(project => project.id === activeTargetId)
        ? activeTargetId
        : !activeTargetId && nextProjects.length
          ? nextProjects[0].id
          : ''
      if (!activeTargetId && nextLoadedProjectId) {
        targetProjectIdRef.current = nextLoadedProjectId
        setTargetProjectId(nextLoadedProjectId)
      }
      loadedProjectIdRef.current = nextLoadedProjectId
      setLoadedProjectId(nextLoadedProjectId)
      setRouteDiagnostic(activeTargetId && !nextLoadedProjectId ? `路由目标 Project ${activeTargetId} 未在服务端返回结果中找到。` : null)
      setError(null)
    } catch (cause) {
      if (reloadRequestRef.current !== requestId) {
        setRouteDiagnostic(`已忽略过期的 Project 加载失败响应，当前路由目标为 ${targetProjectIdRef.current || '未选择 Project'}。`)
        return
      }
      setError(cause instanceof Error ? cause.message : '加载项目失败')
    } finally {
      if (reloadRequestRef.current === requestId) setIsLoading(false)
    }
  }, [])

  useEffect(() => { void reloadProjects() }, [reloadProjects])

  const selectProject = useCallback((id: string) => {
    targetProjectIdRef.current = id
    setTargetProjectId(id)
    setRouteDiagnostic(null)
    const alreadyLoaded = projectsRef.current.some(project => project.id === id)
    if (alreadyLoaded) {
      loadedProjectIdRef.current = id
      setLoadedProjectId(id)
      setIsLoading(false)
      return
    }
    loadedProjectIdRef.current = ''
    setLoadedProjectId('')
    setIsLoading(true)
    void reloadProjects(id)
  }, [reloadProjects])

  const createProject = useCallback(async (input: Pick<ProjectRecord, 'name' | 'brand' | 'goal'>) => {
    const created = toProjectRecord(await api.createProject({ name: input.name, brand: input.brand || '未指定品牌', objective: input.goal }))
    setProjects(current => {
      const nextProjects = [created, ...current]
      projectsRef.current = nextProjects
      return nextProjects
    })
    targetProjectIdRef.current = created.id
    loadedProjectIdRef.current = created.id
    setTargetProjectId(created.id)
    setLoadedProjectId(created.id)
    setRouteDiagnostic(null)
    return created
  }, [])

  const updateProject = useCallback(async (input: Pick<ProjectRecord, 'name' | 'brand' | 'goal'>) => {
    const project = projects.find(candidate => candidate.id === loadedProjectId)
    if (!project) throw new Error('请先选择已保存的 Project。')
    await api.updateProject(project.id, { name: input.name, brand: input.brand, objective: input.goal })
    await reloadProjects()
  }, [loadedProjectId, projects, reloadProjects])

  const createTask = useCallback(async (input: { type: ApiBusinessTaskType; name: string; objective: string }) => {
    const project = projects.find(candidate => candidate.id === loadedProjectId)
    if (!project) throw new Error('请先选择已保存的 Project。')
    const artifacts = await api.listArtifacts(project.id)
    const insightReference = artifacts
      .filter(artifact => artifact.status === 'ready' && (
        artifact.content.startsWith('[prelaunch-insight]')
        || artifact.content.startsWith('[knowledge]')
      ))
      .sort((left, right) => right.updatedAt.localeCompare(left.updatedAt))[0]
    const sourceArtifactIds = taskSourceArtifactIds(project, input.type, insightReference?.id)
    const task = await api.createTask({
      projectId: project.id,
      ...input,
      sourceTaskIds: taskSourceTaskIds(project, input.type),
      sourceArtifactIds,
    })
    await reloadProjects()
    return task
  }, [loadedProjectId, projects, reloadProjects])

  const updateTask = useCallback(async (
    id: string,
    patch: Partial<Pick<BusinessTaskRecord, 'name' | 'objective' | 'status' | 'sourceTaskIds' | 'sourceArtifactIds' | 'outputArtifactIds'>>,
  ) => {
    const task = await api.updateTask(id, patch)
    await reloadProjects()
    return task
  }, [reloadProjects])

  const updateArtifact = useCallback(async (key: ArtifactKey, patch: Partial<ProjectRecord['artifacts'][ArtifactKey]>) => {
    const project = projects.find(candidate => candidate.id === loadedProjectId)
    if (!project) throw new Error('请先选择已保存的 Project。')
    const artifact = project.artifacts[key]
    const summary = patch.summary ?? artifact.summary
    const content = key === 'brief' || key === 'creative' ? summary : `[${key}] ${summary}`
    const status = artifactStatus(patch.status ?? artifact.status)
    if (artifact.id) {
      await api.updateArtifact(artifact.id, { content, status })
    } else {
      await api.createArtifact({ projectId: project.id, kind: artifactKind(key), content, status })
    }
    await reloadProjects()
  }, [loadedProjectId, projects, reloadProjects])

  const advanceArtifact = useCallback((key: ArtifactKey, status: ProjectRecord['artifacts'][ArtifactKey]['status']) => updateArtifact(key, { status }), [updateArtifact])

  const addChangeSet = useCallback(async (budgetLimit?: number) => {
    const project = projects.find(candidate => candidate.id === loadedProjectId)
    if (!project) throw new Error('请先选择已保存的 Project。')
    const artifactIds = [project.artifacts.brief.id, project.artifacts.creative.id].filter((id): id is string => Boolean(id))
    const changeSet = await deliveryApi.createChangeSet({
      projectId: project.id,
      name: '素材组合与探索预算优化',
      artifactIds,
      budgetLimit: budgetLimit ?? project.budget,
    })
    await reloadProjects()
    return changeSet
  }, [loadedProjectId, projects, reloadProjects])

  const preflightChangeSet = useCallback(async (id: string) => {
    const changeSet = await deliveryApi.preflight(id)
    await reloadProjects()
    return changeSet
  }, [reloadProjects])
  const approveChangeSet = useCallback(async (id: string) => {
    const changeSet = await deliveryApi.approve(id)
    await reloadProjects()
    return changeSet
  }, [reloadProjects])
  const executeChangeSet = useCallback(async (id: string) => {
    const changeSet = await deliveryApi.execute(id)
    await reloadProjects()
    return changeSet
  }, [reloadProjects])
  const rollbackChangeSet = useCallback(async (id: string, reason: string) => {
    const changeSet = await deliveryApi.rollback(id, reason)
    await reloadProjects()
    return changeSet
  }, [reloadProjects])
  const value = useMemo(() => ({ projects, currentProject, agencyWorkbench, targetProjectId, loadedProjectId, isLoading, error, routeDiagnostic, reloadProjects, selectProject, createProject, updateProject, createTask, updateTask, advanceArtifact, updateArtifact, addChangeSet, preflightChangeSet, approveChangeSet, executeChangeSet, rollbackChangeSet }), [projects, currentProject, agencyWorkbench, targetProjectId, loadedProjectId, isLoading, error, routeDiagnostic, reloadProjects, selectProject, createProject, updateProject, createTask, updateTask, advanceArtifact, updateArtifact, addChangeSet, preflightChangeSet, approveChangeSet, executeChangeSet, rollbackChangeSet])
  return <ProjectContext.Provider value={value}>{children}</ProjectContext.Provider>
}

function toProjectRecord(project: ApiProject, artifacts: ApiArtifact[] = [], jobs: ApiGenerationJob[] = [], tasks: ApiBusinessTask[] = [], changeSets: DeliveryChangeSet[] = [], operations: ApiOperationalRecord[] = []): ProjectRecord {
  const brief = latestArtifact(artifacts.filter(artifact => artifact.status === 'ready'), 'brief')
    ?? latestArtifact(artifacts, 'brief')
  const latestMediaJob = latestMainCreativeGenerationJob(jobs)
  const media = latestMediaJob?.artifactId
    ? artifacts.find(artifact => artifact.id === latestMediaJob.artifactId)
    : latestMainCreativeArtifact(artifacts)
  const updatedAt = formatDate(project.updatedAt)
  const documents = artifacts.filter(artifact => artifact.kind === 'document')
  return {
    id: project.id,
    code: project.runtime.code,
    name: project.name,
    brand: project.brand,
    product: project.runtime.product,
    goal: project.objective,
    stage: project.runtime.stage,
    progress: project.runtime.progress,
    status: project.runtime.status === 'completed' ? '已完成' : '进行中',
    owner: project.runtime.owner,
    updatedAt,
    budget: project.runtime.budget,
    currency: project.runtime.currency,
    timezone: project.runtime.timezone,
    artifacts: {
      brief: toArtifactRecord('brief', brief, project.updatedAt),
      strategy: toArtifactRecord('strategy', latestDocument(documents, 'strategy'), project.updatedAt),
      creative: toCreativeRecord(media, latestMediaJob, project.updatedAt),
      insight: toArtifactRecord('insight', latestDocument(documents, 'insight'), project.updatedAt),
      delivery: toArtifactRecord('delivery', latestDocument(documents, 'delivery'), project.updatedAt),
    },
    tasks,
    changeSets: changeSets.map(toChangeSetRecord),
    operations,
    knowledgeCount: documents.filter(artifact => artifact.content.startsWith('[knowledge]')).length,
  }
}

const emptyProject: ProjectRecord = {
  id: '', code: '—', name: '尚未连接到服务端', brand: '—', product: '—', goal: '请启动本地 MVP API 后重试。',
  stage: '等待恢复', progress: 0, status: '进行中', owner: '—', updatedAt: '—', budget: 0, currency: 'CNY', timezone: 'Asia/Shanghai',
  artifacts: Object.fromEntries((['brief', 'strategy', 'creative', 'insight', 'delivery'] as ArtifactKey[]).map(key => [key, toArtifactRecord(key, undefined, '')])) as ProjectRecord['artifacts'],
  tasks: [],
  changeSets: [],
  operations: [],
  knowledgeCount: 0,
}

function taskSourceArtifactIds(project: ProjectRecord, type: ApiBusinessTaskType, insightReferenceId?: string): string[] {
  const briefId = project.artifacts.brief.id
  const strategyId = project.artifacts.strategy.id
  const creativeId = project.artifacts.creative.id
  if (type === 'strategy') return [briefId, insightReferenceId].filter((id): id is string => Boolean(id))
  if (type === 'video_edit') return [briefId, strategyId, creativeId, insightReferenceId].filter((id): id is string => Boolean(id))
  return [briefId, strategyId, insightReferenceId].filter((id): id is string => Boolean(id))
}

function taskSourceTaskIds(project: ProjectRecord, type: ApiBusinessTaskType): string[] {
  if (type === 'strategy') return []
  const latestStrategy = project.tasks.filter(task => task.type === 'strategy').at(-1)
  if (type === 'creative') return [latestStrategy?.id].filter((id): id is string => Boolean(id))
  const latestCreative = project.tasks.filter(task => task.type === 'creative').at(-1)
  return [latestStrategy?.id, latestCreative?.id].filter((id): id is string => Boolean(id))
}

function toArtifactRecord(key: ArtifactKey, artifact: ApiArtifact | undefined, fallbackTime: string): ProjectArtifact {
  return {
    id: artifact?.id, key, label: artifactLabel(key), version: `v${artifact?.version ?? 0}.0`,
    status: (artifact?.status === 'ready' ? '已确认' : artifact ? '草稿' : key === 'brief' ? '草稿' : '待确认') as ArtifactStatus,
    owner: '服务端存档', updatedAt: formatDate(artifact?.updatedAt ?? fallbackTime),
    summary: artifact?.content ?? '尚无服务端产物。',
    sourceVersion: artifact?.sourceJobId ? `生成任务 ${artifact.sourceJobId.slice(0, 8)}` : undefined,
  }
}

function toCreativeRecord(artifact: ApiArtifact | undefined, job: ApiGenerationJob | undefined, fallbackTime: string): ProjectArtifact {
  const record = toArtifactRecord('creative', artifact, fallbackTime)
  const presentation = presentCreativeStatus(artifact, job, formatDate)
  return {
    ...record,
    ...presentation,
  }
}

function toChangeSetRecord(changeSet: DeliveryChangeSet): ChangeSetRecord {
  const statusMap: Record<DeliveryChangeSet['status'], ChangeSetRecord['status']> = { draft: '草稿', preflight_passed: '待审批', preflight_failed: '草稿', approved: '已批准', rejected: '已拒绝', executing: '执行中', executed: '已执行', rolled_back: '已回滚' }
  return { id: changeSet.id, title: changeSet.name, status: statusMap[changeSet.status], risk: '中', budgetImpact: changeSet.budgetLimit ?? 0, createdAt: formatDate(changeSet.createdAt), createdBy: '服务端演示用户', version: changeSet.version, evidenceIds: changeSet.execution?.evidence.map(item => item.step) ?? [], rollbackPlan: changeSet.rollback?.reason ?? '服务端将记录模拟回滚原因。', changes: [{ field: '预算边界', before: '¥0', after: `¥${(changeSet.budgetLimit ?? 0).toLocaleString('zh-CN')}` }] }
}

function artifactKind(key: ArtifactKey): ApiArtifact['kind'] {
  return key === 'brief' ? 'brief' : key === 'creative' ? 'image' : 'document'
}

function artifactStatus(status: ProjectRecord['artifacts'][ArtifactKey]['status']): ApiArtifact['status'] {
  return status === '已确认' ? 'ready' : 'draft'
}

function artifactLabel(key: ArtifactKey): string {
  return ({ brief: '策略 Brief', strategy: '策略方案', creative: '创意产物', insight: '洞察报告', delivery: '投放计划' })[key]
}

function latestDocument(artifacts: ApiArtifact[], key: string): ApiArtifact | undefined {
  return artifacts.filter(artifact => artifact.content.startsWith(`[${key}]`)).sort((left, right) => right.updatedAt.localeCompare(left.updatedAt))[0]
}

function latestArtifact(artifacts: ApiArtifact[], kind: ApiArtifact['kind']): ApiArtifact | undefined {
  return artifacts.filter(artifact => artifact.kind === kind).sort((left, right) => right.updatedAt.localeCompare(left.updatedAt))[0]
}

function latestMainCreativeGenerationJob(jobs: ApiGenerationJob[]): ApiGenerationJob | undefined {
  return jobs
    .filter(job => (job.artifactKind === 'image' || job.artifactKind === 'video')
      && job.purpose === undefined
      && job.prerollType === undefined)
    .sort((left, right) => right.updatedAt.localeCompare(left.updatedAt))[0]
}

function latestMainCreativeArtifact(artifacts: ApiArtifact[]): ApiArtifact | undefined {
  return artifacts
    .filter(artifact => (artifact.kind === 'image' || artifact.kind === 'video')
      && artifact.purpose === undefined
      && artifact.prerollType === undefined)
    .sort((left, right) => right.updatedAt.localeCompare(left.updatedAt))[0]
}

function formatDate(value: string): string {
  const date = new Date(value)
  return Number.isNaN(date.valueOf()) ? value : date.toLocaleString('zh-CN', { hour12: false }).replace(/\//g, '-')
}

export function useProject() {
  const value = useContext(ProjectContext)
  if (!value) throw new Error('useProject must be used inside ProjectProvider')
  return value
}
