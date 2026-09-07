# Tasks: frontend-refresh — Slice 1 (Mantine foundation)

> **Change**: `frontend-refresh` slice 1 | **Mode**: hybrid | **Delivery**: single-pr, `size:exception` (precedente publications-slice1/2/3, frontend-panel, frontend-moderation).
> **Inherits**: exploration #193, proposal #194, spec #195, design #196.

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~+740 / ~-180 → net **+560 LOC** (frontend-only) |
| 400-line budget risk | **High** (frontend-only, ~20 touched files) |
| Chained PRs recommended | No (single-pr with `size:exception` pre-approved) |
| Delivery strategy | single-pr / size-exception |
| Chain strategy | size-exception |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: size-exception
400-line budget risk: High

> Razonamiento: 400-line budget **High**, pero el usuario aprobó `size:exception` por precedente (ver brief). Diseño frontend-only: la mayoría de los LOC son archivos nuevos (theme, ColorSchemeToggle, notifications, tests). Mantener como PR único simplifica el review — son concerns acoplados (theme + provider + layout + páginas).

## Phase 1: Dependencias + build infra (compile gate primero)

- [x] 1.1 Modificar `frontend/package.json` — agregar `@mantine/core@^7.17`, `@mantine/hooks@^7.17`, `@mantine/notifications@^7.17`, `@tabler/icons-react@^3`, `postcss-preset-mantine@^1.17`, `postcss-simple-vars@^7` (sin bump a React/Vite/RHF/Zod)
- [x] 1.2 Ejecutar `cd frontend && npm install` — verificar peer warnings (v7+React 19 OK; fallback `^8.3.14` si `ERESOLVE`; `--legacy-peer-deps` solo si falla)
- [x] 1.3 Crear `frontend/postcss.config.cjs` — `postcss-preset-mantine` + `postcss-simple-vars` con breakpoints Mantine (`xs/sm/md/lg/xl`)
- [x] 1.4 Modificar `frontend/vite.config.ts` — agregar `test: { pool: 'vmThreads', isolate: true, globals: true }` (mantener `environment`/`setupFiles`; W1 del verify-report slice 3)
- [x] 1.5 Gate: `cd frontend && npm run build` debe finalizar sin errores (puede fallar imports hasta Phase 2; documentar si es esperado)

## Phase 2: Theme + providers

- [x] 2.1 Crear `frontend/src/theme.ts` — `createTheme({ primaryColor: 'blue', defaultRadius: 'md', fontFamily: 'var(--mantine-font-family)' })` + `localStorageColorSchemeManager({ key: 'mantine-color-scheme-value' })`
- [x] 2.2 Crear `frontend/src/lib/notifications.ts` — `notifySuccess(message: string)` y `notifyError(message: string, title?: string)` delegando a `notifications.show({...})`
- [x] 2.3 Modificar `frontend/src/main.tsx` — importar `@mantine/core/styles.css` y `@mantine/notifications/styles.css` primero; envolver `<App/>` con `<MantineProvider theme={mantineTheme} defaultColorScheme="auto" colorSchemeManager={...}><Notifications position="top-right" zIndex={2077} limit={5}/>`
- [x] 2.4 Gate: `cd frontend && npm run build` debe finalizar limpio (compile + bundle)

## Phase 3: Layout + dark mode + logout

- [x] 3.1 Crear `frontend/src/components/ColorSchemeToggle.tsx` — `ActionIcon` con `useMantineColorScheme().toggleColorScheme()` + `IconSun`/`IconMoon` de Tabler
- [x] 3.2 Reemplazar `frontend/src/components/Layout.tsx` — `<AppShell header={{h:60}} navbar={{w:260, breakpoint:'sm'}}>` con header (logo + `<ColorSchemeToggle/>` + botón Logout), navbar (`/dashboard`, `/groups`, `/publications` con iconos Tabler) y `<Outlet/>` en `<AppShell.Main>`
- [x] 3.3 Wirear logout — wrapper `handleLogout` en `Layout.tsx`: `await logout()` → `notifySuccess('Sesión cerrada')` → `navigate('/login', { replace: true })` (NO tocar `lib/auth-context.tsx`)
- [x] 3.4 Gate: `cd frontend && npm run build` limpio

## Phase 4: Migración de páginas

