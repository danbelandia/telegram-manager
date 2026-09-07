# Exploration: frontend-refresh — Slice 1 (Foundation: Mantine + AppShell + Dark Mode + Notifications)

> **Change**: `frontend-refresh` (Slice 1 de N — fundación UI). Modo **hybrid**.
> **Persisted**: `sdd/frontend-refresh/exploration` (Engram) + este archivo (filesystem).
> **Trigger**: queja del usuario "el frontend está todo muy básico" (citada en memoria reciente). El panel funciona pero la UX es pobre — sin librería UI, sin dark mode, sin feedback de mutaciones, layout ad-hoc.
> **Scope slice 1**: fundación. Slice 2 (futuro) migrará las páginas con tablas / DatePicker y conectará notificaciones al resto de los hooks. Slice 3 (futuro) cubrirá Charts si se piden.
> **Delivery**: single-pr, `size:exception` solicitada por el usuario (precedente: publications-slice1/2/3, frontend-panel, frontend-moderation todos single-pr con exception aprobada).

## Estado heredado (main @ ace1f59)

### Frontend actual
- **Stack**: React 19.2.8 + Vite 8 + TS 7 + React Router 7 + TanStack Query 5 + RHF 7 + Zod 4 (`frontend/package.json:12-32`). CERO librería UI — CSS plano en `frontend/src/styles.css` (395 LOC).
- **Layout ad-hoc**: `frontend/src/components/Layout.tsx` es un `<div>` con sidebar + header hardcodeado. NavLinks con className `layout-nav-link`.
- **Páginas existentes** (todas con CSS plano, sin componentes reusables):
  - `LoginPage.tsx` (74 LOC) — RHF + Zod, formulario en `<main className="login-page">` con `<input>` nativos.
  - `DashboardPage.tsx` (73 LOC) — lista de grupos con `<ul className="group-list">` + `<li className="group-card">`.
  - `GroupsPage.tsx` (7 LOC) — wrapper que reusa `DashboardPage`.
  - `GroupDetailPage.tsx` (161 LOC) — `<dl>` + secciones con `<button className="btn">` y `window.confirm` para lock/delete.
  - `GroupUsersPage.tsx` (167 LOC) — `<input>` de lookup + lista de admins + `UserActions` con `window.confirm`.
  - `GroupRequestsPage.tsx` (117 LOC) — `<table className="data-table">` con badges.
  - `GroupLogsPage.tsx` (74 LOC) — `<table className="data-table">` con badges.
  - `PublicationsPage.tsx` (456 LOC) — formulario complejo con checkboxes, datetime-local, `<table>` historial con botones inline.
- **Sin tests para Layout ni AppShell**.
- **Tests existentes (Vitest + jsdom)**: 7 archivos de test en `pages/*.test.tsx`. Test global `App.test.tsx` (66 LOC) renderiza la app entera con `<AuthProvider>` + `<QueryClientProvider>` + `<MemoryRouter>`.
- **Helpers de test** (`frontend/src/test/helpers.tsx`):
  - `renderWithRouter(ui, entries)` — wrapper con MemoryRouter.
  - `createTestQueryClient()` — QueryClient con `retry:false, gcTime:0`.
  - `renderWithProviders(ui, entries)` — combina ambos.
  - `mockFetchRoutes(routes)` — stub global de fetch por substring de URL.
  - `okJson / errorJson / noContent / matchQuery`.
- **Issue conocido**: W1 del `sdd/publications-slice3/verify-report` — 2 tests slice-2 flakean bajo carga paralela jsdom. Recomendación del verify: `test.pool: 'vmThreads'` (NO es regresión slice-3, es config). **Decisión propuesta**: incluir este fix en slice 1 (ver §G).
- **TS estricto**: `noUnusedLocals`, `noUnusedParameters`, `noFallthroughCasesInSwitch` (`tsconfig.json:17-19`). Cualquier wrapper de Mantine debe respetar esto.
- **Vite proxy** `/api → backend:8080` ya configurado (`vite.config.ts:16-21`).
- **Dockerfile dev**: node:20-alpine + bind mount + volumen anónimo para node_modules (fricción OneDrive/Windows, ver `frontend/Dockerfile:1-15`).
- **Sin Dockerfile de producción** dedicado al frontend — la guía de compose usa el dev server (no es el caso para el MVP actual pero hay que considerar bundle size).

