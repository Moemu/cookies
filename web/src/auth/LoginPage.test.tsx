import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { LoginPage } from './LoginPage'

describe('LoginPage', () => {
  afterEach(() => {
    cleanup()
    vi.unstubAllGlobals()
  })

  it('logs in with the administrator account and returns to the protected route', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response(JSON.stringify({
      actor: { organization_id: 'org_1', principal: { kind: 'user', id: 'admin_1' }, scopes: [] },
      expires_at: '2026-07-24T12:00:00Z',
    }), { status: 200, headers: { 'Content-Type': 'application/json' } })))

    render(<MemoryRouter initialEntries={[{
      pathname: '/login',
      state: { returnTo: '/projects/project_1/strategy/workspaces' },
    }]}>
      <Routes>
        <Route element={<LoginPage />} path="/login" />
        <Route element={<h1>策略工作区</h1>} path="/projects/:projectId/strategy/workspaces" />
      </Routes>
    </MemoryRouter>)

    expect(screen.getByLabelText('账号')).toHaveValue('Admin')
    fireEvent.change(screen.getByLabelText('密码'), { target: { value: '123456' } })
    fireEvent.click(screen.getByRole('button', { name: '登录' }))

    expect(await screen.findByRole('heading', { name: '策略工作区' })).toBeInTheDocument()
    await waitFor(() => expect(fetch).toHaveBeenCalledWith('/platform/v1/auth/login', expect.objectContaining({
      method: 'POST',
      body: JSON.stringify({ username: 'Admin', password: '123456' }),
    })))
  })

  it('shows a generic authentication error without leaving the login page', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response(JSON.stringify({
      error: {
        code: 'INVALID_CREDENTIALS',
        message: '账号或密码错误，请稍后重试',
        request_id: 'req_test',
        retryable: false,
        details: [],
      },
    }), { status: 401, headers: { 'Content-Type': 'application/json' } })))

    render(<MemoryRouter initialEntries={['/login']}>
      <Routes><Route element={<LoginPage />} path="/login" /></Routes>
    </MemoryRouter>)
    fireEvent.change(screen.getByLabelText('密码'), { target: { value: 'wrong' } })
    fireEvent.click(screen.getByRole('button', { name: '登录' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('账号或密码错误')
    expect(screen.getByRole('heading', { name: '欢迎回来' })).toBeInTheDocument()
  })
})
