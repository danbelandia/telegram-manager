# Publications Specification (Slice 1 — Publish Now, Text)

## Purpose

Publicaciones inmediatas de texto a un grupo de Telegram (AGENTS §22).
El admin crea una publicación y el bot la envía sincrónicamente al
grupo seleccionado, registrando la acción en logs. La tabla y el
módulo se diseñan para extenderse en slices posteriores (media,
programación).

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
3. Verificar que `groups.bot_permissions["can_manage_chat"]` sea true
   (403 PERMISSION_DENIED si no lo es) — permiso mínimo que indica
   que el bot es administrador con capacidad de publicar.
4. Insertar fila con status `sending`.
5. Llamar `telegram.SendMessage`.
6. En éxito: actualizar status a `sent`, guardar `message_id`.
7. En fallo: actualizar status a `failed`, guardar `error_message`.
8. Registrar log con `ActionPublishMessage` en ambos casos.
9. Devolver 201 con la fila de publicación.

#### Scenario: Publicación exitosa

- GIVEN un admin autenticado, texto válido y un grupo con el bot como
  admin (`can_manage_chat = true`)
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

- GIVEN el grupo existe pero `can_manage_chat = false`
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
