import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// /api owns the local compatibility login session; /platform/v1 is served by
// the Go product API. They must not share a proxy target in the local demo.
const platformProxyTarget = process.env.VITE_PLATFORM_PROXY_TARGET ?? 'http://127.0.0.1:8080'
const compatibilityApiProxyTarget = process.env.VITE_COMPAT_API_PROXY_TARGET ?? 'http://127.0.0.1:8787'

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/api': compatibilityApiProxyTarget,
      '/platform': platformProxyTarget,
    },
  },
})
