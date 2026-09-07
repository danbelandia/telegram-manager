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

- [x] 2.1 `frontend/src/App.tsx`: rutas §16 completas, root→/dashboard, 404; layout autenticado padre con `<Outlet />`
- [x] 2.2 `frontend/src/components/RequireAuth.tsx`: sin sesión→redirect `/login` con `location.state.from`; con sesión→Outlet
- [x] 2.3 `frontend/src/components/Layout.tsx`: sidebar (Dashboard, Grupos, Logs) + header + Outlet — CSS plano
- [x] 2.4 `frontend/src/pages/LoginPage.tsx`: form RHF+Zod (username, password), submit→`useAuth().login()`, error legible §18, redirect post-login
- [x] 2.5 `frontend/src/pages/NotFoundPage.tsx`: 404 con link a /dashboard

⚠️ **deviation (tests)**: se agregó `frontend/src/test/helpers.tsx` con `mockFetchRoutes` — mock de fetch **por URL** en vez de mocks por orden (los `mockResolvedValueOnce` se desalinean porque el api-client refresca automáticamente ante 401; el refresh consume respuestas destinadas a otras llamadas). ⚠️ **deviation (setup)**: `test/setup.ts` agrega `afterEach(cleanup)` de RTL — sin `globals:true`, RTL no auto-limpia el DOM entre tests y acumula árboles ("multiple elements"). ⚠️ **deviation**: `pages/GroupUsersPage|GroupRequestsPage|GroupLogsPage.tsx` y stubs de `GroupsPage`/`GroupDetailPage` se crearon en PR2 como placeholders accesibles (antes de su fase), para que las rutas §16 resuelvan y los tests de rutas pasen; el contenido real llega en PR3.

## Phase 3: Dashboard + detalle de grupo

- [x] 3.1 `frontend/src/features/groups/types.ts`: `Group` (contrato verificado en `groups_handlers.go`). ⚠️ **deviation**: `member_count` es `number | null` (el backend lo serializa nullable) y `bot_permissions` es `Record<string, boolean>` (mapa, no array)
- [x] 3.2 `frontend/src/features/groups/api.ts`: `listGroups()`, `getGroup(id)` — `getGroup` recibe el ID de Telegram (el backend busca por `telegram_id`)
- [x] 3.3 `frontend/src/features/groups/hooks.ts`: `useGroups`, `useGroup` (TanStack Query, `retry: false` para que el UI ofrezca reintento manual)
- [x] 3.4 `frontend/src/pages/DashboardPage.tsx`: lista grupos (nombre, username, ID, miembros, estado bot, permisos activos), estados loading/vacío/error con reintento, `[Administrar]`→/groups/:id — tests en `DashboardPage.test.tsx` (lista/vacío/error+retry)
- [x] 3.5 `frontend/src/pages/GroupsPage.tsx`: reutiliza Dashboard (lista completa)
- [x] 3.6 `frontend/src/pages/GroupDetailPage.tsx`: datos del grupo (useGroup) + nav a users/requests/logs, estados loading/error/vacío
- [x] 3.7 `frontend/src/pages/GroupUsersPage.tsx`, `GroupRequestsPage.tsx`, `GroupLogsPage.tsx`: placeholders mínimos accesibles con link de vuelta al grupo

## Phase 4: Wiring + tests + verificación

- [x] 4.1 `frontend/src/main.tsx`: +QueryClientProvider (retry false, staleTime 30s), +AuthProvider (orden: Query→Router→Auth), import de styles.css
- [x] 4.2 `frontend/vite.config.ts`: +`server.proxy['/api']`→`http://backend:8080` (configurable con `VITE_PROXY_TARGET` para dev sin Docker)
- [x] 4.3 `frontend/src/styles.css`: creado — layout sidebar, login, cards de grupos, detalle, 404, botones, estados (CSS plano, D6)
- [x] 4.4 Tests: DashboardPage (carga/vacío/error+retry), AuthContext (login/me/logout), LoginPage (validación+submit+error), RequireAuth (redirect/pasa), App (login/dashboard/404) — Vitest+RTL, helpers `mockFetchRoutes` por URL
- [x] 4.5 Verificación: `npm run build` OK, `npm test` 22/22; `App.test.tsx` reescrito con QueryClientProvider + fetch por ruta; `.env.example`: `VITE_API_BASE_URL` comentado (opcional; proxy por defecto)
- [x] 4.6 Suite Go sigue verde (no se toca backend): `go test ./...` OK en backend/

## Phase 5: Cleanup / Docs

- [ ] 5.1 Actualizar `docs/frontend-react-skill.md` si las convenciones cambian (ej. `useAuth` ubicación lib/ vs features/)
- [ ] 5.2 Actualizar `openspec/changes/frontend-panel/state.yaml` (estado por fase)