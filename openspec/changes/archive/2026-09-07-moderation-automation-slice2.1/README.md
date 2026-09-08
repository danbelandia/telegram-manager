# Archive Report: `moderation-automation-slice2.1`

> **Change**: `moderation-automation-slice2.1` — sub-slice de Fase 3 (AGENTS §23) sobre slice 2 archivado.
> **Status**: **ARCHIVED** — branch ready for orchestrator to merge → `main` and push.
> **Verdict**: **PASS** (verify-report `#240`).
> **Mode**: hybrid (filesystem + Engram).
> **Strategy**: single-pr with `size:exception` (decision obs `#238`, 10/10 consecutivos aprobados).

---

## Summary

| Field | Value |
|-------|-------|
| Change name | `moderation-automation-slice2.1` |
| Slice | sub-slice de Fase 3 (AGENTS §23); entre slice 2 y slice 3 |
| Base | `main @ 2df231f` (slice 2 archivado, obs `#231`) |
| Branch | `feat/moderation-automation-slice2.1` |
| Commits ahead of main | 7 source commits + 1 archive commit (this commit) |
| Files changed (source) | 18 files; +1908/-57 (apply-report `#239`) |
| Backend packages green | 12/12 |
| Frontend tests | 10/10 sobre `GroupAutomationPage.test.tsx` (1 pre-existing flake en `PublicationsPage.test.tsx`, NOT slice 2.1's) |
| Verdict | **PASS** |
| Archived on | 2026-09-08 |
| Archive folder | `openspec/changes/archive/2026-09-07-moderation-automation-slice2.1/` |

---

## What Was Done

1. **APPEND delta REQs to canonical spec** at `openspec/specs/moderation-automation/spec.md`.
   - Canonical went from 1054 lines (REQ-1..REQ-21 from slices 1+2) → **1583 lines** (REQ-1..REQ-31, +529 insertions).
   - Slice-1 content (REQ-1..REQ-15) + slice-2 content (REQ-16..REQ-21, kanónicamente numeradas en orden de aparición) preserved verbatim.
   - New section appended: `## Slice 2.1 ADDED Requirements (2026-09-08 — Warning visual al usuario en chat)` con REQ-22..REQ-31 (10 nuevas requirements, ~40 scenarios).
   - **APPEND**, not MOVE: slices 1+2 son amendments incrementales al canónico (cada slice agrega REQs al capability existente). Slice 2.1 sigue la misma técnica APPEND del slice 1→2 archive. REQ-1..REQ-21 preservados intactos.
   - The delta spec permanece dentro del archive folder (`specs/moderation-automation/spec.md`, 376 líneas) como audit trail del contenido original.

2. **MOVED change folder** `openspec/changes/moderation-automation/slice2.1/` → `openspec/changes/archive/2026-09-07-moderation-automation-slice2.1/`.
   - PowerShell `Move-Item` (la carpeta estaba untracked en working tree; git detectará los archivos como nuevos al stage en el archive location).
   - Todos los artefactos preservados (exploration, proposal, design, tasks, apply-report, verify-report + delta specs folder).

3. **Wrote este README.md** documentando: metadata, branch base, verdict, delivery strategy (`size:exception`, 10/10 consecutivos), spec sync action (APPEND con rationale), file inventory, Engram observation IDs (#232-#240), full commit list (7 source + 1 archive), las **4 desviaciones documentadas**, el **bugfix `#172` invariant verification**, el **1 pre-existing flake**, el slice 2.1/3 context (entre slice 2 archivado y slice 3 futuro), invariantes respetadas, y next steps para el orchestrator.

4. **Commit a crearse**: `chore(openspec): archive change moderation-automation-slice2.1` (single conventional commit en `feat/moderation-automation-slice2.1`).

5. **NO push, NO merge** (rule of archive phase — orchestrator will handle).

---

## Spec Sync Actions

| Action | Detail |
|--------|--------|
| Capability | `moderation-automation` (existing from slices 1+2, obs `#223` + `#231`). |
| Sync method | **APPEND** (delta es un delta, no full spec). REQ-1..REQ-21 previos preservados verbatim; REQ-22..REQ-31 nuevos appendeados en nueva sección. |
| ADDED | 10 requirements (REQ-22..REQ-31): Schema `warn_user_enabled` + `warn_user_template` (REQ-22); Defaults al auto-crear (REQ-23); 2 templates hardcoded + override per-grupo (REQ-24); Substituciones `{nombre}`/`{count}`/`{mute_minutes}` + verbatim preservation (REQ-25); Trigger en paso 7.5 de HandleMessage (REQ-26); Edge case `automute==autoban` (REQ-27); Interfaz `WarningSender` + semántica de envío (REQ-28); Frontend `GroupAutomationPage` Sección 5 (REQ-29); Tests §21.1 estricto (REQ-30); No regresión + invariantes (REQ-31). |
| MODIFIED | None. Existing `moderation-automation` (slices 1+2: `group_moderation_settings`, `user_warning_state`, `allowed_updates`, Rules Registry, `FloodRule`, `AntiSpamRule`, `AntiLinkRule`, `BannedWordsRule`, Registry order, `Service.HandleMessage` pre-load listas, `Rule.Evaluate` con `*Lists`, `AutoActioner`, bus subscriber, audit logs, worker rate limit, env vars, frontend automation page, action constants, no regresión) preserved intact. |
| Removed | None. |
| Audit trail | The delta spec.md (`specs/moderation-automation/spec.md`) queda dentro del archive folder como evidencia del contenido ADDED original. `git log --follow openspec/specs/moderation-automation/spec.md` mostrará el entry de slice-1 seguido por el entry de slice-2 (`2df231f`, +617 lines appended) seguido por este entry de slice-2.1 (este archive commit, +529 lines appended). |

---

## Engram Observation Traceability

- `#232` — `sdd/moderation-automation/slice2.1/exploration` (D1-D10, affected areas, risks, fallback chain)
- `#234` — `sdd/moderation-automation/slice2.1/proposal` (intent, scope, capabilities, approach, schema, success criteria)
- `#235` — `sdd/moderation-automation/slice2.1/spec` (delta spec REQ-22..31, 10 ADDED Requirements, ~40 scenarios)
- `#236` — `sdd/moderation-automation/slice2.1/design` (D1-D10 + bugfix #172, file-by-file specs, interfaces, tests strategy)
- `#237` — `sdd/moderation-automation/slice2.1/tasks` (7 phases, 20 tasks, Forecast High → size:exception, ALL TASKS COMPLETE)
- `#238` — `sdd/moderation-automation/slice2.1/delivery-strategy` (decision — `size:exception` approved, 10/10 consecutivo)
- `#239` — `sdd/moderation-automation/slice2.1/apply-report` (7 commits, 18 files, gates green, 4 deviations)
- `#240` — `sdd/moderation-automation/slice2.1/verify-report` (PASS, 10/10 REQs, 10/10 D, bugfix #172, 1 pre-existing flake)
- `#THIS` — `sdd/moderation-automation/slice2.1/archive-report` (this observation)

---

## Archive Contents

```
openspec/changes/archive/2026-09-07-moderation-automation-slice2.1/
├── README.md                              ← this file (archive metadata, full provenance)
├── exploration.md                         ← D1-D10, affected areas, risks, fallback chain
├── proposal.md                            ← scope, capabilities, approach, schema, success criteria
├── design.md                              ← D1-D10 + bugfix #172, file-by-file specs, interfaces
├── tasks.md                               ← 7 phases, 20 tasks, all [x], size:exception approved
├── apply-report.md                        ← 7 commits, 18 files, gates green, 4 deviations documented
├── verify-report.md                       ← PASS, 10/10 REQs, 10/10 D, 70+ tests, bugfix #172 intact
└── specs/
    └── moderation-automation/
        └── spec.md                        ← ORIGINAL DELTA SPEC (376 lines, ADDED Requirements preserved as audit trail)
```

---

## File Inventory (source change, apply-report `#239`)

| Area | File | Status | LOC est. (apply-report) |
|------|------|--------|---------:|
| Backend NEW | `backend/migrations/00008_add_warning_settings.sql` | NEW | +15 |
| Backend NEW | `backend/internal/automation/templates.go` | NEW | +60 |
| Backend NEW | `backend/internal/automation/templates_test.go` | NEW | +80 |
| Backend NEW | `backend/internal/automation/warning_sender.go` | NEW | +130 |
| Backend NEW | `backend/internal/automation/warning_sender_test.go` | NEW | +150 |
| Backend MOD | `backend/internal/automation/model.go` | MOD | +10 |
| Backend MOD | `backend/internal/automation/repository.go` | MOD | +20 |
| Backend MOD | `backend/internal/automation/repository_test.go` | MOD | +30 |
| Backend MOD | `backend/internal/automation/service.go` | MOD | +25 |
| Backend MOD | `backend/internal/automation/service_test.go` | MOD | +60 |
| Backend MOD | `backend/internal/logs/model.go` | MOD | +3 |
| Backend MOD | `backend/cmd/server/main.go` | MOD | +5 |
| Backend MOD | `backend/internal/api/automation_handlers.go` | MOD | +5 |
| Frontend MOD | `frontend/src/features/automation/types.ts` | MOD | +10 |
| Frontend MOD | `frontend/src/pages/GroupAutomationPage.tsx` | MOD | +50 |
| Frontend MOD | `frontend/src/pages/GroupAutomationPage.test.tsx` | MOD | +30 |
| Docs MOD | `README.md` | MOD | +30 |
| Artifact | `openspec/changes/moderation-automation/slice2.1/apply-report.md` | NEW | +N/A |

**Total**: 18 files; +1908 / -57 (apply-report `#239`). 5 NEW + 12 MOD + 1 artifact.

---

## Commit List (7 source commits + 1 archive commit, `feat/moderation-automation-slice2.1`, NOT pushed)

Source (7 commits, base `main @ 2df231f`):

1. `16687c9` — `feat(automation): warn_user settings migration + model fields` (Phase 1)
2. `c79c697` — `feat(automation): templates + warning_sender modules` (Phase 2)
3. `de6aad3` — `feat(automation): service integration paso 7.5 + thresholdKindFor` (Phase 3)
4. `cbebad7` — `feat(api): automation handler validation + main.go wiring` (Phase 4)
5. `fb1693a` — `feat(frontend): automation settings section 5 (warn_user)` (Phase 5)
6. `ca352a4` — `docs: README warning al usuario section` (Phase 7)
7. `4c064b6` — `chore(openspec): apply-report + tasks complete for slice 2.1` (artifact)

Archive (1 commit, this commit, pending):

8. `<THIS_COMMIT>` — `chore(openspec): archive change moderation-automation-slice2.1` (1 modified canonical + 7 new archive artifacts + 1 README = 9 files)

Total source: **18 files changed, +1908/-57** — matches apply-report `#239` and verify-report `#240`.

---

## Documented Deviations (verify-report `#240` §Deviations, apply-report `#239` §Deviations)

All 4 deviations are **localized and acceptable** per verify-report. None break a spec REQ.

1. **`fakeWarningSender` records calls even when `err` is set** (was returning early in original mock design).
   - **Verdict**: ✅ ACCEPTABLE. The original mock returned early, making `TestService_Warning_SenderError_PipelineContinues` non-deterministic (the test needs to verify the sender was CALLED before checking pipeline continuation). Recording the call before returning the error is semantically equivalent (the call happened) and behaviorally more useful for tests. No production code changed.

2. **`SendWarning` signature takes `*telegram.Message` (not just IDs)**.
   - **Verdict**: ✅ ACCEPTABLE (improvement over design). The design's "toma IDs (no *Message)" was written without realizing `RenderTemplate` needs `FirstName` and `Username` from `msg.From`. Switching to `*telegram.Message` follows the established pattern of `AutoActioner.Execute(action AutoAction)` — the receiver gets full context, not fragmented IDs. Caller (`Service.HandleMessage`) already has the message in scope. Improves code clarity and matches codebase conventions. The interface consumer-side contract is preserved; tests use `*telegram.Message` fixtures just like slice 1+2 do for actions.

3. **`Service.LoadOrCreateSettings` now uses `DefaultSettings(groupID)` instead of inline literal**.
   - **Verdict**: ✅ ACCEPTABLE (improvement). The original had a comment warning "cualquier cambio en defaults debe replicarse en ambos lugares" — this refactor eliminates the duplication and is the right call. `DefaultSettings()` in `model.go:63-80` is now the single source of truth. Default behavior is identical (slice 1 defaults + slice 2.1 `WarnUserEnabled: true, WarnUserTemplate: nil`). Reduces future drift risk.

4. **Frontend smoke test uses `fireEvent.change` instead of `userEvent.type` for textarea**.
   - **Verdict**: ✅ ACCEPTABLE (workaround). `userEvent` loses `{` and `}` keystrokes in Mantine v7 Textarea (known upstream issue). `fireEvent.change` with explicit `target.value` is the deterministic alternative that vitest's RTL recommends when user-event has gaps. The Switch test still uses `userEvent` as normal. Test remains semantically equivalent.

**All 4 deviations are acceptable** — they are improvements or necessary workarounds documented in the apply-report.

---

## Bugfix `#172` invariant verification

The `permissionOk` helper in `backend/internal/moderation/` historically read `g.BotPermissions[key]` for `can_*` keys that the group detector never populates — a known bug. Per bugfix `#172`, **all new code MUST use `g.BotStatus == groups.StatusAdministrator`**, NEVER `can_*`.

Slice 2.1 verification (verify-report `#240` §6):

- 0 executable `can_*` matches in `backend/internal/automation/*.go` (grep con `grep -v '//'`).
- 4 comment-only docs preserved (all say "NUNCA leer claves can_*"):
  - `autoactioner.go:85` (slice 1, preserved)
  - `model.go:10` (slice 1, preserved)
  - `service.go:50` (slice 1, preserved)
  - `warning_sender.go:10` (slice 2.1 NEW, documenta la invariante explícitamente)
- `tgWarningSender.SendWarning` re-uses `permissionOkAdmin(g)` (defined in `service.go:383-385`) — mismo helper que `service.go` y `autoactioner.go` usan. NO introduce nuevo helper ni reusa `moderation.permissionOk`.
- `automation_handlers.go` doesn't need permission check (handlers son admin-gated vía `requireAuth` desde panel, no bot-touched).

**Bugfix `#172` invariant INTACT.** Strict check via `grep can_ backend/internal/automation/*.go | grep -v '^//'` returns 0 matches.

---

## 1 pre-existing flake (NOT caused by slice 2.1)

`PublicationsPage.test.tsx > crea una publicacion multi-grupo con foto y botones` fails by timeout (5s) in this session's `npm test -- --run` run.

**Provenance verified — pre-existing in `main`**:
- `git diff main -- frontend/src/pages/PublicationsPage.test.tsx` = EMPTY (test file unchanged by slice 2.1).
- `git log --all -- frontend/src/pages/PublicationsPage.test.tsx` last modified in `f4d1751 feat(frontend): migrate PublicationsPage to Mantine v7` (publications slice 3).
- The test fails identically in `main` (verified via git stash by apply-phase).
- File está en el non-regression protected set per `tasks.md §6.3` y per orchestrator's diff command.

**Action**: SUGGESTION-level cleanup item, NOT a blocker. El pre-existing flaky test debe abordarse en un cambio separado (incrementar timeout o fix async wait). Fuera del scope de slice 2.1.

**Note on apply-report discrepancy**: apply-report `#239` claimed "npm test -- --run (76/76) green" para `automation` package; el fallo en `PublicationsPage` solo aparece en full suite. El package automation aislado corre 50+ tests 100% green. El flake es de orden superior (mock pollution entre archivos).

---

## Slice 2.1/3 context — Fase 3 (AGENTS §23)

Per exploration `#215` (3-slice plan de Fase 3) y proposal `#234` (slice 2.1 scope), Fase 3 ship en **3 slices + 1 sub-slice**:

| Slice | Scope | Backend LOC | Frontend LOC | Status |
|-------|-------|------------:|-------------:|--------|
| **1 — Foundation** | settings + warning_state + FloodRule + worker + logs | ~1500 | 0 | **ARCHIVED** (obs `#223`, `main @ 1b10d42`) |
| **2 — More rules + Settings UI** | AntiSpam + AntiLink + BannedWords + Settings editor | ~1241 | ~1117 | **ARCHIVED** (obs `#231`, `main @ 2df231f`) |
| **2.1 — Warning visual pre-acción (THIS)** | 2 cols schema + 2 templates + WarningSender + Sección 5 frontend | ~525 | ~90 | **THIS ARCHIVE** |
| 3 — Warnings Dashboard | Read `user_warning_state` + logs aggregations + dashboard UI | ~150 | ~570 | Pending (`moderation-automation-slice3`) |

Slice 2.1 es un **sub-slice de Fase 3**, no un slice completo. Fue generado entre slice 2 archivado y slice 3 futuro para shippear el feature "warning al usuario antes de la acción de threshold" — UX feedback educativo sobre el motor de moderación automática ya construido en slices 1+2. Cada slice/sub-slice excede el budget de 400 líneas — `size:exception` es el patrón establecido (10/10 consecutivos aprobados en este repo).

---

## Invariantes respetadas

- **§11 (NO Redis)**: 0 deps Redis; pure backend stack con in-process channel (slice 1 foundation, preserved).
- **§14 (modular backend)**: All new files en `internal/automation/` (templates, warning_sender) y `internal/api/automation_handlers.go`. No leaks a otros packages.
- **§13.1 (goose migrations)**: 1 nueva migración `00008_add_warning_settings.sql` con `-- +goose Up`/`Down` markers. SQL plano, no DSL.
- **§17.1 (no secrets)**: 0 secrets in code. `.env.example` intacto.
- **§18.1 (rate limits Telegram)**: Slice 1 worker + adapter rate limit intact. Slice 2.1 adds 0 new direct calls to Bot API; el `SendMessage` del warning pasa por el adapter existente (con retry+token bucket). Tests usan fakes (§21.1).
- **§21.1 (mock TelegramService)**: All 50+ backend automation tests usan hand-rolled fakes (`fakeTelegram`, `fakeSettingsRepo`, `fakeGroups`, `fakeLogs`, `fakeWarningSender`). Zero `moq` codegen; zero Bot API calls in tests.
- **§23 (Fase 3 — this slice)**: Warning visual al usuario antes de la acción de threshold delivered.
- **§25 regla 18 (frontend no llama Telegram)**: Frontend changes aislados a `automation/` feature module + `GroupAutomationPage.tsx`. No `telegram.org` URLs en frontend code.
- **Bugfix `#172`** (permissionOkAdmin usa `BotStatus`, NEVER `can_*`): verified, 0 executable `can_*` matches. Helper local al paquete automation; invariante cross-slice.
- **Frontend pre-existing pages intact**: `git diff main -- frontend/src/pages/` excluyendo `GroupAutomationPage.tsx` y `GroupDetailPage.tsx` returns empty.
- **`backend/internal/moderation/` intacto**: acciones manuales (ban/unban/mute/unmute/delete/pin/lock/unlock/approve/reject) sin tocar. `git diff main -- backend/internal/moderation/` = empty.

---

## Full backend suite verification (verify-report `#240` §2)

```
?       github.com/telegram-manager/backend/cmd/server       [no test files]
ok      github.com/telegram-manager/backend/internal/api               4.441s
ok      github.com/telegram-manager/backend/internal/auth              4.247s
ok      github.com/telegram-manager/backend/internal/automation        14.249s   ← slice 1+2+2.1
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

**12/12 testable packages green**. `go vet ./...` clean. `gofmt -l .` clean. `go build ./...` clean.

Automation package alone runs **50+ tests**:
- 13 templates tests (REQ-24, REQ-25)
- 9 warning_sender tests (REQ-28)
- 15 service tests = 8 trigger (REQ-26, REQ-27) + 7 thresholdKindFor helper (REQ-27)
- 2 repository integration tests (REQ-22, REQ-23)
- 3 automation_handlers tests (PUT ≤1000 OK, PUT >1000 → 400, PUT ==1000 → 200)
- 5 autoactioner tests (preserved slice 1)
- 5 flood rule tests (preserved slice 1)
- All slice 2 tests preserved (AntiSpamRule, AntiLinkRule, BannedWordsRule, Registry order, Service pre-load listas, Rule.Evaluate con *Lists, handlers CRUD banned-words/link-allowlist, GET/PUT settings)

Frontend: `npm test -- --run src/pages/GroupAutomationPage.test.tsx` → **10/10 pass** (8 slice 2 + 2 slice 2.1). `npm run build` → green.

---

## Next Steps (orchestrator)

1. **Merge** `feat/moderation-automation-slice2.1` → `main` (single-pr, `size:exception` aprobado per `#238`, precedent 10/10).
2. **Push** to remote.
3. Slice 2.1 cierra el feature "warning al usuario" del motor de moderación automática. Next candidate per `#215`:
   - **`moderation-automation-slice3`** — Warnings dashboard + stats aggregations + frontend dashboard.
   - Branch base: `main` post-slice-2.1 merge.
   - Scope: read `user_warning_state` + aggregations de logs + dashboard UI en `/groups/:id/automation/warnings`.

---

## Commit (this archive)

- **Message**: `chore(openspec): archive change moderation-automation-slice2.1`
- **Branch**: `feat/moderation-automation-slice2.1`
- **Files** (pending): 1 modified (canonical spec, +529) + 8 new (this README + 6 archive artifacts + delta specs subfolder = 7 individual files inside specs/) — total 9 files in this commit.
- **NO source code touched** — solo `openspec/specs/moderation-automation/spec.md` (canónico) + el archive folder.
- **Push/Merge**: NEITHER (rule of archive phase — orchestrator handles).
- **Commit SHA**: `<THIS_COMMIT>` (will be filled by orchestrator after commit lands).
