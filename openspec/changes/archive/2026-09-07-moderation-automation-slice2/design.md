# Design: `moderation-automation-slice2` — Anti-spam + Anti-link + Banned-words + Settings UI

> **Change**: `moderation-automation-slice2` (single-pr, size:exception).
> **Predecessor**: slice 1 archivado (`main @ 1b10d42`); canónico `openspec/specs/moderation-automation/spec.md`.
> **Authority**: bugfix `#172` (`permissionOkAdmin` usa `BotStatus == StatusAdministrator`, NUNCA `can_*`).
> **Branch**: `feat/moderation-automation-slice2` base `main @ 1b10d42`.

---

## Technical Approach

Extender el motor de moderación automática de slice 1 con las **3 reglas restantes** del AGENTS §23 (Anti-spam, Anti-link, Banned-words), persistir listas por grupo en 2 tablas nuevas, exponer **9 endpoints REST** para que el admin edite settings/listas, y entregar una página dedicada `/groups/:id/automation` con editor único (5 Switches + 6 NumberInput + 2 TagsInput + 1 Save con `Promise.all`). La firma `Rule.Evaluate` se extiende con `*Lists{BannedWords, LinkAllowlist}` para que el `Service` pre-cargue las listas **una vez por mensaje** (evita N queries por rule). El orden del registry queda `Flood → AntiSpam → AntiLink → BannedWords` (cheap → expensive). Las invariantes de slice 1 (`permissionOkAdmin`, adapter rate-limit, sin Redis, sin microservicios) se preservan.

---

## Architecture Decisions

### D1 — Tablas `banned_words` y `link_allowlist` paralelas, no JSONB

**Choice**: 2 tablas separadas con PK compuesta `(group_id, word|domain)`, FK CASCADE a `groups(telegram_id)`.
**Alternatives**: (a) columna `banned_words TEXT[]` en `group_moderation_settings`; (b) JSONB.
**Rationale**: PK compuesta da idempotencia nativa (`ON CONFLICT DO NOTHING`), queries triviales con índice sobre `group_id`, y FK CASCADE limpia al borrar el grupo. JSONB pierde constraints SQL y hace case-insensitive más caro.

### D2 — Case-insensitive en `banned_words` server-side

**Choice**: el INSERT normaliza `LOWER(word)`; el `BannedWordsRule` lowercases el texto y compara.
**Alternatives**: guardar el word tal cual + lower en el matcher.
**Rationale**: el word guardado es siempre la versión canónica (lowercase). El matcher y el handler usan el mismo path → tests deterministas. Para `link_allowlist` se preserva el case (Telegram hosts son case-insensitive en la práctica; el matcher lowercases en evaluación).

### D3 — Subdomain match con prefijo-punto obligatorio

**Choice**: helper `domainMatches(allow, msg)` exige `.` antes del dominio allowlist.
**Alternatives**: (a) suffix match sin punto; (b) wildcard `*.example.com`.
**Rationale**: `notexample.com` NO debe matchear `example.com` (precedente bugfix común). El helper queda:
```go
// acepta: example.com matchea example.com y sub.example.com
// rechaza: notexample.com (sin punto previo)
host == allow || strings.HasSuffix(host, "."+allow)
```

### D4 — `Rule.Evaluate` extendido con `*Lists` (no dos args sueltos)

**Choice**: nuevo parámetro `lists *Lists` entre `ws *WarningState` y `now time.Time`.
**Alternatives**: (a) dos args `bannedWords []string, allowlist []string`; (b) pasar el repo a las reglas.
**Rationale**: struct nombrada evita firmas largas y permite crecer sin romper la interfaz. El repo en la regla rompe el principio "rules stateless / pure" de slice 1.

### D5 — Registry order fixed en `cmd/server/main.go`

**Choice**: orden de invocación `Register(NewFloodRule/NewAntiSpamRule/NewAntiLinkRule/NewBannedWordsRule)`.
**Alternatives**: API pública de reordering.
**Rationale**: el orden cheap-first (in-mem → CPU → DB-pre-loaded) es decisión arquitectónica, no runtime config. Mantener orden hardcoded evita complejidad.

### D6 — Frontend single Save con `Promise.all`

