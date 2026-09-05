// Cliente HTTP unico del panel (seccion 3 de la guia frontend).
// Centraliza base URL, normalizacion del envelope del backend
// ({data, error}) y, cuando exista auth, el header Authorization.
// Los componentes nunca parsean la respuesta cruda.

export type ApiErrorCode =
  | 'SUCCESS'
  | 'PERMISSION_DENIED'
  | 'TELEGRAM_ERROR'
  | 'VALIDATION_ERROR'
  | 'NOT_FOUND'
  | 'INTERNAL_ERROR'

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

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL ?? '/api'

/**
 * Ejecuta un request contra el backend y normaliza la respuesta al
 * envelope {data, error}. Lanza ApiError con el codigo y el mensaje
 * legible que define la seccion 18 del spec.
 */
export async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE_URL}${path}`, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      // Cuando exista auth (seccion 17.1 del spec), aca se agrega:
      // Authorization: `Bearer ${accessToken}` — nunca desde localStorage.
      ...init?.headers,
    },
  })

  if (!res.ok) {
    throw new ApiError('INTERNAL_ERROR', `El servidor respondió ${res.status}`, res.status)
  }

  const body = (await res.json()) as Envelope<T>
  if (body.error !== null) {
    throw new ApiError(
      (body.error.code as ApiErrorCode) ?? 'INTERNAL_ERROR',
      body.error.message || 'Error de la aplicación',
      res.status,
    )
  }

  return body.data as T
}