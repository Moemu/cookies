import { CreativeApiError } from '../../data/api'

export async function runWithLatestCreativeRevision<T>(
  initialRevision: number,
  mutate: (expectedRevision: number) => Promise<T>,
  refreshRevision: () => Promise<number>,
): Promise<T> {
  try {
    return await mutate(initialRevision)
  } catch (cause) {
    if (!(cause instanceof CreativeApiError) || cause.status !== 412) throw cause
    const latestRevision = await refreshRevision()
    return mutate(latestRevision)
  }
}
