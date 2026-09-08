# Design: Moderation Automation — Slice 3 (Warnings Dashboard)

**Change**: `moderation-automation-slice3` · Slice 3/3 (último) de **Fase 3 — Moderación Automática** (AGENTS §23).
**Mode**: hybrid · Persisted to `openspec/changes/moderation-automation/slice3/design.md` + Engram `sdd/moderation-automation/slice3/design`.
**Predecessors**: exploration #242, proposal #243, spec #244; bugfix #172 (permisos admin usan `BotStatus == StatusAdministrator`, NUNCA `can_*`).
**Strategy**: single-pr, `size:exception` (precedente 10 PRs consecutivos, obs #220/#228/#238).
**Forecast**: ~830 LOC touched across 21 files · 400-line budget risk: **High** · chained PRs: **No** (size:exception solicitada por el usuario).
**Branch base**: `main @ e7d0680` (slice 2.1 ya mergeado).

## Technical Approach

Slice 3 cierra el ciclo de feedback de Fase 3 con **3 endpoints read-only** + **1 reset opcional** sobre datos que slices 1+2+2.1 ya producen (`user_warning_state`, `logs`). Cero cambios al pipeline de evaluación: las funciones nuevas son **additive** en `automation/model.go`, `automation/repository.go`, `logs/model.go`, `logs/repository.go` y `api/automation_handlers.go`. El pipeline (`service.go`/`worker.go`/`autoactioner.go`/`warning_sender.go`/`rules.go`/`templates.go`) y el paquete `moderation/` quedan **intactos** (verificado en archive con `git diff`).

Surface: nueva página `GroupModerationPage` en `/groups/:id/moderation` (registrada en `App.tsx` con `RequireAuth`). Patrón consistente con slice 2 (`/groups/:id/automation` como ruta separada): settings (writes) y dashboard (reads) son **dos intenciones distintas** — mezclarlas en `GroupAutomationPage` (513 LOC) la rompe. Link desde `GroupDetailPage` Tab "Detalle" + rename del botón existente ("Configurar automatización" → "Configurar reglas de moderación") para diferenciar settings vs dashboard.

Refresh on-demand vía botón "Refrescar" (invalida 2 query keys) — **NO** auto-poll (YAGNI; carga innecesaria). Reset con confirmación vía `<Modal>` Mantine v7 (consistente con notifications de slices 2+2.1; `GroupUsersPage.tsx:302-329` es el precedente). §21.1 estricto: tests **nunca** llaman a la Bot API real.

## Architecture Decisions

| # | Decisión | Alternativa descartada | Rationale |
|---|----------|------------------------|-----------|
| **D1** | Surface: nueva página `/groups/:id/moderation` (NO Sección 6 en `GroupAutomationPage`) | Sección 6+7 en `GroupAutomationPage` | Settings (writes, 513 LOC, 5 secciones + Save) y dashboard (reads) son 2 intenciones; mezclar rompe navegación y vuelve la página lenta. Patrón slice 2 ratifica la convención. |
| **D2** | 3 endpoints en `WithAutomation` con gating `if s.automation == nil` | Endpoints directos sin Option | Consistente con slice 2 (12 endpoints totales en el Option, file pasa de 572 a ~720 LOC — manejable). Reusa helpers `requireAuth`, `actorIDFromClaims`, `pathID`, `respondAutomationError`. |
| **D3** | LEFT JOIN `users` para display name | Escanear logs; solo user_id | `users.telegram_id` PK = lookup O(1); ms-scale para 1000 advertencias. Ya poblado vía `chat_join_request` (main.go:209-216); users sin fila → fallback frontend `"user {user_id}"`. |
| **D4** | `logs.CountByActionAndGroup` con `action = ANY($2)` (1 roundtrip) | 3 queries separadas | Array bounded a 3 elementos: Postgres optimiza igual; 1 roundtrip reduce latencia. |
| **D5** | `logs.ActionResetWarnings = "RESET_WARNINGS"` con `ActorID != nil` + metadata `{user_id, warning_count_before_reset}` | Reusar `ActionUpdateAutomationSettings` | Distingue **admin actions** (ActorID != nil) de **auto-actions** (ActorID == nil) de slice 1 — invariante documentada en `logs/model.go:24-35`. Metadata para auditoría (cuánto se perdonó). |
| **D6** | Permission check: `requireAuth` + `actorIDFromClaims` (admin-facing) | `permissionOkAdmin` | Endpoints admin-facing, no bot-facing; `permissionOkAdmin` valida estado del bot — no aplica. Bugfix #172 sigue intacto (slice 3 NO introduce checks nuevos). |
| **D7** | Frontend: nueva `GroupModerationPage.tsx` (2 secciones sin Save) | Reusar `GroupAutomationPage` con secciones extra | Read-only + reset; la separación de rutas refleja la intención del admin ("¿edito o miro?"). |
| **D8** | `GroupDetailPage` Tab Detalle: rename "Configurar automatización" → "Configurar reglas de moderación" + nuevo botón "Ver dashboard de moderación" | Solo nuevo botón sin rename | Diferencia visual settings (writes) vs dashboard (reads). 1 línea cambio + 1 botón nuevo. |
| **D9** | Refresh on-demand vía botón, NO `refetchInterval` | Auto-poll cada 30s | Carga innecesaria al backend (3 endpoints × N grupos × admins). YAGNI para MVP. Si se necesita live, agregar `refetchInterval: 30_000` (2 líneas). |
| **D10** | Cap defensivo top 100 + `truncated:bool` en `/warnings` | Paginación | Si el admin tiene 1000+ advertencias activas, hay un problema más grande (reglas mal calibradas). Frontend muestra `<Alert color="yellow">` si truncated. |
| **D11** | Reset **NO** desmutea al user (intencional) | Reset también ejecuta `unmute` | Slice 3 = DB only; admin usa `POST /unmute` separado desde `GroupDetailPage`. Doc en endpoint + Modal ("Esto no desmutea al usuario"). |
| **D12** | Reset usa `UPDATE ... RETURNING` (single statement atómico) | `SELECT FOR UPDATE` + lock | `UpsertWarningState` ya es UPSERT atómico; race con `HandleMessage` tolerable (admin puede resetear de nuevo). Si la consistencia estricta importa, agregar lock en design. Best-effort. |
| **D13** | **NO** migración nueva (R2) | Agregar índice `(group_id, action, created_at)` en `logs` | Slice 3 es 100% queries sobre tablas existentes. EXPLAIN ANALYZE en dev con datos realistas; crear migración `00009_add_logs_composite_index.sql` solo si >100ms en grupos típicos (YAGNI hasta medir). Documentado en design. |
| **D14** | `DisplayName() string` helper en backend (`WarningStateRow.DisplayName()`) | Helper solo frontend | Backend devuelve `first_name` (string vacío) + `username` (*string nil); el helper encapsula la lógica "FirstName ?? Username ?? 'user {id}'" para mantener el contrato y dar flexibilidad al frontend. |

## Data Flow

```
Admin en /groups/:id/moderation
    │
    ├─ mount ─→ GET /api/groups/{id}/automation/warnings
    │           ├─ Repository.ListActiveWarningStatesByGroup(ctx, groupID, limit=100)
    │           │   └─ SELECT uws.*, COALESCE(u.first_name,''), u.username
    │           │       FROM user_warning_state uws
    │           │       LEFT JOIN users u ON u.telegram_id = uws.user_id
    │           │       WHERE uws.group_id = $1 AND uws.warning_count > 0
    │           │       ORDER BY uws.warning_count DESC, uws.last_warning_at DESC NULLS LAST
    │           │       LIMIT 100
    │           └─ [{user_id, first_name, username, warning_count, ...}]
    │
    ├─ mount ─→ GET /api/groups/{id}/automation/stats?period=24h
    │           ├─ Repository.CountByActionAndGroup(ctx, groupID, ["RULE_TRIGGERED","AUTOMUTE_USER","AUTOBAN_USER"], now-24h)
    │           │   └─ SELECT action, COUNT(*) FROM logs
    │           │       WHERE group_id = $1 AND action = ANY($2) AND created_at >= $3
    │           │       GROUP BY action
    │           └─ {rule_triggered: N, automute: M, autoban: K, period: "24h"}
    │
    └─ click "Reset" en fila u ─→ Modal confirm
            └─ POST /api/groups/{id}/automation/warnings/{u}/reset
                ├─ Repository.ResetWarningState(ctx, groupID, u) → UPDATE ... SET warning_count=0 WHERE ... RETURNING old.warning_count
                ├─ logs.Create(RESET_WARNINGS, ActorID=admin, metadata={user_id, warning_count_before_reset})
                └─ 200 {user_id, warning_count:0, reset:true}
                    └─ useWarnings invalida → refetch automático → u desaparece de la tabla
```

## File Changes

### Backend (10 files, ~360 LOC new + ~80 LOC mod)

| # | File | Action | LOC | Description |
|---|------|--------|-----|-------------|
| 1 | `backend/internal/logs/model.go` | MOD | +4 | `ActionResetWarnings = "RESET_WARNINGS"` con comentario slice 3. |
| 2 | `backend/internal/logs/repository.go` | MOD | +45 | `CountByActionAndGroup(ctx, groupID, actions, since) (map[string]int, error)` — SQL `SELECT action, COUNT(*) ... WHERE action = ANY($2) AND created_at >= $3 GROUP BY action`. 1 roundtrip para 3 actions bounded. Helper `scanCount(row) (string, int, error)` reutilizable. |
| 3 | `backend/internal/logs/repository_test.go` | MOD | +90 | 4 integration tests: happy path 3 actions, filtro `since`, filtro `actions` subset, empty result. Reusa `testDB` + `sampleEntry`. |
| 4 | `backend/internal/automation/model.go` | MOD | +15 | Tipo `WarningStateRow` (8 cols: 6 de `WarningState` embebido + `FirstName string` + `Username *string`). Método `(WarningStateRow).DisplayName() string` → `FirstName ?? ("@"+*Username) ?? "user {user_id}"`. |
| 5 | `backend/internal/automation/repository.go` | MOD | +50 | 2 métodos nuevos: `ListActiveWarningStatesByGroup(ctx, groupID, limit) ([]WarningStateRow, error)` con LEFT JOIN + cap configurable (default 100); `ResetWarningState(ctx, groupID, userID) (oldCount int, error)` con `UPDATE ... RETURNING warning_count` para auditoría (single statement atómico, sin `SELECT FOR UPDATE`). Helper `scanWarningStateRow`. |
| 6 | `backend/internal/automation/repository_test.go` | MOD | +100 | 4 integration tests: filtra `warning_count=0`, LEFT JOIN preserva fila sin users, orden count DESC + NULLS LAST, ResetWarningState retorna oldCount + setea 0. |
| 7 | `backend/internal/api/automation_handlers.go` | MOD | +130 | 3 handlers: `handleListWarnings` (parse :group_id + check `s.automation != nil` + `s.automationGroups.GetByTelegramID` 404 + `ListActiveWarningStatesByGroup(100)` + extra `COUNT(*) WHERE warning_count > 0` para `truncated`); `handleResetWarning` (parse :group_id + :user_id + `actorIDFromClaims` + `ResetWarningState` retorna `oldCount` + si `oldCount == 0` → 404 NOT_FOUND + sino log `ActionResetWarnings` con `metadata{user_id, warning_count_before_reset}`); `handleGetStats` (parse :group_id + `r.URL.Query().Get("period")` whitelist `["24h","7d"]` default `"24h"` +400 si otro + `CountByActionAndGroup` con `["RULE_TRIGGERED","AUTOMUTE_USER","AUTOBAN_USER"]` + `since = now - period*time.Hour`). 12° y 13° entradas en el route table de `WithAutomation`. |
| 8 | `backend/internal/api/automation_handlers_test.go` | MOD | +200 | 6+ tests: `TestAutomationWarnings_List_Shape` (LEFT JOIN visible), `TestAutomationWarnings_List_Truncated`, `TestAutomationWarnings_List_GroupNotFound_404`, `TestAutomationWarnings_List_AutomationNil_404`, `TestAutomationWarnings_Reset_Success_Logs` (verify metadata + ActorID), `TestAutomationWarnings_Reset_NoState_404`, `TestAutomationStats_24h_Default`, `TestAutomationStats_7d`, `TestAutomationStats_PeriodFoo_400`, `TestAutomationStats_GroupNotFound_404`. Extension de `fakeAutomationService` (no requiere nuevo fake — handlers usan `s.automationGroups` directo + el `Service` no se llama desde estos handlers). |
| 9 | `backend/internal/api/server.go` | MOD | +10 | 3 nuevas `mux.HandleFunc` en `WithAutomation`: `GET .../automation/warnings`, `POST .../automation/warnings/{user_id}/reset`, `GET .../automation/stats`. Sin cambio de signature del Option (los handlers usan `s.automationGroups` y `s.automationLogs` que ya están inyectados). |
| 10 | `backend/cmd/server/main.go` | MOD | 0 | Sin cambios — `WithAutomation` ya recibe `logsRepo` (línea 246/257 actual) y los handlers nuevos usan ese mismo log writer para `ActionResetWarnings`. |

### Frontend (10 files, ~610 LOC new + ~50 LOC mod)

| # | File | Action | LOC | Description |
|---|------|--------|-----|-------------|
| 11 | `frontend/src/features/automation/types.ts` | MOD | +35 | `WarningStateRow` (mirror del backend con `display_name` calculado server-side via `WarningStateRow.DisplayName()`), `WarningsResponse {warnings, truncated}`, `StatsPeriod = "24h"\|"7d"`, `AutomationStats {rule_triggered, automute, autoban, period}`. |
| 12 | `frontend/src/features/automation/api.ts` | MOD | +50 | 3 funciones: `listWarnings(groupId): Promise<WarningsResponse>` (GET sin query params), `resetWarning(groupId, userId): Promise<{user_id, warning_count: 0, reset: true}>` (POST), `getStats(groupId, period): Promise<AutomationStats>` (GET `?period=...`). |
| 13 | `frontend/src/features/automation/hooks.ts` | MOD | +80 | 3 hooks: `useWarnings(groupId)` (`useQuery` key `['automation','warnings',groupId]`), `useResetWarning(groupId)` (`useMutation` con `onSuccess` invalidando `warnings` y `stats` keys), `useStats(groupId, period)` (`useQuery` key `['automation','stats',groupId,period]`). Sin `refetchInterval` (YAGNI). |
| 14 | `frontend/src/features/automation/error.ts` | MOD | +15 | Branch 404 NOT_FOUND en `formatAutomationError` → "no hay advertencias activas"; branch 400 VALIDATION_ERROR → mensaje del backend si existe sino "parámetros inválidos". |
| 15 | `frontend/src/pages/GroupModerationPage.tsx` | **NEW** | +280 | 2 secciones sin Save button. **Sección 1 — Estadísticas**: 3 `<Card>` (Mantine v7) con `<Text size="xl">` para counts, `<Select>` period (24h/7d), `<Button>` "Refrescar" (invalida 2 queryKeys). Loading skeleton mientras pending. **Sección 2 — Advertencias activas**: `<Table>` con columnas User (display_name + dimmed user_id), Warnings (Badge numérico), Última advertencia (Intl.RelativeTimeFormat es-AR), Acciones (`<Button size="xs" leftSection={<IconRefresh/>}>Reset</Button>` con `data-testid="warning-reset-{user_id}"`). Empty state `<Text c="dimmed" ta="center">`. Truncation alert si `truncated`. Cap visual 100 con scroll. Reset abre `<Modal>` Mantine v7 (precedente `GroupUsersPage.tsx:302`) con "¿Resetear advertencias de {display_name}? Esto no desmutea al usuario." + botones Cancelar/Resetear. |
| 16 | `frontend/src/pages/GroupModerationPage.test.tsx` | **NEW** | +220 | 7+ tests: render inicial (3 stat cards + tabla), period selector 7d cambia queryKey, empty state, truncation alert, refresh invalida 2 queries, reset modal abre/acepta/cancela, display name fallback a `"user {id}"`, error backend muestra notifyError. |
| 17 | `frontend/src/pages/GroupDetailPage.tsx` | MOD | +15 | Rename línea 217 "Configurar automatización" → "Configurar reglas de moderación" + nuevo `<Button component={Link} to={`/groups/${groupId}/moderation`} variant="light" w={260} data-testid="moderation-dashboard-link">Ver dashboard de moderación</Button>` debajo. |
| 18 | `frontend/src/pages/GroupDetailPage.test.tsx` | MOD | +25 | 1 nuevo test: verifica `getByTestId('moderation-dashboard-link')` con href `/groups/123/moderation` + texto actualizado. Actualizar test existente (línea 77) para reflejar el rename del label. |
| 19 | `frontend/src/App.tsx` | MOD | +3 | `import GroupModerationPage` + `<Route path="/groups/:id/moderation" element={<GroupModerationPage />} />` dentro del `<Route element={<RequireAuth><Layout/></RequireAuth>}>` padre. |
| 20 | `frontend/src/test/helpers.tsx` | MOD | +10 | 3 substring routes nuevas en `mockFetchRoutes` (opcional — el default 401 cubre la mayoría; tests específicos usan fetchMock custom como `GroupAutomationPage.test.tsx:84-120`). Documentado como helper para futuros tests. |
| 21 | `README.md` | MOD | +12 | Sección "Moderación automática: Dashboard" (≤15 líneas) — ubicación `/groups/:id/moderation`, qué muestra (stats + advertencias activas), cómo usar el reset manual (con advertencia "no desmutea"). Reemplaza la frase "y slice 3 (dashboard de warnings) son posteriores y no están en esta versión" del header de status. |

### Doc
- `README.md`: +12 LOC (item 21 arriba). `.env.example`: 0 cambios (slice 3 no introduce env vars).

**Total**: ~830 LOC touched (consistente con forecast ~830 en proposal #243). Excede budget → `size:exception` (precedente 10 PRs consecutivos).

## Interfaces / Contracts

### Backend — tipos nuevos en `automation/model.go`

```go
// WarningStateRow extiende WarningState con display name (best-effort
// LEFT JOIN a users). Para el dashboard de slice 3.
type WarningStateRow struct {
    WarningState              // embebido: GroupID, UserID, WarningCount, LastWarningAt, LastActionAt, ExpiresAt
    FirstName string          // "" si user no estaba en chat_join_request
    Username  *string         // nil si user no estaba en chat_join_request
}

// DisplayName devuelve el nombre legible para mostrar al admin:
// FirstName preferido, fallback a @username, fallback final a "user {id}".
func (w WarningStateRow) DisplayName() string {
    if w.FirstName != "" {
        return w.FirstName
    }
    if w.Username != nil && *w.Username != "" {
        return "@" + *w.Username
    }
    return fmt.Sprintf("user %d", w.UserID)
}
```

### Backend — repository signatures

```go
// automation.Repository.ListActiveWarningStatesByGroup: cap defensivo.
func (r *Repository) ListActiveWarningStatesByGroup(ctx context.Context, groupID int64, limit int) ([]WarningStateRow, error)

// automation.Repository.ResetWarningState: single UPDATE...RETURNING.
func (r *Repository) ResetWarningState(ctx context.Context, groupID, userID int64) (oldCount int, err error)

// logs.Repository.CountByActionAndGroup: 1 roundtrip, ANY array.
func (r *Repository) CountByActionAndGroup(ctx context.Context, groupID int64, actions []string, since time.Time) (map[string]int, error)
```

### Backend — handlers en `api/automation_handlers.go`

| Handler | Ruta | Auth | Response 200 | Errores |
|---------|------|------|--------------|---------|
| `handleListWarnings` | `GET /api/groups/{id}/automation/warnings` | requireAuth | `{warnings: [WarningStateRow...], truncated: bool}` (cap 100) | 404 automation nil · 404 grupo no existe · 500 internal |
| `handleResetWarning` | `POST /api/groups/{id}/automation/warnings/{user_id}/reset` | requireAuth + actorIDFromClaims | `{user_id, warning_count: 0, reset: true}` + log RESET_WARNINGS con metadata `{user_id, warning_count_before_reset}` | 401 no auth · 404 automation nil · 404 grupo no existe · 404 fila sin warning_state (`oldCount == 0`) · 500 internal |
| `handleGetStats` | `GET /api/groups/{id}/automation/stats?period=24h\|7d` | requireAuth | `{rule_triggered, automute, autoban, period}` (default 24h si ausente) | 400 period inválido · 404 automation nil · 404 grupo no existe · 500 internal |

### Frontend — types en `features/automation/types.ts`

```ts
export type StatsPeriod = '24h' | '7d'

export interface AutomationStats {
  rule_triggered: number
  automute: number
  autoban: number
  period: StatsPeriod
}

export interface WarningStateRow {
  user_id: number
  display_name: string  // server-computed via WarningStateRow.DisplayName()
  username: string | null
  warning_count: number
  last_warning_at: string | null  // ISO8601
  last_action_at: string | null
  expires_at: string | null
}

export interface WarningsResponse {
  warnings: WarningStateRow[]
  truncated: boolean  // true si hay > 100 advertencias activas
}
```

## Testing Strategy (§21.1 estricto — cero Bot API)

| Layer | What | Approach |
|-------|------|----------|
| Backend integration | `logs.Repository.CountByActionAndGroup` | 4 casos contra Postgres real (`database.OpenTestDB("logs")` + `goose.Up` + `TRUNCATE logs`): (1) happy path inserta 5/2/1 RULE/AUTOMUTE/AUTOBAN + 3 BAN_USER no contados; (2) filtro `since` inserta logs en -48h vs -1h; (3) filtro `actions` subset; (4) empty grupo. Reusa `sampleEntry` helper. |
| Backend integration | `automation.Repository.ListActiveWarningStatesByGroup` + `ResetWarningState` | 4 casos: (1) filtra count=0; (2) LEFT JOIN preserva fila sin users (`users.telegram_id` NULL → FirstName="" Username=nil); (3) orden count DESC + last_warning_at DESC NULLS LAST; (4) ResetWarningState retorna oldCount + setea 0. Reusa `setupRepoDB` + `insertGroup`. |
| Backend handlers | 3 handlers con fakes (sin DB ni Telegram) | 6+ tests: `TestAutomationWarnings_List_Shape` (LEFT JOIN visible en JSON), `TestAutomationWarnings_List_Truncated`, `TestAutomationWarnings_Reset_200_Logs` (verifica `al.entries` con `ActionResetWarnings`, ActorID=1, metadata correcta), `TestAutomationWarnings_Reset_NoState_404`, `TestAutomationStats_Period24h_Default`, `TestAutomationStats_PeriodFoo_400`, `TestAutomationStats_Period7d`. Reusa `fakeAutomationService`/`fakeAutomationGroups`/`fakeAutomationLogs` + `buildAutomationServer`. Extension del `TestAutomationRoutes_RequireAuth` para incluir las 3 nuevas rutas (lines 208-217). |
| Frontend | `GroupModerationPage` | 7+ casos: render inicial con 2 warnings + stats 12/3/1, period selector 7d cambia fetch URL, empty state cuando `warnings=[]`, truncation alert cuando `truncated=true`, refresh button invalida ambas queries, reset modal abre/acepta/cancela, display name fallback, error backend muestra notifyError. |
| Frontend | `GroupDetailPage` MOD | 1 caso nuevo: label actualizado + `data-testid="moderation-dashboard-link"` + href correcto. Actualizar test existente (línea 77). |
| E2E / integration Telegram | — | NO aplica (slice 3 es 100% queries sobre tablas propias; no introduce calls Telegram). |

## Migration / Rollout

**No migration.** Slice 3 es 100% queries sobre tablas existentes (`user_warning_state`, `logs`, `users`).

**R2 decisión (índice `logs(group_id, action, created_at)`)**: **NO** crear índice en slice 3. Mitigación:
1. Medir con `EXPLAIN ANALYZE` en dev después del deploy con datos realistas (`docker compose up postgres + backend`, poblar `logs` con 1000+ filas de un grupo de prueba).
2. Si `CountByActionAndGroup` es >100ms en grupos típicos, abrir follow-up slice con migración `00009_add_logs_composite_index.sql` (`CREATE INDEX idx_logs_group_action_created ON logs (group_id, action, created_at DESC)`).
3. Documentar este punto en el `apply-report` y `archive-report` para que el próximo mantenedor sepa dónde quedó.

**Rollback**: `git revert` del merge. Sin schema changes. `automation_handlers.go` crece ~130 LOC additive; basta con no registrar las 3 rutas en `server.go` para "apagar" el dashboard. Frontend `GroupModerationPage.tsx` y los cambios en `GroupDetailPage.tsx` quedan inertes (la ruta `/groups/:id/moderation` deja de existir; el botón link roto navega a 404 Mantine v7). Audit logs `RESET_WARNINGS` históricos preservados en `logs`.

**No-regression** (verificación en archive):
- `git diff main -- backend/internal/automation/service.go backend/internal/automation/worker.go backend/internal/automation/autoactioner.go backend/internal/automation/warning_sender.go backend/internal/automation/rules.go backend/internal/automation/templates.go` = empty (pipeline NO tocado).
- `git diff main -- backend/internal/moderation/` = empty (acciones manuales intactas; bugfix #172 fuera de scope).
- `git diff main -- frontend/src/pages/` excluyendo `GroupModerationPage.tsx` (NEW) y `GroupDetailPage.tsx` (MOD) = empty.
- `grep -rn 'can_' backend/internal/automation/*.go | grep -v '^//'` = 0 matches (bugfix #172 invariante).
- §21.1: cero Bot API reales en tests.

## Open Questions — Resolved

Las 8 preguntas abiertas de exploration #242 + proposal #243 resueltas:

1. **¿Índice `(group_id, action, created_at)` en `logs`?** → **NO en slice 3**. D13 + sección Migration. Medir con EXPLAIN ANALYZE en dev; follow-up si >100ms.
2. **`SELECT FOR UPDATE` en reset?** → **NO** (D12). Best-effort; `UPDATE...RETURNING` atómico. Si reporta inconsistencia, agregar lock en follow-up.
3. **¿Modal o `window.confirm` para reset?** → **Modal Mantine v7** (D7). Precedente `GroupUsersPage.tsx:302-329` en el repo.
4. **¿Renombrar "Configurar automatización" → "Configurar reglas de moderación"?** → **SÍ** (D8). 1 línea cambio; diferencia visual settings vs dashboard.
5. **¿Auto-poll en `useStats`?** → **NO** (D9). YAGNI. Refresh on-demand.
6. **¿Stats desglosadas por regla?** → **NO** (D4 + D5). Spec dice solo aggregate. Ya está en `GroupLogsPage` con filtro.
7. **¿Export CSV de warnings?** → **NO** (YAGNI; tabla es suficiente).
8. **¿Paginación en `/warnings`?** → **NO** (D10). Cap top 100 + `truncated:bool` + `<Alert color="yellow">` si >100.

## Risk Table

| # | Riesgo | L | I | Mitigation |
|---|--------|---|---|-----------|
| R1 | `permissionOk` de `moderation` (NO de automation) sigue con bug #172 — afecta ban/unban/mute manuales | High | Med | **Fuera de scope**: `automation.permissionOkAdmin` ya correcto desde slice 1. Slice 3 NO introduce checks nuevos. Verificación: `grep can_ backend/internal/automation/*.go` = 0 matches en executable. |
| R2 | Query `CountByActionAndGroup` sin índice puede ser lenta en grupos con miles de logs | Med | Med | **D13**: NO crear índice ahora. EXPLAIN ANALYZE post-deploy; migración `00009` si >100ms. |
| R3 | LEFT JOIN a `users` lento si tabla tiene miles de filas | Low | Low | `users.telegram_id` PK = lookup O(1). ms-scale para 1000 advertencias. |
| R4 | Reset no desmutea al user (sigue muteado si lo estaba) | Low | Low | **D11 + intencional**. Doc en endpoint + Modal ("Esto no desmutea al usuario"). Admin usa `POST /unmute` separado. |
| R5 | Race: `HandleMessage` paralelo con admin reset | Low | Low | **D12**: `UPDATE...RETURNING` atómico. Race tolerable (admin puede resetear de nuevo). `SELECT FOR UPDATE` opcional si se reporta. |
| R6 | Stats endpoint expone volúmenes a todos los admins autenticados | Low | Low | `requireAuth` requerido; consistente con `GET /settings`, `GET /banned-words`. Aceptable. |
| R7 | Period selector "7d" interpretación ambigua | Low | Low | Label claro "Últimos 7 días" en `<Select>`; `since = now - 7*24h` exacto. |
| R8 | Display name NULL en `users` | Low | Low | Helper `WarningStateRow.DisplayName()` en backend (D14) → fallback `"user {id}"`. |
| R9 | Mantine v7 `<Modal>` vs `<Dialog>` | Low | Low | Import: `import { Modal } from '@mantine/core'`. v7 usa Modal. Precedente `GroupUsersPage.tsx:19`. |
| R10 | Confirmación reset: Modal vs `window.confirm` | Low | Low | **Modal** (D7). Consistente con `GroupUsersPage.tsx:302-329` (ban). Mejor UX con i18n potencial. |
| R11 | LOC ~830 excede 400-line budget | **High** | Med | `size:exception` (precedente 10 PRs consecutivos, obs #220/#228/#238). Tasks phase re-confirmará. |
| R12 | `warning_count > 0` trae usuarios con `last_warning_at` muy antiguo (expirados pero no reseteados físicamente) | Med | Low | Expiración lógica en `HandleMessage` paso 5. Admin puede usar Reset manual. |
| R13 | `warn_user_template` heredado de slice 2.1 puede romper tests frontend | Low | Low | Template NO se renderiza en el dashboard (solo en chat al user). No aplica. |
| R14 | Mock `fakeAutomationService` no necesita extensión — handlers nuevos usan `s.automationGroups` directo | Low | Low | Verificado: `handleListWarnings` solo llama `automationGroups.GetByTelegramID` + `repository.ListActiveWarningStatesByGroup` (no Service); `handleResetWarning` llama `repository.ResetWarningState` + `automationLogs.Create`; `handleGetStats` llama `automationGroups.GetByTelegramID` + `logStore.CountByActionAndGroup` (vía `s.automationLogs` extension). |
| R15 | `WithAutomation` ya recibe `logsRepo` en main.go — NO requiere cambio de signature | Low | Low | Verificado: main.go:246/257 ya pasa `logsRepo`. Los 3 handlers reusan `s.automationLogs.Create(...)` + acceden `db` indirectamente via Service. **Pero** `CountByActionAndGroup` necesita acceso al `*sql.DB` — el handler lo obtiene extendiendo `automationLogWriter` interface (que ya tiene `Create`) con un nuevo método `CountByActionAndGroup`. Alternativa: pasar `*logs.Repository` directo via nuevo interface `logReader`. Decisión: **extender `automationLogWriter` a `logStoreReader` con 2 métodos** (mantiene cohesión, una sola injection). |

## What / Why / Where / Learned

**What**: Diseño técnico completo de slice 3 (Warnings Dashboard) de Fase 3. Define 2 repository methods nuevos (`ListActiveWarningStatesByGroup`, `ResetWarningState`), 1 método nuevo en logs repo (`CountByActionAndGroup`), 3 handlers HTTP, 1 constante `ActionResetWarnings`, 1 página frontend nueva `GroupModerationPage`, 1 link + 1 rename en `GroupDetailPage`. Sin migración nueva.

**Why**: Slices 1+2+2.1 producen todos los datos (`user_warning_state` + logs automation); admin hoy no tiene UI para observarlos. Slice 3 cierra el ciclo de feedback con 3 endpoints read-only + 1 reset opcional.

**Where**: `openspec/changes/moderation-automation/slice3/design.md` (filesystem) + Engram `sdd/moderation-automation/slice3/design` (persistencia).

**Learned**:
- Bugfix #172 sigue intacto: slice 3 NO introduce checks nuevos sobre `can_*`. El grep `grep -rn 'can_' backend/internal/automation/*.go | grep -v '^//'` debe dar 0 matches en executable (verificación en archive).
- 12° + 13° endpoints en `WithAutomation` (file pasa de 572 a ~720 LOC) — manejable.
- `UPDATE...RETURNING` evita `SELECT FOR UPDATE` manteniendo atomicidad (D12).
- `display_name` calculado server-side via `WarningStateRow.DisplayName()` (D14) encapsula el fallback y da flexibilidad — el frontend recibe un string listo, no necesita replicar la lógica de fallback.
- Precedente de Modal en `GroupUsersPage.tsx:302-329` (ban confirmation) — patrón consistente en el repo.
- R2 (índice en `logs`) se decide por medición, no por anticipación (D13) — disciplina YAGNI consistente con `AGENTS.md` §2 ("No agregar infraestructura salvo necesidad concreta").
- `WithAutomation` ya recibe `logsRepo` en main.go; los handlers nuevos reusan `s.automationLogs.Create` para `RESET_WARNINGS` Y extienden el interface a `logStoreReader` con `CountByActionAndGroup` (R15).
- 10° size:exception consecutivo (precedente obs #220/#228/#238) — el patrón del repo es pedir aprobación explícita del usuario antes de apply.