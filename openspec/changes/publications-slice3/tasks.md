# Tasks: Publications — Slice 3 (Programación + Historial Paginado)

> **Change**: `publications-slice3` (cierra Fase 2 del proyecto).
> **Base**: `main @ b90c584` (slice 2 archivado).
> **Delivery**: single-pr, `size:exception` solicitada en proposal — DECIDE en este tasks.
> **Test guard**: §21.1 estricto (cero llamadas Bot API reales; `TelegramService` mockeado en service/worker/handler tests; integration tests contra Postgres real solo para SQL del repo).

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~1276 LOC (22 files, 2 nuevos) |
| 400-line budget risk | **High** |
| Chained PRs recommended | **Yes** |
| Suggested split | PR1 backend+worker (Phases 1–7, ~800 LOC) → PR2 frontend+docs (Phases 8–10, ~480 LOC) |
| Delivery strategy | `single-pr` (precedente slices 1+2 size:exception; re-confirmada en apply) |
| Chain strategy | `size-exception` |

```text
Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: size-exception
400-line budget risk: High
```

> Forecast **High** (~1276 LOC excede 400). Precedente: slices 1 y 2 ambos con `size:exception` aprobado por el mismo `single-pr`. Si prefiere partir el trabajo, Phases 1–7 (backend+worker) cierran un PR compilable y testeable por sí solo; Phases 8–10 (frontend+docs) dependen solo del contrato HTTP ya en `main`.

### Suggested Work Units

| Unit | Goal | Likely PR | Notes |
|------|------|-----------|-------|
| 1 | Backend + worker (Phases 1–7) — model, repo, service, scheduler, handlers, server wiring, `go test`/`vet`/`fmt` verde | PR 1 (base `main`) | Compilable y testeable solo con backend; fakes actualizadas en mismo PR |
| 2 | Frontend + docs (Phases 8–10) — types/api/hooks/error + validate helper + page + tests + README + `.env.example` | PR 2 (base `main`, depende del contrato HTTP de Unit 1) | Compilable y testeable solo con frontend; depende de la API de Unit 1 ya mergeada |

> Manteniendo `single-pr` con `size:exception` la orquestación puede implementar las 10 phases en orden sin splits. Si se rompe en dos PRs, Phases 1–7 = PR 1; Phases 8–10 = PR 2 (ambos target `main`).

---

## Phase 1: PubStore interface extension + fakes discipline (compile gate FIRST)

> Por qué primero: extender firmas (`List/ListByTelegramID` c/limit+offset; nuevos `ClaimScheduledDue`/`Cancel`) rompe TODOS los implementadores de `PubStore`. Compilar verde antes de cualquier lógica nueva garantiza que el resto de las phases se construyen sobre una base coherente.

- [x] 1.1 `backend/internal/publications/service.go` — extender `PubStore` interface: `List(ctx, limit, offset int)` + `ListByTelegramID(ctx, telegramID int64, limit, offset int)`; agregar `ClaimScheduledDue(ctx, limit int) ([]Publication, error)` y `Cancel(ctx, id int64) error` (devuelve `ErrNotFound` o `ErrCancelNotAllowed` según corresponda).
- [x] 1.2 `backend/internal/publications/service_test.go` — extender `fakePubStore`: actualizar firmas de `List`/`ListByTelegramID`; implementar `ClaimScheduledDue` (filter `status='scheduled' && scheduled_at<=now`, marca `sending` in-memory) y `Cancel` (borra si `status='scheduled'`, retorna `ErrCancelNotAllowed` si otro status, `ErrNotFound` si id ausente).
- [x] 1.3 `backend/internal/api/publications_handlers_test.go` — extender `fakePublicationStore`: firmas nuevas de `List`/`ListByTelegramID`; implementar `Delete(ctx, id)` y/o `Cancel` para el handler DELETE; capturar `lastLimit`/`lastOffset` para asserts de paginación.
- [x] 1.4 `backend/internal/api/publications_handlers.go` — actualizar el `publicationStore` interface local (mismas firmas que 1.1) si difiere del paquete `publications`.
- [x] 1.5 **Compile gate**: `cd backend && go build ./...` retorna exit 0. Si falla, NO avanzar; fixear firmas antes.

