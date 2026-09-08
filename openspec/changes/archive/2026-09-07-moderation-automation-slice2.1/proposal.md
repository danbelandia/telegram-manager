# Proposal: Moderation Automation — Slice 2.1 (Warning to user before threshold action)

> **Change**: `moderation-automation-slice2.1` — sub-slice de Fase 3 (AGENTS §23) sobre slice 2 archivado.
> **Mode**: hybrid (filesystem + Engram).
> **Predecessor**: exploration `#232` (`sdd/moderation-automation/slice2.1/exploration`); slice 2 archivado (`main @ 2df231f`, obs `#231`).
> **Authority**: bugfix `#172` (`permissionOkAdmin` usa `BotStatus == StatusAdministrator`, NUNCA `can_*`); AGENTS §11 (no Redis), §14 (modular backend), §18.1 (rate limit Telegram), §21.1 (mock TelegramService).
> **Strategy**: single-pr con `size:exception` (precedente: 9 PRs consecutivos — re-confirmar en `tasks`).
> **Branch base**: `main @ 2df231f` (slice 2 ya mergeado).
> **Forecast**: ~713 LOC (cercano al budget 400; precedent de 7 aprobaciones consecutivas; `tasks` re-confirmará).

---

## Intent

Hoy `automation.Service.HandleMessage` ejecuta el pipeline de 8 pasos (slice 1): evalúa reglas, incrementa `warning_count` y — si llega al threshold — encola mute/ban silencioso en `autoActionCh`. **El usuario no recibe feedback en el chat antes de la acción**: aparece muteado/banneado sin contexto, como castigo sin aviso. Esto rompe la UX educativa de la moderación automática (AGENTS §23: "warnings").

Slice 2.1 agrega un paso 7.5 entre el upsert del counter (paso 7) y los checks de threshold (paso 8): un `sendMessage` al usuario en el chat cuando `warning_count` alcanza `automute_warnings - 1` (pre-mute) o `autoban_warnings - 1` (pre-ban). Cada grupo puede activar/desactivar el aviso y customizar el texto vía una plantilla con substituciones (`{nombre}`, `{count}`, `{mute_minutes}`), con defaults razonables en español Rioplatense.

---

## Scope

### In Scope

- **Migración `00008_add_warning_settings.sql`**: `ALTER TABLE group_moderation_settings ADD COLUMN warn_user_enabled BOOLEAN NOT NULL DEFAULT true, warn_user_template TEXT NULL`. Goose Up/Down reversibles.
- **`automation/templates.go` (NEW)**: función `RenderTemplate(kind WarningKind, custom *string, msg *telegram.Message, settings *Settings, count int16) string` con 2 defaults hardcoded (pre-mute, pre-ban), substituciones `{nombre}` (FirstName → Username sin `@` → literal `"este usuario"`), `{count}`, `{mute_minutes}`. Fallback al default ante cualquier error de substitución o template vacío (log warn, no panic).
- **`automation/warning_sender.go` (NEW)**: interfaz `WarningSender { SendWarning(ctx, groupID, userID, count, kind) error }` + `WarningKind` enum (`pre_mute`, `pre_ban`). Implementación `tgWarningSender` envuelve `telegram.Service` (vista `TelegramMesseger`), `logs.LogWriter`, `SettingsReader`. Re-check de `permissionOkAdmin` antes del send. Timeout 5s vía `context.WithTimeout`. Log éxito/fallo con `ActionWarnUserSent = "WARN_USER_SENT"`, `ActorID=nil`.
- **`automation/service.go` MOD**: inyectar `warningSender` en `NewService`; nuevo helper `thresholdKindFor(s, count)` que prioriza pre-ban sobre pre-mute si `automute == autoban`; llamada a `SendWarning` en paso 7.5 de `HandleMessage`.
- **`automation/model.go` + `repository.go` MOD**: agregar `WarnUserEnabled bool` + `WarnUserTemplate *string` a `Settings`. `DefaultSettings` con `WarnUserEnabled=true, WarnUserTemplate=nil`. Repository: 2 columnas extra en SELECT + placeholders en INSERT/UPDATE.
- **`backend/internal/logs/model.go` MOD**: `ActionWarnUserSent = "WARN_USER_SENT"` (sistema, `ActorID=nil`).
- **`backend/internal/api/automation_handlers.go` MOD**: validar `warn_user_enabled` y `warn_user_template` en PUT (template ≤ 1000 chars si no nil; devolver 400).
- **`backend/cmd/server/main.go` MOD**: construir `sender := automation.NewWarningSender(...)` y pasarlo a `NewService`.
- **Frontend (`features/automation/types.ts` + `pages/GroupAutomationPage.tsx` MOD)**: extender `AutomationSettings` y `AutomationSettingsUpdate` con `warn_user_enabled: boolean` + `warn_user_template: string | null`. `AUTOMATION_DEFAULTS.warn_user_enabled = true`. Sección 5 nueva en `GroupAutomationPage` ("Warning al usuario") con `<Switch>` + `<Textarea autosize>` (placeholder = default pre-mute template, helper text listando `{nombre}`/`{count}`/`{mute_minutes}`).
- **Tests §21.1 estricto**:
  - `automation/templates_test.go` (NEW): 3+ casos de substitución + fallback al default.
  - `automation/warning_sender_test.go` (NEW): 4+ casos (happy path, permission denied, template custom, template vacío).
  - `automation/service_test.go` MOD: 5+ casos del trigger de warning (count=2 pre-mute, count=0 skip, count=threshold skip, toggle off, automute==autoban single warning).
  - `automation/handlers_test.go` MOD: extender PUT con los 2 nuevos campos.
  - `GroupAutomationPage.test.tsx` MOD: 2 smoke tests (render del switch + textarea entra en diff del Save).