### Features (NO se tocan en slice 1)
- `features/auth/{api,types}.ts` — login/me/logout (sin cambios).
- `features/groups/{api,hooks,permissions,types}.ts` — TanStack Query para grupos (sin cambios).
- `features/moderation/{api,error,hooks,types}.ts` — hooks de moderación (sin cambios).
- `features/publications/{api,ButtonsEditor,error,hooks,types,validateScheduledAtClient}.ts` — sin cambios.
- `lib/api-client.ts` — sin cambios (envelope `{data, error}`, refresh transparente).
- `lib/auth-context.tsx` — sin cambios (mantiene `loading, user, login, logout`).

> **Regla slice 1**: NO tocar `features/*` ni `lib/api-client.ts` ni `lib/auth-context.tsx`. Solo se reemplaza el **view layer** (`pages/`, `components/Layout.tsx`, `main.tsx`, `App.tsx`, `styles.css`, helpers de test).

## Compatibilidad confirmada: Mantine v7 + React 19

**Hechos verificados via context7 (consultas 2026-09-07):**

1. **Mantine 7.x** (último estable `7.17.6`): peer dep `react ^18 || ^19`. Compatible con React 19.2.8 porque:
   - Mantine 7.9.0 (mayo 2025) declaró compat con React 18.3 — sentó las bases para React 19.
   - React 19.2 satisface los hooks que Mantine usa (`useId`, `useSyncExternalStore`).
2. **Mantine 8.x** (último estable `8.3.14`): explícitamente compatible con React 19 (release notes 8.1.0 migraron `MutableRefObject` → `RefObject`).
3. **Mantine 9.x** (último estable `9.0.0`): **REQUIERE React 19.2+ estricto** (peer dep `react ^19.2.0`).

**Decisión propuesta**: **Mantine 7.17.6** (la última de la rama 7) por:
- Cumple peer dep `^18 || ^19` y React 19.2.8 lo satisface.
- Bundle ~30-40% más liviano que v8/v9 (sin Portal reuseTargetNode, sin deduplicateInlineStyles).
- Documentación madura y ampliamente probada.
- Si npm install emite warnings de peer dep o TS detecta incompatibilidades de tipo, **fallback**: bump a `^8.3.14` (un solo cambio en `package.json`, no requiere refactor).

> **NO** usar Mantine 9.0: requiere React 19.2 estricto, lo cumplimos, pero v9 trae breaking changes no necesarias (Portal reuse default, timePickerProps, cambios de CSS imports) que no necesitamos para slice 1.

## PostCSS preset — requerido por Mantine v7

**Hecho verificado**: Mantine v7 usa `light-dark()` y breakpoints que dependen de `postcss-preset-mantine` + `postcss-simple-vars`. Sin esto, los estilos de tema no compilan correctamente.

**Paquetes a agregar**:
- `postcss-preset-mantine@^1.17` (latest)
- `postcss-simple-vars@^7`

`postcss.config.cjs` (CommonJS porque Vite carga configs en CJS por defecto):
```js
module.exports = {
  plugins: {
    'postcss-preset-mantine': {},
    'postcss-simple-vars': {
      variables: {
        'mantine-breakpoint-xs': '36em',
        'mantine-breakpoint-sm': '48em',
        'mantine-breakpoint-md': '62em',
        'mantine-breakpoint-lg': '75em',
        'mantine-breakpoint-xl': '88em',
      },
    },
  },
}
```

## Decisiones (con tradeoffs)

