# Group Administration Specification

## Purpose

API REST de administración de grupos (AGENTS.md §12) que el panel
consume: listar/ver grupos, ver usuarios que Telegram expone, y ejecutar
acciones de moderación (ban/unban/mute/unmute, delete/pin, lock/unlock)
con autorización por grupo y manejo de errores con códigos §18.

## Requirements

### Requirement: Listar y ver grupos

El sistema MUST exponer `GET /api/groups` (todos los grupos conocidos,
ordenados por título) y `GET /api/groups/:id` (detalle de un grupo por
su telegram id). `:id` en todas las rutas del cambio es el telegram id
del grupo. Ambas rutas MUST requerir autenticación (access token
válido).

#### Scenario: Listar grupos

- GIVEN un admin autenticado y 2 grupos persistidos
- WHEN se llama `GET /api/groups`
- THEN responde 200 con `data` = arreglo de grupos, sin el token del
  bot

#### Scenario: Grupo inexistente

- GIVEN un admin autenticado
- WHEN se llama `GET /api/groups/-100999`
- THEN responde 404 con error code `NOT_FOUND`

#### Scenario: Sin autenticación

- GIVEN una petición sin access token
- WHEN se llama `GET /api/groups`
- THEN responde 401 con error code `UNAUTHORIZED`

### Requirement: Ver usuarios del grupo

`GET /api/groups/:id/users` MUST devolver lo que Telegram expone: los
administradores del grupo (`getChatAdministrators`) y, si se pasa
`?userId=`, la información de ese usuario puntual (`getChatMember`).
La limitación de no poder listar todos los miembros MUST documentarse en
el spec (AGENTS §7: la Bot API no expone la lista completa).

#### Scenario: Listar administradores

- GIVEN un grupo donde el bot es admin
- WHEN se llama `GET /api/groups/:id/users`
- THEN responde 200 con los administradores del grupo

#### Scenario: Lookup puntual

- GIVEN un grupo y un `userId` existente
- WHEN se llama `GET /api/groups/:id/users?userId=42`
- THEN responde 200 con la información de ese usuario

### Requirement: Acciones de moderación

El sistema MUST exponer rutas POST autenticadas para ban, unban, mute,
unmute (`POST /api/groups/:id/users/:userId/{ban,unban,mute,unmute}`),
delete y pin (`POST /api/groups/:id/messages/:messageId/{delete,pin}`)
y lock/unlock del chat (`POST /api/groups/:id/{lock,unlock}`). Cada
acción MUST: (1) verificar que el grupo existe, (2) verificar que el bot
tiene el permiso requerido (`bot_permissions` persistido), (3) ejecutar
la llamada a Telegram, y (4) registrar el log (spec admin-logs).

#### Scenario: Ban exitoso

- GIVEN grupo existente y bot con `can_restrict_members=true`
- WHEN se llama `POST /api/groups/:id/users/42/ban`
- THEN responde 200, se ejecuta `BanUser` en Telegram y se registra un
  log `BAN_USER` SUCCESS

#### Scenario: Bot sin permiso de restringir

- GIVEN grupo existente con `can_restrict_members=false`
- WHEN se llama `POST /api/groups/:id/users/42/ban`
- THEN responde 403 con code `PERMISSION_DENIED` SIN llamar a Telegram,
  y se registra un log `PERMISSION_DENIED`

#### Scenario: Pin sin permiso

- GIVEN grupo donde `can_pin_messages=false`
- WHEN se llama `POST /api/groups/:id/messages/5/pin`
- THEN responde 403 `PERMISSION_DENIED` sin llamar a Telegram

#### Scenario: Usuario no encontrado por Telegram

- GIVEN grupo válido y userId inexistente
- WHEN se llama `POST /api/groups/:id/users/999/ban`
- THEN responde 404 con code `NOT_FOUND` y log `NOT_FOUND`

### Requirement: Códigos de error del envelope

Los handlers MUST responder con el envelope `respond()`/`respondError()`
del proyecto y códigos de error §18: `VALIDATION_ERROR` (parámetros
inválidos), `PERMISSION_DENIED` (sin permiso del bot o del admin),
`NOT_FOUND` (grupo/target no existe), `TELEGRAM_ERROR` (la Bot API
falla), `INTERNAL_ERROR` (fallo inesperado). Las respuestas MUST NUNCA
exponer el token del bot ni detalles internos.

#### Scenario: Error de Telegram

- GIVEN la Bot API devuelve un error no tipado
- WHEN se ejecuta una acción de moderación
- THEN responde con code `TELEGRAM_ERROR` y mensaje comprensible sin
  internals

#### Scenario: Parámetros inválidos

- GIVEN `:id` o `:userId` no numéricos
- WHEN se llama una ruta de moderación
- THEN responde 400 con code `VALIDATION_ERROR`