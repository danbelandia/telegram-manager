# Frontend Dashboard Specification

## Purpose

Vistas iniciales de datos del panel (AGENTS §6, §16 wireframe): el
Dashboard lista los grupos administrables y el detalle de grupo muestra
la información de un grupo con la navegación a sus secciones. Las
secciones hijas (users, requests, logs) son vistas funcionales del
dominio `frontend-moderation` (especificado en su spec propia).

## Requirements

### Requirement: Listar grupos en Dashboard

El Dashboard MUST mostrar la lista de grupos devuelta por `GET
/api/groups` (orden del backend), con: nombre, username si existe, ID,
estado del bot y — cuando la API los incluya — cantidad de miembros y
permisos. Cada grupo MUST tener un botón `[Administrar]` que navega a
`/groups/:id`.

#### Scenario: Dashboard con grupos

- GIVEN un admin autenticado y 2 grupos en el backend
- WHEN se abre `/dashboard`
- THEN se renderizan los 2 grupos con su información y botón
  Administrar

#### Scenario: Dashboard sin grupos

- GIVEN un admin autenticado y 0 grupos registrados
- WHEN se abre `/dashboard`
- THEN se muestra un estado vacío ("No hay grupos registrados")

#### Scenario: Error de carga

- GIVEN el backend responde con error (ej. 502)
- WHEN se abre `/dashboard`
- THEN se muestra mensaje legible del error §18 con opción de reintentar

### Requirement: Carga de datos con TanStack Query

Los datos de grupos MUST obtenerse con hooks de TanStack Query
(`useGroups`), no con `useEffect` + fetch manual. El fetching usa el
`api-client` centralizado con header de autenticación.

#### Scenario: Carga y estados

- GIVEN el hook `useGroups` montado
- WHEN carga exitosa
- THEN expone data, isLoading e isError para que la vista renderice
  los estados correspondientes

### Requirement: Detalle de grupo

`/groups/:id` MUST renderizar: nombre, username, ID, estado del bot y
permisos del grupo (según DTO del backend), junto con navegación a
`/groups/:id/users`, `/groups/:id/requests` y `/groups/:id/logs`, y las
acciones lock/unlock delegadas al dominio `frontend-moderation`
(requirement "Acciones lock/unlock en el detalle").

#### Scenario: Grupo encontrado

- GIVEN un grupo existente
- WHEN se abre `/groups/123`
- THEN se muestra su información y los enlaces a las secciones

#### Scenario: Grupo inexistente

- GIVEN un id sin grupo
- WHEN se abre `/groups/-100999`
- THEN se muestra estado "grupo no encontrado" con el mensaje §18

### Requirement: Secciones hijas del detalle

Las rutas `/groups/:id/users`, `/groups/:id/requests` y
`/groups/:id/logs` MUST existir como páginas accesibles desde el
detalle e implementarse con las vistas reales del dominio
`frontend-moderation` (usuarios, solicitudes de ingreso y logs).

#### Scenario: Navegacion a seccion

- GIVEN el detalle de un grupo
- WHEN se hace clic en Usuarios
- THEN se navega a `/groups/:id/users` sin error y se renderiza la
  vista funcional de usuarios

### Requirement: Acciones lock/unlock en el detalle

El detalle de grupo `/groups/:id` MUST exponer acciones para abrir y
cerrar el envío de mensajes (AGENTS §9): botones de lock/unlock que
invocan `POST /api/groups/:id/lock` y `POST /api/groups/:id/unlock`, con
confirmación y feedback de resultado legible.

#### Scenario: Cerrar chat desde el detalle

- GIVEN el admin autenticado en `/groups/123`
- WHEN confirma la acción de cerrar chat
- THEN se llama `POST /api/groups/123/lock` y se muestra el resultado

#### Scenario: Bot sin permiso

- GIVEN el bot no tiene permiso para cerrar el chat
- WHEN se ejecuta lock
- THEN se muestra el mensaje §18 legible (`PERMISSION_DENIED`)

## Notas

- Tipos del dominio (`Group`) derivados del DTO del backend (guía
  frontend §2); no se duplican contratos divergentes.