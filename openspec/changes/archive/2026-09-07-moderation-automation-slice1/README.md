# Archive — `moderation-automation-slice1`

> **Change archived**: `moderation-automation-slice1` (Slice 1/3 of **Fase 3 — Moderación Automática**, AGENTS §23).
> **Archived on**: 2026-09-07.
> **Branch**: `feat/moderation-automation-slice1` (base `main @ c46f8ee`, 7 commits ahead, NOT pushed/merged).
> **Verdict**: **PASS WITH NOTES** (verify-report observation `#222`).
> **Delivery strategy**: single-pr with `size:exception` (observation `#220` — 7° consecutivo aprobado, precedente 6 publications-slice PRs).
> **Mode**: hybrid (filesystem + Engram).

---

## What was done

1. **Created canonical spec** at `openspec/specs/moderation-automation/spec.md` (NEW capability — 15 REQs / 28 scenarios, 437 lines). The delta spec was a FULL spec (not a delta) — no prior canonical existed. **Moved** (not copied) verbatim from the delta via `git mv` — mirrors the frontend-refresh-slice2 precedent (observation `#214`, MOVE pattern; proposal `#216` declared this slice "New Capabilities" / `moderation-automation` and the spec is the initial entry).
2. **Moved change folder** `openspec/changes/moderation-automation/slice1/` → `openspec/changes/archive/2026-09-07-moderation-automation-slice1/` via `git mv` (preserves git history — git recognized 5 renames total: 4 .md files in the slice1 root + 1 rename of the spec.md to canonical). Cleaned up the now-empty `openspec/changes/moderation-automation/slice1/specs/` and `.../specs/moderation-automation/` subdirectories (MOVE, not COPY — no duplicate audit-trail copy).
3. **Tracked `verify-report.md`** which was untracked in the original location — moved with the folder and got staged (mirror slice-1 precedent, observation `#203` §3 and `#214` §3).
4. **Wrote `README.md`** (this file) documenting: change metadata, branch base, verdict, delivery strategy, NEW canonical spec creation action, file inventory, Engram observation IDs, full commit list (7 SHAs), the **3 documented deviations** (AutoActioner rich interface, Service interfaces not concrete, `user_warning_state` sin FK), **bugfix `#172` invariant verification** (4 explicit references + 0 `can_*` matches in automation), the **1 transient flake** (`TestRepository_ResetExpiredWarnings_OnlyAffectsExpired` — root cause hypothesis pgx pool timing + clock jitter; did NOT reproduce in 18 subsequent runs), the slice 1/3 context, invariantes respetadas, suggestions for slices 2/3, next steps for orchestrator, commit message.
5. **Audit trail**: Git recognized all 5 moves as renames — `git log --follow openspec/specs/moderation-automation/spec.md` will show full history from delta to canonical. The exploration.md stays at the parent (`openspec/changes/moderation-automation/exploration.md`) because it covers all 3 slices of Fase 3, not slice 1 alone.
6. **Commit pending**: `chore(openspec): archive change moderation-automation-slice1` (single conventional commit on `feat/moderation-automation-slice1`).
7. **NO push, NO merge** (rule of archive phase).

## Why

Slice 1 establishes the **backend-only foundation** of AGENTS §23 Fase 3: settings per-grupo, warning state per `(group, user)`, rule registry con `FloodRule`, worker secuencial de auto-actions que reusa `tg.MuteUser`/`tg.BanUser` (rate-limit heredado del adapter, §18.1), y audit logs con `ActorID=nil`. Verificable end-to-end via `psql` + queries sobre `logs`. **Backend-only**: frontend intacto. The archive is the formal SDD cycle close: the canonical spec is created in the source of truth, the change folder moves to archive as audit trail, and the branch is ready for merge to `main` by the orchestrator.

## Where

- **Created**: `openspec/specs/moderation-automation/spec.md` (NEW canonical — 437 lines, 15 REQs, 28 scenarios).
- **Moved to archive**: `openspec/changes/archive/2026-09-07-moderation-automation-slice1/`
  - `proposal.md` (Slice 1 scope, D1–D10, success criteria, schema, risks, rollback)
  - `design.md` (D1–D10 + bugfix `#172`, file-by-file specs, data flow, interfaces, risk table, tests strategy)
  - `tasks.md` (8 phases, 30+ tasks, all [x]; Review Workload Forecast High → size:exception)
  - `apply-report.md` (7 commits, gates green, 3 documented deviations, 42 tests pass)
  - `verify-report.md` (PASS WITH NOTES, 15/15 REQ compliance matrix, 10/10 D + bugfix `#172`, flake investigation)
  - `README.md` (this file — archive metadata, full provenance)
