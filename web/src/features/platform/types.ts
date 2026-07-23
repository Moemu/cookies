export type Principal = {
  kind: 'user' | 'service'
  id: string
}

export type ActorContext = {
  organization_id: string
  principal: Principal
  scopes: string[]
}

export type CurrentIdentity = {
  actor: ActorContext
  organization: {
    id: string
    name: string
    status: 'active' | 'suspended' | 'archived'
  }
  user: null | {
    id: string
    display_name: string
    status: 'active' | 'suspended' | 'archived'
  }
  membership: null | {
    organization_id: string
    user_id: string
    role: string
    status: 'active' | 'suspended' | 'removed'
  }
}

export type Project = {
  id: string
  organization_id: string
  name: string
  status: 'draft' | 'active' | 'archived'
  primary_brand_id: string | null
  brand_guideline_version_id?: string
  project_context_version: number
  created_at: string
  updated_at: string
}

export type Brand = {
  id: string
  organization_id: string
  name: string
  status: 'active' | 'archived'
  created_at: string
  updated_at: string
}

export type CreateProjectInput = {
  name: string
  primary_brand_id: string | null
  product_ids: string[]
  activate: boolean
}

export type ProjectContext = {
  organization_id: string
  project_id: string
  brand_id: string | null
  product_ids: string[]
  brand_guideline_version_id?: string
  project_context_version: number
}

export type WorkspaceBootstrap = {
  identity: CurrentIdentity
  projects: Project[]
}

export type ProviderJob = {
  id: string
  kind: string
  organization_id: string
  project_id: string
  execution_status: 'queued' | 'running' | 'succeeded' | 'failed' | 'cancelled'
  provider_status: 'submitted' | 'running' | 'outputs_ready' | 'ingesting' | 'succeeded' | 'partially_succeeded' | 'failed' | 'cancelled' | 'expired'
  progress: number
  project_asset_refs: Array<{ project_id: string, asset_version: { asset_id: string, version: number } }>
  error: null | { code: string, message: string, retryable: boolean }
  attempt_count: number
  max_attempts: number
  version: number
  created_at: string
  updated_at: string
}
