# Tasks: Moderation Automation — Slice 2 (Anti-spam + Anti-link + Banned-words + Settings UI)

> **Change**: `moderation-automation-slice2` — Slice 2/3 de Fase 3 (AGENTS §23).
> **Mode**: hybrid (filesystem + Engram).
> **Predecessor**: slice 1 archivado (`main @ 1b10d42`); spec canónico en `openspec/specs/moderation-automation/spec.md` se **amenda** con REQ-7 a REQ-21 vía delta.
> **Authority**: bugfix `#172` (`permissionOkAdmin` usa `BotStatus == StatusAdministrator`).
> **Branch**: `feat/moderation-automation-slice2` base `main @ 1b10d42`.

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~3015 across 21 files |
| 400-line budget risk | High |
| Chained PRs recommended | No |
| Suggested split | single-pr + `size:exception` (precedente 7/7) |
| Delivery strategy | exception-ok |
| Chain strategy | size-exception |

Decision needed before apply: Yes
Chained PRs recommended: No
Chain strategy: size-exception
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Notes |
|------|------|-----------|-------|
| 1 | Slice 2 end-to-end (backend 3 reglas + handlers + frontend page) | PR 1 | single-pr size:exception; precedent 7/7 aprobados |

---

## Phase 1 — Migration + compile gate (signature change)

- [ ] **1.1** Create `backend/migrations/00007_create_moderation_lists.sql` con `banned_words` y `link_allowlist` (PK compuesta, FK CASCADE, CHECK len 1-100/253, índices por `group_id`, goose Up/Down).
- [ ] **1.2** Modify `backend/internal/automation/model.go`: agregar `Lists{BannedWords []string, LinkAllowlist []string}`, `BannedWord`/`LinkAllowlistEntry` value-types y `ErrAutomationGroupNotFound`.
- [ ] **1.3** Modify `backend/internal/automation/rules.go`: extender firma `Rule.Evaluate` con `lists *Lists` entre `ws` y `now`; actualizar `FloodRule.Evaluate` (ignora el nuevo param) y `Registry.Evaluate` (propaga `lists`).
- [ ] **1.4** Modify `backend/internal/automation/rules_test.go`: actualizar tests existentes de `FloodRule` para pasar `nil` como `lists`.
- [ ] **1.5** Modify `backend/internal/automation/service.go`: pasar `nil` a `Registry.Evaluate` (listas se pre-cargan en phase 4).
- [ ] **1.6** Modify `backend/internal/automation/service_test.go`: actualizar tests existentes por la firma extendida.
- [ ] **1.7** Verification: `cd backend && go build ./...` + `go test ./internal/automation/ -count=1` green.

**Dependencies**: ninguna (es la fase base).
**Files**: 6 MOD + 1 NEW.

## Phase 2 — Repository list methods + tests

- [ ] **2.1** Modify `backend/internal/automation/repository.go`: agregar `ListBannedWords`/`AddBannedWord`/`RemoveBannedWord` (LOWER en INSERT) y los 3 simétricos para `link_allowlist` (case preserved). PK + `ON CONFLICT DO NOTHING`.
- [ ] **2.2** Modify `backend/internal/automation/repository_test.go`: agregar 8+ integration cases con `OpenTestDB("automation")` + `goose.Up`: add/remove/list words, atomic set, FK CASCADE al borrar grupo, idempotencia POST, allowlist subdomain match.
- [ ] **2.3** Verification: `go test ./internal/automation/ -count=1` green; 8+ cases nuevos passing.

**Dependencies**: Phase 1 (modelo `Lists` definido).
**Files**: 2 MOD.

## Phase 3 — New rules implementation

- [ ] **3.1** Modify `backend/internal/automation/rules.go`: agregar `AntiSpamRule` (3 sub-detectores: all-caps >70% con len>10, repeated 5+ chars, URL <20 chars), `AntiLinkRule` (regex URL + helper `domainMatches` con suffix `.`), `BannedWordsRule` (`strings.Contains(lower(text), word)`).
- [ ] **3.2** Modify `backend/internal/automation/rules_test.go`: agregar 18+ unit cases (AntiSpam 5: all-caps hit, texto corto sin hit, repeated chars, short URL, toggle off; AntiLink 5: URL fuera, exacto skip, subdominio skip, prefijo-falso hit `notexample.com`, toggle off; BannedWords 4: case-insensitive, substring, vacía skip, toggle off; Registry order 2; Rule.Evaluate con `lists` 2).
- [ ] **3.3** Verification: `go test ./internal/automation/ -count=1` green; 18+ cases nuevos passing.

