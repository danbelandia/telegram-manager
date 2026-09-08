# Design: Publications Batch — POST `/api/publications/batch` + Wizard Modal

> **Change**: `publications-batch` (aditiva sobre Fase 2). Branch `feat/publications-batch` base `main`. Single-pr `size:exception` (precedent 11/11).
> **Spec**: REQ-16..REQ-26. **Mode**: hybrid.

## Technical Approach

Endpoint NUEVO `POST /api/publications/batch` (cap 10); acepta N publicaciones, devuelve `{created[], failed[]}`. Handler looppea sobre `Service.PublishMany`/`Schedule` ya existentes. Cero método nuevo; cero migración. Frontend: modal Mantine desde botón nuevo en `PublicationsPage`, slots (default 2/max 10), summary Alert, "Reintentar fallidas". Mapeo: REQ-16/17/18→handler; REQ-19→`metadata.batch_index`; REQ-21/22/23→`BatchWizard`; REQ-24→reuso helpers; REQ-26→bugfix #172.

## Architecture Decisions

| # | Decision |
|---|----------|
| AD1 | Loop en handler sin `Service.PublishManyBatch`. Reusa `PublishMany`/`Schedule`. Cero método nuevo. |
| AD2 | `now := time.Now()` UNA vez al inicio; pasado a todas `Schedule(..., nowFn)`. |
| AD3 | Failure isolation per-item → `failed[]` con `{index, code, message}`; 200 OK con envelope válida. |
| AD4 | Modal (no ruta); `BatchWizard` con slots; "Reintentar fallidas" pre-filtra a `failed[]`. |
| AD5 | `metadata.batch_index` (int) en JSONB existente; cero migración. |
| AD6 | Tests extienden `fakePublicationStore` con `publishManyCalls`/`scheduleCalls` separados. |
| AD7 | Module NUEVO `features/publications-batch/`. Reusa `error.ts`+`validateScheduledAtClient.ts`+`ButtonsEditor`. |

## Data Flow

```
React (BatchWizard)
  →[POST /api/publications/batch]→ api-client + auth
                ↓
    handleCreatePublicationBatch (requireAuth + actorID)
                ↓
    now := time.Now()
    for i, item := range req.Publications:
        if item.scheduled_at == "":
            rows := service.PublishMany(ctx, actor, payload)
        else:
            t := NormalizeScheduledAt(item.scheduled_at, nowFn)
            rows := service.Schedule(ctx, actor, payload, *t, nowFn)
        rows[j].status == "failed" → failed[]; else → created[]
                ↓
    respond 200 {created:[…], failed:[…]}
                ↓
    React Query invalida ['publications']
                ↓
    BatchWizard: banners verde/rojo + "Reintentar fallidas"
```

Per-item logs: `PublishMany` ya emite `PUBLISH_MESSAGE` con `metadata={publication_id}`. `batch_index` se agrega via override en `publishOne` (en `apply.md`). Items `scheduled` quedan en DB; worker emite log al reclamarlos.

## File Changes

| File | Action | LOC | Descripción |
|------|--------|-----|-------------|
| `backend/internal/api/batch_handlers.go` | NEW | ~90 | `BatchRequest`/`batchError`/`handleCreatePublicationBatch`; `statusCodeFromError`. |
| `backend/internal/api/batch_handlers_test.go` | NEW | ~150 | 8+ tests: cap>10, vacío, JSON bad, all-OK, all-fail, mix, mix sched+now, auth, boundary. |
| `backend/internal/api/publications_handlers_test.go` | MOD | +6 | Añade `publishManyCalls`/`scheduleCalls` al fake. |
| `backend/internal/api/server.go` | MOD | +5 | `POST /api/publications/batch` con `requireAuth`. |
| `backend/internal/logs/model.go` | MOD | +2 | Doc convención `batch_index`. |
| `frontend/src/features/publications-batch/types.ts` | NEW | ~30 | Tipos `BatchRequest/Response/Item`. |
| `frontend/src/features/publications-batch/api.ts` | NEW | ~25 | `createPublicationBatch(input)`. |
| `frontend/src/features/publications-batch/hooks.ts` | NEW | ~45 | `useCreatePublicationBatch` (invalida `['publications']`). |
| `frontend/src/features/publications-batch/BatchWizard.tsx` | NEW | ~280 | Modal Mantine v7: slots, summary, banners, retry. |
| `frontend/src/features/publications-batch/BatchWizard.test.tsx` | NEW | ~180 | 6+ tests con `mockFetchRoutes`. |
| `frontend/src/pages/PublicationsPage.tsx` | MOD | +25 | Botón "Programar en lote" + `<BatchWizard>`. |
| `README.md` | MOD | ~15 | Sub-sección "Publicación en lote". |
| **TOTAL** | | **~853** | Ratio 2.1× vs 400 → `size:exception`. |

