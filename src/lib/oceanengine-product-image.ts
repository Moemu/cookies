export function oceanEngineImageSourceIdentity(value: unknown) {
  if (typeof value !== 'string' || !value.trim()) return ''
  try {
    const parsed = new URL(value.trim(), 'https://cookies.invalid')
    const path = decodeURIComponent(parsed.pathname).replace(/^\/+/, '')
    return path.split('~', 1)[0]
  } catch {
    return ''
  }
}
