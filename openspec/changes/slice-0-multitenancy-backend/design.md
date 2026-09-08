# Design: Slice 0 — Multitenancy Backend (bot-per-tenant)

## Technical Approach

Un tenant = un usuario = un bot (bot-per-tenant, tier Pro único). La migración `00009` agrega `tenant_id` a todo salvo `users` (Q3-a: scope vía join) y convierte `UNIQUE(telegram_id)` en `UNIQUE(tenant_id, telegram_id)`. Un paquete nuevo `internal/tenants` concentra modelo, repo y cifrado AES-GCM; `internal/telegram/registry.go` posee un adapter+poller+bus por tenant; auth propaga `tenant_id` en claims JWT; cada repo scopea con `tenant_id` como primer predicado del `WHERE` y los handlers responden `NOT_FOUND` ante recursos ajenos. El path legacy (`TELEGRAM_BOT_TOKEN` + `ADMIN_*`) sigue vivo como tenant `default`.

```
signup: validate → slug/username libres → getMe → BEGIN → tenants+admins → COMMIT → registry (caliente)
request: requireAuth (tenant_id>0) → handler ownership check → repo WHERE tenant_id=$1 … → adapter del tenant → log
boot: migrate → seed default → registry.BootAll (descifrar, NewAdapter, poller→bus propio por tenant)
```

## Architecture Decisions

