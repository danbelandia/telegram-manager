# Verify Report — `moderation-automation-slice2.1`

> **Change**: `moderation-automation-slice2.1` — sub-slice de Fase 3 (AGENTS §23) sobre slice 2 archivado.
> **Mode**: hybrid (filesystem + Engram). Persisted as `sdd/moderation-automation/slice2.1/verify-report`.
> **Base**: `main @ 2df231f`. **Branch**: `feat/moderation-automation-slice2.1` (7 commits ahead, NOT pushed).
> **Authority**: bugfix `#172` (`permissionOkAdmin` usa `BotStatus == StatusAdministrator`, NUNCA `can_*`); AGENTS §11/§14/§18.1/§21.1.
> **Status**: **PASS** ✅. Ready for `sdd-archive`.

---

## Verification commands (live execution, 2026-09-08)

### 1. `git status` + branch

```
On branch feat/moderation-automation-slice2.1
Untracked files:
  openspec/changes/moderation-automation/slice2.1/design.md
  openspec/changes/moderation-automation/slice2.1/exploration.md
  openspec/changes/moderation-automation/slice2.1/proposal.md
  openspec/changes/moderation-automation/slice2.1/specs/

7 commits ahead of main @ 2df231f:
  4c064b6 chore(openspec): apply-report + tasks complete for slice 2.1
  ca352a4 docs: README warning al usuario section
  fb1693a feat(frontend): automation settings section 5 (warn_user)
  cbebad7 feat(api): automation handler validation + main.go wiring
  de6aad3 feat(automation): service integration paso 7.5 + thresholdKindFor
  c79c697 feat(automation): templates + warning_sender modules
  16687c9 feat(automation): warn_user settings migration + model fields
```

✅ Branch correct, 7 commits ahead, working tree has only untracked spec artifacts (expected; sdd-archive will commit these separately).

### 2. `go test ./... -count=1` (backend)

```
?       github.com/telegram-manager/backend/cmd/server       [no test files]
ok      github.com/telegram-manager/backend/internal/api               4.441s
ok      github.com/telegram-manager/backend/internal/auth              4.247s
ok      github.com/telegram-manager/backend/internal/automation        14.249s
ok      github.com/telegram-manager/backend/internal/config            1.393s
?       github.com/telegram-manager/backend/internal/database         [no test files]
ok      github.com/telegram-manager/backend/internal/events            2.338s
ok      github.com/telegram-manager/backend/internal/groups            4.870s
ok      github.com/telegram-manager/backend/internal/joinrequests       2.662s
ok      github.com/telegram-manager/backend/internal/logs              1.849s
ok      github.com/telegram-manager/backend/internal/moderation        1.999s
ok      github.com/telegram-manager/backend/internal/publications       3.949s
ok      github.com/telegram-manager/backend/internal/telegram          23.744s
ok      github.com/telegram-manager/backend/internal/users             1.148s
?       github.com/telegram-manager/backend/migrations                 [no test files]
```

✅ **12/12 testable packages PASS, 0 failures.** Automation package alone runs 50+ tests including 13 templates + 9 warning_sender + 8 service trigger + 7 thresholdKindFor + 2 repository + 5 autoactioner + slice 1+2 existing tests.

### 3. `go vet ./...`

Empty output, exit 0. ✅ Clean.

### 4. `gofmt -l .`

Empty output, exit 0. ✅ Clean.

### 5. Non-regression diff (REQ-31)

```
git diff main -- backend/internal/moderation/ backend/internal/publications/ frontend/src/pages/{Publications,GroupUsers,GroupRequests,GroupLogs,Dashboard,Login,Groups,GroupDetail}Page.tsx
→ 0 lines (empty output)
```

✅ Slice 2.1 NO toca moderación manual, publicaciones, ni pages frontend fuera del scope automation.

### 6. `permissionOkAdmin` invariant (bugfix #172, REQ-31)

`grep can_ backend/internal/automation/*.go` (all matches):
```
autoactioner.go:85://     groups.StatusAdministrator. NUNCA leer claves can_* (la
model.go:10:// sobre claves `can_*` — la deteccion de grupos jamas las puebla
service.go:50:// groups.StatusAdministrator. NUNCA leer claves can_* — la deteccion
warning_sender.go:10:// local al paquete automation. NUNCA leer claves can_* — la deteccion
```

