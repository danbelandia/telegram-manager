# Tasks: Tenant Settings

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 300–400 |
| 400-line budget risk | Medium |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 (backend) → PR 2 (frontend) → PR 3 (tests) |
| Delivery strategy | ask-on-risk |
| Chain strategy | stacked-to-main |

Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: Medium

### Suggested Work Units

| Unit | Goal | Likely PR | Notes |
|------|------|-----------|-------|
| 1 | Backend repo + handlers + wiring | PR 1 | base: main |
| 2 | Frontend types + API + page + banner | PR 2 | base: PR 1 |
| 3 | Unit tests backend + frontend | PR 3 | base: PR 2 |

## Phase 1: Backend Repository & Auth Fix

- [x] 1.1 Add `UpdateToken(ctx, id, enc, botUsername)` to `backend/internal/tenants/repository.go`. **REQs**: 2. **Done**: compiles, test verifies UPDATE.
- [x] 1.2 Modify `handleMe` in `backend/internal/api/auth_handlers.go` — add `tenant_slug`. **REQs**: 1. **Done**: `/auth/me` returns `tenant_slug`; 401 without token.

## Phase 2: Backend Handlers & Wiring

- [x] 2.1 Create `backend/internal/api/tenant_handlers.go` — `handleGetTenantMe`: repo lookup, return fields, no token. **REQs**: 1, 4. **Done**: 200 correct fields; 404 missing tenant; no token.
- [x] 2.2 Add `handleRotateBotToken` — bcrypt, getMe, encrypt, UpdateToken, RegisterHot. **REQs**: 2, 4. **Done**: 200 rotated; 401 bad pw; 502 invalid token; 400 missing fields; no token in logs.
- [x] 2.3 Add `handleGetTenantStatus` — registry status check. **REQs**: 3. **Done**: returns bot_status enum.
- [x] 2.4 Add `WithTenants` option to `backend/internal/api/server.go`, register routes. **REQs**: 1, 2, 3. **Done**: routes reachable.
- [x] 2.5 Wire deps in `backend/cmd/server/main.go`. **REQs**: 2. **Done**: backend starts, routes respond.

## Phase 3: Frontend Types & API

- [x] 3.1 Create `frontend/src/features/tenant/types.ts` — `TenantMe`, `RotateTokenInput`. **REQs**: 7. **Done**: types compile, match backend.
- [x] 3.2 Create `frontend/src/features/tenant/api.ts` — three API functions. **REQs**: 7. **Done**: typed, error handling.

## Phase 4: Frontend Page & Banner

- [x] 4.1 Create `frontend/src/pages/TenantSettingsPage.tsx` — data + Zod form + feedback. **REQs**: 7. **Done**: loads, submits, toasts.
- [x] 4.2 Create `frontend/src/components/DegradedBanner.tsx` — poll 60s, alert on degraded. **REQs**: 6, 9. **Done**: visible degraded, hidden connected.
- [x] 4.3 Modify `frontend/src/components/Layout.tsx` — NavLink + banner. **REQs**: 5, 6. **Done**: link active, banner renders.

## Phase 5: Frontend Routing

- [x] 5.1 Modify `frontend/src/App.tsx` — `/tenant` under RequireAuth. **REQs**: 8. **Done**: auth shows page; unauth redirects.

## Phase 6: Testing

- [x] 6.1 Unit tests for `handleRotateBotToken` — mock deps. **REQs**: 2, 4. **Done**: 4 scenarios pass.
- [x] 6.2 Unit tests for `handleGetTenantMe` + `handleGetTenantStatus`. **REQs**: 1, 3. **Done**: success + not-found pass.
- [x] 6.3 Unit tests for `TenantSettingsPage` + `DegradedBanner`. **REQs**: 6, 7, 9. **Done**: all states render.
