# Verify Report: groups-detection — paso 8: Detección/registro de grupos

## Verdict: PASS

## Proofs

### Test Suites
- `go build ./...` — OK
- `go vet ./...` — OK
- `gofmt -l .` — limpio (0 archivos)
- `go test ./internal/groups/ -count=1` — 14 tests (9 unit + 5 integración Postgres): PASS
- `go test ./... -count=1 -short` — suite completa: OK (5 paquetes)

### Live Verification (Telegram real)
- Bot agregado a grupo nuevo → evento `my_chat_member` recibido, fila creada (`bot_status=member`).
- Bot promovido a admin → segundo evento, MISMA fila actualizada (`bot_status=administrator`) con `can_*` reales de Telegram.
- Fila única por `telegram_id` (sin duplicados — upsert idempotente).
- Permisos reportados: can_change_info/can_invite_users/can_pin_messages/can_delete_messages/can_restrict_members=true, can_promote_members=false (mapeo `*bool` correcto).

## Spec Traceability (10/10 escenarios COMPLIANT)

| Scenario | Status | Evidence |
|----------|--------|----------|
| Bot promovido a administrador | ✅ | E2E: log registered + fila administrator con can_* |
| Bot agregado como miembro | ✅ | E2E: primer evento → bot_status member |
| Chat privado ignorado | ✅ | Test unit: IgnoresPrivate (g sin cambios, err nil) |
| Update sin estado del bot | ✅ | Test unit: IgnoresNilChatOrMember |
| Bot removido del grupo | ✅ | Test unit: KeepsLeftStatus (bot_status left, permisos nil) |
| Permisos de administrador | ✅ | Test unit: CopiesPresentPermissions (6 can_* mapeados) |
| Bot sin permisos previos | ✅ | Test unit: NoPermissionsForMember (BotPermissions nil) + repo test UpsertPreservesPermissionsWhenNil (CASE WHEN no pisa) |
| Grupo con username | ✅ | Test unit: MapsSupergroup (username "mucomunidad" + title) |
| `member_count` no se toca | ✅ | events.go no referencia MemberCount; repo test preserva el campo |
| Fallo de persistencia no bloqueante | ✅ | main.go: handler con slog.Error, no rompe bus (inspect código) |

## Design Traceability (5/5 decisiones implementadas)

| # | Decisión | Implementación |
|---|----------|----------------|
| 1 | Chat *Chat + OldChatMember | update.go líneas 44-50 |
| 2 | can_* *bool | update.go líneas 58-64 |
| 3 | HandleMyChatMember pura | groups/events.go |
| 4 | Handler resiliente en main.go | main.go |
| 5 | Permisos nil no pisan previo | repository.go CASE WHEN |

## Test Count

| Package | Tests | Notes |
|---------|-------|-------|
| internal/groups | 14 | 9 unit + 5 integración (Postgres) |
| Suite completa | 43 | api(8), config(1), events(2), groups(14), telegram(18) |
| E2E manual | 2 | member + administrator |

## No Migration

Tabla `groups` ya existía (00002). Este cambio no agrega migraciones.

## Known Gaps / Suggestions

| Severity | Item |
|----------|------|
| SUGGESTION | Los tests de integración comparten la DB compose; tras correrlos quedan filas de prueba (`groups.telegram_id=1000000301`). Limpiada manualmente tras E2E. Considerar `TEST_DATABASE_URL` dedicada + cleanup en CI (ya anotado en cambio anterior). |
| SUGGESTION | `OldChatMember` se decodifica pero no se consume aún; quedará disponible para transiciones (promoción/restricción) en pasos futuros. |
| INFO | Limitación documentada: grupos donde el bot ya estaba antes del arranque no emiten evento → no se registran. Mitigación futura (chat_id manual) fuera de MVP. |