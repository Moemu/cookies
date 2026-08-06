import type { AINativeStageId } from './types'

const stages = new Set<AINativeStageId>(['requirement', 'script', 'storyboard', 'video'])

export function readAINativeWorkspaceLocation(search: string): { workspaceId: string; stage: AINativeStageId } | null {
  const params = new URLSearchParams(search)
  const workspaceId = params.get('workspace')?.trim() ?? ''
  const rawStage = params.get('stage') ?? 'requirement'
  if (!workspaceId || !stages.has(rawStage as AINativeStageId)) return null
  return { workspaceId, stage: rawStage as AINativeStageId }
}

export function aiNativeWorkspaceLocation(search: string, workspaceId: string, stage: AINativeStageId): string {
  const params = new URLSearchParams(search)
  params.set('workspace', workspaceId)
  params.set('stage', stage)
  return `?${params.toString()}`
}

export function clearAINativeWorkspaceLocation(search: string): string {
  const params = new URLSearchParams(search)
  params.delete('workspace')
  params.delete('stage')
  const value = params.toString()
  return value ? `?${value}` : ''
}
