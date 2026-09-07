# Proposal: frontend-refresh-slice2 — 3 Páginas de Moderación + Docker Dev Fix

> **Change**: `frontend-refresh-slice2`. **Mode**: hybrid. **Delivery**: single-pr con `size:exception` (precedente 5/5 — re-confirmado en `sdd-tasks`). **Spec**: NEW `frontend-pages-moderation`. **Branch base**: `main @ f4d1751` (ver corrección al final).

## Intent

Tras slice 1 (foundation Mantine v7 + AppShell + dark mode + Notifications — merged `700681c`) las páginas de moderación `GroupUsersPage`, `GroupRequestsPage` y `GroupLogsPage` quedaron **rota visualmente** porque `styles.css` se limpió en slice 1 y eliminó las reglas que esas páginas seguían usando (`btn`, `data-table`, `badge-*`, `state-block`, `back-link`). Además el `docker-compose.yml` tiene un **bug heredado**: el volumen anónimo `node_modules:/app/node_modules` pisa el `node_modules` de la imagen con un volumen vacío, rompiendo Vite en Windows/OneDrive. Slice 2 cierra ambas brechas: migra las 3 páginas a Mantine + wiring de notifications + reemplaza `window.confirm` por `<Modal>` para acciones destructivas irreversibles, y corrige el bug de docker-compose como housekeeping.

## Scope

### In Scope

- **Migración visual** de `GroupUsersPage`, `GroupRequestsPage`, `GroupLogsPage` a componentes Mantine v7 (Cards + SimpleGrid, Table con sticky header, Badge, TextInput, Alert).
- **Wiring de notifications** en mutaciones destructivas: `useBanUser`, `useUnbanUser`, `useMuteUser`, `useUnmuteUser`, `useApproveJoinRequest`, `useRejectJoinRequest` — SOLO en call-sites `pages/*` (no en `features/moderation/hooks.ts`).
- **Reemplazo de `window.confirm()`** por `<Modal>` Mantine **únicamente para `Banear`** (acción irreversible, AGENTS §4). `Mutear`/`Desmutear`/`Desbanear`/`Aprobar`/`Rechazar` son clic directo con `notifySuccess` (reversibles).
- **Docker dev fix**: remover volumen anónimo `node_modules:/app/node_modules`; bind-mount `./frontend/src:/app/src` (solo código fuente); `CMD ["sh","-c","npm install && npm run dev -- --host 0.0.0.0"]`; crear `frontend/.dockerignore`; nota en `README.md`.
- **Tests**: extender `renderWithProviders` (ya provee Mantine+Notifications en slice 1) — sin nuevos wrappers; ajustar selectores de `GroupRequestsPage.test.tsx` que matchean texto de notifications ahora en portal; +1 smoke por página validando el cambio nuevo (Modal + notifications).

### Out of Scope

