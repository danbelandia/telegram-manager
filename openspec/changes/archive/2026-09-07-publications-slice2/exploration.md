# Exploration: Publications Slice 2 — Foto por URL + Botones InlineKeyboard + Multi-Grupo + Historial con Filtro

> Change: `publications` (Slice 2 de 3). Modo hybrid. Slice 1 archivado en
> `openspec/changes/archive/2026-09-07-publications/`. Este archivo vive en
> `openspec/changes/publications-slice2/` para NO pisar la carpeta ya
> archivada del slice 1 y mantener la convencion por-slice del repo.

## Contexto heredado (base obligatoria del decision-making)

Slice 1 (archivado, `main @ ace1f59`) dejo: tabla `publications`
(`00004_create_publications.sql`), modulo `internal/publications`
(model/repository/service), `telegram.SendMessage`, handlers
`POST|GET /api/publications`, pagina `/publications`. Los endpoints
actuales:

- `POST /api/publications` acepta `{text, group_id}` (un grupo) →
  valida texto (≤4096) → grupo existe (404) → `permissionOk` (403) →
  `create(sending)` → `SendMessage` → `update(sent|failed)` → log.
- `GET /api/publications` → lista 50 mas recientes (created_at DESC), sin
  filtro.
- `GET /api/publications/:id` → detalle.

**Bugfix can_manage_chat (topic `sdd/publications/permission-check`)**:
`permissionOk` AHORA usa `group.BotStatus == groups.StatusAdministrator`
(COMMIT 9d4f1ec). El design original exigia `bot_permissions["can_manage_chat"]`,
pero la deteccion de grupos (events.go `permissionsFromMember`) jamas puebla
esa clave. **Regla para este slice: NO reintroducir checks sobre claves
`can_*` para permisos de publicacion; usar SIEMPRE `bot_status == administrator`.**
Esto aplica igual para sendPhoto: la Bot API no pide una `can_*` especifica
para enviar fotos a un grupo siendo admin; el estado de admin exime de las
restricciones de envio del grupo.

**Decisiones de slice 1 que el slice 2 conserva (no re-abrir):**
- Tabla unica `publications` con status `draft|scheduled|sending|sent|failed`.
- `telegram_id` FK a `groups.telegram_id`.
- Fakes escritos a mano (no moq).
- 404 antes de 403 (order).
- Status `failed` (no dejarlo `sending`) ante error.

## Current State (patrones de codigo real a seguir en el design)

### Backend adapter
- `telegram/publications.go`: pattern params struct privado + `doWithRetry`
  → `doPost` → `handleEnvelope`. `sendMessageParams` usa `omitempty`.
- `adapter.go`: `doPost` hace JSON POST, aplica token bucket (~25 req/s,
  §18.1) y decodifica el envelope; `doWithRetry` reintenta 429 hasta 3.
- `telegram.Service` (service.go:60) ya tiene `SendMessage`. Este slice
  agrega `SendPhoto` y un tipo publico para el inline keyboard.

### Backend service
- `publications/service.go`: `Service` con 4 interfaces (GroupReader,
  MessageSender, LogWriter, PubStore). `Publish(ctx, actorID, groupID,
  text)` es sincrono para UN grupo. Aqui se agrega la logica multi-grupo.

### API
- `api/server.go`: `WithPublications(pubs publicationStore)` registra
  `POST/GET /api/publications` y `GET /api/publications/{id}`.
- `api/publications_handlers.go`: `createPublicationRequest{Text, GroupID}`;
  `publicationResponse` (sin photo/buttons hoy); `respondPublicationError`.

### Migraciones
- Convencion `00004_*.sql` con `-- +goose Up` / `-- +goose Down`. Este
  slice = `00005_alter_publications.sql` (ALTER, no CREATE).

### Frontend
- `features/publications/{types,api,hooks,error}.ts`: `Publication`,
  `CreatePublicationInput{text, group_id}`, `createPublication`,
  `listPublications`, `getPublication`, `useCreatePublication` (invalida
  `['publications']`).
- `pages/PublicationsPage.tsx`: tabla listado (texto · grupo · estado ·
  fecha) + form (select grupo + textarea). Slice 2 agrega foto, botones y
  multi-select.
- `features/groups/hooks.ts`: `useGroups()` → lista para el selector
  multi-grupo.
- `test/helpers.tsx`: `mockFetchRoutes` (dispatch por substring de URL,
  NO por orden) + `okJson`/`errorJson`. Tests de GET vs POST mismos path
  usan un mock propio de fetch distinguiendo `init.method`.
- `App.tsx`: ruta `/publications` ya registrada dentro de `RequireAuth`.
  No requiere cambios de ruta en este slice.

