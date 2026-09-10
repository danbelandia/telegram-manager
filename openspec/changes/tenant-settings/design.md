# Design: Tenant Settings

## Technical Approach

Agregar endpoints de configuración del tenant (`/tenants/me`, `/tenants/me/bot-token`, `/tenants/me/status`), modificar `handleMe` para incluir `tenant_slug`, añadir polling de estado en frontend (60s), y crear `TenantSettingsPage` + `DegradedBanner`. El token se cifra con AES-GCM existente; la rotación valida password (bcrypt) → token (getMe) → cifrar → DB → `RegisterHot`.

## Architecture Decisions

### D1: Endpoint dedicado GET /tenants/me (no reusar /auth/me)

| Opción | Tradeoff | Decisión |
|--------|----------|----------|
| Agregar campos a `/auth/me` | Acopla identidad con datos de tenant; cambios futuros rompen auth | **Rechazado** |
| `GET /tenants/me` separado | Un endpoint más, pero职责 clara | **Aceptado** |

### D2: Polling 60s en frontend (no WebSocket)

| Opción | Tradeoff | Decisión |
|--------|----------|----------|
| WebSocket | Real-time, pero complejidad innecesaria para MVP | Rechazado |
| Polling 60s | Simple, suficiente para banner degradado | **Aceptado** |

### D3: Password como confirmación de rotación (no 2FA)

| Opción | Tradeoff | Decisión |
|--------|----------|----------|
| 2FA/SMS | Seguridad fuerte, overkill para MVP | Rechazado |
| Password bcrypt | Simple, requiere que el admin sepa su password | **Aceptado** |

### D4: TenantRepository con UpdateToken (no query inline)

| Opción | Tradeoff | Decisión |
|--------|----------|----------|
| Query SQL inline en handler | Rápido de escribir, viola separación | Rechazado |
| Método en Repository | Patrón consistente con el resto del repo | **Aceptado** |

### D5: DegradedBanner en Layout (no en cada página)

| Opción | Tradeoff | Decisión |
|--------|----------|----------|
| Banner por página | Duplicación, olvidos | Rechazado |
| Banner en Layout | Visible en todas las rutas autenticadas, un solo punto | **Aceptado** |

### D6: Zod para validación de formulario (no HTML native)

| Opción | Tradeoff | Decisión |
|--------|----------|----------|
| HTML required pattern | Sin runtime validation, UX pobre | Rechazado |
| Zod schema | Type-safe, mapeo de errores, consistente con el stack | **Aceptado** |

## Data Flow

```
PUT /api/tenants/me/bot-token
  │
  ├─ 1. requireAuth → claims (tenant_id)
  ├─ 2. Parse body {password, bot_token}
  ├─ 3. bcrypt.CompareHashAndPassword(password, admin.PasswordHash)
  │     └─ 401 UNAUTHORIZED si falla
  ├─ 4. telegram.NewAdapter(bot_token).GetMe()
  │     └─ 502 TELEGRAM_ERROR si falla
  ├─ 5. crypter.Encrypt(bot_token) → enc
  ├─ 6. tenantsRepo.UpdateToken(ctx, tenantID, enc, botUsername)
  ├─ 7. registry.RegisterHot(ctx, tenantID, slug, bot_token, bus)
  └─ 8. 200 {"status": "rotated"}
```

## File Changes

| Archivo | Acción | Descripción |
|---------|--------|-------------|
| `backend/internal/tenants/repository.go` | Modify | Agregar `UpdateToken(ctx, id, enc, botUsername)` |
| `backend/internal/tenants/model.go` | Modify | Sin cambios (campos ya existen) |
| `backend/internal/api/tenant_handlers.go` | **Crear** | 3 handlers: `handleGetTenantMe`, `handleRotateBotToken`, `handleGetTenantStatus` |
| `backend/internal/api/auth_handlers.go` | Modify | `handleMe`: agregar `tenant_slug` via repo lookup |
| `backend/internal/api/server.go` | Modify | Option `WithTenants(...)`, campos, rutas |
| `backend/cmd/server/main.go` | Modify | Wiring: inyectar deps en `WithTenants` |
| `frontend/src/features/tenant/types.ts` | **Crear** | `TenantMe`, `RotateTokenInput` |
| `frontend/src/features/tenant/api.ts` | **Crear** | `getTenantMe()`, `rotateBotToken()`, `getTenantStatus()` |
| `frontend/src/pages/TenantSettingsPage.tsx` | **Crear** | Page con datos + formulario rotación |
| `frontend/src/components/DegradedBanner.tsx` | **Crear** | Banner polling 60s |
| `frontend/src/components/Layout.tsx` | Modify | NavLink `/tenant` + DegradedBanner |
| `frontend/src/App.tsx` | Modify | Ruta `/tenant` bajo RequireAuth |

