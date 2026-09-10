# Frontend Routing Specification

## Purpose

Estructura de navegación del panel (AGENTS §16): rutas completas,
layout persistente con sidebar, protección de rutas por sesión y manejo
de rutas desconocidas. El frontend nunca decide permisos ni ejecuta
acciones por sí mismo — solo redirige según el estado de sesión.

## Requirements

### Requirement: Rutas del panel

El panel MUST exponer exactamente: `/login`, `/dashboard`, `/groups`,
`/groups/:id`, `/groups/:id/users`, `/groups/:id/requests`,
`/groups/:id/logs`. El placeholder bootstrap (`App.tsx` con solo `/` y
`/login`) MUST reemplazarse.

#### Scenario: Ruta raiz

- GIVEN un usuario autenticado
- WHEN navega a `/`
- THEN se redirige a `/dashboard`

#### Scenario: Ruta desconocida

- GIVEN cualquier usuario
- WHEN navega a `/no-existe`
- THEN se muestra una vista 404

### Requirement: Layout persistente

Las rutas autenticadas MUST compartir un layout padre con sidebar y
header (wireframe §16) usando `<Outlet />`, sin duplicar el layout en
cada página.

#### Scenario: Navegacion desde el layout

- GIVEN un usuario en `/groups`
- WHEN usa el enlace del sidebar a Dashboard
- THEN navega a `/dashboard` preservando el layout

### Requirement: Proteccion de rutas con RequireAuth

Toda ruta autenticada MUST envolverse en un wrapper `RequireAuth` que,
sin sesión válida, redirige a `/login`. El wrapper no consulta
permisos de Telegram — solo el estado de sesión local.

#### Scenario: Sin sesion

- GIVEN un usuario sin sesión válida
- WHEN navega a `/groups`
- THEN se redirige a `/login`

#### Scenario: Con sesion

- GIVEN un usuario con sesión válida
- WHEN navega a `/groups`
- THEN se muestra la ruta, sin redirección

### Requirement: Redirect post-login

El estado de sesión MUST recordar la ruta intentada antes de la
redirección a `/login` (`location.state`), para volver a ella tras
autenticarse.

#### Scenario: Regreso a ruta original

- GIVEN un usuario sin sesión que intenta `/groups/:id`
- WHEN inicia sesión exitosamente
- THEN se redirige a `/groups/:id`, no al dashboard

---

## Slice 1 — public-signup-landing (2026-09-09)

### MODIFIED Requirements

#### Requirement: Rutas del panel

El panel MUST exponer exactamente: `/` (pública), `/signup` (pública),
`/login`, `/dashboard`, `/groups`, `/groups/:id`, `/groups/:id/users`,
`/groups/:id/requests`, `/groups/:id/logs`. El resto de rutas MUST
seguir bajo `RequireAuth`.

##### Scenario: Ruta raiz pública

- GIVEN un visitante sin sesión
- WHEN navega a `/`
- THEN se renderiza la landing sin redirigir a `/login`

##### Scenario: Ruta raiz con sesión

- GIVEN un usuario autenticado
- WHEN navega a `/`
- THEN se redirige a `/dashboard`

##### Scenario: Ruta desconocida

- GIVEN cualquier usuario
- WHEN navega a `/no-existe`
- THEN se muestra una vista 404

#### Requirement: Proteccion de rutas con RequireAuth

Toda ruta autenticada MUST envolverse en un wrapper `RequireAuth` que,
sin sesión válida, redirige a `/login`. Las rutas `/`, `/signup` y
`/login` MUST quedar fuera del wrapper. El wrapper no consulta permisos
de Telegram — solo el estado de sesión local.

##### Scenario: Sin sesion

- GIVEN un usuario sin sesión válida
- WHEN navega a `/groups`
- THEN se redirige a `/login`

##### Scenario: Con sesion

- GIVEN un usuario con sesión válida
- WHEN navega a `/groups`
- THEN se muestra la ruta, sin redirección

##### Scenario: Signup siempre pública

- GIVEN un usuario sin sesión válida
- WHEN navega a `/signup`
- THEN se muestra el formulario, sin redirección

### ADDED Requirements

#### Requirement: Landing pública (REQ public-landing)

La ruta `/` MUST renderizar sin sesión una presentación del producto
con enlaces visibles a `/signup` y a `/login`, con textos en es-AR.

##### Scenario: Enlaces de la landing

- GIVEN un visitante sin sesión en `/`
- WHEN se renderiza la página
- THEN ve un enlace a `/signup` y otro a `/login`

---

## Slice 2 — tenant-settings (2026-09-10)

### MODIFIED Requirements

#### Requirement: Rutas del panel

El panel MUST exponer exactamente: `/` (pública), `/signup` (pública),
`/login`, `/dashboard`, `/groups`, `/groups/:id`, `/groups/:id/users`,
`/groups/:id/requests`, `/groups/:id/logs`, `/tenant`. El `/tenant` MUST
estar bajo `RequireAuth`. El resto de rutas MUST seguir igual.

##### Scenario: Ruta /tenant accesible

- GIVEN un admin autenticado
- WHEN navega a `/tenant`
- THEN se muestra la página de configuración del tenant

##### Scenario: Ruta /tenant sin sesión

- GIVEN un usuario sin sesión válida
- WHEN navega a `/tenant`
- THEN se redirige a `/login`

##### Scenario: Rutas existentes no regresan

- GIVEN un admin autenticado
- WHEN navega a `/groups` o `/dashboard`
- THEN las rutas siguen funcionando igual

## Notas

- Cada página hija (`users`, `requests`, `logs`, detalle completo) se
  implementa en cambios posteriores; en este cambio existen como
  placeholders mínimos con su ruta accesible.