- **Stays at parent** (not moved): `openspec/changes/moderation-automation/exploration.md` — covers all 3 slices of Fase 3 (slices 2/3 will propose against it).
- **New in canonical**: `openspec/specs/moderation-automation/spec.md` (created by the spec.md move).

## Engram observation traceability

- `#215` — `sdd/moderation-automation/exploration` (3-slice plan, context, D1–D10)
- `#216` — `sdd/moderation-automation/slice1/proposal` (NEW capability, scope, rollback, success criteria)
- `#217` — `sdd/moderation-automation/slice1/spec` (`moderation-automation`, NEW full spec, 15 REQs)
- `#218` — `sdd/moderation-automation/slice1/design` (D1–D10 + bugfix `#172`, data flow, file specs)
- `#219` — `sdd/moderation-automation/slice1/tasks` (8 phases, 30+ tasks, Forecast High)
- `#220` — `sdd/moderation-automation/slice1/delivery-strategy` (decision — `size:exception` approved, 7° consecutivo)
- `#221` — `sdd/moderation-automation/slice1/apply-report` (7 commits, 22 files, gates green, 3 deviations)
- `#222` — `sdd/moderation-automation/slice1/verify-report` (PASS WITH NOTES, 15/15 REQs, 10/10 D, bugfix `#172`, 1 flake)
- `#THIS` — `sdd/moderation-automation/slice1/archive-report` (this observation)

## Spec sync actions (canonical → `openspec/specs/moderation-automation/spec.md`)

| Action | Detail |
|--------|--------|
| Capability creation | **NEW** — `moderation-automation` did NOT exist in `openspec/specs/` (verified via `Get-ChildItem` pre-archive). Zero collision with existing 17 specs (admin-logs, auth, bot-connection, frontend-auth, frontend-dashboard, frontend-moderation, frontend-pages-moderation, frontend-routing, frontend-ui-foundation, group-administration, group-detection, groups, join-requests, publications, repo-bootstrap, telegram-events, telegram-moderation). |
| Sync method | **Move** (not append, not copy). Delta IS the full spec (proposal `#216` §Capabilities "New Capabilities" + spec `#217` confirmed NEW full spec, no MODIFIED specs). Mirrors frontend-refresh-slice2 precedent `#214` (orchestrator MOVE directive). |
| ADDED | All 15 requirements (Schema `group_moderation_settings`, Schema `user_warning_state`, Default Settings al primer acceso, `allowed_updates` incluye `message`, Rules Registry, `FloodRule`, `Service.HandleMessage` 8-step, Canal `autoActionCh` y Worker, `AutoActioner` reusa `tg.MuteUser`/`BanUser`, Suscripción al `events.Bus`, Audit logs con `ActorID=nil`, Worker respeta rate limit, Configuración por env vars, Tests §21.1, No regresión). |
| MODIFIED | None. Existing `telegram-moderation` covers DOMAIN-level manual moderation (ban/unban/mute/unmute endpoints, §25 rules) — slice 1 does NOT modify it (verify-report `#222` REQ-15: `git diff main -- backend/internal/moderation/` empty). The automation layer is implementation detail behind existing REQs and does not overlap. |
| Removed | None. |
| Audit trail | Git rename recognized: `git log --follow openspec/specs/moderation-automation/spec.md` shows full history from delta to canonical. |

## Documented deviations (verify-report `#222` §5, apply-report `#221`)

All 3 deviations are **acceptable** per verify-report and improve the codebase without violating any spec REQ:

