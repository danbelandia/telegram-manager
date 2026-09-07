# Telegram Moderation Specification

## Purpose

Capa de integración que expone las acciones administrativas de la Bot
API de Telegram (AGENTS.md §15) como métodos tipados de un Service,
ejecutadas con POST+JSON, protegidas por un rate limiter token bucket
(§18.1) y con errores de dominio mapeados desde los errores de la API.
Además de moderación, el adapter cubre los métodos de publicación a
grupos (`SendMessage`, `SendPhoto`) introducidos en slice 2 de
publications, respetando los mismos principios (token en path, rate
limiter, errores tipados).

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

---

## Slice 2 Additions (2026-09-07 — SendPhoto + InlineKeyboard + SendMessage signature)

Las siguientes requirements fueron agregadas por el slice 2 de
publications. El núcleo de la spec canónica (los 4 requirements de
moderación arriba) **no cambia**: las nuevas requirements extienden el
`Service` con tipos públicos serializables a la Bot API y con los
métodos de publicación que requieren el mismo transporte y rate limit
que la moderación.

### ADDED Requirements

### Requirement: Tipos InlineKeyboardMarkup e InlineKeyboardButton

El paquete `telegram` MUST exponer tipos públicos serializables al
JSON esperado por la Bot API:

```go
type InlineKeyboardButton struct {
    Text string `json:"text"`
    URL  string `json:"url"`
}
type InlineKeyboardMarkup struct {
    InlineKeyboard [][]InlineKeyboardButton `json:"inline_keyboard"`
}
```

El nombre del tag JSON del campo array MUST ser `inline_keyboard` y
los nombres de los campos del botón MUST ser `text` y `url`. La
serialización MUST ser validada por tests con un payload JSON exacto
que coincida con el formato de la Bot API.

#### Scenario: Serialización a JSON de la Bot API

- GIVEN un `InlineKeyboardMarkup` con dos filas (1 botón + 2 botones)
- WHEN se serializa con `json.Marshal`
- THEN el resultado contiene la clave
  `"inline_keyboard": [[{"text":"A","url":"https://..."}], [{"text":"B","url":"https://..."},{"text":"C","url":"https://..."}]]`

### Requirement: SendMessage en el adapter (con teclado opcional)

El `telegram.Service` MUST exponer
`SendMessage(ctx, chatID int64, text string, disableWebPagePreview bool, keyboard *InlineKeyboardMarkup) (int64, error)`
que invoque el método `sendMessage` de la Bot API. El parámetro
`keyboard` es opcional: si no es nil, se serializa como `reply_markup`
en el body. Si es nil, se omite del payload (`omitempty`). La llamada
MUST pasar por `doWithRetry` (429 con max 3 reintentos) y respetar el
rate limiter token bucket (§18.1). El retorno es el `message_id` de
Telegram.

#### Scenario: SendMessage con teclado

- GIVEN un `*InlineKeyboardMarkup` con una fila de un botón
- WHEN se llama `SendMessage(ctx, chatID, text, false, keyboard)`
- THEN el body del request contiene `reply_markup.inline_keyboard`
  con la fila serializada correctamente; el método devuelve el
  `message_id` sin error

#### Scenario: SendMessage sin teclado omite reply_markup

- GIVEN `keyboard = nil`
- WHEN se serializa el body del request
- THEN el campo `reply_markup` no está presente (omitempty)

#### Scenario: 429 con retry_after

- GIVEN un stub que responde 429 con `retry_after=2`
- WHEN se llama `SendMessage`
- THEN se espera 2s, se reintenta, y si el reintento es exitoso
  devuelve el `message_id`

### Requirement: Adapter SendPhoto

El `telegram.Service` MUST exponer
`SendPhoto(ctx, chatID int64, photoURL, caption string, keyboard *InlineKeyboardMarkup) (int64, error)`
que invoca el método `sendPhoto` de la Bot API. La foto se envía por
**URL pública** (Telegram la descarga; ≤ 5 MB). El caption MUST
aceptar hasta 1024 caracteres. El `keyboard` opcional: si no es nil,
se serializa como `reply_markup` (con `omitempty` se omite cuando nil).
La llamada MUST pasar por `doWithRetry` (429 con max 3 reintentos) y
respetar el token bucket. Devuelve el `message_id` de Telegram. En
error, el `error_message` propagado MUST ser legible (sin filtrar el
token del bot).

#### Scenario: SendPhoto exitoso

- GIVEN un adapter con token válido y stub que responde con
  `{ok:true, result:{message_id:42}}`
- WHEN se llama `SendPhoto(ctx, chatID, "https://x/y.jpg", "hola", nil)`
- THEN el método HTTP invocado es `sendPhoto`, los campos enviados son
  `chat_id`, `photo` (=URL), `caption`="hola"; retorna `(42, nil)`

#### Scenario: SendPhoto con teclado

- GIVEN un `*InlineKeyboardMarkup` no nil con una fila de dos botones
- WHEN se llama `SendPhoto(..., keyboard)`
- THEN el body contiene `reply_markup.inline_keyboard` con la
  estructura JSON exacta esperada por la Bot API