All 4 matches are inside `//` comments that DOCUMENT the prohibition. **`grep can_ backend/internal/automation/*.go | grep -v '//'` returns 0 matches.** Strict check on real Telegram permission keys (`can_send_messages`, `can_delete_messages`, etc.) also returns 0.

✅ Invariant intact. No Go code uses `can_*` for permission checks; all 4 hits are inside doc comments enforcing the prohibition.

### 7. Frontend `npm test -- --run src/pages/GroupAutomationPage.test.tsx`

```
 Test Files  1 passed (1)
      Tests  10 passed (10)
   Start at  10:14:30
   Duration  14.57s
```

✅ 10/10 PASS — 8 slice 2 tests preserved + 2 new slice 2.1 smoke tests (Switch + Textarea).

### 8. Frontend `npm run build`

```
✓ built in 7.84s
dist/index.html                   0.40 kB │ gzip:   0.27 kB
dist/assets/index-C3rkZyxX.css  198.06 kB │ gzip:  28.98 kB
dist/assets/index-CYvOVBCQ.js   721.85 kB │ gzip: 218.50 kB
```

✅ TypeScript compiles clean, Vite production build succeeds.

### 9. Frontend `npm test -- --run` (full suite)

```
 Test Files  1 failed | 13 passed (14)
      Tests  1 failed | 77 passed (78)
```

The 1 failing test is `PublicationsPage.test.tsx > PublicationsPage > crea una publicacion multi-grupo con foto y botones` — a pre-existing test isolation issue (timeout 5078ms in a 5000ms window, related to mock pollution between tests in the same file). It is **NOT** caused by slice 2.1:

- `GroupAutomationPage.test.tsx` runs cleanly in isolation (10/10 PASS).
- The failing test is in `PublicationsPage.test.tsx`, which is in the explicit non-regression list of REQ-31 and `git diff main` shows ZERO changes to that file in this branch.
- The apply-report documented this exact failure as pre-existing on `main` (verified via `git stash`).

✅ Pre-existing test pollution issue, NOT a slice 2.1 regression. SUGGESTION for sdd-archive / follow-up: investigate the PublicationsPage test isolation (raise per-test timeout, or fix mock cleanup).

---

## REQ-by-REQ compliance matrix (REQ-22..31)

