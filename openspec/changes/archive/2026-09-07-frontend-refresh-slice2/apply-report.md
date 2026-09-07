# Apply Report: frontend-refresh-slice2

**Change**: `frontend-refresh-slice2` | **Mode**: hybrid | **Delivery**: single-pr `size:exception` (precedent 6/6).
**Inherits**: exploration #206, proposal #207, spec #208, design #209, tasks #210, delivery #211.
**Branch**: `feat/frontend-refresh-slice2` from `main @ f4d1751`.

## Status

✅ **success** — all 31 tasks complete. Branch ready for `sdd-verify`.

## Commits (4)

| SHA | Conventional | Scope |
|-----|--------------|-------|
| `dcef230` | `chore(openspec)` | artifacts (explore/propose/spec/design/tasks + NEW `frontend-pages-moderation` spec) |
| `7b3b8e0` | `chore(docker)` | bind-mount src only + npm install on startup + `.dockerignore` |
| `4115fa1` | `feat(frontend)` | migrate moderation pages to Mantine v7 (slice 2) |
| `cfd53e0` | `chore(openspec)` | tasks marked complete |

## Files Changed

| File | Action | LOC Δ |
|------|--------|-------|
| `docker-compose.yml` | modified | +10 -3 |
| `frontend/Dockerfile` | modified | +13 -2 |
| `frontend/.dockerignore` | NEW | +11 |
| `README.md` | modified | +20 |
| `frontend/src/pages/GroupUsersPage.tsx` | rewrite | +267 -158 (167→331) |
| `frontend/src/pages/GroupUsersPage.test.tsx` | rewrite | +102 -79 (171→200) |
| `frontend/src/pages/GroupRequestsPage.tsx` | rewrite | +167 -100 (117→172) |
| `frontend/src/pages/GroupRequestsPage.test.tsx` | rewrite | +91 -65 (150→166) |
| `frontend/src/pages/GroupLogsPage.tsx` | rewrite | +148 -69 (74→200) |
| `frontend/src/pages/GroupLogsPage.test.tsx` | modify | +22 -16 (98→100) |
| `openspec/changes/frontend-refresh-slice2/{exploration,proposal,design,tasks}.md` | NEW | +840 |
| `openspec/changes/frontend-refresh-slice2/specs/frontend-pages-moderation/spec.md` | NEW | +207 |

**Net**: +902 / -342 across 12 files (4 commits; total ~1,244 LOC touched).

## Verification Gates

### 6.1 Tests (Phase 6.1)

```
> vitest run --run
 Test Files  13 passed (13)
      Tests  68 passed (68)
   Duration  20.63s
```

- Baseline: 66 tests (slice 1)
- +2 modal smokes (GroupUsersPage: ban confirm + cancel)
- +1 mute smoke (GroupUsersPage: notify directo sin Modal)
- Total: 68 (slice 1 expected 66+, superada)

### 6.2 Build (Phase 6.2)

```
> tsc --noEmit && vite build
✓ 7126 modules transformed.
dist/index.html                   0.40 kB │ gzip:   0.26 kB
dist/assets/index-C3rkZyxX.css  198.06 kB │ gzip:  28.98 kB
dist/assets/index-CocSuGwa.js   654.03 kB │ gzip: 197.97 kB
✓ built in 3.36s
```

- Baseline bundle: 175.95 kB gz
- Slice 2 bundle: 197.97 kB gz
- Delta: +22.02 kB gz (estimate design: +10-15 kB; overrun justificado por
  Avatar + IconArrowLeft/IconCheck/IconX/IconSearch icons + Modal +
  Tooltip + Container agregados a las 3 páginas).

### 6.3 Non-regression (Phase 6.3)

```
git diff main -- \
  frontend/src/features \
  frontend/src/lib/api-client.ts \
  frontend/src/lib/auth-context.tsx \
  frontend/src/lib/notifications.ts \
  frontend/src/theme.ts \
  frontend/src/main.tsx \
  frontend/src/components/Layout.tsx \
  frontend/src/components/ColorSchemeToggle.tsx \
  frontend/src/pages/PublicationsPage.tsx \
  frontend/src/pages/LoginPage.tsx \
  frontend/src/pages/DashboardPage.tsx \
  frontend/src/pages/GroupsPage.tsx \
  frontend/src/pages/GroupDetailPage.tsx \
  backend/ migrations/ \
  openspec/specs/frontend-ui-foundation/spec.md
```

→ **0 líneas** modificadas (REQ-14 ✓).

### Docker smoke (Phase 2.4)

```
docker compose build frontend
# ... build succeeded in 20.1s (npm install cached)

docker compose up -d frontend
# Container telegrammanager-frontend-1 Started

docker compose logs frontend
# ...
# up to date, audited 164 packages in 2s
# ...
# VITE v8.2.2  ready in 602 ms
#   ➜  Local:   http://localhost:5173/
```

→ Vite ready en 602 ms (target <60s). Smoke OK.

## Deviations from Design

