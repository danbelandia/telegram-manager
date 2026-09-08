// Contrato del dominio moderacion automatica (Fase 3, slice 2) segun
// backend/internal/api/automation_handlers.go. El envelope llega
// normalizado por lib/api-client; aca solo se tipa la forma del data.
//
// Las settings son toggles + thresholds; las listas (banned_words y
// link_allowlist) son strings. El panel las edita en una sola pagina
// (GroupAutomationPage) con Save all paralelo (Promise.all).
//
// Slice 2.1 (warning visual al usuario, REQ-22/29): agrega
// warn_user_enabled (toggle) + warn_user_template (string|null, override
// per-grupo del default hardcoded en Go).

/** Settings de moderacion automatica para un grupo. */
export interface AutomationSettings {
  group_id: number
  enabled: boolean
  anti_spam_enabled: boolean
  anti_link_enabled: boolean
  banned_words_enabled: boolean
  flood_enabled: boolean
  flood_messages: number
  flood_seconds: number
  warning_limit: number
  automute_warnings: number
  automute_minutes: number
  autoban_warnings: number
  warning_expire_days: number
  /** Slice 2.1: si true, el bot envia un mensaje al usuario antes de
   * mute/ban. Default true. */
  warn_user_enabled: boolean
  /** Slice 2.1: plantilla custom (null = usar default del backend).
   * Variables: {nombre}, {count}, {mute_minutes}. Max 1000 chars
   * (validado server-side). */
  warn_user_template: string | null
  /** ISO8601 con milisegundos. */
  updated_at: string
}

/** Body parcial de PUT /automation/settings: todos los campos son
 * opcionales. Si un campo viene, se pisa; si no, se conserva el valor
 * previo. El handler hace merge con los defaults del backend. */
export interface AutomationSettingsUpdate {
  enabled?: boolean
  anti_spam_enabled?: boolean
  anti_link_enabled?: boolean
  banned_words_enabled?: boolean
  flood_enabled?: boolean
  flood_messages?: number
  flood_seconds?: number
  warning_limit?: number
  automute_warnings?: number
  automute_minutes?: number
  autoban_warnings?: number
  warning_expire_days?: number
  /** Slice 2.1: ambos opcionales. Si warn_user_template viene como
   * string vacio, el backend lo acepta (y al renderear cae al
   * default hardcoded via templates.go). */
  warn_user_enabled?: boolean
  warn_user_template?: string | null
}

/** Defaults que el backend retorna cuando la fila no existe (GET
 * inicial). Coinciden con automation.DefaultSettings en Go. */
export const AUTOMATION_DEFAULTS: Omit<AutomationSettings, 'group_id' | 'updated_at'> = {
  enabled: false,
  anti_spam_enabled: false,
  anti_link_enabled: false,
  banned_words_enabled: false,
  flood_enabled: false,
  flood_messages: 5,
  flood_seconds: 10,
  warning_limit: 3,
  automute_warnings: 3,
  automute_minutes: 10,
  autoban_warnings: 5,
  warning_expire_days: 30,
  // Slice 2.1: warning visual ON por default (out-of-the-box).
  warn_user_enabled: true,
  warn_user_template: null,
}

/** Body de POST /automation/banned-words. */
export interface BannedWordRequest {
  word: string
}

/** Body de POST /automation/link-allowlist. */
export interface LinkAllowlistRequest {
  domain: string
}

/** Respuesta de GET/POST/DELETE /automation/banned-words. */
export interface BannedWordsResponse {
  words: string[]
}

/** Respuesta de GET/POST/DELETE /automation/link-allowlist. */
export interface LinkAllowlistResponse {
  domains: string[]
}

// ── Slice 3: Warnings Dashboard ──────────────────────────────────────

/** Periodo del selector de estadisticas (24h default si falta). */
export type StatsPeriod = '24h' | '7d'

/** Conteos agregados de auto-moderacion en la ventana seleccionada. */
export interface AutomationStats {
  rule_triggered: number
  automute: number
  autoban: number
  period: StatsPeriod
}

/** Una fila del dashboard de advertencias activas. display_name
 * viene computado server-side (backend WarningStateRow.DisplayName):
 * FirstName → @username → "user {user_id}". last_warning_at /
 * last_action_at / expires_at son ISO8601 con milisegundos o null. */
export interface WarningStateRow {
  user_id: number
  display_name: string
  username: string | null
  warning_count: number
  last_warning_at: string | null
  last_action_at: string | null
  expires_at: string | null
}

/** Respuesta de GET /automation/warnings. `truncated` indica si el
 * resultset alcanzo el cap defensivo (top 100) y hay mas filas que
 * el frontend no esta viendo. */
export interface WarningsResponse {
  warnings: WarningStateRow[]
  truncated: boolean
}

/** Body de POST /automation/warnings/{user_id}/reset (200 OK). */
export interface ResetWarningResponse {
  user_id: number
  warning_count: 0
  reset: true
}
