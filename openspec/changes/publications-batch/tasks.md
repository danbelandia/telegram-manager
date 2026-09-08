# Tasks: Publications Batch — POST `/api/publications/batch` + BatchWizard Modal

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~853 (per design) |
| 400-line budget risk | High (ratio 2.1×) |
| Chained PRs recommended | No |
| Delivery strategy | exception-ok (single-pr `size:exception`, precedent 11/11) |
| Chain strategy | size-exception |
| Branch | `feat/publications-batch` base `main` |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: size-exception
400-line budget risk: High

Predecessors: slices 1+2+3 archivados en `main`. Sin migration, sin service method nuevo.

---

## Phase 1 — Backend handler + tests (REQ-16/17/18/25/26)

- [x] 1.1 Create `backend/internal/api/batch_handlers.go` with `batchRequest`, `batchFailureItem`, `handleCreatePublicationBatch`, `statusCodeFromError`. Loop con `now := time.Now()` UNA vez al inicio; dispatch per-item a `Service.PublishMany` (inmediato) o `Service.Schedule(..., nowFn)` (programado). Cap 10, batch vacío → 400. Failure isolation per-item → `failed[]`. Zero `can_*` checks (bugfix #172).
- [x] 1.2 Verify: `go build ./...` desde `backend/` MUST be green.
- [x] 1.3 Create `backend/internal/api/batch_handlers_test.go` con 8+ tests (cap>10, vacío, JSON bad, all-OK, all-fail, mix, mix sched+now, auth ausente, boundary 10, group_ids múltiples). Extiende `fakePublicationStore` con counters `publishManyCalls`/`scheduleCalls` separados.
- [x] 1.4 Verify: `go test ./internal/api/ -count=1` MUST be green; 0 calls reales a Bot API.

## Phase 2 — Backend route + log comment (REQ-19/26)

- [x] 2.1 Modify `backend/internal/api/server.go`: dentro de `WithPublications`, añadir `mux.HandleFunc("POST /api/publications/batch", ...) → handleCreatePublicationBatch` con `requireAuth` + `actorIDFromClaims`. ~+5 LOC.
- [x] 2.2 Modify `backend/internal/logs/model.go`: añadir doc comment sobre convención `metadata.batch_index` (int). ~+2 LOC.
- [x] 2.3 Verify: `go test ./... -count=1` MUST be green; no regresiones.

## Phase 3 — Frontend feature module (REQ-21/24)

- [x] 3.1 Create `frontend/src/features/publications-batch/types.ts` con `BatchItemInput`, `BatchRequest`, `BatchFailedItem`, `BatchResponse`, `BatchCreatedItem`. ~+30 LOC.
- [x] 3.2 Create `frontend/src/features/publications-batch/api.ts` con `createPublicationBatch(input)` via `apiClient.post`. ~+25 LOC.
- [x] 3.3 Create `frontend/src/features/publications-batch/hooks.ts` con `useCreatePublicationBatch` (react-query `useMutation` que invalida `['publications']` onSuccess y llama `notifyError` onError). ~+45 LOC.
- [x] 3.4 Verify: `npm test -- --run` MUST be green (no test breakage).

## Phase 4 — Frontend BatchWizard component + tests (REQ-21/22/23/25)

- [x] 4.1 Create `frontend/src/features/publications-batch/BatchWizard.tsx`. Modal Mantine v7 (`<Modal>`). State machine: Editing → Submitting → Result. Default 2 slots editables (Textarea + foto + ButtonsEditor reusado + multi-grupo + datetime-local opcional). Botones `+ Agregar slot` (max 10) y `Remover` por slot. Summary `<Alert>` con cada slot (índice, grupos target, texto truncado, fecha) + errores rojos si inválido. Submit deshabilitado si slot inválido. Result: 2 `<Alert>` separados (verde `created[]`, rojo `failed[]`); botón "Reintentar fallidas" pre-filtra slots a índices en `failed[]`; botón "Cerrar". ~+280 LOC.
- [x] 4.2 Create `frontend/src/features/publications-batch/BatchWizard.test.tsx` con 6+ smoke tests (abre modal, add/remove slot, submit bloqueado si slot inválido, all-success banner, all-fail banner, retry failed subset). Use `renderWithProviders` + `mockFetchRoutes`. ~+180 LOC.
- [x] 4.3 Verify: `npm test -- --run src/features/publications-batch/` MUST be green.

## Phase 5 — PublicationsPage integration (REQ-21/REQ-20 non-regression)

- [x] 5.1 Modify `frontend/src/pages/PublicationsPage.tsx`: añadir botón "Programar en lote" en toolbar + estado `batchOpen` + mount `<BatchWizard opened={batchOpen} onClose={...} />`. Funcionalmente UNTOUCHED. ~+25 LOC.
- [x] 5.2 Verify: `npm test -- --run src/pages/PublicationsPage.test.tsx` MUST be green (no test breakage).

## Phase 6 — Full suite + non-regression (REQ-20/26)

- [x] 6.1 Backend: `go test ./... -count=1` + `go vet ./...` + `gofmt -l .` MUST be clean (0 files).
- [x] 6.2 Frontend: `npm test -- --run` + `npm run build` MUST be clean.
- [x] 6.3 Non-regression: `git diff main -- <untouched paths>` MUST be empty. Untouched list: `backend/internal/publications/{model,repository,service,worker}.go`, `backend/internal/api/publications_handlers.go` (single), `backend/internal/telegram/*`, `backend/cmd/server/main.go`, `backend/migrations/*`, `backend/internal/moderation/`, `backend/internal/automation/`, `frontend/src/features/publications/{types,api,hooks,error,validateScheduledAtClient,ButtonsEditor}.{ts,tsx}`, `frontend/src/components/Layout.tsx`, `frontend/src/App.tsx`. Only the 12 files in design scope may change.
- [x] 6.4 Verify: `grep can_ backend/internal/api/batch_handlers.go` MUST be 0 matches (REQ-26 invariant).

## Phase 7 — README + commits

- [x] 7.1 Update `README.md`: add sub-sección "Publicación en lote" (~15 lines): cap 10, ejemplo curl POST `/api/publications/batch`, semántica `{created[], failed[]}`, nota sobre "Reintentar fallidas" y worker 30s para `scheduled`.
- [x] 7.2 Conventional commits per repo style: `feat(api): batch handler + tests`; `feat(web): batch wizard + module`; `docs: README batch section`. Do NOT push.
- [x] 7.3 Final verify: re-run `go test ./... -count=1 && npm test -- --run` ambos verdes.

---

## Open Architectural Reminders (from design)

- AD1: handler loop, NO `Service.PublishManyBatch`. Reusa `PublishMany`/`Schedule`.
- AD2: `now := time.Now()` UNA vez; pasado a TODAS `Schedule`.
- AD3: failure isolation per-item → `failed[]` con `{index, code, message}`.
- AD4: modal (no ruta), `BatchWizard` slots, "Reintentar fallidas" pre-filtra.
- AD5: `metadata.batch_index` (int) en JSONB existente; cero migración.
- AD6: tests extienden `fakePublicationStore` con counters separados; cero método nuevo.
- AD7: módulo NUEVO `features/publications-batch/`. Reusa `error.ts` + `validateScheduledAtClient.ts` + `ButtonsEditor`.

## Dependencies & Order

Phase 1 → Phase 2 → Phase 3 → Phase 4 → Phase 5 → Phase 6 → Phase 7 (strictly sequential).