## Phase 2: Model + errores + validación (con tests)

- [x] 2.1 `backend/internal/publications/model.go` — agregar errores de dominio: `ErrScheduledInPast`, `ErrCancelNotAllowed`, `ErrInvalidPagination` (mensajes en español consistentes con `ErrBotPermission`/`ErrGroupNotFound`).
- [x] 2.2 `backend/internal/publications/model.go` — agregar helpers: `normalizeScheduledAt(s string) (*time.Time, error)` (RFC3339 con offset → `t.In(UTC)`, retorna `ErrScheduledInPast` si no parsea o si `!t.After(nowFn())`).
- [x] 2.3 `backend/internal/publications/model.go` — agregar `validatePagination(limit, offset int) error` (retorna `ErrInvalidPagination` si `limit<1 || limit>100 || offset<0`).
- [x] 2.4 Tests en `backend/internal/publications/model_test.go` (NEW): `TestNormalizeScheduledAt_FutureUTC_OK`, `TestNormalizeScheduledAt_Past_ReturnsErrScheduledInPast`, `TestNormalizeScheduledAt_Offset_NormalizesToUTC`, `TestValidatePagination_OK`, `TestValidatePagination_OutOfRange`. Inyectar `nowFn` para determinismo.

## Phase 3: Repository (integration tests contra Postgres real)

- [x] 3.1 `backend/internal/publications/repository.go` — extender `List(ctx, limit, offset int)` y `ListByTelegramID(ctx, telegramID, limit, offset int)` con `LIMIT $N OFFSET $M` (orden `created_at DESC`).
- [x] 3.2 `backend/internal/publications/repository.go` — agregar `ClaimScheduledDue(ctx, limit int) ([]Publication, error)`: txn explícita (`BeginTx` + `defer Rollback`) con `SELECT … FOR UPDATE SKIP LOCKED LIMIT $1 ORDER BY scheduled_at ASC WHERE status='scheduled' AND scheduled_at <= now()`, luego `UPDATE publications SET status='sending', updated_at=now() WHERE id = ANY($1)`, `Commit`.
- [x] 3.3 `backend/internal/publications/repository.go` — agregar `Cancel(ctx, id int64) error`: txn con `SELECT status FROM publications WHERE id=$1 FOR UPDATE` → si nil → `ErrNotFound`; si status ≠ `'scheduled'` → `ErrCancelNotAllowed`; si OK → `DELETE FROM publications WHERE id=$1`.
- [x] 3.4 **NEW** `backend/internal/publications/repository_test.go` — integration tests con `OpenTestDB("publications")` + `database.Migrate` + `TRUNCATE publications CASCADE` entre tests: `TestRepository_ClaimScheduledDue_BatchSize` (N due rows, retorna N, todas quedan `sending`); `TestRepository_ClaimScheduledDue_SkipsLockedByAnotherTxn` (dos `*sql.Tx` paralelos, segunda no ve los ids de la primera); `TestRepository_List_LimitOffset_Pagina` (75 filas, `?limit=10&offset=20` retorna 10 filas correctas); `TestRepository_Cancel_DeleteRow` (status scheduled → fila borrada); `TestRepository_Cancel_SentReturnsErrCancelNotAllowed`; `TestRepository_Cancel_NotFoundReturnsErrNotFound`.

## Phase 4: Service — Schedule + CancelScheduled (con tests)

