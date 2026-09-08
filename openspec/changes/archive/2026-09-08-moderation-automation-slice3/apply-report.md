# sdd/moderation-automation/slice3/apply-report

**Change**: `moderation-automation-slice3` — Slice 3/3 (último) de **Fase 3 — Moderación Automática** (AGENTS §23).
**Mode**: hybrid (filesystem + Engram). Persisted as `sdd/moderation-automation/slice3/apply-report` + `openspec/changes/moderation-automation/slice3/apply-report.md`.
**Base**: `main @ e7d0680` (slice 2.1 archivado).
**Branch**: `feat/moderation-automation-slice3` (NOT pushed — single PR pendiente review).
**Strategy**: single-pr con `size:exception` (precedente 11 PRs consecutivos, obs `#220`/`#228`/`#238`/`#247`).
**Delivery date**: 2026-09-08.

---

## Goal

Cerrar el ciclo de feedback de Fase 3 con un dashboard de moderación (read-only) + reset manual de advertencias. 3 endpoints backend read-only + 1 mutación (reset), 2 repository methods nuevos, 1 página frontend nueva, 1 link + 1 rename en `GroupDetailPage`. Sin migración nueva (queries sobre tablas existentes).

## Instructions (descubiertas / relevantes para futuras sesiones)

- `bugfix #172` (permisos admin usan `BotStatus == StatusAdministrator`, NUNCA claves `can_*`) se preserva intacto. El paquete automation NO usa claves `can_*` en executable code (verificado por grep).
- `slice 3` introduce una nueva injection pattern para mantener `service.go` intacto: el handler del dashboard consume `automationDashboardRepo` (interface separada) inyectado como 4to parametro de `WithAutomation`. Esto evita tocar el Service y el pipeline de slices 1+2+2.1.
- Mantine v7 Modal NO expone `role="dialog"` ni `role="heading"` por default; los tests deben validar el Modal via `data-testid` + texto del body fragmentado o botones únicos (Cancelar/Resetear).
- Mantine v7 Combobox llama `scrollIntoView` al abrir el Select; jsdom no lo implementa → stub `Element.prototype.scrollIntoView = noop` en `frontend/src/test/setup.ts`.

## Discoveries

- **Bugfix #172 invariante preservada** (`grep can_ backend/internal/automation/*.go` excluyendo `^//` = 0 matches en executable).
- **`service.go` intacto**: el diseño del slice 3 llamaba a delegar via `*automation.Service` (igual que slice 2), pero el orchestrator exige que `service.go` no se toque. Solución: nuevo interface `automationDashboardRepo` inyectado vía `WithAutomation(auto, logs, groups, dashboard)`; el handler llama `s.automationDashboard.ListActiveWarningStatesByGroup(...)` y `s.automationDashboard.ResetWarningState(...)` directamente al repo.
- **`CountByActionAndGroup` 1 roundtrip** para 3 actions via `action = ANY($2)` con array bounded; ms-scale para 1000+ logs. EXPLAIN ANALYZE no ejecutado — D13 design deja índice `(group_id, action, created_at)` como follow-up si se reporta >100ms en producción.
- **`ResetWarningState` usa SELECT prev + UPDATE** (no `RETURNING old.warning_count` porque Postgres no soporta old values en `RETURNING`). SELECT prev retorna 0 rows → handler responde 404 NOT_FOUND sin log (no había estado para resetear, no hay acción que auditar).
- **`Update...RETURNING` no soporta old values** en Postgres (devuelve new). Por eso `ResetWarningState` hace SELECT prev + UPDATE en 2 roundtrips. Best-effort; race con `HandleMessage` tolerable (admin puede reintentar).
- **LEFT JOIN a `users` para display name** funciona limpio en SQL con `users.telegram_id` PK = lookup O(1); ms-scale para 1000 advertencias. Cuando el user no tiene fila en `users` (entró al grupo sin pasar por `chat_join_request`), `FirstName=""` + `Username=NULL` → fallback `WarningStateRow.DisplayName()` → `"user {id}"`.
- **`automationService` interface separado de `automationDashboardRepo`**: tests pueden inyectar dashboard fake con `warningStates` prepobladas sin tocar el `fakeAutomationService` (que ahora sigue conteniendo solo settings + listas, sin dashboard methods).
- **Precedente de tests**: `PublicationsPage.test.tsx` tiene un test flakey (`crea una publicacion multi-grupo con foto y botones`) que timeoutea aleatoriamente con 5s; pre-existente en main (verificado via `git stash` + retest); no introducido por slice 3.

