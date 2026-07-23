export type Problem = {
  error: {
    code: string
    message: string
    request_id: string
    retryable: boolean
    details: Array<{ field: string; reason: string }>
  }
}

export class ApiProblem extends Error {
  constructor(readonly problem: Problem) {
    super(problem.error.message)
  }
}

export async function apiRequest<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers)
  if (!headers.has('Accept')) headers.set('Accept', 'application/json')
  if (typeof init.body === 'string' && !headers.has('Content-Type')) headers.set('Content-Type', 'application/json')

  const response = await fetch(path, { ...init, headers })
  if (!response.ok) {
    const problem = await response.json().catch(() => null) as Problem | null
    if (problem?.error) throw new ApiProblem(problem)
    throw new Error(`Request failed with status ${response.status}`)
  }
  if (response.status === 204) return undefined as T
  return response.json() as Promise<T>
}
