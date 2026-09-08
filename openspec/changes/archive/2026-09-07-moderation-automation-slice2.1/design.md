# Design: `moderation-automation-slice2.1` — Warning visual al usuario en el chat

> **Change**: `moderation-automation-slice2.1` — sub-slice de Fase 3 (AGENTS §23) sobre slice 2 archivado (`main @ 2df231f`).
> **Mode**: hybrid (filesystem + Engram).
> **Predecessor**: exploration `#232`, proposal `#234`, spec `#235` (delta REQ-22..31, 10 ADDED Requirements, ~40 scenarios).
> **Authority**: bugfix `#172` (`permissionOkAdmin` = `g.BotStatus == StatusAdministrator`, NUNCA `can_*`); AGENTS §11, §14, §18.1, §21.1.
> **Branch**: `feat/moderation-automation-slice2.1` base `main @ 2df231f`.
> **Strategy**: **single-pr con `size:exception`** (precedente 9 PRs consecutivos aprobados; forecast confirmado).
> **Size budget**: ~713 LOC (cercano al 400 — `size:exception` solicitada).

---

## Technical Approach

Insertar un **paso 7.5** en `automation.Service.HandleMessage`, entre el `ws.WarningCount++` post-increment (paso 7) y los checks de threshold (paso 8). El paso 7.5 invoca un nuevo `WarningSender.SendWarning(ctx, chatID, userID, count, kind)` cuando `count == AutomuteWarnings-1` (pre-mute) **o** `count == AutobanWarnings-1` (pre-ban), con `WarnUserEnabled=true`. El send es **síncrono con timeout 5s** vía `context.WithTimeout`; cualquier error del sender se loggea como `WARN_USER_SENT` con `ActorID=nil` y el pipeline continúa (no aborta). Las plantillas tienen defaults hardcoded en `automation/templates.go` (español Rioplatense, pre-mute + pre-ban) con override per-grupo vía `warn_user_template TEXT NULL`. `RenderTemplate` substituye `{nombre}` (FirstName → Username sin `@` → `"este usuario"`), `{count}`, `{mute_minutes}` y cae al default ante cualquier error de substitución o template vacío. Frontend: extender `GroupAutomationPage` con una **Sección 5** ("Warning al usuario") entre Umbrales y Listas con un `<Switch>` y un `<Textarea autosize maxLength=1000>` (placeholder = default pre-mute; helper text listando las variables).

## Architecture Decisions

