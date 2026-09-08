# Delta Spec: Moderation Automation — Slice 3 (Warnings Dashboard UI + Auto-Action Stats + Manual Reset)

> **Change**: `moderation-automation-slice3` — Slice 3/3 (último) de **Fase 3 — Moderación Automática** (AGENTS §23).
> **Mode**: hybrid (filesystem + Engram).
> **Predecessor**: slice 2.1 archivado (`main @ e7d0680`); canónico `openspec/specs/moderation-automation/spec.md` (REQ-1..REQ-31).
> **Branch base**: `main @ e7d0680` (slice 2.1 ya mergeado).
> **Strategy**: single-pr con `size:exception` (precedente 10 PRs consecutivos — `tasks` re-confirmará ~830 LOC).
> **Authority**: bugfix `#172` (`permissionOkAdmin` usa `BotStatus == StatusAdministrator`, NUNCA `can_*`); AGENTS §11, §14, §17.1, §18.1, §21.1.
>
> Esta spec **amenda** el canónico de slice 2.1 vía `## ADDED Requirements` (REQ-32 a REQ-40). Archive usa la técnica APPEND de slices 1→2→2.1 (delta → canónico; REQ-1..REQ-31 previos preservados, REQ-32..REQ-40 nuevos appendeados). No se crea spec paralelo. Cero migración nueva (queries sobre tablas existentes).

## Slice 3 ADDED Requirements (2026-09-08 — Warnings Dashboard UI + Stats + Manual Reset)

### Requirement: Backend GET `/api/groups/{id}/automation/warnings` (lista de advertencias activas)

El backend MUST exponer `GET /api/groups/{id}/automation/warnings` que devuelve `200 {"warnings": [...]}` con la lista de usuarios que tienen `warning_count > 0` en el grupo. El handler MUST autenticarse con `requireAuth` y rechazar con `404 NOT_FOUND` si el módulo automation está deshabilitado (`s.automation == nil`) o si el grupo no existe. El handler MUST invocar `automation.Repository.ListActiveWarningStatesByGroup(ctx, groupID)` y el repositorio MUST ejecutar `SELECT uws.group_id, uws.user_id, uws.warning_count, uws.last_warning_at, uws.last_action_at, uws.expires_at, u.first_name, u.username FROM user_warning_state uws LEFT JOIN users u ON u.telegram_id = uws.user_id WHERE uws.group_id = $1 AND uws.warning_count > 0 ORDER BY uws.warning_count DESC, uws.last_warning_at DESC NULLS LAST LIMIT 100`. La respuesta MUST incluir, además del array, un campo booleano `truncated` en `true` si existen más de 100 filas activas (chequeo vía `SELECT COUNT(*) ... WHERE warning_count > 0` adicional, o comparando `len(warnings) == 100 && COUNT > 100`); si no, `false`. El LEFT JOIN a `users` es defensivo: si el usuario no tiene fila en `users` (entraron al grupo sin pasar por `chat_join_request`), `first_name` queda `""` y `username` queda `NULL`; el frontend aplica fallback a `"user {user_id}"`.

#### Scenario: 200 con shape esperado y LEFT JOIN

- GIVEN un grupo `g` con 2 usuarios con advertencias (`u1` con fila en `users {first_name:"Ana", username:"ana_p"}`, `u2` sin fila en `users`)
- WHEN el admin autenticado ejecuta `GET /api/groups/g/automation/warnings`
- THEN responde 200 con `data.warnings = [{user_id: u1, display_name:"Ana", username:"ana_p", warning_count:3, last_warning_at:"...", last_action_at:..., expires_at:...}, {user_id: u2, display_name:"", username:null, warning_count:1, ...}]` y `data.truncated = false`

#### Scenario: 404 si automation deshabilitado o grupo inexistente

- GIVEN el backend con `s.automation == nil` (no registrado) o `group_id` no presente en `groups`
- WHEN se ejecuta el endpoint autenticado
- THEN responde `404 NOT_FOUND` con mensaje legible

#### Scenario: 401 sin autenticación

- GIVEN request sin token JWT válido
- WHEN se ejecuta el endpoint
- THEN responde `401 UNAUTHORIZED` antes de tocar DB

#### Scenario: Cap 100 con `truncated=true` si hay más

- GIVEN un grupo con 150 usuarios con `warning_count > 0`
- WHEN se ejecuta el endpoint
- THEN el array `warnings` tiene exactamente 100 elementos ordenados por `warning_count DESC, last_warning_at DESC NULLS LAST` y `data.truncated = true`

#### Scenario: Filtra filas con `warning_count == 0`

- GIVEN un grupo con 5 usuarios: 2 con `warning_count > 0` y 3 con `warning_count = 0`
- WHEN se ejecuta el endpoint
- THEN el array `warnings` contiene exactamente los 2 con count > 0; las filas con count=0 NO aparecen

### Requirement: Backend POST `/api/groups/{id}/automation/warnings/{user_id}/reset` (reset manual)