**Choice**: 1 botón "Guardar" dispara en paralelo `PUT settings + N×POST/DELETE words + M×POST/DELETE domains`.
**Alternatives**: (a) Save por sección (3 botones); (b) auto-save on change.
**Rationale**: consistente con publications (`Promise.all`). Errores por sección se acumulan como notificaciones separadas — un fallo NO aborta el resto. Toggles también requieren Save (consistente con el patrón del editor).

### D7 — Audit log con `ActorID` admin (≠ nil)

**Choice**: las 5 constantes nuevas (`UPDATE_AUTOMATION_SETTINGS`, `ADD/REMOVE_BANNED_WORD`, `ADD/REMOVE_LINK_ALLOWLIST`) registran `ActorID != nil`.
**Alternatives**: reusar constantes existentes con `ActorID=nil` (patrón slice 1).
**Rationale**: en slice 1, `ActorID=nil` distingue auto-actions del sistema. Slice 2 son cambios **manuales** del admin desde el panel — `ActorID` es el id del admin. Comentario en `model.go` explica la distinción.

### D8 — Idempotencia vía PK compuesta + `ON CONFLICT DO NOTHING`

**Choice**: POST devuelve 200 con la lista actualizada (nunca 409).
**Alternatives**: devolver 409 si la palabra ya existía.
**Rationale**: el frontend usa diff (POST por palabra nueva, DELETE por removida); reintentos idempotentes simplifican la lógica de Save.

---

## Data Flow

```
Update.Message
   │
   ▼
Subscriber (events.Bus)
   │
   ▼
Service.HandleMessage (slice 1, MOD)
   │
   ├─ LoadOrCreateSettings            → settings
   ├─ permissionOkAdmin (g)            ───── bugfix #172
   ├─ LoadOrCreateWarningState         → ws
   ├─ if BannedWordsEnabled OR AntiLinkEnabled:
   │     └─ bannedWordsRepo.ListBannedWords      ┐
   │        linkAllowlistRepo.ListLinkAllowlist  ┴─ pre-load (1 vez)
   ├─ Registry.Evaluate(msg, settings, ws, lists, now)
   │     ├─ FloodRule         (cheapest, in-mem)
   │     ├─ AntiSpamRule      (CPU puro sobre texto)
   │     ├─ AntiLinkRule      (CPU + lists.LinkAllowlist)
   │     └─ BannedWordsRule   (lists.BannedWords, post-pre-load)
   └─ enqueue AutoAction si WarningCount ≥ threshold
```

```
Panel Admin (GroupAutomationPage)
   │
   ├─ GET  /api/groups/:id/automation/settings       → *
   ├─ GET  /api/groups/:id/automation/banned-words   → []string
   ├─ GET  /api/groups/:id/automation/link-allowlist → []string
   │
   └─ [Save] Promise.all:
         ├─ PUT  /api/groups/:id/automation/settings
         ├─ POST/DELETE /automation/banned-words[/:word]  (diff vs original)
         └─ POST/DELETE /automation/link-allowlist[/:domain]
```

---

## File Changes

### Backend NEW

| Archivo | LOC est. | Descripción |
|---------|---------:|-------------|
| `backend/migrations/00007_create_moderation_lists.sql` | +35 | DDL `banned_words` y `link_allowlist` (PK compuesta, FK CASCADE, CHECK len 1-100/253, índice por `group_id`). Goose Up/Down. Reusa patrón de `00006_create_moderation_automation.sql`. |
| `backend/internal/api/automation_handlers.go` | +260 | 9 handlers: GET/PUT settings, GET/POST/DELETE banned-words, GET/POST/DELETE link-allowlist. Cada uno usa `requireAuth`, `actorIDFromClaims`, decode JSON con envelope `respond/w/respondError`. Mapeo errores a §18. |
| `backend/internal/api/automation_handlers_test.go` | +280 | Tests con fakes de SettingsRepo+ListsRepo+LogWriter. Cubre 200/400/404 paths y auth required. |

### Backend MOD

