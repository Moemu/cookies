import { useCallback, useEffect, useRef, useState } from 'react'
import { updateRequirement, updateScript, updateStoryboard } from './api'
import { createSerialAutosave, type AutosaveStatus, type SerialAutosave } from './autosave'
import { forgetAINativeStageDraft } from './storage'
import type { AINativeAction } from './reducer'
import type { AdScriptDraft, AINativeFrontendState, AINativeRequirement, AINativeRequirementWorkspace, AINativeStageId, StoryboardDraft } from './types'

type EditableStage = Extract<AINativeStageId, 'requirement' | 'script' | 'storyboard'>
type Controllers = {
  requirement: SerialAutosave<AINativeRequirement>
  script: SerialAutosave<AdScriptDraft>
  storyboard: SerialAutosave<StoryboardDraft>
}

function editableFingerprint<T extends { revision: number }>(value: T) {
  return JSON.stringify({ ...value, revision: 0 })
}

export function useAINativeAutosave(projectId: string, state: AINativeFrontendState, dispatch: React.Dispatch<AINativeAction>) {
  const stateRef = useRef(state)
  const controllersRef = useRef<Controllers | null>(null)
  const latestWorkspaceRef = useRef<AINativeRequirementWorkspace | null>(state.workspace)
  const revisionRef = useRef({ requirement: 0, script: 0, storyboard: 0 })
  const [status, setStatus] = useState<AutosaveStatus>('idle')
  const [savedAt, setSavedAt] = useState<Date | null>(null)

  stateRef.current = state
  latestWorkspaceRef.current = state.workspace
  revisionRef.current.requirement = Math.max(revisionRef.current.requirement, state.workspace?.requirement.revision ?? 0)
  revisionRef.current.script = Math.max(revisionRef.current.script, state.script?.revision ?? 0)
  revisionRef.current.storyboard = Math.max(revisionRef.current.storyboard, state.storyboard?.revision ?? 0)

  useEffect(() => {
    const updateStatus = (next: AutosaveStatus) => setStatus(next)
    const onError = () => undefined
    const requirement = createSerialAutosave<AINativeRequirement, AINativeRequirementWorkspace>({
      delayMs: 1200,
      fingerprint: editableFingerprint,
      save: value => {
        const workspace = stateRef.current.workspace
        if (!workspace) throw new Error('没有可保存的 AI 原生广告工作区。')
        return updateRequirement(projectId, workspace.workspace_id, { ...value, revision: revisionRef.current.requirement })
      },
      onSaved: (value, workspace) => {
        revisionRef.current.requirement = workspace.requirement.revision
        latestWorkspaceRef.current = workspace
        forgetAINativeStageDraft(projectId, 'requirement')
        const latest = stateRef.current.workspace?.requirement
        dispatch({ type: 'requirement-loaded', workspace: latest && editableFingerprint(latest) !== editableFingerprint(value) ? { ...workspace, requirement: { ...latest, revision: workspace.requirement.revision } } : workspace })
        setSavedAt(new Date())
      },
      onError,
      onStatus: updateStatus,
    })
    const script = createSerialAutosave<AdScriptDraft, AINativeRequirementWorkspace>({
      delayMs: 1200,
      fingerprint: editableFingerprint,
      save: value => {
        const workspace = stateRef.current.workspace
        if (!workspace) throw new Error('没有可保存的 AI 原生广告工作区。')
        return updateScript(projectId, workspace.workspace_id, { ...value, revision: revisionRef.current.script })
      },
      onSaved: (value, workspace) => {
        revisionRef.current.script = workspace.script?.revision ?? revisionRef.current.script
        latestWorkspaceRef.current = workspace
        forgetAINativeStageDraft(projectId, 'script')
        const latest = stateRef.current.script
        dispatch({ type: 'requirement-loaded', workspace: latest && workspace.script && editableFingerprint(latest) !== editableFingerprint(value) ? { ...workspace, script: { ...latest, revision: workspace.script.revision } } : workspace })
        setSavedAt(new Date())
      },
      onError,
      onStatus: updateStatus,
    })
    const storyboard = createSerialAutosave<StoryboardDraft, AINativeRequirementWorkspace>({
      delayMs: 1200,
      fingerprint: editableFingerprint,
      save: value => {
        const workspace = stateRef.current.workspace
        if (!workspace) throw new Error('没有可保存的 AI 原生广告工作区。')
        return updateStoryboard(projectId, workspace.workspace_id, { ...value, revision: revisionRef.current.storyboard })
      },
      onSaved: (value, workspace) => {
        const saved = workspace.storyboard ?? workspace.storyboard_plan
        revisionRef.current.storyboard = saved?.revision ?? revisionRef.current.storyboard
        latestWorkspaceRef.current = workspace
        forgetAINativeStageDraft(projectId, 'storyboard')
        const latest = stateRef.current.storyboard
        const merged = latest && saved && editableFingerprint(latest) !== editableFingerprint(value) ? { ...latest, revision: saved.revision } : null
        dispatch({ type: 'requirement-loaded', workspace: merged ? { ...workspace, storyboard: merged, storyboard_plan: merged } : workspace })
        setSavedAt(new Date())
      },
      onError,
      onStatus: updateStatus,
    })
    controllersRef.current = { requirement, script, storyboard }
    setStatus('idle')
    return () => {
      requirement.dispose()
      script.dispose()
      storyboard.dispose()
      controllersRef.current = null
    }
  }, [dispatch, projectId, state.workspace?.workspace_id])

  const schedule = useCallback(<T extends AINativeRequirement | AdScriptDraft | StoryboardDraft>(stage: EditableStage, value: T) => {
    const controller = controllersRef.current?.[stage] as SerialAutosave<T> | undefined
    controller?.schedule(value)
  }, [])

  const flush = useCallback(async (stage: EditableStage) => {
    const succeeded = await controllersRef.current?.[stage].flush()
    return { succeeded: succeeded !== false, workspace: latestWorkspaceRef.current ?? stateRef.current.workspace }
  }, [])

  const flushAll = useCallback(async () => {
    const controllers = controllersRef.current
    if (!controllers) return { succeeded: true, workspace: latestWorkspaceRef.current ?? stateRef.current.workspace }
    for (const stage of ['requirement', 'script', 'storyboard'] as const) {
      if (!await controllers[stage].flush()) {
        return { succeeded: false, workspace: latestWorkspaceRef.current ?? stateRef.current.workspace }
      }
    }
    return { succeeded: true, workspace: latestWorkspaceRef.current ?? stateRef.current.workspace }
  }, [])

  return { status, savedAt, schedule, flush, flushAll }
}