| # | Decisión | Alternativas consideradas | Rationale |
|---|----------|---------------------------|-----------|
| **D1** | Trigger en paso 7.5 (entre upsert y threshold), helper `thresholdKindFor(s, count)` prioriza **pre-ban** si ambos thresholds matchean | Trigger en cada hit (count++; no); trigger solo pre-mute (UX inconsistente); trigger asíncrono con canal+worker (exceso de ingeniería) | Post-increment ya vive en paso 7 (`service.go:162`); priorización pre-ban resuelve el edge case `automute==autoban` (REQ-27); síncrono porque el adapter ya tiene retry+token bucket (AGENTS §18.1) y el send típico es <100ms |
| **D2** | 2 defaults hardcoded en `automation/templates.go` (pre-mute + pre-ban) con override per-grupo vía `warn_user_template TEXT NULL` | 1 sola plantilla (rechazada por UX inconsistente entre mute y ban); siempre custom (sin defaults razonables para grupos que no customicen); template siempre default (sin flexibilidad) | Defaults razonables out-of-the-box + flexibilidad por tono de comunidad (juvenil/formal). AGENTS §23 menciona "templated message" |
| **D3** | `RenderTemplate(kind WarningKind, settings *Settings, msg *telegram.Message, count int16) (string, error)` con fallback al default ante cualquier fallo | Validar regex de placeholders (frágil); retornar error al caller (rompe pipeline) | Best-effort, nunca aborta; backend NO confía en el template para SQL/HTML; log warn + `template_used="default"` cuando cae al default |
| **D4** | `WarningSender` interfaz consumer-side en `automation/warning_sender.go`, mismo patrón que `AutoActioner`. Reusa `permissionOkAdmin` local del paquete automation | Reusar `AutoActioner` directamente (semántica distinta: feedback vs acción); helper nuevo en `moderation` (bugfix `#172` lo prohíbe) | Interfaz dedicada facilita testing (§21.1: fakes); re-check de permission entre hit y send (bot pudo haber sido removido) |
| **D5** | Send **síncrono con `context.WithTimeout(5s)`** dentro de `HandleMessage` | Canal `warningCh` + worker paralelo (nuevo lifecycle, shutdown handling); send sin timeout (puede bloquear el bus indefinidamente) | Adapter ya tiene retry+token bucket; 5s es 50× el send típico; timeout duro garantiza que HandleMessage retorna en ≤5s+ε aunque Telegram se cuelgue |
| **D6** | Frontend: Sección 5 al final de `GroupAutomationPage` (después de Listas, antes de Save). `<Switch>` + `<Textarea autosize minRows={2} maxRows={5} maxLength={1000}>` | Sección al principio (oscurece el resto); modal separado (más fricción); sin override (sin flexibilidad por comunidad) | Consistencia con slice 2 (secciones en Papers separados); Save flow existente cubre automáticamente vía `Object.keys(AUTOMATION_DEFAULTS)` |
| **D7** | Tests §21.1 estricto: fakes hand-rolled para `telegram.Service` y `WarningSender`; sin moq para mantener dependencias mínimas | moq (`github.com/matryer/moq`); llamadas Bot API reales (prohibido §21.1) | Precedente de slice 1+2: fakes hand-rolled en `service_test.go`; moq no se introdujo aún (mínimo costo) |
| **D8** | `ActionWarnUserSent = "WARN_USER_SENT"` con `ActorID=nil` (sistema). Metadata: `{rule_name, warning_count, threshold_kind, template_used}` | `ActorID=*actorID` con admin del panel (incorrecto: el warning lo emite el sistema, no un admin); sin metadata (perdemos trazabilidad de default vs custom) | Patrón slice 1 (`ActionAutomuteUser`/`ActionAutobanUser`); metadata ayuda al admin a auditar qué plantilla se usó |
| **D9** | Migración `00008_add_warning_settings.sql` con `ALTER TABLE` no-destructivo (2 columnas con defaults) | Nueva tabla (innecesario, los settings viven en `group_moderation_settings`); ALTER con CHECK constraints (limita flexibilidad del template) | Compatible con slice 1+2; template libre sin CHECK; el handler valida ≤1000 chars en PUT |
| **D10** | Invariante `permissionOkAdmin` intacto: solo `g.BotStatus == StatusAdministrator`, NUNCA `can_*`. Comentario explícito en `warning_sender.go:8` | Reutilizar `moderation.permissionOk` (bugfix `#172` lo prohíbe — la firma de `groups.Group` no está completa en automation) | Helper local al paquete automation (mismo que `service.go:341` y `autoactioner.go:98`); invariante cross-slice |

## Data Flow

```
Telegram Update ─→ events.Bus ─→ automation.Subscriber.Handle
                                          │
                                          ▼
                              Service.HandleMessage (pipeline 8 pasos)
                                          │
                            ┌─────────────┼─────────────┐
                            │             │             │
                  Paso 2 (settings)   Paso 4 (perm)   Paso 7 (counter++)
                            │             │             │
                            │             │             ▼
                            │             │      UpsertWarningState
                            │             │             │
                            │             │             ▼
                            │             │      PASO 7.5 (NUEVO)
                            │             │             │
                            │             │   thresholdKindFor(s, count)
                            │             │             │
                            │             │             ▼
                            │             │   WarningSender.SendWarning
                            │             │             │
                            │             │      ┌──────┴──────┐
                            │             │      │             │
                            │             │   permissionOk   Render
                            │             │   re-check      Template
                            │             │      │             │
                            │             │      └──────┬──────┘
                            │             │             │
                            │             │             ▼
                            │             │    SendMessage(5-arg, 5s timeout)
                            │             │             │
                            │             │             ▼
                            │             │     Log WARN_USER_SENT
                            │             │   ActorID=nil, metadata {…}
                            │             │             │
                            │             │             ▼
                            │             │      Pipeline continúa
                            │             │             │
                            │             │             ▼
                            │             │      Paso 8 (threshold check)
                            │             │             │
                            │             │             ▼
                            │             │      enqueue AutoAction {mute|ban}
                            │             │             │
                            │             │             ▼
                            │             │      Worker drena autoActionCh
                            │             │             │
                            │             │             ▼
                            │             │      AutoActioner.Execute
                            │             │      (re-check permission + tg.MuteUser/BanUser)
                            └─────────────┴─────────────┘
```

## File Changes

### Backend NEW