| Archivo | LOC est. | Notas |
|---------|---------:|-------|
| `backend/internal/automation/model.go` | +20 | Agrega `Lists{BannedWords []string, LinkAllowlist []string}` y `ErrAutomationGroupNotFound` (404 cuando el grupo no existe). |
| `backend/internal/automation/repository.go` | +170 | + `ListBannedWords/AddBannedWord/RemoveBannedWord` y 3 simétricos para `link_allowlist`. Case-normalized en banned (LOWER), case-preserved en allowlist. |
| `backend/internal/automation/rules.go` | +230 | Extiende firma `Rule.Evaluate(..., lists *Lists, now)`; +3 structs (`AntiSpamRule`, `AntiLinkRule`, `BannedWordsRule`). `FloodRule.Evaluate` actualizado para aceptar la nueva firma (pasa nil a `lists`; implementación intacta). `Registry.Evaluate` propaga `lists`. |
| `backend/internal/automation/service.go` | +60 | Si `BannedWordsEnabled OR AntiLinkEnabled` → pre-load listas vía `ListsRepo` interface; pasa `*Lists` al registry. Si ambos toggles off → omite las 2 queries (optimización, test verifica). |
| `backend/internal/automation/rules_test.go` | +400 | +18 unit cases: AntiSpam (all-caps hit / texto corto sin hit / repeated chars / short URL / toggle off), AntiLink (URL fuera / exacto skip / subdominio skip / prefijo-falso hit / toggle off), BannedWords (case-insensitive / substring / vacía skip / toggle off). |
| `backend/internal/automation/service_test.go` | +150 | +5 cases: pre-load cuando ambos toggles activos, omisión cuando ambos off, listas compartidas entre reglas, propagación `lists == nil` defensivo. |
| `backend/internal/automation/repository_test.go` | +250 | +8 integration cases con `OpenTestDB("automation")` + `goose.Up`: add/remove/list words, atomic set, FK CASCADE al borrar grupo, idempotencia en POST. TRUNCATE extendido a las 2 tablas nuevas. |
| `backend/internal/automation/autoactioner.go` | 0 | INTACTO. |
| `backend/internal/automation/autoactioner_test.go` | 0 | INTACTO. |
| `backend/internal/api/server.go` | +30 | + `WithAutomation(svc automationService, lists listsRepo) ServerOption` que monta las 9 rutas bajo `/api/groups/:id/automation/...`. |
| `backend/internal/logs/model.go` | +10 | +5 constantes: `ActionUpdateAutomationSettings`, `ActionAddBannedWord`, `ActionRemoveBannedWord`, `ActionAddLinkAllowlist`, `ActionRemoveLinkAllowlist`. Comentario distingue patrón manual (slice 2) del auto (slice 1). |
| `backend/cmd/server/main.go` | +25 | Construye `automationRepository` (ya existe de slice 1) + `automation.NewService(...)` + `registry.Register(NewAntiSpamRule/NewAntiLinkRule/NewBannedWordsRule)`. Pasa `api.WithAutomation(automationService, automationRepo)` en ambos casos (webhook + polling). |

### Frontend NEW

| Archivo | LOC est. | Descripción |
|---------|---------:|-------------|
| `frontend/src/features/automation/types.ts` | +60 | `AutomationSettings`, `SettingsUpdate`, `BannedWordsList`, `LinkAllowlistList`. |
| `frontend/src/features/automation/api.ts` | +90 | `getSettings(groupId)`, `putSettings(groupId, s)`, `listBannedWords(groupId)`, `addBannedWord(groupId, w)`, `removeBannedWord(groupId, w)`, `listLinkAllowlist(groupId)`, `addAllowlistEntry`, `removeAllowlistEntry`. |
| `frontend/src/features/automation/hooks.ts` | +130 | `useAutomationSettings`, `useBannedWords`, `useLinkAllowlist`, `useUpdateSettings`, `useAddBannedWord`, `useRemoveBannedWord`, `useAddAllowlist`, `useRemoveAllowlist` (todos con `useQuery/useMutation`, `retry:false`, invalidación por prefijo). |
| `frontend/src/features/automation/error.ts` | +50 | `formatAutomationError`, `validateBannedWordClient` (regex `^[\p{L}\p{N}_\- ]{1,100}$`), `validateAllowlistClient`, `validateThresholdsClient` (`autoban > automute > 0`). |
| `frontend/src/pages/GroupAutomationPage.tsx` | +480 | 4 secciones: Switches (1+4), NumberInputs (6), TagsInput banned (con validación), TagsInput allowlist. State local `original` para diff. Save único dispara `Promise.all` paralelo; errores por sección como `notifyError` separados. |
| `frontend/src/pages/GroupAutomationPage.test.tsx` | +220 | Render inicial, agregar palabra → POST, remover → DELETE, toggle state local, Save dispara ≥ 2 roundtrips, error path → `notifyError`, link desde detail. |

