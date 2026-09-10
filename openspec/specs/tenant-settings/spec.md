# Tenant Settings Specification

## Purpose

Endpoints de configuración del tenant: consulta de datos + estado del
bot, rotación de token con confirmación por password, y polling de
estado runtime. El token cifrado se actualiza en DB, se reinicia el
poller, y nunca se expone en logs ni respuestas.

## Requirements

### Requirement: Datos y estado del tenant

El sistema MUST exponer `GET /api/tenants/me` que retorne: `slug`,
`bot_username`, `bot_status` (valores: `connected`, `disconnected`,
`unknown`), y `created_at`. La respuesta MUST NOT incluir el token.

#### Scenario: Tenant autenticado consulta sus datos

- GIVEN un admin autenticado con tenant válido
- WHEN se llama `GET /api/tenants/me`
- THEN responde 200 con slug, bot_username, bot_status y created_at

#### Scenario: Tenant inexistente

- GIVEN un admin cuyo tenant ya no existe en DB
- WHEN se llama `GET /api/tenants/me`
- THEN responde 404 con código `NOT_FOUND`

### Requirement: Rotación de bot token

El sistema MUST exponer `PUT /api/tenants/me/bot-token` que acepte
`{password, bot_token}`. MUST validar la password del admin actual
(bcrypt). MUST validar el nuevo token con Telegram `getMe`. MUST
cifrar el token con AES-GCM y persistirlo. MUST reiniciar el poller
del bot con el nuevo token. La respuesta MUST NOT contener el token.
Los logs MUST NOT contener el token en claro.

#### Scenario: Rotación exitosa

- GIVEN un admin autenticado con password correcta y un bot_token válido
- WHEN se llama `PUT /api/tenants/me/bot-token` con password y bot_token
- THEN responde 200 con `status: "rotated"`
- AND el poller se reinicia con el nuevo token
- AND ni la respuesta ni los logs contienen el token

#### Scenario: Password incorrecta

- GIVEN un admin autenticado con password incorrecta
- WHEN se llama `PUT /api/tenants/me/bot-token`
- THEN responde 401 con código `UNAUTHORIZED`

#### Scenario: Token inválido en Telegram

- GIVEN un admin autenticado con password correcta pero un bot_token que Telegram rechaza en getMe
- WHEN se llama `PUT /api/tenants/me/bot-token`
- THEN responde 502 con código `TELEGRAM_ERROR`
- AND el token anterior permanece vigente

#### Scenario: Campos faltantes

- GIVEN un admin autenticado sin `password` o sin `bot_token`
- WHEN se llama `PUT /api/tenants/me/bot-token`
- THEN responde 400 con código `VALIDATION_ERROR`

### Requirement: Status runtime del bot

El sistema MUST exponer `GET /api/tenants/me/status` que retorne
`bot_status` en tiempo real (consultando el poller/adapter). Valores:
`connected`, `disconnected`, `unknown`. El endpoint MUST ser liviano
(no debe hacer llamadas pesadas a Telegram).

#### Scenario: Bot conectado

- GIVEN un tenant con bot activo y poller funcionando
- WHEN se llama `GET /api/tenants/me/status`
- THEN responde 200 con `bot_status: "connected"`

#### Scenario: Bot desconectado

- GIVEN un tenant con bot cuyo token fue revocado
- WHEN se llama `GET /api/tenants/me/status`
- THEN responde 200 con `bot_status: "disconnected"`

### Requirement: Token hygiene

El token del bot (cifrado o en claro) MUST NOT aparecer en: logs,
respuestas HTTP (incluyendo el PUT de rotación), mensajes de error, ni
headers. Solo el status (connected/disconnected) se expone.

#### Scenario: Inspección de respuesta de rotación

- GIVEN una rotación exitosa
- WHEN se inspecciona la respuesta HTTP del PUT
- THEN no contiene el valor del token, solo el status

#### Scenario: Inspección de logs

- GIVEN cualquier operación de tenant settings
- WHEN se revisan los logs del backend
- THEN no aparece el token en claro
