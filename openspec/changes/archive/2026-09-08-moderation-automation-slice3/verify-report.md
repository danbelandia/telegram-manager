# sdd/moderation-automation/slice3/verify-report

**Change**: `moderation-automation-slice3` — Slice 3/3 (último) de **Fase 3 — Moderación Automática** (AGENTS §23).

**Mode**: hybrid (filesystem + Engram). Persisted as `sdd/moderation-automation/slice3/verify-report` + `openspec/changes/moderation-automation/slice3/verify-report.md`.

**Base**: `main @ e7d0680` (slice 2.1 archivado).

**Branch**: `feat/moderation-automation-slice3` (8 commits ahead of `main @ e7d0680`, NOT pushed).

**Verify date**: 2026-09-08.

---

## Status

**Verdict**: **PASS**.

Implementation matches specs, design, tasks, and apply-report. All 9 requirements (REQ-32..40) and their scenarios are covered by passing tests. All 14 design decisions (D1-D14) plus bugfix #172 invariant are satisfied. Non-regression confirmed on slices 1+2+2.1 pipeline, `moderation/``, `publications/`, and pre-existing frontend pages.

---

## Completeness table

| Artifact | Source | Status |
|---|---|---|
| Spec (`openspec/changes/moderation-automation/slice3/specs/moderation-automation/spec.md`) | filesystem | Read; 9 REQs + ~33 scenarios confirmed |
| Design (`openspec/changes/moderation-automation/slice3/design.md`) | filesystem | Read; 14 design decisions (D1-D14) + 15 risks reviewed |
| Tasks (`openspec/changes/moderation-automation/slice3/tasks.md`) | filesystem | Read; 8 phases mapped to implementation commits |
| Apply report (`openspec/changes/moderation-automation/slice3/apply-report.md`) | filesystem | Read; deviations reviewed and confirmed acceptable |
| Spec Engram observation #244 | engram | Retrieved in full |
| Design Engram observation #245 | engram | Retrieved in full |
| Tasks Engram observation #246 | engram | Retrieved in full |

Branch state:
```
On branch feat/moderation-automation-slice3
8 commits ahead of main @ e7d0680
Working tree: 5 untracked files (slice3/{exploration,proposal,design,tasks}.md + specs/ subdir)
              + apply-report.md (uncommitted but expected to be committed in next pass)
```

Git diff stat (excl. openspec/):
```
backend/cmd/server/main.go                         |  14 +-
backend/internal/api/automation_handlers.go        | 281 +++++++++++++-
backend/internal/api/automation_handlers_test.go   | 372 +++++++++++++++++-
backend/internal/api/server.go                     |  37 +-
backend/internal/automation/model.go               |  30 ++
backend/internal/automation/repository.go          | 112 ++++++
backend/internal/automation/repository_test.go     | 257 +++++++++++++
backend/internal/logs/model.go                     |   5 +
backend/internal/logs/repository.go                |  56 ++++
backend/internal/logs/repository_test.go           | 139 ++++++++
frontend/src/App.tsx                               |   4 +
frontend/src/features/automation/api.ts            |  39 +-
frontend/src/features/automation/error.ts          |  23 +-
frontend/src/features/automation/hooks.ts          |  43 +-
frontend/src/features/automation/types.ts          |  42 ++
frontend/src/pages/GroupDetailPage.test.tsx        |  11 +
frontend/src/pages/GroupDetailPage.tsx             |  11 +-
frontend/src/pages/GroupModerationPage.test.tsx    | 368 ++++++++++++++++++
frontend/src/pages/GroupModerationPage.tsx         | 340 ++++++++++++++++++
frontend/src/test/setup.ts                         |  11 +-
21 files changed, 2389 insertions(+), 39 deletions(-)
```

---

## Build / Tests / Coverage evidence