### Frontend MOD

| Archivo | LOC est. | Notas |
|---------|---------:|-------|
| `frontend/src/App.tsx` | +3 | +1 ruta `/groups/:id/automation` bajo `RequireAuth`. |
| `frontend/src/pages/GroupDetailPage.tsx` | +12 | Botón "Moderación automática" en panel "Detalle", debajo de "Membresía y moderación". |
| `frontend/src/pages/GroupDetailPage.test.tsx` | +15 | +1 test que verifica el link `<a href="/groups/:id/automation">`. |

### Docs

| Archivo | LOC est. | Notas |
|---------|---------:|-------|
| `README.md` | +60 | Sección "Moderación automática" — describe reglas, settings, listas, cómo se configuran. |

**Total**: ~3033 LOC. Excede budget por ~7.5× → `size:exception` (precedente 7/7 PRs).

---

## Interfaces / Contracts

### Go: `automation.Lists`

```go
// Lists son las dos listas que el Service pre-carga una vez por
// mensaje y propaga a las reglas via Rule.Evaluate. Si el Service
// detecta que ningun toggle dependiente esta activo, pasa nil para
// ahorrar 2 queries a la DB.
type Lists struct {
    BannedWords   []string
    LinkAllowlist []string
}
```

### Go: `Rule` interface extendida

```go
// Slice 1: Evaluate(ctx, msg, s, ws, now) *RuleHit
// Slice 2: inserta *Lists entre ws y now. FloodRule actualiza firma;
// la regla sigue stateless (ignora lists).
type Rule interface {
    Name() string
    Evaluate(ctx context.Context, msg *telegram.Message, s *Settings, ws *WarningState, lists *Lists, now time.Time) *RuleHit
}
```

### Go: helper `domainMatches`

```go
// Acepta: example.com matchea example.com y sub.example.com
// Rechaza: notexample.com (sin punto previo), example.company
func domainMatches(allow, msgHost string) bool {
    allow = strings.ToLower(strings.TrimSpace(allow))
    msgHost = strings.ToLower(strings.TrimSpace(msgHost))
    if allow == "" || msgHost == "" { return false }
    return msgHost == allow || strings.HasSuffix(msgHost, "."+allow)
}
```

### Go: 5 nuevas Action constants

```go
const (
    ActionUpdateAutomationSettings = "UPDATE_AUTOMATION_SETTINGS"
    ActionAddBannedWord            = "ADD_BANNED_WORD"
    ActionRemoveBannedWord         = "REMOVE_BANNED_WORD"
    ActionAddLinkAllowlist         = "ADD_LINK_ALLOWLIST"
    ActionRemoveLinkAllowlist      = "REMOVE_LINK_ALLOWLIST"
)
// Distinto de slice 1 (RULE_TRIGGERED/AUTOMUTE/AUTOBAN con ActorID=nil):
// estas 5 son acciones manuales del admin desde el panel.
```

### Backend REST

| Método | Path | Body | Returns |
|--------|------|------|---------|
| GET | `/api/groups/{id}/automation/settings` | — | `{settings: AutomationSettings}` (200 / 404) |
| PUT | `/api/groups/{id}/automation/settings` | `SettingsUpdate` | `{settings: AutomationSettings}` (200 / 400 / 404) |
| GET | `/api/groups/{id}/automation/banned-words` | — | `{words: []string}` (200) |
| POST | `/api/groups/{id}/automation/banned-words` | `{word: string}` | `{words: []string}` (200) |
| DELETE | `/api/groups/{id}/automation/banned-words/{word}` | — | `{words: []string}` (200) |
| GET | `/api/groups/{id}/automation/link-allowlist` | — | `{domains: []string}` (200) |
| POST | `/api/groups/{id}/automation/link-allowlist` | `{domain: string}` | `{domains: []string}` (200) |
| DELETE | `/api/groups/{id}/automation/link-allowlist/{domain}` | — | `{domains: []string}` (200) |

