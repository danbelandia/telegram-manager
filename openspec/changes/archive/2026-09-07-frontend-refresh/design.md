# Design: frontend-refresh — Slice 1 (Mantine foundation)

> **Change**: `frontend-refresh` slice 1 | **Mode**: hybrid | **Delivery**: single-pr, `size:exception` | **Inherits**: exploration #193, proposal #194, spec #195.

## Technical Approach

Replace the ad-hoc view layer (CSS plano 395 LOC + `<div>` layout) with Mantine v7 + AppShell + dark mode + global notifications. Wire notifications ONLY into Login/Logout (slice 2 wires the ~15 remaining hooks). Fix Vitest W1 (`pool: 'vmThreads'`) as a bonus. **Do not touch `features/*`, `lib/*`, backend, migrations, Docker.** All decisions resolve every REQ of `specs/frontend-ui-foundation/spec.md`.

## Architecture Decisions

| # | Topic | Choice | Alternatives | Rationale |
|---|-------|--------|--------------|-----------|
| D1 | UI lib | `@mantine/core@^7.17.0` + `@mantine/hooks@^7.17.0` + `@mantine/notifications@^7.17.0` | v8 (heavier), v9 (breaking), Chakra, MUI | Spec locks v7. Peer dep `react ^18 \|\| ^19` covers 19.2.8; context7-verified. Fallback to v8.3.14 only on `ERESOLVE`. |
| D2 | Icons | `@tabler/icons-react@^3.0.0` | Phosphor, lucide | Official Mantine pairing; tree-shakeable. |
| D3 | PostCSS | `postcss-preset-mantine@^1.17.0` + `postcss-simple-vars@^7.0.0` (spec typo `mante` → corrected) | none | Mantine v7 `light-dark()` + breakpoints require this; otherwise theme silently breaks. |
| D4 | Theme | `createTheme({ primaryColor: 'blue', defaultRadius: 'md' })` + system stack via CSS var `--mantine-font-family`; `defaultColorScheme: 'auto'` | Custom `brand` tuple | Spec locks `primaryColor: 'blue'` (Mantine built-in, `#228be6`). Visual delta vs old `#3b82f6` documented in R7; no custom palette (scope creep). |
| D5 | Color scheme manager | `localStorageColorSchemeManager({ key: 'mantine-color-scheme-value' })` | cookie, in-memory | Spec locks key. localStorage applies pre-paint (no flicker on Vite SPA). |
| D6 | Layout | Mantine `<AppShell header={h:60} navbar={w:260, breakpoint:'sm'}>` with `<Outlet/>` in `<AppShell.Main>` | keep `<div>` grid | AppShell solves responsive (drawer <sm) for free; `<Outlet/>` preserves `frontend-routing` contract. |
| D7 | Wrap order | `<MantineProvider><Notifications/></MantineProvider>` wrapping inside existing `<QueryClientProvider><MemoryRouter>` | swap order | Provider must be above every Mantine consumer; below Router is fine because LoginPage lives outside AppShell. |
| D8 | Forms | Keep RHF + zod; connect Mantine inputs via `<Controller>` | migrate to `@mantine/form` | zod schema already published; `@mantine/form` doesn't replace zod cleanly. Controller wraps any Mantine input. |
| D9 | Wiring slice 1 | Only `LoginPage` (success→`notifySuccess('Bienvenido')`) + `Logout` (`notifySuccess('Sesión cerrada')` before navigate) | wire 15 hooks now | Slice 2 wires the rest with same pattern; mixing visual + mutation feedback = scope creep. |
| D10 | Logout notify | New wrapper `handleLogout` in `Layout.tsx` calls `await logout()`, then `notifySuccess`, then `navigate('/login', {replace:true})` | mutate logout() to notify | `lib/auth-context.tsx` is OUT of scope per spec REQ "no regresión"; notify at call-site only. |
| D11 | Layout test wrapping | Add `<MantineProvider><Notifications/></MantineProvider>` into `renderWithProviders` + refactor `App.test.tsx` + `LoginPage.test.tsx` to use it | keep inline wrappers | Spec REQ requires `renderWithProviders` extension. App.test.tsx & LoginPage.test.tsx become 1-line wrapper swaps; assertions unchanged. |
| D12 | Vitest W1 | `vite.config.ts`: add `test: { pool: 'vmThreads', isolate: true, globals: true }` (keep `environment`/`setupFiles`) | defer | Verify-report slice 3 #190 explicit recommendation. Low risk (each test creates own QueryClient). |
| D13 | Groups page | Implement real `<Table>` per spec REQ (not alias of DashboardPage); columns: Título, Tipo, Miembros, Bot (Badge), Acciones | keep DashboardPage alias | Spec REQ "GroupsPage MUST usar `<Table>`". Decouples table view from dashboard cards. |
| D14 | GroupDetail tabs | 4 `<Tabs.Panel>`: "Detalle" (dl + permissions), "Membresía y moderación" (lock/unlock + delete/pin + button→`/users`), "Solicitudes" (text + button→`/requests`), "Logs" (text + button→`/logs`) | inline full sub-pages | Spec REQ mandates 4 tabs. Sub-pages live on separate routes; tabs provide discoverable UX without duplicating logic. |
| D15 | CSS cleanup | `styles.css`: drop `app-layout`, `sidebar*`, `header*`, `login-page`, `login-form`, `form-field`, `form-error*`, `btn*`, `state-block`, `group-list`, `group-card*`, `group-detail`, `detail-actions`, `group-sections` (all unused after migration). Keep `:root` CSS vars (still referenced), `body`, `*`, `.not-found` | full purge | Grep-verifiable; conservative keeps. |
| D16 | Docs | Add "UI library & Dark mode" subsection to root `README.md` (no `frontend/README.md` exists). | create `frontend/README.md` | Spec REQ mentions both; root is enough for slice 1. |