## Interfaces / Contracts

```go
// batchRequest envuelve N publicaciones; cada item reusa createPublicationRequest.
type batchRequest struct {
    Publications []createPublicationRequest `json:"publications"`
}
// batchError: index = posición ORIGINAL en req.Publications.
type batchError struct {
    Index   int    `json:"index"`
    Code    string `json:"code"`    // §18
    Message string `json:"message"`
}
// 200 OK con envelope válida: {created:[publicationResponse,...], failed:[batchError,...]}
```

```ts
interface BatchItemInput { text: string; photo_url?: string; buttons?: InlineButton[][]; group_ids: number[]; scheduled_at?: string }
interface BatchRequest { publications: BatchItemInput[] }
interface BatchFailedItem { index: number; code: string; message: string }
interface BatchResponse { created: { index: number; publication: Publication }[]; failed: BatchFailedItem[] }
```

## Testing Strategy

| Layer | Cases | Approach |
|-------|-------|----------|
| Backend unit | 8+ | `httptest` + fake con counters separados. |
| Frontend unit | 6+ | `renderWithProviders` + `mockFetchRoutes`. |
| Non-regression | Suite | `go test` + `npm test` verdes sin cambios. |

## Migration / Rollout

**No migration.** Tabla `publications` intacta. `Scheduler` sin cambios (30s, `ClaimScheduledDue(limit=25)`). Rollback: `git revert`.

## Open Questions (resolved)

Endpoint nuevo (REQ-16); loop en handler (AD1); cap 10; failure isolation; 200 OK envelope válida; modal desde `PublicationsPage`; module NUEVO `features/publications-batch/`; retry manual; worker reusado; sin migración; bugfix #172 vigente; `batch_index` JSONB (decisión `apply.md`).

## Risks & Mitigations

| Risk | Mitigation |
|------|------------|
| Worker 10+ filas mismo tick → rate-limit (M/M) | `ClaimScheduledDue(limit=25)` + token bucket ~25 req/s. |
| "Reintentar fallidas" crea duplicados (L/M) | Modal: "Las ya enviadas siguen vigentes — solo republica fallidos". |
| Race `Schedule(now+1s)` vs worker 30s (L/L) | Worker `scheduled_at <= now()`; UX ya lo dice. |
| `batch_index` no persiste en `scheduled` (M/L) | REQ-19 acepta JSONB o log adicional; decisión en `apply.md`. |
| Cap 10 limitante broadcasts (L/M) | 10×10=100 publicaciones; >10 con N requests. |
| Fake desincronizado si service cambia (L/L) | Change NO toca service; fake solo agrega contadores. |
| Frontend bundle delta (M/L) | `<Modal>`/`<Alert>` nativos Mantine v7; sin nuevas deps. |

## Non-Regression

UNTOUCHED: `publications/{model,repository,service,worker}.go`, `publications_handlers.go` (single), `cmd/server/main.go`, `telegram/*`, `migrations/*`, `features/publications/*` (reusados), `Layout.tsx`, `App.tsx`.
