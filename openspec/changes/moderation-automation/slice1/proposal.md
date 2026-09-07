# Proposal: Moderation Automation — Slice 1 (Foundation: Settings + Flood Rule + Auto-Action Worker)

> **Change**: `moderation-automation-slice1` — Slice 1/3 de Fase 3 (AGENTS §23).
> **Mode**: hybrid (filesystem + Engram).
> **Predecessor**: exploration `#215` (`sdd/moderation-automation/exploration`).
> **Authority**: bugfix `#172` (`permissionOkAdmin` usa `BotStatus == StatusAdministrator`, NUNCA `can_*`).
> **Strategy**: single-pr con `size:exception` (precedente: 6 PRs consecutivos publications-slice1/2/3; re-confirmar en `tasks`).

---

## Intent

Establecer la **base backend** de Fase 3 (AGENTS §23) sobre `main @ 24e4c8c` post-publications-slice3: tablas de settings + warning state, registry de reglas con la primera regla (`Flood`), worker de auto-actions que reusa `tg.MuteUser`/`tg.BanUser` (rate-limit heredado del adapter), y audit logs con `ActorID=nil`. **Verificable end-to-end** vía `psql` + `logs` sin tocar el frontend.

---

## Scope

### In Scope

- **Migración `00006_create_moderation_automation.sql`** con Up/Down:
  - `group_moderation_settings(group_id PK FK groups, enabled, anti_spam_enabled, anti_link_enabled, banned_words_enabled, flood_enabled, flood_messages SMALLINT, flood_seconds SMALLINT, warning_limit SMALLINT, automute_warnings SMALLINT, automute_minutes SMALLINT, autoban_warnings SMALLINT, warning_expire_days SMALLINT, updated_at)`.
  - `user_warning_state(group_id, user_id, warning_count SMALLINT, last_warning_at, last_action_at, expires_at)`. PK `(group_id, user_id)`.
- **Módulo backend** `backend/internal/automation/`:
  - `model.go` (Settings, WarningState, RuleHit, Action, errores `ErrSettingsNotFound`, `ErrInvalidThreshold`).
  - `repository.go` (`GetSettings`/`UpsertSettings`, `GetOrCreateWarningState`/`IncrementWarning`/`ListWarningStates`/`ResetExpired`).
  - `rules.go` (interfaz `Rule`, `Registry` con short-circuit, primera regla `FloodRule`).
  - `service.go` (`ModerationService.HandleMessage(ctx, msg)` — pipeline: load settings → check enabled+flood_enabled → ejecutar reglas → bumpear warning → dispatch a `autoActionCh` si threshold).
  - `worker.go` (consumidor secuencial del canal; llama al thin wrapper `AutoActioner`).
  - `events_subscriber.go` (suscribe a `events.Bus` con `Handle(func(*Update))`, filtra `Update.Message != nil`).