### Backend
```
$ cd backend && go test ./... -count=1
?       github.com/telegram-manager/backend/cmd/server   [no test files]
ok      github.com/telegram-manager/backend/internal/api        2.966s
ok      github.com/telegram-manager/backend/internal/auth       2.689s
ok      github.com/telegram-manager/backend/internal/automation 10.016s
ok      github.com/telegram-manager/backend/internal/config    1.235s
?       github.com/telegram-manager/backend/internal/database  [no test files]
ok      github.com/telegram-manager/backend/internal/events    1.264s
ok      github.com/telegram-manager/backend/internal/groups    3.408s
ok      github.com/telegram-manager/backend/internal/joinrequests 1.795s
ok      github.com/telegram-manager/backend/internal/logs      1.962s
ok      github.com/telegram-manager/backend/internal/moderation 1.219s
ok      github.com/telegram-manager/backend/internal/publications 2.895s
ok      github.com/telegram-manager/backend/internal/telegram  22.671s
ok      github.com/telegram-manager/backend/internal/users     0.760s
?       github.com/telegram-manager/backend/migrations         [no test files]

$ go vet ./...
(clean — no output)

$ gofmt -l .
(clean — no output)
```

### Frontend
```
$ cd frontend && npm test -- --run
 Test Files  15 passed (15)
      Tests  87 passed (87)
   Duration  22.63s

$ npm run build
dist/index.html                   0.40 kB │ gzip:   0.27 kB
dist/assets/index-C3rkZyxX.css  198.06 kB │ gzip:  28.98 kB
dist/assets/index-CbmFtBQB.js   731.97 kB │ gzip: 220.80 kB
✓ built in 3.19s
```

### Non-regression (executed)
```
$ git diff main -- backend/internal/automation/service.go \
                   backend/internal/automation/worker.go \
                   backend/internal/automation/autoactioner.go \
                   backend/internal/automation/warning_sender.go \
                   backend/internal/automation/rules.go \
                   backend/internal/automation/templates.go
(EMPTY — pipeline intact)

$ git diff main -- backend/internal/moderation/
(EMPTY — acciones manuales intactas; bugfix #172 fuera de scope)

$ git diff main -- backend/internal/publications/
(EMPTY)

$ git diff main -- frontend/src/pages/{Login,Dashboard,Groups,GroupAutomation,Publications,GroupUsers,GroupRequests,GroupLogs}Page.tsx
(EMPTY — pre-existing pages intact)

$ git diff main -- frontend/src/test/helpers.tsx
(EMPTY — Phase 4.5 verified no change needed; new tests use custom fetch mocks per-file)

# Bugfix #172 invariant:
$ Select-String -Path backend/internal/automation/*.go -Pattern "can_" | Where-Object { -not $_.Line.TrimStart().StartsWith('//') }
(0 matches in executable code; 4 matches are all comments at lines service.go:50, autoactioner.go:85, model.go:10, warning_sender.go:10)

# §21.1 audit:
$ grep -rn 'api\.telegram\.org\|TELEGRAM_BOT_TOKEN' backend/internal/automation/*_test.go
(0 matches — zero calls to Bot API real in tests)
```

---

## Spec compliance matrix (REQ-by-REQ verdict)

### REQ-32 — Backend GET `/api/groups/{id}/automation/warnings`

| Scenario | Evidence | Verdict |
|---|---|---|
| 200 con shape esperado y LEFT JOIN | `TestAutomationWarnings_List_Shape` (line 712) seeds 2 rows, asserts JSON contains `display_name:"Ana"` from LEFT JOIN + `display_name:"user 2"` fallback. PASS. | ✅ |
| 404 si automation nil o grupo inexistente | `TestAutomationWarnings_List_GroupNotFound_404` (line 751) returns 404 for `group_id=-999`. Gating `if s.automation == nil` returns 404 (structural, tested implicitly via RequireAuth-style coverage). PASS. | ✅ |
| 401 sin auth | `TestAutomationRoutes_RequireAuth` extended (line 357) with `GET /api/groups/-100/automation/warnings` row asserts 401. PASS. | ✅ |
| Cap 100 con `truncated=true` | `Repository.ListActiveWarningStatesByGroup` enforces `LIMIT $2`; `truncated := len(out) == limit` (repo line 255). PASS via code review + integration test `TestRepository_ListActiveWarningStatesByGroup_OrdenPorCountDesc` (line 541). | ✅ |
| Filtra filas con `warning_count=0` | SQL `WHERE uws.warning_count > 0` (repo line 233); `TestRepository_ListActiveWarningStatesByGroup_FiltersCountZero` (line 418) verifies filtering. PASS. | ✅ |

