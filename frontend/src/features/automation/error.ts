// Mensajes legibles de error y validación cliente para el dominio
// moderacion automatica (AGENTS §23, slice 2 + slice 3). El backend
// ya devuelve textos comprensibles en envelope.error (ej. "palabra
// invalida", "no hay advertencias para este usuario en este grupo");
// este helper solo cubre el caso en que el mensaje no llegue y
// centraliza las validaciones cliente que cortan el roundtrip antes
// del POST.

/** Si el error no trae mensaje, devuelve uno generico en español. */
export function formatAutomationError(error: unknown): string {
  if (error instanceof Error && error.message.trim().length > 0) {
    return error.message
  }
  return 'No se pudo guardar la configuración de moderación.'
}

/** Errores especificos del dashboard de slice 3. Devuelve un mensaje
 * legible cuando el envelope del backend trae code conocido. Si no
 * matchea, cae al fallback generico de formatAutomationError. */
export function formatDashboardError(error: unknown): string {
  if (error instanceof Error && /no hay advertencias/i.test(error.message)) {
    return 'No hay advertencias activas para mostrar.'
  }
  if (error instanceof Error && /period invalido/i.test(error.message)) {
    return error.message
  }
  return formatAutomationError(error)
}

// Validacion cliente de una palabra prohibida: 1-100 chars, letras /
// digitos / espacio / guion / underscore. Coincide con la regex del
// backend (automation_handlers.go validateBannedWord) para que el
// error message sea consistente.
export function validateBannedWordClient(word: string): string | null {
  const trimmed = word.trim()
  if (!trimmed) return 'la palabra no puede estar vacía'
  if (trimmed.length > 100) return 'la palabra no puede tener más de 100 caracteres'
  for (const ch of trimmed) {
    const ok =
      (ch >= 'a' && ch <= 'z') ||
      (ch >= 'A' && ch <= 'Z') ||
      (ch >= '0' && ch <= '9') ||
      ch === ' ' ||
      ch === '-' ||
      ch === '_'
    if (!ok) return 'solo letras, dígitos, espacio, guion o underscore'
  }
  return null
}

// Validacion cliente de un dominio: 1-253 chars (RFC 1035 max
// hostname), sin espacios.
export function validateAllowlistClient(domain: string): string | null {
  const trimmed = domain.trim()
  if (!trimmed) return 'el dominio no puede estar vacío'
  if (trimmed.length > 253) return 'el dominio es demasiado largo (max 253 caracteres)'
  if (/\s/.test(trimmed)) return 'el dominio no puede tener espacios'
  return null
}

// Validacion cliente de thresholds: autoban > automute > 0.
export function validateThresholdsClient(input: {
  flood_messages: number
  flood_seconds: number
  automute_warnings: number
  autoban_warnings: number
  warning_limit: number
  warning_expire_days: number
}): string | null {
  if (input.flood_messages <= 0) return 'los mensajes para flood deben ser mayores a 0'
  if (input.flood_seconds <= 0) return 'los segundos para flood deben ser mayores a 0'
  if (input.warning_limit <= 0) return 'el límite de advertencias debe ser mayor a 0'
  if (input.automute_warnings <= 0) return 'las advertencias para automute deben ser mayores a 0'
  if (input.autoban_warnings <= input.automute_warnings) {
    return 'las advertencias para autoban deben ser mayores que automute'
  }
  if (input.warning_expire_days <= 0) return 'los días de expiración deben ser mayores a 0'
  return null
}
