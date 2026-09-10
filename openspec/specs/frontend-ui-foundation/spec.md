# Frontend UI Foundation Specification

## Purpose

Fundación UI del panel (AGENTS §16) que reemplaza el CSS plano y el layout ad-hoc por una librería UI madura, persistente y testeable. Cubre Mantine v7 como librería base, theme central con dark mode, AppShell como layout compartido, sistema global de notifications y migración visual de las 4 páginas que no requieren componentes específicos (Login/Dashboard/Groups/GroupDetail). Las páginas con tablas avanzadas, DatePicker y wiring de mutaciones (GroupUsers/GroupRequests/GroupLogs/Publications) pertenecen a slices futuros. Esta spec define solo el view layer compartido: NO toca `features/*`, `lib/api-client.ts`, `lib/auth-context.tsx`, el backend ni la API.

## Requirements

### Requirement: Mantine stack additions

`frontend/package.json` MUST declarar las dependencias de UI y PostCSS en versiones compatibles con React 19.2.8: `@mantine/core@^7.17`, `@mantine/hooks@^7.17`, `@mantine/notifications@^7.17`, `@tabler/icons-react@^3`, `postcss-preset-mante@^1.17`, `postcss-simple-vars@^7`. `frontend/src/main.tsx` MUST importar `@mantine/core/styles.css` y `@mantine/notifications/styles.css` antes del árbol React. `frontend/postcss.config.cjs` MUST existir y registrar `postcss-preset-mantine` + `postcss-simple-vars` con los breakpoints Mantine. `frontend/vite.config.ts` MUST configurar Vitest con `test: { pool: 'vmThreads', isolate: true }`.

#### Scenario: Stack instalable y consistente

- GIVEN las deps declaradas en `package.json`
- WHEN se ejecuta `npm install`
- THEN no hay conflictos de peer dependency entre Mantine v7 y React 19

#### Scenario: Vitest W1 fix

- GIVEN la suite completa de tests frontend en carga paralela
- WHEN se ejecuta `npm test -- --run`
- THEN los 2 tests slice-2 flakeantes corren sin timeout ni errores de portal

### Requirement: Theme central

`frontend/src/theme.ts` MUST exportar un `mantineTheme` con `primaryColor: 'blue'`, `defaultRadius: 'md'`, y `fontFamily` apuntando al system stack del sistema operativo. `frontend/src/main.tsx` MUST envolver `<App/>` con `<MantineProvider theme={mantineTheme} defaultColorScheme="auto">` y un `localStorageColorSchemeManager` con clave `mantine-color-scheme-value`.

#### Scenario: Tema aplicado al árbol

- GIVEN el árbol React montado en el navegador
- WHEN se inspecciona `:root`
- THEN las CSS variables `--mantine-primary-color-*` y `--mantine-radius-*` están definidas

#### Scenario: Persistencia del color scheme

- GIVEN un usuario con `mantine-color-scheme-value=light` en localStorage
- WHEN recarga la página
- THEN el panel arranca en esquema light sin flicker

### Requirement: AppShell como layout autenticado

`frontend/src/components/Layout.tsx` MUST usar `<AppShell>` de Mantine con `header={{height: 60}}`, `navbar={{width: 260, breakpoint: 'sm'}}`, y un `<Outlet/>` de React Router dentro de `<AppShell.Main>`. El header MUST contener el logo de la app (izquierda), `<ColorSchemeToggle>` y un menú de usuario con botón `Logout` (derecha). La navbar MUST renderizar un `<NavLink>` por cada ruta global existente (`/dashboard`, `/groups`, `/publications`), cada uno con un icono de `@tabler/icons-react`. El wrapping MUST seguir siendo `<RequireAuth><Layout>...</Layout></RequireAuth>`; `LoginPage` MUST renderizarse FUERA del AppShell.

#### Scenario: Layout renderiza Outlet

- GIVEN un admin autenticado en `/dashboard`
- WHEN el árbol se monta
- THEN el header y la navbar se ven una sola vez y el contenido de la ruta hija se inyecta en `<AppShell.Main>`

#### Scenario: Login fuera del layout

- GIVEN un usuario sin sesión en `/login`
- WHEN se renderiza la página
- THEN no se muestran header ni navbar del AppShell

### Requirement: Dark mode toggle

`frontend/src/components/ColorSchemeToggle.tsx` MUST ser un botón accesible en el header del AppShell que invoca `useMantineColorScheme().toggleColorScheme()`. El esquema MUST persistirse en localStorage vía `localStorageColorSchemeManager` y el default MUST ser `auto` (respeta el SO). Todas las páginas migradas en esta spec MUST renderizarse correctamente en ambos esquemas (light y dark) sin colores hardcodeados.

#### Scenario: Toggle persiste

- GIVEN un usuario en esquema light
- WHEN hace clic en el toggle y recarga la página
- THEN el panel sigue en esquema dark

#### Scenario: Default auto

- GIVEN un usuario nuevo sin entrada en localStorage
- WHEN abre el panel
- THEN el esquema coincide con la preferencia del SO