| Archivo | LOC est. | Descripción |
|---------|---------:|-------------|
| `backend/migrations/00008_add_warning_settings.sql` | +15 | Goose Up/Down. `ALTER TABLE group_moderation_settings ADD COLUMN warn_user_enabled BOOLEAN NOT NULL DEFAULT true, warn_user_template TEXT NULL`. Reversible, non-destructive con 13 columnas previas. |
| `backend/internal/automation/templates.go` | +60 | `TemplateKind` enum (`TemplatePreMute`, `TemplatePreBan`); `defaultTemplates` map; `MessageContext` struct (`FirstName`, `Username`, `WarningCount`, `MuteMinutes`); `RenderTemplate(kind, settings, msg, count) (string, error)`. Substituciones `{nombre}` (fallback chain FirstName → Username sin `@` → `"este usuario"`), `{count}`, `{mute_minutes}`. Unknown placeholders preservados verbatim. Fallback al default ante template vacío o error de substitución. |
| `backend/internal/automation/templates_test.go` | +80 | ≥3 tests: FirstName disponible; fallback a Username sin `@`; fallback final a `"este usuario"`; `{count}` y `{mute_minutes}` substituidos; template vacío → default pre-mute; placeholder desconocido preservado. |
| `backend/internal/automation/warning_sender.go` | +130 | `WarningKind` const (`pre_mute`/`pre_ban`); `WarningSender` interfaz `{ SendWarning(ctx, groupID, userID, count, kind) error }`; `tgWarningSender` struct que envuelve `TelegramMesseger` (vista mínima de `telegram.Service` solo con `SendMessage`), `LogWriter`, `SettingsReader`. Re-check `permissionOkAdmin` antes del send; `context.WithTimeout(5s)`; `SendMessage(ctx, chatID, text, false, nil)`; log éxito/fallo con `ActionWarnUserSent` + `ActorID=nil` + metadata `{rule_name, warning_count, threshold_kind, template_used}`. `NewWarningSender(tg, logs, settings, logger)`. Comentario explícito invariante bugfix `#172`. |
| `backend/internal/automation/warning_sender_test.go` | +150 | ≥4 tests: happy path (log SUCCESS, tg.SendMessage llamado con args correctos); permission denied → log PERMISSION_DENIED, no panic, no send; timeout 5s → log TELEGRAM_ERROR + return error; template custom → metadata `template_used="custom"`; template vacío → fallback default en metadata. Fakes: `fakeTelegram` (con configurable return), `fakeLogs` (captura entries), `fakeSettingsRepo` (reusa del service_test.go). |

### Backend MOD

| Archivo | LOC est. | Cambios |
|---------|---------:|---------|
| `backend/internal/automation/model.go` | +10 | `Settings` struct: agregar `WarnUserEnabled bool` + `WarnUserTemplate *string` con DB tags. `DefaultSettings(groupID int64)`: `WarnUserEnabled=true, WarnUserTemplate=nil`. |
| `backend/internal/automation/repository.go` | +20 | `GetSettings`: SELECT agrega `warn_user_enabled, warn_user_template` (15 columnas en lugar de 13). `UpsertSettings`: INSERT y DO UPDATE agregan `warn_user_enabled` y `warn_user_template` (15 placeholders en lugar de 13). `scanSettings`: 2 scan targets nuevos. |
| `backend/internal/automation/repository_test.go` | +30 | ≥2 integration tests: (a) `UpsertSettings(ctx, &DefaultSettings{})` → fila persistida tiene `warn_user_enabled=true` y `warn_user_template=NULL`; (b) round-trip del template custom: upsert con `WarnUserTemplate=&"custom {count}"` y `WarnUserEnabled=false`, luego `GetSettings` retorna los mismos valores. Usa `OpenTestDB("automation")` + goose 00008 + `TRUNCATE` antes/después. |
| `backend/internal/automation/service.go` | +25 | `Service` struct: agregar `warningSender WarningSender` + `groups` (ya existe). `NewService` signature: aceptar `warningSender WarningSender` (puede ser `nil`-safe: si nil, paso 7.5 skip). `HandleMessage`: nuevo helper local `thresholdKindFor(s *Settings, count int16) WarningKind`; nueva llamada `s.warningSender.SendWarning(...)` en paso 7.5 con `context.WithTimeout(5s)` si `kind != ""` y `WarnUserEnabled==true`. Log warn si el sender retorna error (no aborta). |
| `backend/internal/automation/service_test.go` | +60 | Actualizar `newSvc` y `newSvcWithLists` para inyectar `fakeWarningSender` (no-op). ≥5 nuevos tests: (a) `count=2 con automute=3 → SendWarning llamado con WarningPreMute, count=2`; (b) `count=0 → NO SendWarning`; (c) `count=3 (umbral) → NO SendWarning`; (d) `WarnUserEnabled=false → NO SendWarning`; (e) `AutomuteWarnings==AutobanWarnings==3, count=2 → exactamente 1 llamada con WarningPreBan`; (f) `permission denied en sender → log warn, pipeline continúa`. |
| `backend/internal/logs/model.go` | +3 | Agregar `ActionWarnUserSent = "WARN_USER_SENT"` al bloque `const()` (sistema, sigue patrón slice 1). |
| `backend/cmd/server/main.go` | +5 | Después de construir `automation.Service(...)`, instanciar `sender := automation.NewWarningSender(bot, automationLogs, settingsRepo, slog.Default())` y pasarlo a `NewService(..., sender)`. Skip si `!cfg.AutomationEnabled`. |
| `backend/internal/api/automation_handlers.go` | +5 | `settingsUpdate` struct: agregar `WarnUserEnabled *bool` + `WarnUserTemplate *string` (nullable para "reset a nil"). `settingsResponse` struct: agregar `WarnUserEnabled bool` + `WarnUserTemplate *string`. PUT handler: merge de los 2 nuevos campos + validación `if req.WarnUserTemplate != nil && len(*req.WarnUserTemplate) > 1000 → 400 VALIDATION_ERROR`. `toSettingsResponse`: agregar los 2 campos al output JSON. |

