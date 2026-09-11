// Contrato del dominio publicaciones segun publications_handlers.go del
// backend (publicationResponse). El envelope llega normalizado por
// lib/api-client; aca solo se tipa la forma del data.
//
// Slice 2: `Publication` agrega `photo_url` y `buttons`; el body de
// creacion pasa a ser multi-grupo (`group_ids: number[]`) y permite
// contenido enriquecido (foto por URL + botones inline de URL).
//
// Slice 3: agrega `scheduled_at` opcional al body y `scheduled_at`
// nullable al response; `PublishRequest` lleva `scheduled_at?` y el
// filtro `limit`/`offset`.

export type PublicationStatus = 'draft' | 'scheduled' | 'sending' | 'sent' | 'failed'

/** Boton de URL dentro de un inline keyboard (rows: InlineButton[][]). */
export interface InlineButton {
  text: string
  url: string
}

/** Publicacion inmediata (publicationResponse). */
export interface Publication {
  id: number
  telegram_id: number
  text: string
  status: PublicationStatus
  message_id: number | null
  error_message: string | null
  actor_id: number | null
  photo_url: string | null
  video_url: string | null
  /** Verbatim JSONB: array de filas, cada fila array de InlineButton. */
  buttons: InlineButton[][] | null
  /** RFC3339; presente en filas `scheduled`. */
  scheduled_at: string | null
  created_at: string
}

/** Body de POST /api/publications. Slice 3: `scheduled_at` opcional. */
export interface PublishRequest {
  text: string
  photo_url?: string
  video_url?: string
  buttons?: InlineButton[][]
  group_ids: number[]
  /** RFC3339 con offset; el backend normaliza a UTC y exige futuro. */
  scheduled_at?: string
}

/** Filtro + paginacion del listado (slice 3: limit/offset). */
export interface PublicationsFilter {
  group_id?: number
  limit?: number
  offset?: number
}

/** Envelope del handler para la respuesta multi-grupo. */
export interface PublicationsListResponse {
  publications: Publication[]
}
