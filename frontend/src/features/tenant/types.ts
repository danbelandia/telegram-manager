// Tipos del dominio tenant-settings (spec tenant-settings REQ 1-4).
// Derivados del contrato real del backend (tenant_handlers.go).
export interface TenantMe {
  slug: string
  bot_username: string | null
  bot_status: string
  created_at: string
}

export interface RotateTokenInput {
  password: string
  bot_token: string
}

export interface TenantStatus {
  bot_status: string
}
