import type {
  ComputerUseEvidence,
  ComputerUseRun,
  ComputerUseRunEvent,
  ControlledExecutionWorkspace,
} from './model'

export class ControlledExecutionApiError extends Error {
  constructor(readonly status: number, message: string) {
    super(message)
    this.name = 'ControlledExecutionApiError'
  }
}

const apiPrefix = '/api/platform/v1/computer-use/projects'

/**
 * Thin platform-control-plane client. It intentionally does not import legacy
 * Delivery execute_mock models, and it never exposes a remote-submit method.
 */
export const controlledExecutionApi = {
  async getWorkspace(projectId: string, runId: string, signal?: AbortSignal): Promise<ControlledExecutionWorkspace> {
    const path = `${apiPrefix}/${encodeURIComponent(projectId)}/runs/${encodeURIComponent(runId)}`
    const [run, events, evidence] = await Promise.all([
      request<ComputerUseRun>(path, { signal }),
      request<{ items?: ComputerUseRunEvent[] }>(`${path}/events`, { signal }),
      request<{ items?: ComputerUseEvidence[] }>(`${path}/evidence`, { signal }),
    ])
    return { run, events: events.items ?? [], evidence: evidence.items ?? [] }
  },

  pause(projectId: string, runId: string, expectedVersion: number) {
    return control(projectId, runId, expectedVersion, 'pause')
  },
  resume(projectId: string, runId: string, expectedVersion: number) {
    return control(projectId, runId, expectedVersion, 'resume')
  },
  cancel(projectId: string, runId: string, expectedVersion: number) {
    return control(projectId, runId, expectedVersion, 'cancel')
  },
  takeOver(projectId: string, runId: string, expectedVersion: number) {
    return control(projectId, runId, expectedVersion, 'takeover')
  },
  releaseTakeover(projectId: string, runId: string, expectedVersion: number) {
    return control(projectId, runId, expectedVersion, 'release_takeover')
  },
}

function control(projectId: string, runId: string, expectedVersion: number, action: 'pause' | 'resume' | 'cancel' | 'takeover' | 'release_takeover') {
  return request<ComputerUseRun>(`${apiPrefix}/${encodeURIComponent(projectId)}/runs/${encodeURIComponent(runId)}:${action}`, {
    method: 'POST',
    body: JSON.stringify({ expected_version: expectedVersion }),
  })
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers)
  if (init.body !== undefined) headers.set('Content-Type', 'application/json')
  const response = await fetch(path, { credentials: 'include', ...init, headers })
  const payload = await response.json().catch(() => undefined) as T | { error?: string; message?: string } | undefined
  if (!response.ok) {
    const message = payload && typeof payload === 'object' && ('error' in payload || 'message' in payload)
      ? payload.error ?? payload.message ?? '受控执行控制面请求失败'
      : '受控执行控制面请求失败'
    throw new ControlledExecutionApiError(response.status, message)
  }
  return payload as T
}
