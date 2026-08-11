import { useEffect, useRef } from 'react'
import { readSSE } from './readSSE'

export function useConversationStream(conversationId: string | undefined, cursorScope: string, onEvent: () => void) {
  const callback = useRef(onEvent)
  useEffect(() => {
    callback.current = onEvent
  }, [onEvent])

  useEffect(() => {
    if (!conversationId) return
    const controller = new AbortController()
    const cursorKey = conversationEventCursorKey(cursorScope, conversationId)
    let lastEventId = sessionStorage.getItem(cursorKey) || ''
    let retryTimer = 0
    let invalidateTimer = 0
    let retryAttempt = 0

    const retry = () => {
      if (!controller.signal.aborted) {
        const delay = Math.min(10_000, 800 * (2 ** retryAttempt)) + Math.round(Math.random() * 250)
        retryAttempt += 1
        retryTimer = window.setTimeout(connect, delay)
      }
    }
    const invalidate = () => {
      window.clearTimeout(invalidateTimer)
      invalidateTimer = window.setTimeout(() => callback.current(), 180)
    }
    const connect = async () => {
      try {
        const response = await fetch(
          `/api/strategy/v1/conversations/${encodeURIComponent(conversationId)}/events`,
          {
            headers: lastEventId ? { 'Last-Event-ID': lastEventId } : {},
            signal: controller.signal,
          },
        )
        if (response.status === 410) {
          lastEventId = ''
          sessionStorage.removeItem(cursorKey)
          invalidate()
          retry()
          return
        }
        if (!response.ok) throw new Error(`Strategy event stream failed (${response.status})`)
        retryAttempt = 0
        await readSSE(response, message => {
          if (message.id) {
            if (!shouldAdvanceConversationEventCursor(lastEventId, message.id)) return
            lastEventId = message.id
            sessionStorage.setItem(cursorKey, lastEventId)
          }
          invalidate()
        }, controller.signal)
        retry()
      } catch {
        retry()
      }
    }

    void connect()
    return () => {
      controller.abort()
      window.clearTimeout(retryTimer)
      window.clearTimeout(invalidateTimer)
    }
  }, [conversationId, cursorScope])
}

export function conversationEventCursorKey(cursorScope: string, conversationId: string) {
  return `strategy:last-event:${encodeURIComponent(cursorScope || 'anonymous')}:${encodeURIComponent(conversationId)}`
}

export function shouldAdvanceConversationEventCursor(current: string, candidate: string) {
  if (!/^[1-9]\d*$/.test(candidate)) return false
  if (!/^[1-9]\d*$/.test(current)) return true
  return BigInt(candidate) > BigInt(current)
}
