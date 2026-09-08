// Cliente HTTP unico del panel (seccion 3 de la guia frontend).
// Centraliza base URL, normalizacion del envelope del backend
// ({data, error}), el header Authorization y el refresh transparente
// del access token (AGENTS 17.1): ante un 401 se refresca con la cookie
// httpOnly y se reejecuta una sola vez.
//
// El access token vive en memoria (nunca localStorage); el refresh token
// es una cookie httpOnly que el navegador maneja solo y el codigo
// cliente nunca lee ni escribe (AGENTS 17.1).

export type ApiErrorCode =
  | 'SUCCESS'
  | 'PERMISSION_DENIED'
  | 'TELEGRAM_ERROR'
  | 'VALIDATION_ERROR'
  | 'CONFLICT'
  | 'NOT_FOUND'
  | 'INTERNAL_ERROR'
  | 'UNAUTHORIZED'
  | 'INVALID_CREDENTIALS'

export class ApiError extends Error {
  constructor(
    readonly code: ApiErrorCode,
    message: string,
    readonly status: number,
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

interface Envelope<T> {
  data: T | null
  error: {
    code: string
    message: string
  } | null
}

// Base URL de la API. Los paths de request() SIEMPRE llegan completos
// con prefijo /api (ej. "/api/groups"); por defecto la base es vacia y
// en desarrollo el proxy de Vite (/api -> backend:8080) resuelve la
// ruta. VITE_API_BASE_URL solo se usa cuando el panel se sirve desde
// una raiz distinta (ej. https://panel.example.com en produccion).
const API_BASE_URL = import.meta.env.VITE_API_BASE_URL ?? ''

// Token de acceso en memoria. Lo setea AuthContext tras login/refresh;
// nunca persiste en localStorage (AGENTS 17.1).
let accessToken: string | null = null

/** Guarda el access token en memoria (lo llama AuthContext). */
export function setAccessToken(token: string | null): void {
  accessToken = token
}

// Callback que AuthContext registra para cerrar la sesion cuando el
// refresh falla (access y refresh expirados).
let onUnauthorized: (() => void) | null = null

/** Registra el manejador de 401 irreparable (lo llama AuthContext). */
export function setOnUnauthorized(handler: (() => void) | null): void {
  onUnauthorized = handler
}

/**
 * Ejecuta un request contra el backend y normaliza la respuesta al
 * envelope {data, error}. Lanza ApiError con el codigo y el mensaje
 * legible que define la seccion 18 del spec.
 *
 * Manejo de 401: si la peticion llevaba access token y el backend
 * responde UNAUTHORIZED (token expirado), refresca una sola vez con la
 * cookie httpOnly y reejecuta. Si el refresh tambien falla, invoca
 * onUnauthorized (AuthContext cierra sesion).
 */
async function rawRequest<T>(
  path: string,
  init?: RequestInit,
  retried = false,
): Promise<T> {
  const res = await fetch(`${API_BASE_URL}${path}`, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      ...(accessToken ? { Authorization: `Bearer ${accessToken}` } : {}),
      ...init?.headers,
    },
    credentials: 'same-origin', // la cookie httpOnly del refresh viaja sola
  })

  if (!res.ok && res.status === 401 && !retried) {
    const refreshed = await tryRefresh()
    if (refreshed) {
      return rawRequest<T>(path, init, true) // reejecuta una sola vez
    }
    onUnauthorized?.()
  }

  if (res.status === 204) {
    return undefined as T // logout sin body
  }

  if (!res.ok) {
    const body = (await res.json().catch(() => null)) as Envelope<T> | null
    if (body?.error) {
      throw new ApiError(
        (body.error.code as ApiErrorCode) ?? 'INTERNAL_ERROR',
        body.error.message || 'Error de la aplicacion',
        res.status,
      )
    }
    throw new ApiError('INTERNAL_ERROR', `El servidor respondio ${res.status}`, res.status)
  }

  const body = (await res.json()) as Envelope<T>
  if (body.error !== null) {
    throw new ApiError(
      (body.error.code as ApiErrorCode) ?? 'INTERNAL_ERROR',
      body.error.message || 'Error de la aplicacion',
      res.status,
    )
  }

  return body.data as T
}

/** Intenta POST /api/auth/refresh con la cookie httpOnly. Devuelve true si renovo el access. */
async function tryRefresh(): Promise<boolean> {
  try {
    const res = await fetch(`${API_BASE_URL}/api/auth/refresh`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'same-origin',
    })
    if (!res.ok) {
      return false
    }
    const body = (await res.json()) as Envelope<{ access_token: string }>
    if (!body.data?.access_token) {
      return false
    }
    setAccessToken(body.data.access_token)
    return true
  } catch {
    return false
  }
}

/**
 * API publica del cliente: request con refresh transparente.
 * Los paths se pasan completos con prefijo /api (ej. "/api/groups").
 */
export async function request<T>(path: string, init?: RequestInit): Promise<T> {
  return rawRequest<T>(path, init)
}