**Dependencies**: Phase 1 (firma extendida).
**Files**: 2 MOD.

## Phase 4 — Service pre-load + registry wiring

- [ ] **4.1** Modify `backend/internal/automation/service.go`: agregar `ListsRepo` interface; pre-load listas cuando `BannedWordsEnabled OR AntiLinkEnabled`; pasar `*Lists` a `Registry.Evaluate`; omitir las 2 queries si ambos toggles off.
- [ ] **4.2** Modify `backend/internal/automation/service_test.go`: agregar 5+ service cases (pre-load cuando toggles on, omisión cuando ambos off, listas compartidas entre reglas, propagation defensiva `lists == nil`).
- [ ] **4.3** Modify `backend/cmd/server/main.go`: registrar las 4 reglas en orden `Flood → AntiSpam → AntiLink → BannedWords` (registry.Register calls); asegurar `automation.NewService(...)` recibe `ListsRepo`.
- [ ] **4.4** Verification: `go test ./internal/automation/ -count=1` green; `go build ./...` green.

**Dependencies**: Phase 2 (repo methods) + Phase 3 (rules).
**Files**: 3 MOD.

## Phase 5 — Backend handlers + routes + audit logs

- [ ] **5.1** Modify `backend/internal/logs/model.go`: agregar 5 constantes (`ActionUpdateAutomationSettings`, `ActionAddBannedWord`, `ActionRemoveBannedWord`, `ActionAddLinkAllowlist`, `ActionRemoveLinkAllowlist`) con comentario distinguiendo patrón manual (slice 2, ActorID admin) vs auto (slice 1, ActorID nil).
- [ ] **5.2** Create `backend/internal/api/automation_handlers.go`: 9 handlers (GET/PUT settings, GET/POST/DELETE banned-words, GET/POST/DELETE link-allowlist) usando `requireAuth` + `actorIDFromClaims` + envelope `respond`/`respondError`. Mapeo errores §18.
- [ ] **5.3** Modify `backend/internal/api/server.go`: agregar `WithAutomation(svc settingsSvc, lists listsSvc, log logWriter) ServerOption` que monta las 9 rutas bajo `/api/groups/:id/automation/...`.
- [ ] **5.4** Modify `backend/cmd/server/main.go`: pasar `api.WithAutomation(...)` en ambos modos (webhook + polling).
- [ ] **5.5** Create `backend/internal/api/automation_handlers_test.go`: handler tests con fakes (SettingsRepo, ListsRepo, LogWriter); cubre 200/400/401/404 + auth required; verifica logs emitidos con `ActorID` correcto.
- [ ] **5.6** Verification: `go test ./... -count=1` green; handler tests passing.

**Dependencies**: Phase 2 (repo) + Phase 4 (service).
**Files**: 3 MOD + 2 NEW.

## Phase 6 — Frontend feature module

- [ ] **6.1** Create `frontend/src/features/automation/types.ts` (`AutomationSettings`, `SettingsUpdate`, `BannedWordsList`, `LinkAllowlistList`).
- [ ] **6.2** Create `frontend/src/features/automation/api.ts` (8 funciones: getSettings, putSettings, listBannedWords, addBannedWord, removeBannedWord, listLinkAllowlist, addAllowlistEntry, removeAllowlistEntry).
- [ ] **6.3** Create `frontend/src/features/automation/hooks.ts` (8 hooks React Query con `retry:false` + invalidación por prefijo).
- [ ] **6.4** Create `frontend/src/features/automation/error.ts` (`formatAutomationError`, `validateBannedWordClient` regex `^[\p{L}\p{N}_\- ]{1,100}$`, `validateAllowlistClient`, `validateThresholdsClient` con regla `autoban > automute > 0`).
- [ ] **6.5** Create `frontend/src/pages/GroupAutomationPage.tsx` con 4 secciones (Switches 1+4, NumberInputs 6, TagsInput banned, TagsInput allowlist), state local `original` para diff, Save único dispara `Promise.all` paralelo, notificaciones por sección.
- [ ] **6.6** Create `frontend/src/pages/GroupAutomationPage.test.tsx` (render inicial, add/remove word/allowlist, toggle state local, Save ≥ 2 roundtrips, error path → `notifyError`).
- [ ] **6.7** Modify `frontend/src/App.tsx`: registrar ruta `/groups/:id/automation` bajo `RequireAuth`.
- [ ] **6.8** Modify `frontend/src/pages/GroupDetailPage.tsx`: agregar botón "Moderación automática" en panel "Detalle", debajo de "Membresía y moderación".
- [ ] **6.9** Modify `frontend/src/pages/GroupDetailPage.test.tsx`: agregar test que verifica el link.
- [ ] **6.10** Verification: `cd frontend && npm test -- --run` green + `npm run build` green.

