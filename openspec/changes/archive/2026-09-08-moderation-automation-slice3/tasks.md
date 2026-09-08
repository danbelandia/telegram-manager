# Tasks: Moderation Automation — Slice 3 (Warnings Dashboard)

**Change**: `moderation-automation-slice3` — Slice 3/3 (último) de **Fase 3 — Moderación Automática** (AGENTS §23).
**Mode**: hybrid. Persisted as `sdd/moderation-automation/slice3/tasks`.
**Predecessors**: exploration `#242`, proposal `#243`, spec `#244`, design `#245`. Base: `main @ e7d0680` (slice 2.1 archivado).
**Strategy**: single-pr con `size:exception` (precedente 10 PRs consecutivos — obs `#220/#228/#238`). El orchestrator re-confirmará al `sdd-apply`.

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~830 across 21 files (consistente con proposal `#243` + design `#245`) |
| 400-line budget risk | High |
| Chained PRs recommended | No (size:exception ya solicitada) |
| Suggested split | single PR — `feat/moderation-automation-slice3` |
| Delivery strategy | single-pr (size:exception) |
| Chain strategy | size-exception |

Decision needed before apply: Yes
Chained PRs recommended: No
Chain strategy: size-exception
400-line budget risk: High

## Phase 1: Logs repo extension (backend, additive)

- [ ] 1.1 `backend/internal/logs/model.go` MOD: agregar `ActionResetWarnings = "RESET_WARNINGS"` con comentario "slice 3 — admin action (ActorID != nil)".
- [ ] 1.2 `backend/internal/logs/repository.go` MOD: agregar `CountByActionAndGroup(ctx, groupID int64, actions []string, since time.Time) (map[string]int, error)`. SQL: `SELECT action, COUNT(*) FROM logs WHERE group_id = $1 AND action = ANY($2) AND created_at >= $3 GROUP BY action`. Helper `scanCount`.
- [ ] 1.3 `backend/internal/logs/repository_test.go` MOD: 4 integration tests — happy path 5/2/1 RULE/AUTOMUTE/AUTOBAN + 3 BAN_USER no contados; filtro `since` (-48h vs -1h); filtro `actions` subset; empty result `map[string]int{}` no nil.
- [ ] 1.4 Verify: `cd backend && go test ./internal/logs/ -count=1` green.

## Phase 2: Automation repo extension (backend, additive)

- [ ] 2.1 `backend/internal/automation/model.go` MOD: agregar `WarningStateRow` struct (embebe `WarningState` + `FirstName string` + `Username *string`) + método `(w WarningStateRow) DisplayName() string` con fallback `FirstName ?? "@"+*Username ?? "user {id}"`.
- [ ] 2.2 `backend/internal/automation/repository.go` MOD: agregar `ListActiveWarningStatesByGroup(ctx, groupID int64, limit int) ([]WarningStateRow, error)` (LEFT JOIN users + WHERE count > 0 + ORDER BY count DESC, last_warning_at DESC NULLS LAST + LIMIT) y `ResetWarningState(ctx, groupID, userID int64) (oldCount int, err error)` (`UPDATE ... SET warning_count=0, last_warning_at=NULL, last_action_at=NULL, expires_at=NULL WHERE group_id=$1 AND user_id=$2 RETURNING old_warning_count`).
- [ ] 2.3 `backend/internal/automation/repository_test.go` MOD: 4 integration tests — filtra count=0; LEFT JOIN preserva fila sin users (FirstName="" Username=nil); orden count DESC + last_warning_at NULLS LAST; ResetWarningState retorna oldCount + setea 0.
- [ ] 2.4 Verify: `cd backend && go test ./internal/automation/ -count=1` green.

## Phase 3: Backend handlers + server routes + WithAutomation extension

- [ ] 3.1 `backend/internal/api/automation_handlers.go` MOD: extender interface `automationLogWriter` a `logStoreReader` con `CountByActionAndGroup` (mantiene cohesión, una sola injection); agregar `handleListWarnings`, `handleResetWarning`, `handleGetStats` (~130 LOC). Validar `period` whitelist `["24h","7d"]` default `24h` → 400 si otro. `handleResetWarning` emite log `ActionResetWarnings` con `ActorID=admin` y metadata `{user_id, warning_count_before_reset}` (usa `oldCount` del repo).
- [ ] 3.2 `backend/internal/api/server.go` MOD: agregar 3 `mux.HandleFunc` dentro de `WithAutomation` para `GET .../automation/warnings`, `POST .../automation/warnings/{user_id}/reset`, `GET .../automation/stats`.
- [ ] 3.3 `backend/cmd/server/main.go` MOD: extender la inyección de `WithAutomation` para pasar `logsRepo` (interface `logStoreReader`) si no lo recibe ya — verificar firma actual; ajustar si es necesario.
- [ ] 3.4 `backend/internal/api/automation_handlers_test.go` MOD: 6+ tests — GET /warnings 200 con LEFT JOIN shape visible; GET /warnings 404 automation nil + grupo inexistente; POST reset 200 + log emitido con metadata + ActorID del admin; POST reset 404 si fila no existe; GET /stats period=24h default; GET /stats period=foo → 400; auth 401 en los 3 endpoints.
- [ ] 3.5 Verify: `cd backend && go test ./... -count=1 && go vet ./... && gofmt -l .` clean.

