import { platformClient } from '../data/platformClient'

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

export const deliveryApi = {
  listChangeSets: (projectId?: string) => projectId ? platformClient.listChangeSets(projectId) : Promise.resolve([]),
  createChangeSet: (input: { projectId: string; name: string; artifactIds: string[]; budgetLimit: number }) =>
    platformClient.createChangeSet(input.projectId, input),
  preflight: (projectId: string, id: string) => platformClient.preflightChangeSet(projectId, id),
  approve: (projectId: string, id: string) => platformClient.approveChangeSet(projectId, id),
  execute: (projectId: string, id: string) => platformClient.executeChangeSet(projectId, id),
  rollback: (projectId: string, id: string, reason: string) => platformClient.rollbackChangeSet(projectId, id, reason),
}

export type DeliverySource = 'mock'
export type DeliveryScenario = 'golden_path' | 'budget_zero' | 'creative_unconfirmed' | 'tracking_missing' | 'incomplete_draft' | 'project_plan_list' | 'approval_queue'

export type DeliveryPlanDraft = {
  name: string
  objective: string
  advertiser: {
    id: string
    name: string
    platform: 'ocean_engine'
  }
  budget: {
    totalMinor: number
    currency: 'CNY'
  }
  schedule: {
    startAt: string
    endAt: string
    timezone: string
  }
  tracking: {
    landingPage: string
    pixelId: string
    conversionEvent: string
  }
  creativeReferences: Array<{
    assetId: string
    version: number
    confirmed: boolean
  }>
  sourceStrategyVersion: string
}

export type DeliveryPlanVersion = DeliveryPlanDraft & {
  planId: string
  organizationId: string
  projectId: string
  versionNumber: number
  canonicalHash: string
  platform: 'ocean_engine_mock'
  advertiser: DeliveryPlanDraft['advertiser'] & {
    source: DeliverySource
    scenario: DeliveryScenario
  }
  source: DeliverySource
  scenario: DeliveryScenario
  createdBy: { kind: 'user' | 'service'; id: string }
  createdAt: string
}

export type DeliveryPlan = {
  id: string
  organizationId: string
  projectId: string
  status: 'draft'
  platform: 'ocean_engine_mock'
  source: DeliverySource
  scenario: DeliveryScenario
  currentVersionNumber: number
  currentVersion: DeliveryPlanVersion
  versions: DeliveryPlanVersion[]
  createdBy: string
  createdAt: string
  updatedAt: string
}

export type DeliveryPreflightCheck = {
  code: 'advertiser_available' | 'budget_positive' | 'schedule_valid' | 'creative_present' | 'creative_confirmed' | 'tracking_complete'
  severity: 'error' | 'warning'
  passed: boolean
  message: string
  repair?: {
    field: string
    section: string
    label: string
  }
}

export type DeliveryPreflightResult = {
  planId: string
  planVersion: number
  passed: boolean
  blocked: boolean
  checks: DeliveryPreflightCheck[]
  source: DeliverySource
  scenario: DeliveryScenario
  checkedAt: string
}

export type DeliveryApproval = {
  approvalId: string
  valid: boolean
  invalidReason?: 'APPROVAL_EXPIRED' | 'APPROVAL_CONTENT_MISMATCH' | 'APPROVAL_SCOPE_EXCEEDED' | 'STALE_PLAN_VERSION'
  planId: string
  planVersion: number
  changeSetId: string
  changeSetVersion: number
  planCanonicalHash: string
  actionHash: string
  hashSummary: string
  action: 'execute'
  scope: 'execute_mock'
  budgetLimit: { totalMinor: number; currency: 'CNY' }
  approvedBy: string
  approvedAt: string
  expiresAt: string
  source: DeliverySource
  scenario: DeliveryScenario
}

export type DeliveryControlChangeSet = {
  id: string
  organizationId: string
  projectId: string
  planId: string
  planName: string
  planVersion: number
  planCanonicalHash: string
  budgetLimit: { totalMinor: number; currency: 'CNY' }
  status: 'draft' | 'preflight_passed' | 'preflight_failed' | 'approved' | 'rejected' | 'executed' | 'rolled_back'
  riskLevel: string
  preflightNotes: string[]
  approvedBy?: string
  approvedAt?: string
  approval?: DeliveryApproval
  source: DeliverySource
  scenario: DeliveryScenario
  version: number
  createdBy: string
  createdAt: string
  updatedAt: string
}

