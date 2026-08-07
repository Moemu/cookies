import type { BriefDocument, BriefDraft, BriefReadiness, BriefVersion, FieldState } from './types'

const fullStrategyFields: Array<{
  field: string
  present: (document: BriefDocument) => boolean
}> = [
  { field: 'campaign.objective', present: document => Boolean(document.campaign.objective.trim()) },
  { field: 'audience.primary', present: document => Boolean(document.audience.primary.trim()) },
  { field: 'proposition', present: document => Boolean(document.proposition.trim()) },
  { field: 'channels', present: document => document.channels.length > 0 },
]

export const fullStrategyFieldLabels: Record<string, string> = {
  'campaign.objective': '推广目标',
  'audience.primary': '核心受众',
  proposition: '核心主张',
  channels: '渠道',
  'project.context': 'Project 上下文',
  'project.brand': 'Project 品牌',
  'project.product': 'Project 产品',
}

export function getFullStrategyReadiness(brief: BriefVersion): BriefReadiness {
  if (brief.full_strategy_readiness) return brief.full_strategy_readiness
  return calculateFullStrategyReadiness(brief.snapshot)
}

export function getFullStrategyDraftReadiness(brief: BriefDraft): BriefReadiness {
  return calculateFullStrategyReadiness(brief.document, brief.field_states)
}

function calculateFullStrategyReadiness(document: BriefDocument, states?: Record<string, FieldState>): BriefReadiness {
  const blockers = fullStrategyFields
    .flatMap(field => {
      if (!field.present(document)) return [{ field: field.field, reason: '完整策略需要该信息' }]
      if (states && states[field.field]?.confirmation !== 'confirmed') {
        return [{ field: field.field, reason: '完整策略需要用户确认' }]
      }
      return []
    })
  return { ready: blockers.length === 0, blockers, warnings: [] }
}
