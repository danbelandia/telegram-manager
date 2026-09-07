// Llamadas HTTP del dominio publicaciones (guia frontend seccion 3). Todo
// pasa por lib/api-client.ts, que normaliza el envelope del backend y
// agrega el access token. Los paths siguen AGENTS 12 / server.go.
//
// Slice 2: `listPublications` acepta filtro opcional por grupo
// (`?group_id=`); `createPublication` envia el nuevo body con foto,
// botones y `group_ids[]`.
import { request } from '../../lib/api-client'
import type { Publication, PublicationsFilter, PublishRequest } from './types'

/** GET /api/publications — listado mas reciente (limite 50). Con
 * filtro `group_id`, llama a `?group_id=<int>` y devuelve las 50 mas
 * recientes de ese grupo (idx_publications_telegram_id). */
export function listPublications(filter?: PublicationsFilter): Promise<Publication[]> {
  const qs = filter?.group_id !== undefined ? `?group_id=${filter.group_id}` : ''
  return request<Publication[]>(`/api/publications${qs}`)
}

/** GET /api/publications/:id — detalle por ID interno. */
export function getPublication(id: number): Promise<Publication> {
  return request<Publication>(`/api/publications/${id}`)
}

/** POST /api/publications — crea y publica a 1..N grupos (slice 2:
 * multi-grupo + foto + botones). La respuesta es `{publications: [...]}` */
export function createPublication(input: PublishRequest): Promise<{ publications: Publication[] }> {
  return request<{ publications: Publication[] }>('/api/publications', {
    method: 'POST',
    body: JSON.stringify(input),
  })
}
