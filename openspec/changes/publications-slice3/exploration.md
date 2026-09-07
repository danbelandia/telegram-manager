# Exploration: Publications — Slice 3 (Programación + Historial)

> **Change**: `publications-slice3` (Slice 3 de 3). Modo hybrid.
> **Path**: `openspec/changes/publications-slice3/exploration.md` (no pisa los archivos archivados de slice 1/2).
> **Persisted**: `sdd/publications-slice3/exploration` (Engram) + este filesystem.
> **Estrategia propuesta**: single-pr (igual que slices 1 y 2), size-exception si la cifra de LOC supera 400.

---

## Contexto heredado (base obligatoria)

**Slice 1** (archivado `2026-09-07-publications/publications/`, merge `ace1f59`):

- Tabla `publications` (migración `00004`) con `scheduled_at TIMESTAMPTZ NULL` y `status` enum que YA incluye `scheduled` (ver `migrations/00004_create_publications.sql:11-12`).
- `telegram.SendMessage(ctx, chatID, text, disablePreview)` añadido al adapter.
- `internal/publications/{model,repository,service}.go`: `Publish` (single-group sync), `List`, `GetByID`.
- `POST /api/publications` crea y publica inmediatamente (single-group).
- `GET /api/publications` (max 50) + `GET /api/publications/:id`.

**Bugfix #172** (topic `sdd/publications/permission-check`): `permissionOk` usa `g.BotStatus == groups.StatusAdministrator`. La detección de grupos NUNCA puebla `can_*`. **Regla**: NO reintroducir checks sobre claves `can_*` para publicaciones; usar SIEMPRE `bot_status==administrator`.

**Slice 2** (archivado `2026-09-07-publications-slice2/`, merge `b90c584`):

- Migración `00005`: ALTER ADD `photo_url TEXT`, `buttons JSONB`.
- `telegram.SendPhoto(ctx, chatID, photoURL, caption, keyboard *InlineKeyboardMarkup)`; `SendMessage` extendido con `keyboard`.
- `PublishMany` multi-grupo SECUENCIAL sobre `group_ids` (max 10). Validación fail-fast 400. Fallo parcial → 201 con cada fila.
- `POST /api/publications` body: `{text, photo_url?, buttons?, group_ids[]}`. Respuesta: `{publications: [...]}`.
- `GET /api/publications?group_id=X` filtra por grupo (índice `idx_publications_telegram_id`).
- Frontend: foto, editor de botones, multi-select, filtro por grupo, preview de foto/botones en historial.

**Estado actual verificable (main @ b90c584)**:

- `publications.Publication.ScheduledAt *time.Time` (model.go:62) — ya existe, **no requiere migración de esquema**.
- `PubStore` interface (service.go:70-76): `Create, GetByID, List, ListByTelegramID, UpdateStatus` — **falta** `ClaimScheduledDue` y `Cancel`.
- `Service` (service.go:91): orquesta `Publish` (single) + `PublishMany` (multi). **Falta** scheduling.
- `handleCreatePublication` (publications_handlers.go:75): siempre llama `PublishMany` (síncrono). **Falta** rama "schedule".
- `maxListLimit = 50` hardcoded (repository.go:13) — slice 2 NO introdujo paginación; ahora la agregamos.
- Frontend query key: `['publications', group_id ?? 'all']` (hooks.ts:17) — listo para extender con offset.
- Frontend status badge ya incluye `scheduled` y `sending` (PublicationsPage.tsx:21-27).
- `mockFetchRoutes` (helpers.tsx:43) despacha por substring de URL — sirve para paginación y DELETE.

**Patrón de goroutine ya en uso** (`cmd/server/main.go:194-202` + `telegram/poller.go`):

```go
poller := telegram.NewPoller(bot, telegram.WithPollerLogger(slog.Default()))
pollerErrCh = make(chan error, 1)
go func() {
    pollerErrCh <- poller.Run(ctx, func(updates []telegram.Update) {
        for i := range updates { bus.Publish(&updates[i]) }
    })
}()
```

Mismo `ctx` de `signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)` cierra el worker limpiamente (10s timeout en `httpServer.Shutdown`).

**Stack confirmado**:

- PostgreSQL 16 (`docker-compose.yml:3` → `postgres:16-alpine`).
- Driver `pgx/v5` (`backend/internal/database/database.go:11`: `_ "github.com/jackc/pgx/v5/stdlib"`).
- Standard `database/sql` (no ORM); pgx stdlib soporta `SELECT ... FOR UPDATE SKIP LOCKED` (Postgres ≥ 9.5).

