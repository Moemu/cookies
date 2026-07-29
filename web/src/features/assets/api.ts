import { apiRequest } from '../../shared/api/client'
import { createClientUUID } from '../../shared/clientId'
import type { BulkRemixPlan } from './aiRemixPlanner'
import type { AssetFeature, CreateUploadResponse, ProjectAsset, SignedRequest, UploadSession } from './types'

export function listProjectAssets(projectId: string, signal?: AbortSignal) {
  return apiRequest<{ items: ProjectAsset[] }>(`/platform/v1/projects/${encodeURIComponent(projectId)}/assets?limit=100`, { signal })
}

export function getAssetPreview(projectId: string, assetId: string, version: number, signal?: AbortSignal) {
  return apiRequest<SignedRequest>(`/platform/v1/projects/${encodeURIComponent(projectId)}/assets/${encodeURIComponent(assetId)}/versions/${version}/preview`, { signal })
}

export function removeProjectAsset(projectId: string, assetId: string, version: number, signal?: AbortSignal) {
  return apiRequest<void>(`/platform/v1/projects/${encodeURIComponent(projectId)}/assets/${encodeURIComponent(assetId)}/versions/${version}`, {
    method: 'DELETE',
    signal,
  })
}

export function listAssetFeatures(projectId: string, signal?: AbortSignal) {
  return apiRequest<{ items: AssetFeature[] }>(`/platform/v1/projects/${encodeURIComponent(projectId)}/assets/features?limit=200`, { signal })
}

export function getAssetFeature(projectId: string, assetId: string, version: number, featureVersion: string, signal?: AbortSignal) {
  return apiRequest<{ feature: AssetFeature | null }>(`/platform/v1/projects/${encodeURIComponent(projectId)}/assets/${encodeURIComponent(assetId)}/versions/${version}/features/${encodeURIComponent(featureVersion)}`, { signal })
}

export function upsertAssetFeature(projectId: string, feature: AssetFeature, signal?: AbortSignal) {
  return apiRequest<AssetFeature>(`/platform/v1/projects/${encodeURIComponent(projectId)}/assets/${encodeURIComponent(feature.asset_id)}/versions/${feature.asset_version}/features/${encodeURIComponent(feature.feature_version)}`, {
    method: 'PUT',
    signal,
    body: JSON.stringify(feature),
  })
}

export function createAssetUpload(projectId: string, file: File, sha256: string, signal?: AbortSignal) {
  return apiRequest<CreateUploadResponse>(`/platform/v1/projects/${encodeURIComponent(projectId)}/assets/uploads`, {
    method: 'POST',
    signal,
    headers: { 'Idempotency-Key': `web-upload-${createClientUUID()}` },
    body: JSON.stringify({
      filename: file.name,
      declared_mime_type: file.type,
      declared_size_bytes: file.size,
      declared_sha256: sha256,
    }),
  })
}

export async function putAssetContent(request: SignedRequest, file: File, signal?: AbortSignal) {
  if (!request.url) throw new Error('上传地址为空，请重新创建上传会话。')
  const headers = new Headers()
  for (const [name, value] of Object.entries(request.headers)) {
    const normalizedName = name.toLowerCase()
    if (normalizedName !== 'host' && normalizedName !== 'content-length') headers.set(name, value)
  }
  if (!headers.has('Content-Type')) headers.set('Content-Type', file.type)
  const response = await fetch(request.url, { method: request.method, headers, body: file, signal })
  if (!response.ok) throw new Error(`文件上传失败（HTTP ${response.status}）`)
}

export function finalizeAssetUpload(projectId: string, uploadId: string, signal?: AbortSignal) {
  return apiRequest<UploadSession>(`/platform/v1/projects/${encodeURIComponent(projectId)}/assets/uploads/${encodeURIComponent(uploadId)}:finalize`, {
    method: 'POST',
    signal,
  })
}