## File Changes

| File | Action | ~LOC | Notes |
|------|--------|------|-------|
| `frontend/package.json` | modify | +12 | D1+D2+D3 deps. No bumps to React/Vite/RHF/Zod. |
| `frontend/postcss.config.cjs` | create | ~17 | D3. Required by Mantine v7. |
| `frontend/src/theme.ts` | create | ~15 | `createTheme` + `MantineColorsTuple` re-export + `localStorageColorSchemeManager`. |
| `frontend/src/lib/notifications.ts` | create | ~12 | D9. `notifySuccess(msg)`, `notifyError(msg, title?)`. |
| `frontend/src/components/Layout.tsx` | replace | ~90 | D6+D10. AppShell + Header (logo + ColorSchemeToggle + Logout) + Navbar (3 NavLinks with Tabler icons) + `<Outlet/>`. Preserves RequireAuth wrapping. |
| `frontend/src/components/ColorSchemeToggle.tsx` | create | ~22 | ActionIcon with `useMantineColorScheme` + `IconSun`/`IconMoon`. |
| `frontend/src/main.tsx` | modify | ~30 | Import `@mantine/core/styles.css` + `@mantine/notifications/styles.css` first; wrap with `<MantineProvider theme={mantineTheme} defaultColorScheme="auto" colorSchemeManager={...}>` + `<Notifications position="top-right" zIndex={2077} limit={5}/>`. Order: `MantineProvider` > `Notifications` inside existing tree. |
| `frontend/src/pages/LoginPage.tsx` | modify | ~75 | D8+D9. `<Center><Paper><Stack><TextInput label="Usuario"/><PasswordInput label="Contraseña"/><Button type="submit">` via `<Controller>`. zod schema unchanged. onSuccess: `notifySuccess('Bienvenido')` + `navigate(from, {replace:true})`. onError: `notifyError(message)`. |
| `frontend/src/pages/DashboardPage.tsx` | modify | ~60 | `<SimpleGrid cols={{base:1, sm:2, lg:3}}>` of `<Card>` per group (Title, Group with Type/Miembros/Bot status, `<Button component={Link}>`→detail). `<Skeleton>` while `isPending`; `<Text>` empty state. |
| `frontend/src/pages/GroupsPage.tsx` | modify | ~60 | D13. Real `<Table>` (thead + tbody); each row `<Badge color={statusColor}>` + `<Button variant="subtle" component={Link}>`. `<Skeleton>` rows when loading. |
| `frontend/src/pages/GroupDetailPage.tsx` | modify | ~120 | D14. `<Title order={2}>` with `{group.title}` + `<Text size="sm">{group.telegram_id}</Text>` + `<Badge>` for bot status. `<Tabs defaultValue="detalle">` with 4 panels. Permissions rendered via `<Group>` + `<Stack>`. Keep RHF-free mutations; reuse existing hooks. |
| `frontend/src/test/helpers.tsx` | modify | ~38 | D11. `renderWithProviders` adds `<MantineProvider theme={mantineTheme}><Notifications/></MantineProvider>` inside existing `<QueryClientProvider>`. Imports `mantineTheme` from `../theme`. |
| `frontend/src/styles.css` | modify | ~80 | D15. Strip dead rules; keep `:root` vars + body reset + `.not-found`. |
| `frontend/src/App.test.tsx` | modify | ~22 | Swap inline wrapper to `renderWithProviders`; assertions unchanged. |
| `frontend/src/pages/LoginPage.test.tsx` | modify | ~25 | Swap inline wrapper to `renderWithProviders`; assertions unchanged. |
| `frontend/src/pages/DashboardPage.test.tsx` | modify | +10 | Add 1 smoke: "renderiza Card por grupo". Existing 3 tests unchanged. |
| `frontend/src/components/Layout.test.tsx` | create | ~30 | Smoke: renders header brand + 3 NavLinks + Outlet content. Mock `useAuth` with user. |
| `frontend/src/lib/notifications.test.ts` | create | ~25 | Smoke: `notifySuccess('x')` + `notifyError('y')` no throw; spy on `notifications.show`. |
| `frontend/vite.config.ts` | modify | +5 | D12. Add `pool: 'vmThreads'`, `isolate: true`, `globals: true` to existing `test`. |
| `README.md` | modify | +10 | Add "UI library & Dark mode" subsection. |

