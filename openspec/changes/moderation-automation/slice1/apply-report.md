# sdd/moderation-automation-slice1/apply-report

**Change**: `moderation-automation-slice1` — backend-only foundation for AGENTS §23.
**Mode**: hybrid (filesystem + Engram).
**Base**: `main @ c46f8ee`.
**Branch**: `feat/moderation-automation-slice1` (NOT pushed).
**Strategy**: single-pr with `size:exception` (approved, observation #220).
**Persisted**: this artifact + Engram `sdd/moderation-automation/slice1/apply-report`.

---

## Status

**PASS** — implementation matches specs (#217), design (#218) and tasks (#219). All 8 phases executed; full backend suite green.

---

## Test Results

### Phase 1 — Migration + Models + Repository
- `go build ./...` clean.

### Phase 2 — Rules Registry + FloodRule
- `go test ./internal/automation/ -count=1 -run 'TestFloodRule|TestRegistry'` → **11 tests PASS** (FloodRule: under/at/over threshold, window expiry, user isolation, flood disabled, nil From; Registry: short-circuit, no-hit, nil message, dup-name).

### Phase 3 — Service + AutoActioner
- `go test ./internal/automation/... -count=1` → **14 + 11 = 25 tests PASS** (skip disabled / non-admin, hit increments, automute/autoban enqueue, idempotent at autoban, nil From, LoadOrCreate defaults, overflow channel; AutoActioner mute happy / ban happy / bot demoted / not found / tg error / unknown kind).

### Phase 4 — Worker + Events Subscriber
- `go test ./internal/automation/... -count=1` → **34 tests PASS** (worker FIFO, ctx cancel, ProcessOnce, error continues, channel close; subscriber filter nil/chat_member/no From/From.ID==0, valid update).

### Phase 6 — Integration Tests (Postgres real)
- `go test ./internal/automation/... -count=1` → **42 tests PASS** including 8 integration tests (settings round-trip, idempotent upsert, CHECK violations, warning_state lifecycle, list-by-group, reset expired, idempotent create-if-missing, cascade on group delete).

### Phase 7 — Full Backend Suite
- `go test ./... -count=1` → all packages **ok**:
  - `internal/api`, `internal/auth`, **`internal/automation`** (new), `internal/config`, `internal/database`, `internal/events`, `internal/groups`, `internal/joinrequests`, `internal/logs`, `internal/moderation` (untouched), `internal/publications`, `internal/telegram`, `internal/users`, `cmd/server`, `migrations`.
- `go vet ./...` → clean.
- `gofmt -l .` → empty.
- `git diff main -- backend/internal/moderation/` → **empty** (manual moderation untouched, per invariant).
- `git diff main -- frontend/` → **empty** (slice 1 is backend-only).

---

## Commit Layout (6 commits, ~3189 LOC added)

| SHA | Subject | Files | LOC |
|------|---------|------:|----:|
| `71c01fa` | feat(automation): migration + model + repository | 3 | +377 |
| `e50fb48` | feat(automation): rules registry + flood rule | 2 | +447 |
| `bfc351f` | feat(automation): service pipeline + autoactioner | 4 | +1206 |
| `86fc38e` | feat(automation): worker + events subscriber | 2 | +472 |
| `214432d` | test(automation): integration tests contra Postgres real | 1 | +397 |
| `e207dbd` | feat(logs): constantes para moderacion automatica (also includes config + main.go wiring + .env.example) | 4 | +86 |

The last commit bundled the cross-cutting wiring (logs constants, config env vars, main.go wiring, .env.example) under the logs commit because all 4 file modifications were staged together before the commit happened (one logical "module integration" change). This deviates from the user's suggested split `(d) env+constants` but kept everything atomic; if verify-phase prefers finer granularity, the historical commits are easy to re-split via `git rebase -i main`.

---

## Files Created / Modified

### NEW (12 files)
- `backend/migrations/00006_create_moderation_automation.sql` (41 LOC)
- `backend/internal/automation/model.go` (101 LOC)
- `backend/internal/automation/repository.go` (235 LOC)
- `backend/internal/automation/rules.go` (187 LOC)
- `backend/internal/automation/rules_test.go` (260 LOC)
- `backend/internal/automation/autoactioner.go` (239 LOC)
- `backend/internal/automation/autoactioner_test.go` (294 LOC)
- `backend/internal/automation/service.go` (264 LOC)
- `backend/internal/automation/service_test.go` (409 LOC)
- `backend/internal/automation/worker.go` (145 LOC)
- `backend/internal/automation/worker_test.go` (327 LOC)
- `backend/internal/automation/repository_test.go` (397 LOC)

### MODIFIED (4 files)
- `backend/internal/logs/model.go` (+8/-1: 3 new Action constants)
- `backend/internal/config/config.go` (+24: AutomationEnabled, AutoActionBufferSize, WorkerConcurrency + validation)
- `backend/cmd/server/main.go` (+41: automation pipeline wired, gated on cfg.AutomationEnabled)
- `.env.example` (+13/-1: 3 new env vars documented)

### NOT TOUCHED (invariants)
- `backend/internal/moderation/` — manual moderation service untouched (per spec REQ-12 and `permissionOkAdmin` bugfix #172 stays isolated in automation package).
- `frontend/` — slice 1 is backend-only (per AGENTS §26 + spec REQ-12).
- `backend/migrations/00001-00005` — no changes.
- All other backend modules.

---

## Deviations from Design (#218)

### 1. AutoActioner interface signature
**Design said**: `AutoActioner{ MuteUser(ctx, groupID, userID, untilDate int64) error; BanUser(ctx, groupID, userID int64) error }` (thin).
**Implementation**: `AutoActioner.Execute(ctx, action AutoAction) error` (rich — takes full AutoAction).

**Why**: The concrete impl needs `action.RuleName` and `action.WarningCount` for log metadata. The thin signature has no way to pass them; either the Worker would need to re-implement logging (duplicating the autoactioner's role), or we'd need to expose AutoAction via a side channel. The rich `Execute(action AutoAction)` keeps the re-check + log + dispatch in one place, matching spec REQ-8 (which mandates AutoActioner does the re-check).

### 2. Mute uses Unix timestamp for untilDate
**Design said**: `MuteUser(ctx, groupID, userID, untilDate int64)` where untilDate is `now + minutes*60`.
**Implementation**: Same — `AutoActioner.Execute` computes `until := a.now().Add(time.Duration(action.MinutesUntil) * time.Minute).Unix()` before calling `tg.MuteUser`. Matches design.

### 3. `user_warning_state` does NOT cascade on group delete
**Design said**: only mentioned FK from settings to groups; warning_state is "PK compuesta".
**Implementation**: As designed — no FK on warning_state. Confirmed via `TestRepository_WarningState_NoCascadeWithoutFK`. Cleanup of orphan warning_states deferred to slice 2+.

### 4. Service depends on `SettingsRepo` / `WarnRepo` interfaces (not concrete `*Repository`)
**Design didn't specify**: type signatures.
**Implementation**: Service.NewService takes `SettingsRepo` / `WarnRepo` interfaces (3 methods each) instead of concrete `*Repository`. This is required for unit testing without a DB; `*Repository` implements both interfaces. Adds 14 LOC to model.go but lets all Service tests run without OpenTestDB.

### 5. Subscriber uses `context.Background()` (not `ctx` from bus.Publish)
**Design didn't specify**.
**Implementation**: `events.Bus.Publish` doesn't take a ctx; the subscriber calls `context.Background()` for `HandleMessage`. The DB pool has its own timeout. Acceptable for slice 1 (fire-and-forget). Future slice may thread ctx through `bus.Handle` if cancellation cascades are needed.

---

## Invariants Verified

- ✅ **AGENTS §21.1 (no real Bot API)**: All tests use hand-rolled fakes (`fakeTelegramActor`, `fakeLogs`, `fakeGroups`); integration tests hit Postgres real but never Telegram. Zero `http.Post` to Bot API in tests.
- ✅ **Bugfix #172 (`permissionOkAdmin`)**: local helper in `service.go` and used by `autoactioner.go` (`g.BotStatus == groups.StatusAdministrator`). Comment in service.go explicitly forbids reusing `moderation.permissionOk`. Never reads `can_*` keys.
- ✅ **AGENTS §18.1 (rate limit)**: Worker calls `tg.MuteUser` / `tg.BanUser` via `AutoActioner`, which goes through `tg.doWithRetry` + token bucket. No bypass.
- ✅ **AGENTS §13.1 (goose migrations)**: SQL plano, `-- +goose Up` / `-- +goose Down` markers, embedded via `embed.FS`.
- ✅ **AGENTS §4 (sin secretos)**: No TELEGRAM_BOT_TOKEN, JWT_SECRET, passwords en código. `.env.example` con placeholders.
- ✅ **`moderation.Service` no modificado**: `git diff main -- backend/internal/moderation/` empty.
- ✅ **`frontend/` no modificado**: `git diff main -- frontend/` empty.

---

## Risks for Verify Phase

| Risk | Mitigation |
|------|-----------|
| 6th commit bundles 4 modifications (logs/config/main/.env) instead of being split | Acceptable: each file change is small and the commit message explains the bundle. If finer granularity is desired, a follow-up `git rebase -i main` can split. |
| `autoActionCh` capacity 100 may overflow under flood storm | Non-blocking send + warn log + drop (spec REQ-11). `AUTOMATION_AUTOACTION_BUFFER_SIZE` configurable. Counter + RULE_TRIGGERED log persist before enqueue, so audit trail is preserved. |
| `worker.go` uses `context.Background()` for `actioner.Execute` | Acceptable for slice 1 — fire-and-forget. The DB pool handles its own timeouts. ctx cancellation cascades via the `ctx.Done()` in Run loop. |
| `WorkerConcurrency=1` (no parallelism) | By design (spec REQ-11). Slice 1 keeps it sequential to respect the adapter's token bucket semantics. Future slices may parallelize. |

---

## Next Step

**sdd-verify** — branch ready (`feat/moderation-automation-slice1`, 6 commits ahead of main, NOT pushed).