# Proposal: Moderation Automation — Slice 3 (Warnings Dashboard UI + Auto-Action Stats + Manual Reset)

> **Change**: `moderation-automation-slice3` — Slice 3/3 (último) de **Fase 3 — Moderación Automática** (AGENTS §23).
> **Mode**: hybrid (filesystem + Engram).
> **Predecessor**: exploration `#242` (`sdd/moderation-automation/slice3/exploration`); slice 2.1 archivado (`main @ e7d0680`, obs `#241`).
> **Authority**: bugfix `#172` (`permissionOkAdmin` usa `BotStatus == StatusAdministrator`, NUNCA `can_*`); AGENTS §11 (no Redis), §14 (modular backend), §17.1 (sin secretos), §18.1 (rate limit Telegram), §21.1 (mock TelegramService).
> **Strategy**: single-pr con `size:exception` (precedente 10 PRs consecutivos — `tasks` re-confirmará).
> **Branch base**: `main @ e7d0680` (slice 2.1 ya mergeado).
> **Forecast**: ~830 LOC (excede budget 400 → `size:exception`).

---

## Intent

Slices 1+2+2.1 producen **todos los datos** que el admin necesita para supervisar la moderación automática: `user_warning_state` (counter por `(group, user)`, `last_warning_at`, `last_action_at`) y logs con 4 acciones (`RULE_TRIGGERED`, `AUTOMUTE_USER`, `AUTOBAN_USER`, `WARN_USER_SENT`). Hoy el admin no tiene forma práctica de consultarlos desde el panel: tendría que correr SQL o leer logs crudos para saber **quién está cerca del threshold**, **cuánto se está moderando** en 24h/7d, o **resetear manualmente** el counter de un user problemático (feature documentada como follow-up en `#215` Open Question #7).

Slice 3 cierra el ciclo de feedback de Fase 3: 3 endpoints backend **read-only** (más 1 mutación opcional de reset), 2 repository methods nuevos con agregaciones SQL, 1 página frontend nueva (`/groups/:id/moderation`), y 1 botón link adicional en `GroupDetailPage`. El admin puede ver el dashboard cuando lo necesita (sin auto-poll, refresh on-demand), inspeccionar advertencias activas con display name, decidir perdonar (reset manual) sin esperar la expiración natural.

---

## Scope

### In Scope

- **Backend `logs.Repository.CountByActionAndGroup(ctx, groupID, actions, since) (map[string]int, error)`**: agregado SQL `SELECT action, COUNT(*) FROM logs WHERE group_id = $1 AND action = ANY($2) AND created_at >= $3 GROUP BY action`. 1 roundtrip para 3 actions (`RULE_TRIGGERED`, `AUTOMUTE_USER`, `AUTOBAN_USER`).
- **Backend `automation.Repository.ListActiveWarningStatesByGroup(ctx, groupID) ([]WarningStateWithUser, error)`**: SELECT con LEFT JOIN a `users ON users.telegram_id = user_warning_state.user_id`, `WHERE warning_count > 0`, `ORDER BY warning_count DESC, last_warning_at DESC NULLS LAST`. Tipo nuevo `WarningStateWithUser` embebe `WarningState` + `FirstName string` + `Username *string`.
- **Backend 3 handlers nuevos en `WithAutomation`** (todos `requireAuth`):
  - `GET /api/groups/{id}/automation/warnings` → lista de advertencias activas con display name.
  - `POST /api/groups/{id}/automation/warnings/{user_id}/reset` → `UPDATE user_warning_state SET warning_count = 0 WHERE group_id = $1 AND user_id = $2`; log `ActionResetWarnings` con `ActorID` del admin, `metadata={user_id, warning_count_before_reset}`.
  - `GET /api/groups/{id}/automation/stats?period=24h|7d` (default `24h`) → `{rule_triggered: N, automute: M, autoban: K, period: "24h"}` desde `logs`. Validación: `period` solo `"24h"` o `"7d"`.
- **Backend constante nueva `logs.ActionResetWarnings = "RESET_WARNINGS"`** (admin action, `ActorID` no nulo). Metadata incluye `warning_count_before_reset` para auditoría.
- **Frontend feature module `automation/` MOD**:
  - `types.ts` (+25 LOC): `WarningStateWithUser`, `WarningsResponse`, `StatsResponse`, `StatsPeriod = "24h" | "7d"`.
  - `api.ts` (+50 LOC): `listWarnings(groupId)`, `resetWarning(groupId, userId)`, `getStats(groupId, period)`.
  - `hooks.ts` (+70 LOC): `useWarnings(groupId)`, `useResetWarning(groupId)` (mutation con `onSuccess` invalidando `['automation','warnings',groupId]`), `useStats(groupId)` + `useStatsPeriod` (combina period state + query). Sin `refetchInterval` (YAGNI).
  - `error.ts` MOD: `formatAutomationError` maneja los nuevos errores (`NOT_FOUND`, `VALIDATION_ERROR` en period inválido).
- **Frontend página NUEVA `pages/GroupModerationPage.tsx`** (~280 LOC): ruta `/groups/:id/moderation` registrada en `App.tsx` con `RequireAuth`. Layout 2 secciones sin Save button:
  - **Sección 1 — Estadísticas (24h)**: 3 stat cards (`Rule triggered`, `Auto-mute`, `Auto-ban`) + `<Select>` Mantine v7 con period selector (`24h`/`7d`) + botón "Refrescar" arriba a la derecha (invalida ambas queries).
  - **Sección 2 — Advertencias activas**: `<Table>` con columnas User (display name + user_id dimmed), Warnings (number badge), Última advertencia (relative date), Acciones (botón Reset con `IconRefresh`). Empty state con `<Text c="dimmed">`. Cap top 100 con warning visible si `>100`.
  - **Reset mutation**: `<Modal>` Mantine v7 de confirmación ("¿Resetear advertencias de {nombre}?"); aceptar dispara POST; cancelar cierra sin acción.
- **Frontend `GroupDetailPage.tsx` MOD** (+15 LOC): en Tab "Detalle", renombrar botón "Configurar automatización" → **"Configurar reglas de moderación"** (1 línea) y agregar nuevo botón debajo: **"Ver dashboard de moderación"** (`component={Link} to={`/groups/${groupId}/moderation`} variant="light" w={260} data-testid="moderation-dashboard-link"`).
- **Frontend `App.tsx` MOD** (+3 LOC): import + route `/groups/:id/moderation` con `RequireAuth`.
- **Frontend tests**:
  - `GroupModerationPage.test.tsx` NEW (~180 LOC): 5+ casos (render inicial con stats 12/3/1 + warnings list, period selector 7d cambia query, empty state, refresh invalida, reset modal abre/acepta/cancela) + 2 smoke (error del backend, display name fallback).
  - `GroupDetailPage.test.tsx` MOD (+20 LOC): 1 caso verificando nuevo botón + href correcto + label actualizado.
  - `test/helpers.tsx` MOD (+10 LOC): extension de `mockFetchRoutes` (3 substring routes nuevas).
- **Backend tests §21.1 estricto** (cero Bot API reales):
  - `logs/repository_test.go` MOD (+90 LOC): 4+ integration tests para `CountByActionAndGroup` (happy path 3 actions × 24h; filtro `since`; filtro `actions`; empty).
  - `automation/repository_test.go` MOD (+60 LOC): 3+ integration tests para `ListActiveWarningStatesByGroup` (filtra count=0; LEFT JOIN preserva sin fila users; orden count DESC).
  - `automation_handlers_test.go` MOD (+150 LOC): 6+ handler tests (GET /warnings 200 con LEFT JOIN shape, 404 grupo inexistente, 404 automation nil, 401 sin auth; POST reset 200 + log emitido, 404 sin warning_state; GET stats period=24h/7d, period=foo → 400, default 24h).
- **Docs**: `README.md` nota breve (≤20 líneas) en "Moderación automática" — ubicación del dashboard + cómo usar el reset manual.

### Out of Scope

- **Edición de reglas / umbrales desde la página del dashboard** → la página es read-only + reset; settings siguen en `/groups/:id/automation` (GroupAutomationPage).
- **Scheduler / background cron** (`ResetExpiredWarnings`) → slice 1 ya dejó el helper listo; el trigger automático es follow-up separado.
- **Auto-poll del dashboard** (`refetchInterval: 30s`) → YAGNI; refresh on-demand cubre el caso (D9).
- **CSV export de warnings** → YAGNI; la tabla es suficiente para inspección.
- **Paginación en `/warnings`** → cap defensivo top 100 + warning visible si >100. Si el admin tiene 1000+ advertencias activas simultáneamente, hay un problema más grande (reglas mal calibradas).
- **Stats desglosadas por regla** (`flood` / `anti_spam` / `anti_link` / `banned_words`) → costoso (parse metadata JSONB); ya está en `GroupLogsPage` con filtro.
- **`SELECT ... FOR UPDATE` en reset** → best-effort (UPSERT de slice 1 es atómico); race con `HandleMessage` tolerable (admin puede resetear de nuevo si lo ve).
- **Whitelist de admins exentos / notificar al bot del reset** → el reset solo limpia DB; el admin usa `POST /unmute` por separado si quiere desmutear.

---

## Capabilities

### Modified Capabilities

- **`moderation-automation`** (delta spec): nuevo REQ-32 (Warnings Dashboard backend), sub-REQ-32.1 (`ListActiveWarningStatesByGroup` con LEFT JOIN), sub-REQ-32.2 (`CountByActionAndGroup`); REQ-33 (3 endpoints `/warnings`, `/warnings/:user_id/reset`, `/stats`); REQ-34 (Frontend `GroupModerationPage` en `/groups/:id/moderation`); REQ-35 (Tests §21.1 estricto — cero Bot API). Spec canónico `openspec/specs/moderation-automation/spec.md` se AMPLÍA (técnica APPEND del archive phase; REQ-1..REQ-31 de slices 1+2+2.1 preservados).

> **Decisión de spec**: **AMEND** vía delta en `openspec/changes/moderation-automation/slice3/specs/moderation-automation/spec.md`. Aplica `ADDED Requirements` por cada nueva REQ.

---

## Approach

| # | Decisión | Cómo |
|---|----------|------|
| **D1** | **Surface**: nueva página `/groups/:id/moderation` (NO Sección 6 en `GroupAutomationPage`). Settings UI (writes, 513 LOC, 5 secciones + Save) y dashboard (reads, observación) son **dos intenciones distintas**. Mezclar rompe navegación y vuelve la página lenta. Patrón slice 2 (`/groups/:id/automation` como ruta separada) confirma la convención de rutas dedicadas por intención. | Registro en `App.tsx` con `RequireAuth`; link desde `GroupDetailPage` Tab "Detalle". |
| **D2** | **3 endpoints nuevos en `WithAutomation`** (mismo gating `if s.automation == nil` que slice 2): `GET /warnings`, `POST /warnings/:user_id/reset`, `GET /stats?period=24h\|7d`. Helpers reusables: `requireAuth`, `actorIDFromClaims`, `pathID`, `r.PathValue`, `respondAutomationError`. Validación `period` con whitelist (`24h` / `7d`); `user_id` positive int64. | Archivo `backend/internal/api/automation_handlers.go` crece de ~572 LOC a ~720 LOC (manejable). |
| **D3** | **LEFT JOIN a `users` para display name**: `users.telegram_id` PK = lookup O(1); para 1000 advertencias activas, 1000 lookups por Hash Join = ms-scale. Para users sin fila en `users` (entraron al grupo sin pasar por `chat_join_request`), `first_name=""` y `username=NULL` → fallback frontend a `"user {user_id}"`. | SQL: `LEFT JOIN users u ON u.telegram_id = uws.user_id`. |
| **D4** | **2 repository methods nuevos** (con integration tests contra Postgres real, `OpenTestDB` + `goose.Up` + `TRUNCATE`): `ListActiveWarningStatesByGroup` (LEFT JOIN, `WHERE warning_count > 0`, ORDER BY count DESC + last_warning_at DESC NULLS LAST) + `CountByActionAndGroup` (`action = ANY($2)` con array = 1 roundtrip para 3 actions). `ANY` con array bounded a 3 elementos es óptimo. | Tipo nuevo `WarningStateWithUser` embebe `WarningState` + FirstName + *Username. |
| **D5** | **Action constant nueva `logs.ActionResetWarnings = "RESET_WARNINGS"`**: admin action, `ActorID` del admin (NO nil — distinto de auto-actions). Metadata `{user_id, warning_count_before_reset}` para auditoría. | `backend/internal/logs/model.go` MOD (+3 LOC). |
| **D6** | **Permission check invariante**: `requireAuth` + `actorIDFromClaims` (admin-facing, no bot-facing). NO se usa `permissionOkAdmin` (esos son checks sobre estado del bot; aquí no aplica). Bugfix `#172` no se ve afectado — el paquete automation ya usa `BotStatus == StatusAdministrator` consistentemente desde slice 1. | `automation_handlers.go` reusa el patrón de slice 2 (consistente en los 12 endpoints). |
| **D7** | **Frontend página NUEVA `GroupModerationPage.tsx`** (2 secciones sin Save button): arriba 3 stat cards con `<Select>` period selector (24h/7d) + `<Button>` Refrescar (invalida 2 query keys vía `queryClient.invalidateQueries`); abajo `<Table>` advertencias activas con columnas User (display name + user_id dimmed), Warnings (number badge), Última advertencia (relative date format), Acciones (`<Button size="xs" variant="default">` con `IconRefresh` para Reset). Empty state con `<Text c="dimmed" ta="center">`. Reset mutation abre `<Modal>` Mantine v7 (no `window.confirm`, consistente con notifications de slices 2+2.1). | Stack Mantine v7 (no instalar deps nuevas). Cap top 100 con warning visible si `>100`. |
| **D8** | **Link update en `GroupDetailPage` Tab "Detalle"**: renombrar botón existente "Configurar automatización" → **"Configurar reglas de moderación"** (1 línea) + agregar nuevo botón debajo: **"Ver dashboard de moderación"** (`variant="light"` con `data-testid="moderation-dashboard-link"`). Cambio de label = 1 línea; diferenciación visual = settings (writes) vs dashboard (reads). | `frontend/src/pages/GroupDetailPage.tsx` MOD (+15 LOC). |
| **D9** | **Refresh: on-demand (button), NO auto-poll**. `useWarnings` y `useStats` usan `useQuery` sin `refetchInterval`. Botón "Refrescar" dispara `queryClient.invalidateQueries(['automation','warnings',groupId])` + `['automation','stats',groupId,period]`. Auto-poll = carga innecesaria al backend (3 endpoints × N grupos × admins concurrentes potenciales). YAGNI para MVP. Si en el futuro se necesita live updates, agregar `refetchInterval: 30_000` (2 líneas). | `frontend/src/features/automation/hooks.ts` MOD (+70 LOC). |
| **D10** | **Tests §21.1 estricto**: cero llamadas Bot API. Backend usa fakes hand-rolled (precedente publications-slice3); Frontend usa `mockFetchRoutes` con substring URL. Test breakdown: backend integration 7 casos (4 logs repo + 3 automation repo); backend handlers 6 casos (200/400/404/401); frontend `GroupModerationPage` 7 casos (5+2 smoke); frontend `GroupDetailPage` 1 caso (link nuevo). | `mockFetchRoutes` extensions ~10 LOC en `frontend/src/test/helpers.tsx`. |

---

## Schema

**Sin migración nueva**. Slice 3 es 100% queries sobre tablas existentes (`user_warning_state`, `logs`, `users`).

**Nota sobre índice (R2)**: la query `CountByActionAndGroup` puede ser lenta en grupos con miles de logs en 7d. **Decidir en `design`** después de `EXPLAIN ANALYZE` en dev: si la query es >100ms en grupos típicos, crear migración `00009_add_logs_composite_index.sql` con `CREATE INDEX idx_logs_group_action_created ON logs (group_id, action, created_at DESC)`. Por ahora **NO** se agrega índice (YAGNI hasta medir).

---

## Affected Areas

### Backend MOD (~9 archivos, ~280 LOC new + ~80 LOC mod)

| Archivo | Acción | LOC est. | Notas |
|---------|--------|---------:|-------|
| `backend/internal/logs/model.go` | MOD | +3 | Constante `ActionResetWarnings = "RESET_WARNINGS"` |
| `backend/internal/logs/repository.go` | MOD | +45 | Método `CountByActionAndGroup(ctx, groupID, actions, since) (map[string]int, error)` |
| `backend/internal/logs/repository_test.go` | MOD | +90 | 4+ integration tests para `CountByActionAndGroup` |
| `backend/internal/automation/model.go` | MOD | +12 | Tipo `WarningStateWithUser` (embebe `WarningState` + FirstName + *Username) |
| `backend/internal/automation/repository.go` | MOD | +35 | Método `ListActiveWarningStatesByGroup(ctx, groupID)` con LEFT JOIN |
| `backend/internal/automation/repository_test.go` | MOD | +60 | 3+ integration tests para `ListActiveWarningStatesByGroup` |
| `backend/internal/api/automation_handlers.go` | MOD | +130 | 3 handlers nuevos: `handleListWarnings`, `handleResetWarning`, `handleGetStats` |
| `backend/internal/api/automation_handlers_test.go` | MOD | +150 | 6+ tests para los 3 handlers nuevos |
| `backend/internal/api/server.go` | MOD | +10 | 3 nuevas HandleFunc dentro de `WithAutomation` |
| `backend/cmd/server/main.go` | MOD | 0 | Sin cambios — `WithAutomation` se sigue invocando igual |

### Frontend MOD (~9 archivos, ~420 LOC new + ~50 LOC mod)

| Archivo | Acción | LOC est. | Notas |
|---------|--------|---------:|-------|
| `frontend/src/features/automation/types.ts` | MOD | +25 | `WarningStateWithUser`, `WarningsResponse`, `StatsResponse`, `StatsPeriod` |
| `frontend/src/features/automation/api.ts` | MOD | +50 | `listWarnings(groupId)`, `resetWarning(groupId, userId)`, `getStats(groupId, period)` |
| `frontend/src/features/automation/hooks.ts` | MOD | +70 | `useWarnings`, `useResetWarning`, `useStats`, `useStatsPeriod`; sin auto-poll |
| `frontend/src/features/automation/error.ts` | MOD | +10 | `formatAutomationError` maneja nuevos errores |
| `frontend/src/pages/GroupModerationPage.tsx` | **NEW** | +280 | Página dashboard; 2 secciones; Modal de reset; period selector; refresh button |
| `frontend/src/pages/GroupModerationPage.test.tsx` | **NEW** | +180 | 5+ tests + 2 smoke tests |
| `frontend/src/pages/GroupDetailPage.tsx` | MOD | +15 | Renombrar "Configurar automatización" → "Configurar reglas de moderación" + nuevo botón "Ver dashboard de moderación" |
| `frontend/src/pages/GroupDetailPage.test.tsx` | MOD | +20 | Test del nuevo link + label actualizado |
| `frontend/src/App.tsx` | MOD | +3 | Import + route `/groups/:id/moderation` |
| `frontend/src/test/helpers.tsx` | MOD | +10 | Extension de `mockFetchRoutes` (3 substring routes nuevas) |

### Docs

- `README.md`: nota breve (≤20 líneas) en sección "Moderación automática" — ubicación del dashboard (`/groups/:id/moderation`) + cómo usar el reset manual.
- `.env.example`: 0 cambios.

**Total estimado**: ~830 LOC touched. **Excede 400-line budget** → `size:exception` necesario (precedente 10 PRs consecutivos aprobados, obs `#220/#228/#238`).

---

## Risks

| # | Riesgo | Likelihood | Impact | Mitigation |
|---|--------|-----------|--------|------------|
| R1 | `permissionOk` de `moderation` (NO de automation) sigue con bug `#172` — afecta ban/unban/mute manuales, FUERA scope | High | Medium | Sin cambios: `automation.permissionOkAdmin` ya es correcto (slices 1+2+2.1). Fix del moderation service = follow-up separado. Invariante preservada por el grep de no-cambios en `backend/internal/moderation/`. |
| R2 | Query `CountByActionAndGroup` sin índice `(group_id, action, created_at)` puede ser lenta en grupos con miles de logs | Medium | Medium | **Decidir en `design`**: medir con `EXPLAIN ANALYZE` en dev sobre grupos con volumen realista. Si >100ms, crear migración `00009_add_logs_composite_index.sql` con `CREATE INDEX idx_logs_group_action_created ON logs (group_id, action, created_at DESC)`. Por ahora sin índice (YAGNI hasta medir). |
| R3 | LEFT JOIN a `users` puede ser lento si la tabla tiene miles de filas | Low | Low | `users.telegram_id` ya es PK → lookup O(1) por user_id. Para 1000 advertencias activas, 1000 lookups por Hash Join = ms-scale. Aceptable. |
| R4 | User reset no desmutear al user (sigue muteado si estaba muteado) | Low | Low | **Intencional**: el reset SOLO limpia el counter en DB. Si el admin quiere desmutear, usa `POST /unmute` desde `GroupDetailPage`. Doc en descripción del endpoint y en el Modal del frontend ("Esto no desmutea al usuario"). |
| R5 | Race condition: `HandleMessage` (slice 1) en paralelo con admin reset | Low | Low | UPSERT en `UpsertWarningState` ya es atómico. Si admin resetea a `count=0` y `HandleMessage` ya incrementó entre el SELECT y el UPDATE del handler de reset, gana `HandleMessage` (write-after-read inconsistency tolerable — el admin puede resetear de nuevo si lo ve). Si la consistencia estricta importa, agregar `SELECT ... FOR UPDATE` en el handler (costo: lock corto en la fila). **Por ahora best-effort**. |
| R6 | Stats endpoint expone volúmenes de moderación a todos los admins (no solo al admin del grupo) | Low | Low | El endpoint requiere `requireAuth` (cualquier admin autenticado). Consistente con `GET /settings`, `GET /banned-words`. Aceptable. |
| R7 | Period selector "7d" cuenta logs de los últimos 7×24h exactos, pero el admin puede interpretar "esta semana" | Low | Low | Label claro en `<Select>`: "Últimos 7 días" (no "esta semana"). Definición exacta: `since = now - 7*24h`. |
| R8 | Display name desde `users` puede ser NULL → frontend muestra "user 12345" pero el admin esperaba un nombre | Low | Low | Fallback explícito en frontend ("user 12345"); admin puede cruzar con `GroupLogsPage` para ver el `msg.From` original del primer `RULE_TRIGGERED`. |
| R9 | Mantine v7 `<Modal>` requiere import correcto (no `Dialog` en v7) | Low | Low | Import: `import { Modal } from '@mantine/core'`. Mantine v7 usa Modal (no Dialog). |
| R10 | Confirmación del reset via `<Modal>` vs `window.confirm` — cuál es el patrón del repo | Low | Low | Usar `<Modal>` de Mantine v7 (consistente con `notifications` de slices 2+2.1, mejor UX con i18n potencial). Repo usa `window.confirm` en `GroupDetailPage` para acciones destructivas (lock/delete), pero el reset merece Modal porque es accion nueva con contexto (nombre del user + warning_count). |
| R11 | LOC est. ~830 excede 400-line budget | **High** | Medium | `size:exception` — precedente 10 PRs consecutivos aprobados en este repo (obs `#220/#228/#238`). El `tasks` phase re-confirmará el forecast. |
| R12 | `user_warning_state.warning_count > 0` puede traer usuarios con `last_warning_at` muy antiguo (expirados pero no reseteados físicamente) | Medium | Low | Slice 1 diseñó `ResetExpiredWarnings` para background (cron futuro). Por ahora la expiración es lógica: `last_warning_at < now - warning_expire_days` → reset en `HandleMessage` paso 5. Si el admin ve count=2 con `last_warning_at=hace 60 días`, sigue siendo válido mostrarlo — el admin puede usar Reset manual. Documentado en la UI. |
| R13 | `warn_user_template` heredado de slice 2.1 puede tener chars raros que rompan tests frontend si se renderiza en la tabla | Low | Low | El template NO se renderiza en el dashboard (solo en el chat al user). El dashboard solo muestra `first_name` + `warning_count`. No aplica. |

---

## Rollback

`git revert` del merge. Sin migración nueva, así que **no hay schema changes que revertir**. `automation_handlers.go` crece ~130 LOC pero las funciones nuevas (`handleListWarnings`, `handleResetWarning`, `handleGetStats`) son additive; basta con no registrar las 3 rutas en `server.go` para "apagar" el dashboard. `frontend/src/pages/GroupModerationPage.tsx` y los cambios en `GroupDetailPage.tsx` quedan inertes (la ruta `/groups/:id/moderation` deja de existir; el botón link roto navega a 404 Mantine v7). Audit logs preservados (`RESET_WARNINGS` históricos quedan en `logs`). `frontend/src/features/automation/{types,api,hooks,error}.ts` MOD son additive (los nuevos tipos/funciones coexisten con los existentes). `GroupAutomationPage` intacta.

---

## Dependencies

- Slice 1 archivado (`main @ 1b10d42`, obs `#223`): `user_warning_state` (migración `00006`) + `logs.ActionRuleTriggered`/`AutomuteUser`/`AutobanUser` + `HandleMessage` pipeline.
- Slice 2 archivado (`main @ 2df231f`, obs `#231`): `AutomationSettings` types frontend + `WithAutomation` option (9 endpoints existentes) + helpers reusables (`requireAuth`, `actorIDFromClaims`, `pathID`, `r.PathValue`, `respondAutomationError`, `automationLogWriter` interface).
- Slice 2.1 archivado (`main @ e7d0680`, obs `#241`): `logs.ActionWarnUserSent` + `warn_user_template` columna (no usada en slice 3 pero existente). Precedente de `size:exception` con `WarningSender` pattern (no se reutiliza — slice 3 no envía mensajes).
- `internal/users/` (slices previos, `model.go`): `users.telegram_id`, `users.first_name`, `users.username` (poblada parcialmente vía `chat_join_request` en `main.go:209-216`).
- `frontend/src/components/` (Mantine v7): `<Modal>` component disponible para el reset confirmation.
- Bugfix `#172` invariante: `automation.permissionOkAdmin` usa `BotStatus == StatusAdministrator`, NUNCA `can_*`. Slice 3 NO introduce checks nuevos (handlers son admin-gated vía `requireAuth`).
- `queryClient` + React Query (precedente slice 2): para invalidación de queries en el refresh.

---

## Delivery

**Single-pr, size:exception solicitada** (precedente: 10 PRs consecutivos aprobados en este repo). Branch: `feat/moderation-automation-slice3` base `main @ e7d0680`. Tasks phase re-confirmará el forecast (~830 LOC excede 400) o recomendará chained (`backend-repo` / `backend-handlers` / `frontend-page` / `frontend-tests`).

---

## Success Criteria

- [ ] `logs.Repository.CountByActionAndGroup(ctx, groupID, actions, since)` retorna `map[string]int` agrupado por action, filtrado por `since` y `action = ANY($2)`, en 1 roundtrip
- [ ] `automation.Repository.ListActiveWarningStatesByGroup(ctx, groupID)` retorna `[]WarningStateWithUser` con `WHERE warning_count > 0`, `ORDER BY warning_count DESC, last_warning_at DESC NULLS LAST`, LEFT JOIN a `users`
- [ ] `GET /api/groups/{id}/automation/warnings` → 200 con shape `{warnings: [{user_id, first_name, username, warning_count, last_warning_at, last_action_at}, ...]}` (LEFT JOIN visible)
- [ ] `POST /api/groups/{id}/automation/warnings/{user_id}/reset` → 200 con `{user_id, warning_count: 0, reset: true}`; existe log `RESET_WARNINGS` con `actor_id` del admin y `metadata.warning_count_before_reset`
- [ ] `GET /api/groups/{id}/automation/stats?period=24h` y `?period=7d` → 200 con `{rule_triggered: N, automute: M, autoban: K, period: "24h"|"7d"}`
- [ ] `GET /api/groups/{id}/automation/stats?period=foo` → 400 `VALIDATION_ERROR` con mensaje legible
- [ ] Si `automation == nil` (kill switch) los 3 endpoints responden 404 con mensaje "modulo de automation no habilitado"
- [ ] Si grupo no existe → 404 `NOT_FOUND` (igual que slice 2)
- [ ] Si sin auth → 401 `UNAUTHORIZED` (los 3 endpoints)
- [ ] Frontend `GroupModerationPage` renderiza en `/groups/:id/moderation` con `RequireAuth`
- [ ] Sección 1 muestra 3 stat cards + `<Select>` period selector (24h/7d) + botón Refrescar; cambiar period dispara nuevo fetch con `?period=7d`
- [ ] Botón Refrescar invalida `['automation','warnings',groupId]` + `['automation','stats',groupId,period]`
- [ ] Sección 2 muestra tabla con display name + dimmed user_id; empty state si `warnings.length === 0`; warning visible si `>100`
- [ ] Click en Reset abre `<Modal>` de Mantine v7; aceptar dispara POST; cancelar cierra sin acción
- [ ] `GroupDetailPage` Tab "Detalle": botón "Configurar automatización" renombrado a "Configurar reglas de moderación" + nuevo botón "Ver dashboard de moderación" con `data-testid="moderation-dashboard-link"`
- [ ] `App.tsx` registra ruta `/groups/:id/moderation`
- [ ] Tests backend: 4+ logs repo, 3+ automation repo, 6+ handlers (200/400/404/401) — todos en verde
- [ ] Tests frontend: 7+ `GroupModerationPage.test.tsx` (5+2 smoke) + 1 `GroupDetailPage.test.tsx` — todos en verde
- [ ] `mockFetchRoutes` extendido con 3 substring routes nuevas
- [ ] §21.1: cero llamadas Bot API reales en tests (audit en `verify`)
- [ ] `go test ./...`, `npm test -- --run`, `go vet ./...`, `gofmt -l .`, `npm run build` en verde
- [ ] `git diff main -- backend/internal/moderation/` = empty (acciones manuales intactas, bugfix `#172` no se ve afectado)
- [ ] `git diff main -- backend/internal/automation/service.go backend/internal/automation/autoactioner.go backend/internal/automation/warning_sender.go backend/internal/automation/templates.go backend/internal/automation/rules.go` = empty (pipeline de evaluación NO tocado)
- [ ] `grep -rn "can_" backend/internal/automation/*.go | grep -v '^//'` = 0 matches (bugfix `#172` invariante)
- [ ] §25 sin secretos en repo; §13.1 sin migración nueva (queries sobre tablas existentes)

---

## Open Questions

Ninguna — la exploración (`#242`) resolvió todo: surface nueva página `/groups/:id/moderation` (D1); 3 endpoints read-only + 1 reset opcional (D2); LEFT JOIN a `users` para display name (D3); 2 repository methods nuevos con SQL optimizado (D4); Action constant `RESET_WARNINGS` con metadata de auditoría (D5); permission check invariante con `requireAuth` (D6); frontend `GroupModerationPage` con 2 secciones + Modal confirmación (D7); link update en `GroupDetailPage` con rename del botón existente (D8); refresh on-demand sin auto-poll (D9); tests §21.1 estricto con breakdown completo (D10). Las decisiones D1-D10 son operativas dentro de este slice y se confirman en `design`. R2 (¿índice `logs(group_id, action, created_at)`?) se decide en `design` midiendo `EXPLAIN ANALYZE` en dev.
