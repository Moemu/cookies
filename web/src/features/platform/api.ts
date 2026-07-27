import { apiRequest } from '../../shared/api/client'
import type { Brand, CreateProjectInput, CurrentIdentity, Project, ProjectContext, ProviderJob, WorkspaceBootstrap } from './types'

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

type ImageJobInput = { prompt: string, width: number, height: number, source_assets?: Array<{ project_id: string, asset_version: { asset_id: string, version: number } }> }

export function createImageJob(projectId: string, projectContextVersion: number, input: ImageJobInput, capability: 'image.generate' | 'image.edit' = 'image.generate') {
  return apiRequest<ProviderJob>(`/platform/v1/projects/${encodeURIComponent(projectId)}/model/jobs`, {
    method: 'POST',
    headers: { 'Idempotency-Key': `web-${capability.replace('.', '-')}-${crypto.randomUUID()}` },
    body: JSON.stringify({ capability, model_alias: 'cookies.image.standard', project_context_version: projectContextVersion, input }),
  })
}

export function getProviderJob(projectId: string, jobId: string, signal?: AbortSignal) {
  return apiRequest<ProviderJob>(`/platform/v1/projects/${encodeURIComponent(projectId)}/model/jobs/${encodeURIComponent(jobId)}`, { signal })
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
