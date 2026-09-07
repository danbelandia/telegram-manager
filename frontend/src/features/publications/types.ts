// Contrato del dominio publicaciones segun publications_handlers.go del
// backend (publicationResponse). El envelope llega normalizado por
// lib/api-client; aca solo se tipa la forma del data.

export type PublicationStatus = 'draft' | 'scheduled' | 'sending' | 'sent' | 'failed'

/** Publicacion inmediata de texto (publicationResponse). */
export interface Publication {
  id: number
  telegram_id: number
  text: string
  status: PublicationStatus
  message_id: number | null
  error_message: string | null
  actor_id: number | null
  created_at: string
}

/** Body de POST /api/publications: group_id es groups.telegram_id. */
export interface CreatePublicationInput {
  text: string
  group_id: number
}
