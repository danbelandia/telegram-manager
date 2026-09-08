// Contexto de sesion (guia frontend seccion 4): unico Context global
// legitimo del panel. El access token vive en memoria via
// lib/api-client.setAccessToken; el refresh token es cookie httpOnly
// manejada por el navegador (AGENTS 17.1) — nunca se lee ni escribe aca.
import { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react'
import * as authApi from '../features/auth/api'
import { setAccessToken, setOnUnauthorized } from './api-client'

export interface SessionUser {
  id: string
  username: string
  tenantId: number | null
  tenantSlug: string | null
}

interface AuthContextValue {
  /** true mientras se restaura la sesion con /me al montar la app. */
  loading: boolean
  /** Identidad del admin autenticado, null si no hay sesion. */
  user: SessionUser | null
  login: (username: string, password: string) => Promise<void>
  logout: () => Promise<void>
}

const AuthContext = createContext<AuthContextValue | null>(null)

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [loading, setLoading] = useState(true)
  const [user, setUser] = useState<SessionUser | null>(null)

  const doLogout = useCallback(async () => {
    setAccessToken(null)
    setUser(null)
    try {
      await authApi.logout() // expira la cookie de refresh en el backend
    } catch {
      // el estado local ya quedo limpio; el fallo de la API no deja
      // sesion abierta en la app
    }
  }, [])

  // Registra el manejador de 401 irreparable: refresh fallido => cerrar sesion.
  useEffect(() => {
    setOnUnauthorized(() => {
      setAccessToken(null)
      setUser(null)
    })
    return () => setOnUnauthorized(null)
  }, [])

  // Restauracion de sesion al montar: si hay cookie de refresh valida,
  // el backend responde con un access token nuevo y /me devuelve la
  // identidad. Sin cookie, la app queda sin sesion (RequireAuth
  // redirige a /login).
  useEffect(() => {
    let cancelled = false
    const resumir = async () => {
      try {
        const me = await authApi.me()
        if (!cancelled) {
          setUser({
            id: me.id,
            username: me.username,
            tenantId: me.tenant_id,
            tenantSlug: me.tenant_slug ?? null,
          })
        }
      } catch {
        // 401 u otro error: sin sesion restaurable
      } finally {
        if (!cancelled) {
          setLoading(false)
        }
      }
    }
    void resumir()
    return () => {
      cancelled = true
    }
  }, [])

  const login = useCallback(async (username: string, password: string) => {
    const res = await authApi.login(username, password)
    setAccessToken(res.access_token)
    const me = await authApi.me()
    setUser({
      id: me.id,
      username: me.username,
      tenantId: me.tenant_id,
      tenantSlug: me.tenant_slug ?? null,
    })
  }, [])

  const value = useMemo(
    () => ({ loading, user, login, logout: doLogout }),
    [loading, user, login, doLogout],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

/** Hook de sesion. Lanzar fuera de <AuthProvider> es un error de wiring. */
export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext)
  if (!ctx) {
    throw new Error('useAuth debe usarse dentro de <AuthProvider>')
  }
  return ctx
}