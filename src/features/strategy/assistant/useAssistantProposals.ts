import { useCallback, useEffect, useState } from 'react'
import { BackendApiError } from '../../../backend/platform'
import { createMutationKey, strategyApi } from '../api'
import type { ArtifactProposal, BriefPatchOperation } from '../types'

function proposalErrorMessage(error: unknown) {
  if (error instanceof BackendApiError && error.code === 'VERSION_CONFLICT') {
    return '建议基于的内容已经变化，已停止覆盖。请查看最新 Brief 后让 AI 重新建议。'
  }
  return error instanceof Error ? error.message : '建议操作失败，请重试。'
}

export function useAssistantProposals(
  workspaceId: string,
  revisionKey: string,
  onApplied: () => Promise<void>,
) {
  const [proposals, setProposals] = useState<ArtifactProposal[]>([])
  const [busyId, setBusyId] = useState('')
  const [error, setError] = useState('')

  const reload = useCallback(async (signal?: AbortSignal) => {
    if (!workspaceId) {
      setProposals([])
      return
    }
    const result = await strategyApi.listAssistantProposals(workspaceId, 'proposed', signal)
    setProposals(result.items)
  }, [workspaceId])

  useEffect(() => {
    const controller = new AbortController()
    setError('')
    void reload(controller.signal).catch(value => {
      if (!(value instanceof DOMException && value.name === 'AbortError')) {
        setError(proposalErrorMessage(value))
      }
    })
    return () => controller.abort()
  }, [reload, revisionKey])

  const apply = useCallback(async (proposal: ArtifactProposal, operations?: BriefPatchOperation[]) => {
    setBusyId(proposal.id)
    setError('')
    try {
      await strategyApi.applyAssistantProposal(
        proposal,
        operations ?? proposal.operations,
        createMutationKey('strategy-assistant-proposal-apply'),
      )
      await onApplied()
      await reload()
      return true
    } catch (value) {
      setError(proposalErrorMessage(value))
      await reload().catch(() => undefined)
      return false
    } finally {
      setBusyId('')
    }
  }, [onApplied, reload])

  const ignore = useCallback(async (proposal: ArtifactProposal) => {
    setBusyId(proposal.id)
    setError('')
    try {
      await strategyApi.ignoreAssistantProposal(
        proposal,
        createMutationKey('strategy-assistant-proposal-ignore'),
      )
      await reload()
      return true
    } catch (value) {
      setError(proposalErrorMessage(value))
      await reload().catch(() => undefined)
      return false
    } finally {
      setBusyId('')
    }
  }, [reload])

  return { proposals, busyId, error, apply, ignore, reload }
}
