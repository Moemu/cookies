import { afterEach, describe, expect, it, vi } from 'vitest'
import { putAssetContent, removeProjectAsset } from './api'

describe('asset upload API', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('keeps signed TOS headers while leaving forbidden transport headers to the browser', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(null, { status: 204 }))
    vi.stubGlobal('fetch', fetchMock)
    const file = new File(['image'], 'asset.png', { type: 'image/png' })

    await putAssetContent({
      url: 'https://tos.example.com/signed-upload',
      method: 'PUT',
      headers: {
        Host: 'tos.example.com',
        'Content-Length': String(file.size),
        'Content-Type': 'image/png',
        'x-tos-forbid-overwrite': 'true',
      },
      expires_at: '2026-07-22T01:00:00Z',
    }, file)

    const [, request] = fetchMock.mock.calls[0] as [string, RequestInit]
    const headers = new Headers(request.headers)
    expect(headers.get('host')).toBeNull()
    expect(headers.get('content-length')).toBeNull()
    expect(headers.get('content-type')).toBe('image/png')
    expect(headers.get('x-tos-forbid-overwrite')).toBe('true')
    expect(request.body).toBe(file)
  })

  it('removes only the selected project asset relationship', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(null, { status: 204 }))
    vi.stubGlobal('fetch', fetchMock)

    await removeProjectAsset('project/one', 'asset/two', 3)

    expect(fetchMock).toHaveBeenCalledWith('/platform/v1/projects/project%2Fone/assets/asset%2Ftwo/versions/3', expect.objectContaining({ method: 'DELETE' }))
  })
})
