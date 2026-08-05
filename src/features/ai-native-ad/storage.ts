import type { AINativeStageId } from './types'

export type AINativeWorkspacePointer = {
  workspaceId: string
  stage: AINativeStageId
}

type StorageLike = Pick<Storage, 'getItem' | 'setItem' | 'removeItem'>

const stages = new Set<AINativeStageId>(['requirement', 'script', 'storyboard', 'video'])
const pointerVersion = 1

function pointerKey(projectId: string) {
  return `cookies.ai-native.current.v1:${projectId}`
}

function draftKey(projectId: string, stage: AINativeStageId) {
  return `cookies.ai-native.draft.v1:${projectId}:${stage}`
}

function browserStorage(): StorageLike | null {
  if (typeof window === 'undefined') return null
  try {
    return window.localStorage
  } catch {
    return null
  }
}

export function readAINativeWorkspacePointer(projectId: string, storage: StorageLike | null = browserStorage()): AINativeWorkspacePointer | null {
  if (!storage) return null
  try {
    const raw = storage.getItem(pointerKey(projectId))
    if (!raw) return null
    const value = JSON.parse(raw) as { version?: unknown; workspaceId?: unknown; stage?: unknown }
    if (value.version !== pointerVersion || typeof value.workspaceId !== 'string' || !value.workspaceId.trim() || !stages.has(value.stage as AINativeStageId)) return null
    return { workspaceId: value.workspaceId, stage: value.stage as AINativeStageId }
  } catch {
    return null
  }
}

export function rememberAINativeWorkspace(projectId: string, pointer: AINativeWorkspacePointer, storage: StorageLike | null = browserStorage()) {
  if (!storage || !projectId.trim() || !pointer.workspaceId.trim()) return
  try {
    storage.setItem(pointerKey(projectId), JSON.stringify({ version: pointerVersion, ...pointer }))
  } catch {
    // Persistence is a recovery aid; storage policy or quota must not block editing.
  }
}

export function forgetAINativeWorkspace(projectId: string, storage: StorageLike | null = browserStorage()) {
  if (!storage) return
  try {
    storage.removeItem(pointerKey(projectId))
  } catch {
    // Ignore unavailable browser storage.
  }
}

export function readAINativeStageDraft<T>(projectId: string, workspaceId: string, stage: AINativeStageId, baseRevision: number, storage: StorageLike | null = browserStorage()): T | null {
  if (!storage) return null
  try {
    const raw = storage.getItem(draftKey(projectId, stage))
    if (!raw) return null
    const value = JSON.parse(raw) as { version?: unknown; workspaceId?: unknown; baseRevision?: unknown; value?: T }
    if (value.version !== pointerVersion || value.workspaceId !== workspaceId || value.baseRevision !== baseRevision || value.value === undefined) return null
    return value.value
  } catch {
    return null
  }
}

export function rememberAINativeStageDraft<T>(projectId: string, workspaceId: string, stage: AINativeStageId, baseRevision: number, value: T, storage: StorageLike | null = browserStorage()) {
  if (!storage || !projectId.trim() || !workspaceId.trim()) return
  try {
    storage.setItem(draftKey(projectId, stage), JSON.stringify({ version: pointerVersion, workspaceId, baseRevision, value }))
  } catch {
    // Local draft persistence is best-effort and must not block the editor.
  }
}

export function forgetAINativeStageDraft(projectId: string, stage: AINativeStageId, storage: StorageLike | null = browserStorage()) {
  if (!storage) return
  try {
    storage.removeItem(draftKey(projectId, stage))
  } catch {
    // Ignore unavailable browser storage.
  }
}
