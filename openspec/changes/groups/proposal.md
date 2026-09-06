# Proposal: groups — paso 7: Modelo Group

## Intent

El dominio `groups` no existe: no hay tabla, struct ni repository.
Este cambio crea la base de datos del dominio para que el paso 8
(detección/registro desde eventos `my_chat_member`) y el paso 10
(endpoints REST) puedan construirse encima sin rework. Es el primer
paso de "Gestión de grupos" (AGENTS.md §6).

## Scope

### In Scope
- Migración goose `00002_create_groups.sql` (tabla `groups`).
- `internal/groups/model.go` — struct `Group`.
- `internal/groups/repository.go` — repo sobre `*sql.DB`:
  `UpsertByTelegramID`, `List`, `GetByTelegramID` (ON CONFLICT
  `telegram_id`).
- `internal/groups/repository_test.go` — tests de integración contra
  Postgres real, skippables con `-short` (go-testing).
- Wiring del repo en `main.go` (inyectable, aunque sin endpoints aún).

### Out of Scope
- Detección/registro desde eventos `my_chat_member` (paso 8).
- Extender `telegram.ChatMember` con permisos `can_*` (paso 8).
- Endpoints REST de grupos (paso 10), service, handlers.
- Eliminación de grupos (no pedida en MVP).

## Capabilities

### New Capabilities
- `groups`: modelo persistente de grupos de Telegram administrables
  (identidad, estado del bot y permisos disponibles).

### Modified Capabilities
- None

## Approach

Approach 1 de la exploración (aprobado): tabla `groups` con
telegram_id UNIQUE, title, username NULL, type, member_count NULL,
bot_status, bot_permissions JSONB NULL, created_at/updated_at;
struct `Group` en `internal/groups`; repository con placeholders `$n`
(convención backend-go-skill §4), scanner manual con `database/sql`.
Sin service: llegarán los endpoints (paso 10).

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `backend/migrations/00002_create_groups.sql` | New | tabla groups |
| `backend/internal/groups/model.go` | New | struct Group |
| `backend/internal/groups/repository.go` | New | UPSERT/List/Get |
| `backend/internal/groups/repository_test.go` | New | integración skippable |
| `backend/cmd/server/main.go` | Modified | construir repo con db |
| `openspec/specs/groups/spec.md` | New | spec base (al archivar) |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Sin Postgres en CI → tests de repo skippeados | Med | `-short` skip documentado; correr localmente con compose |
| JSONB de permisos sin poblar hasta paso 8 | Low | modelo lo soporta; queda NULL |

## Rollback Plan

Migración reversible (`-- +goose Down` `DROP TABLE groups`). Como no
hay consumo externo aún (sin endpoints), revertir = borrar el paquete
`internal/groups` y la migración.

## Dependencies

- Ninguna nueva. `pgx` y goose ya están en `go.mod`.

## Success Criteria

- [ ] `go build/vet/test ./...` verde (tests de repo si hay Postgres).
- [ ] Migración 00002 aplicada por goose al iniciar (polling dev).
- [ ] `UpsertByTelegramID` idempotente (upsert en misma telegram_id
      no duplica filas).
- [ ] `.env.example` sin cambios (sin variables nuevas).
- [ ] Ningún secreto nuevo en el repo.