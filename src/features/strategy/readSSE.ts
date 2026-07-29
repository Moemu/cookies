export type SSEMessage = {
  id: string
  event: string
  data: string
}

export function parseSSEBlock(block: string): SSEMessage | null {
  const message: SSEMessage = { id: '', event: 'message', data: '' }
  const data: string[] = []
  for (const line of block.replaceAll('\r\n', '\n').split('\n')) {
    if (!line || line.startsWith(':')) continue
    const separator = line.indexOf(':')
    const field = separator < 0 ? line : line.slice(0, separator)
    const value = separator < 0 ? '' : line.slice(separator + 1).replace(/^ /, '')
    if (field === 'id' && !value.includes('\0')) message.id = value
    if (field === 'event') message.event = value || 'message'
    if (field === 'data') data.push(value)
  }
  if (data.length === 0) return null
  message.data = data.join('\n')
  return message
}

export async function readSSE(
  response: Response,
  onMessage: (message: SSEMessage) => void,
  signal?: AbortSignal,
) {
  if (!response.ok || !response.body) throw new Error(`SSE request failed (${response.status})`)
  const reader = response.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''
  while (!signal?.aborted) {
    const { done, value } = await reader.read()
    buffer += decoder.decode(value, { stream: !done }).replaceAll('\r\n', '\n')
    let boundary = buffer.indexOf('\n\n')
    while (boundary >= 0) {
      const message = parseSSEBlock(buffer.slice(0, boundary))
      buffer = buffer.slice(boundary + 2)
      if (message) onMessage(message)
      boundary = buffer.indexOf('\n\n')
    }
    if (done) break
  }
}
