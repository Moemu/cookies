import { useEffect, useState } from 'react'
import { strategyApi } from '../api'
import type { ProjectContextManifest } from '../types'

export function useProjectContextManifest(
  workspaceId: string,
  stage: ProjectContextManifest['stage'],
  revisionKey: string,
) {
  const [manifest, setManifest] = useState<ProjectContextManifest | null>(null)
  const [error, setError] = useState('')

  useEffect(() => {
    if (!workspaceId) {
      setManifest(null)
      setError('')
      return
    }
    const controller = new AbortController()
    setError('')
    void strategyApi.getWorkspaceContextManifest(workspaceId, stage, controller.signal)
      .then(setManifest)
      .catch(value => {
        if (!(value instanceof DOMException && value.name === 'AbortError')) {
          setError(value instanceof Error ? value.message : '当前项目上下文暂时无法读取。')
        }
      })
    return () => controller.abort()
  }, [revisionKey, stage, workspaceId])

  return { manifest, error }
}
