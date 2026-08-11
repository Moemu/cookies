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
) {
  let changed = false
  const value = channels.map(channel => {
    if (channel.platform.toLowerCase() !== 'xiaohongshu' ||
      channel.formats.some(format => /图文|image[_ -]?text/i.test(format))) return channel
    changed = true
    return { ...channel, formats: [...channel.formats, '小红书图文笔记'] }
  })
  return { changed, value }
}
