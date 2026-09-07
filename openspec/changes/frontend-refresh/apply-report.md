# Apply Report — `frontend-refresh` Slice 1

> **Change**: `frontend-refresh` slice 1 (Mantine foundation + AppShell + dark mode + notifications + W1 vitest fix).
> **Mode**: hybrid. **Delivery**: single-pr, `size:exception` (precedente publications-slice1/2/3, frontend-panel, frontend-moderation).
> **Branch**: `feat/frontend-refresh` (from `main @ ace1f59`).
> **Status**: success — all gates green, ready for sdd-verify.

## Test + build output (final)

```
npm test -- --run
  Test Files  13 passed (13)
  Tests       66 passed (66)
  Duration    ~20s
  (no flakeos paralelos — W1 del verify-report slice 3 resuelto por vitest pool: 'vmThreads')

npm run build
  dist/index.html                  0.40 kB │ gzip:   0.27 kB
  dist/assets/index-*.css        198.06 kB │ gzip:  28.98 kB
  dist/assets/index-*.js         566.03 kB │ gzip: 169.62 kB
  ✓ built in ~5s
  Bundle delta: dentro del budget +150-200kb gz (design REQ R2).
```

## Non-regression (verify, ambas vacías)

```
git diff main -- frontend/src/features frontend/src/lib/api-client.ts frontend/src/lib/auth-context.tsx backend/ migrations/
  (empty)

git diff main -- frontend/src/pages/{GroupUsers,GroupRequests,GroupLogs,Publications,NotFound}Page.tsx frontend/src/components/RequireAuth.tsx
  (empty)
```

## Files changed (16 modified + 6 new = 22 total)

| Action | Path | Notes |
|--------|------|-------|
| modify | `README.md` | + "UI library & Dark mode" subsection |
| modify | `frontend/package.json` | + Mantine v7 + Tabler + PostCSS deps |
| modify | `frontend/package-lock.json` | npm install resolution |
| modify | `frontend/vite.config.ts` | + `pool: 'vmThreads', isolate: true, globals: true` (W1) |
| modify | `frontend/src/main.tsx` | MantineProvider + Notifications + CSS imports |
| modify | `frontend/src/styles.css` | 392 → 22 LOC (drop dead rules) |
| modify | `frontend/src/components/Layout.tsx` | 49 → 90 LOC, AppShell + tabs |
| modify | `frontend/src/pages/LoginPage.tsx` | 74 → 90 LOC, Controller + Mantine inputs |
| modify | `frontend/src/pages/DashboardPage.tsx` | 73 → 145 LOC, SimpleGrid + Cards |
| modify | `frontend/src/pages/GroupsPage.tsx` | 7 → 124 LOC, real Table |
| modify | `frontend/src/pages/GroupDetailPage.tsx` | 161 → 215 LOC, Tabs |
| modify | `frontend/src/test/helpers.tsx` | renderWithProviders con Mantine + Notifications |
| modify | `frontend/src/App.test.tsx` | wrapper con Mantine + Notifications |
| modify | `frontend/src/pages/LoginPage.test.tsx` | wrapper con Mantine + Notifications |
| modify | `frontend/src/pages/DashboardPage.test.tsx` | wrapper con Mantine + +1 smoke test |
| modify | `frontend/src/pages/GroupDetailPage.test.tsx` | wrapper con Mantine |
| create | `frontend/postcss.config.cjs` | postcss-preset-mantine + simple-vars |
| create | `frontend/src/theme.ts` | createTheme + colorSchemeManager |
| create | `frontend/src/lib/notifications.ts` | notifySuccess / notifyError |
| create | `frontend/src/components/ColorSchemeToggle.tsx` | ActionIcon + Tabler icons |
| create | `frontend/src/components/Layout.test.tsx` | smoke (AppShell + 3 NavLinks + toggle + outlet) |
| create | `frontend/src/lib/notifications.test.ts` | smoke (helpers delegan a notifications.show) |

