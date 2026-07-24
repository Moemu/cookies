import type { FullConfig } from '@playwright/test'
import { rm } from 'node:fs/promises'

export default async function globalTeardown(config: FullConfig) {
  const dataDirectory = config.metadata.e2eDataDirectory
  if (typeof dataDirectory === 'string') await rm(dataDirectory, { recursive: true, force: true })
}
