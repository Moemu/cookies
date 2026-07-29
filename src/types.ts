import type { LucideIcon } from 'lucide-react'
import type { ApiOperationalRecord } from './data/api.js'

export type SystemKey = 'strategy' | 'creative' | 'insight' | 'delivery'
export type DataState = 'ready' | 'loading' | 'empty' | 'error' | 'forbidden'
export type ArtifactKey = 'brief' | 'strategy' | 'creative' | 'insight' | 'delivery'
export type ArtifactStatus = '草稿' | '待确认' | '已确认' | '排队中' | '制作中' | '已完成' | '生成失败' | '已取消' | '待审批' | '执行中'
export type AgencyHealthStatus = 'healthy' | 'watch' | 'blocked'
export type AdPlatform = '巨量引擎' | '腾讯广告' | '快手磁力'
export type BindingHealthStatus = 'normal' | 'warning' | 'expired'
export type ProjectProgressStage = 'intake' | 'strategy' | 'creative' | 'quality_check' | 'human_review' | 'delivery' | 'completed'
export type QualityCheckStatus = 'queued' | 'running' | 'passed' | 'failed'
export type QualityIssueSeverity = 'minor' | 'major' | 'critical'
export type MaterialConfirmationStatus = 'confirmed' | 'changes_requested'

export interface OrganizationRecord {
  id: string
  code: string
  name: string
  owner: string
  currency: 'CNY'
  timezone: 'Asia/Shanghai'
  updatedAt: string
}

export interface ClientRecord {
  id: string
  organizationId: string
  code: string
  name: string
  industry: string
  owner: string
  healthStatus: AgencyHealthStatus
  updatedAt: string
}

export interface BrandRecord {
  id: string
  organizationId: string
  clientId: string
  code: string
  name: string
  category: string
  productLines: string[]
  owner: string
  guidelineStatus: 'ready' | 'missing' | 'outdated'
  updatedAt: string
}

export interface AdAccountBinding {
  id: string
  organizationId: string
  clientId: string
  brandId: string
  projectIds: string[]
  platform: AdPlatform
  accountName: string
  accountDisplayId: string
  currency: 'CNY'
  timezone: 'Asia/Shanghai'
  permissionStatus: BindingHealthStatus
  loginStatus: BindingHealthStatus
  trackingStatus: BindingHealthStatus
  owner: string
  boundAssetIds: string[]
  lastSyncedAt: string
}

export interface ProjectProgress {
  stage: ProjectProgressStage
  stageLabel: string
  stagePercent: number
  taskPercent: number
  riskStatus: AgencyHealthStatus
  blocker?: string
  updatedAt: string
}

export interface NavItem {
  id: string
  label: string
  icon: LucideIcon
  views: string[]
  description: string
  group: string
  layout?: 'dashboard' | 'workspace' | 'table' | 'editor' | 'analysis' | 'operations' | 'settings'
}

export interface SystemDefinition {
  key: SystemKey
  label: string
  shortLabel: string
  statement: string
  icon: LucideIcon
  nav: NavItem[]
}

export interface ProjectArtifact {
  id?: string
  key: ArtifactKey
  label: string
  version: string
  status: ArtifactStatus
  owner: string
  updatedAt: string
  summary: string
  sourceVersion?: string
}

export interface ChangeSetRecord {
  id: string
  title: string
  status: '草稿' | '待审批' | '已批准' | '已拒绝' | '执行中' | '已执行' | '已回滚'
  risk: '低' | '中' | '高'
  budgetImpact: number
  createdAt: string
  createdBy: string
  changes: Array<{ field: string; before: string; after: string }>
  evidenceIds: string[]
  rollbackPlan: string
  version: number
}

export type BusinessTaskType =
  | 'strategy'
  | 'creative'
  | 'video'
  | 'brand_video'
  | 'short_drama_preroll'
  | 'game_preroll'
  | 'commerce_preroll'
  | 'viral_remake'
  | 'video_edit'

export interface BusinessTaskRecord {
  id: string
  projectId: string
  type: BusinessTaskType
  name: string
  objective: string
  status: 'draft' | 'in_progress' | 'ready' | 'completed' | 'failed'
  sourceTaskIds: string[]
  sourceArtifactIds: string[]
  outputArtifactIds: string[]
  version: number
  createdAt: string
  updatedAt: string
}

export interface QualityCheckIssue {
  id: string
  severity: QualityIssueSeverity
  rule: string
  evidence: string
  suggestion: string
}

export interface QualityCheckRun {
  id: string
  organizationId: string
  projectId: string
  assetId: string
  assetVersion: number
  status: QualityCheckStatus
  model: string
  ruleVersion: string
  promptVersion: string
  summary: string
  issues: QualityCheckIssue[]
  createdAt: string
  completedAt?: string
}

export interface MaterialConfirmation {
  id: string
  organizationId: string
  projectId: string
  qualityCheckRunId: string
  assetId: string
  assetVersion: number
  status: MaterialConfirmationStatus
  scope: string
  confirmedBy: string
  note: string
  createdAt: string
}

export interface AssetVersionRecord {
  version: number
  createdBy: string
  sourceTaskId: string
  sourceType: 'model_generation' | 'manual_edit'
  sourceLabel: string
  createdAt: string
  changeSummary: string
}

export interface AssetAuthorizationScope {
  platforms: AdPlatform[]
  regions: string[]
  rightsHolder: string
  expiresAt: string
  note: string
}

export interface AssetVersionPointer {
  id: string
  organizationId: string
  projectId: string
  assetId: string
  workingVersion: number
  qualityCheckedVersion?: number
  humanConfirmedVersion?: number
  deliveryVersion?: number
  versions: AssetVersionRecord[]
  authorization: AssetAuthorizationScope
  deliveryTarget: {
    platform: AdPlatform
    region: string
  }
  owner: string
  updatedAt: string
}

export interface ProjectRecord {
  id: string
  code: string
  name: string
  brand: string
  product: string
  goal: string
  stage: string
  progress: number
  status: '进行中' | '已完成'
  owner: string
  updatedAt: string
  budget: number
  currency: 'CNY'
  timezone: 'Asia/Shanghai'
  artifacts: Record<ArtifactKey, ProjectArtifact>
  tasks: BusinessTaskRecord[]
  changeSets: ChangeSetRecord[]
  operations: ApiOperationalRecord[]
  knowledgeCount: number
}
