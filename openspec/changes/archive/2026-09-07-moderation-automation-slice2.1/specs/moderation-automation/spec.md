# Delta Spec for `moderation-automation` — Slice 2.1 (Warning al usuario pre-acción)

> **Change**: `moderation-automation-slice2.1` — sub-slice de Fase 3 (AGENTS §23) sobre slice 2 archivado.
> **Predecessor**: slice 2 archivado (`main @ 2df231f`, REQ-1..21 en canónico).
> **Base canónica a enmendar**: `openspec/specs/moderation-automation/spec.md` (21 REQs). Archive phase APPENDEARÁ REQ-22..31 al canónico preservando REQ-1..21 intactos.
> **Strategy**: single-pr con `size:exception` (precedente 9 PRs consecutivos).
> **Authority**: bugfix `#172` (`permissionOkAdmin` usa `BotStatus == StatusAdministrator`, NUNCA `can_*`); AGENTS §11, §14, §18.1, §21.1.
>
> Esta spec **amenda** el canónico de slice 2 vía `## ADDED Requirements` (REQ-22 a REQ-31). No se crea spec paralelo. No se modifican REQ-1..21 (no hay `## MODIFIED Requirements` ni `## REMOVED Requirements` en este delta).

---

## ADDED Requirements

### Requirement: REQ-22 — Schema `warn_user_enabled` y `warn_user_template`

El sistema MUST extender `group_moderation_settings` con 2 columnas nuevas vía migración goose `00008_add_warning_settings.sql` (SQL plano, Up/Down reversibles, en `backend/migrations/`): `warn_user_enabled BOOLEAN NOT NULL DEFAULT true` y `warn_user_template TEXT NULL`. La migración MUST ser no-destructiva con los 13+2 columnas previas intactas. Ningún CHECK constraint sobre `warn_user_template` (texto libre, validación en cliente y server).

#### Scenario: Up agrega 2 columnas con defaults

- GIVEN la base con la migración 00007 aplicada (slice 2 archivado)
- WHEN se ejecuta `goose up` para 00008
- THEN `group_moderation_settings` tiene `warn_user_enabled BOOLEAN NOT NULL DEFAULT true` y `warn_user_template TEXT NULL`; las 13 columnas previas quedan intactas; las filas existentes quedan con `warn_user_enabled=true` y `warn_user_template=NULL`

#### Scenario: Down revierte sin pérdida colateral

- GIVEN la base con la migración 00008 aplicada
- WHEN se ejecuta `goose down` para 00008
- THEN `warn_user_template` y `warn_user_enabled` se eliminan; las 13 columnas previas y los datos persisten intactos

---

### Requirement: REQ-23 — Defaults al auto-crear settings

Cuando el sistema auto-crea una fila en `group_moderation_settings` (Settings no existe), MUST usar los defaults de REQ-1 + los nuevos defaults de slice 2.1: `warn_user_enabled=true`, `warn_user_template=nil`. El helper `DefaultSettings` MUST retornar `WarnUserEnabled: true` y `WarnUserTemplate: nil`. La creación es idempotente (UPSERT).

#### Scenario: Primera lectura crea fila con warn_user_enabled=true

- GIVEN un grupo sin fila en `group_moderation_settings`
- WHEN `HandleMessage` se invoca con un mensaje de ese grupo
- THEN la fila creada tiene `warn_user_enabled=true` y `warn_user_template=NULL` junto con los 13 defaults previos

#### Scenario: Defaults consistentes entre memoria y DB

- GIVEN el código compilado
- WHEN se importa `automation.DefaultSettings`
- THEN retorna struct con `WarnUserEnabled == true` y `WarnUserTemplate == nil`

---

### Requirement: REQ-24 — Plantillas hardcoded y override por grupo

El sistema MUST mantener 2 plantillas default hardcoded en `automation/templates.go` (español Rioplatense):

- **pre-mute**: `"⚠️ {nombre}, llevás {count} advertencias. Si seguís, serás silenciado por {mute_minutes} min."`
- **pre-ban**: `"⚠️ {nombre}, llevás {count} advertencias. Si seguís, serás expulsado del grupo."`

Si el grupo tiene `warn_user_template` no-nulo y no-vacío, el sistema MUST usar esa plantilla custom en lugar del default. Si el render de la plantilla custom falla (placeholder desconocido, encoding raro, string vacío después de trim), el sistema MUST caer al default pre-mute o pre-ban según el threshold; MUST loggear un warning estructurado con `template_used="default"` y `reason="custom_render_fallback"`; MUST NO crashear el pipeline.