## Phase 4: Frontend feature module `automation/` (types + api + hooks + error)

- [ ] 4.1 `frontend/src/features/automation/types.ts` MOD: agregar `StatsPeriod = "24h"|"7d"`, `AutomationStats {rule_triggered, automute, autoban, period}`, `WarningStateRow {user_id, display_name, username, warning_count, last_warning_at, last_action_at, expires_at}`, `WarningsResponse {warnings, truncated}`.
- [ ] 4.2 `frontend/src/features/automation/api.ts` MOD: agregar `listWarnings(groupId)`, `resetWarning(groupId, userId)`, `getStats(groupId, period)` con pattern existente.
- [ ] 4.3 `frontend/src/features/automation/hooks.ts` MOD: agregar `useWarnings(groupId)` (queryKey `['automation','warnings',groupId]`), `useResetWarning(groupId)` (mutation con `onSuccess` invalidando `['automation','warnings',groupId]` + `['automation','stats',groupId,*]`), `useStats(groupId, period)` (queryKey `['automation','stats',groupId,period]`). Sin `refetchInterval`.
- [ ] 4.4 `frontend/src/features/automation/error.ts` MOD: ramas `404 NOT_FOUND` → "no hay advertencias activas" y `400 VALIDATION_ERROR` → mensaje backend o "parámetros inválidos".
- [ ] 4.5 `frontend/src/test/helpers.tsx` MOD: extender `mockFetchRoutes` con substring routes nuevas (`/automation/warnings`, `/automation/stats`) solo si tests existentes lo requieren; verificar primero si pasa sin cambios.
- [ ] 4.6 Verify: `cd frontend && npm test -- --run src/features/automation/` green si tests existen (sino skip).

## Phase 5: Frontend `GroupModerationPage` + ruta `/groups/:id/moderation`

- [ ] 5.1 `frontend/src/pages/GroupModerationPage.tsx` NEW (~280 LOC): 2 secciones sin Save button. Stats (3 Card + Select 24h/7d + Button Refresh invalida 2 queryKeys). Warnings (Table con User display_name+user_id dimmed, Warnings Badge, Última advertencia Intl.RelativeTimeFormat es-AR, Acciones Button Reset con `data-testid="warning-reset-{user_id}"`). Empty state Text dimmed. Truncation Alert amarillo si `truncated=true`. Reset abre Modal Mantine v7 ("¿Resetear advertencias de {display_name}? Esto no desmutea al usuario.") con Cancelar/Resetear.
- [ ] 5.2 `frontend/src/pages/GroupModerationPage.test.tsx` NEW (~220 LOC): 7+ tests — render inicial stats 12/3/1 + 2 warnings; period 7d cambia fetch; empty state; truncation alert; refresh invalida 2 queries; reset modal abre/acepta/cancela; display name fallback; error backend notifyError.
- [ ] 5.3 `frontend/src/App.tsx` MOD: import `GroupModerationPage` + Route `/groups/:id/moderation` con `RequireAuth` dentro del Route padre existente.
- [ ] 5.4 Verify: `cd frontend && npm test -- --run src/pages/GroupModerationPage.test.tsx && npm run build` green.

## Phase 6: `GroupDetailPage` link + label update

- [ ] 6.1 `frontend/src/pages/GroupDetailPage.tsx` MOD: rename "Configurar automatización" → "Configurar reglas de moderación" (línea ~217) + nuevo Button `component={Link} variant="light" w={260} data-testid="moderation-dashboard-link" to={\`/groups/${groupId}/moderation\`}` debajo con texto "Ver dashboard de moderación".
- [ ] 6.2 `frontend/src/pages/GroupDetailPage.test.tsx` MOD: actualizar test existente del label + nuevo test que verifica `getByTestId("moderation-dashboard-link")` con href `/groups/123/moderation` y texto actualizado.
- [ ] 6.3 Verify: `cd frontend && npm test -- --run` green.

## Phase 7: Full suite + non-regression

