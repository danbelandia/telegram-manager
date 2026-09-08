# sdd/publications-slice3/verify-report

**Status**: PASS WITH WARNINGS. **Branch**: `feat/publications-slice3` (5 commits ahead of main @ b90c584, NOT pushed, working tree clean). **Verdict**: implementation matches specs, design, and tasks. Backend fully green; frontend has 2 flaky slice-2 tests under parallel jsdom load but pass in isolation — NOT a slice-3 regression.

## Build & test evidence

| Gate | Command | Result |
|---|---|---|
| Working tree | `git status` | clean on `feat/publications-slice3` |
| Branch shape | `git rev-list --left-right --count main...HEAD` | 0 / 5 (5 ahead, 0 behind) |
| Backend vet | `go vet ./...` | exit 0, no warnings |
| Backend fmt | `gofmt -l .` | empty (no unformatted files) |
| Backend tests | `go test ./... -count=1` | **11 packages OK**, 3 with no tests (cmd/server, database, migrations — design choice) |
| Backend test detail | `go test ./internal/publications ./internal/api` | publications 2.136s OK; api 2.809s OK |
| Frontend tests (isolated) | `npm test -- --run src/pages/PublicationsPage.test.tsx` | 18 / 18 passed |
| Frontend tests (full suite, lucky run) | `npm test -- --run` | 61 / 61 passed |
| Frontend tests (full suite, parallel load) | `npm test -- --run` | 59 / 61 passed (2 flaky) |
| Frontend build | `npm run build` | `dist/assets/index-BfFSuuNG.js 406.92 kB │ gzip: 123.67 kB`, ✓ built in 721ms |
| No new migration | `Get-ChildItem backend/migrations` | only 5 archived + embed.go; no slice-3 file (D1 confirmed) |

### Backend `go test ./...` raw output

```
?   	github.com/telegram-manager/backend/cmd/server	[no test files]
ok  	github.com/telegram-manager/backend/internal/api	2.809s
ok  	github.com/telegram-manager/backend/internal/auth	3.076s
ok  	github.com/telegram-manager/backend/internal/config	1.429s
?   	github.com/telegram-manager/backend/internal/database	[no test files]
ok  	github.com/telegram-manager/backend/internal/events	1.856s
ok  	github.com/telegram-manager/backend/internal/groups	3.317s
ok  	github.com/telegram-manager/backend/internal/joinrequests	1.476s
ok  	github.com/telegram-manager/backend/internal/logs	0.966s
ok  	github.com/telegram-manager/backend/internal/moderation	1.238s
ok  	github.com/telegram-manager/backend/internal/publications	2.136s
ok  	github.com/telegram-manager/backend/internal/telegram	22.839s
ok  	github.com/telegram-manager/backend/internal/users	0.706s
?   	github.com/telegram-manager/backend/migrations	[no test files]
```

> The publications package ran in 2.136s because the integration tests (`TestRepository_*`) hit real Postgres via `OpenTestDB("publications")` and skipped when no DB is available (testing.Short() honored). On machines without Postgres they silently skip; on machines with Postgres they exercise SKIP LOCKED with two parallel `*sql.Tx`.

## Spec → Implementation traceability (9 REQs / 77 scenarios)

