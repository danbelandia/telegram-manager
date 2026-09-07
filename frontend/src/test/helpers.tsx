// Helpers de test compartidos: envolver componentes con los providers
// que la app usa (Auth + Query) y mockear la capa de auth.
import { render } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { vi } from 'vitest'

export { render }

// Wrappers para renderizar componentes que dependen de Router.
export function renderWithRouter(ui: React.ReactNode, initialEntries: string[] = ['/']) {
  return render(ui, { wrapper: ({ children }) => <MemoryRouter initialEntries={initialEntries}>{children}</MemoryRouter> })
}

// QueryClient fresco por test (evita cache compartida entre tests).
export function createTestQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  })
}

export function renderWithProviders(ui: React.ReactNode, initialEntries: string[] = ['/']) {
  return render(ui, {
    wrapper: ({ children }) => (
      <MemoryRouter initialEntries={initialEntries}>
        <QueryClientProvider client={createTestQueryClient()}>{children}</QueryClientProvider>
      </MemoryRouter>
    ),
  })
}

// Mock de fetch dirigido por URL. El api-client refresca automaticamente
// ante 401 (vease api-client.ts), asi que los mocks por orden
// (mockResolvedValueOnce) se desalinean: el refresh consume respuestas
// destinadas a otras llamadas. Con dispatch por substring de URL cada
// endpoint responde siempre lo mismo, sin depender del orden.
type RouteHandler = () => Promise<unknown>

interface MockRoutes {
  [urlSubstring: string]: RouteHandler
}

export function mockFetchRoutes(routes: MockRoutes): void {
  vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL) => {
    const url = String(input)
    for (const [part, handler] of Object.entries(routes)) {
      if (url.includes(part)) {
        return handler()
      }
    }
    // Por defecto: 401 UNAUTHORIZED (sin sesion).
    return Promise.resolve({
      ok: false,
      status: 401,
      json: () => Promise.resolve({ data: null, error: { code: 'UNAUTHORIZED', message: 'no autenticado' } }),
    })
  }))
}

const okJson = (data: unknown) => Promise.resolve({
  ok: true,
  status: 200,
  json: () => Promise.resolve({ data, error: null }),
})

const errorJson = (status: number, code: string, message: string) => Promise.resolve({
  ok: false,
  status,
  json: () => Promise.resolve({ data: null, error: { code, message } }),
})

const noContent = () => Promise.resolve({
  ok: true,
  status: 204,
  json: () => Promise.reject(new Error('204 sin body')),
})

export { okJson, errorJson, noContent }