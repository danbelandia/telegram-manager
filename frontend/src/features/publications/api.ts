// Llamadas HTTP del dominio publicaciones (guia frontend seccion 3). Todo
// pasa por lib/api-client.ts, que normaliza el envelope del backend y
// agrega el access token. Los paths siguen AGENTS 12 / server.go.
import { request } from '../../lib/api-client'
import type { CreatePublicationInput, Publication } from './types'

/** GET /api/publications — listado mas reciente (limite 50). */
export function listPublications(): Promise<Publication[]> {
  return request<Publication[]>('/api/publications')
}

/** GET /api/publications/:id — detalle por ID interno. */
export function getPublication(id: number): Promise<Publication> {
  return request<Publication>(`/api/publications/${id}`)
}

/** POST /api/publications — crea y publica ahora (texto). */
export function createPublication(input: CreatePublicationInput): Promise<Publication> {
  return request<Publication>('/api/publications', {
    method: 'POST',
    body: JSON.stringify(input),
  })
}