El backend MUST exponer `POST /api/groups/{id}/automation/warnings/{user_id}/reset` que resetea manualmente el counter de advertencias de un usuario. El handler MUST autenticarse con `requireAuth` y rechazar con `404 NOT_FOUND` si el módulo automation está deshabilitado o si el grupo no existe. El handler MUST invocar `automation.Repository.ResetWarningState(ctx, groupID, userID)` (nuevo método del repo: `UPDATE user_warning_state SET warning_count = 0, last_warning_at = NULL, last_action_at = NULL, expires_at = NULL WHERE group_id = $1 AND user_id = $2`); si la fila no existía (`RowsAffected() == 0`) responde `404 NOT_FOUND` con mensaje `"no hay advertencias para este usuario en este grupo"`. Si la fila existía y el `warning_count` previo era > 0, el handler MUST emitir un log `ActionResetWarnings = "RESET_WARNINGS"` con `ActorID` del admin (no `nil`, distinción de slice 1 auto-actions) y `metadata = {"user_id": <int>, "warning_count_before_reset": <int>}` (incluye el valor previo leído antes del UPDATE). La respuesta exitosa MUST ser `200 {"user_id": <int>, "warning_count": 0, "reset": true}`.

#### Scenario: Reset exitoso emite log con metadata de auditoría

- GIVEN un grupo `g` con `user_warning_state` para `(g, u1)` con `warning_count=3`
- WHEN el admin autenticado ejecuta `POST /api/groups/g/automation/warnings/u1/reset`
- THEN la fila tiene `warning_count=0` y timestamps `NULL`; existe log con `action="RESET_WARNINGS"`, `actor_id=<admin>`, `metadata={"user_id":u1, "warning_count_before_reset":3}`, `status=SUCCESS`; responde 200 con `{user_id:u1, warning_count:0, reset:true}`

#### Scenario: 404 si no existe fila

- GIVEN el grupo `g` NO tiene fila en `user_warning_state` para `(g, u999)`
- WHEN el admin ejecuta el POST
- THEN responde `404 NOT_FOUND` con `"no hay advertencias para este usuario en este grupo"`; NO se emite log

#### Scenario: 401 sin auth

- GIVEN request sin token válido
- WHEN se ejecuta el POST
- THEN responde `401 UNAUTHORIZED` sin tocar DB

#### Scenario: Reset NO desmutear al usuario

- GIVEN un usuario muteado en Telegram (status: restricted) con `warning_count=2`
- WHEN el admin ejecuta el reset
- THEN el counter queda en 0 pero el usuario sigue muteado en Telegram (reset SOLO limpia DB; el admin usa `POST /api/groups/{id}/users/{userId}/unmute` por separado si quiere desmutear)

### Requirement: Backend GET `/api/groups/{id}/automation/stats?period=24h|7d` (agregaciones)

El backend MUST exponer `GET /api/groups/{id}/automation/stats` que devuelve conteos agregados de logs de auto-moderación del grupo en una ventana temporal. El handler MUST autenticarse con `requireAuth` y rechazar con `404 NOT_FOUND` si automation está deshabilitado o el grupo no existe. Query params: `period` ∈ `{"24h", "7d"}` (default `24h` si ausente); cualquier otro valor responde `400 VALIDATION_ERROR` con mensaje `"period invalido (use 24h o 7d)"`. El handler MUST invocar `logs.Repository.CountByActionAndGroup(ctx, groupID, []string{"RULE_TRIGGERED", "AUTOMUTE_USER", "AUTOBAN_USER"}, now - periodDuration)` y retornar `200 {"rule_triggered": N, "automute": M, "autoban": K, "period": "24h"|"7d"}` donde cada contador es la cantidad de logs con `action=...` y `group_id=<groupID>` y `created_at >= now - periodDuration` (en UTC). El cálculo de `periodDuration` MUST ser `24 * time.Hour` para `"24h"` y `7 * 24 * time.Hour` para `"7d"`. Si no hay logs en la ventana, los contadores retornan `0` (no error). El método del repo MUST ejecutar `SELECT action, COUNT(*) FROM logs WHERE group_id = $1 AND action = ANY($2) AND created_at >= $3 GROUP BY action` en 1 roundtrip (array `ANY` con los 3 actions bounded).

#### Scenario: Default 24h sin query param

- GIVEN un grupo con 12 `RULE_TRIGGERED`, 3 `AUTOMUTE_USER`, 1 `AUTOBAN_USER` en últimas 24h; 100 `RULE_TRIGGERED` de hace 3 días
- WHEN se ejecuta `GET /api/groups/g/automation/stats` (sin `period`)
- THEN responde 200 con `data={rule_triggered:12, automute:3, autoban:1, period:"24h"}`; los logs viejos NO se cuentan

#### Scenario: Period=7d extiende la ventana

