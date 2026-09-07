# Delta Spec for Publications (Slice 3 — Programación + Historial)

> **Change**: `publications-slice3` (Slice 3 de 3). Modo hybrid.
> **Persisted**: `openspec/changes/publications-slice3/specs/publications/spec.md`
> (filesystem) + `sdd/publications-slice3/spec` (Engram).
> **Archive step** sync → `openspec/specs/publications/spec.md`
> (mismo patrón que slice 1 archivado en `2026-09-07-publications/` y
> slice 2 archivado en `2026-09-07-publications-slice2/`).
>
> Este slice **NO** modifica `telegram-moderation`: el worker reutiliza
> los métodos `SendMessage(..., keyboard)` y `SendPhoto(...)` del
> adapter (slice 2, archivado). No se agregan métodos nuevos al
> adapter de Telegram. El único dominio tocado es `publications`.

---

## Decisiones tomadas en este spec (DECIDE)

1. **Spec location**: un único delta en
   `openspec/changes/publications-slice3/specs/publications/spec.md`.
   NO se enmienda `openspec/specs/publications/spec.md` en su lugar
   (eso lo hace `sdd-archive`); NO se crea spec para
   `telegram-moderation` (no hay cambios al adapter este slice).
2. **Patrón de claim del worker** (REQ-3): dentro de una transacción
   explícita: `BEGIN; SELECT ... FOR UPDATE SKIP LOCKED LIMIT 25;
   UPDATE publications SET status='sending', updated_at=now() WHERE id
   IN (los reservados); COMMIT`. Sigue el patrón slice 2
   (combinación SELECT explícito + UpdateStatus in-txn). Tras COMMIT,
   el worker llama `publishOne` por fila **fuera** de la transacción.
   Las filas ya son `sending`; cualquier error de Telegram se persiste
   con `UpdateStatus(sent|failed, messageID|errMsg)` igual que en
   `PublishMany`. Esto evita race con un POST inmediato concurrente
   para la misma fila futura (improbable pero posible).
3. **Timezone** (REQ-8): aceptar cualquier RFC3339 con offset
   (`2006-01-02T15:04:05Z07:00`). El parser MUST normalizar a UTC
   antes de persistir (`t.In(UTC)`); la columna `scheduled_at` ya es
   `TIMESTAMPTZ` (00004:14). El frontend usa
   `<input type="datetime-local">` (zona horaria local del navegador)
   y serializa a RFC3339 con offset antes de POST; el servidor
   normaliza.
4. **Validación `scheduled_at`** (REQ-8): MUST ser **estrictamente
   futuro** (`scheduled_at.After(now)`; tolerancia cero). Igual o
   pasado → 400 `ErrScheduledInPast` con `code='VALIDATION_ERROR'`.
5. **Race worker vs. Cancel** (REQ-7): `Cancel` lee `status` antes
   de borrar (SELECT no-transaccional + DELETE). Si el worker ya marcó
   `sending` en el mismo instante, `Cancel` retorna 409
   `ErrCancelNotAllowed`. Aceptado: probabilidad muy baja en monolito
   de una instancia.
6. **DELETE policy** (REQ-7): hard delete **SOLO** si
   `status='scheduled'`. Otros status → 409
   `ErrCancelNotAllowed` con `code='INVALID_STATUS'`. Preserva audit
   trail de `sent`/`failed`. Sin soft delete, sin enum `cancelled`.
7. **Tests de integración contra Postgres real** (REQ-12):
   OBLIGATORIOS para `Repository.ClaimScheduledDue_BatchSize`,
   `Repository.ClaimScheduledDue_SkipsLockedByAnotherTxn`,
   `Repository.List_LimitOffset_Pagina` y `Repository_Cancel_DeleteRow`.
   El test `TestWorker_TickProcessScheduledDue` corre contra el test
   postgres real (no contra fakes) — valida el SQL exacto del claim,
   la transición `scheduled → sending → sent`, y el manejo del
   `error_message`. Resto (service/handler/frontend) con fakes (estilo
   slice 2; §21.1).
