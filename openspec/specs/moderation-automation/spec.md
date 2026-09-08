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

---

## Slice 2 ADDED Requirements (2026-09-08 — Anti-spam + Anti-link + Banned-words + Settings UI)

> **Change**: `moderation-automation-slice2` — Slice 2/3 de Fase 3 (AGENTS §23).
> **Predecessor**: slice 1 archivado (`main @ 1b10d42`); canónico en `openspec/specs/moderation-automation/spec.md`.
> **Authority**: bugfix `#172` (`permissionOkAdmin` usa `BotStatus == StatusAdministrator`, NUNCA `can_*`).
>
> Esta spec **amenda** el canónico de slice 1 vía `## ADDED Requirements` (REQ-7 a REQ-21). Archive al cierre usa la técnica APPEND del slice 1 (delta → canónico; REQ-1..REQ-6 de slice 1 preservados, REQ-7..REQ-21 nuevos appendeados). No se crea spec paralelo.

### Requirement: Schema `banned_words`

El sistema MUST crear la tabla `banned_words` con columnas `group_id`
(BIGINT NOT NULL, FK `groups(telegram_id)` ON DELETE CASCADE), `word`
(TEXT NOT NULL), `created_at` (TIMESTAMPTZ NOT NULL DEFAULT now()).
PK compuesta `(group_id, word)`. CHECK `length(word) BETWEEN 1 AND 100`.
Índice `idx_banned_words_group` sobre `group_id`. Migración
`00007_create_moderation_lists.sql` con Up/Down reversibles (goose,
SQL plano). El insert MUST normalizar la palabra con `LOWER(word)`
para hacer la comparación case-insensitive uniforme.

#### Scenario: Up crea tabla con PK compuesta

- GIVEN la base con la migración 00006 aplicada
- WHEN se ejecuta `goose up` para 00007
- THEN existe la tabla `banned_words` con PK `(group_id, word)`,
  CHECK `length(word) BETWEEN 1 AND 100`, FK CASCADE a `groups`

#### Scenario: Down revierte sin tocar la base previa

- GIVEN la base con la migración 00007 aplicada
- WHEN se ejecuta `goose down` para 00007
- THEN `banned_words` se elimina; las tablas previas (`groups`,
  `group_moderation_settings`, `user_warning_state`) quedan intactas

#### Scenario: FK CASCADE borra palabras al borrar el grupo

- GIVEN un grupo `g` con 3 filas en `banned_words`
- WHEN se ejecuta `DELETE FROM groups WHERE telegram_id = g`
- THEN las 3 filas de `banned_words` para `g` se eliminan
  automáticamente

### Requirement: Schema `link_allowlist`

El sistema MUST crear la tabla `link_allowlist` con la misma shape
que `banned_words`: columnas `group_id` (BIGINT NOT NULL FK CASCADE),
`domain` (TEXT NOT NULL), `created_at` (TIMESTAMPTZ NOT NULL DEFAULT
now()). PK compuesta `(group_id, domain)`. CHECK
`length(domain) BETWEEN 1 AND 253`. Índice
`idx_link_allowlist_group` sobre `group_id`. El dominio se guarda tal
cual lo ingresa el admin (la normalización lowercase del match la
hace el `AntiLinkRule` en evaluación, no el insert).

#### Scenario: Up crea tabla paralela

- GIVEN la base con 00007 parcialmente aplicada (solo `banned_words`)
- WHEN se completa `goose up` para 00007
- THEN existe `link_allowlist` con PK `(group_id, domain)`,
  CHECK `length(domain) BETWEEN 1 AND 253`, FK CASCADE a `groups`

#### Scenario: Down elimina ambas tablas en orden inverso

- GIVEN la base con 00007 aplicada completa
- WHEN se ejecuta `goose down` para 00007
- THEN `link_allowlist` y `banned_words` se eliminan;
  `group_moderation_settings` y `user_warning_state` permanecen

### Requirement: `AntiSpamRule`

El sistema MUST implementar `AntiSpamRule` (registrada en el
`Registry` después de `FloodRule`). La regla MUST detectar **al menos
uno** de los siguientes sub-detectores sobre `msg.Text`:

- **All-caps**: proporción de letras mayúsculas (`unicode.IsUpper`)
  sobre letras totales (`unicode.IsLetter`) > 0.70 AND
  `len(msg.Text) > 10`. Reason: `"message is mostly uppercase"`.
- **Caracteres repetidos**: 5 o más caracteres iguales consecutivos
  (cualquier rune, no solo letras). Reason:
  `"message contains 5+ repeated characters"`.
- **URL sospechosa corta**: cualquier match del patrón
  `https?://\S+|t\.me/\S+|telegram\.me/\S+` con longitud total del
  match < 20 caracteres. Reason: `"short suspicious URL"`.

El primer sub-detector que dispare define el `RuleHit.Reason`. Si
`msg.Text` está vacío o no aplica ningún sub-detector → retorna nil.
La regla MUST respetar `settings.AntiSpamEnabled` (skip si false) y
MUST ser stateless (sin estado mutable; tests de concurrencia no
requeridos).

#### Scenario: All-caps dispara hit

- GIVEN `AntiSpamEnabled=true`, mensaje con texto
  `"COMPREN ESTE PRODUCTO YA"` (>70% mayúsculas, len > 10)
- WHEN `AntiSpamRule.Evaluate` se invoca
- THEN retorna `&RuleHit{RuleName:"anti_spam", Reason:"message is mostly uppercase"}`

#### Scenario: Texto corto en mayúsculas NO dispara

- GIVEN `AntiSpamEnabled=true`, mensaje con texto `"OK"` (len ≤ 10)
- WHEN `AntiSpamRule.Evaluate` se invoca
- THEN retorna nil

#### Scenario: Caracteres repetidos dispara hit

- GIVEN `AntiSpamEnabled=true`, mensaje con texto
  `"holaaaaaaa amigos"` (5+ `a` consecutivas)
- WHEN `AntiSpamRule.Evaluate` se invoca
- THEN retorna `&RuleHit{RuleName:"anti_spam", Reason:"message contains 5+ repeated characters"}`

#### Scenario: URL corta sospechosa dispara hit

- GIVEN `AntiSpamEnabled=true`, mensaje con texto `"mira https://x.co"`
- WHEN `AntiSpamRule.Evaluate` se invoca
- THEN retorna `&RuleHit{RuleName:"anti_spam", Reason:"short suspicious URL"}`

#### Scenario: Toggle deshabilitado salta la regla

- GIVEN `AntiSpamEnabled=false`, mensaje all-caps válido
- WHEN `AntiSpamRule.Evaluate` se invoca
- THEN retorna nil sin inspeccionar el texto

### Requirement: `AntiLinkRule`

