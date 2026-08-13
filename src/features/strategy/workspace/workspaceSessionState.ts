import type { StrategyStage } from './workspaceRoute'

const WORKSPACE_SESSION_PREFIX = 'cookies.strategy.workspace-session.v1'
const volatileWorkspaceSession = new Map<string, string>()

type StoredWorkspaceValue<T> = {
  schema_version: 1
  value: T
}

type WorkspaceSessionStorage = Pick<Storage, 'getItem' | 'setItem' | 'removeItem'>

export function workspaceSessionKey(
  projectId: string,
  workspaceId: string,
  resource: string,
  ...parts: string[]
) {
  return [WORKSPACE_SESSION_PREFIX, projectId, workspaceId, resource, ...parts]
    .map(value => encodeURIComponent(value || 'unknown'))
    .join(':')
}

export function stageScrollSessionKey(projectId: string, workspaceId: string, stage: StrategyStage) {
  return workspaceSessionKey(projectId, workspaceId, 'stage-scroll', stage)
}

export function readWorkspaceSessionValue<T>(key: string, storage?: WorkspaceSessionStorage | null): T | null {
  const candidates: string[] = []
  try {
    const target = storage === undefined ? browserSessionStorage() : storage
    const persisted = target?.getItem(key)
    if (persisted) candidates.push(persisted)
  } catch {
    // Fall through to the in-memory copy when browser storage is unavailable.
  }
  if (storage === undefined) {
    const volatile = volatileWorkspaceSession.get(key)
    if (volatile && !candidates.includes(volatile)) candidates.push(volatile)
  }
  for (const raw of candidates) {
    try {
      const stored = JSON.parse(raw) as Partial<StoredWorkspaceValue<T>>
      if (stored.schema_version === 1 && Object.prototype.hasOwnProperty.call(stored, 'value')) return stored.value as T
    } catch {
      // Ignore a corrupt candidate and try the in-memory recovery copy.
    }
  }
  return null
}

export function writeWorkspaceSessionValue<T>(key: string, value: T, storage?: WorkspaceSessionStorage | null) {
  try {
    const serialized = JSON.stringify({ schema_version: 1, value } satisfies StoredWorkspaceValue<T>)
    if (storage === undefined) volatileWorkspaceSession.set(key, serialized)
    const target = storage === undefined ? browserSessionStorage() : storage
    if (!target) return
    target.setItem(key, serialized)
  } catch {
    // Session recovery is a convenience boundary and must never block editing.
  }
}

export function clearWorkspaceSessionValue(key: string, storage?: WorkspaceSessionStorage | null) {
  if (storage === undefined) volatileWorkspaceSession.delete(key)
  try {
    const target = storage === undefined ? browserSessionStorage() : storage
    if (!target) return
    target.removeItem(key)
  } catch {
    // Session recovery is a convenience boundary and must never block editing.
  }
}

function browserSessionStorage(): WorkspaceSessionStorage | null {
  return typeof window === 'undefined' ? null : window.sessionStorage
}
