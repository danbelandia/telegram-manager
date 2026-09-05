import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'

// Config de Vite para el panel. `test` es de Vitest (vite, mismo motor).
// El server escucha en 0.0.0.0 para el Dockerfile de desarrollo.
// `node_modules` se monta como volumen anonimo en compose (friccion
// OneDrive/Windows con bind mounts, ver exploration.md).
export default defineConfig({
  plugins: [react()],
  server: {
    host: '0.0.0.0',
    port: 5173,
  },
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
  },
})