---

## Hechos confirmados de la Bot API

- **`sendMessage` NO acepta `schedule_date`**. Evidencia:
  - Adapter actual `backend/internal/telegram/publications.go:18-23` define `sendMessageParams{ChatID, Text, DisableWebPagePreview, ReplyMarkup}` — sin campo `schedule_date`.
  - `sendPhotoParams{ChatID, Photo, Caption, ReplyMarkup}` (publications.go:29-34) — idem.
  - Comentario explícito en `backend/internal/telegram/publications.go:10`: "*NO existe `schedule_date` para bots en grupos (exploration; se hace in-process con worker Go)*".
  - Observación #161 (explore slice 1): "*NO existe `schedule_date` para bots enviando a grupos: la programación debe hacerse in-process*".
  - No existe ningún método de la Bot API para programar envíos de bots a grupos.

- **Implicación**: la programación DEBE implementarse in-process (worker Go). No hay atajo de la API.

---

## Decisiones (con tradeoffs)

### D1 — Schema: **sin migración nueva**

- `scheduled_at TIMESTAMPTZ NULL` ya existe (00004:14).
- Status enum ya incluye `scheduled` (00004:11-12).
- Paginación como query param (`?limit=&offset=`), no columna.

**Tradeoff**: ninguna contra una migración. No agrega valor y rompe invariante "una tabla".

### D2 — Programación del POST: `scheduled_at` opcional en el body

```
POST /api/publications
{text, photo_url?, buttons?, group_ids[], scheduled_at?}
```

- Si `scheduled_at == nil` → comportamiento actual (publica ya vía `PublishMany`).
- Si `scheduled_at != nil`:
  - Validar RFC3339; convertir a UTC.
  - Si `scheduled_at <= now()` → **400 VALIDATION_ERROR** "scheduled_at debe ser futuro" (sin crear fila).
  - Si futuro → insertar fila por grupo con `status='scheduled'`, `scheduled_at=...`. NO llamar a Telegram.
  - Respuesta: 201 con `{publications: [N filas status='scheduled']}` (mismo shape que el path inmediato).
- Validación fail-fast UNA vez al inicio (consistente con `validatePayload` de slice 2).

**Tradeoff**: mantener un único endpoint vs. separar `POST /api/publications/scheduled`. **Decisión**: mismo endpoint + branching por presencia del campo. Costo: una rama más en `handleCreatePublication`. Beneficio: API más simple; los admins no necesitan aprender dos endpoints.

### D3 — Worker: **`time.Ticker(30 * time.Second)`** + goroutine única

- Lanzado en `cmd/server/main.go` junto al poller (mismo `ctx` de `signal.NotifyContext`).
- Batch por tick: hasta 25 filas (baja el RTT contra Telegram; 25 chats × 1 msg/seg/ch ≈ 25 seg ≈ 1 tick).
- Estructura nueva: `backend/internal/publications/worker.go` con `type Scheduler struct { store PubStore; svc MessageSender; logs LogWriter; groups GroupReader; logger *slog.Logger }` + `Run(ctx context.Context) error`.
- Lifecycle: `Run` retorna `nil` cuando ctx se cancela (no propagar como error fatal).

**Alternativas consideradas**:
- Ticker 60s: descarta ticks y rompe UX (cancelar scheduled puede esperar hasta 60s).
- Ticker 5s: martilla la DB; 25× cada 5s = 5 qps promedio solo del worker (innecesario).
- Cron externo: introduce infraestructura, viola AGENTS §2.

**Decisión**: 30s. Balance entre reactividad y carga.

### D4 — Concurrencia del worker: **`SELECT ... FOR UPDATE SKIP LOCKED`** dentro de txn

```sql
BEGIN;
SELECT id, telegram_id, text, photo_url, buttons, scheduled_at
  FROM publications
 WHERE status = 'scheduled' AND scheduled_at <= now()
 ORDER BY scheduled_at ASC
 LIMIT 25
 FOR UPDATE SKIP LOCKED;
-- Para cada row: UPDATE publications SET status='sending' WHERE id=...
COMMIT;
```

**Prerrequisitos verificados**: Postgres 16 ✅ (`docker-compose.yml:3`), pgx/v5 stdlib ✅ (`backend/internal/database/database.go:11`), `database/sql` puro (sin ORM). `FOR UPDATE SKIP LOCKED` es Postgres ≥ 9.5 (canónico para job queues Go/Postgres).

