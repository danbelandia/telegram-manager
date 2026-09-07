# Proposal: rest-api — Paso 10: API REST de administración de grupos

## Intent

Resolver el gap del MVP: hoy el backend solo registra grupos y autentica
al admin, pero no expone API REST para administrarlos (§12). Falta
también la capa de moderación sobre la Bot API (§15) con rate limiter
(§18.1) y el registro de acciones (§11). Este cambio habilita el panel
React (paso 11) para ejecutar ban/unban/mute/unmute, borrar/fijar
mensajes, abrir/cerrar chat y gestionar solicitudes de ingreso, con
autorización por grupo y logs de auditoría.

## Scope

### In Scope
- Ampliar `telegram.Service`/`Adapter`: 10 métodos de moderación (§15:
  ban/unban/mute/unmute, delete/pin message, lock/unlock,
  approve/reject join request) + `GetChatMember` + `GetChatAdministrators`.
- `doPost` (JSON body) en el adapter (ChatPermissions no cabe en query).
- Rate limiter token bucket (~25 req/s, §18.1) + reintentos 429 con
  `retry_after`, max 3, dentro del adapter.
- Errores de dominio tipados en el adapter: `ErrPermissionDenied`,
  `ErrTelegramNotFound`, genérico `ErrTelegramAPI` (mapeo 400/403/404).
- Migración `00003`: tablas `users`, `join_requests`, `warnings`, `logs`.
- Dominios: `internal/logs`, `internal/users`, `internal/joinrequests`,
  `internal/moderation`.
- Procesamiento del evento `chat_join_request` en el bus (fila pending).
- Endpoints REST §12 con `requireAuth` + autorización por grupo
  (verifica `bot_permissions` JSONB del grupo antes de ejecutar).
- Registro de cada acción administrativa en `logs` (§11).

### Out of Scope (documentar, no implementar)
- `group_members` (P1): Telegram no permite listar miembros; se omite y
  se documenta como limitación (§25.17).
- Búsqueda de usuarios por nombre/username: solo lookup puntual
  `getChatMember` y listado de admins `getChatAdministrators` (P2).
- Warnings de moderación automática (Fase 3), publicaciones (Fase 2).
- Frontend (paso 11+).

## Capabilities

### New Capabilities
- `telegram-moderation`: Service+Adapter ampliados, doPost, rate
  limiter token bucket, errores de dominio tipados.
- `admin-logs`: modelo `logs` (§11), repo, acciones registradas.
- `group-administration`: endpoints REST (§12) de grupos/users/
  moderación con autorización por `bot_permissions` y manejo de errores
  con códigos §18.
- `join-requests`: tabla `join_requests`, procesamiento del evento
  `chat_join_request`, endpoints approve/reject con logs.

### Modified Capabilities
- None (capabilities previas no cambian a nivel spec).

## Approach

1. **telegram**: ampliar interfaz + adapter (`doPost`, token bucket,
   mapeo de errores) con tests puros (stub HTTP, sin Bot API real).
2. **datos**: migración 00003 + repos de logs/users/joinrequests
   (patrón `OpenTestDB` por suite).
3. **dominios**: `joinrequests` (evento `chat_join_request` → pending;
   approve/reject) y `moderation` (valida grupo + bot_permissions →
   ejecuta Service → escribe log).
4. **api**: `WithModeration(...)` monta rutas §12; servicios y handlers
   separados; envelope con códigos §18.
5. **main**: wiring de deps, repo de grupos reutilizado.

## Affected Areas

| Area | Impact | Descripción |
|------|--------|-------------|
| `backend/internal/telegram/service.go` | Modified | interfaz + errores |
| `backend/internal/telegram/adapter.go` | Modified | doPost, rate limiter, mapeo |
| `backend/internal/telegram/{model,update}.go` | Modified | tipos auxiliares |
| `backend/migrations/00003_*.sql` | New | users/join_requests/warnings/logs |
| `backend/internal/{logs,users,joinrequests,moderation}/` | New | dominios |
| `backend/internal/api/` | Modified | handlers/moderation.go + rutas |
| `backend/cmd/server/main.go` | Modified | wiring |
| `openspec/specs/{4 caps}/spec.md` | New (archive) | specs base |

## Risks

| Riesgo | Prob. | Mitigación |
|--------|-------|------------|
| Ampliar interfaz Service rompe fakeService del poller | Alta | actualizar fakeService en el mismo PR |
| Rate limiter insuficiente → 429 real | Media | token bucket + retry_after max 3 (§18.1) |
| Bot sin permiso → error de Telegram inesperado | Media | validar bot_permissions antes; mapear 403 |
| Migración irreversible con datos | Baja | goose Down documentado; tablas nuevas vacías |
| Cambio enorme (4 capabilities) | Alta | PRs encadenados por work unit (guard §E) |

## Rollback Plan

- Revert por PR encadenado (chain `feat/rest-api`); cada PR es
  autónomo y reversible.
- Migración: `goose down` de `00003` (DROP TABLE users,
  join_requests, warnings, logs) — tablas nuevas, sin pérdida de datos
  existentes.
- El webhook/polling y auth existentes no se tocan en su contrato.

## Dependencies

- `docs/telegram_api_reference.md` (parámetros exactos de la Bot API).
- `groups` repo existente (bot_permissions para autorización).

## Success Criteria

- [ ] Suite backend completa verde (units + integración por OpenTestDB).
- [ ] E2E en vivo: login → GET /api/groups → ban/unban/mute/unmute/
      delete/pin/lock/unlock/approve/reject contra grupo de prueba,
      con logs registrados por acción.
- [ ] Todos los endpoints devuelven envelope con códigos §18 y nunca
      exponen secretos.
- [ ] Los 429 se reintentan respetando retry_after (max 3) sin
      reintentos a ciegas (§18.1).
- [ ] Limitación de `group_members` documentada en el spec.