### Frontend MOD

| Archivo | LOC est. | Cambios |
|---------|---------:|---------|
| `frontend/src/features/automation/types.ts` | +10 | `AutomationSettings`: agregar `warn_user_enabled: boolean` + `warn_user_template: string \| null`. `AutomationSettingsUpdate`: agregar ambos como opcionales. `AUTOMATION_DEFAULTS`: agregar `warn_user_enabled: true` (default-on out-of-the-box). `withDefaults` hereda automáticamente vía spread de `AUTOMATION_DEFAULTS`. |
| `frontend/src/pages/GroupAutomationPage.tsx` | +50 | Nueva Sección 5 "Warning al usuario" (Paper con Border, después de Listas, antes de Save): `<Switch label="Avisar al usuario antes de silenciar/expulsar" checked={draftSettings.warn_user_enabled} onChange={(e) => updateDraft('warn_user_enabled', e.currentTarget.checked)} />`; `<Textarea label="Plantilla del warning (opcional)" placeholder={defaultPreMuteTemplate} value={draftSettings.warn_user_template ?? ""} autosize minRows={2} maxRows={5} maxLength={1000} onChange={(e) => updateDraft('warn_user_template', e.currentTarget.value \|\| null)} />`; `<Text size="xs" c="dimmed">Variables: {nombre}, {count}, {mute_minutes}</Text>`. Sin cambios en Save/Discard flow (cubre automáticamente via `Object.keys(AUTOMATION_DEFAULTS)`). |
| `frontend/src/pages/GroupAutomationPage.test.tsx` | +30 | ≥2 smoke tests: (a) renderiza el Switch de `warn_user_enabled` con label "Avisar al usuario antes de silenciar/expulsar"; (b) el Textarea acepta texto y entra en el diff del Save. Mantener los 8 tests slice 2 existentes verdes. |

### Docs MOD

| Archivo | LOC est. | Cambios |
|---------|---------:|---------|
| `README.md` | +30 | Nueva sección "Warning al usuario" bajo el bloque "Moderación automática" (después del párrafo sobre `/groups/:telegram_id/automation`). Documenta: (1) que el bot avisa al usuario antes de silenciar/expulsar con un mensaje en el chat; (2) las variables disponibles (`{nombre}`, `{count}`, `{mute_minutes}`); (3) el toggle `warn_user_enabled` (default ON); (4) el override por grupo vía `warn_user_template` (max 1000 chars); (5) link a la spec REQ-22..31 en `openspec/specs/moderation-automation/spec.md`. |

**Total**: ~717 LOC (backend ~605 + frontend ~90 + docs ~30).

## Interfaces / Contracts