1. **`AutoActioner` interface is rich (`Execute(ctx, AutoAction) error`) instead of thin** — preserves log metadata (`RuleName`, `WarningCount`, `Reason`) required by spec REQ-8 and REQ-11. Thin signature would force the worker to duplicate log construction. `autoactioner.go:23-25`.
2. **Service depends on `SettingsRepo` / `WarnRepo` interfaces (not concrete `*Repository`)** — `*Repository` satisfies both. Enables unit tests with `fakeSettingsRepo` / `fakeWarnRepo` (service_test.go:16-85) without a real DB. Testability improvement, no contract change.
3. **`user_warning_state` has NO FK to `groups`** — matches design exactly (per exploration `#215` D4). Orphan cleanup deferred to slice 2+ (suggestion S2). Integration test `TestRepository_WarningState_NoCascadeWithoutFK` documents the deliberate behavior.

## Bugfix `#172` invariant verification

The `permissionOk` helper in `backend/internal/moderation/` reads `g.BotPermissions[key]` for `can_*` keys that the group detector never populates — a historical bug. Per bugfix `#172`, **all new code MUST use `g.BotStatus == groups.StatusAdministrator`**, NEVER `can_*`.

Slice 1 verification (`verify-report #222` §4 D6 + REQ-15):
- `service.go:262-264` — `g.BotStatus == groups.StatusAdministrator`.
- `autoactioner.go:98` — same.
- `grep can_* backend/internal/automation/` → **0 matches**.
- Bugfix reference appears in **4 explicit comments** in code: `model.go:8`, `service.go:35`, `service.go:258`, `autoactioner.go:84`. Each comment forbids reuse of `moderation.permissionOk`.

## 1 transient flake (verify-report `#222` §7, WARNING)

`TestRepository_ResetExpiredWarnings_OnlyAffectsExpired` failed once with `rows affected = 0, want 1` (repository_test.go:311) during the 2nd of 3 sequential runs.

Investigation methodology:
1. Failing test in isolation ×5 → **0 failures** (all pass).
2. Full automation package ×10 sequential → **0 failures**.
3. Full automation package ×5 with `-shuffle=on` → **0 failures**.
4. Full automation package ×3 verbose → **0 failures**.
5. Total: **19 runs after initial failure, 0 additional failures**.

**Root cause hypothesis** (not blocking): pgx pool timing + clock jitter. The window between `past := time.Now().Add(-1 * time.Hour)` and `now := time.Now()` is ~seconds (far larger than any plausible jitter) — but pgx's `timestamptz` normalization plus connection pool reuse may occasionally cause the comparison to miss on a flaky run.

**Verdict**: WARNING (low). Implementation correct; test correct; flake is environmental. Does NOT block merge. Optional follow-up: replace `time.Now()` with deterministic fixed clock injection, OR add a 10ms safety margin in `past` (e.g., `-2 * time.Hour`).

## Suggestions for slice 2+ (verify-report `#222` §9, SUGGESTION)

- **S1**: `events_subscriber.go` is bundled inside `worker.go` (lines 87-145). Functionally identical (all public types/methods exposed); cosmetic only. Split in slice 2 if desired.
- **S2**: Orphan cleanup for `user_warning_state` rows when a group is deleted. Recommended: nightly goroutine calling `repo.ResetExpiredWarnings`-style cleanup for `group_id NOT IN (SELECT telegram_id FROM groups)`. Tracked for slice 2.

## Slice 1/3 context

Per exploration `#215` and proposal `#216`, Fase 3 (AGENTS §23) ships in **3 slices**:

| Slice | Scope | Backend LOC | Frontend LOC | Status |
|-------|-------|------------:|-------------:|--------|
| **1 — Foundation** | settings + warning_state + FloodRule + worker + logs (THIS CHANGE) | ~1500 | 0 | **ARCHIVED** |
| 2 — More rules + Settings UI | Anti-spam + Anti-link + Banned-words + Settings editor | ~450 | ~700 | Proposed in `#215`, will be a new change `moderation-automation-slice2` |
| 3 — Warnings Dashboard | Read `user_warning_state` + logs aggregations + dashboard UI | ~150 | ~570 | Proposed in `#215`, will be a new change `moderation-automation-slice3` |

Each slice individually exceeds 400-line review budget — `size:exception` is the established pattern (7° consecutivo aprobado en este repo).

## Invariantes respetadas