**Dependencies**: Phase 5 (handlers disponibles).
**Files**: 5 NEW + 3 MOD.

## Phase 7 — Full suite + non-regression

- [ ] **7.1** Run `cd backend && go test ./... -count=1` (must be green).
- [ ] **7.2** Run `cd backend && go vet ./...` (must be clean).
- [ ] **7.3** Run `cd backend && gofmt -l .` (must return empty).
- [ ] **7.4** Run `cd frontend && npm test -- --run` (must be green).
- [ ] **7.5** Run `cd frontend && npm run build` (must be green).
- [ ] **7.6** Non-regression: `git diff main -- backend/internal/moderation/ backend/internal/publications/ frontend/src/pages/PublicationsPage.tsx frontend/src/pages/GroupUsersPage.tsx` MUST be empty.
- [ ] **7.7** Verify `grep can_* backend/internal/automation/` returns 0 matches (bugfix #172 invariante intacta).
- [ ] **7.8** Verify §21.1: cero llamadas Bot API reales en tests (audit grep `telegram.Bot` en `_test.go` files de `automation/`).

**Dependencies**: Phase 6 completa.
**Files**: 0 (solo verificaciones).

## Phase 8 — README + commits

- [ ] **8.1** Update root `README.md` con sección "Moderación automática" describiendo las 4 reglas, settings, listas (banned-words + link-allowlist), cómo se configuran desde el panel.
- [ ] **8.2** Conventional commits per repo style (sugerido: `feat(automation): add banned_words/link_allowlist migration` → `feat(automation): extend Rule.Evaluate with Lists` → `feat(automation): add AntiSpam/AntiLink/BannedWords rules` → `feat(automation): pre-load lists in Service.HandleMessage` → `feat(api): add automation settings + lists endpoints` → `feat(automation): register all 4 rules in main.go` → `feat(frontend): add GroupAutomationPage` → `docs: README moderation automation section`).

**Dependencies**: Phase 7 verde.
**Files**: 1 MOD.

---

## Resumen de archivos (21 totales)

**NEW (8)**: `00007_create_moderation_lists.sql`, `automation_handlers.go`, `automation_handlers_test.go`, `features/automation/{types,api,hooks,error}.ts`, `pages/GroupAutomationPage.{tsx,test.tsx}` (5+3+1+2 archivos, contando el page+test como 2).

**MOD (13)**: `automation/model.go`, `automation/rules.go`, `automation/rules_test.go`, `automation/service.go`, `automation/service_test.go`, `automation/repository.go`, `automation/repository_test.go`, `api/server.go`, `logs/model.go`, `cmd/server/main.go`, `App.tsx`, `pages/GroupDetailPage.tsx`, `pages/GroupDetailPage.test.tsx`, `README.md`.

## Orden de implementación (compile-gate primero)

Phase 1 → 2 → 3 → 4 → 5 → 6 → 7 → 8. La phase 1 actualiza TODAS las firmas de `Rule.Evaluate` antes de que phase 3 agregue reglas nuevas (publications-slice3 pattern: compile gate primero, lógica después).

## Guard contract (plain-text literal, para que downstream guards matcheen)

Decision needed before apply: Yes
Chained PRs recommended: No
Chain strategy: size-exception
400-line budget risk: High
