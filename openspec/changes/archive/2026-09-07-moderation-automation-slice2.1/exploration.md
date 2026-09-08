# Exploration: `moderation-automation-slice2.1` — Warning to user before threshold action

**Change**: `moderation-automation-slice2.1` — sub-slice de Fase 3 (AGENTS §23) que agrega la UX "advertir antes de actuar".
**Mode**: hybrid (filesystem + Engram). Persisted as `sdd/moderation-automation/slice2.1/exploration` + este archivo.
**Predecessor**: `moderation-automation-slice2` archivado (`main @ 2df231f`, obs #231); canónico `openspec/specs/moderation-automation/spec.md` (21 REQs).
**Authority**: bugfix `#172` (`permissionOkAdmin` usa `g.BotStatus == StatusAdministrator`, NUNCA `can_*`); AGENTS §11 (no Redis), §14 (modular backend), §18.1 (rate limit Telegram).
**Branch propuesta**: `feat/moderation-automation-slice2.1` base `main @ 2df231f`.

---

## Contexto heredado (base obligatoria)

**Estado actual verificable** (post-slice2 archivado, septiembre 2026):

- Backend monolito Go 1.22+, REST API, PostgreSQL 16 + `pgx/v5` stdlib, migraciones `goose` (00001-00007).
- Frontend React 19 + TypeScript + Vite + Mantine v7, React Query + React Router.
- `automation.Service.HandleMessage` (`backend/internal/automation/service.go`) ejecuta el pipeline de 8 pasos (REQ-7). **Paso 7 incrementa `ws.WarningCount++` (post-increment)** antes de la decisión de auto-action (paso 8). El counter persistido en `user_warning_state` (slice 1, migración 00006) ya es post-increment en `HandleMessage` (line 162 de service.go).
- `automation.Settings` (slice 1+2) tiene 13 columnas: `enabled`, 4 toggles, 6 thresholds (`flood_messages`, `flood_seconds`, `warning_limit`, `automute_warnings`, `automute_minutes`, `autoban_warnings`, `warning_expire_days`). Tabla `group_moderation_settings` con CHECK constraints.
- `telegram.Service.SendMessage(ctx, chatID int64, text string, disableWebPagePreview bool, keyboard *InlineKeyboardMarkup) (int64, error)` (`backend/internal/telegram/service.go:62`, implementada en `backend/internal/telegram/publications.go:48`). Pasa por `doWithRetry` (429 con retry_after, max 3 reintentos) + token bucket del Adapter (AGENTS §18.1). Para warnings pasamos `keyboard=nil` (no se necesitan botones inline; el warning es un mensaje unidireccional).
- **El bot NO necesita permisos extra para `sendMessage` siendo admin** (bugfix #172 + telegram-moderation REQ): `g.BotStatus == StatusAdministrator` alcanza para `sendMessage` en grupos. No hay `can_post_messages` en `permissionsFromMember`.
- `events.Bus` ya entrega `*telegram.Update.Message.From{ID, FirstName, Username}` (modelo `User` completo en `backend/internal/telegram/update.go`). El subscriber entrega el mensaje al Service.
- Auto-action worker (`backend/internal/automation/worker.go`) drena `autoActionCh` FIFO; el send del warning NO debe pasar por ahí (no es una auto-action destructiva, es feedback al usuario).
- Logs (`backend/internal/logs/model.go`): constantes `ActionAutomuteUser`/`ActionAutobanUser` con `ActorID=nil` (sistema). Patrón ya establecido en `autoactioner.go:128-147`.
- Frontend `GroupAutomationPage.tsx` (468 LOC, slice 2) tiene 4 secciones: General (1 switch), Reglas (4 sub-toggles), Umbrales (6 NumberInputs), Listas (2 TagsInput). Single Save con `Promise.all`. Settings se persisten como `AutomationSettingsUpdate` (PATCH-like, todos los campos opcionales).
- Frontend `AUTOMATION_DEFAULTS` en `features/automation/types.ts:48` define los defaults que el backend retorna en GET inicial.
- **§21.1 estricto**: cero llamadas Bot API reales en tests. Las signatures pasan por la interfaz `telegram.Service` y se mockean con fakes.

---

## Hechos confirmados de la Bot API

- `sendMessage(chat_id, text, ...)` para un chat ya devuelve el `message_id`; el bot puede enviar aunque el `mute` esté vigente para el target (es admin).
- Restricción: el bot debe seguir siendo admin (`BotStatus == StatusAdministrator`) para poder postear en el grupo.
- Si el bot es removido del grupo entre el hit y el sendMessage, la API devuelve 403 → mapeado a `telegram.ErrPermissionDenied` (autoactioner.go precedent).
- Sin rate limit particular por warning (es texto, mismo bucket que publicaciones; el adapter maneja 429 con `retry_after`).

---

## Decisiones (D1–D10)

### D1 — Cuándo enviar el warning

**Trigger**: en `Service.HandleMessage`, **después** del `ws.WarningCount++` (paso 7) y **antes** del check de threshold (paso 8). El counter ya es el post-increment.

**Condición** (todas simultáneas):
1. `ws.WarningCount > 0` (no enviar antes del primer hit).
2. `ws.WarningCount == settings.AutomuteWarnings - 1`  (pre-mute warning).
3. `ws.WarningCount == settings.AutobanWarnings - 1`   (pre-ban warning).
4. `settings.WarnUserEnabled == true` (toggle por grupo, default `true`).
5. `permissionOkAdmin(group)` ya pasó en el paso 4 del pipeline (no re-check).

**Edge case documentado**: si `AutomuteWarnings == AutobanWarnings` (e.g. ambos = 3), el warning pre-ban absorbe al pre-mute (mismo `count=2`). Solo se envía **un** warning. Documentar en REQ-22.

**Nunca** se envía en `count=0` (no hay warning todavía) ni en el count == threshold (la acción ocurre simultáneamente).

### D2 — Plantillas y substituciones

**Dos plantillas default** (hardcoded en `automation/templates.go`):

```
pre-mute:  "⚠️ {nombre}, llevás {count} advertencias. Si seguís, serás silenciado por {mute_minutes} min."
pre-ban:   "⚠️ {nombre}, llevás {count} advertencias. Si seguís, serás expulsado del grupo."
```

**Override por grupo** vía `warn_user_template TEXT NULL` en `group_moderation_settings`. Si la fila custom contiene `{mute_minutes}` pero el threshold es pre-ban (no hay mute_minutes relevante), se sustituye con el `AutomuteMinutes` del setting (sigue siendo informativo: el admin configuró ese valor).

**Substituciones** (`renderTemplate` en `automation/templates.go`):
- `{nombre}`: `msg.From.FirstName` si no vacío; sino `msg.From.Username` sin `@`; sino literal `"este usuario"`.
- `{count}`: post-increment `ws.WarningCount`.
- `{mute_minutes}`: `settings.AutomuteMinutes`.

**Robustez**: cualquier error de substitución (placeholder desconocido, template vacío) cae al default pre-mute o pre-ban según el threshold. Nunca crashea el pipeline. Log warn si el template custom no se pudo renderizar.

### D3 — Schema (migración 00008)

```sql
-- +goose Up
ALTER TABLE group_moderation_settings
    ADD COLUMN warn_user_enabled  BOOLEAN NOT NULL DEFAULT true,
    ADD COLUMN warn_user_template TEXT NULL;
-- +goose Down
ALTER TABLE group_moderation_settings
    DROP COLUMN warn_user_template,
    DROP COLUMN warn_user_enabled;
```

Sin CHECK constraints (template es libre; validamos en cliente). Sin nueva tabla. Compatible con slice 1+2 (los defaults de los 13 columnas previas quedan intactos).

### D4 — Interfaz `WarningSender`

Nueva interfaz consumer-side en `automation/warning_sender.go` (mismo patrón que `AutoActioner`):

```go
type WarningSender interface {
    SendWarning(ctx context.Context, groupID, userID int64, count int16, kind WarningKind) error
}

type WarningKind string
const (
    WarningPreMute WarningKind = "pre_mute"
    WarningPreBan  WarningKind = "pre_ban"
)
```

Implementación concreta `tgWarningSender` envuelve `telegram.Service` (vista mínima `TelegramMesseger`), `logs.LogWriter`, `SettingsReader` (para leer el template custom) y un `TemplateRenderer` interno. NO reusa `AutoActioner` (semántica distinta: feedback al usuario, no acción administrativa).

Re-check de `permissionOkAdmin(group)` antes de enviar (por si el bot fue removido entre el upsert del warning state y el send). Si falla → log warn, NO enviar (consistente con §25: no exponer secretos y no enviar mensajes sin admin).

Log éxito/fallo con `ActionWarnUserSent = "WARN_USER_SENT"`, `ActorID=nil`.

### D5 — Modificación de `Service.HandleMessage`

Insertar después del log `ActionRuleTriggered` (paso 7) y antes de los checks de threshold (paso 8):

```go
// Slice 2.1: enviar warning al usuario si corresponde.
s.warningSender.SendWarning(ctx, msg.Chat.ID, msg.From.ID, ws.WarningCount, thresholdKindFor(settings, ws.WarningCount))
```

**Sincrónico con timeout de 5s** vía `context.WithTimeout`. Si el send tarda más de 5s o falla por 429/403, log warn y continuar (no aborta el pipeline: el warning es best-effort). Decisión: complejidad de un canal + worker separado no se justifica para un mensaje de feedback — el adapter ya tiene rate-limit + retry, el bus no se bloquea porque HandleMessage es síncrono pero retorna rápido (SendMessage retorna en ms típicamente; 5s es cota generosa).

**Asignación de thresholdKind**: helper local `thresholdKindFor(s *Settings, count int16) WarningKind`:
- Si `count == AutobanWarnings - 1` → `WarningPreBan`.
- Else (count == AutomuteWarnings - 1) → `WarningPreMute`.
- Else → "" (no warning, no se llama SendWarning). Chequeo defensivo en `SendWarning` para no enviar si `kind == ""`.

### D6 — Frontend

**Sección 5 nueva en `GroupAutomationPage`** (entre Umbrales y Listas, o al final antes de Save):
- `<Switch label="Avisar al usuario antes de silenciar/expulsar" checked={draftSettings.warn_user_enabled} onChange=... />`.
- `<Textarea label="Plantilla del warning (opcional)" placeholder={defaultPreMuteTemplate} value={draftSettings.warn_user_template ?? ""} onChange=... autosize minRows={2} maxRows={5} />`.

**Tip**: bajo el Textarea, mostrar el placeholder con la plantilla default para que el admin sepa qué variables puede usar (`{nombre}`, `{count}`, `{mute_minutes}`). Pequeño `<Text size="xs" c="dimmed">` con la lista.

**Tipos**: extender `AutomationSettings` con `warn_user_enabled: boolean` + `warn_user_template: string | null`. Extender `AutomationSettingsUpdate` con ambos campos opcionales. Extender `AUTOMATION_DEFAULTS` con `warn_user_enabled: true` (default-on para que el feature esté "out of the box"). Extender el helper `withDefaults` para incluir el default.

**Save flow**: el `settingsDelta` ya iteraba sobre `Object.keys(AUTOMATION_DEFAULTS)`; con los 2 nuevos campos, el diff los cubre automáticamente. Sin cambios en el handler del Save (Promise.all sigue intacto).

### D7 — Tests

**Backend**:
- `automation/templates_test.go` (NEW): 3+ casos de substitución: nombre desde FirstName, fallback a Username, fallback a "este usuario", `{count}` y `{mute_minutes}` se sustituyen, template vacío → default, placeholder desconocido → se preserva literal.
- `automation/service_test.go`: 3+ casos para el path de warning:
  - `count=2` con `automute=3` → SendWarning llamado con `WarningPreMute`.
  - `count=0` → SendWarning NO llamado.
  - `count=3` (umbral) → SendWarning NO llamado (la acción se ejecuta).
  - `warn_user_enabled=false` → SendWarning NO llamado.
  - `AutomuteWarnings == AutobanWarnings == 3` → `count=2` produce UN solo warning pre-ban.
- `automation/warning_sender_test.go` (NEW): 4+ casos:
  - Happy path → log SUCCESS + telegram.SendMessage llamado con args correctos.
  - `telegram.ErrPermissionDenied` → log PERMISSION_DENIED, no panic.
  - Template custom con `{count}` → substituido correctamente.
  - Template custom vacío / con placeholder desconocido → fallback al default.
- `automation/handlers_test.go` (MOD): extender PUT settings test con los 2 nuevos campos (acepta, persiste, devuelve el row completo).

**Frontend**:
- `GroupAutomationPage.test.tsx`: 2 smoke tests nuevos:
  - "renderiza el switch de warn_user_enabled".
  - "el textarea de warn_user_template acepta texto y entra en el diff del Save".

### D8 — Logs (REVISIÓN del patrón)

- Nueva constante en `logs/model.go`: `ActionWarnUserSent = "WARN_USER_SENT"` (sistema, `ActorID=nil`, sigue el patrón de slice 1).
- Metadata: `{ rule_name, warning_count, threshold_kind, template_used: "default"|"custom" }`.

### D9 — Frontend non-regression

- `GroupAutomationPage` extiende secciones pero el flujo de Save/Discard es idéntico.
- `GroupAutomationPage.test.tsx` (8 tests existentes, slice 2) deben seguir pasando; los 2 nuevos smoke tests se agregan al final.
- `GroupDetailPage` (link "Configurar automatización") intacto.

### D10 — Permission check invariante

`WarningSender` reusa `permissionOkAdmin(group)` local al paquete automation. **No** introduce nuevo helper ni reutiliza `moderation.permissionOk` (bugfix #172). El comentario en `warning_sender.go:8` documenta la invariante explícitamente.

---

## Affected Areas

### Backend NUEVO

| Archivo | LOC est. |
|---------|---------:|
| `backend/migrations/00008_add_warning_settings.sql` | +15 |
| `backend/internal/automation/templates.go` | +60 |
| `backend/internal/automation/templates_test.go` | +80 |
| `backend/internal/automation/warning_sender.go` | +130 |
| `backend/internal/automation/warning_sender_test.go` | +150 |

### Backend MOD

| Archivo | LOC est. | Notas |
|---------|---------:|-------|
| `backend/internal/automation/model.go` | +10 | Agregar `WarnUserEnabled bool` + `WarnUserTemplate *string` a `Settings`. `DefaultSettings` con `WarnUserEnabled=true, WarnUserTemplate=nil`. |
| `backend/internal/automation/repository.go` | +20 | 2 columnas en SELECT + 2 placeholders en INSERT/UPDATE + scan field. |
| `backend/internal/automation/repository_test.go` | +30 | 2 integration tests: defaults al insertar, round-trip del template. |
| `backend/internal/automation/service.go` | +25 | Inyectar `warningSender` en `NewService`; llamada en HandleMessage paso 7.5; helper `thresholdKindFor`. |
| `backend/internal/automation/service_test.go` | +60 | 5+ casos de warning trigger. |
| `backend/internal/logs/model.go` | +3 | `ActionWarnUserSent = "WARN_USER_SENT"`. |
| `backend/cmd/server/main.go` | +5 | `sender := automation.NewWarningSender(bot, logsRepo, automationRepo, slog.Default())` + pasar a `NewService`. |
| `backend/internal/api/automation_handlers.go` | +5 | Validar `warn_user_enabled` y `warn_user_template` en PUT (template <= 1000 chars si no nil). |

### Frontend MOD

| Archivo | LOC est. | Notas |
|---------|---------:|-------|
| `frontend/src/features/automation/types.ts` | +10 | 2 campos en `AutomationSettings`, 2 en `AutomationSettingsUpdate`, 2 en `AUTOMATION_DEFAULTS`. |
| `frontend/src/pages/GroupAutomationPage.tsx` | +50 | Sección 5 con Switch + Textarea + helper text con placeholders. |
| `frontend/src/pages/GroupAutomationPage.test.tsx` | +30 | 2 smoke tests. |

### Docs

| Archivo | LOC est. | Notas |
|---------|---------:|-------|
| `README.md` | +30 | Sección "Warning al usuario" bajo "Moderación automática". |

**Total**: ~710 LOC. Bajo el budget de 400 líneas POR POCO — `size:exception` solicitada (precedente 9 PRs consecutivos). Forecast confirmado en `sdd-tasks`.

---

## Enfoques alternativos (rechazados)

### Opción A — Warning SÍNCRONO desde HandleMessage con timeout 5s ✅ RECOMENDADO
- Pros: simple, una sola ruta de código, log inmediato, sin lifecycle adicional.
- Cons: bloquea HandleMessage hasta 5s si Telegram se cuelga. Mitigación: timeout duro + log warn + continue.
- Esfuerzo: Bajo.

### Opción B — Canal separado `warningCh` + worker paralelo
- Pros: no bloquea HandleMessage ni un milisegundo.
- Cons: nuevo worker, nueva goroutine, nuevo lifecycle, más tests, más shutdown handling. **Exceso de ingeniería** para un feedback no-crítico.
- Esfuerzo: Alto. **Rechazado**: el adapter ya tiene retry+token bucket; el send típico es <100ms.

### Opción C — Enviar warning SOLO en pre-mute, NO en pre-ban
- Pros: 1 sola plantilla.
- Cons: UX inconsistente — el usuario recibe feedback del mute pero no del ban, que es más grave.
- Esfuerzo: Bajo pero producto inferior. **Rechazado**.

### Opción D — Template siempre default (no override por grupo)
- Pros: sin schema, sin Textarea en UI.
- Cons: inflexibilidad para distintos tonos de comunidad (algunos grupos son juveniles, otros formales).
- Esfuerzo: Bajo. **Rechazado**: AGENTS §23 menciona "templated message"; override es coherente con la spec.

---

## Risks

| # | Riesgo | Likelihood | Impact | Mitigation |
|---|--------|-----------|--------|------------|
| 1 | `sendMessage` 5s timeout bloquea HandleMessage bajo Telegram lento | Low | Low | Timeout duro vía `context.WithTimeout`; log warn + continue (NO aborta el pipeline). El send típico es <100ms; 5s es 50× margen. |
| 2 | Template custom con JSON raro / encoding raro | Low | Low | `renderTemplate` cae al default ante cualquier error de substitución. Backend NO confía en el template para SQL ni para HTML. |
| 3 | Bot removido entre hit y warning | Low | Low | `permissionOkAdmin` re-check dentro de `SendWarning` (mismo patrón que `AutoActioner.dispatch`). Log warn sin enviar. |
| 4 | Auto-ban de admin del grupo (target es admin) | Medium | Medium | `sendMessage` a un admin del grupo sigue funcionando aunque esté muteado. NO es acción restrictiva. Si el target es admin y se eligió pre-ban, el adapter del BAN falla downstream (no este send). |
| 5 | Warning duplicado (race condition) | Low | Low | `HandleMessage` es síncrono por Update; el bus publica en orden. Una race implicaría dos Updates concurrentes para el mismo user en el mismo grupo, donde cada uno haría su propio upsert atómico del counter (POSTGRES `INSERT ON CONFLICT DO UPDATE` ya cubre). |
| 6 | Frontend Textarea con template > 1000 chars | Low | Low | Validación cliente (maxLength=1000) + validación server en PUT handler (devolver 400). |
| 7 | `AutomuteWarnings == AutobanWarnings` produce 2 warnings en `count=2` | Low | Medium | Documentado en REQ-22 (edge case). Implementación: helper `thresholdKindFor` prioriza pre-ban si `count == AutobanWarnings - 1` (matchea primero). |
| 8 | Permission check reintroduce bug #172 por copy-paste | Low | High | Helper `permissionOkAdmin` reusado del paquete automation (mismo que service.go y autoactioner.go). Comentario explícito en `warning_sender.go`. |
| 9 | Slice excede 400 LOC | **High** | Medium | Forecast ~710 LOC (backend 525 + frontend 90 + docs 30 + 60% buffer). `size:exception` solicitada (precedente 9 PRs consecutivos). |
| 10 | Migración 00008 choca con ALTER pendiente | Low | High | Backend `cfg.RunMigrations=true` aplica 00008 al `docker compose up`. En prod, paso manual documentado en README (AGENTS §13.1). |

---

## Datos del usuario: fallback chain documentada

El admin decide qué identidad del usuario se muestra en el warning. Tres opciones en orden de preferencia:
1. `msg.From.FirstName` (campo más humano, viene en casi todos los mensajes).
2. `msg.From.Username` sin el `@` (si FirstName está vacío, ej. usuarios que no configuran nombre).
3. Literal `"este usuario"` (último fallback; usuarios que ocultan ambos campos).

Esta cadena es **best-effort y nunca falla**: el template se renderiza siempre, con o sin nombre.

---

## Sequencing del send en HandleMessage (propuesto)

```go
// Paso 6: evaluar reglas.
lists := s.preloadLists(ctx, settings)
hit := s.registry.Evaluate(ctx, msg, settings, ws, lists, s.now())
if hit == nil { return nil }

// Paso 7: incrementar counter.
ws.WarningCount++
now := s.now()
ws.LastWarningAt = &now
// ... expires_at ...
if err := s.warnRepo.UpsertWarningState(ctx, ws); err != nil { ... }

// Paso 7.5 (NUEVO slice 2.1): enviar warning al usuario si corresponde.
if kind := thresholdKindFor(settings, ws.WarningCount); kind != "" {
    s.warningSender.SendWarning(ctx, msg.Chat.ID, msg.From.ID, ws.WarningCount, kind)
}

// Log RULE_TRIGGERED (sin cambio).
s.logs.Create(ctx, ruleEntry)

// Paso 8: encolar auto-action si threshold (sin cambio).
if ws.WarningCount >= settings.AutobanWarnings { ... }
else if ws.WarningCount >= settings.AutomuteWarnings { ... }
```

---

## Open Questions Resolved

| Pregunta | Resolución | Origen |
|----------|------------|--------|
| ¿Síncrono o asíncrono? | Síncrono con timeout 5s vía context | Simplicidad > abstracción para este scope; precedent: AutoActioner.dispatch es síncrono |
| ¿Cuándo enviar? | `count == automute_warnings - 1` OR `count == autoban_warnings - 1` | "Hybrid" del orchestrator: warning antes de la acción, no en cada hit |
| ¿Plantilla única o dos? | Dos (pre-mute, pre-ban) | UX consistente con la severidad de la acción |
| ¿Override por grupo? | Sí vía `warn_user_template TEXT NULL` | AGENTS §23: "templated message"; flexibilidad por comunidad |
| ¿Qué pasa si `automute == autoban`? | Un solo warning pre-ban | Documentado en REQ-22; helper prioriza pre-ban |
| ¿Skip silencioso si permission denied? | Sí, log warn sin enviar | Consistente con AutoActioner.dispatch |
| ¿Re-check permission entre hit y send? | Sí | Mismo patrón que autoactioner.go (bot pudo ser removido) |
| ¿Defaults del template? | Hardcoded en automation/templates.go | Cero config para grupos que no customicen; default razonablemente amigable (Rioplatense español como el resto del proyecto) |

---

## Ready for Proposal

**Sí**. Siguiente paso: `sdd-propose` para `moderation-automation-slice2.1` con el scope completo de este exploration.

**What**: Exploración completa del feature "warning al usuario antes de la acción de threshold" sobre `main @ 2df231f` post-slice2 archivado.
**Why**: El usuario pidió este sub-slice como UX feedback en el camino a la acción automática. Sin él, el usuario muteado/banneado no tiene warning previo — aparece como castigo sin contexto.
**Where**: `openspec/changes/moderation-automation/slice2.1/exploration.md` + Engram `sdd/moderation-automation/slice2.1/exploration`.
**Learned**: El counter `warning_count` ya es post-increment en `HandleMessage:162`, así que el check `count == threshold-1` cae naturalmente en el pipeline. `SendMessage` con `keyboard=nil` es la firma correcta (no `(ctx, chatID, text, disableWebPagePreview bool)` como sugirió el orchestrator — la firma real tiene 5 args, ver `backend/internal/telegram/service.go:62`). El bugfix #172 sigue aplicando: `sendMessage` no requiere `can_*` siendo admin; solo `BotStatus == StatusAdministrator`.