type WireDeliveryPlanDraft = {
  name: string
  objective: string
  advertiser: { id: string; name: string; platform: 'ocean_engine'; source?: DeliverySource; scenario?: DeliveryScenario }
  budget: { total_minor: number; currency: 'CNY' }
  schedule: { start_at: string; end_at: string; timezone: string }
  tracking: { landing_page: string; pixel_id: string; conversion_event: string }
  creative_references: Array<{ asset_id: string; version: number; confirmed: boolean }>
  source_strategy_version: string
}

type WireDeliveryPlanVersion = WireDeliveryPlanDraft & {
  plan_id: string
  organization_id: string
  project_id: string
  version_number: number
  canonical_hash: string
  platform: 'ocean_engine_mock'
  advertiser: WireDeliveryPlanDraft['advertiser'] & { source: DeliverySource; scenario: DeliveryScenario }
  source: DeliverySource
  scenario: DeliveryScenario
  created_by: { kind: 'user' | 'service'; id: string }
  created_at: string
}

type WireDeliveryPlan = {
  id: string
  organization_id: string
  project_id: string
  status: 'draft'
  platform: 'ocean_engine_mock'
  source: DeliverySource
  scenario: DeliveryScenario
  current_version_number: number
  current_version: WireDeliveryPlanVersion
  versions: WireDeliveryPlanVersion[]
  created_by: string
  created_at: string
  updated_at: string
}

type WirePreflightResult = {
  plan_id: string
  plan_version: number
  passed: boolean
  blocked: boolean
  checks: Array<{
    code: DeliveryPreflightCheck['code']
    severity: DeliveryPreflightCheck['severity']
    passed: boolean
    message: string
    repair: DeliveryPreflightCheck['repair'] | null
  }>
  source: DeliverySource
  scenario: DeliveryScenario
  checked_at: string
}

type WireDeliveryApproval = {
  approval_id: string
  valid: boolean
  invalid_reason?: DeliveryApproval['invalidReason']
  plan_id: string
  plan_version: number
  change_set_id: string
  change_set_version: number
  plan_canonical_hash: string
  action_hash: string
  hash_summary: string
  action: 'execute'
  scope: 'execute_mock'
  budget_limit: { total_minor: number; currency: 'CNY' }
  approved_by: string
  approved_at: string
  expires_at: string
  source: DeliverySource
  scenario: DeliveryScenario
}

type WireDeliveryControlChangeSet = {
  id: string
  organization_id: string
  project_id: string
  plan_id: string
  plan_name: string
  plan_version: number
  plan_canonical_hash: string
  budget_limit: { total_minor: number; currency: 'CNY' }
  status: DeliveryControlChangeSet['status']
  risk_level: string
  preflight_notes: string[]
  approved_by?: string
  approved_at?: string
  approval?: WireDeliveryApproval
  source: DeliverySource
  scenario: DeliveryScenario
  version: number
  created_by: string
  created_at: string
  updated_at: string
}