**Totals**: ~+740 LOC, ~-180 LOC → net ~+560 LOC. Within `size:exception` budget.

## Non-regression surface (MUST NOT change)

`features/auth/{api,types}.ts`, `features/groups/{api,hooks,permissions,types}.ts`, `features/moderation/{api,error,hooks,types}.ts`, `features/publications/{api,ButtonsEditor,error,hooks,types,validateScheduledAtClient}.ts`, `lib/api-client.ts`, `lib/auth-context.tsx`, `lib/__mocks__/*`, `pages/{GroupUsersPage,GroupRequestsPage,GroupLogsPage,PublicationsPage,NotFoundPage}.tsx`, `components/RequireAuth.tsx`, `backend/**`, `migrations/**`, `docker-compose.yml`, `frontend/Dockerfile`, `frontend/index.html`, `.env.example`.

## Open Questions (resolved)

| # | Question | Resolution |
|---|----------|------------|
| Q1 | Mantine primary exact blue? | Use built-in `blue` per spec; visual delta documented in R7. |
| Q2 | AppShell sizes? | `header.h=60`, `navbar.w=260, breakpoint='sm'`. Matches wireframe AGENTS §16. |
| Q3 | Navbar route set? | Exactly `/dashboard`, `/groups`, `/publications` (mirrors current). Sub-routes stay inside pages. |
| Q4 | Notifications wiring scope? | Login (success) + Logout only. Slice 2 wires ~15 remaining hooks. |
| Q5 | CSS modules / Tailwind? | Neither. Mantine styles + minimal overrides. |
| Q6 | Font custom? | System stack via Mantine default. |
| Q7 | Dockerfile? | Unchanged. |