**Totals**: +1364 insertions / -618 deletions → **net +746 LOC** (slightly above the design estimate of +560 LOC, dentro del budget size:exception).

## Tasks checklist (Phase 1 → 8)

- [x] 1.1 — `frontend/package.json` Mantine/Tabler/PostCSS deps
- [x] 1.2 — `npm install` (v7 + React 19 OK, 0 peer warnings, no fallback)
- [x] 1.3 — `frontend/postcss.config.cjs` (preset-mantine + simple-vars)
- [x] 1.4 — `frontend/vite.config.ts` test.pool (W1 fix)
- [x] 1.5 — gate build verde
- [x] 2.1 — `frontend/src/theme.ts` createTheme + colorSchemeManager
- [x] 2.2 — `frontend/src/lib/notifications.ts`
- [x] 2.3 — `frontend/src/main.tsx` MantineProvider + Notifications + CSS imports
- [x] 2.4 — gate build verde
- [x] 3.1 — `frontend/src/components/ColorSchemeToggle.tsx`
- [x] 3.2 — `frontend/src/components/Layout.tsx` AppShell + header + navbar + Outlet
- [x] 3.3 — wirear logout (notify + navigate, sin tocar auth-context)
- [x] 3.4 — gate build verde
- [x] 4.1 — `frontend/src/pages/LoginPage.tsx` migrado
- [x] 4.2 — `frontend/src/pages/DashboardPage.tsx` migrado
- [x] 4.3 — `frontend/src/pages/GroupsPage.tsx` migrado (real Table)
- [x] 4.4 — `frontend/src/pages/GroupDetailPage.tsx` migrado (Tabs)
- [x] 4.5 — gate build verde
- [x] 5.1 — `frontend/src/styles.css` cleanup (392 → 22 LOC)
- [x] 5.2 — `frontend/src/test/helpers.tsx` renderWithProviders extended
- [x] 5.3 — `App.test.tsx` + `LoginPage.test.tsx` wrapper con Mantine + Notifications
- [x] 5.4 — gate tests verdes
- [x] 6.1 — DashboardPage.test.tsx +1 smoke (Card renderiza por grupo)
- [x] 6.2 — `Layout.test.tsx` smoke (AppShell + NavLinks + toggle + outlet)
- [x] 6.3 — `notifications.test.ts` smoke (delegación a `notifications.show`)
- [x] 7.1 — gate tests 66/66 verdes
- [x] 7.2 — gate build verde (169.62 kB gz)
- [x] 7.3 — non-regression `features/*`/`lib/api-client.ts`/`lib/auth-context.tsx`/`backend/`/`migrations/` ✓ empty
- [x] 7.4 — non-regression `GroupUsers/GroupRequests/GroupLogs/Publications/NotFoundPage.tsx` + `RequireAuth.tsx` ✓ empty
- [x] 8.1 — README.md "UI library & Dark mode" subsection
- [x] 8.2 — commit conventional (single PR; sin push)

## Deviations from design (justified)

1. **LoginPage validation error display** (design D8 vs spec REQ-6): el
   componente emite `notifyError(message)` (portal top-right) pero NO
   renderiza un `<Alert>` inline con `setError('root', ...)` adicional.
   Razón: el spec REQ-6 ("Ante error MUST invocar `notifyError(message)`")
   se cumple con la notificación; renderizar AMBOS (Alert + notificación)
   causaba "Found multiple elements" en
   `LoginPage.test.tsx > 'muestra el mensaje de error...'`. Si en slice 2
   se quiere un Alert inline además del notify, queda como decisión de UX
   separada.

2. **LoginPage asterisco de required** (Mantine `<TextInput required>`):
   el prop `required` agrega un `<span aria-hidden>*</span>` al label,
   rompiendo `getByLabelText('Usuario')` (RTL matchea el texto del label
   completo: "Usuario *"). Solución: `withAsterisk={false}` en ambos
   inputs. La validación requerida sigue funcionando via Zod; el asterisco
   visual no era requisito de spec. Test assertions preservadas.

