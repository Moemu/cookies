export type AssetStatus = 'processing' | 'ready' | 'quarantined' | 'failed' | 'archived'
export type AssetSource = 'upload' | 'provider_generated' | 'rendered' | 'imported' | 'captured'

export type AssetVersionRef = {
  asset_id: string
  version: number
}

export type ProjectAsset = {
  ref: {
    project_id: string
    asset_version: AssetVersionRef
  }
  asset: {
    id: string
    organization_id: string
    asset_kind: string
    status: AssetStatus
    owner_system: string
    latest_version: number
    created_at: string
    updated_at: string
  }
  version: {
    organization_id: string
    asset_id: string
    version: number
    status: AssetStatus
    source_type: AssetSource
    mime_type: string
    size_bytes: number
    sha256: string
    width_pixels?: number
    height_pixels?: number
    media: {
      duration_seconds?: number
      fps?: number
      codec?: string
      bitrate_bps?: number
      audio_codec?: string
      audio_channels?: number
      audio_sample_rate?: number
      poster_frame_ref?: string
      probe_status: 'not_required' | 'succeeded' | 'failed'
      probe_error?: string
    }
    duration_ms?: number
    frame_rate?: string
    video_codec?: string
    audio_codec?: string
    provider_job_id?: string
    provider_output_id?: string
    render_job_id?: string
    project_context_version?: number
    created_at: string
  }
  created_at: string
}

export type AssetFeature = {
  organization_id: string
  project_id: string
  asset_id: string
  asset_version: number
  schema_version: 'asset_feature_v1'
  feature_version: string
  hook_strength: number
  product_visibility: number
  scene_tags: string[]
  product_tags: string[]
  person_tags: string[]
  action_tags: string[]
  emotion_tags: string[]
  selling_points: string[]
  cta_presence: boolean
  similarity_group?: string
  similarity_risk: 'low' | 'medium' | 'high'
  evidence: string[]
  created_at: string
  updated_at: string
}

export type SignedRequest = {
  url: string
  method: 'GET' | 'PUT'
  headers: Record<string, string>
  expires_at: string
}

export type UploadSession = {
  id: string
  organization_id: string
  project_id: string
  status: 'created' | 'uploaded' | 'processing' | 'succeeded' | 'failed' | 'expired'
  filename: string
  declared_mime_type: string
  declared_size_bytes: number
  declared_sha256: string | null
  project_context_version: number
  project_asset_ref: null | {
    project_id: string
    asset_version: AssetVersionRef
  }
  error_code?: string
  expires_at: string
  created_at: string
  updated_at: string
}

export type CreateUploadResponse = {
  session: UploadSession
  upload: SignedRequest | null
}
