// Mensajes legibles de error para el dominio moderacion (seccion 18 del
// spec). El backend ya devuelve textos comprensibles en envelope.error
// (ej. "el bot no tiene permisos suficientes en este grupo"), asi que
// este helper solo cubre el caso en que el mensaje no llegue
// (decisión D3): mostrar algo util en lugar de un codigo crudo.

/** Si el error no trae mensaje, devuelve uno generico en espanol. */
export function formatModerationError(error: unknown): string {
  if (error instanceof Error && error.message.trim().length > 0) {
    return error.message
  }
  return 'No se pudo completar la acción de moderación.'
}