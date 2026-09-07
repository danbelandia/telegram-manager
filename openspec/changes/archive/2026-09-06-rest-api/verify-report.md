# Verification Report — rest-api

**Change**: rest-api (paso 10, API REST de administración de grupos)
**Version**: spec 2026-09-06 (4 capabilities)
**Mode**: Standard (strict_tdd false)

## Completeness

| Metric | Value |
|--------|-------|
| Tasks total | 30 (1.1-1.9, 2.1-2.10, 3.1-3.8, 4.1-4.3) |
| Tasks complete | 26 |
| Tasks incomplete | 4 (3.7 E2E manual, 4.1-4.3 cleanup/docs) |

## Build & Tests Execution

**Build**: ✅ Passed
```
go build ./... → sin salida (ok)
go vet ./...   → sin salida (ok)
gofmt -l ./internal/ ./cmd/ → sin archivos (formateado)
```

**Tests**: ✅ 0 failed / 0 skipped (integración activa, no -short)
```
go test -count=1 ./...
ok  internal/api        4.059s
ok  internal/auth       4.700s
ok  internal/config     2.112s
ok  internal/events     3.264s
ok  internal/groups     3.833s
ok  internal/joinrequests 3.395s
ok  internal/logs       1.695s
ok  internal/moderation 1.510s
ok  internal/telegram  20.995s
ok  internal/users      1.123s
```
Total tests: 12 telegram + 12 moderation + 4 logs + 8 joinrequests + 3 users + 33 api = **72 tests** (paquetes con tests).

**Coverage**: ➖ No configurada métrica; no es requisito del spec (§21: priorizar lógica crítica).

## Spec Compliance Matrix

| Requirement | Scenario | Test | Result |
|-------------|----------|------|--------|
| Mod-1: Service expone 12 métodos (ctx 1er param) | | `moderation_test.go > TestAdapterBanUser_*`, `TestAdapterApproveRejectJoinRequest`, `TestAdapterGetChatMember_DecodesResult`, `TestAdapterGetChatAdministrators_DecodesResult` | ✅ COMPLIANT |
| Mod-2: transporte POST+JSON (ChatPermissions anidado) | | `TestAdapterBanUser_Success` | ✅ COMPLIANT |
| Mod-3: token nunca en body/query/logs | | `TestAdapterError_DoesNotLeakToken` | ✅ COMPLIANT |
| Mod-4: errores dominio (403/400 permisos, 404 notfound) | | `TestAdapterBanUser_PermissionDenied`, `TestAdapterBanUser_BadRequestNotFound`, `TestAdapterBanUser_BadRequestRights` | ✅ COMPLIANT |
| Mod-5: rate limiter ~25 req/s + 429 retry_after max 3 | | `TestAdapterDoWithRetry_RateLimitThenSuccess`, `TestAdapterDoWithRetry_RateLimitWaitsRetryAfter`, `TestAdapterDoWithRetry_RateLimitNoRetryAfter`, `TestAdapterDoWithRetry_RateLimitExhausted` | ✅ COMPLIANT |
| Logs-1: tabla logs + índice group_id | | migración 00003 (validada por tests DB) | ✅ COMPLIANT |
| Logs-2: Create + ListByGroup desc | | `TestRepository_CreateAndGet`, `TestRepository_ListByGroupFiltersAndOrders`, `TestRepository_ListByGroupEmpty` | ✅ COMPLIANT |
| Logs-3: 9 acciones registran log (éxito/permiso/fallo/notfound) | | `TestService_BanSuccess`, `TestService_PermissionDeniedDoesNotCallTelegram`, `TestService_TelegramErrorLogged`, `TestService_TelegramNotFoundLogged`, `TestRepository_CreateFailureLog` | ✅ COMPLIANT |
| Logs-4: actor_id de claims; eventos sin admin NULL | | `TestModerationActions_PassActorAndIDs` | ✅ COMPLIANT |
| GA-1: GET /api/groups y /:id autenticados | | `TestListGroups_Data`, `TestGetGroup_NotFound`, `TestModerationRoutes_RequireAuth` | ✅ COMPLIANT |
| GA-2: 404 grupo inexistente; 401 sin token | | `TestGetGroup_NotFound`, `TestModerationRoutes_RequireAuth` | ✅ COMPLIANT |
| GA-3: GET users = admins; ?userId= lookup; limitación documentada | | `TestListGroupUsers_Lookup`, `TestListGroupUsers_NoPermission`, `TestListGroupUsers_InvalidUserID` | ✅ COMPLIANT |
| GA-4: acciones POST con flujo grupo→permiso→telegram→log | | `TestModerationActions_PassActorAndIDs`, `TestModerationAction_Errors`, `TestService_*` | ✅ COMPLIANT |
| GA-5: sin permiso → 403 sin llamar a Telegram | | `TestService_PermissionDeniedDoesNotCallTelegram`, `TestModerationAction_Errors (bot sin permiso)` | ✅ COMPLIANT |
| GA-6: userId inexistente → 404 + log NOT_FOUND | | `TestService_TelegramNotFoundLogged`, `TestModerationAction_Errors (telegram not found)` | ✅ COMPLIANT |
| GA-7: envelope §18; ids no numéricos → 400 VALIDATION_ERROR | | `TestModerationAction_Validation`, `TestListGroupUsers_InvalidUserID`, `TestModerationAction_BadBody` | ✅ COMPLIANT |
| JR-1: tabla join_requests + índices + pending único parcial | | migración 00003 (validada por tests DB) | ✅ COMPLIANT |
| JR-2: bus chat_join_request → fila pending upsert | | `TestHandleChatJoinRequest_*`, `TestRepository_UpsertPendingIdempotent`, `TestRepository_UpsertAfterResolveCreatesNewRow` | ✅ COMPLIANT |
| JR-3: GET join-requests autenticado | | `TestListJoinRequests_Data`, `TestModerationRoutes_RequireAuth` | ✅ COMPLIANT |
| JR-4: approve/reject flujo (existe→permiso→telegram→status→log) | | `TestService_ApproveSuccess`, `TestService_RejectRequiresCanInviteUsers` | ✅ COMPLIANT |
| JR-5: requestId inexistente → 404 | | `TestService_ApproveNotFound`, `TestModerationAction_Errors (solicitud no encontrada)` | ✅ COMPLIANT |
| JR-6: ya decidida → 409 sin llamar Telegram | | `TestService_ApproveAlreadyDecidedNoTelegram`, `TestModerationAction_Errors (ya decidida)` | ✅ COMPLIANT |
| JR-7: Telegram falla → TELEGRAM_ERROR sin decidir | | `TestService_ApproveTelegramErrorKeepsPending` | ✅ COMPLIANT |
| JR-8: request de otro grupo → 404 (no filtrar) | | `TestService_ApproveWrongGroup` | ✅ COMPLIANT |

