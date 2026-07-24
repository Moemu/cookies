export type CreativeArtifactStatus = '草稿' | '待确认' | '排队中' | '制作中' | '已完成' | '生成失败' | '已取消'

export interface MediaArtifact {
  kind: string
  status: 'draft' | 'ready' | 'archived'
  content: string
  sourceJobId?: string
  updatedAt: string
}

export interface MediaGenerationJob {
  artifactKind: string
  status: 'queued' | 'running' | 'succeeded' | 'failed' | 'cancelled'
  model?: string
  diagnostic?: string
  updatedAt: string
}

export interface CreativeStatusPresentation {
  status: CreativeArtifactStatus
  summary: string
  owner: string
  sourceVersion?: string
  updatedAt: string
}

const jobStatusPresentation: Record<MediaGenerationJob['status'], CreativeArtifactStatus> = {
  queued: '排队中',
  running: '制作中',
  succeeded: '已完成',
  failed: '生成失败',
  cancelled: '已取消',
}

export function presentCreativeStatus(
  artifact: MediaArtifact | undefined,
  job: MediaGenerationJob | undefined,
  formatDate: (value: string) => string,
): CreativeStatusPresentation {
  if (job) {
    const mediaLabel = job.artifactKind === 'video' ? '视频' : '图片'
    const generatedAt = formatDate(job.updatedAt)
    const model = job.model ?? '服务端已存档'
    const sourceVersion = `AI 生成 · 模型 ${model} · 生成于 ${generatedAt}`
    const status = jobStatusPresentation[job.status]

    return {
      status,
      summary: job.status === 'succeeded'
        ? artifact?.content ?? `${mediaLabel}资产已保存，可用于后续投放模拟。`
        : job.status === 'failed'
          ? `${mediaLabel}生成失败：${job.diagnostic ?? '请重试任务。'}`
          : job.status === 'cancelled'
            ? `${mediaLabel}生成任务已取消。`
            : `${mediaLabel}生成任务${job.status === 'queued' ? '正在排队' : '正在运行'}。`,
      owner: 'AI 生成 · 服务端资产',
      sourceVersion,
      updatedAt: generatedAt,
    }
  }

  if (artifact?.status === 'ready') {
    return {
      status: '已完成',
      summary: artifact.content || `${artifact.kind === 'video' ? '视频' : '图片'}资产已保存，可用于后续投放模拟。`,
      owner: artifact.sourceJobId ? 'AI 生成 · 服务端资产' : '服务端资产',
      sourceVersion: artifact.sourceJobId ? `AI 生成任务 ${artifact.sourceJobId.slice(0, 8)} · 生成于 ${formatDate(artifact.updatedAt)}` : undefined,
      updatedAt: formatDate(artifact.updatedAt),
    }
  }

  return {
    status: artifact ? '草稿' : '待确认',
    summary: artifact?.content ?? '尚无服务端产物。',
    owner: '服务端存档',
    updatedAt: formatDate(artifact?.updatedAt ?? ''),
  }
}
