export type PreflightCheck = {
  code: 'confirmed_brief' | 'ready_creative' | 'budget_boundary'
  passed: boolean
  message: string
  repair: string
}

export type DeliveryChangeSet = {
  id: string
  projectId: string
  name: string
  status: 'draft' | 'preflight_passed' | 'preflight_failed' | 'approved' | 'rejected' | 'executing' | 'executed' | 'rolled_back'
  artifactIds: string[]
  budgetLimit?: number
  preflight?: { passed: boolean; checks: PreflightCheck[]; checkedAt: string }
  execution?: { simulated: true; evidence: Array<{ step: string; status: string; message: string; recordedAt: string }>; executedAt: string }
  rollback?: { simulated: true; reason: string; rolledBackAt: string }
  version: number
  createdAt: string
  updatedAt: string
}

const apiBase = `${import.meta.env.VITE_API_BASE_URL ?? 'http://127.0.0.1:8787'}/api`

async function request<T>(path: string, method = 'GET', body?: unknown): Promise<T> {
  const response = await fetch(`${apiBase}${path}`, {
    method,
    headers: body === undefined ? undefined : { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  const payload = await response.json() as T | { error?: { message?: string } }
  if (!response.ok) {
    const error = payload as { error?: { message?: string } }
    throw new Error(error.error?.message ?? '投放模拟请求失败')
  }
  return payload as T
}

export const deliveryApi = {
  listChangeSets: (projectId?: string) => request<DeliveryChangeSet[]>(`/change-sets${projectId ? `?projectId=${encodeURIComponent(projectId)}` : ''}`),
  createChangeSet: (input: { projectId: string; name: string; artifactIds: string[]; budgetLimit: number }) =>
    request<DeliveryChangeSet>('/change-sets', 'POST', input),
  preflight: (id: string) => request<DeliveryChangeSet>(`/change-sets/${id}/preflight`, 'POST'),
  approve: (id: string) => request<DeliveryChangeSet>(`/change-sets/${id}/approve`, 'POST', { actor: 'Amelia Meng', role: 'demo-approver' }),
  execute: (id: string) => request<DeliveryChangeSet>(`/change-sets/${id}/execute`, 'POST', { actor: 'Amelia Meng' }),
  rollback: (id: string, reason: string) => request<DeliveryChangeSet>(`/change-sets/${id}/rollback`, 'POST', { actor: 'Amelia Meng', reason }),
}
