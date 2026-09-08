# Proposal: Moderation Automation — Slice 2 (Anti-spam + Anti-link + Banned-words + Settings UI)

> **Change**: `moderation-automation-slice2` — Slice 2/3 de Fase 3 (AGENTS §23).
> **Mode**: hybrid (filesystem + Engram).
> **Predecessor**: exploration `#215` (`sdd/moderation-automation/exploration`); slice 1 archivado en `openspec/changes/archive/2026-09-07-moderation-automation-slice1/` (merge `main @ 1b10d42`, observación `#220`).
> **Authority**: bugfix `#172` (`permissionOkAdmin` usa `BotStatus == StatusAdministrator`, NUNCA `can_*`).
> **Strategy**: single-pr con `size:exception` (precedente: 7 PRs consecutivos — re-confirmar en `tasks`).
> **Branch base**: `main @ 1b10d42` (slice 1 ya mergeado).

---

## Intent

Extender `moderation-automation` (AGENTS §23, Fase 3) con las **3 reglas que faltan** (Anti-spam, Anti-link, Banned-words), persistir listas de palabras y dominios por grupo, y dar al admin un **editor de settings por grupo** (toggles, umbrales, lista de palabras y link allowlist) en una página dedicada del panel. Cierra el motor de reglas; el dashboard de advertencias (warnings + stats) queda para slice 3.

---

## Scope

### In Scope

- **Migración `00007_create_moderation_lists.sql`**: tablas `banned_words(group_id, word)` y `link_allowlist(group_id, domain)` con PK compuesta `(group_id, word|domain)`, FK `groups(telegram_id)` ON DELETE CASCADE, índice por prefijo para `LIKE`-queries si hicieran falta.
- **Repository** (extender `backend/internal/automation/repository.go`): métodos `ListBannedWords(groupID) ([]string, error)`, `AddBannedWord(groupID, word)`, `RemoveBannedWord(groupID, word)`, `SetBannedWords(groupID, words []string)` (atomic replace en transacción), y los 4 análogos para `link_allowlist`.
- **3 Rule implementations** nuevas en `backend/internal/automation/rules.go` (extender el archivo existente):
  - **`AntiSpamRule`** — detecta (a) ALL CAPS (`>70%` letras mayúsculas en `len(text)>10`), (b) caracteres repetidos (`5+` chars iguales consecutivos), (c) URLs muy cortas (`<20` chars después de `https?://|t\.me/|telegram\.me/`). Cada sub-detector emite `RuleHit` con `Reason` específico.
  - **`AntiLinkRule`** — regex `https?://[^\s]+|t\.me/[^\s]+|telegram\.me/[^\s]+`; si el dominio está en `link_allowlist` del grupo → skip; si no → hit.
  - **`BannedWordsRule`** — case-insensitive `strings.Contains(strings.ToLower(text), word)` para cada fila de `banned_words`; primera palabra matchada define `Reason`.
- **Service registration**: `cmd/server/main.go` agrega al registry (después del `FloodRule` de slice 1) `NewAntiSpamRule()`, `NewAntiLinkRule()`, `NewBannedWordsRule(bannedWordsRepo, linkAllowlistRepo)`. Orden: cheap → expensive (Flood in-mem, AntiSpam CPU, AntiLink CPU, BannedWords DB).
- **Endpoints** en nuevo `backend/internal/api/automation_handlers.go` (montados vía nuevo `WithAutomation` option en `api/server.go`):
  - `GET  /api/groups/{id}/automation/settings` → `*Settings` JSON.
  - `PUT  /api/groups/{id}/automation/settings` → body parcial; persiste UPSERT; logs `ActionUpdateAutomationSettings` con `ActorID` del admin.
  - `GET  /api/groups/{id}/automation/banned-words` → `[]string`.
  - `POST /api/groups/{id}/automation/banned-words {word}`; `DELETE /api/groups/{id}/automation/banned-words/{word}`. Cada cambio loguea `ActionAddBannedWord` / `ActionRemoveBannedWord`.
  - `GET  /api/groups/{id}/automation/link-allowlist` → `[]string`. `POST` / `DELETE` simétricos con `ActionAddLinkAllowlist` / `ActionRemoveLinkAllowlist`.
