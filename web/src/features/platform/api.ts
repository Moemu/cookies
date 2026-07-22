import { apiRequest } from '../../shared/api/client'
import type { Brand, CreateProjectInput, CurrentIdentity, Project, ProjectContext, WorkspaceBootstrap } from './types'

export async function getWorkspaceBootstrap(signal?: AbortSignal): Promise<WorkspaceBootstrap> {
  const [identity, projectList] = await Promise.all([
    apiRequest<CurrentIdentity>('/platform/v1/me', { signal }),
    apiRequest<{ items: Project[] }>('/platform/v1/projects', { signal }),
  ])
  return { identity, projects: projectList.items }
}

export function getProjectContext(projectId: string, signal?: AbortSignal) {
  return apiRequest<ProjectContext>(`/platform/v1/projects/${encodeURIComponent(projectId)}/context`, { signal })
}

export function createBrand(name: string, signal?: AbortSignal) {
  return apiRequest<Brand>('/platform/v1/brands', {
    method: 'POST',
    body: JSON.stringify({ name }),
    signal,
  })
}

export function createProject(input: CreateProjectInput, signal?: AbortSignal) {
  return apiRequest<Project>('/platform/v1/projects', {
    method: 'POST',
    body: JSON.stringify(input),
    signal,
  })
}