export const deliveryPlanApi = {
  async list(projectId: string): Promise<DeliveryPlan[]> {
    const response = await deliveryPlanRequest<{ items: WireDeliveryPlan[]; source: DeliverySource; scenario: DeliveryScenario }>(
      projectId,
      '/plans',
    )
    return (response.items ?? []).map(toDeliveryPlan)
  },
  async create(projectId: string, draft: DeliveryPlanDraft): Promise<DeliveryPlan> {
    const response = await deliveryPlanRequest<WireDeliveryPlan>(projectId, '/plans', {
      method: 'POST',
      body: JSON.stringify(toWireDraft(draft)),
    })
    return toDeliveryPlan(response)
  },
  async update(projectId: string, planId: string, expectedVersion: number, draft: DeliveryPlanDraft): Promise<DeliveryPlan> {
    const response = await deliveryPlanRequest<WireDeliveryPlan>(projectId, `/plans/${encodeURIComponent(planId)}`, {
      method: 'PATCH',
      body: JSON.stringify({ expected_version: expectedVersion, ...toWireDraft(draft) }),
    })
    return toDeliveryPlan(response)
  },
  async preflight(projectId: string, planId: string): Promise<DeliveryPreflightResult> {
    const response = await deliveryPlanRequest<WirePreflightResult>(projectId, `/plans/${encodeURIComponent(planId)}/preflight`, { method: 'POST' })
    return {
      planId: response.plan_id,
      planVersion: response.plan_version,
      passed: response.passed,
      blocked: response.blocked,
      checks: response.checks.map(check => ({ ...check, repair: check.repair ?? undefined })),
      source: response.source,
      scenario: response.scenario,
      checkedAt: response.checked_at,
    }
  },
  async listChangeSets(projectId: string): Promise<DeliveryControlChangeSet[]> {
    const response = await deliveryPlanRequest<{ items: WireDeliveryControlChangeSet[] }>(projectId, '/change-sets')
    return (response.items ?? []).map(toDeliveryControlChangeSet)
  },
  async getChangeSet(projectId: string, changeSetId: string): Promise<DeliveryControlChangeSet> {
    return toDeliveryControlChangeSet(await deliveryPlanRequest<WireDeliveryControlChangeSet>(
      projectId,
      `/change-sets/${encodeURIComponent(changeSetId)}`,
    ))
  },
  async createChangeSet(projectId: string, planId: string, expectedVersion: number): Promise<DeliveryControlChangeSet> {
    return toDeliveryControlChangeSet(await deliveryPlanRequest<WireDeliveryControlChangeSet>(
      projectId,
      `/plans/${encodeURIComponent(planId)}:create-change-set`,
      { method: 'POST', body: JSON.stringify({ expected_version: expectedVersion }) },
    ))
  },
  async preflightChangeSet(projectId: string, changeSetId: string, expectedVersion: number): Promise<DeliveryControlChangeSet> {
    return deliveryChangeSetAction(projectId, changeSetId, 'preflight', expectedVersion)
  },
  async approveChangeSet(projectId: string, changeSetId: string, expectedVersion: number): Promise<DeliveryControlChangeSet> {
    return deliveryChangeSetAction(projectId, changeSetId, 'approve', expectedVersion)
  },
  async executeChangeSet(projectId: string, changeSetId: string, expectedVersion: number): Promise<DeliveryControlChangeSet> {
    const response = await deliveryPlanRequest<{ change_set: WireDeliveryControlChangeSet }>(
      projectId,
      `/change-sets/${encodeURIComponent(changeSetId)}:execute`,
      { method: 'POST', body: JSON.stringify({ expected_version: expectedVersion }) },
    )
    return toDeliveryControlChangeSet(response.change_set)
  },
}

async function deliveryChangeSetAction(
  projectId: string,
  changeSetId: string,
  action: 'preflight' | 'approve',
  expectedVersion: number,
) {
  const response = await deliveryPlanRequest<WireDeliveryControlChangeSet>(
    projectId,
    `/change-sets/${encodeURIComponent(changeSetId)}:${action}`,
    { method: 'POST', body: JSON.stringify({ expected_version: expectedVersion }) },
  )
  return toDeliveryControlChangeSet(response)
}

async function deliveryPlanRequest<T>(projectId: string, path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers)
  if (init.body !== undefined) headers.set('Content-Type', 'application/json')
  const response = await fetch(`/api/delivery/v1/projects/${encodeURIComponent(projectId)}${path}`, { credentials: 'include', ...init, headers })
  const payload = await response.json() as T | { error?: { code?: string; message?: string } }
  if (!response.ok) {
    const problem = payload as { error?: { code?: string; message?: string } }
    throw new Error(problem.error?.code === 'VERSION_CONFLICT'
      ? '计划已被其他版本更新，请刷新后再试。'
      : problem.error?.message ?? 'Delivery API 请求失败')
  }
  return payload as T
}

