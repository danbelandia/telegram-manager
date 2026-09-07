// Contrato del dominio moderacion según los handlers reales del backend
// (internal/api: groups_handlers.go memberResponse,
// joinrequests_handlers.go joinRequestResponse, logs_handlers.go
// logEntryResponse). El envelope llega normalizado por lib/api-client;
// aca solo se tipa la forma del data.

/** Un miembro/administrador del grupo (memberResponse). */
export interface GroupUser {
  user_id: number
  first_name: string
  username: string | null
  status: string
  can_restrict_members: boolean | null
  can_delete_messages: boolean | null
  can_pin_messages: boolean | null
  can_invite_users: boolean | null
}

/** Solicitud de ingreso a un grupo (joinRequestResponse). */
export interface JoinRequest {
  id: number
  group_id: number
  user_id: number
  first_name: string
  username: string | null
  status: 'pending' | 'approved' | 'rejected'
  requested_at: string
  decided_at: string | null
  decided_by: number | null
}

/** Entrada de auditoria de un grupo (logEntryResponse). */
export interface LogEntry {
  id: number
  actor_id: number | null
  group_id: number
  action: string
  target_user_id: number | null
  metadata: Record<string, unknown>
  status: string
  error_message: string | null
  created_at: string
}