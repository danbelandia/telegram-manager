// Tests del AuthContext (spec frontend-auth): restauracion de sesion
// con /me, login que persiste el token, logout que limpia estado.
// Se usa mockFetchRoutes (by URL) porque el api-client refresca
// automaticamente ante 401 — los mocks por orden se desalinean.
import { act, renderHook, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { ReactNode } from 'react'
import { AuthProvider, useAuth } from './auth-context'
import { createTestQueryClient, errorJson, mockFetchRoutes, noContent, okJson } from '../test/helpers'

// Wrapper con QueryClientProvider: AuthProvider usa useQueryClient() para
// resolver el cliente y limpiar la cache en logout/401 (slice 4 spec).
// createTestQueryClient devuelve un cliente fresco por render — gcTime: 0
// para que la cache no contamine entre tests.
const wrapper = ({ children }: { children: ReactNode }) => (
  <QueryClientProvider client={createTestQueryClient()}>
    <AuthProvider>{children}</AuthProvider>
  </QueryClientProvider>
)

describe('AuthContext', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('restaura la sesion con /me exitoso al montar', async () => {
    mockFetchRoutes({
      '/api/auth/me': () => okJson({ id: '1', username: 'admin', tenant_id: 7, tenant_slug: 'acme' }),
    })

    const { result } = renderHook(() => useAuth(), { wrapper })

    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(result.current.user).toEqual({ id: '1', username: 'admin', tenantId: 7, tenantSlug: 'acme' })
  })

  it('queda sin sesion cuando /me falla', async () => {
    mockFetchRoutes({
      '/api/auth/me': () => errorJson(401, 'UNAUTHORIZED', 'no autenticado'),
      '/api/auth/refresh': () => errorJson(401, 'UNAUTHORIZED', 'refresh invalido'),
    })

    const { result } = renderHook(() => useAuth(), { wrapper })

    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(result.current.user).toBeNull()
  })

  it('login guarda token en memoria y expone el usuario', async () => {
    mockFetchRoutes({
      // /me del primer montaje falla (sin sesion); el login luego re-valida
      '/api/auth/login': () => okJson({ access_token: 'jwt-nuevo' }),
      '/api/auth/me': () => okJson({ id: '7', username: 'adm', tenant_id: 7 }),
      '/api/auth/refresh': () => okJson({ access_token: 'jwt-refresh' }),
    })

    const { result } = renderHook(() => useAuth(), { wrapper })
    await waitFor(() => expect(result.current.loading).toBe(false))

    await act(async () => {
      await result.current.login('adm', 'secreto')
    })

    expect(result.current.user).toEqual({ id: '7', username: 'adm', tenantId: 7, tenantSlug: null })
  })

  it('rechaza login con credenciales invalidas y no expone usuario', async () => {
    mockFetchRoutes({
      '/api/auth/login': () => errorJson(401, 'INVALID_CREDENTIALS', 'credenciales invalidas'),
      '/api/auth/me': () => errorJson(401, 'UNAUTHORIZED', 'no autenticado'),
      '/api/auth/refresh': () => errorJson(401, 'UNAUTHORIZED', 'refresh invalido'),
    })

    const { result } = renderHook(() => useAuth(), { wrapper })
    await waitFor(() => expect(result.current.loading).toBe(false))

    await expect(result.current.login('mal', 'mal')).rejects.toBeInstanceOf(Error)
    expect(result.current.user).toBeNull()
  })

  it('logout limpia la sesion', async () => {
    mockFetchRoutes({
      '/api/auth/me': () => okJson({ id: '7', username: 'adm', tenant_id: 7 }),
      '/api/auth/logout': noContent,
    })

    const { result } = renderHook(() => useAuth(), { wrapper })
    await waitFor(() => expect(result.current.user).toEqual({ id: '7', username: 'adm', tenantId: 7, tenantSlug: null }))

    await act(async () => {
      await result.current.logout()
    })

    expect(result.current.user).toBeNull()
  })

  // Fuga de datos cross-user: si el usuario A cierra sesion y el
  // usuario B abre la misma pestana, B no debe ver los grupos
  // cacheados de A. AuthProvider debe limpiar el cache del
  // QueryClient que recibe por contexto (slice 4 spec).
  it('logout limpia la cache de TanStack Query', async () => {
    mockFetchRoutes({
      '/api/auth/me': () => okJson({ id: '7', username: 'adm', tenant_id: 7 }),
      '/api/auth/logout': noContent,
    })

    // Cliente fresco por test (gcTime: 0 en createTestQueryClient no
    // aplica aca porque necesitamos setQueryData + clear verificable
    // sobre el MISMO cliente que el AuthProvider recibio por contexto).
    const testQueryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })
    testQueryClient.setQueryData(['groups'], [{ id: 1, title: 'viejo' }])

    const wrapperWithQuery = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={testQueryClient}>
        <AuthProvider>{children}</AuthProvider>
      </QueryClientProvider>
    )

    const { result } = renderHook(() => useAuth(), { wrapper: wrapperWithQuery })
    await waitFor(() =>
      expect(result.current.user).toEqual({
        id: '7',
        username: 'adm',
        tenantId: 7,
        tenantSlug: null,
        isSuperAdmin: false,
      }),
    )

    await act(async () => {
      await result.current.logout()
    })

    expect(testQueryClient.getQueryData(['groups'])).toBeUndefined()
  })
})