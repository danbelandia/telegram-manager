// Llamadas HTTP del dominio tenant-settings. Todo pasa por
// lib/api-client.ts (guia frontend seccion 3).
import { request } from '../../lib/api-client'
import type { RotateTokenInput, TenantMe, TenantStatus } from './types'

/** GET /api/tenants/me. Datos del tenant autenticado (sin token). */
export function getTenantMe(): Promise<TenantMe> {
  return request<TenantMe>('/api/tenants/me')
}

/** PUT /api/tenants/me/bot-token. Rotacion con password + nuevo token. */
export function rotateBotToken(body: RotateTokenInput): Promise<{ status: string }> {
  return request<{ status: string }>('/api/tenants/me/bot-token', {
    method: 'PUT',
    body: JSON.stringify(body),
  })
}

/** GET /api/tenants/me/status. Estado runtime del bot (liviano). */
export function getTenantStatus(): Promise<TenantStatus> {
  return request<TenantStatus>('/api/tenants/me/status')
}