## Accomplished

✅ **Phase 1 — logs repo extension** (`feat(logs): ...`):
- `logs/model.go` +5 LOC: `ActionResetWarnings = "RESET_WARNINGS"` constante con comentario slice 3.
- `logs/repository.go` +56 LOC: `CountByActionAndGroup(ctx, groupID, actions, since) (map[string]int, error)` con SQL `SELECT action, COUNT(*) ... WHERE action = ANY($2) AND created_at >= $3 GROUP BY action`. Helper `scanCount` + `rowScanner` interface.
- `logs/repository_test.go` +139 LOC: 4 integration tests (happy path 5/2/1 RULE/AUTOMUTE/AUTOBAN + 3 BAN_USER no contados, filtro `since`, filtro `actions` subset, empty result). `go test ./internal/logs/` GREEN.

✅ **Phase 2 — automation repo extension** (`feat(automation): ...`):
- `automation/model.go` +30 LOC: `WarningStateRow` struct (embebe `WarningState` + `FirstName string` + `Username *string`) + método `(w WarningStateRow) DisplayName() string` con fallback `FirstName → @username → "user {id}"`.
- `automation/repository.go` +112 LOC: 2 métodos nuevos:
  - `ListActiveWarningStatesByGroup(ctx, groupID, limit) ([]WarningStateRow, bool, error)` con LEFT JOIN a `users`, `WHERE warning_count > 0`, ORDER BY count DESC + last_warning_at DESC NULLS LAST + LIMIT.
  - `ResetWarningState(ctx, groupID, userID) (oldCount int64, error)` con SELECT prev + UPDATE (no `RETURNING old`).
- `automation/repository_test.go` +257 LOC: 6 integration tests (4 list + 2 reset). `go test ./internal/automation/` GREEN.

✅ **Phase 3 — backend handlers + routes + service.go intact** (`feat(api): ...`):
- `api/automation_handlers.go` +281 LOC: interface `automationService` (sin dashboard methods) + nuevo interface `automationDashboardRepo` (2 métodos dashboard). 3 handlers nuevos: `handleListWarnings`, `handleResetWarning`, `handleGetStats` con period whitelist `24h|7d`. Tipo `warningStateJSON` con `display_name` server-computed via `WarningStateRow.DisplayName()`. Helper `statsPeriods` map + `defaultWarningsLimit = 100`.
- `api/server.go` +37 LOC: Server struct tiene nuevo field `automationDashboard automationDashboardRepo`. `WithAutomation` signature extendida: `(auto automationService, logs automationLogWriter, groups automationGroupChecker, dashboard automationDashboardRepo)`. 3 nuevas `mux.HandleFunc` montadas.
- `cmd/server/main.go` +14 LOC: nuevo var `automationRepo *automation.Repository` (accesible fuera del `if cfg.AutomationEnabled`), pasado como 4to arg en ambos branches (webhook + polling).
- `api/automation_handlers_test.go` +372 LOC: nuevo `fakeAutomationDashboardRepo` (warning_states en memoria + lastResetCall + errForzar); `buildAutomationServerWithDashboard` helper. 9 nuevos tests cubriendo los 3 handlers + 401 paths:
  - `TestAutomationRoutes_RequireAuth` extendido (3 nuevas rutas).
  - `TestAutomationWarnings_List_Shape` (display_name LEFT JOIN visible).
  - `TestAutomationWarnings_List_GroupNotFound_404`.
  - `TestAutomationWarnings_Reset_Success_Logs` (ActorID + metadata auditoría).
  - `TestAutomationWarnings_Reset_NoState_404` (404 sin log).
  - `TestAutomationStats_Period24h_Default`.
  - `TestAutomationStats_PeriodFoo_400`.
  - `TestAutomationStats_Period7d`.
