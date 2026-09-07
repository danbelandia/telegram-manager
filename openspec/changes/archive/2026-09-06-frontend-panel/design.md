# Design: Frontend Panel (routing + auth + dashboard)

## Technical Approach

Reemplazar el bootstrap del panel por una app React con estructura de
features (guía frontend §1): Router completo con `RequireAuth` y layout
padre con `<Outlet />`, Context `AuthProvider` + `useAuth()` para la
sesión (access token en memoria, refresh en cookie httpOnly), cliente
HTTP centralizado con inyección de token y 1 retry de refresh, y
TanStack Query para los datos de grupos. Sin librería de UI (CSS
plano); RHF+Zod para el login. Consume el backend ya existente
(archivos `auth_handlers.go`, `groups_handlers.go` — contratos
verificados abajo).

## Architecture Decisions

| Decisión | Opciones | Tradeoff | Elección |
|---|---|---|---|
| D1: Estructura | pages sueltas vs features/ | features agrupa api+types+hooks por dominio (guía §1) | `app/`, `features/`, `pages/`, `components/`, `lib/` |
| D2: Sesión | localStorage vs Context en memoria | localStorage = XSS (spec §17.1 lo prohíbe) | `AuthProvider` (Context en memoria) |
| D3: Refresh | en cliente vs cookie httpOnly | el navegador maneja la cookie solo (spec §17.1) | Cookie httpOnly + 1 retry en api-client |
| D4: Data fetching | useEffect+fetch vs TanStack Query | Query da cache/estados (guía §3 la pide explícita) | TanStack Query (`@tanstack/react-query`) |
| D5: Form login | useState vs RHF+Zod | validación cliente + patrón para forms futuros (decisión usuario) | React Hook Form + Zod |
| D6: UI kit | shadcn/ui vs CSS plano | se difiere hasta tablas reales (decisión usuario) | CSS plano en este cambio |
| D7: Vite proxy | config proxy localhost vs api relativa | `VITE_API_BASE_URL` ya existe en api-client | `server.proxy['/api']→:8080` en vite.config (solo dev) |

## Data Flow

```
LoginPage (RHF+Zod)
   │ POST /api/auth/login {username,password}
   ▼
api-client.request() ──► backend ──► {access_token} + Set-Cookie refresh
   │                                                (navegador la guarda)
   ▼
AuthContext.setToken(access) ──► RequireAuth deja pasar
   │
   ▼
DashboardPage ── useGroups(AppQueryClient) ── GET /api/groups (Bearer)
   │                                              │
   └── data/loading/error ─► render lista + [Administrar]
                                   │
                                   ▼
                          /groups/:id (GroupDetailPage)
```

Refresh transparente: cualquier request 401 → `POST /api/auth/refresh`
(cookie automática) → si OK, reejecuta con token nuevo; si falla,
`AuthContext.logout()` → redirect `/login`.

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `frontend/package.json` | Modify | +`@tanstack/react-query`, `react-hook-form`, `zod` |
| `frontend/src/main.tsx` | Modify | +QueryClientProvider, +AuthProvider envuelven App |
| `frontend/src/App.tsx` | Modify | Rutas §16: `/login`, layout autenticado con Outlet (`/dashboard`, `/groups`, `/groups/:id`, .../users, .../requests, .../logs), 404, root→/dashboard |
| `frontend/src/lib/api-client.ts` | Modify | +`setAccessToken`, +refresh con 1 retry en 401; devuelve `{data,error}` ya normalizado (envelope verificado) |
| `frontend/src/lib/auth-context.tsx` | Create | `AuthProvider`, `useAuth()`: estado sesión, `login()`, `logout()`, restauración con `/me` |
| `frontend/src/components/RequireAuth.tsx` | Create | wrapper de rutas protegidas, redirect a `/login` con `location.state` |
| `frontend/src/components/Layout.tsx` | Create | sidebar+header + `<Outlet />` (wireframe §16) |
| `frontend/src/pages/LoginPage.tsx` | Modify | form RHF+Zod real → `useAuth().login()` |
| `frontend/src/pages/DashboardPage.tsx` | Modify | `useGroups` + estados vacío/error/reintento |
| `frontend/src/pages/GroupDetailPage.tsx` | Create | datos del grupo + nav a users/requests/logs |
| `frontend/src/pages/GroupsPage.tsx` | Create | alias de Dashboard (lista completa) |
| `frontend/src/pages/GroupUsersPage.tsx` | Create | placeholder mínimo |
| `frontend/src/pages/GroupRequestsPage.tsx` | Create | placeholder mínimo |
| `frontend/src/pages/GroupLogsPage.tsx` | Create | placeholder mínimo |
| `frontend/src/pages/NotFoundPage.tsx` | Create | 404 |
| `frontend/src/features/auth/{api,types}.ts` | Create | llamadas auth + tipos (`LoginResponse: {access_token}`) |
| `frontend/src/features/groups/{api,types,hooks}.ts` | Create | GET /api/groups + `Group` type + `useGroups` |
| `frontend/src/App.test.tsx` | Modify | tests de rutas con auth mockeado |
| `frontend/vite.config.ts` | Modify | +proxy `/api`→`http://backend:8080` (dev) |
| `frontend/src/styles.css` | Create | estilos planos del layout (CSS mínimo) |

## Interfaces / Contracts

```ts
// Contratos reales del backend (verificados en auth_handlers.go y groups_handlers.go)
type Group = {
  id: number; telegram_id: number; title: string; username: string | null
  type: string; member_count: number | null; bot_status: string
  bot_permissions: Record<string, boolean>
}
type LoginResponse = { access_token: string }
type MeResponse = { id: string; username: string }
// Envelope del backend: { data: T | null, error: { code, message } | null }
// api-client lanza ApiError({code, message, status}) y devuelve data
```

## Testing Strategy

| Layer | Qué | Cómo |
|---|---|---|
| Unit | api-client (401→refresh→retry; refresh fallido→logout) | Vitest, fetch mockeado |
| Unit | AuthContext (login/logout/restauración /me) | RTL renderHook |
| Unit | LoginPage (validación Zod, submit, mensaje error) | RTL + RHF |
| Unit | RequireAuth (sin sesión→redirect, con sesión→pasa) | MemoryRouter |
| Integration | App rutas (dashboard con grupos mockeados, 404) | RTL + QueryClient real |

No E2E automatizado (spec §21: solo lógica crítica).

## Migration / Rollout

No requiere migraciones ni cambios de backend. En dev, Vite dev server
con proxy a `:8080`. No toca Dockerfile ni compose (solo vite.config
dev).

## Open Questions

- [x] RHF+Zod ahora (resuelto por usuario)
- [x] UI lib diferida (resuelto por usuario)
- [x] `VITE_API_BASE_URL` vs proxy: se usa proxy en dev (navegador sin CORS); si el usuario prefiere URL absoluta, se setea `VITE_API_BASE_URL` en `.env` sin proxy (alternativa válida)