import { afterEach, describe, expect, it, vi } from 'vitest'
import { createClientUUID } from './clientId'

describe('createClientUUID', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('uses the browser UUID implementation when it is available', () => {
    const randomUUID = vi.fn(() => 'native-uuid')
    vi.stubGlobal('crypto', { randomUUID })

    expect(createClientUUID()).toBe('native-uuid')
    expect(randomUUID).toHaveBeenCalledOnce()
  })

  it('generates a UUID when randomUUID is unavailable on an HTTP origin', () => {
    vi.stubGlobal('crypto', {
      getRandomValues: (bytes: Uint8Array) => {
        bytes.set(Array.from({ length: 16 }, (_, index) => index))
        return bytes
      },
    })

    expect(createClientUUID()).toBe('00010203-0405-4607-8809-0a0b0c0d0e0f')
  })
})
