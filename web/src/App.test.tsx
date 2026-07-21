import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import App from './App'

describe('App', () => {
  it('mounts independent module workspaces from the shared shell', () => {
    render(<App />)

    expect(screen.getByRole('heading', { name: '策略工作区' })).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: '创意' }))
    expect(screen.getByRole('heading', { name: '创意工作区' })).toBeInTheDocument()
  })
})