## Testing Strategy

| Layer | Coverage | Approach |
|-------|----------|----------|
| Existing 49+ tests | Must stay green | Wrapper refactor (`renderWithProviders`) covers App/LoginPage/DashboardPage/etc. Assertions preserved. |
| New smoke tests | 3 files | `Layout.test.tsx`, `notifications.test.ts`, +1 in DashboardPage. |
| W1 fix | `pool: 'vmThreads'` | Eliminates 2 slice-2 flakeos reported in `sdd/publications-slice3/verify-report` #190. |

## Migration / Rollout

**No data migration.** Single revertable PR. Steps: install deps → swap layout → migrate pages → extend tests → commit. Verify with `npm run build` (must succeed; bundle delta ~+150-200kb gz) + `npm test -- --run` (all green, zero flakeos). Rollback = `git revert <merge>`.

## Risk Table

| # | Risk | Probability | Impact | Mitigation |
|---|------|-------------|--------|------------|
| R1 | Peer dep `ERESOLVE` Mantine v7 + React 19 | Low | High | Bump to `@mantine/*@^8.3.14` (1-line change, same API surface). |
| R2 | Bundle +150-200kb gz | Certain | Low | Acceptable for admin panel; lazy-load deferred to slice future. |
| R3 | W1 `pool: 'vmThreads'` breaks tests with shared state | Low | Medium | Each test instantiates own `QueryClient`; verified by reading helpers. |
| R4 | CSS cleanup leaves dead rules | Medium | Low | Grep every removed selector across `src/**/*.{tsx,ts}` before commit. |
| R5 | AppShell responsive untested in jsdom | Certain | Low | jsdom ignores breakpoint; smoke covers desktop, real responsive validated in browser. |
| R6 | Notifications portal in jsdom | Low | Low | Mantine v7 uses `document.body` automatically; verified in context7. |
| R7 | `primaryColor: 'blue'` (Mantine `#228be6`) ≠ old `#3b82f6` | Certain | Cosmetic | Documented in README. Custom palette if user complains. |
| R8 | Scope creep (wire all hooks slice 1) | Low | Medium | Spec D9 + REQ explicit. Slice 2 already scheduled. |

## Interfaces / Contracts

```ts
// frontend/src/theme.ts
import { createTheme, localStorageColorSchemeManager, type MantineColorsTuple } from '@mantine/core'
export const mantineTheme = createTheme({
  primaryColor: 'blue',
  defaultRadius: 'md',
  fontFamily: 'var(--mantine-font-family)',
})
export const colorSchemeManager = localStorageColorSchemeManager({ key: 'mantine-color-scheme-value' })

// frontend/src/lib/notifications.ts
import { notifications } from '@mantine/notifications'
export function notifySuccess(message: string): void { notifications.show({ color: 'green', message }) }
export function notifyError(message: string, title?: string): void { notifications.show({ color: 'red', title, message }) }
```

## Spec REQ → Design Section Map

| Spec REQ | Resolved by |
|----------|-------------|
| Mantine stack additions | D1+D2+D3, package.json, postcss.config.cjs, main.tsx imports |
| Theme central | D4+D5, theme.ts, main.tsx provider |
| AppShell layout | D6, Layout.tsx (RequireAuth wrapping unchanged) |
| Dark mode toggle | ColorSchemeToggle.tsx, D5 persistence |
| Notifications global | main.tsx, notifications.ts (D9) |
| Login migrado | LoginPage.tsx (D8+D9) |
| Dashboard migrado | DashboardPage.tsx |
| Groups migrado | GroupsPage.tsx (D13) |
| GroupDetail migrado | GroupDetailPage.tsx (D14) |
| Logout con notificación | Layout.tsx handleLogout (D10) |
| Helpers de test | helpers.tsx (D11) |
| Smoke tests por página | Layout.test.tsx, notifications.test.ts, DashboardPage.test.tsx extension |
| Documentación | README.md (D16) |
| No regresión | Non-regression surface section |
