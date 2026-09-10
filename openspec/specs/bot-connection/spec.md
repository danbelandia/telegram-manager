# bot-connection Specification

## Purpose

Conectar el bot de Telegram de forma verificable: validar que el token configurado funciona al arrancar y exponer el estado de esa conexión vía un endpoint de salud, sin revelar nunca el token.

## Requirements

### Requirement: Validación del token al arrancar

The backend MUST validate `TELEGRAM_BOT_TOKEN` using the official Telegram Bot API method `getMe` (source: https://core.telegram.org/bots/api#getme). If the token is invalid, the backend MUST fail fast at startup with a non-zero exit code and a message that does NOT include the token.

#### Scenario: Token válido

- GIVEN a valid bot token in `.env`
- WHEN the backend starts
- THEN `getMe` succeeds
- AND the backend continues startup normally

#### Scenario: Token inválido

- GIVEN an invalid bot token in `.env`
- WHEN the backend starts
- THEN startup fails with a non-zero exit code
- AND the token is not present in the error message or logs

### Requirement: Endpoint de salud

The backend MUST expose `GET /api/health` returning a JSON envelope `{"data": {...}}` with: `status` (`"ok"` when the server responds), `db` (`"ok"` when PostgreSQL ping succeeds, `"error"` otherwise), and `bot_connected` (boolean reflecting the last `getMe` result) plus `bot_username` when known. The endpoint MUST NOT include the bot token or any secret in the response.

#### Scenario: Stack saludable

- GIVEN a running backend with valid token and healthy PostgreSQL
- WHEN requesting `GET /api/health`
- THEN the response contains `"status": "ok"`, `"db": "ok"`, and `"bot_connected": true`
- AND `bot_username` matches the bot identity from `getMe`

#### Scenario: Base de datos caída

- GIVEN PostgreSQL is unreachable
- WHEN requesting `GET /api/health`
- THEN the response contains `"db": "error"`
- AND `status` is not `"ok"`

#### Scenario: Respuesta sin secretos

- GIVEN any server state
- WHEN requesting `GET /api/health`
- THEN the response payload does not contain the bot token, `DATABASE_URL`, or any password

### Requirement: Token nunca expuesto

The bot token MUST NOT appear in HTTP responses, startup logs, or structured logs under any log level. No log line MAY include the value of `TELEGRAM_BOT_TOKEN`.

#### Scenario: Inspección de logs

- GIVEN the backend ran with a valid configuration
- WHEN the logs are inspected
- THEN no occurrence of the bot token value is found

### Requirement: Adapter de Telegram aislado

Telegram API calls MUST be made only through an adapter in `internal/telegram` exposing a service interface; no HTTP handler or service outside that package MAY call the Bot API directly. The adapter MUST be structured so a rate limiter can be added later without changing its consumers.

#### Scenario: Llamada desde otro paquete

- GIVEN a service that needs to check bot connection
- WHEN it calls the Telegram service interface
- THEN it never constructs a Bot API URL or token directly

---

## Slice 2 — tenant-settings (2026-09-10)

### ADDED Requirements

#### Requirement: Polling de estado runtime

El sistema MUST soportar un polling periódico del estado del bot
(contra Telegram `getMe`) con intervalo configurable (default 60s).
El estado resultante MUST alimentar el campo `bot_status` consultable
vía `GET /api/tenants/me/status` y vía `GET /api/tenants/me`. El
poller MUST reiniciarse cuando el token se rota.

##### Scenario: Polling exitoso

- GIVEN un bot con token válido y polling habilitado
- WHEN transcurre el intervalo de 60s
- THEN se ejecuta getMe y `bot_status` se actualiza a `connected`

##### Scenario: Polling fallido

- GIVEN un bot con token revocado
- WHEN se ejecuta getMe durante el polling
- THEN `bot_status` se actualiza a `disconnected`

##### Scenario: Reinicio tras rotación de token

- GIVEN un poller con token A activo
- WHEN el admin rota el token a B vía `PUT /api/tenants/me/bot-token`
- THEN el poller se reinicia inmediatamente con token B
- AND el primer getMe con B determina el nuevo `bot_status`