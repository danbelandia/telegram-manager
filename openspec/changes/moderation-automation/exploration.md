# Exploration: Moderación Automática (Fase 3 — AGENTS §23)

> **Change**: `moderation-automation` — Fase 3 completa (7 features de AGENTS §23: Anti-spam, Anti-link, Palabras prohibidas, Flood detection, Warnings, Auto-mute, Auto-ban). **Este explore propone el scope TOTAL y un slicing plan; design + tasks abordarán UN slice por vez.**
> **Mode**: hybrid (filesystem + Engram).
> **Path**: `openspec/changes/moderation-automation/exploration.md`.
> **Persisted**: `sdd/moderation-automation/exploration` (Engram) + este filesystem.
> **Slices propuestos**: 3 (Foundation → More rules + frontend → Dashboard).

---

## Contexto heredado (base obligatoria)

**Estado actual verificable** (`main @ 24e4c8c` post-publications-slice3, septiembre 2026):

- **Backend** monolito modular Go 1.22+, REST API, PostgreSQL 16 + `pgx/v5` stdlib, migraciones `goose` (00001-00005).
- **Frontend** React 19 + TypeScript + Vite + Mantine v7 (fundación: `frontend-ui-foundation`), React Query + React Router.
- **Eventos Telegram**: webhook + poller publican en `events.Bus` (`backend/internal/events/bus.go`) — entrega síncrona, en orden, a todos los handlers registrados. Hoy `nil` consumers (el handler es registro y log; nada de negocio se suscribe todavía).
- **Modelo `Update`** (`backend/internal/telegram/update.go`): ya incluye `Message{MessageID, From{User{ID,FirstName,Username}}, Chat{ID,Type,Title,Username}, Text}` — todo lo necesario para evaluar reglas sobre mensajes entrantes sin migración.
- **Modelo `ChatMember`**: ya expone `CanDeleteMessages`, `CanRestrictMembers`, `CanPinMessages`, `CanInviteUsers` (`update.go:55-66`) — pero la detección de grupos (`backend/internal/groups/events.go:57-58`) SOLO persiste `can_delete_messages` y `can_restrict_members` en `bot_permissions` (bug histórico de la tabla).
- **`permissionOk` invariant (bugfix #172, observation #172, topic `sdd/publications/permission-check`)**: usar `g.BotStatus == groups.StatusAdministrator`, **NUNCA** `can_*`. La Bot API no exige ninguna `can_*` para sendMessage/restrictChatMember/banChatMember en grupos — basta con ser admin. Aplicar a todo código NUEVO de este change.
- **`moderation.Service` existente** (`backend/internal/moderation/service.go`): orquesta ban/unban/mute/unmute/delete/pin/lock/unlock/approve/reject. Cada acción pasa por `runAction` que:
  1. Verifica grupo existe (`ErrGroupNotFound` → 404).
  2. `permissionOk(group, permissionFor(action))` — **usa `g.BotPermissions[key]`, patrón bugfix #172 (roto para claves no-pobladas)**.
  3. Llama al adapter (rate-limited, errores tipados).
  4. Audita en `logs` con `ActorID=adminID`.
- **Logs** (`backend/internal/logs/model.go`): `ActorID *int64` es nil cuando la acción la genera el sistema (comentario línea 41). Ya existe `ActionBanUser`, `ActionMuteUser` etc. — auto-actions pueden **reusar estas constantes** y dejar `ActorID=nil`.
- **`warnings` table** (precursor, migración `00003:39-46`): `id, group_id, user_id, reason, created_at` — pensada como historial de advertencias individuales. **No tiene counter**; cada insert es un evento discreto. Necesitamos una tabla NUEVA para el counter persistente (decisión D3).
- **Adapter telegram**: ya expone `BanUser`, `MuteUser` (vía `restrictChatMember`), `DeleteMessage`. Rate limiter token bucket + 429 retry con `retry_after` (slice 0 del MVP). Las auto-actions reusan los mismos métodos — no hay que tocar el adapter.
- **Frontend patterns** (`frontend/src/features/moderation/*`): hooks `useBanUser` etc., `features/{api,hooks,types,error}.ts` por dominio, query keys anidadas bajo `['groups', id, ...]`. GroupDetailPage tiene `<Tabs>` (fundación slice 1). Frontend UI foundation Mantine v7 ya consolidada (`frontend-ui-foundation`).
- **Slice pattern precedente** (`publications-slice3`, archivado): backend-first con **compile gate** (Phase 1 actualiza TODAS las fakes antes de lógica), integration tests con `OpenTestDB` + `goose.Up`, `single-pr` con `size:exception` (~1276 LOC). Forecast ~400-line budget excedido en cada slice.

---

## Hechos confirmados de la Bot API (relevantes para este change)

| Necesidad | ¿Bot API lo soporta? | Evidencia |
|-----------|----------------------|-----------|
| Leer el contenido de mensajes que enviaron OTROS usuarios | ✅ Sí | `Update.Message.Text` ya llega en `getUpdates` y webhook (ver `update.go:17-22`). El MVP ya está habilitado para recibirlos; solo hay que consumirlos. **Sin migración nueva**. |
| Borrar un mensaje ajeno (`can_delete_messages`) | ✅ Sí | `deleteMessage(chat_id, message_id)` — `telegram_api_reference.md §5`. Ya implementado en adapter. |
| Mutear / restringir | ✅ Sí | `restrictChatMember(chat_id, user_id, permissions, until_date)` — `telegram_api_reference.md §4`. Permiso bot: `can_restrict_members`. |
| Banear permanente | ✅ Sí | `banChatMember(chat_id, user_id, until_date, revoke_messages)` — `telegram_api_reference.md §4`. |
| Auto-leer mensajes de grupos donde el bot NO es admin | ❌ No | El bot solo recibe updates de chats donde está. Pero ya pedimos `message` en `allowed_updates` (`poller.go` y `webhook.go` lo setean en algún slice). **Verificar** en design. |
| Restringir a un admin del grupo | ❌ No | Telegram devuelve 400/403; el adapter ya lo mapea a `ErrPermissionDenied`. |
| Listar todos los miembros del grupo | ❌ No | Solo admins vía `getChatAdministrators`. **Confirmado** (`telegram_api_reference.md §3`). |
| Detectar mensajes editados (regex sobre versión anterior) | ❌ No para esta fase | `Update.EditedMessage` existe pero NO se procesa hoy. Documentar como follow-up. |

**Confirmación clave**: NO necesitamos métodos nuevos de la Bot API. Todo lo necesario YA está implementado en el adapter (`telegram.Service`). El trabajo es 100% backend (rule engine + tablas) + frontend (settings UI + dashboard).

---

## Decisiones (con tradeoffs)

### D1 — Trigger source: webhook + poller → bus → consumer de automatización

`Update{Message}` ya se publica en `events.Bus` (`bus.go:39-46`) por el webhook handler y el poller. Hoy no hay consumer. **Decision**: registrar `automation.Handler` en `cmd/server/main.go` junto al wiring existente; el handler:

1. Filtra `update.Message == nil` (no es mensaje — no aplica).
2. Carga `group_moderation_settings` del grupo (cache en memoria con invalidación simple — ver D4).
3. Si `enabled == false` o el bot no es admin del grupo → `return`.
4. Ejecuta pipeline de reglas (D2) sobre `(msg, settings, user_warning_state)`.
5. Si hay hit → bump `warning_count`, posiblemente ejecutar auto-action (D5).

**Tradeoff**: ejecutar en el mismo goroutine del bus (Publish es síncrono, `bus.go:39-46`) hace que un handler lento retrase a los siguientes. Pero la velocidad de evaluación es O(reglas × texto del mensaje) — microsegundos. Las auto-actions (mute/ban) sí pueden tardar 100-500ms por Telegram, pero son async-friendly: encolamos la acción vía canal interno y dejamos que un worker las drene respetando el rate limiter existente. **Ver D5**.

### D2 — Rule evaluator pattern: registry de funciones con prioridad

```go
// Regla devuelve (true, reason) si el mensaje es violación; (false, "") si no.
type Rule func(ctx context.Context, msg *telegram.Message, user *telegram.User, settings *Settings) (bool, string)

// Registry: nombre → Rule. Se itera en orden hasta el primer hit (short-circuit).
type Registry struct { rules []namedRule }
```

Reglas implementadas (slice 1: flood; slice 2: anti-link, anti-spam, banned-words; slice 3 sin reglas nuevas, solo dashboard):

| Rule | Señales | Configuración por grupo |
|------|---------|--------------------------|
| Flood | N mensajes del mismo user en W segundos | `flood_enabled`, `flood_messages`, `flood_seconds` |
| Anti-link | URLs en `msg.Text` (regex `https?://\|t\.me/`) | `anti_link_enabled`, `link_allowlist` (slice[]) |
| Anti-spam | Caps lock + repeticiones de caracteres + >X% mayúsculas en mensajes largos | `anti_spam_enabled`, `caps_threshold` (futuro: más señales) |
| Palabras prohibidas | `strings.Contains(strings.ToLower(text), word)` para cada word | `banned_words_enabled`, `banned_words` (tabla aparte) |

**Tradeoff**: ¿pipeline corto-circuito vs. todas las reglas? **Corto-circuito**: una vez que una regla dispara, el mensaje ya es violación; contar dos warnings por el mismo mensaje es ruido. Con corto-circuito, el "primer hit" define `reason`. Si el admin quiere acumular, lo cambiamos a "all-rules" en un follow-up.

### D3 — Settings storage: tabla `group_moderation_settings` + tabla `banned_words`

```sql
-- 00006_create_moderation_automation.sql
CREATE TABLE group_moderation_settings (
    group_id              BIGINT PRIMARY KEY REFERENCES groups(telegram_id) ON DELETE CASCADE,
    enabled               BOOLEAN NOT NULL DEFAULT false,
    anti_spam_enabled     BOOLEAN NOT NULL DEFAULT false,
    anti_link_enabled     BOOLEAN NOT NULL DEFAULT false,
    banned_words_enabled  BOOLEAN NOT NULL DEFAULT false,
    flood_enabled         BOOLEAN NOT NULL DEFAULT false,
    flood_messages        INT     NOT NULL DEFAULT 5  CHECK (flood_messages > 0),
    flood_seconds         INT     NOT NULL DEFAULT 10 CHECK (flood_seconds > 0),
    warning_limit         INT     NOT NULL DEFAULT 3  CHECK (warning_limit > 0),
    automute_warnings     INT     NOT NULL DEFAULT 3  CHECK (automute_warnings > 0),
    automute_minutes      INT     NOT NULL DEFAULT 10 CHECK (automute_minutes > 0),
    autoban_warnings      INT     NOT NULL DEFAULT 5  CHECK (autoban_warnings > automute_warnings),
    warning_expire_days   INT     NOT NULL DEFAULT 30 CHECK (warning_expire_days > 0),
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE banned_words (
    group_id  BIGINT NOT NULL,
    word      TEXT   NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (group_id, word)
);
```

**Por qué no JSONB en `groups` (Opción B)**: queries SQL sobre columnas indexables (count por flood window), constraints CHECK sobre rangos válidos, validación en DB; evita drift entre keys del JSON.

**Por qué no todo en una tabla con arrays (Opción C)**: `banned_words` puede crecer sin límite (cientos de palabras); un índice `(group_id, word)` PK permite upsert eficiente y borrado por palabra individual desde la UI.

### D4 — Warning state storage: tabla `user_warning_state` (counter) + reuso de `warnings` (history)

```sql
-- Misma migración 00006
CREATE TABLE user_warning_state (
    group_id         BIGINT      NOT NULL,
    user_id          BIGINT      NOT NULL,
    warning_count    INT         NOT NULL DEFAULT 0 CHECK (warning_count >= 0),
    last_warning_at  TIMESTAMPTZ,
    last_action      TEXT,                       -- 'mute' | 'ban' | NULL
    last_action_at   TIMESTAMPTZ,
    PRIMARY KEY (group_id, user_id)
);
CREATE INDEX idx_user_warning_state_group ON user_warning_state (group_id);
```

**Conceptualmente dos cosas distintas**:
- `user_warning_state`: estado agregado por (group, user). UNA fila por par.
- `warnings` (existente, 00003): historial de cada incidente (uno por hit de regla). Cada auto-mute/ban inserta una fila en `warnings` con `reason='automute'|'autoban'|'flood'|'anti-link'|...`.

**Auto-reset**: `last_warning_at` se consulta en cada hit; si `now() - last_warning_at > warning_expire_days`, decrementar (o resetear) `warning_count`. Lógica en service, no en DB.

**Tradeoff**: ¿counter separado vs. `COUNT(*)` sobre `warnings`?
- `COUNT(*)` por cada mensaje es O(log N) por índice, pero para N grande y mensajes en hot path, escanea el rango. Counter es O(1) lookup.
- Counter también permite reset sin perder historial.

### D5 — Auto-action trigger: encolar vía canal interno + worker, NO en línea con el bus

```go
// En automation.Service.HandleMessage:
if needsAutoAction {
    autoActionCh <- autoAction{groupID, userID, actionType, untilDate}
    // return inmediato; el worker drena secuencialmente.
}

// Worker (en automation/worker.go):
for act := range autoActionCh {
    runAutoAction(ctx, act)  // llama al adapter, escribe log con ActorID=nil
}
```

**Por qué no llamar directo**: el bus es síncrono (`bus.go:39-46`); una llamada lenta a `restrictChatMember` (100-500ms típico, peor en 429) bloquea otros handlers. Encolar mantiene `HandleMessage` en <5ms.

**Rate limit**: el adapter YA tiene el token bucket (`telegram/rate_limiter.go`); el worker hereda la garantía de respetar 25 req/seg global sin duplicar lógica. **Sin Redis**, consistente con §2.

**Por qué no reutilizar `moderation.Service.Mute/Ban`**: el `permissionOk` de ese servicio usa `g.BotPermissions[key]` (bugfix #172: solo `can_restrict_members` y `can_delete_messages` están pobladas). Para auto-actions queremos un gate basado en `BotStatus==Administrator` (publications pattern, ya correcto). **Decision**: thin wrapper propio (`automation.AutoActioner.Mute(ctx, groupID, userID, untilDate)`) que:
1. Verifica `g.BotStatus == StatusAdministrator` (publications pattern).
2. Llama al adapter (`tg.MuteUser`).
3. Escribe log con `ActorID: nil`, `Action: ActionAutomuteUser`.
4. Mapea errores tipados del adapter a logs con status.

Esto evita el bug, mantiene rate limit (adapter), y mantiene audit (§4).

### D6 — Logs de auditoría

Reusamos la tabla `logs` existente con:
- **Reusar**: `ActionMuteUser`, `ActionBanUser` (slice 1 ya cubre auto-mute/ban con `ActorID=nil`).
- **Nuevos**: `ActionRuleTriggered` (cada hit de regla, `Metadata={"rule":"flood","reason":"5 msgs in 10s"}`); `ActionAutoResetWarnings` (cuando expiran).

El frontend ya muestra `actor_id` como nullable. Para entries de automation se mostrará como "(sistema)" — `frontend-moderation` ya tipa `actor_id: number | null`. **Sin nuevos componentes de UI para logs**; solo el formato de presentación del actor nulo.

### D7 — Frontend UI

| Slice | UI | Componentes nuevos | Hooks nuevos |
|-------|----|---------------------|---------------|
| 2 | Tab "Moderación automática" en GroupDetailPage (Tabs ya existentes). Form con: `enabled` switch, 4 toggles de regla, inputs numéricos para thresholds, `<TagsInput>` Mantine para `banned_words`, `<TagsInput>` para `link_allowlist`. | `GroupAutomationPage.tsx` (sub-página) o componente embebido en `GroupDetailPage`. | `useAutomationSettings(groupId)`, `useUpdateAutomationSettings(groupId)` |
| 3 | Tab "Advertencias" en GroupDetailPage. Tabla `<Table>` con columnas: `Usuario`, `Warnings`, `Última advertencia`, `Última acción`. Filtros por estado. Stats cards: `auto-mutes hoy`, `auto-bans hoy`, `warnings totales`. | `GroupWarningsPage.tsx` (sub-página) o sección embebida. Stats cards. | `useWarnings(groupId)`, `useAutomationStats(groupId)` (lee logs con agregación por día) |

**Decisión clave**: ¿página nueva (`/groups/:id/automation`, `/groups/:id/warnings`) o Tabs dentro de `GroupDetailPage`?
- GroupDetailPage ya tiene `<Tabs>` (Users, Requests, Logs).
- **Decision**: agregar dos Tabs nuevas — `Automática` y `Advertencias` — en GroupDetailPage. Rutas no cambian. Consistente con la fundación Mantine slice 1.

### D8 — Frontend: settings editor

- Switch principal: `Habilitar moderación automática` (off → todas las reglas no corren, mensaje al usuario).
- 4 sub-toggles: `Anti-spam`, `Anti-link`, `Palabras prohibidas`, `Flood detection`.
- Inputs numéricos: `Flood: N mensajes en W segundos`, `Advertencias para auto-mute`, `Duración de auto-mute (min)`, `Advertencias para auto-ban`, `Días para expirar advertencias`.
- `<TagsInput>` (Mantine v7 nativo, ya en uso en el repo) para `banned_words` y `link_allowlist`. Permite agregar/borrar por palabra individual.
- Guardar: botón "Guardar cambios" llama `useUpdateAutomationSettings`. `notifySuccess` / `notifyError` ya importados en `features/moderation/error.ts`.

### D9 — Permission check del automation service

**Decision**: el automation service verifica `g.BotStatus == StatusAdministrator` (publications pattern, bugfix #172). **NO** usa `permissionOk` de `moderation` (que chequea `can_*`). La justificación:
- Auto-actions son sensibles (auto-ban es destructivo, AGENTS §4). Pero el gate NO es "el admin tiene permiso en Telegram" (eso lo hace el adapter via 403); es "el bot sigue siendo admin en este grupo desde la última detección". Si el bot fue removido, el adapter recibe 403 → log `PERMISSION_DENIED` automáticamente.
- Reusar `moderation.permissionOk` arrastra el bug histórico (claves no-pobladas).

### D10 — Tests

| Capa | What | Approach | Archivos |
|------|------|----------|----------|
| Unit (rules) | Cada regla con tabla de mensajes → (hit, reason) | `func TestFloodRule_*` etc. — fakes `Settings`+`Message` | `internal/automation/rules/{flood,anti_link,banned_words,anti_spam}_test.go` (NEW) |
| Unit (service) | `HandleMessage` con pipeline completo: trigger flood, bump counter, trigger automute cuando `count >= limit`, etc. | fakes `AutomationSettings`, `TelegramService`, `LogWriter` | `internal/automation/service_test.go` (NEW) |
| Unit (worker) | `Worker` drena canal, llama adapter, log; respeta orden; ctx cancel | fakes | `internal/automation/worker_test.go` (NEW) |
| Integration (repo) | CRUD settings + banned_words + warning_state UPSERT; warning_expire_days reset | `OpenTestDB("automation")` + `goose.Up` + `TRUNCATE` | `internal/automation/repository_test.go` (NEW) |
| Handler | `GET/PUT /groups/:id/automation/settings`, `POST/DELETE /groups/:id/automation/banned-words`, `GET /groups/:id/warnings`, `GET /groups/:id/automation/stats` | fakes de los stores | `internal/api/automation_handlers_test.go` (NEW) |
| Frontend | Toggles renderizan estado actual, TagsInput agrega/borrar palabras, stats cards muestran agregados correctos | Vitest + `mockFetchRoutes` (extender para PUT y DELETE) | `GroupAutomationPage.test.tsx`, `GroupWarningsPage.test.tsx` (NEW) |
| §21.1 guard | Cero llamadas Bot API reales en tests | adapter solo en tests de `httptest.NewServer`; service/worker tests NO usan adapter real | audit en `verify` |

---

## Affected Areas

### Backend (nuevos archivos)

| Archivo | Acción | LOC est. | Notas |
|---------|--------|----------|-------|
| `backend/migrations/00006_create_moderation_automation.sql` | **NEW** | +60 | `group_moderation_settings`, `banned_words`, `user_warning_state` |
| `backend/internal/automation/model.go` | **NEW** | +90 | `Settings`, `BannedWord`, `WarningState`, errores `ErrSettingsNotFound`, `ErrInvalidThreshold` |
| `backend/internal/automation/repository.go` | **NEW** | +200 | `SettingsRepo` (Get/Upsert), `BannedWordsRepo` (List/Add/Delete/Bulk), `WarningStateRepo` (Get/Upsert/Increment/ResetExpired/ListByGroup) |
| `backend/internal/automation/repository_test.go` | **NEW** | +180 | Integration tests Postgres real |
| `backend/internal/automation/rules.go` | **NEW** | +120 | Registry + 4 rules (Flood slice 1, los otros 3 slice 2) |
| `backend/internal/automation/rules_test.go` | **NEW** | +200 | Tabla de tests por rule |
| `backend/internal/automation/service.go` | **NEW** | +250 | `HandleMessage(ctx, msg)`, `GetSettings/UpsertSettings`, `GetWarnings`, `GetStats` |
| `backend/internal/automation/service_test.go` | **NEW** | +250 | Pipeline completo + settings CRUD |
| `backend/internal/automation/worker.go` | **NEW** | +120 | `Worker` struct, `Run(ctx)` con canal interno, ejecuta auto-actions |
| `backend/internal/automation/worker_test.go` | **NEW** | +120 | Channel drain, rate limit respeto, ctx cancel |
| `backend/internal/api/automation_handlers.go` | **NEW** | +200 | 6 endpoints (ver §D11) |
| `backend/internal/api/automation_handlers_test.go` | **NEW** | +220 | Handler tests con fakes |
| `backend/internal/api/server.go` | MOD | +15 | Rutas nuevas + `WithAutomation` option |
| `backend/internal/api/middleware_auth.go` | MOD | +5 | Ninguna (reusa auth existente) |
| `backend/internal/logs/model.go` | MOD | +10 | Constantes `ActionRuleTriggered`, `ActionAutomuteUser`, `ActionAutobanUser` |
| `backend/internal/events/bus.go` | MOD | 0 | Sin cambios (handler se registra en main.go) |
| `backend/internal/config/config.go` | MOD | +5 | `AUTOMATION_AUTOACTION_BUFFER_SIZE` (default 100) |
| `backend/cmd/server/main.go` | MOD | +30 | Instanciar `automation.Service`, `Worker`; registrar handler en `bus.Handle(...)`; lanzar goroutine del worker |

### Frontend (nuevos archivos)

| Archivo | Acción | LOC est. |
|---------|--------|----------|
| `frontend/src/features/automation/types.ts` | **NEW** | +60 |
| `frontend/src/features/automation/api.ts` | **NEW** | +80 |
| `frontend/src/features/automation/hooks.ts` | **NEW** | +100 |
| `frontend/src/features/automation/error.ts` | **NEW** | +40 |
| `frontend/src/features/automation/validateThresholds.ts` | **NEW** | +50 |
| `frontend/src/pages/GroupAutomationPage.tsx` | **NEW** | +280 |
| `frontend/src/pages/GroupAutomationPage.test.tsx` | **NEW** | +180 |
| `frontend/src/pages/GroupWarningsPage.tsx` | **NEW** | +200 |
| `frontend/src/pages/GroupWarningsPage.test.tsx` | **NEW** | +150 |
| `frontend/src/pages/GroupDetailPage.tsx` | MOD | +30 (agregar 2 Tabs) |
| `frontend/src/pages/GroupDetailPage.test.tsx` | MOD | +40 |
| `frontend/src/test/helpers.tsx` | MOD | +20 (rutas PUT/DELETE en `mockFetchRoutes`) |

### Docs

- `README.md`: sección "Moderación automática" — ~50 líneas (qué reglas hay, cómo configurar, ejemplos de cada una, nota sobre rate limit del bot).
- `.env.example`: agregar `AUTOMATION_AUTOACTION_BUFFER_SIZE=100` + comentario.

**Total estimado**: ~2950 LOC touched (sumando backend + frontend + docs). **Excede 400-line budget por mucho** — justifica el slicing en 3 PRs con `size:exception` cada uno (precedente: publications-slice1/2/3, frontend-refresh-slice2).

---

## Enfoques alternativos (rechazados)

### Opción A — **3 slices (Foundation → Rules + Frontend → Dashboard)** ✅ RECOMENDADO

- **Slice 1** (Foundation): schema + counter + pipeline + Flood rule + auto-mute/ban via thin wrapper. SIN frontend. Verificable end-to-end con logs y psql.
- **Slice 2** (More rules + Settings UI): agrega anti-link, anti-spam, banned-words rules + frontend `GroupAutomationPage` (settings editor).
- **Slice 3** (Warnings dashboard): agrega `GroupWarningsPage` (tabla de warnings + stats cards) leyendo de `user_warning_state` y agregaciones de logs.

**Pros**: cada slice entregable, testeable, fusionable independientemente; coincide con el patrón publications-slice1/2/3 (single-pr con size:exception por slice, forecast LOC cada uno ~900-1100).

**Cons**: 3 PRs vs. 1.

### Opción B — Un único mega-PR (sin slicing)

- **Pros**: un solo ciclo de review.
- **Cons**: ~2950 LOC, supera budget por 7×. Imposible verificar incrementalmente. Riesgo alto de bloqueo del review.

### Opción C — Solo backend en slice 1, frontend en slice 2 (2 slices)

- **Pros**: backend-first verificable; frontend en una sola capa.
- **Cons**: frontend demasiado grande en un solo slice (~1000 LOC); pierde la oportunidad de entregar valor visible (UI) en slice 2 (que debería ser "más reglas + UI").

### Opción D — Todo el frontend desde slice 1 (con placeholder)

- **Pros**: UI lista desde el primer merge.
- **Cons**: backend sin probar antes de pegar UI; romper el flujo de "compile gate primero" del slice pattern.

---

## Risks

| Riesgo | Likelihood | Impact | Mitigation |
|--------|-----------|--------|-----------|
| `permissionOk` de `moderation` (NO de automation) sigue con bug #172 — afecta ban/unban/mute manuales, fuera de scope | High | Medium | Documentar como issue separado; este change usa su propio `permissionOkAdmin` (publications pattern). Fuera de scope del MVP de automation; fix propuesto en follow-up. |
| Bot API no entrega `Update.Message` si el bot no es admin | Low | Medium | Verificar `bot_status == administrator` ANTES de evaluar reglas. Si no, skip silencioso + log debug. |
| Race condition: HandleMessage y UpsertSettings concurrentes | Low | Low | Settings son UPSERT idempotente. Worker es secuencial (canal). Sin race. |
| Canal de auto-actions se desborda bajo flood masivo | Low | Medium | `AUTOMATION_AUTOACTION_BUFFER_SIZE=100` configurable; si se llena, log warn + drop (preferible a bloquear el bus). Buffer > 100 requiere re-diseño con Redis (viola §2). |
| Tag spam de un usuario con miles de mensajes simultáneos | Low | Low | El rule engine evalúa por mensaje individual; flood rule detecta > N en W segundos y bumpea 1 warning por el "primer exceso" (no por mensaje). |
| Auto-ban sobre admin del grupo | Low | High | El adapter recibe 400/403 → `ErrPermissionDenied` → log `PERMISSION_DENIED` (no auto-ban real). UI muestra el log legible. **Riesgo aceptable**. |
| `allowed_updates` no incluye `message` | Medium | High | Verificar `webhook.go` y `poller.go` incluyen `message` en `allowed_updates` al registrar. Si falta, NO llegan mensajes. **Verificar en design de slice 1**. |
| Frontend `<TagsInput>` para banned_words no soporta caracteres unicode/puntuación raros | Low | Low | Validar en cliente con regex (letras/números/espacios); caracteres especiales se escapan antes de POST. **Validación** en `validateThresholds.ts`. |
| Migración 00006 rompe FK hacia `groups(telegram_id)` si se ejecutan tests aislados | Low | Low | Integration tests usan `OpenTestDB("automation")` con `TRUNCATE` que incluye `groups CASCADE`. Patrón ya validado en publications-slice3. |
| Memory leak por cache de settings en automation service | Low | Low | Sin cache por ahora: cada `HandleMessage` lee DB. Si performance requiere cache, agregar LRU en slice posterior. Verificar con `EXPLAIN ANALYZE` en slice 1. |
| Worker no respeta rate limit global | Low | High | El worker llama al adapter (`tg.MuteUser`, `tg.BanUser`), que pasa por `doWithRetry` con token bucket. Rate limit YA garantizado. **Sin Redis, sin infraestructura extra**. |
| Single-slice excede 400 LOC | **High** | Medium | **Forecast cada slice: 900-1100 LOC**. Precedente: publications-slice1/2/3, frontend-refresh-slice2 — todos con `size:exception` aprobado por el mismo `single-pr`. Aplicar misma estrategia. |
| Auto-ban no se puede deshacer desde UI | Low | Low | Frontend siempre expone el botón "Desbanear" del moderation actual (`GroupUsersPage`); los admins pueden revertir cualquier auto-ban manualmente. |
| `warning_count` se incrementa antes de saber si la regla es real (no bypass) | Low | Low | El order es: 1) rule fires; 2) check `enabled` flag and per-rule toggles; 3) increment; 4) check threshold. No incremento si disabled. |

