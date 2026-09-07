# Tasks: rest-api — API REST de administración de grupos

## Review Workload Forecast

| Campo | Valor |
|-------|-------|
| Líneas estimadas | ~1700-2100 (12 archivos nuevos, 7 modificados, migración, tests) |
| Riesgo budget 400 líneas | High |
| PRs encadenados recomendados | Yes |
| Split sugerido | PR1 telegram → PR2 datos+dominios → PR3 API+wiring |
| Delivery strategy | ask-on-risk |
| Chain strategy | feature-branch-chain (misma chain; aún sin remote) |

```text
Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: feature-branch-chain
400-line budget risk: High
```

**Nota**: approve Y reject requieren `can_invite_users` (referencia §7; el
design decía `can_restrict_members` para reject — se corrige aquí).
`unlock` = `setChatPermissions` con `can_send_messages: true` + flags de
envío estándar en true.

### Work Units

| Unit | Goal | PR | Base | Verification |
|------|------|----|------|--------------|
| 1 | telegram: interfaz+adapter (doPost, token bucket, errores) | PR1 | feat/rest-api | suite telegram verde |
| 2 | migración + dominios users/logs/joinrequests/moderation | PR2 | PR1 | tests repos + moderation |
| 3 | handlers §12 + options + wiring main | PR3 | PR2 | tests handlers + E2E |

## Phase 1: Telegram core (PR1)

- [x] 1.1 Ampliar `internal/telegram/service.go`: +12 métodos en `Service`; +errors `ErrPermissionDenied`, `ErrTelegramNotFound`, `ErrTelegramAPI{Code,Description}`
- [x] 1.2 `internal/telegram/adapter.go`: +`doPost(ctx, method, bodyJSON, result)` reutilizando envelope/mapeo
- [x] 1.3 `adapter.go`: rate limiter token bucket (~25 req/s, `wait(ctx)` bloqueante con mutex) — en `rate_limiter.go`
- [x] 1.4 `adapter.go`: reintento 429 con `retry_after` y max 3 en `doGetQuery`/`doPost` (por llamada de moderación)
- [x] 1.5 `adapter.go`: mapeo errores (403→`ErrPermissionDenied`, 404→`ErrTelegramNotFound`; 400 clasificado por description: "not found"→`ErrTelegramNotFound`, "rights"/"permission"→`ErrPermissionDenied`, resto→`ErrTelegramAPI`; 401→`ErrInvalidToken`, 409→`ErrWebhookConflict`, 429→`RateLimitError`)
- [x] 1.6 `internal/telegram/model.go`: ~~+`ChatUserInfo`~~ — **desviación documentada**: se reutiliza `ChatMember` (update.go, ya tiene Status+User+can_*) en vez de crear `ChatUserInfo`; toca update.go +`CanSendMessages`
- [x] 1.7 `adapter_test.go` + `moderation_test.go`: tests doPost (payload JSON, método POST), token bucket, reintento 429 (éxito tras retry_after, sin retry_after no reintenta, agotamiento), mapeo 400/403/404, decode getChatMember/getChatAdministrators
- [x] 1.8 `poller_test.go`: actualizar `fakeService` con los 12 métodos
- [x] 1.9 Verificar: `go test ./internal/telegram/...` verde + gofmt/vet

## Phase 2: Datos + dominios (PR2)

