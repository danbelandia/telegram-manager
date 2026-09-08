# Exploration: Moderation Automation — Slice 3 (Warnings Dashboard)

> **Change**: `moderation-automation-slice3` — Slice 3/3 (último) de **Fase 3 — Moderación Automática** (AGENTS §23).
> **Mode**: hybrid (filesystem + Engram). Persisted as `sdd/moderation-automation/slice3/exploration` + este filesystem.
> **Path**: `openspec/changes/moderation-automation/slice3/exploration.md`.
> **Predecessor**: slice 2.1 archivado (`main @ e7d0680`, obs `#241`). Canónico en `openspec/specs/moderation-automation/spec.md` (REQ-1..REQ-31, 31 requisitos).
> **Parent exploration**: `#215` (`sdd/moderation-automation/exploration`) — 3-slice plan que preveía "Dashboard" como scope de este slice.
> **Authority**: bugfix `#172` (permisos admin usan `BotStatus == StatusAdministrator`, NUNCA claves `can_*`); AGENTS §11 (no Redis), §14 (modular backend), §17.1 (sin secretos), §18.1 (rate limits), §21.1 (mock TelegramService).
> **Scope**: features de **lectura/observación** sobre datos que slices 1+2+2.1 ya producen — `user_warning_state` (creado en slice 1) + logs (`RULE_TRIGGERED` / `AUTOMUTE_USER` / `AUTOBAN_USER`, slice 1; `WARN_USER_SENT`, slice 2.1). Cero cambios al pipeline de evaluación. Cero mutaciones nuevas excepto un reset manual opcional.

---

## Current State (verificable en `main @ e7d0680` post-slice-2.1)

### Lo que YA existe (producido por slices 1+2+2.1, listo para consumir)

| Recurso | Fuente | Tabla / Handler | Notas |
|---------|--------|-----------------|-------|
| `user_warning_state` (counter por `(group, user)`) | slice 1, migración `00006` | PK compuesta `(group_id, user_id)`, columnas `warning_count SMALLINT`, `last_warning_at TIMESTAMPTZ NULL`, `last_action_at TIMESTAMPTZ NULL`, `expires_at TIMESTAMPTZ NULL` | `automation.Repository.ListWarningStates(ctx, groupID)` YA existe (repository.go:158-183) pero retorna TODAS las filas (incluye `warning_count=0`). Necesitamos un variant con `WHERE warning_count > 0` ordenado por count desc. |
| Logs de auto-moderación | slice 1, 3 constantes | `logs.ActionRuleTriggered = "RULE_TRIGGERED"`, `ActionAutomuteUser = "AUTOMUTE_USER"`, `ActionAutobanUser = "AUTOBAN_USER"` (todas con `ActorID=nil`) | `logs.Repository.ListByGroup(ctx, groupID)` ya existe (repository.go:42) pero carga filas + unmarshal JSONB metadata por fila (costoso para agregaciones). Necesitamos un `CountByActionAndGroup(ctx, groupID, actions, since)` que use `COUNT(*) GROUP BY action` y filtros `action IN (...) AND created_at >= since`. |
| `users` table (display name) | slices previos, `internal/users/` | Columnas `telegram_id BIGINT`, `first_name TEXT`, `username TEXT NULL` (model.go:14-21) | **Best-effort**: la fila existe solo para usuarios que pasaron por un `chat_join_request` (main.go:209-216 popula `usersRepo.UpsertByTelegramID`). Usuarios ya en el grupo sin join request reciente pueden no tener fila → fallback a `user_id` como display name. LEFT JOIN conserva todas las warning_states. |
| `AutomationSettings` (frontend types) | slice 2, slice 2.1 | `frontend/src/features/automation/types.ts` | Cubre settings + listas + warning template. **No incluye** `WarningState` ni `AutomationStats`. Necesitamos agregar 2-3 types. |
| `WithAutomation` option (server.go:163-177) | slice 2 | 9 endpoints actuales: GET/PUT settings, GET/POST/DELETE banned-words, GET/POST/DELETE link-allowlist | Los handlers nuevos se agregan DENTRO de este Option, mismo gating `if s.automation == nil`. |
| `automationLogWriter` interface | slice 2, automation_handlers.go:54 | `Create(ctx, *logs.Entry) error` | Reusable para emitir `ActionResetWarnings` (slice 3) sin ampliar contrato. |
| `actorIDFromClaims(w, r)` helper | slice 2 | `requireAuth` middleware | Reusable para extraer admin en handlers nuevos. |
| `pathID(w, r, "id")` + `r.PathValue("user_id")` | slice 2 | Helpers existentes | Reusable para el reset endpoint. |
| `respondAutomationError(w, err)` | slice 2, automation_handlers.go:563 | Mapea `ErrNotFound` / `ErrAutomationGroupNotFound` / default → §18 | Reusable. |
| `GroupAutomationPage` (settings editor) | slice 2 + 2.1 | `frontend/src/pages/GroupAutomationPage.tsx` | 513 LOC, 5 secciones + Save button. **Carga mounts SETTINGS UI**. NO es lugar para el dashboard (settings vs observation son dos intenciones distintas). |
| `GroupDetailPage` (overview) | frontend-refresh-slice1 | `frontend/src/pages/GroupDetailPage.tsx` | 3 Tabs: Detalle / Solicitudes / Logs. Panel Detalle tiene 2 botones link (Membresía y moderación, Configurar automatización). Lugar correcto para un tercer botón: "Ver dashboard de moderación". |

