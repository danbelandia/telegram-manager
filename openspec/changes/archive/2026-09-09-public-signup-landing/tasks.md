# Tasks: public-signup-landing

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 350–450 |
| 400-line budget risk | Medium |
| Chained PRs recommended | No |
| Suggested split | Single PR (rama `feat/public-signup-landing`, se crea en apply) |
| Delivery strategy | single-pr |
| Chain strategy | pending |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: pending
400-line budget risk: Medium

### Suggested Work Units

| Unit | Goal | Likely PR | Notes |
|------|------|-----------|-------|
| 1 | Signup público + routing + badge + tests | PR 1 | Base `main`; tests/docs incluidos; solo frontend |

## Phase 1: Foundation (types/API/context)

- [x] T1 types auth — `frontend/src/features/auth/types.ts`: `MeResponse+=tenant_id/slug?`, `SignupRequest/Response`, `SessionUser+=tenantId/Slug`. REQs: restore, happy, badge. Done: `npx tsc --noEmit -p frontend`.
- [x] T2 `signup()` API — `frontend/src/features/auth/api.ts`: `POST /api/auth/signup`. REQ: happy. Done: `npx vitest run frontend/src/features/auth --reporter=basic`.
- [x] T3 contexto sesión — `frontend/src/lib/auth-context.tsx`: poblar `tenantId/Slug` en `login()` y restore `/me`. REQs: restore, badge. Done: `npx vitest run frontend/src/lib/auth-context --reporter=basic`.

## Phase 2: Core (pages)

- [x] T4 `PublicOnly` — crear `frontend/src/components/PublicOnly.tsx` (D1): con `user`→`/dashboard`. REQs: routing-raíz-sesión, signup-pública. Done: `npx vitest run frontend/src/components/PublicOnly --reporter=basic`.
- [x] T5 `LandingPage` — crear `frontend/src/pages/LandingPage.tsx` es-AR con links `/signup`, `/login`. REQ: landing. Done: render muestra ambos links (`vitest run frontend/src/pages/LandingPage`).
- [x] T6 `SignupPage` — crear `frontend/src/pages/SignupPage.tsx` (D2–D4): RHF+zod espejo backend, `signup→login→/dashboard`, 409 substring, 400/502 `setError`, `reset({bot_token:''})` en `finally`. REQs: happy, degraded, 409, errors, hygiene. Done: `npx vitest run frontend/src/pages/SignupPage --reporter=basic`.
- [x] T7 `LoginPage` prefill — modificar `frontend/src/pages/LoginPage.tsx` (D6): `useSearchParams ?username=&created=` aviso "Cuenta creada, iniciá sesión". REQ: login-prefill. Done: `npx vitest run frontend/src/pages/LoginPage --reporter=basic`.

## Phase 3: Integration (routes/badge)

- [x] T8 rutas `App.tsx` — modificar `frontend/src/App.tsx`: `/`, `/signup`, `/login` públicas fuera `RequireAuth`; resto protegidas; `*`→404. REQs: rutas, RequireAuth, landing. Done: `npx vitest run frontend/src/App --reporter=basic`.
- [x] T9 badge tenant — modificar `frontend/src/components/Layout.tsx` (D5): `tenantSlug ?? Tenant #tenantId`, nunca vacío. REQ: badge. Done: `npx vitest run frontend/src/components/Layout --reporter=basic`.

## Phase 4: Testing/verification

- [x] T10 tests signup — `*.test.tsx` signup: schema bloquea sin red, 201→auto-login→`/dashboard`, login-falla→`/login?username&created=1`, 409→campo, higiene token. Token solo `SIGNUP_BOT_TOKEN_EXAMPLE`. REQs: happy, degraded, 409, errors, hygiene. Done: `npx vitest run frontend/src/pages/SignupPage --reporter=basic`.
- [x] T11 tests login/context/layout — prefill+aviso, restore `/me` con `tenant_id`, badge slug vs `#7`. REQs: login-prefill, restore, badge, rutas, RequireAuth, landing. Done: `npx vitest run frontend --reporter=basic`.
- [x] T12 cierre — releer 11 REQs vs T1–T11; prohibido backend, pool vitest y tokens reales. Done: `rg -i "bot[0-9]{6,}:[A-Za-z0-9_-]{20,}" frontend || true` vacío + `npx tsc --noEmit -p frontend`.
