// Helpers de test compartidos: envolver componentes con los providers
// que la app usa (Auth + Query + Router + Mantine + Notifications) y
// mockear la capa de auth. `renderWithProviders` ahora provee el contexto
// de Mantine para que los componentes que usan `<Button>`, `<TextInput>`,
// etc. rendericen sin error de "MantineProvider was not found" (spec
// REQ-11 — frontend-refresh slice 1).
import { render } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MantineProvider } from '@mantine/core'
import { Notifications } from '@mantine/notifications'
import { vi } from 'vitest'
import { mantineTheme } from '../theme'

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

// Provider comun de la app: QueryClient + Router + Mantine + Notifications.
// Notifications se monta para que los helpers notifySuccess/notifyError
// puedan invocarse sin tirar en tests.
export function renderWithProviders(ui: React.ReactNode, initialEntries: string[] = ['/']) {
  return render(ui, {
    wrapper: ({ children }) => (
      <MantineProvider theme={mantineTheme} defaultColorScheme="light">
        <Notifications position="top-right" />
        <QueryClientProvider client={createTestQueryClient()}>
          <MemoryRouter initialEntries={initialEntries}>{children}</MemoryRouter>
        </QueryClientProvider>
      </MantineProvider>
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

// matchQuery helper: parsea los query params de un URL y devuelve true
// si todos los pares clave/valor en `expected` matchean. Usado por
// tests que necesitan distinguir GET con ?limit=&offset= del GET sin
// filtro (slice 3 paginacion).
export function matchQuery(url: string, expected: Record<string, string>): boolean {
  try {
    const u = new URL(url, 'http://x')
    for (const [k, v] of Object.entries(expected)) {
      if (u.searchParams.get(k) !== v) return false
    }
    return true
  } catch {
    return false
  }
}