# Design: Moderation Automation — Slice 1 (Foundation: Settings + Flood + Auto-Action Worker)

> **Change**: `moderation-automation-slice1` — backend-only foundation for AGENTS §23.
> **Mode**: hybrid (filesystem + Engram). Persisted as `sdd/moderation-automation/slice1/design`.
> **Base**: `main @ 24e4c8c` (post-publications-slice3). Branch: `feat/moderation-automation-slice1`.
> **Authority**: exploration `#215`, proposal (this slice), spec REQ-1..REQ-12; bugfix `#172` (`permissionOkAdmin` uses `g.BotStatus == StatusAdministrator`, NEVER `can_*`).
> **Strategy**: single-pr with `size:exception` (precedent: 6 publications-slice PRs).

## Technical Approach

A new backend package `internal/automation/` consumes the existing `events.Bus` (already wired in `cmd/server/main.go`), filters `Update.Message != nil`, runs a registry of rules (`FloodRule` only in slice 1), and persists warning state per `(group, user)`. When `warning_count` crosses configured thresholds, an in-process channel enqueues a `AutoAction` that a sequential worker dispatches via a thin wrapper (`AutoActioner`) which re-checks `permissionOkAdmin` and calls `telegram.Service.MuteUser`/`BanUser` — never raw HTTP, so the adapter's token bucket + 429 retry stay in effect (§18.1). Audit logs use existing `logs.Entry` with `ActorID=nil` for system-generated actions. No frontend changes; verifiable end-to-end via `psql` + log queries.

## Architecture Decisions

| # | Decision | Choice | Alternatives | Rationale |
|---|----------|--------|--------------|-----------|
| **D1** | Trigger source | `events.Bus` consumer registered in `main.go` (`bus.Handle(subscriber.Handle)`); filter `Update.Message != nil` | Direct polling in automation goroutine; webhook handler hook | Bus already supports N consumers (spec `telegram-events` REQ); zero changes to transport. |
| **D2** | Rule registry | `Rule` interface + `Registry` (slice iteration, short-circuit on first hit) | Per-rule goroutines; chained middleware | Slice 1 has 1 rule; slice 2 adds 3 more — registry keeps Open/Closed. |
| **D3** | Settings storage | Structured table `group_moderation_settings` with CHECK constraints | JSONB column on `groups`; per-group config in env | Queryable columns + invariants (`autoban_warnings > automute_warnings`); no schema-less ops. |
| **D4** | Warning state | `user_warning_state` with PK `(telegram_id, user_id)`, counter + timestamps | Reuse existing `warnings` table; Redis counter | `warnings` (00003) stores incidents (reason text); we need a persistent counter for thresholds. Redis forbidden (§2). |
| **D5** | Auto-action transport | `chan AutoAction` + sequential worker; non-blocking send; configurable buffer | Direct call from `HandleMessage`; goroutine per hit | Reuses publications pattern; sequential preserves rate-limit semantics; non-blocking protects the bus. |
| **D6** | Permission check | Local helper `permissionOkAdmin(g) = g.BotStatus == StatusAdministrator`; **`AutoActioner` wraps `tg.MuteUser`/`BanUser` directly** | Reuse `moderation.permissionOk`; reuse `moderation.Service` | Bugfix `#172`: `moderation.permissionOk` reads `can_*` keys that the group detector never populates. Publications pattern with `BotStatus` is verified. |
| **D7** | Audit logs | New constants `ActionRuleTriggered`/`ActionAutomuteUser`/`ActionAutobanUser`; `ActorID=nil` | Reuse `ActionMuteUser`/`ActionBanUser` | Distinguish system-triggered from manual in queries; `ActorID=nil` is the existing convention for system events. |
| **D8** | Frontend | **No changes** in slice 1 | Stub UI | §26 forbids building ahead of MVP; backend is verifiable via DB + logs. |
| **D9** | Tests | Unit (rules, service, worker) with hand-rolled fakes; integration (repo) with `database.OpenTestDB("automation")` + `goose.Up` + `TRUNCATE ... CASCADE`; zero Bot API calls | `moq` codegen; httptest server | §21.1 strict; matches publications-slice3 testing pattern. |
| **D10** | `allowed_updates` invariant | Re-confirmed `message` is in `telegram.MVPAllowedUpdates` (`poller.go:13`) and in `cmd/server/main.go:183` `SetWebhook` call | (no change) | Spec REQ-4 explicit; verified in real code. |

## Data Flow

