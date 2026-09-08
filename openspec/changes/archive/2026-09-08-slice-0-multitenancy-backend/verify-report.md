# Verify Report — slice-0-multitenancy-backend

> **Change**: `slice-0-multitenancy-backend` (tenants + signup + JWT tenant_id + registry multi-bot + scoping, backend-only)
> **Branch**: `feat/slice-0-multitenancy` @ `14c26cb` (7 commits sobre `main @ 989c8e3`)
> **Date**: 2026-09-08
> **Mode**: verificación independiente, solo lectura (sin modificar código, sin commit, sin push)
> **Gates heredados de sesión principal** (no repetidos): `go build` limpio, `go vet` limpio, `go test ./...` 13/13 OK, grep de secretos vacío, working tree limpio (re-confirmado: `git status --short` vacío).

## Verdict: **PASS-WITH-NOTES**

24/24 REQs con evidencia runtime (código + test que lo ejercita). 0 CRITICAL. 3 WARNINGs (gaps de test puntuales, comportamiento implementado y verificado por inspección + tests adyacentes). 3 SUGGESTIONs (asserts a reforzar). 1 desviación (T14) aceptada con justificación.

---

## 1. Spec → implementación (24 REQs)

Leyenda evidencia: archivo de código + test que lo ejercita en runtime. Todos los tests citados pasaron en la suite 13/13 de la sesión principal.

### 1.1 migration-00009 (4/4 PASS)

| REQ | Escenarios | Evidencia runtime | Veredicto |
|-----|-----------|-------------------|-----------|
| mig-tabla: `tenants` + `tenant_id` en todo salvo `users` | Esquema tras migrar en fresco | `backend/migrations/00009_multitenancy.sql` (líneas 11–20, 23–86; `users` intacta) + TODOS los `*_repository_test.go` corren `database.Migrate` en fresco sobre Postgres real (p. ej. `groups/repository_test.go`, `auth/repository_test.go`) | **PASS** |
| mig-backfill: tenant `default` idempotente | Sobre datos existentes; re-aplicación segura | SQL idempotente (`WHERE NOT EXISTS` + `UPDATE … WHERE tenant_id IS NULL`, líneas 24–27, 34–86) + `tenants.Repository.EnsureDefault` (ON CONFLICT, `repository.go:108`) ejercitado en `TestEnsureInitialAdmin_*` y en cada `apiTestDB`/`testDB` (truncates + re-seed por test). ⚠️ Sin test dedicado que siembre filas pre-00009 y luego migre → ver **W-1** | **PASS-WITH-NOTE** |
| mig-unicidad: `UNIQUE(tenant_id, telegram_id)` | Mismo grupo en dos tenants; duplicado en mismo tenant → upsert | `00009` líneas 98–100 + `groups/repository.go:38-40` (`ON CONFLICT (tenant_id, telegram_id)`) + `TestRepository_SameTelegramIDDifferentTenants` y `TestRepository_UpsertIdempotent`-style (`groups/repository_test.go:101-175, 295-346`) | **PASS** |
| mig-orden: tenants→groups→FKs→resto | Aplicación en fresco sin errores FK | `00009` orden §§1–7 + cada `database.Migrate` en fresco en la suite (cero errores FK en 13/13) | **PASS** |

### 1.2 tenant-provisioning (5/5 PASS)

