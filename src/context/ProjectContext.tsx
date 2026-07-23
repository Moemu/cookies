import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
import { api, type ApiArtifact, type ApiGenerationJob, type ApiProject } from '../data/api'
import type { ArtifactKey, ArtifactStatus, ChangeSetRecord, ProjectArtifact, ProjectRecord } from '../types'
import { deliveryApi, type DeliveryChangeSet } from '../api/delivery'
import { presentCreativeStatus } from '../lib/media-status'

interface ProjectContextValue {
  projects: ProjectRecord[]
  currentProject: ProjectRecord
  isLoading: boolean
  error: string | null
  reloadProjects: () => Promise<void>
  selectProject: (id: string) => void
  createProject: (input: Pick<ProjectRecord, 'name' | 'brand' | 'goal'>) => Promise<ProjectRecord>
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
  const [currentProjectId, setCurrentProjectId] = useState('')
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const currentProject = projects.find(project => project.id === currentProjectId) ?? projects[0] ?? emptyProject

  const reloadProjects = useCallback(async () => {
    setIsLoading(true)
    try {
      const apiProjects = await api.listProjects()
      const nextProjects = await Promise.all(apiProjects.map(async project => {
        const [artifacts, jobs, changeSets] = await Promise.all([
          api.listArtifacts(project.id),
          api.listJobs(project.id),
          deliveryApi.listChangeSets(project.id),
        ])
        return toProjectRecord(project, artifacts, jobs, changeSets)
      }))
      setProjects(nextProjects)
      setCurrentProjectId(current => nextProjects.some(project => project.id === current) ? current : nextProjects[0]?.id ?? '')
      setError(null)
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : '加载项目失败')
    } finally {
      setIsLoading(false)
    }
  }, [])

  useEffect(() => { void reloadProjects() }, [reloadProjects])

  const selectProject = useCallback((id: string) => setCurrentProjectId(id), [])

  const createProject = useCallback(async (input: Pick<ProjectRecord, 'name' | 'brand' | 'goal'>) => {
    const created = toProjectRecord(await api.createProject({ name: input.name, brand: input.brand || '未指定品牌', objective: input.goal }))
    setProjects(current => [created, ...current])
    setCurrentProjectId(created.id)
    return created
  }, [])

  const updateArtifact = useCallback(async (key: ArtifactKey, patch: Partial<ProjectRecord['artifacts'][ArtifactKey]>) => {
    const project = projects.find(candidate => candidate.id === currentProjectId)
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
  }, [currentProjectId, projects, reloadProjects])

  const advanceArtifact = useCallback((key: ArtifactKey, status: ProjectRecord['artifacts'][ArtifactKey]['status']) => updateArtifact(key, { status }), [updateArtifact])

  const addChangeSet = useCallback(async (budgetLimit?: number) => {
    const project = projects.find(candidate => candidate.id === currentProjectId)
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
  }, [currentProjectId, projects, reloadProjects])

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
  const value = useMemo(() => ({ projects, currentProject, isLoading, error, reloadProjects, selectProject, createProject, advanceArtifact, updateArtifact, addChangeSet, preflightChangeSet, approveChangeSet, executeChangeSet, rollbackChangeSet }), [projects, currentProject, isLoading, error, reloadProjects, selectProject, createProject, advanceArtifact, updateArtifact, addChangeSet, preflightChangeSet, approveChangeSet, executeChangeSet, rollbackChangeSet])
  return <ProjectContext.Provider value={value}>{children}</ProjectContext.Provider>
}

function toProjectRecord(project: ApiProject, artifacts: ApiArtifact[] = [], jobs: ApiGenerationJob[] = [], changeSets: DeliveryChangeSet[] = []): ProjectRecord {
  const brief = latestArtifact(artifacts, 'brief')
  const latestMediaJob = latestMediaGenerationJob(jobs)
  const media = latestMediaJob?.artifactId
    ? artifacts.find(artifact => artifact.id === latestMediaJob.artifactId)
    : latestArtifact(artifacts, 'image') ?? latestArtifact(artifacts, 'video')
  const updatedAt = formatDate(project.updatedAt)
  const documents = artifacts.filter(artifact => artifact.kind === 'document')
  return {
    id: project.id,
    code: project.name.slice(0, 2).toUpperCase() || 'PR',
    name: project.name,
    brand: project.brand,
    product: project.brand,
    goal: project.objective,
    stage: '需求确认',
    progress: 12,
    status: '进行中',
    updatedAt,
    budget: 8600,
    currency: 'CNY',
    timezone: 'Asia/Shanghai',
    artifacts: {
      brief: toArtifactRecord('brief', brief, project.updatedAt),
      strategy: toArtifactRecord('strategy', latestDocument(documents, 'strategy'), project.updatedAt),
      creative: toCreativeRecord(media, latestMediaJob, project.updatedAt),
      insight: toArtifactRecord('insight', latestDocument(documents, 'insight'), project.updatedAt),
      delivery: toArtifactRecord('delivery', latestDocument(documents, 'delivery'), project.updatedAt),
    },
    changeSets: changeSets.map(toChangeSetRecord),
  }
}

const emptyProject: ProjectRecord = {
  id: '', code: '—', name: '尚未连接到服务端', brand: '—', product: '—', goal: '请启动本地 MVP API 后重试。',
  stage: '等待恢复', progress: 0, status: '进行中', updatedAt: '—', budget: 0, currency: 'CNY', timezone: 'Asia/Shanghai',
  artifacts: Object.fromEntries((['brief', 'strategy', 'creative', 'insight', 'delivery'] as ArtifactKey[]).map(key => [key, toArtifactRecord(key, undefined, '')])) as ProjectRecord['artifacts'],
  changeSets: [],
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

function latestMediaGenerationJob(jobs: ApiGenerationJob[]): ApiGenerationJob | undefined {
  return jobs
    .filter(job => job.artifactKind === 'image' || job.artifactKind === 'video')
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
