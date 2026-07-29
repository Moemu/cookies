import { apiRequest } from '../../shared/api/client'
import type {
  Experience,
  ExperienceAudit,
  ExperienceReference,
  ExperienceStatus,
  InsightReport,
  PerformanceOverview,
  PreLaunchInsight,
  RecordExperienceReferenceInput,
} from './types'

const base = (projectId: string) => `/api/insights/v1/projects/${encodeURIComponent(projectId)}`

export function listInsightReports(projectId: string, signal?: AbortSignal) {
  return apiRequest<{ items: InsightReport[] }>(`${base(projectId)}/reports?limit=100`, { signal })
}

export function createInsightReport(projectId: string, input: { execution_id: string }) {
  return apiRequest<InsightReport>(`${base(projectId)}/reports`, { method: 'POST', body: JSON.stringify(input) })
}

export function confirmInsightReport(projectId: string, reportId: string, expectedVersion: number) {
  return apiRequest<InsightReport>(`${base(projectId)}/reports/${encodeURIComponent(reportId)}:confirm`, {
    method: 'POST', body: JSON.stringify({ expected_version: expectedVersion }),
  })
}

export function createExperience(projectId: string, reportId: string, input: {
  expected_report_version: number
  conclusion: string
  conditions: string[]
  counterexamples: string[]
}) {
  return apiRequest<Experience>(`${base(projectId)}/reports/${encodeURIComponent(reportId)}:create-experience`, {
    method: 'POST', body: JSON.stringify(input),
  })
}

export function listExperiences(projectId: string, status?: ExperienceStatus, signal?: AbortSignal) {
  const query = status ? `&status=${status}` : ''
  return apiRequest<{ items: Experience[] }>(`${base(projectId)}/experiences?limit=100${query}`, { signal })
}

const experienceAction = (projectId: string, experienceId: string, action: string) =>
  `${base(projectId)}/experiences/${encodeURIComponent(experienceId)}:${action}`

export function confirmExperience(projectId: string, experienceId: string, expectedVersion: number) {
  return apiRequest<Experience>(experienceAction(projectId, experienceId, 'confirm'), {
    method: 'POST', body: JSON.stringify({ expected_version: expectedVersion }),
  })
}

// 拒绝、请求复审和废除都必须写明理由：结论从不被静默改写。
export function rejectExperience(projectId: string, experienceId: string, expectedVersion: number, reason: string) {
  return apiRequest<Experience>(experienceAction(projectId, experienceId, 'reject'), {
    method: 'POST', body: JSON.stringify({ expected_version: expectedVersion, reason }),
  })
}

export function requestExperienceReview(projectId: string, experienceId: string, expectedVersion: number, reason: string) {
  return apiRequest<Experience>(experienceAction(projectId, experienceId, 'request-review'), {
    method: 'POST', body: JSON.stringify({ expected_version: expectedVersion, reason }),
  })
}

export function retireExperience(projectId: string, experienceId: string, expectedVersion: number, reason: string) {
  return apiRequest<Experience>(experienceAction(projectId, experienceId, 'retire'), {
    method: 'POST', body: JSON.stringify({ expected_version: expectedVersion, reason }),
  })
}

export function reviseExperience(projectId: string, experienceId: string, input: {
  expected_version: number
  conclusion: string
  conditions: string[]
  counterexamples: string[]
  reason?: string
}) {
  return apiRequest<Experience>(experienceAction(projectId, experienceId, 'revise'), {
    method: 'POST', body: JSON.stringify(input),
  })
}

export function recordExperienceReference(projectId: string, experienceId: string, input: RecordExperienceReferenceInput) {
  return apiRequest<ExperienceReference>(experienceAction(projectId, experienceId, 'record-reference'), {
    method: 'POST', body: JSON.stringify(input),
  })
}

export function listExperienceReferences(projectId: string, experienceId: string, signal?: AbortSignal) {
  return apiRequest<{ items: ExperienceReference[] }>(
    `${base(projectId)}/experiences/${encodeURIComponent(experienceId)}/references?limit=100`, { signal })
}

// 引用记录视图要的是"全项目谁引用了经验"，一次取回，不按经验逐条拉。
export function listProjectExperienceReferences(projectId: string, signal?: AbortSignal) {
  return apiRequest<{ items: ExperienceReference[] }>(`${base(projectId)}/experience-references?limit=100`, { signal })
}

export function listExperienceAudits(projectId: string, experienceId: string, signal?: AbortSignal) {
  return apiRequest<{ items: ExperienceAudit[] }>(
    `${base(projectId)}/experiences/${encodeURIComponent(experienceId)}/audits?limit=100`, { signal })
}

export function listExperienceLineage(projectId: string, experienceId: string, signal?: AbortSignal) {
  return apiRequest<{ items: Experience[] }>(
    `${base(projectId)}/experiences/${encodeURIComponent(experienceId)}/lineage`, { signal })
}

export function getPreLaunchInsights(projectId: string, signal?: AbortSignal) {
  return apiRequest<PreLaunchInsight>(`${base(projectId)}/prelaunch`, { signal })
}

export function getPerformanceOverview(projectId: string, signal?: AbortSignal) {
  return apiRequest<PerformanceOverview>(`${base(projectId)}/performance`, { signal })
}