Auth: todas requieren `requireAuth` (panel admin). NO `permissionOkAdmin` (los handlers son admin-gated, no tocan el bot).

### Frontend hooks (queries + mutations)

| Hook | Tipo | Key |
|------|------|-----|
| `useAutomationSettings(groupId)` | query | `['automation','settings',groupId]` |
| `useBannedWords(groupId)` | query | `['automation','banned-words',groupId]` |
| `useLinkAllowlist(groupId)` | query | `['automation','link-allowlist',groupId]` |
| `useUpdateSettings(groupId)` | mutation | invalida `['automation','settings',groupId]` |
| `useAddBannedWord(groupId)` | mutation | invalida `['automation','banned-words',groupId]` |
| `useRemoveBannedWord(groupId)` | mutation | invalida `['automation','banned-words',groupId]` |
| `useAddAllowlist(groupId)` | mutation | invalida `['automation','link-allowlist',groupId]` |
| `useRemoveAllowlist(groupId)` | mutation | invalida `['automation','link-allowlist',groupId]` |

---

## Testing Strategy

| Layer | What | How |
|-------|------|-----|
| Unit (rules) | AntiSpam/AntiLink/BannedWords + Registry order + propagation de `lists` | 18+ casos tabla mensaje→hit en `rules_test.go`; fakes `*Lists`; verifica `lists` defensivo en nil. |
| Unit (service) | Pre-load de listas (ambos toggles on / ambos off / uno on) + propagation a reglas | 5+ casos con `ListsRepo` fake que cuenta invocaciones. |
| Integration (repo) | 8+ casos: add/remove/list words, mismo allowlist, FK CASCADE, idempotencia | `OpenTestDB("automation")` + `goose.Up` + TRUNCATE CASCADE; INSERT directo a `groups` para FK. |
| Handler | 200/400/404 + auth required (401) | `api.NewServer` con `WithAutomation(...)` + httptest; fake SettingsRepo + ListsRepo + LogWriter. |
| Frontend | render, save flow (Promise.all), add/remove word/allowlist, error path | `renderWithProviders` + `mockFetchRoutes` por substring URL. |

§21.1 estricto: cero llamadas Bot API reales. `automation.Service` opera contra fakes en tests; el adapter concreto se testea aparte (publications-slice3 precedent).

---

## Migration / Rollout

`goose Up` aplica 00007 (nuevo, no toca tablas previas). `goose Down` elimina `banned_words` y `link_allowlist` (orden inverso). En dev, `cfg.RunMigrations=true` aplica 00007 automáticamente al `docker compose up` (igual que 00006). En prod, paso manual documentado en `README.md` §Migraciones.

Rollback: `git revert` del merge. Migración 00007 Down elimina las 2 tablas. `cmd/server/main.go` deja de registrar las 3 reglas nuevas (registry queda con `Flood`); el pipeline sigue funcionando con la regla de slice 1. Frontend: ruta `/groups/:id/automation` queda muerta (404 al navegar; aceptable durante rollback). Audit logs preservados.

---

## Open Questions

