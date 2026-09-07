# Telegram Moderation Specification

## Purpose

Capa de integración que expone las acciones administrativas de la Bot
API de Telegram (AGENTS.md §15) como métodos tipados de un Service,
ejecutadas con POST+JSON, protegidas por un rate limiter token bucket
(§18.1) y con errores de dominio mapeados desde los errores de la API.

## Requirements

### Requirement: Métodos de moderación

El `telegram.Service` MUST exponer los métodos: `BanUser`, `UnbanUser`,
`MuteUser`, `UnmuteUser`, `DeleteMessage`, `PinMessage`, `LockGroup`,
`UnlockGroup`, `ApproveJoinRequest`, `RejectJoinRequest`,
`GetChatMember`, `GetChatAdministrators`, todos con `ctx` como primer
parámetro. Los métodos MUST invocar el método correspondiente de la Bot
API (`banChatMember`, `unbanChatMember`, `restrictChatMember`,
`deleteMessage`, `pinChatMessage`, `setChatPermissions`,
`approveChatJoinRequest`, `declineChatJoinRequest`, `getChatMember`,
`getChatAdministrators`).

#### Scenario: Ban de usuario

- GIVEN un adapter conectado con token válido
- WHEN se llama `BanUser(ctx, chatID, userID, untilDate, revokeMessages)`
- THEN se invoca `banChatMember` con esos parámetros y no devuelve error

#### Scenario: Mute conrestricción completa

- GIVEN un adapter conectado
- WHEN se llama `MuteUser(ctx, chatID, userID, untilDate)`
- THEN se invoca `restrictChatMember` con `can_send_messages=false` y
  capacidades de envío deshabilitadas

#### Scenario: Lock/Unlock del grupo

- GIVEN un adapter conectado
- WHEN se llama `LockGroup` / `UnlockGroup(ctx, chatID)`
- THEN se invoca `setChatPermissions` con `can_send_messages=false`
  (lock) o `true` con capacidades por defecto (unlock)

### Requirement: Transporte POST con JSON

El adapter MUST enviar los métodos de moderación con HTTP POST y body
JSON (los parámetros anidados como `ChatPermissions` no son seguros en
query string). El token del bot MUST ir en el path
(`/bot<TOKEN>/<método>`), nunca en el body, ni en query, ni en logs.

#### Scenario: Payload JSON correcto

- GIVEN un stub HTTP que registra requests
- WHEN se ejecuta un método de moderación
- THEN el request es POST con Content-Type application/json y body con
  los parámetros esperados

#### Scenario: Token nunca expuesto

- GIVEN cualquier ejecución de moderación
- THEN la URL del request contiene el token en el path y ningún log o
  error lo imprime

### Requirement: Errores de dominio tipados

El adapter MUST traducir los errores de la Bot API a errores de
dominio: 403/400 de permisos → `ErrPermissionDenied`; 404 →
`ErrTelegramNotFound`; otros códigos (excepto 401/409/429 que ya
existen) → `ErrTelegramAPI` con código y descripción. Los servicios de
negocio MUST recibir estos errores tipados, nunca strings de la API.

#### Scenario: Bot sin permisos

- GIVEN el bot no es administrador del grupo
- WHEN se ejecuta `BanUser`
- THEN devuelve `ErrPermissionDenied`

#### Scenario: Chat inexistente

- GIVEN un chat_id inválido
- WHEN se ejecuta cualquier método de moderación
- THEN devuelve `ErrTelegramNotFound`

### Requirement: Rate limiter token bucket

El adapter MUST limitar sus requests a ~25 req/seg globales con un
token bucket interno (mutex, sin infraestructura externa). Ante un 429
con `retry_after`, MUST esperar ese tiempo y reintentar, con máximo 3
reintentos; agotados, devuelve `RateLimitError`. Las acciones en lote
MUST ejecutarse secuencialmente (canal interno + worker), nunca en
paralelo sin control.

#### Scenario: 429 con retry_after

- GIVEN un stub que responde 429 con `retry_after=2`
- WHEN se ejecuta un método de moderación
- THEN se espera 2s y se reintenta; si el reintento es exitoso,
  devuelve nil

#### Scenario: Agotamiento de reintentos

- GIVEN un stub con 429 persistente
- WHEN se ejecuta un método de moderación
- THEN tras 3 reintentos devuelve `RateLimitError` sin seguir
  reintentando a ciegas