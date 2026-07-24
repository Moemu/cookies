import { apiRequest } from '../../shared/api/client'
import type { Experience, InsightReport, PerformanceOverview, PreLaunchInsight } from './types'

const base = (projectId: string) => `/api/insights/v1/projects/${encodeURIComponent(projectId)}`

export function listInsightReports(projectId: string, signal?: AbortSignal) {
  return apiRequest<{ items: InsightReport[] }>(`${base(projectId)}/reports?limit=100`, { signal })
}

export function createInsightReport(projectId: string, input: { execution_id: string, summary: string, findings: string[] }) {
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

export function listExperiences(projectId: string, signal?: AbortSignal) {
  return apiRequest<{ items: Experience[] }>(`${base(projectId)}/experiences?limit=100`, { signal })
}

export function getPreLaunchInsights(projectId: string, signal?: AbortSignal) {
  return apiRequest<PreLaunchInsight>(`${base(projectId)}/prelaunch`, { signal })
}

export function getPerformanceOverview(projectId: string, signal?: AbortSignal) {
  return apiRequest<PerformanceOverview>(`${base(projectId)}/performance`, { signal })
}