Ninguna (resueltas en exploration #215, proposal #224, spec #225).

---

## Risk Table

| # | Risk | Likelihood | Impact | Mitigation |
|---|------|-----------|--------|------------|
| 1 | Domain matcher confunde `example.com` con `notexample.com` | Medium | Medium | Helper exige `.` antes del allow domain; tests cubren `example.com`/`sub.example.com`/`notexample.com`. |
| 2 | Race `UpsertSettings` mientras `HandleMessage` lee settings | Low | Low | UPSERT idempotente; lectura después de upsert ve estado consistente (doc en design). |
| 3 | `banned_words` duplicados al agregar múltiples veces | Low | Low | PK compuesta + `ON CONFLICT DO NOTHING`; POST devuelve 200 con lista actualizada. |
| 4 | `<TagsInput>` no acepta caracteres unicode | Low | Low | Validación cliente `^[\p{L}\p{N}_\- ]{1,100}$`; trim+lowercase antes de POST. |
| 5 | Save con muchos roundtrips secuenciales | Low | Low | Save usa `Promise.all`; tolera errores parciales (notificaciones por sección). |
| 6 | `BannedWordsRule` carga todas las palabras por mensaje | Low | Low | Pre-load una vez por mensaje vía Service; LRU cache en slice 2.5 si performance lo pide. |
| 7 | Case-sensitivity inconsistente frontend/backend | Low | Low | Backend normaliza `LOWER(word)` en INSERT; matcher lowercases texto. Frontend trim+lowercase antes de POST. |
| 8 | `permissionOk` bugfix #172 reintroducido en handlers | Low | Medium | Handlers de slice 2 son admin-gated (panel auth), no tocan el bot. `grep can_* backend/internal/automation/` debe dar 0 matches. |
| 9 | Slice excede 400 LOC | **High** | Medium | Forecast ~3033 LOC. `size:exception` solicitada (precedente 7/7 PRs aprobados). |
| 10 | Cambiar firma `Rule.Evaluate` rompe reglas externas | Low | Low | Las reglas viven solo en `automation/` (sin export fuera del paquete). No hay consumers externos. |

---

## Resolved Decisions Tracking

| Decisión | Estado |
|----------|--------|
| Schema separado vs JSONB | ✅ D1: 2 tablas paralelas |
| Case handling banned words | ✅ D2: server-side LOWER |
| Subdomain match helper | ✅ D3: `HasSuffix("."+allow)` |
| Signature extend Rule | ✅ D4: `*Lists` como argumento único |
| Registry order | ✅ D5: hardcoded en `cmd/server/main.go` |
| Frontend Save pattern | ✅ D6: single Save + `Promise.all` |
| Audit log Actor | ✅ D7: admin (≠ nil) para manuales |
| POST idempotencia | ✅ D8: PK compuesta + `ON CONFLICT DO NOTHING` |

---

## Resolves Every Spec REQ

| REQ | Cómo se resuelve |
|-----|------------------|
| REQ-7 (Schema `banned_words`) | Migración 00007 + DDL con PK compuesta, FK CASCADE, CHECK 1-100, índice. |
| REQ-8 (Schema `link_allowlist`) | Migración 00007 + DDL simétrico, CHECK 1-253, case preserved. |
| REQ-9 (AntiSpamRule) | `rules.go` AntiSpamRule con 3 sub-detectores (all-caps, repeated 5+, short URL). |
| REQ-10 (AntiLinkRule) | `rules.go` AntiLinkRule con regex + `domainMatches` helper (suffix con `.`). |
| REQ-11 (BannedWordsRule) | `rules.go` BannedWordsRule con `strings.Contains(lower(text), word)`. |
| REQ-12 (Registry order) | `cmd/server/main.go` registra en orden Flood→AntiSpam→AntiLink→BannedWords. |
| REQ-13 (Service pre-load) | `service.go` HandleMessage carga listas si `BannedWordsEnabled OR AntiLinkEnabled`. |
| REQ-14 (Rule.Evaluate extendido) | Interface modificada con `lists *Lists`; FloodRule actualizada; tests verdes. |
| REQ-15 (GET/PUT settings) | `automation_handlers.go` handleGetSettings + handlePutSettings + log ActionUpdateAutomationSettings. |
| REQ-16 (CRUD banned-words) | 3 handlers + 3 methods en repository + idempotencia PK. |
| REQ-17 (CRUD link-allowlist) | 3 handlers + 3 methods en repository + case preserved. |
| REQ-18 (Frontend GroupAutomationPage) | Página completa con 5 switches + 6 NumberInputs + 2 TagsInput + Save con Promise.all + notificaciones. |
| REQ-19 (5 Action constants) | `logs/model.go` con constantes y comentario de distinción manual vs auto. |
| REQ-20 (Tests §21.1) | 18+ unit + 5+ service + 8+ integration + handlers + frontend tests con mockFetchRoutes. |
| REQ-21 (No regresión) | `moderation/` intacto, `publications/` intacto, páginas frontend preexistentes intactas, `permissionOkAdmin` invariante, `git diff` asserts en verify. |

---

## What Comes After

Después de `sdd-design`, `sdd-tasks` debe:
1. Re-confirmar `size:exception` (forecast ~3033 LOC > 400).
2. Si prefiere chained: partir en `backend-rules+handlers` (~1100 LOC) / `frontend-page+settings` (~1930 LOC).
3. Tasks backend-first con compile gate (Phase 1 actualiza TODAS las firmas de `Rule.Evaluate` antes de agregar reglas nuevas).

**Ready for tasks.**
