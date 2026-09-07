# Proposal: Publications — Slice 3 (Programación + Historial)

## Intent

Cerrar `publications` (AGENTS §22) con **programación de envíos** y **paginación del historial**. La Bot API **NO soporta `schedule_date`** para bots en grupos (evidencia en `telegram/publications.go:10`, observación #161, slice-1 #162): la programación DEBE implementarse **in-process** con un worker Go (ticker + `SELECT ... FOR UPDATE SKIP LOCKED`). Reutiliza el helper `publishOne` (slice-2 design #176 / verify #181 — comportamiento observable idéntico, ya respeta bugfix #172 `bot_status==administrator`).

## Scope

### In Scope

- **Worker in-process** `backend/internal/publications/worker.go` con `Scheduler{store PubStore, groups GroupReader, tg MessageSender, log LogWriter, interval time.Duration}.Run(ctx)`: ticker configurable (prod 30s, tests 5ms), batch 25, `SELECT ... FOR UPDATE SKIP LOCKED` en transacción (Postgres 16 + pgx/v5 stdlib). Lifecycle: goroutine en `cmd/server/main.go` con el mismo `signal.NotifyContext` que el poller; `Run` retorna `nil` cuando `ctx.Done()`.
- **POST `/api/publications`** extendido: body conserva `{text, photo_url?, buttons?, group_ids[]}` y agrega opcional `scheduled_at` (RFC3339, UTC). Branching en handler:
  - `scheduled_at == nil` → comportamiento actual (`PublishMany` síncrono).
  - `scheduled_at != nil && futuro` → validar payload igual que publish-now; insertar 1 fila por grupo con `status='scheduled'`, `scheduled_at`, **NO** llamar a Telegram. Respuesta 201 `{publications: [...]}` (mismo shape).
  - `scheduled_at <= now()` → **400 `ErrScheduledInPast`** sin crear fila.
- **GET `/api/publications?group_id=&limit=&offset=`**: paginado. `limit` default 50, max 100; `offset` default 0. **400 `ErrInvalidPagination`** si `limit<1 || limit>100 || offset<0`. Compatible con filtro `?group_id=` de slice 2. **Sin** status filter.
- **DELETE `/api/publications/:id`**: cancelación. Permitido **solo si `status='scheduled'`** → **hard delete + 204**. Otros status (`sending|sent|failed`) → **409 `ErrCancelNotAllowed`** con `code='INVALID_STATUS'`. Inexistente → 404.
- **Errores nuevos**: `ErrScheduledInPast` (400), `ErrCancelNotAllowed` (409), `ErrInvalidPagination` (400). Mapeo a envelope estándar (`code`, `message`).
- **Frontend `PublicationsPage`**:
  - Form **dual-mode**: radio "Publicar ahora" / "Programar". Cuando "Programar" → muestra `<input type="datetime-local">`; validación cliente "debe ser en el futuro"; al blur sincroniza al state (`scheduled_at`).
  - Listado: badges ya cubren `scheduled/sending/sent/failed` (slice 2). **Nueva columna acción** con botón **"Cancelar"** visible **solo** cuando `status === 'scheduled'` → `DELETE /api/publications/:id` con confirmación inline.
  - **Paginación**: botones **Prev / Next** controlando `offset` (state local); deshabilitados cuando la página returned < `limit`. `limit` fijo en 50 (default backend).
  - React Query: `usePublications({group_id, limit, offset})` con key `['publications', group_id ?? 'all', limit, offset]`; `useCancelPublication()` mutation que invalida `['publications']` (todas las variantes).
- **Tests** (§21.1 estricto — fakes, cero Bot API real):
  - **Service**: `TestService_Schedule_InsertsScheduled_SinLlamarTelegram`; `TestService_Schedule_PastDate_ErrScheduledInPast`; `TestService_CancelScheduled_OK`; `TestService_CancelSent_ErrCancelNotAllowed`.
  - **Repository** (integración Postgres real): `TestRepository_ClaimScheduledDue_BatchSize`; `TestRepository_ClaimScheduledDue_SkipsLockedByAnotherTxn`; `TestRepository_List_LimitOffset_Pagina`; `TestRepository_Cancel_DeleteRow`.
  - **Worker** (`worker_test.go` NEW): tick con N due rows, M<N claimed, resto queda `scheduled`; mark `sending` → send success → `sent`; send failure → `failed` + log `TELEGRAM_ERROR`; permission denied → `failed` + log `PERMISSION_DENIED`; `ctx` cancelado → retorna `nil` (sin error fatal).
  - **Handler**: POST con `scheduled_at` futuro → 201; POST con `scheduled_at` pasado → 400; DELETE scheduled → 204; DELETE sent → 409; GET `?limit=10&offset=20` → 200 con slice correcto.
  - **Frontend**: datetime-local renderiza y valida "en el futuro"; botón Cancelar visible solo en `scheduled`; Prev/Next deshabilitados según `length<limit`.
- **README**: nueva sub-sección **"Programación"** dentro de "Publicaciones" + nota explícita: "Telegram no soporta scheduling nativo desde bots; el panel programa in-process con un worker que revisa cada 30 segundos". Límite: cancelación solo de filas `scheduled`.

### Out of Scope

- Auto-retry de filas `failed` (consistente con slices 1/2: el operador crea una nueva).
- Status filter en el listado (`?status=`) — UI complexity sin valor en MVP.
- Soft delete / status `cancelled` — `DELETE` solo aplica a `scheduled`.
- Editor recurrente (cron-like), zonas horarias por grupo, paginación cursor-based.
- `publishMediaGroup`, `editMessage`, callbacks — cubiertos por futuras fases.

## Capabilities

### Modified Capabilities

- `publications` (delta): worker in-process + scheduling (`scheduled_at`) + `DELETE` + paginación. Mismas invariantes: `permissionOk` usa `bot_status==administrator` (#172); `publishOne` compartido (slice-2 verify #181); `PublishMany` secuencial (§18.1).

### New Capabilities

_Ninguna._ Las REQs viven como extensiones del spec canónico `openspec/specs/publications/spec.md` ("Slice 3 Extensions", mismo patrón que slice 2).

## Approach

| # | Decisión | Cómo |
|---|----------|------|
| 1 | **Sin migración** | `scheduled_at TIMESTAMPTZ NULL` y `status='scheduled'` ya existen desde 00004 (`migrations/00004_create_publications.sql:11-12,14`). Tabla única. |
| 2 | **Mismo endpoint `POST /api/publications`** | `scheduled_at` opcional en body. Branching en handler. Sin nuevo endpoint `POST /api/publications/scheduled` (UX simple). |
| 3 | **Worker con `time.Ticker`** | 30s prod, configurable para tests (`interval time.Duration` en struct). Batch 25 (25 chats × 1 msg/seg ≈ 25s ≤ 1 tick). Sin Redis, sin cron externo (§2). |
| 4 | **Concurrencia `SELECT ... FOR UPDATE SKIP LOCKED`** | En txn explícita; Postgres ≥ 9.5 (16 ✓). `pgx/v5` stdlib soporta nativamente. Canónico para job queues Go/Postgres. |
| 5 | **Worker reusa `publishOne`** | Helper privado de slice 2 (`service.go:203`). NO reimplementa path de envío. Ya respeta `permissionOk` (#172) y logging. |
| 6 | **Sin auto-retry** | Falla → `failed` + `error_message` legible. Operador decide. Consistente con slices 1/2. |
| 7 | **`DELETE` solo para `scheduled` (hard delete)** | `sent`/`failed` preservan audit. Status check en service antes de borrar. 204/404/409. |
| 8 | **Paginación `?limit=&offset=`** | Default 50, max 100. Sin cursor. Compatible con `?group_id=`. Sin status filter en MVP. |
| 9 | **Sin cambios al adapter** | `SendMessage`/`SendPhoto` ya cubren foto+caption+botones (slice 2). Worker llama lo mismo. |
| 10 | **Logs**: tick (`due N, claimed M`) + `ActionPublishMessage` por fila | Sin nuevas constantes. |
| 11 | **Frontend dual-mode form + Cancelar + Prev/Next** | datetime-local nativo; badges ya muestran status (slice 2). |
| 12 | **Tests con fakes** (§21.1) | Repository: Postgres real (integration). Service/Worker/Handler/Frontend: fakes. Cero Bot API real. |
| 13 | **README: sección Programación + nota "Telegram no soporta scheduling nativo"** | Disipar preguntas del admin. |

## Affected Areas

| Area | Impact |
|------|--------|
| `backend/internal/publications/model.go` | Modified (+20 — `ErrScheduledInPast`, `ErrCancelNotAllowed`, `ErrInvalidPagination`, `validateScheduledAt`) |
| `backend/internal/publications/repository.go` | Modified (+60 — `ClaimScheduledDue` SKIP LOCKED, `Cancel`, `List`/`ListByTelegramID` con `limit/offset`) |
| `backend/internal/publications/service.go` | Modified (+100 — `Schedule(ctx, actor, payload)`, `CancelScheduled(ctx, id)`, branching por `scheduled_at`) |
| `backend/internal/publications/worker.go` | **New** (+130 — `Scheduler` struct, `Run(ctx)`, tick loop, dispatch via `publishOne`) |
| `backend/internal/publications/service_test.go` | Modified (+80) |
| `backend/internal/publications/worker_test.go` | **New** (+120) |
| `backend/internal/publications/repository_test.go` | Modified (+60 — integration tests SKIP LOCKED + paginación) |
| `backend/internal/api/publications_handlers.go` | Modified (+90 — branching `scheduled_at`, `handleDeletePublication`, paginación) |
| `backend/internal/api/publications_handlers_test.go` | Modified (+90) |
| `backend/internal/api/server.go` | Modified (+5 — DELETE route) |
| `backend/cmd/server/main.go` | Modified (+15 — lanzar `Scheduler.Run(ctx)` como goroutine, `errCh`) |
| `frontend/src/features/publications/{types,api,hooks,error}.ts` | Modified (+55) |
| `frontend/src/pages/PublicationsPage.tsx` | Modified (+120) |
| `frontend/src/pages/PublicationsPage.test.tsx` | Modified (+100) |
| `README.md` | Modified (+25 — sección Programación) |

**Total estimado**: ~900 LOC touched. **Excede 400-line budget**.

## Risks

| Risk | Mitigation |
|------|------------|
| Race worker tick vs. `Cancel` concurrente | `Cancel` lee `status='scheduled'` ANTES de borrar; si worker ya marcó `sending`, retorna **409 `ErrCancelNotAllowed`**. Documentado. |
| Bot admin removido con fila `scheduled` pendiente | Worker relee grupo + `permissionOk` → `failed` con `error_message` legible (`PERMISSION_DENIED`). |
| Restart deja filas `scheduled` con `sched_at` pasado | Próximo tick las recoge; comportamiento correcto. |
| Batch > tick window (25×1s vs 30s) | Tick siguiente recoge restantes. Nunca se pierde. |
| PR > 400 LOC | **size:exception solicitada** (precedente slices 1+2). Si `tasks` forecast excede → chained PRs (backend+worker / frontend+README). **DECIDE en tasks**. |
| Cancelación de `sent`/`failed` | No permitido por diseño (preserva audit). 409 explícito. |
| `interval` mal configurado en prod | Constante `DefaultSchedulerInterval = 30 * time.Second`; `main.go` la pasa al constructor. Tests usan valor bajo. |

## Rollback

Sin migración → **rollback trivial**: `git revert` del merge. Tabla intacta (filas `scheduled` simplemente dejan de procesarse; admin las borra manualmente o ejecuta DELETE SQL). Frontend vuelve a formulario sin `scheduled_at`.

## Dependencies

- Slice 1 archivado (`ace1f59`) + slice 2 archivado (`b90c584`) + bugfix #172 (`9d4f1ec`).
- Helper `publishOne` (slice 2) — reutilización literal.
- `telegram.Service.SendMessage` + `SendPhoto` + token bucket + `doWithRetry`.
- Postgres ≥ 9.5 (16 ✓) + `pgx/v5` stdlib (soporta `FOR UPDATE SKIP LOCKED`).

## Delivery

**Single-pr, size:exception solicitada** (precedente slices 1+2). Branch: `feat/publications-slice3` base `main @ b90c584`. Forecast ~900 LOC → **DECIDE en tasks**: confirmar single-pr o split chained (backend+worker / frontend+README).

## Success Criteria

- [ ] `go test ./...` y `npm test -- --run` en verde; `go vet`/`gofmt`/`tsc --noEmit`/`vite build` limpios
- [ ] POST `{group_ids:[2], scheduled_at:"2026-09-08T10:00:00Z"}` → 201, 2 filas con `status='scheduled'`, **cero** llamadas a `SendMessage`/`SendPhoto`
- [ ] POST `scheduled_at` pasado → 400 `ErrScheduledInPast`
- [ ] Worker tick levanta 1 fila `scheduled` vencida → `sending` → `sent` con `message_id`; log `PUBLISH_MESSAGE/SUCCESS`
- [ ] Worker tick con error de Telegram → `failed` + `error_message` legible
- [ ] DELETE scheduled → 204; DELETE sent → 409; DELETE inexistente → 404
- [ ] GET `?limit=10&offset=20` → 200 con 10 filas (o menos si final); `limit=101` / `offset=-1` → 400
- [ ] Frontend: radio dual-mode; datetime-local bloquea envío si pasado; botón Cancelar solo en `scheduled`; Prev/Next funciona
- [ ] Fakes `Service`/`PubStore` actualizadas (flag explícito en tasks)
- [ ] Worker respeta §21.1: cero llamadas reales a Bot API en tests
- [ ] Bugfix #172 vigente: `permissionOk` sigue con `bot_status == administrator`

## Open Questions

Ninguna — la exploración resolvió todo (Bot API no soporta scheduling, SKIP LOCKED canónico, sin Redis, sin status filter, hard delete, intervalos fijos).