**Alternativas consideradas**:
- `SELECT` ingenuo + `UPDATE … WHERE status='scheduled'`: race condition entre worker y un POST inmediato concurrente para la misma fila. Riesgo bajo (filas nuevas son futuras, no chocan) pero existe.
- Advisory lock (`pg_try_advisory_xact_lock`): innecesario en monolito de una instancia.
- Distributed lock con Redis: viola AGENTS §2.

**Decisión**: SKIP LOCKED. Sin Redis, sin overhead, idiomático.

### D5 — Lógica de envío del worker (reutiliza `publishOne`)

Tras COMMIT de la reserva, por cada fila:

1. Releer fila (puede haber cambiado el grupo tras la reserva).
2. Verificar existencia del grupo (`GetByTelegramID`) + `permissionOk`. Si falla → `UpdateStatus(failed, errMsg)` + log `PUBLISH_MESSAGE/PERMISSION_DENIED`. **Continúa con la siguiente fila**.
3. Llamar `SendPhoto(photo_url, caption, keyboard)` o `SendMessage(text, keyboard)` según `photo_url` (mismo dispatch que `PublishMany`).
4. `UpdateStatus(sent, messageID)` o `UpdateStatus(failed, errMsg)` + log.

**Factor clave**: el método `publishOne` (service.go:203) ya implementa exactamente este flujo y respeta `permissionOk`. **Reutilización literal** — el worker NO reimplementa el path de envío, llama `publishOne` por fila reservada.

### D6 — Retry en fallo de Telegram: **NO hay retry automático**

- Fila queda `failed` con `error_message` legible.
- Operador debe crear una nueva publicación.

**Justificación** (consistente con slice 1/2): el contrato "publish" es "lo intentamos una vez; si falla, queda registro". Auto-retry sorprende al admin y duplicaría mensajes si el problema es lógico (ej. caption > 1024). Documentar en README.

### D7 — Cancelación: `DELETE /api/publications/:id`

- Permitido SOLO si `status='scheduled'`. Hard delete.
- 204 No Content en éxito.
- 409 Conflict con `code='INVALID_STATUS'` si status ∈ {sending, sent, failed}.
- 404 NOT_FOUND si no existe.

**Tradeoff**:
- Soft delete (status='cancelled'): requiere migración para nuevo valor del enum y más código. NO se justifica para una cancelación de un scheduled que aún no se envió.
- Hard delete programado: pierde audit trail para filas que AÚN no se enviaron. **Riesgo bajo** porque "cancelar" implica "no quiero que salga" — el admin sabe que la fila desaparece.

**Decisión**: hard delete solo para `scheduled`. **Documentar** que cancelar `sent`/`failed` no está permitido y por qué (preservar audit trail).

### D8 — Paginación del listado: `?limit=&offset=`

- Default `limit=50`, max `100`. Default `offset=0`.
- 400 VALIDATION_ERROR si limit<1, limit>100, offset<0.
- Compatible con el filtro `?group_id=` existente.
- **Sin** status filter en MVP (la lista muestra todos los estados: scheduled, sending, sent, failed).

**Alternativas consideradas**:
- Cursor-based (más complejo; no se justifica para 50-100 filas).
- Status filter: agrega UI complexity; mejor empezar simple. Documentar como follow-up.

### D9 — Cambios al adapter: **ninguno**

- `SendMessage` y `SendPhoto` ya soportan foto, caption y botones.
- El worker llama a los mismos métodos slice 2.

### D10 — Logs

- Por tick: `slog.Info("publications worker tick", "due", N, "claimed", M)` con N=row count del query, M=rows transitioned to sending.
- Por fila enviada: log existente `ActionPublishMessage` (slice 1). **Sin nuevas constantes de log**.
- Errores del worker: `slog.Error("publications worker tick error", "error", err)`.

### D11 — Frontend

- Form: nuevo `<input type="datetime-local" name="scheduled_at">` (opcional). Validación cliente: si está seteado, debe ser futuro.
- Botón submit cambia texto dinámicamente: "Publicar ahora" (sin scheduled_at) o "Programar" (con scheduled_at).
- Listado: controles de paginación Prev/Next usando offset (botones disabled cuando no hay más datos).
- Cancel button en cada fila con status='scheduled': `DELETE /api/publications/:id`.
- React Query: `usePublications({group_id, limit, offset})` con key `['publications', group_id ?? 'all', limit, offset]`. `useCancelPublication()` mutación que invalida `['publications']`.

### D12 — Tests

