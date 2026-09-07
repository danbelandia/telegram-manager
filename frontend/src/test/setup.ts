// Setup de Vitest: matchers de jest-dom y cleanup automatico de
// React Testing Library. Sin globals:true en la config, RTL no limpia
// por si solo los arboles entre tests (acumula DOM y rompe queries).
import '@testing-library/jest-dom/vitest'
import { cleanup } from '@testing-library/react'
import { afterEach } from 'vitest'

afterEach(() => {
  cleanup()
})