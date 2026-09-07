# Moderation Automation Specification (Slice 1 — Foundation)

## Purpose

Backend del motor de moderación automática (AGENTS §23) ejecutado por el
bot al recibir mensajes. Slice 1 establece la base: settings per-grupo,
warning state per `(group, user)`, un **rule registry** con la primera
regla (`FloodRule`), un **worker secuencial** que dispara auto-actions
(mute/ban) reusando el adapter de Telegram, y audit logs con
`ActorID=nil`. Slice 1 es **backend-only**; el panel reactivo entra en
slice 2/3.

> **Bot API**: `restrictChatMember` y `banChatMember` se usan sin cambios
> al adapter. Rate-limit del token bucket del adapter cubre al worker
> (AGENTS §18.1). El check de admin usa `BotStatus == StatusAdministrator`
> (publications pattern, bugfix #172) — NUNCA claves `can_*`.

---

## Requirements

### Requirement: Schema `group_moderation_settings`

El sistema MUST crear la tabla `group_moderation_settings` con columnas:
`group_id` (BIGINT PK FK `groups(telegram_id)` ON DELETE CASCADE),
`enabled` (bool default false), `anti_spam_enabled`, `anti_link_enabled`,
`banned_words_enabled`, `flood_enabled` (bool default false),
`flood_messages` (SMALLINT default 5), `flood_seconds` (SMALLINT default 10),
`warning_limit` (SMALLINT default 3), `automute_warnings` (SMALLINT
default 3), `automute_minutes` (SMALLINT default 60), `autoban_warnings`
(SMALLINT default 5), `updated_at` (timestamptz default now()). Migración
`00006_create_moderation_automation.sql` con Up/Down reversibles
(goose, SQL plano).

#### Scenario: Up crea tabla con defaults

- GIVEN la base con la migración 00005 aplicada
- WHEN se ejecuta `goose up` para 00006
- THEN existe la tabla `group_moderation_settings` con PK sobre
  `group_id` y todas las columnas con sus defaults declarados

#### Scenario: Down revierte sin pérdida colateral

- GIVEN la base con la migración 00006 aplicada
- WHEN se ejecuta `goose down` para 00006
- THEN la tabla `group_moderation_settings` se elimina; las tablas
  previas (`groups`, `logs`, etc.) quedan intactas

### Requirement: Schema `user_warning_state`

El sistema MUST crear la tabla `user_warning_state` con PK compuesta
`(group_id, user_id)` y columnas `warning_count` (SMALLINT default 0),
`last_warning_at` (timestamptz nullable), `last_action_at` (timestamptz
nullable), `expires_at` (timestamptz nullable). Índice
`idx_user_warning_state_group` sobre `group_id`.

#### Scenario: Up crea tabla con PK compuesta

- GIVEN la base con 00006 parcialmente aplicada (solo
  `group_moderation_settings`)
- WHEN se ejecuta `goose up` para completar 00006
- THEN existe `user_warning_state` con PK `(group_id, user_id)`,
  `warning_count` default 0, índice sobre `group_id`

#### Scenario: Down elimina ambas tablas en orden inverso

- GIVEN la base con 00006 aplicada completa
- WHEN se ejecuta `goose down` para 00006
- THEN `user_warning_state` y `group_moderation_settings` se eliminan;
  `groups` permanece intacta

### Requirement: Default Settings al primer acceso

Cuando `HandleMessage` accede a un grupo sin fila en
`group_moderation_settings`, el sistema MUST auto-crear la fila con los
defaults de REQ-1 (`enabled=false`, todos los toggles `false`) y usarla
para el resto del pipeline. La creación es idempotente (UPSERT).

#### Scenario: Primera lectura crea fila con defaults

- GIVEN un grupo sin fila en `group_moderation_settings`
- WHEN `HandleMessage` se invoca con un mensaje de ese grupo
- THEN se crea una fila con `enabled=false`, todos los toggles `false`,
  thresholds con sus defaults

#### Scenario: Defaults aplicados cuando settings no existe

- GIVEN un grupo sin fila de settings
- WHEN el service evalúa reglas
- THEN el pipeline lee defaults en memoria (`flood_enabled=false`,
  `flood_messages=5`, `flood_seconds=10`, etc.) sin error

### Requirement: `allowed_updates` incluye `message`

El backend MUST solicitar el tipo `message` en `allowed_updates` tanto
en `getUpdates` (poller) como en `setWebhook` (modo webhook). Esto ya
está garantizado por `telegram.MVPAllowedUpdates = []string{"message",
"chat_member", "my_chat_member", "chat_join_request"}` y su uso
consistente en `poller.go` y `cmd/server/main.go`.

#### Scenario: Poller pide `message`

- GIVEN el backend en modo polling
- WHEN el `Poller` invoca `GetUpdates`
- THEN el request a la Bot API incluye `allowed_updates=["message", ...]`

### Requirement: Rules Registry

El paquete `automation` MUST exponer una interfaz `Rule` con
`Name() string` y `Check(ctx, msg, settings, warningState) (hit bool,
reason string)`. Debe existir un `Registry` con `Register(name, rule)`
y `Evaluate(msg, settings, ws) *RuleHit` que itera las reglas registradas
en orden de registro y retorna el **primer hit** (short-circuit). Si
ninguna regla dispara, retorna `nil`. En slice 1 solo `FloodRule` está
registrada.

#### Scenario: Registry devuelve primer hit

- GIVEN dos reglas `R1` y `R2` registradas; `R1` siempre dispara,
  `R2` siempre dispara
- WHEN `Evaluate` se invoca
- THEN retorna `&RuleHit{RuleName:"R1", ...}` y `R2.Check` NO se llama

#### Scenario: Registry sin hits retorna nil

- GIVEN una regla registrada que nunca dispara
- WHEN `Evaluate` se invoca
- THEN retorna `nil`

### Requirement: `FloodRule`

La `FloodRule` MUST contar mensajes por `(group, user)` dentro de una
ventana deslizante de `settings.flood_seconds`. Si el usuario envía
`settings.flood_messages` o más mensajes en esa ventana, retorna
`RuleHit{RuleName:"flood", Reason:"user sent N messages in W seconds"}`.
Implementación MUST ser thread-safe (mutex por `groupID`) y resetear el
buffer cada minuto para evitar crecimiento ilimitado.

#### Scenario: Por debajo del umbral → no hit

- GIVEN `flood_messages=5`, `flood_seconds=10`
- WHEN el mismo usuario envía 4 mensajes en 10 segundos
- THEN `FloodRule.Check` retorna `hit=false`

#### Scenario: En el umbral → hit

- GIVEN `flood_messages=5`, `flood_seconds=10`
- WHEN el mismo usuario envía 5 mensajes en 10 segundos
- THEN `Check` retorna `hit=true`, `reason="user sent 5 messages in 10 seconds"`

#### Scenario: Por encima del umbral → hit

- GIVEN `flood_messages=5`, `flood_seconds=10`
- WHEN el usuario envía 8 mensajes en 10 segundos
- THEN `Check` retorna `hit=true` (con count actual)

#### Scenario: Ventana expirada → reset

- GIVEN `flood_messages=5`, `flood_seconds=10`; 5 mensajes hace 30s
- WHEN el usuario envía un mensaje nuevo
- THEN los timestamps fuera de la ventana se descartan; el mensaje
  reciente no dispara hit

#### Scenario: Usuarios distintos aislados

- GIVEN `flood_messages=5`, `flood_seconds=10`
- WHEN `user1` envía 4 mensajes y `user2` envía 4 mensajes en la misma
  ventana
- THEN ningún `Check` retorna hit (los contadores son independientes)

### Requirement: `Service.HandleMessage`

`Service.HandleMessage(ctx, msg)` MUST ejecutar este pipeline:

1. Si `msg.From == nil` o `msg.From.ID == 0` → return sin acción.
2. Cargar `settings` del grupo (auto-create si falta).
3. Si `!settings.enabled` → return (skip silencioso).
4. Verificar `permissionOkAdmin(group)` (`BotStatus == StatusAdministrator`);
   si falso → return (sin log).
5. Cargar `warningState` del usuario (auto-create si falta).
6. `hit, reason := Registry.Evaluate(msg, settings, ws)`.
7. Si `hit`: `warning_count++`, actualizar `last_warning_at`, registrar
   log `ActionRuleTriggered` con `metadata={rule_name, reason, warning_count}`.
8. Si `warning_count >= settings.autoban_warnings` → enqueue
   `autoActionCh{Type:ban, ...}`; si `warning_count >= settings.automute_warnings`
   → enqueue `autoActionCh{Type:mute, minutes: settings.automute_minutes, ...}`.

#### Scenario: Settings deshabilitado → skip silencioso

- GIVEN un grupo con `settings.enabled=false`
- WHEN `HandleMessage` recibe un mensaje
- THEN no se crea log, no se evalúan reglas, no se incrementa
  `warning_count`

#### Scenario: Bot no admin → skip silencioso

- GIVEN un grupo con `bot_status=member`
- WHEN `HandleMessage` se invoca
- THEN no se evalúan reglas; no se incrementa `warning_count`

#### Scenario: Hit incrementa warning_count

- GIVEN `flood_messages=3`, `flood_seconds=10`, `automute_warnings=5`,
  `autoban_warnings=10`, usuario con `warning_count=0`
- WHEN el usuario envía 3 mensajes en 10s
- THEN existe log `RULE_TRIGGERED` con `warning_count=1` en metadata; la
  fila `user_warning_state` del usuario tiene `warning_count=1`

#### Scenario: Threshold automute encola auto-action

- GIVEN `automute_warnings=3`, usuario con `warning_count=2`
- WHEN ocurre un rule hit → `warning_count` pasa a 3
- THEN se encola `AutoAction{Type:mute, Minutes:60, ...}` en
  `autoActionCh`; el log `RULE_TRIGGERED` muestra `warning_count=3`

#### Scenario: Threshold autoban encola auto-action

- GIVEN `autoban_warnings=5`, usuario con `warning_count=4`
- WHEN ocurre un rule hit → `warning_count` pasa a 5
- THEN se encola `AutoAction{Type:ban, ...}`; el log `RULE_TRIGGERED`
  muestra `warning_count=5`

#### Scenario: Umbral ya alcanzado no reencola

- GIVEN usuario con `warning_count >= autoban_warnings`
- WHEN ocurre un nuevo rule hit
- THEN NO se encola otra auto-action (idempotente hasta próximo reset);
  solo se loguea el hit

### Requirement: Canal `autoActionCh` y Worker

El módulo MUST exponer `autoActionCh chan AutoAction` con buffer
configurable (`AUTOMATION_AUTOACTION_BUFFER_SIZE`, default 100). El
`Worker` lee del canal en orden FIFO, llama a `AutoActioner.MuteUser` /
`BanUser`, y registra log con `ActionAutomuteUser` o `ActionAutobanUser`
y `ActorID=nil`.

#### Scenario: Mute se ejecuta y loguea

- GIVEN una `AutoAction{Type:mute, GroupID:g, UserID:u, Minutes:60}`
  encolada
- WHEN el worker la procesa
- THEN `AutoActioner.MuteUser(ctx, g, u, 60)` se invoca y existe log
  `AUTOMUTE_USER` con `actor_id=NULL`, `status=SUCCESS`,
  `metadata={rule_name, warning_count}`

#### Scenario: Ban se ejecuta y loguea

- GIVEN una `AutoAction{Type:ban, ...}` encolada
- WHEN el worker la procesa
- THEN `AutoActioner.BanUser(ctx, g, u, 0, true)` se invoca (ban
  indefinido + revoke) y existe log `AUTOBAN_USER` con `actor_id=NULL`,
  `status=SUCCESS`

### Requirement: `AutoActioner` reusa `tg.MuteUser`/`BanUser`

El wrapper `AutoActioner` MUST:

1. Re-llamar `groupsRepo.GetByTelegramID(groupID)` para revalidar
   `permissionOkAdmin` (bot pudo haber sido removido entre el hit y el
   dispatch).
2. Si no admin → log `PERMISSION_DENIED` con `ActorID=nil`; no llama a
   Telegram.
3. Si admin → invocar `tg.MuteUser(ctx, chatID, userID, untilDate)` o
   `tg.BanUser(ctx, chatID, userID, untilDate, revokeMessages)`. Para
   `mute`, calcular `untilDate = now + minutes*60` (unix). Para `ban`
   usar `untilDate=0` (indefinido) + `revokeMessages=true`.
4. Mapear errores de `tg` a status de log (PERMISSION_DENIED, NOT_FOUND,
   TELEGRAM_ERROR).

#### Scenario: AutoActioner reusa tg.MuteUser

- GIVEN un grupo donde el bot sigue siendo admin
- WHEN el worker llama `AutoActioner.MuteUser(ctx, g, u, 10)`
- THEN internamente se invoca `tg.MuteUser(ctx, g, u, now+600)` y se
  registra log `AUTOMUTE_USER/SUCCESS`

#### Scenario: Bot removido entre hit y dispatch

- GIVEN un grupo donde el bot era admin al momento del hit pero ya no
  al momento del dispatch
- WHEN el worker llama `AutoActioner.MuteUser(...)`
- THEN NO se llama a `tg.MuteUser`; existe log con
  `status=PERMISSION_DENIED`, `ActorID=nil`, `error_message` legible

### Requirement: Suscripción al `events.Bus`

El módulo MUST registrar un handler en `events.Bus` que filtre
`Update.Message != nil` y delegue a `Service.HandleMessage`. El wiring
ocurre en `cmd/server/main.go` con `bus.Handle(subscriber.Handle)`
después de los handlers existentes.

#### Scenario: Update con Message se entrega

- GIVEN el subscriber registrado en el bus
- WHEN `bus.Publish(&Update{Message: ...})` se invoca
- THEN `Service.HandleMessage` se llama con ese `Message`

#### Scenario: Update sin Message se ignora

- GIVEN el subscriber registrado
- WHEN `bus.Publish(&Update{ChatMember: ...})` (sin `Message`) se invoca
- THEN `Service.HandleMessage` NO se llama; no se crea log

### Requirement: Audit logs con `ActorID=nil`

`internal/logs/model.go` MUST exponer 3 constantes nuevas:
`ActionRuleTriggered = "RULE_TRIGGERED"`, `ActionAutomuteUser =
"AUTOMUTE_USER"`, `ActionAutobanUser = "AUTOBAN_USER"`. Toda auto-action
generada por el worker MUST registrar log con `ActorID=nil` para
distinguirlas de las acciones manuales (`actor_id != nil`).

#### Scenario: Constantes existen

- GIVEN el paquete `logs` compilado
- WHEN se importan `logs.ActionRuleTriggered`,
  `logs.ActionAutomuteUser`, `logs.ActionAutobanUser`
- THEN existen con los valores string declarados

#### Scenario: Auto-actions loguean con actor NULL

- GIVEN el worker procesa una `AutoAction`
- WHEN persiste el log correspondiente
- THEN `entry.ActorID == nil` (SQL `actor_id IS NULL`)

### Requirement: Worker respeta rate limit

El worker MUST invocar `AutoActioner`, que MUST llamar a
`telegram.Service.MuteUser` / `BanUser`. Estas llamadas pasan por el
token bucket del adapter (§18.1). El worker NO llama a la Bot API
directamente ni por vías que eviten el rate limit.

#### Scenario: Worker canaliza por tg.MuteUser

- GIVEN el worker procesa una `AutoAction{mute, ...}`
- WHEN se observa la traza (test fake registra la llamada)
- THEN `tg.MuteUser` es el método invocado, no `http.Post` directo al
  endpoint de la Bot API

### Requirement: Configuración por variables de entorno

`internal/config/config.go` MUST leer y validar:

- `AUTOMATION_ENABLED` (bool, default `true`) — mata/arranca el
  pipeline.
- `AUTOMATION_AUTOACTION_BUFFER_SIZE` (int, default `100`) — tamaño del
  buffer de `autoActionCh`.
- `AUTOMATION_WORKER_CONCURRENCY` (int, default `1`) — workers
  concurrentes (slice 1: secuencial).

#### Scenario: Defaults aplicados sin env vars

- GIVEN proceso arrancado sin `AUTOMATION_*`
- WHEN `config.Load` corre
- THEN `cfg.AutomationEnabled=true`,
  `cfg.AutoActionBufferSize=100`, `cfg.WorkerConcurrency=1`

#### Scenario: `AUTOMATION_ENABLED=false` corta el pipeline

- GIVEN `AUTOMATION_ENABLED=false`
- WHEN arranca el backend
- THEN el subscriber NO se registra; el worker NO se inicia; `HandleMessage`
  no se invoca para ningún update

#### Scenario: Buffer size configurable

- GIVEN `AUTOMATION_AUTOACTION_BUFFER_SIZE=500`
- WHEN arranca el backend
- THEN `autoActionCh` tiene capacidad 500; logs/registros lo confirman

### Requirement: Tests §21.1

Los tests MUST cubrir:

- **Unit por regla**: `FloodRule` (4-5 casos: bajo umbral, en umbral,
  sobre umbral, expiración de ventana, aislamiento entre usuarios).
- **Service unit** con fakes (5+ casos: `enabled=false` skip, `bot≠admin`
  skip, hit incrementa counter, threshold `automute` encola, threshold
  `autoban` encola, threshold ya alcanzado es idempotente).
- **Repository integration** contra PostgreSQL real
  (`OpenTestDB("automation")` + `goose.Up` + `TRUNCATE groups CASCADE`):
  get default settings, increment warning, list warning states.
- **Worker unit** con fake `telegram.Service`: mute, ban, re-check admin
  → log `PERMISSION_DENIED`.
- **Events subscriber**: handler despachado solo cuando
  `Update.Message != nil`.
- Cero llamadas reales a la Bot API (§21.1 estricto).

#### Scenario: Test unit FloodRule — bajo umbral

- GIVEN `flood_messages=5`, 4 mensajes del usuario en ventana
- WHEN `Check` se invoca
- THEN retorna `hit=false`

#### Scenario: Test integration — `GetOrCreateWarningState` crea fila

- GIVEN Postgres real con migración 00006 aplicada
- WHEN se invoca `GetOrCreateWarningState(groupID, userID)` por primera
  vez
- THEN existe fila con `warning_count=0`, `last_warning_at=NULL`

#### Scenario: Test worker — re-check admin tras remoción

- GIVEN fake `tg.MuteUser` registrado y un grupo donde `bot_status` pasa
  a `member` antes del dispatch
- WHEN el worker procesa una `AutoAction{mute, ...}`
- THEN `tg.MuteUser` NO se invoca; log tiene
  `status=PERMISSION_DENIED`

### Requirement: No regresión

- `moderation.Service` (acciones manuales: ban/unban/mute/unmute/delete/
  pin/lock/unlock/approve/reject) MUST quedar intacto.
- `publications` y el frontend MUST quedar intactos.
- Las invariantes de AGENTS MUST mantenerse: §21.1 tests mockean
  `TelegramService`; §18 rate limit vía adapter; §13 migraciones con
  goose; §4 auto-actions registradas; §25 sin Redis, sin secretos en el
  repo.

#### Scenario: `moderation.Service` no modificado

- GIVEN el branch `feat/moderation-automation-slice1`
- WHEN `git diff main -- backend/internal/moderation/` corre
- THEN el output está vacío (cero líneas modificadas en ese paquete)

#### Scenario: Frontend intacto

- GIVEN el branch del slice
- WHEN `git diff main -- frontend/` corre
- THEN el output está vacío

#### Scenario: Sin secretos en el repo

- GIVEN el branch del slice
- WHEN `git diff main` corre buscando `TELEGRAM_BOT_TOKEN`, `JWT_SECRET`,
  passwords
- THEN no aparecen literales de secretos en código
