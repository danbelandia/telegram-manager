// Contrato del dominio moderacion automatica (Fase 3, slice 2) segun
// backend/internal/api/automation_handlers.go. El envelope llega
// normalizado por lib/api-client; aca solo se tipa la forma del data.
//
// Las settings son toggles + thresholds; las listas (banned_words y
// link_allowlist) son strings. El panel las edita en una sola pagina
// (GroupAutomationPage) con Save all paralelo (Promise.all).

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
