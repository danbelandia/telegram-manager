// Llamadas HTTP del dominio groups (guia frontend seccion 3). Todo pasa
// por lib/api-client.ts, que normaliza el envelope del backend y agrega
// el access token cuando existe.
import { request } from '../../lib/api-client'
import type { Group } from './types'

/** GET /api/groups — todos los grupos administrables. */
export function listGroups(): Promise<Group[]> {
  return request<Group[]>('/api/groups')
}

/** GET /api/groups/:id — detalle de un grupo. El :id es el ID de Telegram. */
export function getGroup(telegramId: string): Promise<Group> {
  return request<Group>(`/api/groups/${telegramId}`)
}