| # | Topic | Decisión | Alternativas | Rationale |
|---|-------|----------|--------------|-----------|
| **D1** | UI library | **Mantine v7.17.6** (`@mantine/core`, `@mantine/hooks`, `@mantine/notifications`) | v8, v9, Chakra, MUI, Radix, shadcn | AGENTS §16 deja a elección; Mantine = madurez, accesibilidad built-in, dark mode nativo, notifications listas, peso aceptable (~150-200kb gz total). v7 evita breaking changes de v8/v9. |
| **D2** | Form library | **Mantener react-hook-form + zod + Controller de RHF** | Migrar a `@mantine/form` | RHF + zod ya integrados en `LoginPage`, `PublicationsPage` (slice 2/3), tests existen. `@mantine/form` no reemplaza zod fácilmente (sin validador nativo). Costo de migración > beneficio. Controller de RHF envuelve inputs de Mantine en una línea. |
| **D3** | Icons | **`@tabler/icons-react@^3`** | `@phosphor-icons/react`, `lucide-react` | Tabler es la librería oficial recomendada por Mantine, ya se usa internamente en sus ejemplos. Bundle tree-shakeable. Phosphor también sirve pero requiere aprender otro set. |
| **D4** | Theme | **`MantineProvider theme={{ primaryColor: 'blue', defaultRadius: 'md' }}` + dark mode `useMantineColorScheme` + `localStorageColorSchemeManager('mantine-color-scheme-value')`** | Custom palette, sin dark mode | Blue es el primary actual (`styles.css:11`). Default radius 'md' (~4px) coincide con el visual actual. Dark mode toggleable con persistencia — UX básica que el usuario ya pidió implícitamente al quejarse. |
| **D5** | AppShell layout | **`AppShell` con `header={{height:60}}` + `navbar={{width:260, breakpoint:'sm', collapsed:{mobile:false}}}`** | Mantener `<div>` con grid, Sidebar library | AppShell resuelve responsive (mobile drawer) sin código custom. Header tiene logo + dark toggle + user menu. Navbar tiene NavLink con iconos Tabler. |
| **D6** | AppShell wrapping | **`<RequireAuth><Layout>...</Layout></RequireAuth>` (igual que ahora)** + `<Layout>` usa AppShell con `<Outlet/>` interno | Wrapper global, HOC | Mantiene la estructura del routing actual (spec frontend-routing). Cero cambios en `App.tsx`. LoginPage queda FUERA del AppShell. |
| **D7** | Notifications | **`<Notifications position="top-right" zIndex={2077} limit={5} />` global + helpers `notifySuccess/notifyError` en `lib/notifications.ts`** | Sin notificaciones, react-toastify | Notifications de Mantine ya viene con la librería (D1). Top-right no tapa contenido. Helpers centralizados para tests futuros. |
| **D8** | Slice 1 hook wiring | **Solo Login (success → notify "Bienvenido") + Logout (notify "Sesión cerrada")**. Otros hooks (useCreatePublication, useBanUser, etc.) se difieren a slice 2. | Wirear todo en slice 1, no wirear nada | Wiring todos en slice 1 es scope creep (mezcla migración visual con feedback de mutaciones). Wirear Login da feedback inmediato al usuario y valida el patrón. Slice 2 aplica el mismo patrón a los ~15 hooks restantes. |
| **D9** | Dark mode toggle | **Botón en `AppShellHeader` con `useMantineColorScheme()` + persistencia via `localStorageColorSchemeManager` (key `mantine-color-scheme-value`)** | Toggle en página de settings, sin persistencia | Persistencia evita reset al refresh. Botón en header = estándar UX. Default `auto` respeta SO del usuario. |
| **D10** | CSS strategy | **Reemplazar `styles.css` con imports de Mantine + eliminar reglas muertas. NO usar CSS modules.** | CSS modules per page, Tailwind | Mantine trae sus propios estilos (`@mantine/core/styles.css`). CSS plano sigue siendo lo más simple para reglas custom (login container). Tailwind introduce otra dependencia. |
| **D11** | Fonts | **`@mantine/core` font-family por defecto (system stack via CSS var `--mantine-font-family`)** | Google Fonts (Inter), local font file | System stack = cero requests externos, FCP óptimo. AGENTS §16 prioriza simplicidad. Inter se puede agregar después si el usuario pide. |
| **D12** | PostCSS | **`postcss.config.cjs` con `postcss-preset-mantine` + `postcss-simple-vars`** | Sin PostCSS, otros plugins | Mantine v7 documenta este setup como oficial. Sin esto, el tema se rompe silenciosamente. |
| **D13** | Vitest pool (W1 fix) | **Agregar `test: { pool: 'vmThreads', isolate: true }` en `vite.config.ts`** para eliminar flakeos paralelos del verify-report slice 3 | Dejar como está, fixear por archivo, deferred | El verify-report explícitamente recomienda `pool: 'vmThreads'`. Costo: 1 línea de config. Beneficio: tests slice-2 vuelven a correr limpios en suite completa. Out-of-scope original pero adjacent. |
| **D14** | Bundle impact | **Aceptable ~150-200kb gz total (core + hooks + notifications + tabler icons)** | Bundle más chico (no icons), lazy load | Panel admin no es crítico en bundle size. Tabler tree-shakea los icons que importás. Sin lazy load por ahora (todo se carga upfront). |
| **D15** | Dockerfile | **SIN cambios** — vite build sigue generando dist/ que nginx sirve | Multi-stage nginx build, no incluido todavía | El `Dockerfile` actual es dev server. Cuando se haga prod build, el Mantine bundle va igual. Slice 1 NO toca Docker. |
| **D16** | Tests strategy | **Extender `Providers` wrapper en helpers.tsx para incluir `<MantineProvider theme={...}><Notifications /></MantineProvider>`. Re-render tests existentes usando `renderWithProviders` o actualizando wrappers ad-hoc. Agregar 1 test smoke por página migrada.** | Reescribir todos los tests, agregar tests nuevos exhaustivos | Tests existentes deben pasar sin cambios masivos (los wrappers se componen). 1 test por página migrada prueba la integración real con Mantine. |
| **D17** | Helpers de notificación | **`lib/notifications.ts` exporta `notifySuccess(msg: string)` y `notifyError(msg: string, title?: string)`** — wrap `notifications.show({...})`** | Inline `notifications.show`, sin helpers | Helpers permiten cambiar el formato (icon, color) en un solo lugar. Test-friendly (mockeable). |