**Compliance summary**: 24/24 escenarios compliant

## Correctness (Static Evidence)

| Requirement | Status | Notes |
|------------|--------|-------|
| Service telegram con 12 métodos | ✅ Implementado | moderation.go; params de restricción en structs |
| doPost + rate limiter en Adapter | ✅ Implementado | rate_limiter.go (token bucket síncrono, ~25 req/s) |
| Migración 00003 | ✅ Implementado | users, join_requests, warnings, logs |
| Domains users/logs/joinrequests/moderation | ✅ Implementado | repositorios + evento + servicio orquestador |
| API handlers §12 | ✅ Implementado | groups, moderation, joinrequests, logs |
| Wiring main.go (webhook+polling) | ✅ Implementado | bus maneja chat_join_request → users + pending |
| Rate limit enforcement real en adapter | ✅ Implementado | reintento con retry_after, max 3 |

## Coherence (Design)

| Decision | Followed? | Notes |
|----------|-----------|-------|
| D1: una interfaz con 12 métodos; consumidores interfaz mínima | ✅ Yes | telegram.Service; moderation.TelegramActions |
| D2: doPost nuevo (ChatPermissions no va en query) | ✅ Yes | |
| D3: token bucket síncrono wait(ctx) en Adapter, sin worker extra | ✅ Yes | D3 worker solo si batch; MVP sin batch |
| D4: reintento 429 retry_after max 3 | ✅ Yes | doWithRetry |
| D5: errores tipados nuevos | ✅ Yes | ErrPermissionDenied/ErrTelegramNotFound/ErrTelegramAPI |
| D6: users.telegram_id id; join_requests.user_id telegram id, sin FK | ✅ Yes | migración sin FK |
| D7: requireAuth identidad; autorización en Service | ✅ Yes | middleware_auth + moderation.Service |
| D8: GET users = admins + getChatMember | ✅ Yes | P2 decidida |
| D9: requestId = join_requests.id BIGSERIAL | ✅ Yes | |
| D10: path params int64 → VALIDATION_ERROR | ✅ Yes | pathID helper |
| Mapa permisos: approve Y reject → can_invite_users | ✅ Yes (corrección) | tasks.md nota: corrige design (reject era can_restrict_members); verificado en referencia §7 |
| unlock: can_send_messages true + flags envío estándar | ✅ Yes | mutePermissions/unmutePermissions en moderation.go |

## Issues Found

**CRITICAL**: None

**WARNING**:
- Task 3.7 (E2E manual contra grupo de prueba real) pendiente — requiere grupo Telegram de prueba + admin; no automatizable en CI.
- Tasks Phase 4 (4.1 docs referencia, 4.2 documentar limitación group_members, 4.3 state.yaml final) pendientes de execute en archive.

**SUGGESTION**:
- Desviaciones de tasks documentadas (1.6 ChatMember en vez de ChatUserInfo; 2.8 suite DB por paquete; 3.6 fakes puros en vez de OpenTestDB "rest"; 3.1 +WithLogs): ninguna rompe spec, todas registradas en tasks.md.

## Verdict

**PASS WITH WARNINGS**
24/24 escenarios de spec con test pasando; 0 fallos de build/vet/tests reales; pendientes sólo E2E manual (3.7) y cleanup Phase 4 para archive.