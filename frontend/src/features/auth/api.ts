// Llamadas HTTP del dominio auth (guia frontend seccion 3). Todo pasa
// por lib/api-client.ts, que normaliza el envelope del backend y agrega
// el access token cuando existe.
import { request } from '../../lib/api-client'
import type { LoginResponse, MeResponse } from './types'

/** POST /api/auth/login. La cookie del refresh la setea el navegador. */
export function login(username: string, password: string): Promise<LoginResponse> {
  return request<LoginResponse>('/api/auth/login', {
    method: 'POST',
    body: JSON.stringify({ username, password }),
  })
}

/** POST /api/auth/refresh. Usa la cookie httpOnly; devuelve el nuevo access token. */
export function refresh(): Promise<LoginResponse> {
  return request<LoginResponse>('/api/auth/refresh', { method: 'POST' })
}

/** POST /api/auth/logout. El backend expira la cookie de refresh. */
export function logout(): Promise<void> {
  return request<void>('/api/auth/logout', { method: 'POST' })
}

/** GET /api/auth/me. Prueba y restauracion de sesion con el access token vigente. */
export function me(): Promise<MeResponse> {
  return request<MeResponse>('/api/auth/me')
}