# Tasks: Frontend Panel (routing + auth + dashboard)

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 600-850 (18 archivos frontend + deps) |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR1: estructura+auth+api-client → PR2: routing+layout+login → PR3: dashboard+grupos |
| Delivery strategy | ask-on-risk |
| Chain strategy | feature-branch-chain (igual que rest-api) |

Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: feature-branch-chain
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | PR | Base | Notes |
|------|------|----|------|-------|
| 1 | Deps + api-client con auth + features/auth + AuthContext | PR 1 | feat/frontend-panel | base del resto; tests api-client 401→refresh→retry |
| 2 | Router completo + RequireAuth + Layout + LoginPage + 404 | PR 2 | branch PR1 | tests rutas protegidas + login |
| 3 | DashboardPage + GroupDetailPage + placeholders + useGroups | PR 3 | branch PR2 | tests de carga/error |

## Phase 1: Dependencias e infraestructura de auth

- [x] 1.1 `frontend/package.json`: instalar `@tanstack/react-query`, `react-hook-form`, `zod` (+ tipos)
- [x] 1.2 `frontend/src/features/auth/types.ts`: `LoginResponse` (`access_token`), `MeResponse` (`id`, `username`) — contrato verificado en `auth_handlers.go`
- [x] 1.3 `frontend/src/features/auth/api.ts`: `login()`, `refresh()`, `logout()`, `me()` vía `request()`
- [x] 1.4 `frontend/src/lib/api-client.ts`: +`setAccessToken()`, manejo 401→refresh→1 retry→si falla `onUnauthorized` callback; ApiError ya existe (envelope correcto). ⚠️ **deviation**: `API_BASE_URL` por defecto pasó de `/api` a `''` — el bootstrap doblaba el prefijo (`/api/api/...`) porque los paths ya vienen con `/api`; el proxy de Vite resuelve en dev, `VITE_API_BASE_URL` opcional en prod
- [x] 1.5 `frontend/src/lib/auth-context.tsx`: `AuthProvider` + `useAuth()` — login/logout, restauración con `me()` al montar, estado `{authenticated, loading}` (⚠️ se expone `user` en vez de `authenticated`; `user===null` → sin sesión)

## Phase 2: Routing + layout + login

- [ ] 2.1 `frontend/src/App.tsx`: rutas §16 completas, root→/dashboard, 404; layout autenticado padre con `<Outlet />`
- [ ] 2.2 `frontend/src/components/RequireAuth.tsx`: sin sesión→redirect `/login` con `location.state.from`; con sesión→Outlet
- [ ] 2.3 `frontend/src/components/Layout.tsx`: sidebar (Dashboard, Grupos, Logs) + header + Outlet — CSS plano
- [ ] 2.4 `frontend/src/pages/LoginPage.tsx`: form RHF+Zod (username, password), submit→`useAuth().login()`, error legible §18, redirect post-login
- [ ] 2.5 `frontend/src/pages/NotFoundPage.tsx`: 404 con link a /dashboard

## Phase 3: Dashboard + detalle de grupo

- [ ] 3.1 `frontend/src/features/groups/types.ts`: `Group` (contrato verificado en `groups_handlers.go`: id, telegram_id, title, username, type, member_count, bot_status, bot_permissions) + `memberResponse` si aplica
- [ ] 3.2 `frontend/src/features/groups/api.ts`: `listGroups()`, `getGroup(id)`
- [ ] 3.3 `frontend/src/features/groups/hooks.ts`: `useGroups`, `useGroup` (TanStack Query)
- [ ] 3.4 `frontend/src/pages/DashboardPage.tsx`: lista grupos (nombre, username, ID, estado bot, perms), estados loading/vacío/error con reintento, `[Administrar]`→/groups/:id
- [ ] 3.5 `frontend/src/pages/GroupsPage.tsx`: reutiliza Dashboard (lista completa)
- [ ] 3.6 `frontend/src/pages/GroupDetailPage.tsx`: datos del grupo + nav a users/requests/logs
- [ ] 3.7 `frontend/src/pages/GroupUsersPage.tsx`, `GroupRequestsPage.tsx`, `GroupLogsPage.tsx`: placeholders mínimos accesibles

## Phase 4: Wiring + tests + verificación

- [ ] 4.1 `frontend/src/main.tsx`: +QueryClientProvider, +AuthProvider (orden: Auth→Query)
- [ ] 4.2 `frontend/vite.config.ts`: +`server.proxy['/api']`→`http://backend:8080` (dev; CORS)
- [ ] 4.3 `frontend/src/styles.css`: estilos layout (sidebar, cards lista grupos, estados)
- [ ] 4.4 Tests: api-client (401→refresh→retry, refresh fallido→logout), AuthContext (login/me/logout), LoginPage (validación+submit+error), RequireAuth (redirect/pasa), App (dashboard con grupos mock + 404) — Vitest+RTL, `api.ts` con `vi.mock`
- [ ] 4.5 Verificación: `npm run build` (tsc --noEmit + vite build) y `npm test` verdes; reemplazar/ajustar `App.test.tsx` bootstrap; actualizar `.env.example` si hace falta `VITE_API_BASE_URL`
- [ ] 4.6 Suite Go sigue verde (no se toca backend): `go test ./...` en backend/

## Phase 5: Cleanup / Docs

- [ ] 5.1 Actualizar `docs/frontend-react-skill.md` si las convenciones cambian (ej. `useAuth` ubicación lib/ vs features/)
- [ ] 5.2 Actualizar `openspec/changes/frontend-panel/state.yaml` (estado por fase)