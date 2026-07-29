import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// /api owns the local compatibility login session; /platform/v1 is served by
// the Go product API. They must not share a proxy target in the local demo.
const platformProxyTarget = process.env.VITE_PLATFORM_PROXY_TARGET ?? 'http://127.0.0.1:8080'
const compatibilityApiProxyTarget = process.env.VITE_COMPAT_API_PROXY_TARGET ?? 'http://127.0.0.1:8787'

export default defineConfig({
  plugins: [react()],
  build: {
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (id.includes('/node_modules/lucide-react/')) return 'icons'
          if (id.includes('/node_modules/react/') || id.includes('/node_modules/react-dom/') || id.includes('/node_modules/react-router')) {
            return 'react-vendor'
          }
        },
      },
    },
  },
  server: {
    host: '127.0.0.1',
    port: 5173,
    proxy: {
      '/api': localAPIProxy(),
      '/platform': localAPIProxy(),
      '/healthz': localAPIProxy(),
      '/readyz': localAPIProxy(),
    },
  },
})

function localAPIProxy() {
  return {
    target: 'http://127.0.0.1:8080',
    timeout: 180_000,
    proxyTimeout: 180_000,
  }
}