```
Telegram ──→ getUpdates (poller) / setWebhook (production)
             └─→ events.Bus.Publish(*Update)  (synchronous fan-out)
                  ├─ logging handler            (existing)
                  ├─ groups handler             (existing)
                  ├─ joinrequests handler       (existing)
                  └─ automation.Subscriber ──── filter: u.Message != nil && u.Message.From != nil
                       └─ Service.HandleMessage(ctx, msg)
                          1. early return if msg.From.ID == 0
                          2. LoadOrCreateSettings(groupID)        ─→ group_moderation_settings
                          3. if !settings.Enabled → return
                          4. permissionOkAdmin(groupsRepo.GetByTelegramID(...))
                             if false → return (silent)
                          5. LoadOrCreateWarningState(groupID, userID)
                          6. Registry.Evaluate(msg, settings, ws)
                             ├─ FloodRule (in-memory sliding window)
                             └─ (slice 2: anti-link, anti-spam, banned-words)
                          7. if hit → ws.WarningCount++, log RULE_TRIGGERED, UpsertWarningState
                          8. if ws.WarningCount >= autoban → autoActionCh ← AutoAction{Ban}
                             else if >= automute → autoActionCh ← AutoAction{Mute, Minutes}
                             (non-blocking send: select with default)
                                    │
                                    ▼
                          automation.Worker.Run(ctx)  ─→ for each AutoAction:
                                                            AutoActioner.dispatch(action)
                                                              ├─ re-read g, re-check permissionOkAdmin
                                                              ├─ tg.MuteUser / tg.BanUser  (token bucket + 429 retry)
                                                              └─ log AUTOMUTE_USER / AUTOBAN_USER  (ActorID=nil)
```

## File Changes

### NEW — Migration

**`backend/migrations/00006_create_moderation_automation.sql`** (~60 LOC)

```sql
-- +goose Up
-- Tablas de moderacion automatica (Fase 3, slice 1 — foundation).
-- group_moderation_settings: settings per-grupo con toggles y thresholds
-- (CHECK constraints). FK a groups.telegram_id (id natural) con CASCADE.
-- user_warning_state: counter per (group, user) para los thresholds
-- auto-mute/auto-ban. PK compuesta para upsert eficiente. Indices sobre
-- group_id (listado) y expires_at (cleanup periodico).
CREATE TABLE group_moderation_settings (
    group_id              BIGINT      PRIMARY KEY REFERENCES groups(telegram_id) ON DELETE CASCADE,
    enabled               BOOLEAN     NOT NULL DEFAULT false,
    anti_spam_enabled     BOOLEAN     NOT NULL DEFAULT false,
    anti_link_enabled     BOOLEAN     NOT NULL DEFAULT false,
    banned_words_enabled  BOOLEAN     NOT NULL DEFAULT false,
    flood_enabled         BOOLEAN     NOT NULL DEFAULT false,
    flood_messages        SMALLINT    NOT NULL DEFAULT 5   CHECK (flood_messages > 0),
    flood_seconds         SMALLINT    NOT NULL DEFAULT 10  CHECK (flood_seconds > 0),
    warning_limit         SMALLINT    NOT NULL DEFAULT 3   CHECK (warning_limit > 0),
    automute_warnings     SMALLINT    NOT NULL DEFAULT 3   CHECK (automute_warnings > 0),
    automute_minutes      SMALLINT    NOT NULL DEFAULT 10  CHECK (automute_minutes > 0),
    autoban_warnings      SMALLINT    NOT NULL DEFAULT 5   CHECK (autoban_warnings > automute_warnings),
    warning_expire_days   SMALLINT    NOT NULL DEFAULT 30  CHECK (warning_expire_days > 0),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE user_warning_state (
    group_id         BIGINT      NOT NULL,
    user_id          BIGINT      NOT NULL,
    warning_count    SMALLINT    NOT NULL DEFAULT 0 CHECK (warning_count >= 0),
    last_warning_at  TIMESTAMPTZ,
    last_action_at   TIMESTAMPTZ,
    expires_at       TIMESTAMPTZ,
    PRIMARY KEY (group_id, user_id)
);
CREATE INDEX idx_user_warning_state_group    ON user_warning_state (group_id);
CREATE INDEX idx_user_warning_state_expires  ON user_warning_state (expires_at);

-- +goose Down
DROP TABLE IF EXISTS user_warning_state;
DROP TABLE IF EXISTS group_moderation_settings;
```

