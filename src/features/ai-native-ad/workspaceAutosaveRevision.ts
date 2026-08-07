export type AutosaveEditableStage = 'requirement' | 'script' | 'storyboard'

export type WorkspaceAutosaveRevisions = {
  workspaceId: string
  requirementRevision: number
  scriptRevision: number
  storyboardRevision: number
}

function normalizeRevision(value: number) {
  return Number.isFinite(value) && value > 0 ? Math.floor(value) : 0
}

export function syncAutosaveRevisionCursor(
  current: WorkspaceAutosaveRevisions | null,
  incoming: WorkspaceAutosaveRevisions,
): WorkspaceAutosaveRevisions {
  const normalized = {
    workspaceId: incoming.workspaceId,
    requirementRevision: normalizeRevision(incoming.requirementRevision),
    scriptRevision: normalizeRevision(incoming.scriptRevision),
    storyboardRevision: normalizeRevision(incoming.storyboardRevision),
  }
  if (!current || current.workspaceId !== normalized.workspaceId) return normalized
  return {
    workspaceId: current.workspaceId,
    requirementRevision: Math.max(current.requirementRevision, normalized.requirementRevision),
    scriptRevision: Math.max(current.scriptRevision, normalized.scriptRevision),
    storyboardRevision: Math.max(current.storyboardRevision, normalized.storyboardRevision),
  }
}

export function autosaveRevisionFor(
  cursor: WorkspaceAutosaveRevisions | null,
  workspaceId: string,
  stage: AutosaveEditableStage,
) {
  if (!cursor || !workspaceId || cursor.workspaceId !== workspaceId) {
    throw new Error('AI native autosave workspace changed before the pending save completed')
  }
  if (stage === 'requirement') return cursor.requirementRevision
  if (stage === 'script') return cursor.scriptRevision
  return cursor.storyboardRevision
}
