# Tasks: Sistema de Licencias y Super-Admin

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 550–750 |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 → PR 2 → PR 3 |
| Delivery strategy | ask-on-risk |
| Chain strategy | stacked-to-main |

Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Notes |
|------|------|-----------|-------|
| 1 | Backend foundation + auth + license enforcement | PR 1 | Migration, models, repo, auth claims, license service, middleware |
| 2 | Backend admin CRUD + routes wiring | PR 2 | Admin handlers, super-admin middleware, server.go routes, main.go wiring |
| 3 | Frontend admin panel + license card + tests | PR 3 | AdminTenantsPage, LicenseCard, Layout, auth context, tests |

---

## Group 1: Backend Foundation (Migration, Model, Repo)

- [x] 1.1 Create `backend/migrations/00010_license_system.sql` with UP (license columns on tenants, is_super_admin on admins, seed) and DOWN (reverse order restore). Verify `goose up` / `goose down` cycle.
- [x] 1.2 Modify `backend/internal/tenants/model.go` — add `TrialEndsAt`, `ExpiresAt`, `Plan`, `MaxGroups`, `MaxMessagesDay` to `Tenant` struct; replace `StatusActive`/`StatusDegraded` constants with `StatusTrial`/`StatusActive`/`StatusSuspended`/`StatusExpired`; add `LicenseUpdate` struct.
- [x] 1.3 Modify `backend/internal/tenants/repository.go` — update `scanTenant` to scan all new columns; update all SELECT queries (`GetByID`, `GetBySlug`, `ListWithTokens`) to include new columns; add `ListAll`, `GetLicense`, `UpdateLicense` methods.
- [x] 1.4 Modify `backend/internal/auth/admin.go` — add `IsSuperAdmin bool` and `TenantSlug string` to `Admin` struct.
- [x] 1.5 Modify `backend/internal/auth/repository.go` — update `get()` to scan `is_super_admin`; update SELECT to JOIN tenants for slug; add `HasSuperAdmin()` method.
- [x] 1.6 Modify `backend/internal/auth/tokens.go` — add `TenantSlug string` and `IsSuperAdmin bool` to `Claims` struct; update `issue()` to populate from Admin.
- [x] 1.7 Modify `backend/internal/auth/service.go` — ensure `Login` returns admin with `IsSuperAdmin` and `TenantSlug` populated via updated repo query.
- [x] 1.8 Update all references to old status constants (`StatusDegraded`) across codebase (e.g., registry/poller code).

**Verification**: `go build ./...` compiles; migration runs up and down cleanly.

---

## Group 2: Backend Logic (License Service, Admin Handlers, Middleware)

- [x] 2.1 Create `backend/internal/license/service.go` — `TenantGetter` interface, `Service` struct, `Enforce()` (auto-expire trials), `Info()` methods. Define `ErrSuspended`, `ErrExpired` errors.
- [x] 2.2 Create `backend/internal/api/middleware_license.go` — `requireLicense` method on Server. Maps `ErrSuspended` → 403 `LICENSE_SUSPENDED`, `ErrExpired` → 403 `LICENSE_EXPIRED`.
- [x] 2.3 Modify `backend/internal/api/middleware_auth.go` — add `requireSuperAdmin` method checking `claims.IsSuperAdmin`.
- [x] 2.4 Create `backend/internal/api/admin_handlers.go` — `handleAdminListTenants`, `handleAdminGetTenant`, `handleAdminUpdateTenant` with status transition validation.
- [x] 2.5 Modify `backend/internal/api/auth_handlers.go` — `handleMe` adds `is_super_admin` from claims; uses `claims.TenantSlug` with legacy DB fallback.
- [x] 2.6 Modify `backend/internal/api/tenant_handlers.go` — `handleGetTenantMe` adds license fields to response; add `handleGetTenantLicense` for `GET /api/tenants/me/license` (exempt from license check).

**Verification**: Unit tests for license service and admin handlers pass.

---

## Group 3: Backend Integration (Server Routes, Wiring)

