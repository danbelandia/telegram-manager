# Frontend Pages Moderation Specification

## Purpose

Capa de presentación Mantine v7 para las 3 páginas de moderación que quedaron sin migrar tras slice 1 (`frontend-ui-foundation`): `GroupUsersPage`, `GroupRequestsPage` y `GroupLogsPage`. Slice 1 fundó el panel (theme, AppShell, Notifications global, helpers `notifySuccess`/`notifyError`, `<SimpleGrid>`+`<Card>` en Dashboard, `<Table>` en Groups, `<Tabs>` en GroupDetail). Slice 2 cierra las páginas que requieren patrones específicos del view layer: `<Modal>` de confirmación para acciones destructivas irreversibles, sticky header en tablas de auditoría, `<Badge>` por estado, wiring de notifications en mutaciones de TanStack Query a nivel de call-site. Adicionalmente corrige el bug heredado de `docker-compose.yml` (volumen anónimo que pisaba `node_modules` en Windows/OneDrive). NO modifica el contrato de la API, los hooks de `features/moderation/*`, ni el backend. Los requisitos de dominio (ban requiere confirmación, approve/reject, listado de logs) viven en `frontend-moderation`; este spec describe solo el view layer y los detalles de presentación que aquel spec no cubre.

## Requirements

### Requirement: GroupUsersPage migrado a Cards + SimpleGrid

`frontend/src/pages/GroupUsersPage.tsx` MUST renderizar los administradores como `<Card withBorder padding="md" radius="md" data-testid="user-card">` dentro de `<SimpleGrid cols={{ base: 1, sm: 2 }} spacing="md">` (consistente con DashboardPage). Cada Card MUST mostrar nombre (`<Text fw={500}>`), username (`<Text c="dimmed">` precedido por `@`), un `<Badge>` por status (`blue` para `administrator`, `gray` para `creator`/`member`, `yellow` para `restricted`/`left`/`kicked`) y los 4 botones de acción (REQ-2). El lookup por ID MUST ser un `<TextInput>` Mantine con placeholder `"ID de Telegram del usuario"` dentro de un `<form>` que en submit dispara `useGroupUser(groupId, lookupSubmitted)`. Estados: loading → `<Skeleton height={120} count={3}/>`; error → `<Alert color="red">` con botón "Reintentar"; vacío → `<Text c="dimdim">"Sin administradores visibles…"</Text>` (preservando la nota de limitación de la Bot API).

#### Scenario: Vista de administradores renderiza Cards

- GIVEN un grupo con 2 admins (`Juan` administrator, `Ana` creator)
- WHEN se abre `/groups/123/users`
- THEN se renderizan 2 Cards con nombre, `@username` y Badge azul/gris

#### Scenario: Estado de carga usa Skeleton

- GIVEN el hook `useGroupUsers` en `isPending`
- WHEN se abre la página
- THEN se ven Skeletons en lugar de Cards

### Requirement: Acciones de usuarios con variantes Mantine y feedback

Los 4 botones por usuario MUST ser `<Button>` Mantine: `Banear` con `color="red" variant="filled"`, `Mutear` con `color="yellow" variant="filled"`, `Desbanear` y `Desmutear` con `variant="light"`. Las acciones reversibles (`Mutear`, `Desbanear`, `Desmutear`) MUST ejecutarse directamente al hacer clic SIN Modal de confirmación, invocando `mutate(args, { onSuccess: () => notifySuccess('…'), onError: (e) => notifyError(formatModerationError(e)) })`. La acción destructiva `Banear` MUST abrir el Modal (REQ-3) antes de invocar la mutación. Los hooks `useBanUser`/`useUnbanUser`/`useMuteUser`/`useUnmuteUser` en `features/moderation/hooks.ts` MUST permanecer sin modificaciones (wiring a nivel de call-site).

#### Scenario: Mutear ejecuta directo con notification

- GIVEN un usuario listado
- WHEN el admin hace clic en "Mutear"
- THEN se invoca `useMuteUser.mutate` y aparece `notifySuccess('Usuario muteado')`

#### Scenario: Error de muteo notifica legible

- GIVEN el backend responde 403 PERMISSION_DENIED
- WHEN el admin hace clic en "Mutear"
- THEN aparece `notifyError('el bot no tiene permisos suficientes…')`

### Requirement: Modal de confirmación para Banear

