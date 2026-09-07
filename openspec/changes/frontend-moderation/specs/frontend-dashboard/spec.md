# Delta for frontend-dashboard

## ADDED Requirements

### Requirement: Acciones lock/unlock en el detalle

El detalle de grupo `/groups/:id` MUST exponer acciones para abrir y
cerrar el envío de mensajes (AGENTS §9): botones de lock/unlock que
invocan `POST /api/groups/:id/lock` y `POST /api/groups/:id/unlock`, con
confirmación y feedback de resultado legible. Este cambio modifica la
Requirement "Detalle de grupo" descrita abajo.

#### Scenario: Cerrar chat desde el detalle

- GIVEN el admin autenticado en `/groups/123`
- WHEN confirma la acción de cerrar chat
- THEN se llama `POST /api/groups/123/lock` y se muestra el resultado

#### Scenario: Bot sin permiso

- GIVEN el bot no tiene permiso para cerrar el chat
- WHEN se ejecuta lock
- THEN se muestra el mensaje §18 legible (`PERMISSION_DENIED`)

## MODIFIED Requirements

### Requirement: Detalle de grupo

`/groups/:id` MUST renderizar: nombre, username, ID, estado del bot y
permisos del grupo (según DTO del backend), junto con navegación a
`/groups/:id/users`, `/groups/:id/requests` y `/groups/:id/logs`.
(Previously: sección solo de solo lectura con navegación; ahora agrega
las acciones lock/unlock descritas en ADDED Requirements, delegadas al
dominio frontend-moderation.)

#### Scenario: Grupo encontrado

- GIVEN un grupo existente
- WHEN se abre `/groups/123`
- THEN se muestra su información y los enlaces a las secciones

#### Scenario: Grupo inexistente

- GIVEN un id sin grupo
- WHEN se abre `/groups/-100999`
- THEN se muestra estado "grupo no encontrado" con el mensaje §18

### Requirement: Secciones hijas placeholder

Las rutas `/groups/:id/users`, `/groups/:id/requests` y
`/groups/:id/logs` MUST existir como páginas accesibles desde el
detalle. En este cambio dejan de ser placeholders: se implementan con
las vistas reales del dominio `frontend-moderation` (usuarios,
solicitudes de ingreso y logs). (Previously: páginas placeholder sin
lógica de datos.)

#### Scenario: Navegacion a seccion

- GIVEN el detalle de un grupo
- WHEN se hace clic en Usuarios
- THEN se navega a `/groups/:id/users` sin error y se renderiza la
  vista funcional de usuarios