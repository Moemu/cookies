import { apiRequest } from '../../shared/api/client'
import type { ProposalInput, StrategyOutput, StrategyProposal } from './types'

const projectPath = (projectId: string) => `/api/strategy/v1/projects/${encodeURIComponent(projectId)}`

export function createProposal(projectId: string, input: ProposalInput, signal?: AbortSignal) {
  return apiRequest<StrategyProposal>(`${projectPath(projectId)}/proposals`, {
    method: 'POST',
    headers: { 'Idempotency-Key': `web-proposal-${crypto.randomUUID()}` },
    body: JSON.stringify(input),
    signal,
  })
}

export function generateStrategy(projectId: string, proposalId: string, signal?: AbortSignal) {
  return apiRequest<StrategyProposal>(`${projectPath(projectId)}/proposals/${encodeURIComponent(proposalId)}/generate`, {
    method: 'POST',
    headers: { 'Idempotency-Key': `web-strategy-${crypto.randomUUID()}` },
    signal,
  })
}

export function approveStrategy(projectId: string, strategyId: string, signal?: AbortSignal) {
  return apiRequest<StrategyOutput>(`${projectPath(projectId)}/strategies/${encodeURIComponent(strategyId)}/approve`, {
    method: 'POST',
    headers: { 'Idempotency-Key': `web-strategy-approval-${crypto.randomUUID()}` },
    signal,
  })
}