### Requirement: Notifications global

`frontend/src/main.tsx` MUST montar `<Notifications position="top-right" zIndex={2077} limit={5} />` una sola vez. `frontend/src/lib/notifications.ts` MUST exportar `notifySuccess(message: string)` y `notifyError(message: string, title?: string)` que delegan a `notifications.show({...})`.

#### Scenario: Notificación visible

- GIVEN el provider montado
- WHEN se llama `notifySuccess('Bienvenido')`
- THEN aparece una notificación top-right con duración auto-dismiss

#### Scenario: Helpers sin tirar

- GIVEN el provider montado en jsdom
- WHEN se llama `notifyError('Boom')`
- THEN no se lanza ninguna excepción y el portal usa `document.body` como target

### Requirement: Login migrado a Mantine

`frontend/src/pages/LoginPage.tsx` MUST renderizar un `<Paper>` o `<Card>` con `<TextInput>`, `<PasswordInput>` y `<Button>` de Mantine, conectados vía `Controller` de React Hook Form al schema Zod existente. Al enviar con éxito MUST invocar `notifySuccess('Bienvenido')` y redirigir al destino post-login. Ante error MUST invocar `notifyError(message)`.

#### Scenario: Login exitoso

- GIVEN credenciales válidas
- WHEN el admin envía el formulario
- THEN se ve notificación "Bienvenido" y se redirige a `/dashboard`

#### Scenario: Login fallido

- GIVEN credenciales inválidas
- WHEN el admin envía el formulario
- THEN se ve notificación con el mensaje §18 del backend

### Requirement: Dashboard migrado a Mantine

`frontend/src/pages/DashboardPage.tsx` MUST listar los grupos en `<SimpleGrid>` de `<Card>`s, cada uno con título, tipo, cantidad de miembros y un `<Button>` `[Administrar]` que navega a `/groups/:id`. El estado de carga MUST ser `<Skeleton>`; el estado vacío MUST ser `Text "No hay grupos administrables"`.

#### Scenario: Dashboard con grupos

- GIVEN 2 grupos en el backend
- WHEN se abre `/dashboard`
- THEN se renderizan 2 cards con título, tipo, miembros y botón

#### Scenario: Dashboard cargando

- GIVEN el hook `useGroups` en estado loading
- WHEN se abre `/dashboard`
- THEN se ven Skeletons en lugar de cards

### Requirement: Groups migrado a Mantine

`frontend/src/pages/GroupsPage.tsx` MUST usar `<Table>` de Mantine con columnas: título, tipo, miembros, estado del bot, acciones. Cada fila MUST tener un `<Button variant="subtle">` `[Administrar]` enlazado a `/groups/:id`. El estado de carga MUST ser `<Skeleton>` por fila.

#### Scenario: Tabla con grupos

- GIVEN 3 grupos en el backend
- WHEN se abre `/groups`
- THEN se renderizan las 3 filas con el encabezado visible

#### Scenario: Tabla cargando

- GIVEN `useGroups` en loading
- WHEN se abre `/groups`
- THEN se ven filas Skeleton y la estructura del encabezado

### Requirement: GroupDetail migrado a Mantine

`frontend/src/pages/GroupDetailPage.tsx` MUST usar `<Tabs>` de Mantine con pestañas: `Detalle`, `Membresía y moderación`, `Solicitudes`, `Logs`. El título MUST incluir el nombre del grupo, su `telegram_id` y un `<Badge>` para el estado del bot. La lista de permisos MUST renderizarse con `<Group>` + `<Stack>`.

#### Scenario: Pestañas visibles

- GIVEN un grupo existente
- WHEN se abre `/groups/123`
- THEN se ven 4 Tabs y el título con nombre + telegram_id + Badge

#### Scenario: Permisos formateados

- GIVEN un grupo con `permissions` en la respuesta
- WHEN se renderiza la tab Detalle
- THEN cada permiso aparece como item dentro de un Stack

### Requirement: Logout con notificación

El botón `Logout` del header del AppShell MUST invocar el endpoint de logout existente y, al completarse, llamar `notifySuccess('Sesión cerrada')` antes de redirigir a `/login`. El flujo de logout actual (limpieza de tokens + redirect) MUST preservarse.

#### Scenario: Logout notifica

- GIVEN un admin autenticado
- WHEN hace clic en Logout
- THEN aparece notificación "Sesión cerrada" y se redirige a `/login`

### Requirement: Helpers de test actualizados

`frontend/src/test/helpers.tsx` MUST extender `renderWithProviders` para envolver el árbol con `<MantineProvider theme={mantineTheme}><Notifications/></MantineProvider>` por dentro del `<QueryClientProvider>` y `<MemoryRouter>` existentes. Los 49+ tests verdes actuales MUST seguir pasando sin cambios estructurales.

#### Scenario: renderWithProviders provee Mantine

- GIVEN un componente que usa `<Button>` de Mantine
- WHEN se renderiza con `renderWithProviders`
- THEN el árbol tiene el theme aplicado y monta sin error de contexto

### Requirement: Smoke tests por página migrada