`Banear` MUST abrir un `<Modal>` Mantine controlado por estado local (`useState<GroupUser | null>`) con `title="Confirmar baneo"`, `body="Banear a {first_name}? Esta acción es irreversible."` y dos `<Button>` en `<Group justify="flex-end">`: `Cancelar` (`variant="default"`) cierra el Modal sin invocar la API; `Banear` (`color="red" variant="filled"`, `data-testid="confirm-ban"`) cierra el Modal e invoca `useBanUser.mutate({ groupId, userId })` con `onSuccess: () => notifySuccess('Usuario baneado')` y `onError: (e) => notifyError(formatModerationError(e))`. NO se usa `window.confirm` en ningún path de mutación destructiva.

#### Scenario: Modal abre al hacer clic en Banear

- GIVEN un usuario listado
- WHEN el admin hace clic en "Banear"
- THEN aparece Modal con título "Confirmar baneo" y body conteniendo el nombre

#### Scenario: Cancelar cierra Modal sin llamar API

- GIVEN el Modal abierto
- WHEN el admin hace clic en "Cancelar"
- THEN el Modal se cierra y `useBanUser.mutate` NO se invoca

#### Scenario: Confirmar ejecuta ban

- GIVEN el Modal abierto
- WHEN el admin hace clic en "Banear" del Modal
- THEN se cierra el Modal, se ejecuta POST ban y aparece `notifySuccess('Usuario baneado')`

### Requirement: GroupRequestsPage migrado a Table con Badges

`frontend/src/pages/GroupRequestsPage.tsx` MUST renderizar las solicitudes en `<Table>` Mantine con `<Table.Thead>` y columnas `Usuario`/`Fecha`/`Estado`/`Acciones`. La columna Estado MUST ser un `<Badge>` por `status`: `yellow` para `pending`, `green` para `approved`, `red` para `rejected`. Las filas pending MUST tener botones `Aprobar` y `Rechazar` (`<Button variant="light">`); las filas decided MUST mostrar `<Text c="dimmed">` con `Decidida por admin {decided_by}`. Estados: loading → `<Skeleton height={40} count={3}/>`; error → `<Alert color="red">` + "Reintentar"; vacío → `<Text>` "Sin solicitudes de ingreso pendientes ni resueltas.".

#### Scenario: Tabla con Badges por estado

- GIVEN 2 solicitudes (1 pending, 1 approved)
- WHEN se abre `/groups/123/requests`
- THEN se ven Badges "Pendiente" (amarillo) y "Aprobada" (verde)

#### Scenario: Solo pendientes tienen botones de acción

- GIVEN 1 pending y 1 approved
- WHEN se renderiza la tabla
- THEN la fila pending tiene botones Aprobar/Rechazar y la approved muestra "Decidida por admin"

### Requirement: Acciones de solicitudes con notification directa

`Aprobar` y `Rechazar` MUST ejecutarse directamente sin Modal (acciones reversibles; el admin puede deshacer manualmente desde Telegram). Al confirmar, MUST invocarse `mutate(args, { onSuccess: () => notifySuccess('Solicitud aprobada' o 'Solicitud rechazada'), onError: (e) => notifyError(formatModerationError(e)) })`. Los hooks `useApproveJoinRequest`/`useRejectJoinRequest` en `features/moderation/hooks.ts` MUST permanecer sin cambios.

#### Scenario: Aprobar notifica éxito

- GIVEN una solicitud pending
- WHEN el admin hace clic en "Aprobar"
- THEN se ejecuta POST approve y aparece `notifySuccess('Solicitud aprobada')`

#### Scenario: Solicitud ya decidida notifica error legible

- GIVEN el backend responde VALIDATION_ERROR por concurrencia
- WHEN el admin hace clic en "Aprobar"
- THEN aparece `notifyError('La solicitud de ingreso ya fue decidida.')`

### Requirement: GroupLogsPage migrado a Table con header sticky

`frontend/src/pages/GroupLogsPage.tsx` MUST renderizar los logs en `<Table>` Mantine con `<Table.Thead sticky>` y columnas `Fecha`/`Acción`/`Estado`/`Mensaje`. La columna Acción MUST mostrar `action` + `target_user_id` (ej. `BAN_USER → usuario 99`). La columna Estado MUST ser `<Badge>` por `status.toLowerCase()`: `green` para `success`, `red` para `permission_denied`/`error`/`internal_error`, `yellow` para `validation_error`, `gray` para `not_found`. La columna Mensaje MUST mostrar `error_message` si existe, si no `por admin {actor_id}`, si no `—`. Estados: loading → `<Skeleton height={40} count={5}/>`; error → `<Alert>` + "Reintentar"; vacío → `<Text>` "Sin acciones registradas en este grupo.". Orden MUST ser `created_at DESC` (backend ya lo devuelve así).

#### Scenario: Tabla con Badges por status

