import { useEffect, useRef } from 'react'
import { readSSE } from './readSSE'

export function useConversationStream(conversationId: string | undefined, onEvent: () => void) {
  const callback = useRef(onEvent)
  useEffect(() => {
    callback.current = onEvent
  }, [onEvent])

  useEffect(() => {
    if (!conversationId) return
    const controller = new AbortController()
    let lastEventId = sessionStorage.getItem(`strategy:last-event:${conversationId}`) || ''
    let retryTimer = 0

    const retry = () => {
      if (!controller.signal.aborted) retryTimer = window.setTimeout(connect, 1200)
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
          sessionStorage.removeItem(`strategy:last-event:${conversationId}`)
          callback.current()
          retry()
          return
        }
        await readSSE(response, message => {
          if (message.id) {
            lastEventId = message.id
            sessionStorage.setItem(`strategy:last-event:${conversationId}`, lastEventId)
          }
          callback.current()
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
    }
  }, [conversationId])
}