- GIVEN los mismos logs anteriores (12/3/1 en 24h + 100/0/0 en 3 días)
- WHEN se ejecuta `GET .../stats?period=7d`
- THEN responde 200 con `data={rule_triggered:112, automute:3, autoban:1, period:"7d"}`

#### Scenario: 400 si period inválido

- GIVEN request con `?period=foo` o `?period=12h`
- WHEN se ejecuta el endpoint
- THEN responde `400 VALIDATION_ERROR` con mensaje `"period invalido (use 24h o 7d)"`; NO se toca DB

#### Scenario: 404 si automation nil o grupo no existe

- GIVEN backend sin módulo automation o `group_id` inexistente
- WHEN se ejecuta el endpoint autenticado
- THEN responde `404 NOT_FOUND`

#### Scenario: 401 sin autenticación

- GIVEN request sin token válido
- WHEN se ejecuta el endpoint
- THEN responde `401 UNAUTHORIZED`

### Requirement: Frontend `GroupModerationPage` en `/groups/:id/moderation`

El frontend MUST renderizar `GroupModerationPage` en la ruta `/groups/:id/moderation` (registrada en `App.tsx` con `RequireAuth`). La página MUST mostrar 2 secciones (sin Save button — es read-only + reset):

- **Sección 1 — "Estadísticas"**: 3 stat cards (Mantine v7 `<Card>` + `<Text>` grande para el contador) en fila horizontal con labels "Reglas disparadas", "Auto-mute", "Auto-ban" usando `useStats(groupId, period)`. Arriba a la derecha: `<Select>` Mantine v7 con opciones `[{value:"24h",label:"Últimas 24 horas"}, {value:"7d",label:"Últimos 7 días"}]` (default `24h`) y `<Button>` "Refrescar" que llama `queryClient.invalidateQueries(['automation','warnings',groupId])` + `['automation','stats',groupId,period]`. La sección MUST incluir un loading skeleton mientras `useStats` está pending.
- **Sección 2 — "Advertencias activas"**: `<Table>` Mantine v7 con columnas: **User** (texto con display_name si no vacío, sino `"user {user_id}"` dimmed; debajo `id: <user_id>` dimmed pequeño), **Warnings** (número formateado como badge), **Última advertencia** (formato relativo, ej. "hace 2h" usando Intl.RelativeTimeFormat), **Acciones** (`<Button size="xs" variant="default" leftSection={<IconRefresh/>}>` con texto "Reset" + `data-testid="warning-reset-{user_id}"`). La fuente MUST ser `useWarnings(groupId)`; mientras pending, mostrar skeleton rows. Si `data.warnings.length === 0`, mostrar `<Text c="dimmed" ta="center">No hay advertencias activas</Text>`. Si `data.truncated === true`, mostrar `<Alert color="yellow">Mostrando las primeras 100 advertencias activas</Alert>` arriba de la tabla. Click en "Reset" abre `<Modal>` Mantine v7 con título "Resetear advertencias" + texto `¿Resetear las advertencias de {display_name}?` y 2 botones: "Cancelar" (cierra) y "Resetear" (dispara `useResetWarning.mutateAsync({groupId, userId})`; al éxito cierra el modal e invalida `['automation','warnings',groupId]`; al error muestra `notifyError(formatAutomationError(err))`). Sin auto-poll — refresh es on-demand vía botón (YAGNI para MVP).

#### Scenario: Render inicial con stats y warnings cargados

- GIVEN `mockFetchRoutes` configurado para devolver `/warnings` → `{warnings:[{user_id:1, display_name:"Ana", warning_count:3, last_warning_at:"2026-09-08T10:00:00Z"}, {user_id:2, display_name:"", warning_count:1, last_warning_at:null}], truncated:false}` y `/stats?period=24h` → `{rule_triggered:12, automute:3, autoban:1, period:"24h"}`
- WHEN se renderiza la página en `/groups/123/moderation`
- THEN las 3 stat cards muestran 12, 3, 1; la tabla muestra 2 filas (Ana con badge "3" y "hace Xh", user 2 con badge "1" y "—"); cada fila tiene su botón Reset

#### Scenario: Cambiar period a 7d dispara nueva query

- GIVEN la página cargada con stats 24h
- WHEN el admin cambia el Select a "Últimos 7 días"
- THEN se ejecuta `GET /api/groups/123/automation/stats?period=7d`; las stat cards se actualizan con la nueva respuesta

#### Scenario: Click Refrescar invalida ambas queries

- GIVEN stats y warnings cargados
- WHEN el admin hace click en "Refrescar"
- THEN se disparan 2 requests: `GET .../automation/warnings` y `GET .../automation/stats?period=<actual>`; el periodo actual se preserva en la URL de stats

#### Scenario: Empty state cuando no hay advertencias

- GIVEN respuesta `/warnings` con `{warnings:[], truncated:false}`
- WHEN se renderiza la página
- THEN la tabla muestra `<Text c="dimmed" ta="center">No hay advertencias activas</Text>`; NO se renderiza `<Table>` vacía

