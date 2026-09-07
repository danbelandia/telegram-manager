// Mensajes legibles de error para el dominio publicaciones (seccion 18
// del spec). El backend ya devuelve textos comprensibles en
// envelope.error (ej. "el bot no tiene permisos suficientes en este
// grupo"), asi que este helper solo cubre el caso en que el mensaje no
// llegue (misma convencion que features/moderation/error.ts, decision D3).

/** Si el error no trae mensaje, devuelve uno generico en espanol. */
export function formatPublicationsError(error: unknown): string {
  if (error instanceof Error && error.message.trim().length > 0) {
    return error.message
  }
  return 'No se pudo publicar el mensaje.'
}
