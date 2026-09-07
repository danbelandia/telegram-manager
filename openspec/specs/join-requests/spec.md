# Join Requests Specification

## Purpose

Gestión de solicitudes de ingreso a grupos (AGENTS.md §10). El backend
procesa los eventos `chat_join_request` de Telegram, los persiste como
pendientes y expone endpoints para que un admin apruebe o rechace, con
registro de la decisión.

## Requirements

### Requirement: Tabla de solicitudes

El sistema MUST crear la tabla `join_requests` con: `id` (PK, el
`requestId` de las rutas), `group_id` (telegram id del grupo),
`user_id` (telegram id del solicitante), `status` (`pending` |
`approved` | `rejected`), `requested_at`, `decided_at` (nullable),
`decided_by` (id del admin, nullable). Debe haber un índice por
`group_id` y por estado.

#### Scenario: Persistencia de solicitud

- GIVEN un evento `chat_join_request` recibido
- WHEN el backend lo procesa
- THEN existe una fila con status `pending`, user_id y group_id del
  evento

### Requirement: Procesamiento del evento

El bus de eventos MUST manejar `chat_join_request` creando una
solicitud pendiente (upsert si ya existe una pendiente del mismo
usuario en el mismo grupo). El evento llega del webhook o polling como
un `Update` con `ChatJoinRequest` (ya parseado por el adapter).

#### Scenario: Nuevo evento crea solicitud

- GIVEN un update con `chat_join_request` de un usuario nuevo
- WHEN el bus publica el update
- THEN se persiste la solicitud en estado `pending`

#### Scenario: Dup de una pendiente

- GIVEN una solicitud `pending` ya persistida de user X en grupo G
- WHEN llega otro evento `chat_join_request` de X en G
- THEN no se duplica (upsert) y sigue `pending`

### Requirement: Listar solicitudes

El sistema MUST exponer `GET /api/groups/:id/join-requests`
(autenticada) devolviendo las solicitudes del grupo con el estado y
fechas; el panel las muestra como `Pendiente` con acciones Aceptar/
Rechazar.

#### Scenario: Lista de pendientes

- GIVEN un grupo con 2 solicitudes pendientes
- WHEN se llama `GET /api/groups/:id/join-requests`
- THEN responde 200 con ambas solicitudes y sus estados

### Requirement: Aprobar/Rechazar

El sistema MUST exponer `POST /api/groups/:id/join-requests/:requestId/
approve` y `/reject` (autenticadas). Cada decisión MUST: (1) verificar
que la solicitud existe y pertenece al grupo, (2) verificar permiso del
bot (`can_invite_users` para `approve`; para `reject` aplicar cuando
corresponda), (3) llamar a `ApproveJoinRequest`/`RejectJoinRequest`
de Telegram, (4) actualizar status, `decided_at` y `decided_by`, y
(5) registrar log.

#### Scenario: Aprobación exitosa

- GIVEN solicitud `pending` y bot con permiso de invitación
- WHEN se llama `POST .../join-requests/7/approve`
- THEN responde 200, la solicitud pasa a `approved` con `decided_by` =
  id del admin, y se registra un log SUCCESS

#### Scenario: Rechazo exitoso

- GIVEN solicitud `pending`
- WHEN se llama `POST .../join-requests/7/reject`
- THEN responde 200 y la solicitud pasa a `rejected`

#### Scenario: Solicitud inexistente

- GIVEN un `requestId` que no existe
- WHEN se llama `POST .../join-requests/99/approve`
- THEN responde 404 con code `NOT_FOUND`

#### Scenario: Solicitud ya decidida

- GIVEN una solicitud con status `approved`
- WHEN se intenta aprobar de nuevo
- THEN responde 409 (o `VALIDATION_ERROR`) y no se vuelve a llamar a
  Telegram

#### Scenario: Telegram rechaza la decisión

- GIVEN la solicitud expiró y Telegram devuelve error al aprobar
- WHEN se llama approve
- THEN responde con code `TELEGRAM_ERROR`, la solicitud queda sin
  decidir y se registra un log de fallo