#### Scenario: Truncation alert cuando hay >100 advertencias

- GIVEN respuesta `/warnings` con 100 elementos y `truncated:true`
- WHEN se renderiza la página
- THEN arriba de la tabla aparece `<Alert color="yellow">Mostrando las primeras 100 advertencias activas</Alert>`; la tabla renderiza solo las 100 filas

#### Scenario: Display name fallback cuando users vacío

- GIVEN respuesta `/warnings` con un usuario sin fila en `users` (display_name="" y username=null)
- WHEN se renderiza la tabla
- THEN la columna User muestra `"user {user_id}"` con `id: <user_id>` dimmed debajo

#### Scenario: Reset abre Modal y al confirmar dispara POST + invalida cache

- GIVEN la página con warnings cargados; `mockFetchRoutes` para `POST /warnings/{user_id}/reset` configurado con respuesta 200
- WHEN el admin hace click en "Reset" sobre la fila de Ana y luego click en "Resetear" en el Modal
- THEN se ejecuta `POST /api/groups/123/automation/warnings/1/reset`; al éxito el modal cierra, `useWarnings` se invalida y refetchea automáticamente; la fila de Ana desaparece (count reseteado)

#### Scenario: Reset cancelado NO dispara POST

- GIVEN el Modal abierto tras click en "Reset"
- WHEN el admin hace click en "Cancelar" del Modal
- THEN el modal cierra; NO se ejecuta ningún POST; la fila sigue visible

### Requirement: Extensiones al feature module `automation/` (frontend)

El frontend MUST extender `features/automation/` con tipos, API functions y hooks nuevos consistentes con el patrón slice 2+2.1:

- **`types.ts` MOD**: agregar `WarningState { user_id: number, display_name: string, username: string|null, warning_count: number, last_warning_at: string|null, last_action_at: string|null, expires_at: string|null }`, `WarningsResponse { warnings: WarningState[], truncated: boolean }`, `StatsPeriod = "24h"|"7d"`, `AutomationStats { rule_triggered: number, automute: number, autoban: number, period: StatsPeriod }`.
- **`api.ts` MOD**: agregar `listWarnings(groupId: number): Promise<WarningsResponse>` (`GET /api/groups/{groupId}/automation/warnings`), `resetWarning(groupId: number, userId: number): Promise<{user_id:number, warning_count:0, reset:true}>` (`POST /api/groups/{groupId}/automation/warnings/{userId}/reset`), `getStats(groupId: number, period: StatsPeriod): Promise<AutomationStats>` (`GET /api/groups/{groupId}/automation/stats?period={period}`).
- **`hooks.ts` MOD**: agregar `useWarnings(groupId)` (`useQuery` con queryKey `['automation','warnings',groupId]`, retry:false), `useResetWarning(groupId)` (`useMutation` que invalida `['automation','warnings',groupId]` en `onSuccess`), `useStats(groupId, period)` (`useQuery` con queryKey `['automation','stats',groupId,period]`, retry:false, sin `refetchInterval`). Sin auto-poll — refresh es on-demand (el caller usa `queryClient.invalidateQueries` desde el botón "Refrescar").
- **`error.ts` MOD**: `formatAutomationError` ya maneja errores genéricos; agregar rama específica para `404 NOT_FOUND` (devuelve `"no hay advertencias activas"`) y `400 VALIDATION_ERROR` (devuelve el mensaje del backend si existe, sino `"parámetros inválidos"`).

#### Scenario: useWarnings retorna lista con truncated

- GIVEN `queryClient` configurado con `mockFetchRoutes` para `/warnings` → `{warnings:[...], truncated:true}`
- WHEN se invoca `useWarnings(123)` desde un componente con `QueryClientProvider`
- THEN `data.warnings` es la lista y `data.truncated === true`; `isLoading` es `false`

#### Scenario: useResetWarning invalida cache al éxito

- GIVEN cache con `['automation','warnings',123]` poblado
- WHEN se invoca `mutation.mutate(1)` y el POST responde 200
- THEN `queryClient` invalida `['automation','warnings',123]`; un `useWarnings(123)` subsiguiente dispara refetch

#### Scenario: useStats cambia queryKey con period

- GIVEN `useStats(123, "24h")` y `useStats(123, "7d")` ambos invocados
- WHEN el componente los consume
- THEN son 2 queries independientes con keys distintas (`['automation','stats',123,'24h']` y `['automation','stats',123,'7d']`); cambiar period dispara nuevo fetch solo para la key afectada

### Requirement: `GroupDetailPage` link + label update

El frontend MUST modificar `GroupDetailPage.tsx` Tab "Detalle" así:

1. Renombrar el botón existente "Configurar automatización" → **"Configurar reglas de moderación"** (mantiene `to={`/groups/${groupId}/automation`}` y `data-testid="automation-link"`).
2. Agregar debajo un nuevo `<Button component={Link} variant="light" w={260} data-testid="moderation-dashboard-link" to={`/groups/${groupId}/moderation`}>` con texto "Ver dashboard de moderación".

