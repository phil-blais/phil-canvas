import { defineConfig, type ProxyOptions } from 'vite'
import react from '@vitejs/plugin-react'

// Backend Go service during local dev. Override with VITE_BACKEND_ORIGIN if needed.
const backend = process.env.VITE_BACKEND_ORIGIN ?? 'http://localhost:8080'

// Mirrors firebase.json's Hosting rewrites, so both `vite dev` and
// `vite preview` route these same-origin paths to the local Go backend.
const proxy: Record<string, ProxyOptions> = {
  // WebSocket relay — ws proxying must be enabled.
  '/ws': { target: backend, ws: true, changeOrigin: true },
  // HTTP API surfaces routed to the Go backend (mirrors firebase.json rewrites).
  '/auth': { target: backend, changeOrigin: true },
  '/rooms': { target: backend, changeOrigin: true },
  '/scenes': { target: backend, changeOrigin: true },
}

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: { proxy },
  // `preview` is a separate server from `server` (used by `vite preview`,
  // which serves the production `dist/` build) — it doesn't inherit `server.proxy`.
  preview: { proxy },
})
