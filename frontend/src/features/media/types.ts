/** Respuesta de POST /api/media/upload. */
export interface MediaUploadResponse {
  filename: string
  url: string
  type: 'photo' | 'video'
}