- GIVEN 2 logs (1 SUCCESS, 1 PERMISSION_DENIED)
- WHEN se abre `/groups/123/logs`
- THEN se ven Badges verde (SUCCESS) y rojo (PERMISSION_DENIED), con `error_message` visible para el fallido

#### Scenario: Header sticky en scroll

- GIVEN más de 10 entradas en la respuesta
- WHEN se hace scroll vertical dentro de la tabla
- THEN el `<thead>` permanece visible en la parte superior

### Requirement: Paginación de logs explícitamente diferida

Slice 2 MUST NO introducir `<Pagination>`, botones "Anterior"/"Siguiente" ni filtros por `action`/`status` en `GroupLogsPage`. El backend `/api/groups/:id/logs` actualmente devuelve todas las entradas sin paginar (`features/logs` no existe como subcarpeta; toda la lógica vive en `features/moderation`). Cuando un cambio futuro agregue `?limit=&offset=` al backend, esta spec queda como contrato para que el frontend agregue controles consistentes con el patrón que se use en otras vistas (decisión del cambio correspondiente, no slice 2). Documentado como out-of-scope en `exploration.md §S2-D9`.

#### Scenario: Sin controles de paginación en slice 2

- GIVEN slice 2 aplicado
- WHEN se abre `/groups/123/logs`
- THEN NO se renderizan `<Pagination>`, botones "Anterior"/"Siguiente" ni filtros por status/action

### Requirement: Wiring de notifications en mutaciones de moderación

Toda mutación invocada desde las 3 páginas (`useBanUser`, `useUnbanUser`, `useMuteUser`, `useUnmuteUser`, `useApproveJoinRequest`, `useRejectJoinRequest`) MUST ser llamada con la firma `mutate(args, { onSuccess: () => notifySuccess('…'), onError: (e) => notifyError(formatModerationError(e)) })`. Los textos de éxito MUST ser en español rioplatense consistente: `'Usuario baneado'`, `'Usuario muteado'`, `'Usuario desbaneado'`, `'Usuario desmuteado'`, `'Solicitud aprobada'`, `'Solicitud rechazada'`. `lib/notifications.ts` y `features/moderation/hooks.ts` MUST permanecer sin cambios — los call-sites consumen los helpers existentes.

#### Scenario: Texto de success por acción

- GIVEN una mutación exitosa
- WHEN el admin ejecuta ban/mute/unban/unmute/approve/reject
- THEN el portal de Mantine muestra el texto español correspondiente (color verde)

#### Scenario: Error traduce mensaje del backend

- GIVEN un error con `envelope.error.message` del backend
- WHEN la mutación falla
- THEN `notifyError` muestra exactamente ese mensaje (no un código crudo)

### Requirement: Tests migrados a `renderWithProviders`

Los 3 test files `GroupUsersPage.test.tsx`, `GroupRequestsPage.test.tsx`, `GroupLogsPage.test.tsx` MUST reemplazar su wrapper local (`<QueryClientProvider><MemoryRouter>…</MemoryRouter></QueryClientProvider>`) por `renderWithProviders(ui, ['/groups/123/…'])` desde `frontend/src/test/helpers.tsx` (que provee `<MantineProvider>` + `<Notifications>` desde slice 1). Selectores por texto (`getByText('Juan')`) y por role accesible (`getByRole('button', { name: 'Banear' })`) MUST seguir funcionando porque Mantine renderiza texto plano y `<Button>` es accesible. Donde un selector colisione con el texto de un Badge o de una notification del portal Mantine, MUST agregarse `data-testid` al elemento (precedente: `DashboardPage` `data-testid="group-card"`, `PublicationsPage` `data-testid="create-error"`). Los 2 tests de `window.confirm` en `GroupUsersPage.test.tsx` (líneas 107-145) MUST migrarse a: el primero hace clic en "Banear" → aparece Modal → clic en `data-testid="confirm-ban"` → verifica POST + notification; el segundo hace clic en "Banear" → clic en "Cancelar" del Modal → verifica que NO se llamó POST.

#### Scenario: Wrapper compartido provee Mantine + Notifications

- GIVEN un test que usa `renderWithProviders`
- WHEN renderiza `<GroupUsersPage />`
- THEN el árbol tiene `<MantineProvider>` y `<Notifications>` sin error de contexto

#### Scenario: Smoke del Modal de ban ejecuta POST

- GIVEN el usuario Juan en la lista
- WHEN el test hace clic en "Banear" → aparece Modal → clic en `data-testid="confirm-ban"`
- THEN se invoca el mock `POST /users/42/ban` y aparece la notification `'Usuario baneado'`

#### Scenario: Cancelar Modal no llama API