- **Wiring** en `cmd/server/main.go`: instanciar `automation.Service` + `Worker` + `Subscriber`; `bus.Handle(subscriber.Handle)`; lanzar goroutine del worker junto al `Scheduler` de publications.
- **Thin wrapper `AutoActioner`**: gate `permissionOkAdmin(g) = g.BotStatus == StatusAdministrator` (publications pattern, bugfix #172); llama `tg.MuteUser`/`tg.BanUser`; escribe log con `ActorID=nil`. **NO** reusa `moderation.Service` (claves `can_*` no-pobladas).
- **Audit logs**: tabla `logs` existente con `ActorID=nil`. Nuevas constantes en `backend/internal/logs/model.go`: `ActionRuleTriggered` (cada hit, metadata `{rule, reason}`), `ActionAutomuteUser`, `ActionAutobanUser`.
- **Config**: `AUTOMATION_AUTOACTION_BUFFER_SIZE` (default 100) en `backend/internal/config/config.go`.
- **Tests (§21.1 estricto — cero Bot API real)**:
  - Unit por rule (`FloodRule_*`) + service (pipeline completo) + worker (channel drain, ctx cancel, overflow).
  - Integration repo con `OpenTestDB("automation")` + `goose.Up` + `TRUNCATE ... CASCADE`.
  - Subscriber: `events.Bus` fake; verifica filtrado `Update.Message == nil`.

### Out of Scope (deferred a slices posteriores)

- **Reglas anti-spam, anti-link, banned-words** → slice 2 (extiende `Registry`).
- **Tabla `banned_words`** → slice 2 (acompaña a banned-words rule; no requerida por Flood).
- **Settings editor UI** (`GroupAutomationPage` con toggles + `<TagsInput>`) → slice 2.
- **Warnings dashboard UI** (`GroupWarningsPage` + stats cards) → slice 3.
- **Refactor de `moderation.permissionOk`** (bugfix #172 keys no-pobladas) → issue separado, fuera de scope.
- **EditedMessage support** → follow-up.
- **Reset manual de warnings / notificaciones push** → follow-ups.

---

## Capabilities

### New Capabilities

- **`moderation-automation`**: settings per-group + warning state per `(group, user)` + rule registry + `FloodRule` + auto-action worker + audit logs con `ActorID=nil`. Spec canónico en `openspec/specs/moderation-automation/spec.md`.

### Modified Capabilities

_Ninguna al nivel de spec._ El bus ya soporta múltiples consumers (`telegram-events` REQ "Múltiples consumidores futuros") y `allowed_updates` ya incluye `message` (REQ "Filtro de tipos de update"). Slice 1 solo agrega **un consumer más** + tablas + reglas — no cambia requisitos del transporte ni del spec `telegram-moderation` (acciones manuales siguen iguales).

---

## Approach (decisiones D1–D10, scoped a slice 1)

| # | Decisión | Implementación |
|---|----------|----------------|
| **D1** | **Trigger source**: `events.Bus` consumer | `cmd/server/main.go`: `bus.Handle(subscriber.Handle)`; filtra `Update.Message == nil`. No publica nada nuevo; solo consume. |
| **D2** | **Rule registry con short-circuit** | `type Rule interface { Name() string; Check(ctx, msg, user, settings) (hit bool, reason string) }`; `Registry` itera en orden, primer hit gana. Solo `FloodRule` registrada en slice 1. |
| **D3** | **Settings storage**: tabla estructurada (NO JSONB) | Columnas indexables + CHECK constraints (`flood_messages>0`, `autoban_warnings>automute_warnings`). Una fila por grupo. |
| **D4** | **Warning state**: counter persistente en tabla nueva | `user_warning_state` PK `(group_id, user_id)`. Auto-reset por `expires_at = last_warning_at + warning_expire_days` (lógica en service al leer). |
| **D5** | **Auto-action**: canal interno + worker secuencial | `autoActionCh chan autoAction` (buffer configurable, default 100). Worker drena, llama `AutoActioner`. **NO** llama `moderation.Service` (evita bug #172). |
| **D6** | **Permission check propio**: `permissionOkAdmin` | `g.BotStatus == StatusAdministrator` (publications pattern, bugfix #172). Documentado en service como "NO usar `moderation.permissionOk`". |
| **D7** | **Audit logs con `ActorID=nil`** | Nuevas constantes `ActionRuleTriggered`/`ActionAutomuteUser`/`ActionAutobanUser`. Tabla `logs` reusada. |
| **D8** | **Frontend**: sin cambios | Slice 1 es backend-only. Validación con `psql` + logs. |
| **D9** | **Tests con fakes (§21.1)** | `TelegramService` mockeado (moq o a mano). Integration: `OpenTestDB("automation")` con `goose.Up`. Cero Bot API real. |
| **D10** | **Verificación `allowed_updates`** | Diseño re-confirma que `poller.go` (REQ "Filtro de tipos de update") y `webhook.go` (`setWebhook` con `allowed_updates`) ya incluyen `message`. Sin cambios en este slice. |

---

## Schema (migración 00006)

```sql
-- +goose Up
CREATE TABLE group_moderation_settings (
    group_id              BIGINT      PRIMARY KEY REFERENCES groups(telegram_id) ON DELETE CASCADE,
    enabled               BOOLEAN     NOT NULL DEFAULT false,
    anti_spam_enabled     BOOLEAN     NOT NULL DEFAULT false,
    anti_link_enabled     BOOLEAN     NOT NULL DEFAULT false,
    banned_words_enabled  BOOLEAN     NOT NULL DEFAULT false,
    flood_enabled         BOOLEAN     NOT NULL DEFAULT false,
    flood_messages        SMALLINT    NOT NULL DEFAULT 5   CHECK (flood_messages > 0),
    flood_seconds         SMALLINT    NOT NULL DEFAULT 10  CHECK (flood_seconds > 0),
    warning_limit         SMALLINT    NOT NULL DEFAULT 3   CHECK (warning_limit > 0),
    automute_warnings     SMALLINT    NOT NULL DEFAULT 3   CHECK (automute_warnings > 0),
    automute_minutes      SMALLINT    NOT NULL DEFAULT 10  CHECK (automute_minutes > 0),
    autoban_warnings      SMALLINT    NOT NULL DEFAULT 5   CHECK (autoban_warnings > automute_warnings),
    warning_expire_days   SMALLINT    NOT NULL DEFAULT 30  CHECK (warning_expire_days > 0),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE user_warning_state (
    group_id         BIGINT      NOT NULL,
    user_id          BIGINT      NOT NULL,
    warning_count    SMALLINT    NOT NULL DEFAULT 0 CHECK (warning_count >= 0),
    last_warning_at  TIMESTAMPTZ,
    last_action_at   TIMESTAMPTZ,
    expires_at       TIMESTAMPTZ,
    PRIMARY KEY (group_id, user_id)
);
CREATE INDEX idx_user_warning_state_group ON user_warning_state (group_id);

-- +goose Down
DROP TABLE IF EXISTS user_warning_state;
DROP TABLE IF EXISTS group_moderation_settings;
```

---

## Affected Areas

### Backend NUEVO

| Archivo | LOC est. |
|---------|---------:|
| `backend/migrations/00006_create_moderation_automation.sql` | +50 |
| `backend/internal/automation/model.go` | +80 |
| `backend/internal/automation/repository.go` | +180 |
| `backend/internal/automation/repository_test.go` | +160 |
| `backend/internal/automation/rules.go` | +100 |
| `backend/internal/automation/rules_test.go` | +150 |
| `backend/internal/automation/service.go` | +220 |
| `backend/internal/automation/service_test.go` | +200 |
| `backend/internal/automation/worker.go` | +110 |
| `backend/internal/automation/worker_test.go` | +100 |
| `backend/internal/automation/events_subscriber.go` | +60 |
| `backend/internal/automation/events_subscriber_test.go` | +60 |

### Backend MOD

| Archivo | LOC est. | Notas |
|---------|---------:|-------|
| `backend/internal/logs/model.go` | +8 | Constantes `ActionRuleTriggered`, `ActionAutomuteUser`, `ActionAutobanUser` |
| `backend/internal/config/config.go` | +5 | `AUTOMATION_AUTOACTION_BUFFER_SIZE` (default 100) |
| `backend/cmd/server/main.go` | +25 | Instanciar `Service`/`Worker`/`Subscriber`; `bus.Handle(...)`; goroutine del worker |

### Frontend
**Sin cambios en slice 1.** Validación del feature vía `psql` (`SELECT * FROM user_warning_state;`) + `logs` (`SELECT * FROM logs WHERE actor_id IS NULL ORDER BY created_at DESC;`).

**Total estimado**: ~1508 LOC. **Excede 400-line budget** → single-pr con `size:exception` (precedente: 6 publicaciones-slice PRs consecutivos aprobados). Tasks phase re-confirmará o propondrá chained (`backend-core / events+worker`).

---

## Risks

| Riesgo | Likelihood | Mitigation |
|--------|-----------|------------|
| `allowed_updates` no incluye `message` en `poller.go` / `events.go` (`setWebhook`) | Low (verificado en `telegram-events` spec REQ "Filtro de tipos de update") | Diseño re-confirma explícitamente: smoke test post-merge con un mensaje real → `psql -c "SELECT * FROM user_warning_state"` muestra la fila. Si NO aparece, falla crítica. |
| Buffer `autoActionCh` overflow bajo flood masivo | Medium | `AUTOMATION_AUTOACTION_BUFFER_SIZE=100` configurable; `non-blocking send` (select con default) → log warn + drop si lleno. Re-diseño con Redis queda fuera de §2. |
| Worker no respeta rate limit | Low | Worker llama `tg.MuteUser`/`tg.BanUser` que pasan por `doWithRetry` + token bucket del adapter. **Sin bypass posible**. |
| Bot admin removido con auto-action encolada | Low | Worker re-lee `g.BotStatus` antes de actuar; si `≠ StatusAdministrator` → log `PERMISSION_DENIED`, drop. |
| Auto-ban sobre admin del grupo (Telegram 400/403) | Low | Adapter ya mapea a `ErrPermissionDenied` → log `PERMISSION_DENIED`, counter no incrementa. Riesgo aceptable. |
| `warning_count` se incrementa antes de rule real fire | Low | Order estricto: 1) rule.Check returns hit; 2) settings loaded + `enabled` + `flood_enabled` true; 3) increment; 4) threshold check + dispatch. Si disabled → skip silencioso. |
| Race `HandleMessage` vs `UpsertSettings` concurrentes | Low | UPSERT idempotente; worker secuencial (canal). Sin race material. |
| Migración 00006 FK hacia `groups(telegram_id)` rompe tests aislados | Low | Integration tests usan `OpenTestDB("automation")` + `TRUNCATE groups CASCADE` (patrón publications-slice3). |
| `permissionOkAdmin` reintroduce bug #172 por copy-paste | Low | Helper aislado en `automation.AutoActioner`; tests cubren ambos casos (admin + member); comentario explícito "NO usar `moderation.permissionOk`". |
| Memory leak por cache de settings | Low | **Sin cache** en slice 1. Lectura por mensaje. LRU solo en slice 2 si `EXPLAIN ANALYZE` lo justifica. |
| Slice excede 400 LOC | **High** | Forecast ~1500. `size:exception` solicitada (precedente 6 PRs). Tasks confirmará o propondrá chained. |

