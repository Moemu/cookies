import { createContext, useCallback, useContext, useMemo, useState, type ReactNode } from 'react'
import { initialProjects } from '../data/projects'
import type { ArtifactKey, ChangeSetRecord, ProjectRecord } from '../types'

interface ProjectContextValue {
  projects: ProjectRecord[]
  currentProject: ProjectRecord
  selectProject: (id: string) => void
  createProject: (input: Pick<ProjectRecord, 'name' | 'brand' | 'goal'>) => ProjectRecord
  advanceArtifact: (key: ArtifactKey, status: ProjectRecord['artifacts'][ArtifactKey]['status']) => void
  updateArtifact: (key: ArtifactKey, patch: Partial<ProjectRecord['artifacts'][ArtifactKey]>) => void
  addChangeSet: () => ChangeSetRecord
  updateChangeSetStatus: (id: string, status: ChangeSetRecord['status']) => void
  rollbackChangeSet: (id: string) => void
}

const ProjectContext = createContext<ProjectContextValue | null>(null)

export function ProjectProvider({ children }: { children: ReactNode }) {
  const [projects, setProjects] = useState<ProjectRecord[]>(() => initialProjects)
  const [currentProjectId, setCurrentProjectId] = useState(initialProjects[0].id)
  const currentProject = projects.find(project => project.id === currentProjectId) ?? projects[0]

  const selectProject = useCallback((id: string) => setCurrentProjectId(id), [])

  const createProject = useCallback((input: Pick<ProjectRecord, 'name' | 'brand' | 'goal'>) => {
    const seed = initialProjects[0]
    const project: ProjectRecord = {
      ...structuredClone(seed), id: `prj-2607-${String(Date.now()).slice(-4)}`, code: input.name.slice(0, 2).toUpperCase(), name: input.name, brand: input.brand || '未指定品牌', goal: input.goal, product: '待补充', stage: '需求确认', progress: 8, status: '进行中', updatedAt: '2026-07-22 17:10', budget: 0, changeSets: [],
      artifacts: Object.fromEntries(Object.entries(seed.artifacts).map(([key, value]) => [key, { ...value, version: 'v0.1', status: key === 'brief' ? '草稿' : '待确认', updatedAt: '2026-07-22 17:10' }])) as ProjectRecord['artifacts'],
    }
    setProjects(current => [project, ...current])
    setCurrentProjectId(project.id)
    return project
  }, [])

  const advanceArtifact = useCallback((key: ArtifactKey, status: ProjectRecord['artifacts'][ArtifactKey]['status']) => {
    setProjects(current => current.map(project => project.id !== currentProjectId ? project : { ...project, updatedAt: '2026-07-22 17:10', artifacts: { ...project.artifacts, [key]: { ...project.artifacts[key], status, updatedAt: '2026-07-22 17:10' } } }))
  }, [currentProjectId])

  const updateArtifact = useCallback((key: ArtifactKey, patch: Partial<ProjectRecord['artifacts'][ArtifactKey]>) => {
    setProjects(current => current.map(project => project.id !== currentProjectId ? project : { ...project, updatedAt: '2026-07-22 17:10', artifacts: { ...project.artifacts, [key]: { ...project.artifacts[key], ...patch, updatedAt: '2026-07-22 17:10' } } }))
  }, [currentProjectId])

  const addChangeSet = useCallback(() => {
    const changeSet: ChangeSetRecord = { id: `CS-2607-${String(Date.now()).slice(-3)}`, title: '素材组合与探索预算优化', status: '草稿', risk: '中', budgetImpact: 8600, createdAt: '2026-07-22 17:10', createdBy: 'Amelia Meng', version: 1, evidenceIds: ['EV-024', 'INS-014'], rollbackPlan: '恢复当前计划版本并重新启用原广告组。', changes: [{ field: '新素材覆盖', before: '18%', after: '30% 目标' }, { field: '探索预算', before: '¥0', after: '¥8,600' }] }
    setProjects(current => current.map(project => project.id !== currentProjectId ? project : { ...project, changeSets: [changeSet, ...project.changeSets] }))
    return changeSet
  }, [currentProjectId])

  const updateChangeSetStatus = useCallback((id: string, status: ChangeSetRecord['status']) => {
    setProjects(current => current.map(project => project.id !== currentProjectId ? project : { ...project, changeSets: project.changeSets.map(change => change.id === id ? { ...change, status, version: change.version + 1 } : change) }))
  }, [currentProjectId])

  const rollbackChangeSet = useCallback((id: string) => updateChangeSetStatus(id, '已回滚'), [updateChangeSetStatus])
  const value = useMemo(() => ({ projects, currentProject, selectProject, createProject, advanceArtifact, updateArtifact, addChangeSet, updateChangeSetStatus, rollbackChangeSet }), [projects, currentProject, selectProject, createProject, advanceArtifact, updateArtifact, addChangeSet, updateChangeSetStatus, rollbackChangeSet])
  return <ProjectContext.Provider value={value}>{children}</ProjectContext.Provider>
}

export function useProject() {
  const value = useContext(ProjectContext)
  if (!value) throw new Error('useProject must be used inside ProjectProvider')
  return value
}