El sistema MUST implementar `AntiLinkRule`. La regla MUST detectar
cualquier URL que matchee el patrón
`https?://\S+|t\.me/\S+|telegram\.me/\S+` en `msg.Text`. Si el host
extraído del match (lowercased) está en `lists.LinkAllowlist` **o** es
un subdominio directo de un dominio en la allowlist (ej.
`sub.example.com` matchea `example.com`), la URL se ignora. Cualquier
URL fuera de la allowlist → retorna
`&RuleHit{RuleName:"anti_link", Reason:"message contains link to <host>"}`
donde `<host>` es el host extraído del primer match. La regla MUST
respetar `settings.AntiLinkEnabled`. Si la lista `lists.LinkAllowlist`
es nil o vacía → toda URL dispara hit.

#### Scenario: URL fuera de allowlist dispara hit

- GIVEN `AntiLinkEnabled=true`, `link_allowlist = []`,
  mensaje `"mira https://spam.example.com"`
- WHEN `AntiLinkRule.Evaluate` se invoca
- THEN retorna `&RuleHit{RuleName:"anti_link", Reason:"message contains link to spam.example.com"}`

#### Scenario: Dominio exacto en allowlist → skip

- GIVEN `AntiLinkEnabled=true`, `link_allowlist = ["example.com"]`,
  mensaje `"mira https://example.com/x"`
- WHEN `AntiLinkRule.Evaluate` se invoca
- THEN retorna nil

#### Scenario: Subdominio en allowlist → skip

- GIVEN `AntiLinkEnabled=true`, `link_allowlist = ["example.com"]`,
  mensaje `"mira https://docs.example.com/y"`
- WHEN `AntiLinkRule.Evaluate` se invoca
- THEN retorna nil (suffix match acepta `*.example.com`)

#### Scenario: Dominio con prefijo distinto NO matchea allowlist

- GIVEN `AntiLinkEnabled=true`, `link_allowlist = ["example.com"]`,
  mensaje `"mira https://notexample.com/z"`
- WHEN `AntiLinkRule.Evaluate` se invoca
- THEN retorna hit (suffix `example.com` requiere `.` previo)

#### Scenario: Toggle deshabilitado salta la regla

- GIVEN `AntiLinkEnabled=false`, mensaje con URL fuera de allowlist
- WHEN `AntiLinkRule.Evaluate` se invoca
- THEN retorna nil

### Requirement: `BannedWordsRule`

El sistema MUST implementar `BannedWordsRule`. La regla MUST iterar
`lists.BannedWords` y, para cada palabra, aplicar
`strings.Contains(strings.ToLower(msg.Text), word)`. El primer match
define `RuleHit.RuleName="banned_words"` y
`Reason="message contains banned word: <word>"`. La regla MUST
respetar `settings.BannedWordsEnabled`. Si `lists.BannedWords` es nil
o vacío → retorna nil (skip). El text vacío → retorna nil.

#### Scenario: Palabra exacta matchea case-insensitive

- GIVEN `BannedWordsEnabled=true`, `banned_words = ["spam"]`,
  mensaje `"compré SPAM ayer"`
- WHEN `BannedWordsRule.Evaluate` se invoca
- THEN retorna `&RuleHit{RuleName:"banned_words", Reason:"message contains banned word: spam"}`

#### Scenario: Substring matchea

- GIVEN `BannedWordsEnabled=true`, `banned_words = ["mal"]`,
  mensaje `"esto es una palabra mala"`
- WHEN `BannedWordsRule.Evaluate` se invoca
- THEN retorna hit con `Reason:"message contains banned word: mal"`

#### Scenario: Lista vacía → skip

- GIVEN `BannedWordsEnabled=true`, `banned_words = []`,
  cualquier mensaje
- WHEN `BannedWordsRule.Evaluate` se invoca
- THEN retorna nil

#### Scenario: Toggle deshabilitado salta la regla

- GIVEN `BannedWordsEnabled=false`, mensaje con palabra baneada
- WHEN `BannedWordsRule.Evaluate` se invoca
- THEN retorna nil

### Requirement: Rule registry order cheap→expensive

El `Registry` MUST evaluar las reglas registradas en el siguiente
orden: `Flood → AntiSpam → AntiLink → BannedWords`. Esto MUST ser
configurable solo vía el orden de invocación a `registry.Register(...)`
en `cmd/server/main.go` — sin API pública para reordenar. El
short-circuit MUST mantenerse: la primera regla que retorne `*RuleHit`
corta la iteración. La justificación (cheap-first) es que
`FloodRule` es O(1) en memoria, `AntiSpamRule` y `AntiLinkRule` son
O(n) en CPU puro sobre el texto, y `BannedWordsRule` es la única que
depende de una lista externa (potencialmente grande) ya pre-cargada
por el Service.

#### Scenario: Orden registrado respetado por `Rules()`

- GIVEN las 4 reglas registradas en este orden: Flood, AntiSpam,
  AntiLink, BannedWords
- WHEN `Registry.Rules()` se invoca
- THEN el slice devuelto mantiene el orden exacto: índice 0 Flood,
  1 AntiSpam, 2 AntiLink, 3 BannedWords

#### Scenario: Short-circuit entre reglas costosas

- GIVEN `FloodRule` registrada que dispara hit, `BannedWordsRule`
  registrada con fakes que cuentan invocaciones
- WHEN `Registry.Evaluate` se invoca con un mensaje que dispara flood
- THEN `BannedWordsRule.Evaluate` NO se invoca (short-circuit)

### Requirement: `Service.HandleMessage` pre-carga listas una vez

`Service.HandleMessage` MUST invocar `bannedWordsRepo.ListBannedWords`
y `linkAllowlistRepo.ListLinkAllowlist` exactamente **una vez** por
mensaje, antes de invocar `Registry.Evaluate`, y pasar el resultado a
cada regla como argumento `lists *Lists{BannedWords, LinkAllowlist}`
del nuevo `Rule.Evaluate` extendido. Si `!settings.BannedWordsEnabled`
Y `!settings.AntiLinkEnabled`, el Service MAY omitir las dos llamadas
a la DB (optimización; tests verifican que las llamadas se hacen
cuando al menos un toggle dependiente está activo).

#### Scenario: Las dos listas se cargan una sola vez

- GIVEN un mensaje, `BannedWordsEnabled=true`, `AntiLinkEnabled=true`
- WHEN `Service.HandleMessage` se invoca con un fake repo que cuenta
  invocaciones
- THEN `ListBannedWords` y `ListLinkAllowlist` se llaman exactamente
  1 vez cada una antes de `Registry.Evaluate`

#### Scenario: Listas se omiten si ambos toggles están en false

- GIVEN `BannedWordsEnabled=false`, `AntiLinkEnabled=false`
- WHEN `Service.HandleMessage` se invoca
- THEN `ListBannedWords` y `ListLinkAllowlist` NO se invocan; las
  reglas se evalúan con `lists == nil` (cada regla es defensiva con
  nil)

#### Scenario: Listas se cargan si al menos un toggle está activo

- GIVEN `BannedWordsEnabled=false`, `AntiLinkEnabled=true`
- WHEN `Service.HandleMessage` se invoca
- THEN `ListLinkAllowlist` se invoca 1 vez; `ListBannedWords` se omite

### Requirement: `Rule.Evaluate` extendido con `lists *Lists`

