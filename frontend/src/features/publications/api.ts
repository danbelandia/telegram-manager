// Llamadas HTTP del dominio publicaciones (guia frontend seccion 3). Todo
// pasa por lib/api-client.ts, que normaliza el envelope del backend y
// agrega el access token. Los paths siguen AGENTS 12 / server.go.
//
// Slice 2: `listPublications` acepta filtro opcional por grupo
// (`?group_id=`); `createPublication` envia el nuevo body con foto,
// botones y `group_ids[]`.
//
// Slice 3: `listPublications` agrega `?limit=` y `?offset=` (paginacion).
// `cancelPublication(id)` cancela una fila `scheduled` (DELETE).
import { request } from '../../lib/api-client'
import type { Publication, PublicationsFilter, PublishRequest } from './types'

/** GET /api/publications — listado paginado. Default backend limit=50,
 * offset=0. */
export function listPublications(filter?: PublicationsFilter): Promise<Publication[]> {
  const qs = buildListQueryString(filter)
  return request<Publication[]>(`/api/publications${qs}`)
}

/** GET /api/publications/:id — detalle por ID interno. */
export function getPublication(id: number): Promise<Publication> {
  return request<Publication>(`/api/publications/${id}`)
}

/** POST /api/publications — crea y publica (o programa) a 1..N grupos.
 * La respuesta es `{publications: [...]}`. */
export function createPublication(input: PublishRequest): Promise<{ publications: Publication[] }> {
  return request<{ publications: Publication[] }>('/api/publications', {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

/** DELETE /api/publications/:id — cancela una publicacion `scheduled`.
 * Devuelve 204 No Content en exito; el helper api-client maneja el
 * caso 204 sin body automaticamente. */
export function cancelPublication(id: number): Promise<void> {
  return request<void>(`/api/publications/${id}`, { method: 'DELETE' })
}

// buildListQueryString arma la query string de `listPublications`. Si
// todos los campos son undefined/vacios, devuelve ''. Mantener unico
// lugar para evitar inconsistencias entre hooks y tests.
function buildListQueryString(filter?: PublicationsFilter): string {
  if (!filter) return ''
  const parts: string[] = []
  if (filter.group_id !== undefined) parts.push(`group_id=${filter.group_id}`)
  // limit/offset se envian SIEMPRE que esten definidos (>=0). El backend
  // aplica defaults si no llegan. Asi la query URL es estable para
  // tests y para query keys de React Query (cambio de offset => cache miss).
  if (filter.limit !== undefined) parts.push(`limit=${filter.limit}`)
  if (filter.offset !== undefined) parts.push(`offset=${filter.offset}`)
  return parts.length ? `?${parts.join('&')}` : ''
}
