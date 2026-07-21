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
  const response = await fetch(path, { ...init, headers: { Accept: 'application/json', ...init.headers } })
  if (!response.ok) {
    throw new ApiProblem(await response.json() as Problem)
  }
  return response.json() as Promise<T>
}