La interfaz `Rule` MUST extender su firma de `Evaluate` para aceptar
un nuevo argumento `lists *Lists` entre `ws *WarningState` y `now
time.Time`. `Lists` MUST ser un struct exported con campos
`BannedWords []string` y `LinkAllowlist []string`. La `FloodRule`
existente MUST ser actualizada para aceptar el nuevo argumento
(pasarlo por ignorarlo; la regla sigue stateless). El `Registry.Evaluate`
MUST propagar `lists` a cada regla registrada. Toda regla nueva o
modificada MUST manejar `lists == nil` defensivamente (retornar nil
si la regla no puede evaluar sin listas).

#### Scenario: Nueva firma compila y tests de slice 1 siguen pasando

- GIVEN el branch con la firma extendida
- WHEN `go build ./...` corre
- THEN el código compila sin errores; los tests de `FloodRule`
  (sub-umbral, en umbral, sobre umbral, expiración, aislamiento)
  siguen verdes

#### Scenario: Registry propaga lists a todas las reglas

- GIVEN un Registry con `FloodRule` y `BannedWordsRule` registradas,
  un `*Lists` con `BannedWords:["spam"]`
- WHEN `Registry.Evaluate` se invoca
- THEN ambas reglas reciben el mismo puntero `*Lists`

### Requirement: GET/PUT `/api/groups/{id}/automation/settings`

El backend MUST exponer:

- `GET /api/groups/{id}/automation/settings` → 200 con `*Settings` JSON
  (envelope `data.settings`). Si no existe fila en
  `group_moderation_settings`, retorna defaults (`enabled=false`,
  todos los toggles `false`, thresholds de DB). Autenticación
  requerida (`requireAuth`). Grupo inexistente → 404 `NOT_FOUND`.
- `PUT /api/groups/{id}/automation/settings` → body `*Settings`
  parcial o completo; el handler MUST normalizar defaults para
  campos ausentes y llamar `repository.UpsertSettings`. Log
  `ActionUpdateAutomationSettings` con `ActorID` del admin,
  `metadata={settings_changed}`. Autenticación requerida. Errores
  de validación (ej. `autoban_warnings <= automute_warnings`) → 400
  `VALIDATION_ERROR`.

#### Scenario: GET sin fila retorna defaults

- GIVEN un grupo `g` sin fila en `group_moderation_settings`
- WHEN se ejecuta `GET /api/groups/g/automation/settings`
- THEN responde 200 con envelope
  `data.settings = {enabled:false, anti_spam_enabled:false, ...}`

#### Scenario: GET con fila existente

- GIVEN un grupo `g` con fila persistida
  (`enabled=true`, `anti_spam_enabled=true`, `flood_messages=3`)
- WHEN se ejecuta GET
- THEN responde 200 con esos valores exactos

#### Scenario: PUT upsert y loguea

- GIVEN un admin autenticado y body con `enabled=true`,
  `anti_spam_enabled=true`, `automute_warnings=5`
- WHEN se ejecuta `PUT /api/groups/g/automation/settings`
- THEN la fila se upserta; existe log `UPDATE_AUTOMATION_SETTINGS`
  con `actor_id` del admin y `metadata` con los campos cambiados

#### Scenario: 404 si el grupo no existe

- GIVEN `group_id` que no está en `groups`
- WHEN se ejecuta GET o PUT
- THEN responde 404 `NOT_FOUND`

### Requirement: CRUD `/api/groups/{id}/automation/banned-words`

El backend MUST exponer:

- `GET /api/groups/{id}/automation/banned-words` → 200 con
  `[]string` ordenado alfabéticamente. Autenticación requerida.
- `POST /api/groups/{id}/automation/banned-words` con body `{word}`
  → 200 con la lista actualizada. La palabra se guarda lowercased
  (server-side normalization). Si la palabra ya existía (PK
  compuesta), `ON CONFLICT DO NOTHING` → no error, devuelve la lista
  actual. Log `ActionAddBannedWord` con `ActorID` del admin.
- `DELETE /api/groups/{id}/automation/banned-words/{word}` → 200 con
  la lista actualizada. La palabra buscada se lowercases antes del
  DELETE. Si no existía → no error, devuelve la lista actual.
  Log `ActionRemoveBannedWord` con `ActorID` del admin.

#### Scenario: GET lista vacía

- GIVEN un grupo `g` sin filas en `banned_words`
- WHEN se ejecuta GET
- THEN responde 200 con `data.words = []`

#### Scenario: POST agrega palabra nueva

- GIVEN un grupo `g` con `banned_words = []`
- WHEN se ejecuta POST con `{word:"spam"}`
- THEN responde 200 con `data.words = ["spam"]`; existe log
  `ADD_BANNED_WORD` con `actor_id` y `metadata={word:"spam"}`

#### Scenario: POST idempotente

- GIVEN un grupo `g` con `banned_words = ["spam"]`
- WHEN se ejecuta POST con `{word:"SPAM"}`
- THEN responde 200 con `data.words = ["spam"]` (sin duplicar);
  se registra exactamente 1 log `ADD_BANNED_WORD`

#### Scenario: DELETE remueve palabra

- GIVEN un grupo `g` con `banned_words = ["spam","mal"]`
- WHEN se ejecuta DELETE `/automation/banned-words/spam`
- THEN responde 200 con `data.words = ["mal"]`; existe log
  `REMOVE_BANNED_WORD`

### Requirement: CRUD `/api/groups/{id}/automation/link-allowlist`

El backend MUST exponer el mismo patrón simétrico que
`banned-words`:

- `GET /api/groups/{id}/automation/link-allowlist` → 200 con
  `[]string` ordenado alfabéticamente. Autenticación requerida.
- `POST /api/groups/{id}/automation/link-allowlist` con body
  `{domain}` → 200 con la lista actualizada. El dominio se guarda tal
  cual (case se preserva); el matcher lowercases en evaluación. Si ya
  existía → no error. Log `ActionAddLinkAllowlist` con `ActorID`.
- `DELETE /api/groups/{id}/automation/link-allowlist/{domain}` → 200
  con la lista actualizada. Si no existía → no error. Log
  `ActionRemoveLinkAllowlist` con `ActorID`.

#### Scenario: GET lista vacía

- GIVEN un grupo `g` sin filas en `link_allowlist`
- WHEN se ejecuta GET
- THEN responde 200 con `data.domains = []`

#### Scenario: POST agrega dominio

- GIVEN un grupo `g` con `link_allowlist = []`
- WHEN se ejecuta POST con `{domain:"example.com"}`
- THEN responde 200 con `data.domains = ["example.com"]`; existe log
  `ADD_LINK_ALLOWLIST`

#### Scenario: DELETE remueve dominio

- GIVEN un grupo `g` con `link_allowlist = ["example.com"]`
- WHEN se ejecuta DELETE `/automation/link-allowlist/example.com`
- THEN responde 200 con `data.domains = []`; existe log
  `REMOVE_LINK_ALLOWLIST`

### Requirement: Frontend `GroupAutomationPage`

El frontend MUST renderizar `GroupAutomationPage` en la ruta
`/groups/:id/automation` (registrada en `App.tsx` con
`RequireAuth`). La página MUST mostrar 4 secciones:

