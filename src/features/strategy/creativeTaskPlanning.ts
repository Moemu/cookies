import type { BriefVersion, PackageVersion, StrategyDraft } from './types'

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