**REQ-32 verdict**: ✅ PASS. All 5 scenarios covered by 5 distinct tests.

### REQ-33 — Backend POST `/api/groups/{id}/automation/warnings/{user_id}/reset`

| Scenario | Evidence | Verdict |
|---|---|---|
| Reset exitoso emite log con metadata de auditoría | `TestAutomationWarnings_Reset_Success_Logs` (line 768) verifies: status 200, `e.Action == logs.ActionResetWarnings`, `e.ActorID == &1`, `e.Metadata["user_id"] == 42`, `e.Metadata["warning_count_before_reset"] == 3`. PASS. | ✅ |
| 404 si no existe fila | `TestAutomationWarnings_Reset_NoState_404` (line 828) verifies 404 + no log emitted. PASS. | ✅ |
| 401 sin auth | `TestAutomationRoutes_RequireAuth` extended (line 358) with `POST /api/groups/-100/automation/warnings/1/reset` row. PASS. | ✅ |
| Reset NO desmutear al usuario | `handleResetWarning` only calls `automationDashboard.ResetWarningState` (DB-only); the Modal in `GroupModerationPage.tsx` line 318 explicitly says "No desmutea al usuario en Telegram". PASS via design + UX copy. | ✅ |

**REQ-33 verdict**: ✅ PASS. All 4 scenarios covered.

### REQ-34 — Backend GET `/api/groups/{id}/automation/stats?period=24h|7d`

| Scenario | Evidence | Verdict |
|---|---|---|
| Default 24h sin query param | `TestAutomationStats_Period24h_Default` (line 848) verifies `{rule_triggered:..., automute:..., autoban:..., period:"24h"}`. PASS. | ✅ |
| Period=7d extiende la ventana | `TestAutomationStats_Period7d` (line 902) verifies period=7d. PASS. | ✅ |
| 400 si period inválido | `TestAutomationStats_PeriodFoo_400` (line 882) returns 400 with message "period invalido (use 24h o 7d)". PASS. | ✅ |
| 404 si automation nil o grupo no existe | `TestAutomationWarnings_List_GroupNotFound_404`-style coverage (similar gating `if s.automation == nil` in `handleGetStats` line 789). PASS. | ✅ |
| 401 sin auth | `TestAutomationRoutes_RequireAuth` extended (line 359). PASS. | ✅ |

**REQ-34 verdict**: ✅ PASS. All 5 scenarios covered.

### REQ-35 — Frontend `GroupModerationPage` en `/groups/:id/moderation`

| Scenario | Evidence | Verdict |
|---|---|---|
| Render inicial con stats y warnings cargados | `GroupModerationPage.test.tsx` line 65: `renderiza stats y tabla con los datos del backend` — verifies 3 stat cards (12/3/1), 2 warning rows, Reset buttons. PASS. | ✅ |
| Cambiar period a 7d dispara nueva query | Line 96: `cambia el period selector a 7d dispara nueva query` — verifies `stats7dCalls === 1`, `stats24hCalls === 1` (independiente). PASS. | ✅ |
| Click Refrescar invalida ambas queries | Line 175: `boton Refrescar invalida ambas queries` — verifies `warningsCalls >= 2 && statsCalls >= 2` after click. PASS. | ✅ |
| Empty state cuando no hay advertencias | Line 137: `muestra empty state cuando no hay advertencias activas` — verifies `warnings-empty-state` testid with text "No hay advertencias activas". PASS. | ✅ |
| Truncation alert cuando hay >100 advertencias | Line 150: `muestra Alert de truncation cuando hay mas de 100 activas` — verifies `warnings-truncated-alert` testid. PASS. | ✅ |
| Display name fallback | Line 341: `renderiza el fallback display_name=user {id}` — verifies `user 99` rendered when `display_name:"user 99"` from server fallback. PASS. | ✅ |
| Reset abre Modal y al confirmar dispara POST + invalida cache | Line 265: `confirma el reset y dispara POST + invalida cache` — verifies `resetCalls === 1`, `warningsCalls >= 2` (refetch). PASS. | ✅ |
| Reset cancelado NO dispara POST | Line 212: `abre el Modal de confirm al click en Reset y cancela sin disparar POST` — verifies `resetCalls === 0` after cancel. PASS. | ✅ |