export async function calculateSHA256(file: File) {
  const digest = await crypto.subtle.digest('SHA-256', await file.arrayBuffer())
  return Array.from(new Uint8Array(digest), (byte) => byte.toString(16).padStart(2, '0')).join('')
}

export type SavedRemixPlan = {
  id: string
  organization_id: string
  project_id: string
  schema_version: 'remix_plan_v1' | 'remix_plan_v2'
  client_plan_id: string
  target_seconds: number
  actual_seconds: number
  pace: BulkRemixPlan['pace']
  segments: Array<{
    segment: string
    label: string
    target_seconds: number
    actual_seconds: number
    shots: Array<{
      id: string
      segment: string
      source: 'existing_asset'
      asset_version: { asset_id: string; version: number }
      timeline: {
        start_seconds: number
        duration_seconds: number
        in_point_seconds: number
        out_point_seconds: number
      }
      creative: {
        scene: string
        shot_type: string
        camera_angle: string
        dialogue_or_narration: string
        subtitle: string
        transition: string
        cta_element: string
      }
      planning: {
        score: number
        reason_codes: string[]
        reason: string
        evidence: string[]
      }
      risks: string[]
    }>
    clips: Array<{
      id: string
      segment: string
      asset_version: { asset_id: string; version: number }
      label: string
      source_type: string
      mime_type: string
      aspect: string
      start_seconds: number
      duration_seconds: number
      in_point_seconds: number
      out_point_seconds: number
      score: number
      reason: string
    }>
  }>
  warnings: string[]
  summary: {
    selected_assets: number
    used_assets: number
    coverage_percent: number
    strategy: string
  }
  created_at: string
  updated_at: string
}

export type RemixRenderJob = {
  id: string
  organization_id: string
  project_id: string
  plan_id: string
  status: 'queued' | 'running' | 'requires_review' | 'succeeded' | 'failed'
  progress: number
  target_format: 'mp4'
  target_quality: 'draft' | 'standard' | 'high'
  requires_review: boolean
  quality_report_id?: string
  output_asset?: { project_id: string; asset_version: { asset_id: string; version: number } }
  error_code?: string
  error_message?: string
  created_at: string
  updated_at: string
}

export type QualityReport = {
  id: string
  organization_id: string
  project_id: string
  render_job_id: string
  output_asset?: { project_id: string; asset_version: { asset_id: string; version: number } }
  verdict: 'pass' | 'major' | 'critical'
  score: number
  dimensions: Array<{ name: string; score: number; verdict: 'pass' | 'major' | 'critical'; summary: string }>
  issues: Array<{
    code: string
    severity: 'major' | 'critical'
    dimension: string
    start_seconds: number
    end_seconds: number
    description: string
    repair_suggestion: string
  }>
  evidence: Array<{ kind: string; timestamp_sec: number; summary: string }>
  repair_suggestions: string[]
  created_at: string
  updated_at: string
}

export type RemixPreroll = {
  id: string
  plan_id: string
  hook_type: string
  reference_asset: { asset_id: string; version: number }
  style_constraints: string[]
  duration_seconds: number
  mode: 'prompt_only' | 'generate_video'
  prompt_draft: string
  quality_verdict: 'pass' | 'major' | 'critical'
  status: 'draft' | 'ready' | 'failed' | 'applied'
  error_code?: string
  error_message?: string
}

export type AgentRun = {
  id: string
  workflow: 'render_diagnosis'
  status: 'queued' | 'running' | 'succeeded' | 'failed' | 'cancelled'
  target: { render_job_id: string }
  steps: Array<{ id: string; label: string; status: string; summary: string }>
  tool_calls: Array<{ id: string; name: string; status: string; error_message?: string; references?: Array<{ type: string; id: string }> }>
  trace_spans: Array<{ id: string; parent_id?: string; name: string; kind: 'agent' | 'tool' | 'model'; status: string; model?: string; error_message?: string; input_tokens?: number; output_tokens?: number }>
  output?: Record<string, unknown>
  error_message?: string
}