- **Docs**: `README.md` sección "Warning al usuario" bajo "Moderación automática".

### Out of Scope

- **Dashboard de advertencias** (`GroupWarningsPage` + stats) → slice 3.
- **Whitelist de admins exentos** (target admin no se silencia) → follow-up (afecta el adapter del mute/ban, no el send del warning).
- **Cache de templates** (LRU) → solo si métricas lo justifican.
- **EditedMessage / trigger de warning al editar** → follow-up.

---

## Capabilities

### Modified Capabilities

- **`moderation-automation`** (delta spec): nuevo REQ-22 (warning al usuario pre-acción), sub-REQ-22.1 (trigger condition), sub-REQ-22.2 (template rendering con fallback), sub-REQ-22.3 (edge case automute==autoban). Spec canónico `openspec/specs/moderation-automation/spec.md` se AMPLÍA (técnica APPEND del archive phase).

> **Decisión de spec**: **AMEND** vía delta en `openspec/changes/moderation-automation/slice2.1/specs/moderation-automation/spec.md`. Aplica `ADDED Requirements` por cada nueva REQ.

---

## Approach

| # | Decisión | Cómo |
|---|----------|------|
| **D1** | **Trigger**: warning en `HandleMessage` paso 7.5, **después** del `ws.WarningCount++` (paso 7) y **antes** de los checks de threshold (paso 8). Condición: `count > 0` ∧ `count == AutomuteWarnings-1` OR `count == AutobanWarnings-1` ∧ `WarnUserEnabled`. | Helper `thresholdKindFor(s, count)` prioriza pre-ban. |
| **D2** | **Plantillas**: 2 defaults hardcoded en `automation/templates.go` (pre-mute + pre-ban). Override per-grupo vía `warn_user_template TEXT NULL`. Substituciones `{nombre}` (fallback chain FirstName → Username → "este usuario"), `{count}` (post-increment), `{mute_minutes}` (`AutomuteMinutes`). | `renderTemplate` cae al default ante placeholder desconocido o template vacío. Backend NO confía en el template para SQL ni HTML. |
| **D3** | **Schema**: `ALTER TABLE group_moderation_settings` agrega 2 columnas. Sin CHECK, sin nueva tabla. Compatible con slice 1+2. | Migración `00008`. |
| **D4** | **Interfaz `WarningSender`** consumer-side en `automation/warning_sender.go` (patrón `AutoActioner`). Reusa `permissionOkAdmin` local del paquete automation (bugfix #172). | NO reusa `AutoActioner` (semántica distinta: feedback, no acción). |
| **D5** | **Síncrono con timeout 5s** vía `context.WithTimeout`. Si falla o tarda > 5s, log warn + continue (NO aborta el pipeline). | Send típico <100ms; 5s es 50× margen. Adapter ya tiene rate-limit + retry (429 `retry_after`). |
| **D6** | **Frontend**: sección 5 al final de `GroupAutomationPage` (entre Umbrales y Listas). `<Switch>` + `<Textarea autosize>` con helper text listando variables. | Save flow existente cubre los 2 nuevos campos automáticamente (diff sobre `Object.keys(AUTOMATION_DEFAULTS)`). |
| **D7** | **Tests**: §21.1 estricto — cero llamadas Bot API reales. Fakes hand-rolled (publications-slice3 precedent). | `fakeSettingsRepo`, `fakeWarningSender`, `fakeLogsWriter`. |
| **D8** | **Logs**: nueva constante `ActionWarnUserSent = "WARN_USER_SENT"`, `ActorID=nil`. Metadata `{rule_name, warning_count, threshold_kind, template_used: "default"\|"custom"}`. | Patrón slice 1 (`ActionAutomuteUser`/`ActionAutobanUser`). |
| **D9** | **Frontend non-regression**: las 8 secciones existentes de `GroupAutomationPage` intactas. Single Save con `Promise.all` preservado. | `GroupDetailPage` intacto. |
| **D10** | **Permission check invariante**: `permissionOkAdmin` reusado del paquete automation (mismo que `service.go` y `autoactioner.go`). Comentario explícito en `warning_sender.go:8`. | NO reutiliza `moderation.permissionOk` (bugfix #172). |

---

## Schema (migración 00008)

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

---

## Affected Areas

| Area | Impact | LOC est. |
|------|--------|---------:|
| `backend/migrations/00008_add_warning_settings.sql` | **NEW** | +15 |
| `backend/internal/automation/templates.go` (+ test) | **NEW** | +140 |
| `backend/internal/automation/warning_sender.go` (+ test) | **NEW** | +280 |
| `backend/internal/automation/model.go` | MOD (`Settings` +2 campos, `DefaultSettings`) | +10 |
| `backend/internal/automation/repository.go` | MOD (2 cols SELECT + INSERT/UPDATE) | +20 |
| `backend/internal/automation/repository_test.go` | MOD (+2 integration cases) | +30 |
| `backend/internal/automation/service.go` | MOD (`NewService` signature, helper, paso 7.5) | +25 |
| `backend/internal/automation/service_test.go` | MOD (+5 cases) | +60 |
| `backend/internal/api/automation_handlers.go` | MOD (PUT validation) | +5 |
| `backend/internal/logs/model.go` | MOD (`ActionWarnUserSent`) | +3 |
| `backend/cmd/server/main.go` | MOD (`NewWarningSender` + wiring) | +5 |
| `frontend/src/features/automation/types.ts` | MOD (2 campos en 3 tipos) | +10 |
| `frontend/src/pages/GroupAutomationPage.tsx` | MOD (Sección 5: Switch + Textarea) | +50 |
| `frontend/src/pages/GroupAutomationPage.test.tsx` | MOD (+2 smoke tests) | +30 |
| `README.md` | MOD (sección warning al usuario) | +30 |

**Total**: ~713 LOC. Cercano al budget 400 — `size:exception` solicitada (precedente 9 PRs consecutivos). Forecast confirmado en `tasks`.

---

## Risks

| Risk | Likelihood | Mitigation |
|------|-----------|------------|
| `sendMessage` timeout 5s bloquea HandleMessage bajo Telegram lento | Low | Timeout duro vía `context.WithTimeout`; log warn + continue (NO aborta). Send típico <100ms; 5s es 50× margen. |
| Template custom con placeholder desconocido / encoding raro | Low | `renderTemplate` cae al default ante cualquier error; backend NO confía en el template para SQL/HTML. |
| Bot removido entre hit y sendMessage | Low | `permissionOkAdmin` re-check dentro de `SendWarning` (mismo patrón que `AutoActioner.dispatch`). Log warn sin enviar. |
| `AutomuteWarnings == AutobanWarnings` produce 2 warnings en mismo `count` | Low | Documentado en REQ-22.3; helper `thresholdKindFor` prioriza pre-ban si `count == AutobanWarnings-1`. |
| Warning duplicado por race condition | Low | `HandleMessage` síncrono por Update; UPSERT atómico de `user_warning_state` cubre concurrencia. |
| Frontend Textarea con template > 1000 chars | Low | `maxLength={1000}` cliente + validación server en PUT (400). |
| Permission check reintroduce bug #172 | Low | Helper `permissionOkAdmin` reusado del paquete automation; comentario explícito en `warning_sender.go:8`. |
| Slice excede 400 LOC | **High** | Forecast ~713 LOC. `size:exception` solicitada (precedente 9 consecutivos). `tasks` re-confirmará. |
| Migración 00008 choca con ALTER pendiente | Low | `cfg.RunMigrations=true` aplica 00008 al `docker compose up`; prod = paso manual documentado en README (§13.1). |

---

## Rollback

`git revert` del merge. Migración 00008 Down elimina las 2 columnas. `cmd/server/main.go` deja de inyectar `warningSender` (constructor `nil`-safe o se omite el wiring); `HandleMessage` skip el paso 7.5. Audit logs preservados (`WARN_USER_SENT` históricos quedan en `logs`). Frontend: `warn_user_enabled` y `warn_user_template` se persisten pero la UI los ignora (`Settings` shape los acepta como opcionales); sin UI los campos quedan inertes en la base.

---

## Dependencies

- Slice 2 archivado (`main @ 2df231f`, obs `#231`) + spec canónico `openspec/specs/moderation-automation/spec.md` (21 REQs).
- `automation.Service.HandleMessage` paso 7 ya incrementa `warning_count` post-increment (`backend/internal/automation/service.go:162`).
- `telegram.Service.SendMessage(ctx, chatID, text, disableWebPagePreview, keyboard)` (`backend/internal/telegram/service.go:62`) — pasamos `keyboard=nil` (warning unidireccional, sin botones inline).
- `events.Bus` entrega `*telegram.Update.Message` con `From{ID, FirstName, Username}` completo.
- Bugfix `#172` (`permissionOkAdmin` invariante).
- `AutoActioner` como patrón de referencia para `WarningSender` (mismo paquete, misma estructura).

---

## Delivery

**Single-pr, size:exception solicitada** (precedente: 9 PRs consecutivos aprobados). Branch: `feat/moderation-automation-slice2.1` base `main @ 2df231f`. Tasks phase re-confirmará el forecast (~713 LOC cercano a 400) o recomendará chained (`backend-templates-sender` / `service-handler` / `frontend-section`).

---

## Success Criteria

- [ ] `00008_add_warning_settings.sql` Up/Down aplica limpia; 2 columnas con defaults correctos
- [ ] `Settings.WarnUserEnabled` default `true`, `Settings.WarnUserTemplate` default `nil`
- [ ] `renderTemplate` substituye `{nombre}` (FirstName → Username sin `@` → "este usuario"), `{count}`, `{mute_minutes}` correctamente
- [ ] Template vacío / placeholder desconocido → fallback al default pre-mute o pre-ban según threshold
- [ ] `thresholdKindFor` prioriza pre-ban cuando `AutomuteWarnings == AutobanWarnings`
- [ ] `HandleMessage` paso 7.5 llama `SendWarning` solo si `count == automute_warnings - 1` OR `count == autoban_warnings - 1` AND `WarnUserEnabled == true`
- [ ] `SendWarning` timeout 5s vía `context.WithTimeout`; log warn + continue si falla
- [ ] `permissionOkAdmin` re-check dentro de `SendWarning`; log warn sin enviar si bot removido
- [ ] Log `WARN_USER_SENT` con `ActorID=nil` y metadata `{rule_name, warning_count, threshold_kind, template_used}`
- [ ] Frontend Sección 5 en `GroupAutomationPage` con `<Switch>` + `<Textarea>` + helper text listando variables
- [ ] `AUTOMATION_DEFAULTS.warn_user_enabled = true`; `withDefaults` incluye los 2 nuevos campos
- [ ] Save flow cubre los 2 nuevos campos vía diff automático (sin cambios en el handler del Save)
- [ ] Tests backend: 3+ templates, 5+ service, 4+ warning_sender, 2+ repository integration, +PUT handler
- [ ] Tests frontend: 2 smoke tests nuevos; los 8 existentes (slice 2) siguen verdes
- [ ] `permissionOkAdmin` invariante intacto (`grep can_* backend/internal/automation/` = 0 matches)
- [ ] §21.1: cero llamadas Bot API reales en tests (audit en `verify`)
- [ ] `go test ./...`, `npm test`, `go vet`, `gofmt -l .`, `npm run build` en verde
- [ ] §25 sin secretos en repo; sin cambios en `backend/internal/moderation/` (acciones manuales intactas)

---

## Open Questions

Ninguna — la exploración (`#232`) resolvió todo: trigger síncrono con timeout 5s, dos plantillas con override per-grupo, edge case automute==autoban prioriza pre-ban, re-check de permission, fallback al default ante error de substitución. Las decisiones D1-D10 son operativas dentro de este slice y se confirman en `design`.