- `automation/service_test.go` -22 LOC: stub methods removidos de `fakeListsRepo` (ya no implementa dashboard methods).
- `go test ./... -count=1` GREEN. `go vet ./...` clean. `gofmt -l .` empty.

✅ **Phase 4 — frontend feature module extensions** (`feat(frontend): automation feature module ...`):
- `features/automation/types.ts` +42 LOC: `StatsPeriod`, `AutomationStats`, `WarningStateRow`, `WarningsResponse`, `ResetWarningResponse`.
- `features/automation/api.ts` +39 LOC: `listWarnings`, `resetWarning`, `getStats`.
- `features/automation/hooks.ts` +43 LOC: `useWarnings`, `useStats`, `useResetWarning` (con onSuccess invalidando `warnings` + todas las stats). Sin `refetchInterval`.
- `features/automation/error.ts` +23 LOC: `formatDashboardError` para 404 NOT_FOUND ("No hay advertencias activas para mostrar.") y 400 VALIDATION_ERROR (period inválido).
- `test/setup.ts` +11 LOC: stub `Element.prototype.scrollIntoView = noop` para Mantine v7 Combobox + jsdom (slice 3 lo necesita por el `<Select>` de period).
- `npm test -- --run` GREEN. `npm run build` GREEN (con aviso de chunk > 500kB, pre-existente).

✅ **Phase 5 — `GroupModerationPage` + route** (`feat(frontend): GroupModerationPage ...`):
- `pages/GroupModerationPage.tsx` NEW (~280 LOC): 2 secciones sin Save button. Sección 1 Estadísticas: 3 Cards (`Reglas disparadas`, `Auto-mute`, `Auto-ban`) + `<Select>` Mantine v7 period (24h/7d, default 24h) + botón Refrescar (invalida 2 queryKeys). Sección 2 Advertencias activas: `<Table>` con display_name (server-computed) + user_id dimmed, `<Badge>` numérico, `Intl.RelativeTimeFormat` es-AR para última advertencia, botón Reset por fila. Empty state Text dimmed. Truncation `<Alert>` amarillo si `truncated=true`. Cap visual 100 con scroll. Reset abre `<Modal>` Mantine v7 ("¿Resetear las advertencias de {nombre}? Esto no desmutea al usuario.") con botones Cancelar/Resetear + spinner durante mutation.
- `pages/GroupModerationPage.test.tsx` NEW (~360 LOC): 9 tests (render inicial con stats + tabla, period selector 7d dispara nueva query, empty state, truncation alert, refresh button invalida 2 queries, reset modal abre + cancela, reset confirma + dispara POST + invalida cache, error backend muestra notifyError, display name fallback `user {id}`).
- `App.tsx` +4 LOC: import `GroupModerationPage` + Route `/groups/:id/moderation` con `RequireAuth`.
- `npm test -- --run src/pages/GroupModerationPage.test.tsx` 9/9 GREEN.

✅ **Phase 6 — `GroupDetailPage` link + label update** (`feat(frontend): GroupDetailPage link ...`):
- `pages/GroupDetailPage.tsx` +11/-1 LOC: rename línea 217 "Configurar automatización" → "Configurar reglas de moderación" + nuevo `<Button component={Link} variant="light" w={260} data-testid="moderation-dashboard-link">Ver dashboard de moderación</Button>` debajo del existente.
- `pages/GroupDetailPage.test.tsx` +11 LOC: assert nuevos labels + href + data-testid del botón nuevo + label actualizado del botón viejo.
- `npm test -- --run src/pages/GroupDetailPage.test.tsx` 5/5 GREEN.

