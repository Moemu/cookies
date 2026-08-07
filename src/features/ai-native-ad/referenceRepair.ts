import type { StoryboardAsset } from './types'

export type ReferenceRepairSuggestion = {
  reason: string
  recommendedFeedback: string
  alternatives: string[]
}

export function referenceRepairSuggestion(
  errorCode: string,
  errorMessage: string,
  asset: Pick<StoryboardAsset, 'name' | 'role'>,
): ReferenceRepairSuggestion | null {
  const privacyRejected = errorCode.startsWith('InputImageSensitiveContentDetected') || /input image.+real person/i.test(errorMessage)
  const copyrightRejected = /copyright restrictions|copyright|著作权|版权/i.test(errorMessage)
  if (!privacyRejected && !copyrightRejected) return null

  if (privacyRejected) {
    return {
      reason: '参考图片可能包含可识别的真实人物或清晰面部，视频模型因隐私保护拒绝使用。',
      recommendedFeedback: `保留“${asset.name}”的${asset.role === 'scene_reference' ? '场景功能和商品展示目的' : '构图与广告用途'}；人物改为虚构成年人物的背影、侧后方或手部，不出现清晰正脸，镜面与玻璃中不出现可识别面部倒影。`,
      alternatives: ['改为无人场景，仅保留商品与环境', '使用手部或背影完成动作，不展示面部'],
    }
  }

  return {
    reason: '参考图片可能包含受版权保护的角色、品牌标识或受限视觉元素，视频模型拒绝使用。',
    recommendedFeedback: `保留“${asset.name}”的场景与商品展示目的；移除受版权保护的角色、品牌标识、包装文字和水印，改用原创通用环境与无标识道具。`,
    alternatives: ['改用无品牌的通用场景', '仅保留商品主体，背景改为原创纯净空间'],
  }
}
