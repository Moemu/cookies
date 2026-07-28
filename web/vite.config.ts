import react from '@vitejs/plugin-react'
import { defineConfig } from 'vitest/config'

const apiTarget = process.env.COOKIES_WEB_API_PROXY_TARGET || 'http://127.0.0.1:8080'
const webHost = process.env.COOKIES_WEB_HOST || '127.0.0.1'
const configuredWebPort = Number.parseInt(process.env.COOKIES_WEB_PORT || '5173', 10)
const webPort = Number.isNaN(configuredWebPort) ? 5173 : configuredWebPort

export default defineConfig({
  plugins: [react()],
  server: {
    host: webHost,
    port: webPort,
    strictPort: true,
    proxy: {
      '/platform': apiTarget,
      '/api': apiTarget,
    },
  },
  test: {
    environment: 'jsdom',
    setupFiles: './src/test/setup.ts',
  },
})
