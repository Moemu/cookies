import { createServer } from 'node:http'

let mode: 'success' | 'create_failure' | 'task_failure' = 'success'
let sequence = 0
const tasks = new Map<string, 'succeeded' | 'failed'>()

createServer(async (request, response) => {
  const url = new URL(request.url ?? '/', 'http://127.0.0.1:8791')
  if (url.pathname === '/test/mode' && request.method === 'GET') {
    response.writeHead(200).end()
    return
  }
  if (url.pathname === '/test/mode' && request.method === 'POST') {
    let raw = ''
    for await (const chunk of request) raw += chunk
    const next = JSON.parse(raw) as { mode?: typeof mode }
    mode = next.mode ?? 'success'
    response.writeHead(204).end()
    return
  }
  if (url.pathname === '/api/v3/contents/generations/tasks' && request.method === 'POST') {
    if (mode === 'create_failure') {
      response.writeHead(503, { 'Content-Type': 'application/json' }).end(JSON.stringify({ error: { message: 'provider rejected request' } }))
      return
    }
    const id = `e2e-task-${++sequence}`
    tasks.set(id, mode === 'task_failure' ? 'failed' : 'succeeded')
    response.writeHead(200, { 'Content-Type': 'application/json' }).end(JSON.stringify({ id }))
    return
  }
  const taskId = url.pathname.match(/^\/api\/v3\/contents\/generations\/tasks\/([^/]+)$/)?.[1]
  if (taskId && request.method === 'GET') {
    const status = tasks.get(taskId) ?? 'failed'
    response.writeHead(200, { 'Content-Type': 'application/json' }).end(JSON.stringify(
      status === 'succeeded'
        ? { status, url: `https://assets.test/${taskId}.mp4` }
        : { status, error: { message: 'provider task failed' } },
    ))
    return
  }
  if (url.pathname.match(/^\/api\/v3\/contents\/generations\/tasks\/[^/]+\/cancel$/) && request.method === 'POST') {
    response.writeHead(200, { 'Content-Type': 'application/json' }).end(JSON.stringify({ status: 'cancelled' }))
    return
  }
  response.writeHead(404).end()
}).listen(8791, '127.0.0.1')