### NEW — Automation package (`backend/internal/automation/`)

| File | LOC | Responsibility |
|------|----:|----------------|
| `model.go` | ~80 | `Settings`, `WarningState` (DB tags), `RuleHit`, `AutoAction{Kind, GroupID, UserID, UntilDate, Minutes, Reason, RuleName}`. Sentinel errors `ErrInvalidThreshold`, `ErrPermissionDeniedAction` (worker re-check failure). |
| `repository.go` | ~180 | `Repository{db *sql.DB}`. `GetSettings` (returns zero-value `Settings` if not found — caller decides persist; matches publications `UpsertByTelegramID` idempotent style), `UpsertSettings` (ON CONFLICT DO UPDATE), `GetWarningState`, `UpsertWarningState` (atomic `INSERT ON CONFLICT (group_id,user_id) DO UPDATE SET warning_count = user_warning_state.warning_count + 1, last_warning_at = now() ...`), `ListWarningStates(groupID)`, `ResetExpiredWarnings` (worker call). |
| `rules.go` | ~110 | `type Rule interface { Name() string; Evaluate(ctx, msg *telegram.Message, settings *Settings, ws *WarningState, now time.Time) *RuleHit }`. `Registry{Evaluate(msg, settings, ws) *RuleHit}` iterates registered rules in order, returns first non-nil hit. `FloodRule` keeps `map[groupID]map[userID][]time.Time` with `sync.Mutex` (per-group locks). `Evaluate` prunes entries older than `(now - flood_seconds)`, appends current timestamp, returns hit if `len(window) >= flood_messages`. **Pruning is the bounded-buffer mechanism** — empty windows reset. |
| `service.go` | ~220 | `Service{settingsRepo, warnRepo, registry, logs, groups, channel chan<- AutoAction, logger}`. `HandleMessage(ctx, msg)` implements the 8-step pipeline from the data flow above. `permissionOkAdmin(g *groups.Group) bool = g != nil && g.BotStatus == groups.StatusAdministrator` — local helper, comment explicitly references bugfix `#172` to forbid reuse of `moderation.permissionOk`. `LoadOrCreateSettings(ctx, groupID)` upserts defaults on miss. Silent skip on disabled / non-admin / nil `From`. Returns error only on internal DB failure. |
| `autoactioner.go` | ~110 | Consumer-side interface `AutoActioner{ MuteUser(ctx, groupID, userID, untilDate int64) error; BanUser(ctx, groupID, userID int64) error }`. Concrete `autoActioner{telegram.Service, logs.LogWriter, groups.GroupReader, logger}` re-reads group via `GetByTelegramID`, re-checks `permissionOkAdmin`, maps errors to `logs.Status*`, writes `ActionAutomuteUser` / `ActionAutobanUser` with `ActorID=nil` and metadata `{rule_name, warning_count, reason}`. |
| `worker.go` | ~100 | `Worker{ch <-chan AutoAction, actioner AutoActioner, logger}`. `Run(ctx)` reads `ch` in FIFO order, calls `actioner.MuteUser` or `BanUser`, logs. Returns nil on ctx cancel. **No bypass of `tg.MuteUser`/`BanUser`** — token bucket + 429 retry live in `telegram.Adapter.doWithRetry`. |
| `events_subscriber.go` | ~60 | `Subscriber{bus *events.Bus, service *Service}`. `Register()` calls `bus.Handle(s.Handle)`. `Handle(u *telegram.Update)` short-circuits on `u == nil \|\| u.Message == nil \|\| u.Message.From == nil`, otherwise calls `service.HandleMessage(ctx, u.Message)`. |

### MODIFIED

