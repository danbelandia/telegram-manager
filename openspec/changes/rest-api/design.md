# Design: rest-api — API REST de administración de grupos

## Technical Approach

Ampliar la capa `telegram` (Service + Adapter con doPost, rate limiter
token bucket y errores tipados), crear los dominios `users`, `logs`,
`joinrequests` y `moderation` sobre una migración nueva, y montar las
rutas §12 como nuevas `Option`s del `api.Server` (patrón `WithAuth`).
El flujo por acción vive en `moderation.Service`: validar grupo →
chequear `bot_permissions` → llamar Telegram → registrar log.

## Architecture Decisions

| # | Decisión | Alternativas | Rationale |
|---|----------|-------------|-----------|
| D1 | Ampliar UNA interfaz `telegram.Service` con los 12 métodos nuevos | Dos interfaces (dev+moderation) | §15 muestra un solo `TelegramService`; consumidores tipan la interfaz mínima que necesiten |
| D2 | Nuevo método `doPost(ctx, method, body, result)` en Adapter; reutiliza envelope/errores de `doGetQuery` | Extender doGetQuery | ChatPermissions anidado no es seguro en query; POST+JSON es el estándar de la Bot API |
| D3 | Rate limiter token bucket síncrono (`wait(ctx)` bloqueante) DENTRO del Adapter | Worker canal (spec) | MVP no tiene API de lote (N acciones = N requests del panel); el wait serializa a ~25 req/s. Worker se añade si surge operación batch real |
| D4 | Reintento 429 dentro del adapter con retry_after y max 3 (configurable) | Solo devolver RateLimitError | §18.1 exige reintentar respetando retry_after, no devolver el error de una |
| D5 | Errores tipados nuevos: `ErrPermissionDenied`, `ErrTelegramNotFound`, `ErrTelegramAPI{Code,Description}` | Seguir con error genérico | §18 y skill §2 exigen dominio tipado; el mapeo vive en el adapter |
| D6 | `users` tabla con telegram_id; `join_requests.user_id` = telegram id | FK a users.id | Telegram es fuente de verdad; simplifica el flujo del evento sin joins |
| D7 | Autenticación por ruta = `requireAuth`; autorización por grupo = `moderation.Service` | En el middleware | Skill §7: middleware solo identidad; autorización es capa de servicio |
| D8 | Handler `GET /groups/:id/users` usa `getChatAdministrators`+`getChatMember` sobre el Service de Telegram | Tabla group_members | P1: Telegram no lista miembros; P2: lookup puntual + admins |
| D9 | `requestId` de rutas = `join_requests.id` (BIGSERIAL) | telegram user_id | El evento chat_join_request no trae id propio; la fila es la fuente |
| D10 | Todos los ids en path son int64; parseo con strconv → `VALIDATION_ERROR` | Router params string | Consistente con el resto y con telegram ids |

## Data Flow

    Panel (React, paso 11)
      │  POST /api/groups/:id/users/:userId/ban  (Bearer access)
      ▼
    api.Server ── requireAuth (identidad) ──> moderation_handlers
      │  VALIDATION_ERROR si params inválidos
      ▼
    moderation.Service
      1. groupsRepo.GetByTelegramID(:id)      → NOT_FOUND si no existe
      2. chequear bot_permissions[key]→false  → PERMISSION_DENIED (sin llamar a Telegram)
      3. telegramSvc.BanUser(...)              → tipo de error dominio (PERMISSION_DENIED/NOT_FOUND/TELEGRAM_ERROR)
      4. logsRepo.Create(action, status,...)   → siempre se registra
      ▼
    telegram.Adapter ── rate limiter token bucket (wait) ── doPost ──> Bot API

Webhook/polling: `chat_join_request` → bus → handler `joinrequests.HandleChatJoinRequest` → repo upsert pending.

## File Changes

