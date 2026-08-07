import type { AINativeStageId } from './types'

const stages = new Set<AINativeStageId>(['requirement', 'script', 'storyboard', 'video'])

export function readAINativeWorkspaceLocation(search: string): { workspaceId: string; stage: AINativeStageId; repairAssetId?: string } | null {
  const params = new URLSearchParams(search)
  const workspaceId = params.get('workspace')?.trim() ?? ''
  const rawStage = params.get('stage') ?? 'requirement'
  if (!workspaceId || !stages.has(rawStage as AINativeStageId)) return null
  const repairAssetId = rawStage === 'storyboard' ? params.get('repair_asset')?.trim() : ''
  return { workspaceId, stage: rawStage as AINativeStageId, ...(repairAssetId ? { repairAssetId } : {}) }
}

export function aiNativeWorkspaceLocation(search: string, workspaceId: string, stage: AINativeStageId, repairAssetId = ''): string {
  const params = new URLSearchParams(search)
  params.set('workspace', workspaceId)
  params.set('stage', stage)
  if (stage === 'storyboard' && repairAssetId.trim()) params.set('repair_asset', repairAssetId.trim())
  else params.delete('repair_asset')
  return `?${params.toString()}`
}

export function clearAINativeWorkspaceLocation(search: string): string {
  const params = new URLSearchParams(search)
  params.delete('workspace')
  params.delete('stage')
  params.delete('repair_asset')
  const value = params.toString()
  return value ? `?${value}` : ''
}
