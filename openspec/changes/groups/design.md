# Design: groups — paso 7: Modelo Group

## Technical Approach

Approach 1 aprobado en exploración: migración goose `00002` + paquete
`internal/groups` con `model.go` y `repository.go`. El repository es la
única pieza nueva con lógica; usará `database/sql` + `pgx` (ya
presentes), placeholders `$n`, y expondrá `UpsertByTelegramID`,
`List`, `GetByTelegramID`. Sin service ni handlers (pasos 8 y 10).
Se reutiliza el patrón de la migración `00001` (SQL plano, `-- +goose
Up/Down`, comentario de contexto).

## Architecture Decisions

| # | Decisión | Alternativas | Por qué |
|---|----------|--------------|---------|
| 1 | `bot_status` y `type` como TEXT con constantes Go propias del paquete `groups` | enum nativo de Postgres; reutilizar `telegram.MemberStatus*` | Consistente con el código actual (strings); el mapeo telegram→groups queda explícito en el paso 8; menos fricción de migraciones. Un enum Postgres agrega casting sin beneficio en el MVP |
| 2 | `bot_permissions JSONB` mapeado a `map[string]bool` (nil = NULL) | columnas booleanas individuales; `json.RawMessage` | El grupo `can_*` varía y es opcional; JSONB evita tabla extra y N columnas; en Go un `map[string]bool` es directo de consumir (mayormente true cuando están activos). `latest_status`/`1231030323` no aplican |
| 3 | Repository concreto `Repository` + `NewRepository(db *sql.DB)`; sin interfaz por ahora | interfaz `GroupStore` declarada ya | Las interfaces se declaran donde se consumen (convención del proyecto); hoy no hay consumidor. La interfaz llega con la detección (paso 8) o los endpoints (paso 10) |
| 4 | `UpsertByTelegramID` con `INSERT ... ON CONFLICT (telegram_id) DO UPDATE SET updated_at = now()` | `SELECT` + `UPDATE/INSERT` separados | Atómico, idempotente; cumple el escenario "una fila por telegram_id" sin carrera |
| 5 | `ErrNotFound` como error de dominio en `internal/groups` | reusar `sql.ErrNoRows` hacia afuera | El handler (paso 18/10) traducirá a `NOT_FOUND` sin tocar el driver; no se fuga `sql.ErrNoRows` fuera del paquete |
| 6 | Wiring del repo en `main.go` se difiere al paso 8 | construirlo y guardarlo en `api.Server` ahora | Un campo muerto en Server sin endpoints ni consumers no aporta; "código simple y mantenible" ($25.10). Ajuste menor al scope de la proposal, justificado acá |
| 7 | Tests de repo = integración contra Postgres real, skippables (`-short` o sin `TEST_DATABASE_URL`) | sqlmock; SQLite | Convención del proyecto: no mockear la capa DB (backend-go-skill §8); los constraints y tipos de Postgres no son equivalentes en SQLite |

## Data Flow

```
main.go (dev polling)
  db (pgx) ──> NewRepository(db)  [cuando llegue el paso 8]

Grupo nuevo/update ──> UpsertByTelegramID(g) ──> INSERT..ON CONFLICT
Consulta panel ──> List() / GetByTelegramID(id) ──> filas → Group{}
```

## File Changes

| File | Acción | Descripción |
|------|--------|-------------|
| `backend/migrations/00002_create_groups.sql` | Create | tabla `groups` + down |
| `backend/internal/groups/model.go` | Create | `Group`, `BotStatus` consts, `ErrNotFound` |
| `backend/internal/groups/repository.go` | Create | `NewRepository`, `UpsertByTelegramID`, `List`, `GetByTelegramID`; scanner manual |
| `backend/internal/groups/repository_test.go` | Create | integración Postgres real (skippable `-short`) |
| `backend/cmd/server/main.go` | — | Sin cambios en este paso (decisión 6) |

## Interfaces / Contracts

```go
// internal/groups/model.go
type BotStatus string
const (
    StatusAdministrator BotStatus = "administrator"
    StatusMember        BotStatus = "member"
    StatusRestricted    BotStatus = "restricted"
    StatusLeft          BotStatus = "left"
    StatusKicked        BotStatus = "kicked"
    StatusCreator       BotStatus = "creator"
)

type Group struct {
    ID             int64
    TelegramID     int64
    Title          string
    Username       *string
    Type           string
    MemberCount    *int64
    BotStatus      BotStatus
    BotPermissions map[string]bool // nil si desconocidos
    CreatedAt      time.Time
    UpdatedAt      time.Time
}

var ErrNotFound = errors.New("groups: not found")

// internal/groups/repository.go
func NewRepository(db *sql.DB) *Repository
func (r *Repository) UpsertByTelegramID(ctx context.Context, g *Group) error
func (r *Repository) List(ctx context.Context) ([]Group, error)
func (r *Repository) GetByTelegramID(ctx context.Context, telegramID int64) (*Group, error)
```

SQL base:

```sql
CREATE TABLE groups (
    id             BIGSERIAL PRIMARY KEY,
    telegram_id    BIGINT        NOT NULL UNIQUE,
    title          TEXT          NOT NULL,
    username       TEXT,
    type           TEXT          NOT NULL,
    member_count   BIGINT,
    bot_status     TEXT          NOT NULL DEFAULT 'member',
    bot_permissions JSONB,
    created_at     TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ   NOT NULL DEFAULT now()
);
```

## Testing Strategy

| Capa | Qué | Cómo |
|------|-----|------|
| Integración | upsert idempotente (1 fila), list ordenado, get encontrado/notfound, updated_at cambia | Postgres real: `TEST_DATABASE_URL` (default `postgres://telegram:telegram@localhost:5432/telegram_manager?sslmode=disable`), aplicar `database.Migrate` en el test (goose FS embebido), `t.Skip` si `-short` o conexión falla; casos table-driven (go-testing) |

No hay unit tests de servicio (no existe service). El scan se cubre
implícitamente en los casos de integración.

## Migration / Rollout

`00002_create_groups.sql` aplicada automáticamente en dev por goose al
iniciar (patrón ya existente). Rollback: `goose down` / migrate down
borra la tabla; sin consumo externo, no hay data que preservar.

## Open Questions

- None