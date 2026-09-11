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
  tenant_id: number
  tenant_slug?: string
  is_super_admin: boolean
}

/** Body de POST /api/auth/signup (alta publica de tenant bot-per-tenant). */
export interface SignupRequest {
  slug: string
  username: string
  password: string
  bot_token: string
}

/** Respuesta 201 de POST /api/auth/signup. El token NUNCA vuelve. */
export interface SignupResponse {
  tenant: { id: number; slug: string }
  admin: { id: string; username: string }
}