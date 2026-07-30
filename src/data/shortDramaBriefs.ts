export type LocalShortDramaBrief = {
  id: string
  version: number
  name: string
  objective: string
  applicableGenres: string[]
  reviewedSellingPoints: string[]
  callToAction: string
  prohibited: string[]
  recommendedHookStrategy: 'conflict_reversal' | 'suspense_reveal' | 'identity_contrast' | 'selling_point_bridge'
}

export const localShortDramaBriefs: LocalShortDramaBrief[] = [
  {
    id: 'brief_local_urban_reversal_v1',
    version: 1,
    name: '都市逆袭 · 身份反转拉片',
    objective: '引导观看正片',
    applicableGenres: ['都市情感', '逆袭', '职场'],
    reviewedSellingPoints: ['被轻视', '身份反转'],
    callToAction: '点击看她如何反转局面',
    prohibited: ['不得虚构人物身份', '不得夸大剧情', '不得泄露完整结局'],
    recommendedHookStrategy: 'conflict_reversal',
  },
  {
    id: 'brief_local_suspense_truth_v1',
    version: 1,
    name: '悬疑真相 · 信息缺口拉片',
    objective: '用未解事实引导观看正片',
    applicableGenres: ['悬疑', '复仇', '家庭秘密'],
    reviewedSellingPoints: ['关键线索出现', '真相尚未揭开'],
    callToAction: '点击正片揭开真相',
    prohibited: ['不得新增关键证据', '不得提前公布真凶', '不得改写人物关系'],
    recommendedHookStrategy: 'suspense_reveal',
  },
  {
    id: 'brief_local_identity_contrast_v1',
    version: 1,
    name: '身份反差 · 认知错位拉片',
    objective: '利用人物认知反差引导观看正片',
    applicableGenres: ['豪门', '重生', '职场逆袭'],
    reviewedSellingPoints: ['身份被误判', '真实能力待揭示'],
    callToAction: '点击看所有人如何改口',
    prohibited: ['不得虚构新身份', '不得改变人物设定', '不得使用未授权人物素材'],
    recommendedHookStrategy: 'identity_contrast',
  },
]

export function findLocalShortDramaBrief(id: string): LocalShortDramaBrief {
  return localShortDramaBriefs.find(brief => brief.id === id) ?? localShortDramaBriefs[0]
}

export function shortDramaVideoLabel(
  video: { sourceType?: string; durationSeconds?: number; width?: number; height?: number },
  index: number,
): string {
  const source = video.sourceType === 'provider_generated'
    ? 'AI 生成正片'
    : video.sourceType === 'rendered'
      ? '剪辑成片'
      : '项目正片'
  const sequence = String(index + 1).padStart(2, '0')
  const duration = video.durationSeconds && video.durationSeconds > 0
    ? `${video.durationSeconds.toFixed(0)} 秒`
    : '时长待识别'
  const dimensions = video.width && video.height ? ` · ${video.width}×${video.height}` : ''
  return `${source} ${sequence} · ${duration}${dimensions}`
}
