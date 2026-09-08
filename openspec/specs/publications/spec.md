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
> Slice 3 (archivada en
> `openspec/changes/archive/2026-09-07-publications-slice3/`,
> mergeada en este archivo) cierra Fase 2 del proyecto: programación
> in-process con worker `time.Ticker` + `SELECT ... FOR UPDATE SKIP
> LOCKED`, paginación del historial (`?limit=` / `?offset=`), y
> cancelación de filas `scheduled` (`DELETE /api/publications/:id`).
> NO se introducen cambios al adapter de Telegram: el worker reusa
> `SendMessage` + `SendPhoto` (métodos slice 2). Bugfix #172 sigue
> vigente — `permissionOk` usa `g.BotStatus == StatusAdministrator`,
> también en el worker (`Scheduler.permissionOk`).

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

---

## Slice 3 Extensions (2026-09-07 — Programación + Paginación + Cancel)

Las siguientes requirements fueron agregadas o modificadas por el
slice 3 (archivado en `openspec/changes/archive/2026-09-07-publications-slice3/`).
Las requirements de slices 1 y 2 se conservan tal cual en este archivo
(las versiones vigentes aparecen aquí reemplazando a las anteriores;
las versiones anteriores permanecen en los archives de
`2026-09-07-publications/` y `2026-09-07-publications-slice2/`).

Slice 3 **NO** modifica `telegram-moderation`: el worker reutiliza
los métodos `SendMessage(..., keyboard)` y `SendPhoto(...)` del
adapter (slice 2). No se agregan métodos nuevos al adapter. El único
dominio tocado es `publications`.

> **Bot API**: `sendMessage` NO acepta `schedule_date` para bots en
> grupos. La programación se implementa **in-process** con un worker
> Go (`time.Ticker` + `SELECT ... FOR UPDATE SKIP LOCKED`), no como
> atajo de la Bot API.

### ADDED Requirements

### Requirement: Worker in-process (Scheduler)

El paquete `publications` MUST exponer un `Scheduler` lanzable como
goroutine. `Scheduler.Run(ctx context.Context) error` MUST ejecutar el
siguiente ciclo:

1. Lanzar `time.NewTicker(s.interval)`. Constante
   `DefaultSchedulerInterval = 30 * time.Second` se usa en producción.
   Tests inyectan `interval` bajo (5ms típico).
2. En cada tick, invocar `store.ClaimScheduledDue(ctx, 25)` dentro de
   una transacción explícita con este patrón:
   - `SELECT id, telegram_id, text, photo_url, buttons, scheduled_at
      FROM publications WHERE status='scheduled' AND scheduled_at <=
      now() ORDER BY scheduled_at ASC LIMIT 25 FOR UPDATE SKIP LOCKED`.
   - Por cada id reservado, `UPDATE publications SET status='sending',
      updated_at=now() WHERE id=$1`.
   - `COMMIT`.