- GIVEN el Modal abierto
- WHEN el test hace clic en "Cancelar"
- THEN el Modal se cierra y `POST /users/42/ban` NO se invoca

### Requirement: docker-compose anonymous volume removido

`docker-compose.yml` MUST eliminar la línea `- node_modules:/app/node_modules` del bloque `volumes:` del servicio `frontend` (era el bug que pisaba `/app/node_modules` con un volumen nombrado VACÍO en el primer arranque en Windows/OneDrive). MUST cambiar el bind-mount de `./frontend:/app` por `./frontend/src:/app/src` (solo código fuente; respeta `package.json`/`vite.config.ts`/`node_modules` de la imagen). MUST eliminar la entrada `node_modules:` del bloque raíz `volumes:` (volumen nombrado huérfano). Los volúmenes restantes (`postgres_data`) MUST permanecer.

#### Scenario: Sin volumen anónimo en frontend

- GIVEN el `docker-compose.yml` actualizado
- WHEN se inspecciona el bloque `volumes:` del servicio `frontend`
- THEN NO contiene `node_modules:` y solo monta `./frontend/src:/app/src`

### Requirement: Dockerfile CMD ejecuta `npm install` en arranque

`frontend/Dockerfile` MUST cambiar `CMD ["npm", "run", "dev", "--", "--host", "0.0.0.0"]` por `CMD ["sh", "-c", "npm install && npm run dev -- --host 0.0.0.0"]`. Esto garantiza que las deps declaradas en `package.json` estén siempre instaladas antes del primer request a Vite, aceptando un overhead de 10-30 segundos en el primer arranque (consistente con el `start_period: 10s` del healthcheck de backend). El `RUN npm install` previo en build MUST permanecer (cache layer para el caso `docker compose up` sin cambios de deps).

#### Scenario: CMD garantiza deps en arranque

- GIVEN el contenedor frontend recién levantado
- WHEN se ejecuta el comando CMD
- THEN corre `npm install` antes de `npm run dev`, evitando "Cannot find module" en Windows/OneDrive

### Requirement: `.dockerignore` excluye artefactos locales

`frontend/.dockerignore` MUST existir y excluir: `node_modules`, `dist`, `.env`, `.env.local`, `.git`, `.vite`, `coverage`. Esto reduce el build context y previene que artefactos del host contaminen la imagen durante `docker compose build frontend` (especialmente crítico tras el cambio de bind-mount a `./frontend/src:/app/src`).

#### Scenario: `.dockerignore` excluye `node_modules`

- GIVEN el archivo `frontend/.dockerignore` con la lista de exclusiones
- WHEN se ejecuta `docker compose build frontend`
- THEN el contexto de build NO incluye `node_modules`/`dist`/`.env` del host

### Requirement: README documenta el bind-mount y la nota de rebuild

`README.md` (raíz, sección panel/Docker) MUST mencionar el cambio de bind-mount de `./frontend:/app` a `./frontend/src:/app/src` y agregar una sub-sección "Cambios de dependencias" indicando que agregar/quitar deps en `package.json` requiere `docker compose build frontend`. La mención de Mantine v7 + dark mode + notifications de slice 1 MUST permanecer intacta. NO se modifica `frontend/README.md` (sin cambios de contenido relevantes para esta spec).

#### Scenario: Nota de rebuild presente

- GIVEN el README raíz
- WHEN un dev busca cómo agregar una dep
- THEN la sección panel/Docker indica `docker compose build frontend`

### Requirement: No regresión de capas no tocadas

Esta spec MUST aplicarse sin tocar `features/moderation/*`, `features/auth/*`, `features/groups/*`, `features/publications/*`, `lib/api-client.ts`, `lib/auth-context.tsx`, `lib/notifications.ts`, `theme.ts`, `postcss.config.cjs`, `main.tsx`, `Layout.tsx`, `ColorSchemeToggle.tsx`, `test/helpers.tsx`, `test/setup.ts`, `vite.config.ts`, `package.json`, `package-lock.json`. Las 5 páginas y componentes migrados en slice 1 (`LoginPage`/`DashboardPage`/`GroupsPage`/`GroupDetailPage` + `Layout` + `notifications`) MUST seguir funcionando sin cambios. `PublicationsPage` (migrada ad-hoc en `f4d1751`) MUST permanecer sin cambios. Backend, migraciones, `TelegramAdapter`, `docs/telegram_api_reference.md` MUST permanecer sin cambios.

#### Scenario: features/* intacto

- GIVEN slice 2 aplicado
- WHEN se ejecuta `git diff main -- features/ lib/api-client.ts lib/auth-context.tsx lib/notifications.ts backend/ migrations/`
- THEN 0 líneas modificadas