---

## Dependency order (entre slices)

```
Slice 1 (Foundation) ──→ Slice 2 (Rules + Settings UI) ──→ Slice 3 (Warnings Dashboard)
       │                          │                                  │
       └─ backend solo            ├─ backend (3 reglas + handlers)  └─ frontend solo
                                  └─ frontend (settings editor)
```

- **Slice 1 → 2**: slice 2 agrega reglas al registry existente. Sin nuevas tablas (banned_words ya en 00006; si no estuviera, se agrega en slice 1). El frontend de slice 2 lee el `AutomationSettings` que slice 1 ya expuso.
- **Slice 2 → 3**: slice 3 lee `user_warning_state` (creado en slice 1) y agrega una pantalla sobre los mismos datos. Sin cambios de schema.
- **Slice 1 standalone**: si el usuario decide parar tras slice 1, el sistema ya tiene auto-mute/ban vía flood + audit en logs. Suficiente para validar el patrón.

---

## LOC estimates por slice (forecast)

| Slice | Backend LOC | Frontend LOC | Total | 400-line budget risk | Delivery |
|-------|------------:|-------------:|------:|----------------------|----------|
| 1 — Foundation | ~950 (model, repo, rules/flood, service, worker, handlers settings, handlers warnings read-only, main, config, log constants) | 0 | ~950 | **High** | single-pr + size:exception (precedente) |
| 2 — More rules + Settings UI | ~450 (3 reglas + banned_words handler + tests) | ~700 (page + tags + tests + helpers) | ~1150 | **High** | single-pr + size:exception |
| 3 — Warnings dashboard | ~150 (stats endpoint + agregación) | ~570 (page + stats cards + tests) | ~720 | **High** | single-pr + size:exception |
| **Total** | **~1550** | **~1270** | **~2820** | — | 3 PRs size:exception |