Ambos botones conviven en la misma sección; el primero lleva a la página de settings (writes, slice 2), el segundo al dashboard de observación (reads, slice 3). El cambio de label es solo de UX para diferenciar las dos intenciones (settings vs dashboard); no cambia comportamiento del botón.

#### Scenario: Label actualizado + nuevo botón en `GroupDetailPage`

- GIVEN `GroupDetailPage` renderizado para el grupo 123 con tab "Detalle" activo
- WHEN se inspeccionan los botones de la sección de moderación
- THEN existe botón con texto "Configurar reglas de moderación" + `href="/groups/123/automation"` + `data-testid="automation-link"`; y debajo, botón con texto "Ver dashboard de moderación" + `href="/groups/123/moderation"` + `data-testid="moderation-dashboard-link"`

### Requirement: Action constant `RESET_WARNINGS`

`backend/internal/logs/model.go` MUST exponer la constante `ActionResetWarnings = "RESET_WARNINGS"`. Esta acción es de **tipo admin manual** (ActorID no nulo, igual que las 5 constantes de slice 2); la distingue de las auto-actions de slice 1 (`ActorID=nil`). El handler del endpoint de reset MUST registrar el log con `metadata = {"user_id": <int>, "warning_count_before_reset": <int>}` donde `warning_count_before_reset` es el valor del counter leído antes del UPDATE (capturado en una transacción o en un SELECT previo; el design define el mecanismo exacto).

#### Scenario: Constante existe

- GIVEN el paquete `logs` compilado
- WHEN se importa `logs.ActionResetWarnings`
- THEN existe con el valor string `"RESET_WARNINGS"`

#### Scenario: Log manual con ActorID no nulo

- GIVEN un admin con id `42` ejecuta el reset
- WHEN el handler persiste el log
- THEN `entry.ActorID != nil` (`actor_id = 42`); `entry.Action == "RESET_WARNINGS"`; `entry.Metadata["user_id"]` y `entry.Metadata["warning_count_before_reset"]` están poblados con los valores esperados

### Requirement: Tests §21.1 estricto (cero Bot API real)

Los tests MUST cubrir (§21.1 — cero llamadas a la Bot API; los tests del adapter Telegram ya cubren esa parte):

- **Backend `logs/repository_test.go` MOD (+90 LOC)**: 4+ integration tests para `CountByActionAndGroup`:
  1. Happy path: inserta logs `RULE_TRIGGERED` (5), `AUTOMUTE_USER` (2), `AUTOBAN_USER` (1), `BAN_USER` (3, no contado) en últimas 24h; verifica `map["RULE_TRIGGERED"]==5`, `map["AUTOMUTE_USER"]==2`, `map["AUTOBAN_USER"]==1`, `len(map)==3`.
  2. Filtro `since`: inserta 5 logs hace 48h y 5 logs hace 1h; con `since = now - 24h` retorna solo los 5 recientes.
  3. Filtro `actions`: invoca con `actions = ["RULE_TRIGGERED"]`; retorna solo el conteo de esa action (los demás no aparecen en el map).
  4. Empty: grupo sin logs → retorna `map[string]int{}` (no nil, no error).
- **Backend `automation/repository_test.go` MOD (+60 LOC)**: 3+ integration tests para `ListActiveWarningStatesByGroup`:
  1. Filtra `warning_count=0`: inserta 3 filas (count=3, count=0, count=1); retorna solo 2 (las de count>0).
  2. LEFT JOIN preserva fila sin `users`: inserta fila en `user_warning_state` sin fila en `users`; retorna `WarningStateWithUser{FirstName:"", Username:nil}` (no panic, no error).
  3. Orden por count DESC + last_warning_at DESC NULLS LAST: inserta 3 filas (count=2/last=NULL, count=2/last=t1, count=1/last=t2); orden resultante es `[count=2/last=t1, count=2/last=NULL, count=1/last=t2]`.
- **Backend `automation_handlers_test.go` MOD (+150 LOC)**: 6+ handler tests con fakes (sin DB ni Telegram):
  1. `GET /warnings` 200 con shape LEFT JOIN visible.
  2. `GET /warnings` 404 cuando `automation == nil`.
  3. `POST /warnings/{user_id}/reset` 200 + log emitido con `ActionResetWarnings` + metadata correcta + `actor_id` del admin.
  4. `POST /warnings/{user_id}/reset` 404 cuando la fila no existe (fake repo retorna `RowsAffected == 0`).
  5. `GET /stats?period=24h` 200 con shape correcto; default `24h` si ausente.
  6. `GET /stats?period=foo` → 400 `VALIDATION_ERROR` con mensaje legible.
  7. Auth required (401 sin token) en los 3 endpoints.
