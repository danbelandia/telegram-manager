// Contrato del grupo segun groups_handlers.go (verificado en la capa
// REST del backend). El envelope del backend llega normalizado por
// lib/api-client.ts, aca solo se tipa la forma del data.
export interface Group {
  id: string
  telegram_id: number
  title: string
  username: string | null
  type: string
  member_count: number | null
  bot_status: string
  bot_permissions: Record<string, boolean>
}