- **Logs**: 5 constantes nuevas en `backend/internal/logs/model.go`: `ActionUpdateAutomationSettings`, `ActionAddBannedWord`, `ActionRemoveBannedWord`, `ActionAddLinkAllowlist`, `ActionRemoveLinkAllowlist` — todas con `ActorID` del admin (NO `nil`; son cambios manuales).
- **Frontend** (Mantine v7, `frontend-ui-foundation` precedent):
  - Nuevo `frontend/src/features/automation/{types,api,hooks,error,validateThresholds}.ts` (estructura simétrica a `features/publications/`).
  - Nueva página `frontend/src/pages/GroupAutomationPage.tsx` con: switch principal `Habilitar moderación automática`, 4 toggles (`Flood`, `Anti-spam`, `Anti-link`, `Banned-words`), 7 `NumberInput`s (thresholds), `<TagsInput>` Mantine v7 para `banned_words` y para `link_allowlist`, **un único botón "Guardar" al fondo** que dispara 3 roundtrips (PUT settings, POST/DELETE por palabra/dominio nuevo). Notificación `notifySuccess`/`notifyError` por sección.
  - Ruta nueva `/groups/:id/automation` registrada en `App.tsx` (`GroupAutomationPage`); link desde `GroupDetailPage` (Panel "Detalle" → botón "Moderación automática").
- **Tests** (§21.1 estricto):
  - **Unit** por rule (tabla mensaje→hit, 5+ casos cada una): all-caps sin hit por longitud corta, repeated chars hit, short URL hit, allowlist domain skip, banned-word case-insensitive match.
  - **Service** unit: pipeline con 4 reglas; short-circuit flood-then-banned-words; cheap-first ordering.
  - **Repository** integration: `OpenTestDB("automation")` + `goose.Up` + `TRUNCATE ... CASCADE`; cases: add/remove/list words, atomic SetWords, FK CASCADE al borrar grupo.
  - **Handler**: 200/400/404 paths con fakes; auth required.
  - **Frontend**: render con `mockFetchRoutes`, agregar palabra dispara POST, remover dispara DELETE, toggle cambia state local + PUT, save all dispara 3 roundtrips, error path muestra `notifyError`.
- **Docs**: `README.md` sección "Moderación automática" con ejemplo de cada regla; `.env.example` sin cambios (slice 1 ya cubre env).

### Out of Scope (slice 3 / follow-ups)