### README
- `README.md` NO documenta publicaciones todavia (linea 6-10 dice que
  fases 2-4 estan "pendientes"). Slice 2 debe agregar una seccion de
  publicaciones real y actualizar la tabla de uso del panel
  (añadir `/publications`).

## Datos confirmados de la Bot API (con cita)

Verificado contra `https://core.telegram.org/bots/api` (docs oficial,
Bot API current):

**sendPhoto** (`#sendphoto`):
- Parametro `photo`: *"Pass a file_id as String to send a photo that
  exists on the Telegram servers (recommended), pass an HTTP URL as a
  String for Telegram to get a photo from the Internet, or upload a new
  photo using multipart/form-data. The photo must be at most 10 MB in
  size. The photo's width and height must not exceed 10000 in total.
  Width and height ratio must be at most 20."* → **10 MB max** para el
  campo `photo`, y restricciones de dimension (ancho+alto ≤10000, ratio
  ≤20).
- Parametro `caption`: *"Photo caption (may also be used when resending
  photos by file_id), 0-1024 characters after entities parsing"* →
  **caption limit 1024**.
- Parametro `reply_markup`: InlineKeyboardMarkup (o ReplyKeyboardMarkup /
  ReplyKeyboardRemove / ForceReply).
- Devuelve el `Message` enviado (contiene `message_id`).

**"Sending files" (limite por HTTP URL)**: *"Provide Telegram with an HTTP
  URL for the file to be sent. Telegram will download and send the file.
  5 MB max size for photos and 20 MB max for other types of content."*
→ **Nota importante**: cuando se envia la foto por **HTTP URL** (nuestro
caso), el limite real es **5 MB**, no los 10 MB del campo `photo`.
Ademas, la URL debe ser **publica y accesible desde internet** para que
Telegram la descargue. Documentar esto en el design como limitacion
operacional (sin endpoint de upload en el MVP).

**InlineKeyboardMarkup** (`#inlinekeyboardmarkup`):
- Campo `inline_keyboard`: *"Array of button rows, each represented by an
  Array of InlineKeyboardButton objects."*

**InlineKeyboardButton** (`#inlinekeyboardbutton`):
- *"Exactly one of the fields other than text, icon_custom_emoji_id, and
  style must be used to specify the type of the button."*
- `text`: *"Label text on the button"* (obligatorio).
- `url`: *"HTTP or tg:// URL to be opened when the button is pressed."*
  → botones de URL: suficientes para el MVP.
- `callback_data`: *"Data to be sent in a callback query to the bot when
  the button is pressed, 1-64 bytes."* → limit **1-64 bytes**; y usar
  callback_data implica recibir y responder `callback_query` updates
  (logica de bot dentro del chat), fuera del alcance de un panel que
  solo publica.

**Limites de cantidad de botones/rows**: La documentacion oficial de la
Bots API no expone un limite numerico explicito en el snippet vigente de
`InlineKeyboardMarkup`/`InlineKeyboardButton`. El limite historico
comunmente documentado por la comunidad es de max **~100 botones totales**
y **~8 filas**. Como NO lo confirme en la fuente oficial, el design debe
imponer un limite propio de validacion (MUST) conservador y razonable —
p. ej. **max 8 filas x 8 botones = 64 botones** — y dejarlo documentado
como eleccion de producto, no como constraint de la Bot API.

**Facts relevantes heredados (no re-verificados, ya en la referencia del
repo)**: `sendMessage` y `sendPhoto` comparten `reply_markup`; el rate
limiter global impone envio secuencial multi-grupo (§18.1, nunca paralelo).

## Alcance Slice 2 (definido con decisiones y tradeoffs)

### D1 — foto por URL: columna `photo_url TEXT NULL`

**Decision**: agregar `photo_url` nullable a `publications` (ALTER 00005).
Con `text` obligatorio existente; la foto es opcional.

**Alternativas**:
- Tabla separada `publication_media`: rechazado (sobre-complica un esquema
  que con una columna nullable basta; solo habria 1 media por publicacion
  en el MVP; se puede revisar en slice 3 si se quiere sendMediaGroup).
- Colocar la foto como JSONB dentro de un blob de contenido: no aporta
  sobre una columna tipada TEXT.

**Validacion** (servicio): si `photo_url` no es vacio, debe:
1. Parsear como URL y exigir esquema `http` o `https`.
2. Aplicar longitud maxima (URLs largas, p. ej. ≤ 2000 chars).
No validar que la URL exista ni el tamano real (Telegram lo valida y
devuelve error → se mapea a TELEGRAM_ERROR). Documentar: si la URL no es
publica/accesible o la imagen supera 5 MB, Telegram rechaza y la
publicacion queda `failed`.