- **Reglas activas**: 4 `Switch` (Mantine v7) en orden —
  `Habilitar moderación automática` (settings.enabled),
  `Flood` (flood_enabled), `Anti-spam` (anti_spam_enabled),
  `Anti-link` (anti_link_enabled), `Banned-words`
  (banned_words_enabled). (5 switches en total: 1 principal + 4
  toggles por regla.)
- **Umbrales**: 6 `NumberInput` Mantine v7 — `flood_messages`,
  `flood_seconds`, `warning_limit`, `automute_warnings`,
  `automute_minutes`, `autoban_warnings`. Validación cliente:
  `autoban_warnings > automute_warnings > 0`.
- **Palabras baneadas**: `<TagsInput>` Mantine v7 para
  `banned_words`. Validación cliente: regex
  `^[\p{L}\p{N}_\- ]{1,100}$`. Trim + lowercase antes de POST.
- **Link allowlist**: `<TagsInput>` Mantine v7 para
  `link_allowlist`. Sin regex estricto (acepta punto, guion).
  Trim antes de POST.

**Un único botón "Guardar"** al fondo MUST disparar los roundtrips
en paralelo vía `Promise.all`:

1. `PUT /api/groups/:id/automation/settings` con los toggles + thresholds.
2. Por cada palabra en `banned_words`: `POST o DELETE` según diff vs
   el estado original.
3. Idem para `link_allowlist`.

Notificaciones `notifySuccess`/`notifyError` por sección. Cada
operación que falle NO aborta el resto (se acumulan y se muestran
como notificaciones separadas). El link desde `GroupDetailPage` →
panel "Detalle" → botón "Moderación automática" (debajo de
   "Membresía y moderación") MUST existir.

#### Scenario: Carga inicial con datos

- GIVEN `mockFetchRoutes` configurado para devolver settings
  (`enabled=true`, `flood_messages=5`), 2 palabras, 1 dominio
- WHEN se renderiza `GroupAutomationPage` en `/groups/g/automation`
- THEN los switches reflejan los valores del GET; los `<TagsInput>`
  muestran las palabras y dominios del GET

#### Scenario: Save dispara 3 roundtrips en paralelo

- GIVEN la página con settings cargados, `Promise.allSpy` mock
- WHEN el admin modifica 1 palabra, 1 toggle, 1 threshold y hace
  click en Guardar
- THEN se ejecutan en paralelo: 1 PUT settings, 1 POST/DELETE word,
  0 cambios en allowlist (Promise.all con al menos 2 operaciones)

#### Scenario: Error en una sección NO aborta las otras

- GIVEN `PUT /settings` responde 500, `POST /banned-words` responde 200
- WHEN el admin hace click en Guardar
- THEN la palabra se agrega (notifySuccess); settings muestra
  `notifyError` con mensaje legible

#### Scenario: Link desde GroupDetailPage

- GIVEN un grupo cargado en `GroupDetailPage`
- WHEN el admin hace click en "Moderación automática"
- THEN navega a `/groups/:id/automation`

### Requirement: Action constants de auditoría (5 nuevas)

`backend/internal/logs/model.go` MUST exponer 5 constantes nuevas:

- `ActionUpdateAutomationSettings = "UPDATE_AUTOMATION_SETTINGS"`
- `ActionAddBannedWord            = "ADD_BANNED_WORD"`
- `ActionRemoveBannedWord         = "REMOVE_BANNED_WORD"`
- `ActionAddLinkAllowlist         = "ADD_LINK_ALLOWLIST"`
- `ActionRemoveLinkAllowlist      = "REMOVE_LINK_ALLOWLIST"`

Todas las acciones manuales de settings/lists MUST registrar log con
`ActorID` del admin (no `nil` — son cambios manuales, no del
sistema). Esto es distinto del patrón de slice 1 donde `ActorID=nil`
marcaba auto-actions.

#### Scenario: Constantes existen

- GIVEN el paquete `logs` compilado
- WHEN se importan las 5 constantes nuevas
- THEN existen con los valores string declarados

#### Scenario: Log manual tiene actor no nulo

- GIVEN un admin con id `42` ejecuta `PUT /settings`
- WHEN el handler persiste el log
- THEN `entry.ActorID != nil` (`actor_id = 42`)

### Requirement: Tests §21.1 (backend + frontend)

Los tests MUST cubrir (§21.1 estricto — cero llamadas Bot API
reales):

- **Unit por regla** (extender `rules_test.go`): `AntiSpamRule` (5+
  casos: all-caps hit, texto corto sin hit, repeated chars hit,
  short URL hit, toggle off skip); `AntiLinkRule` (5+ casos: URL
  fuera de allowlist, dominio exacto skip, subdominio skip,
  prefijo-falso hit, toggle off skip); `BannedWordsRule` (4+ casos:
  match case-insensitive, substring, lista vacía skip, toggle off
  skip). Total ≥ 18 casos nuevos.
- **Service tests** (extender `service_test.go`): 5+ casos
  adicionales cubriendo pre-load de listas (lista vacía cuando
  toggles off, doble carga cuando ambos toggles activos, listas
  compartidas entre reglas).
- **Repository integration** (extender `repository_test.go`):
  `OpenTestDB("automation")` + `goose.Up` + TRUNCATE; 8+ casos:
  `AddBannedWord`/`RemoveBannedWord`/`ListBannedWords`, mismo
  patrón para `link_allowlist`, FK CASCADE al borrar grupo,
  idempotencia en POST (palabra duplicada → no error).
- **Handler tests** (`automation_handlers_test.go`): 200/400/404 en
  GET y PUT settings; 200 en GET/POST/DELETE banned-words; 200 en
  GET/POST/DELETE link-allowlist; auth required (401 sin token).
- **Frontend tests** (`GroupAutomationPage.test.tsx`):
  `mockFetchRoutes` configurado por substring URL; render inicial,
  agregar palabra dispara POST, remover dispara DELETE, toggle
  cambia state local, Save dispara ≥ 2 roundtrips, error path
  muestra `notifyError`.

#### Scenario: Unit AntiSpamRule — all-caps hit

- GIVEN `AntiSpamEnabled=true`, mensaje `"COMPREN ESTE PRODUCTO YA"`
- WHEN `Evaluate` se invoca
- THEN retorna `RuleHit{RuleName:"anti_spam", Reason:"message is mostly uppercase"}`

#### Scenario: Service — listas pre-cargadas una vez

- GIVEN un fake repo que cuenta invocaciones,
  `BannedWordsEnabled=true`, `AntiLinkEnabled=true`
- WHEN `HandleMessage` se invoca con un mensaje
- THEN `ListBannedWords` se invoca exactamente 1 vez; la misma
  instancia `*Lists` se pasa a `Registry.Evaluate`

#### Scenario: Integration repo — FK CASCADE borra palabras

- GIVEN Postgres real con migración 00007 aplicada; grupo `g` con 2
  palabras insertadas
- WHEN se ejecuta `DELETE FROM groups WHERE telegram_id = g`
- THEN las 2 filas de `banned_words` para `g` desaparecen