## Affected Areas

### Archivos nuevos (5)
- `frontend/postcss.config.cjs` — config PostCSS (D12).
- `frontend/src/theme.ts` — `mantineTheme` + `localStorageColorSchemeManager('mantine-color-scheme-value')`.
- `frontend/src/lib/notifications.ts` — `notifySuccess`, `notifyError` (D17).
- `frontend/src/components/Layout.tsx` (REEMPLAZA actual) — AppShell + header + navbar (D5/D6).
- `frontend/src/components/ColorSchemeToggle.tsx` — botón toggle dark/light con iconos Tabler.

### Archivos modificados (12)
- `frontend/package.json` — agregar deps Mantine v7 + PostCSS (D1, D3, D12).
- `frontend/src/main.tsx` — wrap con `<MantineProvider>` + `<Notifications>`, importar CSS de Mantine (D7).
- `frontend/src/App.tsx` — sin cambios estructurales (Layout sigue siendo el wrapper autenticado). Solo si Layout cambia el nombre del import — no.
- `frontend/src/pages/LoginPage.tsx` — `Paper + TextInput + PasswordInput + Button` (sigue RHF + zod).
- `frontend/src/pages/DashboardPage.tsx` — `Card + Grid + Skeleton + Button` para grupos.
- `frontend/src/pages/GroupsPage.tsx` — `Table` o `SimpleGrid` (DECIDE: Table es más útil para muchos grupos, ver §G).
- `frontend/src/pages/GroupDetailPage.tsx` — `Tabs` (manteniendo las 3 secciones existentes: info, lock/unlock, messages). Los sub-links a users/requests/logs se mantienen como NavLink en el header del detalle.
- `frontend/src/test/helpers.tsx` — extender `renderWithProviders` con `<MantineProvider><Notifications/></MantineProvider>`.
- `frontend/src/styles.css` — limpiar reglas que quedan sin uso después de migrar (login-page, btn, group-card, etc.). Mantener solo lo que sobreviva (back-link, not-found).
- `frontend/index.html` — sin cambios significativos (Mantine no requiere fonts externas por defecto).
- `frontend/vite.config.ts` — agregar `test: { pool: 'vmThreads', isolate: true }` (D13).
- `frontend/README.md` — sección "UI / Dark mode" con link a Mantine docs.

