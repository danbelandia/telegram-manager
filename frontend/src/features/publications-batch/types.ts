// Contrato del dominio publications-batch (publications-batch change).
// Modulo separado de features/publications/ para mantener cohesion:
// cada item del batch reusa los mismos campos que un PublishRequest
// individual, pero la envelope y la respuesta son distintas.
//
// Coherencia con el backend (backend/internal/api/batch_handlers.go):
//
//   - BatchRequest: { publications: BatchItemInput[] } (cap 10 server-side).
//   - BatchResponse: { created: BatchCreatedItem[], failed: BatchFailedItem[] }.
//   - BatchFailedItem.code: VALIDATION_ERROR | PERMISSION_DENIED |
//     NOT_FOUND | TELEGRAM_ERROR | INTERNAL_ERROR (seccion 18 spec).
//
// Las validaciones cliente (texto, foto, botones, grupos, scheduled_at)
// viven en features/publications/{error,validateScheduledAtClient}.ts
// y se REUSAN — este modulo no duplica helpers (REQ-24).

import type { InlineButton, Publication } from '../publications/types'

/** Item del batch: misma shape que PublishRequest individual. */
export interface BatchItemInput {
  text: string
  photo_url?: string
  video_url?: string
  buttons?: InlineButton[][]
  group_ids: number[]
  /** RFC3339 con offset; ausente o vacio = publicacion inmediata. */
  scheduled_at?: string
}

/** Envelope de POST /api/publications/batch. */
export interface BatchRequest {
  publications: BatchItemInput[]
}

/** Fila fallida: index = posicion ORIGINAL en req.publications. */
export interface BatchFailedItem {
  index: number
  code: string
  message: string
}

/** Fila creada: index original + publicacion completa (publicationResponse). */
export interface BatchCreatedItem {
  index: number
  publication: Publication
}

/** Body de 200 OK de POST /api/publications/batch. */
export interface BatchResponse {
  created: BatchCreatedItem[]
  failed: BatchFailedItem[]
}