8. **Sin reintroducir checks `can_*`** (REQ-4): el worker reusa
   `publishOne` (slice 2, design #176 / verify #181), que respeta
   bugfix #172 (`g.BotStatus == StatusAdministrator`). Si el bot es
   admin removido antes del tick, la fila queda `failed` con
   `error_message` legible (test cubre).
9. **Errores nuevos** mapeados al envelope HTTP estándar
   (`{code, message}`):
   - `ErrScheduledInPast` → 400 `VALIDATION_ERROR` con mensaje en
     español.
   - `ErrInvalidPagination` → 400 `VALIDATION_ERROR` con mensaje en
     español.
   - `ErrCancelNotAllowed` → 409 `INVALID_STATUS` con mensaje en
     español.

---

## ADDED Requirements

### Requirement: Worker in-process (Scheduler)

El paquete `publications` MUST exponer un `Scheduler` lanzable como
goroutine, con la siguiente firma observable:

```go
type Scheduler struct { /* store, groups, tg, logs, interval, logger */ }

func (s *Scheduler) Run(ctx context.Context) error
```

`Scheduler.Run` MUST ejecutar el siguiente ciclo:

1. Lanzar `time.NewTicker(s.interval)`. Constante
   `DefaultSchedulerInterval = 30 * time.Second` se usa en
   producción. Tests inyectan `interval` bajo (5ms típico) para
   acelerar.
2. En cada tick, invocar `store.ClaimScheduledDue(ctx, 25)` dentro de
   una transacción explícita con el siguiente patrón:
   - `SELECT id, telegram_id, text, photo_url, buttons, scheduled_at
      FROM publications WHERE status='scheduled' AND scheduled_at <=
      now() ORDER BY scheduled_at ASC LIMIT 25 FOR UPDATE SKIP LOCKED`
   - Por cada id reservado, `UPDATE publications SET status='sending',
      updated_at=now() WHERE id=$1` (UpdateStatus reutiliza el helper
     existente; mismo patrón que slice 2).
   - `COMMIT`.
3. Por cada fila devuelta (transición `scheduled → sending`), invocar
   `publishOne(ctx, actorID, telegramID, text, hasPhoto, photoURL,
   buttons)`. El helper ya implementa permissionOk (#172), dispatch
   `SendPhoto`/`SendMessage`, `UpdateStatus(sent|failed, ...)`, y log
   `ActionPublishMessage`.
4. NO reintentar filas `failed` automáticamente. La fila queda
   `failed` con `error_message` legible; el operador crea una nueva
   publicación.
5. Retornar `nil` cuando `ctx.Done()` se dispara (cancelación
   limpia, sin error fatal).
6. Emitir por tick `slog.Info("publications worker tick", "due", N,
   "claimed", M, "interval", s.interval.String())`. Errores del SQL
   no-recuperables se loguean con `slog.Error` (no abortan el loop;
   el siguiente tick reintenta).

`Scheduler.Run` MUST ser lanzado como goroutine desde
`cmd/server/main.go` con el mismo `ctx` que el poller
(`signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)`),
siguiendo el mismo patrón que `telegram.Poller` en main.go:194-202.

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
5. La cancelación MUST ser **hard delete** (sin soft delete, sin
   status `cancelled`). Filas `sent`/`failed` preservan audit trail.
6. La lectura del status ocurre ANTES del DELETE para detectar la
   race con el worker (escenario "Race worker tick vs DELETE
   scheduled").

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

---

## MODIFIED Requirements

### Requirement: POST /api/publications (crear + enviar multi-grupo + scheduling)

El endpoint `POST /api/publications` MUST aceptar
`{text, photo_url?, buttons?, group_ids: int64[], scheduled_at?}` donde
cada `group_id` es `groups.telegram_id`. El campo `scheduled_at` es
opcional; cuando está presente, es RFC3339 con offset (se normaliza a
UTC). La autenticación es requerida (`requireAuth`).

El servicio MUST branchear según la presencia de `scheduled_at`:

- **`scheduled_at == nil`** → comportamiento actual (publicación
  inmediata vía `PublishMany` síncrono, ver slice 2).
- **`scheduled_at != nil && futuro (scheduled_at > now())`** → validar
  payload igual que publish-now (mismas reglas: texto, foto, botones,
  group_ids 1..10). Para cada `group_id`, insertar una fila con
  `status='scheduled'`, `scheduled_at` en UTC, `photo_url`, `buttons`,
  `actor_id`. NO llamar a Telegram. Devolver **201 Created** con
  `{publications: [N Publication, ...]}` (mismo shape que slice 2;
  cada Publication con `status='scheduled'`, `scheduled_at` poblado,
  `message_id=null`).
- **`scheduled_at != nil && scheduled_at <= now()`** → **400
  VALIDATION_ERROR** con `code='VALIDATION_ERROR'` y mensaje "scheduled_at
  debe ser una fecha futura". NO se crea fila; NO se llama Telegram.

Cada fila `scheduled` será procesada por el `Scheduler` (ver
`Worker in-process`). NO se reintenta automáticamente una fila
`failed`. Errores per-grupo en el path inmediato se manejan igual que
slice 2 (fila `failed` con `error_message`, log independiente). Cada
fila es autónoma (sin transacción entre filas).

(Previously — slice 2: el body era
`{text, photo_url?, buttons?, group_ids: int64[]}` y la respuesta
siempre 201 con cada fila en su estado post-envío. Slice 3 agrega
`scheduled_at` opcional; cuando está presente y es futuro, NO se
envía, las filas quedan `scheduled` para que el worker las procese.)

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
- THEN responde 400 con `code='VALIDATION_ERROR'` y mensaje "scheduled_at
  debe ser una fecha futura"; **NO** se crea fila; **NO** se llama
  Telegram

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
  Si `< 1` o `> 100` → **400 VALIDATION_ERROR** con código
  `VALIDATION_ERROR`.
- `offset=<int>` (opcional, **default 0**): desplazamiento desde el
  inicio. Si `< 0` → **400 VALIDATION_ERROR**.

Respuesta: **200** con `{data: [Publication, ...]}` ordenado por
`created_at` DESC, hasta `limit` registros. Cada registro incluye
los mismos campos que slice 2: `id`, `telegram_id`, `text`,
`status`, `message_id`, `error_message`, `actor_id`, `created_at`,
`updated_at`, `photo_url`, `buttons`, **`scheduled_at`** (TIMESTAMPTZ
nullable, presente en filas `scheduled`).

El índice `idx_publications_telegram_id` MUST ser usado por el plan de
query cuando hay filtro `?group_id=`. Sin status filter en MVP.

(Previously — slice 2: el endpoint listaba las 50 más recientes sin
filtro y aceptaba solo `?group_id=`. Slice 3 agrega `?limit=` (default
50, max 100) y `?offset=` (default 0). El cap de 50 deja de ser
hardcoded y pasa a ser el default del query param.)

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
- Cada publicación muestra: texto, foto si `photo_url` (render `<img>`
  o link), botones como chips/labels, grupo, status (badge), fecha, y
  **`scheduled_at` formateado cuando `status='scheduled'`**. Si
  `status='sent'`, mostrar link al `message_id` del chat. Si
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
  `scheduled_at` MUST ser estrictamente futuro (validación cliente
  en `onBlur` del input o al submit; bloquea el envío si está
  pasado). Mensaje de error "la fecha debe ser futura".
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
    Cambio de cualquier parámetro recarga la query.
  - `useCancelPublication()` mutation: ejecuta
    `DELETE /api/publications/:id` y al éxito invalida
    `['publications']` (todas las variantes) con
    `queryClient.invalidateQueries`.

(Previously — slice 2: el form tenía solo textarea + select de grupo
único y el listado mostraba texto/grupo/status/fecha. Slice 2 agregó
foto, editor de botones, multi-select, preview de foto y botones en
el listado, y filtro por grupo con invalidación de query key. Slice
3 agrega modo dual "Publicar ahora" / "Programar", datetime-local,
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
- THEN se muestra "se requiere al menos un grupo" sin enviar el POST

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
    `publishOne` se llama 2 veces, ambas terminan `sent` (con fake de
    `MessageSender`).
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
los **límites operacionales y de comportamiento** acumulados de los
3 slices:

- **Slice 1**: publicación inmediata de texto a un grupo.
- **Slice 2**: foto por URL pública (Telegram descarga, ≤ 5 MB),
  caption ≤ 1024 cuando hay foto, texto ≤ 4096 sin foto, máximo 10
  grupos por publicación, botones de URL únicamente (sin
  `callback_data` en MVP), máximo 8 filas × 8 botones.
- **Slice 3 (nuevo)**:
  - **Programación**: campo opcional `scheduled_at` (RFC3339 con
    offset, se normaliza a UTC). Fila queda `status='scheduled'` y
    NO se envía al momento de crear. Un worker in-process revisa
    cada 30 segundos y envía las filas cuya `scheduled_at` ya
    pasó. Cancelación solo de filas `scheduled` (DELETE
    `/api/publications/:id`). NO se reintenta automáticamente filas
    `failed`.
  - **Paginación**: `?limit=` (default 50, max 100), `?offset=`
    (default 0). Compatible con `?group_id=`.
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
| Errores de scheduling y paginación | ADDED | 3 escenarios (pasado, limit fuera de rango, cancel no permitido) |
| Frontend dual-mode + Cancel + Paginación | MODIFIED | 16 escenarios (8 slice 2 + 8 slice 3) |
| Tests backend (worker + service + repo + handler) | MODIFIED | 13 escenarios (3 slice 2 + 10 slice 3) |
| Tests frontend (datetime-local + cancel + pagination) | MODIFIED | 11 escenarios (4 slice 2 + 7 slice 3) |
| README sección publicaciones | MODIFIED | 1 escenario acumulado |

**Totales**: 9 requirements, **77 escenarios** (32 heredados de slice
2 + 45 nuevos de slice 3).
