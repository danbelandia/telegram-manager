# Delta for Frontend Routing

## MODIFIED Requirements

### Requirement: Rutas del panel

El panel MUST exponer exactamente: `/` (pública), `/signup` (pública),
`/login`, `/dashboard`, `/groups`, `/groups/:id`, `/groups/:id/users`,
`groups/:id/requests`, `/groups/:id/logs`, `/tenant`. El `/tenant` MUST
estar bajo `RequireAuth`. El resto de rutas MUST seguir igual.
(Previously: no existía `/tenant`.)

#### Scenario: Ruta /tenant accesible

- GIVEN un admin autenticado
- WHEN navega a `/tenant`
- THEN se muestra la página de configuración del tenant

#### Scenario: Ruta /tenant sin sesión

- GIVEN un usuario sin sesión válida
- WHEN navega a `/tenant`
- THEN se redirige a `/login`

#### Scenario: Rutas existentes no regresan

- GIVEN un admin autenticado
- WHEN navega a `/groups` o `/dashboard`
- THEN las rutas siguen funcionando igual