- **Frontend `GroupModerationPage.test.tsx` NEW (+180 LOC)**: 5+ casos principales + 2 smoke:
  1. Render inicial muestra stats + warnings correctos.
  2. Period selector `7d` cambia la query.
  3. Empty state cuando `warnings=[]`.
  4. Truncation alert cuando `truncated=true`.
  5. Reset flow: click en botón Reset abre Modal, click en "Resetear" dispara POST + cierra modal + invalida cache.
  6. (smoke) Error del backend muestra `notifyError` con mensaje legible.
  7. (smoke) Display name fallback a `"user {user_id}"` cuando display_name vacío.
- **Frontend `GroupDetailPage.test.tsx` MOD (+20 LOC)**: 1 caso verificando que el botón "Ver dashboard de moderación" existe con `data-testid="moderation-dashboard-link"` + `href=/groups/:id/moderation`; y que el botón existente ahora tiene label "Configurar reglas de moderación".
- **Frontend `test/helpers.tsx` MOD (+10 LOC)**: extender `mockFetchRoutes` con substring routes nuevas (`/automation/warnings`, `/automation/stats`).

#### Scenario: Test logs repo — happy path CountByActionAndGroup

- GIVEN Postgres real con migración 00006; inserta 5 `RULE_TRIGGERED`, 2 `AUTOMUTE_USER`, 1 `AUTOBAN_USER`, 3 `BAN_USER` (no contados) en últimas 24h
- WHEN `CountByActionAndGroup(ctx, groupID, ["RULE_TRIGGERED","AUTOMUTE_USER","AUTOBAN_USER"], now-24h)` corre
- THEN retorna `map[string]int{"RULE_TRIGGERED":5, "AUTOMUTE_USER":2, "AUTOBAN_USER":1}`

#### Scenario: Test handler — 400 en period inválido

- GIVEN handler con fake automationService
- WHEN se ejecuta `GET /api/groups/-100/automation/stats?period=foo`
- THEN responde `400 VALIDATION_ERROR` con `"period invalido (use 24h o 7d)"`

#### Scenario: Test frontend — render inicial de GroupModerationPage

- GIVEN `mockFetchRoutes` con `/warnings` y `/stats?period=24h`
- WHEN se renderiza la página con `renderWithProviders`
- THEN los 3 contadores (12/3/1) aparecen; la tabla tiene 2 filas con sus botones Reset; el Modal NO está abierto

#### Scenario: Test frontend — GroupDetailPage tiene los 2 botones

- GIVEN `GroupDetailPage` renderizado para grupo 123 con tab "Detalle"
- WHEN se inspecciona la sección de moderación
- THEN existe `getByTestId("automation-link")` con texto "Configurar reglas de moderación"; y `getByTestId("moderation-dashboard-link")` con texto "Ver dashboard de moderación" + href `/groups/123/moderation`

### Requirement: Non-regression (slices 1+2+2.1 + páginas frontend + invariantes)

El slice 3 MUST preservar íntegramente:

- Pipeline de slices 1+2+2.1: `backend/internal/automation/{service,worker,autoactioner,warning_sender,rules,templates}.go` intactos (cero modificaciones en HandleMessage, Registry, WarningSender). Las funciones de slice 3 son NUEVAS en archivos existentes (`repository.go`, `model.go`) o handlers NUEVOS en `automation_handlers.go`; no se reemplaza lógica previa.
- `backend/internal/moderation/` intacto (acciones manuales: ban/unban/mute/unmute/delete/pin/lock/unlock/approve/reject) — bugfix `#172` sigue fuera de scope.
- `permissionOkAdmin` invariante en automation: solo `BotStatus == StatusAdministrator`, NUNCA claves `can_*` (verificación: `grep -rn "can_" backend/internal/automation/` retorna 0 matches en código executable).
- `backend/internal/publications/` intacto.
- Frontend pages intactas: `DashboardPage`, `PublicationsPage`, `LoginPage`, `GroupsPage`, `GroupUsersPage`, `GroupRequestsPage`, `GroupLogsPage` no se modifican. `GroupAutomationPage` queda intacta (settings editor separado del nuevo dashboard). Solo `GroupDetailPage` se modifica (label update + 1 botón nuevo).
- Migración nueva: 0 (slice 3 es 100% queries sobre tablas existentes).
- §21.1: cero llamadas Bot API reales en tests (audit en `verify`).
- §25 sin secretos en el repo.
- §18.1 rate limit del adapter Telegram intacto (slice 3 no introduce calls Telegram nuevas).

#### Scenario: Pipeline automation intacto

- GIVEN el branch `feat/moderation-automation-slice3`
- WHEN `git diff main -- backend/internal/automation/service.go backend/internal/automation/worker.go backend/internal/automation/autoactioner.go backend/internal/automation/warning_sender.go backend/internal/automation/rules.go backend/internal/automation/templates.go` corre
- THEN el output está vacío (estos archivos no se tocan; las adiciones viven en `model.go`, `repository.go`, `api/automation_handlers.go` y archivos de test)

#### Scenario: `moderation.Service` no modificado