3. Por cada fila devuelta (transición `scheduled → sending`), invocar
   `publishOne(...)` que implementa `permissionOk` (`g.BotStatus ==
   StatusAdministrator`, bugfix #172), dispatch `SendPhoto` /
   `SendMessage`, `UpdateStatus(sent|failed, ...)`, y log
   `ActionPublishMessage`.
4. NO reintentar filas `failed` automáticamente.
5. Retornar `nil` cuando `ctx.Done()` se dispara (cancelación
   limpia).
6. Emitir por tick `slog.Info("publications worker tick", "due", N,
   "claimed", M, "interval", s.interval.String())`.

Lanzado como goroutine desde `cmd/server/main.go` con el mismo
`ctx` (`signal.NotifyContext`) que el poller (`main.go:194-202`),
siguiendo el patrón de `telegram.Poller`. Constante `claimBatchSize
= 25`.

#### Scenario: Tick reclama N filas due

- GIVEN 2 publicaciones con `status='scheduled'` y `scheduled_at <
  now()`
- WHEN el worker ejecuta un tick
- THEN `ClaimScheduledDue` retorna 2 ids; ambas filas pasan a
  `status='sending'`; el worker llama `publishOne` 2 veces; el log de
  tick muestra `due=2, claimed=2`

#### Scenario: Tick salta filas no-vencidas

- GIVEN 3 filas `scheduled` con `scheduled_at` futuro (a 1h) y 0
  filas due
- WHEN el worker ejecuta un tick
- THEN `ClaimScheduledDue` retorna 0 ids; ninguna fila cambia de
  estado; log de tick `due=0, claimed=0`

#### Scenario: Claim respeta SKIP LOCKED entre transacciones

- GIVEN 2 filas due y dos transacciones concurrentes T1 y T2
- WHEN T1 ejecuta `ClaimScheduledDue(limit=25)` y aún no commitea
- THEN T2 ejecuta `ClaimScheduledDue` y NO recibe los ids de T1 (SKIP
  LOCKED); tras COMMIT de T1, una nueva transacción T3 ve los ids
  restantes

#### Scenario: Fila con Telegram error queda failed (sin retry)

- GIVEN una fila `scheduled` due, el worker la reclama, `publishOne`
  llama `SendMessage` y el adapter devuelve error
- WHEN el worker termina de procesar esa fila
- THEN la fila queda `status='failed'` con `error_message` con la
  descripción legible; log `PUBLISH_MESSAGE` con status
  `TELEGRAM_ERROR`; el worker NO reintenta, sigue con la siguiente
  fila

#### Scenario: Cancelación de ctx detiene el worker limpiamente

- GIVEN un worker corriendo con `interval=1s`
- WHEN `ctx.Done()` se dispara
- THEN `Run` retorna `nil` (sin error); no quedan goroutines
  pendientes; las filas `scheduled` no se tocan

#### Scenario: Worker respeta rate limit del adapter

- GIVEN 2 filas due apuntando al mismo grupo y el adapter con token
  bucket
- WHEN el worker las reclama y procesa
- THEN las llamadas `Send*` se ejecutan SECUENCIALMENTE (nunca
  paralelo), respetando el token bucket global (§18.1)

#### Scenario: Worker marca failed por permissionOk

- GIVEN una fila due apuntando a un grupo donde el bot NO es
  administrador
- WHEN el worker la reclama y procesa
- THEN `publishOne` detecta `g.BotStatus != administrator` → fila
  queda `status='failed'` con `error_message` "el bot no es
  administrador del grupo"; log `PUBLISH_MESSAGE` con status
  `PERMISSION_DENIED`

### Requirement: DELETE /api/publications/:id

El endpoint `DELETE /api/publications/:id` MUST permitir cancelar una
publicación programada. Reglas:

1. Autenticación requerida (`requireAuth`).
2. Si `id` no existe → **404 NOT_FOUND**.
3. Si existe con `status='scheduled'` → **hard delete + 204 No
   Content**. La fila ya no existe en la tabla; el próximo tick del
   worker la ignora.
4. Si existe con `status ∈ {sending, sent, failed}` → **409 Conflict**
   con `code='INVALID_STATUS'` y mensaje en español "no se puede
   cancelar una publicación ya enviada o en curso".
5. Hard delete (sin soft delete, sin status `cancelled`). Filas
   `sent`/`failed` preservan audit trail.
6. Lectura del status ANTES del DELETE para detectar la race con el
   worker.

#### Scenario: Cancelar una publicación scheduled

- GIVEN una fila con `status='scheduled'` e id=10
- WHEN se ejecuta `DELETE /api/publications/10`
- THEN responde 204 No Content; la fila ya no existe en la tabla;
  el próximo tick del worker la ignora

#### Scenario: Cancelar una publicación sent → 409

- GIVEN una fila con `status='sent'` e id=20
- WHEN se ejecuta `DELETE /api/publications/20`
- THEN responde 409 con `code='INVALID_STATUS'` y mensaje legible;
  la fila sigue en la tabla con `status='sent'`

#### Scenario: Cancelar una publicación sending → 409

- GIVEN una fila con `status='sending'` e id=30 (worker ya la marcó
  pero aún no envió)
- WHEN se ejecuta `DELETE /api/publications/30`
- THEN responde 409 con `code='INVALID_STATUS'` y mensaje legible;
  la fila sigue en la tabla

#### Scenario: Cancelar id inexistente → 404

- GIVEN no existe publicación con id=999
- WHEN se ejecuta `DELETE /api/publications/999`
- THEN responde 404 NOT_FOUND

#### Scenario: Race worker tick vs DELETE scheduled

- GIVEN una fila `scheduled` y el worker en el medio de un tick
  (acaba de marcarla `sending` en la misma transacción)
- WHEN llega `DELETE` casi simultáneamente
- THEN el DELETE puede: (a) si llega antes de que el worker
  commitee, leer `scheduled` y borrar la fila (204); (b) si llega
  después, leer `sending` y retornar 409. Ambos outcomes son
  aceptables; el spec NO exige orden. Si llegan exactamente en el
  medio y el worker gana, el DELETE lee `sending` → 409.

### Requirement: Errores de scheduling y paginación

El paquete `publications` MUST exponer tres errores de dominio:

```go
var (
    ErrScheduledInPast   = errors.New("scheduled_at debe ser una fecha futura")
    ErrInvalidPagination = errors.New("limit debe estar entre 1 y 100 y offset >= 0")
    ErrCancelNotAllowed  = errors.New("no se puede cancelar una publicación ya enviada o en curso")
)
```

Mapeo: `ErrScheduledInPast` → 400 `VALIDATION_ERROR`;
`ErrInvalidPagination` → 400 `VALIDATION_ERROR`;
`ErrCancelNotAllowed` → 409 `INVALID_STATUS`. Mensajes en español.

#### Scenario: scheduled_at en el pasado → 400

- GIVEN `scheduled_at = "2020-01-01T00:00:00Z"` (claramente pasado)
- WHEN se ejecuta `POST /api/publications`
- THEN responde 400 `VALIDATION_ERROR` "scheduled_at debe ser una
  fecha futura"; NO se crea fila; NO se llama Telegram

#### Scenario: limit fuera de rango → 400

- GIVEN `?limit=101` o `?limit=0`
- WHEN se ejecuta `GET /api/publications`
- THEN responde 400 `VALIDATION_ERROR` "limit debe estar entre 1 y
  100"

#### Scenario: offset negativo → 400

- GIVEN `?offset=-1`
- WHEN se ejecuta `GET /api/publications`
- THEN responde 400 `VALIDATION_ERROR` "offset debe ser >= 0"

#### Scenario: Cancelación de fila no-scheduled → 409

- GIVEN una fila con `status='sent'` o `status='sending'`
- WHEN se ejecuta `DELETE /api/publications/:id`
- THEN responde 409 `INVALID_STATUS` con mensaje legible; la fila
  permanece intacta

### MODIFIED Requirements

### Requirement: POST /api/publications (multi-grupo + scheduling opcional)

El endpoint `POST /api/publications` MUST aceptar
`{text, photo_url?, buttons?, group_ids: int64[], scheduled_at?}`
donde cada `group_id` es `groups.telegram_id`. El campo `scheduled_at`
es opcional; cuando está presente, es RFC3339 con offset (se normaliza
a UTC). La autenticación es requerida (`requireAuth`).

El servicio MUST branchear según la presencia de `scheduled_at`:

- **`scheduled_at == nil`** → comportamiento slice 2 (`PublishMany`
  síncrono inmediato).
- **`scheduled_at != nil && futuro (scheduled_at > now())`** →
  validar payload igual que publish-now. Para cada `group_id`, insertar
  una fila con `status='scheduled'`, `scheduled_at` en UTC,
  `photo_url`, `buttons`, `actor_id`. NO llamar a Telegram. Devolver
  **201 Created** con `{publications: [N Publication, ...]}`.
- **`scheduled_at != nil && scheduled_at <= now()`** → **400
  VALIDATION_ERROR** "scheduled_at debe ser una fecha futura". NO se
  crea fila; NO se llama Telegram.

Cada fila `scheduled` será procesada por el `Scheduler`. NO se
reintenta automáticamente una fila `failed`. Cada fila es autónoma
(sin transacción entre filas).

(Previously — slice 2: el body era `{text, photo_url?, buttons?,
group_ids: int64[]}` y la respuesta siempre 201 con cada fila en su
estado post-envío. Slice 2 nunca aceptaba `scheduled_at`. Slice 3
agrega `scheduled_at` opcional; cuando está presente y es futuro, NO
se envía, las filas quedan `scheduled` para que el worker las procese.)

#### Scenario: Publicación exitosa multi-grupo con foto + botones (sin scheduling)

- GIVEN un admin autenticado, `text` válido, `photo_url` http(s),
  `buttons` con una fila, `group_ids = [g1, g2]` y bot admin en ambos
- WHEN envía `POST /api/publications` (sin `scheduled_at`)
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

#### Scenario: Programación futura — multi-grupo, fila scheduled, sin Telegram

- GIVEN un admin autenticado, `text` válido, `photo_url` http(s),
  `buttons` opcionales, `group_ids = [g1, g2]`,
  `scheduled_at = "2027-01-01T10:00:00Z"` (futuro), bot admin en
  ambos grupos
- WHEN envía `POST /api/publications`
- THEN responde 201 con `data.publications = [pub1, pub2]`,
  `pub1.status = "scheduled"` y `pub1.scheduled_at = "2027-01-01..."`,
  `pub2.status = "scheduled"` y `pub2.scheduled_at` igual; **NO** se
  llama a Telegram; **NO** se emite log `PUBLISH_MESSAGE` (la
  auditoría se emite cuando el worker procesa cada fila)

#### Scenario: Programación con scheduled_at en el pasado → 400

- GIVEN `scheduled_at = "2020-01-01T00:00:00Z"` (claramente pasado)
- WHEN se ejecuta POST
- THEN responde 400 con `code='VALIDATION_ERROR'` y mensaje
  "scheduled_at debe ser una fecha futura"; **NO** se crea fila;
  **NO** se llama Telegram

#### Scenario: Programación con scheduled_at sin offset (Z) se acepta

- GIVEN `scheduled_at = "2027-06-15T14:00:00Z"` (UTC explícito)
- WHEN se ejecuta POST
- THEN la fila se inserta con `scheduled_at` normalizado a UTC; el
  worker la reclama cuando `now() >= scheduled_at`

#### Scenario: Programación con offset +03:00 se normaliza a UTC

- GIVEN `scheduled_at = "2027-06-15T17:00:00+03:00"` (equivale a
  `14:00:00Z`)
- WHEN se ejecuta POST
- THEN la fila se inserta con `scheduled_at = 2027-06-15T14:00:00Z`
  (UTC normalizado); el worker la reclama en el momento correcto

#### Scenario: Programación con datetime-local del frontend

- GIVEN el admin eligió `Programar` en el form y seleccionó
  `2027-06-15 14:00` en su zona horaria local (UTC-3, ej. Argentina)
- WHEN el frontend serializa a RFC3339 con offset (`17:00:00Z`) y
  envía POST
- THEN el backend normaliza a `14:00:00Z` (UTC) y persiste; la fila
  se procesa en el momento UTC correcto independientemente de la zona
  horaria del cliente

### Requirement: GET /api/publications (listado paginado con filtro opcional)

El endpoint `GET /api/publications` MUST devolver publicaciones
ordenadas por `created_at` DESC. La autenticación es requerida.

Query params aceptados:

- `group_id=<int64>` (opcional): si está presente, MUST filtrar por
  `telegram_id = group_id`. Compatible con slice 2.
- `limit=<int>` (opcional, **default 50, max 100**): tamaño de página.
  Si `< 1` o `> 100` → **400 VALIDATION_ERROR**.
- `offset=<int>` (opcional, **default 0**): desplazamiento desde el
  inicio. Si `< 0` → **400 VALIDATION_ERROR**.

Respuesta: **200** con `{data: [Publication, ...]}` ordenado por
`created_at` DESC, hasta `limit` registros. Cada registro incluye:
`id`, `telegram_id`, `text`, `status`, `message_id`, `error_message`,
`actor_id`, `created_at`, `updated_at`, `photo_url`, `buttons`,
**`scheduled_at`** (TIMESTAMPTZ nullable, presente en filas
`scheduled`).

El índice `idx_publications_telegram_id` MUST ser usado por el plan
de query cuando hay filtro `?group_id=`. Sin status filter en MVP.

(Previously — slice 2: el endpoint listaba las 50 más recientes sin
filtro y aceptaba solo `?group_id=`. El cap de 50 era hardcoded. Slice
3 agrega `?limit=` (default 50, max 100) y `?offset=` (default 0). El
cap de 50 deja de ser hardcoded y pasa a ser el default del query
param.)

#### Scenario: Listado sin filtro

- GIVEN 3 publicaciones existentes
- WHEN se ejecuta GET `/api/publications`
- THEN responde 200 con las 3 publicaciones ordenadas por fecha
  DESC, cada una con `photo_url`, `buttons`, `scheduled_at`

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

#### Scenario: Paginación limit y offset

- GIVEN 75 publicaciones existentes
- WHEN se ejecuta GET `/api/publications?limit=10&offset=20`
- THEN responde 200 con 10 publicaciones (filas 21..30 ordenadas por
  `created_at` DESC)

#### Scenario: Paginación última página parcial

- GIVEN 75 publicaciones existentes
- WHEN se ejecuta GET `/api/publications?limit=10&offset=70`
- THEN responde 200 con 5 publicaciones (filas 71..75)

#### Scenario: limit > 100 → 400

- GIVEN `?limit=101`
- WHEN se ejecuta GET
- THEN responde 400 `VALIDATION_ERROR` "limit debe estar entre 1 y
  100"

#### Scenario: limit < 1 → 400

- GIVEN `?limit=0`
- WHEN se ejecuta GET
- THEN responde 400 `VALIDATION_ERROR` "limit debe estar entre 1 y
  100"

#### Scenario: offset < 0 → 400

- GIVEN `?offset=-1`
- WHEN se ejecuta GET
- THEN responde 400 `VALIDATION_ERROR` "offset debe ser >= 0"

#### Scenario: Paginación combinada con filtro por grupo

- GIVEN 30 publicaciones en `g1` y 20 en `g2`
- WHEN se ejecuta GET `/api/publications?group_id=g1&limit=10&offset=10`
- THEN responde 200 con 10 publicaciones de `g1` (filas 11..20 de g1
  por `created_at` DESC)

#### Scenario: Default limit cuando no se envía

- GIVEN 60 publicaciones existentes
- WHEN se ejecuta GET `/api/publications` (sin `?limit`)
- THEN responde 200 con 50 publicaciones (default)

### Requirement: Frontend PublicationsPage

La ruta `/publications` MUST renderizar `PublicationsPage` envuelta en
`RequireAuth`. La página MUST mostrar:

- Lista de publicaciones con estados: loading, vacío, error, y filtro
  por grupo (selector con opción "Todos" + cada grupo del panel desde
  `GET /api/groups`).
- Cada publicación muestra: texto, foto si `photo_url` (render
  `<img>` o link), botones como chips/labels, grupo, status (badge),
  fecha, y **`scheduled_at` formateado cuando `status='scheduled'`**.
  Si `status='sent'`, mostrar link al `message_id` del chat. Si
  `status='failed'`, mostrar `error_message` como tooltip.
- **Formulario dual-mode**: radio / segmented control con dos
  opciones: **"Publicar ahora"** (default) y **"Programar"**. Cuando
  el modo es "Publicar", el formulario NO muestra input de fecha.
  Cuando el modo es "Programar", MUST mostrar `<input
  type="datetime-local" name="scheduled_at">`. El botón submit MUST
  cambiar su label dinámicamente: "Publicar" en modo inmediato,
  "Programar" en modo programado.
- Validación cliente en español: `text` no vacío; `photo_url` http(s)
  si presente; al menos un grupo seleccionado; si modo "Programar",
  `scheduled_at` MUST ser estrictamente futuro (validación cliente en
  `onBlur` o al submit; bloquea el envío si está pasado). Mensaje "la
  fecha debe ser futura".
- **Botón "Cancelar"** por fila visible **SOLO cuando
  `status === 'scheduled'`**. Al hacer click: confirmación inline
  (modal o confirm) y luego `DELETE /api/publications/:id`. Tras
  éxito, refetch de la lista.
- **Paginación Prev / Next**: botones en la parte inferior del
  listado. `Prev` deshabilitado cuando `offset === 0`. `Next`
  deshabilitado cuando la página retornada tiene menos de `limit`
  filas. Estado `limit/offset` local en el componente.
- React Query:
  - `usePublications({group_id, limit, offset})` con query key
    `['publications', {group_id: group_id ?? 'all', limit, offset}]`.
  - `useCancelPublication()` mutation: ejecuta
    `DELETE /api/publications/:id` y al éxito invalida
    `['publications']` (todas las variantes) con
    `queryClient.invalidateQueries`.

(Previously — slice 2: el form tenía solo textarea + select de grupo
único y el listado mostraba texto/grupo/status/fecha. Slice 2 agregó
foto, editor de botones, multi-select, preview de foto y botones en el
listado, y filtro por grupo con invalidación de query key. Slice 3
agrega modo dual "Publicar ahora" / "Programar", datetime-local,
botón Cancelar (solo `scheduled`), controles Prev/Next, y campos
`scheduled_at`/`message_id` link/`error_message` tooltip en el
listado.)

#### Scenario: Página carga con datos y filtro "Todos"

- GIVEN 2 publicaciones existentes y 2 grupos en `GET /api/groups`
- WHEN se navega a `/publications`
- THEN se renderizan ambas publicaciones con foto/botones cuando
  aplica; el selector muestra "Todos" + los 2 grupos; la query key
  es `['publications', {group_id: 'all', limit: 50, offset: 0}]`

#### Scenario: Filtro por grupo recarga la lista

- GIVEN 3 publicaciones (2 en g1, 1 en g2)
- WHEN el admin selecciona "g1" en el filtro
- THEN se ejecuta GET `/api/publications?group_id=g1`; se muestran
  solo las 2 publicaciones de `g1`; la query key cambia a
  `['publications', {group_id: g1, ...}]`

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
- WHEN se envía el formulario (modo "Publicar ahora")
- THEN se ejecuta POST, se invalida la lista, ambas publicaciones
  aparecen en el listado con status `sent` y foto

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

#### Scenario: Modo "Programar" muestra datetime-local

- GIVEN la página cargada en modo default ("Publicar ahora")
- WHEN el admin selecciona el radio "Programar"
- THEN el `<input type="datetime-local">` se vuelve visible; el
  botón submit cambia su label a "Programar"

#### Scenario: Validación cliente: scheduled_at en el pasado

- GIVEN modo "Programar" activo y el admin seleccionó una fecha
  pasada
- WHEN intenta enviar el formulario
- THEN se muestra "la fecha debe ser futura" sin enviar el POST

#### Scenario: Programación multi-grupo futura

- GIVEN modo "Programar", texto válido, `photo_url`, `buttons`,
  `group_ids = [g1, g2]`, `scheduled_at` futura
- WHEN envía el formulario
- THEN se ejecuta POST; la respuesta 201 muestra N filas con
  `status='scheduled'` y `scheduled_at` poblado; la lista refrescada
  muestra ambas filas con badge "scheduled" y el `scheduled_at`
  formateado

#### Scenario: Botón Cancelar visible solo en scheduled

- GIVEN 3 publicaciones: 1 `scheduled`, 1 `sent`, 1 `failed`
- WHEN se renderiza la lista
- THEN solo la fila `scheduled` muestra el botón "Cancelar"; las
  filas `sent` y `failed` no lo muestran

#### Scenario: Cancelación elimina la fila

- GIVEN una fila `scheduled` y el botón "Cancelar" visible
- WHEN el admin confirma la cancelación
- THEN se ejecuta `DELETE /api/publications/:id`; tras 204, la fila
  desaparece de la lista (query invalida y refetch)

#### Scenario: Paginación Next deshabilitado al final

- GIVEN 60 publicaciones y `limit=50`, `offset=0`
- WHEN la página retorna 50 filas (exactamente `limit`)
- THEN el botón "Next" está habilitado (puede haber más); "Prev"
  está deshabilitado

#### Scenario: Paginación Next deshabilitado al final real

- GIVEN 60 publicaciones, `limit=50`, `offset=50`
- WHEN la página retorna 10 filas (última página parcial)
- THEN el botón "Next" está deshabilitado (no hay más); "Prev"
  está habilitado

#### Scenario: Cambio de offset recarga la query

- GIVEN la lista en `offset=0`
- WHEN el admin hace click en "Next"
- THEN se ejecuta GET `/api/publications?offset=50&limit=50`; la
  query key cambia a `['publications', {..., offset: 50}]`; "Prev"
  se habilita

### Requirement: Tests backend

Los tests MUST cubrir (mockeando `TelegramService` — nunca Bot API
real; §21.1; integration tests contra Postgres real para SQL):

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
    se verifica el orden == orden de `group_ids`.
  - Fallo parcial: un grupo Telegram-rechaza, los demás OK.
  - Permission denied en al menos un grupo.
  - Caption 1024 vs texto 4096 aplicado según haya foto.
  - **`Schedule` inserta N filas `scheduled` con mismo `scheduled_at`,
    NO llama a Telegram (verificación con fake spy).**
  - **`Schedule` con `scheduled_at` en el pasado retorna
    `ErrScheduledInPast`.**
  - **`CancelScheduled` con `status='scheduled'` retorna nil y borra
    la fila.**
  - **`CancelScheduled` con `status='sent'` retorna
    `ErrCancelNotAllowed` y NO borra la fila.**
- **Repositorio** (integración Postgres real):
  - `List`/`ListByTelegramID` con `limit/offset` pagina
    correctamente (verifica orden, tamaño y offset).
  - `ClaimScheduledDue` con N due rows retorna exactamente N ids.
  - `ClaimScheduledDue` SKIP LOCKED: dos transacciones concurrentes,
    la segunda NO recibe los ids de la primera.
  - `ClaimScheduledDue` marca `status='sending'` dentro de la misma
    transacción.
  - `Cancel(id)` borra la fila y retorna error si no existe.
- **Worker** (`worker_test.go`):
  - Tick con 2 due rows → 2 filas pasan a `sending`, luego
    `publishOne` se llama 2 veces, ambas terminan `sent` (con fake
    de `MessageSender`).
  - Tick con error de Telegram → fila queda `failed` con
    `error_message` legible; log `TELEGRAM_ERROR`.
  - Tick con permission denied → fila queda `failed` con
    `error_message` legible; log `PERMISSION_DENIED`.
  - Tick con 0 due rows → no cambia estado; log `due=0, claimed=0`.
  - **Integración contra test postgres real**: end-to-end
    `Scheduler.Run` con `interval=5ms` y 1 fila due → después de un
    tick la fila queda `sent` con `message_id`. Valida el SQL exacto
    del claim.
  - `ctx` cancelado durante un tick → `Run` retorna `nil`.
- **Handler** (`publications_handlers_test.go`):
  - POST con `scheduled_at` futuro → 201 con N filas `scheduled`.
  - POST con `scheduled_at` pasado → 400 `VALIDATION_ERROR`.
  - POST con `scheduled_at` y offset (ej. `+03:00`) → la fila se
    persiste con UTC normalizado.
  - DELETE scheduled → 204; fila borrada.
  - DELETE sent → 409 `INVALID_STATUS`; fila intacta.
  - DELETE id inexistente → 404 NOT_FOUND.
  - GET `?limit=10&offset=20` → 200 con slice correcto.
  - GET `?limit=101` → 400 `VALIDATION_ERROR`.
  - GET `?offset=-1` → 400 `VALIDATION_ERROR`.
  - GET `?group_id=X&limit=10&offset=0` → 200 con filtro + paginación
    combinados.

(Previously — slice 2: los tests cubrían SendMessage, SendPhoto,
servicio con sendMessage/Photo OK/error, handler POST 201/400, GET
listado y GET por id, validaciones nuevas, multi-grupo secuencial,
fallo parcial y filtro por grupo. Slice 3 agrega cobertura del worker
(vida, claim, send, error paths, integración), Schedule/Cancel del
servicio, SKIP LOCKED y paginación del repositorio, y
DELETE/paginación del handler.)

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

#### Scenario: Schedule inserta N filas sin llamar Telegram

- GIVEN un servicio con `MessageSender` spy (sin mock configurado
  para publicar)
- WHEN se ejecuta `Schedule({text, group_ids: [g1, g2], scheduled_at:
  "2027-01-01T10:00:00Z"})`
- THEN se crean 2 filas con `status='scheduled'` y `scheduled_at`
  poblado; el spy NO registra ninguna llamada a `Send*`; no se emite
  log `PUBLISH_MESSAGE` (la auditoría es del worker)

#### Scenario: Schedule con fecha pasada retorna error

- GIVEN `scheduled_at = "2020-01-01T00:00:00Z"`
- WHEN se ejecuta `Schedule`
- THEN retorna `ErrScheduledInPast`; no se crea fila

#### Scenario: CancelScheduled borra fila scheduled

- GIVEN una fila con `status='scheduled'` e id=10
- WHEN se ejecuta `CancelScheduled(10)`
- THEN retorna nil; la fila ya no existe en la tabla

#### Scenario: CancelScheduled rechaza fila sent

- GIVEN una fila con `status='sent'` e id=20
- WHEN se ejecuta `CancelScheduled(20)`
- THEN retorna `ErrCancelNotAllowed`; la fila sigue en la tabla con
  `status='sent'`

#### Scenario: Worker tick processa fila due (integration)

- GIVEN test postgres con 1 fila `scheduled` y `scheduled_at < now()`
- WHEN se ejecuta `Scheduler.Run` con `interval=5ms` durante 100ms
- THEN la fila termina `sent` con `message_id` no nulo; el log de
  tick muestra `claimed=1`; el log `PUBLISH_MESSAGE` se emite con
  status SUCCESS

#### Scenario: Claim SKIP LOCKED respeta concurrencia (integration)

- GIVEN 2 filas due en test postgres
- WHEN dos goroutines invocan `ClaimScheduledDue(limit=25)`
  concurrentemente
- THEN cada una recibe exactamente 1 id distinto; no hay duplicados;
  ambas filas quedan `sending`

### Requirement: Tests frontend

Los tests de `PublicationsPage` MUST usar `mockFetchRoutes` (o mock
propio distinguiendo GET/POST/DELETE por `init.method` y query
params) para simular las respuestas de la API. Deben cubrir: carga
exitosa, lista vacía, error de carga, creación exitosa con
invalidación, error al crear con mensaje visible, formulario con foto
+ botones + multi-grupo, filtro por grupo que recarga con
`?group_id=`, **modo "Programar" con datetime-local**, **botón
Cancelar visible solo en `scheduled`**, **paginación Prev/Next** que
cambia la query key.

(Previously — slice 2: los tests cubrían carga, error, creación con
grupo único, foto+botones+multi-grupo y filtro por grupo. Slice 3
agrega cobertura del modo dual, datetime-local, Cancelar y
paginación.)

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

#### Scenario: Modo "Programar" muestra datetime-local

- GIVEN el form en modo default ("Publicar ahora")
- WHEN el admin selecciona el radio "Programar"
- THEN el `<input type="datetime-local">` aparece en el DOM; el
  botón submit cambia su label

#### Scenario: Validación cliente: scheduled_at en el pasado

- GIVEN modo "Programar" activo y una fecha pasada seleccionada
- WHEN el admin intenta enviar el formulario
- THEN se muestra "la fecha debe ser futura" y NO se ejecuta POST

#### Scenario: Cancelar fila scheduled

- GIVEN una fila `scheduled` renderizada con botón "Cancelar"
- WHEN el admin confirma la cancelación
- THEN se ejecuta `DELETE /api/publications/:id` con el id correcto;
  tras 204 la lista se invalida y refetchea

#### Scenario: Cancelar NO se muestra en sent/failed

- GIVEN 3 publicaciones: 1 `scheduled`, 1 `sent`, 1 `failed`
- WHEN se renderiza la lista
- THEN solo la fila `scheduled` tiene el botón "Cancelar"

#### Scenario: Paginación Next deshabilitada al final

- GIVEN una página retornada con menos filas que `limit`
- WHEN se renderiza la lista
- THEN el botón "Next" está deshabilitado; "Prev" depende de
  `offset > 0`

#### Scenario: Paginación Next avanza offset

- GIVEN la lista en `offset=0` con botón "Next" habilitado
- WHEN el admin hace click en "Next"
- THEN se ejecuta GET `/api/publications?limit=50&offset=50`; la
  query key refleja el nuevo offset

### Requirement: README sección publicaciones

El `README.md` MUST incluir una sección "Publicaciones" que documente
los **límites operacionales y de comportamiento** acumulados de los 3
slices:

- **Slice 1**: publicación inmediata de texto a un grupo.
- **Slice 2**: foto por URL pública (Telegram descarga, ≤ 5 MB),
  caption ≤ 1024 cuando hay foto, texto ≤ 4096 sin foto, máximo 10
  grupos por publicación, botones de URL únicamente (sin
  `callback_data` en MVP), máximo 8 filas × 8 botones.
- **Slice 3 (nuevo)**:
  - **Programación**: campo opcional `scheduled_at` (RFC3339 con
    offset, se normaliza a UTC). Fila queda `status='scheduled'` y
    NO se envía al momento de crear. Un worker in-process revisa
    cada 30 segundos y envía las filas cuya `scheduled_at` ya pasó.
    Cancelación solo de filas `scheduled` (DELETE
    `/api/publications/:id`). NO se reintenta automáticamente filas
    `failed`.
  - **Paginación**: `?limit=` (default 50, max 100), `?offset=`
    (default 0). Compatible con `?group_id=`.
  - Variable de entorno `PUBLICATIONS_SCHEDULER_INTERVAL_SECONDS`
    (default `30`) documentada en `.env.example`.
- **Nota explícita**: "Telegram no soporta scheduling nativo desde
  bots; el panel programa in-process con un worker Go (canónico
  `SELECT ... FOR UPDATE SKIP LOCKED`)."

La tabla de "Uso del panel" MUST incluir `/publications`. El MUST
sobre el límite de cancelación ("solo filas `scheduled`") MUST
aparecer visible para disipar preguntas del admin.

(Previously — slice 2: el README documentaba los límites
operacionales de foto, botones, multi-grupo y filtro. Slice 3 agrega
la sub-sección "Programación", la nota sobre scheduling in-process, y
los límites de paginación y cancelación.)

#### Scenario: README incluye programación + paginación

- GIVEN el `README.md` revisado
- WHEN un admin lee la sección Publicaciones
- THEN encuentra los límites acumulados de los 3 slices (5 MB,
  1024, 4096, 10 grupos, 8×8 botones, paginación 50/100, cancelación
  solo de `scheduled`), la nota de que el panel solo publica (no
  recibe callback_data), y la nota explícita de que Telegram no
  soporta scheduling nativo

---

## Slice 3 Extensions — Summary

| REQ | Tipo | Cobertura |
|-----|------|-----------|
| `POST /api/publications` con `scheduled_at` opcional | MODIFIED | 10 escenarios (6 slice 2 + 4 scheduling: futuro, pasado, Z, offset) |
| `GET /api/publications` paginado | MODIFIED | 11 escenarios (4 slice 2 + 7 paginación: limit/offset, bordes, errores) |
| `Worker in-process (Scheduler)` | ADDED | 7 escenarios (tick due, tick vacío, SKIP LOCKED, fallo Telegram, ctx cancel, rate limit, permission denied) |
| `DELETE /api/publications/:id` | ADDED | 5 escenarios (scheduled 204, sent 409, sending 409, 404, race worker) |
| Errores de scheduling y paginación | ADDED | 4 escenarios (pasado, limit fuera de rango, offset negativo, cancel no permitido) |
| Frontend dual-mode + Cancel + Paginación | MODIFIED | 16 escenarios (8 slice 2 + 8 slice 3) |
| Tests backend (worker + service + repo + handler) | MODIFIED | 13 escenarios (3 slice 2 + 10 slice 3) |
| Tests frontend (datetime-local + cancel + pagination) | MODIFIED | 11 escenarios (4 slice 2 + 7 slice 3) |
| README sección publicaciones | MODIFIED | 1 escenario acumulado |

**Totales**: 9 requirements, **78 escenarios** (32 heredados de slice
2 + 46 nuevos de slice 3 — nota: el conteo del delta ascendió a 77
originalmente; aquí 78 tras corregir el conteo del REQ "Errores" de 3
a 4 escenarios en la versión canónica).

---

## Slice 4 ADDED Requirements (publications-batch)

Las siguientes requirements fueron agregadas por el slice 4 — change
`publications-batch` (archivado en
`openspec/changes/archive/2026-09-08-publications-batch/`,
mergeado en este archivo). Las requirements de slices 1, 2 y 3 se
conservan tal cual arriba.

Slice 4 **NO** introduce migración, **NO** agrega método nuevo a
`publications.Service` ni a `publicationStore`, **NO** modifica el
`Scheduler` ni el adapter de Telegram. El handler `POST
/api/publications/batch` reusa `Service.PublishMany` (slice 2) y
`Service.Schedule` (slice 3) vía un loop en el handler, capturando
`now := time.Now()` UNA sola vez. Tabla `publications`, `POST
/api/publications` single y `POST /api/publications` single
permanecen intactos. Bugfix #172 sigue vigente — el handler NO
consulta `bot_permissions["can_*"]` (delegado en `service.permissionOk`
dentro de `PublishMany`/`Schedule`). La verificación estática se
realiza con `TestBatch_NoCanChecksInvariant`.

> **Decisiones técnicas documentadas en este slice**:
> - **Status code del batch**: **200 OK** siempre que la envelope sea
>   válida. 400 SOLO por envelope inválida (validaciones de §18
>   aplicadas fail-fast antes del loop). HTTP 201 NO se usa aunque
>   cada item cree filas (consistente con la convención del proyecto
>   de unificar 200 para batch envelopes).
> - **Cap de batch**: `len(publications) > 10` → 400 `VALIDATION_ERROR`.
>   El cap es por envelope, NO por grupos internos (10 items × 10
>   groups = 100 publicaciones máximo por request).
> - **Failure isolation per-publication**: cada item se dispatcha en
>   aislamiento; un fallo per-item NO aborta el resto.
> - **`now time.Time` consistente**: capturado UNA vez al inicio del
>   handler y pasado a TODAS las llamadas `Schedule(...)`.
> - **`metadata.batch_index`**: convención documentada en
>   `logs/model.go`; correlación por `actor_id + created_at window`
>   (no se persiste en el JSONB explícitamente — decisión deliberada
>   para preservar REQ-20 / non-regression del log emitter).
> - **Frontend UX partial-failure**: dos banners separados
>   (verde `created[]`, rojo `failed[]`); botón "Reintentar fallidas"
>   pre-filtra slots a SOLO los índices en `failed[]`.

### ADDED Requirements

### Requirement: POST /api/publications/batch

El endpoint `POST /api/publications/batch` MUST aceptar
`{publications: [PublicationInput, ...]}` donde cada `PublicationInput`
tiene la misma shape que el body de `POST /api/publications`
(`text`, `photo_url?`, `buttons?`, `group_ids: int64[]`,
`scheduled_at?`). Autenticación requerida (`requireAuth`).

Reglas MUST:

1. `len(publications) == 0` → **400 VALIDATION_ERROR** "se requiere al
   menos una publicación"; NO se procesa nada.
2. `len(publications) > 10` → **400 VALIDATION_ERROR** "máximo 10
   publicaciones por batch"; NO se procesa nada.
3. JSON malformado → **400 VALIDATION_ERROR** "body inválido"; NO se
   procesa nada.
4. Auth ausente → **401 UNAUTHORIZED**.
5. Si la envelope es válida → **200 OK** con
   `{created: [...], failed: [...]}`. Cada item se dispatcha en
   aislamiento; un fallo per-item NO aborta el resto.
6. Para cada item, en orden:
   - Si `scheduled_at` ausente → `service.PublishMany(...)` con el
     payload del item (inmediato).
   - Si `scheduled_at` presente y futuro → `NormalizeScheduledAt` +
     `service.Schedule(..., nowFn)` (programado; el worker reclamará).
   - Si `scheduled_at` presente y pasado → `failed[]` con
     `code='VALIDATION_ERROR'` y mensaje "scheduled_at debe ser una
     fecha futura"; NO se llama Telegram; NO se crea fila.
7. Si `PublishMany`/`Schedule` retorna error a nivel de payload → el
   item va a `failed[]` con `code='VALIDATION_ERROR'`. NO aborta el
   resto del batch.
8. Filas resultantes con `status ∈ {sent, scheduled}` → `created[]`.
   Filas con `status='failed'` (provenientes del service) → `failed[]`
   con `code` mapeado según el origen del fallo.

#### Scenario: Batch vacío → 400

- GIVEN `POST /api/publications/batch` con `{publications: []}`
- WHEN el handler ejecuta la validación de envelope
- THEN responde 400 `VALIDATION_ERROR` "se requiere al menos una
  publicación"; NO se crea fila; NO se llama Telegram

#### Scenario: Batch excede cap de 10 → 400

- GIVEN `POST /api/publications/batch` con 11 items
- WHEN el handler ejecuta la validación de envelope
- THEN responde 400 `VALIDATION_ERROR` "máximo 10 publicaciones por
  batch"; NO se crea fila; NO se llama Telegram

#### Scenario: JSON malformado → 400

- GIVEN `POST /api/publications/batch` con body inválido
- WHEN el handler parsea el body
- THEN responde 400 `VALIDATION_ERROR` "body inválido"

#### Scenario: Auth ausente → 401

- GIVEN el request llega sin JWT válido
- WHEN el middleware `requireAuth` evalúa
- THEN responde 401 `UNAUTHORIZED` sin parsear el body

#### Scenario: Batch all-OK inmediato → 200 con created[]

- GIVEN 2 items válidos (sin `scheduled_at`) y ambos con grupos donde
  el bot es admin
- WHEN se ejecuta el batch
- THEN responde 200 con `created` conteniendo 2 filas con
  `status='sent'` y `message_id` poblado; `failed` vacío; existen 2
  logs `PUBLISH_MESSAGE`

#### Scenario: Batch mixto immediate + scheduled → 200 con ambos paths

- GIVEN 3 items: 1 inmediato, 1 programado a futuro, 1 con
  `scheduled_at` en el pasado
- WHEN se ejecuta el batch
- THEN `created` contiene 2 filas (`sent` + `scheduled`); `failed`
  contiene 1 item con `code='VALIDATION_ERROR'` por el `scheduled_at`
  pasado; el item programado NO se envía (queda fila `scheduled` para
  el worker)

#### Scenario: Batch all-fail → 200 con failed[]

- GIVEN 3 items con grupos inexistentes
- WHEN se ejecuta el batch
- THEN `created` está vacío; `failed` contiene 3 items con
  `code='NOT_FOUND'` y mensaje legible; NO se llama Telegram

#### Scenario: Boundary 10 → 200

- GIVEN 10 items válidos
- WHEN se ejecuta el batch
- THEN responde 200 con 10 entradas en `created`

#### Scenario: Per-item failure isolation → un fallo NO aborta

- GIVEN 3 items: items[0] OK, items[1] grupo inexistente, items[2] OK
- WHEN se ejecuta el batch
- THEN `created` contiene 2 filas (items[0] y items[2] en orden); el
  envío a items[2] ocurre DESPUÉS de evaluar items[1]; `failed`
  contiene 1 item con `index=1`

#### Scenario: Multi-grupo por item → N filas en created[]

- GIVEN 1 item con `group_ids = [g1, g2, g3]` y bot admin en los 3
- WHEN se ejecuta el batch
- THEN `created` contiene 3 filas (una por grupo), cada una con
  `status='sent'` y `message_id` poblado; existen 3 logs
  `PUBLISH_MESSAGE`

#### Scenario: scheduled_at con offset → normalizado a UTC

- GIVEN 1 item con `scheduled_at = "2027-06-15T17:00:00+03:00"`
  (equivale a `14:00:00Z`)
- WHEN se ejecuta el batch
- THEN la fila se inserta con `scheduled_at` normalizado a
  `2027-06-15T14:00:00Z` (UTC); el worker la reclamará en el momento
  UTC correcto

#### Scenario: Fila con status='failed' del service → failed[] mapeado

- GIVEN 1 item con un grupo donde el bot NO es admin
- WHEN se ejecuta el batch
- THEN `failed` contiene 1 item con `code='PERMISSION_DENIED'` y
  mensaje legible; existe log `PUBLISH_MESSAGE` con status
  `PERMISSION_DENIED` para esa fila

### Requirement: Per-item audit log

Cada item exitoso (`created[]`) genera su log `PUBLISH_MESSAGE` con
`metadata` conteniendo `publication_id` (int) y opcionalmente
`message_id` (int). Items programados (`scheduled_at` presente) NO
emiten log inmediato; quedan como fila `scheduled` que el worker
reclamará y registrará su log al procesarlos.

Convención documentada en `logs/model.go`: el campo
`metadata.batch_index` (int) puede usarse para correlación entre
items del mismo batch y sus logs. La convención acepta JSONB o log
adicional — correlación en queries se hace por `actor_id + created_at
window` cuando el índice no se persiste explícitamente.

#### Scenario: Log por item exitoso

- GIVEN un item inmediato con envío exitoso a `message_id=123`
- WHEN el handler completa el batch
- THEN existe un log `PUBLISH_MESSAGE` con `metadata.publication_id`
  poblado y `status='SUCCESS'`

#### Scenario: Items programados NO emiten log inmediato

- GIVEN un item con `scheduled_at` futuro
- WHEN el handler completa el batch
- THEN NO existe log `PUBLISH_MESSAGE` inmediato; la fila queda
  `scheduled` para que el worker la procese

### Requirement: Non-regression del feature base

El feature `publications-batch` MUST preservar sin modificaciones:

1. `POST /api/publications` (endpoint single, REQ-3/REQ-12 slices
   1/2/3).
2. Tabla `publications` (sin nuevas columnas; `scheduled_at`,
   `photo_url`, `buttons` ya viven de slices anteriores).
3. `publications.Scheduler` (worker in-process, slice 3).
4. `publications.Service` (cero método nuevo; `PublishMany` y
   `Schedule` reusados verbatim).
5. `publicationStore` interface (cero método nuevo).
6. `frontend/src/features/publications/*` (helpers de validación
   reusados; el módulo `publications-batch` es NUEVO y separado).
7. `frontend/src/pages/PublicationsPage.tsx` (funcionalmente intacto;
   solo se agregan botón + `<BatchWizard>` mount).

#### Scenario: Single endpoint intacto

- GIVEN el endpoint `POST /api/publications` (single)
- WHEN un admin envía una publicación a un grupo único
- THEN responde 201 con la fila `sent` (NO 200); comportamiento de
  slice 2/3 intacto

#### Scenario: Scheduler intacto

- GIVEN el worker `Scheduler.Run(...)` corriendo con
  `interval=DefaultSchedulerInterval`
- WHEN llega un tick
- THEN `ClaimScheduledDue(25)` se ejecuta como en slice 3; no se
  introducen cambios en el claim pattern

### Requirement: Frontend BatchWizard modal

El módulo `frontend/src/features/publications-batch/` MUST exponer un
modal Mantine v7 (`<Modal>`) llamado `BatchWizard`. Reglas MUST:

1. Se abre desde un botón "Programar en lote" en
   `PublicationsPage` (toolbar del formulario existente).
2. Default: 2 slots editables. Máximo 10 slots (botón "Agregar slot"
   deshabilitado al llegar a 10).
3. Cada slot contiene: Textarea (`text`), input `photo_url` opcional,
   `ButtonsEditor` reusado (multi-fila de botones URL),
   multi-select de grupos (checkboxes desde `useGroups()`),
   `<input type="datetime-local">` opcional (`scheduled_at`).
4. Summary `<Alert>` antes del submit: lista cada slot (índice, grupos
   target, texto truncado, fecha si programada); errores rojos
   inline por slot inválido.
5. Submit deshabilitado si CUALQUIER slot es inválido (texto vacío,
   URL no http(s), 0 grupos seleccionados, `scheduled_at` pasado).
6. Al submit exitoso: `useCreatePublicationBatch` ejecuta
   `POST /api/publications/batch` y al éxito invalida
   `['publications']` (todas las variantes).
7. Estado Result: dos `<Alert>` separados — uno verde con `created[]`
   (uno por fila) y uno rojo con `failed[]` (uno por índice, código,
   mensaje).
8. Botón "Reintentar fallidas" si `failed[]` no vacío: filtra los
   slots a SOLO los índices en `failed[]`, pre-rellena los datos del
   slot original, permite edición, incrementa un contador interno
   `batch_attempt` (debugging), y re-dispara el submit.
9. Botón "Cerrar" siempre presente.
10. El modal NO se auto-cierra tras el submit; el admin decide cuándo
    cerrarlo.

Reuso obligatorio: `formatPublicationsError`,
`validatePhotoUrlClient`, `validateButtonsClient`,
`validateGroupIdsClient`, `validateScheduledAtClient` del módulo
`features/publications/`.

#### Scenario: Modal abre con 2 slots por default

- GIVEN la página `/publications` cargada
- WHEN el admin hace click en "Programar en lote"
- THEN el modal se abre con 2 slots vacíos editables; el botón
  "Agregar slot" está habilitado; "Remover" por slot deshabilitado
  cuando hay 2 slots (no se puede quedar con 1)

#### Scenario: Agregar/remover slot hasta 10

- GIVEN el modal abierto
- WHEN el admin hace click en "Agregar slot" 8 veces
- THEN hay 10 slots; "Agregar slot" queda deshabilitado; los botones
  "Remover" de cada slot están habilitados

#### Scenario: Submit deshabilitado si slot inválido

- GIVEN el modal con 2 slots, uno de ellos con texto vacío
- WHEN el admin intenta enviar el formulario
- THEN el botón submit está deshabilitado; el slot inválido muestra
  mensaje de error inline

#### Scenario: Response all-success → banner verde

- GIVEN la respuesta del backend es `{created: [p1, p2], failed: []}`
- WHEN el modal entra en estado Result
- THEN se muestra UN `<Alert>` verde con ambos items; NO se muestra
  banner rojo; botón "Cerrar" presente; botón "Reintentar fallidas"
  NO aparece

#### Scenario: Response all-fail → banner rojo + retry

- GIVEN la respuesta del backend es `{created: [], failed: [e1, e2]}`
- WHEN el modal entra en estado Result
- THEN se muestra UN `<Alert>` rojo con ambos errores (índice, code,
  message); botón "Reintentar fallidas" presente

#### Scenario: Response parcial → dos banners

- GIVEN la respuesta del backend es `{created: [p1], failed: [e2]}`
- WHEN el modal entra en estado Result
- THEN se muestra `<Alert>` verde con p1 + `<Alert>` rojo con e2;
  botón "Reintentar fallidas" presente; el reintento filtrará a
  SOLO el índice 2

#### Scenario: "Reintentar fallidas" filtra y pre-rellena

- GIVEN 3 slots y `failed = [{index: 0}, {index: 2}]`
- WHEN el admin hace click en "Reintentar fallidas"
- THEN los slots visibles quedan reducidos a 2 (índices 0 y 2
  originales); los datos pre-rellenan desde el slot original; el
  contador `batch_attempt` se incrementa; el submit re-dispara con
  SOLO esos 2 items

#### Scenario: Summary Alert muestra slots y fechas

- GIVEN 2 slots válidos, uno con `scheduled_at` futura
- WHEN se renderiza el modal
- THEN el Summary Alert muestra ambos slots con índice, texto
  truncado, grupo target, fecha si aplica

#### Scenario: Error HTTP del batch → Alert rojo con mensaje formateado

- GIVEN el backend responde 400 con `{code: 'VALIDATION_ERROR',
  message: '...'}`
- WHEN el modal procesa el error
- THEN se muestra `<Alert>` rojo con `formatPublicationsError(...)`
  aplicado al mensaje

### Requirement: Tests backend (batch) y frontend (BatchWizard)

**Backend** (`batch_handlers_test.go`) MUST cubrir (mockeando
`TelegramService` y `publicationStore` — nunca Bot API real; §21.1):

- `TestBatch_CapExceeded`: 11 items → 400 + `VALIDATION_ERROR`.
- `TestBatch_Empty`: `[]` → 400 + mensaje exacto.
- `TestBatch_MalformedJSON`: body inválido → 400.
- `TestBatch_AllSuccess`: 2 items válidos → 200 con 2 entradas en
  `created`, 0 en `failed`.
- `TestBatch_AllFail`: 3 items con grupos inexistentes → 200 con
  `failed[]` conteniendo 3 items con `code='NOT_FOUND'`.
- `TestBatch_MixedImmediateAndScheduled`: 3 items (inmediato,
  programado futuro, programado pasado) → 200 con 2 en `created`, 1
  en `failed`.
- `TestBatch_PerItemFailureIsolation`: 3 items con el del medio
  fallando → el tercero se procesa igualmente.
- `TestBatch_MultiGroupPerItem`: 1 item con `group_ids = [g1, g2]`
  → 2 filas en `created`.
- `TestBatch_RequireAuth`: sin JWT → 401 sin parsear body.
- `TestBatch_CapBoundary`: 10 items → 200 (NO 400).
- `TestBatch_RowFailedPopulatesFailedArray`: item con bot no admin
  → `failed[]` con `code='PERMISSION_DENIED'`.
- `TestBatch_ScheduledWithOffset_NormalizesToUTC`: offset +03:00
  → fila con UTC normalizado.
- `TestBatch_NoCanChecksInvariant`: lectura estática del archivo
  `batch_handlers.go`, asserts ausencia de cada clave `can_*`
  conocida de la Bot API (`can_post_messages`, `can_edit_messages`,
  `can_delete_messages`, `can_manage_chat`, `can_pin_messages`,
  `can_invite_users`, `can_promote_members`, `can_change_info`,
  `can_restrict_members`).

**Frontend** (`BatchWizard.test.tsx`) MUST cubrir (con `mockFetchRoutes`
o mock propio distinguiendo POST):

- "abre con 2 slots por default": estado inicial del modal.
- "agregar slot hasta 10": habilitación/deshabilitación del botón.
- "submit deshabilitado si slot inválido": validación cliente.
- "response all-success → banner verde": render del Result.
- "response all-fail → banner rojo + retry": render + retry button.
- "response parcial → dos banners + retry": render de ambos.
- "Reintentar fallidas filtra y re-envía": filter a `failed[]`.
- "Summary Alert muestra slots y fechas": render del summary.
- "error HTTP del batch → Alert rojo formateado": manejo de error.

Total mínimo: **8 tests backend** + **6 tests frontend** (el spec
exige 8+ y 6+; el branch entrega 15 backend + 9 frontend — supera
el mínimo).

#### Scenario: Servicio batch con Telegram mockeado — all-OK

- GIVEN un handler con fake store que retorna filas `sent` para
  PublishMany
- WHEN se ejecuta `POST /api/publications/batch` con 2 items válidos
- THEN la respuesta es 200 con `created` conteniendo 2 filas y
  `failed` vacío

#### Scenario: Servicio batch con error parcial de Telegram

- GIVEN un handler con fake store donde PublishMany retorna error
  para el primer item y OK para el segundo
- WHEN se ejecuta el batch
- THEN el primer item va a `failed[]` con `code='TELEGRAM_ERROR'`;
  el segundo va a `created[]`; existen 2 logs

#### Scenario: TestBatch_NoCanChecksInvariant pasa

- GIVEN el archivo `batch_handlers.go` cargado en el test
- WHEN el test itera sobre todas las claves `can_*` conocidas
- THEN ninguna clave aparece como token Go (`can_post_messages`,
  etc.) en código ejecutable (los comments sobre la invariante son
  explícitamente exceptuados)

#### Scenario: Test frontend "Reintentar fallidas filtra slots"

- GIVEN el modal con 3 slots y respuesta con `failed=[{index:0},
  {index:2}]`
- WHEN el admin hace click en "Reintentar fallidas"
- THEN la lista visible queda con 2 slots (índices 0 y 2 originales);
  un nuevo submit ejecuta `POST /api/publications/batch` con SOLO 2
  items; la query es observable

### Requirement: Bugfix #172 invariant + §21.1 strict

El handler `handleCreatePublicationBatch` MUST:

1. NO consultar `bot_permissions["can_*"]` ni ningún subcampo de
   permisos de la Bot API.
2. Delegar la verificación per-grupo en `service.permissionOk`
   reusado dentro de `PublishMany`/`Schedule` (source of truth
   bugfix #172: `g.BotStatus == StatusAdministrator`).
3. NO llamar a la Bot API real desde los tests; usar fakes/spies.

#### Scenario: Static guard sin claves `can_*`

- GIVEN el archivo `batch_handlers.go`
- WHEN el test `TestBatch_NoCanChecksInvariant` lo inspecciona
- THEN las claves `can_*` aparecen SOLO en comments que documentan
  la invariante siendo respetada; cero ocurrencias en código
  ejecutable

#### Scenario: Tests sin llamadas a `api.telegram.org`

- GIVEN el suite de tests del paquete `internal/api`
- WHEN se ejecuta `go test ./... -count=1`
- THEN no hay requests salientes a `api.telegram.org` (verificable
  vía httptest con servers locales)

### Requirement: README sección publicación en lote

El `README.md` MUST incluir una sub-sección "Publicación en lote
(publications-batch)" que documente:

- Cap de 10 publicaciones por request.
- Semántica `{created[], failed[]}`: cada item se procesa en
  aislamiento; fallo per-item NO aborta el resto.
- Status code **200 OK** con envelope válida; **400
  VALIDATION_ERROR** SOLO por envelope inválida.
- Comando curl de ejemplo para `POST /api/publications/batch`.
- Botón "Programar en lote" en `/publications`.
- "Reintentar fallidas" en el wizard: pre-filtra y re-envía.
- Items programados (`scheduled_at` presente) son procesados por el
  worker de slice 3 (30s default).

La tabla de "Uso del panel" MUST incluir el botón si existe, o la
ruta `/publications` permanece como entry point.

#### Scenario: README incluye sección batch

- GIVEN el `README.md` revisado
- WHEN un admin lee la sección "Publicación en lote"
- THEN encuentra cap 10, semántica `{created[], failed[]}`, status
  code 200/400, ejemplo curl, y nota sobre el worker de 30s para
  items programados

---

## Slice 4 Extensions — Summary

| REQ | Tipo | Cobertura |
|-----|------|-----------|
| `POST /api/publications/batch` | ADDED | 12 escenarios (env: cap>10, vacío, JSON bad, auth; happy: all-OK, all-fail, mixto sched+now, multi-grupo, boundary, offset UTC, row failed, isolation per-item) |
| Per-item audit log | ADDED | 2 escenarios (log por item exitoso, scheduled sin log inmediato) |
| Non-regression del feature base | ADDED | 2 escenarios (single intacto, Scheduler intacto) |
| Frontend BatchWizard modal | ADDED | 10 escenarios (default 2 slots, add/remove, submit disabled, all-success, all-fail, partial, retry filter, summary, error HTTP, reuso helpers) |
| Tests backend (batch) + frontend (BatchWizard) | ADDED | 4 escenarios (servicio mockeado all-OK, error parcial, static invariant, retry filter) |
| Bugfix #172 invariant + §21.1 strict | ADDED | 2 escenarios (static guard sin `can_*`, tests sin red externa) |
| README sección publicación en lote | ADDED | 1 escenario (incluye cap 10, semántica, status code, curl, worker) |

**Totales**: 7 ADDED requirements, **~33 escenarios** nuevos. Sumado a
los **78 escenarios** de slices 1+2+3 + los escenarios adicionales
del slice 4, el canónico de `publications` cubre **>110 escenarios**
totales. La numeración de requisitos lógicos del feature pasa a
**REQ-1..REQ-26** (15 previos de slices 1+2+3 + 11 nuevos REQ-16..26
según numeración del delta original — nota: el delta lista REQ-16
como "REQ-16 ..26" y la spec canónica agrupa los 7 ADDED en 7
bloques `### Requirement:`; la numeración lógica REQ-N del delta se
preserva en los títulos canónicos vía el sufijo `(REQ-N)` cuando
aplica).

**Invariante de bugfix #172**: el handler `batch_handlers.go` NO
consulta `bot_permissions["can_*"]`. La verificación se realiza con
`TestBatch_NoCanChecksInvariant` (lectura estática del archivo +
asserts). El handler delega la verificación per-grupo en
`service.permissionOk` (source of truth: `g.BotStatus ==
StatusAdministrator`).