```go
// backend/internal/automation/templates.go (NEW)

// TemplateKind identifica la plantilla a renderizar.
type TemplateKind string

const (
    TemplatePreMute TemplateKind = "pre_mute"
    TemplatePreBan  TemplateKind = "pre_ban"
)

// MessageContext agrupa los datos del usuario + threshold para
// RenderTemplate. Es value-only (no punteros) — RenderTemplate es
// defensivo ante campos vacíos.
type MessageContext struct {
    FirstName    string // user.From.FirstName
    Username     string // user.From.Username (sin @)
    WarningCount int16  // post-increment
    MuteMinutes  int16  // settings.AutomuteMinutes
}

// defaultTemplates son los textos hardcoded en español Rioplatense.
// Slice 2.1: defaults razonables para grupos que no customicen.
var defaultTemplates = map[TemplateKind]string{
    TemplatePreMute: "⚠️ {nombre}, llevás {count} advertencias. Si seguís, serás silenciado por {mute_minutes} min.",
    TemplatePreBan:  "⚠️ {nombre}, llevás {count} advertencias. Si seguís, serás expulsado del grupo.",
}

// RenderTemplate retorna el texto final a enviar. Si `custom` es no-nil
// y no-vacío, se usa esa plantilla en lugar del default; cualquier
// error de substitución (placeholder desconocido, encoding raro, string
// vacío después de trim) cae al default pre-mute o pre-ban según
// `kind`. Log warn si el custom no se pudo renderizar.
func RenderTemplate(kind TemplateKind, settings *Settings, msg *telegram.Message, count int16, custom *string) string
```

```go
// backend/internal/automation/warning_sender.go (NEW)

// WarningKind identifica el threshold que se aproxima (pre-mute o
// pre-ban). Vive en warning_sender.go (no en model.go) porque es
// dominio de la interfaz sender, no del paquete general.
type WarningKind string

const (
    WarningPreMute WarningKind = "pre_mute"
    WarningPreBan  WarningKind = "pre_ban"
)

// WarningSender es la interfaz consumer-side que Service.HandleMessage
// invoca en paso 7.5. La firma toma IDs (no *Message) para que el
// sender sea trivial de mockear.
type WarningSender interface {
    SendWarning(ctx context.Context, groupID, userID int64, count int16, kind WarningKind) error
}

// TelegramMesseger es la vista minima del adapter que el sender
// necesita (5-arg signature; keyboard=nil para warnings unidireccionales).
type TelegramMesseger interface {
    SendMessage(ctx context.Context, chatID int64, text string, disableWebPagePreview bool, keyboard *telegram.InlineKeyboardMarkup) (int64, error)
}

// tgWarningSender implementa WarningSender sobre el adapter de Telegram.
type tgWarningSender struct {
    tg       TelegramMesseger
    logs     LogWriter
    settings SettingsReader
    groups   GroupReader
    logger   *slog.Logger
}

// NewWarningSender construye el wrapper concreto. logger puede ser nil
// (usa slog.Default()). Reusa permissionOkAdmin del paquete automation
// (bugfix #172 invariante: BotStatus == StatusAdministrator, NUNCA can_*).
func NewWarningSender(tg TelegramMesseger, logs LogWriter, settings SettingsReader, groups GroupReader, logger *slog.Logger) WarningSender
```

```go
// backend/internal/automation/service.go (MOD)

// Service struct: nuevo campo
type Service struct {
    // ... existing ...
    warningSender WarningSender // puede ser nil (no-op en tests sin slice 2.1)
}

// thresholdKindFor resuelve el kind del warning. Prioriza pre-ban si
// ambos thresholds matchean (edge case automute==autoban, REQ-27).
func thresholdKindFor(s *Settings, count int16) WarningKind {
    if s.AutobanWarnings > 0 && count == s.AutobanWarnings-1 {
        return WarningPreBan
    }
    if s.AutomuteWarnings > 0 && count == s.AutomuteWarnings-1 {
        return WarningPreMute
    }
    return ""
}

// En HandleMessage paso 7.5 (entre upsert y threshold check):
if s.warningSender != nil && settings.WarnUserEnabled {
    if kind := thresholdKindFor(settings, ws.WarningCount); kind != "" {
        ctxSend, cancel := context.WithTimeout(ctx, 5*time.Second)
        defer cancel()
        if err := s.warningSender.SendWarning(ctxSend, msg.Chat.ID, msg.From.ID, ws.WarningCount, kind); err != nil {
            s.logger.Warn("automation: warning send failed",
                "kind", string(kind),
                "group_id", msg.Chat.ID,
                "user_id", msg.From.ID,
                "count", ws.WarningCount,
                "error", err)
            // Pipeline continúa — paso 8 sigue normalmente.
        }
    }
}
```

```typescript
// frontend/src/features/automation/types.ts (MOD)

export interface AutomationSettings {
  // ... existing 13 campos ...
  warn_user_enabled: boolean
  warn_user_template: string | null
}

export interface AutomationSettingsUpdate {
  // ... existing 13 campos opcionales ...
  warn_user_enabled?: boolean
  warn_user_template?: string | null  // null = reset a default
}

export const AUTOMATION_DEFAULTS = {
  // ... existing 13 defaults ...
  warn_user_enabled: true,  // default-on (out-of-the-box)
  // warn_user_template se omite del default (queda null)
}
```

