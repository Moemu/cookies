import { useEffect, useRef } from 'react'
import { strategyApi } from './api'
import { readSSE } from './readSSE'
import type { ActivityConnection, TaskActivitySnapshot } from './types'

type ActivityStreamOptions = {
  projectId: string
  workspaceId?: string
  cursorScope?: string
  onSnapshot: (snapshot: TaskActivitySnapshot) => void
  onConnection: (connection: ActivityConnection, message?: string) => void
}

export function useActivityStream({
  projectId,
  workspaceId = '',
  cursorScope = '',
  onSnapshot,
  onConnection,
}: ActivityStreamOptions) {
  const snapshotCallback = useRef(onSnapshot)
  const connectionCallback = useRef(onConnection)
  useEffect(() => {
    snapshotCallback.current = onSnapshot
  }, [onSnapshot])
  useEffect(() => {
    connectionCallback.current = onConnection
  }, [onConnection])

  useEffect(() => {
    if (!projectId) return
    const controller = new AbortController()
    const storageKey = activityCursorKey(cursorScope, projectId, workspaceId)
    let lastSnapshotId = readActivityCursor(storageKey)
    let appliedSnapshotId = ''
    let retryTimer = 0
    let retryAttempt = 0
    let hasConnected = false
    let connection: ActivityConnection | '' = ''

    const emitConnection = (next: ActivityConnection, message = '') => {
      if (controller.signal.aborted || (connection === next && !message)) return
      connection = next
      connectionCallback.current(next, message)
    }
    const applySnapshot = (snapshot: TaskActivitySnapshot) => {
      if (snapshot.snapshot_id === appliedSnapshotId) return
      appliedSnapshotId = snapshot.snapshot_id
      lastSnapshotId = snapshot.snapshot_id
      writeActivityCursor(storageKey, lastSnapshotId)
      snapshotCallback.current(snapshot)
    }
    const reconcile = async () => {
      const snapshot = await strategyApi.listActivities(projectId, workspaceId, controller.signal)
      applySnapshot(snapshot)
    }
    const recoverAndSchedule = async (reason: string) => {
      if (controller.signal.aborted) return
      try {
        await reconcile()
        emitConnection('reconnecting', reason)
      } catch {
        emitConnection('offline', '活动状态暂时无法连接，系统会自动重试。')
      }
      const delay = activityReconnectDelay(retryAttempt, Math.random())
      retryAttempt += 1
      retryTimer = window.setTimeout(connect, delay)
    }
    const connect = async () => {
      if (controller.signal.aborted) return
      emitConnection(hasConnected ? 'reconnecting' : 'connecting')
      if (!hasConnected) {
        try {
          await reconcile()
        } catch {
          // The stream may still be reachable; connection state is decided below.
        }
      }
      try {
        const query = new URLSearchParams({ limit: '50' })
        if (workspaceId) query.set('workspace_id', workspaceId)
        const response = await fetch(
          `/api/strategy/v1/projects/${encodeURIComponent(projectId)}/activities/events?${query.toString()}`,
          {
            headers: {
              Accept: 'text/event-stream',
              ...(lastSnapshotId ? { 'Last-Event-ID': lastSnapshotId } : {}),
            },
            signal: controller.signal,
          },
        )
        if (!response.ok) throw new Error(`Activity stream failed (${response.status})`)
        hasConnected = true
        retryAttempt = 0
        emitConnection('live')
        await readSSE(response, message => {
          if (message.event !== 'activity.snapshot') return
          const snapshot = parseActivitySnapshot(message.data)
          applySnapshot(snapshot)
        }, controller.signal)
        if (!controller.signal.aborted) {
          await recoverAndSchedule('活动连接已断开，正在核对最新状态。')
        }
      } catch {
        if (!controller.signal.aborted) {
          await recoverAndSchedule('活动连接异常，正在核对最新状态。')
        }
      }
    }

    void connect()
    return () => {
      controller.abort()
      window.clearTimeout(retryTimer)
    }
  }, [cursorScope, projectId, workspaceId])
}

export function parseActivitySnapshot(data: string): TaskActivitySnapshot {
  const value = JSON.parse(data) as Partial<TaskActivitySnapshot>
  if (value.contract_version !== 'strategy-task-activity-snapshot/v1' ||
    typeof value.snapshot_id !== 'string' || !value.snapshot_id.startsWith('sha256:') ||
    typeof value.captured_at !== 'string' || !Array.isArray(value.items)) {
    throw new Error('Activity snapshot does not satisfy strategy-task-activity-snapshot/v1')
  }
  return value as TaskActivitySnapshot
}

export function activityReconnectDelay(attempt: number, jitter: number) {
  const boundedAttempt = Math.max(0, Math.min(attempt, 6))
  const boundedJitter = Math.max(0, Math.min(jitter, 1))
  return Math.min(5_000, 500 * (2 ** boundedAttempt) + Math.round(boundedJitter * 150))
}

export function activityCursorKey(cursorScope: string, projectId: string, workspaceId: string) {
  const scope = cursorScope || `anonymous:${projectId}`
  return `strategy:activity-snapshot:${encodeURIComponent(scope)}:${encodeURIComponent(workspaceId || 'project')}`
}

function readActivityCursor(key: string) {
  try {
    return window.sessionStorage.getItem(key) ?? ''
  } catch {
    return ''
  }
}

function writeActivityCursor(key: string, value: string) {
  try {
    window.sessionStorage.setItem(key, value)
  } catch {
    // A cursor is only a reconnect optimization; snapshots remain rebuildable.
  }
}