| # | Decisión | Opción elegida | Alternativas descartadas | Rationale |
|---|----------|----------------|--------------------------|-----------|
| D1 | Modelo bot-per-tenant, 1 user = 1 tenant, tier Pro único | Cerrada por spec | Pool de bots compartidos; multi-user por tenant | Sin scheduler/cuotas en este slice; simplifica registry y signup |
| D2 | `username` UNIQUE global (Q1-a) | Mantener `UNIQUE(username)` en `admins` | UNIQUE(tenant_id, username) | Login es `username+password` sin slug; evita colisiones de identidad |
| D3 | Modo global + N pollers (Q2) | `TELEGRAM_MODE` único; un `Poller` por tenant | Modo por tenant; webhook multiplexado ya | Webhook multiplexado = futuro; polling por tenant no requiere URL pública |
| D4 | `users` sin `tenant_id`, scope vía join (Q3-a) | `EXISTS` sobre `groups`/`join_requests` del tenant | `tenant_id` en `users`; tabla `group_members` nueva | `users` es identidad Telegram global; evita falsos duplicados y nueva tabla |
| D5 | Bus por tenant vs bus con routing | **Un `events.Bus` por tenant** | Un bus global con envelope tenant-tagged | Cero cambios en `Bus`/`Update`/handlers; un pánico/handler lento no contamina otros tenants; costo = N slices triviales |
| D6 | Descifrado al boot vs por-request | **Descifrar al boot, adapter guarda token en memoria** | Descifrar por request | Por-request paga AES-GCM + lectura DB en cada llamada a Telegram; el token en memoria ya vive en el `Adapter` actual |
| D7 | Transacción signup ¿incluye `getMe`? | **No: `getMe` fuera y antes de `BEGIN`** | Todo dentro de la tx | No se retienen locks/conn DB durante una llamada de red; atomicidad intacta (nada persiste antes de `getMe`) |
| D8 | FKs hijas hacia `groups(telegram_id)` | **Re-apuntar a compuesta `(tenant_id, telegram_id)`** | FK a `groups(id)` PK | Preserva el patrón id-natural del código (rutas y Telegram usan `telegram_id`); §1 detalla orden |
| D9 | Ownership check en handlers, no middleware | Handler tras `pathID`, retorna `NOT_FOUND` | Middleware de autorización por grupo | El id del recurso solo se conoce en el handler (igual que `moderation.Service`: Grupo→Permiso→Telegram→Log); `NOT_FOUND` no revela existencia |
| D10 | `TENANT_TOKEN_ENC_KEY` opcional en `config.Load` | Solo fail-fast si presente-pero-inválida | Requerida siempre | Backward-compat: deploys legacy sin la var deben arrancar (§6) |
| D11 | Sin `can_*` en código (bugfix #172) | Capacidad = `bot_status='administrator'` | Leer `bot_permissions`/`can_*` | Invariante existente; el design no introduce ningún campo `can_*` |

## 1. DDL migración `00009` (goose SQL plano)

Orden estricto (spec §orden): `tenants` → `groups` (+backfill) → resto de tablas → FKs compuestas → `NOT NULL` → unicidad.

```sql
-- +goose Up
-- 1. tenants
CREATE TABLE tenants (
    id BIGSERIAL PRIMARY KEY,
    slug TEXT NOT NULL UNIQUE CHECK (length(slug) BETWEEN 1 AND 63),
    tier TEXT NOT NULL DEFAULT 'pro' CHECK (tier = 'pro'),
    bot_token_encrypted BYTEA,               -- NULL = tenant legacy default
    bot_username TEXT,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','degraded')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 2. groups: columna FK + backfill idempotente + unicidad compuesta
ALTER TABLE groups ADD COLUMN tenant_id BIGINT REFERENCES tenants(id);
INSERT INTO tenants (slug) VALUES ('default')
WHERE NOT EXISTS (SELECT 1 FROM tenants WHERE slug = 'default');
UPDATE groups SET tenant_id = (SELECT id FROM tenants WHERE slug = 'default')
WHERE tenant_id IS NULL;
ALTER TABLE groups ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE groups DROP CONSTRAINT groups_telegram_id_key;  -- UNIQUE(telegram_id) de 00002
ALTER TABLE groups ADD CONSTRAINT groups_tenant_telegram_unique UNIQUE (tenant_id, telegram_id);
CREATE INDEX idx_groups_tenant ON groups (tenant_id);

-- 3. resto: admins (username sigue UNIQUE global, Q1-a), join_requests,
--    warnings, logs, group_members (si existe), publications,
--    group_moderation_settings, user_warning_state, banned_words, link_allowlist.
--    Patrón por tabla (ejemplo admins):
ALTER TABLE admins ADD COLUMN tenant_id BIGINT REFERENCES tenants(id);
UPDATE admins SET tenant_id = (SELECT id FROM tenants WHERE slug = 'default')
WHERE tenant_id IS NULL;
ALTER TABLE admins ALTER COLUMN tenant_id SET NOT NULL;
CREATE INDEX idx_admins_tenant ON admins (tenant_id);
-- users: SIN tenant_id (Q3-a). Sin cambios.

-- 4. FKs que referenciaban groups(telegram_id) — inválidas tras soltar el
--    UNIQUE simple — se re-apuntan a la compuesta. Patrón por hija
--    (publications, group_moderation_settings, banned_words, link_allowlist):
ALTER TABLE publications ADD COLUMN tenant_id BIGINT;
UPDATE publications p SET tenant_id = g.tenant_id FROM groups g
WHERE p.tenant_id IS NULL AND g.telegram_id = p.telegram_id;
-- Determinista: pre-00009 telegram_id era UNIQUE, el join no ambigua.
ALTER TABLE publications DROP CONSTRAINT publications_telegram_id_fkey;
ALTER TABLE publications ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE publications ADD CONSTRAINT publications_group_fk
  FOREIGN KEY (tenant_id, telegram_id) REFERENCES groups(tenant_id, telegram_id) ON DELETE CASCADE;
-- group_moderation_settings.group_id, banned_words.group_id,
-- link_allowlist.group_id: idem (columna group_id + tenant_id → FK compuesta).
-- join_requests/warnings/logs/user_warning_state usan ids Telegram sin FK:
-- solo agregan tenant_id + backfill vía join a groups por group_id/telegram_id
-- y filas sin grupo resoluble → tenant default:
--   UPDATE logs SET tenant_id = (SELECT id FROM tenants WHERE slug='default') WHERE tenant_id IS NULL;

-- +goose Down
ALTER TABLE publications DROP CONSTRAINT publications_group_fk;
-- (reverso por tabla) … DROP COLUMN tenant_id por tabla; …
ALTER TABLE groups DROP CONSTRAINT groups_tenant_telegram_unique;
ALTER TABLE groups ADD CONSTRAINT groups_telegram_id_key UNIQUE (telegram_id);
ALTER TABLE groups DROP COLUMN tenant_id;
DROP TABLE tenants;
```

## 2. Paquete `internal/tenants/`

| Archivo | Contenido |
|---------|-----------|
| `model.go` | `type Tenant struct { ID int64; Slug, Tier string; BotTokenEncrypted []byte; BotUsername *string; Status string; CreatedAt, UpdatedAt time.Time }`; `ErrNotFound`, `ErrSlugTaken` |
| `repository.go` | `type Repository struct{ db *sql.DB }`; `Create(ctx, slug, tier, enc, botUsername) (int64, error)` (mapea `unique_violation` slug→`ErrSlugTaken`); `GetByID`, `GetBySlug`, `ListWithTokens(ctx) ([]Tenant, error)` (solo `bot_token_encrypted NOT NULL`, para el boot); `SetStatus(ctx, id, status)` |
| `crypto.go` | AES-GCM: clave = `base64(TENANT_TOKEN_ENC_KEY)` decodificado **o** string crudo, MUST resultar 32 B exactos o error fail-fast; nonce 12 B de `crypto/rand` **prepended** al ciphertext (`nonce‖ciphertext`); `Encrypt(plain []byte) ([]byte, error)`, `Decrypt(blob []byte) ([]byte, error)`; NUNCA loguear plaintext ni clave; tests con `REDACTED`/fixtures, no secretos reales |

## 3. Cambios en auth

- `Claims` suma `TenantID int64 \`json:"tenant_id,omitempty"\``; `TokenManager.issue` lo incluye en access **y** refresh.
- `Admin` suma `TenantID int64`; repo `get` selecciona `tenant_id`; `Create(ctx, username, hash, tenantID)`; `GetByID` sin cambios de firma.
- Signup transaccional (`auth.Service.Signup`, orden exacto): ① validar (vacios/password mínimo → `VALIDATION_ERROR`, sin TG ni DB) → ② slug libre? (`CONFLICT`) → ③ username libre global? (`CONFLICT`) → ④ `getMe` con adapter efímero (`NewAdapter(botToken).GetMe`; rechazo → `TELEGRAM_ERROR`/502, sin persistir) → ⑤ `BEGIN`: `INSERT tenants` → `INSERT admins(tenant_id…)` (bcrypt costo 12) → `COMMIT`; cualquier fallo → rollback total, cero filas huérfanas. Tras commit, handler cifra token (AES-GCM) vía `UPDATE` o dentro del paso ⑤ si el repo expone tx — elegido: insert con `bot_token_encrypted` ya cifrado + `bot_username` del `getMe` en el mismo `INSERT` (una sola tx).
- `requireAuth`: `TenantID == 0` (legacy) → `401` "re-login requerido"; si válido, inyecta `*Claims` (ya incluye tenant) en contexto.
- `handleMe` responde `{id, username, tenant_id}`; `handleRefresh` re-lee admin por `sub` y emite access **con el `tenant_id` actual** (refresh legacy sin tenant → access con tenant, sin re-signup); login emite ambos con tenant y setea cookie `refresh_token` igual que hoy.
- `EnsureInitialAdmin`: con tabla vacía crea tenant `default` (si falta) y el admin vinculado a él; si no vacía, no toca nada.

## 4. Registry `internal/telegram/registry.go`

```go
type tenantRuntime struct { adapter *Adapter; bus *events.Bus; cancel context.CancelFunc; status string }
type Registry struct { mu sync.Mutex; tenants map[int64]*tenantRuntime }
```

- `BootAll(ctx, list []tenants.Tenant, decrypt func([]byte)(string,error))`: por tenant con token: descifrar → `NewAdapter` (rate limiter propio ~25 req/s ya existe en `Adapter`) → `GetMe` de validación; fallo de uno NO bloquea otros (log sin token). `ErrInvalidToken` → status `degraded` + poller con **backoff exponencial** (1s→máx 5 min, acotado, re-`GetMe` periódico) en vez de `Poller.Run` fatal; resto de errores → `Poller` normal (su backoff transitorio ya existe).
- Poller por tenant: `NewPoller(adapter).Run(ctxTenant, func(up){ tenantBus.Publish(&up) })`. Modo global `TELEGRAM_MODE`: `polling` = N pollers; `webhook` sigue con adapter legacy único (multiplexado = futuro, fuera del slice).
- `RegisterHot(ctx, tenantID, tokenPlain string, bus *events.Bus)`: con `mu`, si existe lo detiene y reemplaza; levanta poller sin tocar los demás (signup 201 → activo inmediato).
- `AdapterFor(tenantID) (*Adapter, bool)` para `moderationService` por tenant (o mapa de services por tenant construido en `main`).
- `StopAll()`: cancela cada `ctxTenant`; `main` lo llama en shutdown graceful (10s como el HTTP server).
- Handlers de negocio por tenant se registran en **su propio bus** (D5): `groups.UpsertByTelegramID` con `tenant_id`, join-requests, automation subscriber — mismo código, repos scopeados + adapter del tenant.

## 5. Scoping

Patrón por repo — `tenant_id` **primer** predicado del `WHERE`, `tenantID` primer parámetro:

```go
ListByTenant(ctx, tenantID int64) ([]Group, error)
// WHERE tenant_id = $1 ORDER BY title
GetByTenant(ctx, tenantID, telegramID int64) (*Group, error)
// WHERE tenant_id = $1 AND telegram_id = $2 → ErrNotFound si ajeno
```

Aplica a `groups`, `joinrequests` (`ListByGroup(ctx, tenantID, groupID)` + `GetByID` verifica join a `groups`), `logs` (`ListByGroup`, `CountByActionAndGroup`), `publications`, `automation` settings/listas/state. Ownership check **en handlers** (D9), justo tras `pathID` y antes de llamar a Telegram: `GetByTenant` → `ErrNotFound` → `404 NOT_FOUND` (grupo ajeno indistinguible de inexistente; sin llamada a Telegram). `users` global (Q3-a), ejemplo:

```sql
SELECT u.* FROM users u WHERE u.telegram_id = $2
AND EXISTS (SELECT 1 FROM join_requests j JOIN groups g
  ON g.telegram_id = j.group_id AND g.tenant_id = $1 WHERE j.user_id = u.telegram_id)
-- sin presencia en el tenant → NOT_FOUND
```

## 6. `main.go` wiring + config + `.env.example`

- `config`: `TenantTokenEncKey string` (opcional; si non-empty valida 32 B tras base64/raw o fail-fast); `TELEGRAM_BOT_TOKEN`, `ADMIN_*` siguen **requeridos** (legacy intacto).
- `main` orden: `Load` → `Connect` → `Migrate` → `EnsureInitialAdmin` (adopta `default`) → `tenantsRepo.ListWithTokens` → `registry.BootAll` (+ adapter legacy con `TELEGRAM_BOT_TOKEN` para tenant `default` si no tiene token propio) → buses/handlers por tenant → `api.NewServer` con `authSvc` (signup/login/refresh/me), `POST /api/auth/signup` → `httpServer` + pollers; `SIGTERM` → `registry.StopAll()` + `httpServer.Shutdown`.
- `.env.example` suma (sin secretos reales): `TENANT_TOKEN_ENC_KEY=` (base64 de 32 B; requerida solo para signup de nuevos tenants) + comentario de generación (`openssl rand -base64 32`).

## Trazabilidad spec → diseño

| Spec / Requirement | Cobertura |
|--------------------|-----------|
| provisioning: signup crea tenant+admin+token | §3 orden ①–⑤ + §2 repo/crypto; 201 `{tenant,admin}`, token nunca en resp/logs |
| provisioning: atomicidad ante fallo | §3 paso ⑤ una sola tx + rollback; `getMe` fuera (§D7) |
| provisioning: slug duplicado 409 | §3 paso ② previo a TG |
| provisioning: username global 409 | §3 paso ③ + D2/Q1-a, `UNIQUE(username)` intacto |
| provisioning: token inválido 502 | §3 paso ④ `getMe` antes de persistir, sin filas |
| provisioning: validación 400 | §3 paso ①, sin TG ni DB |
| isolation: listados filtrados | §5 `ListByTenant`, `tenant_id` primer predicado |
| isolation: grupo ajeno NOT_FOUND | §5 ownership en handler, sin llamada TG |
| isolation: users vía join | §5 query `EXISTS`, `users` sin cambios (§1) |
| isolation: sin `can_*` | D11 + §5: solo `bot_status` |
| registry: poller por tenant al boot | §4 `BootAll`, fallo aislado |
| registry: caliente tras signup | §4 `RegisterHot` con mutex |
| registry: revocado backoff degraded | §4 `degraded` + backoff exponencial, log sin token |
| registry: rate limiter por instancia | §4: el existente por `Adapter` (~25/s, 429+`retry_after`, 3 reintentos) se hereda por tenant |
| auth: access con `tenant_id` 15 min | §3 Claims + TTLs intactos |
| auth: legacy sin tenant 401 | §3 `requireAuth` rechaza `TenantID==0` |
| auth: refresh 7d + re-emite con tenant | §3 `handleRefresh` re-lee admin actual |
| auth: `me` con `tenant_id` | §3 `handleMe` |
| auth: bootstrap adopta `default` | §3 `EnsureInitialAdmin`, bcrypt 12 |
| migration: tenants + columnas | §1 DDL ordenado |
| migration: backfill idempotente | §1 `WHERE NOT EXISTS` + `UPDATE … WHERE tenant_id IS NULL` |
| migration: unicidad compuesta | §1 drop `UNIQUE(telegram_id)` → `UNIQUE(tenant_id, telegram_id)`; upsert cambia a `ON CONFLICT (tenant_id, telegram_id)` |
| migration: orden | §1 tenants→groups→resto→FKs→NOT NULL |
| compat: arranque legacy | §6 config: `TENANT_TOKEN_ENC_KEY` opcional, `TELEGRAM_BOT_TOKEN`/`ADMIN_*` intactos |
| compat: sesiones legacy con gracia | §3: access legacy 401 re-login, refresh legacy re-emite con tenant |
| compat: no remover vars legacy | §6 + `.env.example` solo aditivo |

## Testing Strategy

| Capa | Qué | Cómo |
|------|-----|------|
| Unit | `crypto.go` roundtrip + clave inválida; `Claims` con tenant; `requireAuth` legacy→401; signup orden (TG mock) | Go tests, `TelegramService` mockeado (moq/mano), jamás Bot API real |
| Integration | 00009 en fresco + sobre datos legacy + re-aplicación; unicidad compuesta; FKs compuestas; `ListByTenant` no fuga | Postgres real (testdb), goose |
| E2E manual | boot 2 tenants (2 pollers), revoke uno → degraded sin afectar otro, signup → poller caliente | Tokens `REDACTED`, logs sin token |

## File Changes

| Archivo | Acción | Descripción |
|---------|--------|-------------|
| `backend/migrations/00009_multitenancy.sql` | Crear | DDL §1 |
| `backend/internal/tenants/{model,repository,crypto}.go` | Crear | §2 |
| `backend/internal/telegram/registry.go` | Crear | §4 |
| `backend/internal/{auth,groups,joinrequests,logs,users,publications,automation}/*.go` | Modificar | Claims/Admin `tenant_id`, queries scopeadas, signup, upserts compuestos |
| `backend/internal/api/{middleware_auth,auth_handlers,groups_handlers,…}.go` | Modificar | Rechazo legacy, `me`, signup, ownership `NOT_FOUND` |
| `backend/internal/config/config.go`, `backend/cmd/server/main.go`, `.env.example` | Modificar | §6 wiring |

## Open Questions

- Ninguna bloqueante; webhook multiplexado por tenant queda explícitamente fuera del slice.
