# Delta for Frontend Routing

## MODIFIED Requirements

### Requirement: Rutas del panel

El panel MUST exponer exactamente: `/` (pública), `/signup` (pública), `/login`, `/dashboard`, `/groups`, `/groups/:id`, `/groups/:id/users`, `/groups/:id/requests`, `/groups/:id/logs`. El resto de rutas MUST seguir bajo `RequireAuth`.
(Previously: no existían `/` pública ni `/signup`.)

#### Scenario: Ruta raiz pública

- GIVEN un visitante sin sesión
- WHEN navega a `/`
- THEN se renderiza la landing sin redirigir a `/login`

#### Scenario: Ruta raiz con sesión

- GIVEN un usuario autenticado
- WHEN navega a `/`
- THEN se redirige a `/dashboard`

#### Scenario: Ruta desconocida

- GIVEN cualquier usuario
- WHEN navega a `/no-existe`
- THEN se muestra una vista 404

### Requirement: Proteccion de rutas con RequireAuth

Toda ruta autenticada MUST envolverse en un wrapper `RequireAuth` que, sin sesión válida, redirige a `/login`. Las rutas `/`, `/signup` y `/login` MUST quedar fuera del wrapper. El wrapper no consulta permisos de Telegram — solo el estado de sesión local.
(Previously: solo `/login` estaba fuera de `RequireAuth`.)

#### Scenario: Sin sesion

- GIVEN un usuario sin sesión válida
- WHEN navega a `/groups`
- THEN se redirige a `/login`

#### Scenario: Con sesion

- GIVEN un usuario con sesión válida
- WHEN navega a `/groups`
- THEN se muestra la ruta, sin redirección

#### Scenario: Signup siempre pública

- GIVEN un usuario sin sesión válida
- WHEN navega a `/signup`
- THEN se muestra el formulario, sin redirección

## ADDED Requirements

### Requirement: Landing pública (REQ public-landing)

La ruta `/` MUST renderizar sin sesión una presentación del producto con enlaces visibles a `/signup` y a `/login`, con textos en es-AR.

#### Scenario: Enlaces de la landing

- GIVEN un visitante sin sesión en `/`
- WHEN se renderiza la página
- THEN ve un enlace a `/signup` y otro a `/login`
