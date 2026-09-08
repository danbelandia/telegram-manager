# Tasks: `moderation-automation-slice2.1` — Warning to user before threshold action

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: size-exception
400-line budget risk: High

## Phase 1: Migration + models + Settings extension (compile gate)

- [x] 1.1 Create `backend/migrations/00008_add_warning_settings.sql` — goose Up/Down; ALTER TABLE `group_moderation_settings` adds `warn_user_enabled BOOLEAN NOT NULL DEFAULT true`, `warn_user_template TEXT NULL`.
- [x] 1.2 Modify `backend/internal/automation/model.go` — add `WarnUserEnabled bool` + `WarnUserTemplate *string` to `Settings`; `DefaultSettings{WarnUserEnabled: true, WarnUserTemplate: nil}`.
- [x] 1.3 Modify `backend/internal/automation/repository.go` — extend SELECT/INSERT/UPDATE + `scanSettings` for 2 new cols.
- [x] 1.4 Modify `backend/internal/automation/repository_test.go` — 2 integration tests via `OpenTestDB("automation")` + goose 00008 (defaults on insert; round-trip custom template).
- [x] 1.5 Verify: `go build ./...` AND `go test ./internal/automation/ -count=1` green.

## Phase 2: Templates + warning_sender (new modules)

- [x] 2.1 Create `backend/internal/automation/templates.go` — `TemplateKind`, `MessageContext`, 2 defaults (pre-mute + pre-ban), `RenderTemplate` with `{nombre}` (FirstName → Username sin `@` → `"este usuario"`), `{count}`, `{mute_minutes}`; unknown verbatim; fallback to default on empty/invalid.
- [x] 2.2 Create `backend/internal/automation/warning_sender.go` — `WarningKind`, `WarningSender` interface, `TelegramMesseger` view, `tgWarningSender` re-checks `permissionOkAdmin`, `context.WithTimeout(5s)`, calls `SendMessage(ctx, chatID, text, false, nil)`, logs `ActionWarnUserSent` with `ActorID=nil` + metadata `{warning_count, threshold_kind, template_used}`.
- [x] 2.3 Create `backend/internal/automation/templates_test.go` — ≥3 tests (FirstName, Username fallback, "este usuario" fallback, `{count}`/`{mute_minutes}`, empty→default, unknown verbatim). **13 tests added**.
- [x] 2.4 Create `backend/internal/automation/warning_sender_test.go` — ≥4 tests (happy SUCCESS, `ErrPermissionDenied` no panic, timeout 5s → TELEGRAM_ERROR, custom → metadata `template_used="custom"`). **9 tests added**.
- [x] 2.5 Verify: `go test ./internal/automation/ -count=1` green.

## Phase 3: Service integration + thresholdKindFor

- [x] 3.1 Modify `backend/internal/logs/model.go` — add `ActionWarnUserSent = "WARN_USER_SENT"` constant.
- [x] 3.2 Modify `backend/internal/automation/service.go` — add `warningSender WarningSender` field; extend `NewService`; in `HandleMessage` between upsert (paso 7) and threshold (paso 8) call `SendWarning` with `context.WithTimeout(5s)`; add `thresholdKindFor(s, count)` helper prioritizing pre-ban.
- [x] 3.3 Modify `backend/internal/automation/service_test.go` — update existing tests; add ≥5 cases: count==automute-1 (pre-mute), count==autoban-1 (pre-ban), count==0 skip, count==threshold skip, `WarnUserEnabled=false` skip, automute==autoban single warning, sender error → log warn + pipeline continues. **8 trigger tests + 7 thresholdKindFor tests added**.
- [x] 3.4 Verify: `go test ./internal/automation/ -count=1` green.

## Phase 4: main.go wiring + handlers validation

- [x] 4.1 Modify `backend/cmd/server/main.go` — construct `sender := automation.NewWarningSender(bot, logsRepo, settingsRepo, groupsRepo, slog.Default())` and pass to `NewService`; gate on `cfg.AutomationEnabled`.
- [x] 4.2 Modify `backend/internal/api/automation_handlers.go` — add `WarnUserEnabled *bool` + `WarnUserTemplate *string` to PUT body/response; validate `warn_user_template` ≤ 1000 chars (400 VALIDATION_ERROR if exceeded).
- [x] 4.3 Modify `backend/internal/api/automation_handlers_test.go` — extend PUT test with 2 new fields + >1000 chars returns 400. **3 tests added (PUT 2 fields + GET round-trip, template >1000 → 400, template ==1000 → 200)**.
- [x] 4.4 Verify: `go build ./...` green.

## Phase 5: Frontend Section 5

- [x] 5.1 Modify `frontend/src/features/automation/types.ts` — add `warn_user_enabled: boolean` + `warn_user_template: string | null` to `AutomationSettings`; both optional in `AutomationSettingsUpdate`; extend `AUTOMATION_DEFAULTS` (`warn_user_enabled: true`) + `withDefaults`.
- [x] 5.2 Modify `frontend/src/pages/GroupAutomationPage.tsx` — add Sección 5: Switch (label "Avisar al usuario antes de silenciar/expulsar") + Textarea (autosize minRows={2} maxRows={5} maxLength={1000} placeholder={defaultPreMuteTemplate}) + helper Text "Variables: {nombre}, {count}, {mute_minutes}".
- [x] 5.3 Modify `frontend/src/pages/GroupAutomationPage.test.tsx` — add 2 smoke tests (Switch render + Textarea in Save diff); preserve 8 slice 2 tests. **2 smoke tests added (8 slice 2 intact)**.
- [x] 5.4 Verify: `npm test -- --run` AND `npm run build` green.

## Phase 6: Full suite + non-regression

- [x] 6.1 `go test ./... -count=1` green; `go vet ./...` clean; `gofmt -l .` empty.
- [x] 6.2 `npm test -- --run` AND `npm run build` green (PublicationsPage pre-existing failures unrelated to slice 2.1 — confirmed via `git stash` test on main).
- [x] 6.3 Non-regression diff empty: `git diff main -- backend/internal/moderation/ backend/internal/publications/ frontend/src/pages/{Publications,GroupUsers,GroupRequests,GroupLogs,Dashboard,Login,Groups,GroupDetail}Page.tsx` → **0 lines**.
- [x] 6.4 `grep can_ backend/internal/automation/*.go | grep -v '^//'` returns **0 matches**.

## Phase 7: README + commits

- [x] 7.1 Update root `README.md` — add "Warning al usuario" section under "Moderación automática" (toggle default ON, `warn_user_template` ≤ 1000 chars, variables, link spec REQ-22..31).
- [x] 7.2 Conventional commits per repo style — do NOT push. **6 commits**: 16687c9 migration+model+repository+log-const, c79c697 templates+warning_sender, de6aad3 service paso 7.5+thresholdKindFor, cbebad7 main+handlers, fb1693a frontend section 5, ca352a4 README.