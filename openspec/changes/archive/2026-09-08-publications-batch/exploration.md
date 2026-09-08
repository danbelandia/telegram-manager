# Exploration: `publications-batch` — Publicación en lote

> **Change**: `publications-batch` — feature aditiva sobre Fase 2 (AGENTS §22). NO listada explícitamente en §22 ("Funciones futuras" menciona inmedita / programada / múltiples grupos, pero NO "en lote"); es una adición natural que aprovecha la tabla y servicios ya existentes.
> **Mode**: hybrid (filesystem + Engram).
> **Persisted**: `sdd/publications-batch/exploration` (Engram) + `openspec/changes/publications-batch/exploration.md` (filesystem).
> **Path**: `openspec/changes/publications-batch/exploration.md` (carpeta NUEVA, NO `publications-slice4`, para no chocar con paths archivados).

---

## Resumen ejecutivo (1 línea)

Nuevo endpoint `POST /api/publications/batch` que acepta `N≤10` publicaciones independientes (cada una con su `scheduled_at?`, foto, botones y multi-grupo) y devuelve `{created[], failed[]}` con failure-per-publication aislado; el frontend agrega un modal "Programar en lote" en `PublicationsPage` con slots editables y resumen antes de enviar.

---

## Hechos confirmados del codebase (base obligatoria)

**Estado actual verificable** (`main` post-slice3 archivado):

- **Backend** monolito modular Go, REST API, PostgreSQL 16 + `pgx/v5`, migraciones `goose` (00001-00005), slice 3 archivado.
- **Publicaciones** (`backend/internal/publications/`):
  - `Service.Publish(ctx, actorID, groupID, text, photoURL, buttons) (*Publication, error)` — slice 1, single-group.
  - `Service.PublishMany(ctx, actorID, payload PublishPayload) ([]Publication, error)` — slice 2, **multi-grupo SECUENCIAL**, validacion fail-fast, errores per-grupo NO abortan.
  - `Service.Schedule(ctx, actorID, payload, scheduledAt, nowFn) ([]Publication, error)` — slice 3, **multi-grupo SECUENCIAL con `scheduled_at` único**, persistiendo `status='scheduled'`.
  - `validatePayload` (service.go:446) cubre texto/foto/botones/grupos (1..10); errores per-grupo (404/403/Telegram) NO entran aca.
  - `PublishMany` (service.go:184) itera `payload.GroupIDs` llamando a `publishOne` que retorna UNA fila con su `Status` final (`sent`/`failed`); cada fila crea su log independiente (`ActionPublishMessage`).
- **Handler actual** (`backend/internal/api/publications_handlers.go`):
  - `handleCreatePublication` (línea 95) despacha: si `scheduled_at` presente → `Schedule`, sino → `PublishMany`. Una sola publicación con N grupos.
  - `respondPublicationError` mapea errores de dominio a HTTP; `validationErrorSet` (línea 337) lista los 400.
  - `publicationStore` interface (línea 28) consume `Publish`, `PublishMany`, `Schedule`, `GetByID`, `List`, `ListByTelegramID`, `CancelScheduled`.