#### Scenario: Sin override usa default pre-mute

- GIVEN `warn_user_template=NULL`
- WHEN `renderTemplate(WarningPreMute, nil, msg, settings, count)` se invoca
- THEN retorna el string hardcoded pre-mute con `{nombre}`, `{count}`, `{mute_minutes}` substituidos

#### Scenario: Con override usa custom pre-ban

- GIVEN `warn_user_template="🚨 {nombre}, ojo: vas por {count} strikes."`
- WHEN `renderTemplate(WarningPreBan, &custom, msg, settings, count)` se invoca
- THEN retorna el string custom con substituciones aplicadas; el default NO se usa

#### Scenario: Template vacío cae al default

- GIVEN `warn_user_template=""` (string vacío)
- WHEN `renderTemplate` se invoca
- THEN retorna el default pre-mute (o pre-ban según kind); NO panic, NO error propagado al caller

---

### Requirement: REQ-25 — Substituciones de placeholders

`renderTemplate` MUST substituir placeholders en este orden estricto:

- `{nombre}` → `msg.From.FirstName` si no vacío; sino `msg.From.Username` SIN el prefijo `@` si no vacío; sino literal `"este usuario"`.
- `{count}` → el `count` entero pasado al helper (post-increment `ws.WarningCount`).
- `{mute_minutes}` → `settings.AutomuteMinutes` (solo si está presente en el template).

Cualquier placeholder desconocido (ej. `{foo}`) MUST preservarse **verbatim** en el output sin causar error. Las substituciones MUST ser case-sensitive (`{Nombre}` ≠ `{nombre}`). El sistema MUST NO usar el template para SQL ni para HTML — el render es plain text.

#### Scenario: FirstName disponible

- GIVEN `msg.From.FirstName="Juan"`, `Username=""`, `count=2`
- WHEN `renderTemplate(WarningPreMute, nil, msg, settings, 2)` corre
- THEN el output contiene `"Juan"` (no `"este usuario"`, no el username)

#### Scenario: Fallback a Username sin `@`

- GIVEN `msg.From.FirstName=""`, `msg.From.Username="@juanp"`, `count=2`
- WHEN `renderTemplate` corre
- THEN el output contiene `"juanp"` (sin `@`)

#### Scenario: Fallback final a literal

- GIVEN `msg.From.FirstName=""`, `msg.From.Username=""`, `count=2`
- WHEN `renderTemplate` corre
- THEN el output contiene `"este usuario"`; NO panic

#### Scenario: Placeholder desconocido preservado

- GIVEN template custom `"{nombre} tiene {foo} y {count} advertencias"`
- WHEN `renderTemplate` corre con `FirstName="Ana"`, `count=3`
- THEN el output es `"Ana tiene {foo} y 3 advertencias"` (placeholder `{foo}` literal)

---

### Requirement: REQ-26 — Trigger del send en HandleMessage paso 7.5

En `Service.HandleMessage`, **después** del upsert del warning state (paso 7) y **antes** de los checks de threshold (paso 8), el sistema MUST invocar `warningSender.SendWarning` **SI Y SOLO SI** todas estas condiciones se cumplen simultáneamente:

1. `ws.WarningCount > 0` (NO enviar antes del primer hit).
2. `ws.WarningCount == settings.AutomuteWarnings - 1` (pre-mute) **OR** `ws.WarningCount == settings.AutobanWarnings - 1` (pre-ban).
3. `settings.WarnUserEnabled == true`.
4. `permissionOkAdmin(group)` ya pasó en el paso 4 (NO re-check aquí; lo hace el sender internamente).

El trigger MUST NO abortar el pipeline si el send falla o tarda: cualquier error del sender se loggea y se continúa con el paso 8 (threshold check). El helper `thresholdKindFor(settings, count)` resuelve el `WarningKind` ("pre_mute" / "pre_ban" / "" si ninguno aplica) y la llamada solo se hace si `kind != ""`.

#### Scenario: count == automute_warnings - 1 envía pre-mute

- GIVEN `AutomuteWarnings=3`, `AutobanWarnings=5`, `WarnUserEnabled=true`, `ws.WarningCount=2`, `permissionOkAdmin=true`
- WHEN `HandleMessage` corre
- THEN `SendWarning` se invoca con `kind="pre_mute"`, `count=2`; el log `WARN_USER_SENT` se registra

