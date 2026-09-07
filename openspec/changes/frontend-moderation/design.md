# Design: Frontend de moderación de grupos

## Technical Approach

Consumir la API de moderación ya existente del backend (specs
group-administration, join-requests, admin-logs) desde el panel, con el
mismo patrón de `features/groups`: `types.ts` (DTO reales) + `api.ts`
(request vía api-client) + `hooks.ts` (TanStack Query: `useQuery` para
lecturas, `useMutation` + `invalidateQueries` para acciones). Las tres
páginas placeholder se convierten en vistas reales y el detalle suma
lock/unlock y delete/pin. Confirmación nativa para acciones destructivas
(§4). Errores §18 legibles mostrando `ApiError.message` del backend (ya
trae texto comprensible, ej. "el bot no tiene permisos suficientes").

## Architecture Decisions

| Decisión | Opciones | Tradeoff | Decisión |
|----------|----------|----------|----------|
| D1: Ubicación de types/API | En `features/groups/` vs `features/moderation/` | groups ya es grande; moderation tiene identidad propia | `features/moderation/` nuevo (D3 del cambio previo: módulos separados) |
| D2: Mutaciones pesadas | TanStack Query vs fetch manual en página | Query: invalidation automática, estado pending | `useMutation` de TanStack Query |
| D3: Mensajes de error | Mapear code→texto en frontend vs mostrar `ApiError.message` | El backend ya envía texto legible §18 | Mostrar `ApiError.message` directo; helper mínimo solo para fallback |
| D4: Confirmación | Modal propio vs `window.confirm` | Modal = más UI; confirm nativo = simple, consistente con §4 | `window.confirm` (testeable con stub) |
| D5: Query keys | `['users', id]` vs `['groups', id, 'users']` | Anidado permite invalidar todo el grupo de una vez | `['groups', id, 'users'|'requests'|'logs']` |
| D6: `messageId` | Input manual vs listado de mensajes | No existe listado en Bot API (§8) | Input numérico manual en el detalle |
| D7: Duración ban/mute | Input `until_date` vs indefinido | Backend soporta ambos; input infla UI | Indefinido (decisión usuario): confirmar sin duración |

## Data Flow

    GroupUsersPage ── useQuery(['groups', id, 'users']) ──→ GET /groups/:id/users
          │  [Banear] ── confirm ── useMutation ──→ POST .../users/:id/ban
          │                    └─ invalidate users + logs
          ▼
    GroupRequestsPage ── useQuery(['groups', id, 'requests']) ──→ GET .../join-requests
          │  [Aprobar/Rechazar] ── useMutation ──→ POST .../join-requests/:rid/approve|reject
          │                    └─ invalidate requests + logs
          ▼
    GroupLogsPage ── useQuery(['groups', id, 'logs']) ──→ GET /groups/:id/logs
          │
    GroupDetailPage ── [lock/unlock] ──useMutation──→ POST /groups/:id/lock|unlock
          │              [delete/pin + messageId] ──→ POST .../messages/:mid/delete|pin
          └─ invalidate ['groups', id] + logs

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `frontend/src/features/moderation/types.ts` | Create | `GroupUser`, `JoinRequest`, `LogEntry` (DTO reales) |
| `frontend/src/features/moderation/api.ts` | Create | listGroupUsers, getGroupUser, ban/unban/mute/unmute, deleteMessage, pinMessage, lock, unlock, listJoinRequests, approve, reject, listGroupLogs |
| `frontend/src/features/moderation/hooks.ts` | Create | useGroupUsers, useJoinRequests, useGroupLogs + useMutations con invalidate |
| `frontend/src/features/moderation/error.ts` | Create | `formatModerationError` (fallback legible) |
| `frontend/src/pages/GroupUsersPage.tsx` | Modify | placeholder → vista real + acciones con confirm |
| `frontend/src/pages/GroupRequestsPage.tsx` | Modify | placeholder → vista real |
| `frontend/src/pages/GroupLogsPage.tsx` | Modify | placeholder → vista real |
| `frontend/src/pages/GroupDetailPage.tsx` | Modify | + lock/unlock + delete/pin (messageId input) |
| `frontend/src/pages/GroupUsersPage.test.tsx` | Create | tests con mockFetchRoutes |
| `frontend/src/pages/GroupDetailPage.test.tsx` | Create | tests pendientes + nuevas acciones |
| `frontend/src/pages/GroupRequestsPage.test.tsx` | Create | tests |
| `frontend/src/pages/GroupLogsPage.test.tsx` | Create | tests |
| `frontend/src/styles.css` | Modify | tablas/estados/acciones mínimas |
| `docs/frontend-react-skill.md` | Modify | patrón useMutation + invalidate |

## Interfaces / Contracts

```ts
// features/moderation/types.ts — DTOs reales (backend internal/api)
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
```

Key mutation signature: `useBanUser()` retorna `{ mutate, isPending }`;
`mutate({ groupId, userId, untilDate?, revokeMessages? })`. Todas las
mutaciones invalidan `['groups', id, 'logs']`; las de usuarios además
`['groups', id, 'users']`; lock/unlock además `['groups', id]`;
approve/reject además `['groups', id, 'requests']`.

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| Unit (páginas) | Render de vistas + acciones | `mockFetchRoutes` por URL + `window.confirm` stub; verificar body del POST y invalidación |
| Unit (api) | Endpoints mapean a request | directo con mock fetch |

Confirm en tests: `vi.stubGlobal('confirm', vi.fn(() => true))`; el caso
"cancelado" con `() => false` verifica que NO se llama a fetch.

## Migration / Rollout

No migration required (solo frontend). Rama: crear
`feat/frontend-moderation` desde `feat/frontend-panel-dashboard`, commit
por unidad de trabajo, PR encadenado.

## Open Questions

- Ninguna (D7 resuelto por el usuario: ban/mute indefinidos, sin input de duración).