| File | Change |
|------|--------|
| `backend/internal/logs/model.go` | +3 constants: `ActionRuleTriggered = "RULE_TRIGGERED"`, `ActionAutomuteUser = "AUTOMUTE_USER"`, `ActionAutobanUser = "AUTOBAN_USER"`. |
| `backend/internal/config/config.go` | +3 fields + env loaders: `AutomationEnabled bool` (`envBoolOr("AUTOMATION_ENABLED", true)`), `AutoActionBufferSize int` (`envIntOr("AUTOMATION_AUTOACTION_BUFFER_SIZE", 100)`), `WorkerConcurrency int` (`envIntOr("AUTOMATION_WORKER_CONCURRENCY", 1)`). Validation: buffer > 0, concurrency >= 1. |
| `backend/cmd/server/main.go` | +~30 LOC after `logsRepo` is built: `automationRepo := automation.NewRepository(db); autoActionCh := make(chan automation.AutoAction, cfg.AutoActionBufferSize); registry := automation.NewRegistry(); registry.Register(automation.NewFloodRule()); actioner := automation.NewAutoActioner(bot, logsRepo, groupsRepo, slog.Default()); service := automation.NewService(automationRepo, automationRepo, registry, logsRepo, groupsRepo, autoActionCh, slog.Default()); worker := automation.NewWorker(autoActionCh, actioner, slog.Default()); subscriber := automation.NewSubscriber(bus, service)`. Wrap subscriber registration + worker goroutine in `if cfg.AutomationEnabled { subscriber.Register(); go func(){ _ = worker.Run(ctx) }() }`. |
| `.env.example` | +3 lines: `AUTOMATION_ENABLED=true`, `AUTOMATION_AUTOACTION_BUFFER_SIZE=100`, `AUTOMATION_WORKER_CONCURRENCY=1` (with comments). |

### Tests (NEW — §21.1 strict)

| File | Cases | Type |
|------|------:|------|
| `automation/rules_test.go` | 5+ | FloodRule under/at/over threshold, window expiration, user isolation |
| `automation/service_test.go` | 5+ | `enabled=false` skip, `bot≠admin` skip, hit increments, threshold automute enqueues, threshold autoban enqueues, already-at-threshold idempotent; fakes for `*Repository`, `LogWriter`, `GroupReader`, channel capture |
| `automation/repository_test.go` | 3+ | `OpenTestDB("automation")` + `database.Migrate` + `TRUNCATE groups, group_moderation_settings, user_warning_state CASCADE`; `GetOrCreate` defaults; `UpsertWarningState` atomic increment; `ListWarningStates` |
| `automation/worker_test.go` | 3+ | Fake `AutoActioner`; FIFO drain; ctx cancel; buffer overflow → drop (logged) |
| `automation/autoactioner_test.go` | 2+ | Re-check permission denial after bot demoted; happy path log written |
| `automation/events_subscriber_test.go` | 1+ | `Update.Message == nil` → handler not called; `Message.From == nil` → handler not called |

## Testing Strategy

| Layer | What | Approach |
|-------|------|----------|
| Unit | `FloodRule.Evaluate`, `Registry` short-circuit, `Service.HandleMessage` pipeline, `Worker.Run` FIFO + ctx, `AutoActioner` permission re-check | Hand-rolled fakes (no `moq` codegen); deterministic `nowFn` injected into rules; `telegram.Service` is a stub interface — never real Bot API |
| Integration | `Repository` against PostgreSQL real | `database.OpenTestDB(t, "automation")` + `database.Migrate` + `TRUNCATE ... CASCADE` (publications-slice3 pattern); `t.Skip` on `testing.Short()` and unreachable PG |
| E2E | Smoke: bot receives a message → `psql` shows `user_warning_state` row + log | Manual run with test bot; documented in slice-1 verify report |

## Migration / Rollout

- **Schema**: single goose migration `00006_create_moderation_automation.sql` (Up + Down). Applied automatically in dev (`RUN_MIGRATIONS=true`); manual step in prod per AGENTS §13.1.
- **Rollback**: `git revert` + `goose down` to 00005. Workers exit on ctx cancel. Audit logs preserved.
- **Kill switch**: `AUTOMATION_ENABLED=false` short-circuits subscriber registration and worker goroutine — no live traffic, no logs from automation. Verified in spec REQ-11.
- **Feature flagging**: None needed beyond `AUTOMATION_ENABLED`; per-group `enabled` toggle lives in `group_moderation_settings` (default `false`).

## Interfaces / Contracts

```go
// backend/internal/automation/rules.go
type Rule interface {
    Name() string
    Evaluate(ctx context.Context, msg *telegram.Message, s *Settings, ws *WarningState, now time.Time) *RuleHit
}

// backend/internal/automation/autoactioner.go (consumer-side, per publications pattern)
type AutoActioner interface {
    MuteUser(ctx context.Context, groupID, userID int64, untilDate int64) error
    BanUser(ctx context.Context, groupID, userID int64) error
}

// Non-blocking enqueue pattern in Service.HandleMessage (the only non-obvious bit)
select {
case s.autoActionCh <- action:
default:
    s.logger.Warn("automation: autoActionCh full, dropping", "rule", action.RuleName, "group_id", action.GroupID, "user_id", action.UserID)
}
```

## Open Questions Resolved