- **Adapter Telegram** (`backend/internal/telegram/`): `SendMessage` y `SendPhoto` con `keyboard *InlineKeyboardMarkup` opcional. Rate limiter token bucket (~25 req/s) dentro del adapter; respeta §18.1 (no reintentar a ciegas, leer `retry_after`).
- **Frontend**:
  - `frontend/src/features/publications/{types,api,hooks,error}.ts` + `ButtonsEditor.tsx` + `validateScheduledAtClient.ts`.
  - `frontend/src/pages/PublicationsPage.tsx` (~506 LOC) migrado a Mantine v7 (commit `f4d1751`, observación #204). Flow actual: UNA publicación, dual-mode "Publicar ahora" / "Programar", multi-grupo via checkboxes, `ButtonsEditor`, datetime-local para `scheduled_at`.
  - `frontend/src/test/helpers.tsx` provee `mockFetchRoutes(routes)`, `renderWithProviders(ui, initialEntries)`, `matchQuery(url, expected)`.
- **Bugfix #172 (VIGENTE)**: `permissionOk` usa `g.BotStatus == groups.StatusAdministrator`; NUNCA claves `can_*`.
- **Worker in-process** (slice 3): ya existe `publications.Scheduler` que reclama filas `scheduled` cada 30s con `FOR UPDATE SKIP LOCKED`. El batch **reutiliza 100%** ese worker (no hay work nuevo en backend para scheduled items).
- **AGENTS §22 lista "Publicación en múltiples grupos" pero NO "Publicación en lote"**: este change es aditivo, no reemplaza nada.

## Decisiones (con tradeoffs)

22 decisiones (D1-D22) incluyendo:

- **D1**: Sin migración nueva (reutiliza `publications` existente)
- **D2**: Endpoint NUEVO `POST /api/publications/batch` (separado de `POST /api/publications`)
- **D3**: Handler en archivo NUEVO `backend/internal/api/batch_handlers.go`
- **D4**: `Service.PublishManyBatch` / `ScheduleBatch` NO se agregan. Handler hace loop llamando a `PublishMany` / `Schedule` existentes
- **D5**: Cap 10 publicaciones por batch
- **D6**: Validación per-publication independiente (failure isolation)
- **D7**: Status 200 OK siempre que envelope válida; 400 SOLO para envelope errors
- **D8**: `now time.Time` se computa UNA vez al inicio del batch
- **D9**: Cada `failed[]` lleva `{index, code, message}`
- **D10**: Frontend modal (NO nueva ruta)
- **D11**: Modal = lista editable de N slots (default 2, max 10)
- **D12**: Nuevo módulo `frontend/src/features/publications-batch/`
- **D13**: Helpers reusados sin duplicar
- **D14**: "Reintentar fallidas" re-envía solo fallidos
- **D15**: Sin auto-retry de fallos
- **D16**: Logs con `metadata.batch_index`
- **D17**: Worker scheduler sin cambios
- **D18**: `publicationStore` interface NO crece
- **D19**: Tests backend 8+ casos en `batch_handlers_test.go`
- **D20**: Tests frontend 6+ casos en `BatchWizard.test.tsx`
- **D21**: README sub-sección
- **D22**: Branch `feat/publications-batch`, delivery single-pr `size:exception`

## Hechos confirmados de la Bot API (sin cambios necesarios)

- Publicar a N grupos con UN texto: `sendMessage(chat_id, text)` por grupo
- Publicar a N grupos con foto + caption: `sendPhoto(chat_id, photo, caption)` por grupo
- Programar publicaciones: Telegram Bot API NO soporta `schedule_date` para bots en grupos

El batch NO necesita ningún método nuevo del adapter.

## Affected Areas

### Backend (~150 LOC)
- `backend/internal/api/batch_handlers.go` NEW
- `backend/internal/api/batch_handlers_test.go` NEW
- `backend/internal/api/server.go` MODIFIED
- `backend/internal/logs/model.go` MODIFIED (comentario)

### Frontend (~500 LOC)
- `frontend/src/features/publications-batch/types.ts` NEW
- `frontend/src/features/publications-batch/api.ts` NEW
- `frontend/src/features/publications-batch/hooks.ts` NEW
- `frontend/src/features/publications-batch/BatchWizard.tsx` NEW
- `frontend/src/features/publications-batch/BatchWizard.test.tsx` NEW
- `frontend/src/pages/PublicationsPage.tsx` MODIFIED

### Docs
- `README.md` MODIFIED

**TOTAL**: ~847 LOC vs budget 400 → ratio 2.1× → `size:exception` (precedente 11/11)

## Non-regression

UNTOUCHED: `publications/{model,repository,service,worker}.go`, `publications_handlers.go` (single), `cmd/server/main.go`, `telegram/*`, `migrations/*`, `features/publications/*` (reusados), `Layout.tsx`, `App.tsx`.

## Ready for Proposal

Sí. Todas las decisiones cerradas, código verificado en disco, contrato del endpoint especificado, riesgos mapeados, non-regression lista, precedent `size:exception` confirmado.