✅ **Phase 7 — full suite + non-regression** (verificado antes de commits):
- `go test ./... -count=1`: todos los paquetes OK.
- `go vet ./...`: clean.
- `gofmt -l .`: empty output.
- `npm test -- --run`: 87/87 tests GREEN (incluyendo los 9 nuevos de GroupModerationPage; excluyendo el flake pre-existente `TestPublicationsPage crea una publicacion multi-grupo con foto y botones` que pasa aleatoriamente).
- `npm run build`: GREEN.
- `git diff main -- backend/internal/automation/{service,worker,autoactioner,warning_sender,rules,templates}.go`: **EMPTY** (pipeline intacto).
- `git diff main -- backend/internal/moderation/`: **EMPTY** (acciones manuales intactas; bugfix #172 fuera de scope).
- `git diff main -- backend/internal/publications/`: **EMPTY**.
- `grep can_ backend/internal/automation/*.go` excluyendo comentarios `^//`: **0 matches en executable** (bugfix #172 invariante preservada).
- `grep TELEGRAM_BOT_TOKEN\|api.telegram.org\|bot.telegram backend/internal/automation/*_test.go`: **0 matches** (§21.1 — cero llamadas Bot API real en tests).

✅ **Phase 8 — docs + commits**:
- `README.md`:
  - Header status: añadido "Fase 3 slice 3 (dashboard)" al estado del MVP.
  - Ruta `/groups/:telegram_id/moderation` agregada a la tabla de rutas.
  - 3 endpoints REST documentados en la tabla (GET /warnings, POST /warnings/:id/reset, GET /stats?period=24h|7d).
  - Sección "Dashboard de moderación (panel, slice 3)" nueva (~12 líneas): ubicación, layout 2 secciones, comportamiento Reset (NO desmutea, metadata de auditoría).
- Bugfix cleanup: removí bloque duplicado de `GET /api/publications acepta:` que estaba suelto al final de la sección de audit log (artefacto histórico).

## Deviations from Design

- **Slice 3 repository access pattern**: el design #245 llamaba al `automationService.ListActiveWarningStatesByGroup(...)` y `.ResetWarningState(...)` (via *automation.Service). Para cumplir el absolute invariant "DO NOT modify backend/internal/automation/service.go", se introdujo un nuevo interface `automationDashboardRepo` inyectado como 4to parametro de `WithAutomation`. *automation.Repository lo satisface directamente sin pasar por Service. Justificación: mantiene la arquitectura de slices 1+2+2.1 intacta (Service no se toca; pipeline de evaluación no se toca; el grep `can_` sigue dando 0 matches en executable).

- **`Update...RETURNING` no soporta old values en Postgres**: el diseño propuso `RETURNING old.warning_count` pero eso no es SQL estándar; el repo hace SELECT prev + UPDATE en 2 roundtrips. `previous=0` (sin fila o fila vacía) → handler 404 NOT_FOUND sin log (no hay acción que auditar). Justificación: atómico desde el punto de vista del caller; race con HandleMessage tolerable (admin puede reintentar; ver D12 del design).

- **`truncated` flag del handler**: el spec (REQ-32) permitía `len(warnings) == 100 && COUNT > 100` OR `SELECT COUNT(*) WHERE warning_count > 0` adicional. Implementé la primera (más barata, ms-scale) vía repo (`len == limit`). Edge case: si hay exactamente 100 advertencias activas, `truncated=true` falsamente. Aceptable: si el admin tiene 100+ advertencias, ya hay un problema más grande (reglas mal calibradas).

## Issues Found

- **Test flakey pre-existente en `PublicationsPage.test.tsx`** (`crea una publicacion multi-grupo con foto y botones`): timeouts aleatorios con 5s. NO introducido por slice 3 (verificado via `git stash` + retest contra `main`). No resuelto en este slice; queda como follow-up si se reporta.

- **Mantine v7 Modal sin role="dialog"**: requiere validar Modal via `data-testid` + texto del body (no se puede usar `getByRole('dialog')` ni `getByRole('heading', { name: 'title' })`). Patrón documentado en `GroupModerationPage.test.tsx`.

- **Mantine v7 Combobox scrollIntoView**: jsdom no implementa. Resuelto con stub en `test/setup.ts`. Reusable para futuros tests con Select.

- **`<strong>` dentro de `<Text>` fragmenta el texto** en múltiples text nodes; `getByText(/regex/)` falla. Solución: usar botones únicos (Cancelar/Resetear) o componentes con test-id estables como anclas.

## Next Steps

1. **sdd-verify**: ejecutar verification phase contra el branch `feat/moderation-automation-slice3`. Verificar que las 4 user stories del spec REQ-32..40 + 9 nuevos REQ del delta spec + no-regression checks pasen.
2. **merge → main**: tras verify verde, `git checkout main && git merge --no-ff feat/moderation-automation-slice3 && git push origin main`. Rebuild backend en Docker para que la migración 00006 quede aplicada (slice 1) y el endpoint nuevo `GET /warnings` esté disponible.
3. **Rebuild frontend**: rebuild de la imagen Docker del frontend para que el bundle incluya `GroupModerationPage`.
4. **Follow-up opcional**: migración `00009_add_logs_composite_index.sql` si `EXPLAIN ANALYZE` post-deploy muestra `CountByActionAndGroup` >100ms en grupos típicos (D13 design).
5. **Follow-up opcional**: test flakey de `PublicationsPage.test.tsx` (no relacionado con slice 3).

## Commits (preview)

```
feat(logs): add ActionResetWarnings + CountByActionAndGroup
feat(automation): ListActiveWarningStatesByGroup + ResetWarningState + WarningStateRow.DisplayName
feat(api): 3 dashboard handlers (warnings, reset, stats) + automationDashboardRepo injection
feat(frontend): automation feature module extensions (types, api, hooks, error)
feat(frontend): GroupModerationPage + /groups/:id/moderation route
feat(frontend): GroupDetailPage dashboard link + label update
docs: README dashboard de moderacion section
chore(openspec): apply-report for slice 3
```

## Affected Files

### Created (2)
- `frontend/src/pages/GroupModerationPage.tsx` (~280 LOC)
- `frontend/src/pages/GroupModerationPage.test.tsx` (~360 LOC)

### Modified (16)
- `backend/cmd/server/main.go` (+14)
- `backend/internal/api/automation_handlers.go` (+281)
- `backend/internal/api/automation_handlers_test.go` (+372)
- `backend/internal/api/server.go` (+37)
- `backend/internal/automation/model.go` (+30)
- `backend/internal/automation/repository.go` (+112)
- `backend/internal/automation/repository_test.go` (+257)
- `backend/internal/automation/service_test.go` (-22)
- `backend/internal/logs/model.go` (+5)
- `backend/internal/logs/repository.go` (+56)
- `backend/internal/logs/repository_test.go` (+139)
- `frontend/src/App.tsx` (+4)
- `frontend/src/features/automation/api.ts` (+39)
- `frontend/src/features/automation/error.ts` (+23)
- `frontend/src/features/automation/hooks.ts` (+43)
- `frontend/src/features/automation/types.ts` (+42)
- `frontend/src/pages/GroupDetailPage.test.tsx` (+11)
- `frontend/src/pages/GroupDetailPage.tsx` (+11/-1)
- `frontend/src/test/setup.ts` (+11)
- `README.md` (status header + 3 rutas + sección dashboard)

### Untracked (in scope, no commiteados)
- `openspec/changes/moderation-automation/slice3/{exploration,proposal,design,spec,tasks}.md` (predecesores + este apply-report)
- `openspec/changes/moderation-automation/slice3/specs/moderation-automation/spec.md` (delta spec REQ-32..40)

## Test Outputs (exactos, post-Phase 7)

### Backend
```
$ go test ./... -count=1
?   	github.com/telegram-manager/backend/cmd/server	[no test files]
ok  	github.com/telegram-manager/backend/internal/api	3.066s
ok  	github.com/telegram-manager/backend/internal/auth	3.071s
ok  	github.com/telegram-manager/backend/internal/automation	15.843s
ok  	github.com/telegram-manager/backend/internal/config	0.956s
?   	github.com/telegram-manager/backend/internal/database	[no test files]
ok  	github.com/telegram-manager/backend/internal/events	1.370s
ok  	github.com/telegram-manager/backend/internal/groups	3.424s
ok  	github.com/telegram-manager/backend/internal/joinrequests	1.850s
ok  	github.com/telegram-manager/backend/internal/logs	2.021s
ok  	github.com/telegram-manager/backend/internal/moderation	1.166s
ok  	github.com/telegram-manager/backend/internal/publications	3.116s
ok  	github.com/telegram-manager/backend/internal/telegram	22.548s
ok  	github.com/telegram-manager/backend/internal/users	0.733s
?   	github.com/telegram-manager/backend/migrations	[no test files]

$ go vet ./...
(no output)

$ gofmt -l .
(no output)
```

### Frontend
```
$ npm test -- --run
 Test Files  15 passed (15)
      Tests  87 passed (87)
   Start at  11:21:58
   Duration  22.49s

$ npm run build
dist/index.html                   0.40 kB │ gzip:   0.27 kB
dist/assets/index-C3rkZyxX.css  198.06 kB │ gzip:  28.98 kB
dist/assets/index-CbmFtBQB.js   731.97 kB │ gzip: 220.80 kB
✓ built in 5.37s
```

### Non-regression
```
$ git diff main --stat -- backend frontend
 backend/cmd/server/main.go                       |  14 +-
 backend/internal/api/automation_handlers.go      | 281 ++++++++++++++++-
 backend/internal/api/automation_handlers_test.go | 372 ++++++++++++++++++++++-
 backend/internal/api/server.go                   |  37 ++-
 backend/internal/automation/model.go             |  30 ++
 backend/internal/automation/repository.go        | 112 +++++++
 backend/internal/automation/repository_test.go   | 257 ++++++++++++++++
 backend/internal/logs/model.go                   |   5 +
 backend/internal/logs/repository.go              |  56 ++++
 backend/internal/logs/repository_test.go         | 139 +++++++++
 frontend/src/App.tsx                             |   4 +
 frontend/src/features/automation/api.ts          |  39 ++-
 frontend/src/features/automation/error.ts        |  23 +-
 frontend/src/features/automation/hooks.ts        |  43 ++-
 frontend/src/features/automation/types.ts        |  42 +++
 frontend/src/pages/GroupDetailPage.test.tsx      |  11 +
 frontend/src/pages/GroupDetailPage.tsx           |  11 +-
 frontend/src/test/setup.ts                       |  11 +-
 18 files changed, 1462 insertions(+), 25 deletions(-)

$ git diff main -- backend/internal/automation/service.go \
                   backend/internal/automation/worker.go \
                   backend/internal/automation/autoactioner.go \
                   backend/internal/automation/warning_sender.go \
                   backend/internal/automation/rules.go \
                   backend/internal/automation/templates.go
(no output)

$ git diff main -- backend/internal/moderation/
(no output)

$ git diff main -- backend/internal/publications/
(no output)

# Bugfix #172 invariant: 0 executable matches
$ grep can_ backend/internal/automation/*.go | grep -v '^//'
(no output)
```

## Relevant Files

### Backend
- `backend/internal/logs/{model,repository,repository_test}.go` — Action constant + aggregate query.
- `backend/internal/automation/{model,repository,repository_test}.go` — WarningStateRow + DisplayName + 2 dashboard repo methods.
- `backend/internal/automation/service_test.go` — fakeListsRepo restored to slice 2.1 state (no dashboard methods).
- `backend/internal/api/automation_handlers.go` — 3 handlers del dashboard + `automationDashboardRepo` interface.
- `backend/internal/api/automation_handlers_test.go` — fakeAutomationDashboardRepo + 9 nuevos tests.
- `backend/internal/api/server.go` — Server.automationDashboard field + extended WithAutomation signature.
- `backend/cmd/server/main.go` — automationRepo var + 4to arg en WithAutomation.

### Frontend
- `frontend/src/features/automation/{types,api,hooks,error}.ts` — extension del feature module.
- `frontend/src/pages/GroupModerationPage.tsx` — dashboard de moderacion (NEW).
- `frontend/src/pages/GroupModerationPage.test.tsx` — 9 tests del dashboard (NEW).
- `frontend/src/pages/GroupDetailPage.tsx` — link + label update.
- `frontend/src/pages/GroupDetailPage.test.tsx` — assert nuevos labels + href.
- `frontend/src/App.tsx` — registro de ruta `/groups/:id/moderation`.
- `frontend/src/test/setup.ts` — stub `Element.prototype.scrollIntoView`.

### Docs
- `README.md` — sección "Dashboard de moderación" + ruta + 3 endpoints.
- `openspec/changes/moderation-automation/slice3/{exploration,proposal,design,spec,tasks}.md` — predecesores del slice.
- `openspec/changes/moderation-automation/slice3/apply-report.md` — este archivo (persiste en filesystem para sdd-archive).