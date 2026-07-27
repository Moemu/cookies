import type { LucideIcon } from 'lucide-react'
import type { ApiOperationalRecord } from './data/api'

export type SystemKey = 'strategy' | 'creative' | 'insight' | 'delivery'
export type DataState = 'ready' | 'loading' | 'empty' | 'error' | 'forbidden'
export type ArtifactKey = 'brief' | 'strategy' | 'creative' | 'insight' | 'delivery'
export type ArtifactStatus = '草稿' | '待确认' | '已确认' | '排队中' | '制作中' | '已完成' | '生成失败' | '已取消' | '待审批' | '执行中'

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