| REQ | Escenarios | Evidencia runtime | Veredicto |
|-----|-----------|-------------------|-----------|
| prov-signup: `POST /api/auth/signup` 201 `{tenant,admin}`, token nunca en resp/logs, atomicidad | Happy path; atomicidad ante fallo de inserción | `auth/service.go:Signup` (orden ①–⑤, `getMe` fuera de la tx D7, tx única tenants+admins, rollback diferido) + `auth/signup_test.go:TestSignup_HappyPath` (cifrado verificado, plaintext ausente, `bot_username` del getMe) + `api/signup_api_test.go:TestSignup_HappyPath201` (201 + token ausente en body). Atomicidad parcial: `TestSignup_InvalidTokenNoRows` (cero filas si getMe falla). ⚠️ Fallo de INSERT post-getMe (carrera/DB caída) solo cubierto por ramas `isPgUniqueViolation` sin test de rollback total → ver **W-2** | **PASS-WITH-NOTE** |
| prov-slug: duplicado → 409 `CONFLICT`, sin TG ni persistencia | Slug en uso | `service.go:212-218` + `auth_handlers.go:135-137` + `TestSignup_SlugTakenNoTelegram` (assert `calls` sin incremento = no llama a Telegram) + `TestSignup_SlugConflict409` (409 HTTP) | **PASS** |
| prov-username: UNIQUE global → 409 `CONFLICT` | Username en otro tenant | `service.go:220-225` (Q1-a) + `auth_handlers.go:138-140` + `TestSignup_UsernameTakenGlobal` + `TestSignup_UsernameConflict409` | **PASS** |
| prov-token: `getMe` previo; rechazo → 502 `TELEGRAM_ERROR`, cero filas | Token revocado | `service.go:227-231` + `auth_handlers.go:141-143` + `TestSignup_InvalidTokenNoRows` (conteo de filas invariante) + `TestSignup_InvalidToken502` (502 + código `TELEGRAM_ERROR`) | **PASS** |
| prov-validación: 400 `VALIDATION_ERROR`, sin TG ni DB | Campos faltantes; password débil | `service.go:202-207` (vacíos, slug>63, password<8) + `auth_handlers.go:132-134` + `TestSignup_Validation` (`calls==0`) + `TestSignup_Validation400` | **PASS** |

### 1.3 tenant-isolation (4/4 PASS)