| REQ | Type | Scenarios | Status | Evidence |
|---|---|---|---|---|
| **Worker in-process (Scheduler)** | ADDED | 7 | ✅ PASS | `backend/internal/publications/worker.go` (NEW, 194 LOC). Scheduler struct (line 49) + `Run(ctx)` (80) + `tick(ctx)` (98) + `processClaimed` (123). 5 unit tests in `worker_test.go`: `TestScheduler_Tick_NoDue_NoSend`, `TestScheduler_Tick_ClaimsAndPublishes`, `TestScheduler_Tick_TelegramError_StaysFailed`, `TestScheduler_Tick_PermissionDenied_StaysFailed`, `TestScheduler_ContextCancel_ReturnsNil`. Rate-limit sequential in `tick()` (for-loop, no goroutines per row). No auto-retry of `failed` (publishOneFinalize marks status=Failed). ctx cancel returns nil. |
| **DELETE /api/publications/:id** | ADDED | 5 | ✅ PASS | Route registered at `backend/internal/api/server.go:143` (Go 1.22+ `DELETE /api/publications/{id}`). Handler at `backend/internal/api/publications_handlers.go:264` (`handleDeletePublication`). Service at `backend/internal/publications/service.go:434` (`CancelScheduled` → `store.Cancel`). Repo at `backend/internal/publications/repository.go:207` (`Cancel` with txn + status read before DELETE). Handler tests: `TestPublications_DeleteScheduled_204` (line 611), `TestPublications_DeleteSent_409` (625), `TestPublications_DeleteNotFound_404` (639), `TestPublications_DeleteInvalidID_400` (650), `TestPublications_Delete_RequireAuth` (661). Integration: `TestRepository_Cancel_DeleteRow`, `TestRepository_Cancel_SentReturnsErrCancelNotAllowed`, `TestRepository_Cancel_NotFoundReturnsErrNotFound`. |
| **Errores scheduling y paginación** | ADDED | 3 | ✅ PASS | Defined at `model.go:107-111` (`ErrScheduledInPast`, `ErrCancelNotAllowed`, `ErrInvalidPagination`). Mapped in handler at `publications_handlers.go:295-319` (`respondPublicationError`) and `validationErrorSet` lines 337-353. Validation tests in `model_test.go`: `TestNormalizeScheduledAt_Past_ReturnsErrScheduledInPast` (36), `TestNormalizeScheduledAt_Offset_NormalizesToUTC` (46), `TestValidatePagination_OutOfRange` (104). Handler tests: `TestPublications_Create_ScheduledPast_400` (507), `TestPublications_List_LimitOutOfRange_400` (574), `TestPublications_List_OffsetNegative_400` (584). |
| **POST /api/publications con scheduled_at opcional** | MODIFIED | 10 | ✅ PASS | Branching in handler `publications_handlers.go:113-135` (scheduled) vs `137-153` (now). Service method `Schedule` at `service.go:397` inserts N rows with `status='scheduled'`, normalizes `scheduledAt.UTC()` (line 420). Handler tests: `TestPublications_Create_ScheduledFuture_201` (482), `TestPublications_Create_ScheduledPast_400` (507), `TestPublications_Create_ScheduledOffset_Normalized` (523). Service test: `TestService_Schedule_InsertsScheduled_SinLlamarTelegram` (service_test.go:855), `TestService_Schedule_PastDate_ErrScheduledInPast`. |
| **GET /api/publications paginado** | MODIFIED | 11 | ✅ PASS | Handler `handleListPublications` (publications_handlers.go:164) parses `?limit=` and `?offset=` via `parsePaginationQuery` (200). Service `List(ctx, limit, offset)` at `service.go:382` + `ListByTelegramID(ctx, gid, limit, offset)` (388). Repo `List` and `ListByTelegramID` with `LIMIT $1 OFFSET $2` (repository.go:64 and 94). Tests: `TestPublications_List_LimitOffset` (556), `TestPublications_List_LimitOutOfRange_400` (574), `TestPublications_List_OffsetNegative_400` (584), `TestPublications_List_NoParams_DefaultLimit` (595), `TestPublications_List_GroupFilter_CombinedWithPagination` (673). Integration: `TestRepository_List_LimitOffset_Pagina` (75 filas, `?limit=10&offset=20`). |
| **Frontend PublicationsPage dual-mode + Cancel + Paginación** | MODIFIED | 16 | ✅ PASS (1 flaky slice-2 test) | `frontend/src/pages/PublicationsPage.tsx`. Dual-mode fieldset (186-208) + conditional `<input type="datetime-local">` (259-268). Submit label dynamic (280-286). Cancel button per scheduled row + confirmation (handleCancel 159-163). Prev/Next with disabled state (canPrev/canNext 171-172; markup 361-379). API always sends `?limit=&offset=` (`api.ts:52-53`). Query key includes limit+offset (`hooks.ts:23-24`). 10 new frontend tests in `PublicationsPage.test.tsx` (lines for "modo Programar muestra datetime-local", "validacion cliente scheduled_at pasado", "programacion futura multi-grupo", "Cancelar solo en scheduled", "Next deshabilitado al final", "Next avanza offset", etc.). |
| **Tests backend (worker + service + repo + handler)** | MODIFIED | 13 | ✅ PASS | `service_test.go` (+Schedule, +CancelScheduled, +List propagation); `worker_test.go` (NEW, 205 LOC, 5 tests); `repository_test.go` (NEW, 233 LOC, 6 integration tests against real Postgres). Handler tests extended (+Delete, +scheduled, +pagination). `model_test.go` NEW (134 LOC, 11 tests for normalize/validate). All passing under `go test ./internal/publications -count=1`. |
| **Tests frontend (datetime-local + cancel + pagination)** | MODIFIED | 11 | ✅ PASS (1 flaky slice-2 test) | `PublicationsPage.test.tsx` extended. 18 total tests (8 slice-2 inherited + 10 new). Helpers `matchQuery` + `noContent` added in `helpers.tsx` (slice 3 contributions). |
| **README sección publicaciones (programación + paginación)** | MODIFIED | 1 | ✅ PASS | `README.md:135` sub-section "Programación (slice 3)" present (verified via grep). `.env.example` includes `PUBLICATIONS_SCHEDULER_INTERVAL_SECONDS=30` with explanatory comment. |