| Question | Resolution | Source |
|----------|------------|--------|
| Where does the subscriber wire? | `cmd/server/main.go` after existing `bus.Handle` calls; gated by `cfg.AutomationEnabled`. `Subscriber.Register()` calls `bus.Handle(s.Handle)`. | Spec REQ-10 |
| In-memory flood window storage | `FloodRule` holds `map[groupID]map[userID][]time.Time` with per-group `sync.Mutex`; pruning on each Evaluate keeps it bounded. **No** package-level cache for settings (read per message). | Exploration D10, slice 1 simplification |
| Channel overflow handling | Non-blocking send (`select` with `default`); drop + warn log. Buffer size `AUTOMATION_AUTOACTION_BUFFER_SIZE` (default 100). Bus never blocks. | Spec REQ-11 mitigation |
| Kill switch | `AUTOMATION_ENABLED=false` skips subscriber.Register + worker goroutine entirely; existing code paths untouched. | Spec REQ-11 scenario "AUTOMATION_ENABLED=false corta el pipeline" |

## Risks

| Risk | Likelihood | Mitigation |
|------|-----------|------------|
| `allowed_updates` doesn't include `message` | Low | Verified in real code: `telegram/poller.go:13` (`MVPAllowedUpdates = ["message",...]`); `cmd/server/main.go:183` passes `MVPAllowedUpdates` to `SetWebhook`. Smoke test post-merge: send real message → `psql -c "SELECT * FROM user_warning_state"` shows the row. |
| In-memory `FloodRule` map unbounded under load | Low | Pruning on every `Evaluate` drops timestamps older than `flood_seconds`. Idle groups/users leave empty slices (cheap). Worst case: one entry per `(group, user)` between hits. No memory leak across restarts — by design (resets on process start; spec accepts). |
| `autoActionCh` buffer overflow under flood | Medium | Non-blocking send + warn log + drop. `AUTOMATION_AUTOACTION_BUFFER_SIZE` configurable. Counter increment + log persist before enqueue, so we never lose the audit trail. |
| Bot admin removed between hit and dispatch | Low | `AutoActioner.dispatch` re-reads `groupsRepo.GetByTelegramID` + re-checks `permissionOkAdmin`; on failure logs `PERMISSION_DENIED` with `ActorID=nil` and skips Telegram call. |
| `permissionOkAdmin` reintroduces bug `#172` by copy-paste | Low | Helper defined **inside** `internal/automation/` (not exported); comment block explicitly references `#172` and forbids reuse of `moderation.permissionOk`; tests cover both admin and member `BotStatus`. |
| `warning_count` race between concurrent messages | Low | `UpsertWarningState` uses `INSERT ON CONFLICT DO UPDATE SET warning_count = user_warning_state.warning_count + 1` — atomic at the DB level; no read-modify-write race. |
| `moderation.Service` accidentally modified | Low | Slice 1 is backend-only; no `backend/internal/moderation/*` touches. Verify phase asserts `git diff main -- backend/internal/moderation/` empty. |

## Relevant Files (existing — pattern sources)

- `backend/internal/events/bus.go` — `Bus.Handle(Handler)` registration pattern.
- `backend/internal/telegram/poller.go:13` — `MVPAllowedUpdates` includes `message`.
- `backend/internal/telegram/service.go` — `MuteUser`/`BanUser` signatures (consumer-side contract).
- `backend/internal/publications/worker.go:192` — `permissionOk(g) = g.BotStatus == StatusAdministrator` (precedent for `permissionOkAdmin`).
- `backend/internal/publications/service.go:572` — same precedent, with bugfix comment.
- `backend/internal/logs/model.go` — `Entry{ActorID *int64}`, `Status*`, `Action*` constants.
- `backend/internal/database/{testdb.go,migrate.go}` — `OpenTestDB(t, name)` + `Migrate(ctx, db)` helpers.
- `backend/internal/groups/{model.go,repository.go}` — `Group.BotStatus`, `GetByTelegramID` for re-check.
- `backend/migrations/00004_create_publications.sql` — goose Up/Down style reference.
- `backend/cmd/server/main.go:179-215` — webhook + poller wiring; new subscriber/worker slot in here.

---

**Forecast**: ~1500 LOC touched (matches proposal estimate). Exceeds 400-line PR review budget → **`size:exception` confirmed**; precedent: 6 publications-slice PRs. Tasks phase will re-confirm or recommend chained (`backend-core` / `events-subscriber+worker`).