#### Scenario: count == autoban_warnings - 1 envía pre-ban

- GIVEN `AutomuteWarnings=3`, `AutobanWarnings=5`, `WarnUserEnabled=true`, `ws.WarningCount=4`
- WHEN `HandleMessage` corre
- THEN `SendWarning` se invoca con `kind="pre_ban"`, `count=4`

#### Scenario: count=0 NO envía

- GIVEN `ws.WarningCount=0` (justo después del upsert previo, pero se reinicia por expiración)
- WHEN `HandleMessage` corre y dispara un hit que lleva count a 1
- THEN NO se llama `SendWarning` (count=1 ≠ automute-1=2 ni autoban-1=4)

#### Scenario: Toggle off salta el send

- GIVEN `WarnUserEnabled=false`, `ws.WarningCount=2`, `AutomuteWarnings=3`
- WHEN `HandleMessage` corre
- THEN NO se llama `SendWarning`; el pipeline continúa al threshold check

#### Scenario: count == threshold (acción en marcha) NO envía

- GIVEN `AutomuteWarnings=3`, `ws.WarningCount=3` (count == threshold)
- WHEN `HandleMessage` corre
- THEN NO se llama `SendWarning` (count=3 ≠ automute-1=2); la auto-action mute se encola normalmente en paso 8

---

### Requirement: REQ-27 — Edge case `AutomuteWarnings == AutobanWarnings`

Cuando `AutomuteWarnings == AutobanWarnings` (ej. ambos = 3), el helper `thresholdKindFor` MUST retornar `WarningPreBan` para `count == AutomuteWarnings - 1` (prioriza pre-ban sobre pre-mute). El resultado: solo **UN** warning se envía por `count=2`, NO dos. Esto aplica para cualquier valor compartido.

#### Scenario: automute=autoban=3 produce UN warning pre-ban en count=2

- GIVEN `AutomuteWarnings=3`, `AutobanWarnings=3`, `WarnUserEnabled=true`, `ws.WarningCount=2`
- WHEN `HandleMessage` corre
- THEN `SendWarning` se invoca **exactamente 1 vez** con `kind="pre_ban"`; NO se invoca con `kind="pre_mute"`

#### Scenario: thresholdKindFor prioridad

- GIVEN `AutomuteWarnings=5`, `AutobanWarnings=5`, `count=4`
- WHEN `thresholdKindFor(settings, 4)` retorna
- THEN retorna `WarningPreBan` (no `WarningPreMute`)

#### Scenario: helper defensivo retorna vacío si count no aplica

- GIVEN `AutomuteWarnings=3`, `AutobanWarnings=5`, `count=1`
- WHEN `thresholdKindFor(settings, 1)` retorna
- THEN retorna `""` (string vacío); el caller NO invoca `SendWarning`

---

### Requirement: REQ-28 — Interfaz `WarningSender` y semántica de envío

El paquete `automation` MUST exponer:

- `type WarningKind string` con constantes `WarningPreMute WarningKind = "pre_mute"` y `WarningPreBan WarningKind = "pre_ban"`.
- `type WarningSender interface { SendWarning(ctx context.Context, groupID, userID int64, count int, kind WarningKind) error }`.

La implementación concreta `tgWarningSender` MUST:

1. Re-validar `permissionOkAdmin(group)` antes de invocar el send (bugfix `#172` invariante: `BotStatus == StatusAdministrator`, NUNCA claves `can_*`).
2. Si permission falla → log warn con `status=PERMISSION_DENIED`, `ActorID=nil`; NO enviar; retornar error.
3. Usar `context.WithTimeout(5s)` para acotar el send.
4. Invocar `telegram.Service.SendMessage(ctx, chatID, renderedText, false, nil)` (5-arg signature; `keyboard=nil`).
5. Registrar log éxito con `ActionWarnUserSent = "WARN_USER_SENT"`, `ActorID=nil`, `metadata={rule_name, warning_count, threshold_kind, template_used: "default"|"custom"}`.

Si el send retorna error o el timeout expira → log warn + return error (NO panic). El caller en `HandleMessage` loggea el fallo y continúa al paso 8.

#### Scenario: Happy path logea SUCCESS