### Archivos NO modificados (conscientes)
- `features/auth/*`, `features/groups/*`, `features/moderation/*`, `features/publications/*`, `features/publications/ButtonsEditor.tsx` — sin cambios.
- `lib/api-client.ts`, `lib/auth-context.tsx` — sin cambios.
- `pages/GroupUsersPage.tsx`, `pages/GroupRequestsPage.tsx`, `pages/GroupLogsPage.tsx`, `pages/PublicationsPage.tsx` — se migran en slice 2 (cuando lleguen los componentes específicos como DatePicker, tablas avanzadas, dropdown de filtros).
- `pages/NotFoundPage.tsx` — slice 3 o se queda simple (DECIDE en tasks).

### Docs / Config (2)
- `README.md` — sección frontend agregada: stack UI, dark mode toggle, link a Mantine docs.
- `.env.example` — sin cambios (no se agrega env var en slice 1).

## Estrategia de testing

### Tests existentes que DEBEN seguir verdes sin cambios (8 archivos)
- `App.test.tsx` (3 tests: redirect /, dashboard con sesión, 404) — sigue funcionando porque los wrappers se componen.
- `LoginPage.test.tsx` (3 tests) — usa wrapper custom con AuthProvider; debe migrar a `renderWithProviders` o seguir igual si no necesita Mantine para testear.
- `DashboardPage.test.tsx` (3 tests) — usa wrapper con QueryClient + MemoryRouter; debe seguir verde (Dashboard sigue usando `useGroups`).
- `GroupDetailPage.test.tsx` (5 tests) — mismo patrón, sigue verde.
- `GroupUsersPage.test.tsx` (7 tests) — mismo.
- `GroupRequestsPage.test.tsx` — no leí el archivo; verificar.
- `GroupLogsPage.test.tsx` — no leí el archivo; verificar.
- `PublicationsPage.test.tsx` (15+ tests) — sigue verde (NO se migra en slice 1).

### Tests NUEVOS a agregar (3-4)
- `pages/LoginPage.test.tsx` — extender/agregar 1 test: "muestra PasswordInput de Mantine" (puede ser parte del test existente).
- `pages/DashboardPage.test.tsx` — extender con 1 test: "renderiza cards de Mantine para cada grupo" (refactor a `renderWithProviders`).
- `components/Layout.test.tsx` (NUEVO) — smoke test: "renderiza header + navbar + outlet". Mockea `useAuth` para tener user.
- `lib/notifications.test.ts` (NUEVO) — smoke test: `notifySuccess('x')` y `notifyError('y')` no tiran y llaman `notifications.show`.

### Estrategia para tests con Mantine + jsdom
- `Notifications` necesita un portal target. Mantine v7 crea automáticamente `document.body` como target — funciona en jsdom.
- `MantineProvider` setea CSS vars en `:root` — funciona en jsdom.
- `@testing-library/user-event` (ya instalado) sigue siendo la herramienta para simular clicks.

## Riesgos

