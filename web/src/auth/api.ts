import { apiRequest } from '../shared/api/client'
import type { CurrentIdentity } from '../features/platform/types'

export function login(username: string, password: string) {
  return apiRequest<{ actor: CurrentIdentity['actor']; expires_at: string }>('/platform/v1/auth/login', {
    method: 'POST',
    body: JSON.stringify({ username, password }),
  })
}

export function logout() {
  return apiRequest<void>('/platform/v1/auth/logout', { method: 'POST' })
}

export function getCurrentIdentity(signal?: AbortSignal) {
  return apiRequest<CurrentIdentity>('/platform/v1/me', { signal })
}