- **Dashboard de advertencias** (`GroupWarningsPage` + stats cards agregadas sobre `logs` filtrado por fecha) → slice 3.
- **EditedMessage** → follow-up (Bot API ya expone `Update.EditedMessage`).
- **Refactor `moderation.permissionOk`** (bugfix #172 claves no-pobladas) → issue separado.
- **Reset manual de warnings / push notifications / cache de settings** → follow-ups.
- **`moq` codegen**: tests siguen hand-rolled fakes (publications-slice3 precedent).

---

## Capabilities

### Modified Capabilities

- **`moderation-automation`** (delta spec): 3 reglas nuevas en el registry (AntiSpam, AntiLink, BannedWords), 2 tablas nuevas (`banned_words`, `link_allowlist`), endpoints CRUD de settings + lists, Action constants de auditoría, página `GroupAutomationPage` en `/groups/:id/automation`. Spec canónico en `openspec/specs/moderation-automation/spec.md` se AMPLÍA con REQs nuevas (no se reemplaza).

> **Decisión de spec (DECIDE)**: **AMEND** el spec canónico existente (`openspec/specs/moderation-automation/spec.md`) vía delta en `openspec/changes/moderation-automation-slice2/specs/moderation-automation/spec.md`. Aplica `ADDED Requirements` por cada nueva REQ. Archive al cierre usa la técnica MOVE de slice 1 (delta → canonical). NO se crea spec paralelo (`moderation-automation-settings/`) — sería duplicación.

---

## Approach

| # | Decisión | Cómo |
|---|----------|------|
| **D1** | **Schema**: 2 tablas paralelas `banned_words` y `link_allowlist` (PK `(group_id, word|domain)`, FK CASCADE a `groups`). | Migración `00007`. Misma shape simétrica — queries y repository son cuasi-iguales. |
| **D2** | **Case-insensitive**: `banned_words.word` se normaliza `LOWER()` al insertar; `BannedWordsRule` lowercases el texto y cada `word`. | Migración tiene CHECK `length(word) BETWEEN 1 AND 100`. Sin índices funcionales (volumen bajo por grupo; PK basta). |
| **D3** | **Subdomain match** en `link_allowlist`: matcher acepta dominio exacto **y** cualquier subdominio (`example.com` matchea `sub.example.com`). | Helper `domainMatches(allowDomain, msgDomain)`: extract host de URL, compare suffix. No `*`/wildcards. |
| **D4** | **Rule registry order**: Flood → AntiSpam → AntiLink → BannedWords. Cheap (CPU/mem) primero; expensive (DB query en BannedWords) último. | Slice 1 ya tiene `Registry.Register()`; slice 2 agrega 3 más en `cmd/server/main.go`. |
| **D5** | **AntiSpamRule** stateless (CPU puro sobre el texto). | Sin estado mutable; sin tests de concurrencia. |
| **D6** | **AntiLinkRule** stateless. Recibe `linkAllowlist []string` cacheada en el mensaje via `Service.HandleMessage` (load una vez por mensaje, no por rule). | El Service pre-carga las dos listas antes de invocar el registry; pasa al rule vía el contexto o extendiendo la firma `Evaluate`. Decisión concreta: extender `Rule.Evaluate` para recibir `now` (ya está) + `lists *Lists` con `BannedWords []string` y `LinkAllowlist []string` (pre-cargadas por Service). Sin cambios al registry. |
| **D7** | **BannedWordsRule** recibe `lists.BannedWords`. Match `strings.Contains(lower(text), word)`; primer match define `Reason`. | Si la lista está vacía, retorna nil (skip). |
| **D8** | **Settings endpoints**: PUT upsert full row (frontend envía state completo). El handler normaliza defaults (si falta campo, default) y llama `repository.UspsertSettings`. | Validación: `autoban_warnings > automute_warnings > 0` (CHECK en DB ya lo enforce). |
| **D9** | **Lists endpoints**: `POST` y `DELETE` operan sobre una sola palabra/dominio por request. `GET` devuelve lista ordenada alfabéticamente. | Cada cambio → `ActionAdd/RemoveX` con `ActorID` del admin. |
| **D10** | **Frontend**: single Save button al fondo de la página; el form mantiene un `dirty` state local y solo hace roundtrip de los campos modificados. | Si el admin toca una palabra del `<TagsInput>`, el componente actualiza state local; el botón Save hace PUT de settings + POST/DELETE por palabra/dominio cambiado. Toggles también requieren Save (consistente con publications, que es POST todo de una). |
| **D11** | **Link desde GroupDetailPage**: nuevo botón "Moderación automática" en el panel "Detalle", abajo del existente "Membresía y moderación". | Reusa el patrón `component={Link} to={…}`. Sin tocar `Tabs`. |

---

## Schema (migración 00007)

```sql
-- +goose Up
CREATE TABLE banned_words (
    group_id   BIGINT NOT NULL REFERENCES groups(telegram_id) ON DELETE CASCADE,
    word       TEXT   NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (group_id, word),
    CHECK (length(word) BETWEEN 1 AND 100)
);
CREATE INDEX idx_banned_words_group ON banned_words (group_id);

CREATE TABLE link_allowlist (
    group_id   BIGINT NOT NULL REFERENCES groups(telegram_id) ON DELETE CASCADE,
    domain     TEXT   NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (group_id, domain),
    CHECK (length(domain) BETWEEN 1 AND 253)
);
CREATE INDEX idx_link_allowlist_group ON link_allowlist (group_id);

-- +goose Down
DROP TABLE IF EXISTS link_allowlist;
DROP TABLE IF EXISTS banned_words;
```

---

## Affected Areas

| Area | Impact | LOC est. |
|------|--------|---------:|
| `backend/migrations/00007_create_moderation_lists.sql` | **NEW** | +30 |
| `backend/internal/automation/rules.go` | MOD (+3 rules, `Rule.Evaluate` extended signature) | +260 |
| `backend/internal/automation/rules_test.go` | MOD (+18 cases) | +400 |
| `backend/internal/automation/repository.go` | MOD (+4 banned + 4 allowlist methods) | +150 |
| `backend/internal/automation/repository_test.go` | MOD (+8 integration cases) | +250 |
| `backend/internal/automation/service.go` | MOD (`HandleMessage` pre-carga lists) | +60 |
| `backend/internal/automation/service_test.go` | MOD (+5 pipeline cases) | +150 |
| `backend/internal/api/automation_handlers.go` | **NEW** (9 handlers) | +260 |
| `backend/internal/api/automation_handlers_test.go` | **NEW** | +280 |
| `backend/internal/api/server.go` | MOD (new `WithAutomation` option) | +35 |
| `backend/internal/logs/model.go` | MOD (+5 Action constants) | +10 |
| `backend/cmd/server/main.go` | MOD (+3 registry.Register, +repo wiring) | +20 |
| `frontend/src/features/automation/{types,api,hooks,error,validateThresholds}.ts` | **NEW** | +340 |
| `frontend/src/pages/GroupAutomationPage.tsx` (+ test) | **NEW** | +620 |
| `frontend/src/App.tsx` | MOD (+1 route) | +3 |
| `frontend/src/pages/GroupDetailPage.tsx` | MOD (+1 botón) | +12 |
| `frontend/src/pages/GroupDetailPage.test.tsx` | MOD (+1 case) | +15 |
| `README.md` | MOD (sección moderación automática) | +60 |

**Total**: ~2955 LOC. Excede budget por ~7×. **`size:exception`** (precedente 7/7 PRs aprobados).

---

## Risks

| Risk | Likelihood | Mitigation |
|------|-----------|------------|
| Domain matcher (allowlist) confunde `example.com` con `notexample.com` | Medium | Suffix match solo si hay `.` antes del dominio; tests cubren casos `example.com`, `sub.example.com`, `notexample.com`. |
| Race `UpsertSettings` mientras `HandleMessage` lee settings | Low | UPSERT idempotente; lectura después de upsert siempre ve estado consistente. Documentar en design. |
| Banned-words duplicados al agregar múltiples veces | Low | PK compuesta `(group_id, word)` + `ON CONFLICT DO NOTHING`; POST devuelve 200 con lista actualizada. |
| `<TagsInput>` no acepta caracteres unicode (ej. emoji) | Low | Validar regex cliente `^[\p{L}\p{N}_\- ]{1,100}$`; bloquear en frontend antes de POST. |
| Frontend Save con muchos roundtrips secuenciales (palabras+settings) | Low | Save usa `Promise.all`; tolera errores parciales (lista de notificaciones). |
| `BannedWordsRule` carga todas las palabras en cada mensaje | Low | Volumen bajo (<1000 palabras típico); pre-carga una vez por mensaje vía Service. LRU cache en slice 2.5 si performance lo pide. |
| Case-sensitivity inconsistente entre frontend/backend | Low | Backend normaliza `LOWER(word)` en insertar; matcher lowercases el texto. Frontend trim+lowercase antes de POST (validación cliente). |
| `permissionOk` bugfix #172 reintroducido en handlers | Low | Handlers de slice 2 son admin-gated (panel auth), no tocan el bot. Documentado en comentarios de los nuevos handlers. |
| Slice excede 400 LOC | **High** | Forecast ~2955 LOC. `size:exception` solicitada (precedente 7/7). Tasks phase re-confirmará o propondrá chained (`backend-rules+handlers` / `frontend-page+settings`). |

---

## Rollback

`git revert` del merge. Migración 00007 Down elimina `link_allowlist` y `banned_words`. `cmd/server/main.go` deja de registrar las 3 reglas nuevas (registry queda solo con `Flood`); el pipeline sigue funcionando con la regla de slice 1. Frontend: ruta `/groups/:id/automation` queda muerta (frontend no carga, devuelve 404 — aceptable durante rollback). Audit logs preservados (`RULE_TRIGGERED` con `rule_name` antiguo queda como histórico).

---

## Dependencies

- Slice 1 archivado (`main @ 1b10d42`, observación `#220`) + spec canónico `openspec/specs/moderation-automation/spec.md`.
- Tablas `group_moderation_settings`, `user_warning_state` (migración 00006).
- `events.Bus` ya entrega `Update.Message` (verificado slice 1, REQ-4).
- `telegram.Service.MuteUser`/`BanUser` con token bucket (rate-limit heredado, §18.1).
- Frontend: Mantine v7 (`TagsInput`, `Switch`, `NumberInput` ya en uso en publications-slice2/slice3), `frontend-ui-foundation`, `frontend-pages-moderation`.

---

## Delivery

**Single-pr, size:exception solicitada** (precedente: 7 PRs consecutivos aprobados — slice 1, slice2/3/4 de publications, frontend-refresh slice 1/2, frontend-pages-moderation). Branch: `feat/moderation-automation-slice2` base `main @ 1b10d42`. Tasks phase re-confirmará el forecast (~2955 LOC > 400) o recomendará chained.

---

## Success Criteria

- [ ] `00007_create_moderation_lists.sql` Up/Down aplica limpia; FK CASCADE borra palabras al borrar grupo
- [ ] `banned_words` lowercase-normalizado al insertar; `BannedWordsRule` matchea case-insensitive
- [ ] `link_allowlist` match exacto + subdominio; test cubre `example.com`, `sub.example.com`, `notexample.com`
- [ ] `AntiSpamRule`: all-caps hit (>70% mayúsculas, len>10); repeated chars hit (5+); short URL hit
- [ ] `AntiLinkRule`: hit por URL fuera de allowlist; allowlist hit NO registrado
- [ ] Registry respeta orden cheap→expensive: Flood → AntiSpam → AntiLink → BannedWords
- [ ] Service pre-carga `banned_words`/`link_allowlist` una vez por mensaje (no por rule)
- [ ] `PUT /api/groups/:id/automation/settings` upsert idempotente; log `UPDATE_AUTOMATION_SETTINGS` con ActorID del admin
- [ ] `POST/DELETE /api/groups/:id/automation/banned-words[/:word]` loguean `ADD/REMOVE_BANNED_WORD`; idempotencia vía PK
- [ ] Mismo patrón para `/link-allowlist`
- [ ] Frontend `GroupAutomationPage` en `/groups/:id/automation`: toggles + thresholds + 2 `<TagsInput>` + Save all
- [ ] Save dispara 3 roundtrips paralelos; notificaciones por sección; error path con `notifyError`
- [ ] Link desde `GroupDetailPage` → "Detalle" → botón "Moderación automática"
- [ ] `permissionOkAdmin` invariante intacto (`grep can_* backend/internal/automation/` = 0 matches)
- [ ] §21.1: cero llamadas Bot API reales en tests (audit en `verify`)
- [ ] `go test ./...`, `npm test`, `go vet`, `gofmt -l .`, `npm run build` en verde
- [ ] §25 sin secretos en repo; sin cambios en `backend/internal/moderation/`

---

## Open Questions

Ninguna — la exploración (#215) y el slice 1 archivado (#218-#223) resolvieron todo (3 reglas faltantes según AGENTS §23, schema de listas, permission check vía `BotStatus`, rate limit del adapter, sin Redis, frontend pattern con Mantine v7 ya consolidado). Las decisiones D1-D11 son operativas dentro de este slice y se confirman en `design`.