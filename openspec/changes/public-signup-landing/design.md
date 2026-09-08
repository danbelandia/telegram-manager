# Design: public-signup-landing

## Technical Approach

Frontend-only. `/` y `/signup` públicas fuera de `RequireAuth`; `SignupPage` espeja backend con RHF + zod (patrón `LoginPage`), llama `signup()` y auto-loguea con `login()`; `SessionUser` suma tenant para el badge. Sin tocar backend.

## Architecture Decisions

| # | Opción A | Opción B | Decisión |
|---|----------|----------|----------|
| D1 | `PublicOnly` dedicado que redirige autenticados a `/dashboard` | `if (user)` inline en cada página pública | A: un wrapper reutilizable en `App.tsx` para `/`, `/signup`, `/login`; simétrico a `RequireAuth` |
| D2 | Auto-login cliente (`signup → login → /dashboard`) | Backend devuelve tokens en 201 | A (Q3): sin cambio backend; el 201 solo trae `{tenant, admin}` |
| D3 | Función `async` plana en `onSubmit` | `useMutation` (TanStack) | Plana: el flujo es secuencial único (signup→login→navigate) con `isSubmitting` de RHF; evita dependencia y respeta patrón `LoginPage` |
| D4 | 409 por substring (`slug`/`username` en mensaje, fallback genérico) | Pedir al backend código estructurado por campo | Substring (Q3): sin tocar backend; `setError` al campo matcheado, genérico si no matchea |
| D5 | Cachear `slug` en sesión + fallback `"Tenant #id"` | Solo `tenant_id` | Ambos (Q3): badge usa `tenantSlug ?? Tenant #tenantId`; `MeResponse.tenant_slug` es opcional |
| D6 | Prefill `/login?username=&created=` vía `useSearchParams` | Pasar por `location.state` | Query params (Q2): sobrevive al redirect de degradación tras auto-login fallido |

## Data Flow

```
SignupPage ──POST /api/auth/signup──→ 201 {tenant, admin}
    │ token solo en campo + variable local
    ├──OK──→ login(u,p) ──→ /me (tenant_id) ──→ /dashboard (badge)
    └──login falla──→ /login?username={u}&created=1 (aviso + prefill)
409 → setError(slug|username|genérico) · 400 → setError(campo) · 502 → ayuda @BotFather
```

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `frontend/src/App.tsx` | Modify | `/`, `/signup`, `/login` fuera de `RequireAuth`; resto intacto; `*` → 404 |
| `frontend/src/pages/LandingPage.tsx` | Create | Presentación es-AR con links a `/signup` y `/login` |
| `frontend/src/components/PublicOnly.tsx` | Create | Si `user` → `/dashboard`; si no, `children` |
| `frontend/src/pages/SignupPage.tsx` | Create | Schema + 4 campos + submit + mapeo errores + `reset` en `finally` |
| `frontend/src/pages/LoginPage.tsx` | Modify | Prefill `username`, aviso `created=1` vía `useSearchParams` |
| `frontend/src/features/auth/types.ts` | Modify | `MeResponse += tenant_id/slug?`; suma `SignupRequest/Response` |
| `frontend/src/features/auth/api.ts` | Modify | Suma `signup()` → `POST /api/auth/signup` |
| `frontend/src/lib/auth-context.tsx` | Modify | `SessionUser += tenantId/tenantSlug nullable`; pobla en `login()` y restore; expone para `Layout` |
| `frontend/src/components/Layout.tsx` | Modify | Badge en header junto al username: `slug ?? Tenant #id`, nunca vacío |
| `*.test.tsx` (Signup, Login, auth-context, Layout) | Modify | `mockFetchRoutes`; mocks `/me` con `tenant_id`; token sintético `SIGNUP_BOT_TOKEN_EXAMPLE` |

## Interfaces / Contracts

```ts
// types.ts (diff)
interface MeResponse { id: string; username: string; tenant_id: number; tenant_slug?: string }
interface SignupRequest { slug: string; username: string; password: string; bot_token: string }
interface SignupResponse { tenant: { id: number; slug: string }; admin: { id: string; username: string } }
interface SessionUser { id: string; username: string; tenantId: number | null; tenantSlug: string | null }
// api.ts
signup(body: SignupRequest): Promise<SignupResponse>
// zod espejo backend (único snippet no obvio)
slug: z.string().trim().min(1).max(63); username: trim min(1); password: min(8); bot_token: trim min(1)
```

Submit: `signup()` → `login(u,p)` → `/dashboard`; login falla → `/login?username=&created=1`; 409/400/502 → `setError`; `finally: reset({bot_token: ''})`. Token solo en campo + local (no contexto, storage, QueryClient ni logs).

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| Unit | Schema zod bloquea sin red; 409→campo correcto; badge slug vs `#id`; higiene token | Vitest + `mockFetchRoutes`, `SIGNUP_BOT_TOKEN_EXAMPLE` |
| Integration | Signup 201→auto-login→`/dashboard`; login fallido→`/login?username&created=1`; restore `/me` con `tenant_id` | `renderWithProviders` + AuthProvider real, pool `vmThreads` intacto |
| E2E | No | Fuera de alcance |

## Migration / Rollout

No migration required. Solo frontend, sin flags; backend inalterado (v7, RHF+zod, es-AR, dark mode intactos).

## Trazabilidad spec → diseño

| REQ | Diseño |
|-----|--------|
| routing: rutas panel | App.tsx: `/`, `/signup` públicas; resto bajo `RequireAuth`; `*` 404 |
| routing: RequireAuth | Solo envuelve protegidas; `/`, `/signup`, `/login` fuera; chequea sesión local |
| routing: landing pública | `LandingPage` con links `/signup` + `/login`, es-AR |
| auth: login prefill | `LoginPage` lee `?username=&created=` (Q2) |
| auth: restore sesión | `auth-context` puebla `tenantId/Slug` desde `/me` |
| auth: auto-registro | `SignupPage` valida cliente, 201→auto-login→`/dashboard` |
| auth: degradación | Login falla→`/login?username&created=1` con aviso |
| auth: 409 | Substring `slug`/`username`, fallback genérico |
| auth: 400/502 | 400→`setError(campo)`; 502→guía @BotFather con reintento |
| auth: badge | Header `Layout`: `slug ?? Tenant #id` |
| auth: higiene | Token solo campo + local; `reset` en `finally`; sintético en tests |

## Open Questions

- None (Q1–Q3 cerradas; backend `POST /api/auth/signup` se asume existente).