export type KnowledgeCitation = {
  document_id: string
  chunk_id: string
  title: string
  source_uri: string
  section: string
  start_line: number
  end_line: number
  snippet: string
}

export type EvalRun = {
  id: string
  status: 'succeeded'
  planner_version: string
  prompt_version: string
  score: number
  total_cases: number
  passed_cases: number
  failed_cases: string[]
  results: Array<{ id: string; case_id: string; case_type: string; score: number; passed: boolean; expected: string; actual: string; reason: string }>
}

export type FeedbackEvent = {
  id: string
  event_type: string
  target_type: string
  target_id: string
  rating?: number
  comment?: string
  created_at: string
}

export function createRemixPlan(projectId: string, plan: BulkRemixPlan, signal?: AbortSignal) {
  return apiRequest<SavedRemixPlan>(`/platform/v1/projects/${encodeURIComponent(projectId)}/remix-plans`, {
    method: 'POST',
    signal,
    body: JSON.stringify({
      schema_version: plan.schemaVersion,
      client_plan_id: plan.id,
      target_seconds: plan.targetSeconds,
      actual_seconds: plan.actualSeconds,
      pace: plan.pace,
      segments: plan.segments.map((segment) => ({
        segment: segment.segment,
        label: segment.label,
        target_seconds: segment.targetSeconds,
        actual_seconds: segment.actualSeconds,
        shots: segment.shots.map((shot) => ({
          id: shot.id,
          segment: shot.segment,
          source: shot.source,
          asset_version: shot.assetVersion,
          timeline: {
            start_seconds: shot.timeline.startSeconds,
            duration_seconds: shot.timeline.durationSeconds,
            in_point_seconds: shot.timeline.inPointSeconds,
            out_point_seconds: shot.timeline.outPointSeconds,
          },
          creative: {
            scene: shot.creative.scene,
            shot_type: shot.creative.shotType,
            camera_angle: shot.creative.cameraAngle,
            dialogue_or_narration: shot.creative.dialogueOrNarration,
            subtitle: shot.creative.subtitle,
            transition: shot.creative.transition,
            cta_element: shot.creative.ctaElement,
          },
          planning: {
            score: shot.planning.score,
            reason_codes: shot.planning.reasonCodes,
            reason: shot.planning.reason,
            evidence: shot.planning.evidence,
          },
          risks: shot.risks,
        })),
        clips: segment.clips.map((clip) => ({
          id: clip.id,
          segment: clip.segment,
          asset_version: { asset_id: clip.assetId, version: clip.version },
          label: clip.label,
          source_type: clip.sourceType,
          mime_type: clip.mimeType,
          aspect: clip.aspect,
          start_seconds: clip.startSeconds,
          duration_seconds: clip.durationSeconds,
          in_point_seconds: clip.inPointSeconds,
          out_point_seconds: clip.outPointSeconds,
          score: clip.score,
          reason: clip.reason,
        })),
      })),
      warnings: plan.warnings,
      summary: {
        selected_assets: plan.summary.selectedAssets,
        used_assets: plan.summary.usedAssets,
        coverage_percent: plan.summary.coveragePercent,
        strategy: plan.summary.strategy,
      },
    }),
  })
}

export function getRemixPlan(projectId: string, planId: string, signal?: AbortSignal) {
  return apiRequest<SavedRemixPlan>(`/platform/v1/projects/${encodeURIComponent(projectId)}/remix-plans/${encodeURIComponent(planId)}`, { signal })
}

export function listRemixPlans(projectId: string, limit = 20, signal?: AbortSignal) {
  return apiRequest<{ items: SavedRemixPlan[] }>(`/platform/v1/projects/${encodeURIComponent(projectId)}/remix-plans?limit=${limit}`, { signal })
}

export function createRemixRenderJob(projectId: string, planId: string, targetQuality: RemixRenderJob['target_quality'] = 'standard', signal?: AbortSignal) {
  return apiRequest<RemixRenderJob>(`/platform/v1/projects/${encodeURIComponent(projectId)}/remix-render-jobs`, {
    method: 'POST',
    signal,
    body: JSON.stringify({
      plan_id: planId,
      target_format: 'mp4',
      target_quality: targetQuality,
    }),
  })
}