- [x] 4.1 `backend/internal/publications/service.go` — modificar `PublishMany(ctx, actorID, payload, scheduledAt *time.Time)`: si `scheduledAt == nil` → path síncrono slice 2; si != nil → rama `schedule`: `validatePayload` + `validateScheduledAt(scheduledAt, nowFn)` → loop por `group_ids` insertando filas `status='scheduled'` (sin tocar Telegram, sin log).
- [x] 4.2 `backend/internal/publications/service.go` — agregar `CancelScheduled(ctx, id int64) error`: `store.GetByID` → 404 si no existe; `status != 'scheduled'` → `ErrCancelNotAllowed`; `store.Cancel(id)` (race check adentro).
- [x] 4.3 `backend/internal/publications/service.go` — propagar `limit/offset` en `List(ctx, limit, offset)` y `ListByTelegramID(ctx, gid, limit, offset)` (signature ya actualizada en 1.1).
- [x] 4.4 `backend/internal/publications/service_test.go` — extender tests: `TestService_Schedule_InsertsScheduled_SinLlamarTelegram` (spy verifica cero `Send*`); `TestService_Schedule_PastDate_ErrScheduledInPast`; `TestService_CancelScheduled_OK`; `TestService_CancelScheduled_Sent_ReturnsErrCancelNotAllowed`; `TestService_CancelScheduled_NotFound_ErrNotFound`; `TestService_List_PropagatesLimitOffset`.

## Phase 5: Worker — Scheduler in-process (con tests)

- [x] 5.1 **NEW** `backend/internal/publications/worker.go` — definir `Scheduler` struct (`store PubStore`, `groups GroupReader`, `tg MessageSender`, `logs LogWriter`, `interval time.Duration`, `logger *slog.Logger`); constante `claimBatchSize = 25` y `DefaultSchedulerInterval = 30 * time.Second`.
- [x] 5.2 `worker.go` — `NewScheduler(...)` constructor + `Run(ctx context.Context) error` con `time.NewTicker(s.interval)` + `defer Stop()`; en cada tick: `store.ClaimScheduledDue(ctx, claimBatchSize)`; por cada fila invocar `publishOne(ctx, row.ActorID, row.TelegramID, row.Text, row.PhotoURL!=nil, row.PhotoURL, row.Buttons)`; `slog.Info("publications worker tick", "due", len(rows), "claimed", len(rows), "interval", s.interval.String())`; retorna `nil` cuando `ctx.Done()`.
- [x] 5.3 `worker.go` — método `tick(ctx)` privado para unit-tests deterministas (no esperar el ticker). Manejo de error SQL: `slog.Error` no aborta el loop; el próximo tick reintenta.
- [x] 5.4 **NEW** `backend/internal/publications/worker_test.go` — tests unit (con fakes de Phase 1): `TestScheduler_Tick_NoDue_NoSend` (`due=0` → cero `Send*`, log due=0); `TestScheduler_Tick_ClaimsAndPublishes` (2 due → ambas a `sending` → `sent` con `message_id`); `TestScheduler_Tick_TelegramError_StaysFailed`; `TestScheduler_Tick_PermissionDenied_StaysFailed` (bot member → `failed` con `error_message`); `TestScheduler_ContextCancel_ReturnsNil` (cancel mid-tick → `Run` retorna nil sin panic). Fakes `MessageSender`/`GroupReader`/`LogWriter`/`PubStore` ya extendidas en Phase 1.

## Phase 6: Handlers + server wiring + main

