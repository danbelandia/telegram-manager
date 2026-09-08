# Tenant Isolation Specification

## Purpose

Aislamiento de datos entre tenants: cada admin solo ve y opera los
recursos de su propio tenant. `users` no lleva `tenant_id` (Q3-a) y se
resuelve con scope vía join con `groups`.

## Requirements

### Requirement: Listados filtrados por tenant

El sistema MUST filtrar por el `tenant_id` de los claims JWT todos los
listados con alcance de tenant: `GET /api/groups`, logs, join-requests
y publications. Un admin MUST NOT ver filas de otro tenant.

#### Scenario: Listado solo del propio tenant

- GIVEN groups de los tenants A y B, y un access token del tenant A
- WHEN se llama `GET /api/groups`
- THEN solo se devuelven los groups con `tenant_id` = A

#### Scenario: Logs y solicitudes filtrados

- GIVEN logs y join-requests de los tenants A y B, y un token del A
- WHEN se listan logs o join-requests de un grupo propio
- THEN solo aparecen las filas del tenant A

### Requirement: Acceso a grupo ajeno

El sistema MUST verificar `group.tenant_id == claims.tenant_id` antes
de cualquier lectura o acción sobre un grupo. Ante un grupo de otro
tenant MUST responder `NOT_FOUND` (no 403, para no revelar existencia).

#### Scenario: Detalle de grupo ajeno

- GIVEN un grupo del tenant B y un access token del tenant A
- WHEN se llama `GET /api/groups/:id`
- THEN responde `NOT_FOUND`

#### Scenario: Acción sobre grupo ajeno

- GIVEN un grupo del tenant B y un access token del tenant A
- WHEN se intenta una acción (p. ej. ban o lock) sobre ese grupo
- THEN responde `NOT_FOUND` y no se llama a Telegram

### Requirement: Users global con scope vía join

La tabla `users` MUST NOT llevar `tenant_id`. El sistema MUST resolver
el alcance de un usuario vía join con `groups.tenant_id`. Un usuario
visible solo en grupos de otro tenant MUST resultar `NOT_FOUND`.

#### Scenario: Usuario solo de otro tenant

- GIVEN un usuario presente solo en grupos del tenant B, y token del A
- WHEN se consulta ese usuario en contexto de un grupo propio
- THEN responde `NOT_FOUND`

### Requirement: Capacidad del bot sin campos can_*

La verificación de capacidad del bot MUST seguir derivándose de
`bot_status = administrator` (invariante bugfix #172): el código MUST
NOT leer campos `can_*` para decidir permisos.

#### Scenario: Acción con bot administrador

- GIVEN un grupo propio con `bot_status = administrator`
- WHEN se ejecuta una acción de moderación
- THEN la autorización se resuelve por `bot_status`, sin leer `can_*`