#### Scenario: Handler — auth required en GET settings

- GIVEN el handler sin sesión activa (sin token válido)
- WHEN se ejecuta `GET /api/groups/g/automation/settings`
- THEN responde 401 `UNAUTHORIZED`

#### Scenario: Frontend — agregar palabra dispara POST

- GIVEN la página con `banned_words = []`
- WHEN el admin agrega `"spam"` al `<TagsInput>` y hace click en
  Guardar
- THEN se observa 1 request `POST` a `/automation/banned-words` con
  body `{word:"spam"}`

### Requirement: No regresión

- `backend/internal/automation/rules.go` MUST quedar extendido, NO
  reemplazado: la firma de `FloodRule.Evaluate` se modifica, pero
  la implementación de flood detection se preserva.
- `backend/internal/moderation/` (acciones manuales) MUST quedar
  intacto — bugfix `#172` sigue fuera de scope.
- `publications`, `frontend/src/pages/{DashboardPage,PublicationsPage,
  LoginPage, GroupsPage, GroupUsersPage, GroupRequestsPage,
  GroupLogsPage}.tsx` MUST quedar intactos.
- Las invariantes de AGENTS MUST mantenerse: §4/§25 sin secretos en
  el repo; §18 rate limit vía adapter (auto-actions); §21.1 tests
  mockean `TelegramService`; §13 migraciones con goose.

#### Scenario: `backend/internal/moderation/` intacto

- GIVEN el branch `feat/moderation-automation-slice2`
- WHEN `git diff main -- backend/internal/moderation/` corre
- THEN el output está vacío (cero líneas modificadas)

#### Scenario: `FloodRule` sigue funcionando con la firma extendida

- GIVEN la nueva firma de `Rule.Evaluate(..., lists *Lists, now)`
- WHEN los 5 escenarios de `FloodRule` (sub-umbral, en umbral, sobre
  umbral, expiración, aislamiento) corren
- THEN todos pasan con la firma extendida (pasa `lists=nil` a
  `FloodRule`)

#### Scenario: Páginas frontend preexistentes intactas

- GIVEN el branch del slice
- WHEN `git diff main -- frontend/src/pages/` excluyendo
  `GroupAutomationPage.tsx` y `GroupDetailPage.tsx` corre
- THEN el output está vacío

#### Scenario: permissionOkAdmin invariante

- GIVEN el branch del slice
- WHEN `grep -rn "can_" backend/internal/automation/` corre
- THEN no aparecen checks sobre claves `can_*` (solo
  `permissionOkAdmin` con `BotStatus == StatusAdministrator`)

## Slice 2.1 ADDED Requirements (2026-09-08 — Warning visual al usuario en chat)

> **Change**: `moderation-automation-slice2.1` — sub-slice de Fase 3 (AGENTS §23) sobre slice 2 archivado.
> **Predecessor**: slice 2 archivado (`main @ 2df231f`); canónico en `openspec/specs/moderation-automation/spec.md` (21 REQs).
> **Authority**: bugfix `#172` (`permissionOkAdmin` usa `BotStatus == StatusAdministrator`, NUNCA `can_*`).
>
> Esta spec **amenda** el canónico de slice 2 vía `## ADDED Requirements` (REQ-22 a REQ-31). Archive usa la técnica APPEND del slice 1→2 (delta → canónico; REQ-1..REQ-21 previos preservados, REQ-22..REQ-31 nuevos appendeados). No se crea spec paralelo.

### Requirement: Schema `warn_user_enabled` y `warn_user_template`

El sistema MUST extender `group_moderation_settings` con 2 columnas
nuevas vía migración goose `00008_add_warning_settings.sql` (SQL
plano, Up/Down reversibles, en `backend/migrations/`):
`warn_user_enabled BOOLEAN NOT NULL DEFAULT true` y
`warn_user_template TEXT NULL`. La migración MUST ser no-destructiva
con las 13 columnas previas intactas. Ningún CHECK constraint sobre
`warn_user_template` (texto libre, validación en cliente y server).

#### Scenario: Up agrega 2 columnas con defaults

- GIVEN la base con la migración 00007 aplicada (slice 2 archivado)
- WHEN se ejecuta `goose up` para 00008
- THEN `group_moderation_settings` tiene
  `warn_user_enabled BOOLEAN NOT NULL DEFAULT true` y
  `warn_user_template TEXT NULL`; las 13 columnas previas quedan
  intactas; las filas existentes quedan con `warn_user_enabled=true`
  y `warn_user_template=NULL`

#### Scenario: Down revierte sin pérdida colateral

- GIVEN la base con la migración 00008 aplicada
- WHEN se ejecuta `goose down` para 00008
- THEN `warn_user_template` y `warn_user_enabled` se eliminan; las
  13 columnas previas y los datos persisten intactos

### Requirement: Defaults al auto-crear settings

Cuando el sistema auto-crea una fila en `group_moderation_settings`
(settings no existe), MUST usar los defaults de slice 1+2 + los
nuevos de slice 2.1: `warn_user_enabled=true`,
`warn_user_template=nil`. El helper `DefaultSettings(groupID)` MUST
retornar `WarnUserEnabled: true` y `WarnUserTemplate: nil`. La
creación es idempotente (UPSERT).

#### Scenario: Primera lectura crea fila con `warn_user_enabled=true`

- GIVEN un grupo sin fila en `group_moderation_settings`
- WHEN `HandleMessage` se invoca con un mensaje de ese grupo
- THEN la fila creada tiene `warn_user_enabled=true` y
  `warn_user_template=NULL` junto con los 13+2 defaults previos

#### Scenario: Defaults consistentes entre memoria y DB

- GIVEN el código compilado
- WHEN se importa `automation.DefaultSettings`
- THEN retorna struct con `WarnUserEnabled == true` y
  `WarnUserTemplate == nil`

### Requirement: Plantillas hardcoded y override por grupo

El sistema MUST mantener 2 plantillas default hardcoded en
`automation/templates.go` (español Rioplatense):

- **pre-mute**: `"⚠️ {nombre}, llevás {count} advertencias. Si seguís, serás silenciado por {mute_minutes} min."`
- **pre-ban**: `"⚠️ {nombre}, llevás {count} advertencias. Si seguís, serás expulsado del grupo."`

Si el grupo tiene `warn_user_template` no-nulo y no-vacío, el sistema
MUST usar esa plantilla custom en lugar del default. Si el render de
la plantilla custom falla (placeholder desconocido, encoding raro,
string vacío después de trim), el sistema MUST caer al default
pre-mute o pre-ban según el threshold; MUST loggear un warning
estructurado con `template_used="default"` y
`reason="custom_render_fallback"`; MUST NO crashear el pipeline.

#### Scenario: Sin override usa default pre-mute

- GIVEN `warn_user_template=NULL`
- WHEN `RenderTemplate(WarningPreMute, nil, msg, settings, count)` se invoca
- THEN retorna el string hardcoded pre-mute con `{nombre}`,
  `{count}`, `{mute_minutes}` substituidos

#### Scenario: Con override usa custom pre-ban