---

## Rollback

`git revert` del merge. Migración 00006 Down: `DROP TABLE user_warning_state, group_moderation_settings`. Workers auto-detienen (ctx cancel). Frontend intacto. **Audit logs se preservan** — siguen siendo válidos para análisis post-rollback. **No perder datos**: ninguna otra tabla afectada.

---

## Dependencies

- `backend/internal/events/bus.go` (multi-consumer REQ ya cumplido).
- `backend/internal/telegram` (`Service.MuteUser`, `Service.BanUser` con token bucket).
- `backend/internal/logs` (`LogWriter`, `Action*` constants; tabla `logs` con `ActorID *int64`).
- `backend/internal/groups` (`Repository.GetByID` para `permissionOkAdmin`).
- `backend/internal/config` (extender con `AUTOMATION_AUTOACTION_BUFFER_SIZE`).
- `goose` 00001-00005 ya aplicados.

---

## Delivery

**Single-pr, size:exception solicitada** (precedente: 6 PRs consecutivos publications-slice1/2/3 + frontend-refresh-slice2). Branch: `feat/moderation-automation-slice1` base `main @ 24e4c8c`. **Tasks phase re-confirmará** el forecast (~1500 LOC > 400) o recomendará chained (`backend-core` / `events-subscriber + worker`).

