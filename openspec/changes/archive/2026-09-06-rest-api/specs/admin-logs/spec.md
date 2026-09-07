# Admin Logs Specification

## Purpose

Registro de auditoría de acciones administrativas (AGENTS.md §11). Cada
acción importante sobre un grupo genera una fila en `logs` con actor,
grupo, acción, target, resultado y error. Los logs alimentan la vista
`/groups/:id/logs` del panel.

## Requirements

### Requirement: Modelo de datos

El sistema MUST crear la tabla `logs` con columnas: `id` (PK),
`actor_id` (admin que ejecutó), `group_id` (telegram id del grupo),
`action` (ej. `BAN_USER`), `target_user_id` (usuario afectado, nullable),
`metadata` (JSONB, nullable), `status` (`SUCCESS` | `PERMISSION_DENIED` |
`TELEGRAM_ERROR` | `VALIDATION_ERROR` | `NOT_FOUND` | `INTERNAL_ERROR`),
`error_message` (nullable), `created_at`. Debe existir un índice por
`group_id` para listar los logs de un grupo.

#### Scenario: Creación de log

- GIVEN un grupo y un admin autenticado
- WHEN se inserta un log de acción
- THEN la fila persiste con actor_id, group_id, action, status y
  created_at correctos

### Requirement: Repositorio de logs

El sistema MUST proveer un repositorio con operaciones de crear un log
y listar logs por grupo (ordenados por `created_at` descendente).

#### Scenario: Listar logs de un grupo

- GIVEN varios logs del grupo 1 y uno del grupo 2
- WHEN se listan los logs del grupo 1
- THEN se devuelven solo los del grupo 1, del más reciente al más
  antiguo

#### Scenario: Logs sin datos sensibles

- GIVEN un log con metadata
- THEN la respuesta de la API nunca contiene el token del bot,
  passwords ni JWT

### Requirement: Registro de acciones

Toda acción administrativa ejecutada por el panel (ban, unban, mute,
unmute, delete, pin, lock, unlock, approve, reject) MUST generar un log.
En éxito el `status` es `SUCCESS`; si Telegram rechaza por permisos,
`PERMISSION_DENIED`; si falla la llamada, `TELEGRAM_ERROR`; si el target
no existe, `NOT_FOUND`.

#### Scenario: Log de éxito

- GIVEN un ban exitoso
- THEN existe un log con action `BAN_USER`, status `SUCCESS` y
  target_user_id del usuario baneado

#### Scenario: Log de fallo por permisos

- GIVEN el bot sin permisos para la acción
- WHEN la acción devuelve `ErrPermissionDenied`
- THEN existe un log con status `PERMISSION_DENIED` y el error
  descrito en `error_message`

### Requirement: Actor de auditoría

El `actor_id` del log MUST ser el id del admin autenticado (claims del
access token) que ejecutó la acción. Si la acción la genera el sistema
(evento de Telegram, sin admin), `actor_id` MUST ser NULL.

#### Scenario: Log con actor admin

- GIVEN un admin autenticado ejecutando un ban
- THEN el log tiene `actor_id` = id del admin (de los claims)

#### Scenario: Log de evento sin actor

- GIVEN se registra una solicitud de ingreso por evento de Telegram
- THEN el log (si aplica) tiene `actor_id` NULL