- GIVEN `warn_user_template="🚨 {nombre}, ojo: vas por {count} strikes."`
- WHEN `RenderTemplate(WarningPreBan, &custom, msg, settings, count)` se invoca
- THEN retorna el string custom con substituciones aplicadas; el
  default NO se usa

#### Scenario: Template vacío cae al default

- GIVEN `warn_user_template=""` (string vacío)
- WHEN `RenderTemplate` se invoca
- THEN retorna el default pre-mute (o pre-ban según `kind`); NO
  panic, NO error propagado al caller

### Requirement: Substituciones de placeholders

`RenderTemplate` MUST substituir placeholders en este orden
estricto:

- `{nombre}` → `msg.From.FirstName` si no vacío; sino
  `msg.From.Username` sin el prefijo `@` si no vacío; sino literal
  `"este usuario"`.
- `{count}` → el `count` entero pasado al helper (post-increment
  `ws.WarningCount`).
- `{mute_minutes}` → `settings.AutomuteMinutes` (solo si está
  presente en el template).

Cualquier placeholder desconocido (ej. `{foo}`) MUST preservarse
**verbatim** en el output sin causar error. Las substituciones MUST
ser case-sensitive (`{Nombre}` ≠ `{nombre}`). El sistema MUST NO
usar el template para SQL ni para HTML — el render es plain text.

#### Scenario: FirstName disponible

- GIVEN `msg.From.FirstName="Juan"`, `Username=""`, `count=2`
- WHEN `RenderTemplate(WarningPreMute, nil, msg, settings, 2)` corre
- THEN el output contiene `"Juan"` (no `"este usuario"`, no el
  username)

#### Scenario: Fallback a Username sin `@`

- GIVEN `msg.From.FirstName=""`, `msg.From.Username="@juanp"`,
  `count=2`
- WHEN `RenderTemplate` corre
- THEN el output contiene `"juanp"` (sin `@`)

#### Scenario: Fallback final a literal

- GIVEN `msg.From.FirstName=""`, `msg.From.Username=""`, `count=2`
- WHEN `RenderTemplate` corre
- THEN el output contiene `"este usuario"`; NO panic

#### Scenario: Placeholder desconocido preservado

- GIVEN template custom `"{nombre} tiene {foo} y {count} advertencias"`
- WHEN `RenderTemplate` corre con `FirstName="Ana"`, `count=3`
- THEN el output es `"Ana tiene {foo} y 3 advertencias"`
  (placeholder `{foo}` literal)

### Requirement: Trigger del send en HandleMessage paso 7.5

En `Service.HandleMessage`, **después** del upsert del warning state
(paso 7) y **antes** de los checks de threshold (paso 8), el sistema
MUST invocar `warningSender.SendWarning` **SI Y SOLO SI** todas estas
condiciones se cumplen simultáneamente:

1. `ws.WarningCount > 0` (NO enviar antes del primer hit).
2. `ws.WarningCount == settings.AutomuteWarnings - 1` (pre-mute)
   **OR** `ws.WarningCount == settings.AutobanWarnings - 1` (pre-ban).
3. `settings.WarnUserEnabled == true`.
4. `permissionOkAdmin(group)` ya pasó en el paso 4 (NO re-check aquí;
   lo hace el sender internamente).

El trigger MUST NO abortar el pipeline si el send falla o tarda:
cualquier error del sender se loggea y se continúa con el paso 8
(threshold check). El helper `thresholdKindFor(settings, count)`
resuelve el `WarningKind` (`"pre_mute"` / `"pre_ban"` / `""` si
ninguno aplica) y la llamada solo se hace si `kind != ""`.

#### Scenario: `count == automute_warnings - 1` envía pre-mute

- GIVEN `AutomuteWarnings=3`, `AutobanWarnings=5`,
  `WarnUserEnabled=true`, `ws.WarningCount=2`,
  `permissionOkAdmin=true`
- WHEN `HandleMessage` corre
- THEN `SendWarning` se invoca con `kind="pre_mute"`, `count=2`; el
  log `WARN_USER_SENT` se registra

#### Scenario: `count == autoban_warnings - 1` envía pre-ban

- GIVEN `AutomuteWarnings=3`, `AutobanWarnings=5`,
  `WarnUserEnabled=true`, `ws.WarningCount=4`
- WHEN `HandleMessage` corre
- THEN `SendWarning` se invoca con `kind="pre_ban"`, `count=4`

#### Scenario: `count=0` NO envía

- GIVEN `ws.WarningCount=0` (recién reseteado por expiración)
- WHEN `HandleMessage` corre y dispara un hit que lleva count a 1
- THEN NO se llama `SendWarning` (count=1 ≠ automute-1=2 ni
  autoban-1=4)

#### Scenario: Toggle off salta el send

- GIVEN `WarnUserEnabled=false`, `ws.WarningCount=2`,
  `AutomuteWarnings=3`
- WHEN `HandleMessage` corre
- THEN NO se llama `SendWarning`; el pipeline continúa al threshold
  check

#### Scenario: `count == threshold` (acción en marcha) NO envía

- GIVEN `AutomuteWarnings=3`, `ws.WarningCount=3`
  (count == threshold)
- WHEN `HandleMessage` corre
- THEN NO se llama `SendWarning` (count=3 ≠ automute-1=2); la
  auto-action mute se encola normalmente en paso 8

### Requirement: Edge case `AutomuteWarnings == AutobanWarnings`

Cuando `AutomuteWarnings == AutobanWarnings` (ej. ambos = 3), el
helper `thresholdKindFor` MUST retornar `WarningPreBan` para
`count == AutomuteWarnings - 1` (prioriza pre-ban sobre pre-mute).
El resultado: solo **UN** warning se envía por `count=2`, NO dos.
Esto aplica para cualquier valor compartido.

#### Scenario: `automute=autoban=3` produce UN warning pre-ban en `count=2`

- GIVEN `AutomuteWarnings=3`, `AutobanWarnings=3`,
  `WarnUserEnabled=true`, `ws.WarningCount=2`
- WHEN `HandleMessage` corre
- THEN `SendWarning` se invoca **exactamente 1 vez** con
  `kind="pre_ban"`; NO se invoca con `kind="pre_mute"`

#### Scenario: `thresholdKindFor` prioridad

- GIVEN `AutomuteWarnings=5`, `AutobanWarnings=5`, `count=4`
- WHEN `thresholdKindFor(settings, 4)` retorna
- THEN retorna `WarningPreBan` (no `WarningPreMute`)

#### Scenario: helper defensivo retorna vacío si `count` no aplica

- GIVEN `AutomuteWarnings=3`, `AutobanWarnings=5`, `count=1`
- WHEN `thresholdKindFor(settings, 1)` retorna
- THEN retorna `""` (string vacío); el caller NO invoca `SendWarning`

### Requirement: Interfaz `WarningSender` y semántica de envío

El paquete `automation` MUST exponer:

- `type WarningKind string` con constantes
  `WarningPreMute WarningKind = "pre_mute"` y
  `WarningPreBan WarningKind = "pre_ban"`.
