# Design: frontend-refresh-slice2 — 3 Páginas de Moderación + Docker Dev Fix

## Technical Approach

Migración visual de 3 páginas que quedaron rotas tras slice 1 (GroupUsers/GroupRequests/GroupLogs) a Mantine v7, más wiring de notifications en los call-sites de mutaciones, más housekeeping del bug heredado de `docker-compose.yml` (volumen anónimo que pisaba `node_modules` en Windows/OneDrive). Reutiliza 100% la fundación de slice 1: `MantineProvider`, `<Notifications>` portal, helpers `notifySuccess/notifyError` en `lib/notifications.ts`, `renderWithProviders` extendido. **0 cambios** en `features/*`, `lib/api-client.ts`, `lib/auth-context.tsx`, backend, contratos API, ni `lib/notifications.ts`. Spec canónico: `openspec/changes/frontend-refresh-slice2/specs/frontend-pages-moderation/spec.md` (NEW, 14 REQs, 24 scenarios — heredado de la spec #208).

## Architecture Decisions

| Topic | Choice | Alternatives | Rationale |
|---|---|---|---|
| D1 GroupUsersPage layout | `<Card>` por admin en `<SimpleGrid cols={{base:1,sm:2}}>` | `<Table>` (over-eng 4 acciones/fila en mobile) / `<Stack>` vertical | Consistencia con DashboardPage; responsive natural |
| D2 Banear confirmación | `<Modal>` con texto destructivo explícito | `window.confirm` (legacy) / Modal genérico | AGENTS §4: solo irreversibles requieren Modal |
| D3 Mute/Unban/Unmute | Clic directo + `notifySuccess`, sin Modal | Modal para todas | Reversibles; UX friccionada innecesaria |
| D4 GroupRequestsPage layout | `<Table>` Mantine + `<Badge>` | SimpleGrid de Cards | Lista naturalmente tabular (5-50 filas) |
| D5 Approve/Reject | Clic directo + notify, sin Modal | Modal de confirmación | Reversible (admin puede deshacer desde Telegram) |
| D6 GroupLogsPage layout | `<Table>` + `<Table.Thead sticky>` | Stack vertical de cards | Append-only, sticky mejora UX con 50+ entradas |
| D7 Paginación logs | NO (diferida slice 3) | Filtros + Pagination ahora | Requiere backend `?limit&offset`, fuera de housekeeping |
| D8 Wiring notifications | SOLO en call-sites `pages/*` (NO en `features/moderation/hooks.ts`) | Wirear en hooks | Invariante slice 1: `lib/notifications.ts` único punto de cambio |
| D9 Lookup por ID | `<form onSubmit>` + `<TextInput>` (sin debounce) | `onChange` debounced | Single query intencional por submit, evita spam |
| D10 Status badge map | `SUCCESS`=green, `PERMISSION_DENIED`/`INTERNAL_ERROR`=red, `VALIDATION_ERROR`=yellow, `NOT_FOUND`=gray, `TELEGRAM_ERROR`=orange | Color libre | Mapeo explícito según AGENTS §11/§18 |
| D11 Tests strategy | `renderWithProviders` (slice 1) + `data-testid` donde colisión con Badge/notification | Reescribir tests | Wrapper provee Mantine+Notifications; selectores por texto siguen funcionando |
| D12 Docker triada | bind-mount `./frontend/src:/app/src` + `CMD npm install && npm run dev` + `.dockerignore` | Multi-stage prod build | (a) fuera scope MVP. (c) garantiza deps siempre correctas +10-30s |
| D13 Wrapper en tests | `renderWithProviders` extiende `test/helpers.tsx` ya migrado en slice 1 | Mantener wrappers locales | Slice 1 ya validó que 49+ tests pasan con el wrapper extendido |

## Data Flow

```
[ Admin ] ─click Banear─▶ [ GroupUsersPage ] ─setState(banTarget)─▶ [ Mantine Modal ]
                                       │                                       │
                                       │                                       ▼ click confirm
                                       │                              [ useBanUser.mutate ]
                                       │                                       │
                                       │                          ┌────────────┴────────────┐
                                       │                          │                         │
                                       │                    success│                         │error
                                       │                          ▼                         ▼
                                       │              notifySuccess('Usuario baneado')  notifyError(formatModerationError(err))
                                       │                          │                         │
                                       │                          └──invalidate usersKey+logsKey (hook interno)──┘
                                       │
                                       └──────useQuery──────▶ [ TanStack Query ] ──▶ [ backend REST ]
```

## File Changes

| File | Action | LOC Δ | Description |
|---|---|---|---|
| `frontend/src/pages/GroupUsersPage.tsx` | rewrite | 167→180 | Cards+SimpleGrid; `<Modal>` para ban; TextInput lookup; notifications wiring en los 4 hooks |
| `frontend/src/pages/GroupRequestsPage.tsx` | rewrite | 117→110 | Table+Badge; Aprobar/Rechazar clic directo con notify; elimina banners inline |
| `frontend/src/pages/GroupLogsPage.tsx` | rewrite | 74→95 | Table con `<Table.Thead sticky>`; Badge color map; columna Mensaje con fallback chain |
| `frontend/src/pages/GroupUsersPage.test.tsx` | adjust | 171→190 | Wrapper `renderWithProviders`; `data-testid="user-card"`/`"confirm-ban"`; +2 modal smokes |
| `frontend/src/pages/GroupRequestsPage.test.tsx` | adjust | 150→155 | Wrapper swap; `findByText` matchea portal; +1 notification smoke |
| `frontend/src/pages/GroupLogsPage.test.tsx` | adjust | 98→100 | Wrapper swap; sin selectores nuevos |
| `frontend/.dockerignore` | NEW | 6 | `node_modules`, `dist`, `.env`, `.env.local`, `.git`, `.vite`, `coverage`, `*.log` |
| `docker-compose.yml` | modify | 72→70 | Quita `- node_modules:/app/node_modules`; `./frontend:/app` → `./frontend/src:/app/src`; elimina `node_modules:` raíz |
| `frontend/Dockerfile` | modify | 15→16 | CMD → `["sh", "-c", "npm install && npm run dev -- --host 0.0.0.0"]` |
| `README.md` | +12 | 302→314 | Sub-sección "Cambios de dependencias" en Docker dev panel |

**Invariantes (NO tocar, spec REQ-14)**: `features/auth/*`, `features/groups/*`, `features/moderation/*`, `features/publications/*`, `lib/api-client.ts`, `lib/auth-context.tsx`, `lib/notifications.ts`, `theme.ts`, `main.tsx`, `Layout.tsx`, `ColorSchemeToggle.tsx`, `test/helpers.tsx`, `test/setup.ts`, `vite.config.ts`, `package.json`, `package-lock.json`, `postcss.config.cjs`, páginas slice 1 (Login/Dashboard/Groups/GroupDetail), `PublicationsPage` (migrada ad-hoc en `f4d1751`), backend, migraciones, `TelegramAdapter`.

## Interfaces / Contracts

**Sin nuevos contratos TS**. La firma `mutate(args, { onSuccess, onError })` ya existe en TanStack Query v5. **Textos de notificación** (contrato copy):

| Acción | Texto success |
|---|---|
| `useBanUser` | `'Usuario baneado'` |
| `useUnbanUser` | `'Usuario desbaneado'` |
| `useMuteUser` | `'Usuario muteado'` |
| `useUnmuteUser` | `'Usuario desmuteado'` |
| `useApproveJoinRequest` | `'Solicitud aprobada'` |
| `useRejectJoinRequest` | `'Solicitud rechazada'` |

Errores: `notifyError(formatModerationError(err))` (helper `features/moderation/error.ts:8`). **Status badge map** (logs): `SUCCESS`→green, `PERMISSION_DENIED`→red, `INTERNAL_ERROR`/`ERROR`→red, `VALIDATION_ERROR`→yellow, `NOT_FOUND`→gray, `TELEGRAM_ERROR`→orange. Lookup: `useState<string>` para `lookupId` (input value), `useState<string>` para `lookupSubmitted` (committed en submit). Modal state: `useState<GroupUser | null>` (`null` = cerrado).

## Testing Strategy

| Layer | What | Approach |
|---|---|---|
| Unit (Vitest) | Render+estados de las 3 páginas | `renderWithProviders` + `mockFetchRoutes` (existente `test/helpers.tsx:56-71`) |
| Smoke Modal (NEW×2) | Click Banear → Modal abre con título/texto; Cancelar cierra sin POST | `data-testid="user-card"`, `data-testid="confirm-ban"` |
| Smoke notification (NEW×1) | Click Aprobar → `findByText(/solicitud aprobada/i)` matchea portal `<Notifications/>` | Wrapper extendido provee `<Notifications position="top-right"/>` |
| Concurrencia | Aprobar con `VALIDATION_ERROR` backend → notifyError legible | `formatModerationError` traduce envelope |
| No-regresión | `git diff main -- features/ lib/... backend/ migrations/` → 0 líneas | Gate final del apply |

## Migration / Rollout

**No migration** (no DB, no API contract). Rollback = `git revert <merge>`. Riesgo aceptable: el bug original de `node_modules` queda restaurado como estado temporal. Branch: `feat/frontend-refresh-slice2` desde `main @ f4d1751` (slice 1 mergeada en `700681c` + PublicationsPage ad-hoc `f4d1751` — proposal §Branch Base Correction).

## Open Questions (resolved)

| # | Pregunta | Respuesta |
|---|---|---|
| OQ1 | ¿Debounce vs onSubmit para lookup? | **onSubmit** — single query intencional, sin spam |
| OQ2 | ¿Modal también para mute? | **No** — mute reversible, clic directo + notify |
| OQ3 | ¿Status logs mayúsculas o lowercase? | Backend devuelve mayúsculas (`SUCCESS`); mapa usa `status.toLowerCase()` como key |
| OQ4 | ¿Paginación de logs? | Diferida a slice 3 (spec REQ-7 documenta el contrato `?limit&offset`) |
| OQ5 | ¿PublicationsPage en este slice? | **No** — migrada ad-hoc en `f4d1751`, intacta |
| OQ6 | ¿Branch base? | `main @ f4d1751` (slice 1 ya mergeada) |

## File-by-file specifications

### `frontend/src/pages/GroupUsersPage.tsx` (~180 LOC)

**Imports**: `Container, Stack, Title, Text, TextInput, Card, SimpleGrid, Skeleton, Alert, Button, Group, Badge, Modal` de `@mantine/core`; `IconSearch` de `@tabler/icons-react`; `notifySuccess`/`notifyError` de `lib/notifications`; `formatModerationError` de `features/moderation/error`; hooks `useGroupUsers`/`useGroupUser`/`useBanUser`/`useUnbanUser`/`useMuteUser`/`useUnmuteUser` (sin cambios).

**Estructura**:
- Header: `<Stack gap="md"><Title order={2}>Membresía y moderación</Title>` + nota de alcance.
- Lookup: `<form onSubmit>` que setea `lookupSubmitted`; `<TextInput leftSection={<IconSearch size={16}/>} placeholder="ID de Telegram del usuario" label="Buscar usuario por telegram_id o nombre">`.
- Lista: `<SimpleGrid cols={{base:1, sm:2}} spacing="md">` con `<Card data-testid="user-card">` por admin. Cada Card: `<Text fw={500}>{first_name}</Text>`, `<Text c="dimmed">@{username}</Text>` (condicional), `<Badge color={STATUS_COLOR[user.status]}>{user.status}</Badge>`, `<Group>` con 4 botones (Banear `color="red"` filled, Mutear `color="yellow"` filled, Desbanear/Desmutear `variant="light"`).
- Modal state: `const [banTarget, setBanTarget] = useState<GroupUser | null>(null)`.
- Modal JSX: `<Modal opened={banTarget !== null} onClose={() => setBanTarget(null)} title="Confirmar baneo" centered>` con `<Text>Banear a {banTarget?.first_name}? Esta acción es irreversible.</Text>` y `<Group justify="flex-end">` con Cancelar (`variant="default"`) + Banear (`color="red" variant="filled" data-testid="confirm-ban"`).
- Botón Banear Card onClick: `setBanTarget(user)`.
- Botón Banear Modal onClick: `ban.mutate({groupId, userId: banTarget.user_id}, { onSuccess: () => { notifySuccess('Usuario baneado'); setBanTarget(null) }, onError: (e) => notifyError(formatModerationError(e)) }); setBanTarget(null)`.
- Lookup result: muestra Card de lookup si `lookup.data` está presente.
- Estados: loading → `<Skeleton height={120} count={3}/>`; error → `<Alert color="red">` con botón Reintentar; empty → `<Text c="dimmed">"Sin administradores visibles…"</Text>`.

### `frontend/src/pages/GroupRequestsPage.tsx` (~110 LOC)

**Imports**: `Paper, Stack, Title, Text, Table, Badge, Button, Group, Skeleton, Alert` de `@mantine/core`; `IconCheck, IconX` de Tabler.

**Estructura**:
- `<Paper p="md" withBorder>` + `<Stack>`.
- `<Title order={2}>Solicitudes de ingreso</Title>`.
- Loading: `TableSkeleton` con 3 filas Skeleton dentro de `<Table striped highlightOnHover>`.
- Empty: `<Text c="dimmed">"Sin solicitudes de ingreso pendientes ni resueltas."`.
- Tabla: columnas Usuario/Fecha/Estado/Acciones; fila pending → `<Group>` con Aprobar (`variant="light" color="green" leftSection={<IconCheck/>}`) y Rechazar (`variant="light" color="red" leftSection={<IconX/>}`); fila decided → `<Text c="dimmed">"Decidida por admin {decided_by}"</Text>`.
- Status Badge color map: `pending`=yellow, `approved`=green, `rejected`=red.
- Mutations: approve/reject directos con notify (sin Modal).

### `frontend/src/pages/GroupLogsPage.tsx` (~95 LOC)

**Imports**: `Paper, Stack, Title, Text, Table, Badge, Tooltip, Skeleton, Alert` de `@mantine/core`.

**Estructura**:
- `<Paper p="md" withBorder>` + `<Stack>` + `<Title order={2}>Logs del grupo</Title>`.
- Loading: `TableSkeleton` con 5 filas.
- Empty: `<Text c="dimmed">"Sin acciones registradas en este grupo."`.
- Tabla: `<Table.ScrollContainer minWidth={500}>` wrapper + `<Table striped highlightOnHover withTableBorder>` con `<Table.Thead sticky>` (Mantine v7 prop).
- Columnas: Fecha/Acción/Estado/Mensaje.
- Columna Acción: `entry.action + (entry.target_user_id ? \` → usuario ${entry.target_user_id}\` : '')`.
- Columna Estado: `STATUS_BADGE_COLOR[entry.status.toLowerCase()]` mapea a color Mantine.
- Columna Mensaje: chain `entry.error_message ?? (entry.actor_id ? \`por admin ${entry.actor_id}\` : '—')`, envuelto en `<Tooltip label={entry.error_message}>` cuando aplica.

### Tests delta (~40 LOC agregados)

- `GroupUsersPage.test.tsx`: reemplazar `renderUsers` por `renderWithProviders(<MemoryRouter initialEntries={['/groups/123/users']}><Routes><Route path="/groups/:id/users" element={<GroupUsersPage/>}/></Routes></MemoryRouter>, ['/groups/123/users'])`. Agregar `data-testid="user-card"` al Card JSX y `data-testid="confirm-ban"` al botón del Modal. Smoke 1 (modal ejecuta POST): click Banear en card Juan → `await screen.findByRole('heading', {name: 'Confirmar baneo'})` → click `data-testid="confirm-ban"` → `waitFor(() => expect(spy).toHaveBeenCalled())` + `await screen.findByText('Usuario baneado')`. Smoke 2 (Cancelar no llama API): click Banear → click "Cancelar" → `waitFor(() => expect(spy).not.toHaveBeenCalled())`. Eliminar el mock de `window.confirm` (legacy).
- `GroupRequestsPage.test.tsx`: wrapper swap. Smoke notification: click Aprobar → `await screen.findByText(/solicitud aprobada/i)` (matchea portal `<Notifications/>`).
- `GroupLogsPage.test.tsx`: wrapper swap. Sin nuevos tests.

### `docker-compose.yml` (líneas 62-64, 71)

```yaml
# ANTES:
    volumes:
      - ./frontend:/app
      - node_modules:/app/node_modules
...
volumes:
  node_modules:
  postgres_data:

# DESPUÉS:
    volumes:
      # Bind-mount SOLO de src/ para hot reload (Vite observa /app/src).
      # package.json, vite.config.ts y node_modules viven en la imagen;
      # cambios requieren `docker compose build frontend` (ver README).
      - ./frontend/src:/app/src
...
volumes:
  postgres_data:
```

### `frontend/Dockerfile`

```dockerfile
# Comentario línea 1-3 actualizado: "CMD re-corre npm install al arrancar
# para tolerar cambios de package.json sin rebuild de imagen. Trade-off:
# +10-30s en primer arranque (compatible con start_period=10s del backend)."
EXPOSE 5173
CMD ["sh", "-c", "npm install && npm run dev -- --host 0.0.0.0"]
```

### `frontend/.dockerignore` (NEW)

```
node_modules
dist
.vite
coverage
.env
.env.local
*.log
.vscode
.idea
.DS_Store
.git
```

### `README.md`

Agregar después de "UI library & Dark mode" (línea 114), sub-sección:

```markdown
### Cambios de dependencias en el frontend

El contenedor `frontend` ahora bind-monta solo `./frontend/src` y ejecuta
`npm install` en cada arranque:

- Cambios en archivos dentro de `frontend/src/` → hot reload automático, sin rebuild.
- Cambios en `package.json` / `package-lock.json` → requieren `docker compose build frontend`.
- Cambios en `vite.config.ts`, `postcss.config.cjs` o `frontend/Dockerfile` → requieren `docker compose build frontend`.
- Primer arranque tarda 10-30s extra por el `npm install`; arranques subsecuentes usan cache de npm.

`frontend/.dockerignore` excluye `node_modules`, `dist`, `.env`, `coverage`, etc.
```

## Risk Table

| # | Risk | P | I | Mitigation |
|---|---|---|---|---|
| R1 | `.dockerignore` no excluye `node_modules` → build context contaminado con deps del host | M | M | Crear `.dockerignore` ANTES del cambio `docker-compose.yml`; README documenta `docker compose build --no-cache frontend` |
| R2 | CMD con `npm install` agrega 10-30s al primer arranque | C | L | Documentado en README; aceptable (start_period=10s del backend healthcheck) |
| R3 | Cambio bind-mount requiere `docker compose build frontend` para cambios en `vite.config.ts`/`package.json` | M | M | README sub-sección "Cambios de dependencias" explícita; cache de npm reduce arranques subsecuentes |