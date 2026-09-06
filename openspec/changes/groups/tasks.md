# Tasks: groups — paso 7: Modelo Group

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~300-350 |
| 400-line budget risk | Low |
| Chained PRs recommended | No |
| Suggested split | single PR en feat/groups (base feat/telegram-events) |
| Delivery strategy | ask-on-risk |
| Chain strategy | feature-branch-chain (misma chain; aún sin remote) |

Decision needed before apply: Yes
Chained PRs recommended: No
Chain strategy: feature-branch-chain
400-line budget risk: Low

### Suggested Work Units

| Unit | Goal | Likely PR | Notes |
|------|------|-----------|-------|
| 1 | Migración + model + repo + tests (todo el paso 7) | PR groups | single PR bajo 400 líneas; base = feat/telegram-events |

## Phase 1: Foundation (modelo persistente)

- [ ] 1.1 Crear `backend/migrations/00002_create_groups.sql`: tabla `groups` según design (telegram_id BIGINT NOT NULL UNIQUE, title TEXT NOT NULL, username TEXT NULL, type TEXT NOT NULL, member_count BIGINT NULL, bot_status TEXT NOT NULL DEFAULT 'member', bot_permissions JSONB NULL, created_at/updated_at TIMESTAMPTZ DEFAULT now()) con `-- +goose Up/Down` (DROP TABLE).
- [ ] 1.2 Crear `backend/internal/groups/model.go`: struct `Group` (ID, TelegramID, Title, Username *string, Type, MemberCount *int64, BotStatus, BotPermissions map[string]bool, CreatedAt, UpdatedAt), consts `Status*` (member default) y `var ErrNotFound`.

## Phase 2: Core (repository)

- [ ] 2.1 Crear `backend/internal/groups/repository.go`: `NewRepository(db *sql.DB) *Repository` y helper `scanGroup(rows/row)` que mapea fila→Group (JSONB → map[string]bool, NULL → nil).
- [ ] 2.2 Implementar `UpsertByTelegramID(ctx, g *Group) error` con `INSERT ... ON CONFLICT (telegram_id) DO UPDATE SET ... updated_at = now()` y placeholders `$n` (escenario Unicidad).
- [ ] 2.3 Implementar `List(ctx) ([]Group, error)` con `ORDER BY title ASC` (escenario Listado).
- [ ] 2.4 Implementar `GetByTelegramID(ctx, telegramID int64) (*Group, error)`; `sql.ErrNoRows` → `ErrNotFound` (escenario Consulta).

## Phase 3: Testing (integración Postgres real)

- [ ] 3.1 Crear `backend/internal/groups/repository_test.go`: helper de setup que aplica `database.Migrate` y falla/skipea si `-short` o sin `TEST_DATABASE_URL` (default `postgres://telegram:telegram@localhost:5432/telegram_manager?sslmode=disable`).
- [ ] 3.2 Tests table-driven que cubran: upsert idempotente (1 fila), listado ordenado por title, get encontrado, get inexistente → ErrNotFound, updated_at cambia y created_at se conserva (escenarios del spec: Unicidad, Listado, Consulta, Timestamps).

## Phase 4: Verification

- [ ] 4.1 `go build ./...`, `go vet ./...`, `go test ./... -count=1` (sin short para correr integración si hay Postgres) y `gofmt -l .` limpio.
- [ ] 4.2 Verificar en vivo: `docker compose up -d` y confirmar que goose aplica `00002` al iniciar el backend (log "migrations applied").

## Notas

- Sin cambios en `main.go` en este paso (wiring difiere al paso 8, decisión 6 del design).
- `.env.example` sin cambios (sin variables nuevas).
- Commit: convencional `feat(groups): modelo Group con migracion y repositorio`.