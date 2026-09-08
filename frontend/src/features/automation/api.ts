// Llamadas HTTP del dominio moderacion automatica (Fase 3, slice 2 + slice 3).
// 12 endpoints bajo /api/groups/{groupId}/automation/... (9 de slice 2
// para settings + listas + 3 de slice 3 para el dashboard: GET /warnings,
// POST /warnings/{user_id}/reset, GET /stats). Pasa por lib/api-client
// que normaliza el envelope {data, error} y maneja el refresh del access token.
import { request } from '../../lib/api-client'
import type {
  AutomationSettings,
  AutomationSettingsUpdate,
  AutomationStats,
  BannedWordRequest,
  BannedWordsResponse,
  LinkAllowlistRequest,
  LinkAllowlistResponse,
  ResetWarningResponse,
  StatsPeriod,
  WarningsResponse,
} from './types'

/** GET /api/groups/{groupId}/automation/settings */
export function getAutomationSettings(groupId: number): Promise<AutomationSettings> {
  return request<AutomationSettings>(`/api/groups/${groupId}/automation/settings`)
}

/** PUT /api/groups/{groupId}/automation/settings */
export function putAutomationSettings(
  groupId: number,
  update: AutomationSettingsUpdate,
): Promise<AutomationSettings> {
  return request<AutomationSettings>(`/api/groups/${groupId}/automation/settings`, {
    method: 'PUT',
    body: JSON.stringify(update),
  })
}

/** GET /api/groups/{groupId}/automation/banned-words */
export function listBannedWords(groupId: number): Promise<BannedWordsResponse> {
  return request<BannedWordsResponse>(`/api/groups/${groupId}/automation/banned-words`)
}

/** POST /api/groups/{groupId}/automation/banned-words */
export function addBannedWord(groupId: number, body: BannedWordRequest): Promise<BannedWordsResponse> {
  return request<BannedWordsResponse>(`/api/groups/${groupId}/automation/banned-words`, {
    method: 'POST',
    body: JSON.stringify(body),
  })
}

/** DELETE /api/groups/{groupId}/automation/banned-words/{word} */
export function removeBannedWord(groupId: number, word: string): Promise<BannedWordsResponse> {
  return request<BannedWordsResponse>(`/api/groups/${groupId}/automation/banned-words/${encodeURIComponent(word)}`, {
    method: 'DELETE',
  })
}

/** GET /api/groups/{groupId}/automation/link-allowlist */
export function listLinkAllowlist(groupId: number): Promise<LinkAllowlistResponse> {
  return request<LinkAllowlistResponse>(`/api/groups/${groupId}/automation/link-allowlist`)
}

/** POST /api/groups/{groupId}/automation/link-allowlist */
export function addAllowlistEntry(groupId: number, body: LinkAllowlistRequest): Promise<LinkAllowlistResponse> {
  return request<LinkAllowlistResponse>(`/api/groups/${groupId}/automation/link-allowlist`, {
    method: 'POST',
    body: JSON.stringify(body),
  })
}

/** DELETE /api/groups/{groupId}/automation/link-allowlist/{domain} */
export function removeAllowlistEntry(groupId: number, domain: string): Promise<LinkAllowlistResponse> {
  return request<LinkAllowlistResponse>(`/api/groups/${groupId}/automation/link-allowlist/${encodeURIComponent(domain)}`, {
    method: 'DELETE',
  })
}

// ── Slice 3: Warnings Dashboard ──────────────────────────────────────

/** GET /api/groups/{groupId}/automation/warnings */
export function listWarnings(groupId: number): Promise<WarningsResponse> {
  return request<WarningsResponse>(`/api/groups/${groupId}/automation/warnings`)
}

/** POST /api/groups/{groupId}/automation/warnings/{userId}/reset */
export function resetWarning(
  groupId: number,
  userId: number,
): Promise<ResetWarningResponse> {
  return request<ResetWarningResponse>(
    `/api/groups/${groupId}/automation/warnings/${userId}/reset`,
    { method: 'POST' },
  )
}

/** GET /api/groups/{groupId}/automation/stats?period=24h|7d */
export function getStats(groupId: number, period: StatsPeriod): Promise<AutomationStats> {
  return request<AutomationStats>(
    `/api/groups/${groupId}/automation/stats?period=${period}`,
  )
}
