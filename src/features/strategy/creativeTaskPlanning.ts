import type { BriefVersion, PackageVersion, StrategyDocument, StrategyDraft } from './types'

export function findPublishedPackageForDraft(
  packages: PackageVersion[],
  briefVersion: Pick<BriefVersion, 'brief_id' | 'version'>,
  draft: Pick<StrategyDraft, 'id' | 'current_revision'> | null,
) {
  if (!draft) return null
  return packages
    .filter(item => item.status === 'published' &&
      item.snapshot.strategy_id === draft.id &&
      item.snapshot.strategy_revision === draft.current_revision &&
      item.snapshot.brief?.brief_id === briefVersion.brief_id &&
      item.snapshot.brief?.version === briefVersion.version)
    .sort((left, right) => right.version - left.version)[0] ?? null
}

export function createRouteRevisionChannelStrategy(
  channels: StrategyDocument['channel_strategy'],
  businessCode = 'xiaohongshu_image_text',
) {
  let changed = false
  const value = channels.map(channel => {
    const platform = channel.platform.toLowerCase()
    if (businessCode === 'brand_video') {
      if (!['xiaohongshu', 'douyin', 'kuaishou', 'wechat_official_account'].includes(platform)) return channel
      const formats = channel.formats.some(format => /视频|短片|video|tvc/i.test(format))
        ? channel.formats
        : [...channel.formats, '品牌短视频']
      const role = /brand|awareness|品牌|认知|种草|心智/i.test(channel.role)
        ? channel.role
        : `${channel.role}；品牌认知`
      if (formats !== channel.formats || role !== channel.role) changed = true
      return { ...channel, role, formats }
    }
    if (platform !== 'xiaohongshu' ||
      channel.formats.some(format => /图文|image[_ -]?text/i.test(format))) return channel
    changed = true
    return { ...channel, formats: [...channel.formats, '小红书图文笔记'] }
  })
  return { changed, value }
}