- [x] 6.1 `backend/internal/api/publications_handlers.go` — extender `handleCreatePublication` con `scheduledAt` opcional en body (parse RFC3339); si presente → `service.PublishMany(ctx, actorID, payload, &scheduledAt)` (rama schedule); si nil → flujo slice 2.
- [x] 6.2 `backend/internal/api/publications_handlers.go` — modificar `handleListPublications` para leer `limit`/`offset` query params, llamar `validatePagination` (helper de Phase 2), propagar al `service.List(...)` o `service.ListByTelegramID(...)`.
- [x] 6.3 `backend/internal/api/publications_handlers.go` — **nuevo** `handleDeletePublication(w, r)`: parse `:id`, `service.CancelScheduled(ctx, id)`; mapear `ErrNotFound` → 404, `ErrCancelNotAllowed` → 409 `INVALID_STATUS`, OK → 204 No Content.
- [x] 6.4 `backend/internal/api/publications_handlers.go` — extender mapping de errores nuevos (`ErrScheduledInPast`, `ErrInvalidPagination`, `ErrCancelNotAllowed`) al envelope `{code, message}` con status HTTP correcto (400/400/409).
- [x] 6.5 `backend/internal/api/server.go` — registrar ruta `DELETE /api/publications/{id}` (Go 1.22+ `r.PathValue` o chi/mux equivalente; consistente con estilo actual del archivo).
- [x] 6.6 `backend/internal/config/config.go` — agregar `PublicationsSchedulerInterval time.Duration` (parse de `PUBLICATIONS_SCHEDULER_INTERVAL_SECONDS`, default `30s`, validar `> 0`).
- [x] 6.7 `backend/cmd/server/main.go` — instanciar `publications.NewScheduler(store, groupsRepo, tgAdapter, logRepo, cfg.PublicationsSchedulerInterval)`; lanzar como goroutine con el mismo `ctx` (`signal.NotifyContext`) que `telegram.Poller` (`main.go:194-202`); log `publications scheduler started`; panic recovery consistente con el poller.
- [x] 6.8 `backend/internal/api/publications_handlers_test.go` — extender con tests: POST `scheduled_at` futuro → 201 + N filas `scheduled`; POST `scheduled_at` pasado → 400 `VALIDATION_ERROR`; POST offset `+03:00` normalizado a UTC; GET `?limit=10&offset=20` → 200 con slice correcto; GET `?limit=101` → 400; GET `?offset=-1` → 400; DELETE scheduled → 204; DELETE sent → 409 `INVALID_STATUS`; DELETE 404 NOT_FOUND.
- [x] 6.9 `.env.example` — agregar `PUBLICATIONS_SCHEDULER_INTERVAL_SECONDS=30` + comentario explicando su efecto.

## Phase 7: Backend suite green (gate)

- [x] 7.1 `cd backend && go test ./...` retorna exit 0.
- [x] 7.2 `cd backend && go vet ./...` retorna exit 0 sin warnings.
- [x] 7.3 `cd backend && gofmt -l .` retorna sin output (todo formatted).
- [ ] 7.4 Smoke manual opcional: `docker compose up -d postgres` + `curl` POST con `scheduled_at` futuro y verificar fila en DB.

## Phase 8: Frontend — tipos, api, hooks, error, helper, página, tests