### D2 — foto + texto: UNA llamada `sendPhoto` con `caption`

**Decision**: cuando hay foto, UNA sola llamada `sendPhoto(photo_url,
caption=text, reply_markup=buttons)`; el texto va como caption. Cuando NO
hay foto, se mantiene `sendMessage(text, reply_markup)`.

**Alternativa**: `sendPhoto` + `sendMessage` en dos llamadas: rechazado
(2 requests por grupo, 2 message_id, mas riesgo de orden y 2 filas por
pensar; el caption es el mecanismo nativo de Telegram para texto+media).

**Consecuencia**: con foto, el limite de texto pasa a ser **1024**
(caption de `sendPhoto`), NO 4096. Validar en el servicio: si hay
`photo_url`, aplicar limite de caption 1024; si no, el de 4096. El frontend
debe reflejar este max dinamico.

### D3 — botones: UNA columna `buttons JSONB NULL`

**Decision**: agregar `buttons JSONB NULL` a `publications` con la forma
`[ [{text,url}, {text,url}], [{text,url}] ]` = array de filas, cada fila
array de `{text, url}`. Serializar a `InlineKeyboardMarkup.inline_keyboard`
en el adapter.

**Alternativas**:
- Tabla `publication_buttons` + `publication_button_rows`: rechazado por
  sobre-ingenieria para un MVP; la estructura JSONB es estable y legible;
  si luego se quiere soporte de edicion/callback_data, se migra.
- JSONB de una sola lista plana `[{text,url}]`: no permite filas múltiples
  (agrupar botones en la misma fila es un requerimiento de UX real).

**callback_data NO se implementa** (decision documentada): el panel solo
PUBLICA en el chat; `callback_data` solo tiene sentido como comportamiento
dentro del chat (el bot debe recibir `callback_query`, validarla y
responder con `answerCallbackQuery`). Eso es una feature de robotica
in-chat, distinta de "crear una publicacion"; está fuera del MVP y se
registra como consideracion futura (slice 3/fase 4 de automatizaciones).
**Minimo viable: botones de URL solamente.**

**Validacion** (servicio):
- Max 8 filas, max 8 botones por fila (64 total) — limite propio
  documentado (la Bot API no lo fija explicitamente; ver facts).
- Cada boton: `text` no vacio, ≤ 64 chars; `url` http(s) valida, non
  vacia.
- Si `buttons` viene presente pero vacio ([]), tratarlo como NULL (sin
  teclado).

### D4 — multi-grupo: `group_ids []int64` → UNA fila por grupo, envio secuencial

**Decision**: `POST /api/publications` acepta `{text, photo_url, buttons,
group_ids: []int64}` (reemplaza `group_id`). Por cada `group_id` valido se
CREA **una fila** `publications` (mismo contenido, distinto `telegram_id`).
El envio se hace **secuencialmente** (Nunca paralelo, §18.1: el token
bucket del adapter ya limita; una goroutine por grupo romperia el orden y
saturaria el bucket).

**Respuesta**: 201 con `{publications: []Publication}` (la lista de filas
creadas, cada una con su propio status sent|failed y message_id/error).

**Flujo por grupo** (reutiliza la logica existente de Publish):
configurar por grupo: validar grupo existe (404) y `permissionOk` (403).

**Decisiones de forma:**
- Limite `len(group_ids)` **max 10** (400 VALIDATION_ERROR si excede).
  Evita bloquear el request HTTP demasiado (10 envios secuenciales ≈ hasta
  10s+ por posible 429). Slice 1 ya advirtio el riesgo de bloquear con N
  envios; con max 10 se mantiene razonable.
- **Fallo parcial**: si un grupo falla (404/403/Telegram), NO abortar los
  demas. Cada grupo se procesa independientemente; la respuesta 201 devuelve
  todas las filas con su status individual. El admin ve que una fallo y las
  otras no. No hay transaccion entre grupos (cada fila es autonoma).
- Validaciones comunes (texto, photo_url, buttons) se hacen UNA sola vez
  ANTES de iterar grupos (fail-fast 400).
- **Orden de iteracion**: se preserva el orden de `group_ids` recibido
  (= orden en que se enviaran a Telegram). Test lo verifica.

**Nota de sincronia vs async**: se mantiene sincrono (como slice 1) con
la cota de 10. Si en el futuro un batching >10 o publicaciones muy pesadas
piden async+polling, es slice 3/historial (fuera de alcance).

### D5 — GET /api/publications: filtro por `group_id` + mantener limite 50