- [x] 3.1 Modify `backend/internal/api/server.go` — add `licenseSvc`, `adminTenantsLister`, `adminTenantsGetter`, `adminTenantsUpdater` fields; add `WithLicense` and `WithAdmin` option functions; register admin routes under `requireAuth(requireSuperAdmin(...))`.
- [x] 3.2 Modify existing route registrations in `server.go` — wrap protected handlers with `requireLicense` (except `auth/*` and `/tenants/me/license`). Register `GET /api/tenants/me/license` as exempt.
- [x] 3.3 Modify `cmd/server/main.go` — create `license.NewService(tenantsRepo)`, pass to `api.WithLicense(licenseSvc)`, pass `tenantsRepo` to `api.WithAdmin(...)`.
- [x] 3.4 Verify full startup: `go build`, migrations run, server starts, `GET /api/auth/me` returns `is_super_admin` field.

**Verification**: Server starts; `GET /api/auth/me` returns `is_super_admin`; `GET /api/admin/tenants` returns 403 for non-super-admin.

---

## Group 4: Frontend (Admin Page, License Card, Sidebar, Auth)

- [x] 4.1 Modify `frontend/src/features/auth/types.ts` — add `is_super_admin` to `MeResponse`.
- [x] 4.2 Modify `frontend/src/lib/auth-context.tsx` — add `isSuperAdmin` to `SessionUser`; map `me.is_super_admin` in `doLogin` and session restore (backward compat: `?? false`).
- [x] 4.3 Modify `frontend/src/lib/api-client.ts` — add `LICENSE_SUSPENDED`, `LICENSE_EXPIRED`, `FORBIDDEN`, `INVALID_TRANSITION` to `ApiErrorCode`.
- [x] 4.4 Modify `frontend/src/features/tenant/types.ts` — add `license_status`, `plan`, `trial_ends_at`, `expires_at`, `max_groups`, `max_messages_day` to `TenantMe`.
- [x] 4.5 Create `frontend/src/features/admin/types.ts` — `AdminTenant`, `UpdateTenantLicense` interfaces.
- [x] 4.6 Create `frontend/src/features/admin/api.ts` — `adminListTenants`, `adminGetTenant`, `adminUpdateTenant` functions.
- [x] 4.7 Modify `frontend/src/features/tenant/api.ts` — add `getTenantLicense()` function and `LicenseInfo` type.
- [x] 4.8 Create `frontend/src/components/LicenseCard.tsx` — Mantine Paper with status badge, plan, trial remaining, dates, limits.
- [x] 4.9 Create `frontend/src/components/LicenseErrorBanner.tsx` — Alert shown when 403 LICENSE_SUSPENDED/EXPIRED received.
- [x] 4.10 Create `frontend/src/pages/AdminTenantsPage.tsx` — Mantine Table with status filter, loads via `adminListTenants`.
- [x] 4.11 Modify `frontend/src/pages/TenantSettingsPage.tsx` — render `<LicenseCard tenant={tenant} />`.
- [x] 4.12 Modify `frontend/src/components/Layout.tsx` — make nav items conditional on `user?.isSuperAdmin`; add `IconShield` import.
- [x] 4.13 Modify `frontend/src/App.tsx` — add route `/admin/tenants` → `<AdminTenantsPage />`.

**Verification**: `npm run build` compiles; sidebar shows Admin Panel only for super-admins; LicenseCard renders on tenant settings.

---

## Group 5: Tests

- [x] 5.1 Create `backend/internal/license/service_test.go` — mock `TenantGetter`, test: active → proceed, trial valid → proceed, trial expired → ErrExpired, suspended → ErrSuspended, expired → ErrExpired.
- [x] 5.2 Create `backend/internal/api/admin_handlers_test.go` — test list/get/update with super-admin mock; test 403 for non-super-admin; test invalid status transition → 400.
- [x] 5.3 Create `backend/internal/api/middleware_license_test.go` — test enforcement on protected routes: active passes, suspended → 403, expired → 403, auth routes exempt.
- [x] 5.4 Update `backend/internal/auth/tokens_test.go` — test `IsSuperAdmin` and `TenantSlug` in claims; test legacy token backward compatibility (missing field defaults to false).
- [x] 5.5 Verify `GET /api/tenants/me/license` returns license data for own tenant (integration-style test or manual curl).
- [x] 5.6 Verify `GET /api/admin/tenants` returns all tenants for super-admin; returns 403 for regular admin.

**Verification**: `go test ./...` passes; all spec scenarios covered.