- [x] 8.1 `frontend/src/features/publications/types.ts` — extender `PublishRequest` con `scheduled_at?: string`; extender `PublicationsFilter` con `limit?: number`, `offset?: number`; agregar campo `scheduled_at: string | null` al tipo `Publication` (TIMESTAMPTZ nullable).
- [x] 8.2 `frontend/src/features/publications/api.ts` — `listPublications({group_id?, limit, offset})` arma `?limit&offset&group_id`; **nuevo** `cancelPublication(id: number): Promise<void>` (DELETE → maneja 204 sin body).
- [x] 8.3 `frontend/src/features/publications/hooks.ts` — `usePublications` query key `['publications', {group_id: group_id ?? 'all', limit, offset}]` (incluye ambos params); **nuevo** `useCancelPublication()` mutation → `cancelPublication(id)` + `queryClient.invalidateQueries(['publications'])`.
- [x] 8.4 `frontend/src/features/publications/error.ts` — `formatPublicationsError` mapea `ErrScheduledInPast`, `ErrCancelNotAllowed`, `ErrInvalidPagination` a mensajes en español.
- [x] 8.5 **NEW** `frontend/src/features/publications/validateScheduledAtClient.ts` — helper `validateScheduledAtClient(localValue: string, now: Date): { ok: boolean; iso?: string; error?: string }` (parsea `datetime-local`, normaliza a ISO UTC con offset, valida estrictamente futuro).
- [x] 8.6 `frontend/src/pages/PublicationsPage.tsx` — agregar radio/segmented "Publicar ahora" / "Programar" (default "ahora"); cuando "Programar", mostrar `<input type="datetime-local" name="scheduled_at">`; submit label dinámico ("Publicar" vs "Programar"); en cada fila con `status === 'scheduled'` agregar botón **Cancelar** con confirmación inline → `useCancelPublication().mutate(id)`; controles **Prev/Next** al pie (Prev disabled si `offset===0`, Next disabled si `data.length < limit`); columna `scheduled_at` formateada en filas `scheduled`.
- [x] 8.7 `frontend/src/test/helpers.tsx` — extender `mockFetchRoutes`: helper para responder `204` sin body en DELETE; soporte para `?limit&offset` en GET (routing por query string).
- [x] 8.8 `frontend/src/pages/PublicationsPage.test.tsx` — nuevos tests: "modo Programar muestra datetime-local"; "validación cliente fecha pasada bloquea POST"; "Cancelar fila scheduled ejecuta DELETE"; "Cancelar NO visible en sent/failed"; "Next deshabilitado al final real (length<limit)"; "Next deshabilitado al final exacto (length===limit)"; "Next avanza offset (query key cambia)"; "Prev deshabilitado cuando offset===0".

## Phase 9: Frontend suite green (gate)

- [x] 9.1 `cd frontend && npm test -- --run` retorna exit 0 (todos los tests slice 2 + nuevos verdes).
- [x] 9.2 `cd frontend && npm run build` retorna exit 0 (Vite build limpio; type-check pasa).

## Phase 10: README + commit final

- [x] 10.1 `README.md` — sub-sección **"Programación"** dentro de "Publicaciones": `scheduled_at` opcional (RFC3339, UTC), worker in-process revisa cada 30 segundos, cancelación solo de filas `scheduled` (DELETE), **no** auto-retry de `failed`. Sub-sección **"Paginación"**: `?limit` default 50 max 100, `?offset` default 0. **Nota explícita**: "Telegram no soporta scheduling nativo desde bots; el panel programa in-process con un worker que revisa cada 30 segundos".
- [ ] 10.2 Commit final conventional (chore/feat), NO push (single-pr local con `size:exception`); si se rompe en 2 PRs, PR 1 incluye 1.1–7.3 + commit, PR 2 incluye 8.1–10.2 + commit.

---

## Verification (final)

- Backend `go test ./...` / `go vet ./...` / `gofmt -l .` verdes.
- Frontend `npm test -- --run` / `npm run build` verdes.
- §21.1 audit: cero llamadas a Bot API real en tests; `TelegramService` mockeado en service/worker/handler.
- Smoke: `POST {group_ids:[g1,g2], scheduled_at:"2027-01-01T10:00:00Z"}` → 201 con 2 filas `status='scheduled'`, **cero** `Send*` invocadas.
- Smoke: `POST scheduled_at=2020-01-01T00:00:00Z` → 400 `VALIDATION_ERROR`.
- Integration: `Scheduler.Run` con `interval=5ms` procesa 1 fila due → `sent` con `message_id`; 2 transacciones concurrentes en `ClaimScheduledDue` no se pisan ids (SKIP LOCKED).
- Smoke: `DELETE /api/publications/<scheduled-id>` → 204; `DELETE /api/publications/<sent-id>` → 409 `INVALID_STATUS`.
- Smoke: `GET /api/publications?limit=10&offset=20` retorna slice correcto; `?limit=101` → 400; `?offset=-1` → 400.
- Frontend: radio "Programar" muestra datetime-local; Cancelar visible solo en `scheduled`; Prev/Next funcionales.
- Bugfix #172 vigente: `permissionOk` sigue con `g.BotStatus == StatusAdministrator`; tests de worker cubren fila `failed` cuando bot deja de ser admin.