- GIVEN `permissionOkAdmin=true`, render exitoso, `telegram.SendMessage` retorna message_id sin error
- WHEN `SendWarning(ctx, g, u, 2, WarningPreMute)` corre
- THEN retorna `nil`; existe log `WARN_USER_SENT` con `actor_id=NULL`, `status=SUCCESS`, `metadata={warning_count:2, threshold_kind:"pre_mute", template_used:"default"}`

#### Scenario: Bot removido entre hit y send → skip silencioso

- GIVEN `permissionOkAdmin=false` (bot dejó de ser admin entre paso 4 y paso 7.5)
- WHEN `SendWarning` corre
- THEN NO se invoca `telegram.SendMessage`; log warn con `status=PERMISSION_DENIED`, `error_message="bot not administrator"`; retorna error

#### Scenario: Timeout 5s retorna error

- GIVEN `telegram.SendMessage` tarda > 5s (red lenta)
- WHEN `SendWarning` corre con `context.WithTimeout(5s)`
- THEN retorna `context.DeadlineExceeded` (envuelto); log warn con `status=TELEGRAM_ERROR`, `error_message="send timeout"`; el caller en HandleMessage continúa al paso 8

#### Scenario: Template custom usado se refleja en metadata

- GIVEN `warn_user_template="custom text {count}"`, render custom exitoso
- WHEN `SendWarning` corre
- THEN el log `WARN_USER_SENT` tiene `metadata.template_used="custom"` (no `"default"`)

---

### Requirement: REQ-29 — Frontend `GroupAutomationPage` Sección 5

`GroupAutomationPage` MUST renderizar una **Sección 5** ("Warning al usuario") entre Umbrales y Listas (o al final antes del botón Guardar), conteniendo:

- Un `<Switch>` Mantine v7 con label `"Avisar al usuario antes de silenciar/expulsar"`, `checked={draftSettings.warn_user_enabled}`, `onChange` que actualiza el draft.
- Un `<Textarea autosize>` Mantine v7 con label `"Plantilla del warning (opcional)"`, `placeholder={defaultPreMuteTemplate}` (string del default pre-mute), `value={draftSettings.warn_user_template ?? ""}`, `maxLength={1000}`, `minRows={2}`, `maxRows={5}`.
- Un `<Text size="xs" c="dimmed">` debajo del Textarea listando las variables disponibles: `"Variables: {nombre}, {count}, {mute_minutes}"`.

Los tipos TypeScript MUST extenderse: `AutomationSettings` con `warn_user_enabled: boolean` + `warn_user_template: string | null`; `AutomationSettingsUpdate` con ambos opcionales; `AUTOMATION_DEFAULTS` con `warn_user_enabled: true`. El helper `withDefaults` MUST incluir los 2 nuevos campos. El Save flow existente (Promise.all sobre `Object.keys(AUTOMATION_DEFAULTS)`) MUST cubrirlos automáticamente sin cambios en el handler.

#### Scenario: Render inicial con defaults

- GIVEN `mockFetchRoutes` configurado para devolver settings `warn_user_enabled=true, warn_user_template=null`
- WHEN `GroupAutomationPage` se monta en `/groups/g/automation`
- THEN el Switch refleja `checked=true`; el Textarea está vacío; el helper text muestra `"Variables: {nombre}, {count}, {mute_minutes}"`

#### Scenario: Modificar template entra en el diff del Save

- GIVEN la página con `warn_user_template=null`
- WHEN el admin tipea `"⚠️ {nombre}, tenés {count} strikes"` y clickea Guardar
- THEN `settingsDelta` incluye `warn_user_template: "⚠️ {nombre}, tenés {count} strikes"`; el PUT settings lo envía al backend

#### Scenario: Tests slice 2 siguen verdes

- GIVEN los 8 tests existentes de `GroupAutomationPage.test.tsx`
- WHEN `npm test` corre
- THEN los 8 tests pasan; los 2 nuevos smoke tests se agregan al final

---

### Requirement: REQ-30 — Tests §21.1 estricto

Los tests MUST cubrir (§21.1 — cero llamadas Bot API reales; fakes hand-rolled o moq-generated):

