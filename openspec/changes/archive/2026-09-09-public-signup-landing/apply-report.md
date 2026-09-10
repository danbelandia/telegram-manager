# Apply Report: public-signup-landing

Rama: `feat/public-signup-landing` (desde `main @ 8f0626c`). Sin push.
Modo: Standard (config `strict_tdd: false`). Delivery: single-pr (decisión cerrada).

## Trazabilidad tarea → archivos → REQs

| Tarea | Archivos | REQs cubiertos |
|-------|----------|----------------|
| T1 types auth | `frontend/src/features/auth/types.ts` | restore, happy, badge |
| T2 `signup()` API | `frontend/src/features/auth/api.ts` | happy |
| T3 contexto sesión | `frontend/src/lib/auth-context.tsx`, `auth-context.test.tsx` | restore, badge |
| T4 `PublicOnly` | `frontend/src/components/PublicOnly.tsx` (+ `.test.tsx`) | routing-raíz-sesión, signup-pública |
| T5 `LandingPage` | `frontend/src/pages/LandingPage.tsx` (+ `.test.tsx`) | landing |
| T6 `SignupPage` | `frontend/src/pages/SignupPage.tsx`, `frontend/src/lib/api-client.ts` (`CONFLICT`) | happy, degraded, 409, errors, hygiene |
| T7 Login prefill | `frontend/src/pages/LoginPage.tsx` | login-prefill |
| T8 rutas App | `frontend/src/App.tsx`, `App.test.tsx` | rutas, RequireAuth, landing |
| T9 badge tenant | `frontend/src/components/Layout.tsx`, `Layout.test.tsx` | badge |
| T10 tests signup | `frontend/src/pages/SignupPage.test.tsx` (8 tests) | happy, degraded, 409, errors, hygiene |
| T11 tests login/context/layout | `LoginPage.test.tsx` (+1), `auth-context.test.tsx`, `Layout.test.tsx` (+2), `App.test.tsx` (+2) | login-prefill, restore, badge, rutas, RequireAuth, landing |
| T12 cierre | este reporte; `git diff main --stat` sin `backend/` | — |

Cobertura de los 11 REQs (3 frontend-routing + 8 frontend-auth): todos
tienen al menos un test que los ejercita (ver tabla; T10/T11 + suites
existentes de `RequireAuth` y 404).

## Desviaciones del diseño

1. **`ApiErrorCode += 'CONFLICT'`** (`lib/api-client.ts`, +1 línea):
   el union no incluía el código 409 del backend y el `setError` por
   status de `SignupPage` necesitaba compararlo con seguridad de tipos.
   Aditivo, sin cambio de runtime.
2. **`reset(values, { keepErrors: true })` en `SignupPage`**: el
   `reset()` plano del task borraba los errores que `setError` acababa
   de marcar en el `catch` (el `finally` corre después). `keepErrors`
   preserva campo + resto del form y limpia solo el token. Sin esto,
   los tests de 409/400 fallaban (bug encontrado implementando T10).
3. **Badge con tercer fallback `'Tenant'`**: D5 pedía `slug ?? Tenant
   #id` "nunca vacío"; si ambos son null (sesión legacy sin tenant)
   se muestra `'Tenant'`. Defensa sin costo.
4. **`--reporter=basic` no existe** en vitest 5: se corrió con el
   reporter por defecto. Sin impacto.

Sin otras desviaciones: Q1=/ pública, Q2 degradación a
`/login?username=&created=1`, Q3 (badge slug/#id, auto-login cliente,
función async plana, 409 substring con fallback, reset en finally) y
todas las reglas duras se respetaron.

## Verificación

- `tsc --noEmit -p frontend`: exit 0.
- `npm test -- --run` (rama): 18 archivos ok, 112/113 tests ok.
  El único fallo es `PublicationsPage.test.tsx` (timeouts 5s bajo
  carga paralela): **pre-existente** — en `main` limpio la suite
  completa también falla 1 archivo (96/97), y aislado pasa 18/18 en
  la rama. Archivo no tocado por este cambio (no usa Layout ni auth
  nuevo). Pool de vitest intacto.
- `npm run build`: ok en 5.87s (solo warning pre-existente de chunk
  > 500 kB).
- Higiene token: patrón `bot\d{6,}:[A-Za-z0-9_-]{20,}` sin matches en
  `frontend/`; `SIGNUP_BOT_TOKEN_EXAMPLE` solo en
  `SignupPage.test.tsx`; `bot_token` solo en campo + variable local
  + `reset` (nunca contexto/storage/QueryClient/logs).
- `git diff main --stat`: solo `frontend/` (17 archivos, +723/−21).
  Cero cambios en `backend/`.
- Sin dev-servers ni procesos e2e en background (solo runs de vitest
  ya finalizados).

## Commits (11, sin push)

`8cfb483` types → `2dc54ad` signup api → `8e6b32f` contexto →
`19441a1` PublicOnly → `5ee300a` Landing → `c00667c` SignupPage →
`590b4a7` login prefill → `f4dbebf` rutas → `a9f9f16` badge →
`a7ab266` tests signup → `0fb00e1` tests login.