- GIVEN el branch del slice
- WHEN `git diff main -- backend/internal/moderation/` corre
- THEN el output está vacío (cero líneas modificadas en ese paquete)

#### Scenario: Páginas frontend preexistentes intactas

- GIVEN el branch del slice
- WHEN `git diff main -- frontend/src/pages/` excluyendo `GroupModerationPage.tsx` (NEW) y `GroupDetailPage.tsx` (MOD) corre
- THEN el output está vacío

#### Scenario: permissionOkAdmin invariante

- GIVEN el branch del slice
- WHEN `grep -rn "can_" backend/internal/automation/` corre
- THEN no aparecen checks sobre claves `can_*` (solo `permissionOkAdmin` con `BotStatus == StatusAdministrator`)

#### Scenario: Sin secretos en repo

- GIVEN el branch del slice
- WHEN `git diff main` busca `TELEGRAM_BOT_TOKEN`, `JWT_SECRET`, passwords hardcoded
- THEN no aparecen literales de secretos en código

## Decisions taken (vs canonical)

- **Spec sync method**: AMEND via ADDED Requirements en delta file. Archive APPEND technique (mismo patrón que slice 1 → slice 2 → slice 2.1 archive).
- **Numeración**: REQ-32..40 para evitar colisión con REQ-1..6 (slice 1 foundation), REQ-7..21 (slice 2 settings UI) y REQ-22..31 (slice 2.1 warnings visuales).
- **Surface**: ruta nueva `/groups/:id/moderation` (NO Sección 6 en `GroupAutomationPage`). Settings (writes) y dashboard (reads) son dos intenciones distintas; patrón slice 2 (`/groups/:id/automation`) ratifica la convención de rutas dedicadas.
- **3 endpoints backend** en `WithAutomation` (mismo gating `if s.automation == nil` que slice 2): GET `/warnings`, POST `/warnings/{user_id}/reset`, GET `/stats?period=24h|7d`. Auth: `requireAuth` (admin-facing; no bot-facing).
- **Permission check**: `requireAuth` + `actorIDFromClaims` (consistente con handlers de slice 2+2.1). NO se usa `permissionOkAdmin` (esos son checks sobre estado del bot; aquí no aplica).
- **Display name**: LEFT JOIN a `users ON users.telegram_id = user_warning_state.user_id`. Si no hay fila en `users`, `first_name=""` + `username=NULL` → fallback frontend `"user {user_id}"`.
- **Action constant nueva**: `ActionResetWarnings = "RESET_WARNINGS"`. Es admin action (`ActorID != nil`), metadata `{user_id, warning_count_before_reset}`.
- **Cap 100**: top 100 advertencias ordenadas por `warning_count DESC, last_warning_at DESC NULLS LAST`. Campo `truncated` boolean en respuesta si >100. YAGNI para paginación.
- **Period selector**: whitelist `"24h"|"7d"` (default `24h`). Cualquier otro valor → 400 VALIDATION_ERROR. Mapeo exacto: `24h = 24*time.Hour`, `7d = 7*24*time.Hour`.
- **Repository methods nuevos**: `ListActiveWarningStatesByGroup` (LEFT JOIN, WHERE count>0, ORDER BY count DESC, last_warning_at DESC NULLS LAST) y `ResetWarningState` (UPDATE SET count=0, last_warning_at=NULL, last_action_at=NULL, expires_at=NULL WHERE group_id AND user_id). También extender `automation.Repository` con `ResetWarningState` (single-row) — distinto del `ResetExpiredWarnings` ya existente (que es bulk por expires_at).
- **Logs aggregation**: `logs.Repository.CountByActionAndGroup(ctx, groupID, actions []string, since time.Time) (map[string]int, error)` — 1 roundtrip con `action = ANY($2)`.
- **Frontend refresh**: on-demand (button) sin `refetchInterval`. YAGNI auto-poll.
- **Reset UX**: Modal Mantine v7 con confirmación (consistente con notifications de slices 2+2.1; NO `window.confirm`).
- **GroupDetailPage label**: rename + nuevo botón (1 línea cambio de label + 1 botón nuevo, ambos en la misma sección).
- **Migración nueva**: 0 (queries sobre tablas existentes). R2 (¿índice `logs(group_id, action, created_at)`?) se decide en `design` midiendo `EXPLAIN ANALYZE` en dev.

## Coverage map