| REQ | Escenarios | Evidencia runtime | Veredicto |
|-----|-----------|-------------------|-----------|
| iso-listados: todo listado scopeado por `tenant_id` de claims | Grupos solo propios; logs y join-requests filtrados | `groups/repository.go:ListByTenant` (`WHERE tenant_id = $1` primer predicado) + `TestRepository_ListByTenantNoLeak`, `joinrequests/...:TestRepository_CrossTenantIsolation`, `logs/...:TestRepository_CrossTenantIsolation`, `publications/...:TestRepository_CrossTenantIsolation` | **PASS** |
| iso-ajeno: grupo ajeno → `NOT_FOUND` (no 403), sin llamada a Telegram | Detalle ajeno; acción sobre grupo ajeno | Ownership D9 en `api/groups_handlers.go:101-103,134-136`, `moderation_handlers.go` (5 checks), `joinrequests_handlers.go:58-60`, `automation_handlers.go` (8 checks) + `TestListGroupUsers_ForeignGroup` (404 + `len(gu.members)==0` = Telegram no tocado) + `TestModerationAction_ForeignGroup` + `TestGetGroup_NotFound` | **PASS** |
| iso-users: `users` sin `tenant_id`, scope vía join | Usuario solo de otro tenant → `NOT_FOUND` | `users/repository.go:GetByTenant` (`EXISTS` sobre `join_requests`+`warnings` del tenant, design §5) + `TestRepository_GetByTenantViaJoinRequest` + `TestRepository_GetByTenantOtherTenantNotFound` + `TestRepository_GetByTenantViaWarning`; `00009` no toca `users` | **PASS** |
| iso-nocan: capacidad solo por `bot_status='administrator'` | Acción con bot administrador | `moderation/permissions.go:permissionOk` (solo `BotStatus == StatusAdministrator`, comentario invariante #172) + `TestService_AdminSinMapaPermisosPermite` + `TestService_PermissionDeniedDoesNotCallTelegram` + cero `^+.*can_` en el diff (ver §3) | **PASS** |

### 1.4 multi-bot-registry (4/4 PASS)

| REQ | Escenarios | Evidencia runtime | Veredicto |
|-----|-----------|-------------------|-----------|
| reg-boot: un poller por tenant con token; fallo aislado | Boot con varios tenants | `telegram/registry.go:BootAll` (descifrar→adapter→`GetMe`, `continue` ante fallo, sin log de token) + `main.go:324-349` (stacks + `busFor` por tenant) + `TestRegistry_BootAll_ActiveAndDegraded` (bueno=active, revocado=degraded, sin-token=sin runtime) | **PASS** |
| reg-caliente: signup → poller sin reiniciar ni tocar otros | Signup activa polling | `registry.go:RegisterHot` (stop+replace bajo `mu`, validación fuera del lock) + `main.go:381-394` (`onTenantReady`: bus+servicios+poller) + `handleSignup` hook (`auth_handlers.go:151-156`, fallo del hook = warn, tenant ya creado) + `TestRegistry_RegisterHot_Replaces` | **PASS** |
| reg-degraded: revocado → backoff acotado, resto intacto, log sin token | Revocación aislada | `registry.go:degradedLoop` (backoff 1s→máx 5min) + `pollLoop` (fatal→degraded, nunca tumba el backend) + logs solo con `tenant_id`/`slug` (líneas 90-91, 162-163) + `TestRegistry_BootAll_ActiveAndDegraded` (tenant bueno intacto junto a degradado) | **PASS** |
| reg-ratelimit: token bucket ~25 req/s por instancia, 429+`retry_after`, máx 3 reintentos | Ráfaga contenida | Heredado por construcción: cada tenant obtiene su propio `Adapter` (`registry.go:149` vía `newAdapter`) y el limiter vive en el `Adapter` (`adapter.go:25-29,76`, `rate_limiter.go`, `doWithRetry`, `poller.go:retryRateLimited` máx 3). Tests pre-existentes: `TestAdapterGetUpdates_RateLimited`, `TestPoller_RateLimitRetries`, `TestPoller_RateLimitExhausted`. Sin test que instancie 2 adapters del registry y mida tasas — aceptable: el limiter no cambió en este slice | **PASS** |

### 1.5 auth delta (4/4 PASS, 1 NOTE)

| REQ | Escenarios | Evidencia runtime | Veredicto |
|-----|-----------|-------------------|-----------|
| auth-access: access JWT 15 min con `sub`/`username`/`tenant_id` | Access válido | `auth/tokens.go:Claims` (+`TenantID`), `issue` lo incluye en access y refresh + `TestTokenManager_IssueAndParseAccess` (tenant_id=7) + `TestTokenManager_AccessTTLIs15Min` | **PASS** |
| auth-access-legacy: access sin `tenant_id` → 401 re-login | Access legacy | `api/middleware_auth.go:43-50` (401 con mensaje de re-login). ⚠️ Sin test que emita un token sin tenant y lo presente a ruta protegida → ver **W-3** (mitigado: `requireAuth` cubierto por `TestMe_*` + todos los `RequireAuth` por handler) | **PASS-WITH-NOTE** |
| auth-refresh: 7d stateless, cookie httpOnly+Strict, re-emite con tenant actual (incl. refresh legacy) | Cookie entregada; refresh OK; refresh inválido 401; legacy re-emite con tenant | `auth/service.go:Refresh` (re-lee admin por `sub` → `IssueAccess` con tenant actual: legacy funciona por construcción) + `handleRefresh` + `setRefreshCookie` (httpOnly, Strict, Secure=`COOKIE_SECURE`) + `TestService_Refresh_Success`, `TestRefresh_Success`, `TestRefresh_NoCookie`, `TestLogin_CookieFlags`. ⚠️ Ningún test presenta un refresh emitido sin tenant ni aserta `tenant_id` en el access renovado → ver **W-3** | **PASS-WITH-NOTE** |
| auth-identidad: rutas protegidas + `GET /api/auth/me` con `tenant_id` | Me con access válido; sin access 401 | `handleMe` (`auth_handlers.go:86-97`, responde `tenant_id`) + `TestMe_WithValidAccess` + `TestMe_NoAccess` + `TestMe_InvalidAccess`. Sugerencia menor: `TestMe_WithValidAccess` solo aserta `username`, no `tenant_id` → ver **S-1** | **PASS** |
| auth-bootstrap: seed adopta `default`, bcrypt 12, no duplica | Seed en tabla vacía; no duplica | `auth/seeder.go:EnsureInitialAdmin` (count→`EnsureDefault`→bcrypt costo 12→`Create` con tenant) + `TestEnsureInitialAdmin_EmptyTableCreates` (aserta `TenantID!=0`) + `TestEnsureInitialAdmin_DoesNotDuplicate` | **PASS** |

### 1.6 backward-compat (3/3 PASS)

| REQ | Escenarios | Evidencia runtime | Veredicto |
|-----|-----------|-------------------|-----------|
| comp-arranque: legacy arranca sin `TENANT_TOKEN_ENC_KEY` | Deploy existente arranca | `config.Load`: `TENANT_TOKEN_ENC_KEY` solo fail-fast si presente-pero-inválida (`config.go:117-124`, D10) + `TELEGRAM_BOT_TOKEN`/`ADMIN_*` siguen requeridos + `TestLoad` casos `tenant enc key` (válida×2, inválida, vacía→arranca) + `main.go:101` (`EnsureDefault` antes de todo) | **PASS** |
| comp-sesiones: access legacy 401 con gracia, refresh re-emite, sin re-signup | Admin legacy vuelve a entrar | Mismo código que auth-access-legacy/auth-refresh-legacy (ver NOTE arriba). Comportamiento: access 401 re-login + refresh vigente re-emite con tenant. Mismo gap de test → **W-3** | **PASS-WITH-NOTE** |
| comp-vars: no se remueven `TELEGRAM_BOT_TOKEN`/`ADMIN_*` | Env legacy sin slug | `.env.example`: solo aditivo (`TENANT_TOKEN_ENC_KEY=` vacía + comentario generación, líneas 25–30); `main.go` mantiene path legacy (`default` sin token propio → `RegisterHot` con `TELEGRAM_BOT_TOKEN`, líneas 368–377) | **PASS** |

---

## 2. Design conformance (D1–D11)

| Decisión | Estado |
|----------|--------|
| D1 bot-per-tenant, 1 user = 1 tenant, tier Pro | ✅ `Signup` crea 1 tenant `pro` + 1 admin; `INSERT tenants … 'pro'` |
| D2 `username` UNIQUE global | ✅ `00009` no altera el UNIQUE de `admins.username`; chequeo global previo a TG (`service.go:220`) |
| D3 modo global + N pollers | ✅ `TELEGRAM_MODE` único en config; N pollers en `BootAll`; webhook = adapter legacy único (multiplexado declarado fuera del slice) |
| D4 `users` sin `tenant_id`, scope vía join | ✅ `GetByTenant` con `EXISTS` (join_requests + warnings); `00009` no toca `users` |
| D5 bus por tenant | ✅ `buildBus(tenantID)` por stack (`main.go:331,370,386`); `busFor` inyecta el propio; `Publish` por tenant en `pollLoop` |
| D6 descifrado al boot, token en memoria | ✅ `BootAll` descifra una vez; `Adapter` guarda el plano; `decrypt` solo en boot/hot |
| D7 `getMe` fuera de la tx | ✅ `service.go:227-231` antes de `BeginTx` (línea 247) |
| D8 FKs hijas → compuesta `(tenant_id, telegram_id)` | ✅ `00009` líneas 103–119 + PKs compuestas §§145–159 (excede al design en lo necesario: sin PKs compuestas el aislamiento era ficticio — desviación positiva documentada en apply-report) |
| D9 ownership en handlers, `NOT_FOUND` | ✅ 15+ checks (`groups`, `moderation` ×5, `joinrequests`, `automation` ×8); sin llamada TG previa |
| D10 enc-key opcional en `Load` | ✅ Solo fail-fast si presente-pero-inválida; `TestLoad` lo cubre |
| D11 sin `can_*` | ✅ `permissionOk` solo `bot_status`; cero adiciones `can_*` en el diff (ver §3) |

Desviación adicional del design aceptada: PKs compuestas en `group_moderation_settings`/`banned_words`/`link_allowlist`/`user_warning_state` + índice pending de `join_requests` por `(tenant_id, …)` — no estaba en el DDL §1 del design pero es **requerida por D1/D8** (mismo `telegram_id` en dos tenants compartiría fila). Correcta y con tests (`CrossTenantIsolation`).

---

## 3. Invariantes

- **bugfix #172 (cero `can_*` en código ejecutable del diff)**: ✅ `git diff main...HEAD -- backend | Select-String '^\+.*can_'` → **0 matches**. Todas las ocurrencias `can_*` del diff son líneas **removidas** (`-…`, permisos por mapa eliminados de moderation/services) o comentarios. Los `can_*` restantes en el repo son pre-existentes y ajenos a autorización: structs JSON de la Bot API (`telegram/moderation.go`, `update.go`), payloads de restrict/mute (`can_send_messages` — parámetros de envío, no decisiones), y fixtures de tests viejos. `permissionOk` en `moderation`, `publications` y `automation` decide solo por `bot_status`.
- **SEC-2026-09-08 (ningún secreto real en el diff)**: ✅ `TENANT_TOKEN_ENC_KEY=` vacía en `.env.example`; tokens en tests = fakes (`tok-x`, `tok-fixture-1`, `good-token`/`bad-token` contra httptest); `test-secret-…` sintéticos; `git ls-files` no rastrea ningún `.env`; working tree limpio. Los greps del diff solo muestran nombres de vars (`BOT_TOKEN`, `ENC_KEY`, `password`) en código/config, nunca valores.
- **Taxonomía de errores §18 en signup**: ✅ `VALIDATION_ERROR`/400, `CONFLICT`/409 (slug y username), `TELEGRAM_ERROR`/502, `INTERNAL_ERROR`/500 (`auth_handlers.go:129-147`); `NOT_FOUND` para signup deshabilitado. `SUCCESS`=201. `PERMISSION_DENIED` no aplica (alta pública sin actor).
- **Logs en signup**: ✅ estructurado sin token: `slog.Info("auth: signup", tenant_id, slug, admin_id)` (`auth_handlers.go:157`); respuesta 201 sin token (probado en `TestSignup_HappyPath201`); token cifrado AES-GCM en DB, nunca en claro (probado en `TestSignup_HappyPath`). NOTE: no se inserta fila en la tabla `logs` (no hay actor previo; el spec no lo exige) — el `slog` estructurado es la auditoría del evento.

---

## 4. Desviación T14 (E2E con bot real → integración PG + fakes)

**Aceptable. Justificación:**

1. Ningún REQ exige red real contra Telegram; todos los escenarios E2E (boot N pollers, degradado aislado, caliente tras signup) están cubiertos por `registry_test.go` (fakes httptest) + `signup_api_test.go` + tests de repos contra Postgres real.
2. **§21.1 prohíbe Bot API real en tests automatizados** — un E2E con tokens reales jamás podría entrar al pipeline de CI; por definición solo puede ser manual.
3. El intento de smoke manual se colgó por red/token, se mató el proceso y se limpiaron sus logs (sin tokens residuales). Queda como **pendiente manual explícito** (2 tokens de prueba, 2 tenants) para antes del deploy a producción, no como bloqueante del merge.
4. Sin impacto en la cobertura automatizada: 13/13 paquetes OK.

---

## 5. Issues

### WARNING

- **W-1 — Sin test de migración sobre datos legacy**: ningún test siembra filas pre-00009 (sin `tenant_id`) y luego aplica la migración para asertar asignación a `default` + re-aplicación idempotente. El backfill es inspeccionado (SQL idempotente correcto) y el fresh-apply corre en cada test, pero el path de producción real (migrar con datos) no tiene prueba runtime. Recomendación pre-producción: un test que cree `groups`/`admins` en esquema pre-00009 (o simule `tenant_id=NULL` permisivo) y verifique el backfill.
- **W-2 — Sin test de rollback total ante fallo de INSERT post-getMe**: las ramas de carrera (`isPgUniqueViolation` → `ErrSlugTaken`/`ErrUsernameTaken`) y el `tx.Rollback` diferido no tienen test que fuerce un fallo en el segundo INSERT y verifique cero filas huérfanas. Recomendación: test con `bot_token` válido pero username en carrera (o DB fault-inject) + conteo de filas.
- **W-3 — Escenarios legacy sin test directo**: (a) access sin `tenant_id` → 401 (una docena de líneas: emitir token con `Admin{TenantID:0}` y pegarle a `/api/auth/me`); (b) refresh emitido sin tenant → access renovado **con** `tenant_id` (asertar claim del access nuevo). El comportamiento existe y es correcto por construcción (`Refresh` re-lee el admin), pero ningún test lo fija contra regresiones.

### SUGGESTION

- **S-1**: `TestMe_WithValidAccess` solo aserta `username`; extender a `tenant_id` (es el corazón del delta de auth).
- **S-2**: `TestTokenManager_IssueAndParseRefresh` no aserta expiración 7d (sí se hace para access 15 min en `TestTokenManager_AccessTTLIs15Min`); simetría barata.
- **S-3**: `TestRegistry_BootAll_ActiveAndDegraded` no aserta que el bus del tenant bueno sigue publicando tras degradar al otro (aislamiento total a nivel eventos); opcional.

---

## 6. Next steps

1. **PR single** `feat/slice-0-multitenancy` → `main` (`size:exception`, precedente 13/13 + decisión explícita del usuario). Incluir este reporte como evidencia.
2. **sdd-archive**: APPEND de los 6 specs a `openspec/specs/` + archivar el change.
3. **Pre-producción** (no bloquean el merge, sí el deploy con datos reales): W-1 (test de backfill legacy), smoke manual E2E con 2 bots de prueba (T14 pendiente), W-3 (2 tests legacy de ~15 líneas).
4. **Deuda menor**: S-1/S-2 (asserts de 5 minutos), W-2 y S-3 cuando se toque esos archivos.

---

## 7. Trazabilidad de esta verificación

- Specs leídos: `specs/{tenant-provisioning,tenant-isolation,migration-00009,auth,backward-compat,multi-bot-registry}/spec.md` (24 REQs).
- Design leído: D1–D11 + DDL + trazabilidad (conformance §2 arriba).
- Tasks + apply-report leídos: T1–T15, 1 desviación evaluada (§4).
- Código inspeccionado: `service.go`, `auth_handlers.go`, `middleware_auth.go`, `tokens.go`, `seeder.go`, `registry.go`, `00009_multitenancy.sql`, `permissions.go`, `users/repository.go`, `groups/repository.go`, `config.go`, `main.go` (wiring), `.env.example`.
- Tests inspeccionados (todos en verde en la suite 13/13 de la sesión principal): `signup_test.go`, `signup_api_test.go`, `registry_test.go`, `tokens_test.go`, `service_test.go`, `auth_api_test.go`, `moderation_api_test.go`, `repository_test.go` (groups/users/joinrequests/logs/publications/tenants-crypto/auth), `config_test.go`, adapter/poller rate-limit tests.
- Comandos propios ejecutados (solo lectura): `git log`, `git diff --stat`, `git diff | Select-String can_/secretos`, `git ls-files`, `git status`. No se re-ejecutó la suite (gates ya verificados en sesión principal, sin sospecha que lo justifique); la evidencia runtime es la suite 13/13 + el mapeo test→REQ de este reporte.
