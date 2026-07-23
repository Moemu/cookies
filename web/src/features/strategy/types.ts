export type ProposalInput = {
  brand: string
  product: string
  target_audience: string
  platform: string
  budget: string
  compliance: string[]
}

export type StrategyOutput = {
  id: string
  proposal_id: string
  status: 'draft' | 'approved'
  content: Record<string, unknown>
  approved_at?: string
}

export type StrategyProposal = {
  id: string
  project_id: string
  status: 'created' | 'generating' | 'generated' | 'failed'
  template_version: string
  input: ProposalInput
  strategy?: StrategyOutput
  created_at: string
  updated_at: string
}
