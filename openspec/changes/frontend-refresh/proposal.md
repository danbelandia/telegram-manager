# Proposal: frontend-refresh — Slice 1 (Mantine + AppShell + Dark Mode + Notifications)

## Intent

El frontend del panel (AGENTS §16) funciona pero la UX es pobre: cero librería UI, CSS plano en `styles.css` (395 LOC), `Layout.tsx` ad-hoc como `<div>`, sin dark mode, sin feedback de mutaciones. Slice 1 introduce una fundación UI moderna (Mantine v7 + AppShell + dark mode + notifications) sin tocar `features/*` ni lógica de negocio. Slice 2 (futuro) migrará `GroupUsersPage` / `GroupRequestsPage` / `GroupLogsPage` / `PublicationsPage` con componentes específicos (DatePicker, tablas avanzadas) y conectará notifications a los ~15 hooks restantes.

## Scope

### In Scope

- Deps frontend: `@mantine/core@^7.17`, `@mantine/hooks@^7.17`, `@mantine/notifications@^7.17`, `@tabler/icons-react@^3`, `postcss-preset-mantine@^1.17`, `postcss-simple-vars@^7`.
- Theme (`src/theme.ts`): `primaryColor: 'blue'`, `defaultRadius: 'md'`, system font stack, `localStorageColorSchemeManager('mantine-color-scheme-value')`.
- AppShell: header 60px + navbar 260px + breakpoint `sm` + `<Outlet/>` interno. Wrapping: `<RequireAuth><Layout>…</Layout></RequireAuth>` (sin cambios estructurales en `App.tsx`).
- Páginas migradas: `LoginPage` (Paper + TextInput + PasswordInput + Button vía Controller/RHF/zod), `DashboardPage` (Card + Grid + Skeleton), `GroupsPage` (Table), `GroupDetailPage` (Tabs info/lock/messages). NO migradas: `GroupUsersPage`, `GroupRequestsPage`, `GroupLogsPage`, `PublicationsPage` (slice 2). `NotFoundPage`: DECIDE.
- Notifications: `<Notifications position="top-right" zIndex={2077} limit={5} />` en `main.tsx`; helpers `notifySuccess/notifyError` en `lib/notifications.ts`; wiring slice 1: solo Login (success → "Bienvenido") + Logout ("Sesión cerrada").
- Dark mode toggle en `AppShellHeader` con `useMantineColorScheme()`.
- CSS: reemplazar `styles.css` con `@mantine/core/styles.css` + `@mantine/notifications/styles.css`; limpiar reglas muertas tras migración.
- PostCSS: `postcss.config.cjs` con `postcss-preset-mantine` + `postcss-simple-vars` (breakpoints Mantine).
- Vitest W1 fix: `vite.config.ts` → `test: { pool: 'vmThreads', isolate: true }` (recomendación del verify-report slice 3 #190).
- Tests: extender `renderWithProviders` en `helpers.tsx` con `<MantineProvider><Notifications/></MantineProvider>`; 1 smoke test por página migrada + `Layout.test.tsx` (NUEVO) + `notifications.test.ts` (NUEVO).
- Docs: sección UI/dark mode en root `README.md` + `frontend/README.md`.

### Out of Scope

- Migración visual de `GroupUsersPage`, `GroupRequestsPage`, `GroupLogsPage`, `PublicationsPage` (slice 2 — necesitan DatePicker/tabla avanzada).
- Wiring de notifications a hooks restantes (slice 2).
- Cambios en `features/*`, `lib/api-client.ts`, `lib/auth-context.tsx`.
- Dockerfile, backend, migraciones de BD, contrato de API.

## Capabilities

### New Capabilities

- `frontend-ui-foundation`: librería UI Mantine v7, theme central, AppShell, dark mode persistente, sistema de notifications global, helpers test-friendly. Cubre el view layer compartido y desbloquea slices futuros (2: páginas complejas; 3: Charts).

### Modified Capabilities

- None. Las migraciones de `LoginPage`/`DashboardPage`/`GroupsPage`/`GroupDetailPage` son detalle de implementación de `frontend-auth`, `frontend-dashboard`, `frontend-routing` — los **requisitos no cambian**. `frontend-routing` sigue requiriendo "Layout persistente con sidebar/header usando `<Outlet/>`" (AppShell lo cumple); `frontend-auth` sigue requiriendo login vía JWT; `frontend-dashboard` sigue listando grupos desde `/api/groups`.

## Approach

| # | Decisión | Cómo |
|---|----------|------|
| 1 | Mantine v7.17 | Peer dep `^18 \|\| ^19` cubre React 19.2.8; fallback `^8.3.14` si peer warning. NO v9 (breaking changes innecesarios). |
| 2 | Forms | Mantener RHF + zod + Controller (no migrar a `@mantine/form`; zod no se reemplaza). |
| 3 | Icons | `@tabler/icons-react@^3` (oficial Mantine, tree-shakeable). |
| 4 | Layout | AppShell reemplaza `<div>` en `Layout.tsx`; LoginPage queda fuera. |
| 5 | Notifications | Helpers centralizados en `lib/notifications.ts`; wiring mínimo slice 1 (Login/Logout). |
| 6 | Dark mode | `useMantineColorScheme` + `localStorageColorSchemeManager` (default `auto`). |
| 7 | Vitest | `pool: 'vmThreads'` + `isolate: true` (W1 del verify-report slice 3). |
| 8 | CSS | Imports Mantine en `main.tsx`; `styles.css` reducido a overrides mínimos. |
| 9 | PostCSS | `postcss.config.cjs` con presets (requerido por v7 para tema). |

## Affected Areas

| Area | Impact |
|------|--------|
| `frontend/postcss.config.cjs` | New |
| `frontend/src/theme.ts` | New |
| `frontend/src/lib/notifications.ts` | New |
| `frontend/src/components/Layout.tsx` | Replaced (AppShell) |
| `frontend/src/components/ColorSchemeToggle.tsx` | New |
| `frontend/package.json` | Modified (deps) |
| `frontend/src/main.tsx` | Modified (MantineProvider + CSS imports) |
| `frontend/src/pages/{Login,Dashboard,Groups,GroupDetail}Page.tsx` | Modified |
| `frontend/src/test/helpers.tsx` | Modified (renderWithProviders) |
| `frontend/src/styles.css` | Modified (cleanup) |
| `frontend/vite.config.ts` | Modified (test.pool) |
| `frontend/README.md` + root `README.md` | Modified (sección UI/dark) |

`features/*`, `lib/*`, backend, migraciones, Docker: **no modificados**.

## Risks

| Risk | Mitigation |
|------|------------|
| Peer dep Mantine v7 + React 19 type mismatch | Fallback `^8.3.14`; `--legacy-peer-deps` solo si falla. |
| Bundle +150-200kb gz | Aceptable para panel admin; lazy load queda para slice futuro si hace falta. |
| W1 fix rompe tests con estado global | Riesgo bajo (cada test crea su QueryClient propio). |
| Dark mode flicker en SSR | No aplica (Vite SPA); localStorage aplica antes del primer paint. |
| `primaryColor: 'blue'` difiere del `--primary: #3b82f6` actual | Verificar visualmente; ajustar palette en `theme.ts` si difiere. |
| `styles.css` queda con reglas muertas | Grep selectores antes de commit; eliminar lo no usado. |

## Rollback

Revert del PR único (`git revert <merge>`). No hay migración de BD ni cambios de contrato API. Vite/npm: `npm install` recupera estado limpio.

## Dependencies

- `main @ ace1f59` (publications-slice3 archivado).
- React 19.2.8 + Vite 8 + TS 7 + RHF 7 + Zod 4 (sin cambios).
- Recomendación W1 de `sdd/publications-slice3/verify-report` (#190).

## Success Criteria

- [ ] `npm run build` ok; bundle delta +150-200kb gz (medido).
- [ ] `npm test -- --run` 100% verde, sin flakeos paralelos (W1 resuelto).
- [ ] LoginPage usa PasswordInput Mantine + notifica "Bienvenido" al éxito.
- [ ] Logout notifica "Sesión cerrada".
- [ ] Toggle dark/light persiste en localStorage tras refresh.
- [ ] AppShell responsive: navbar colapsa a drawer en mobile (<sm).
- [ ] LoginPage queda fuera del AppShell.
- [ ] `features/*`, `lib/*`, backend, migraciones: sin cambios en `git diff main`.
- [ ] README sección UI + dark mode actualizada.

## Open Questions

Ninguna — la exploración (#193) resolvió las 10 preguntas abiertas.