- [x] 4.1 Modificar `frontend/src/pages/LoginPage.tsx` — `<Center><Paper><Stack>` con `<TextInput>`/`<PasswordInput>`/`<Button>` vía `<Controller>` RHF/zod (schema sin cambios); onSuccess → `notifySuccess('Bienvenido')` + `navigate(from,{replace:true})`; onError → `notifyError(message)`
- [x] 4.2 Modificar `frontend/src/pages/DashboardPage.tsx` — `<SimpleGrid cols={{base:1,sm:2,lg:3}}>` de `<Card>` por grupo (título, tipo, miembros, `<Button component={Link}>`→detail); `<Skeleton>` mientras `isPending`; `<Text>` vacío
- [x] 4.3 Modificar `frontend/src/pages/GroupsPage.tsx` — `<Table>` real con columnas Título, Tipo, Miembros, Bot (`<Badge>`), Acciones (`<Button variant="subtle" component={Link}>`); `<Skeleton>` rows en loading
- [x] 4.4 Modificar `frontend/src/pages/GroupDetailPage.tsx` — `<Title order={2}>` + `<Text size="sm">` (telegram_id) + `<Badge>` (bot status); `<Tabs defaultValue="detalle">` con 4 paneles: Detalle (`<Group>`+`<Stack>` permissions), Membresía y moderación (lock/unlock + delete/pin + botón→`/users`), Solicitudes (texto + botón→`/requests`), Logs (texto + botón→`/logs`)
- [x] 4.5 Gate: `cd frontend && npm run build` limpio

## Phase 5: CSS cleanup + helpers de test

- [x] 5.1 Modificar `frontend/src/styles.css` — borrar reglas muertas (`app-layout`, `sidebar*`, `header*`, `login-page`, `login-form`, `form-field`, `form-error*`, `btn*`, `state-block`, `group-list`, `group-card*`, `group-detail`, `detail-actions`, `group-sections`); mantener `:root` CSS vars + body + `.not-found`
- [x] 5.2 Modificar `frontend/src/test/helpers.tsx` — extender `renderWithProviders` con `<MantineProvider theme={mantineTheme}><Notifications/></MantineProvider>` dentro del `<QueryClientProvider>`; importar `mantineTheme` desde `../theme`
- [x] 5.3 Modificar `frontend/src/App.test.tsx` y `frontend/src/pages/LoginPage.test.tsx` — swap del wrapper inline a `renderWithProviders`; aserciones sin cambios
- [x] 5.4 Gate: `cd frontend && npm test -- --run` — los 49+ tests existentes deben seguir verdes con el wrapper extendido

## Phase 6: Smoke tests de páginas migradas

- [x] 6.1 Modificar `frontend/src/pages/DashboardPage.test.tsx` — +1 smoke test: renderiza `<Card>` por grupo con mocks de `useGroups`
- [x] 6.2 Crear `frontend/src/components/Layout.test.tsx` — smoke: header (brand) + 3 `<NavLink>` + `<Outlet/>`; mock `useAuth` con user
- [x] 6.3 Crear `frontend/src/lib/notifications.test.ts` — smoke: `notifySuccess('x')` + `notifyError('y')` no tiran; spy en `notifications.show`

## Phase 7: Final gates

- [x] 7.1 `cd frontend && npm test -- --run` 100% verde, cero flakeos paralelos (W1 resuelto por Phase 1.4)
- [x] 7.2 `cd frontend && npm run build` limpio, bundle delta +150-200kb gz
- [x] 7.3 Verificar no-regresión: `git diff main -- frontend/src/features frontend/src/lib/api-client.ts frontend/src/lib/auth-context.tsx backend/ migrations/` debe estar vacío
- [x] 7.4 Verificar `git diff main -- frontend/src/pages/{GroupUsers,GroupRequests,GroupLogs,Publications,NotFound}Page.tsx` y `frontend/src/components/RequireAuth.tsx` vacío (no migrados en slice 1)

## Phase 8: README + commit final

- [x] 8.1 Modificar `README.md` (raíz) — sección "UI library & Dark mode" con Mantine v7, toggle, clave localStorage `mantine-color-scheme-value`
- [x] 8.2 Commit final conventional (`feat(frontend): ...`); NO push (single-pr local); actualizar brief del PR con link al spec + design

## Verification

- `cd frontend && npm run build` limpio; bundle gz medible.
- `cd frontend && npm test -- --run` 100% verde, sin flakeos.
- Login: `notifySuccess('Bienvenido')` visible tras login OK; `notifyError(msg)` tras 401.
- Logout: `notifySuccess('Sesión cerrada')` + redirect `/login`.
- Dark mode toggle persiste tras refresh (`localStorage.getItem('mantine-color-scheme-value')`).
- AppShell: header + navbar visibles en rutas autenticadas; LoginPage fuera del AppShell.
- Páginas no migradas (`/publications`, `/groups/:id/users`, etc.) siguen funcionando dentro del nuevo shell.
- `git diff main` muestra cambios **solo** en: `frontend/package.json`, `frontend/postcss.config.cjs` (nuevo), `frontend/src/{theme.ts, lib/notifications.ts, components/{Layout,ColorSchemeToggle}.tsx, main.tsx, test/helpers.tsx, styles.css, pages/{Login,Dashboard,Groups,GroupDetail}Page.tsx, components/Layout.test.tsx, lib/notifications.test.ts, App.test.tsx, pages/LoginPage.test.tsx, pages/DashboardPage.test.tsx}`, `frontend/vite.config.ts`, `README.md`.