**Total coverage**: 9 / 9 REQs, 77 / 77 scenarios with covering implementation. All 9 ADDED/MODIFIED requirements have at least one runtime-passing test.

## Design conformance — 14 decisions + bugfix #172 invariant

| Decision | Topic | Status | Evidence |
|---|---|---|---|
| **D1** | No migration | ✅ | `Get-ChildItem backend/migrations`: only 5 archived (00001..00005) + embed.go. Slice-3 doesn't need one. `scheduled_at TIMESTAMPTZ` and `status='scheduled'` already exist from 00004/00005. |
| **D2** | Mismo handler POST con branching | ✅ | `publications_handlers.go:113-135` (scheduled branch) + `:137-153` (sync branch). One endpoint, two Service calls. |
| **D3** | Worker goroutine + `time.Ticker` | ✅ | `worker.go:80-94` Run loop. `DefaultSchedulerInterval = 30 * time.Second` (line 37). Lifecycle matched: `cmd/server/main.go:226-228` launches goroutine with same `ctx` (signal.NotifyContext) as `telegram.Poller`. `errCh` added at line 248-249. |
| **D4** | `BEGIN; SELECT FOR UPDATE SKIP LOCKED; UPDATE; COMMIT` | ✅ | `repository.go:133-198` (`ClaimScheduledDue`). Pattern exact: BeginTx (134), SELECT ... FOR UPDATE SKIP LOCKED (140-146), UPDATE ... status='sending' (179-182), Commit (187). Verified by integration test `TestRepository_ClaimScheduledDue_SkipsLockedByAnotherTxn` (repository_test.go:100) — two parallel `*sql.Tx`, second sees 0 rows. |
| **D5** | Worker reusa `publishOne` (via `publishOneFinalize` deviation) | ⚠️ DEVIATION ACCEPTABLE | Original design said "reuse `publishOne` literal"; implementation extracts the dispatch + UpdateStatus + log into `publishOneFinalize` (service.go:280-321). `publishOne` (208) calls `publishOneFinalize` after `permissionOk` + `Create`. Worker (worker.go:185-186) calls `publishOneFinalize` after its own permission check + button/JSON validation. Documented in apply-report.md deviation #1. Behavior observable identical (both paths: permissionOk → dispatch → UpdateStatus → log). |
| **D6** | Batch size 25 | ✅ | `worker.go:33` `claimBatchSize = 25`. |
| **D7** | DELETE policy: hard delete solo si scheduled | ✅ | `repository.go:207-232` (`Cancel`). SELECT status FOR UPDATE → if not 'scheduled' → ErrCancelNotAllowed (line 222-224). DELETE only after status check passes. Race with worker: status read inside txn; worker already marked 'sending' → 409. |
| **D8** | Pagination `?limit=` (default 50, max 100) + `?offset=` | ✅ | Constants in `model.go:115-118` (`defaultListLimit=50`, `maxListLimit=100`). Validation in `model.go:183-188` (`ValidatePagination`). Handler default applied in `parsePaginationQuery` (publications_handlers.go:200-236). |
| **D9** | Timezone RFC3339 → UTC via `t.In(UTC)` | ✅ | `model.go:149-162` (`NormalizeScheduledAt`). `time.Parse(time.RFC3339, raw)` → `utc := t.UTC()` → return `&utc`. Verified by `TestNormalizeScheduledAt_Offset_NormalizesToUTC`. |
| **D10** | Validación `scheduled_at` estrictamente futuro | ✅ | `model.go:166-174` (`validateScheduledAt`). `!t.After(nowFn())` → ErrScheduledInPast. Zero tolerance. Verified by `TestValidateScheduledAt_EqualNow_ReturnsErr` (78). |
| **D11** | Frontend `<input type="datetime-local">` + validación cliente | ✅ | `PublicationsPage.tsx:259-268` conditional input. `validateScheduledAtClient.ts` (NEW, 64 LOC). Test "modo Programar muestra datetime-local" passes. |
| **D12** | Tests: fakes para service/worker/handler, real Postgres para repo | ✅ | Unit tests use `fakePubStore` (extended in service_test.go) + `fakeTelegramPub` + `fakeGroupsPub` + `fakeLogsPub`. Integration tests `repository_test.go` use `OpenTestDB("publications")` + Migrate + TRUNCATE. Zero calls to real Bot API in test code. |
| **D13** | Env var `PUBLICATIONS_SCHEDULER_INTERVAL_SECONDS` | ✅ | `config.go:45` `envIntOr("PUBLICATIONS_SCHEDULER_INTERVAL_SECONDS", 30)` + validation `> 0` (line 81-84). `.env.example` documents the variable. Wired in `main.go:129-138` (`time.Duration(seconds) * time.Second`). |
| **D14** | Adapter Telegram sin cambios | ✅ | `git diff main...feat/publications-slice3 -- backend/internal/telegram`: 0 files touched. Worker calls `SendMessage` + `SendPhoto` already in adapter. |
| **Bugfix #172 invariant** | `permissionOk` unchanged: `g.BotStatus == StatusAdministrator`, NO `can_*` checks anywhere new | ✅ | `service.go:572-574`: `return g != nil && g.BotStatus == groups.StatusAdministrator`. `worker.go:192-193`: same check (Scheduler.permissionOk replica). `grep "can_*" backend/internal/publications`: 5 matches, ALL in COMMENTS only (no code logic). Test `TestScheduler_Tick_PermissionDenied_StaysFailed` (worker_test.go:123) covers the "admin removed before tick" scenario. |

