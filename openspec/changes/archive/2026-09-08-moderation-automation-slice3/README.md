# Archive — `moderation-automation-slice3`

> **Change**: `moderation-automation-slice3` — **Slice 3/3 (último)** de **Fase 3 — Moderación Automática** (AGENTS §23).
> **Mode**: hybrid (filesystem + Engram). Persisted as `sdd/moderation-automation/slice3/archive-report` + this `README.md`.
> **Base**: `main @ e7d0680` (slice 2.1 archivado, obs `#241`).
> **Branch**: `feat/moderation-automation-slice3` (8 commits ahead of `main @ e7d0680`, NOT pushed, NOT merged).
> **Archived on**: 2026-09-08 → `openspec/changes/archive/2026-09-08-moderation-automation-slice3/`.
> **Verdict**: **PASS** (verify-report obs `#249`).
> **Delivery strategy**: single-pr con `size:exception` (decision obs `#247`, 11/11 consecutivos aprobados por el usuario; precedente obs `#220`/`#228`/`#238`).
> **Commit SHA (this archive)**: `6e3c804` — `chore(openspec): archive change moderation-automation-slice3` (9 files changed, 2360 insertions(+); 1 tracked file renamed 100% + 1 canonical modified + 7 new files).

## Status

Slice 3 **cierra Fase 3** (`Fase 3 — Moderación Automática`, AGENTS §23) con la entrega del **dashboard de moderación** (read-only stats + lista de advertencias activas) + **reset manual** opcional para admin actions. Con este archive, **Fase 3 queda cerrada al 100%** — los 3 slices + 1 sub-slice planificados en exploration `#215` están archivados:

| Slice | Scope | Status |
|-------|-------|--------|
| **1 — Foundation** | settings + warning_state + FloodRule + worker + logs (backend-only) | ARCHIVED (obs `#223`, `main @ 1b10d42`) |
| **2 — More rules + Settings UI** | AntiSpam + AntiLink + BannedWords + Settings editor (full-stack) | ARCHIVED (obs `#231`, `main @ 2df231f`) |
| **2.1 — Warning visual pre-acción** | 2 cols schema + 2 templates + WarningSender + Sección 5 frontend | ARCHIVED (obs `#241`, `main @ e7d0680`) |
| **3 — Warnings Dashboard (THIS)** | Read `user_warning_state` + logs aggregations + dashboard UI + reset manual | **THIS ARCHIVE** |

## What was done (this archive)

1. **APPENDED REQ-32..REQ-40 to canonical spec** at `openspec/specs/moderation-automation/spec.md` (APPEND technique; slices 1+2+2.1 REQ-1..REQ-31 preserved verbatim). Canonical went from **1583 lines (REQ-1..REQ-31) → 1902 lines (REQ-1..REQ-40, 40 total)**, **+319 insertions**. New section heading added: `## Slice 3 ADDED Requirements (2026-09-08 — Dashboard de moderación + Stats + Reset)`. Slice 1+2+2.1 REQ-1..REQ-31 content preserved verbatim. Audit trail: `git log --follow openspec/specs/moderation-automation/spec.md` will show: slice-1's NEW entry → slice-2's APPEND (commit `2df231f`, +617) → slice-2.1's APPEND (commit `e7d0680`, +529) → this slice-3's APPEND (commit `e7d0680+1`, +319).