**Decision**: `GET /api/publications?group_id=<int64>` filtra por
`telegram_id`. Sin filtro → 50 mas recientes (comportamiento hoy). Con
filtro → mismos 50, filtrados por grupo. **No agregar paginacion** en este
slice (decisión: mantener 50; si crece la necesidad, slice 3/historial
agrega offset/limit o paginacion real). El índice `idx_publications_telegram_id`
ya existe (00004), asi que el filtro es eficiente.

### D6 — migracion 00005_alter_publications.sql

**Decision**: goose Up/Down:
```sql
-- +goose Up
ALTER TABLE publications
    ADD COLUMN photo_url TEXT,
    ADD COLUMN buttons   JSONB;
-- +goose Down
ALTER TABLE publications
    DROP COLUMN photo_url,
    DROP COLUMN buttons;
```
Ambas NULL-ables, sin backfill (datos de slice 1 existentes quedan con
photo_url NULL / buttons NULL = publicacion de texto). Columnas null
existentes no rompen scan si el scanner maneja NULL.

### D7 — Backfill/refresh de grupos: NINGUNO

**Decision**: no hay cambio de esquema en `groups`; el multi-select usa
`GET /api/groups` (existe). El check de `permissionOk` por grupo requiere
cargar cada grupo via `GetByTelegramID`. No se necesita refrescar
nada de grupos en este slice.

### D8 — telegram adapter: `SendPhoto` + tipo publico de inline keyboard

**Decision**: agregar a `telegram.Service`:
```go
SendPhoto(ctx, chatID int64, photoURL, caption string, keyboard *InlineKeyboardMarkup) (int64, error)
```
Donde `InlineKeyboardMarkup`/`InlineKeyboardButton` son tipos publicos del
paquete telegram:
```go
type InlineKeyboardButton struct {
    Text string `json:"text"`
    URL  string `json:"url"`
}
type InlineKeyboardMarkup struct {
    InlineKeyboard [][]InlineKeyboardButton `json:"inline_keyboard"`
}
```
`SendMessage` se extiende (o se agrega variante) para aceptar el keyboard
nullable; para mantener compat, `SendMessage` mantiene su firma y se agrega
`ReplyMarkup` pasandolo dentro del params si no es nil (con `omitempty` no
se puede usar nil struct; usar puntero a `InlineKeyboardMarkup`, omit si nil).

**Nota getMessagesKey**: `sendPhoto` devuelve `Message{message_id}`; se
reusa el patron `sendMessageResult{MessageID}` o un struct compartido.

### D9 — frontend: extender form y listado

**Decision**:
- Form: input `photo_url` (texto, opcional, validacion http/https del
  lado del usuario con `URL` + mensaje), editor de botones (filas de
  `{text, url}` con botón para agregar/quitar fila y boton por fila para
  agregar/quitar boton), selector **multi-grupo** (checkboxes sobre
  `useGroups()`), textarea `text` (max dinamico 4096 o 1024 si hay foto).
  Validacion zod o manual: al menos un grupo, texto no vacio, botones
  validados (text≤64, url http(s)).
- `types.ts`: `CreatePublicationInput{text, photo_url?, buttons?,
  group_ids: number[]}`; `Publication` extiende con `photo_url`, `buttons`
  (verbatim JSON) para el listado.
- `api.ts`: `createPublication` → `POST` con `group_ids`; `listPublications`
  acepta `filter?: {group_id?: number}` → query param.
- `hooks.ts`: `usePublications(filter)`; key `['publications', group_id]`.
- Listado: muestra foto (imagen `<img>` si photo_url), botones (preview de
  chips/labels), status, y un **filtro por grupo** (select que recarga con
  el query param). `group_id` invalida la query.
- `error.ts`: mensajes nuevos para validacion de URL/foto.

### D10 — testing

**Backend (unit, con TelegramService mockeado — nunca Bot API real):**
- adapter: `SendPhoto` decodifica message_id; serializa `reply_markup`
  correctamente (assert del JSON de `inline_keyboard`); 429 retry;
  `SendMessage` con/ sin keyboard.
- serializacion del inline keyboard (filas correctas, omit cuando nil).
- service: multi-grupo envio SECUENCIAL (mismo orden que group_ids, verificar
  con fake que registra orden), fallo parcial (uno falla, resto OK),
  fail-fast de validaciones comunes, photo caption limit 1024 vs text 4096,
  validacion de photo_url (http/https, long), validacion de buttons
  (límites), permiso por grupo con `bot_status`.
- handler: POST 201 con array de filas, POST 400 (sin grupos, >10, url
  invalida, caption muy largo), GET filtro por group_id, fallo parcial en
  la respuesta.

