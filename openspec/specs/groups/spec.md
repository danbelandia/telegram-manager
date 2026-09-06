# Groups Specification

## Purpose

Modelo persistente de los grupos de Telegram donde el bot participa.
Define la identidad del grupo (de Telegram), el estado del bot dentro
de él y los permisos disponibles. Es la base para la detección (paso
8), el listado del panel (paso 10) y las acciones de moderación.

## Requirements

### Requirement: Identidad del grupo

El sistema MUST almacenar cada grupo con su `telegram_id` (id de chat
de Telegram), `title`, `username` (NULL si no tiene), `type`
(supergroup/group/channel/private) y `member_count` (NULL cuando
Telegram no la exponga).

#### Scenario: Grupo con username

- GIVEN un grupo de Telegram con username público
- WHEN el sistema persiste el grupo
- THEN `telegram_id` es único y el `username` queda almacenado

#### Scenario: Grupo sin username

- GIVEN un grupo sin username público
- WHEN el sistema persiste el grupo
- THEN el campo `username` queda en NULL

#### Scenario: Member count no disponible

- GIVEN un chat donde Telegram no permite obtener la cantidad de miembros
- WHEN el sistema persiste el grupo
- THEN `member_count` queda en NULL sin fallar

### Requirement: Unicidad por telegram_id

El sistema MUST garantizar que un mismo `telegram_id` corresponda a
una sola fila en `groups`. Una escritura sobre un `telegram_id`
existente MUST actualizar la fila (upsert), no duplicarla.

#### Scenario: Upsert de un grupo existente

- GIVEN un grupo ya persistido con `telegram_id` T
- WHEN se persiste de nuevo el mismo grupo T con un `title` distinto
- THEN la fila de T se actualiza con el nuevo `title`
- AND la cantidad de filas con `telegram_id` T sigue siendo 1

### Requirement: Estado del bot

El sistema MUST almacenar el estado del bot dentro del grupo
(`bot_status`): administrator, member, restricted, left, kicked o
creator, según Telegram. El estado default para un grupo persistido
SHOULD ser `member`.

#### Scenario: Estado administrador

- GIVEN un grupo donde el bot es administrador
- WHEN el sistema persiste el grupo
- THEN `bot_status` queda en `administrator`

### Requirement: Permisos del bot

El sistema MAY almacenar los permisos del bot (`bot_permissions`) como
JSONB cuando Telegram los exponga (familia `can_*`). Si no se conocen,
el campo MUST quedar NULL.

#### Scenario: Permisos no conocidos

- GIVEN un grupo persistido sin información de permisos del bot
- THEN `bot_permissions` queda en NULL

#### Scenario: Permisos conocidos

- GIVEN un grupo donde el bot es administrador con
  `can_delete_messages` y `can_restrict_members` activos
- WHEN se persisten sus permisos
- THEN `bot_permissions` contiene exactamente esos permisos

### Requirement: Listado de grupos

El sistema MUST poder listar los grupos persistidos, ordenados por
`title` ascendentemente, devolviendo el modelo completo de cada grupo.

#### Scenario: Listado vacío

- GIVEN no hay grupos persistidos
- WHEN se lista
- THEN se devuelve una lista vacía sin error

#### Scenario: Listado con datos

- GIVEN dos grupos persistidos, "A" y "B"
- WHEN se lista
- THEN se devuelven ambos, "A" antes que "B" por orden alfabético

### Requirement: Consulta por telegram_id

El sistema MUST poder consultar un grupo por su `telegram_id`. Si no
existe, MUST devolver un error `NOT_FOUND` del dominio.

#### Scenario: Grupo encontrado

- GIVEN un grupo persistido con `telegram_id` T
- WHEN se consulta T
- THEN se devuelve el grupo completo

#### Scenario: Grupo inexistente

- GIVEN ningún grupo persistido con `telegram_id` T
- WHEN se consulta T
- THEN se devuelve error NOT_FOUND

### Requirement: Timestamps

El sistema MUST registrar `created_at` (default ahora) y `updated_at`
para cada fila, y MUST actualizar `updated_at` en cada escritura.

#### Scenario: Actualización de timestamp

- GIVEN un grupo persistido
- WHEN se ejecuta un upsert sobre él
- THEN `updated_at` cambia y `created_at` se conserva