1. **Mantine v7 + React 19 type peer dep**: si npm install emite `ERESOLVE` o TS muestra `Cannot find module '@types/react'` con tipos incompatibles, fallback a v8 (D1 contingency). Acción preventiva: usar `--legacy-peer-deps` solo si falla; preferir fix limpio.
2. **Bundle size delta**: ~150-200kb gz puede impactar FCP en conexiones lentas. Aceptable para un panel admin, mitigable en slice futuro con lazy loading.
3. **W1 fix colateral**: cambiar `pool: 'vmThreads'` puede romper tests que dependen de estado global compartido (no es el caso aquí — cada test usa su propio QueryClient). Riesgo bajo.
4. **CSS cleanup incompleto**: si `styles.css` queda con reglas que nadie usa después de migrar, quedan ~200 LOC de basura. Acción: grep por selectores antes de commit; eliminar todo lo que no se referencia en JSX.
5. **AppShell responsive**: con `breakpoint: 'sm'`, en mobile el navbar colapsa a drawer. Si jsdom no respeta el breakpoint, los tests pueden no cubrir el path mobile. Aceptable: smoke test con desktop viewport; el responsive real se prueba en navegador.
6. **Dark mode flicker**: con SSR deshabilitado (Vite SPA), el flicker es nulo — el color scheme se aplica antes del primer paint via localStorage manager. Sin riesgo.
7. **`Theme` con primaryColor 'blue'**: Mantine v7 tiene `blue.6` como default blue. Verificar visualmente que coincida con `--primary: #3b82f6` actual (`styles.css:11`). Si difiere, custom palette en D4.
8. **`<Notifications>` z-index**: si hay modales de Mantine en el futuro (slice 2+), pueden tapar las notificaciones. `zIndex={2077}` es el default recomendado por Mantine para estar debajo de Modal/Drawer (que usan 200+). OK.

## Ready for Proposal

**Sí**. Botón disparador claro, decisiones con tradeoffs, affected areas mapeados, dependencias confirmadas vía context7, riesgo W1 integrable como bonus. La fundación desbloquea slice 2 (DatePicker + tablas avanzadas + wiring de mutaciones) y slice 3 (Charts si se piden).

## Open questions resueltas en este exploration

| # | Pregunta | Resolución |
|---|----------|------------|
| Q1 | ¿Mantine v7 o v8? | **v7.17.6** — peer dep cubre React 19, más liviano, evita breaking changes innecesarios. Fallback a v8 si peer dep warning. |
| Q2 | ¿Mantener RHF o migrar a @mantine/form? | **Mantener RHF + zod** — schemas y tests ya existen, Controller envuelve inputs Mantine. |
| Q3 | ¿Tabler o Phosphor icons? | **Tabler** — oficial de Mantine, tree-shakeable. |
| Q4 | ¿Cómo wrappear MantineProvider en `Providers`? | En `renderWithProviders` + `createTestQueryClient` se mantiene, se agrega `<MantineProvider theme={mantineTheme}><Notifications /></MantineProvider>` como wrapper más externo. |
| Q5 | ¿Table o SimpleGrid para GroupsPage? | **Table** — escala mejor con muchos grupos. SimpleGrid se reserva para Dashboard. |
| Q6 | ¿W1 fix entra en slice 1? | **Sí** — 1 línea de config, riesgo bajo, beneficio inmediato (2 tests slice-2 vuelven a correr limpios). |
| Q7 | ¿Wirear todos los hooks a notificaciones en slice 1? | **No** — solo Login/Logout. Slice 2 wirea los ~15 hooks restantes con el mismo patrón. |
| Q8 | ¿Font custom? | **No** — system stack de Mantine. Inter se puede agregar después si se pide. |
| Q9 | ¿Bundle size aceptable? | **Sí** — ~150-200kb gz para un panel admin es razonable. Documentado en README. |
| Q10 | ¿Cambia Dockerfile? | **No** — slice 1 NO toca Docker. Multi-stage nginx queda para cuando se haga build de prod (futuro). |

## Próximo paso

**sdd-proposal**: definir scope exacto (REQ-1 fundación, REQ-2 dark mode, REQ-3 notifications wiring login), approach (single-pr, size:exception) y rollback (revert commit).