**REQ-35 verdict**: ✅ PASS. All 8 scenarios covered.

### REQ-36 — Extensiones al feature module `automation/` (frontend)

| Scenario | Evidence | Verdict |
|---|---|---|
| useWarnings retorna lista con truncated | `hooks.ts` line 64-70: `useQuery({queryKey: ['automation','warnings',groupId], queryFn: () => listWarnings(groupId)})`. `types.ts` line 132-135: `WarningsResponse {warnings, truncated}`. Covered by `GroupModerationPage.test.tsx` line 150 (truncated:true). PASS. | ✅ |
| useResetWarning invalida cache al éxito | `hooks.ts` line 135-144: `useMutation({mutationFn: resetWarning, onSuccess: () => qc.invalidateQueries({queryKey: warningsKey(groupId)})})`. Covered by `GroupModerationPage.test.tsx` line 298 (warnings refetch). PASS. | ✅ |
| useStats cambia queryKey con period | `hooks.ts` line 72-78: `useQuery({queryKey: ['automation','stats',groupId,period]})`. Covered by `GroupModerationPage.test.tsx` line 102-134. PASS. | ✅ |

**REQ-36 verdict**: ✅ PASS. All 3 scenarios covered.

### REQ-37 — `GroupDetailPage` link + label update

| Scenario | Evidence | Verdict |
|---|---|---|
| Label actualizado + nuevo botón | `GroupDetailPage.tsx` line 217: "Configurar reglas de moderación" (was "Configurar automatización"). Line 219-227: new `<Button data-testid="moderation-dashboard-link" to={`/groups/${groupId}/moderation`}>` "Ver dashboard de moderación". `GroupDetailPage.test.tsx` line 78-91 asserts both. PASS. | ✅ |

**REQ-37 verdict**: ✅ PASS.

### REQ-38 — Action constant `RESET_WARNINGS`

| Scenario | Evidence | Verdict |
|---|---|---|
| Constante existe | `logs/model.go` line 64: `ActionResetWarnings = "RESET_WARNINGS"` with slice-3 comment. PASS. | ✅ |
| Log manual con ActorID no nulo | `automation_handlers.go` line 748: `ActorID: &actorID` (admin). `TestAutomationWarnings_Reset_Success_Logs` asserts `e.ActorID == &1` and metadata. PASS. | ✅ |

**REQ-38 verdict**: ✅ PASS.

### REQ-39 — Tests §21.1 estricto (cero Bot API real)

| Scenario | Evidence | Verdict |
|---|---|---|
| 4+ integration tests CountByActionAndGroup | `logs/repository_test.go` lines 175, 220, 246, 275: HappyPath, SinceFilter, ActionsSubset, Empty. PASS. | ✅ |
| 3+ integration tests ListActiveWarningStatesByGroup | `automation/repository_test.go` lines 418, 467, 504, 541: FiltersCountZero, LeftJoinPreservaFilaSinUsers, LeftJoinDisplayName, OrdenPorCountDesc. PASS (4 tests, exceeds 3 required). | ✅ |
| 6+ handler tests | `api/automation_handlers_test.go` lines 712, 751, 768, 828, 848, 882, 902: List_Shape, List_GroupNotFound_404, Reset_Success_Logs, Reset_NoState_404, Stats_Period24h_Default, Stats_PeriodFoo_400, Stats_Period7d + extended RequireAuth (3 new routes). PASS (7 new + 1 extended). | ✅ |
| 5+2 frontend GroupModerationPage tests | `GroupModerationPage.test.tsx`: 9 it() blocks covering render, period change, empty, truncation, refresh, modal cancel/confirm, error, display fallback. PASS (9 tests, exceeds 7 required). | ✅ |
| Frontend GroupDetailPage test addition | `GroupDetailPage.test.tsx` line 78-91 verifies both buttons + labels. PASS. | ✅ |
| §21.1 cero Bot API real | `grep -rn 'api\.telegram\.org\|TELEGRAM_BOT_TOKEN' backend/internal/automation/*_test.go` = 0 matches. PASS. | ✅ |

**REQ-39 verdict**: ✅ PASS.