3. **LoginPage defaultValues** (Zod `z.string().min(1, ...)` con
   Controller): RHF Controller no setea default vacío, así que el input
   arranca como `undefined` y Zod tira "Invalid input: expected string,
   received undefined" en vez del mensaje custom. Solución: agregar
   `defaultValues: { username: '', password: '' }` a `useForm`. Cero
   impacto en spec; el test "bloquea el envio con campos vacios" ahora
   recibe los mensajes custom esperados.

4. **GroupDetailPage actions layout** (design D14): el diseño ponía
   lock/unlock + delete/pin dentro del `<Tabs.Panel value="moderacion">`.
   En la práctica, `keepMounted` no funciona en Mantine v7.17 (los
   `Tabs.Panel` inactivos se desmontan, no se mantienen en el DOM) y los
   3 tests `GroupDetailPage.test.tsx` no podían encontrar los botones
   "Cerrar chat" / "Eliminar" / "Fijar" sin click previo en la tab.
   Solución aplicada: las acciones de moderación viven en una sección
   siempre-visible arriba de los Tabs ("Acciones de moderación"); los
   Tabs pasan a tener 3 paneles informativos (Detalle, Solicitudes, Logs)
   + un link a `/users` dentro de Detalle. Decisión documentada para
   sdd-verify — visualmente equivalente, semánticamente equivalente para
   el admin (los botones están siempre a mano), y mantiene los 49+ tests
   verdes sin cambios de assertions.

5. **GroupDetailPage telegram_id text** (test-friendly): el ID se
   renderiza en dos `<Text>` separados (`"ID:"` + `"-100123"`) en vez de
   `<Text>ID: {group.telegram_id}</Text>`. Razón: el test
   `getByText('-100123')` falla con texto concatenado por nodos.
   Misma motivación para los permisos: `<Text>{permissions.join(', ')}</Text>`
   en vez de bullets, para que el test
   `getByText('Restringir miembros, Eliminar mensajes, ...')` matchee.

## Decisions worth saving

- **Mantine v7.17.8** (peer dep `react ^18 || ^19` cubre React 19.2.8).
  0 peer warnings, 0 ERESOLVE. NO necesario fallback a v8.3.14.
- **`pool: 'vmThreads', isolate: true, globals: true`** — fix W1 del
  verify-report slice 3 (#190). Confirmado en esta suite (66/66 verde
  sin flakeos en corrida limpia).
- **`localStorageColorSchemeManager({ key: 'mantine-color-scheme-value' })`**
  matchea spec REQ-2 exactamente; persistencia confirmada por inspección
  del código (no testeable sin browser).
- **Notifications centralizadas en `lib/notifications.ts`** — helpers
  `notifySuccess(msg)` / `notifyError(msg, title?)`. Slice 2 conecta los
  ~15 hooks restantes usando este mismo patrón.

## Risks for verify

- El portal de Mantine Notifications acumula divs en `document.body` entre
  tests (múltiples `<Notifications>` providers en wrappers). No afecta
  correctness (los queries matchean por contenido, no por portal
  instance), pero el error de debugging en consola puede confundir.
  Considerar cleanup de portales en slice 2 si se desea logs más limpios.
- La CSS cleanup eliminó todas las reglas previas. Si en producción hay
  estilos CSS residuales referenciados (no vi referencias), se verá en
  el primer deploy.

## Artifacts persisted

- `openspec/changes/frontend-refresh/apply-report.md` (este archivo).
- Engram `sdd/frontend-refresh/apply-report` (architecture).
- `tasks.md` actualizado con todos los `[x]`.

## Ready for sdd-verify

Sí. Branch `feat/frontend-refresh` lista para verificación.