- [ ] 7.1 Backend: `cd backend && go test ./... -count=1 && go vet ./... && gofmt -l .` clean (cero output de gofmt).
- [ ] 7.2 Frontend: `cd frontend && npm test -- --run && npm run build` clean.
- [ ] 7.3 Non-regression pipeline: `git diff main -- backend/internal/automation/service.go backend/internal/automation/worker.go backend/internal/automation/autoactioner.go backend/internal/automation/warning_sender.go backend/internal/automation/rules.go backend/internal/automation/templates.go` = empty.
- [ ] 7.4 Non-regression `moderation/`: `git diff main -- backend/internal/moderation/` = empty.
- [ ] 7.5 Non-regression publications: `git diff main -- backend/internal/publications/` = empty.
- [ ] 7.6 Non-regression pages frontend: `git diff main -- frontend/src/pages/ LoginPage DashboardPage GroupsPage GroupAutomationPage PublicationsPage GroupUsersPage GroupRequestsPage GroupLogsPage GroupDetailPage.tsx` = solo cambios en `GroupDetailPage.tsx`.
- [ ] 7.7 Invariante bugfix #172: `grep -rn 'can_' backend/internal/automation/*.go | grep -v '^//'` = 0 matches executable.
- [ ] 7.8 §21.1 audit: `grep -rn 'TELEGRAM_BOT_TOKEN\|api.telegram.org\|bot\.telegram' backend/internal/automation/*_test.go` = 0 calls a Bot API real en tests.
- [ ] 7.9 §25 sin secretos: `git diff main` busca `TELEGRAM_BOT_TOKEN`, `JWT_SECRET`, passwords hardcoded = 0 matches.

## Phase 8: Docs + commits

- [ ] 8.1 `README.md` MOD: agregar sección "Dashboard de moderación" (~12 líneas) bajo "Moderación automática" — ubicación `/groups/:id/moderation`, qué muestra (stats 24h/7d + advertencias activas), cómo usar el reset manual con advertencia "no desmutea al usuario".
- [ ] 8.2 Commits conventional siguiendo estilo del repo: un commit por phase lógico (`feat(automation): slice3 logs repo + tests`, `feat(automation): slice3 automation repo + tests`, `feat(api): slice3 handlers + routes`, `feat(automation): slice3 frontend feature module`, `feat(ui): slice3 GroupModerationPage + route`, `feat(ui): slice3 GroupDetailPage link + label`, `docs: slice3 dashboard de moderación`). NO "Co-Authored-By" ni AI attribution.
- [ ] 8.3 Branch: `feat/moderation-automation-slice3` base `main @ e7d0680`. PR único al `main` con descripción que linkea este tasks.md + design.md + proposal.md.

## Dependency Graph

```
Phase 1 (logs repo)        ──┐
                              ├──→ Phase 3 (handlers) ──┐
Phase 2 (automation repo)  ──┘                         │
                                                      ├──→ Phase 7 (full suite)
Phase 4 (frontend feature) ──┐                         │
                              ├──→ Phase 5 (page) ────┤
                              │                       │
                  (Phase 6 GroupDetailPage ──────────┘)

Phase 8 (docs + commits) ←── Phase 7 verified
```

## Out-of-Scope Reminders (consistente con proposal `#243`)

- Edición de reglas / umbrales desde el dashboard (sigue en `/groups/:id/automation`).
- Auto-poll / refetchInterval (YAGNI).
- CSV export, paginación, stats por regla, SELECT FOR UPDATE (todas descartadas en design D10-D13).
- Migración nueva (slice 3 es 100% queries sobre tablas existentes).

## Open Questions for Apply

- **R2 (índice `logs(group_id, action, created_at)`)**: NO crear índice en este slice. Si `EXPLAIN ANALYZE` post-deploy muestra >100ms en grupos típicos, abrir follow-up con migración `0009`. Documentar este punto en el `apply-report`.

## What / Why / Where / Learned

**What**: 8 phases de implementación secuencial para slice 3 — logs repo (Phase 1), automation repo (Phase 2), handlers+routes (Phase 3), frontend feature module (Phase 4), GroupModerationPage + ruta (Phase 5), GroupDetailPage link+rename (Phase 6), full suite + non-regression (Phase 7), docs+commits (Phase 8).

**Why**: Slices 1+2+2.1 producen todos los datos que el admin necesita supervisar (`user_warning_state` + logs automation); slice 3 cierra el ciclo de feedback con 3 endpoints read-only + 1 reset opcional + 1 página de observación.

**Where**:
- `openspec/changes/moderation-automation/slice3/tasks.md` (este archivo)
- Engram `sdd/moderation-automation/slice3/tasks`

**Learned**:
- Forecast ~830 LOC excede budget 400 → `size:exception` consistente con precedente 10 PRs consecutivos (obs `#220/#228/#238`).
- Bugfix `#172` invariante: slice 3 NO introduce checks `can_*`; verificación en Phase 7.7.
- 12° + 13° endpoints en `WithAutomation` (file pasa de 572 a ~720 LOC) — manejable.
- `UPDATE...RETURNING` evita `SELECT FOR UPDATE` manteniendo atomicidad (D12 del design).
- `display_name` calculado server-side via `WarningStateRow.DisplayName()` (D14) — frontend recibe string listo.