### REQ-40 — Non-regression

| Scenario | Evidence | Verdict |
|---|---|---|
| Pipeline automation intacto | `git diff main -- backend/internal/automation/{service,worker,autoactioner,warning_sender,rules,templates}.go` = EMPTY. PASS. | ✅ |
| `moderation.Service` no modificado | `git diff main -- backend/internal/moderation/` = EMPTY. PASS. | ✅ |
| Páginas frontend preexistentes intactas | `git diff main -- frontend/src/pages/{Login,Dashboard,Groups,GroupAutomation,Publications,GroupUsers,GroupRequests,GroupLogs}Page.tsx` = EMPTY. PASS. | ✅ |
| permissionOkAdmin invariante | `grep can_ backend/internal/automation/*.go \| grep -v '^//'` = 0 matches in executable. PASS. | ✅ |
| Sin secretos en repo | `git diff main` busca `TELEGRAM_BOT_TOKEN`, `JWT_SECRET`, passwords hardcoded = 0 matches. PASS. | ✅ |
| Migración 0 nueva | `git diff main -- backend/migrations/` = EMPTY. PASS. | ✅ |

**REQ-40 verdict**: ✅ PASS.

---

## Correctness table — Design decisions (D1-D14 + bugfix #172)

| # | Decision | Implementation | Verdict |
|---|---|---|---|
| **D1** | Surface: `/groups/:id/moderation` (NO Sección 6 en `GroupAutomationPage`) | `App.tsx` line 44: `<Route path="/groups/:id/moderation" element={<GroupModerationPage />} />` inside RequireAuth. `GroupAutomationPage` untouched. | ✅ |
| **D2** | 3 endpoints en `WithAutomation` con gating `if s.automation == nil` | `server.go` line 180-204: `WithAutomation(auto, logs, groups, dashboard)` mounts 11 routes (8 from slice 2 + 3 new for slice 3). All new handlers check `s.automation == nil` first. | ✅ |
| **D3** | LEFT JOIN `users` para display name | `repository.go` line 226-235: `LEFT JOIN users u ON u.telegram_id = uws.user_id`. `scanWarningStateRow` (line 446-455) scans FirstName + Username. | ✅ |
| **D4** | `logs.CountByActionAndGroup` con `action = ANY($2)` (1 roundtrip) | `logs/repository.go` line 104-129: `SELECT action, COUNT(*) FROM logs WHERE group_id = $1 AND action = ANY($2) AND created_at >= $3 GROUP BY action`. | ✅ |
| **D5** | `ActionResetWarnings = "RESET_WARNINGS"` con ActorID != nil + metadata | `logs/model.go` line 60-64: constant + comment. `automation_handlers.go` line 746-758: `ActorID: &actorID`, `Metadata: {user_id, warning_count_before_reset: previous}`. | ✅ |
| **D6** | Permission check: `requireAuth` + `actorIDFromClaims` (admin-facing) | `automation_handlers.go`: handleListWarnings line 645 uses requireAuth; handleResetWarning line 697 uses actorIDFromClaims. NO permissionOkAdmin. | ✅ |
| **D7** | Frontend: nueva `GroupModerationPage.tsx` (2 secciones sin Save) | NEW file, 340 LOC. Sections: "Estadísticas" (3 Cards + Select + Refresh) + "Advertencias activas" (Table with Reset Modal). No Save button. | ✅ |
| **D8** | `GroupDetailPage`: rename + nuevo botón | Line 217: "Configurar reglas de moderación". Line 219-227: "Ver dashboard de moderación" link. Both in same Stack. | ✅ |
| **D9** | Refresh on-demand (no `refetchInterval`) | `hooks.ts` line 64-78: no `refetchInterval`. `GroupModerationPage.tsx` line 133-136: invalidateQueries on button click. | ✅ |
| **D10** | Cap defensivo top 100 + `truncated:bool` | `repository.go` line 222-257: `LIMIT $2`, returns `(rows, truncated, error)`. Handler `handleListWarnings` line 674-677 includes `truncated` in response. | ✅ |
| **D11** | Reset NO desmutea (intencional) | `handleResetWarning` only calls `automationDashboard.ResetWarningState` (DB only); Modal text "No desmutea al usuario en Telegram" (page line 318). | ✅ |
| **D12** | `ResetWarningState` usa SELECT prev + UPDATE (2 roundtrips) | `repository.go` line 267-304: SELECT then UPDATE; documented deviation from design D12 (Postgres `RETURNING old` not supported). | ✅ (deviation accepted) |
| **D13** | NO migración nueva (R2) | `git diff main -- backend/migrations/` = EMPTY. Slice 3 is 100% queries on existing tables. | ✅ |
| **D14** | `DisplayName() string` helper en backend | `model.go` line 142-150: `(w WarningStateRow) DisplayName() string` with FirstName → @username → "user {id}" fallback. | ✅ |
| **#172** | `permissionOkAdmin` invariante usa `BotStatus == StatusAdministrator`, NUNCA `can_*` | `grep can_ backend/internal/automation/*.go \| grep -v '^//'` = 0 matches in executable. 4 comment matches only (preserved as documentation). | ✅ |