- **Backend unit — templates** (`automation/templates_test.go`, NEW, ≥3 casos): FirstName disponible; fallback a Username sin `@`; fallback a "este usuario"; `{count}` y `{mute_minutes}` substituidos; template vacío → default; placeholder desconocido → literal verbatim.
- **Backend unit — service** (`automation/service_test.go`, MOD, ≥5 casos nuevos): count=automute-1 dispara pre-mute; count=autoban-1 dispara pre-ban; count=0 skip; count=threshold skip; WarnUserEnabled=false skip; automute==autoban produce UN solo warning pre-ban.
- **Backend unit — warning_sender** (`automation/warning_sender_test.go`, NEW, ≥4 casos): happy path (telegram fake recibe SendMessage + log SUCCESS); `ErrPermissionDenied` mapeado a log PERMISSION_DENIED sin panic; timeout 5s retorna error y loggea TELEGRAM_ERROR; template custom vacío → fallback default en log metadata.
- **Backend integration — repository** (`automation/repository_test.go`, MOD, ≥2 casos nuevos con `OpenTestDB("automation")` + goose 00008 + TRUNCATE): insert con defaults `warn_user_enabled=true, warn_user_template=NULL`; round-trip del template custom.
- **Backend handler** (`automation_handlers_test.go`, MOD, ≥1 caso): PUT settings acepta body con `warn_user_enabled` + `warn_user_template` no-vacío ≤1000 chars; persiste; devuelve 200 con la fila completa. PUT con template >1000 chars → 400 `VALIDATION_ERROR`.
- **Frontend** (`GroupAutomationPage.test.tsx`, MOD, ≥2 casos nuevos): renderiza el switch `warn_user_enabled` con su label; el Textarea acepta texto y el cambio entra en el diff del Save.
- **Frontend non-regression**: los 8 tests slice 2 existentes MUST seguir pasando.

#### Scenario: Template — FirstName disponible

- GIVEN `msg.From.FirstName="Juan"`, render template pre-mute
- WHEN `RenderTemplate` corre
- THEN el output contiene `"Juan, llevás N advertencias"`

#### Scenario: Service — count=2 con automute=3 → SendWarning pre-mute

- GIVEN fake `WarningSender` que captura invocaciones, `AutomuteWarnings=3`, `WarnUserEnabled=true`, `ws.WarningCount=2` después del upsert
- WHEN `HandleMessage` corre
- THEN el fake registra 1 llamada con `kind=WarningPreMute`, `count=2`

#### Scenario: Service — automute==autoban=3, count=2 → UN warning pre-ban

- GIVEN `AutomuteWarnings=3`, `AutobanWarnings=3`, fake `WarningSender`
- WHEN `HandleMessage` corre con `ws.WarningCount=2`
- THEN el fake registra **exactamente 1 llamada** con `kind=WarningPreBan`

#### Scenario: WarningSender — permission denied → no panic

- GIVEN fake `telegram.Service` que retorna `telegram.ErrPermissionDenied`, fake `groupsRepo` con `BotStatus=StatusMember`
- WHEN `SendWarning` corre
- THEN NO panic; log con `status=PERMISSION_DENIED`; `telegram.SendMessage` NO se invoca; retorna error

#### Scenario: WarningSender — timeout 5s retorna error

- GIVEN fake `telegram.Service.SendMessage` que duerme 6s
- WHEN `SendWarning` corre con contexto de 5s
- THEN retorna error en ≤5.1s; log con `status=TELEGRAM_ERROR`, `error_message~="timeout"`; el caller continúa

#### Scenario: Repository — defaults al crear settings nuevos

- GIVEN Postgres con migración 00008 aplicada; grupo `g` sin fila
- WHEN `repo.UpsertSettings(ctx, g, &DefaultSettings{})` corre
- THEN la fila insertada tiene `warn_user_enabled=true` y `warn_user_template=NULL`

#### Scenario: Handler — PUT con template ≤1000 chars acepta

- GIVEN handler con fake repo, body `{warn_user_enabled: false, warn_user_template: "custom {count}"}`
- WHEN `PUT /api/groups/g/automation/settings` corre
- THEN responde 200; la fila persistida tiene `warn_user_enabled=false` y `warn_user_template="custom {count}"`

#### Scenario: Handler — PUT con template >1000 chars rechaza

- GIVEN body con `warn_user_template` de 1001 caracteres
- WHEN `PUT` corre
- THEN responde 400 `VALIDATION_ERROR` con mensaje legible; la fila NO se modifica

#### Scenario: Frontend — Switch de warn_user_enabled renderiza