> Cada slice excede 400 LOC — `sdd-tasks` debe confirmar `size:exception` por slice (precedente: publications-slice1/2/3, frontend-refresh-slice2). Si el usuario prefiere chained, cada slice puede partirse en `backend → frontend` (con `feature/moderation-automation-sliceN` como branch base).

---

## Open Questions (a resolver en proposal/design, no bloqueantes para explore)

1. **¿El frontend de slice 1 debe existir para validar UX?** → NO, slice 1 es backend-first. Validación con `psql` + logs.
2. **¿Cache de settings en memoria?** → NO por ahora. Lectura por mensaje es OK mientras el threshold de tráfico lo permita. Si se observa latencia, agregar LRU en slice 2.
3. **¿Auto-mute debe ser visible en `GroupUsersPage`?** → SÍ, el `status` del miembro cambia (a `restricted`); ya hay refresh por `usersKey` cuando se invalida. NO requiere cambio en slice 1.
4. **¿Auto-ban debe notificar al admin?** → SÍ, vía log (auditoría). UI: el log aparece en `GroupLogsPage` con action `BAN_USER` y `actor_id=null`. Sin notifications push en este change.
5. **¿Soporte para `EditedMessage`?** → NO. Solo mensajes nuevos. Documentar como follow-up.
6. **¿Hard delete de warnings antiguos?** → NO. Mantener historial. El reset es lógico (vía `warning_expire_days`), no físico.
7. **¿Reset manual de warnings desde UI?** → NO en este change. Documentar como follow-up (botón "Reset advertencias" en slice 3 si hay tiempo).