| Aspecto | REQ | Status |
|---------|-----|--------|
| GET /warnings 200 con LEFT JOIN visible | REQ-32 | ✅ |
| GET /warnings 404 automation nil | REQ-32 | ✅ |
| GET /warnings 401 sin auth | REQ-32 | ✅ |
| GET /warnings cap 100 + truncated | REQ-32 | ✅ |
| GET /warnings filtra count=0 | REQ-32 | ✅ |
| POST reset 200 + log con metadata | REQ-33 | ✅ |
| POST reset 404 si no existe fila | REQ-33 | ✅ |
| POST reset 401 sin auth | REQ-33 | ✅ |
| POST reset NO desmutear | REQ-33 | ✅ |
| GET stats 200 period=24h default | REQ-34 | ✅ |
| GET stats 200 period=7d | REQ-34 | ✅ |
| GET stats 400 period inválido | REQ-34 | ✅ |
| GET stats 404 + 401 | REQ-34 | ✅ |
| GroupModerationPage render inicial | REQ-35 | ✅ |
| GroupModerationPage period selector 7d | REQ-35 | ✅ |
| GroupModerationPage refresh on-demand | REQ-35 | ✅ |
| GroupModerationPage empty state | REQ-35 | ✅ |
| GroupModerationPage truncation alert | REQ-35 | ✅ |
| GroupModerationPage display name fallback | REQ-35 | ✅ |
| GroupModerationPage reset flow Modal | REQ-35 | ✅ |
| GroupModerationPage cancel NO dispara POST | REQ-35 | ✅ |
| Frontend types (WarningState, Stats, etc.) | REQ-36 | ✅ |
| Frontend API (listWarnings, resetWarning, getStats) | REQ-36 | ✅ |
| Frontend hooks (useWarnings, useResetWarning, useStats) | REQ-36 | ✅ |
| Frontend error.ts 404 + 400 ramas | REQ-36 | ✅ |
| GroupDetailPage label update + nuevo botón | REQ-37 | ✅ |
| ActionResetWarnings constante + ActorID + metadata | REQ-38 | ✅ |
| Backend tests CountByActionAndGroup (4 casos) | REQ-39 | ✅ |
| Backend tests ListActiveWarningStatesByGroup (3 casos) | REQ-39 | ✅ |
| Backend tests handlers (200/400/404/401) | REQ-39 | ✅ |
| Frontend tests GroupModerationPage (5+2 smoke) | REQ-39 | ✅ |
| Frontend tests GroupDetailPage link | REQ-39 | ✅ |
| §21.1 cero Bot API real | REQ-39 | ✅ |
| Pipeline automation intacto | REQ-40 | ✅ |
| moderation/ intacto | REQ-40 | ✅ |
| Pages frontend preexistentes intactas | REQ-40 | ✅ |
| permissionOkAdmin invariante | REQ-40 | ✅ |
| §25 sin secretos | REQ-40 | ✅ |
| Migración 0 nueva | REQ-40 | ✅ |

**Total**: 9 REQ + ~33 scenarios. Happy paths + edge cases + error states cubiertos.

## What

Wrote delta spec REQ-32..40 (9 requirements, ~33 scenarios) amending `moderation-automation` canonical via ADDED Requirements. Backend read-only endpoints (warnings list + stats) + 1 admin reset mutation + 2 repository methods (LEFT JOIN + CountByActionAndGroup) + 1 frontend page nueva (GroupModerationPage) + 1 link update en GroupDetailPage + ActionResetWarnings constant + tests §21.1 estricto + non-regression completo.

## Why

User's orchestrator requested spec phase for slice 3 with REQ-32..40 numbering (avoiding collision with REQ-1..6 slice 1, REQ-7..21 slice 2, REQ-22..31 slice 2.1). Slices 1+2+2.1 ya producen todos los datos (`user_warning_state` + logs de auto-moderación); admin hoy no tiene UI para observarlos. Slice 3 cierra el ciclo de feedback con 3 endpoints (1 reset opcional, 2 read-only).

## Where

- `openspec/changes/moderation-automation/slice3/specs/moderation-automation/spec.md` (delta file, este documento)
- Engram `sdd/moderation-automation/slice3/spec` (artifact, topic_key upsert)

## Learned

- REQ numbering cross-slice debe trackearse con cuidado: REQ-1..6 = slice 1 (foundation), REQ-7..21 = slice 2 (settings UI), REQ-22..31 = slice 2.1 (warning visual). Slice 3 continúa REQ-32..40.
- "Slice X ADDED Requirements" pattern en canónico es el precedente para nombrar secciones nuevas en el canónico después del archive APPEND.
- `permissionOkAdmin` invariante (`BotStatus == StatusAdministrator`, NEVER `can_*`) es constraint cross-slice no-negociable — encoded explícitamente en REQ-40 con `grep` verification.
- Spec size budget: este archivo tiene ~500 LOC, dentro del rango aceptable para slices con muchos endpoints nuevos (slice 2 spec tuvo ~600 LOC).
- El reset endpoint es la única mutación nueva de slice 3. Costo: 1 endpoint + 1 action constant + 1 Modal en frontend. Útil para admin que perdona tras clean streak.
- LEFT JOIN a `users` para display name es la solución pragmática (tabla ya poblada parcialmente vía `chat_join_request`); no requiere parse JSONB de logs.
- Cap 100 + campo `truncated` es YAGNI-defensible: si admin tiene >100 advertencias activas simultáneas, hay problema más grande (reglas mal calibradas).