- `type WarningSender interface { SendWarning(ctx context.Context, msg *telegram.Message, count int16, kind WarningKind) error }`.

La implementación concreta `tgWarningSender` MUST:

1. Re-validar `permissionOkAdmin(group)` antes de invocar el send
   (bugfix `#172` invariante: `BotStatus == StatusAdministrator`,
   NUNCA claves `can_*`).
2. Si permission falla → log warn con `status=PERMISSION_DENIED`,
   `ActorID=nil`; NO enviar; retornar error.
3. Usar `context.WithTimeout(5s)` para acotar el send.
4. Invocar
   `telegram.Service.SendMessage(ctx, chatID, renderedText, false, nil)`
   (firma de 5 argumentos; `keyboard=nil`).
5. Registrar log éxito con
   `ActionWarnUserSent = "WARN_USER_SENT"`, `ActorID=nil`,
   `metadata={warning_count, threshold_kind, template_used: "default"|"custom"}`.

Si el send retorna error o el timeout expira → log warn + return
error (NO panic). El caller en `HandleMessage` loggea el fallo y
continúa al paso 8.

#### Scenario: Happy path logea SUCCESS

- GIVEN `permissionOkAdmin=true`, render exitoso,
  `telegram.SendMessage` retorna message_id sin error
- WHEN `SendWarning(ctx, msg, 2, WarningPreMute)` corre
- THEN retorna `nil`; existe log `WARN_USER_SENT` con
  `actor_id=NULL`, `status=SUCCESS`,
  `metadata={warning_count:2, threshold_kind:"pre_mute", template_used:"default"}`

#### Scenario: Bot removido entre hit y send → skip silencioso

- GIVEN `permissionOkAdmin=false` (bot dejó de ser admin entre paso
  4 y paso 7.5)
- WHEN `SendWarning` corre
- THEN NO se invoca `telegram.SendMessage`; log warn con
  `status=PERMISSION_DENIED`,
  `error_message="bot not administrator"`; retorna error

#### Scenario: Timeout 5s retorna error

- GIVEN `telegram.SendMessage` tarda > 5s (red lenta)
- WHEN `SendWarning` corre con `context.WithTimeout(5s)`
- THEN retorna `context.DeadlineExceeded` (envuelto); log warn con
  `status=TELEGRAM_ERROR`, `error_message="send timeout"`; el caller
  en HandleMessage continúa al paso 8

#### Scenario: Template custom usado se refleja en metadata

- GIVEN `warn_user_template="custom text {count}"`, render custom
  exitoso
- WHEN `SendWarning` corre
- THEN el log `WARN_USER_SENT` tiene
  `metadata.template_used="custom"` (no `"default"`)

### Requirement: Frontend `GroupAutomationPage` Sección 5

`GroupAutomationPage` MUST renderizar una **Sección 5** ("Warning al
usuario") entre Umbrales y Listas (o al final antes del botón
Guardar), conteniendo:

- Un `<Switch>` Mantine v7 con label
  `"Avisar al usuario antes de silenciar/expulsar"`,
  `checked={draftSettings.warn_user_enabled}`, `onChange` que
  actualiza el draft.
- Un `<Textarea autosize>` Mantine v7 con label
  `"Plantilla del warning (opcional)"`,
  `placeholder={defaultPreMuteTemplate}` (string del default
  pre-mute), `value={draftSettings.warn_user_template ?? ""}`,
  `maxLength={1000}`, `minRows={2}`, `maxRows={5}`.
- Un `<Text size="xs" c="dimmed">` debajo del Textarea listando las
  variables disponibles:
  `"Variables: {nombre}, {count}, {mute_minutes}"`.

Los tipos TypeScript MUST extenderse: `AutomationSettings` con
`warn_user_enabled: boolean` + `warn_user_template: string | null`;
`AutomationSettingsUpdate` con ambos opcionales;
`AUTOMATION_DEFAULTS` con `warn_user_enabled: true`. El helper
`withDefaults` MUST incluir los 2 nuevos campos. El Save flow
existente (`Promise.all` sobre `Object.keys(AUTOMATION_DEFAULTS)`)
MUST cubrirlos automáticamente sin cambios en el handler.

#### Scenario: Render inicial con defaults

- GIVEN `mockFetchRoutes` configurado para devolver settings
  `warn_user_enabled=true, warn_user_template=null`
- WHEN `GroupAutomationPage` se monta en `/groups/g/automation`
- THEN el Switch refleja `checked=true`; el Textarea está vacío; el
  helper text muestra
  `"Variables: {nombre}, {count}, {mute_minutes}"`

#### Scenario: Modificar template entra en el diff del Save

- GIVEN la página con `warn_user_template=null`
- WHEN el admin tipea
  `"⚠️ {nombre}, tenés {count} strikes"` y clickea Guardar
- THEN `settingsDelta` incluye
  `warn_user_template: "⚠️ {nombre}, tenés {count} strikes"`; el PUT
  settings lo envía al backend

#### Scenario: Tests slice 2 siguen verdes

- GIVEN los 8 tests existentes de `GroupAutomationPage.test.tsx`
- WHEN `npm test` corre
- THEN los 8 tests pasan; los 2 nuevos smoke tests se agregan al
  final

### Requirement: Tests §21.1 estricto

Los tests MUST cubrir (§21.1 — cero llamadas Bot API reales; fakes
hand-rolled):

- **Backend unit — templates** (`automation/templates_test.go`, NEW,
  ≥3 casos): FirstName disponible; fallback a Username sin `@`;
  fallback a "este usuario"; `{count}` y `{mute_minutes}`
  substituidos; template vacío → default; placeholder desconocido →
  literal verbatim.
- **Backend unit — service** (`automation/service_test.go`, MOD,
  ≥5 casos nuevos): `count=automute-1` dispara pre-mute;
  `count=autoban-1` dispara pre-ban; `count=0` skip;
  `count=threshold` skip; `WarnUserEnabled=false` skip;
  `automute==autoban` produce UN solo warning pre-ban.
- **Backend unit — warning_sender** (`automation/warning_sender_test.go`,
  NEW, ≥4 casos): happy path (telegram fake recibe `SendMessage` +
  log `SUCCESS`); `ErrPermissionDenied` mapeado a log
  `PERMISSION_DENIED` sin panic; timeout 5s retorna error y loggea
  `TELEGRAM_ERROR`; template custom vacío → fallback default en log
  metadata.
- **Backend integration — repository**
  (`automation/repository_test.go`, MOD, ≥2 casos nuevos con
  `OpenTestDB("automation")` + goose 00008 + TRUNCATE): insert con
  defaults `warn_user_enabled=true, warn_user_template=NULL`;
  round-trip del template custom.
- **Backend handler** (`automation_handlers_test.go`, MOD, ≥1 caso):
  PUT settings acepta body con `warn_user_enabled` +
  `warn_user_template` no-vacío ≤1000 chars; persiste; devuelve 200
  con la fila completa. PUT con template >1000 chars → 400
  `VALIDATION_ERROR`.
- **Frontend** (`GroupAutomationPage.test.tsx`, MOD, ≥2 casos
  nuevos): renderiza el switch `warn_user_enabled` con su label; el
  Textarea acepta texto y el cambio entra en el diff del Save.
