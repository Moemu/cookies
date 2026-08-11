import { useCallback, useEffect, useState } from 'react'
import { strategyApi } from '../api'
import type { ArtifactProposal, BriefPatchOperation } from '../types'

export function useResearchAdoptionProposals(
	workspaceId: string,
	researchRunId: string,
	revisionKey: string,
	onArtifactChanged: () => Promise<void>,
) {
	const [items, setItems] = useState<ArtifactProposal[]>([])
	const [busyId, setBusyId] = useState('')
	const [error, setError] = useState('')

	const refresh = useCallback(async (signal?: AbortSignal) => {
		if (!workspaceId || !researchRunId) {
			setItems([])
			return
		}
		try {
			const result = await strategyApi.listResearchAdoptionProposals(workspaceId, researchRunId, signal)
			setItems(result.items)
			setError('')
		} catch (reason) {
			if (signal?.aborted) return
			setError(reason instanceof Error ? reason.message : '研究采纳建议加载失败')
		}
	}, [researchRunId, workspaceId])

	useEffect(() => {
		const controller = new AbortController()
		void refresh(controller.signal)
		return () => controller.abort()
	}, [refresh, revisionKey])

	const perform = useCallback(async (
		proposal: ArtifactProposal,
		action: 'apply' | 'ignore' | 'remap',
		operations?: BriefPatchOperation[],
	) => {
		setBusyId(proposal.id)
		setError('')
		try {
			if (action === 'apply') {
				await strategyApi.applyResearchAdoptionProposal(proposal.id, proposal.version, operations)
				await onArtifactChanged()
			} else if (action === 'remap') {
				await strategyApi.remapResearchAdoptionProposal(proposal.id, proposal.version)
			} else {
				await strategyApi.ignoreResearchAdoptionProposal(proposal.id, proposal.version)
			}
			await refresh()
			return true
		} catch (reason) {
			setError(reason instanceof Error ? reason.message : '研究采纳操作失败')
			await refresh()
			return false
		} finally {
			setBusyId('')
		}
	}, [onArtifactChanged, refresh])

	return {
		items,
		busyId,
		error,
		refresh,
		apply: (proposal: ArtifactProposal, operations?: BriefPatchOperation[]) => perform(proposal, 'apply', operations),
		ignore: (proposal: ArtifactProposal) => perform(proposal, 'ignore'),
		remap: (proposal: ArtifactProposal) => perform(proposal, 'remap'),
	}
}
