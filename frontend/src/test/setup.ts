// Setup de Vitest: matchers de jest-dom y cleanup automatico de
// React Testing Library. Sin globals:true en la config, RTL no limpia
// por si solo los arboles entre tests (acumula DOM y rompe queries).
import '@testing-library/jest-dom/vitest'
import { cleanup } from '@testing-library/react'
import { afterEach } from 'vitest'

// Mantine v7 usa ResizeObserver (Tabler icons lo requiere para
// layout). jsdom no lo provee — stubbamos un observer no-op.
class ResizeObserverStub {
  observe() {}
  unobserve() {}
  disconnect() {}
}
;(globalThis as { ResizeObserver?: unknown }).ResizeObserver =
  (globalThis as { ResizeObserver?: unknown }).ResizeObserver ?? ResizeObserverStub

afterEach(() => {
  cleanup()
})