- **Backend service**:
  - `TestService_Schedule_InsertsScheduled` (no llama Telegram).
  - `TestService_Schedule_PastDate_ReturnsErrScheduledInPast` (400).
  - `TestService_CancelScheduled_OK`, `TestService_CancelSent_ReturnsErrCancelNotAllowed`.
- **Backend repository**:
  - `TestRepository_ClaimScheduledDue_BatchSize` (integración con Postgres real).
  - `TestRepository_ClaimScheduledDue_SkipsLockedByAnotherTxn` (concurrencia).
  - `TestRepository_List_LimitOffset` (paginación).
- **Backend worker** (`worker_test.go`):
  - Tick con N due rows, M<N claimed, others stay scheduled.
  - Mark sending + send + update sent.
  - Telegram error → mark failed, log.
  - Permission denied → mark failed, log PERMISSION_DENIED.
  - ctx cancel → return nil.
- **Backend handler** (extend `publications_handlers_test.go`):
  - POST con `scheduled_at` futuro → 201, status='scheduled', no Telegram call.
  - POST con `scheduled_at` pasado → 400.
  - DELETE permitido (status=scheduled) → 204.
  - DELETE bloqueado (status=sent) → 409.
  - GET `?limit=10&offset=20` → 200 con slice correcto.
- **Frontend** (`PublicationsPage.test.tsx`):
  - Scheduling form con datetime-local.
  - Pagination prev/next deshabilita correctamente.
  - Cancel button para fila scheduled.

### D13 — README: sección "Programación" añadida a "Publicaciones"

- Cómo programar (datetime picker).
- Cómo cancelar.
- **Nota explícita**: "Telegram no soporta scheduling nativo desde bots; el panel programa in-process con un worker que revisa cada 30 segundos".
- Límite: cancelación solo de filas `scheduled` (no `sent`/`failed`).

---

## Affected Areas

### Backend

| Archivo | Acción | LOC est. | Notas |
|---------|--------|----------|-------|
| `backend/internal/publications/model.go` | Modificar | +20 | `ErrScheduledInPast`, `ErrCancelNotAllowed`, helper `validateScheduledAt` |
| `backend/internal/publications/repository.go` | Modificar | +60 | `ClaimScheduledDue(ctx, limit)` con SKIP LOCKED, `Cancel(ctx, id)`, `List(ctx, limit, offset)`, `ListByTelegramID(ctx, gid, limit, offset)` |
| `backend/internal/publications/service.go` | Modificar | +100 | `Schedule(ctx, actor, payload)` (variante de `PublishMany` que setea status=scheduled + scheduled_at, NO llama Telegram); `Cancel(ctx, id)` con guard de status |
| `backend/internal/publications/worker.go` | **Crear** | +130 | `Scheduler`, `Run(ctx)`, tick loop, batch processing |
| `backend/internal/publications/service_test.go` | Modificar | +80 | Tests de Schedule + Cancel + validaciones |
| `backend/internal/publications/worker_test.go` | **Crear** | +120 | Tests del worker (tick, claim, send, error paths) |
| `backend/internal/api/publications_handlers.go` | Modificar | +90 | `handleCreatePublication` branching por `scheduled_at`; `handleDeletePublication`; `handleListPublications` con `?limit&offset` |
| `backend/internal/api/publications_handlers_test.go` | Modificar | +90 | Tests de scheduling + DELETE + paginación |
| `backend/internal/api/server.go` | Modificar | +5 | `WithPublications` agrega DELETE route |
| `backend/cmd/server/main.go` | Modificar | +15 | Lanzar `publications.Scheduler.Run(ctx)` como goroutine, errCh propagation |

### Frontend

| Archivo | Acción | LOC est. |
|---------|--------|----------|
| `frontend/src/features/publications/types.ts` | Modificar | +10 |
| `frontend/src/features/publications/api.ts` | Modificar | +15 |
| `frontend/src/features/publications/hooks.ts` | Modificar | +20 |
| `frontend/src/features/publications/error.ts` | Modificar | +10 |
| `frontend/src/pages/PublicationsPage.tsx` | Modificar | +120 |
| `frontend/src/pages/PublicationsPage.test.tsx` | Modificar | +100 |

### Docs

- `README.md`: sección "Programación" dentro de "Publicaciones" — +25 líneas.

**Estimación total**: ~900 LOC touched. **Excede el budget de 400** — **DECIDE en tasks**: single-pr con size-exception, o split en backend (con worker) / frontend + README. La historia muestra que slices 1 y 2 se entregaron como single-pr con exception; misma estrategia recomendada.

---

## Enfoques alternativos (rechazados)

