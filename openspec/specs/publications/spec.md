# Publications Specification

## Purpose

Publicaciones inmediatas a uno o varios grupos de Telegram (AGENTS §22).
El admin crea una publicación (texto, foto por URL opcional, botones
inline de URL opcionales) y el bot la envía sincrónicamente al/los
grupo(s) seleccionado(s), registrando la acción en logs. La tabla y el
módulo se diseñan para extenderse en slices posteriores (scheduling,
medios adicionales, automatizaciones).

> **Histórico de slices**: Slice 1 cubrió el publish-now de texto a un
> grupo (`ace1f59`). Slice 2 (archivada en
> `openspec/changes/archive/2026-09-07-publications-slice2/`,
> mergeada en este archivo) extiende a foto por URL, botones inline,
> envío multi-grupo secuencial y filtro por grupo en el historial,
> respetando el bugfix #172 (`permissionOk` usa `bot_status ==
> administrator`, NO reintroducir checks sobre claves `can_*`).

## Requirements

### Requirement: Tabla publications

El sistema MUST crear la tabla `publications` con columnas: `id`
(SERIAL PK), `telegram_id` (int64, FK a `groups.telegram_id`), `text`
(text, non-empty, ≤4096), `status` (enum `draft | scheduled | sending |
sent | failed`), `message_id` (int64 nullable), `scheduled_at`
(timestamptz nullable), `error_message` (text nullable), `actor_id`
(bigint nullable), `created_at`, `updated_at`. Debe existir un índice
por `telegram_id` y otro por `created_at` DESC.

#### Scenario: Creación de publicación

- GIVEN un admin autenticado que envía `POST /api/publications`
- WHEN el envío a Telegram es exitoso
- THEN persiste una fila con status `sent`, `message_id` no nulo y
  `error_message` NULL

#### Scenario: Envío fallido

- GIVEN un envío que Telegram rechaza
- THEN la fila queda con status `failed`, `message_id` NULL y
  `error_message` con la descripción del error

### Requirement: SendMessage en el adapter

El `telegram.Service` MUST exponer `SendMessage(ctx, chatID int64,
text string, disableWebPagePreview bool) (int64, error)` que invoque
el método `sendMessage` de la Bot API. Debe pasar por `doWithRetry`
(429 con max 3 reintentos) y respetar el rate limiter token bucket
(§18.1). El retorno es el `message_id` de Telegram.

#### Scenario: Envío exitoso

- GIVEN un adapter con token válido y rate limiter disponible
- WHEN se llama `SendMessage(ctx, chatID, text, false)`
- THEN se invoca `sendMessage` con esos parámetros y devuelve el
  `message_id` sin error

#### Scenario: 429 con retry_after

- GIVEN un stub que responde 429 con `retry_after=2`
- WHEN se llama `SendMessage`
- THEN se espera 2s, se reintenta, y si el reintento es exitoso
  devuelve el `message_id`

### Requirement: POST /api/publications (crear + enviar)

El endpoint `POST /api/publications` MUST aceptar `{text, group_id}`
donde `group_id` es `groups.telegram_id`. La autenticación es
requerida (`requireAuth`). El servicio MUST:

1. Validar `text` non-empty y ≤ 4096 chars (400 VALIDATION_ERROR).
2. Verificar que `group_id` exista en la tabla groups (404
   NOT_FOUND si no existe).
3. Verificar que el bot sea administrador del grupo
   (`groups.bot_status == administrator`; 403 PERMISSION_DENIED si no
   lo es). Enmienda 2026-09-07: el original exigía
   `bot_permissions["can_manage_chat"]`, pero la detección de grupos
   nunca puebla esa clave (events.go copia solo
   can_delete/restrict/pin/invite/promote/change_info), lo que
   producía 403 para todos los grupos. La Bot API no exige permisos
   `can_*` para sendMessage en grupos; ser administrador es el estado
   confiable y además exime de restricciones de envío del grupo.
4. Insertar fila con status `sending`.
5. Llamar `telegram.SendMessage`.
6. En éxito: actualizar status a `sent`, guardar `message_id`.
7. En fallo: actualizar status a `failed`, guardar `error_message`.
8. Registrar log con `ActionPublishMessage` en ambos casos.
9. Devolver 201 con la fila de publicación.

#### Scenario: Publicación exitosa

- GIVEN un admin autenticado, texto válido y un grupo donde el bot es
  administrador (`bot_status = administrator`)
- WHEN envía `POST /api/publications` con `{text, group_id}`
- THEN responde 201 con la publicación (status `sent`, message_id
  no nulo), y existe un log `PUBLISH_MESSAGE` con status `SUCCESS`

#### Scenario: Texto vacío

- GIVEN `text = ""`
- WHEN se ejecuta POST
- THEN responde 400 VALIDATION_ERROR "texto no puede estar vacio"

#### Scenario: Texto excede 4096 chars

- GIVEN `text` de 4097 caracteres
- WHEN se ejecuta POST
- THEN responde 400 VALIDATION_ERROR "texto excede 4096 caracteres"

#### Scenario: Grupo no encontrado

- GIVEN un `group_id` que no existe en la tabla groups
- WHEN se ejecuta POST
- THEN responde 404 NOT_FOUND "grupo no encontrado"

#### Scenario: Bot sin permisos

- GIVEN el grupo existe pero el bot NO es administrador
  (`bot_status = member`)
- WHEN se ejecuta POST
- THEN responde 403 PERMISSION_DENIED "el bot no tiene permisos
  suficientes en este grupo"

#### Scenario: Telegram rechaza el envío

- GIVEN el adapter devuelve un error de Telegram
- WHEN se ejecuta POST
- THEN la publicación queda con status `failed`, se registra log con
  `TELEGRAM_ERROR`, y se responde con el error mapeado (502
  TELEGRAM_ERROR o 403 PERMISSION_DENIED según el tipo)

### Requirement: GET /api/publications (listado)

El endpoint `GET /api/publications` MUST devolver publicaciones
ordenadas por `created_at` DESC, con un límite de 50 registros. No
se requiere paginación en Slice 1. Cada registro incluye: `id`,
`telegram_id`, `text`, `status`, `message_id`, `error_message`,
`actor_id`, `created_at`. La autenticación es requerida.

#### Scenario: Listado con datos

- GIVEN 3 publicaciones existentes
- WHEN se ejecuta GET
- THEN responde 200 con las 3 publicadas ordenadas por fecha DESC

#### Scenario: Listado vacío

- GIVEN no hay publicaciones
- WHEN se ejecuta GET
- THEN responde 200 con array vacío

### Requirement: GET /api/publications/:id

El endpoint `GET /api/publications/:id` MUST devolver la publicación
completa por su ID interno. 404 NOT_FOUND si no existe.

#### Scenario: Publicación encontrada

- GIVEN una publicación con id=1
- WHEN se ejecuta GET /api/publications/1
- THEN responde 200 con la publicación completa

#### Scenario: Publicación inexistente

- GIVEN no existe publicación con id=999
- WHEN se ejecuta GET /api/publications/999
- THEN responde 404 NOT_FOUND

### Requirement: Frontend PublicationsPage

La ruta `/publications` MUST renderizar `PublicationsPage` envuelta
en `RequireAuth`. La página MUST mostrar:

- Lista de publicaciones con estados: loading, vacío, error.
- Formulario de creación: textarea para `text` + selector de grupo
  (options desde `GET /api/groups`).
- Al enviar: `POST /api/publications`, invalidar query de lista, mostrar
  errores legibles (§18).
- Cada publicación muestra: texto truncado, grupo, status (badge),
  fecha.

#### Scenario: Página carga con datos

- GIVEN 2 publicaciones existentes
- WHEN se navega a `/publications`
- THEN se renderizan ambas con status y fecha

#### Scenario: Página sin datos

- GIVEN no hay publicaciones
- WHEN se navega a `/publications`
- THEN se muestra estado vacío con mensaje "No hay publicaciones"

#### Scenario: Error de carga

- GIVEN el backend responde 500
- WHEN se carga la página
- THEN se muestra error con opción de reintentar

#### Scenario: Crear publicación exitosa

- GIVEN un texto válido y un grupo seleccionado
- WHEN se envía el formulario
- THEN se ejecuta POST, se invalida la lista, la nueva publicación
  aparece en el listado

#### Scenario: Crear publicación con error

- GIVEN un grupo donde el bot no es admin
- WHEN se envía el formulario
- THEN se muestra el mensaje legible del error (PERMISSION_DENIED)

### Requirement: Tests backend

Los tests MUST cubrir:

- **Servicio**: mock de `TelegramService` (sendMessage OK, sendMessage
  error, grupo no encontrado, permiso denegado). Verificar status de
  la publicación y escritura de log en cada caso.
- **Handler**: requests HTTP contra los 3 endpoints con envelope
  correcto (data/error), códigos de status.
- **Repositorio**: CRUD de la tabla publications contra PostgreSQL real
  o mock.

#### Scenario: Servicio con Telegram mockeado

- GIVEN un servicio con TelegramService mock que retorna message_id
- WHEN se ejecuta Publish con datos válidos
- THEN la publicación queda con status `sent` y message_id guardado

#### Scenario: Servicio con Telegram error

- GIVEN un servicio con TelegramService mock que retorna error
- WHEN se ejecuta Publish
- THEN la publicación queda con status `failed`, error_message
  poblado, y se registra log con TELEGRAM_ERROR

### Requirement: Tests frontend

Los tests de `PublicationsPage` MUST usar `mockFetchRoutes` para
simular las respuestas de la API. Deben cubrir: carga exitosa, lista
vacía, error de carga, creación exitosa con invalidación, y error al
crear con mensaje visible.

#### Scenario: Test de carga

- GIVEN mockFetchRoutes con respuesta de lista
- WHEN se renderiza PublicationsPage
- THEN aparecen las publicaciones renderizadas

#### Scenario: Test de error

- GIVEN mockFetchRoutes con respuesta 500
- WHEN se renderiza PublicationsPage
- THEN se muestra el estado de error

### Requirement: Registro de auditoría

Cada intento de publicación (éxito o fallo) MUST generar un log con:
`ActionPublishMessage`, `actor_id` del admin, `group_id` (telegram
id), `metadata` con `publication_id` y `message_id` (si éxito),
`status` (SUCCESS o el código correspondiente), y `error_message` si
falló.

#### Scenario: Log de publicación exitosa

- GIVEN un envío exitoso con message_id=123
- THEN existe un log con action `PUBLISH_MESSAGE`, metadata
  `{"publication_id": 1, "message_id": 123}`, status `SUCCESS`

#### Scenario: Log de publicación fallida

- GIVEN un envío que falla
- THEN existe un log con action `PUBLISH_MESSAGE`, status
  `TELEGRAM_ERROR` o `PERMISSION_DENIED`, y error_message no nulo

---

## Slice 2 Extensions (2026-09-07 — Foto URL + Botones + Multi-Grupo + Filtro)

Las siguientes requirements fueron agregadas o modificadas por el
slice 2. Las requirements de slice 1 arriba se conservan tal cual;
cuando una requirement de slice 1 fue **modificada** por slice 2, la
versión vigente aparece aquí reemplazando a la anterior (la versión
anterior permanece en el archivo de slice 1 archivado).

### ADDED Requirements

### Requirement: Validaciones de payload (publicación multi-grupo)

El servicio `publications` MUST validar **una sola vez** al inicio del
`PublishMany` (fail-fast, antes de iterar grupos):

- `text`: no vacío. Si hay `photo_url`, ≤ 1024 caracteres (límite
  caption de `sendPhoto`); si no, ≤ 4096 caracteres. Excedido →
  `VALIDATION_ERROR` (HTTP 400) con mensaje legible.
- `photo_url`: opcional. Si presente, MUST ser URL con esquema `http`
  o `https` y longitud ≤ 2048 caracteres. URL inválida o esquema no
  soportado → `VALIDATION_ERROR` (400). No se valida accesibilidad ni
  tamaño real (Telegram lo rechaza y la fila queda `failed` con
  `error_message` legible).
- `buttons`: opcional. Si presente, MUST ser array de filas donde cada
  fila es un array de `{text, url}`. Límites MUST: máximo 8 filas,
  máximo 8 botones por fila (64 botones totales), `text` no vacío y
  ≤ 64 caracteres, `url` no vacía, http(s), ≤ 256 caracteres. Si
  `buttons = []` o solo contiene filas vacías, se trata como ausente
  (sin teclado). Botones inválidos → `VALIDATION_ERROR` (400).
- `group_ids`: array no vacío, longitud entre 1 y 10. Vacío → 400
  "se requiere al menos un grupo". Exceder 10 → 400 "máximo 10
  grupos por publicación".

Las validaciones anteriores corren **una sola vez** al recibir el
payload; los errores per-grupo (grupo inexistente, bot no admin,
Telegram rechaza) NO abortan el resto: se registran como status
`failed` en la fila correspondiente.

#### Scenario: Texto excede caption con foto

- GIVEN `text` de 1100 caracteres y `photo_url` presente
- WHEN se ejecuta `POST /api/publications`
- THEN responde 400 `VALIDATION_ERROR` "el texto excede 1024 caracteres cuando se envía con foto"

#### Scenario: photo_url no es http/https

- GIVEN `photo_url = "ftp://example.com/a.jpg"`
- WHEN se ejecuta POST
- THEN responde 400 `VALIDATION_ERROR` "la URL de la foto debe empezar con http o https"

#### Scenario: photo_url excede 2048 caracteres

- GIVEN una `photo_url` válida en esquema pero con 2049 caracteres
- WHEN se ejecuta POST
- THEN responde 400 `VALIDATION_ERROR` "la URL de la foto excede 2048 caracteres"

#### Scenario: Botones exceden 8 filas

- GIVEN `buttons` con 9 filas
- WHEN se ejecuta POST
- THEN responde 400 `VALIDATION_ERROR` "máximo 8 filas de botones"

#### Scenario: Botón con url no http(s)

- GIVEN un botón con `url = "javascript:alert(1)"`
- WHEN se ejecuta POST
- THEN responde 400 `VALIDATION_ERROR` "la URL del botón debe empezar con http o https"

#### Scenario: group_ids vacío

- GIVEN `group_ids = []`
- WHEN se ejecuta POST
- THEN responde 400 `VALIDATION_ERROR` "se requiere al menos un grupo"

#### Scenario: group_ids excede 10

- GIVEN `group_ids` con 11 ids
- WHEN se ejecuta POST
- THEN responde 400 `VALIDATION_ERROR` "máximo 10 grupos por publicación"

### Requirement: PublishMany multi-grupo secuencial

El servicio MUST exponer `PublishMany(ctx, actorID, payload)` donde
`payload = {text, photo_url, buttons, group_ids}`. Flujo MUST:

1. Validar payload completo (ver "Validaciones de payload"). Si falla,
   abortar con 400 sin tocar Telegram ni la tabla.
2. Para cada `group_id` en el orden recibido: verificar existencia del
   grupo en `groups` (404 si no existe) y `permissionOk`
   (`g.BotStatus == StatusAdministrator`, 403 si no es admin). Si la
   validación per-grupo falla, registrar fila con status `failed`,
   `error_message` legible, log con `ActionPublishMessage` y pasar al
   siguiente grupo (NO abortar el resto).
3. Para cada grupo válido: insertar fila con status `sending`, llamar
   `SendPhoto` (si hay `photo_url`) o `SendMessage` (si no), capturar
   `message_id`. En éxito → status `sent`. En error → status `failed`
   con `error_message`. Log por fila con `ActionPublishMessage`,
   `metadata = {publication_id, message_id?}`.
4. Envío MUST ser **secuencial** (un grupo a la vez), nunca paralelo:
   respeta el token bucket global del adapter (§18.1).
5. Devolver todas las filas creadas (orden = orden de `group_ids`).

#### Scenario: Publicación exitosa a N grupos

- GIVEN `group_ids = [g1, g2, g3]`, los 3 existen y el bot es admin en
  los 3
- WHEN se ejecuta `PublishMany`
- THEN crea 3 filas; cada fila queda `sent` con su `message_id`; los
  3 logs `PUBLISH_MESSAGE` existen; respuesta con `[pub1, pub2, pub3]`

#### Scenario: Fallo parcial — un grupo falla, el resto NO se aborta

- GIVEN `group_ids = [g1, g2, g3]`, `g2` no existe
- WHEN se ejecuta `PublishMany`
- THEN `g1` queda `sent`, `g2` queda `failed` con `error_message`
  "grupo no encontrado", `g3` queda `sent`; respuesta con las 3
  filas; el envío a `g3` ocurrió DESPUÉS de procesar `g2`

#### Scenario: Fallo parcial — bot sin admin en un grupo

- GIVEN `group_ids = [g1, g2]`, en `g2` el bot NO es admin
- WHEN se ejecuta `PublishMany`
- THEN `g1` queda `sent`, `g2` queda `failed` con `error_message`
  sobre permiso; el envío a `g1` y la evaluación de `g2` se
  ejecutan en el orden de `group_ids`

#### Scenario: Orden secuencial verificable

- GIVEN un fake de `TelegramService` que registra el orden de
  invocaciones
- WHEN se ejecuta `PublishMany` con 3 grupos
- THEN las llamadas a `Send*` se invocan **exactamente** en el orden
  de `group_ids` recibido (test asserts índices)

#### Scenario: Telegram rechaza el envío de un grupo

- GIVEN `group_ids = [g1, g2]`; `SendMessage` para `g1` falla
- WHEN se ejecuta `PublishMany`
- THEN `g1` queda `failed` con `error_message` legible, `g2` se
  intenta y queda `sent`; el procesamiento continúa

### Requirement: Tipos InlineKeyboardMarkup/InlineKeyboardButton (adapter)

El paquete `telegram` MUST exponer tipos públicos:

```go
type InlineKeyboardButton struct {
    Text string `json:"text"`
    URL  string `json:"url"`
}
type InlineKeyboardMarkup struct {
    InlineKeyboard [][]InlineKeyboardButton `json:"inline_keyboard"`
}
```

Los nombres de los tags JSON MUST coincidir con los campos esperados
por la Bot API (`text`, `url`, `inline_keyboard`). La serialización
MUST ser validada por tests con un payload JSON exacto.

#### Scenario: Serialización correcta

- GIVEN un `InlineKeyboardMarkup` con dos filas (1 botón + 2 botones)
- WHEN se serializa a JSON
- THEN el resultado contiene `"inline_keyboard": [[{"text":"A","url":"https://..."}], [{"text":"B","url":"https://..."},{"text":"C","url":"https://..."}]]`

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

#### Scenario: sendPhoto exitoso

- GIVEN un adapter con token válido y stub que responde con
  `{ok:true, result:{message_id:42}}`
- WHEN se llama `SendPhoto(ctx, chatID, "https://x/y.jpg", "hola", nil)`
- THEN el método HTTP invocado es `sendPhoto`, los campos enviados son
  `chat_id`, `photo` (=URL), `caption`="hola"; retorna `(42, nil)`

#### Scenario: sendPhoto con reply_markup

- GIVEN un `*InlineKeyboardMarkup` no nil con una fila de dos botones
- WHEN se llama `SendPhoto(...)` con ese keyboard
- THEN el body del request contiene `reply_markup.inline_keyboard` con
  la estructura JSON exacta esperada por la Bot API

#### Scenario: 429 con retry_after

- GIVEN un stub que responde 429 con `retry_after=2` en el primer
  intento y éxito en el segundo
- WHEN se llama `SendPhoto`
- THEN se espera 2s, se reintenta, y devuelve el `message_id`

#### Scenario: reply_markup nil se omite del payload

- GIVEN `keyboard = nil`
- WHEN se serializa el body del request
- THEN el campo `reply_markup` no está presente (omitempty)

### Requirement: README sección publicaciones

El `README.md` MUST incluir una sección "Publicaciones" que documente
los **límites operacionales** del slice 2: foto por URL pública con
descarga por Telegram (≤ 5 MB, debe ser accesible), caption ≤ 1024
cuando hay foto, texto ≤ 4096 sin foto, máximo 10 grupos por
publicación, botones de URL únicamente (sin `callback_data` en MVP),
máximo 8 filas × 8 botones. La tabla de "Uso del panel" MUST incluir
`/publications`.

#### Scenario: README incluye límites

- GIVEN el `README.md` revisado
- WHEN un admin lee la sección Publicaciones
- THEN encuentra los límites (5 MB, 1024, 4096, 10 grupos, 8×8
  botones) y la nota de que el panel solo publica (no recibe
  callback_data)

### MODIFIED Requirements

### Requirement: Tabla publications

El sistema MUST crear la tabla `publications` con columnas: `id`
(SERIAL PK), `telegram_id` (int64, FK a `groups.telegram_id`), `text`
(text, non-empty, ≤4096 sin foto / ≤1024 con foto), `status` (enum
`draft | scheduled | sending | sent | failed`), `message_id` (int64
nullable), `scheduled_at` (timestamptz nullable), `error_message`
(text nullable), `actor_id` (bigint nullable), `created_at`,
`updated_at`, **`photo_url` (TEXT NULL)**, **`buttons` (JSONB NULL)**.
Debe existir un índice por `telegram_id` y otro por `created_at` DESC.
La columna `buttons` MUST almacenar `[[{text,url}, ...], ...]` (array
de filas; cada fila array de `{text, url}`).

(Previously — slice 1: la tabla se creaba con `text`, `status`,
`message_id`, `scheduled_at`, `error_message`, `actor_id`,
`created_at`, `updated_at` y FK a `groups.telegram_id` — sin columnas
para foto ni botones. Slice 2 agrega `photo_url` y `buttons` vía
migración 00005, sin backfill: filas existentes quedan con
`photo_url=NULL`, `buttons=NULL`.)

#### Scenario: Migración 00005 ALTER aplica limpia

- GIVEN la base con la tabla `publications` de slice 1 y N filas
  existentes
- WHEN se ejecuta `goose up` para 00005
- THEN las columnas `photo_url TEXT` y `buttons JSONB` quedan
  agregadas, ambas NULL-able; las filas existentes mantienen sus
  valores y `photo_url=NULL`, `buttons=NULL`

#### Scenario: Migración 00005 Down revierte

- GIVEN la base con la migración 00005 aplicada
- WHEN se ejecuta `goose down` para 00005
- THEN las columnas `photo_url` y `buttons` se eliminan; las filas
  existentes no se pierden (solo se dropean las columnas)

#### Scenario: Forma del JSONB buttons

- GIVEN una fila con `buttons = [[{"text":"Ir","url":"https://a"}], [{"text":"B","url":"https://b"}, {"text":"C","url":"https://c"}]]`
- WHEN se lee la fila
- THEN el valor se deserializa a `[][]InlineKeyboardButton` con 2
  filas (1 + 2 botones)

#### Scenario: Creación de publicación con foto + botones

- GIVEN un admin autenticado que envía `POST /api/publications`
- WHEN el envío a Telegram (con `sendPhoto`) es exitoso
- THEN persiste una fila con `photo_url` no nulo, `buttons` no nulo,
  status `sent`, `message_id` no nulo y `error_message` NULL

#### Scenario: Envío fallido preserva payload

- GIVEN un envío que Telegram rechaza
- THEN la fila queda con `photo_url`/`buttons` con los valores
  enviados, status `failed`, `message_id` NULL y `error_message`
  con la descripción del error

### Requirement: SendMessage en el adapter

El `telegram.Service` MUST exponer `SendMessage(ctx, chatID int64,
text string, disableWebPagePreview bool, keyboard *InlineKeyboardMarkup) (int64, error)`
que invoque el método `sendMessage` de la Bot API. El parámetro
`keyboard` es opcional: si no es nil, se serializa como
`reply_markup`. Si es nil, se omite del payload (`omitempty`). La
llamada MUST pasar por `doWithRetry` (429 con max 3 reintentos) y
respetar el rate limiter token bucket (§18.1). El retorno es el
`message_id` de Telegram.

(Previously — slice 1: la firma era
`SendMessage(ctx, chatID, text, disableWebPagePreview)` sin keyboard.
Slice 1 archivada no enviaba botones.)

#### Scenario: Envío exitoso sin teclado

- GIVEN un adapter con token válido y rate limiter disponible
- WHEN se llama `SendMessage(ctx, chatID, text, false, nil)`
- THEN el body NO contiene `reply_markup`; el request a
  `sendMessage` devuelve el `message_id` sin error

#### Scenario: Envío con teclado

- GIVEN un `*InlineKeyboardMarkup` con una fila de un botón
- WHEN se llama `SendMessage(..., keyboard)`
- THEN el body contiene `reply_markup.inline_keyboard` con la fila
  serializada correctamente; el método devuelve el `message_id`

#### Scenario: 429 con retry_after

- GIVEN un stub que responde 429 con `retry_after=2`
- WHEN se llama `SendMessage`
- THEN se espera 2s, se reintenta, y si el reintento es exitoso
  devuelve el `message_id`

### Requirement: POST /api/publications (crear + enviar multi-grupo)

El endpoint `POST /api/publications` MUST aceptar
`{text, photo_url?, buttons?, group_ids: int64[]}` donde cada
`group_id` es `groups.telegram_id`. La autenticación es requerida
(`requireAuth`). El servicio MUST:

1. Validar payload completo (ver "Validaciones de payload"). Falla →
   400 `VALIDATION_ERROR` con mensaje en español. **No** se crea fila,
   **no** se llama Telegram.
2. Para cada `group_id`, en orden: verificar existencia en `groups`
   (404 → fila `failed`, sigue) y `permissionOk`
   (`g.BotStatus == StatusAdministrator`; si no → fila `failed`,
   sigue).
3. Para cada grupo válido: insertar fila con `photo_url`, `buttons`,
   status `sending`.
4. Llamar `telegram.SendPhoto` (si hay `photo_url`) o
   `telegram.SendMessage` (si no). Si hay botones, se pasan al adapter.
5. En éxito: status `sent`, `message_id`.
6. En fallo: status `failed`, `error_message`.
7. Registrar log por fila con `ActionPublishMessage`, metadata
   `{publication_id, message_id?}`, status según resultado.
8. Devolver **201 Created** con el envelope
   `{publications: [Publication, ...]}` en el mismo orden que
   `group_ids`. Cada `Publication` incluye `id`, `telegram_id`,
   `text`, `status`, `message_id`, `error_message`, `photo_url`,
   `buttons`, `actor_id`, `created_at`.

El fallo parcial (uno o más grupos con `failed`) NO cambia el status
code: la respuesta sigue siendo 201 con cada fila y su `status`
individual. Errores de payload (validación) son 400. No hay
transacción entre filas: cada fila es autónoma.

(Previously — slice 1: el body era `{text, group_id}` con un único
grupo y la respuesta 201 contenía la fila única. Slice 1 no aceptaba
`photo_url`, `buttons` ni `group_ids[]`. El check de admin usaba
`can_manage_chat`, reemplazado por `bot_status == administrator` en
bugfix #172.)

#### Scenario: Publicación exitosa multi-grupo con foto + botones

- GIVEN un admin autenticado, `text` válido, `photo_url` http(s),
  `buttons` con una fila, `group_ids = [g1, g2]` y bot admin en ambos
- WHEN envía `POST /api/publications`
- THEN responde 201 con `data.publications = [pub1, pub2]`,
  `pub1.status = "sent"`, `pub2.status = "sent"`, ambos con
  `photo_url` y `buttons`; existen 2 logs `PUBLISH_MESSAGE`

#### Scenario: 400 por group_ids vacío

- GIVEN `group_ids = []`
- WHEN se ejecuta POST
- THEN responde 400 `VALIDATION_ERROR` "se requiere al menos un grupo"

#### Scenario: 400 por URL de foto no http(s)

- GIVEN `photo_url = "ftp://x/y.jpg"`
- WHEN se ejecuta POST
- THEN responde 400 `VALIDATION_ERROR`; no se crea fila ni se llama
  Telegram

#### Scenario: Fallo parcial se devuelve con 201

- GIVEN `group_ids = [g1, g2, g3]`, `g2` no existe
- WHEN se ejecuta POST
- THEN responde 201 con 3 filas; `pub1.status="sent"`,
  `pub2.status="failed"` con `error_message` legible,
  `pub3.status="sent"`; el envío a `g3` ocurre después de evaluar
  `g2`; existen 3 logs (2 SUCCESS + 1 NOT_FOUND)

#### Scenario: Caption excede 1024 con foto

- GIVEN `text` con 1100 caracteres y `photo_url` presente
- WHEN se ejecuta POST
- THEN responde 400 `VALIDATION_ERROR`; no se crea fila

#### Scenario: Bot no admin en un grupo, el resto se procesa

- GIVEN `group_ids = [g1, g2]`, en `g2` el bot es `member`
- WHEN se ejecuta POST
- THEN responde 201 con 2 filas; `pub1.status="sent"`,
  `pub2.status="failed"` con `error_message` "el bot no es
  administrador del grupo"; existe log `PUBLISH_MESSAGE` para `g2`
  con status `PERMISSION_DENIED`

### Requirement: GET /api/publications (listado con filtro opcional)

El endpoint `GET /api/publications` MUST devolver publicaciones
ordenadas por `created_at` DESC, con un límite de 50 registros. Si
el query param `group_id=<int64>` está presente, MUST filtrar por
`telegram_id = group_id`. Sin filtro → todas las 50 más recientes.
Cada registro incluye: `id`, `telegram_id`, `text`, `status`,
`message_id`, `error_message`, `actor_id`, `created_at`,
`photo_url`, `buttons`. La autenticación es requerida. El índice
`idx_publications_telegram_id` MUST ser usado por el plan de query
cuando hay filtro.

(Previously — slice 1: el endpoint listaba las 50 más recientes sin
filtro, sin incluir `photo_url` ni `buttons` en la respuesta. Slice 2
agrega filtro opcional `?group_id=` y los campos de foto/botones.)

#### Scenario: Listado sin filtro

- GIVEN 3 publicaciones existentes
- WHEN se ejecuta GET `/api/publications`
- THEN responde 200 con las 3 publicaciones ordenadas por fecha
  DESC, cada una con `photo_url`, `buttons`

#### Scenario: Listado con filtro por grupo

- GIVEN publicaciones en grupos g1 (3) y g2 (2)
- WHEN se ejecuta GET `/api/publications?group_id=g1`
- THEN responde 200 con 3 publicaciones, todas con
  `telegram_id = g1`

#### Scenario: Listado filtrado sin resultados

- GIVEN publicaciones solo en `g1`
- WHEN se ejecuta GET `/api/publications?group_id=g2`
- THEN responde 200 con array vacío

#### Scenario: Listado vacío

- GIVEN no hay publicaciones
- WHEN se ejecuta GET
- THEN responde 200 con array vacío

### Requirement: Frontend PublicationsPage

La ruta `/publications` MUST renderizar `PublicationsPage` envuelta
en `RequireAuth`. La página MUST mostrar:

- Lista de publicaciones con estados: loading, vacío, error, y filtro
  por grupo (selector con opción "Todos" + cada grupo del panel
  desde `GET /api/groups`).
- Cada publicación muestra: texto, foto si `photo_url` (render
  `<img>` o link), botones como chips/labels, grupo, status (badge),
  fecha.
- Formulario de creación: textarea para `text` (maxlength dinámico:
  1024 si hay foto, 4096 si no), input `photo_url` (opcional,
  validación http(s) con `URL`), editor de botones (lista de filas
  con campos `text`/`url`; botones "Agregar fila" / "Quitar fila" /
  "Agregar botón" / "Quitar botón" por fila), multi-select de
  grupos (checkboxes desde `useGroups()`; mínimo 1 grupo).
- Validación cliente con mensajes en español; al menos un grupo
  requerido; URL http(s); text no vacío.
- Al enviar: `POST /api/publications`, invalidar query `['publications']`
  (todas las variantes de filtro), mostrar errores legibles (§18).
- React Query: `usePublications(filter)` con key
  `['publications', group_id ?? 'all']`. El cambio de filtro recarga
  la query (no es la misma key).

(Previously — slice 1: el form tenía solo textarea + select de grupo
único y el listado mostraba texto/grupo/status/fecha. Slice 2 agrega
foto, editor de botones, multi-select, preview de foto y botones en el
listado, y filtro por grupo con invalidación de query key.)

#### Scenario: Página carga con datos y filtro "Todos"

- GIVEN 2 publicaciones existentes y 2 grupos en `GET /api/groups`
- WHEN se navega a `/publications`
- THEN se renderizan ambas publicaciones con foto/botones cuando
  aplica; el selector muestra "Todos" + los 2 grupos

#### Scenario: Filtro por grupo recarga la lista

- GIVEN 3 publicaciones (2 en g1, 1 en g2)
- WHEN el admin selecciona "g1" en el filtro
- THEN se ejecuta GET `/api/publications?group_id=g1`; se muestran
  solo las 2 publicaciones de `g1`

#### Scenario: Página sin datos

- GIVEN no hay publicaciones
- WHEN se navega a `/publications`
- THEN se muestra estado vacío con mensaje "No hay publicaciones"

#### Scenario: Error de carga

- GIVEN el backend responde 500
- WHEN se carga la página
- THEN se muestra error con opción de reintentar

#### Scenario: Crear publicación multi-grupo exitosa

- GIVEN un texto válido, una `photo_url` http(s), una fila de
  botones y 2 grupos seleccionados
- WHEN se envía el formulario
- THEN se ejecuta POST, se invalida la lista, ambas publicaciones
  aparecen en el listado con status y foto

#### Scenario: Crear publicación con URL inválida

- GIVEN `photo_url = "no-es-url"`
- WHEN se envía el formulario
- THEN se muestra mensaje legible "la URL debe empezar con http o
  https" sin enviar el POST

#### Scenario: Crear publicación sin grupos seleccionados

- GIVEN texto y foto válidos pero 0 grupos marcados
- WHEN se intenta enviar
- THEN se muestra "se requiere al menos un grupo" sin enviar el
  POST

### Requirement: Tests backend

Los tests MUST cubrir (mockeando `TelegramService` — nunca Bot API
real; §21.1):

- **Adapter**:
  - `SendMessage` con/sin teclado decodifica `message_id` y omite
    `reply_markup` cuando nil.
  - `SendPhoto` decodifica `message_id`; payload contiene `photo`,
    `caption`, `reply_markup` cuando hay keyboard.
  - Serialización de `InlineKeyboardMarkup` con el JSON exacto
    esperado por la Bot API.
  - 429 con `retry_after` se respeta en `SendMessage` y `SendPhoto`
    (3 reintentos máx).
- **Servicio**:
  - Validaciones de payload (text > 1024 con foto, text > 4096 sin
    foto, `photo_url` no http(s), `buttons` excede 8×8, `group_ids`
    vacío o > 10).
  - `PublishMany` SECUENCIAL: fake registra orden de invocaciones y
    se verifica el orden == orden de `group_ids` (assertions de
    índices entre filas).
  - Fallo parcial: un grupo Telegram-rechaza, los demás OK.
  - Permission denied en al menos un grupo: las filas de los grupos
    válidos se procesan y la del grupo no-admin queda `failed`.
  - Caption 1024 vs texto 4096 aplicado según haya foto.
- **Handler**: requests HTTP contra POST/GET con envelope correcto
  (data/error), 201 multi-grupo, 400 validaciones, 200 con/sin
  filtro.

(Previously — slice 1: los tests cubrían SendMessage, servicio con
sendMessage OK/error, handler POST 201/400, GET listado y GET por
id. Slice 2 agrega cobertura de SendPhoto, serialización de
keyboard, validaciones nuevas, multi-grupo secuencial, fallo
parcial y filtro por grupo.)

#### Scenario: Servicio con Telegram mockeado — multi-grupo OK

- GIVEN un servicio con TelegramService mock que retorna message_id
- WHEN se ejecuta `PublishMany` con 2 grupos válidos
- THEN 2 filas quedan con status `sent` y `message_id` guardado

#### Scenario: Servicio con Telegram error parcial

- GIVEN un servicio con TelegramService mock que retorna error para
  el primer grupo y OK para el segundo
- WHEN se ejecuta `PublishMany`
- THEN la primera fila queda `failed` con `error_message` poblado, la
  segunda queda `sent`, existen 2 logs

#### Scenario: SendPhoto decodifica message_id

- GIVEN un stub HTTP que responde con `{ok:true, result:{message_id:7}}`
- WHEN se llama `SendPhoto(...)` con `keyboard=nil`
- THEN retorna `(7, nil)` y el body enviado contiene `chat_id`,
  `photo`, `caption` (sin `reply_markup`)

### Requirement: Tests frontend

Los tests de `PublicationsPage` MUST usar `mockFetchRoutes` (o mock
propio distinguiendo GET/POST por `init.method`) para simular las
respuestas de la API. Deben cubrir: carga exitosa, lista vacía,
error de carga, creación exitosa con invalidación, error al crear
con mensaje visible, formulario con foto + botones + multi-grupo, y
filtro por grupo que recarga con `?group_id=`.

(Previously — slice 1: los tests cubrían carga, error y creación con
grupo único. Slice 2 agrega casos con foto+botones+multi-grupo y filtro.)

#### Scenario: Test de carga con foto y botones

- GIVEN `mockFetchRoutes` con respuesta de lista que incluye
  `photo_url` y `buttons`
- WHEN se renderiza `PublicationsPage`
- THEN se muestra la imagen y los chips de los botones

#### Scenario: Test de filtro por grupo

- GIVEN `mockFetchRoutes` configurado para devolver listas distintas
  según query param
- WHEN el admin cambia el filtro a un grupo
- THEN la pantalla muestra el subconjunto y se observa la nueva
  request con `?group_id=`

#### Scenario: Test de formulario multi-grupo

- GIVEN `mockFetchRoutes` que acepta POST con `group_ids` de 2 ids
- WHEN se envía el formulario con foto + botones + 2 grupos
- THEN se ejecuta POST con el body correcto y se invalida la lista

#### Scenario: Test de error

- GIVEN `mockFetchRoutes` con respuesta 500
- WHEN se renderiza `PublicationsPage`
- THEN se muestra el estado de error

### Requirement: Registro de auditoría

Cada intento de publicación (éxito o fallo) MUST generar un log con:
`ActionPublishMessage`, `actor_id` del admin, `group_id` (telegram
id), `metadata` con `publication_id` y `message_id` (si éxito),
`status` (SUCCESS, NOT_FOUND, PERMISSION_DENIED, TELEGRAM_ERROR,
VALIDATION_ERROR según corresponda), y `error_message` si falló.
En multi-grupo, **un log por grupo** (no un log consolidado). El
log se emite incluso si la fila queda `failed` (sin `message_id`).

(Previously — slice 1: cada publicación generaba exactamente un log.
Slice 2 mantiene un log por intento de publicación, pero como
`PublishMany` produce N filas, se emiten N logs.)

#### Scenario: Log de publicación exitosa

- GIVEN un envío exitoso con `message_id=123`
- THEN existe un log con action `PUBLISH_MESSAGE`, metadata
  `{"publication_id": 1, "message_id": 123}`, status `SUCCESS`

#### Scenario: Log de publicación fallida por Telegram

- GIVEN un envío que falla con error de Telegram
- THEN existe un log con action `PUBLISH_MESSAGE`, status
  `TELEGRAM_ERROR`, y `error_message` no nulo

#### Scenario: Multi-grupo genera N logs

- GIVEN `PublishMany` con 3 grupos: 2 OK, 1 con bot no admin
- WHEN el proceso termina
- THEN existen 3 logs `PUBLISH_MESSAGE`: 2 con `SUCCESS`, 1 con
  `PERMISSION_DENIED`
