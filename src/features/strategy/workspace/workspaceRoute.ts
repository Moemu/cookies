export const STRATEGY_STAGES = ['intake', 'brief', 'strategy', 'review', 'handoff'] as const
export type StrategyStage = (typeof STRATEGY_STAGES)[number]

export const STRATEGY_PANELS = ['assistant', 'research', 'materials', 'activity', 'history'] as const
export type StrategyPanel = (typeof STRATEGY_PANELS)[number]

export interface StrategyWorkspaceLocation {
  stage: StrategyStage
  panel?: StrategyPanel
  resource?: string
}

export interface ParsedStrategyWorkspaceRoute {
  projectId: string
  workspaceId: string
  location: StrategyWorkspaceLocation
  needsCanonicalRedirect: boolean
}

const legacyWorkspaceViews: Readonly<Record<string, StrategyWorkspaceLocation>> = {
  '概览': { stage: 'intake' },
  'overview': { stage: 'intake' },
  '对话': { stage: 'intake' },
  'conversation': { stage: 'intake' },
  'brief': { stage: 'brief' },
  '研究': { stage: 'brief', panel: 'research' },
  'research': { stage: 'brief', panel: 'research' },
  '策略': { stage: 'strategy' },
  'strategy': { stage: 'strategy' },
  '创意任务策略': { stage: 'handoff' },
  'creative-task-strategy': { stage: 'handoff' },
  '实验': { stage: 'strategy' },
  'experiments': { stage: 'strategy' },
  '评审': { stage: 'review' },
  'review': { stage: 'review' },
  '变更记录': { stage: 'strategy', panel: 'history' },
  'history': { stage: 'strategy', panel: 'history' },
}

export function isStrategyStage(value: string): value is StrategyStage {
  return (STRATEGY_STAGES as readonly string[]).includes(value)
}

export function isStrategyPanel(value: string): value is StrategyPanel {
  return (STRATEGY_PANELS as readonly string[]).includes(value)
}

export function resolveLegacyStrategyWorkspaceView(view?: string): StrategyWorkspaceLocation {
  const normalized = view?.trim()
  if (!normalized) return { stage: 'intake' }
  return legacyWorkspaceViews[normalized] ?? legacyWorkspaceViews[normalized.toLowerCase()] ?? { stage: 'intake' }
}

export function strategyWorkspacePath(
  projectId: string,
  workspaceId: string,
  location: StrategyWorkspaceLocation,
): string {
  const path = `/projects/${encodeURIComponent(projectId)}/strategy/workspaces/${encodeURIComponent(workspaceId)}/${location.stage}`
  const search = new URLSearchParams()
  if (location.panel) search.set('panel', location.panel)
  if (location.resource?.trim()) search.set('resource', location.resource.trim())
  if (!search.size) return path
  return `${path}?${search.toString()}`
}

export function parseStrategyWorkspaceRoute(location: string): ParsedStrategyWorkspaceRoute | null {
  const url = new URL(location, 'https://cookies.local')
  const parts = url.pathname.split('/').filter(Boolean)
  if (parts[0] !== 'projects' || parts[2] !== 'strategy' || parts[3] !== 'workspaces' || !parts[1] || !parts[4]) {
    return null
  }

  const stageValue = decodeSegment(parts[5] ?? '')
  const panelValue = url.searchParams.get('panel')?.trim() ?? ''
  const resourceValue = url.searchParams.get('resource')?.trim() ?? ''
  const legacyView = url.searchParams.get('view') ?? undefined
  const stageIsCanonical = isStrategyStage(stageValue)
  const panelIsCanonical = !panelValue || isStrategyPanel(panelValue)
  const legacyLocation = resolveLegacyStrategyWorkspaceView(legacyView)
  const locationValue: StrategyWorkspaceLocation = {
    stage: stageIsCanonical ? stageValue : legacyLocation.stage,
    panel: panelIsCanonical && panelValue
      ? panelValue as StrategyPanel
      : stageIsCanonical ? undefined : legacyLocation.panel,
    resource: resourceValue || undefined,
  }

  return {
    projectId: decodeSegment(parts[1]),
    workspaceId: decodeSegment(parts[4]),
    location: locationValue,
    needsCanonicalRedirect: parts.length !== 6 || !stageIsCanonical || !panelIsCanonical || Boolean(legacyView),
  }
}

function decodeSegment(value: string): string {
  try {
    return decodeURIComponent(value)
  } catch {
    return value
  }
}
