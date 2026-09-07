// Contrato del dominio publicaciones segun publications_handlers.go del
// backend (publicationResponse). El envelope llega normalizado por
// lib/api-client; aca solo se tipa la forma del data.
//
// Slice 2: `Publication` agrega `photo_url` y `buttons`; el body de
// creacion pasa a ser multi-grupo (`group_ids: number[]`) y permite
// contenido enriquecido (foto por URL + botones inline de URL).

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
  /** Verbatim JSONB: array de filas, cada fila array de InlineButton. */
  buttons: InlineButton[][] | null
  created_at: string
}

/** Body de POST /api/publications (slice 2: multi-grupo + foto + botones). */
export interface PublishRequest {
  text: string
  photo_url?: string
  buttons?: InlineButton[][]
  group_ids: number[]
}

/** Filtro opcional del listado. */
export interface PublicationsFilter {
  group_id?: number
}

/** Envelope del handler para la respuesta multi-grupo (slice 2). */
export interface PublicationsListResponse {
  publications: Publication[]
}
