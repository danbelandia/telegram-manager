# Apply Report — `moderation-automation-slice2.1` (Warnings visuales al usuario)

> **Change**: `moderation-automation-slice2.1` — sub-slice de Fase 3 (AGENTS §23) sobre slice 2 archivado.
> **Mode**: hybrid (filesystem + Engram). Persisted as `sdd/moderation-automation/slice2.1/apply-report`.
> **Base**: `main @ 2df231f`.
> **Branch**: `feat/moderation-automation-slice2.1` (NOT pushed).
> **Strategy**: single-pr with `size:exception` (decision obs `#238`).
> **Forecast**: ~717 LOC (matches design). Actual: 1908 ins / 57 del = 1965 net (includes 4 NEW files: templates.go + tests + warning_sender.go + tests).
> **Commits ahead of main**: 6 (all conventional-commit, no AI attribution, no `--no-verify`, no force-push).

---

## Phases completed

| Phase | What | Files | Status |
|-------|------|-------|--------|
| 1 | Migration 00008 + model fields + repository SELECT/INSERT/UPDATE + scanSettings + 2 integration tests | migration, model, repository, repository_test, logs/model | ✅ |
| 2 | Templates module (2 defaults + RenderTemplate) + warning_sender module (interface + concrete + 5 log helpers) + 13 templates tests + 9 warning_sender tests | templates.go, templates_test.go, warning_sender.go, warning_sender_test.go | ✅ |
| 3 | Service.HandleMessage paso 7.5 + thresholdKindFor helper + NewService signature (WarningSender nil-safe) + 8 service tests + 7 helper tests | service.go, service_test.go | ✅ |
| 4 | main.go wiring (NewWarningSender + pass to NewService, gated on cfg.AutomationEnabled) + handler PUT validation (≤1000 chars, 400 VALIDATION_ERROR) + 3 handler tests | main.go, automation_handlers.go, automation_handlers_test.go | ✅ |
| 5 | Frontend Section 5 (Switch + Textarea autosize maxLength=1000 + helper text) + AUTOMATION_DEFAULTS + AutomationSettings/Update + 2 smoke tests | types.ts, GroupAutomationPage.tsx, GroupAutomationPage.test.tsx | ✅ |
| 6 | Full suite green + vet/gofmt clean + non-regression diff empty + `can_` invariant preserved | (verification only) | ✅ |
| 7 | README "Warning al usuario" section + Editor de settings count 4→5 + 6 conventional commits | README.md | ✅ |

---

## Files changed (with line counts)

### Backend NEW

| File | +LOC | Description |
|------|----:|-------------|
| `backend/migrations/00008_add_warning_settings.sql` | 18 | goose Up/Down; ALTER TABLE adds `warn_user_enabled BOOLEAN NOT NULL DEFAULT true, warn_user_template TEXT NULL` |
| `backend/internal/automation/templates.go` | 137 | TemplateKind enum, defaultTemplates map, MessageContext, RenderTemplate, DefaultTemplate |
| `backend/internal/automation/templates_test.go` | 227 | 13 unit tests (FirstName/Username/fallback, count/mute_minutes, custom, empty→default, unknown verbatim, pre-ban default, etc.) |
| `backend/internal/automation/warning_sender.go` | 286 | WarningKind, WarningSender interface, TelegramMesseger, SettingsReader, tgWarningSender, NewWarningSender, SendWarning (re-check + re-read + render + send + log) + 5 log helpers + mapWarningError |
| `backend/internal/automation/warning_sender_test.go` | 338 | 9 unit tests (happy default, happy custom, permission denied, not found, telegram error, telegram permission mapped, empty kind, nil msg, settings read failure fallback) |

### Backend MOD