- **PublicationsPage**: ya migrada ad-hoc en commit `f4d1751`. Verificado — no tocar.
- **`features/auth/*`, `features/groups/*`, `features/moderation/*`, `features/publications/*`**: **0 cambios** (invariante slice 1).
- **`lib/api-client.ts`, `lib/auth-context.tsx`**: **0 cambios**.
- **`lib/notifications.ts`**: helpers `notifySuccess`/`notifyError` solo **USADOS** vía call-sites — sin modificaciones.
- **GroupLogsPage filtros / paginación** (S2-D9 diferido): requiere cambios de backend (`?limit&offset&action=&status=`), fuera de scope housekeeping+migration.
- **Backend, migraciones, TelegramAdapter, contrato API**: **0 cambios**.
- **Mantine version bump** (v7→v8): no requerido; v7.17.8 + React 19.2.8 = 0 peer warnings (slice 1 verify #201).
- **Re-attempt 4-tab layout en GroupDetailPage** (W-V1 de slice 1 verify #201): workaround diferido.

## Capabilities

### New Capabilities

- **`frontend-pages-moderation`**: NEW spec cubriendo REQs de view layer de las 3 páginas migradas (Modal destructivo, sticky header en logs, badges de status, notifications wiring en call-sites). Análogo a `frontend-ui-foundation` (slice 1) pero scoped a pages específicas — sync method = copia verbatim del delta al canónico `openspec/specs/frontend-pages-moderation/spec.md`.

### Modified Capabilities

- None. Las migraciones son implementation detail de `frontend-moderation` (que describe el dominio, no la presentación). Los REQs de dominio ya están cumplidos.

## Approach

20 decisiones en exploration (`S2-D1`…`S2-D20`). Highlights:

| # | Decisión | Por qué |
|---|----------|---------|
| S2-D2 | `GroupUsersPage`: `<Card>` por admin en `<SimpleGrid cols={{base:1,sm:2}}>` | Tabla over-engineered para 1-50 admins con 4 acciones/fila; Cards responsive consistentes con DashboardPage |
| S2-D3/D4 | Modal SOLO para `Banear`; mute/unban/unmute clic directo + notify | AGENTS §4: confirmación obligatoria solo en destructivas irreversibles |
| S2-D5/D6 | `GroupRequestsPage`: `<Table>` Mantine + Badge; approve/reject clic directo (reversible) | Lista naturalmente tabular; test existente sigue matcheando |
| S2-D7 | Reemplazar `<p className="state-block state-ok">` por `notifySuccess` | Consistencia con LoginPage slice 1 — notifications globales, no banners inline duplicados |
| S2-D8 | `GroupLogsPage`: `<Table.Thead sticky>` + `created_at DESC` (backend ya devuelve DESC) | Logs append-only, sticky header mejora UX con 50+ entradas |
| S2-D11 | Wirear `notifySuccess`/`notifyError` SOLO en call-sites, NO en `features/moderation/hooks.ts` | Preserva invariante slice 1; `lib/notifications.ts` sigue siendo único punto de cambio de formato |
| S2-D13 | `CMD ["sh","-c","npm install && npm run dev -- --host 0.0.0.0"]` | `npm install` en build queda escondido por bind-mount; en arranque garantiza deps correctas (acepta +10-30s arranque) |
| S2-D14 | Bind-mount `./frontend/src:/app/src` (no `./frontend:/app`) | Evita pisar `package.json`/`vite.config.ts`/`node_modules` del host; hot reload sigue funcionando vía Vite watching `src/` |
| S2-D15 | Eliminar `node_modules:` del bloque `volumes:` raíz | Volumen nombrado huérfano — ya no se necesita |
| S2-D16 | `frontend/.dockerignore`: `node_modules`, `dist`, `.env`, `.git`, `.vite` | Build context chico + `npm install` determinista en build |

Texto de notifications en español consistente (`'Usuario baneado'`, `'Solicitud aprobada'`, etc.) usando `formatModerationError` para errores. Cobertura de tests: `renderWithProviders` slice 1 ya provee `<MantineProvider><Notifications/></MantineProvider>`; slice 2 solo ajusta selectores que matchean texto de notifications ahora en portal Mantine.

## Affected Areas

### Modificados (frontend)

| File | Action | ~Δ LOC |
|------|--------|--------|
| `frontend/src/pages/GroupUsersPage.tsx` | rewrite | ~165 → ~180 |
| `frontend/src/pages/GroupRequestsPage.tsx` | rewrite | ~117 → ~110 |
| `frontend/src/pages/GroupLogsPage.tsx` | rewrite | ~74 → ~95 |
| `frontend/src/pages/GroupUsersPage.test.tsx` | adjust | ~171 → ~190 |
| `frontend/src/pages/GroupRequestsPage.test.tsx` | adjust | ~150 → ~155 |
| `frontend/src/pages/GroupLogsPage.test.tsx` | wrapper swap | ~98 → ~100 |
| `frontend/.dockerignore` | NEW | ~6 |
| `README.md` | +12 LOC | ~302 → ~314 |

### Modificados (devops)

| File | Action | ~Δ LOC |
|------|--------|--------|
| `docker-compose.yml` | modify | 72 → 70 |
| `frontend/Dockerfile` | modify (CMD) | 15 → 16 |

### NO modificados (invariantes slice 1)

`features/**`, `lib/api-client.ts`, `lib/auth-context.tsx`, `lib/notifications.ts`, `theme.ts`, `postcss.config.cjs`, `main.tsx`, `Layout.tsx`, `ColorSchemeToggle.tsx`, `test/helpers.tsx`, `test/setup.ts`, `vite.config.ts`, `package.json`, `package-lock.json`, páginas migradas en slice 1, backend, migraciones.

## Risks

| Risk | P | I | Mitigation |
|------|---|---|------------|
| `.dockerignore` no excluye `node_modules` → context de build contaminado | M | M | Crear `.dockerignore` ANTES del cambio; `docker compose build --no-cache frontend` en primer arranque |
| Cambio de `CMD` agrega 10-30s al arranque | C | L | Documentado en README; aceptable para dev |
| Cambio de bind-mount `./frontend:/app` → `./frontend/src:/app/src` requiere `docker compose build frontend` para `vite.config.ts`/`package.json` | M | M | README: "cambios en deps requieren rebuild" |
| Notifications portal acumula en jsdom — `findByText(/decisión enviada/i)` matchea contra `document.body` | L | L | Slice 1 verify #201 ya validó este comportamiento en jsdom |
| Selector de ban test se rompe al pasar de `window.confirm` a `<Modal>` | M | M | Test actualizado: mock de `window.confirm` → `click` en `<Button>"Cancelar"` del Modal; smoke cubre el flujo nuevo |
| Reseción de tests con `renderWithProviders` cambia orden de providers | L | L | Slice 1 ya validó que wrapper extendido no rompe 49+ tests |

## Rollback Plan

Revert del PR único (`git revert <merge>`). No hay migración de BD ni cambios de contrato API. `git checkout main && docker compose down && docker compose build frontend` recupera estado limpio. Para Docker: `git revert` restaura `docker-compose.yml` y `Dockerfile`; `docker compose build --no-cache frontend` regenera la imagen con el bind-mount anterior (operativo, solo con el bug original del volumen anónimo conocido — aceptable como estado de rollback temporal).

## Dependencies

- `main @ f4d1751` (slice 1 merge `700681c` + PublicationsPage ad-hoc `f4d1751`).
- React 19.2.8 + Vite 8 + TS 7 + RHF 7 + Zod 4 + Mantine v7.17.8 + Tabler v3.46.0 (sin cambios).
- Helpers `notifySuccess`/`notifyError` en `lib/notifications.ts` (slice 1).
- `renderWithProviders` extendido en `test/helpers.tsx` (slice 1).

## Success Criteria

- [ ] `npm run build` verde; bundle delta +10-15 kB gz (3 páginas + Modal + Table sticky).
- [ ] `npm test -- --run` 100% verde, 0 flakeos paralelos (W1 slice 1 resuelto).
- [ ] `GroupUsersPage` renderiza con Cards+SimpleGrid; `Banear` muestra Modal con texto destructivo y notifica éxito/error.
- [ ] `GroupRequestsPage` renderiza con Table+Badge; approve/reject notifica sin Modal.
- [ ] `GroupLogsPage` renderiza con Table sticky; badge por status; `created_at DESC`.
- [ ] Notifications wiring funciona en ban/unban/mute/unmute/approve/reject via call-sites.
- [ ] `docker compose up --build` levanta sin errores; Vite ready en <60s.
- [ ] `frontend/.dockerignore` excluye `node_modules`/`dist`/`.env`/`.git`/`.vite`.
- [ ] README documenta el cambio de bind-mount y la nota "cambios en deps requieren rebuild".
- [ ] `git diff main -- features/ lib/api-client.ts lib/auth-context.tsx backend/ migrations/` → **0 líneas** (no-regresión).

## Branch Base Correction

> **Discrepancia con exploration (`S2-D20`)**: la exploración asumió que `feat/frontend-refresh` seguía sin mergear y propuso branchear desde ahí. **El estado real** es:
>
> - Slice 1 mergeado a `main` como commit **`700681c`**.
> - PublicationsPage ad-hoc mergeado como **`f4d1751`** (rama `feat/publications-page-mantine` ya mergeada).
> - `feat/frontend-refresh` **ya no existe** como branch separada — `main` está en `f4d1751`.
>
> **Decisión**: branchear **`feat/frontend-refresh-slice2`** desde **`main @ f4d1751`** (current tip). Esto:
>
> 1. Evita duplicación de historia (no cherry-pick de slice 1).
> 2. Mantiene el branch dedicada para review independiente (slice 2 vs slice 1 ya cerrado).
> 3. Permite rebase limpio si `main` recibe más merges durante apply (publications-slice4, etc.).
>
> La exploración del usuario debe actualizarse como housekeeping (`mem_update` sobre `sdd/frontend-refresh-slice2/exploration`, campo `S2-D20`) — fuera de scope del proposal mismo.

## Open Questions

Ninguna. La exploración resolvió las decisiones abiertas; la corrección de branch base se documenta arriba.