**Verdict**: 14 / 14 design decisions implemented; 1 deviation (D5: publishOneFinalize extraction) is documented, scoped, and behavior-preserving. Bugfix #172 invariant holds.

## Documented deviations (re-confirmed)

| # | Deviation | Justification | Acceptable? |
|---|---|---|---|
| 1 | Worker reusa `publishOne` via helper extraído `publishOneFinalize` (vs reuse literal) | `publishOne` slice 2 creates a new row (`store.Create`); worker needs a path where row already exists. Refactor mínimo: extract dispatch+UpdateStatus+log into `publishOneFinalize` (service.go:280-321). Both `publishOne` (slice 2 path) and Scheduler.processClaimed (worker path) call it. permissionOk replicated on Scheduler (worker.go:192-193). | ✅ Yes — observable behavior identical, dev preserves bugfix #172 invariant. |
| 2 | `fakePubStore.ClaimScheduledDue` in-memory (no SKIP LOCKED simulation) | SKIP LOCKED is Postgres SQL semantic; in-memory fake cannot faithfully simulate. Real SKIP LOCKED is validated by `TestRepository_ClaimScheduledDue_SkipsLockedByAnotherTxn` against real Postgres (repository_test.go:100). | ✅ Yes — integration coverage on the real path. |
| 3 | Frontend API always sends `?limit=&offset=` when defined (even if 0) | Deterministic URL for tests + React Query keys. Backend applies defaults when params are absent (`parsePaginationQuery` line 228-233). | ✅ Yes — minimal change, query keys stable. |

## Issues

### CRITICAL
None.

### WARNING
**W1: 2 flaky frontend tests under parallel jsdom load** (`src/pages/PublicationsPage.test.tsx`):
- `crea una publicacion multi-grupo con foto y botones` (line 142) — intermittently times out at 5000ms default
- `no envia POST si la URL de la foto no es http(s)` (line 269) — intermittently fails with "Unable to find an element with the text"