- [x] 2.1 Migración `00003_create_moderation_tables.sql`: users, join_requests (+idx group_id/status, único parcial pending), warnings, logs (+idx group_id); Down DROP
- [x] 2.2 `internal/users/{model,repository}.go`: UpsertByTelegramID, GetByTelegramID
- [x] 2.3 `internal/logs/{model,repository}.go`: Entry (actor_id, group_id, action, target_user_id, metadata, status, error_message, created_at), Create, ListByGroup — **nota**: se añadió `requested_at`/`decided_at`/`decided_by` según spec join-requests
- [x] 2.4 `internal/joinrequests/{model,repository}.go`: upsert pending (user+group) vía índice parcial único, ListByGroup, GetByID, Resolve(status, decided_by)
- [x] 2.5 `internal/joinrequests/events.go`: `HandleChatJoinRequest(ctx, upd, *Request)` pura (patrón groups.HandleMyChatMember); el bus persiste con UpsertPending
- [x] 2.6 `internal/moderation/permissions.go`: mapa acción→bot_permission (ban/unban/mute/unmute/lock/unlock→can_restrict_members; delete→can_delete_messages; pin→can_pin_messages; approve/reject→can_invite_users)
- [x] 2.7 `internal/moderation/service.go`: flujo por acción (grupo existe→permiso→telegram→log); interfaces consumidoras `GroupPermissionReader`, `TelegramActions`, `RequestStore`, `LogWriter`; errores mapeados con `%w` (errors.Is)
- [x] 2.8 Tests repos: OpenTestDB suite **por paquete** (`users`/`logs`/`joinrequests`) — **desviación documentada**: el task pedía suite `rest` única, pero la convención del proyecto (testdb.go) exige una base por paquete para `go test ./...` en paralelo (goose se pisa en base compartida)
- [x] 2.9 Tests moderation service con fake de telegram (éxito, PERMISSION_DENIED sin llamar, NOT_FOUND, log en fallo, approve/reject: ya decidida 409, otro grupo 404, error de Telegram deja pending)
- [x] 2.10 Test events: chat_join_request→pending, upsert no duplica (índice parcial), re-solicitud tras rechazo crea fila nueva

## Phase 3: API + wiring (PR3)

- [x] 3.1 `internal/api/server.go`: +options `WithGroups`, `WithModeration`, `WithJoinRequests`, **+`WithLogs`** (no estaba en el task: GET /groups/:id/logs del §12 necesita su option; se añadió por consistencia) que montan rutas en mux
- [x] 3.2 `internal/api/groups_handlers.go`: GET /api/groups, GET /api/groups/:id (404 NOT_FOUND), GET /api/groups/:id/users (?userId= lookup | admins)
- [x] 3.3 `internal/api/moderation_handlers.go`: POST ban/unban/mute/unmute, delete/pin, lock/unlock (params int64→VALIDATION_ERROR; codes §18). Body opcional para ban (untilDate/revokeMessages) y mute (untilDate) — ban sin body es indefinido con revocación (default Telegram)
- [x] 3.4 `internal/api/joinrequest_handlers.go`: GET join-requests; POST approve/reject (404 inexistente, 409 ya decidida, TELEGRAM_ERROR)
- [x] 3.5 `cmd/server/main.go`: wiring (repos nuevos, moderationSvc, handler bus chat_join_request que registra usuario en users + solicitud pending, options) en webhook y polling
- [x] 3.6 Tests handlers (`moderation_api_test.go`) — **desviación documentada**: el task pedía DB real OpenTestDB "rest" + fake telegram; se usaron **fakes puros** (authenticator fijo, groupStore/groupUsers/moderation/joinRequests/logStore stubs) porque los handlers no tocan DB directamente y el middleware se satisface con una identidad fija. Tests: 401 sin token (15 rutas), actor+ids pasados al servicio, mapeo de errores §18 (404/403/502/409), IDs inválidos→400, body inválido→400 sin llamar al servicio, data de listados (groups/join-requests/logs/users lookup)
- [ ] 3.7 E2E manual: login → GET /groups → ban/unban/mute/unmute/delete/pin/lock/unlock/approve/reject contra grupo de prueba; verificar logs por acción
- [x] 3.8 Suite completa `go test ./...` verde; gofmt/vet limpio (verificado antes del commit b153e72^..HEAD)

## Phase 4: Cleanup / Docs

- [ ] 4.1 Actualizar `docs/telegram_api_reference.md` si algún detalle de permiso difiere de lo implementado
- [ ] 4.2 Documentar limitación `group_members` (spec group-administration ya cubre) y confirmar en README/spec de archive
- [ ] 4.3 Actualizar `openspec/changes/rest-api/state.yaml` (estado final por fase)