### Opción A — **Worker con SKIP LOCKED** ✅ RECOMENDADO

- Pros: canónico, sin infraestructura, aprovecha Postgres 16. Una sola instancia, monolito (sin lock distribuido).
- Cons: requiere transacción explícita en el repository.
- Esfuerzo: Medium.

### Opción B — Naive SELECT + UPDATE sin locking

- Pros: simple.
- Cons: race con POST inmediato concurrente para misma fila (improbable pero existe). No es canónico para job queues.
- **Rechazada**: el costo de SKIP LOCKED es despreciable; no hay razón para asumir riesgo.

### Opción C — Servicio externo (cron, Kubernetes CronJob, sidecar)

- Pros: no toca el backend.
- Cons: introduce infraestructura. Viola AGENTS §2.
- **Rechazada**.

### Opción D — Un único mega-change Fase 2 (sin slicing)

- Pros: un solo ciclo.
- Cons: ~1500+ LOC; excede budget por mucho; imposible de verificar incrementalmente.
- **Rechazada**: la decisión de los 3 slices se tomó en slice 1 (observation #161) y se ejecutó con éxito en slices 1 y 2.

---

## Risks

| Riesgo | Likelihood | Impact | Mitigation |
|--------|-----------|--------|-----------|
| Worker tick coincide con POST inmediato del mismo admin para un grupo | Low | Low | El POST inmediato crea una NUEVA fila (con status='sending'), no toca filas 'scheduled'. Sin colisión. |
| Worker acumula delay si el batch excede el tick (25 filas × 1 seg/ch ≈ 25s vs. 30s tick) | Medium | Low | Documentar: el tick siguiente recoge las restantes. Nunca se pierde. |
| Restart deja filas `scheduled` con `scheduled_at` pasado | Medium | Low | Próximo tick las recoge; comportamiento correcto, sin lógica extra. |
| Cancelación concurrente con worker tick | Low | Medium | `Cancel` verifica `status='scheduled'` ANTES de borrar (operación no-transaccional). Si el worker ya marcó `sending`, `Cancel` retorna 409. Aceptable. |
| Schema status enum: añadir valor futuro requeriría migración | Low | Low | No agregamos `cancelled` (decisión D7). Si en futuro se necesita, es trivial. |
| Bot admin removido mientras hay scheduled pendiente | Low | Medium | El worker relee el grupo y aplica permissionOk (D5). Falla con PERMISSION_DENIED legible. |
| Single-pr excede 400 LOC | High | Medium | Size-exception (como slices 1 y 2). Si la estimación es ~900 LOC, considerar chained PRs: P1 backend+worker, P2 frontend+README. **DECIDE en tasks**. |
| pgx stdlib + SKIP LOCKED: ¿soporta `FOR UPDATE SKIP LOCKED` correctamente? | Low | Low | Confirmado por documentación de pgx (https://github.com/jackc/pgx/blob/master/stdlib/sql.go) + uso extendido en Go ecosystem. Tests de integración en repository_test.go lo cubren. |

---

## Ready for Proposal

**Sí.** Informar al usuario:

> **Slice 3 — Programación + Historial** agrega:
> 1. **`scheduled_at` opcional en `POST /api/publications`**: si está en el futuro, la fila queda `scheduled` (NO se envía). Si está en el pasado, 400.
> 2. **Worker in-process** (goroutine en `cmd/server/main.go`): ticker 30s, batch de 25, `SELECT ... FOR UPDATE SKIP LOCKED` (Postgres 16 ✓, pgx/v5 ✓). Reutiliza `publishOne` para el envío. NO reintenta automáticamente filas `failed`.
> 3. **`DELETE /api/publications/:id`** para cancelar filas `scheduled` (hard delete; 409 para otros status).
> 4. **Paginación** del historial: `?limit=` (default 50, max 100), `?offset=`. Compatible con `?group_id=`.
> 5. **Frontend**: datetime-local input para programar, controles Prev/Next, botón Cancelar en filas `scheduled`.
> 6. **README**: sección "Programación" + nota explícita de que Telegram no soporta scheduling nativo.
>
> **Confirmación Bot API**: `sendMessage` NO acepta `schedule_date` — el scheduling es 100% in-process.
>
> **Schema**: sin migración nueva. `scheduled_at` y `scheduled` ya existen desde 00004.
>
> **LOC est.**: ~900 → **size-exception esperada** (igual que slices 1 y 2); si se prefiere split, chained PR backend→frontend+README.
>
> ¿Procedemos con la propuesta?