import { describe, expect, it } from 'vitest'
import { parseSSEBlock, readSSE } from './readSSE'

describe('Strategy SSE parser', () => {
  it('parses multiline data and ignores heartbeat comments', () => {
    expect(parseSSEBlock(': heartbeat')).toBeNull()
    expect(parseSSEBlock('id: 42\nevent: brief.updated\ndata: {"brief":\ndata: 8}')).toEqual({
      id: '42',
      event: 'brief.updated',
      data: '{"brief":\n8}',
    })
  })

  it('reads events split across network chunks', async () => {
    const encoder = new TextEncoder()
    const response = new Response(new ReadableStream({
      start(controller) {
        controller.enqueue(encoder.encode('id: 1\nevent: message.'))
        controller.enqueue(encoder.encode('created\ndata: {"id":"msg_1"}\n\n'))
        controller.close()
      },
    }))
    const events: string[] = []
    await readSSE(response, (event) => events.push(event.event))
    expect(events).toEqual(['message.created'])
  })
})
