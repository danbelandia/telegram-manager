// Helper cliente para validar y serializar `scheduled_at` (slice 3).
// El input viene de un <input type="datetime-local"> (formato
// "YYYY-MM-DDTHH:MM", zona horaria local del navegador); el backend
// exige RFC3339 con offset, normaliza a UTC y rechaza fechas pasadas.
// El cliente valida primero para no enviar POST innecesarios.

/** Resultado de la validacion: ok + iso UTC, o error legible. */
export type ValidateScheduledAtResult =
  | { ok: true; iso: string }
  | { ok: false; error: string }

/**
 * Valida y convierte el valor de `datetime-local` a RFC3339 con
 * offset (zona horaria local del navegador) listo para POST.
 *
 * Reglas:
 *   - valor vacio -> error "selecciona una fecha".
 *   - parse fallido -> error "fecha invalida".
 *   - <= now() -> error "la fecha debe ser futura".
 *   - OK -> ISO con offset (ej. "2027-06-15T17:00:00-03:00").
 *
 * `now` se inyecta para tests deterministas.
 */
export function validateScheduledAtClient(localValue: string, now: Date): ValidateScheduledAtResult {
  if (!localValue || !localValue.trim()) {
    return { ok: false, error: 'selecciona una fecha' }
  }
  // datetime-local no trae zona: lo interpretamos como local del browser
  // usando `new Date(value)` que ya aplica el offset local del runtime.
  const parsed = new Date(localValue)
  if (Number.isNaN(parsed.getTime())) {
    return { ok: false, error: 'fecha invalida' }
  }
  if (parsed.getTime() <= now.getTime()) {
    return { ok: false, error: 'la fecha debe ser futura' }
  }
  // toISOString retorna UTC ("...Z") pero el backend normaliza cualquier
  // offset igual; mandamos el string con offset local para que la fila
  // se programe en el momento UTC correcto independientemente de la
  // zona del cliente.
  const isoWithOffset = toRFC3339WithLocalOffset(parsed)
  return { ok: true, iso: isoWithOffset }
}

/** Serializa un Date a "YYYY-MM-DDTHH:MM:SS+HH:MM" usando la zona
 * horaria del Date (no UTC). Si el Date es UTC, emite "Z". */
export function toRFC3339WithLocalOffset(d: Date): string {
  const pad = (n: number) => String(n).padStart(2, '0')
  const yyyy = d.getFullYear()
  const mm = pad(d.getMonth() + 1)
  const dd = pad(d.getDate())
  const hh = pad(d.getHours())
  const mi = pad(d.getMinutes())
  const ss = pad(d.getSeconds())
  const tzOffsetMin = -d.getTimezoneOffset() // minutos al este de UTC
  if (tzOffsetMin === 0) {
    return `${yyyy}-${mm}-${dd}T${hh}:${mi}:${ss}Z`
  }
  const sign = tzOffsetMin >= 0 ? '+' : '-'
  const abs = Math.abs(tzOffsetMin)
  const oh = pad(Math.floor(abs / 60))
  const om = pad(abs % 60)
  return `${yyyy}-${mm}-${dd}T${hh}:${mi}:${ss}${sign}${oh}:${om}`
}