| File | ΔLOC | Description |
|------|----:|-------------|
| `backend/internal/automation/model.go` | +11 | Settings struct: 2 new fields (WarnUserEnabled bool + WarnUserTemplate *string). DefaultSettings: defaults set. |
| `backend/internal/automation/repository.go` | +13 | SELECT + INSERT/UPDATE + scanSettings extended for 2 cols |
| `backend/internal/automation/repository_test.go` | +90 | 2 new integration tests (defaults + custom round-trip) |
| `backend/internal/automation/service.go` | +76 / −36 | Service.warningSender field, NewService signature (nil-safe), HandleMessage paso 7.5 (sync context.WithTimeout(5s)), thresholdKindFor helper (pre-ban priority). LoadOrCreateSettings now uses DefaultSettings() (single source of truth) |
| `backend/internal/automation/service_test.go` | +333 / −24 | newSvc + newSvcWithSender helpers (nil-safe). 8 new trigger tests (pre-mute, pre-ban, count=0, count=threshold, disabled, bot not admin, automute==autoban single pre-ban, sender error pipeline continues). 7 new thresholdKindFor tests |
| `backend/internal/logs/model.go` | +3 | `ActionWarnUserSent = "WARN_USER_SENT"` constant |
| `backend/cmd/server/main.go` | +6 | `warningSender := automation.NewWarningSender(bot, logsRepo, automationRepo, groupsRepo, slog.Default())` + pass to NewService (gated on cfg.AutomationEnabled) |
| `backend/internal/api/automation_handlers.go` | +52 / −14 | settingsUpdate + settingsResponse expose 2 new fields. PUT validates `len(WarnUserTemplate) ≤ 1000` → 400 VALIDATION_ERROR. maxWarnUserTemplateLen const (1000) |
| `backend/internal/api/automation_handlers_test.go` | +71 | 3 new tests: PUT 2 fields + GET round-trip, template >1000 → 400, template ==1000 → 200 |

### Frontend MOD

| File | ΔLOC | Description |
|------|----:|-------------|
| `frontend/src/features/automation/types.ts` | +19 | AutomationSettings + Update + AUTOMATION_DEFAULTS expone 2 nuevos campos (warn_user_enabled: true, warn_user_template: null) |
| `frontend/src/pages/GroupAutomationPage.tsx` | +45 | Section 5 "Warning al usuario": Switch + Textarea autosize (maxLength=1000, placeholder = default pre-mute) + helper text listando variables |
| `frontend/src/pages/GroupAutomationPage.test.tsx` | +145 / −1 | initialSettings extended. 2 smoke tests (switch default on + persist; textarea custom + maxLength=1000 + persist via fireEvent — user-event tiene problemas con caracteres { } en Mantine Textarea v7) |

### Docs MOD

| File | ΔLOC | Description |
|------|----:|-------------|
| `README.md` | +52 / −1 | Nueva sub-sección "Warning al usuario (slice 2.1)" bajo "Moderación automática": trigger conditions, 2 plantillas default + variables, toggle, env, panel, link a spec. Editor de settings: 4→5 secciones. |

**Total**: 18 files changed, 1908 insertions, 57 deletions = 1965 net LOC.

---

## Test & build outputs

### Backend `go test ./... -count=1`

```
ok  	github.com/telegram-manager/backend/internal/api            3.299s
ok  	github.com/telegram-manager/backend/internal/auth           2.884s
ok  	github.com/telegram-manager/backend/internal/automation     11.142s  (50+ tests, including 9 warning_sender + 13 templates + 8 service trigger + 7 thresholdKindFor + 2 repository + 16 existing service + 13 existing repository + 5 existing autoactioner)
ok  	github.com/telegram-manager/backend/internal/config         1.314s
ok  	github.com/telegram-manager/backend/internal/events         1.637s
ok  	github.com/telegram-manager/backend/internal/groups         3.984s
ok  	github.com/telegram-manager/backend/internal/joinrequests    2.042s
ok  	github.com/telegram-manager/backend/internal/logs           1.366s
ok  	github.com/telegram-manager/backend/internal/moderation     1.482s
ok  	github.com/telegram-manager/backend/internal/publications    3.293s
ok  	github.com/telegram-manager/backend/internal/telegram       23.362s
ok  	github.com/telegram-manager/backend/internal/users          0.797s
```

