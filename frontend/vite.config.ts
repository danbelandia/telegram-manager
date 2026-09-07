import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'

// Config de Vite para el panel. `test` es de Vitest (vite, mismo motor).
// El server escucha en 0.0.0.0 para el Dockerfile de desarrollo.
// `node_modules` se monta como volumen anonimo en compose (friccion
// OneDrive/Windows con bind mounts, ver exploration.md).
// El proxy /api -> backend resuelve el acceso a la API en desarrollo
// (decision D7 del change frontend-panel): el frontend usa rutas
// relativas /api/... y el proxy evita CORS.
export default defineConfig({
  plugins: [react()],
  server: {
    host: '0.0.0.0',
    port: 5173,
    proxy: {
      '/api': {
        target: process.env.VITE_PROXY_TARGET ?? 'http://backend:8080',
        changeOrigin: true,
      },
    },
  },
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
  },
})