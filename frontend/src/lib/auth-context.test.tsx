// Tests del AuthContext (spec frontend-auth): restauracion de sesion
// con /me, login que persiste el token, logout que limpia estado.
// Se usa mockFetchRoutes (by URL) porque el api-client refresca
// automaticamente ante 401 — los mocks por orden se desalinean.
import { act, renderHook, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { ReactNode } from 'react'
import { AuthProvider, useAuth } from './auth-context'
import { errorJson, mockFetchRoutes, noContent, okJson } from '../test/helpers'

const wrapper = ({ children }: { children: ReactNode }) => <AuthProvider>{children}</AuthProvider>

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
})