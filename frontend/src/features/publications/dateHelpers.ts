// Helpers para convertir entre Date y el formato "YYYY-MM-DDTHH:MM"
// que usan los inputs datetime-local y validateScheduledAtClient.

/** Date -> string "YYYY-MM-DDTHH:MM" (local timezone). */
export function dateToLocalInput(d: Date): string {
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

/** string "YYYY-MM-DDTHH:MM" | null -> Date | null. */
export function localInputToDate(s: string | null): Date | null {
  if (!s || !s.trim()) return null
  const d = new Date(s)
  return Number.isNaN(d.getTime()) ? null : d
}
