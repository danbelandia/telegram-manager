# Delta for telegram-moderation — Slice 2 (SendPhoto + InlineKeyboard types + SendMessage signature)

> Change: `publications` (Slice 2 de 3). Modifica la spec canónica
> `openspec/specs/telegram-moderation/spec.md` para extender el
> `telegram.Service` con `SendPhoto` y los tipos públicos
> `InlineKeyboardMarkup`/`InlineKeyboardButton`, y modificar la firma
> de `SendMessage` para aceptar un `keyboard *InlineKeyboardMarkup`
> opcional. El núcleo de la spec (POST+JSON, token en path, errores
> de dominio, rate limiter token bucket) **no cambia**.

## ADDED Requirements

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

## MODIFIED Requirements

### Requirement: Métodos de moderación y publicación

El `telegram.Service` MUST exponer los métodos: `BanUser`, `UnbanUser`,
`MuteUser`, `UnmuteUser`, `DeleteMessage`, `PinMessage`, `LockGroup`,
`UnlockGroup`, `ApproveJoinRequest`, `RejectJoinRequest`,
`GetChatMember`, `GetChatAdministrators`, **`SendMessage`,
`SendPhoto`**, todos con `ctx` como primer parámetro. Los métodos
MUST invocar el método correspondiente de la Bot API. `SendPhoto`
invoca `sendPhoto` con `photo` por URL pública, `caption`, y
`reply_markup` opcional. `SendMessage` MUST aceptar un parámetro
adicional `keyboard *InlineKeyboardMarkup` que, cuando no es nil, se
serializa como `reply_markup` en el body. El paquete MUST exponer
los tipos `InlineKeyboardMarkup` e `InlineKeyboardButton`.

(Previously: el `telegram.Service` exponía los métodos de moderación
de la Bot API y `SendMessage` (firmado como
`SendMessage(ctx, chatID, text, disableWebPagePreview)`). No existía
`SendPhoto` ni los tipos de inline keyboard, y `SendMessage` no
aceptaba `reply_markup`.)

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

#### Scenario: SendPhoto exitoso

- GIVEN un adapter con token válido y stub que responde con
  `{ok:true, result:{message_id:42}}`
- WHEN se llama
  `SendPhoto(ctx, chatID, "https://x/y.jpg", "hola", nil)`
- THEN el método HTTP invocado es `sendPhoto`, los campos enviados
  son `chat_id`, `photo` (=URL), `caption`="hola"; retorna `(42, nil)`

#### Scenario: SendPhoto con teclado

- GIVEN un `*InlineKeyboardMarkup` no nil con una fila de dos botones
- WHEN se llama `SendPhoto(..., keyboard)`
- THEN el body contiene `reply_markup.inline_keyboard` con la
  estructura JSON exacta esperada por la Bot API

#### Scenario: Ban de usuario (sin cambios respecto a slice base)

- GIVEN un adapter conectado con token válido
- WHEN se llama `BanUser(ctx, chatID, userID, untilDate, revokeMessages)`
- THEN se invoca `banChatMember` con esos parámetros y no devuelve
  error

#### Scenario: Mute con restricción completa (sin cambios)

- GIVEN un adapter conectado
- WHEN se llama `MuteUser(ctx, chatID, userID, untilDate)`
- THEN se invoca `restrictChatMember` con `can_send_messages=false`
  y capacidades de envío deshabilitadas