function toWireDraft(draft: DeliveryPlanDraft): WireDeliveryPlanDraft {
  return {
    name: draft.name,
    objective: draft.objective,
    advertiser: draft.advertiser,
    budget: { total_minor: draft.budget.totalMinor, currency: draft.budget.currency },
    schedule: { start_at: draft.schedule.startAt, end_at: draft.schedule.endAt, timezone: draft.schedule.timezone },
    tracking: {
      landing_page: draft.tracking.landingPage,
      pixel_id: draft.tracking.pixelId,
      conversion_event: draft.tracking.conversionEvent,
    },
    creative_references: draft.creativeReferences.map(reference => ({
      asset_id: reference.assetId,
      version: reference.version,
      confirmed: reference.confirmed,
    })),
    source_strategy_version: draft.sourceStrategyVersion,
  }
}

function toDeliveryPlan(plan: WireDeliveryPlan): DeliveryPlan {
  return {
    id: plan.id,
    organizationId: plan.organization_id,
    projectId: plan.project_id,
    status: plan.status,
    platform: plan.platform,
    source: plan.source,
    scenario: plan.scenario,
    currentVersionNumber: plan.current_version_number,
    currentVersion: toDeliveryPlanVersion(plan.current_version),
    versions: plan.versions.map(toDeliveryPlanVersion),
    createdBy: plan.created_by,
    createdAt: plan.created_at,
    updatedAt: plan.updated_at,
  }
}

function toDeliveryPlanVersion(version: WireDeliveryPlanVersion): DeliveryPlanVersion {
  return {
    planId: version.plan_id,
    organizationId: version.organization_id,
    projectId: version.project_id,
    versionNumber: version.version_number,
    canonicalHash: version.canonical_hash,
    platform: version.platform,
    name: version.name,
    objective: version.objective,
    advertiser: {
      id: version.advertiser.id,
      name: version.advertiser.name,
      platform: version.advertiser.platform,
      source: version.advertiser.source,
      scenario: version.advertiser.scenario,
    },
    budget: { totalMinor: version.budget.total_minor, currency: version.budget.currency },
    schedule: { startAt: version.schedule.start_at, endAt: version.schedule.end_at, timezone: version.schedule.timezone },
    tracking: {
      landingPage: version.tracking.landing_page,
      pixelId: version.tracking.pixel_id,
      conversionEvent: version.tracking.conversion_event,
    },
    creativeReferences: version.creative_references.map(reference => ({
      assetId: reference.asset_id,
      version: reference.version,
      confirmed: reference.confirmed,
    })),
    sourceStrategyVersion: version.source_strategy_version,
    source: version.source,
    scenario: version.scenario,
    createdBy: version.created_by,
    createdAt: version.created_at,
  }
}

function toDeliveryControlChangeSet(value: WireDeliveryControlChangeSet): DeliveryControlChangeSet {
  return {
    id: value.id,
    organizationId: value.organization_id,
    projectId: value.project_id,
    planId: value.plan_id,
    planName: value.plan_name,
    planVersion: value.plan_version,
    planCanonicalHash: value.plan_canonical_hash,
    budgetLimit: {
      totalMinor: value.budget_limit.total_minor,
      currency: value.budget_limit.currency,
    },
    status: value.status,
    riskLevel: value.risk_level,
    preflightNotes: value.preflight_notes ?? [],
    approvedBy: value.approved_by,
    approvedAt: value.approved_at,
    approval: value.approval ? {
      approvalId: value.approval.approval_id,
      valid: value.approval.valid,
      invalidReason: value.approval.invalid_reason,
      planId: value.approval.plan_id,
      planVersion: value.approval.plan_version,
      changeSetId: value.approval.change_set_id,
      changeSetVersion: value.approval.change_set_version,
      planCanonicalHash: value.approval.plan_canonical_hash,
      actionHash: value.approval.action_hash,
      hashSummary: value.approval.hash_summary,
      action: value.approval.action,
      scope: value.approval.scope,
      budgetLimit: {
        totalMinor: value.approval.budget_limit.total_minor,
        currency: value.approval.budget_limit.currency,
      },
      approvedBy: value.approval.approved_by,
      approvedAt: value.approval.approved_at,
      expiresAt: value.approval.expires_at,
      source: value.approval.source,
      scenario: value.approval.scenario,
    } : undefined,
    source: value.source,
    scenario: value.scenario,
    version: value.version,
    createdBy: value.created_by,
    createdAt: value.created_at,
    updatedAt: value.updated_at,
  }
}