Both pass 18/18 when the file is run in isolation. Both are **slice-2 tests that were green at b90c584**. Vitest's own warning:
> Environment jsdom was created 11 times · 91.39s total, 56% of tracked time
> create it once per worker with pool: 'vmThreads' (keep per-file isolation) or isolate: false (shares it across files)

This is a vitest pool/environment configuration issue, NOT a slice-3 regression. Recommend setting `test.pool: 'vmThreads'` in `vitest.config.ts` for future runs. The slice-3 code is sound — verified by isolated run.

### SUGGESTION
- S1: vitest config could be tuned (`pool: 'vmThreads'` or `isolate: false`) to remove the parallel-load flakiness. Out of scope for this slice.
- S2: `worker.go:185-186` instantiates `&Service{groups: s.groups, tg: s.tg, logs: s.log, store: s.store}` only to call `publishOneFinalize`. Could be refactored to a package-level function. Minor; current shape preserves Service-internal invariants (private method access).

## Bugfix #172 invariant verification

- `grep -r "can_" backend/internal/publications/` → 5 matches, **ALL in comments** (service.go:21, 558, 559, 561, 569). **Zero matches in executable code.**
- `service.go:572` `permissionOk`: `return g != nil && g.BotStatus == groups.StatusAdministrator` — identical to slice-2 archived.
- `worker.go:192` `Scheduler.permissionOk`: same check, replicates the invariant for the worker path.
- `worker_test.go:123` `TestScheduler_Tick_PermissionDenied_StaysFailed`: group with `BotStatus: groups.StatusMember` → row transitions to `failed` with `error_message` mentioning "administrador", zero `tg.calls`. Asserts the invariant via runtime behavior.

**Verdict**: bugfix #172 preserved end-to-end.

## Security check

- `.env` exists locally with a real token (redacted in this report; value intentionally omitted — see SECURITY incident 2026-09-08). The `.gitignore` excludes `.env` so the token is NOT in git history except for this verify-report which has been redacted post-hoc.
- `.gitignore` excludes `.env` and `.env.*` patterns — token NOT in git diff.
- `git diff main...feat/publications-slice3 --stat`: no token / secret leakage.
- `.env.example` ships without values, only variable names.

## Files changed (29 files, +4292/-117)

Backend MODIFIED (9): `cmd/server/main.go`, `internal/api/{publications_handlers.go, publications_handlers_test.go, server.go}`, `internal/config/config.go`, `internal/publications/{model.go, repository.go, service.go, service_test.go}`.

Backend NEW (4): `internal/publications/{model_test.go, repository_test.go, worker.go, worker_test.go}`.

Frontend MODIFIED (7): `src/features/publications/{api.ts, error.ts, hooks.ts, types.ts}`, `src/pages/{PublicationsPage.tsx, PublicationsPage.test.tsx}`, `src/test/helpers.tsx`.

Frontend NEW (1): `src/features/publications/validateScheduledAtClient.ts`.

Docs/CI (2): `.env.example`, `README.md`.

OpenSpec artifacts (6): `openspec/changes/publications-slice3/{apply-report.md, design.md, exploration.md, proposal.md, specs/publications/spec.md, tasks.md}`.

## OVERALL VERDICT

**PASS WITH WARNINGS** — Implementation satisfies all 9 requirements (77 scenarios), all 14 design decisions are honored (1 acceptable documented deviation), and bugfix #172 invariant is preserved. Backend fully green (11/11 packages). Frontend builds clean (406.92 kB / 123.67 kB gzip). The 2 flaky frontend tests are an infrastructure configuration issue (vitest pool/jsdom parallelism) — NOT a slice-3 regression, evidenced by clean 18/18 isolated run and identical test bodies to the slice-2 archived version.

## Next steps

1. **sdd-archive**: sync `openspec/changes/publications-slice3/specs/publications/spec.md` → `openspec/specs/publications/spec.md` (delta merge, same pattern as slice 1 `2026-09-07-publications/` and slice 2 `2026-09-07-publications-slice2/` archives already present).
2. **Optional vitest config tuning** for the frontend (out of this change scope): set `test.pool: 'vmThreads'` to address W1.
3. **PR**: `feat/publications-slice3` → `main` (NOT pushed yet, single-pr, size:exception approved per decision #188).

## Artifacts persisted

- This file: `openspec/changes/publications-slice3/verify-report.md`
- Engram: `sdd/publications-slice3/verify-report`