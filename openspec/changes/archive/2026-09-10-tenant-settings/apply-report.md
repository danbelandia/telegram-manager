# Apply Report: Tenant Settings

## Commits

1. `feat(tenants): add UpdateToken method to repository` — `backend/internal/tenants/repository.go`
2. `feat(api): add tenant_slug to /auth/me and tenant settings handlers` — `backend/internal/api/` (server.go, auth_handlers.go, tenant_handlers.go)
3. `feat(api): wire WithTenants in main.go for both webhook and polling` — `backend/cmd/server/main.go`
4. `feat(auth): add GetAdminByID and CheckPassword methods` — `backend/internal/auth/` (admin.go, service.go)
5. `feat(frontend): add tenant settings types, API, page, and banner` — `frontend/src/features/tenant/`, `frontend/src/pages/TenantSettingsPage.tsx`, `frontend/src/components/DegradedBanner.tsx`
6. `feat(frontend): add /tenant route and NavLink in Layout` — `frontend/src/App.tsx`, `frontend/src/components/Layout.tsx`
7. `test: add backend and frontend tests for tenant settings` — `backend/internal/api/tenant_handlers_test.go`, `frontend/src/pages/TenantSettingsPage.test.tsx`, `frontend/src/components/DegradedBanner.test.tsx`

## Files Changed

| File | Action | Description |
|------|--------|-------------|
| `backend/internal/tenants/repository.go` | Modified | Added `UpdateToken` method for token rotation |
| `backend/internal/auth/admin.go` | Modified | Added `CheckPassword` method using bcrypt |
| `backend/internal/auth/service.go` | Modified | Added `GetAdminByID` method |
| `backend/internal/api/server.go` | Modified | Added `tenantSettingsRepo`, `tenantSettingsRegistry`, `tenantSettingsCrypter` interfaces; `WithTenants` option; tenant routes |
| `backend/internal/api/auth_handlers.go` | Modified | `handleMe` now includes `tenant_slug` via repo lookup |
| `backend/internal/api/tenant_handlers.go` | **Created** | 3 handlers: `handleGetTenantMe`, `handleRotateBotToken`, `handleGetTenantStatus` |
| `backend/internal/api/tenant_handlers_test.go` | **Created** | 6 unit tests for tenant handlers + auth/me slug |
| `backend/cmd/server/main.go` | Modified | Wired `WithTenants` for both webhook and polling modes |
| `frontend/src/features/tenant/types.ts` | **Created** | `TenantMe`, `RotateTokenInput`, `TenantStatus` types |
| `frontend/src/features/tenant/api.ts` | **Created** | `getTenantMe()`, `rotateBotToken()`, `getTenantStatus()` |
| `frontend/src/pages/TenantSettingsPage.tsx` | **Created** | Settings page with tenant data + Zod rotation form |
| `frontend/src/pages/TenantSettingsPage.test.tsx` | **Created** | 7 unit tests for TenantSettingsPage |
| `frontend/src/components/DegradedBanner.tsx` | **Created** | Polling banner (60s) for disconnected/unknown bot status |
| `frontend/src/components/DegradedBanner.test.tsx` | **Created** | 3 unit tests for DegradedBanner |
| `frontend/src/components/Layout.tsx` | Modified | Added NavLink "Configuracion" + DegradedBanner |
| `frontend/src/App.tsx` | Modified | Added `/tenant` route under RequireAuth |
| `openspec/changes/tenant-settings/tasks.md` | Modified | All tasks marked [x] complete |

## Test Results

### Backend

```
go test ./... -count=1 -short  →  ALL PASS (14 packages)
go vet ./...                    →  CLEAN
```

### Frontend

```
npm test -- --run  →  122 passed, 1 failed (pre-existing timeout in PublicationsPage.test.tsx, unrelated)
npm run build      →  BUILD OK (tsc + vite)
```

The single failing test (`PublicationsPage > crea una publicacion multi-grupo con foto y botones`) is a pre-existing timeout issue in `PublicationsPage.test.tsx` that was already failing before this change. It is NOT related to tenant-settings.

## Deviations from Design

None — implementation matches design.md exactly.

- D1: `GET /tenants/me` dedicated endpoint (not reusing `/auth/me`) ✅
- D2: Polling 60s in DegradedBanner ✅
- D3: Password confirmation for rotation (bcrypt) ✅
- D4: `UpdateToken` in Repository (not inline query) ✅
- D5: DegradedBanner in Layout (single point, all routes) ✅
- D6: Zod for form validation ✅

## Issues Found

None.

## Desviaciones de las Reglas Duras

- **Sin nueva migración**: La tabla `tenants` ya tiene `bot_token_encrypted`, `bot_username`, `status`. `UpdateToken` hace `UPDATE SET ...`. ✅
- **Bugfix #172**: No se introdujeron `can_*` en código ejecutable. ✅
- **Error taxonomy §18**: Todos los errores usan `respondError` con códigos del spec (`VALIDATION_ERROR`, `UNAUTHORIZED`, `TELEGRAM_ERROR`, `NOT_FOUND`, `INTERNAL_ERROR`). ✅
- **Token nunca en logs/API**: `handleRotateBotToken` retorna `{"status": "rotated"}`, no el token. Logs solo registran `tenant_id`. ✅
- **Tests mockeando TelegramService**: Tests backend usan fakes para `tenantSettingsRepo`, `tenantSettingsRegistry`, `tenantSettingsCrypter`. Tests frontend usan `mockFetchRoutes`. ✅
- **es-AR**: Mensajes de error y UI en español argentino. ✅
- **Dark mode**: Banner usa `variant="light"` que funciona en ambos schemes. ✅
- **vitest pool vmThreads**: Configuración intacta en `vite.config.ts`. ✅
- **NUNCA tokens reales**: Tests usan tokens sintéticos ("123456:ABC", "valid-token"). ✅
- **RegisterHot reutilizado**: `handleRotateBotToken` llama `s.tenantRegistry.RegisterHot(...)`. ✅

## Estado de la Rama

- Rama: `feat/tenant-settings`
- Base: `main @ 50afd88`
- Commits: 7 (convencionales)
- Push: NO (como se pidió)
- Estado: LISTA PARA VERIFICACIÓN