**Frontend:** extender `PublicationsPage.test.tsx` con `mockFetchRoutes`
(o mock propio para distinguir GET/POST): crear con foto+grupos multiples,
error de URL invalida, filtro por grupo (re-fetch con query param), preview
de foto/botones. Reutilizar patron de `mockFetchRoutes`.

### D11 — README

**Decision**: Slice 2 incluye actualizar `README.md`: agregar seccion de
publicaciones (que sea (multi-grupo, foto por URL, botones de URL), y
limites operacionales (URL publica, 5 MB por URL, caption 1024 con foto),
y añadir `/publications` a la tabla de "Uso del panel". Es parte del alcance
del slice (se entrega con la feature).

## Telegram capability fuera de alcance (para documentar, NO simular)

- **sendMediaGroup** (albumes de foto): NO se implementa en el MVP ni en
  este slice (una sola foto por publicacion). Si se quiere, seria slice 3+
  y requeriria otra columna/tabla y otra estructura de datos.
- **Botones con callback_data**: NO se implementan (ver D3). Es un
  comportamiento de bot-in-chat (recibir `callback_query`, responder
  `answerCallbackQuery`) que el panel no puede ni debe simular. Documentar
  como consideracion futura.
- **Upload propio de imagenes (multipart a Telegram)**: NO; el MVP usa URL
  publica unicamente. Limite de 5 MB por URL (vs 10 MB si se sube el
  archivo directo). No hay endpoint de upload en el backend.
- **Enviar a un grupo sin ser admin**: la Bot API lo rechaza con 400/403
  por restricciones del chat o por falta de derechazo; el service ya lo
  pre-valida via `bot_status`.

## Affected Areas (archivos que el design cubrira)

Backend:
- `backend/internal/telegram/telegram_service` (service.go) — interfaz `Service`: `SendPhoto`, tipos publicos `InlineKeyboardMarkup`/`InlineKeyboardButton`.
- `backend/internal/telegram/adapter.go` o `publications.go` — impl `SendPhoto` + keyboard en `sendMessage`/`sendPhoto` params.
- `backend/internal/telegram/publications_test.go` — tests adapter (nuevo/ampliado).
- `backend/internal/publications/model.go` — `Publication.PhotoURL`, `Publication.Buttons`, `ErrPhotoURLInvalid`, `ErrButtonsInvalid`, limites.
- `backend/internal/publications/repository.go` — `Create`/`scanPublication`/`List(filter)` incluyen `photo_url`/`buttons`; `ListByGroupID` o `List(filter)`.
- `backend/internal/publications/service.go` — `PublishMany` (multi-grupo secuencial), rechazar `photo_url`/`buttons`/texto caption, `SendPhoto` vs `SendMessage` segun haya foto.
- `backend/internal/api/publications_handlers.go` — `createPublicationRequest{text, photo_url, buttons, group_ids}`, respuesta `{publications}`, `GET ?group_id=`.
- `backend/internal/api/server.go` — `WithPublications` (poco cambio; ruta misma, query param).
- `backend/migrations/00005_alter_publications.sql` — `ALTER` Up/Down.

Frontend:
- `frontend/src/features/publications/types.ts` — `CreatePublicationInput`, `Publication` ampliada.
- `frontend/src/features/publications/api.ts` — `createPublication` multi-grupo, `listPublications(filter)`.
- `frontend/src/features/publications/hooks.ts` — `usePublications(filter)`.
- `frontend/src/features/publications/error.ts` — mensajes nuevos.
- `frontend/src/pages/PublicationsPage.tsx` — form (foto, botones, multi-select), listado (preview + filtro).
- `frontend/src/pages/PublicationsPage.test.tsx` — tests ampliados.

Docs:
- `README.md` — seccion publicaciones + tabla panel.

## Ready for Proposal

**Si.** Informar al usuario:

> Slice 2 (publicaciones con foto, botones y multi-grupo):
> - **Foto por URL** via una sola llamada `sendPhoto` con el texto como
>   `caption` (captions limitados a 1024 cuando hay foto, 5 MB max por URL).
> - **Botones inline de URL** (sin callback_data: el panel solo publica, no
>   es bot-in-chat; se reserva para futuro).
> - **Multi-grupo**: `POST /api/publications {group_ids:[...max 10]}` crea
>   una fila por grupo y envia SECUENCIAL (nunca paralelo, §18.1), con
>   fallo parcial.
> - Migracion 00005 (photo_url + buttons), filtro por grupo en el listado,
>   README actualizado.
>
> ¿Arrancamos la propuesta/spec del Slice 2?
