# Tasks: Moderation Automation — Slice 1 (Foundation)

Backend-only foundation for AGENTS §23.

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~1639 across 13 files |
| 400-line budget risk | High |
| Chained PRs recommended | No |
| Suggested split | Single PR (`size:exception`) |
| Delivery strategy | single-pr |
| Chain strategy | size-exception |

Decision needed before apply: Yes
Chained PRs recommended: No
Chain strategy: size-exception
400-line budget risk: High

## Phase 1 — Migration + Models + Repository (compile gate)

- [x] 1.1 `backend/migrations/00006_create_moderation_automation.sql`: `group_moderation_settings` FK→`groups(telegram_id)` + CHECKs + `autoban>automute`; `user_warning_state` PK `(group_id,user_id)` + idx.
- [x] 1.2 `backend/internal/automation/model.go` — `Settings`, `WarningState`, `RuleHit`, `AutoAction`.
- [x] 1.3 `backend/internal/automation/repository.go` — `GetSettings`, `UpsertSettings`, `GetWarningState`, `UpsertWarningState` (atomic +1), `ListWarningStates`, `ResetExpiredWarnings`.
- [x] 1.4 Verify `cd backend && go build ./...` clean.

## Phase 2 — Rules Registry + FloodRule

- [x] 2.1 `backend/internal/automation/rules.go` — `Rule` interface, `Registry{Evaluate}` short-circuit, `FloodRule` (per-group mutex + sliding-window prune).
- [x] 2.2 `backend/internal/automation/rules_test.go` — 5+ cases (under/at/over threshold, window expiry, user isolation).
- [x] 2.3 Verify `cd backend && go test ./internal/automation/ -count=1 -run TestFlood` green.

## Phase 3 — Service + AutoActioner

- [x] 3.1 `backend/internal/automation/autoactioner.go` — `AutoActioner` + concrete impl re-checking `permissionOkAdmin` (bugfix #172), calls `tg.MuteUser`/`BanUser`, logs `ActorID=nil`.
- [x] 3.2 `backend/internal/automation/service.go` — `Service{HandleMessage}` 8-step pipeline; local `permissionOkAdmin` (forbidden reuse of `moderation.permissionOk`).
- [x] 3.3 `backend/internal/automation/autoactioner_test.go` — 2+ cases (happy path + re-check denial).
- [x] 3.4 `backend/internal/automation/service_test.go` — 5+ cases (skip disabled/non-admin, hit++, automute/autoban enqueue, idempotent).
- [x] 3.5 Verify `cd backend && go test ./internal/automation/... -count=1` green.

## Phase 4 — Worker + Events Subscriber

- [x] 4.1 `backend/internal/automation/worker.go` — `Worker{ch, actioner, logger}`; `Run(ctx)` FIFO, nil on cancel.
- [x] 4.2 `backend/internal/automation/events_subscriber.go` (merged in worker.go) — `Subscriber{bus, service}`; `Register()` → `bus.Handle`; filter nil `Message`/`From`.
- [x] 4.3 `backend/internal/automation/worker_test.go` — 3+ cases (FIFO, ctx cancel, overflow).
- [x] 4.4 `backend/internal/automation/events_subscriber_test.go` (in worker_test.go) — 1+ case (nil `Message`/`From` ignored).
- [x] 4.5 Verify `cd backend && go test ./internal/automation/... -count=1` green.

## Phase 5 — Logs + Config + main.go Wiring

- [x] 5.1 `backend/internal/logs/model.go` — add `ActionRuleTriggered`, `ActionAutomuteUser`, `ActionAutobanUser`.
- [x] 5.2 `backend/internal/config/config.go` — add `AutomationEnabled` (true), `AutoActionBufferSize` (100, >0), `WorkerConcurrency` (1, >=1).
- [x] 5.3 `backend/cmd/server/main.go` — after `logsRepo` build repo, channel, registry, actioner, service, worker, subscriber; gate `Register()`+`go worker.Run(ctx)` on `cfg.AutomationEnabled`.
- [x] 5.4 `.env.example` — append 3 lines with comments.

## Phase 6 — Integration Tests (Postgres real)

- [x] 6.1 `backend/internal/automation/repository_test.go` — `OpenTestDB("automation")`+`database.Migrate`+`TRUNCATE ... CASCADE`; cases: defaults create, atomic increment, list.
- [x] 6.2 Verify `cd backend && go test ./internal/automation/... -count=1` green.

## Phase 7 — Full Backend Suite

- [x] 7.1 Verify `go test ./... -count=1`, `go vet ./...`, `gofmt -l .` clean (cd `backend/`).
- [x] 7.2 Verify `git diff main -- backend/internal/moderation/` empty.
- [x] 7.3 Verify `git diff main -- frontend/` empty.

## Phase 8 — Conventional Commits

- [x] 8.1 Split: (a) migration+models+repo, (b) FloodRule, (c) service+autoactioner, (d) worker+subscriber, (e) integration tests, (f) logs+config+main+.env. Inspect via `git log --oneline main..HEAD`.

## Final

- [x] 8.2 `openspec/changes/moderation-automation/slice1/apply-report.md` written.
- [x] 8.3 Engram `sdd/moderation-automation/slice1/apply-report` persisted.

**Total tasks**: 22 / 22 complete.
**Branch**: `feat/moderation-automation-slice1` (6 commits ahead of `main @ c46f8ee`, NOT pushed).
**Ready for**: `sdd-verify`.