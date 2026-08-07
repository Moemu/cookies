import { useCallback, useEffect, useRef, useState } from 'react'
import { updateRequirement, updateScript, updateStoryboard } from './api'
import { createSerialAutosave, type AutosaveStatus, type SerialAutosave } from './autosave'
import { forgetAINativeStageDraft } from './storage'
import type { AINativeAction } from './reducer'
import type { AdScriptDraft, AINativeFrontendState, AINativeRequirement, AINativeRequirementWorkspace, AINativeStageId, StoryboardDraft } from './types'
import { autosaveRevisionFor, syncAutosaveRevisionCursor, type WorkspaceAutosaveRevisions } from './workspaceAutosaveRevision'

type EditableStage = Extract<AINativeStageId, 'requirement' | 'script' | 'storyboard'>
type Controllers = {
  workspaceId: string
  requirement: SerialAutosave<AINativeRequirement>
  script: SerialAutosave<AdScriptDraft>
  storyboard: SerialAutosave<StoryboardDraft>
}

function revisionsFromWorkspace(workspace: AINativeRequirementWorkspace): WorkspaceAutosaveRevisions {
  const storyboard = workspace.storyboard ?? workspace.storyboard_plan
  return {
    workspaceId: workspace.workspace_id,
    requirementRevision: workspace.requirement.revision,
    scriptRevision: workspace.script?.revision ?? 0,
    storyboardRevision: storyboard?.revision ?? 0,
  }
}

function revisionsFromState(state: AINativeFrontendState): WorkspaceAutosaveRevisions | null {
  const workspace = state.workspace
  if (!workspace) return null
  return {
    workspaceId: workspace.workspace_id,
    requirementRevision: workspace.requirement.revision,
    scriptRevision: state.script?.revision ?? 0,
    storyboardRevision: state.storyboard?.revision ?? 0,
  }
}

function editableFingerprint<T extends { revision: number }>(value: T) {
  return JSON.stringify({ ...value, revision: 0 })
}

function asAutosaveError(cause: unknown) {
  const candidate = cause as { status?: number; code?: string; message?: string } | null
  if (candidate?.status === 412 || candidate?.code === 'CREATIVE_VERSION_CONFLICT') {
    return new Error('该广告已在其他页面产生新版本。当前修改仍保留在本地，请重新打开该广告后再保存。')
  }
  return cause instanceof Error ? cause : new Error('AI 效果广告自动保存失败，请重试。')
}