---

## Success Criteria

- [ ] `go test ./...` verde; `go vet`/`gofmt`/`sqlc` (si aplica) limpios
- [ ] Migración 00006 con Up + Down aplicados en dev; `groups(telegram_id)` FK válida
- [ ] `events.Bus` entrega `Update.Message` al subscriber; filtrado `Update.Message == nil` cubre otros tipos
- [ ] `FloodRule.Check` retorna hit cuando N msgs en W seg; bumpea `warning_count` cuando hit + `flood_enabled`
- [ ] `warning_count >= automute_warnings` → encolado en `autoActionCh` → worker llama `tg.MuteUser` → log `ActionAutomuteUser/SUCCESS` con `ActorID=nil`
- [ ] `warning_count >= autoban_warnings` → log `ActionAutobanUser/SUCCESS` con `ActorID=nil`
- [ ] Cada rule hit loguea `ActionRuleTriggered` con metadata `{rule:"flood", reason:"N msgs in W sec"}`
- [ ] Worker drena canal en orden FIFO; respeta rate limit (verificable con fakes); ctx cancel retorna `nil`
- [ ] Buffer overflow → log warn + drop; bus NO se bloquea
- [ ] Bot admin removido → worker loguea `PERMISSION_DENIED`, NO ejecuta acción
- [ ] `permissionOkAdmin` verifica `g.BotStatus == StatusAdministrator` (tests cubren admin + member + creator)
- [ ] `allowed_updates` confirmado en diseño: `message` presente en `poller.go` (REQ "Filtro de tipos de update") y en `setWebhook` de `events.go`
- [ ] Frontend sin cambios; verificable con `psql` + `logs` (sin UI)
- [ ] §21.1 estricto: cero llamadas Bot API reales en tests (audit en `verify`)

---

## Open Questions

Ninguna — la exploración (#215) y el bugfix #172 resolvieron todo (Bot API cubre `restrictChatMember`/`banChatMember`; rate limit del adapter cubre al worker; permission check vía `BotStatus`; short-circuit en registry; sin Redis; canales internos Go para auto-actions).