import { apiRequest } from '../../shared/api/client'
import type { DeliveryChangeSet, DeliveryExecutionResult, DeliveryMetricSnapshot, DeliveryPlan, DeliveryPlanDetail } from './types'

const base = (projectId: string) => `/api/delivery/v1/projects/${encodeURIComponent(projectId)}`

export function listDeliveryPlans(projectId: string, signal?: AbortSignal) {
  return apiRequest<{ items: DeliveryPlan[] }>(`${base(projectId)}/plans?limit=100`, { signal })
}

export function getDeliveryPlan(projectId: string, planId: string, signal?: AbortSignal) {
  return apiRequest<DeliveryPlanDetail>(`${base(projectId)}/plans/${encodeURIComponent(planId)}`, { signal })
}

export function createDeliveryPlan(projectId: string, input: {
  creative_package_id: string
  name: string
  objective: string
  budget_cents: number
  start_at: string
  end_at: string
}) {
  return apiRequest<DeliveryPlan>(`${base(projectId)}/plans`, { method: 'POST', body: JSON.stringify(input) })
}

export function createDeliveryChangeSet(projectId: string, planId: string, expectedVersion: number) {
  return apiRequest<DeliveryChangeSet>(`${base(projectId)}/plans/${encodeURIComponent(planId)}:create-change-set`, {
    method: 'POST', body: JSON.stringify({ expected_version: expectedVersion }),
  })
}

export function transitionDeliveryChangeSet(projectId: string, changeSetId: string, action: 'preflight' | 'approve' | 'rollback', expectedVersion: number) {
  return apiRequest<DeliveryChangeSet>(`${base(projectId)}/change-sets/${encodeURIComponent(changeSetId)}:${action}`, {
    method: 'POST', body: JSON.stringify({ expected_version: expectedVersion }),
  })
}

export function executeDeliveryChangeSet(projectId: string, changeSetId: string, expectedVersion: number) {
  return apiRequest<DeliveryExecutionResult>(`${base(projectId)}/change-sets/${encodeURIComponent(changeSetId)}:execute`, {
    method: 'POST', body: JSON.stringify({ expected_version: expectedVersion }),
  })
}

export function listDeliveryExecutions(projectId: string, signal?: AbortSignal) {
  return apiRequest<{ items: DeliveryExecutionResult[] }>(`${base(projectId)}/executions?limit=100`, { signal })
}

export function createDemoMetricSnapshot(projectId: string, executionId: string) {
  return apiRequest<DeliveryMetricSnapshot>(`${base(projectId)}/executions/${encodeURIComponent(executionId)}/metric-snapshots`, {
    method: 'POST',
    body: JSON.stringify({ dataset_version: 'preroll-demo/v1' }),
  })
}

export function listMetricSnapshots(projectId: string, executionId: string, signal?: AbortSignal) {
  return apiRequest<{ items: DeliveryMetricSnapshot[] }>(`${base(projectId)}/executions/${encodeURIComponent(executionId)}/metric-snapshots?limit=100`, { signal })
}
