// Llamadas HTTP del dominio moderacion (guia frontend seccion 3). Todo
// pasa por lib/api-client.ts, que normaliza el envelope del backend y
// agrega el access token. Los paths siguen AGENTS 12 / server.go.
import { request } from '../../lib/api-client'
import type { GroupUser, JoinRequest, LogEntry } from './types'

// ── Usuarios ──────────────────────────────────────────────────────────

/** GET /api/groups/:id/users — administradores del grupo. */
export function listGroupUsers(groupId: string): Promise<GroupUser[]> {
  return request<GroupUser[]>(`/api/groups/${groupId}/users`)
}

/** GET /api/groups/:id/users?userId= — lookup puntual de un miembro. */
export function getGroupUser(groupId: string, userId: string): Promise<GroupUser> {
  return request<GroupUser>(`/api/groups/${groupId}/users?userId=${userId}`)
}

/** POST .../users/:userId/ban — baneo indefinido con revocacion de mensajes. */
export function banUser(groupId: string, userId: number): Promise<{ status: string }> {
  return request<{ status: string }>(`/api/groups/${groupId}/users/${userId}/ban`, {
    method: 'POST',
    body: JSON.stringify({ revoke_messages: true }),
  })
}

/** POST .../users/:userId/unban. */
export function unbanUser(groupId: string, userId: number): Promise<{ status: string }> {
  return request<{ status: string }>(`/api/groups/${groupId}/users/${userId}/unban`, {
    method: 'POST',
  })
}

/** POST .../users/:userId/mute — mute indefinido (decisión D7). */
export function muteUser(groupId: string, userId: number): Promise<{ status: string }> {
  return request<{ status: string }>(`/api/groups/${groupId}/users/${userId}/mute`, {
    method: 'POST',
  })
}

/** POST .../users/:userId/unmute. */
export function unmuteUser(groupId: string, userId: number): Promise<{ status: string }> {
  return request<{ status: string }>(`/api/groups/${groupId}/users/${userId}/unmute`, {
    method: 'POST',
  })
}

// ── Mensajes ──────────────────────────────────────────────────────────

/** POST .../messages/:messageId/delete. */
export function deleteMessage(groupId: string, messageId: number): Promise<{ status: string }> {
  return request<{ status: string }>(`/api/groups/${groupId}/messages/${messageId}/delete`, {
    method: 'POST',
  })
}

/** POST .../messages/:messageId/pin. */
export function pinMessage(groupId: string, messageId: number): Promise<{ status: string }> {
  return request<{ status: string }>(`/api/groups/${groupId}/messages/${messageId}/pin`, {
    method: 'POST',
  })
}

// ── Chat ──────────────────────────────────────────────────────────────

/** POST /api/groups/:id/lock — cierra el envio de mensajes (AGENTS 9). */
export function lockGroup(groupId: string): Promise<{ status: string }> {
  return request<{ status: string }>(`/api/groups/${groupId}/lock`, { method: 'POST' })
}

/** POST /api/groups/:id/unlock — abre el envio de mensajes. */
export function unlockGroup(groupId: string): Promise<{ status: string }> {
  return request<{ status: string }>(`/api/groups/${groupId}/unlock`, { method: 'POST' })
}

// ── Solicitudes de ingreso ────────────────────────────────────────────

/** GET /api/groups/:id/join-requests. */
export function listJoinRequests(groupId: string): Promise<JoinRequest[]> {
  return request<JoinRequest[]>(`/api/groups/${groupId}/join-requests`)
}

/** POST .../join-requests/:requestId/approve. */
export function approveJoinRequest(groupId: string, requestId: number): Promise<{ status: string }> {
  return request<{ status: string }>(`/api/groups/${groupId}/join-requests/${requestId}/approve`, {
    method: 'POST',
  })
}

/** POST .../join-requests/:requestId/reject. */
export function rejectJoinRequest(groupId: string, requestId: number): Promise<{ status: string }> {
  return request<{ status: string }>(`/api/groups/${groupId}/join-requests/${requestId}/reject`, {
    method: 'POST',
  })
}

/** Resultado individual de un batch approve/reject. */
export interface BatchJoinResultItem {
  id: number
  status: string
  error?: string | null
}

/** POST .../join-requests/batch — batch approve/reject. */
export function batchDecideJoinRequests(
  groupId: string,
  action: 'approve' | 'reject',
  requestIds: number[],
): Promise<{ results: BatchJoinResultItem[] }> {
  return request<{ results: BatchJoinResultItem[] }>(`/api/groups/${groupId}/join-requests/batch`, {
    method: 'POST',
    body: JSON.stringify({ action, request_ids: requestIds }),
  })
}

// ── Logs ──────────────────────────────────────────────────────────────

/** GET /api/groups/:id/logs — auditoria del grupo, mas reciente primero. */
export function listGroupLogs(groupId: string): Promise<LogEntry[]> {
  return request<LogEntry[]>(`/api/groups/${groupId}/logs`)
}