---

## Ready for Proposal

**Sí**, con 3 slices propuestos. Informar al usuario:

> **Cambio `moderation-automation` (Fase 3 — AGENTS §23)** agrega 7 features de moderación automática en 3 slices:
>
> **Slice 1 — Foundation** (≈950 LOC, backend only):
> 1. Migración `00006_create_moderation_automation.sql` con `group_moderation_settings`, `banned_words`, `user_warning_state`.
> 2. `internal/automation/{model,repository,rules,service,worker}.go` + handlers en `internal/api/automation_handlers.go`.
> 3. Regla Flood implementada (las 3 restantes en slice 2).
> 4. Auto-mute/auto-ban vía canal interno + worker que llama al adapter (rate-limit garantizado).
> 5. Logs automáticos (`ActorID=nil`, acciones `MUTE_USER`/`BAN_USER` reusadas + `RULE_TRIGGERED`/`AUTOMUTE_USER`/`AUTOBAN_USER` nuevas).
> 6. Suscripción al `events.Bus` en `cmd/server/main.go`.
>
> **Slice 2 — More rules + Settings UI** (≈1150 LOC):
> 7. Anti-link, Anti-spam, Palabras prohibidas.
> 8. Frontend `GroupAutomationPage` (tab en `GroupDetailPage`) con toggles, inputs numéricos, `<TagsInput>` Mantine v7.
> 9. CRUD de `banned_words` por grupo.
>
> **Slice 3 — Warnings Dashboard** (≈720 LOC):
> 10. Frontend `GroupWarningsPage` (tab en `GroupDetailPage`) con tabla de warnings por usuario.
> 11. Stats cards: auto-mutes hoy, auto-bans hoy, total warnings (agregación sobre `logs` filtrado por fecha).
>
> **Confirmaciones Bot API**:
> - El bot YA recibe `Update.Message` para mensajes de OTROS usuarios vía webhook/poller (verificable en slice 1 que `allowed_updates` incluya `message`).
> - Las auto-actions usan los métodos YA implementados en el adapter (`deleteMessage`, `restrictChatMember` vía `MuteUser`, `banChatMember`).
>
> **Schema**: nueva migración `00006`. Tabla `warnings` existente (00003) se REUSA para historial por incidente.
>
> **Permission invariant (bugfix #172)**: `permissionOkAdmin(g) == g.BotStatus == StatusAdministrator` — patrón publications, NO `can_*`.
>
> **Rate limit**: el worker pasa por el adapter (token bucket, sin Redis, sin infra extra — consistente con AGENTS §2/§18.1).
>
> **LOC est. total**: ~2820 en 3 PRs con `size:exception` cada uno (precedente: publications-slice1/2/3, frontend-refresh-slice2).
>
> ¿Procedemos con el proposal del Slice 1 (Foundation) y dejamos 2 y 3 como proposal separados cuando se implemente cada uno?
