# Proposal: Publications Batch — POST `/api/publications/batch` + Wizard Modal

## Intent

Hoy programar N publicaciones distintas (texto/foto/botones/grupos/fecha) requiere N envíos manuales al endpoint single `POST /api/publications`. El admin quiere armar varios anuncios del día en una sola pasada, ver qué pasó y reintentar solo los fallidos sin re-armar todo. Resolvemos con un nuevo endpoint batch (cap 10) y un wizard modal en `PublicationsPage`.

## Scope

### In Scope

- **Backend**: handler nuevo `handleCreatePublicationBatch` en archivo NUEVO `backend/internal/api/batch_handlers.go`. Reutiliza `Service.PublishMany` / `Service.Schedule` sin tocarlos. Sin migración nueva. Wire en `server.go` (`POST /api/publications/batch`).
- **Frontend**: módulo NUEVO `frontend/src/features/publications-batch/{types,api,hooks}.ts` + componente `BatchWizard.tsx` (modal Mantine, lista de slots default 2 / max 10, add/remove, Alert resumen, botón "Reintentar fallidas"). Botón NUEVO "Programar en lote" en `PublicationsPage.tsx` + mount del modal.
- **Tests**: `batch_handlers_test.go` (15 handler tests) + `BatchWizard.test.tsx` (9 smoke tests).
- **README**: sub-sección "Publicación en lote".

### Out of Scope

- Nuevos métodos en `publications.Service` o en la interface `publicationStore`.
- Migración nueva / tabla `publication_batches`.
- Cambios al `publications.Scheduler`.
- Recurrencia por grupo, cron, zonas horarias, drafts persistidos.
- Auto-retry de fallos.

## Capabilities

### Modified Capabilities

- `publications`: nuevo sub-capability "batch" con `{created[], failed[]}` y cap 10. Delta spec REQ-16..26.

### New Capabilities

_Ninguna._

## Approach

22 decisiones tomadas en exploration D1-D22. Highlights:

- Endpoint NUEVO separado (body shape distinto, response `{created[], failed[]}`)
- Archivo NUEVO `batch_handlers.go`
- Cero método nuevo en service (loop en handler llamando a `PublishMany` / `Schedule`)
- Cap 10 server-side
- Status code 200 OK (batch procesado; per-item failures a `failed[]`); 400 solo por envelope inválida
- `now := time.Now()` capturado UNA vez
- Failure isolation per-publication
- Code de fallo = dominio (VALIDATION_ERROR / PERMISSION_DENIED / etc.)
- Modal Mantine desde botón nuevo (NO nueva ruta)
- Lista de slots editable (default 2 / max 10)
- Feature module NUEVO separado
- "Reintentar fallidas" re-envía SOLO índices en `failed[]`
- Sin auto-retry
- Logs con `metadata.batch_index` (NUEVO campo opcional, sin migración)
- Worker scheduler sin cambios
- Tests con fakes (§21.1)
- README sub-sección
- Branch base `main`, single-pr `size:exception` (precedente 11/11)
- Non-regression estricta

## Affected Areas

| Archivo | Acción | LOC |
|---------|--------|-----|
| `backend/internal/api/batch_handlers.go` | **NEW** | +90 |
| `backend/internal/api/batch_handlers_test.go` | **NEW** | +150 |
| `backend/internal/api/server.go` | Modified | +5 |
| `backend/internal/logs/model.go` | Modified (comentario) | +2 |
| `frontend/src/features/publications-batch/types.ts` | **NEW** | +30 |
| `frontend/src/features/publications-batch/api.ts` | **NEW** | +25 |
| `frontend/src/features/publications-batch/hooks.ts` | **NEW** | +45 |
| `frontend/src/features/publications-batch/BatchWizard.tsx` | **NEW** | +280 |
| `frontend/src/features/publications-batch/BatchWizard.test.tsx` | **NEW** | +180 |
| `frontend/src/pages/PublicationsPage.tsx` | Modified | +25 |
| `README.md` | Modified | +15 |
| **TOTAL** | | **+847** |

## Risks

- 847 LOC > 400 budget → ratio 2.1× → `size:exception` solicitada (precedente 11/11).
- Worker recoge 10+ filas `scheduled` mismo tick → token bucket ~25 req/s ya ordena.
- "Reintentar fallidas" crea duplicados → documentado en modal.
- Cap 10 limitante para broadcasts masivos → 10×10 = 100 publicaciones potenciales; documentado.

## Rollback

Sin migración → `git revert` del merge. Tabla intacta (filas `scheduled` simplemente dejan de procesarse). Frontend vuelve a no ofrecer el botón. Cero residuos.

## Dependencies

- Slices 1+2+3 archivados (`publications.Service` con `PublishMany` + `Schedule` battle-tested).
- `telegram.Service.SendMessage` / `SendPhoto` + token bucket + `doWithRetry`.
- `publications.Scheduler` (slice 3) — worker in-process sin cambios.
- `fakePublicationStore` con métodos `PublishMany`/`Schedule` ya implementados.
- `frontend/test/helpers.tsx` (`renderWithProviders`, `mockFetchRoutes`).

## Delivery

**Single-pr con `size:exception`** — Branch `feat/publications-batch` base `main`. Precedente 11/11.

## Success Criteria

- POST batch 3 items (2 immediate + 1 scheduled) → 200 con `created[]`/`failed[]` correctos.
- `len > 10` → 400; `publications: []` → 400; JSON malformado → 400.
- Sin `Authorization` → 401.
- Batch mixto (válidos + inválidos) → 200; cada item aislado.
- `Service` y `publicationStore` interface: cero método nuevo. `fakePublicationStore`: cero método nuevo.
- Worker scheduler sin cambios; las `scheduled` del batch las recoge.
- Frontend: modal abre desde botón nuevo; slots add/remove; submit bloqueado si slot inválido; `failed[]` muestra "Reintentar fallidas".
- Tests: 15 handler + 9 frontend, todos en verde.
- `go test ./...` + `npm test -- --run` + `gofmt`/`tsc --noEmit` limpios.
- README con sub-sección "Publicación en lote".
- Bugfix #172 vigente (`permissionOk` con `bot_status==administrator`).

## Open Questions

_Ninguna._