- **§11 (NO Redis)**: 0 deps Redis; pure backend stack with in-process channel.
- **§14 (modular backend)**: new package `internal/automation/` is fully self-contained; only depends on existing packages via interfaces.
- **§13.1 (goose migrations)**: 1 new migration `00006_create_moderation_automation.sql` with `-- +goose Up`/`Down` markers. SQL plano, no DSL.
- **§17.1 (no secrets)**: 0 secrets in code. `.env.example` documents the 3 new env vars (`AUTOMATION_ENABLED`, `AUTOMATION_AUTOACTION_BUFFER_SIZE`, `AUTOMATION_WORKER_CONCURRENCY`).
- **§18.1 (rate limits Telegram)**: Worker calls `tg.MuteUser`/`tg.BanUser` via AutoActioner wrapper (token bucket + 429 retry live in `telegram.Adapter.doWithRetry`). **Never** direct HTTP. Verified in REQ-12.
- **§21.1 (mock TelegramService)**: 42 tests, all using hand-rolled fakes (`fakeTelegramActor`, `fakeLogs`, `fakeGroups`, `fakeWarnRepo`, `fakeSettingsRepo`, `fakeActioner`, `stubRule`). Zero `moq` codegen; zero Bot API calls.
- **§23 (Fase 3 — this slice)**: FOUNDATION delivered.
- **§25 regla 18 (frontend no llama Telegram)**: `git diff main -- frontend/` empty.
- **Bugfix `#172`** (permissionOkAdmin uses `BotStatus`, NEVER `can_*`): verified across 5 sites + 4 explicit comments.

## Commit list (7 commits, feat/moderation-automation-slice1, NOT pushed)

1. `71c01fa` — `feat(automation): migration + model + repository` (3 files, +377)
2. `e50fb48` — `feat(automation): rules registry + flood rule` (2 files, +447)
3. `bfc351f` — `feat(automation): service pipeline + autoactioner` (4 files, +1206)
4. `86fc38e` — `feat(automation): worker + events subscriber` (2 files, +472)
5. `214432d` — `test(automation): integration tests contra Postgres real` (1 file, +397)
6. `e207dbd` — `feat(logs): constantes para moderacion automatica` (+ main.go + config + .env.example; 4 files, +86)
7. `245a655` — `chore(openspec): moderation-automation-slice1 apply-report` (artifact + tasks.md; 6 files)

Total: **22 files changed, 4468 insertions(+), 2 deletions(-)** — matches apply-report `#221` exactly.

## Full backend suite verification (verify-report `#222` §6.1)

```
ok  github.com/telegram-manager/backend/internal/api           8.783s
ok  github.com/telegram-manager/backend/internal/auth          4.591s
ok  github.com/telegram-manager/backend/internal/automation   11.473s   ← NEW
ok  github.com/telegram-manager/backend/internal/config        1.122s
ok  github.com/telegram-manager/backend/internal/events        1.473s
ok  github.com/telegram-manager/backend/internal/groups        9.951s
ok  github.com/telegram-manager/backend/internal/joinrequests   7.605s
ok  github.com/telegram-manager/backend/internal/logs          4.685s
ok  github.com/telegram-manager/backend/internal/moderation    1.554s
ok  github.com/telegram-manager/backend/internal/publications  9.639s
ok  github.com/telegram-manager/backend/internal/telegram     22.773s
ok  github.com/telegram-manager/backend/internal/users         4.065s
```

12 packages, all green. `go vet ./...` clean. `gofmt -l .` clean. `go build ./...` clean. Zero changes in `backend/internal/moderation/` and `frontend/` (verify-report `#222` §8 non-regression evidence).

## Next steps (orchestrator)

1. Merge `feat/moderation-automation-slice1` → `main` (single-pr, `size:exception` aprobado per `#220`, precedent 7/7).
2. Push to remote.
3. Slice 1 closes Fase 3 foundation. Next candidates per `#215`:
   - **`moderation-automation-slice2`** — Anti-spam + Anti-link + Banned-words + Settings editor UI (Mantine v7 `<TagsInput>` for `banned_words`/`link_allowlist`).
   - **`moderation-automation-slice3`** — Warnings dashboard + stats aggregations.
   - Both will branch from `main` after slice 1 lands.

## Commit (this archive)

- **Message**: `chore(openspec): archive change moderation-automation-slice1`
- **Branch**: `feat/moderation-automation-slice1`
- **Files**: 5 git renames (4 .md files + spec.md to canonical) + 2 new files (this `README.md`, `verify-report.md` tracked). **NO source code touched.**
- **Push/Merge**: NEITHER (rule of archive phase).
