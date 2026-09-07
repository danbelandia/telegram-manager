# Exploration: frontend-refresh-slice2 — Migrar 3 páginas de moderación + housekeeping docker-compose

> **Change**: `frontend-refresh-slice2`. **Mode**: hybrid (filesystem + Engram).
> **Inherits**: slice 1 archivado (#194-#203, base `main @ ace1f59`). El branch `feat/frontend-refresh` aún no mergeado — slice 2 parte de ese mismo branch o de main post-merge según decida el orchestrator.
> **Delivery**: single-pr con `size:exception` (precedente: publications-slice1/2/3, frontend-panel, frontend-moderation, frontend-refresh slice 1 — el usuario lo re-confirma en fase tasks).
> **Persisted**: `sdd/frontend-refresh-slice2/exploration` (Engram) + este archivo (filesystem).

## Estado heredado (post slice 1 archivado)

### Lo que slice 1 dejó funcionando
- **Foundation**: Mantine v7.17.8, AppShell con dark mode, `<Notifications>` global, helpers `notifySuccess/notifyError` en `frontend/src/lib/notifications.ts` (13 LOC). Verificado: `npm test -- --run` 66/66 verde, bundle 169.62 kB gz, 0 peer warnings, W1 vitest resuelto.
- **Migradas en slice 1**: LoginPage, DashboardPage (SimpleGrid+Cards), GroupsPage (Table), GroupDetailPage (Tabs+acciones always-visible — 3 tabs vs 4 spec deviation documentada en verify-report #201).
- **Wired a notifications en slice 1**: solo Login (success → "Bienvenido") + Logout ("Sesión cerrada").
- **Test helpers**: `renderWithProviders` extendido con `<MantineProvider theme={mantineTheme} defaultColorScheme="light">` + `<Notifications position="top-right"/>` (helpers.tsx:32-43).
- **Polyfills**: `setup.ts` provee `ResizeObserver` stub para Mantine v7 (Tabler lo requiere).
- **Spec canónico**: `openspec/specs/frontend-ui-foundation/spec.md` (NEW, 14 REQs). El slice 1 NO modificó specs existentes (frontend-auth/frontend-dashboard/frontend-routing/frontend-moderation) bajo el principio "visual swap es implementation detail behind existing REQs".

### Estado actual de las 3 páginas a migrar

Las 3 páginas (`GroupUsersPage`, `GroupRequestsPage`, `GroupLogsPage`) **no fueron tocadas** en slice 1 y hoy **renderizan rotas** porque:
- `frontend/src/styles.css` (limpieza slice 1) eliminó las reglas `app-layout`, `sidebar*`, `header*`, `login-page`, `login-form`, `form-field`, `btn*`, `state-block`, `group-list`, `group-card*`, `group-detail`, `detail-actions`, `group-sections`, `data-table`, `badge*`, `table-wrap`, `user-actions`, `user-lookup`, `user-lookup-result`, `back-link`, `state-block-weak`, `state-ok`.
- Las páginas siguen usando `<main className="page">`, `<button className="btn">`, `<table className="data-table">`, `<span className="badge badge-*">`, `<Link className="back-link">`, `<p className="state-block state-error">`, etc.
- Resultado: textos sin layout, botones sin estilo, tablas sin bordes, modales nativos del browser (`window.confirm`).

### Páginas NO tocadas en slice 2 (scope out)
- **PublicationsPage** (456 LOC): formulario complejo con datetime-local, paginación, historial con botones inline. La exploración slice 1 (#193) la mencionó para slice 2 PERO la exploración actual del usuario la **excluye explícitamente** del scope slice 2 — queda para slice 3 (la migración del PublicationsPage es ortogonal a las 3 de moderación).
- **NotFoundPage**: sin cambios necesarios.

### Lo que las páginas tienen en común (patrón slice 1)
- `<main className="page">` → `<Container>` o `<Box>` Mantine.
- `<Link className="back-link">` → `<Button variant="subtle" leftSection={<IconArrowLeft/>} component={Link}>` (consistente con la dev UX del wireframe AGENTS §16).
- `<h1>`, `<h2>` → `<Title order={1|2}>` Mantine.
- `<p className="state-block state-error">` → `<Alert color="red">` (mejor que `<Notification>` para errores inline persistentes hasta el retry).
- `<button className="btn">` → `<Button variant="default">` o `<Button color="red">` para destructivos.
- `<button className="btn btn-danger">` → `<Button color="red">` (Mantine v7 usa `color`, no clases).
- `<table className="data-table">` → `<Table>` con `striped`/`highlightOnHover` (consistente con GroupsPage slice 1).
- `<span className="badge badge-pending">` → `<Badge color="yellow">` (Mantine color scale).
- `window.confirm()` → `<Modal>` con `<Group justify="flex-end">` y dos `<Button>` (Cancelar/Confirmar).
- `<input>` nativo en forms → `<TextInput>` Mantine.
- Mensajes de éxito inline (`<p className="state-block state-ok">Decisión enviada.</p>`) → se reemplazan por `notifySuccess('Decisión enviada')` (consistente con slice 1 LoginPage).

### Docker-compose: el bug heredado

**Estado actual** (`docker-compose.yml:62-64`):
```yaml
frontend:
  build: { context: ./frontend }
  volumes:
    - ./frontend:/app
    - node_modules:/app/node_modules   # <-- volumen anónimo
volumes:
  node_modules:                       # <-- volumen nombrado declarado
  postgres_data:
```

**Comportamiento**:
- `Dockerfile` (line 10): `RUN npm install` corre en build, instala deps en `/app/node_modules` dentro de la imagen.
- Compose bind-mounts `./frontend:/app` → oculta la mayoría del `/app` de la imagen con el contenido del host (incluyendo `package.json`, código fuente).
- Segundo volumen `node_modules:/app/node_modules` → **sustituye** el `/app/node_modules` de la imagen con un volumen nombrado VACÍO en el primer arranque.
- **Bug**: el `npm install` del build queda escondido bajo el volumen. El contenedor arranca con `/app/node_modules` vacío o stale. En Windows/OneDrive el `node_modules` local del host ni siquiera existe (OneDrive excluye `node_modules/`), entonces `vite` tira "Cannot find module 'react'" o tarda minutos en fallar.

**Causa raíz**: comentario original `frontend/Dockerfile:1-3` dice "friccion OneDrive/Windows con bind mounts" — fue un workaround que introdujo un bug peor. Slice 1 NO tocó Docker (decisión D15). Este slice lo corrige como housekeeping.

### Features (NO se tocan — invariante slice 1 se preserva)
- `features/auth/*`, `features/groups/*`, `features/moderation/*`, `features/publications/*` — **0 cambios** (la regla de slice 1 sigue vigente: el wiring de notifications es a nivel del call-site en `pages/*`, no en los hooks).
- `lib/api-client.ts`, `lib/auth-context.tsx` — **0 cambios**.
- Backend, migraciones, TelegramAdapter — **0 cambios** (slice 2 es 100% frontend + docker-compose).

## Compatibilidad confirmada (re-verificación para slice 2)

**Hechos verificados via context7 (consultas 2026-09-07, slice 1 — vigentes)**:
- Mantine v7.17.8 + React 19.2.8 = 0 peer warnings. NO requiere upgrade a v8.
- `@mantine/notifications` ya cargado por `main.tsx`; los helpers `notifySuccess/notifyError` listos.
- `Modal`, `Alert`, `Badge`, `TextInput`, `Table`, `Loader`, `Pagination`, `Tabs` (ya usado en GroupDetail) — todos disponibles sin nuevas deps.
- `@tabler/icons-react` v3.46.0 ya instalado — provee `IconArrowLeft`, `IconUserOff`, `IconUserCheck`, `IconLock`, `IconLockOpen`, `IconBan`, `IconAlertTriangle`, `IconSearch`, `IconRefresh`, etc.
- `MantineProvider` + `Notifications` ya envuelven el árbol (no requiere re-configurar providers).

## Decisiones (con tradeoffs)

| # | Topic | Decisión | Alternativas | Rationale |
|---|-------|----------|--------------|-----------|
| **S2-D1** | Convención de spec | **Crear nuevo spec `frontend-pages-moderation`** (NEW capability, análogo a slice 1 que creó `frontend-ui-foundation`) | (a) Extender `frontend-ui-foundation` con nuevos REQs; (b) tratar como implementation detail de `frontend-moderation` sin spec propio | Slice 1 sentó precedente: cuando un slice introduce REQs nuevas que NO están en specs existentes, se crea nueva capability. Slice 2 introduce REQs de UI específicas para las 3 páginas (ej. "el ban usa Modal de confirmación con texto destructivo explícito", "logs agrupados por fecha en header sticky") que NO existen en `frontend-moderation` (ese spec describe el dominio, no el view layer). `frontend-ui-foundation` ya está cerrado y mezclar REQs nuevas rompería la auditoría. |
| **S2-D2** | `GroupUsersPage` layout | **`<Card>` por administrador (no `<Table>`)** dentro de un `<SimpleGrid cols={{base:1, sm:2}}>` | (a) `<Table>` 1 fila por admin; (b) `<Stack>` vertical | Tabla para admins es over-engineered: hay 1-50 admins típicos, las acciones por fila son 4 botones (`Banear`/`Desbanear`/`Mutear`/`Desmutear`) — `<Table>` se rompe en mobile. Cards + SimpleGrid = mismo patrón que DashboardPage slice 1 (consistencia visual). |
| **S2-D3** | `GroupUsersPage` acciones destructivas | **`Banear` requiere `<Modal>` de confirmación con texto explícito + `<Button color="red">` "Sí, banear"** | (a) `window.confirm` se mantiene; (b) Modal pero con texto genérico | `Banear` es irreversible (revoca mensajes, AGENTS §4). Slice 1 dejó `window.confirm` en GroupDetailPage porque NO migró esa página — slice 2 reemplaza confirmaciones nativas con `<Modal>` Mantine consistente con Login/Dashboard. Texto explícito ("Esta acción es irreversible y revoca los mensajes del usuario en este grupo") reduce errores. |
| **S2-D4** | `GroupUsersPage` acciones reversibles | **`Mutear`, `Desbanear`, `Desmutear` NO requieren Modal** — clic directo con feedback inmediato (`notifySuccess('Usuario muteado')`) | Modal en todas las acciones | AGENTS §4: confirmación obligatoria solo en acciones destructivas importantes. `Mutear` es reversible. UX innecesariamente friccionada con confirmaciones redundantes. |
| **S2-D5** | `GroupRequestsPage` layout | **`<Table>` Mantine con columnas Usuario, Fecha, Estado, Acciones** (idéntico patrón a slice 1 GroupsPage) | SimpleGrid de Cards | Lista de solicitudes es naturalmente tabular (5-50 filas, comparación visual rápida por estado). Ya testeado en `GroupRequestsPage.test.tsx:54-67` esperando estructura tabular. |
| **S2-D6** | `GroupRequestsPage` acciones | **`Aprobar`/`Rechazar` son clics directos SIN Modal** (acciones reversibles en backend: si admin se equivoca, Telegram no lo rechaza) | Modal de confirmación | AGENTS §4: confirmación obligatoria solo en destructivas. Aprobar se puede deshacer con `kickChatMember` manual desde Telegram. Slice 2 notifica éxito (`notifySuccess('Solicitud aprobada')`) y la fila se refresca por `invalidateQueries(['groups', id, 'requests'])` que ya hace el hook. |
| **S2-D7** | `GroupRequestsPage` concurrencia | **Reemplazar `<p className="state-block state-ok">Decisión enviada.</p>` (success) y `<p className="state-block state-error">…</p>` (error) por `notifySuccess`/`notifyError` con `formatModerationError`** | Mantener banners inline | Slice 1 sentó precedente: notifications globales, no banners inline duplicados. El test `findByText(/decisión enviada/i)` (GroupRequestsPage.test.tsx:114, 133) actualmente matchea `<p>` inline — **debe actualizarse** a `findByText` dentro del portal de Mantine Notifications (selector `:has-text` o `screen.getByRole('alert')`). |
| **S2-D8** | `GroupLogsPage` layout | **`<Table>` con `<Table.Thead sticky>` + orden por `created_at` DESC** (backend ya devuelve DESC) | Stack vertical de cards | Logs son append-only, naturalmente tabulares; el sticky header mejora UX cuando hay 50+ entradas (caso real: grupos activos). Test existente `findByText('BAN_USER')` sigue funcionando. |
| **S2-D9** | `GroupLogsPage` filtros | **NO agregar ColumnFilters / Pagination en slice 2** — out of scope, documentado para slice 3 si el usuario lo pide | Filtros por status/actor/action en slice 2 | AGENTS §11 dice "no asumir capacidades que la Bot API no expone" pero aquí NO es la Bot API: el backend `/api/groups/:id/logs` devuelve todos los logs sin paginación. Agregar filtros/paginación es una feature nueva (backend + frontend) — NO scope de housekeeping + migration. Lo dejo como sugerencia explícita en el apply-report. |
| **S2-D10** | `GroupLogsPage` reintento | **`Reintentar` se mantiene como `<Button variant="default">` con `<IconRefresh>`** (consistente con las 3 páginas) | Cambiar a retry automático con TanStack Query | Slice 1 ya eligió `retry: false` en todos los hooks (helpers + design D11). El botón manual es decisión explícita. |
| **S2-D11** | Wiring de notifications en mutaciones | **Wirear `onSuccess`/`onError` en `useBanUser`, `useUnbanUser`, `useMuteUser`, `useUnmuteUser`, `useApproveJoinRequest`, `useRejectJoinRequest` SOLO a nivel del call-site en `pages/*` (NO en `features/moderation/hooks.ts`)** | Wirear en `hooks.ts` | Slice 1 D8: helpers de notification en `lib/`, call-sites los invocan. Slice 2 mantiene ese patrón — los hooks no cambian, las páginas llaman `mutate(args, { onSuccess: () => notifySuccess('Usuario baneado'), onError: (e) => notifyError(formatModerationError(e)) })`. Esto preserva la invariante slice 1 ("no tocar features/*") y permite que `lib/notifications.ts` siga siendo el único punto de cambio de formato. |
| **S2-D12** | Texto de notifications | **Mensajes en español consistentes**: `notifySuccess('Usuario baneado')`, `notifySuccess('Usuario muteado')`, `notifySuccess('Solicitud aprobada')`, `notifySuccess('Solicitud rechazada')`, `notifyError(formatModerationError(err))` | Tonos inconsistentes, inglés | Mismo idioma y registro que el resto del panel (AGENTS §16 wireframe en español rioplatense). El helper `formatModerationError` (features/moderation/error.ts:8) ya traduce errores del backend. |
| **S2-D13** | Dockerfile dev | **Cambiar `CMD ["npm", "run", "dev", "--", "--host", "0.0.0.0"]` por `CMD ["sh", "-c", "npm install && npm run dev -- --host 0.0.0.0"]`** | (a) Multi-stage prod build; (b) bind-mount de solo `/app/src` (no `/app`) | (a) fuera de scope (MVP es dev only). (b) problema: en OneDrive/Windows el host no tiene `node_modules`, y excluirlo del bind-mount deja el de la imagen — pero al agregar una dep nueva, hay que rebuildear la imagen (`docker compose build frontend`). El approach (c) garantiza deps siempre correctas: cada arranque corre `npm install` (~10-30s), respeta package-lock.json. Tradeoff: arranque más lento, pero **siempre funciona**. |
| **S2-D14** | docker-compose: bind mount | **Cambiar `./frontend:/app` por `./frontend/src:/app/src` (solo el código fuente se monta; build artifacts y deps viven en la imagen)** | Mantener `./frontend:/app` y confiar en volúmenes nombrados | (a) Eliminar volumen anónimo `node_modules:/app/node_modules` (era el bug). (b) Bind-mount de solo `src/` evita pisar `package.json`/`node_modules`/`vite.config.ts` del host (en OneDrive `package.json` SÍ se trackea). (c) Hot reload sigue funcionando porque Vite observa cambios en `src/`. (d) Para agregar una dep nueva: `docker compose build frontend` (rebuild de imagen), OJO documentado en README. |
| **S2-D15** | docker-compose: declaración de volúmenes | **Eliminar `node_modules:` del bloque `volumes:` raíz** (era volumen nombrado huérfano); mantener solo `postgres_data:` | (a) Renombrar a `frontend_node_modules:`; (b) convertir a `.dockerignore` | (a) Re-introduce el bug. (b) `.dockerignore` no soluciona el bind-mount. Eliminar es lo correcto porque ya no se necesita. |
| **S2-D16** | `.dockerignore` (frontend/) | **Crear `frontend/.dockerignore`** con `node_modules`, `dist`, `.env`, `.git` para acelerar builds | Dejar Dockerfile copiar todo | `.dockerignore` excluye `node_modules` del build context → context más chico, `npm install` en build es determinista (no contaminado por deps del host). Estándar de hygiene Docker. |
| **S2-D17** | Estrategia de tests | **Refactor `renderUsers/renderRequests/renderLogs` para usar `renderWithProviders` + agregar `data-testid` donde los selectores por texto choquen con badges/notifications** | Reescribir tests desde cero | Tests existentes siguen pasando si el wrapper provee Mantine (ya lo hace `renderWithProviders`). Selectores `getByText('Juan')` siguen matcheando porque el `<Text>` Mantine renderiza el texto plano. Selectores `getByRole('button', {name: 'Banear'})` siguen funcionando porque Mantine `<Button>` es accesible. Único problema: notificaciones — `findByText(/decisión enviada/i)` en GroupRequestsPage test ahora matchea contra `document.body` (portal de Notifications), pero el portal está dentro del wrapper de Mantine que `renderWithProviders` ya provee. |
| **S2-D18** | Cobertura de smoke tests | **+1 smoke por página migrada: "el Modal de confirmación de ban muestra el texto destructivo y ejecuta mutate al confirmar"** | (a) Solo smoke del render; (b) test exhaustivo del flujo | Slice 1 ya agregó smoke de render por página. Slice 2 valida la interacción crítica nueva (Modal de confirmación). Tests exhaustivos del flujo (ej. probar todos los estados del Modal) están en `apply-report`/`verify-report` como exhaustivo — slice 2 prioriza la integración del cambio nuevo, no el rebote de lo viejo. |
| **S2-D19** | Documentación | **Agregar nota "Docker dev: cambios en package.json requieren `docker compose build frontend`" al README raíz** | (a) Inline en `docker-compose.yml` con `#`; (b) sección troubleshooting separada | README raíz ya tiene sección "Levantar la stack" (líneas 25-63); agregar una sub-sección "Cambios de dependencias" cerca del panel section (línea ~94) es el lugar natural. Slice 1 ya usa este patrón para la nota de UI library (líneas 95-114). |
| **S2-D20** | Branch | **`feat/frontend-refresh-slice2`** basada en `feat/frontend-refresh` (que ya está commiteada localmente con slice 1) — el orchestrator confirma si el slice 1 mergea antes o después | (a) Branch nueva desde `main` y cherry-pick slice 1; (b) commit sobre `feat/frontend-refresh` directo | (a) Duplica trabajo. (b) Asume merge previo de slice 1. (c) branch dedicada desde `feat/frontend-refresh` mantiene historia limpia y permite rebase si slice 1 se ajusta post-verify. |

## Affected Areas

### Archivos modificados (frontend)

| File | Action | ~LOC | Notes |
|------|--------|------|-------|
| `frontend/src/pages/GroupUsersPage.tsx` | rewrite | ~165 → ~180 | Cards + SimpleGrid + Modal para ban; lookup con TextInput Mantine; wiring de notifications en ban/mute/unban/unmute; mantiene los hooks `useGroupUsers`/`useGroupUser`/`useBanUser`/etc. sin cambios |
| `frontend/src/pages/GroupRequestsPage.tsx` | rewrite | ~117 → ~110 | Table Mantine con Badge por status; acciones Aprobar/Rechazar sin Modal; wiring de notifications; reemplaza `lastResult`/`actionError` inline por notifications |
| `frontend/src/pages/GroupLogsPage.tsx` | rewrite | ~74 → ~95 | Table con sticky header; Badge por status; formato de error_message/actor_id consistente |
| `frontend/src/pages/GroupUsersPage.test.tsx` | rewrite renderUsers + agregar modal smoke | ~171 → ~190 | Cambiar wrapper a `renderWithProviders`; ajustar selector de ban (`getByRole('button', {name: /sí, banear/i})` después del Modal); agregar 2 smoke (modal abre/cierra) |
| `frontend/src/pages/GroupRequestsPage.test.tsx` | adjust selectors | ~150 → ~155 | Wrapper a `renderWithProviders`; `findByText(/decisión enviada/i)` ahora matchea portal de notifications (funciona porque `renderWithProviders` monta `<Notifications/>`); agregar 1 smoke para notification de approve |
| `frontend/src/pages/GroupLogsPage.test.tsx` | wrapper swap | ~98 → ~100 | Wrapper a `renderWithProviders`; selectores de texto/Badge sin cambios |
| `frontend/.dockerignore` | NEW | ~6 | `node_modules`, `dist`, `.env`, `.git`, `.vite` |
| `README.md` | +12 LOC | ~302 → ~314 | Sub-sección "Cambios de dependencias" en Docker dev |

### Archivos modificados (devops)

| File | Action | ~LOC | Notes |
|------|--------|------|-------|
| `docker-compose.yml` | modify | 72 → 70 | Quitar `- node_modules:/app/node_modules` (línea 64); cambiar `- ./frontend:/app` por `- ./frontend/src:/app/src`; quitar `node_modules:` del bloque `volumes:` (línea 71) |
| `frontend/Dockerfile` | modify | 15 → 16 | CMD cambia a `["sh", "-c", "npm install && npm run dev -- --host 0.0.0.0"]` (línea 14); comentario actualizado para reflejar S2-D13 |

### Archivos NO modificados (invariantes slice 1 preservadas)

- `features/auth/{api,types}.ts`, `features/groups/{api,hooks,permissions,types}.ts`, `features/moderation/{api,error,hooks,types}.ts`, `features/publications/*` — **0 cambios**.
- `lib/api-client.ts`, `lib/auth-context.tsx` — **0 cambios**.
- `lib/notifications.ts` (helpers ya en su lugar; S2-D11 lo usa via call-sites).
- `theme.ts`, `postcss.config.cjs`, `main.tsx`, `Layout.tsx`, `ColorSchemeToggle.tsx`, `test/helpers.tsx`, `test/setup.ts`, `vite.config.ts`, `package.json` — **0 cambios**.
- Las 5 páginas migradas en slice 1 (LoginPage/DashboardPage/GroupsPage/GroupDetailPage + Layout.test/notifications.test) — **0 cambios**.
- Backend, migraciones, TelegramAdapter, `docs/telegram_api_reference.md` — **0 cambios**.

## Decisión sobre spec: `frontend-pages-moderation` (NEW)

**Decisión**: crear `openspec/specs/frontend-pages-moderation/spec.md` (NEW capability) en el directorio canónico, análogo al patrón de slice 1 (`frontend-ui-foundation`).

**Razones**:
1. **No es un detalle de `frontend-moderation`**: ese spec describe el dominio (DTOs, endpoints, estados), NO la capa de presentación. Los requisitos de UI (Modal de confirmación destructiva, notifications globales, sticky header en logs, badges de status) son **nuevos** y propios del view layer.
2. **No es un delta de `frontend-ui-foundation`**: ese spec describe la fundación reusable (theme, AppShell, Notifications provider). Slice 2 es el **consumo** de esa fundación por 3 páginas específicas — REQs distintas, nivel de abstracción distinto.
3. **El audit trail de slice 1 lo permite**: archive-report #203 dice "MODIFIED None (existing frontend-auth/frontend-dashboard/frontend-routing/frontend-moderation are NOT modified — visual swap is implementation detail behind existing REQs)" — slice 1 NO modificó `frontend-moderation` porque la fundación UI es reutilizable. Slice 2 introduce **detalles visuales específicos** de 3 páginas que `frontend-moderation` no contempla, justificando un nuevo spec.
4. **Consistencia con slice 1**: ambos slices crean una capability nueva. Las dos juntas dan una cobertura completa: foundation (UI) → consumption (páginas).

**Sync method** (archive-phase): copia verbatim del delta al canónico (`openspec/specs/frontend-pages-moderation/spec.md`), igual que slice 1 hizo con `frontend-ui-foundation`.

## No-regression surface (MUST NOT change)

`features/auth/*`, `features/groups/*`, `features/moderation/*`, `features/publications/*`, `lib/api-client.ts`, `lib/auth-context.tsx`, `backend/**`, `migrations/**`, `telegram_api_reference.md`, todas las páginas migradas en slice 1, todos los componentes slice 1 (Layout, ColorSchemeToggle), `main.tsx`, `theme.ts`, `helpers.tsx`, `setup.ts`, `vite.config.ts`, `package.json`, `package-lock.json`.

## Riesgo table

| # | Riesgo | P | I | Mitigación |
|---|--------|---|---|------------|
| R1 | `data-testid` faltante rompe selector `getByText('Juan')` cuando hay 2 admins "Juan" | L | M | Smoke tests ajustan con `getAllByText` o `findAllByText` si hay duplicados |
| R2 | Portal de Notifications se monta fuera del contenedor principal → `findByText` en jsdom busca en `document.body` completo | L | L | Slice 1 verify-report #201 ya validó que funciona en jsdom |
| R3 | docker-compose cambio rompe `docker compose up` en Windows/OneDrive si `.dockerignore` no excluye `node_modules` correctamente | M | M | Crear `frontend/.dockerignore` ANTES del cambio; rebuild from scratch (`docker compose build --no-cache frontend`) la primera vez; documentar en README |
| R4 | Cambio `CMD` a `sh -c "npm install && ..."` agrega 10-30s al arranque | C | L | Documentado en README; aceptable para dev (ya hay healthcheck de 10s en backend) |
| R5 | Cambio de bind-mount `./frontend:/app` → `./frontend/src:/app/src` requiere `docker compose build frontend` para que `vite.config.ts`/`postcss.config.cjs` se actualicen | M | M | Documentar en README: "cambios en vite.config.ts o package.json requieren rebuild de la imagen" |
| R6 | Wirear notifications en call-sites diverge entre GroupUsers y GroupRequests (cada uno copy-paste su bloque) | L | L | Mantener el mismo shape `{ onSuccess: () => notifySuccess('...'), onError: (e) => notifyError(formatModerationError(e)) }` en los 2 archivos; consistencia explícita en design phase |
| R7 | Reseción de tests para usar `renderWithProviders` cambia el orden de providers y rompe aserciones que dependían del orden anterior | L | L | Slice 1 verify #201 ya validó que los 49+ tests pasaron con el wrapper extendido sin cambios estructurales; slice 2 mantiene exactamente el mismo wrapper |
| R8 | El test "no llama a la API si el admin cancela la confirmacion" (GroupUsersPage.test.tsx:128-145) ya no aplica porque slice 2 cambia de `window.confirm` a `<Modal>` | M | M | Test se actualiza: en vez de mockear `window.confirm`, hacer `click` en el botón "Cancelar" del Modal. Smoke test del Modal cubre el flujo nuevo |

## Spec sketch (preliminar — la fase sdd-spec definirá los REQs exactos)

Cap provisional: **REQ-1 a REQ-8**, ~12 escenarios totales:
- REQ-1: GroupUsersPage migrado a Cards + SimpleGrid
- REQ-2: Modal de confirmación para acciones destructivas (ban)
- REQ-3: GroupRequestsPage migrado a Table + badges
- REQ-4: GroupLogsPage migrado a Table con sticky header
- REQ-5: Wiring de notifications en mutaciones destructivas (call-site)
- REQ-6: docker-compose anonymous volume removido; bind-mount limitado a src/
- REQ-7: Dockerfile CMD ejecuta `npm install` en arranque
- REQ-8: README documenta el cambio y la nota "cambios en deps requieren rebuild"

## Sugerencias para slice 3 (out of scope slice 2, capturado para que no se olvide)

1. **PublicationsPage migration** (456 LOC, formulario complejo con datetime-local + paginación + cancel) — sucesor natural.
2. **GroupLogsPage filtros + paginación** (S2-D9 diferido): requiere cambios en backend (`?limit&offset&action=&status=`), no solo frontend.
3. **GroupRequestsPage filtro por status** (mostrar solo `pending` por default, con toggle para ver histórico).
4. **Re-attempt 4-tab layout en GroupDetailPage** (verify-report #201 W-V1): workaround usando Mantine v8.3.14 o render condicional en lugar de `<Tabs>`.
5. **Wirear notifications en mutaciones de PublicationsPage** (slice 3): mismo patrón S2-D11.
6. **`.env.example`**: agregar `PUBLICATIONS_SCHEDULER_INTERVAL_SECONDS` si no está (verificar).

## Verificación planeada (para fase sdd-verify)

- `cd frontend && npm run build` verde; bundle delta +10-15 kB gz (3 páginas + Modal + Table sticky).
- `cd frontend && npm test -- --run` 100% verde, 0 flakeos paralelos (W1 slice 1 resuelto).
- `docker compose up --build` levanta sin errores; `docker compose logs frontend` muestra "Vite ready" en <60s.
- Smoke manual: navegar a `/groups/123/users`, hacer clic en "Banear" → Modal aparece con texto destructivo → confirmar → notification verde "Usuario baneado" + fila desaparece o cambia status.
- Smoke manual: navegar a `/groups/123/logs`, hacer scroll → header sticky permanece visible.
- `git diff feat/frontend-refresh -- features/ lib/api-client.ts lib/auth-context.tsx backend/ migrations/` → **0 líneas** (no-regresión).

## Ready for Proposal

**YES** — todas las decisiones tienen tradeoff explícito, todas las invariantes slice 1 están preservadas, el bug docker-compose tiene causa raíz identificada + fix aplicado (S2-D13/S2-D14), el wiring de notifications sigue el patrón existente (call-sites, no hooks). El orchestrator debe:

1. Lanzar `sdd-propose` con scope acotado a 3 páginas + docker-compose + 1 spec nuevo `frontend-pages-moderation`.
2. Confirmar la rama base: ¿slice 2 parte de `feat/frontend-refresh` (no mergeada) o de `main` (post-merge)? Default propuesto: branch dedicada `feat/frontend-refresh-slice2` desde `feat/frontend-refresh` (S2-D20).
3. Re-confirmar `size:exception` (precedente 5/5 con aprobación).
