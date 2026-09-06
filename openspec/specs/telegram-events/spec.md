# telegram-events Specification

## Purpose
El backend ingesta updates de Telegram (modo polling o webhook según
`TELEGRAM_MODE`) y los despacha a través de un bus interno de eventos.
Los cambios de negocio (grupos, solicitudes de ingreso, moderación) se
suscriben a ese bus; este spec cubre solo el transporte y el despacho.

## Requirements

### Requirement: Transporte de updates en el adapter

The `Adapter` MUST expose `GetUpdates(ctx, offset, timeout, allowed_updates)`,
`SetWebhook(ctx, url, secret_token)` and `DeleteWebhook(ctx)` using the
official Bot API methods (`getUpdates`, `setWebhook`, `deleteWebhook`).
Request parameters MUST follow the curated reference
(`docs/telegram_api_reference.md` §1-2). The bot token MUST NOT appear in
logs or in any error message.

#### Scenario: Polling recibe un lote de updates

- GIVEN a call to `GetUpdates` with `offset` set to the last processed `update_id + 1`
- WHEN Telegram returns a non-empty `result` array
- THEN the updates are decoded into typed `Update` values with `update_id` and the MVP-relevant fields

#### Scenario: getUpdates rechazado por token inválido

- GIVEN the server response has `error_code` 401
- THEN the error maps to `ErrInvalidToken`

#### Scenario: Conflicto con webhook activo

- GIVEN the server response has `error_code` 409
- THEN the error is fatal and clearly states that a webhook is active and `deleteWebhook` is required

### Requirement: Poller long polling

In `polling` mode the backend MUST run a loop that calls `GetUpdates`
repeatedly with long polling timeout 30s. The poller MUST advance `offset`
to `max(update_id)+1` only after a successful batch, and MUST back off on
network errors without advancing `offset` (messages are re-delivered and
processing is idempotent). On `429`, the poller MUST wait
`retry_after` seconds and retry up to 3 times.

#### Scenario: Lote exitoso avanza el offset

- GIVEN a successful batch with `update_id` values 10 and 11
- WHEN the batch is published to the bus
- THEN the next poll uses `offset` 12

#### Scenario: Error de red no avanza el offset

- GIVEN `GetUpdates` fails with a network error
- WHEN the poller retries
- THEN the next call keeps the previous `offset`

#### Scenario: 429 respeta retry_after

- GIVEN Telegram returns 429 with `retry_after: 2`
- WHEN the poller handles the response
- THEN it waits at least 2 seconds before retrying, at most 3 times

### Requirement: Filtro de tipos de update

The poller and webhook setup MUST request only the MVP-relevant update
types via `allowed_updates`: `message`, `chat_member`, `my_chat_member`,
`chat_join_request` (explicit, because Telegram does not send
`chat_member`/`my_chat_member` by default).

#### Scenario: allowed_updates configurados

- GIVEN the backend starts in polling mode
- WHEN the first `getUpdates` request is built
- THEN `allowed_updates` contains exactly the four MVP types

### Requirement: Bus de eventos

The backend MUST provide an `internal/events` bus with `Publish(*Update)`
and a way to register handlers (`Handle(func(*Update))`). Every published
update MUST be delivered to all registered handlers. In this change the
backend MUST register a handler that logs each received update
(`update_id` and type) as an audit/diagnostic log entry.

#### Scenario: Update publicado es entregado a un handler

- GIVEN a handler registered on the bus
- WHEN `Publish` is called with an update
- THEN the handler is invoked with that update

#### Scenario: Despacho de eventos en webhook no bloquea la respuesta

- GIVEN a valid webhook request
- WHEN the handler publishes the update
- THEN the HTTP response returns 200 promptly without waiting for handlers to finish

### Requirement: Endpoint webhook con secreto

The backend MUST expose `POST /api/telegram/webhook`. When
`TELEGRAM_WEBHOOK_SECRET` is set, requests MUST be rejected with 401
unless header `X-Telegram-Bot-Api-Secret-Token` matches the secret, compared
in constant time (`subtle.ConstantTimeCompare`). A valid request is
decoded into an `Update`, published to the bus, and answered 200.

#### Scenario: Secret correcto

- GIVEN a request with the correct `X-Telegram-Bot-Api-Secret-Token`
- WHEN the webhook handler processes it
- THEN it decodes and publishes the update and responds 200

#### Scenario: Secret inválido o ausente

- GIVEN a request without the header, or with a wrong secret
- WHEN the webhook handler processes it
- THEN it responds 401 and does not publish anything

### Requirement: Modo webhook exige URL

The backend MUST fail fast at startup when `TELEGRAM_MODE=webhook` and
`TELEGRAM_WEBHOOK_URL` is missing.

#### Scenario: Webhook sin URL

- GIVEN `TELEGRAM_MODE=webhook` and no `TELEGRAM_WEBHOOK_URL`
- WHEN `config.Load` runs
- THEN it returns an error

### Requirement: En modo webhook el backend registra el webhook

In `webhook` mode, at startup the backend MUST call `setWebhook` with the
configured URL, secret token and `allowed_updates`; failure to register
MUST stop startup with a clear error.

#### Scenario: setWebhook exitoso

- GIVEN `TELEGRAM_MODE=webhook` with a valid URL and secret
- WHEN the backend starts
- THEN `setWebhook` is called with `allowed_updates` and startup continues

#### Scenario: setWebhook falla

- GIVEN `TELEGRAM_MODE=webhook` and Telegram rejects `setWebhook`
- WHEN the backend starts
- THEN startup fails with a clear error

### Requirement: Múltiples consumidores futuros

The bus MUST support more than one handler so later changes (groups,
join requests) subscribe without touching the transport.

#### Scenario: Dos handlers registrados

- GIVEN two handlers registered on the bus
- WHEN `Publish` is called once
- THEN both handlers are invoked exactly once