- GIVEN `mockFetchRoutes` con settings `warn_user_enabled=true`
- WHEN se renderiza `GroupAutomationPage`
- THEN existe un Switch con label `"Avisar al usuario antes de silenciar/expulsar"` en `checked=true`

---

### Requirement: REQ-31 — No regresión e invariantes

- `backend/internal/moderation/` (acciones manuales: ban/unban/mute/unmute/delete/pin/lock/unlock/approve/reject) MUST quedar intacto (bugfix `#172` fuera de scope).
- El helper `permissionOkAdmin(group)` (en paquete `automation`, usa `BotStatus == StatusAdministrator`) MUST ser el ÚNICO check de permisos para `sendMessage` — NUNCA claves `can_*` (`grep -rn "can_" backend/internal/automation/` = 0 matches sobre checks de permisos).
- Los paths de slice 1 (pipeline 8 pasos, threshold check, auto-action worker) y slice 2 (pre-load de listas, AntiSpamRule, AntiLinkRule, BannedWordsRule, Registry order cheap→expensive) MUST quedar intactos.
- §21.1 estricto: cero llamadas Bot API reales en tests (`mockFetchRoutes`/fakes + `telegram.Service` interface mockeada).
- El send síncrono con timeout 5s MUST NO bloquear el bus más allá del peor caso (5s + log warn + continue). El pipeline HandleMessage retorna en ≤5s+ε aunque Telegram se cuelgue.
- Sin secretos en el repo (§25): `TELEGRAM_BOT_TOKEN`, `JWT_SECRET`, passwords MUST NO aparecer en diffs de código.
- Sin cambios en `frontend/src/pages/{DashboardPage, PublicationsPage, LoginPage, GroupsPage, GroupUsersPage, GroupRequestsPage, GroupLogsPage}.tsx`.
- `GroupDetailPage` MUST quedar intacto salvo el link existente a `/groups/:id/automation`.
- Migración 00008 es compatible: NO rompe ALTER pendiente; `cfg.RunMigrations=true` aplica al `docker compose up`; en producción, paso manual documentado en README (AGENTS §13.1).

#### Scenario: `backend/internal/moderation/` intacto

- GIVEN el branch `feat/moderation-automation-slice2.1`
- WHEN `git diff main -- backend/internal/moderation/` corre
- THEN el output está vacío

#### Scenario: permissionOkAdmin invariante

- GIVEN el branch
- WHEN `grep -rn 'can_' backend/internal/automation/` corre
- THEN 0 matches sobre checks de permisos (`can_*`); solo se ve `permissionOkAdmin` usando `BotStatus == StatusAdministrator`

#### Scenario: Slice 1+2 paths intactos

- GIVEN el branch
- WHEN `go test ./internal/automation/...` corre con los tests existentes de slice 1+2
- THEN todos los tests preexistentes (incluyendo `FloodRule` 5 casos, `Service` 5+ casos slice 1, `Registry` order, `Rule.Evaluate` con `lists *Lists`, pre-load de listas) siguen verdes sin modificación

#### Scenario: Frontend no-regression

- GIVEN el branch
- WHEN `npm test` corre sobre `GroupAutomationPage.test.tsx`
- THEN los 8 tests slice 2 + 2 nuevos tests slice 2.1 pasan en verde

#### Scenario: Pipeline no bloquea más de 5s

- GIVEN fake `telegram.Service.SendMessage` que duerme 10s
- WHEN `HandleMessage` corre
- THEN retorna en ≤5.1s con el warning loggeado como `TELEGRAM_ERROR`; el threshold check (paso 8) corre normalmente; el bus no queda bloqueado

#### Scenario: §21.1 — sin llamadas Bot API reales

- GIVEN el branch
- WHEN `grep -rn "TELEGRAM_BOT_TOKEN" backend/internal/automation/` y `grep -rn "https://api.telegram.org" backend/internal/automation/` corren
- THEN 0 matches (los tests usan la interfaz `telegram.Service` mockeada)

---

## Sync Method (archive phase)

El archive phase APPENDEARÁ REQ-22..31 al canónico `openspec/specs/moderation-automation/spec.md` **después** del bloque "Slice 2 ADDED Requirements" (línea 1054 actual), preservando REQ-1..21 intactos. La nueva sección se titulará **"Slice 2.1 ADDED Requirements (2026-09-08 — Warning al usuario pre-acción)"** siguiendo el patrón de slice 2. Numeración REQ-22..31 evita colisión con REQ-1..21 existentes.