## Interfaces / Contracts

```go
// tenants/repository.go — nuevo método
func (r *Repository) UpdateToken(ctx context.Context, id int64, enc []byte, botUsername *string) error

// api/tenant_handlers.go — signatures
func (s *Server) handleGetTenantMe(w http.ResponseWriter, r *http.Request)
func (s *Server) handleRotateBotToken(w http.ResponseWriter, r *http.Request)
func (s *Server) handleGetTenantStatus(w http.ResponseWriter, r *http.Request)

// api/server.go — option
func WithTenants(repo *tenants.Repository, registry *telegram.Registry, crypter *tenants.Crypter) Option
```

```typescript
// frontend/src/features/tenant/types.ts
interface TenantMe { slug: string; bot_username: string | null; bot_status: string; created_at: string }
interface RotateTokenInput { password: string; bot_token: string }
```

## Testing Strategy

| Capa | Qué testear | Enfoque |
|------|-------------|---------|
| Unit | `handleRotateBotToken` (bcrypt, getMe, encrypt, RegisterHot) | Mock `telegramService`, `bcrypt.Compare`, `Crypter` |
| Unit | `handleGetTenantMe` | Mock `tenants.Repository` |
| Unit | `UpdateToken` SQL | Integration test con DB real |
| Unit | Frontend `TenantSettingsPage` | Mock API, test loading/error/success states |
| Unit | `DegradedBanner` | Mock polling, test visible/hidden |

## Migration / Rollout

Sin nueva migración. La tabla `tenants` ya tiene `bot_token_encrypted`, `bot_username`, `status`. El método `UpdateToken` hace `UPDATE SET bot_token_encrypted=$2, bot_username=$3, updated_at=now() WHERE id=$1`.

## Matriz Trazabilidad Spec → Diseño

| REQ | Spec | Diseño | Cobertura |
|-----|------|--------|-----------|
| 1. Datos y estado del tenant | `GET /tenants/me` → slug, bot_username, bot_status, created_at | `handleGetTenantMe` + `TenantRepository.GetByID` | ✅ |
| 2. Rotación de bot token | `PUT /tenants/me/bot-token` → password + token → validate → encrypt → DB → RegisterHot | `handleRotateBotToken` + flujo §Data Flow | ✅ |
| 3. Status runtime del bot | `GET /tenants/me/status` → bot_status from registry | `handleGetTenantStatus` + `registry.Status(tenantID)` | ✅ |
| 4. Token hygiene | Token never in logs/responses | `handleRotateBotToken` returns `{"status":"rotated"}`, no token field; logs only `tenant_id` | ✅ |
| 5. NavLink Tenant | Navbar link `/tenant` after Publications | `NAV_ITEMS` array + `IconSettings` | ✅ |
| 6. DegradedBanner | Banner on disconnected/unknown, polling 60s | `DegradedBanner` component + `useEffect` polling | ✅ |
| 7. TenantSettingsPage | Datos readonly + form rotación + feedback | `TenantSettingsPage` + Zod schema + `notifySuccess/Error` | ✅ |
| 8. Ruta /tenant | Under RequireAuth | `App.tsx` route inside RequireAuth wrapper | ✅ |
| 9. Polling de estado | 60s interval, feed bot_status | `DegradedBanner` polling `GET /tenants/me/status` | ✅ |

## Open Questions

Ninguno. Las 3 decisiones cerradas (Q1, Q2, Q3) resuelven los puntos abiertos del spec.
