# Exploration: groups — paso 7: Modelo Group

## Current State

- **Base lista y archivada**: `repo-bootstrap` (Docker, Postgres+goose,
  config, adapter) y `telegram-events` (Adapter/Poller/Bus/webhook). El
  Bus ya despacha updates, pero el único handler es logging.
- **No existe el dominio `groups`**: ni tabla, ni struct, ni repository.
  La única migración es `00001_create_admins.sql`.
- **Capa DB**: `internal/database` expone `*sql.DB` (driver `pgx`
  stdlib) y `Migrate(ctx, db)` con goose; `main.go` aplica migraciones
  si `cfg.RunMigrations`.
- **`api.Server`** recibe `db pinger` y `botStatus botStatusProvider` —
  interfaces declaradas donde se consumen (convención del proyecto).
- **Modelo Telegram**: `telegram.Chat` (ID, Type, Title, Username),
  `ChatMember.Status` (creator/administrator/member/restricted/left/
  kicked). NO modela todavía los permisos del bot (`can_*`) que trae
  `my_chat_member` — eso es dependencia del paso 8, no de este.

## Affected Areas

- `backend/migrations/00002_create_groups.sql` — nueva tabla (goose).
- `backend/internal/groups/model.go` — struct `Group`.
- `backend/internal/groups/repository.go` — repo sobre `*sql.DB`.
- `backend/internal/groups/repository_test.go` — tests de integración
  skippables (Postgres real, go-testing).
- `backend/cmd/server/main.go` — wiring: construir el repo con `db` y
  dejarlo disponible (aunque los endpoints llegan en el paso 10).
- `openspec/specs/groups/spec.md` — spec base nuevo (en archive).

## Approaches

1. **Modelo + migración + repository CRUD mínimo** — tabla `groups`
   (id, telegram_id UNIQUE, title, username NULL, type, member_count
   NULL, bot_status, bot_permissions JSONB NULL, created_at,
   updated_at); `Group` struct; repo con `UpsertByTelegramID`,
   `List`, `GetByTelegramID` (ON CONFLICT para el paso 8).
   - Pros: deja la base para los pasos 8 (upsert desde eventos) y 10
     (endpoints); un solo cambio coherente; testable con Postgres real.
   - Cons: levemente más que "solo el struct" (pero el paso 8 lo
     necesitaría igual — no es trabajo duplicado).
   - Effort: Low

2. **Solo migración + struct** (sin repository)
   - Pros: mínimo absoluto, "exactamente" el paso 7 literal.
   - Cons: el paso 8 tendría que crear el repo casi idéntico; nada
     testable; agregar el repository después implica retocar wiring.
   - Effort: Low

3. **Modelo + repo + service ligero** (preparar List/Get de negocio)
   - Pros: capa de servicio desde el inicio.
   - Cons: sin reglas de negocio reales aún (CRUD puro); agrega una
     capa que no se ejerce hasta el paso 10. Contradice "código simple
     y mantenible" (AGENTS §25.10).
   - Effort: Medium

## Recommendation

**Approach 1**: migración 00002 + `internal/groups` con `model.go` y
`repository.go` (UpsertByTelegramID / List / GetByTelegramID). El repo
es la pieza crítica del dominio y el paso 8 depende de `Upsert`;
crearlo ahora evita rework y es testeable. Sin service todavía (llegará
con los endpoints, paso 10). Tests de repo = integración contra
Postgres real (convención backend-go-skill §8), skippables con
`-short` / sin conexión (go-testing).

## Risks

- Tests de integración de repositorios requieren Postgres disponible;
  si no hay, los tests se saltan y la cobertura del repo queda sin
  ejercer localmente — documentado, no bloqueante.
- `bot_permissions` como JSONB: el paso 8 debe extender
  `telegram.ChatMember` con `can_*` para poblar el campo. Fuera de
  alcance de este paso, pero el modelo ya lo soporta.
- No usar enum nativo de Postgres para `type`/`bot_status` (TEXT con
  constantes en Go): consistente con el código actual, menos fricción
  de migraciones.

## Ready for Proposal

Sí. El orchestrator debe crear el cambio `groups` (explore done),
proponer con alcance = paso 7 exacto (modelo + tabla + repo), sin
detección de eventos ni endpoints (pasos 8 y 10).