2. **MOVED change folder** `openspec/changes/moderation-automation/slice3/` → `openspec/changes/archive/2026-09-08-moderation-automation-slice3/` via PowerShell `Move-Item` (the `apply-report.md` was tracked at commit `6bb38a0`; the other 6 files — `exploration.md`, `proposal.md`, `design.md`, `tasks.md`, `verify-report.md`, and `specs/moderation-automation/spec.md` — were untracked in the working tree before the move). All artifacts preserved in the archive folder as audit trail:
   - `exploration.md` (slice 3 exploration, D1-D10, 12 risks, 5 approaches)
   - `proposal.md` (slice 3 scope, capabilities, D1-D10, success criteria)
   - `design.md` (D1-D14 + bugfix #172, file-by-file specs, interfaces, tests strategy)
   - `tasks.md` (8 phases, Review Workload Forecast High → size:exception)
   - `apply-report.md` (8 source commits, 21 files, gates green, 3 deviations)
   - `verify-report.md` (PASS, 9 REQs + 14 design decisions + bugfix #172 invariant)
   - `specs/moderation-automation/spec.md` (ORIGINAL DELTA SPEC — 408 lines, ADDED Requirements preserved as audit trail of content applied to canonical)
   - `README.md` (this file — archive metadata, full provenance)

3. **Wrote `README.md`** (this file) documenting: change metadata, branch base, verdict (PASS), delivery strategy (`size:exception`, 11/11 consecutivos), spec sync action (APPEND with rationale), file inventory (21 files source, +2389/-39), Engram observation IDs, full commit list (8 source SHAs + 1 archive SHA), the **3 documented deviations** (interface `automationDashboardRepo` injection pattern, SELECT prev + UPDATE instead of `RETURNING old`, `len == limit` shortcut for `truncated`), the **bugfix `#172` invariant verification** (0 executable `can_*` matches; 4 comment-only docs preserved in `service.go:50`, `autoactioner.go:85`, `model.go:10`, `warning_sender.go:10`), the **2 SUGGESTION-level items** (R2 future `logs(group_id, action, created_at)` index if EXPLAIN ANALYZE >100ms; pre-existing flaky test in `PublicationsPage.test.tsx`), the Fase 3 closure context (slice 3/3 closing the entire Fase 3), invariantes respetadas (§11/§14/§17.1/§18.1/§21.1/§23/§25 + bugfix #172), and next steps for orchestrator.

4. **Commit pending**: `chore(openspec): archive change moderation-automation-slice3` (single conventional commit on `feat/moderation-automation-slice3`).

5. **NO push, NO merge** (rule of archive phase — orchestrator handles the merge to `main` and the rebuild of backend + frontend in Docker).

## Why

Slice 3 closes the **feedback loop** of Fase 3 (AGENTS §23). Slices 1+2+2.1 producen todos los datos que el admin necesita para supervisar la moderación automática: `user_warning_state` (counter por `(group, user)`, `last_warning_at`, `last_action_at`, `expires_at`) y logs con 4 acciones (`RULE_TRIGGERED`, `AUTOMUTE_USER`, `AUTOBAN_USER`, `WARN_USER_SENT`). Antes de slice 3 el admin no tenía UI práctica para consultarlos — tenía que correr SQL o leer logs crudos para saber **quién está cerca del threshold**, **cuánto se está moderando** en 24h/7d, o **resetear manualmente** el counter de un user problemático.

Slice 3 entrega:
- **3 endpoints backend read-only + 1 mutación opcional de reset** (`GET /warnings`, `POST /warnings/{user_id}/reset`, `GET /stats?period=24h|7d`).
- **2 repository methods nuevos** con agregaciones SQL (`ListActiveWarningStatesByGroup` con LEFT JOIN a `users` para display name, `CountByActionAndGroup` para stats en 1 roundtrip).
- **1 constante `ActionResetWarnings = "RESET_WARNINGS"`** (admin action, `ActorID != nil`, metadata `{user_id, warning_count_before_reset}` para auditoría).
- **1 página frontend nueva** (`GroupModerationPage` en `/groups/:id/moderation`, 2 secciones sin Save button — read-only + reset).
- **1 link + 1 rename en `GroupDetailPage`** (Tab "Detalle" — label update "Configurar automatización" → "Configurar reglas de moderación" + nuevo botón "Ver dashboard de moderación").
- **Cero migración nueva** (queries 100% sobre tablas existentes `user_warning_state`, `logs`, `users`).

Con esto, **Fase 3 — Moderación Automática (AGENTS §23)** queda cerrada al 100%. Después de merge a `main`, el motor de moderación automática cubre: settings + warning state + reglas (flood + anti-spam + anti-link + banned-words) + warning visual pre-acción + auto-mute + auto-ban + worker secuencial + audit logs + dashboard de admin.

## Where

- **AMENDED in place**: `openspec/specs/moderation-automation/spec.md` (canonical, 1583 → 1902 lines; REQ-1..REQ-40, 40 total).
- **Moved to archive**: `openspec/changes/archive/2026-09-08-moderation-automation-slice3/`
  - `README.md` (this file — archive metadata, full provenance)
  - `exploration.md` (D1-D10, affected areas, risks, fallback chain, 5 approaches)
  - `proposal.md` (slice 3 scope, capabilities, D1-D10, success criteria, 13 risks)
  - `design.md` (D1-D14 + bugfix #172, file-by-file specs, data flow, interfaces, risk table, tests strategy)
  - `tasks.md` (8 phases, Review Workload Forecast High → size:exception)
  - `apply-report.md` (8 source commits, 21 files, gates green, 3 documented deviations)
  - `verify-report.md` (PASS, 9 REQs + 14 design decisions + bugfix #172 invariant, 1 pre-existing flake)
  - `specs/moderation-automation/spec.md` (ORIGINAL DELTA SPEC — 408 lines, ADDED Requirements preserved as audit trail)
- **Removed from active changes**: `openspec/changes/moderation-automation/slice3/` (no longer exists; verified via `Get-ChildItem` post-move — only `exploration.md` remains at parent, which covers all 3 slices of Fase 3 and is intentionally NOT moved).
- **Stays at parent** (not moved): `openspec/changes/moderation-automation/exploration.md` — covers all 3 slices of Fase 3 (slice 3 was proposed against it).

## Engram observation traceability

| ID | Topic |
|----|-------|
| `#215` | `sdd/moderation-automation/exploration` (3-slice plan of Fase 3, D1–D10, all features of AGENTS §23) |
| `#242` | `sdd/moderation-automation/slice3/exploration` (slice 3 scope, D1-D10, 12 risks, 5 approaches, 8 open questions resolved) |
| `#243` | `sdd/moderation-automation/slice3/proposal` (slice 3 scope, D1-D10, 13 risks, success criteria, ~830 LOC forecast) |
| `#244` | `sdd/moderation-automation/slice3/spec` (delta spec, 9 ADDED REQs REQ-32..40, ~33 scenarios, coverage map) |
| `#245` | `sdd/moderation-automation/slice3/design` (D1-D14 + bugfix #172, file-by-file specs, interfaces, 15 risks) |
| `#246` | `sdd/moderation-automation/slice3/tasks` (8 phases, Forecast High → size:exception, all tasks complete) |
| `#247` | `sdd/moderation-automation-slice3/delivery-strategy` (decision — `size:exception` approved, 11/11 consecutivo) |
| `#248` | `sdd/moderation-automation/slice3/apply-report` (8 source commits, 21 files, gates green, 3 documented deviations) |
| `#249` | `sdd/moderation-automation/slice3/verify-report` (PASS, 9 REQs + 14 design decisions + bugfix #172 invariant) |
| `#THIS` | `sdd/moderation-automation/slice3/archive-report` (this file's Engram counterpart) |

## Spec sync actions (canonical → `openspec/specs/moderation-automation/spec.md`)

| Action | Detail |
|--------|--------|
| Capability | `moderation-automation` (existing from slice 1 obs `#223` + slice 2 obs `#231` + slice 2.1 obs `#241`). |
| Sync method | **APPEND** (NOT MOVE, NOT COPY). Rationale: slice 1 was a NEW capability (delta was the full spec, so MOVE was correct there, obs `#223`). Slices 2 + 2.1 + 3 son AMENDMENTS (delta adds new REQs to existing canonical), so APPEND preserves slice-1 REQ-1..REQ-15 + slice-2 REQ-16..REQ-21 + slice-2.1 REQ-22..REQ-31 intact. |
| ADDED | 9 requirements (REQ-32..REQ-40 in canonical numbering): Backend `GET /warnings` (REQ-32); Backend `POST /warnings/{user_id}/reset` (REQ-33); Backend `GET /stats?period=24h\|7d` (REQ-34); Frontend `GroupModerationPage` en `/groups/:id/moderation` (REQ-35); Extensiones al feature module `automation/` (REQ-36); `GroupDetailPage` link + label update (REQ-37); Action constant `RESET_WARNINGS` (REQ-38); Tests §21.1 estricto (REQ-39); Non-regression (REQ-40). |
| MODIFIED | None. Existing slice-1 REQ-1..REQ-15 + slice-2 REQ-16..REQ-21 + slice-2.1 REQ-22..REQ-31 all preserved verbatim. |
| Removed | None. |
| Audit trail | The delta spec.md (`specs/moderation-automation/spec.md`) stays inside the archive folder. `git log --follow openspec/specs/moderation-automation/spec.md` will show: slice-1's NEW entry → slice-2's APPEND (commit `2df231f`, +617) → slice-2.1's APPEND (commit `e7d0680`, +529) → this slice-3's APPEND (commit pending archive SHA, +319). The slice-1 canonical entry was a NEW file (move); slice-2 + slice-2.1 + slice-3 are modifications (append). |

## Documented deviations (verify-report `#249` §Deviations, apply-report `#248` §Deviations)

All 3 deviations are **localized and acceptable** per verify-report. None break a spec REQ.

1. **`service.go` intacto — interfaz `automationDashboardRepo` inyectada como 4to arg de `WithAutomation`**. El design #245 llamaba a delegar via `*automation.Service`; el orchestrator (en apply) introdujo un nuevo interface `automationDashboardRepo` (con `ListActiveWarningStatesByGroup` + `ResetWarningState`) inyectado directamente, evitando tocar `service.go`. **Acceptable** — mantiene la arquitectura de slices 1+2+2.1 intacta (Service no se toca; pipeline de evaluación no se toca; el grep `can_` sigue dando 0 matches en executable).

2. **`UPDATE...RETURNING` no soporta old values en Postgres → SELECT prev + UPDATE (2 roundtrips)**. El diseño propuso `RETURNING old.warning_count` pero eso no es SQL estándar (Postgres devuelve new, no old). Implementación: SELECT prev → if 0 rows → 404 NOT_FOUND sin log; sino UPDATE + log `ActionResetWarnings`. **Acceptable** — atómico desde el punto de vista del caller; race con `HandleMessage` tolerable (admin puede resetear de nuevo). `SELECT FOR UPDATE` opcional si se reporta inconsistencia.

3. **`truncated` flag via `len(warnings) == limit`** — spec (REQ-32) permitía `len == 100 && COUNT > 100` OR `SELECT COUNT(*) WHERE warning_count > 0` adicional. Implementación usa la primera (más barata, ms-scale). **Acceptable** — documentado como edge case; si el admin tiene exactamente 100 advertencias activas, `truncated=true` falsamente. Si el admin tiene 100+ advertencias, ya hay un problema más grande (reglas mal calibradas).

## Bugfix `#172` invariant verification

The `permissionOk` helper in `backend/internal/moderation/` historically read `g.BotPermissions[key]` for `can_*` keys that the group detector never populates — a known bug. Per bugfix `#172`, **all new code MUST use `g.BotStatus == groups.StatusAdministrator`**, NEVER `can_*`.

Slice 3 verification (verify-report `#249` §Correctness):
- 0 executable `can_*` matches in `backend/internal/automation/*.go` (`Select-String -Path backend/internal/automation/*.go -Pattern "can_" | Where-Object { -not $_.Line.TrimStart().StartsWith('//') }` returns 0 matches).
- 4 comment-only docs preserved (all say "NUNCA leer claves can_*"): `service.go:50` (slice 1), `autoactioner.go:85` (slice 1), `model.go:10` (slice 1), `warning_sender.go:10` (slice 2.1 NEW).
- `automation_handlers.go` doesn't need a permission check (handlers son admin-gated vía `requireAuth` desde panel, no bot-touched; slice 3 usa `actorIDFromClaims` para el log, NO `permissionOkAdmin`).
- The `automationDashboardRepo` interface (deviation #1) no introduce checks nuevos — solo encapsula acceso al repo.

**Bugfix `#172` invariant INTACT.**

## Suggestions (verify-report `#249` §Issues, SUGGESTION-level — NOT blockers)

1. **R2 — future `logs(group_id, action, created_at)` composite index**. La query `CountByActionAndGroup` puede ser lenta en grupos con miles de logs en 7d. Slice 3 NO creó el índice (decisión YAGNI en design D13 — medir con `EXPLAIN ANALYZE` en dev post-deploy). **Sugerencia**: si la query reporta >100ms en grupos típicos, abrir follow-up con migración `00009_add_logs_composite_index.sql` (`CREATE INDEX idx_logs_group_action_created ON logs (group_id, action, created_at DESC)`). Documentado en design #245 D13.

2. **Pre-existing flaky test in `PublicationsPage.test.tsx`** (`crea una publicacion multi-grupo con foto y botones`) — timeout 5s aleatorio. **Provenance**: pre-existing en `main @ e7d0680` (publications slice 3 last touched this file). NOT introduced by slice 3 (`git diff main -- frontend/src/pages/PublicationsPage.test.tsx` = EMPTY). **Sugerencia**: SUGGESTION-level cleanup item, NOT a blocker. Out of slice 3 scope.

3. **Minor apply-report self-reporting inaccuracies** (verify-report `#249` §Issues — SUGGESTION): apply-report claims `service_test.go -22 LOC` pero `git diff main` muestra 0 líneas; claims "9 nuevos tests" pero actual es 8 new + extended RequireAuth. Implementation matches design exactly; report wording was off. NOT blocking.

## Slice 3/3 context — Fase 3 (AGENTS §23) closure

Per exploration `#215` (3-slice plan + 1 sub-slice), proposal `#224` (slice 2 scope), proposal `#234` (slice 2.1 scope), and proposal `#243` (slice 3 scope), Fase 3 (AGENTS §23) ships in **3 slices + 1 sub-slice**:

| Slice | Scope | Backend LOC | Frontend LOC | Status |
|-------|-------|------------:|-------------:|--------|
| **1 — Foundation** | settings + warning_state + FloodRule + worker + logs | ~1500 | 0 | **ARCHIVED** (obs `#223`, `main @ 1b10d42`) |
| **2 — More rules + Settings UI** | AntiSpam + AntiLink + BannedWords + Settings editor | ~1241 | ~1117 | **ARCHIVED** (obs `#231`, `main @ 2df231f`) |
| **2.1 — Warning visual pre-acción** | 2 cols schema + 2 templates + WarningSender + Sección 5 frontend | ~525 | ~90 | **ARCHIVED** (obs `#241`, `main @ e7d0680`) |
| **3 — Warnings Dashboard (THIS)** | Read `user_warning_state` + logs aggregations + dashboard UI + reset manual | ~1110 | ~610 | **THIS ARCHIVE** (`main @ e7d0680+1`) |

Each slice individually exceeds the 400-line review budget — `size:exception` es el patrón establecido (11/11 consecutivos aprobados en este repo).

After merge of slice 3 to `main`, **Fase 3 — Moderación Automática (AGENTS §23) queda cerrada al 100%**. Next candidates per exploration `#215` original plan: **Fase 4 — Motor de automatizaciones** (TRIGGER → CONDITION → ACTION, modular e independiente del motor de moderación).

## Invariantes respetadas

- **§11 (NO Redis)**: 0 deps Redis; pure backend stack con in-process channel (slice 1 foundation, preserved).
- **§14 (modular backend)**: All new files en `internal/automation/` (model, repository, repository_test) y `internal/api/automation_handlers.go` + `server.go`. No leaks a otros packages. Service pipeline intacto.
- **§13.1 (goose migrations)**: **0 new migrations** (slice 3 es 100% queries sobre tablas existentes `user_warning_state`, `logs`, `users`). R2 es follow-up opcional.
- **§17.1 (no secrets)**: 0 secrets in code. `.env.example` intacto.
- **§18.1 (rate limits Telegram)**: Slice 1 worker + adapter rate limit intact. Slice 3 adds 0 new direct calls to Bot API; los 3 nuevos endpoints son admin-gated.
- **§21.1 (mock TelegramService)**: All backend automation tests usan hand-rolled fakes (`fakeAutomationDashboardRepo` es nuevo en slice 3, además de `fakeAutomationService`/`fakeAutomationGroups`/`fakeAutomationLogs` existentes). 0 Bot API real calls en tests (§21.1 audit: `grep -rn 'api\.telegram\.org\|TELEGRAM_BOT_TOKEN' backend/internal/automation/*_test.go` = 0 matches).
- **§23 (Fase 3 — this slice)**: DASHBOARD delivered; Fase 3 closed.
- **§25 regla 18 (frontend no llama Telegram)**: Frontend changes aislados a `automation/` feature module + `GroupModerationPage.tsx` (NEW) + `GroupDetailPage.tsx` (MOD). No `telegram.org` URLs en frontend code.
- **Bugfix `#172`** (permissionOkAdmin usa `BotStatus`, NEVER `can_*`): verified, 0 executable `can_*` matches en slice 3.
- **Frontend pre-existing pages intact**: `git diff main -- frontend/src/pages/` excluyendo `GroupModerationPage.tsx` (NEW) y `GroupDetailPage.tsx` (MOD) returns empty.
- **`backend/internal/moderation/` intacto**: acciones manuales (ban/unban/mute/unmute/delete/pin/lock/unlock/approve/reject) sin tocar. `git diff main -- backend/internal/moderation/` = empty.
- **`backend/internal/publications/` intacto**: `git diff main -- backend/internal/publications/` = empty.

## Full backend suite verification (verify-report `#249` §Build)

```
ok      github.com/telegram-manager/backend/internal/api                2.966s
ok      github.com/telegram-manager/backend/internal/auth               2.689s
ok      github.com/telegram-manager/backend/internal/automation         10.016s
ok      github.com/telegram-manager/backend/internal/config             1.235s
ok      github.com/telegram-manager/backend/internal/events             1.264s
ok      github.com/telegram-manager/backend/internal/groups             3.408s
ok      github.com/telegram-manager/backend/internal/joinrequests        1.795s
ok      github.com/telegram-manager/backend/internal/logs               1.962s
ok      github.com/telegram-manager/backend/internal/moderation         1.219s
ok      github.com/telegram-manager/backend/internal/publications        2.895s
ok      github.com/telegram-manager/backend/internal/telegram          22.671s
ok      github.com/telegram-manager/backend/internal/users              0.760s
```

**12/12 testable packages green**. `go vet ./...` clean. `gofmt -l .` clean.

Frontend: `npm test -- --run` → **15/15 test files pass, 87/87 tests green**. `npm run build` → green (with bundle warning >500kB pre-existing).

Non-regression:
- `git diff main -- backend/internal/automation/{service,worker,autoactioner,warning_sender,rules,templates}.go` = EMPTY (pipeline intact).
- `git diff main -- backend/internal/moderation/` = EMPTY.
- `git diff main -- backend/internal/publications/` = EMPTY.
- `git diff main -- frontend/src/pages/` excluyendo `GroupModerationPage.tsx` (NEW) y `GroupDetailPage.tsx` (MOD) = EMPTY.
- `grep can_ backend/internal/automation/*.go | grep -v '^//'` = 0 matches executable.
- `grep 'api\.telegram\.org\|TELEGRAM_BOT_TOKEN' backend/internal/automation/*_test.go` = 0 matches (§21.1 audit).

## Commit list (9 commits = 8 source + 1 archive, feat/moderation-automation-slice3, NOT pushed)

Source (8 commits, base `main @ e7d0680`):

1. `0889d9a` — `feat(logs): ActionResetWarnings + CountByActionAndGroup + tests` (Phase 1)
2. `d798ee2` — `feat(automation): dashboard repo - WarningStateRow + DisplayName + ListActiveWarningStatesByGroup + ResetWarningState` (Phase 2)
3. `03c6c44` — `feat(api): 3 dashboard handlers (warnings, reset, stats) + automationDashboardRepo injection` (Phase 3)
4. `a2ae099` — `feat(frontend): automation feature module extensions (types, api, hooks, error)` (Phase 4)
5. `f6aefe1` — `feat(frontend): GroupModerationPage + /groups/:id/moderation route + 9 tests` (Phase 5)
6. `75e2143` — `feat(frontend): GroupDetailPage dashboard link + label rename` (Phase 6)
7. `60c9cf6` — `docs: README dashboard de moderacion section` (Phase 8)
8. `6bb38a0` — `chore(openspec): apply-report for slice 3` (artifact)

Archive (1 commit): `6e3c804` — `chore(openspec): archive change moderation-automation-slice3`.

Total source: **21 files changed, +2389/-39** — matches apply-report `#248` and verify-report `#249`.

## Next steps (orchestrator)

1. **Merge `feat/moderation-automation-slice3` → `main`** (single-pr, `size:exception` aprobado per `#247`, precedent 11/11).
2. **Push to remote.**
3. **Rebuild backend in Docker** (no migration needed — slice 3 is 100% queries on existing tables). The 3 nuevos endpoints (`GET /warnings`, `POST /warnings/{user_id}/reset`, `GET /stats`) quedan disponibles inmediatamente.
4. **Rebuild frontend in Docker** to ship `GroupModerationPage` bundle.
5. **After merge**: **Fase 3 — Moderación Automática (AGENTS §23) queda cerrada al 100%**. Next candidates per exploration `#215` original plan: **Fase 4 — Motor de automatizaciones** (TRIGGER → CONDITION → ACTION, modular e independiente del motor de moderación, EN OTRO CAMBIO separado).
6. **Follow-ups opcionales** (no bloqueantes):
   - R2 — Migración `00009_add_logs_composite_index.sql` si `EXPLAIN ANALYZE` post-deploy muestra `CountByActionAndGroup` >100ms en grupos típicos (design #245 D13).
   - Pre-existing flaky test in `PublicationsPage.test.tsx` — out of slice 3 scope; cleanup en cambio separado.

## Commit (this archive)

- **SHA**: `6e3c804`
- **Message**: `chore(openspec): archive change moderation-automation-slice3`
- **Branch**: `feat/moderation-automation-slice3`
- **Files**: 1 modified (canonical spec, +319) + 1 rename (apply-report.md tracked → archive, 100% rename detected by git) + 7 new (README + exploration + proposal + design + tasks + verify-report + specs subfolder)
- **Total**: 9 files changed, 2360 insertions(+) (the rename is not counted as a deletion because git detected 100% content match)
- **NO source code touched** — solo `openspec/specs/moderation-automation/spec.md` (canónico) + el archive folder.
- **Push/Merge**: NEITHER (rule of archive phase — orchestrator handles).
- **Commit SHA**: `6e3c804`.