```typescript
// frontend/src/pages/GroupAutomationPage.tsx (MOD)
// Nueva Sección 5 entre Listas y Save:
<Paper withBorder p="lg" radius="md">
  <Stack gap="sm">
    <Title order={4}>Warning al usuario</Title>
    <Switch
      label="Avisar al usuario antes de silenciar/expulsar"
      checked={draftSettings.warn_user_enabled}
      onChange={(e) => updateDraft('warn_user_enabled', e.currentTarget.checked)}
    />
    <Textarea
      label="Plantilla del warning (opcional)"
      description="Variables disponibles: {nombre}, {count}, {mute_minutes}"
      placeholder={defaultPreMuteTemplate}
      value={draftSettings.warn_user_template ?? ''}
      onChange={(e) => updateDraft('warn_user_template', e.currentTarget.value || null)}
      autosize
      minRows={2}
      maxRows={5}
      maxLength={1000}
    />
  </Stack>
</Paper>
```

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| **Backend unit — templates** | `RenderTemplate` con cada combinación de `MessageContext` + `custom` + `kind`. Fallback chain de `{nombre}`. Unknown placeholders preservados. Empty custom → default. | `automation/templates_test.go` (NEW), ≥3 casos. Sin DB, sin red. |
| **Backend unit — service** | Trigger condition paso 7.5: `count==automute-1` dispara pre-mute; `count==autoban-1` dispara pre-ban; `count==threshold` skip; `count==0` skip; `WarnUserEnabled=false` skip; `automute==autoban` produce UN warning pre-ban; sender error → log warn + pipeline continúa. | `automation/service_test.go` (MOD), ≥5 casos nuevos. Fakes: `fakeWarningSender`, `fakeSettingsRepo`, `fakeLogs`, `fakeGroups` (reusar de service_test.go actual). |
| **Backend unit — warning_sender** | Happy path (SendMessage llamado, log SUCCESS); `ErrPermissionDenied` (log PERMISSION_DENIED, no send, no panic); timeout 5s (log TELEGRAM_ERROR, return error); custom template metadata `template_used="custom"`; empty custom metadata `template_used="default"`. | `automation/warning_sender_test.go` (NEW), ≥4 casos. Fakes: `fakeTelegram` con configurable return + delay, `fakeLogs`, `fakeSettingsRepo`, `fakeGroups`. |
| **Backend integration — repository** | Round-trip de los 2 nuevos campos: defaults `warn_user_enabled=true, warn_user_template=NULL` al insertar; custom no-NULL persiste. | `automation/repository_test.go` (MOD), ≥2 casos. `OpenTestDB("automation")` + goose 00008 + `TRUNCATE` antes/después. |
| **Backend handler** | PUT settings acepta body con `warn_user_enabled=false` + `warn_user_template="custom {count}"` no-vacío ≤1000 chars (persiste, devuelve 200 con fila completa). PUT con template >1000 chars → 400 `VALIDATION_ERROR`. PUT con `warn_user_template=null` (omitido) → conserva valor previo. | `automation_handlers_test.go` (MOD), ≥1 caso nuevo por path. |
| **Frontend** | Smoke test: el Switch `warn_user_enabled` renderiza con label correcto; el Textarea acepta texto y entra en el diff del Save (PUT enviado). | `GroupAutomationPage.test.tsx` (MOD), ≥2 casos. `mockFetchRoutes` con `/automation/settings`. |
| **Frontend non-regression** | Los 8 tests slice 2 existentes siguen verdes (switch principal, toggles, umbrales, banned_words, link_allowlist, Save/Discard, error states). | Sin cambios en los tests slice 2. |
| **End-to-end manual** | Levantar `docker compose up`, abrir `/groups/:id/automation`, tipear template custom, guardar; en otro cliente Telegram, simular un flood con un user de prueba; verificar que el bot envía el warning con el template custom en `count==automute-1`. | Manual QA post-merge; sin automation E2E (el adapter no se mockea en CI). |

## Migration / Rollout

