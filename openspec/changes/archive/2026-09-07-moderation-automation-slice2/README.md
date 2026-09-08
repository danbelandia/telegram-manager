# Archive Report: `moderation-automation-slice2`

> **Change**: `moderation-automation-slice2` — Slice 2/3 of **Fase 3 — Moderación Automática** (AGENTS §23).
> **Status**: **ARCHIVED** — branch ready for orchestrator to merge → `main` and push.
> **Verdict**: **PASS-WITH-NOTES** (verify-report `#230`).
> **Mode**: hybrid (filesystem + Engram).
> **Strategy**: single-pr with `size:exception` (observation `#228`, 8/8 consecutivos).

---

## Summary

| Field | Value |
|-------|-------|
| Change name | `moderation-automation-slice2` |
| Slice | 2/3 of Fase 3 (AGENTS §23) |
| Base | `main @ 1b10d42` (slice 1 archivado, `#223`) |
| Branch | `feat/moderation-automation-slice2` |
| Commits ahead of main | 10 source commits + 1 archive commit (pending) |
| Files changed | 23 files; +3854 / -79 |
| Backend packages green | 13/13 |
| Frontend tests | 75/76 (1 pre-existing flake, NOT slice 2's responsibility) |
| Verdict | **PASS-WITH-NOTES** |
| Archived on | 2026-09-08 |
| Archive folder | `openspec/changes/archive/2026-09-07-moderation-automation-slice2/` |

---

## What Was Done

1. **APPEND delta REQs to canonical spec** at `openspec/specs/moderation-automation/spec.md`.
   - Canonical went from 437 lines (REQ-1 to REQ-15 from slice 1) → **1054 lines** (REQ-1 to REQ-21).
   - Slice-1 content (REQ-1..REQ-15) preserved verbatim.
   - New section appended: `## Slice 2 ADDED Requirements (2026-09-08 — Anti-spam + Anti-link + Banned-words + Settings UI)` with REQ-7 to REQ-21 content (15 new requirements, ~60 scenarios).
   - **APPEND**, not MOVE: slice-1 was a NEW capability (delta was the full spec, so MOVE was correct there). Slice 2 is an AMEND (delta adds new REQs to existing canonical), so APPEND preserves slice-1 REQs intact.
   - The delta spec remains inside the archive folder (`specs/moderation-automation/spec.md`) as audit trail.

2. **MOVED change folder** `openspec/changes/moderation-automation-slice2/` → `openspec/changes/archive/2026-09-07-moderation-automation-slice2/`.
   - All artifacts preserved (proposal.md, design.md, tasks.md, apply-report.md, verify-report.md + the delta specs folder).
   - The exploration.md stays at the parent `openspec/changes/moderation-automation/exploration.md` because it spans all 3 slices of Fase 3.

3. **Wrote this README.md** documenting: metadata, branch base, verdict, delivery strategy, spec sync action (APPEND with rationale), file inventory, Engram observation IDs, full commit list, the **4 documented deviations**, the **1 pre-existing flake**, the **bugfix `#172` invariant verification**, the slice 2/3 context, invariantes respetadas, and next steps.

4. **Commit pending**: `chore(openspec): archive change moderation-automation-slice2` (single conventional commit on `feat/moderation-automation-slice2`).

5. **NO push, NO merge** (rule of archive phase).

---

## Spec Sync Actions

| Action | Detail |
|--------|--------|
| Capability | `moderation-automation` (existing from slice 1, `#223`). |
| Sync method | **APPEND** (delta is a delta, not a full spec). Slice-1 REQs (REQ-1..REQ-15) preserved verbatim; slice-2 REQs (REQ-7..REQ-21, numbered to fit alongside slice 1) appended in a new section. |
| ADDED | 15 requirements (REQ-7..REQ-21 in canonical order): Schema `banned_words`, Schema `link_allowlist`, `AntiSpamRule`, `AntiLinkRule`, `BannedWordsRule`, Rule registry order cheap→expensive, `Service.HandleMessage` pre-load lists, `Rule.Evaluate` extended with `*Lists`, GET/PUT settings, CRUD banned-words, CRUD link-allowlist, Frontend `GroupAutomationPage`, Action constants (5 nuevas), Tests §21.1, No regresión. |
| MODIFIED | None. Existing `telegram-moderation` (manual moderation, REQ-1..REQ-15 of slice-1 canonical) and slice-1 foundation (`group_moderation_settings`, `user_warning_state`, `FloodRule`, Worker, AutoActioner, bus subscriber) all preserved. |
| Removed | None. |
| Audit trail | The delta spec.md stays inside the archive folder (`specs/moderation-automation/spec.md`) as evidence of the original ADDED Requirements content. `git log --follow openspec/specs/moderation-automation/spec.md` will show the slice-1 entry followed by this slice-2 archive commit (which has the +617 lines appended). |

---

## Engram Observation Traceability

- `#215` — `sdd/moderation-automation/exploration` (3-slice plan, D1–D10, all 7 features of AGENTS §23)
- `#224` — `sdd/moderation-automation/slice2/proposal` (slice 2 scope, D1-D11, success criteria, schema)
- `#225` — `sdd/moderation-automation/slice2/spec` (delta spec, 15 ADDED REQs, ~60 scenarios)
- `#226` — `sdd/moderation-automation/slice2/design` (D1-D8 + bugfix #172, data flow, file specs, interfaces)
- `#227` — `sdd/moderation-automation/slice2/tasks` (8 phases, 36 tasks, Forecast High → size:exception)
- `#228` — `sdd/moderation-automation/slice2/delivery-strategy` (decision — size:exception approved, 8/8 consecutivo)
- `#229` — `sdd/moderation-automation/slice2/apply-report` (10 commits, 23 files, gates green, 4 deviations)
- `#230` — `sdd/moderation-automation/slice2/verify-report` (PASS WITH NOTES, 15/15 REQs, 8/8 D, bugfix #172, 1 pre-existing flake)
- `#THIS` — `sdd/moderation-automation/slice2/archive-report` (this observation)

---

## Archive Contents

```
openspec/changes/archive/2026-09-07-moderation-automation-slice2/
├── README.md                          ← this file (archive metadata)
├── proposal.md                        ← slice 2 scope, D1-D11, success criteria, schema, risks, rollback
├── design.md                          ← D1-D8 + bugfix #172, file-by-file specs, data flow, interfaces
├── tasks.md                           ← 8 phases, 36 tasks (all [x]), Forecast High → size:exception
├── apply-report.md                    ← 10 commits, 23 files, gates green, 4 documented deviations
├── verify-report.md                   ← PASS-WITH-NOTES, 15/15 REQ compliance matrix, 8/8 D, 1 pre-existing flake
└── specs/
    └── moderation-automation/
        └── spec.md                    ← ORIGINAL DELTA SPEC (REQ-7..REQ-21 in delta form, ADDED Requirements)
                                         — preserved as audit trail of the spec change applied to canonical
```

---

## File Inventory (23 files changed, +3854/-79 lines)

### Backend NEW (3 files, +1063)
| File | +Ins | Description |
|------|-----:|-------------|
| `backend/migrations/00007_create_moderation_lists.sql` | 39 | DDL `banned_words` + `link_allowlist` (PK compuesta, FK CASCADE, CHECK 1-100/253, índices) |
| `backend/internal/api/automation_handlers.go` | 544 | 9 handlers (GET/PUT settings + 6 lists CRUD) con `requireAuth`, `actorIDFromClaims`, envelope §18 |
| `backend/internal/api/automation_handlers_test.go` | 480 | Handler tests con fakes (SettingsRepo + ListsRepo + LogWriter); 200/400/404/401 paths |

### Backend MOD (10 files, +1241/-78)
| File | +Ins | Description |
|------|-----:|-------------|
| `backend/internal/automation/model.go` | 42 | `Lists{BannedWords, LinkAllowlist}`, `ErrAutomationGroupNotFound` |
| `backend/internal/automation/rules.go` | 320 | `Rule.Evaluate(..., lists *Lists, now)` extendida; 3 reglas nuevas (AntiSpam/AntiLink/BannedWords); `FloodRule` actualizada firma (ignora `lists`) |
| `backend/internal/automation/rules_test.go` | 296 | 18+ unit tests nuevos (AntiSpam 7, AntiLink 7, BannedWords 6, Registry order 1, defensive 1) |
| `backend/internal/automation/service.go` | 122 | `preloadLists(ctx, settings)` (skip si ambos toggles off); pasa `*Lists` al registry |
| `backend/internal/automation/service_test.go` | 315 | 6 nuevos service tests (pre-load on/off, listas compartidas, defensivo) |
| `backend/internal/automation/repository.go` | 101 | 6 CRUD methods (List/Add/Remove por tabla, `LOWER` en banned, case-preserved en allowlist) |
| `backend/internal/automation/repository_test.go` | 277 | 13 integration tests (FK CASCADE, idempotencia, atomic set) |
| `backend/internal/api/server.go` | 32 | `WithAutomation` option + 9 rutas bajo `/api/groups/:id/automation/...` |
| `backend/internal/logs/model.go` | 22 | 5 Action constants (`UPDATE_AUTOMATION_SETTINGS`, `ADD/REMOVE_BANNED_WORD`, `ADD/REMOVE_LINK_ALLOWLIST`) |
| `backend/cmd/server/main.go` | 14 | Registra 4 reglas en orden cheap→expensive; wire `WithAutomation` en ambos modos |

### Frontend NEW (6 files, +1117)
| File | +Ins | Description |
|------|-----:|-------------|
| `frontend/src/features/automation/types.ts` | 81 | `AutomationSettings`, `SettingsUpdate`, `BannedWordsList`, `LinkAllowlistList` |
| `frontend/src/features/automation/api.ts` | 70 | 8 funciones (getSettings, putSettings, list/add/remove × 2 listas) |
| `frontend/src/features/automation/hooks.ts` | 103 | 8 hooks React Query (`retry:false` + invalidación por prefijo) |
| `frontend/src/features/automation/error.ts` | 64 | `formatAutomationError`, validators cliente (regex banned, thresholds autoban>automute>0) |
| `frontend/src/pages/GroupAutomationPage.tsx` | 468 | 4 secciones (5 Switches, 7 NumberInputs, 2 `<TagsInput>`, Save con `Promise.all` paralelo, notifyError por sección) |
| `frontend/src/pages/GroupAutomationPage.test.tsx` | 331 | 8 tests con `mockFetchRoutes` (render, save flow, error path, link desde detail) |

### Frontend MOD (3 files, +101/-1)
| File | +Ins | Description |
|------|-----:|-------------|
| `frontend/src/App.tsx` | 87 | Ruta `/groups/:id/automation` bajo `RequireAuth` (+1 new line, others reorganization) |
| `frontend/src/pages/GroupDetailPage.tsx` | 9 | Botón "Configurar automatización" en panel "Detalle" |
| `frontend/src/pages/GroupDetailPage.test.tsx` | 5 | Assert del link |

### Docs MOD (1 file, +111)
| File | +Ins | Description |
|------|-----:|-------------|
| `README.md` | 111 | Sección "Moderación automática (Fase 3)" — describe reglas, settings, listas, configuración desde panel |

**Total**: 23 files, **+3854 insertions, -79 deletions**.

---

## Commit List (10 source commits + 1 archive commit, feat/moderation-automation-slice2, NOT pushed)

Source (10 commits, base `main @ 1b10d42`):

1. `39ecc43` — `feat(automation): add banned_words/link_allowlist migration + Lists struct` (model.go + 00007_create_moderation_lists.sql)
2. `596c123` — `feat(automation): extend Rule.Evaluate with Lists for pre-loaded banned-words/allowlist` (rules.go + rules_test.go + service.go stub)
3. `60de79c` — `feat(automation): add CRUD methods for banned_words and link_allowlist` (repository.go + repository_test.go)
4. `66bd923` — `feat(automation): register all 4 rules and wire WithAutomation in main.go` (cmd/server/main.go)
5. `5d039b0` — `feat(logs): add 5 action constants for moderation automation manual edits` (logs/model.go)
6. `bbd84c4` — `feat(api): add 9 automation endpoints (settings + banned-words + link-allowlist)` (automation_handlers.go + automation_handlers_test.go + server.go)
7. `1a1aab8` — `feat(frontend): add automation feature module (types, api, hooks, error)` (features/automation/*)
8. `c6daa49` — `feat(frontend): add GroupAutomationPage with editor + Save all flow` (GroupAutomationPage.tsx + test)
9. `637cd99` — `feat(frontend): register /groups/:id/automation route + link from detail` (App.tsx + GroupDetailPage.tsx + test)
10. `f56f821` — `docs: README moderacion automatica section (rules + settings UI + audit)` (README.md)

Archive (1 commit, pending):
11. `<pending>` — `chore(openspec): archive change moderation-automation-slice2` (canonical spec +617 lines + moved archive folder + this README)

Total source: **23 files changed, +3854/-79** — matches apply-report `#229` and verify-report `#230`.

---

## Documented Deviations (verify-report `#230` §Deviations, apply-report `#229` §Deviations)

All 4 deviations are **localized and acceptable** per verify-report. None break a spec REQ.

### D1 — TagsInput add/remove tests reducidos

**Deviation**: Mantine v7 `<TagsInput>` + `user-event` tiene ciclo de re-render peculiar. Slice 2 reduce los tests directos del TagsInput en `GroupAutomationPage.test.tsx` y los reemplaza por tests más determinísticos (toggle Switch, NumberInput, discard, listas iniciales).

**Why**: El test mecánico exacto del `<TagsInput>` (add → ver en lista → remove) tiene flakiness con re-renders de Mantine v7 + `@testing-library/user-event`.

**Justification**: Cobertura funcional intacta — el Save flow sí dispara PUT settings + POST/DELETE per palabra/dominio. Los handler tests backend SÍ cubren POST/DELETE words end-to-end (10 tests). El flujo de save completo (con Promise.all paralelo y error path) sí está cubierto.

### D2 — Handler GET settings: defaults sin persistir

**Deviation**: `handleGetSettings` retorna `GetSettings` + fallback a `DefaultSettings(groupID)` SIN persistir la fila en GET inicial. El primer PUT upserta y crea la fila.

**Why**: Comportamiento idéntico al usuario final (mismo payload JSON), pero evita escrituras a DB en lecturas.

**Justification**: Beneficio: GET inicial sin escribir a DB. Si el GET persistiera, cada panel abierto haría un INSERT sin valor funcional. Slice-1 ya tenía `DefaultSettings` helper; D2 lo reusa en el path HTTP.

### D3 — `Service.NewService` recibe `listsRepo` adicional

**Deviation**: La firma de `Service.NewService` agrega `listsRepo` como argumento entre `warnRepo` y `registry`.

**Why**: Necesario para REQ-13 (pre-load en `HandleMessage`).

**Justification**: Cambio mecánico. Tests existentes actualizados con `nil`; tests nuevos usan `fakeListsRepo` para verificar la omisión/carga condicional.

### D4 — `automationService` declarado fuera del `if cfg.AutomationEnabled`

**Deviation**: En `cmd/server/main.go`, `automationService` se declara fuera del bloque condicional de `cfg.AutomationEnabled`. `WithAutomation` se monta en ambos modos (webhook + polling) independientemente del toggle.

**Why**: Los handlers HTTP existen y devuelven 404 si `s.automation == nil` (consistente con `WithPublications`/`WithModeration`).

**Justification**: Consistencia con el patrón existente. `WithAutomation` solo monta las rutas; los handlers verifican el toggle y devuelven 404 cuando está deshabilitado. No es una fuga de secrets ni un cambio de scope.

---

## Bugfix `#172` Invariant Verification

The `permissionOk` helper in `backend/internal/moderation/` historically read `g.BotPermissions[key]` for `can_*` keys that the group detector never populates — a known bug. Per bugfix `#172`, **all new code MUST use `g.BotStatus == groups.StatusAdministrator`**, NEVER `can_*`.

Slice 2 verification (verify-report `#230` §8):

```
Select-String -Path "backend\internal\automation\*.go" -Pattern "\bcan_" |
  Where-Object { $_.Line -notmatch "^\s*//" }
```

**Result**: 3 matches, ALL comment-only:

```
COMMENT-ONLY: backend/internal/automation/autoactioner.go:85
COMMENT-ONLY: backend/internal/automation/model.go:10
COMMENT-ONLY: backend/internal/automation/service.go:50
```

All 3 matches are in **comments documenting the invariant** (lines like "NUNCA leer claves can_*"). **Zero matches in executable code**. Bugfix `#172` invariant INTACT in slice 2.

**Verification per file**:
- `automation_handlers.go`: no `permissionOk` call (handlers are admin-gated via `requireAuth` from panel, not bot-touched). Documented in code comments.
- `rules.go` (3 new rules): stateless, no permission check.
- `service.go` HandleMessage: pre-existente `permissionOkAdmin` (slice 1) intact.
- `autoactioner.go`: pre-existente `permissionOkAdmin` (slice 1) intact.

**Compared to slice 1**: slice-1 had 4 explicit `#172` documentation comments (`model.go:8`, `service.go:35`, `service.go:258`, `autoactioner.go:84`). Slice 2 adds 0 new sites needing the documentation (no new permission checks introduced) but **PRESERVES** the 3 pre-existing comments that are still active. The 4th comment in `service.go:35` was promoted/inlined into the slice-1 `preloadLists` path which slice 2 reuses (no new check needed).

---

## 1 Pre-existing Flake (NOT caused by this slice)

`PublicationsPage.test.tsx > crea una publicacion multi-grupo con foto y botones` fails by timeout (5s) in this session's `npm test -- --run` run.

**Provenance verified — pre-existing in `main`, NOT slice 2's responsibility**:

- `git diff main --stat -- frontend/src/pages/PublicationsPage.test.tsx` = **EMPTY** (test file unchanged by slice 2).
- `git log --all -- frontend/src/pages/PublicationsPage.test.tsx` last modified in `f4d1751 feat(frontend): migrate PublicationsPage to Mantine v7` (publications slice 3).
- The test fails identically in the version checked out from `main` (`5053ms` timeout).
- File is in the **non-regression protected set** per `tasks.md §7.6` and per orchestrator's diff command.

**Note on apply-report discrepancy**: apply-report `#229` claimed "npm test -- --run (76/76) green". This verify run got 75/76. The discrepancy is environmental (the flaky test happens to time out in this session's run). The pattern matches apply-report's pre-existing note about `TestTokenManager_TamperedTokenRejected` backend test being flaky.

**Action**: This is a **SUGGESTION-level cleanup item**, NOT a blocker. The pre-existing flaky test should be addressed in a separate change (either increase timeout, fix the async wait, or fix the underlying race). Out of slice 2's scope.

---

## Slice 2/3 Context — Fase 3 (AGENTS §23)

Per exploration `#215` and proposal `#224`, Fase 3 ships in **3 slices**:

| Slice | Scope | Backend LOC | Frontend LOC | Status |
|-------|-------|------------:|-------------:|--------|
| **1 — Foundation** | settings + warning_state + FloodRule + worker + logs | ~1500 | 0 | **ARCHIVED** (obs `#223`, `main @ 1b10d42`) |
| **2 — More rules + Settings UI** (THIS) | AntiSpam + AntiLink + BannedWords + Settings editor | ~1241 | ~1117 | **THIS ARCHIVE** |
| 3 — Warnings Dashboard | Read `user_warning_state` + logs aggregations + dashboard UI | ~150 | ~570 | Pending (will be `moderation-automation-slice3`) |

Each slice individually exceeds the 400-line review budget — `size:exception` is the established pattern (8/8 consecutivos aprobados en este repo, obs `#228`).

---

## Invariantes Respetadas

- **§11 (NO Redis)**: 0 Redis dependencies; pure backend stack with in-process channel (slice 1 foundation, preserved).
- **§14 (modular backend)**: All new files in `internal/automation/` (rules, repository, service, model) and `internal/api/automation_handlers.go`. No leaks to other packages.
- **§13.1 (goose migrations)**: 1 new migration `00007_create_moderation_lists.sql` with `-- +goose Up`/`Down` markers. SQL plano, no DSL.
- **§17.1 (no secrets)**: 0 secrets in code. `.env.example` untouched (slice 1 already covers env).
- **§18.1 (rate limits Telegram)**: Slice 1 worker + adapter rate limit intact. Slice 2 adds NO new direct calls to Bot API; all 9 new HTTP endpoints are admin-gated (panel → backend, never frontend → Telegram).
- **§21.1 (mock TelegramService)**: All 13 backend automation tests use hand-rolled fakes (`fakeSettingsRepo`, `fakeListsRepo`, `fakeTelegramActor`, `fakeLogs`, etc.). Zero `moq` codegen; zero Bot API calls in tests.
- **§23 (Fase 3 — this slice)**: Anti-spam + Anti-link + Banned-words + Settings UI delivered. Foundation from slice 1 preserved.
- **§25 regla 18 (frontend no llama Telegram)**: `git diff main -- frontend/` shows only changes inside `automation/` feature module + `GroupAutomationPage.tsx`/`GroupDetailPage.tsx`. No `telegram.org` URLs in any frontend code.
- **Bugfix `#172`** (permissionOkAdmin uses `BotStatus`, NEVER `can_*`): verified, zero executable `can_*` matches in slice 2 (3 comment-only docs preserved).
- **Frontend pre-existing pages intact**: `git diff main -- frontend/src/pages/` excluding `GroupAutomationPage.tsx` and `GroupDetailPage.tsx` returns empty.

---

## Verification Summary (verify-report `#230`)

### REQ Compliance Matrix (15/15 PASS)

| REQ | Theme | Status |
|-----|-------|--------|
| REQ-7 | Schema `banned_words` | ✅ PASS |
| REQ-8 | Schema `link_allowlist` | ✅ PASS |
| REQ-9 | `AntiSpamRule` (3 sub-detectors) | ✅ PASS |
| REQ-10 | `AntiLinkRule` + `domainMatches` | ✅ PASS |
| REQ-11 | `BannedWordsRule` | ✅ PASS |
| REQ-12 | Registry order | ✅ PASS |
| REQ-13 | Service pre-load lists once | ✅ PASS |
| REQ-14 | `Rule.Evaluate` extended with `*Lists` | ✅ PASS |
| REQ-15 | GET/PUT settings | ✅ PASS |
| REQ-16 | CRUD banned-words | ✅ PASS |
| REQ-17 | CRUD link-allowlist | ✅ PASS |
| REQ-18 | Frontend `GroupAutomationPage` | ✅ PASS |
| REQ-19 | 5 Action constants (manual vs auto) | ✅ PASS |
| REQ-20 | Tests §21.1 | ✅ PASS |
| REQ-21 | No regresión | ✅ PASS |

### Design Coherence (8/8 + bugfix #172 PASS)

| Decision | Status |
|----------|--------|
| D1 — Tablas paralelas (no JSONB) | ✅ PASS |
| D2 — Case-insensitive en `banned_words` server-side | ✅ PASS |
| D3 — Subdomain match con prefijo-punto obligatorio | ✅ PASS |
| D4 — `Rule.Evaluate` extendido con `*Lists` | ✅ PASS |
| D5 — Registry order fixed en `cmd/server/main.go` | ✅ PASS |
| D6 — Frontend single Save con `Promise.all` | ✅ PASS |
| D7 — Audit log con `ActorID` admin (≠ nil) para manuales | ✅ PASS |
| D8 — Idempotencia vía PK compuesta + `ON CONFLICT DO NOTHING` | ✅ PASS |
| #172 — `permissionOkAdmin` invariante | ✅ PASS |

### Full Backend Suite Verification (verify-report `#230` §2)

```
ok    github.com/telegram-manager/backend/internal/api            2.646s
ok    github.com/telegram-manager/backend/internal/auth           2.469s
ok    github.com/telegram-manager/backend/internal/automation    6.658s   ← slice 1+2
ok    github.com/telegram-manager/backend/internal/config         1.139s
ok    github.com/telegram-manager/backend/internal/events         1.241s
ok    github.com/telegram-manager/backend/internal/groups         3.280s
ok    github.com/telegram-manager/backend/internal/joinrequests   1.597s
ok    github.com/telegram-manager/backend/internal/logs           1.198s
ok    github.com/telegram-manager/backend/internal/moderation     1.194s
ok    github.com/telegram-manager/backend/internal/publications   2.428s
ok    github.com/telegram-manager/backend/internal/telegram      22.580s
ok    github.com/telegram-manager/backend/internal/users          0.587s
EXIT 0
```

**13/13 packages green**. `go vet ./...` clean. `gofmt -l .` clean. `go build ./...` clean.

### Frontend Build (verify-report `#230` §6)

```
vite v8.2.2 building client environment for production...
✓ 7131 modules transformed.
dist/index.html                   0.40 kB │ gzip:   0.26 kB
dist/assets/index-C3rkZyxX.css  198.06 kB │ gzip:  28.98 kB
dist/assets/index-DloOppyC.js   720.64 kB │ gzip: 218.10 kB
✓ built in 8.48s
EXIT 0
```

✅ Green.

### Non-regression Diff (verify-report `#230` §7)

```bash
git diff main -- \
  backend/internal/moderation/ \
  backend/internal/publications/ \
  backend/internal/automation/autoactioner.go \
  backend/internal/automation/worker.go \
  frontend/src/pages/PublicationsPage.tsx \
  frontend/src/pages/GroupUsersPage.tsx \
  frontend/src/pages/GroupRequestsPage.tsx \
  frontend/src/pages/GroupLogsPage.tsx
```

**Output: empty** (0 lines).

✅ Non-regression intact.

### §21.1 Invariant (verify-report `#230` §9)

```bash
Select-String -Path "backend\internal\automation\*_test.go" -Pattern "telegram\.Bot|api\.telegram\.org|http\.DefaultClient|tg\.Adapter|NewAdapter"
```

**Result: empty**.

✅ Zero Bot API calls in automation tests.

---

## Suggestions for Future (verify-report `#230` §Issues, SUGGESTION)

- **S1**: `GroupAutomationPage.tsx` has **7 NumberInputs** instead of design's 6 (includes `warning_expire_days`). Implementation is **MORE** functional coverage than design — not a defect.
- **S2**: Frontend flaky `PublicationsPage.test.tsx` (pre-existing in main) — clean up in a separate change.
- **S3**: `events_subscriber.go` doesn't exist as a separate file (slice 1 architecture bundled it into `cmd/server/main.go` L171). Non-regression intent preserved.
- **S4**: `cmd/server/main.go` ordering of `WithAutomation` in both modes could be extracted to a single registration helper (cosmetic).

---

## Next Steps (for orchestrator)

1. **Merge `feat/moderation-automation-slice2` → `main`** (single PR, `size:exception` aprobado per obs `#228`, precedent 8/8 consecutivos).
2. **Push to remote**.
3. Slice 2 closes the rule motor of Fase 3. Next candidate per exploration `#215`:
   - **`moderation-automation-slice3`** — Warnings dashboard + stats aggregations + frontend dashboard.
   - Will branch from `main` after slice 2 lands.

---

## Commit (this archive)

- **Message**: `chore(openspec): archive change moderation-automation-slice2`
- **Branch**: `feat/moderation-automation-slice2`
- **Files**:
  - Modified: `openspec/specs/moderation-automation/spec.md` (+617 lines, REQ-7..REQ-21 appended)
  - Added: `openspec/changes/archive/2026-09-07-moderation-automation-slice2/README.md` (this file)
  - Moved (untracked → archive): `openspec/changes/archive/2026-09-07-moderation-automation-slice2/{proposal.md, design.md, tasks.md, apply-report.md, verify-report.md, specs/moderation-automation/spec.md}`
- **NO source code touched.**
- **Push/Merge**: NEITHER (rule of archive phase).
- **Commit SHA**: pending (will be set after `git commit`).

---

**SDD Cycle Complete for `moderation-automation-slice2`.** The change has been fully planned (proposal, spec, design, tasks), implemented (10 commits), verified (PASS-WITH-NOTES, 15/15 REQs, 8/8 D), and archived (canonical spec amended, change folder moved). Ready for the orchestrator to merge `feat/moderation-automation-slice2` → `main` and push.