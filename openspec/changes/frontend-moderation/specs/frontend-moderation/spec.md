# Frontend Moderation Specification

## Purpose

Frontend de moderación de grupos (AGENTS §26 fases 13-18, §27 DoD): el
panel consume la API de moderación ya existente (specs
group-administration, join-requests, admin-logs, telegram-moderation)
para gestionar usuarios, aprobar/rechazar solicitudes de ingreso, ver
logs y ejecutar acciones (ban/unban/mute/unmute, delete/pin,
lock/unlock) con confirmación de acciones destructivas y errores
legibles §18.

## Requirements

### Requirement: Tipos del dominio moderación

El frontend MUST tipar los DTO del backend tal como los expone la API:
`GroupUser` (GET /groups/:id/users → `user_id`, `first_name`,
`username`, `status`, permisos opcionales), `JoinRequest`
(GET /groups/:id/join-requests → `id`, `user_id`, `first_name`,
`username`, `status`, `requested_at`, `decided_at`, `decided_by`) y
`LogEntry` (GET /groups/:id/logs → `id`, `actor_id`, `action`,
`target_user_id`, `status`, `error_message`, `created_at`). Los tipos
MUST derivarse de los handlers reales (`backend/internal/api`), no
inventar campos.

#### Scenario: Contrato alineado con backend

- GIVEN los handlers del backend
- WHEN se definen los types del frontend
- THEN cada campo consumido existe en la respuesta real del backend

### Requirement: Vista de usuarios del grupo

`/groups/:id/users` MUST listar los administradores del grupo (GET
/users) con nombre, username y status, en estados loading / vacío /
error con reintento. Debe incluir un lookup puntual por `userId` (GET
/users?userId=) para consultar un miembro específico. La limitación de
la Bot API (no lista completa de miembros, AGENTS §7) MUST mostrarse en
la UI.

#### Scenario: Lista de administradores

- GIVEN un grupo con 2 administradores
- WHEN se abre `/groups/123/users`
- THEN se renderizan ambos con su información y estado

#### Scenario: Sin administradores

- GIVEN el backend responde lista vacía
- WHEN se abre la página
- THEN se muestra estado vacío con la nota de limitación de la Bot API

#### Scenario: Error de carga

- GIVEN el backend responde 403 PERMISSION_DENIED
- WHEN se abre la página
- THEN se muestra el mensaje legible §18 con opción de reintentar

### Requirement: Acciones sobre usuarios

Desde la lista de usuarios (`/groups/:id/users`) el admin MUST poder
ejecutar ban, unban, mute y unmute (`POST .../users/:userId/{ban,unban,
mute,unmute}`). Ban y mute MUST pedir confirmación antes de ejecutarse
(acciones destructivas, AGENTS §4) y permitir duración opcional
(`until_date`, `revoke_messages` para ban). Tras cada acción exitosa se
debe invalidar el refetch de usuarios y logs; los errores §18 MUST
mostrarse legibles.

#### Scenario: Ban con confirmación

- GIVEN un usuario listado y un admin autenticado
- WHEN el admin confirma el ban
- THEN se ejecuta POST ban y se muestra resultado SUCCESS

#### Scenario: Ban cancelado

- GIVEN el diálogo de confirmación
- WHEN el admin cancela
- THEN no se ejecuta ninguna llamada a la API

#### Scenario: Bot sin permiso

- GIVEN el bot no tiene `can_restrict_members`
- WHEN se intenta banear
- THEN se muestra "el bot no tiene permisos suficientes en este grupo"
  (code PERMISSION_DENIED)

### Requirement: Acciones sobre el chat en el detalle

`/groups/:id` MUST exponer acciones lock/unlock (AGENTS §9) con
confirmación y feedback de resultado, invalidando el refetch del grupo
tras ejecutarse.

#### Scenario: Cerrar chat

- GIVEN el detalle de un grupo
- WHEN el admin confirma `POST /groups/:id/lock`
- THEN se muestra resultado SUCCESS y el grupo se refresca

#### Scenario: Sin permiso del bot

- GIVEN el bot sin `can_change_info` / permiso requerido
- WHEN se ejecuta lock
- THEN se muestra mensaje legible PERMISSION_DENIED

### Requirement: Acciones sobre mensajes

El panel MUST permitir delete y pin de un mensaje por `messageId`
provisto por el admin (la Bot API no lista mensajes; AGENTS §8). Delete
MUST pedir confirmación. Errores §18 legibles.

#### Scenario: Eliminar mensaje

- GIVEN un `messageId` válido
- WHEN el admin confirma delete
- THEN se ejecuta POST delete y se muestra resultado

#### Scenario: Pin sin permiso

- GIVEN el bot sin `can_pin_messages`
- WHEN se intenta pin
- THEN se muestra mensaje legible PERMISSION_DENIED

### Requirement: Vista de solicitudes de ingreso

`/groups/:id/requests` MUST listar solicitudes (GET /join-requests) con
usuario, fecha y estado, en estados loading / vacío / error. El admin
MUST poder aprobar o rechazar cada solicitud pendiente (POST
.../approve, .../reject), invalidando el refetch tras decidir. Errores
§18 legibles (incluido "la solicitud ya fue decidida" como
VALIDATION_ERROR).

#### Scenario: Aprobar solicitud pendiente

- GIVEN una solicitud `pending`
- WHEN el admin aprueba
- THEN se ejecuta POST approve y la solicitud deja de mostrarse como
  pendiente

#### Scenario: Solicitud ya decidida (concurrencia)

- GIVEN una solicitud que otro admin ya aprobó
- WHEN el admin intenta aprobar de nuevo
- THEN se muestra "la solicitud ya fue decidida"

### Requirement: Vista de logs

`/groups/:id/logs` MUST listar las entradas de auditoría del grupo (GET
/logs) ordenadas por `created_at` descendente, mostrando acción, status,
target, error si lo hay y fecha legible. Estados loading / vacío /
error.

#### Scenario: Logs de un grupo

- GIVEN 2 logs SUCCESS del grupo
- WHEN se abre `/groups/123/logs`
- THEN se renderizan del más reciente al más antiguo con acción y status

#### Scenario: Sin logs

- GIVEN un grupo sin acciones registradas
- WHEN se abre la página
- THEN se muestra estado vacío

### Requirement: Datos con TanStack Query

Todas las lecturas MUST usar hooks de TanStack Query
(`useGroupUsers`, `useJoinRequests`, `useGroupLogs`) y las mutaciones
hooks `useMutation` que invalidan `queryClient` de usuarios/logs/grupo
tras cada acción (patrón `features/groups` de la guía frontend §3-4).

#### Scenario: Mutación invalida consultas

- GIVEN una acción de moderación exitosa
- WHEN se completa la mutación
- THEN `invalidateQueries` refresca la lista afectada y los logs