import type { LucideIcon } from 'lucide-react'

export type SystemKey = 'strategy' | 'creative' | 'insight' | 'delivery'
export type DataState = 'ready' | 'loading' | 'empty' | 'error' | 'forbidden'
export type ArtifactKey = 'brief' | 'strategy' | 'creative' | 'insight' | 'delivery'
export type ArtifactStatus = '草稿' | '待确认' | '已确认' | '制作中' | '待审批' | '执行中'

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
  status: '草稿' | '待审批' | '已批准' | '已拒绝' | '已执行' | '已回滚'
  risk: '低' | '中' | '高'
  budgetImpact: number
  createdAt: string
  createdBy: string
  changes: Array<{ field: string; before: string; after: string }>
  evidenceIds: string[]
  rollbackPlan: string
  version: number
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
  updatedAt: string
  budget: number
  currency: 'CNY'
  timezone: 'Asia/Shanghai'
  artifacts: Record<ArtifactKey, ProjectArtifact>
  changeSets: ChangeSetRecord[]
}