| File | Acción | Descripción |
|------|--------|-------------|
| `backend/internal/telegram/service.go` | Modify | +12 métodos en Service; +3 errores; RateLimitError se mantiene |
| `backend/internal/telegram/adapter.go` | Modify | +doPost, +rate limiter, +mapeo 400/403/404, +reintento 429 |
| `backend/internal/telegram/model.go` | Modify | +ChatUserInfo/resultados de getChatMember |
| `backend/migrations/00003_create_moderation_tables.sql` | Create | users, join_requests, warnings, logs (+indices) |
| `backend/internal/users/{model,repository}.go` | Create | UpsertByTelegramID, GetByTelegramID |
| `backend/internal/logs/{model,repository}.go` | Create | Create, ListByGroup |
| `backend/internal/joinrequests/{model,repository,events}.go` | Create | upsert pending, ListByGroup, Resolve |
| `backend/internal/moderation/{service,permissions}.go` | Create | flujo acción→log; mapa acción→permiso bot |
| `backend/internal/api/{groups_handlers,moderation_handlers,joinrequest_handlers}.go` | Create | rutas §12 |
| `backend/internal/api/server.go` | Modify | +WithGroups, +WithModeration, +WithJoinRequests |
| `backend/cmd/server/main.go` | Modify | wiring repos/servicios/options + handler bus chat_join_request |
| `backend/internal/telegram/poller_test.go` | Modify | fakeService implementa los 12 métodos nuevos |
| tests nuevos | Create | adapter doPost/rate/errores; users/logs/joinrequests repos; moderation service; handlers |

## Interfaces / Contracts

```go
// telegram.Service (ampliación — firmas)
BanUser(ctx context.Context, chatID, userID int64, untilDate int64, revokeMessages bool) error
UnbanUser(ctx context.Context, chatID, userID int64) error
MuteUser(ctx context.Context, chatID, userID int64, untilDate int64) error
UnmuteUser(ctx context.Context, chatID, userID int64) error
DeleteMessage(ctx context.Context, chatID, messageID int64) error
PinMessage(ctx context.Context, chatID, messageID int64) error
LockGroup(ctx context.Context, chatID int64) error
UnlockGroup(ctx context.Context, chatID int64) error
GetChatMember(ctx context.Context, chatID, userID int64) (ChatUserInfo, error)
GetChatAdministrators(ctx context.Context, chatID int64) ([]ChatUserInfo, error)
ApproveJoinRequest(ctx context.Context, chatID, userID int64) error
RejectJoinRequest(ctx context.Context, chatID, userID int64) error

// moderation.Service (flujo por acción)
Ban(ctx, actor *auth.Claims, groupID, userID int64) error   // +Unban/Mute/Unmute/Lock/Unlock
DeleteMessage(ctx, actor, groupID, messageID int64) error   // +Pin
Approve(ctx, actor, groupID, requestID int64) error         // +Reject (via joinrequests)

// asegura: grupo existe → permiso bot → telegram → log
type GroupPermissionReader interface {   // grupos repo (lado consumidor)
    GetByTelegramID(ctx context.Context, id int64) (*groups.Group, error)
}
type LogWriter interface { Create(ctx context.Context, e *logs.Entry) error }
```

Permisos por acción (mapa `actionsRequiring` en moderation):

| Acción | bot_permission |
|--------|----------------|
| ban / unban / mute / unmute / lock / unlock | `can_restrict_members` |
| delete | `can_delete_messages` |
| pin | `can_pin_messages` |
| join request approve | `can_invite_users` |
| join request reject | `can_restrict_members` |

Métodos de la Bot API: `banChatMember`, `unbanChatMember`,
`restrictChatMember` (permissions+until_date), `deleteMessage`,
`pinChatMessage`, `setChatPermissions`, `approveChatJoinRequest`,
`declineChatJoinRequest`, `getChatMember`, `getChatAdministrators`.
Mute = restrictChatMember con fechas nil? no: `untilDate=0` = indefinido;
permissions completas false. Unmute = permissions true. Lock = permisos
grupo con can_send_messages false; Unlock = true.

## Testing Strategy

| Capa | Qué | Cómo |
|------|-----|------|
| Unit | adapter doPost, mapeo errores, token bucket, reintento 429 | stub HTTP (`newBotStubServer`) |
| Unit | moderation service: flujo completo con mock de telegram | fake de interfaz telegram (a mano) |
| Unit | evento chat_join_request → pending; Resolve | fake bus + repo real OpenTestDB("rest") |
| Integration | repos users/logs/joinrequests | OpenTestDB suite `rest` |
| Integration | handlers §12 (con DB + fake telegram) | patrón auth_api_test (httptest real) |
| E2E manual | acciones contra grupo de prueba | curl + verificación de logs |

## Migration / Rollout

- goose Up `00003` automático en dev (RunMigrations). Down: DROP
  tablas nuevas (vacías al deploy inicial — sin pérdida).
- Sin feature flags. PRs encadenados por domino (ver tasks).

## Open Questions

- [ ] Confirmar permisos exactos de `declineChatJoinRequest` y
      `approveChatJoinRequest` en la referencia (can_restrict_members /
      can_invite_users) antes de fijar el mapa.
- [ ] En `SetChatPermissions` de unlock: ¿restaurar solo
      can_send_messages o el set completo de defaults?