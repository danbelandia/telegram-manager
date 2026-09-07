// Tipos del dominio auth. Derivados del contrato real del backend
// (backend/internal/api/auth_handlers.go) — ver guia frontend seccion 2.

/** Respuesta de POST /api/auth/login y /api/auth/refresh. */
export interface LoginResponse {
  access_token: string
}

/** Respuesta de GET /api/auth/me (identidad del admin autenticado). */
export interface MeResponse {
  id: string
  username: string
}