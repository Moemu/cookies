import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from 'react'
import {
  buildAgencyWorkbench,
  createBackendProject,
  enrichProjectRecord,
  loadWorkspaceBootstrap,
  toProjectRecord,
} from '../backend/platform'
import type { ApiAgencyWorkbench, ApiBusinessTaskType } from '../data/api'
import type {
  ArtifactKey,
  BusinessTaskRecord,
  ChangeSetRecord,
  ProjectRecord,
} from '../types'
import type { DeliveryChangeSet } from '../api/delivery'

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
  updateTask: (
    id: string,
    patch: Partial<Pick<BusinessTaskRecord, 'name' | 'objective' | 'status' | 'sourceTaskIds' | 'sourceArtifactIds' | 'outputArtifactIds'>>,
  ) => Promise<BusinessTaskRecord>
  advanceArtifact: (
    key: ArtifactKey,
    status: ProjectRecord['artifacts'][ArtifactKey]['status'],
  ) => Promise<void>
  updateArtifact: (
    key: ArtifactKey,
    patch: Partial<ProjectRecord['artifacts'][ArtifactKey]>,
  ) => Promise<void>
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
  const requestRef = useRef(0)

  useEffect(() => {
    projectsRef.current = projects
  }, [projects])

  const reloadProjects = useCallback(async (expectedProjectId?: string) => {
    const requestId = requestRef.current + 1
    requestRef.current = requestId
    setIsLoading(true)
    try {
      const bootstrap = await loadWorkspaceBootstrap()
      let nextProjects = bootstrap.projects.map((project) => toProjectRecord(project, bootstrap.identity))
      const requestedId = expectedProjectId || targetProjectIdRef.current || nextProjects[0]?.id || ''
      const selected = nextProjects.find((project) => project.id === requestedId)
      if (selected) {
        const enriched = await enrichProjectRecord(selected)
        nextProjects = nextProjects.map((project) => project.id === enriched.id ? enriched : project)
      }

      if (requestRef.current !== requestId) return
      projectsRef.current = nextProjects
      setProjects(nextProjects)
      setAgencyWorkbench(buildAgencyWorkbench(bootstrap.identity, bootstrap.projects, nextProjects))
      setLoadedProjectId(selected?.id ?? '')
      if (!targetProjectIdRef.current && selected) {
        targetProjectIdRef.current = selected.id
        setTargetProjectId(selected.id)
      }
      setRouteDiagnostic(
        requestedId && !selected
          ? `路由目标 Project ${requestedId} 不在当前身份可访问的项目列表中。`
          : null,
      )
      setError(null)
    } catch (cause) {
      if (requestRef.current !== requestId) return
      setError(cause instanceof Error ? cause.message : '无法从 Go 服务端加载项目。')
    } finally {
      if (requestRef.current === requestId) setIsLoading(false)
    }
  }, [])

  useEffect(() => {
    void reloadProjects()
  }, [reloadProjects])

  const selectProject = useCallback((id: string) => {
    if (!id || targetProjectIdRef.current === id && loadedProjectId === id) return
    targetProjectIdRef.current = id
    setTargetProjectId(id)
    setRouteDiagnostic(null)
    const cached = projectsRef.current.find((project) => project.id === id)
    if (cached) setLoadedProjectId(id)
    void reloadProjects(id)
  }, [loadedProjectId, reloadProjects])

  const createProject = useCallback(async (
    input: Pick<ProjectRecord, 'name' | 'brand' | 'goal'>,
  ) => {
    const bootstrap = await loadWorkspaceBootstrap()
    const backendProject = await createBackendProject({
      name: input.name,
      brand: input.brand,
    })
    const created = toProjectRecord(backendProject, bootstrap.identity)
    const nextProjects = [created, ...projectsRef.current]
    projectsRef.current = nextProjects
    targetProjectIdRef.current = created.id
    setProjects(nextProjects)
    setAgencyWorkbench((current) => {
      if (!current) return current
      return {
        ...current,
        projects: [
          {
            id: created.id,
            organizationId: bootstrap.identity.organization.id,
            clientId: '',
            brandId: backendProject.primary_brand_id ?? '',
            name: created.name,
            brand: created.brand,
            objective: created.goal,
            runtime: {
              code: created.code,
              product: created.product,
              stage: created.stage,
              progress: created.progress,
              status: 'active',
              owner: created.owner,
              budget: created.budget,
              currency: created.currency,
              timezone: created.timezone,
            },
            progressDetail: {
              stage: 'intake',
              stageLabel: '需求下达',
              stagePercent: 0,
              taskPercent: 0,
              riskStatus: 'healthy',
              updatedAt: created.updatedAt,
            },
            version: backendProject.project_context_version,
            createdAt: backendProject.created_at,
            updatedAt: backendProject.updated_at,
          },
          ...current.projects,
        ],
      }
    })
    setTargetProjectId(created.id)
    setLoadedProjectId(created.id)
    setRouteDiagnostic(null)
    return created
  }, [])

  const unsupported = useCallback(async (): Promise<never> => {
    throw new Error('此操作必须在已接入 Go 领域契约的真实工作区中完成。')
  }, [])

  const currentProject = projects.find((project) => project.id === loadedProjectId) ?? emptyProject
  const value = useMemo<ProjectContextValue>(() => ({
    projects,
    currentProject,
    agencyWorkbench,
    targetProjectId,
    loadedProjectId,
    isLoading,
    error,
    routeDiagnostic,
    reloadProjects,
    selectProject,
    createProject,
    updateProject: unsupported,
    createTask: unsupported,
    updateTask: unsupported,
    advanceArtifact: unsupported,
    updateArtifact: unsupported,
    addChangeSet: unsupported,
    preflightChangeSet: unsupported,
    approveChangeSet: unsupported,
    executeChangeSet: unsupported,
    rollbackChangeSet: unsupported,
  }), [
    createProject,
    currentProject,
    error,
    isLoading,
    loadedProjectId,
    projects,
    agencyWorkbench,
    reloadProjects,
    routeDiagnostic,
    selectProject,
    targetProjectId,
    unsupported,
  ])

  return <ProjectContext.Provider value={value}>{children}</ProjectContext.Provider>
}

const emptyProject: ProjectRecord = {
  id: '',
  code: '—',
  name: '正在连接 Go 服务端',
  brand: '—',
  product: '—',
  goal: '请确认 cookies-api 已启动。',
  stage: '加载中',
  progress: 0,
  status: '进行中',
  owner: '—',
  updatedAt: '—',
  budget: 0,
  currency: 'CNY',
  timezone: 'Asia/Shanghai',
  artifacts: {
    brief: emptyArtifact('brief', '策略 Brief'),
    strategy: emptyArtifact('strategy', '策略方案'),
    creative: emptyArtifact('creative', '创意产物'),
    delivery: emptyArtifact('delivery', '投放计划'),
    insight: emptyArtifact('insight', '洞察报告'),
  },
  tasks: [],
  changeSets: [] as ChangeSetRecord[],
  operations: [],
  knowledgeCount: 0,
}

function emptyArtifact(key: ArtifactKey, label: string) {
  return {
    key,
    label,
    version: 'v0',
    status: '待确认' as const,
    owner: 'Go 服务端',
    updatedAt: '—',
    summary: '尚无持久化数据。',
  }
}

export function useProject() {
  const value = useContext(ProjectContext)
  if (!value) throw new Error('useProject must be used inside ProjectProvider')
  return value
}