- **Frontend non-regression**: los 8 tests slice 2 existentes MUST
  seguir pasando.

#### Scenario: Template — FirstName disponible

- GIVEN `msg.From.FirstName="Juan"`, render template pre-mute
- WHEN `RenderTemplate` corre
- THEN el output contiene `"Juan, llevás N advertencias"`

#### Scenario: Service — `count=2` con `automute=3` → SendWarning pre-mute

- GIVEN fake `WarningSender` que captura invocaciones,
  `AutomuteWarnings=3`, `WarnUserEnabled=true`,
  `ws.WarningCount=2` después del upsert
- WHEN `HandleMessage` corre
- THEN el fake registra 1 llamada con `kind=WarningPreMute`,
  `count=2`

#### Scenario: Service — `automute==autoban=3`, `count=2` → UN warning pre-ban

- GIVEN `AutomuteWarnings=3`, `AutobanWarnings=3`, fake
  `WarningSender`
- WHEN `HandleMessage` corre con `ws.WarningCount=2`
- THEN el fake registra **exactamente 1 llamada** con
  `kind=WarningPreBan`

#### Scenario: WarningSender — permission denied → no panic

- GIVEN fake `telegram.Service` que retorna
  `telegram.ErrPermissionDenied`, fake `groupsRepo` con
  `BotStatus=StatusMember`
- WHEN `SendWarning` corre
- THEN NO panic; log con `status=PERMISSION_DENIED`;
  `telegram.SendMessage` NO se invoca; retorna error

#### Scenario: WarningSender — timeout 5s retorna error

- GIVEN fake `telegram.Service.SendMessage` que duerme 6s
- WHEN `SendWarning` corre con contexto de 5s
- THEN retorna error en ≤5.1s; log con `status=TELEGRAM_ERROR`,
  `error_message~="timeout"`; el caller continúa

#### Scenario: Repository — defaults al crear settings nuevos

- GIVEN Postgres con migración 00008 aplicada; grupo `g` sin fila
- WHEN `repo.UpsertSettings(ctx, g, &DefaultSettings{})` corre
- THEN la fila insertada tiene `warn_user_enabled=true` y
  `warn_user_template=NULL`

#### Scenario: Handler — PUT con template ≤1000 chars acepta

- GIVEN handler con fake repo, body
  `{warn_user_enabled: false, warn_user_template: "custom {count}"}`
- WHEN `PUT /api/groups/g/automation/settings` corre
- THEN responde 200; la fila persistida tiene
  `warn_user_enabled=false` y
  `warn_user_template="custom {count}"`

#### Scenario: Handler — PUT con template >1000 chars rechaza

- GIVEN body con `warn_user_template` de 1001 caracteres
- WHEN `PUT` corre
- THEN responde 400 `VALIDATION_ERROR` con mensaje legible; la fila
  NO se modifica

#### Scenario: Frontend — Switch de `warn_user_enabled` renderiza

- GIVEN `mockFetchRoutes` con settings `warn_user_enabled=true`
- WHEN se renderiza `GroupAutomationPage`
- THEN existe un Switch con label
  `"Avisar al usuario antes de silenciar/expulsar"` en
  `checked=true`

### Requirement: No regresión e invariantes

- `backend/internal/moderation/` (acciones manuales: ban/unban/
  mute/unmute/delete/pin/lock/unlock/approve/reject) MUST quedar
  intacto (bugfix `#172` fuera de scope).
- El helper `permissionOkAdmin(group)` (en paquete `automation`,
  usa `BotStatus == StatusAdministrator`) MUST ser el ÚNICO check
  de permisos para `sendMessage` — NUNCA claves `can_*`
  (`grep can_ backend/internal/automation/*.go | grep -v '^//'` = 0
  matches sobre checks de permisos).
- Los paths de slice 1 (pipeline 8 pasos, threshold check,
  auto-action worker) y slice 2 (pre-load de listas, `AntiSpamRule`,
  `AntiLinkRule`, `BannedWordsRule`, Registry order
  cheap→expensive) MUST quedar intactos.
- §21.1 estricto: cero llamadas Bot API reales en tests (fakes
  hand-rolled sobre `telegram.Service`).
- El send síncrono con timeout 5s MUST NO bloquear el bus más allá
  del peor caso (5s + log warn + continue). El pipeline
  `HandleMessage` retorna en ≤5s+ε aunque Telegram se cuelgue.
- Sin secretos en el repo (§25): `TELEGRAM_BOT_TOKEN`, `JWT_SECRET`,
  passwords MUST NO aparecer en diffs de código.
- Sin cambios en
  `frontend/src/pages/{DashboardPage, PublicationsPage, LoginPage,
  GroupsPage, GroupUsersPage, GroupRequestsPage, GroupLogsPage}.tsx`.
- `GroupDetailPage` MUST quedar intacto salvo el link existente a
  `/groups/:id/automation`.
- Migración 00008 es compatible: NO rompe ALTER pendiente;
  `cfg.RunMigrations=true` aplica al `docker compose up`; en
  producción, paso manual documentado en README (AGENTS §13.1).

#### Scenario: `backend/internal/moderation/` intacto

- GIVEN el branch `feat/moderation-automation-slice2.1`
- WHEN `git diff main -- backend/internal/moderation/` corre
- THEN el output está vacío

#### Scenario: `permissionOkAdmin` invariante

- GIVEN el branch
- WHEN
  `grep can_ backend/internal/automation/*.go | grep -v '^//'`
  corre
- THEN 0 matches sobre checks de permisos (`can_*`); solo se ve
  `permissionOkAdmin` usando
  `BotStatus == StatusAdministrator`

#### Scenario: Slice 1+2 paths intactos

- GIVEN el branch
- WHEN `go test ./internal/automation/...` corre con los tests
  existentes de slice 1+2
- THEN todos los tests preexistentes (incluyendo `FloodRule` 5 casos,
  `Service` 5+ casos slice 1, `Registry` order,
  `Rule.Evaluate` con `lists *Lists`, pre-load de listas) siguen
  verdes sin modificación

#### Scenario: Frontend no-regression

- GIVEN el branch
- WHEN `npm test` corre sobre `GroupAutomationPage.test.tsx`
- THEN los 8 tests slice 2 + 2 nuevos tests slice 2.1 pasan en
  verde

#### Scenario: Pipeline no bloquea más de 5s

- GIVEN fake `telegram.Service.SendMessage` que duerme 10s
- WHEN `HandleMessage` corre
- THEN retorna en ≤5.1s con el warning loggeado como
  `TELEGRAM_ERROR`; el threshold check (paso 8) corre normalmente;
  el bus no queda bloqueado

#### Scenario: §21.1 — sin llamadas Bot API reales

- GIVEN el branch
- WHEN
  `grep "TELEGRAM_BOT_TOKEN" backend/internal/automation/*.go` y
  `grep "https://api.telegram.org" backend/internal/automation/*.go`
  corren
- THEN 0 matches (los tests usan la interfaz `telegram.Service`
  mockeada)