**Migración 00008 (`backend/migrations/00008_add_warning_settings.sql`)**:
- **Up**: `ALTER TABLE group_moderation_settings ADD COLUMN warn_user_enabled BOOLEAN NOT NULL DEFAULT true, ADD COLUMN warn_user_template TEXT NULL`.
- **Down**: `ALTER TABLE group_moderation_settings DROP COLUMN warn_user_template, DROP COLUMN warn_user_enabled`.
- En **dev**: `cfg.RunMigrations=true` aplica 00008 al `docker compose up` (automático; AGENTS §13.1).
- En **prod**: paso manual y documentado en README (AGENTS §13.1).
- Filas existentes quedan con `warn_user_enabled=true` y `warn_user_template=NULL` → todos los grupos existentes arrancan con el feature activo out-of-the-box (out-of-the-box "on" es la decisión D2; el admin puede apagarlo desde el panel).

**Rollout feature flag**: no aplica — el toggle es per-grupo (`warn_user_enabled`). No hay flag global; el feature se enciende/apaga por grupo desde la UI.

**Rollback**: `git revert` del merge + `goose down` (DROP COLUMN). El frontend deja de enviar los 2 campos; el backend los persiste como opcionales. `Service.HandleMessage` pasa `kind=""` y skip el sender.

## Open Questions

Ninguna — la exploración (`#232`) y la propuesta (`#234`) resolvieron todo: trigger síncrono con timeout 5s, dos plantillas con override per-grupo, edge case `automute==autoban` prioriza pre-ban, re-check de permission, fallback al default ante error de substitución. Las decisiones D1-D10 son operativas dentro de este slice.

| Pregunta | Resolución | Origen |
|----------|------------|--------|
| ¿Síncrono o asíncrono? | Síncrono con timeout 5s vía `context.WithTimeout` | Exploración D5; precedent: `AutoActioner.Execute` es síncrono |
| ¿Cuándo enviar? | `count == automute_warnings - 1` OR `count == autoban_warnings - 1` | Exploración D1; "hybrid" del orchestrator |
| ¿Plantilla única o dos? | Dos (pre-mute, pre-ban) | Exploración D2; UX consistente con severidad |
| ¿Override por grupo? | Sí vía `warn_user_template TEXT NULL` | Exploración D2; AGENTS §23 "templated message" |
| ¿Qué pasa si `automute == autoban`? | UN solo warning pre-ban | Spec REQ-27; helper `thresholdKindFor` prioriza pre-ban |
| ¿Skip silencioso si permission denied? | Sí, log warn sin enviar | Spec REQ-28; consistente con `AutoActioner.Execute` |
| ¿Re-check permission entre hit y send? | Sí | Spec REQ-28; mismo patrón que `AutoActioner.dispatch` |
| ¿Defaults del template? | Hardcoded en `automation/templates.go` (Rioplatense) | Exploración D2; cero config para grupos que no customicen |
| ¿Max length del template? | 1000 chars (server 400, client `maxLength`) | Spec REQ-30 handler test; previene payloads absurdos |
| ¿Tipo de `warn_user_template` en JSON? | `*string` (nullable; nil = reset) | Spec REQ-29; frontend usa `string \| null` |

## Risks

| # | Riesgo | Likelihood | Impact | Mitigation |
|---|--------|-----------|--------|------------|
| 1 | `SendMessage` 5s timeout bloquea HandleMessage bajo Telegram lento | Low | Low | Timeout duro vía `context.WithTimeout`; log warn + continue (NO aborta el pipeline). Send típico <100ms; 5s es 50× margen. |
| 2 | Template custom con placeholder desconocido / encoding raro | Low | Low | `RenderTemplate` cae al default ante cualquier error; backend NO confía en el template para SQL/HTML. |
| 3 | Bot removido entre hit y sendMessage | Low | Low | `permissionOkAdmin` re-check dentro de `SendWarning` (mismo patrón que `AutoActioner.dispatch`). Log warn sin enviar. |
| 4 | `AutomuteWarnings == AutobanWarnings` produce 2 warnings en `count=2` | Low | Medium | Documentado en spec REQ-27; helper `thresholdKindFor` prioriza pre-ban si `count == AutobanWarnings-1`. |
| 5 | Warning duplicado por race condition | Low | Low | `HandleMessage` es síncrono por Update; UPSERT atómico de `user_warning_state` cubre concurrencia. |
| 6 | Frontend Textarea con template > 1000 chars | Low | Low | `maxLength={1000}` cliente + validación server en PUT (400 `VALIDATION_ERROR`). |
| 7 | Permission check reintroduce bug `#172` | Low | High | Helper `permissionOkAdmin` reusado del paquete automation (mismo que `service.go:341` y `autoactioner.go:98`). Comentario explícito en `warning_sender.go:8`. |
| 8 | Slice excede 400 LOC | **High** | Medium | Forecast ~717 LOC. `size:exception` solicitada (precedente 9 PRs consecutivos). `sdd-tasks` re-confirmará. |
| 9 | Migración 00008 choca con ALTER pendiente | Low | High | Backend `cfg.RunMigrations=true` aplica 00008 al `docker compose up`. En prod, paso manual documentado en README (AGENTS §13.1). |
| 10 | `Settings` struct crece y rompe `Scan()` existente | Low | High | `repository_test.go` extiende el integration test del round-trip; `scanSettings` se actualiza explícitamente con los 2 nuevos targets en el mismo orden que SELECT. |

