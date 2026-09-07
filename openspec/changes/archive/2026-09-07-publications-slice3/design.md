# Design: Publications — Slice 3 (Programación + Historial Paginado)

> **Change**: `publications-slice3` (cierra Fase 2). **Mode**: hybrid.
> **Persisted**: `openspec/changes/publications-slice3/design.md` + Engram `sdd/publications-slice3/design`.
> **Base**: `main @ b90c584` (slice 2 archivado). **Delivery**: single-pr, size:exception solicitada en proposal.

## Technical Approach

Worker in-process (`time.Ticker` 30s + `SELECT ... FOR UPDATE SKIP LOCKED` en txn explícita) que reclama filas `scheduled` vencidas y las entrega a `publishOne` (helper privado de slice 2 — reutilización literal, respeta bugfix #172). POST `/api/publications` extiende con `scheduled_at` opcional (RFC3339 → UTC) con branching: vacío → flujo síncrono slice 2; futuro → inserta N filas `status=scheduled` sin tocar Telegram; pasado → 400. GET `/api/publications` paginado (`?limit&offset`, default 50, max 100). DELETE `/api/publications/:id` cancela **solo** `status=scheduled` (hard delete; otros status → 409). Frontend dual-mode + Cancelar + Prev/Next.

## Architecture Decisions

| # | Topic | Choice | Tradeoff | Rationale |
|---|-------|--------|----------|-----------|
| D1 | Schema | **Sin migración** | — | `scheduled_at TIMESTAMPTZ` y `status='scheduled'` ya viven en 00004 (`migrations/00004_create_publications.sql:11-14`). Rollback trivial. |
| D2 | Endpoint POST | **Mismo handler, branching por `scheduled_at`** | Endpoint separado `/scheduled` | Una API para el admin. Branching vive en `Service.PublishMany` (pre-validación) + ruta nueva de insert sin dispatch. |
| D3 | Scheduler | **Goroutine + `time.Ticker` configurable** | Cron externo / librería | Mismo lifecycle que `telegram.Poller` (`main.go:194-202`, `signal.NotifyContext`). `DefaultSchedulerInterval = 30 * time.Second`. Tests inyectan 5 ms. |
| D4 | Concurrencia | **`BEGIN; SELECT … FOR UPDATE SKIP LOCKED LIMIT 25; UPDATE … status='sending'; COMMIT`** | Naive SELECT+UPDATE (race) / advisory lock | Canónico Go/Postgres para job queues. SKIP LOCKED es Postgres ≥ 9.5 (16 ✓). `pgx/v5` stdlib lo soporta nativo. Mantiene concurrencia segura si el backend se escala a 2+ instancias (futuro). |
| D5 | Worker dispatch | **Reusa `publishOne`** (`service.go:203`) | Reimplementar path | `publishOne` ya hace permissionOk (#172) + dispatch SendMessage/SendPhoto + UpdateStatus + log `ActionPublishMessage`. Comportamiento observable idéntico. |
| D6 | Batch size | **25** | 10 / 50 | 25 × 1 msg/seg ≤ 25s < tick 30s. Próximo tick absorbe resto si se atrasa. |
| D7 | DELETE policy | **Hard delete solo si `status='scheduled'`** | Soft delete (status='cancelled') | `sent`/`failed` preservan audit. Race con worker: lectura `status` no-transaccional antes del DELETE → 409 si worker ya marcó `sending`. |
| D8 | Pagination | **`?limit=` (default 50, max 100) + `?offset=` (default 0)** | Cursor-based / status filter | Simple, sin schema. `validatePagination` 400 si fuera de rango. Sin status filter en MVP. |
| D9 | Timezone | **RFC3339 con offset, normalizado a `t.In(UTC)`** | UTC-only / naive time | Acepta `Z` y `+03:00`. Persistido ya es `TIMESTAMPTZ`. |
| D10 | Validación `scheduled_at` | **Estrictamente futuro** (`t.After(now)`, tolerancia cero) | Margen 1 min | UX clara: "debe ser en el futuro" → el backend rechaza al instante. |
| D11 | Frontend datetime | **`<input type="datetime-local">` + validación cliente** | datepicker custom | Nativo, mobile-friendly. Cliente envía ISO local convertido a UTC ISO antes del POST. |
| D12 | Tests | **Service/Worker/Handler/Frontend con fakes; Repository integration Postgres real** | Mockear SKIP LOCKED | §21.1 estricto. SKIP LOCKED requiere SQL real (tests de concurrencia con dos conexiones). |
| D13 | Env var | **`PUBLICATIONS_SCHEDULER_INTERVAL_SECONDS`** (default 30) | Hardcoded | Configurable para tuning prod y tests. Default seguro. |
| D14 | Adapter Telegram | **Sin cambios** | — | `SendMessage` + `SendPhoto` ya cubren foto+caption+botones (slice 2). Worker los llama igual. |

## Data Flow

```
POST /api/publications
  ├─ scheduled_at nil          → service.PublishMany (síncrono, slice 2)
  └─ scheduled_at set          → service.PublishMany (branch schedule)
                                    └─ validatePayload + validateScheduledAt (> now)
                                    └─ N filas con status='scheduled'
                                    └─ 201 {publications:[…]}

Worker.Run(ctx)  [goroutine, ticker 30s]
  for each tick:
    rows := store.ClaimScheduledDue(ctx, 25)        [txn SKIP LOCKED]
    for each row:
      publishOne(ctx, actorID=row.ActorID, groupID=row.TelegramID,
                 text=row.Text, hasPhoto=row.PhotoURL!=nil,
                 photoURL=row.PhotoURL, buttons=row.Buttons)
      └─ permissionOk + SendMessage|SendPhoto + UpdateStatus + log
  return nil on ctx.Done()

DELETE /api/publications/:id
  service.CancelScheduled(ctx, id)
    ├─ store.GetByID            → 404 si no existe
    ├─ status != 'scheduled'    → 409 ErrCancelNotAllowed
    └─ store.Cancel (DELETE WHERE id=$1 AND status='scheduled')
        ├─ 1 row affected       → 204
        └─ 0 rows               → 409 (race con worker)
```

### Claim pattern (Repository.ClaimScheduledDue)

```go
func (r *Repository) ClaimScheduledDue(ctx context.Context, limit int) ([]Publication, error) {
    tx, err := r.db.BeginTx(ctx, nil)
    if err != nil { return nil, err }
    defer tx.Rollback() // noop si Commit OK

    rows, err := tx.QueryContext(ctx, `
        SELECT id, telegram_id, text, photo_url, buttons, scheduled_at, actor_id
        FROM publications
        WHERE status='scheduled' AND scheduled_at <= now()
        ORDER BY scheduled_at ASC
        LIMIT $1
        FOR UPDATE SKIP LOCKED`, limit)
    // scan into slice; defer rows.Close()

    ids := make([]int64, 0, len(pubs))
    for _, p := range pubs { ids = append(ids, p.ID) }

    _, err = tx.ExecContext(ctx, `
        UPDATE publications SET status='sending', updated_at=now()
        WHERE id = ANY($1)`, pq.Array(ids))

    return pubs, tx.Commit()
}
```

### Scheduler struct (worker.go)

```go
type Scheduler struct {
    store    PubStore          // extended with ClaimScheduledDue
    groups   GroupReader
    tg       MessageSender
    log      LogWriter
    interval time.Duration
}

func NewScheduler(store PubStore, groups GroupReader, tg MessageSender, log LogWriter, interval time.Duration) *Scheduler

func (s *Scheduler) Run(ctx context.Context) error {
    t := time.NewTicker(s.interval)
    defer t.Stop()
    for {
        select {
        case <-ctx.Done():
            return nil
        case <-t.C:
            s.tick(ctx)
        }
    }
}

func (s *Scheduler) tick(ctx context.Context) {
    rows, err := s.store.ClaimScheduledDue(ctx, claimBatchSize) // 25
    if err != nil { slog.Warn("publications worker: claim failed", "err", err); return }
    for _, row := range rows {
        s.processClaimed(ctx, row) // publishOne path
    }
    slog.Info("publications worker tick", "due", len(rows), "interval", s.interval.String())
}
```

### PubStore extension

```go
type PubStore interface {
    // ... slice 1+2 (Create, GetByID, UpdateStatus, …)
    List(ctx context.Context, limit, offset int) ([]Publication, error)            // paginado
    ListByTelegramID(ctx context.Context, telegramID int64, limit, offset int) ([]Publication, error)
    ClaimScheduledDue(ctx context.Context, limit int) ([]Publication, error)        // slice 3
    Cancel(ctx context.Context, id int64) error                                     // slice 3: ErrCancelNotAllowed | ErrNotFound
}
```

> NOTA: cambiar firmas de `List`/`ListByTelegramID` rompe fakes en `service_test.go` y `publications_handlers_test.go`. Se actualizan en el mismo PR (3 archivos fake).

## File Changes

| File | Action | LOC est. | Notes |
|------|--------|---------:|-------|
| `backend/internal/publications/model.go` | MOD | +25 | `ErrScheduledInPast`, `ErrCancelNotAllowed`, `ErrInvalidPagination`, `normalizeScheduledAt(s)`, `validateScheduledAt(t, nowFn)` |
| `backend/internal/publications/repository.go` | MOD | +75 | `ClaimScheduledDue` (SKIP LOCKED txn), `Cancel` (DELETE con race check), `List`/`ListByTelegramID` c/limit+offset |
| `backend/internal/publications/service.go` | MOD | +110 | `PublishMany` branching `scheduled_at` → `Schedule(payload, when)`; `CancelScheduled(ctx, id)`; `List`/`ListByTelegramID` propagan limit+offset |
| `backend/internal/publications/worker.go` | **NEW** | +135 | `Scheduler` struct + `Run(ctx)` + `tick(ctx)` + `processClaimed(ctx, row)` |
| `backend/internal/publications/service_test.go` | MOD | +95 | Tests schedule + cancel; extender `fakePubStore` con `ClaimScheduledDue`/`Cancel` |
| `backend/internal/publications/worker_test.go` | **NEW** | +130 | Tests fake-driven del scheduler |
| `backend/internal/publications/repository_test.go` | **NEW** | +90 | Integration tests con `OpenTestDB("publications")` |
| `backend/internal/api/publications_handlers.go` | MOD | +95 | Branch `scheduled_at`; `handleListPublications` lee limit/offset; **nuevo** `handleDeletePublication` |
| `backend/internal/api/publications_handlers_test.go` | MOD | +100 | Tests POST scheduled, GET paginación, DELETE; extender `fakePublicationStore` |
| `backend/internal/api/server.go` | MOD | +5 | Ruta `DELETE /api/publications/{id}` |
| `backend/internal/config/config.go` | MOD | +8 | `PublicationsSchedulerInterval` (int seconds, default 30) |
| `backend/cmd/server/main.go` | MOD | +20 | `Scheduler` deps + goroutine + `errCh`; log "publications scheduler started" |
| `frontend/src/features/publications/types.ts` | MOD | +8 | `PublishRequest.scheduled_at?`, `PublicationsFilter.limit/offset?` |
| `frontend/src/features/publications/api.ts` | MOD | +20 | `listPublications` arma `?limit&offset&group_id`; **nuevo** `cancelPublication(id)` |
| `frontend/src/features/publications/hooks.ts` | MOD | +25 | `usePublications` query key incluye limit+offset; **nuevo** `useCancelPublication` |
| `frontend/src/features/publications/error.ts` | MOD | +20 | `formatPublicationsError` mapea `ErrScheduledInPast`, `ErrCancelNotAllowed`, `ErrInvalidPagination` |
| `frontend/src/features/publications/validateScheduledAtClient.ts` | **NEW** | +25 | Helper cliente para `datetime-local` → ISO UTC |
| `frontend/src/pages/PublicationsPage.tsx` | MOD | +130 | Radio dual-mode; `<input type="datetime-local">` condicional; columna acción + botón Cancelar; Prev/Next |
| `frontend/src/pages/PublicationsPage.test.tsx` | MOD | +110 | Tests datetime-local, Cancelar, paginación |
| `frontend/src/test/helpers.tsx` | MOD | +10 | `mockFetchRoutes`: helper para DELETE (204 sin body) + paginated GET |
| `.env.example` | MOD | +5 | Variable `PUBLICATIONS_SCHEDULER_INTERVAL_SECONDS=30` + comentario |
| `README.md` | MOD | +30 | Sub-sección "Programación" + nota "Telegram no soporta scheduling nativo" + env var |

**Total estimado**: ~1240 LOC touched (incluye `repository_test.go` nuevo). **Excede 400-line budget** — `tasks` confirma single-pr vs chained.

## Testing Strategy

| Layer | What | Approach | Files |
|-------|------|----------|-------|
| Unit (svc) | `Schedule` inserta sin Telegram; valida pasado; `CancelScheduled` ok/rechaza sent | fakes (extender `fakePubStore`/`fakeTelegramPub`) | `service_test.go` |
| Unit (worker) | tick_no_due_no_send; tick_claims_and_publishes_one; ctx_cancel_stops | fakes (mismo fakePubStore + fakeTelegramPub) | `worker_test.go` (NEW) |
| Integration (repo) | `ClaimScheduledDue_BatchSize`, `ClaimScheduledDue_SkipsLockedByAnotherTxn` (dos `*sql.Tx` paralelas), `List_LimitOffset_Pagina`, `Cancel_DeleteRow`, `Cancel_SentReturnsErrCancelNotAllowed`, `Cancel_NotFoundReturnsErrNotFound` | `OpenTestDB("publications")` + `goose.Up` + `TRUNCATE publications CASCADE` entre tests | `repository_test.go` (NEW) |
| Handler | POST `scheduled_at` futuro 201; POST `scheduled_at` pasado 400 `ErrScheduledInPast`; POST offset `+03:00` normalizado a UTC; GET `?limit=10&offset=20` 200; GET `limit=101` 400; DELETE `scheduled` 204; DELETE `sent` 409; DELETE 404 | `httptest` + `fakePublicationStore` (extendido) | `publications_handlers_test.go` |
| Frontend | datetime-local solo cuando Programar; validación cliente "futuro"; Cancelar visible solo `scheduled`; Prev disabled cuando `offset===0`; Next disabled cuando `length<limit`; Cancel dispara DELETE | Vitest + `@testing-library/react` + `mockFetchRoutes` (extendido para DELETE) + `okJson`/`noContent` | `PublicationsPage.test.tsx` |
| §21.1 guard | Cero llamadas Bot API reales | `telegram` adapter solo en tests de `httptest.NewServer` (publications_test.go existente); worker tests NO usan adapter real | audit en `verify` |

## Migration / Rollout

**No migration required** (D1). Rollback: `git revert` del merge. Tabla `publications` intacta; filas `scheduled` huérfanas las borra admin con `DELETE FROM publications WHERE status='scheduled'` o vía el propio DELETE endpoint si la app aún responde.

## Open Questions

Ninguna pendiente — resueltas en `exploration.md` y `proposal.md` (Bot API no soporta scheduling → worker in-process; SKIP LOCKED canónico; sin Redis; sin status filter; hard delete; interval fijo + env var).

## Notes para `sdd-tasks`

- **Fakes a extender** (flag explícito en tasks): `fakePubStore` (`ClaimScheduledDue`, `Cancel`, `List(ctx,limit,offset)`, `ListByTelegramID(ctx,gid,limit,offset)`); `fakePublicationStore` (idem + `Delete` para DELETE handler).
- **Integration test**: NUEVO archivo `repository_test.go` (no existe). Patrón `OpenTestDB("publications")` + `database.Migrate` + `TRUNCATE publications CASCADE` entre tests.
- **Race SKIP LOCKED**: integration test abre 2 conexiones (`sql.DB` permite `Conn()`/`BeginTx`), una toma el lock con `SELECT ... FOR UPDATE`, la otra confirma que la misma fila NO aparece en su SELECT. Es la verificación más valiosa del slice.
- **400-line budget risk**: **High**. Forecast ~1240 LOC. DECIDE en `sdd-tasks` según §E del common protocol: chained (backend+worker / frontend+docs) o single-pr con size:exception (precedente slices 1+2).