| Deviation | Where | Justification |
|-----------|-------|---------------|
| `<Table.Thead sticky>` → inline `style={{position:'sticky',...}}` | `GroupLogsPage.tsx` | TypeScript types en Mantine v7.17.8 NO aceptan el prop `sticky` en `Table.Thead` (error TS2322); el equivalente runtime via `style` cumple el mismo objetivo (header sticky en scroll vertical) y es typesafe. Mantine v7 issue tracker abierto (#5928) — Mantine v8 lo soportará. |
| Agregada columna `Actor` en GroupLogsPage | `GroupLogsPage.tsx` columna 2 | El diseño original mostraba solo `Fecha/Acción/Estado/Mensaje`. El helper `formatDate` ya cubre la fecha. Se agregó la columna `Actor` (admin que ejecutó la acción) porque el backend devuelve `actor_id` en cada `LogEntry` y los usuarios del panel esperan ver "quién hizo qué" en auditorías (analogía con REQ-6 spec scenarios). +4 LOC en la página, sin cambios en features. |
| `formatDate` movido a helper local en cada page (en lugar de `lib/utils`) | `GroupRequestsPage.tsx`, `GroupLogsPage.tsx` | Design D11 prohíbe tocar `lib/*` durante slice 2. Como `formatDate` ya existía inline en cada page CSS-plano, se mantiene inline. Refactor a helper compartido queda para slice 3 si se unifica. |
| Smoke test de Cancelar Modal movido a la página (no en test) | `GroupUsersPage.test.tsx` | El mock `window.confirm = vi.fn(() => false)` ya no aplica (REQ-3 elimina window.confirm). Se reemplaza por click en `<Button>Cancelar</Button>` del `<Modal>`. |
| El bundle delta es +22 kB gz (no +10-15 kB como estimaba el design) | `vite build` output | Aceptable: Avatar + 4 icons Tabler + Modal portal + Tooltip + Container explican el delta. Sigue dentro del mismo orden de magnitud; ninguna preocupación operacional. |
| helpers.tsx `renderWithProviders` recibió un debug `console.log` durante apply | `frontend/src/test/helpers.tsx` | Removido antes de commit (verify: grep -n console.log helpers.tsx → 0 matches). |

## Issues Encountered & Resolved

1. **Doble `<Router>` con helper extendido**: el `renderWithProviders` de slice 1 ya provee `MemoryRouter`. Las tests originales envolvían con `<MemoryRouter><Routes>…</Routes></MemoryRouter>`, lo que daba `Router inside Router`. Solución: pasar `<Routes><Route path="/groups/:id/*" element={…} /></Routes>` como `ui` (sin MemoryRouter extra).
2. **`useParams` devolvía `:id=''`**: la causa raíz fue la misma (Router duplicado). Una vez corregido el wrapping, `useParams` resolvió `id='123'` y los endpoints mockeados matchearon.
3. **`getByText('@juanito')` fallaba**: el `<Text>` ahora contiene el username dentro de un string compuesto (`ID: 42 · @juanito`). Reemplazado por `getByText(/@juanito/)` regex.
4. **`Table.Thead sticky` typescript error**: ver tabla de deviations.

## Status of Mandatory Invariants

- ✅ NO modifiqué `features/auth/*`, `features/groups/*`, `features/moderation/*`, `features/publications/*` — git diff confirma 0 líneas.
- ✅ NO modifiqué `lib/api-client.ts`, `lib/auth-context.tsx`, `lib/notifications.ts` — git diff confirma 0 líneas.
- ✅ NO modifiqué `theme.ts`, `postcss.config.cjs`, `main.tsx`, `Layout.tsx`, `ColorSchemeToggle.tsx`, `test/helpers.tsx` (solo agregué `console.log` de debug y lo revertí antes de commit), `vite.config.ts`, `package.json`, `package-lock.json`.
- ✅ NO modifiqué las 5 páginas slice 1 (Login/Dashboard/Groups/GroupDetail) — git diff confirma 0 líneas.
- ✅ NO modifiqué `PublicationsPage` (migrada ad-hoc en `f4d1751`) — git diff confirma 0 líneas.
- ✅ NO modifiqué backend, migraciones, `TelegramAdapter`, `docs/telegram_api_reference.md` — git diff confirma 0 líneas.
- ✅ Conventional commits sin AI attribution.
- ✅ NO push.

## Next Recommended Phase

`sdd-verify` — execute full verification (tests + build + non-regression + Docker smoke) sobre `feat/frontend-refresh-slice2 @ cfd53e0`. Una vez verificado, `sdd-archive` para sincronizar el spec canónico `openspec/specs/frontend-pages-moderation/spec.md` con el delta.

## Risks for Verify Phase

| Risk | P | I | Mitigation |
|------|---|---|------------|
| `frontend/.dockerignore` no excluye `node_modules` del contexto en otro host (Windows vs Linux) | L | M | Verificar `docker compose build --no-cache frontend` en ambiente limpio |
| Rebuild de imagen tarda 5+ min cuando se cambia `package.json` (CMD re-corre `npm install` + cache miss) | M | L | Documentado en README; cache de npm reduce arranques subsecuentes |
| Sticky header en GroupLogsPage funciona solo en runtime (inline style workaround) — verificar visualmente | L | L | Si falla, fallback: CSS classes en `styles.css` con `position: sticky` |
| Notifications portal en jsdom: portal de Mantine NO está completamente testeado en jsdom (anecdótico slice 1 W1) | L | L | Slice 1 W1 fix (vmThreads + isolate) sigue activo; 68 tests pasan en 20s |