export function useAINativeAutosave(projectId: string, state: AINativeFrontendState, dispatch: React.Dispatch<AINativeAction>) {
  const stateRef = useRef(state)
  const controllersRef = useRef<Controllers | null>(null)
  const latestWorkspaceRef = useRef<AINativeRequirementWorkspace | null>(state.workspace)
  const revisionRef = useRef<WorkspaceAutosaveRevisions | null>(null)
  const lastErrorRef = useRef<Error | null>(null)
  const [status, setStatus] = useState<AutosaveStatus>('idle')
  const [savedAt, setSavedAt] = useState<Date | null>(null)

  stateRef.current = state
  latestWorkspaceRef.current = state.workspace
  const incomingRevisions = revisionsFromState(state)
  revisionRef.current = incomingRevisions ? syncAutosaveRevisionCursor(revisionRef.current, incomingRevisions) : null

  useEffect(() => {
    const workspaceId = state.workspace?.workspace_id
    if (!workspaceId) {
      controllersRef.current = null
      setStatus('idle')
      lastErrorRef.current = null
      return
    }

    const isActiveWorkspace = () => stateRef.current.workspace?.workspace_id === workspaceId
    const updateStatus = (next: AutosaveStatus) => {
      if (isActiveWorkspace()) setStatus(next)
    }
    const onError = (cause: unknown) => {
      if (isActiveWorkspace()) lastErrorRef.current = asAutosaveError(cause)
    }
    const recordSavedWorkspace = (workspace: AINativeRequirementWorkspace) => {
      if (!isActiveWorkspace() || workspace.workspace_id !== workspaceId) return false
      revisionRef.current = syncAutosaveRevisionCursor(revisionRef.current, revisionsFromWorkspace(workspace))
      latestWorkspaceRef.current = workspace
      lastErrorRef.current = null
      setSavedAt(new Date())
      return true
    }

    const requirement = createSerialAutosave<AINativeRequirement, AINativeRequirementWorkspace>({
      delayMs: 1200,
      fingerprint: editableFingerprint,
      save: value => updateRequirement(projectId, workspaceId, {
        ...value,
        revision: autosaveRevisionFor(revisionRef.current, workspaceId, 'requirement'),
      }),
      onSaved: (value, workspace) => {
        if (!recordSavedWorkspace(workspace)) return
        forgetAINativeStageDraft(projectId, 'requirement')
        const latest = stateRef.current.workspace?.requirement
        dispatch({
          type: 'requirement-loaded',
          workspace: latest && editableFingerprint(latest) !== editableFingerprint(value)
            ? { ...workspace, requirement: { ...latest, revision: workspace.requirement.revision } }
            : workspace,
        })
      },
      onError,
      onStatus: updateStatus,
    })

    const script = createSerialAutosave<AdScriptDraft, AINativeRequirementWorkspace>({
      delayMs: 1200,
      fingerprint: editableFingerprint,
      save: value => updateScript(projectId, workspaceId, {
        ...value,
        revision: autosaveRevisionFor(revisionRef.current, workspaceId, 'script'),
      }),
      onSaved: (value, workspace) => {
        if (!recordSavedWorkspace(workspace)) return
        forgetAINativeStageDraft(projectId, 'script')
        const latest = stateRef.current.script
        dispatch({
          type: 'requirement-loaded',
          workspace: latest && workspace.script && editableFingerprint(latest) !== editableFingerprint(value)
            ? { ...workspace, script: { ...latest, revision: workspace.script.revision } }
            : workspace,
        })
      },
      onError,
      onStatus: updateStatus,
    })

    const storyboard = createSerialAutosave<StoryboardDraft, AINativeRequirementWorkspace>({
      delayMs: 1200,
      fingerprint: editableFingerprint,
      save: value => updateStoryboard(projectId, workspaceId, {
        ...value,
        revision: autosaveRevisionFor(revisionRef.current, workspaceId, 'storyboard'),
      }),
      onSaved: (value, workspace) => {
        if (!recordSavedWorkspace(workspace)) return
        forgetAINativeStageDraft(projectId, 'storyboard')
        const latest = stateRef.current.storyboard
        const saved = workspace.storyboard ?? workspace.storyboard_plan
        const merged = latest && saved && editableFingerprint(latest) !== editableFingerprint(value)
          ? { ...latest, revision: saved.revision }
          : null
        dispatch({
          type: 'requirement-loaded',
          workspace: merged ? { ...workspace, storyboard: merged, storyboard_plan: merged } : workspace,
        })
      },
      onError,
      onStatus: updateStatus,
    })

    controllersRef.current = { workspaceId, requirement, script, storyboard }
    lastErrorRef.current = null
    setStatus('idle')
    return () => {
      requirement.dispose()
      script.dispose()
      storyboard.dispose()
      if (controllersRef.current?.workspaceId === workspaceId) controllersRef.current = null
    }
  }, [dispatch, projectId, state.workspace?.workspace_id])

  const schedule = useCallback(<T extends AINativeRequirement | AdScriptDraft | StoryboardDraft>(stage: EditableStage, value: T) => {
    const workspaceId = stateRef.current.workspace?.workspace_id
    const controllers = controllersRef.current
    if (!workspaceId || !controllers || controllers.workspaceId !== workspaceId) return
    const controller = controllers[stage] as SerialAutosave<T>
    controller.schedule(value)
  }, [])

  const flush = useCallback(async (stage: EditableStage) => {
    const workspaceId = stateRef.current.workspace?.workspace_id
    const controllers = controllersRef.current
    if (!workspaceId || !controllers || controllers.workspaceId !== workspaceId) {
      return { succeeded: false, workspace: stateRef.current.workspace }
    }
    const succeeded = await controllers[stage].flush()
    const stillActive = stateRef.current.workspace?.workspace_id === workspaceId
    return {
      succeeded: succeeded !== false && stillActive,
      workspace: stillActive ? latestWorkspaceRef.current ?? stateRef.current.workspace : stateRef.current.workspace,
    }
  }, [])

  const flushAll = useCallback(async () => {
    const workspaceId = stateRef.current.workspace?.workspace_id
    const controllers = controllersRef.current
    if (!workspaceId || !controllers || controllers.workspaceId !== workspaceId) {
      return { succeeded: false, workspace: stateRef.current.workspace }
    }
    for (const stage of ['requirement', 'script', 'storyboard'] as const) {
      if (!await controllers[stage].flush() || stateRef.current.workspace?.workspace_id !== workspaceId) {
        return { succeeded: false, workspace: stateRef.current.workspace }
      }
    }
    return { succeeded: true, workspace: latestWorkspaceRef.current ?? stateRef.current.workspace }
  }, [])

  const lastError = useCallback(() => lastErrorRef.current, [])

  return { status, savedAt, schedule, flush, flushAll, lastError }
}