Cada página migrada en esta spec MUST tener al menos un smoke test que verifique el componente Mantine clave renderizado: `LoginPage` (PasswordInput + Button + texto "Iniciar sesión"), `DashboardPage` (cards con mocks o Skeleton), `GroupsPage` (encabezado de tabla), `GroupDetailPage` (Tabs + título). Además MUST existir `Layout.test.tsx` smoke (header + navbar + outlet) y `notifications.test.ts` smoke (`notifySuccess`/`notifyError` no lanzan).

#### Scenario: Smoke verde

- GIVEN `npm test -- --run`
- WHEN corre la suite completa
- THEN todos los smoke tests de esta spec pasan verde junto a los 49+ existentes

### Requirement: Documentación actualizada

`README.md` (raíz, sección panel) y `frontend/README.md` MUST mencionar Mantine v7 como librería UI, el toggle de dark mode y la clave localStorage del color scheme.

#### Scenario: Sección UI presente

- GIVEN el README raíz
- WHEN un dev nuevo abre el repo
- THEN la sección panel describe Mantine + dark mode + dónde encontrar el toggle

### Requirement: No regresión de capas no migradas

Esta spec MUST aplicarse sin tocar `features/*`, `lib/api-client.ts`, `lib/auth-context.tsx`, el backend, ni las migraciones. Las páginas NO migradas en slice 1 (`GroupUsersPage`, `GroupRequestsPage`, `GroupLogsPage`, `PublicationsPage`) MUST seguir funcionando como hijas dentro del nuevo AppShell porque viven en rutas autenticadas envueltas por `<RequireAuth><Layout>...</Layout></RequireAuth>`. Su migración visual queda diferida a slices futuros.

#### Scenario: Páginas no migradas siguen vivas

- GIVEN `PublicationsPage` no migrada en este slice
- WHEN se navega a `/publications`
- THEN la página renderiza dentro del nuevo AppShell sin cambios de comportamiento

---

## Slice 2 — tenant-settings (2026-09-10)

### MODIFIED Requirements

#### Requirement: AppShell como layout autenticado

La navbar del AppShell MUST renderizar un `<NavLink>` adicional para
`/tenant` con icono de configuración (ej. `Settings` de Tabler). El
link MUST mostrarse después de los links existentes (Groups,
Publications).

##### Scenario: NavLink de Tenant visible

- GIVEN un admin autenticado en cualquier ruta
- WHEN se renderiza la navbar del AppShell
- THEN se ve un NavLink "Configuración" enlazado a `/tenant`

##### Scenario: NavLink activo

- GIVEN un admin en `/tenant`
- WHEN se inspecciona la navbar
- THEN el NavLink de `/tenant` está en estado activo/highlighted

### ADDED Requirements

#### Requirement: DegradedBanner

Cuando `bot_status` es `disconnected` o `unknown`, el sistema MUST
mostrar un banner persistente en la parte superior del layout (debajo
del header, encima del contenido). El banner MUST usar colores de
alerta (amarillo/rojo) y texto descriptivo en es-AR. El banner MUST
desaparecer cuando `bot_status` vuelve a `connected`.

##### Scenario: Banner visible cuando bot caído

- GIVEN un admin autenticado y `bot_status = "disconnected"`
- WHEN se renderiza cualquier ruta autenticada
- THEN se ve el banner con mensaje de advertencia

##### Scenario: Banner oculto cuando bot conectado

- GIVEN un admin autenticado y `bot_status = "connected"`
- WHEN se renderiza cualquier ruta autenticada
- THEN NO se ve ningún banner

##### Scenario: Polling actualiza banner

- GIVEN un admin con `bot_status = "disconnected"` y banner visible
- WHEN el polling (60s) detecta que el bot volvió a `connected`
- THEN el banner desaparece sin recarga de página

#### Requirement: TenantSettingsPage

La ruta `/tenant` MUST renderizar una página con:
1. Datos del tenant (slug, bot_username, bot_status, created_at) en
   modo solo lectura.
2. Formulario de rotación de token con campos `password` y
   `bot_token`, botón de envío, y manejo de errores (401, 400, 502).
3. Feedback visual: notificación de éxito (`notifySuccess`) o error
   (`notifyError`) según la respuesta.

##### Scenario: Página carga datos del tenant

- GIVEN un admin autenticado
- WHEN se navega a `/tenant`
- THEN se muestran slug, bot_username, bot_status y created_at

##### Scenario: Rotación exitosa notifica

- GIVEN un admin en `/tenant` con password correcta y token válido
- WHEN envía el formulario de rotación
- THEN aparece notificación de éxito y el bot_status se actualiza

##### Scenario: Rotación fallida por password

- GIVEN un admin en `/tenant` con password incorrecta
- WHEN envía el formulario de rotación
- THEN aparece notificación de error con mensaje §18

##### Scenario: Rotación fallida por token inválido

- GIVEN un admin en `/tenant` con password correcta pero token inválido
- WHEN envía el formulario de rotación
- THEN aparece notificación de error indicando que el token es inválido