import type { LucideIcon } from 'lucide-react'

export type SystemKey = 'strategy' | 'creative' | 'insight' | 'delivery'

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