## Spec Coverage

| Aspecto | REQ | Status |
|---------|-----|--------|
| Schema migration Up/Down | REQ-22 | ✅ (migración 00008) |
| Defaults al auto-crear settings | REQ-23 | ✅ (`DefaultSettings` + UPSERT) |
| 2 templates hardcoded | REQ-24 | ✅ (`templates.go` defaults) |
| Override per-group | REQ-24 | ✅ (`warn_user_template TEXT NULL`) |
| Fallback chain `{nombre}` | REQ-25 | ✅ (`RenderTemplate`) |
| Substitutions `{count}`, `{mute_minutes}` | REQ-25 | ✅ |
| Unknown placeholder verbatim | REQ-25 | ✅ |
| Trigger condition paso 7.5 | REQ-26 | ✅ (`thresholdKindFor` + `SendWarning`) |
| Helper `thresholdKindFor` | REQ-26, REQ-27 | ✅ |
| Edge case `automute==autoban` | REQ-27 | ✅ (prioridad pre-ban) |
| `WarningSender` interface + impl | REQ-28 | ✅ (`warning_sender.go`) |
| Re-check permission | REQ-28, REQ-31 | ✅ |
| Timeout 5s `context` | REQ-28 | ✅ |
| Log `ActionWarnUserSent` con `ActorID=nil` | REQ-28 | ✅ (`logs/model.go` + sender) |
| Frontend Sección 5 | REQ-29 | ✅ (`GroupAutomationPage.tsx`) |
| Tip con variables | REQ-29 | ✅ (`<Text size="xs" c="dimmed">`) |
| `AUTOMATION_DEFAULTS` extendido | REQ-23, REQ-29 | ✅ (`features/automation/types.ts`) |
| Tests §21.1 estricto | REQ-30 | ✅ (fakes hand-rolled, sin Bot API real) |
| Test coverage (3+5+4+2+handler+2+non-regression) | REQ-30 | ✅ |
| Non-regression `moderation/` | REQ-31 | ✅ (no se toca) |
| Non-regression frontend pages | REQ-31 | ✅ (solo `GroupAutomationPage.tsx` cambia) |
| `permissionOkAdmin` invariante | REQ-31 | ✅ (`grep -rn 'can_' backend/internal/automation/` sigue en 0) |
| §21.1: cero Bot API real en tests | REQ-31 | ✅ (interface mockeada) |
| §25 sin secretos | REQ-31 | ✅ |

**Total**: 10 REQ + ~40 scenarios deltas cubiertos. Diseño listo para `sdd-tasks`.

---

## What / Why / Where

**What**: Diseño técnico completo del feature "warning visual al usuario en el chat" sobre `automation.Service.HandleMessage` paso 7.5, con 2 columnas nuevas en `group_moderation_settings`, 2 plantillas default (pre-mute + pre-ban) con override per-grupo, interfaz `WarningSender` con timeout 5s, y Sección 5 en el frontend.
**Why**: El orchestrator pidió el diseño técnico del slice 2.1 sobre la propuesta y spec aprobadas. El usuario del bot recibe feedback antes de ser muteado/banneado (UX educativa; AGENTS §23 "warnings").
**Where**: `openspec/changes/moderation-automation/slice2.1/design.md` + Engram `sdd/moderation-automation/slice2.1/design`.
**Learned**: El counter `warning_count` ya es post-increment en `HandleMessage:162` (verificado leyendo `service.go`), así que el check `count == threshold-1` cae naturalmente en el pipeline. `SendMessage` con `keyboard=nil` es la firma correcta de 5 args (no 4) — `telegram.Service.SendMessage(ctx, chatID, text, disableWebPagePreview, keyboard)` en `telegram/service.go:62`. El bugfix `#172` (`permissionOkAdmin` invariante) sigue aplicando: `sendMessage` no requiere `can_*` siendo admin.
