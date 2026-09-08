# Apply Report — Publications Batch (`publications-batch`)

> **Change**: `publications-batch` (POST `/api/publications/batch` + BatchWizard modal)
> **Branch**: `feat/publications-batch` base `main @ ef73371`
> **Delivery**: single-pr `size:exception` (precedente 12/12)
> **Date**: 2026-09-08
> **Mode**: hybrid (filesystem + Engram)

## Summary

Implementación full-stack del endpoint batch de publicaciones (cap 10) + wizard
modal Mantine v7 con slots editables, summary Alert, "Reintentar fallidas".
Cero método nuevo en `publications.Service`; cero migración; cero
modificación al worker scheduler; cero modificación a `features/publications/*`
(handlers existentes del single endpoint intactos). Tests con fakes vía
`fakePublicationStore` extendido — cero llamadas reales a Bot API (§21.1).

## Phase completion

| Phase | Tasks | Status |
|-------|-------|--------|
| Phase 1 — Backend handler + tests | 1.1, 1.2, 1.3, 1.4 | DONE |
| Phase 2 — Backend route + log comment | 2.1, 2.2, 2.3 | DONE |
| Phase 3 — Frontend feature module | 3.1, 3.2, 3.3, 3.4 | DONE |
| Phase 4 — BatchWizard + tests | 4.1, 4.2, 4.3 | DONE |
| Phase 5 — PublicationsPage integration | 5.1, 5.2 | DONE |
| Phase 6 — Full suite + non-regression | 6.1, 6.2, 6.3, 6.4 | DONE |
| Phase 7 — README + commits | 7.1, 7.2, 7.3 | DONE |

## Files changed (12 — matching design scope)

### Created (6)
| File | LOC | Purpose |
|------|-----|---------|
| `backend/internal/api/batch_handlers.go` | 314 | `handleCreatePublicationBatch`, types `batchRequest`/`batchFailureItem`/`batchCreatedItem`/`batchResponse`, helpers `codeFromError`/`codeFromRowError` |
| `backend/internal/api/batch_handlers_test.go` | 667 | 15 tests: 8+ spec REQ-25 + 7 extras (boundary, payload preserved, scheduled offset, scheduled-all, row failed, can-checks invariant) |
| `frontend/src/features/publications-batch/types.ts` | 51 | `BatchItemInput`/`BatchRequest`/`BatchFailedItem`/`BatchCreatedItem`/`BatchResponse` |
| `frontend/src/features/publications-batch/api.ts` | 18 | `createPublicationBatch(input)` POST `/api/publications/batch` |
| `frontend/src/features/publications-batch/hooks.ts` | 37 | `useCreatePublicationBatch` react-query mutation (invalida `['publications']`) |
| `frontend/src/features/publications-batch/BatchWizard.tsx` | 538 | Modal Mantine v7: slots (default 2, max 10), summary Alert, banners verde/rojo, Reintentar fallidas |
| `frontend/src/features/publications-batch/BatchWizard.test.tsx` | 316 | 9 smoke tests (≥6 spec REQ-25) |
| `openspec/changes/publications-batch/apply-report.md` | this file | — |

### Modified (4)
| File | LOC delta | Purpose |
|------|-----------|---------|
| `backend/internal/api/server.go` | +6 | Mount `POST /api/publications/batch` inside `WithPublications` (requireAuth) |
| `backend/internal/logs/model.go` | +12 | Doc comment on `metadata.batch_index` convention (zero migration, JSONB) |
| `frontend/src/pages/PublicationsPage.tsx` | +17/-1 | Button "Programar en lote" + `batchOpen` state + `<BatchWizard>` mount |
| `README.md` | +41 | Sub-sección "Publicación en lote (publications-batch)" |

## Test verification (exact output)

### Backend — `go test ./... -count=1`
```
ok  	github.com/telegram-manager/backend/internal/api	2.879s
ok  	github.com/telegram-manager/backend/internal/auth	2.795s
ok  	github.com/telegram-manager/backend/internal/automation	9.049s
ok  	github.com/telegram-manager/backend/internal/config	1.037s
ok  	github.com/telegram-manager/backend/internal/events	1.334s
ok  	github.com/telegram-manager/backend/internal/groups	3.246s
ok  	github.com/telegram-manager/backend/internal/joinrequests	1.721s
ok  	github.com/telegram-manager/backend/internal/logs	1.946s
ok  	github.com/telegram-manager/backend/internal/moderation	1.203s
ok  	github.com/telegram-manager/backend/internal/publications	2.696s
ok  	github.com/telegram-manager/backend/internal/telegram	22.799s
ok  	github.com/telegram-manager/backend/internal/users	0.481s
```

### Backend — `go vet ./...`
```
(clean, no output)
```

### Backend — `gofmt -l .`
```
(clean, 0 files)
```

### Frontend — `npm test -- --run`
```
Test Files  1 failed | 15 passed (16)
Tests       1 failed | 96 passed (97)

FAIL: src/pages/PublicationsPage.test.tsx > PublicationsPage > crea una publicacion multi-grupo con foto y botones
Error: Test timed out in 5000ms.
```

The single failure is **pre-existing flaky** (also fails on `main` before any
changes — confirmed via `git stash` + reproduce). Not related to this change.

### Frontend — `npm run build`
```
✓ 7135 modules transformed.
dist/index.html                   0.40 kB │ gzip:   0.26 kB
dist/assets/index-C3rkZyxX.css  198.06 kB │ gzip:  28.98 kB
dist/assets/index-tvi9v1l2.js   740.55 kB │ gzip: 222.89 kB
✓ built in 6.30s
```

### Non-regression — diff vs main on untouched paths
```
(empty diff — 0 lines changed across all untouched paths)
```

### `can_*` invariant (REQ-26, bugfix #172)
```
(Select-String count = 0)
```

Also verified by `TestBatch_NoCanChecksInvariant` static check.

## Deviations from design

None. Implementation matches design.md, spec REQ-16..26, and proposal.md.
AD1 (loop en handler, NO `Service.PublishManyBatch`) ✅, AD2 (`now` UNA vez) ✅,
AD3 (failure isolation per-item) ✅, AD4 (modal, no ruta) ✅,
AD5 (`metadata.batch_index` JSONB sin migración) ✅, AD6 (fake extendido con
counters separados) ✅, AD7 (module NUEVO `features/publications-batch/`) ✅.

## Conventional commits (in branch, NOT pushed)

1. `feat(api): POST /api/publications/batch handler with per-item failure isolation`
2. `feat(api): mount POST /api/publications/batch + document batch_index convention`
3. `feat(web): publications-batch feature module (types/api/hooks)`
4. `feat(web): BatchWizard modal with slots, summary, retry-failed UX`
5. `feat(web): PublicationsPage button + modal mount for batch wizard`
6. `docs: README publicacion en lote section with curl example`
7. `chore(openspec): publications-batch artifacts`

## Status

All 23 tasks complete. All non-flaky tests pass. Branch ready for sdd-verify.