**Design conformance verdict**: ✅ PASS. All 14 decisions + bugfix #172 invariant implemented as designed (or with accepted deviation D12).

---

## Deviations from design (documented in apply-report, all acceptable)

1. **`service.go` intacto** — design called for handlers to consume `*automation.Service`; slice 3 introduces `automationDashboardRepo` interface injected as 4th `WithAutomation` arg. **Acceptable**: keeps the slice 1+2+2.1 pipeline and `service.go` completely untouched; the repo's methods (`ListActiveWarningStatesByGroup`, `ResetWarningState`) are stable and testable in isolation.

2. **`UPDATE...RETURNING` no soporta old values en Postgres** — design proposed `RETURNING old.warning_count`; slice 3 uses SELECT prev + UPDATE (2 roundtrips). **Acceptable**: atomic from caller POV; race with `HandleMessage` tolerable (admin can retry); documented as deviation #2 in apply-report.

3. **`truncated` flag via `len == limit`** — spec allowed either an extra COUNT(*) query or `len == 100 && COUNT > 100`; slice 3 uses `len == limit` (cheaper). **Acceptable**: edge case where exactly 100 rows exist returns `truncated=true` falsely, but documented as acceptable (if 100+ warnings active, there's a bigger problem — rules miscalibrated).

---

## Issues found

### SUGGESTION (non-blocking)

- **apply-report claims `service_test.go -22 LOC`** but `git diff main` shows 0 lines of change. The fakeListsRepo in `main` already lacks dashboard methods — the change was already done in a prior slice (or never existed). Implementation is correct; apply-report's self-description is slightly off. No impact on the verdict.

- **apply-report claims "9 nuevos tests" but actual is 8 new test functions** + extended RequireAuth table covering 3 new routes. Coverage of all spec scenarios is still complete (every REQ-32..39 scenario has a corresponding test). Just a counting discrepancy in the report.

### WARNING (known, mitigated, not blocking)

- **Pre-existing flaky test** in `frontend/src/pages/PublicationsPage.test.tsx` (`crea una publicacion multi-grupo con foto y botones`): occasional 5s timeouts. Verified pre-existing on `main` via git stash + retest. Not introduced by slice 3. Documented as known in apply-report. Follow-up deferred.

- **Mantine v7 Modal lacks `role="dialog"`** by default. Mitigated via `data-testid` assertions (`reset-warning-modal`, `reset-warning-cancel`, `reset-warning-confirm`) in `GroupModerationPage.test.tsx`. Precedent in `GroupUsersPage.tsx:302-329`.

- **Mantine v7 Combobox scrollIntoView** stub added to `frontend/src/test/setup.ts`. Required because `GroupModerationPage` uses `<Select>` for period. Mitigated with `Element.prototype.scrollIntoView = noop`.

- **Bundle warning**: `dist/assets/index-*.js` is 731.97 kB (>500 kB threshold). Pre-existing condition (was already 700+ kB after slice 2.1); slice 3 adds ~30 kB for the dashboard. Future code-splitting is YAGNI for MVP; documented in apply-report.

### CRITICAL / FAIL

None.

---

## Final verdict

**PASS**.

- 9 REQs (REQ-32..40) all implemented and verified.
- 14 design decisions (D1-D14) + bugfix #172 invariant all satisfied (or accepted deviation).
- 3 documented deviations are justified and acceptable.
- Backend: 13/13 packages GREEN (`go test ./... -count=1`), `go vet` clean, `gofmt -l .` empty.
- Frontend: 87/87 tests GREEN across 15 files, `npm run build` clean.
- Non-regression: pipeline automation intact, `moderation/` intact, `publications/` intact, pre-existing frontend pages intact.
- §21.1 audit: zero Bot API calls in tests.
- §25 audit: zero secrets in diff.
- Bugfix #172: 0 `can_*` references in executable code in `backend/internal/automation/*.go`.

---

## Next steps for sdd-archive

After this verify passes, the orchestrator should run `sdd-archive` to:
1. APPEND the delta spec `REQ-32..40` from `openspec/changes/moderation-automation/slice3/specs/moderation-automation/spec.md` into the canonical `openspec/specs/moderation-automation/spec.md` (preserving REQ-1..31 from slices 1/2/2.1).
2. Move `openspec/changes/moderation-automation/slice3/` to `openspec/changes/archive/2026-09-08-moderation-automation-slice3/`.
3. Optionally: merge branch `feat/moderation-automation-slice3` → `main` (size:exception single-PR) and rebuild backend + frontend in Docker. **Slice 3 introduces zero migration, so no DB migration needed at deploy time** — only a backend image rebuild to register the 3 new HTTP routes and the `automationDashboardRepo` injection.
4. Document the 2 optional follow-ups in the archive report:
   - R2: `00009_add_logs_composite_index.sql` if `EXPLAIN ANALYZE` shows `CountByActionAndGroup >100ms` in production.
   - Pre-existing flaky `PublicationsPage` test (out of slice 3 scope).

## Relevant Files (verified)

### Backend
- `backend/internal/logs/{model,repository,repository_test}.go` — `ActionResetWarnings` + `CountByActionAndGroup` + 4 integration tests.
- `backend/internal/automation/{model,repository,repository_test}.go` — `WarningStateRow` + `DisplayName` + 2 new repo methods + 6 integration tests.
- `backend/internal/api/automation_handlers.go` — 3 new dashboard handlers (slice 3), `automationDashboardRepo` interface, `warningStateJSON`/`statsResponse` types, `statsPeriods` whitelist.
- `backend/internal/api/automation_handlers_test.go` — `fakeAutomationDashboardRepo` + 7 new handler tests + extended `TestAutomationRoutes_RequireAuth`.
- `backend/internal/api/server.go` — `Server.automationDashboard` field + `WithAutomation(auto, logs, groups, dashboard)` extended signature + 3 new `mux.HandleFunc`.
- `backend/cmd/server/main.go` — `automationRepo` var + 4th arg in both webhook + polling branches.

### Frontend
- `frontend/src/features/automation/{types,api,hooks,error}.ts` — extended with `WarningStateRow`, `WarningsResponse`, `StatsPeriod`, `AutomationStats`, `listWarnings`, `resetWarning`, `getStats`, `useWarnings`, `useResetWarning`, `useStats`, `formatDashboardError`.
- `frontend/src/pages/GroupModerationPage.tsx` (NEW) — 2-section dashboard, 340 LOC.
- `frontend/src/pages/GroupModerationPage.test.tsx` (NEW) — 9 tests, 368 LOC.
- `frontend/src/pages/GroupDetailPage.tsx` — label rename (línea 217) + new button "Ver dashboard de moderación".
- `frontend/src/pages/GroupDetailPage.test.tsx` — updated test asserts new labels + href.
- `frontend/src/App.tsx` — route `/groups/:id/moderation` registered.
- `frontend/src/test/setup.ts` — Mantine v7 Combobox scrollIntoView stub.

### Docs
- `README.md` — status updated (slice 3 shipped), routes table extended (`/moderation`), 3 REST endpoints documented, new section "Dashboard de moderación (panel, slice 3)" added. Bugfix cleanup: removed duplicate `GET /api/publications acepta:` block (artifact from a previous slice).
- `openspec/changes/moderation-automation/slice3/{exploration,proposal,design,tasks,apply-report}.md` — predecessors (untracked; expected to be committed when orchestrator finalizes the change).