export function getRemixRenderJob(projectId: string, jobId: string, signal?: AbortSignal) {
  return apiRequest<RemixRenderJob>(`/platform/v1/projects/${encodeURIComponent(projectId)}/remix-render-jobs/${encodeURIComponent(jobId)}`, { signal })
}

export function createQualityReport(projectId: string, renderJobId: string, signal?: AbortSignal) {
  return apiRequest<QualityReport>(`/platform/v1/projects/${encodeURIComponent(projectId)}/remix-quality-reports`, {
    method: 'POST',
    signal,
    body: JSON.stringify({
      render_job_id: renderJobId,
      policy: 'fail_critical',
    }),
  })
}

export function getRenderJobQualityReport(projectId: string, jobId: string, signal?: AbortSignal) {
  return apiRequest<{ quality_report: QualityReport | null }>(`/platform/v1/projects/${encodeURIComponent(projectId)}/remix-render-jobs/${encodeURIComponent(jobId)}/quality-report`, { signal })
}

export function createRemixPreroll(projectId: string, planId: string, referenceAsset: { asset_id: string; version: number }, styleConstraints: string[], signal?: AbortSignal) {
  return apiRequest<RemixPreroll>(`/platform/v1/projects/${encodeURIComponent(projectId)}/remix-prerolls`, {
    method: 'POST',
    signal,
    body: JSON.stringify({
      plan_id: planId,
      hook_type: 'conflict',
      reference_asset: referenceAsset,
      style_constraints: styleConstraints,
      duration_seconds: 4,
      mode: 'generate_video',
    }),
  })
}

export function applyRemixPreroll(projectId: string, prerollId: string, signal?: AbortSignal) {
  return apiRequest<SavedRemixPlan>(`/platform/v1/projects/${encodeURIComponent(projectId)}/remix-prerolls/${encodeURIComponent(prerollId)}/apply`, {
    method: 'POST',
    signal,
  })
}

export function createAgentRun(projectId: string, renderJobId: string, signal?: AbortSignal) {
  return apiRequest<AgentRun>(`/platform/v1/projects/${encodeURIComponent(projectId)}/agent-runs`, {
    method: 'POST',
    signal,
    body: JSON.stringify({
      workflow: 'render_diagnosis',
      target: { render_job_id: renderJobId },
    }),
  })
}

export function searchKnowledge(projectId: string, query: string, signal?: AbortSignal) {
  return apiRequest<{ items: Array<{ citations: KnowledgeCitation[] }> }>(`/platform/v1/projects/${encodeURIComponent(projectId)}/knowledge/search?q=${encodeURIComponent(query)}&limit=3`, { signal })
}

export function createEvalRun(projectId: string, signal?: AbortSignal) {
  return apiRequest<EvalRun>(`/platform/v1/projects/${encodeURIComponent(projectId)}/remix-eval-runs`, {
    method: 'POST',
    signal,
    body: JSON.stringify({
      planner_version: 'planner-v1',
      prompt_version: 'prompt-v1',
      submissions: [
        { case_id: 'remix_mmlu_hook_mcq_v1', choice_id: 'a' },
        { case_id: 'remix_mmlu_rubric_v1', answer_text: 'authorized timeline risk' },
      ],
    }),
  })
}

export function createFeedbackEvent(projectId: string, targetId: string, rating: number, comment: string, assetVersion?: { asset_id: string; version: number }, signal?: AbortSignal) {
  return apiRequest<FeedbackEvent>(`/platform/v1/projects/${encodeURIComponent(projectId)}/remix-feedback-events`, {
    method: 'POST',
    signal,
    body: JSON.stringify({
      event_type: 'rating',
      target_type: assetVersion ? 'asset' : 'remix_plan',
      target_id: targetId,
      asset_version: assetVersion,
      rating,
      comment,
    }),
  })
}