### Lo que NO existe (gap a llenar)

1. **No hay endpoint de lectura** para la lista de `user_warning_state` activos. `ListWarningStates` existe pero no se expone vía HTTP; slice 1 lo dejó como "para dashboard de slice 3".
2. **No hay agregaciones sobre logs**. Stats cards ("auto-mutes hoy", "auto-bans hoy", "warnings totales") mencionadas en exploration `#215` D7 → sin endpoint hoy.
3. **No hay UI** de visualización de advertencias o stats. El admin hoy tiene que correr SQL o leer logs crudos para saber quién está cerca del threshold.
4. **No hay reset manual** de advertencias desde el panel (item documentado como follow-up en `#215` Open Question #7).

### Hechos confirmados de la Bot API (sin cambios)

Slice 3 es 100% backend queries + frontend rendering. **Cero llamadas a la Bot API**. Las decisiones de slice 3 NO requieren confirmar nada nuevo con Telegram: `user_warning_state` se llena por el pipeline de slice 1 (que ya usa `tg.MuteUser`/`tg.BanUser` reusando el adapter). Stats se computan sobre la tabla `logs` (que el pipeline de slice 1 ya popula). El reset manual también es puro DB (`UPDATE user_warning_state`). El bot no se entera del reset (correcto: el reset es una decisión del admin, no un evento de Telegram).

---

## Decisiones (D1-D10)

### D1 — Surface: nueva página `/groups/:id/moderation` (NO Sección 6 en GroupAutomationPage)

**Decisión**: renderizar el dashboard en una **página nueva** `/groups/:id/moderation` (registrada en `App.tsx` con `RequireAuth`), accesible vía un nuevo botón en `GroupDetailPage` → Tab Detalle.

**Rationale**:

- Settings UI (GroupAutomationPage, 513 LOC con 5 secciones + Save) y dashboard (read-only, "qué está pasando") son **dos intenciones distintas**. Mezclar las dos cosas en una sola página la vuelve lenta de navegar y confunde al admin ("¿aquí edito o miro?").
- El patrón slice 2 (`/groups/:id/automation` como ruta separada) ya estableció la convención de rutas dedicadas por intención. Continuamos con esa convención.
- GroupAutomationPage ya tiene Sections 1-5; agregar Sections 6-7 la haría ~700 LOC y rompería la cadencia visual.

**Alternativa descartada — Sección 6+7 en GroupAutomationPage**:

- Pros: 1 ruta menos en App.tsx; 1 link menos en GroupDetailPage.
- Cons: settings page se vuelve pesada; query de stats hace roundtrip aunque el admin solo quiera ver settings; el "Guardar" button de la página (settings) queda ambiguo cuando el dashboard no tiene estado a guardar.

### D2 — Backend endpoints (4 handlers, todos `requireAuth`)

| Método + Ruta | Devuelve | Acción |
|---------------|----------|--------|
| `GET /api/groups/{id}/automation/warnings` | `200 {"warnings": [{user_id, first_name?, username?, warning_count, last_warning_at, last_action_at}, ...]}` (ordenado `warning_count DESC, last_warning_at DESC`, solo `warning_count > 0`) | Lista de advertencias activas con LEFT JOIN a `users` para display name. |
| `POST /api/groups/{id}/automation/warnings/{user_id}/reset` | `200 {"user_id": int, "warning_count": 0, "reset": true}` | Resetea el counter del user (admin decide cuándo perdonar). Loguea `ActionResetWarnings` con ActorID del admin. |
| `GET /api/groups/{id}/automation/stats?period=24h\|7d` | `200 {"rule_triggered": N, "automute": M, "autoban": K, "period": "24h"}` (default 24h) | Conteos agrupados por `action` desde `logs`, filtrados por group + action ∈ set + created_at >= now - period. |

**Validación**:

- `period` solo acepta `"24h"` o `"7d"` (otro valor → `400 VALIDATION_ERROR`).
- `user_id` en path: positive int64, mismo helper `pathID` que slices 1+2.
- Si el grupo no existe → `404 NOT_FOUND` (igual que slice 2).
- Si `automation == nil` (kill switch) → `404 NOT_FOUND` con mensaje "modulo de automation no habilitado" (consistente con todos los handlers slice 2).

### D3 — LEFT JOIN a `users` para display name

**Decisión**: el endpoint `/warnings` devuelve un LEFT JOIN de `user_warning_state` con `users ON users.telegram_id = user_warning_state.user_id`. Devuelve `first_name` (string, puede ser "") y `username` (*string, puede ser NULL). Si la fila no existe en `users`, los campos de display name son NULL/`""` y el frontend muestra "user `{user_id}`" como fallback.

**Rationale**:

- El usuario lo sugirió como decisión abierta ("best-effort use last known msg.From.FirstName from logs OR show user_id"). El LEFT JOIN es más barato que parsear logs y mejor que mostrar user_id puro:
  - Para usuarios que entraron vía `chat_join_request` (main.go:209-216) → tenemos `FirstName` y `Username`.
  - Para usuarios que ya estaban en el grupo antes de este slice → user_id fallback (acceptable; admin puede usar el ID para `unban` manual si lo necesita).
- No agrega migraciones ni dependencias. Reusa `internal/users/` que ya está en main.go.

**Alternativa descartada — escanear logs para el last-known FirstName**:

- Pros: encuentra un nombre para cualquier user que haya tenido al menos 1 hit.
- Cons: O(N) parse de JSONB metadata por usuario; orden no determinista si un user tiene múltiples entradas; costoso en grupos grandes. **Descartado**.

### D4 — Repository methods (2 nuevos, ambos con integración tests)

#### D4.1 — `automation.Repository.ListActiveWarningStatesByGroup`

```go
// ListActiveWarningStatesByGroup devuelve los warning_state con
// warning_count > 0, ordenados por count DESC y luego por
// last_warning_at DESC. LEFT JOIN a users para display name.
// Usado por GET /automation/warnings (slice 3 dashboard).
func (r *Repository) ListActiveWarningStatesByGroup(ctx context.Context, groupID int64) ([]WarningStateWithUser, error)
```

**Tipo nuevo en `model.go`**:

```go
// WarningStateWithUser extiende WarningState con el display name
// (best-effort) desde la tabla users.
type WarningStateWithUser struct {
    WarningState               // embebido: GroupID, UserID, WarningCount, LastWarningAt, LastActionAt, ExpiresAt
    FirstName string  // puede ser ""
    Username  *string // puede ser nil si el user nunca se vio via join_request
}
```

**SQL**:

```sql
SELECT uws.group_id, uws.user_id, uws.warning_count,
       uws.last_warning_at, uws.last_action_at, uws.expires_at,
       u.first_name, u.username
FROM user_warning_state uws
LEFT JOIN users u ON u.telegram_id = uws.user_id
WHERE uws.group_id = $1 AND uws.warning_count > 0
ORDER BY uws.warning_count DESC, uws.last_warning_at DESC NULLS LAST
```

**Justificación de NULLS LAST**: si un user fue reseteado (no debería, ya filtramos count > 0) o por alguna razón `last_warning_at` quedó NULL, ordenamos por count primero y los NULLs caen al final (estables).

#### D4.2 — `logs.Repository.CountByActionAndGroup`

```go
// CountByActionAndGroup cuenta filas de logs agrupadas por action,
// filtradas por group_id + action ∈ set + created_at >= since.
// Devuelve un mapa action→count. Acciones sin filas se omiten del
// mapa (el handler rellena con 0). Usado por GET /automation/stats.
func (r *Repository) CountByActionAndGroup(ctx context.Context, groupID int64, actions []string, since time.Time) (map[string]int, error)
```

**SQL**:

```sql
SELECT action, COUNT(*) AS n
FROM logs
WHERE group_id = $1
  AND action = ANY($2)
  AND created_at >= $3
GROUP BY action
```

**Por qué `ANY($2)` con array**: query única para 3 actions. Alternativa sería 3 queries separados; `ANY` es 1 roundtrip y Postgres optimiza igual (los strings son cortos y la lista es bounded a 3 elementos).

**Por qué filtrar `action = ANY(...)` y no devolver todo el group + filtro en Go**: 1 grupo puede tener miles de logs en 7 días; filtrar en la DB es 10-100x más barato (índice en `group_id` ayuda).

### D5 — Action constant nueva: `ActionResetWarnings`

```go
// Slice 3: admin resetea manualmente el warning_count de un user.
// ActorID = admin (no nil — es acción manual del panel).
ActionResetWarnings = "RESET_WARNINGS"
```

**Metadata del log**:

```go
Metadata: map[string]any{
    "user_id":       userID,
    "warning_count_before_reset": ws.WarningCount,  // valor antes
},
```

**Por qué exponer el count pre-reset en metadata**: auditoría — el admin (u otro admin que lee los logs) puede ver cuántas advertencias tenía el user cuando se le perdonó. Sin esto, el log dice "RESET_WARNINGS" pero no cuánto se reseteó.

### D6 — Permission check: `requireAuth` + `actorIDFromClaims`, sin `permissionOkAdmin`

Consistente con todos los handlers de automation (slices 2+2.1): estos endpoints son **admin-facing**, no bot-facing. El admin ya está autenticado en el panel. NO usamos `permissionOkAdmin` (esos son checks sobre el estado del bot en el grupo; aquí no aplica).

### D7 — Frontend: nueva página `GroupModerationPage.tsx`

**Layout (2 secciones, sin Save button)**:

```
┌────────────────────────────────────────────────┐
│ ← Volver al grupo                              │
│                                                │
│ Dashboard de moderación automática             │
│ [Refrescar]                                    │
│                                                │
│ ┌─ Actividad reciente (periodo: 24h ⇄ 7d) ─┐ │
│ │  ┌──────┐ ┌──────┐ ┌──────┐               │ │
│ │  │  12  │ │  3   │ │  1   │               │ │
│ │  │Rule  │ │Auto- │ │Auto- │               │ │
│ │  │trig- │ │mute  │ │ban   │               │ │
│ │  │gered │ │      │ │      │               │ │
│ │  └──────┘ └──────┘ └──────┘               │ │
│ └────────────────────────────────────────────┘ │
│                                                │
│ ┌─ Advertencias activas ─────────────────────┐ │
│ │  User            Warnings  Última       Acc│ │
│ │  Juan Pérez          3     hace 2 h    [↻] │ │
│ │  987654321           2     hace 1 día  [↻] │ │
│ │  ...                                       │ │
│ │  (vacío: "No hay advertencias activas.")  │ │
│ └────────────────────────────────────────────┘ │
└────────────────────────────────────────────────┘
```

**Stack de componentes** (Mantine v7):

- `<Select>` para period selector (24h / 7d) — `data={[{value:'24h',label:'Últimas 24h'}, {value:'7d',label:'Últimos 7 días'}]}`.
- 3 `<Card>` o 3 columnas de `<Stack>` con `<Text size="xl">` para los counts.
- `<Table>` con columnas: User (display name + user_id dimmed), Warnings (number badge), Última advertencia (relative date format), Acciones (`<Button size="xs" variant="default">` con `IconRefresh`).
- Empty state: `<Text c="dimmed" ta="center" p="md">No hay advertencias activas.</Text>`.
- `<Button>` "Refrescar" arriba a la derecha del Title — dispara `queryClient.invalidateQueries(...)` para los 2 query keys.
- Reset mutation: `useResetWarning(groupId)` con confirmación via `<Modal>` de Mantine v7 (patrón ya disponible; ver `frontend/src/components/`).

### D8 — Frontend: link desde GroupDetailPage

En `GroupDetailPage.tsx` → Tab "detalle" → agregar debajo de "Configurar automatización":

```tsx
<Button
  component={Link}
  to={`/groups/${groupId}/moderation`}
  variant="light"
  w={260}
  data-testid="moderation-dashboard-link"
>
  Ver dashboard de moderación
</Button>
```

(Renombrar el botón "Configurar automatización" → "Configurar reglas de moderación" para diferenciar del nuevo "Ver dashboard". **DECIDIR en proposal/design si esta micro-copia es aceptable o si conviene otro label**.)

### D9 — Refresh strategy: on-demand (button), no auto-poll

**Decisión**: NO auto-poll. El admin hace click en "Refrescar" para invalidar las 2 queries (`['automation','warnings',groupId]` y `['automation','stats',groupId]`).

**Rationale**:

- Auto-poll cada 30s/60s agrega carga innecesaria al backend (3 endpoints × N grupos × admins concurrentes potenciales). El admin entra al dashboard cuando quiere ver; el refresh es discreto.
- Mantine v7 no provee auto-poll nativo en React Query; habría que usar `refetchInterval` en `useQuery`. Aceptable, pero el costo/beneficio no se justifica para este feature (los admins probablemente miran el dashboard cuando reciben un log de RULE_TRIGGERED, no en background).
- Si en el futuro se necesita live updates, se cambia `refetchInterval: 30_000` en `useQuery` (2 líneas). YAGNI ahora.

**Alternativa descartada — WebSocket / SSE para live updates**:

- Pros: live.
- Cons: requiere infra adicional (SSE handler en Go, mantener conexión, manejo de reconexión, etc.). **Fuera de scope MVP** y contradice §11 (NO Redis, NO infra extra).

### D10 — Tests strategy

#### Backend

- **Repository integration** (`logs/repository_test.go` MOD, ≥4 casos nuevos): `CountByActionAndGroup` happy path (3 actions × 24h window); filtro por `since` (logs antiguos fuera del período NO se cuentan); filtro por `actions` (logs con otras acciones NO se cuentan); empty result cuando no hay logs.
- **Repository integration** (`automation/repository_test.go` MOD, ≥3 casos nuevos): `ListActiveWarningStatesByGroup` filtra count=0; LEFT JOIN preserva warning_states sin fila en users; orden por count DESC.
- **Handler** (`automation_handlers_test.go` MOD, ≥6 casos nuevos):
  - `GET /warnings` 200 con LEFT JOIN shape; 404 si grupo no existe; 404 si automation nil; 401 sin auth.
  - `POST /warnings/:user_id/reset` 200 + log `RESET_WARNINGS` con actor; 404 si user_id no tiene warning_state; 401 sin auth; 404 si grupo no existe.
  - `GET /stats?period=24h` 200 con shape correcto; `?period=7d` similar; `?period=foo` → 400 VALIDATION_ERROR; default period=24h si ausente.
- **§21.1 estricto**: cero llamadas Bot API en tests (no se necesita ningún fake nuevo — los handlers slice 3 NO tocan Telegram).

#### Frontend

- `GroupModerationPage.test.tsx` NEW, ≥5 casos:
  - Render inicial con stats 12/3/1 + warnings list de 2 users + period selector 24h selected → render correcto.
  - Period selector 7d cambia `?period=7d` en el siguiente fetch.
  - Empty state cuando `/warnings` devuelve `{warnings: []}`.
  - Click en "Refrescar" invalida ambas.
  - Click en Reset abre Modal de confirmación; aceptar dispara POST; cancelar no dispara nada.
- `GroupModerationPage.test.tsx` 2 smoke tests adicionales:
  - Error del backend muestra `<Alert color="red">` con mensaje legible.
  - User con `first_name="Juan"` se muestra como "Juan" + dimmed "@username" (o fallback a user_id).
- `GroupDetailPage.test.tsx` MOD, ≥1 caso: el nuevo botón "Ver dashboard de moderación" existe y tiene el href correcto.
- `mockFetchRoutes` extensions (en `frontend/src/test/helpers.tsx` MOD, ~10 líneas): 3 nuevas rutas (GET warnings, POST reset, GET stats).

---

## Affected Areas

### Backend MOD (~7 archivos, ~280 LOC new + ~80 LOC mod)

| Archivo | Acción | LOC est. | Notas |
|---------|--------|----------|-------|
| `backend/internal/logs/model.go` | MOD | +3 | Constante `ActionResetWarnings = "RESET_WARNINGS"` |
| `backend/internal/logs/repository.go` | MOD | +45 | Método `CountByActionAndGroup(ctx, groupID, actions, since) (map[string]int, error)` |
| `backend/internal/logs/repository_test.go` | MOD | +90 | 4+ integration tests para `CountByActionAndGroup` |
| `backend/internal/automation/model.go` | MOD | +12 | Tipo `WarningStateWithUser` (embebe `WarningState` + FirstName + *Username) |
| `backend/internal/automation/repository.go` | MOD | +35 | Método `ListActiveWarningStatesByGroup(ctx, groupID)` con LEFT JOIN |
| `backend/internal/automation/repository_test.go` | MOD | +60 | 3+ integration tests para `ListActiveWarningStatesByGroup` |
| `backend/internal/api/automation_handlers.go` | MOD | +130 | 3 handlers nuevos: `handleListWarnings`, `handleResetWarning`, `handleGetStats`; + extension de la interfaz `automationService` (no necesita nuevos métodos — `Repository.ListActiveWarningStatesByGroup` se llama directo, similar a como `GetSettings` lo hace el handler con `s.automationGroups.GetByTelegramID`) |
| `backend/internal/api/automation_handlers_test.go` | MOD | +150 | 6+ tests para los 3 handlers nuevos |
| `backend/internal/api/server.go` | MOD | +10 | 3 nuevas HandleFunc dentro de `WithAutomation` |
| `backend/cmd/server/main.go` | MOD | 0 | Sin cambios — `WithAutomation` se sigue invocando igual |

### Frontend MOD (~6 archivos, ~420 LOC new + ~50 LOC mod)

| Archivo | Acción | LOC est. | Notas |
|---------|--------|----------|-------|
| `frontend/src/features/automation/types.ts` | MOD | +25 | `WarningStateWithUser`, `WarningsResponse`, `StatsResponse` |
| `frontend/src/features/automation/api.ts` | MOD | +50 | `listWarnings(groupId)`, `resetWarning(groupId, userId)`, `getStats(groupId, period)` |
| `frontend/src/features/automation/hooks.ts` | MOD | +70 | `useWarnings(groupId)`, `useResetWarning(groupId)`, `useStats(groupId)`, `useStatsPeriod(groupId)` (combina period state + query); query keys `['automation','warnings',groupId]`, `['automation','stats',groupId,period]` |
| `frontend/src/pages/GroupModerationPage.tsx` | **NEW** | +280 | Página dashboard; 2 secciones; Modal de reset; period selector; refresh button |
| `frontend/src/pages/GroupModerationPage.test.tsx` | **NEW** | +180 | 5+ tests + 2 smoke tests |
| `frontend/src/pages/GroupDetailPage.tsx` | MOD | +15 | Tercer botón link "Ver dashboard de moderación" debajo de "Configurar reglas de moderación" |
| `frontend/src/pages/GroupDetailPage.test.tsx` | MOD | +20 | Test del nuevo link |
| `frontend/src/App.tsx` | MOD | +3 | Import + route `/groups/:id/moderation` |
| `frontend/src/test/helpers.tsx` | MOD | +10 | Extension de `mockFetchRoutes` (no new helpers — solo documenta que las substring routes existentes ya cubren) |

### Docs

- `README.md`: nota breve (≤20 líneas) en la sección "Moderación automática" — "El dashboard de moderación está disponible en `/groups/:id/moderation` y muestra advertencias activas + stats de los últimos 24h/7d."
- `.env.example`: 0 cambios.

**Total estimado**: ~830 LOC touched. Sigue excediendo 400-line budget → `size:exception` necesario (precedente 10 PRs consecutivos aprobados en este repo, obs `#220/#228/#238`).

---

## Approaches Considered

### Opción A — **3 endpoints + nueva página `/groups/:id/moderation` + LEFT JOIN users + reset** ✅ RECOMENDADO

- Pros: scopes acotados; reusa todo el código existente; no toca el pipeline de evaluation; tests son local (no Bot API); convención consistente con slice 2.
- Cons: 4 archivos modificados en backend, 6 en frontend; supera 400 LOC.
- Effort: Medium (~1.5 días de implementación + 0.5 día de tests).

### Opción B — Sección 6 + 7 en GroupAutomationPage + sin reset

- Pros: 1 ruta menos; ~120 LOC menos de frontend.
- Cons: settings page se vuelve lenta de navegar; query de stats hace roundtrip innecesario cuando el admin solo quiere editar settings; reset perdido (feature útil omitido).
- Effort: Low-Medium (~1 día).

### Opción C — Dashboard minimal: solo stats cards, sin lista de warnings

- Pros: ~150 LOC menos; solo 1 endpoint nuevo.
- Cons: la lista de advertencias activas es el feature más útil para el admin ("¿quién está cerca del threshold?"). Omitirla pierde valor central.
- Effort: Low (~0.5 día).

### Opción D — Página `/groups/:id/moderation` + sin LEFT JOIN (solo user_id como display name)

- Pros: ~10 LOC menos en backend; SQL más simple; tests más directos.
- Cons: peor UX — admin no ve el nombre del usuario problemático, solo un número. Para grupos grandes (miles de usuarios) el admin tiene que cruzar manualmente con el log o el panel de users.
- Effort: Low.

### Opción E — Auto-poll cada 30s con `refetchInterval`

- Pros: dashboard "live".
- Cons: carga innecesaria al backend; contradice la simplicidad del MVP.
- Effort: +5 LOC en hooks; sin impacto en backend.

**Decisión final**: Opción A. Combina el máximo valor para el admin (lista de users con nombres + stats + reset) con scope acotado y consistencia con la arquitectura existente. Reset incluido por su utilidad (admin perdona después de un clean streak) y por su bajo costo (1 endpoint + 1 modal + 1 mutation).

---

## Risks

| # | Riesgo | Likelihood | Impact | Mitigation |
|---|--------|-----------|--------|-----------|
| R1 | `permissionOk` de `moderation` (NO de automation) sigue con bug #172 — afecta ban/unban/mute manuales, fuera de scope | High | Medium | Sin cambios: `automation.permissionOkAdmin` ya es correcto (slices 1+2+2.1). Documentado en archive reports `#223/#231/#241`. Fix del moderation service → follow-up separado. |
| R2 | Query de stats sin índice en `(group_id, action, created_at)` puede ser lenta en grupos con miles de logs | Medium | Medium | Verificar en design: si no hay índice, crear migración `00009_add_logs_composite_index.sql` con `CREATE INDEX idx_logs_group_action_created ON logs (group_id, action, created_at DESC)`. **DECIDIR en design** después de `EXPLAIN ANALYZE` en dev. |
| R3 | LEFT JOIN a `users` puede ser lento si la tabla tiene miles de filas | Low | Low | `users.telegram_id` ya es PK → lookup O(1) por user_id. Para 1000 advertencias activas, 1000 lookups por `Hash Join` = ms-scale. Aceptable. |
| R4 | User reset no notifica al bot (sigue muteado si estaba muteado) | Low | Low | Intencional: el reset SOLO resetea el counter en DB. Si el admin quiere desmutear, usa `GroupDetailPage` → POST `/unmute`. Doc en `reset` endpoint description. |
| R5 | Race condition: HandleMessage (slices 1) en paralelo con admin reset | Low | Low | `UPSERT` en `UpsertWarningState` ya es atómico. Si el admin resetea a count=0 y HandleMessage ya incrementó a count=1 entre el SELECT y el UPDATE del handler, gana HandleMessage (write-after-read inconsistency tolerable — el admin puede resetear de nuevo si lo ve). Si la consistencia estricta importa, agregar `SELECT ... FOR UPDATE` en el handler de reset (costo: lock corto en la fila). **DECIDIR en design** si vale la pena. Por ahora aceptar best-effort. |
| R6 | Stats endpoint expone volúmenes de moderación a todos los admins (no solo al admin del grupo) | Low | Low | El endpoint requiere `requireAuth` (cualquier admin autenticado). El grupo es del sistema; si el admin tiene sesión válida, puede ver stats de cualquier grupo. Consistente con GET `/settings`, GET `/banned-words`. Aceptable. |
| R7 | Period selector "7d" cuenta logs de los últimos 7×24h exactos, pero el admin puede interpretar "esta semana" | Low | Low | Label claro "Últimos 7 días" (no "esta semana"). Definición exacta: `since = now - 7*24h`. |
| R8 | Display name desde `users` puede ser NULL → frontend muestra "user 12345" pero el admin esperaba un nombre | Low | Low | Fallback explícito en frontend ("user 12345"); admin puede cruzar con `GroupLogsPage` para ver el `msg.From` original del primer RULE_TRIGGERED. |
| R9 | Frontend `<Modal>` de Mantine v7 requiere import correcto (Modal not Dialog en v7) | Low | Low | Verificar import en design (`import { Modal } from '@mantine/core'`); Mantine v7 usa Modal no Dialog. |
| R10 | Confirmación del reset via window.confirm() vs Modal — cuál es el patrón del repo | Low | Low | El repo usa `window.confirm` en GroupDetailPage (`confirmLock` línea 50-52, `confirmDelete` línea 60-62). Pero para reset (mutación nueva + importante + destructiva del audit trail) usar `<Modal>` de Mantine v7 para consistencia con slices 2+2.1 (que usan notifications). **DECIDIR en design**. |
| R11 | LOC est. ~830 excede 400-line budget | **High** | Medium | `size:exception` — precedente 10 PRs consecutivos aprobados en este repo (obs `#220/#228/#238`). Aplicar misma estrategia. |
| R12 | `user_warning_state.warning_count > 0` puede traer usuarios cuya última acción fue hace 30+ días (expirados pero no reseteados) | Medium | Low | Slice 1 diseñó `ResetExpiredWarnings` para correr en background (cron futuro). Por ahora la expiración es lógica: `last_warning_at < now - warning_expire_days` → reset en `HandleMessage` paso 5. Si el admin ve una fila con count=2 y last_warning_at=hace 60 días, sigue siendo válido mostrarla — el admin puede usar Reset manual. Documentado en UI. |

---

## Dependency order

```
Slice 1 (Foundation — archivado) ──→ Slice 2 (Rules + Settings UI — archivado) ──→ Slice 2.1 (Warning visual — archivado) ──→ Slice 3 (Dashboard — ESTE)
       │                                       │                                          │                                       │
       └─ user_warning_state tabla             ├─ banned_words / link_allowlist           ├─ warn_user_enabled/template            └─ READ-only endpoints + UI
       └─ logs.ActionRuleTriggered/Automute    └─ 9 endpoints automation                 └─ Section 5 frontend                     └─ 3 endpoints + nueva página
                                                 └─ GroupAutomationPage                      └─ templates.go                              └─ LEFT JOIN users
```

- **Slice 3 standalone-dependency**: solo lee datos. Si el sistema tiene slices 1+2+2.1 aplicados, slice 3 funciona. Si slices 1+2+2.1 NO están aplicados, slice 3 no tiene sentido (no hay warning_state ni logs de automation que mostrar).
- **No introduce cambios** a `Service.HandleMessage`, `Worker`, `AutoActioner`, `WarningSender`, ni a las reglas. Cero impacto en el pipeline de evaluación.

---

## Open Questions (a resolver en proposal/design, no bloqueantes para explore)

1. **¿Crear índice `(group_id, action, created_at)` en `logs`?** → depende de R2. **Recomendación**: empezar sin índice; medir con EXPLAIN ANALYZE en dev; agregar migración 0009 si la query es >100ms en grupos típicos. **Resolver en design**.
2. **`SELECT ... FOR UPDATE` en el reset?** → depende de R5. **Recomendación**: empezar best-effort; si reporta el admin de inconsistencias raras, agregar lock. **Resolver en design**.
3. **¿Modal o `window.confirm` para el reset?** → R10. **Recomendación**: Modal (consistente con notifications de slices 2+2.1, mejor UX con i18n potencial). **Resolver en design**.
4. **¿Renombrar el botón "Configurar automatización" a "Configurar reglas de moderación"?** → D8. **Recomendación**: SÍ, para diferenciar del nuevo "Ver dashboard de moderación". Cambio de label = 1 línea. **Resolver en proposal**.
5. **¿Auto-poll en `useStats`?** → D9. **Recomendación**: NO. YAGNI. Si en el futuro se necesita, agregar `refetchInterval: 30_000` (2 líneas). **Resolver en design** (probablemente "no" sin más debate).
6. **¿Mostrar el action breakdown de stats por regla (`rule_triggered` desglosado en `flood`/`anti_spam`/`anti_link`/`banned_words`)?** → Spec dice solo `RULE_TRIGGERED` aggregate. **Recomendación**: NO — el breakdown requiere parsear metadata JSONB, costoso y ya está en `GroupLogsPage` con filtro. **Resolver en design**.
7. **¿Endpoint `GET /api/groups/:id/automation/warnings/export.csv`?** → exploración `#215` D7 mencionaba "filtros por estado" pero slice 1 no lo implementó. **Recomendación**: NO en este slice (CSV = follow-up; YAGNI; la tabla es suficiente para inspección). **Resolver en design**.
8. **¿Paginación en `/warnings`?** → R8 dice "1000 advertencias activas" es peor caso. **Recomendación**: NO en MVP — si el admin tiene 1000+ advertencias activas simultáneamente, hay un problema más grande (las reglas están mal calibradas). Limitar a top 100 + warning "mostrando 100 de N" si supera. **Resolver en design** con cap defensivo.

---

## Ready for Proposal

**Sí**. Resumen para el usuario:

> **Slice 3 — Warnings Dashboard** (Fase 3 — AGENTS §23, último slice):
>
> 1. **Backend (~280 LOC new + 80 LOC mod)**:
>    - `logs.Repository.CountByActionAndGroup(ctx, groupID, actions, since)` — agregado SQL `COUNT(*) GROUP BY action`.
>    - `automation.Repository.ListActiveWarningStatesByGroup(ctx, groupID)` — LEFT JOIN con `users` para display name.
>    - 3 handlers nuevos en `automation_handlers.go`: `GET /automation/warnings`, `POST /automation/warnings/:user_id/reset`, `GET /automation/stats?period=24h|7d`.
>    - Constante nueva `ActionResetWarnings = "RESET_WARNINGS"` (admin action, ActorID del admin).
> 2. **Frontend (~420 LOC new + 50 LOC mod)**:
>    - `GroupModerationPage.tsx` nueva en `/groups/:id/moderation`.
>    - 3 stat cards (Rule triggered / Auto-mute / Auto-ban) con period selector 24h/7d.
>    - Tabla de advertencias activas con display name (LEFT JOIN) + botón Reset.
>    - Modal de confirmación para el Reset.
>    - Link desde `GroupDetailPage` → "Ver dashboard de moderación".
> 3. **Tests** (§21.1 estricto — cero llamadas Bot API):
>    - Backend integration: `CountByActionAndGroup` (4 casos) + `ListActiveWarningStatesByGroup` (3 casos).
>    - Backend handler: 6 casos cubriendo happy + error + auth.
>    - Frontend: `GroupModerationPage.test.tsx` (5+2 tests) + `GroupDetailPage.test.tsx` MOD (1 test).
> 4. **No regresión**: slices 1+2+2.1 intactos; `backend/internal/moderation/` intacto; pipeline de evaluation NO tocado; `permissionOkAdmin` invariante preservado (este slice NO introduce checks nuevos).
>
> **Confirmaciones**: ninguna nueva sobre la Bot API — slice 3 es 100% queries sobre `user_warning_state` + `logs` + `users` (tablas propias).
>
> **LOC est.**: ~830. Excede budget → `size:exception` (precedente 10 PRs consecutivos aprobados en este repo).
>
> **Strategy**: single-pr con `size:exception`.
>
> ¿Procedemos con `sdd-propose`?

---

## What / Why / Where / Learned

**What**: Exploración completa del slice 3 (Warnings Dashboard) de Fase 3 — última entrega de AGENTS §23. Define 3 endpoints backend (read-only + 1 reset opcional), 2 repository methods nuevos, 1 página frontend nueva, 1 link adicional en GroupDetailPage.

**Why**: Slices 1+2+2.1 producen todos los datos necesarios (`user_warning_state` + logs de automation); el admin hoy no tiene UI para observarlos. Slice 3 cierra el ciclo de feedback: el admin puede ver quién está cerca del threshold, cuánto se está moderando, y resetear manualmente si quiere.

**Where**:
- `openspec/changes/moderation-automation/slice3/exploration.md` (este filesystem)
- Engram `sdd/moderation-automation/slice3/exploration` (próximo `mem_save`)

**Learned**:
- Cero llamadas Bot API en este slice → §21.1 se respeta trivialmente (no se necesita mock nuevo).
- La tabla `users` ya está poblada parcialmente (main.go:209-216 la llena en `chat_join_request`). LEFT JOIN es la solución pragmática para display names sin escanear logs.
- Slice 1 ya dejó `Repository.ListWarningStates` y `Repository.ResetExpiredWarnings` listos para "dashboard de slice 3" — slice 3 solo agrega el variant con `warning_count > 0` ordenado + LEFT JOIN, sin reescribir lo que ya existe.
- Los 9 endpoints slice 2 + 3 endpoints slice 3 = 12 endpoints totales en `WithAutomation` option. Sigue siendo un option pequeño; el file `automation_handlers.go` pasa de 572 LOC a ~720 LOC (manejable).
- El botón "Reset" en el dashboard es la única mutación nueva. Es opcional pero útil (admin perdona después de un clean streak). Costo: 1 endpoint + 1 action constant + 1 Modal en frontend.