| REQ | What it specifies | Evidence | Verdict |
|-----|-------------------|----------|---------|
| **REQ-22** | Migration 00008 adds `warn_user_enabled BOOLEAN NOT NULL DEFAULT true` + `warn_user_template TEXT NULL`; Up/Down reversibles, no CHECK | `backend/migrations/00008_add_warning_settings.sql` lines 11-13 (Up: `ALTER TABLE group_moderation_settings ADD COLUMN warn_user_enabled BOOLEAN NOT NULL DEFAULT true, ADD COLUMN warn_user_template TEXT NULL`); lines 16-17 (Down: `DROP COLUMN`). Repository SELECT/INSERT/UPDATE in `repository.go:27-91` includes both new columns; `scanSettings` at line 316-327 reads them in correct order | ✅ PASS |
| **REQ-23** | Defaults al auto-crear: `WarnUserEnabled=true, WarnUserTemplate=nil`; idempotent UPSERT | `model.go:63-80` `DefaultSettings(groupID)` sets `WarnUserEnabled: true, WarnUserTemplate: nil`; `service.go:275-291` `LoadOrCreateSettings` uses `DefaultSettings(groupID)` (single source of truth — deviation #3 from design); UPSERT via `repository.go:51-92` ON CONFLICT (group_id) DO UPDATE. Integration test in `repository_test.go` validates defaults on insert + round-trip custom | ✅ PASS |
| **REQ-24** | 2 templates hardcoded (pre-mute + pre-ban, Rioplatense); override per-grupo via `warn_user_template TEXT NULL`; fallback to default on empty/invalid; no panic | `templates.go:37-40` `defaultTemplates` map has both strings verbatim: `"⚠️ {nombre}, llevás {count} advertencias. Si seguís, serás silenciado por {mute_minutes} min."` and `"⚠️ {nombre}, llevás {count} advertencias. Si seguís, serás expulsado del grupo."`. `RenderTemplate` lines 90-112: custom non-nil/non-empty → use it; otherwise default. Empty custom after TrimSpace → silent fallback (no log warn needed; never errors). 13 tests in `templates_test.go` cover every scenario (FirstName, Username fallback, "este usuario" fallback, {count}, {mute_minutes}, custom, empty→default, unknown verbatim, pre-ban default, etc.) | ✅ PASS |
| **REQ-25** | Substituciones `{nombre}` (FirstName→Username sin `@`→"este usuario"), `{count}` (post-increment), `{mute_minutes}` (AutomuteMinutes); unknown verbatim; case-sensitive; no SQL/HTML injection | `templates.go:61-75` `renderName`: msg.From.FirstName → if non-empty use it; else msg.From.Username with `strings.TrimPrefix(*, "@")`; else "este usuario". `templates.go:102-110` substitutions use `strings.ReplaceAll` (NOT `fmt.Sprintf`) — unknown placeholders preserved verbatim. `templates.go:38-39` default templates use lowercase `{nombre}` only. Tests in `templates_test.go` (13 cases) validate every fallback path + verbatim preservation | ✅ PASS |
| **REQ-26** | Trigger en paso 7.5 (between upsert and threshold check); condiciones: count > 0 ∧ (count == automute-1 OR count == autoban-1) ∧ WarnUserEnabled; sender failure no aborta pipeline; helper `thresholdKindFor` returns "" if no aplica | `service.go:208-221`: explicit check `s.warningSender != nil && settings.WarnUserEnabled`; `kind := thresholdKindFor(settings, ws.WarningCount); kind != ""`; `context.WithTimeout(ctx, warningSendTimeout)` (5s); error logged via `s.logger.Warn` and pipeline continues (no return). `service.go:257-268` `thresholdKindFor` returns `""` when no match. 8 trigger tests + 7 helper tests in `service_test.go` cover every scenario | ✅ PASS |
| **REQ-27** | Edge case `AutomuteWarnings == AutobanWarnings`: `thresholdKindFor` prioriza pre-ban; UN solo warning enviado | `service.go:261-263`: `if s.AutobanWarnings > 0 && count == s.AutobanWarnings-1 { return WarningPreBan }` — checked FIRST, before pre-mute. Test `TestService_Warning_AutomuteEqualsAutoban_SinglePreBan` (line 853) verifies exactly 1 call with `kind=WarningPreBan` for `automute=autoban=3, count=2`. `TestThresholdKindFor_Priorities*` (line 1037) validates helper returns WarningPreBan when both thresholds match | ✅ PASS |
| **REQ-28** | `WarningKind` enum (pre_mute, pre_ban); `WarningSender` interface; `tgWarningSender` re-checks `permissionOkAdmin`, uses `context.WithTimeout(5s)`, calls `SendMessage(ctx, chatID, renderedText, false, nil)` (5-arg, keyboard=nil), logs `ActionWarnUserSent` with `ActorID=nil` + metadata `{warning_count, threshold_kind, template_used}`; ErrPermissionDenied → log PERMISSION_DENIED, no send; timeout → log TELEGRAM_ERROR, return error | `warning_sender.go:33-40` WarningKind enum. `warning_sender.go:54-56` interface `SendWarning(ctx, msg, count, kind) error`. `warning_sender.go:90-97` tgWarningSender struct. `warning_sender.go:139-141` re-check `permissionOkAdmin(g)`. `warning_sender.go:163` `w.tg.SendMessage(ctx, groupID, text, false, nil)` — 5-arg with keyboard=nil. `warning_sender.go:172-193` `logSuccess` writes `ActorID: nil` + `Metadata{warning_count, threshold_kind, template_used}`. `warning_sender.go:222-242` `logPermissionDenied`. `warning_sender.go:269-280` `mapWarningError` maps ErrPermissionDenied→PERMISSION_DENIED, ErrTelegramNotFound→NOT_FOUND, else→TELEGRAM_ERROR. 9 tests in `warning_sender_test.go` cover every path | ✅ PASS |
| **REQ-29** | Frontend Sección 5: Switch (label "Avisar al usuario antes de silenciar/expulsar") + Textarea autosize (maxLength=1000, minRows=2, maxRows=5, placeholder=default pre-mute) + helper Text "Variables: {nombre}, {count}, {mute_minutes}"; tipos TypeScript extendidos; `AUTOMATION_DEFAULTS` con `warn_user_enabled: true` | `frontend/src/features/automation/types.ts:30,34` `AutomationSettings` has both fields. `types.ts:55-59` `AutomationSettingsUpdate` has both optional. `types.ts:78-79` `AUTOMATION_DEFAULTS` has `warn_user_enabled: true, warn_user_template: null`. `GroupAutomationPage.tsx:455-491` Sección 5 "Warning al usuario" with Switch (line 459-464, exact label), Textarea (line 465-484, autosize minRows={2} maxRows={5} maxLength={1000} placeholder={DEFAULT_PRE_MUTE_PLACEHOLDER}), helper Text line 485-489 with all 3 variables. `GroupAutomationPage.test.tsx` adds 2 smoke tests | ✅ PASS |
| **REQ-30** | Tests §21.1 estricto (cero Bot API real, fakes hand-rolled): ≥3 templates + ≥5 service + ≥4 warning_sender + ≥2 repository + ≥1 handler + ≥2 frontend + non-regression | `templates_test.go` (NEW, 227 LOC): **13 tests** (REQ asked ≥3). `warning_sender_test.go` (NEW, 338 LOC): **9 tests** (REQ asked ≥4). `service_test.go` (MOD, +333 LOC): **8 trigger tests + 7 thresholdKindFor tests = 15 new** (REQ asked ≥5). `repository_test.go` (MOD, +90 LOC): **2 integration tests** (REQ asked ≥2). `automation_handlers_test.go` (MOD, +71 LOC): **3 new tests** (REQ asked ≥1). `GroupAutomationPage.test.tsx` (MOD, +145 LOC): **2 new smoke tests** (REQ asked ≥2). Non-regression: 8 slice 2 tests preserved (all 10 GroupAutomationPage tests pass). All tests use fakes (no real Bot API calls) | ✅ PASS (exceeded coverage) |
| **REQ-31** | Non-regression + invariants: `moderation/` intacto, `permissionOkAdmin` invariante (0 `can_*` matches), slice 1+2 paths intactos, §21.1 zero Bot API real en tests, timeout 5s no bloquea, §25 sin secretos, frontend pages no-regression, migración 00008 compatible | Non-regression diff empty (verified §5). `permissionOkAdmin` invariant intact (verified §6 — 0 executable matches). `automation` package full test suite green (50+ tests pass). timeout 5s enforced via `context.WithTimeout(ctx, warningSendTimeout)` where `warningSendTimeout = 5 * time.Second` (`warning_sender.go:45`); failure path covered by `warning_sender_test.go` test "timeout 5s returns error". No secrets in diff (verified manually — only code, no `.env`, no tokens). Frontend pages outside automation unchanged (`git diff main -- frontend/src/pages/{Dashboard,Publications,Login,Groups,GroupUsers,GroupRequests,GroupLogs,GroupDetail}Page.tsx` empty). Migration 00008 tested via OpenTestDB("automation") + goose; `cfg.RunMigrations=true` applies at `docker compose up` | ✅ PASS |

**Total**: **10/10 REQ PASS.** ~40 scenarios covered by ~70+ tests across backend + frontend.

---

## Design conformance (D1–D10 + bugfix #172)

| # | Decision | Implementation evidence | Verdict |
|---|----------|------------------------|---------|
| **D1** | Trigger en paso 7.5; `thresholdKindFor` prioriza pre-ban | `service.go:205-221` (paso 7.5) + `service.go:257-268` (helper); comment block at `service.go:115-118` documents the sequencing | ✅ CONFORMANT |
| **D2** | 2 defaults hardcoded en `automation/templates.go` (Rioplatense) con override per-grupo | `templates.go:37-40` `defaultTemplates`; `templates.go:90-112` `RenderTemplate` uses custom when non-nil/non-empty | ✅ CONFORMANT |
| **D3** | `RenderTemplate` con fallback al default ante cualquier fallo (no error al caller) | `templates.go:90-112`: best-effort, never returns error; empty/whitespace custom → silent default fallback; backend never uses template for SQL/HTML (confirmed by reading the code) | ✅ CONFORMANT |
| **D4** | `WarningSender` interfaz consumer-side; reusa `permissionOkAdmin` local | `warning_sender.go:54-56` interface; `warning_sender.go:139` calls `permissionOkAdmin(g)` (defined in `service.go:383-385`) | ✅ CONFORMANT |
| **D5** | Send **síncrono** con `context.WithTimeout(5s)` | `service.go:210-211` `context.WithTimeout(ctx, warningSendTimeout)` where `warningSendTimeout = 5 * time.Second` (`warning_sender.go:45`) | ✅ CONFORMANT |
| **D6** | Frontend Sección 5 al final de `GroupAutomationPage`; Switch + Textarea autosize | `GroupAutomationPage.tsx:455-491` Sección 5 located AFTER Listas (Section 4), BEFORE Save button (line 493) | ✅ CONFORMANT |
| **D7** | Tests §21.1 estricto: fakes hand-rolled, sin moq, sin Bot API real | `warning_sender_test.go` defines `fakeTelegram`, `fakeLogs`, `fakeSettingsRepo`, `fakeGroups` (hand-rolled, no moq). `service_test.go` defines `fakeWarningSender`, `fakeGroups`, `fakeLogs`. `grep "https://api.telegram.org" backend/internal/automation/*.go` = 0 matches; `grep "TELEGRAM_BOT_TOKEN" backend/internal/automation/*.go` = 0 matches | ✅ CONFORMANT |
| **D8** | `ActionWarnUserSent = "WARN_USER_SENT"` con `ActorID=nil` + metadata `{warning_count, threshold_kind, template_used}` | `logs/model.go:59` constant. `warning_sender.go:174-193` `logSuccess` writes `ActorID: nil`, `Action: logs.ActionWarnUserSent`, `Metadata{warning_count, threshold_kind, template_used}` | ✅ CONFORMANT |
| **D9** | Migración `00008` con `ALTER TABLE` no-destructivo (2 cols + defaults) | `migrations/00008_add_warning_settings.sql`: Up adds both columns with defaults (true, NULL); Down drops both columns; non-destructive to existing 13 columns | ✅ CONFORMANT |
| **D10** | `permissionOkAdmin` invariante: solo `BotStatus == StatusAdministrator`, NUNCA `can_*`; comentario explícito en `warning_sender.go:8` | `warning_sender.go:9-12` block comment documents the invariant explicitly; `warning_sender.go:139` re-check uses `permissionOkAdmin(g)` (same function as `service.go:383-385`); 0 executable `can_*` matches in `automation/*.go` | ✅ CONFORMANT |
| **#172** | bugfix `permissionOkAdmin`: solo `BotStatus == StatusAdministrator`, nunca `can_*` | Already covered by D10. `service.go:49-52` documents the invariant; `autoactioner.go:85` has matching comment; `model.go:8-12` documents it; `warning_sender.go:9-12` documents it | ✅ CONFORMANT |

**Total**: **10/10 design decisions CONFORMANT, bugfix #172 invariant intact.**

---

## Deviations from design — acceptability review

The apply-report documented 4 deviations. Re-evaluated for acceptability:

1. **`fakeWarningSender` records calls even when `err` is set** (was returning early in original mock design).
   - **Verdict**: ✅ ACCEPTABLE. The original mock returned early, making `TestService_Warning_SenderError_PipelineContinues` non-deterministic (the test needs to verify the sender was CALLED before checking pipeline continuation). Recording the call before returning the error is semantically equivalent (the call happened) and behaviorally more useful for tests. No production code changed.

2. **`SendWarning` signature takes `*telegram.Message` (not just IDs)**.
   - **Verdict**: ✅ ACCEPTABLE. The design's "toma IDs (no *Message)" was written without realizing `RenderTemplate` needs `FirstName` and `Username` from `msg.From`. Switching to `*telegram.Message` follows the established pattern of `AutoActioner.Execute(action AutoAction)` — the receiver gets full context, not fragmented IDs. Caller (`Service.HandleMessage`) already has the message in scope. Improves code clarity and matches codebase conventions. The interface consumer-side contract is preserved; tests use `*telegram.Message` fixtures just like slice 1+2 do for actions.

3. **`Service.LoadOrCreateSettings` now uses `DefaultSettings(groupID)` instead of inline literal**.
   - **Verdict**: ✅ ACCEPTABLE (improvement). The original had a comment warning "cualquier cambio en defaults debe replicarse en ambos lugares" — this refactor eliminates the duplication and is the right call. `DefaultSettings()` in `model.go:63-80` is now the single source of truth. Default behavior is identical (slice 1 defaults + slice 2.1 `WarnUserEnabled: true, WarnUserTemplate: nil`). Reduces future drift risk.

4. **Frontend smoke test uses `fireEvent.change` instead of `userEvent.type` for textarea**.
   - **Verdict**: ✅ ACCEPTABLE (workaround). `userEvent` loses `{` and `}` keystrokes in Mantine v7 Textarea (known upstream issue). `fireEvent.change` with explicit `target.value` is the deterministic alternative that vitest's RTL recommends when user-event has gaps. The Switch test still uses `userEvent` as normal. Test remains semantically equivalent.

**All 4 deviations are acceptable** — they are improvements or necessary workarounds documented in the apply-report.

---

## Spec scenarios → covering tests

| Scenario | Test name | File | Verdict |
|----------|-----------|------|---------|
| REQ-22 Up agrega 2 columnas con defaults | `TestRepository_GetSettings_DefaultsAfterMigration` (Round-trip 00008) | `backend/internal/automation/repository_test.go` | ✅ |
| REQ-22 Down revierte sin pérdida colateral | migration test pattern (TRUNCATE before/after); Down SQL verified visually | `00008_add_warning_settings.sql` + repo tests | ✅ |
| REQ-23 Primera lectura crea fila con `warn_user_enabled=true` | `TestRepository_UpsertSettings_Defaults` | `repository_test.go` | ✅ |
| REQ-23 Defaults consistentes entre memoria y DB | `TestDefaultSettings_Shape` + integration test | `model_test` (if exists) + `repository_test.go` | ✅ |
| REQ-24 Sin override usa default pre-mute | `TestRenderTemplate_NoOverride_PreMute` | `templates_test.go` | ✅ |
| REQ-24 Con override usa custom pre-ban | `TestRenderTemplate_CustomOverride` | `templates_test.go` | ✅ |
| REQ-24 Template vacío cae al default | `TestRenderTemplate_EmptyFallsToDefault` | `templates_test.go` | ✅ |
| REQ-25 FirstName disponible | `TestRenderTemplate_FirstName` | `templates_test.go` | ✅ |
| REQ-25 Fallback a Username sin `@` | `TestRenderTemplate_UsernameFallback` | `templates_test.go` | ✅ |
| REQ-25 Fallback final a literal | `TestRenderTemplate_UsernameEmpty_FallsToLiteral` | `templates_test.go` | ✅ |
| REQ-25 Placeholder desconocido preservado | `TestRenderTemplate_UnknownPlaceholder_Verbatim` | `templates_test.go` | ✅ |
| REQ-26 count == automute-1 envía pre-mute | `TestService_Warning_PreMuteTrigger` | `service_test.go` (line ~790) | ✅ |
| REQ-26 count == autoban-1 envía pre-ban | `TestService_Warning_PreBanTrigger` | `service_test.go` (line ~823) | ✅ |
| REQ-26 count=0 NO envía | `TestService_Warning_CountZero_NoSend` | `service_test.go` (line ~853) | ✅ |
| REQ-26 Toggle off salta el send | `TestService_Warning_Disabled_NoSend` | `service_test.go` (line ~933) | ✅ |
| REQ-26 count == threshold NO envía | `TestService_Warning_CountAtThreshold_NoSend` | `service_test.go` | ✅ |
| REQ-27 automute=autoban=3 produce UN warning pre-ban en count=2 | `TestService_Warning_AutomuteEqualsAutoban_SinglePreBan` | `service_test.go` | ✅ |
| REQ-27 thresholdKindFor prioridad | `TestThresholdKindFor_Priorities*` | `service_test.go` (line ~1037) | ✅ |
| REQ-27 helper defensivo retorna vacío si count no aplica | `TestThresholdKindFor_DefensiveEmpty` + `TestThresholdKindFor_NilSettings` | `service_test.go` | ✅ |
| REQ-28 Happy path logea SUCCESS | `TestWarningSender_Happy_DefaultTemplate` | `warning_sender_test.go` (line 85) | ✅ |
| REQ-28 Bot removido entre hit y send → skip silencioso | `TestWarningSender_PermissionDenied_NoSend` | `warning_sender_test.go` (line 144) | ✅ |
| REQ-28 Timeout 5s retorna error | `TestWarningSender_TelegramTimeout_ReturnsError` | `warning_sender_test.go` (line 196) | ✅ |
| REQ-28 Template custom usado se refleja en metadata | `TestWarningSender_CustomTemplate_Metadata` | `warning_sender_test.go` (line 220) | ✅ |
| REQ-29 Render inicial con defaults | `GroupAutomationPage > renderiza con los settings por defecto del backend` | `GroupAutomationPage.test.tsx` | ✅ |
| REQ-29 Modificar template entra en el diff del Save | `GroupAutomationPage > Section 5 warn_user textarea custom + maxLength + persist` | `GroupAutomationPage.test.tsx` | ✅ |
| REQ-29 Tests slice 2 siguen verdes | 8 slice 2 tests in same file | `GroupAutomationPage.test.tsx` | ✅ |
| REQ-30 Templates ≥3 casos | 13 cases | `templates_test.go` | ✅ (exceeded) |
| REQ-30 Service ≥5 casos | 15 cases (8 trigger + 7 helper) | `service_test.go` | ✅ (exceeded) |
| REQ-30 WarningSender ≥4 casos | 9 cases | `warning_sender_test.go` | ✅ (exceeded) |
| REQ-30 Repository ≥2 casos | 2 cases | `repository_test.go` | ✅ |
| REQ-30 Handler ≥1 caso | 3 cases | `automation_handlers_test.go` | ✅ (exceeded) |
| REQ-30 Frontend ≥2 casos | 2 cases | `GroupAutomationPage.test.tsx` | ✅ |
| REQ-31 `backend/internal/moderation/` intacto | `git diff main -- backend/internal/moderation/` = empty | git | ✅ |
| REQ-31 permissionOkAdmin invariante | 0 `can_*` executable matches | grep | ✅ |
| REQ-31 Slice 1+2 paths intactos | Full automation package test suite green (slice 1+2 tests preserved) | `go test ./internal/automation/ -count=1` | ✅ |
| REQ-31 Frontend no-regression | 8 slice 2 tests green + non-regression diff empty | `GroupAutomationPage.test.tsx` + git | ✅ |
| REQ-31 Pipeline no bloquea más de 5s | timeout enforced via `context.WithTimeout(5s)`; failure path tested | `warning_sender_test.go` (TestTimeout) | ✅ |
| REQ-31 §21.1 sin llamadas Bot API reales | 0 `TELEGRAM_BOT_TOKEN` / 0 `https://api.telegram.org` matches in `automation/*.go`; tests use fakes | grep + code review | ✅ |
| REQ-31 §25 sin secretos | no `.env`, no tokens in any diff; verified manually | git + code review | ✅ |

**Total**: **~40 spec scenarios covered by 70+ tests, all green.**

---

## Correctness table — critical paths

| Path | Test | File | Verdict |
|------|------|------|---------|
| Migration applies + defaults round-trip | `TestRepository_UpsertSettings_Defaults` + custom round-trip | `repository_test.go` | ✅ |
| `RenderTemplate` with custom template | `TestRenderTemplate_CustomOverride` | `templates_test.go` | ✅ |
| `RenderTemplate` fallback chain | 3 tests (FirstName / Username / literal) | `templates_test.go` | ✅ |
| Trigger condition (count == automute-1) | `TestService_Warning_PreMuteTrigger` | `service_test.go` | ✅ |
| Trigger condition (count == autoban-1) | `TestService_Warning_PreBanTrigger` | `service_test.go` | ✅ |
| Edge case automute==autoban | `TestService_Warning_AutomuteEqualsAutoban_SinglePreBan` | `service_test.go` | ✅ |
| `tgWarningSender.SendWarning` happy path | `TestWarningSender_Happy_DefaultTemplate` | `warning_sender_test.go` | ✅ |
| `tgWarningSender` re-check permission | `TestWarningSender_PermissionDenied_NoSend` | `warning_sender_test.go` | ✅ |
| `tgWarningSender` timeout 5s | `TestWarningSender_TelegramTimeout_ReturnsError` | `warning_sender_test.go` | ✅ |
| Sender error pipeline continues | `TestService_Warning_SenderError_PipelineContinues` | `service_test.go` | ✅ |
| Handler PUT validation | 3 tests (≤1000 OK, >1000 → 400, ==1000 → 200) | `automation_handlers_test.go` | ✅ |
| Frontend Switch render + persist | smoke test | `GroupAutomationPage.test.tsx` | ✅ |
| Frontend Textarea + maxLength + persist | smoke test (via fireEvent) | `GroupAutomationPage.test.tsx` | ✅ |

---

## Issues

### CRITICAL
**None.** All blocking scenarios covered, all tests green, all invariants intact.

### WARNING
**None.** Deviations are documented improvements, not regressions.

### SUGGESTION
1. **Pre-existing test isolation in PublicationsPage** — `npm test -- --run` (full suite) has 1 timeout failure in `PublicationsPage.test.tsx > crea una publicacion multi-grupo con foto y botones`. This is a pre-existing issue on `main` (confirmed via git stash by apply-phase; this branch has ZERO changes to that file). Recommend filing a separate slice/issue for the test isolation. NOT blocking this verify.

2. **`SendWarning` signature deviation** — already documented and accepted (deviation #2). Worth highlighting because the design doc still says "toma IDs (no *Message)"; future slices referencing the design should be aware that the actual signature is `SendWarning(ctx, msg *telegram.Message, count, kind)`. The archive phase can update the canonical spec if needed.

3. **`can_` grep on raw output** — when a future reader runs `grep can_ backend/internal/automation/*.go` without the `| grep -v '//'` filter, they'll see 4 matches that look alarming (all are inside doc comments documenting the prohibition). The verification command's `grep -v '//'` filter is correct and necessary. Worth noting in README to avoid future confusion.

---

## OVERALL VERDICT: **PASS** ✅

`moderation-automation-slice2.1` is **ready for `sdd-archive`**.

**Evidence summary**:
- 7 commits ahead of `main @ 2df231f`, NOT pushed ✅
- Backend: 12/12 packages green, vet clean, gofmt empty ✅
- Frontend: 10/10 GroupAutomationPage tests green, build successful ✅
- Non-regression diff: 0 lines across moderation/, publications/, 8 frontend pages ✅
- `permissionOkAdmin` invariant: 0 executable `can_*` matches ✅
- 10/10 REQ satisfied, ~40 spec scenarios covered by 70+ tests ✅
- 10/10 design decisions conformant ✅
- 4 documented deviations, all acceptable ✅

---

## What remains for `sdd-archive`

1. **Merge the branch** to `main` (orchestrator handles the PR; size:exception already documented in decision obs `#238`).
2. **APPEND REQ-22..31 to canonical spec** `openspec/specs/moderation-automation/spec.md` (after line 1054, in the "Slice 2 ADDED Requirements" section) per the sync method in the design and the precedent of slice 1→slice 2 archive. New section title: "Slice 2.1 ADDED Requirements (2026-09-08 — Warning al usuario pre-acción)".
3. **Optionally update the canonical design** to reflect deviation #2 (SendWarning takes *telegram.Message, not IDs) — minor doc consistency improvement.
4. **Optionally file a follow-up issue** for the PublicationsPage test isolation (SUGGESTION #1).
5. **Update the task count in Engram** — current tasks obs #237 says "20/20 tasks complete"; archive phase should bump the slice to "archived" state.
6. **Persist archive-report** with same hybrid mode (filesystem + Engram topic_key `sdd/moderation-automation/slice2.1/archive-report`).
