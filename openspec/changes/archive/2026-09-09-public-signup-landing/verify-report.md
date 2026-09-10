# Verify Report — public-signup-landing (Slice 1 Frontend SaaS)

> **Change**: `public-signup-landing` (landing + signup + tenant-aware frontend)
> **Branch**: `feat/public-signup-landing` base `main @ 8f0626c`
> **Date**: 2026-09-09
> **Delivered as**: single-PR `size:exception` (precedente 14/14)

## Verdict: **PASS**

- 0 CRITICAL, 0 WARNING (1 pre-existente documentado)
- 11/11 REQs con evidencia runtime
- D1–D6 conformes, 4 desviaciones aceptadas
- Token hygiene verificada (grep vacío sobre el diff)

## Build / Test evidence

```
tsc --noEmit      → exit 0 (sin errores)
npm run build     → OK (warning chunk size pre-existente)
vitest (full)     → 112/113 (1 flake pre-existente: PublicationsPage timeout bajo carga paralela)
vitest (aislado)  → 18/18 PublicationsPage.test.tsx
git diff main -- backend/ → vacío (sin cambios backend)
```

## REQ traceability (11 REQs)

| REQ | Evidence | Verdict |
|-----|----------|---------|
| public-landing (routes: 3 modified/1 added) | `LandingPage.tsx` renderiza hero + links `/signup` + `/login`; `PublicOnly.tsx` redirige autenticados a `/dashboard`; `App.tsx` desanida `/` de `RequireAuth` | ✅ |
| signup-happy | `SignupPage.tsx` 201 → `signup()` → `login()` → `navigate('/dashboard')`; badge `slug ?? 'Tenant #id'` visible en Layout | ✅ |
| signup-degraded | `SignupPage.tsx` catch → `navigate('/login', { state: { created, username } })`; `LoginPage.tsx` lee `state.created` y `state.username`, muestra aviso + prefill | ✅ |
| signup-409 | `SignupPage.tsx` catch `CONFLICT` → `message.includes('slug')` → error en campo slug; fallback genérico "Ya registrado" | ✅ |
| signup-400/502 | `SignupPage.tsx` catch `VALIDATION_ERROR` → error junto a cada campo; `TELEGRAM_ERROR` → mensaje comprensible con guía @BotFather | ✅ |
| badge-tenant | `Layout.tsx` `tenantSlug ?? 'Tenant #' + tenantId ?? 'Tenant'`; nunca vacío; `auth-context.tsx` `restore()` popula desde `/me` | ✅ |
| token-hygiene | `SignupPage.tsx` bot_token en campo controlado + `try/finally { reset() }`; grep del diff frontend por `TELEGRAM_BOT_TOKEN=` y token literal → 0 matches; QueryClient/cache/storage no retienen el valor | ✅ |
| session-auth-delta | `types.ts` `MeResponse.tenant_id: number`; `auth-context.tsx` `SessionUser.tenantId/tenantSlug`, poblados en `login()` + `restore()`; `me()` incluye tenant | ✅ |
| login-prefill | `LoginPage.tsx` lee `location.state?.username` vía `useLocation().state`, pre-rellena `setValue('username', ...)` al montar | ✅ |
| restore-tenant | `auth-context.tsx` `restore()` llama `me()` que devuelve `tenant_id`, lo setea en `session.user.tenantId`; sin tenant_slug hasta que backend lo exponga | ✅ |
| routing-public | `App.tsx` `/` → `PublicOnly <LandingPage>`, `/signup` → `PublicOnly <SignupPage>`, `/login` → `PublicOnly <LoginPage>`, `*` → `NotFound`; fuera de `RequireAuth` | ✅ |

## Design conformance (D1–D6)

| Decision | Topic | Status |
|----------|-------|--------|
| D1 PublicOnly | Wrapper que redirige autenticados a `/dashboard`, `/` y `/signup` como componente | ✅ |
| D2 Auto-login cliente | `signup()` → `login()` (`POST login` + `GET me`) → `navigate('/dashboard')` | ✅ |
| D3 Async plana | Función async en handler submit, NO `useMutation`; `reset()` en `finally` | ✅ |
| D4 409 substring | `message.includes('slug')` → error slug; fallback genérico | ✅ |
| D5 Slug + fallback | `tenantSlug ?? 'Tenant #' + tenantId ?? 'Tenant'` en Layout | ✅ |
| D6 Prefill query | `location.state?.username` + `location.state?.created` en LoginPage | ✅ |

## Deviations

| # | Deviation | Acceptable? |
|---|-----------|-------------|
| 1 | `ApiErrorCode += 'CONFLICT'` (+1 línea en union type) | ✅ aditivo, type-safety |
| 2 | `reset(values, { keepErrors: true })` en vez de `reset()` plano | ✅ bug real found by tests |
| 3 | Tercer fallback `'Tenant'` si slug e id son null | ✅ belt-and-suspenders |
| 4 | Reporter por defecto (vitest 5 no soporta `--reporter=basic`) | ✅ cosmético |

## Invariantes

- **Token hygiene**: grep del diff frontend por `TELEGRAM_BOT_TOKEN=`, `8643598485`, `AAEuI0SEBHk1vpPNAYt70HWiOfQw9T4o` → 0 matches. Solo `SIGNUP_BOT_TOKEN_EXAMPLE` en tests.
- **Backend intacto**: `git diff main -- backend/` → vacío.
- **Pool vitest**: no modificado, pool `vmThreads` con `isolate: true` sigue activo.
- **es-AR**: todos los textos de UI en español rioplatense.
- **Dark mode**: componentes Mantine estándar (se hereda el tema).
- **Flake pre-existente**: `PublicationsPage > crea una publicacion multi-grupo con foto y botones` — 5048ms vs 5000ms. Aislado: 18/18. Documentado y verificado en main limpio.

## Issues

### CRITICAL
None.

### WARNING
None.

### SUGGESTION
- **S1 (pre-existente)**: PublicationsPage flaky test (timeout bajo carga paralela). Documentado desde slice 2 de publications. Solución: bump timeout a 10000ms en ese test específico, o migrar a `pool: 'vmThreads'` en vitest config.
- **S2 (slice 2)**: Proponer `tenant_slug` en `GET /api/auth/me` del backend para que el Layout muestre el slug siempre (no solo post-signup). Gap documentado en apply-report y design.

## Next steps
1. PR single `feat/public-signup-landing` → `main` (size:exception).
2. sdd-archive: APPEND delta specs a `openspec/specs/` + mover change a archive.
