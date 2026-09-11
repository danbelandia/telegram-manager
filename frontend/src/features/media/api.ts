import { request } from '../../lib/api-client'
import type { MediaUploadResponse } from './types'

/**
 * Sube un archivo (foto o video) al backend via multipart/form-data.
 * POST /api/media/upload — devuelve la referencia para usar en publicaciones.
 */
export function uploadMedia(file: File): Promise<MediaUploadResponse> {
  const formData = new FormData()
  formData.append('file', file)

  return request<MediaUploadResponse>('/api/media/upload', {
    method: 'POST',
    body: formData,
    // NO setear Content-Type — fetch lo genera automaticamente con el
    // boundary correcto para multipart/form-data.
    headers: {},
  })
}