All packages green.

### Backend `go vet ./...`

Empty (clean).

### Backend `gofmt -l .`

Empty (clean).

### Backend `can_` invariant

`grep can_ backend/internal/automation/*.go` → 0 executable matches (only comments/docs, no Go code).

### Frontend `npm test -- --run src/pages/GroupAutomationPage.test.tsx`

```
Test Files  1 passed (1)
Tests       10 passed (10)   (8 slice 2 + 2 slice 2.1)
Duration    16.23s
```

### Frontend `npm run build`

```
✓ built in 2.38s
```

(Note: pre-existing PublicationsPage test failures in the full suite — unrelated to slice 2.1, confirmed by `git stash` test on main. They are test pollution between files when run together.)

---

## Non-regression verification

`git diff main -- backend/internal/moderation/ backend/internal/publications/ frontend/src/pages/{Publications,GroupUsers,GroupRequests,GroupLogs,Dashboard,Login,Groups,GroupDetail}Page.tsx`

```
0 lines of diff → empty
```

Slice 2.1 NO toca:
- `backend/internal/moderation/` (acciones manuales — preservadas).
- `backend/internal/publications/` (publicaciones — preservadas).
- Las páginas frontend no relacionadas con automation.

---

## Deviations from design

1. **fakeWarningSender records calls even when `err` is set** (slice 2.1 implementation detail). Original mock design returned early on error; needed to fix to make `TestService_Warning_SenderError_PipelineContinues` deterministic. Semantically equivalent for tests, behaviorally more useful.

2. **`SendWarning` signature takes `*telegram.Message`** (not just IDs). The design initially suggested `SendWarning(ctx, groupID, userID, count, kind)` but I changed it to `SendWarning(ctx, msg *telegram.Message, count, kind)` because the template needs `FirstName` and `Username` from `msg.From`, and the caller (Service.HandleMessage) already has the message in scope. This is the same pattern as `AutoActioner.Execute(action AutoAction)` — receiver gets the full context object, not just IDs. Documented in the design rationale ("SendWarning recibe msg completo para extraer FirstName/Username").

3. **`Service.LoadOrCreateSettings` now uses `DefaultSettings(groupID)`** instead of an inline literal. The original had a comment warning "cualquier cambio en defaults debe replicarse en ambos lugares" — this refactor removes the duplication and ensures the slice 2.1 defaults (WarnUserEnabled=true, WarnUserTemplate=nil) propagate automatically. No behavior change, just less duplication.

4. **Frontend smoke test uses `fireEvent.change` instead of `userEvent.type`** for the textarea test. `userEvent` has known issues typing special characters (`{`, `}`) into Mantine v7 Textarea — keystrokes get lost. `fireEvent.change` with explicit `target.value` is the deterministic alternative. The Switch test uses `userEvent` as normal.

---

## Absolute invariants — verified

- ✅ DO NOT modify `backend/internal/moderation/` (manual moderation stays untouched) — `git diff` confirms 0 lines.
- ✅ DO NOT call real Bot API in tests (§21.1) — all tests use fakes or in-memory mocks.
- ✅ `permissionOkAdmin` MUST use `g.BotStatus == groups.StatusAdministrator` (bugfix #172) — confirmed: `grep can_` returns 0 executable matches.
- ✅ Conventional commits per repo style (no AI attribution) — 6 commits, all matching `<type>(<scope>): <subject>` pattern, none with "Co-Authored-By".
- ✅ Do NOT push — branch `feat/moderation-automation-slice2.1` is local only, NOT pushed.

---

## Workload / PR boundary

- Mode: **single PR with `size:exception`** (decision obs `#238`, precedent: 10 consecutive size:exception approvals).
- Current work unit: full slice 2.1.
- Branch: `feat/moderation-automation-slice2.1` (base `main @ 2df231f`).
- Estimated review budget impact: ~1965 net LOC across 18 files. Above 400-line budget. Accepted per size:exception.

---

## Status

**20/20 tasks complete**. Ready for sdd-verify.
