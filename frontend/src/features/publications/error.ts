// Mensajes legibles de error para el dominio publicaciones (seccion 18
// del spec). El backend ya devuelve textos comprensibles en
// envelope.error; este helper solo cubre el caso en que el mensaje no
// llegue y agrega mensajes de validacion cliente (slice 2 + slice 3)
// que cortan el envio antes del POST.

/** Si el error no trae mensaje, devuelve uno generico en espanol. */
export function formatPublicationsError(error: unknown): string {
  if (error instanceof Error && error.message.trim().length > 0) {
    return error.message
  }
  return 'No se pudo publicar el mensaje.'
}

/** Validacion cliente de la URL de la foto (slice 2: http(s) requerido). */
export function validatePhotoUrlClient(url: string): string | null {
  if (!url.trim()) return null // vacio = sin foto, OK
  try {
    const u = new URL(url)
    if (u.protocol !== 'http:' && u.protocol !== 'https:') {
      return 'la URL debe empezar con http o https'
    }
    return null
  } catch {
    return 'la URL debe empezar con http o https'
  }
}

/** Validacion cliente de la cantidad de grupos. */
export function validateGroupIdsClient(ids: number[]): string | null {
  if (ids.length === 0) return 'se requiere al menos un grupo'
  if (ids.length > 10) return 'máximo 10 grupos por publicación'
  return null
}

/** Validacion cliente de la estructura de botones. */
export function validateButtonsClient(rows: { text: string; url: string }[][]): string | null {
  if (rows.length === 0) return null
  if (rows.length > 8) return 'máximo 8 filas de botones'
  for (const row of rows) {
    if (row.length > 8) return 'máximo 8 botones por fila'
    for (const btn of row) {
      if (!btn.text.trim()) return 'el texto del botón no puede estar vacío'
      if (btn.text.length > 64) return 'el texto del botón excede 64 caracteres'
      if (!btn.url.trim()) return 'la URL del botón no puede estar vacía'
      try {
        const u = new URL(btn.url)
        if (u.protocol !== 'http:' && u.protocol !== 'https:') {
          return 'la URL del botón debe empezar con http o https'
        }
      } catch {
        return 'la URL del botón debe empezar con http o https'
      }
    }
  }
  return null
}
