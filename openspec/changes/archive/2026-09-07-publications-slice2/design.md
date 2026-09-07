# Design: Publications — Slice 2 (Foto URL + Botones + Multi-Grupo + Filtro)

> **Change**: `publications` (Slice 2 de 3). Persisted as `sdd/publications-slice2/design` (hybrid mode).
> **Strategy**: single-pr. Estimación: ~600 LOC touched → dentro del budget de 400 líneas PR-reviewable si tasks se aplica como un solo PR acotado (ver Risks).

## Technical Approach

Sobre la tabla única de slice 1 (`publications`, migración 00004) se agregan dos columnas (`photo_url TEXT`, `buttons JSONB`) vía migración `00005`. El adapter extiende la interfaz `telegram.Service` con `SendPhoto(ctx, chatID, photoURL, caption, keyboard *InlineKeyboardMarkup)` y modifica `SendMessage` para aceptar un keyboard opcional. El servicio `publications` agrega `PublishMany(ctx, actorID, payload)` que itera secuencialmente sobre `group_ids` (sin goroutines paralelas — §18.1) llamando a `SendPhoto` o `SendMessage` según haya foto. La API expone el nuevo body y agrega filtro `?group_id=` al listado. El frontend extiende el formulario con un editor de botones y multi-select de grupos. Se respeta bugfix #172: el check de admin usa `g.BotStatus == StatusAdministrator`, sin reintroducir claves `can_*`.

## Architecture Decisions

| # | Decisión | Alternativa | Rationale |
|---|----------|-------------|-----------|
| D1 | `SendMessage(ctx, chatID, text, disableWebPagePreview, keyboard *InlineKeyboardMarkup)` (firma extendida) + `SendPhoto(ctx, chatID, photoURL, caption, keyboard *InlineKeyboardMarkup)` (método paralelo) | `Send(ctx, chatID, opts SendOptions)` unificado con struct | Spec DECIDE: extender `SendMessage` con keyboard + método paralelo `SendPhoto`. Patrón narrow consistente con `Ban/Mute/Delete`. Costo: toda fake (`poller_test.go fakeService`) debe agregar `SendPhoto` y la nueva firma de `SendMessage` — flag explícito en tasks. |
| D2 | Botones almacenados como JSONB `[[{text,url},...],...]` (un array por fila, filas en array externo) | Tabla `publication_buttons` | Spec: una sola fila por publicación × grupo, sin joins; límite 8×8 acotado. Validación fail-fast en servicio. |
| D3 | Foto + texto en una sola llamada `sendPhoto(caption=text, reply_markup)` | `sendPhoto` + `sendMessage` separada | Spec DECIDE: caption de `sendPhoto` cubre el texto cuando hay foto (≤ 1024). Nunca se hacen dos llamadas (rompe el orden y duplica rate limit). |
| D4 | `PublishMany` SECUENCIAL sobre `group_ids` (loop simple, sin goroutines) | Worker pool / goroutines por grupo | Spec + §18.1: el token bucket global ordena; 10 grupos × ~200ms ≈ 2s sync aceptable; paralelizar complica fallo parcial ordenado y logs. |
| D5 | Validación de payload FAIL-FAST antes de iterar; errores per-grupo (404/403/telegram) NO abortan | Validar por cada grupo | Spec: 400 limpio para payload; 201 con `status: failed` por fila cuando Telegram/permisos fallan. El admin ve el resultado per-row sin reintentar el resto. |
| D6 | HTTP 201 incluso si TODAS las filas quedan `failed` (no hay error de payload) | 207 Multi-Status | Spec DECIDE: 207 es WebDAV-específico, agrega complejidad sin reducir client work (cada fila ya carga su status). 400/404/403 a nivel payload siguen siendo los códigos de fallo. |
| D7 | Service `Publish` (slice 1, single-group) QUEDA — `PublishMany` es adicional, no reemplazo | Refactor a `PublishMany` | Spec mantiene compatibilidad: el handler slice 1 sigue llamando `Publish` (un único group_id). `PublishMany` lo envuelve cuando hay 1 grupo o llama internamente a `Publish`. Decisión: **`PublishMany` internamente llama a `Publish` para 1 grupo; loop directo para N** (mantiene UN solo path de envío y un único set de mocks de prueba). |
| D8 | `PubStore` agrega `ListByTelegramID(ctx, telegramID)` (filtro opcional); `List` sin cambios de firma | Single `List(ctx, filter)` | Coherente con `idx_publications_telegram_id` ya existente; la rama sin filtro usa `List()` actual. |
| D9 | Frontend query key `['publications', group_id ?? 'all']`; multi-select es checkboxes (no select múltiple nativo HTML) | Select múltiple | UX consistente con `useGroups()` (mismo set); checkboxes son explícitos sobre qué se publica. |
| D10 | Límites: `text ≤ 4096` sin foto / `≤ 1024` con foto; `photo_url ≤ 2048 chars` http(s); botones `8×8`, `text ≤ 64`, `url ≤ 256` http(s); `group_ids 1..10` | Sin límites | Spec: límites autoimpuestos (Bot API no acota `inline_keyboard`; `caption` ≤ 1024 oficial; `text` ≤ 4096 oficial; `photo_url` ≤ 2048 es razonable para URLs públicas). |

## Data Flow

```
POST /api/publications {text, photo_url?, buttons?, group_ids[]}
   │
   ▼
handler.handleCreatePublication (decode body, extract actorID)
   │
   ▼
publications.PublishMany(ctx, actorID, payload)
   │
   ├─► validatePayload(text, photo_url, buttons, group_ids)   ── fail-fast 400
   │
   └─► for each group_id (sequencial):
         │
         ├─► groups.GetByTelegramID ─── 404 → fila failed + log
         ├─► permissionOk (BotStatus==administrator)  ─── sino → fila failed + log PERMISSION_DENIED
         ├─► store.Create(fila status=sending, photo_url, buttons)
         │
         └─► if photo_url:
                telegram.SendPhoto(chatID, photo_url, text, keyboard)
             else:
                telegram.SendMessage(chatID, text, false, keyboard)
                    │
                    ├─► doWithRetry (429 → retry_after, max 3)
                    ├─► token bucket wait
                    ├─► doPost("sendPhoto"|"sendMessage", params, &result)
                    │       └─► handleEnvelope → Err* mapeo
                    │
                    └─► update fila (sent+message_id | failed+error_message)
                           └─► logs.Create(PUBLISH_MESSAGE, status, metadata{publication_id, message_id?})
   │
   ▼
201 Created { publications: [Publication, ...] }   (mismo orden que group_ids)
```

```
GET /api/publications?group_id=<int64>
   │
   ▼
handler.handleListPublications (parse query → ?group_id)
   │
   ▼
publications.List(ctx)               sin filtro
publications.ListByTelegramID(ctx, id) con filtro  (índice idx_publications_telegram_id)
   │
   ▼
200 OK [Publication, ...]   (incluye photo_url, buttons)
```

## File Changes

| Archivo | Acción | Descripción | LOC est. |
|---------|--------|-------------|----------|
| `backend/migrations/00005_alter_publications.sql` | Crear | `ALTER ADD photo_url TEXT, buttons JSONB`; Down `DROP COLUMN`. | ~15 |
| `backend/internal/telegram/types_keyboard.go` | Crear | `InlineKeyboardMarkup`, `InlineKeyboardButton` con tags JSON correctos. | ~25 |
| `backend/internal/telegram/service.go` | Modificar | `SendMessage` agrega `keyboard *InlineKeyboardMarkup`; nuevo `SendPhoto`. | +10 |
| `backend/internal/telegram/publications.go` | Modificar | `SendMessage` extiende firma; agregar `SendPhoto` (mismo patrón: sendPhotoParams, sendPhotoResult, doWithRetry → doPost). | +30 |
| `backend/internal/telegram/publications_test.go` | Modificar | +2 tests: `SendPhoto_DecodesMessageID`, `SendPhoto_429Retries`, `InlineKeyboard_JSONMarshal`. | +90 |
| `backend/internal/telegram/poller_test.go` | Modificar | `fakeService` agrega `SendPhoto` y nueva firma `SendMessage`. | +8 |
| `backend/internal/publications/model.go` | Modificar | `Publication`: agregar `PhotoURL *string`, `Buttons [][][]InlineKeyboardButton` (en JSONB), `MarshalButtons`/`UnmarshalButtons` helpers. Errores: `ErrPhotoURLEmpty`, `ErrPhotoURLScheme`, `ErrPhotoURLTooLong`, `ErrTooManyGroups`, `ErrButtonsMalformed`, `ErrButtonsLimit`. | +60 |
| `backend/internal/publications/repository.go` | Modificar | `Create` inserta photo_url + buttons (JSONB); `scanPublication` lee photo_url + buttons (NULL → nil); nuevo `ListByTelegramID(ctx, telegramID)` (usa índice). | +35 |
| `backend/internal/publications/service.go` | Modificar | `validatePayload(text, photo_url, buttons, group_ids)`; `PublishMany(ctx, actorID, payload)` que llama a `Publish` para 1 grupo o itera; `Publish` extendido para aceptar `photoURL` + `keyboard`; `MessageSender` interface agrega `SendPhoto` + keyboard en `SendMessage`; `PubStore` agrega `ListByTelegramID`. | +180 |
| `backend/internal/publications/service_test.go` | Modificar | Actualizar fakes (`fakeTelegramPub` con `SendPhoto` y nueva firma `SendMessage`; `fakePubStore` con `ListByTelegramID`). +7 tests: PublishMany_OK_multi_grupo, PublishMany_orden_secuencial, PublishMany_fallo_parcial_Telegram, PublishMany_fallo_parcial_permiso, PublishMany_caption_1024_con_foto, PublishMany_validacion_text_vacio, PublishMany_validacion_URL_no_http, PublishMany_validacion_botones_8x8, PublishMany_validacion_grupos_vacio_y_excede, ListByTelegramID_filtra. | +200 |
| `backend/internal/api/publications_handlers.go` | Modificar | `publicationStore` interface: agrega `PublishMany` + `ListByTelegramID`; nuevo body `createPublicationRequest{Text, PhotoURL, Buttons, GroupIDs}`; `publicationResponse` agrega `photo_url` + `buttons`; nuevo `handleListPublications` parsea `?group_id=`; mapeo de errores nuevos (400 validation). | +90 |
| `backend/internal/api/publications_handlers_test.go` | Modificar | fake `publicationStore` con nuevos métodos; +7 tests: Create_OK_multi_grupo (201, `publications:[...]`), Create_OK_single_con_foto_y_botones, Create_400_group_ids_vacio, Create_400_URL_no_http, Create_400_botones_mas_8x8, Create_403_uno_sin_admin, List_con_filtro. | +110 |
| `backend/cmd/server/main.go` | Sin cambios | `bot` satisface `MessageSender` ampliado (la nueva firma es compatible con la asignación tipada). |
| `README.md` | Modificar | Agregar sección "Publicaciones" + fila `/publications` en tabla de uso del panel. | +20 |
| `frontend/src/features/publications/types.ts` | Modificar | `Publication`: agregar `photo_url` + `buttons`. `CreatePublicationInput` → `PublishRequest {text, photo_url?, buttons?, group_ids: number[]}`. Tipo `InlineButton {text, url}`. | +15 |
| `frontend/src/features/publications/api.ts` | Modificar | `listPublications({group_id?})` → `request('/api/publications' + (group_id ? `?group_id=${group_id}` : ''))`. `createPublication(PublishRequest)`. | +10 |
| `frontend/src/features/publications/hooks.ts` | Modificar | `usePublications({group_id?})` query key `['publications', group_id ?? 'all']`. `useCreatePublication` invalida `['publications']` (todas las variantes). | +10 |
| `frontend/src/features/publications/error.ts` | Modificar | Mensajes para nuevos códigos de validación (URL no http(s), >10 grupos, >8×8, sin grupos). | +10 |
| `frontend/src/features/publications/ButtonsEditor.tsx` | Crear | Componente controlado: array de filas, cada fila array de `{text, url}`. Botones "Agregar/quitar fila", "Agregar/quitar botón por fila". Validación cliente. | +80 |
| `frontend/src/pages/PublicationsPage.tsx` | Modificar | Form: `photo_url` input + `ButtonsEditor` + multi-select (checkboxes desde `useGroups()`); `maxLength` dinámico 1024 si foto, 4096 si no. Listado: filtro por grupo (`usePublications({group_id})`), columna foto (`<img>` o link) + chips de botones + estado. | +120 |
| `frontend/src/pages/PublicationsPage.test.tsx` | Modificar | +5 tests: lista_con_foto_y_botones, filtro_por_grupo_recarga, formulario_con_foto_y_multi_grupo, validacion_URL_no_http_sin_enviar_POST, validacion_sin_grupos_sin_enviar_POST. | +120 |

## Interfaces / Contracts

**Go (telegram)** — patrón narrow paralelo:

```go
// types_keyboard.go
type InlineKeyboardButton struct {
    Text string `json:"text"`
    URL  string `json:"url"`
}
type InlineKeyboardMarkup struct {
    InlineKeyboard [][]InlineKeyboardButton `json:"inline_keyboard"`
}

// service.go (modificado)
type Service interface {
    // ... métodos existentes ...
    SendMessage(ctx context.Context, chatID int64, text string, disableWebPagePreview bool, keyboard *InlineKeyboardMarkup) (int64, error)
    SendPhoto(ctx context.Context, chatID int64, photoURL, caption string, keyboard *InlineKeyboardMarkup) (int64, error)
}

// publications.go (nuevo método)
type sendPhotoParams struct {
    ChatID      int64                 `json:"chat_id"`
    Photo       string                `json:"photo"`           // URL pública
    Caption     string                `json:"caption,omitempty"`
    ReplyMarkup *InlineKeyboardMarkup `json:"reply_markup,omitempty"`
}
func (a *Adapter) SendPhoto(ctx context.Context, chatID int64, photoURL, caption string, keyboard *InlineKeyboardMarkup) (int64, error) {
    var result struct{ MessageID int64 `json:"message_id"` }
    err := a.doWithRetry(ctx, func() error {
        return a.doPost(ctx, "sendPhoto", sendPhotoParams{
            ChatID: chatID, Photo: photoURL, Caption: caption, ReplyMarkup: keyboard,
        }, &result)
    })
    if err != nil { return 0, err }
    return result.MessageID, nil
}
```

**Go (publications)** — nuevas interfaces:

```go
// MessageSender ampliado (compatible con slice 1: la nueva firma de SendMessage
// requiere actualizar fakes — flag en tasks).
type MessageSender interface {
    SendMessage(ctx context.Context, chatID int64, text string, disableWebPagePreview bool, keyboard *telegram.InlineKeyboardMarkup) (int64, error)
    SendPhoto(ctx context.Context, chatID int64, photoURL, caption string, keyboard *telegram.InlineKeyboardMarkup) (int64, error)
}

// PubStore ampliado
type PubStore interface {
    Create(ctx context.Context, p *Publication) error
    GetByID(ctx context.Context, id int64) (*Publication, error)
    List(ctx context.Context) ([]Publication, error)
    ListByTelegramID(ctx context.Context, telegramID int64) ([]Publication, error) // NUEVO
    UpdateStatus(ctx context.Context, id int64, status Status, messageID *int64, errMsg *string) error
}

// Payload + result
type PublishPayload struct {
    Text     string                         `json:"text"`
    PhotoURL *string                        `json:"photo_url,omitempty"`
    Buttons  [][]telegram.InlineKeyboardButton `json:"buttons,omitempty"`
    GroupIDs []int64                        `json:"group_ids"`
}
```

**REST** — body nuevo (publications_handlers.go):

```go
type createPublicationRequest struct {
    Text     string                              `json:"text"`
    PhotoURL *string                             `json:"photo_url,omitempty"`
    Buttons  [][]telegram.InlineKeyboardButton   `json:"buttons,omitempty"`
    GroupIDs []int64                             `json:"group_ids"`
}
// Respuesta: envelope data.publications:[Publication, ...] (mismo orden que group_ids)
```

## Testing Strategy

| Capa | Qué probar | Cómo |
|------|-----------|------|
| **Adapter (telegram)** | `SendPhoto` decodifica `message_id`; `SendPhoto` 429 retry; `SendMessage` con/sin teclado omite `reply_markup` cuando nil; `InlineKeyboardMarkup` JSON exacto `{"inline_keyboard":[[{"text":"A","url":"https://..."}]]}` | `httptest.NewServer` + decode body. Sin llamadas reales a Bot API. |
| **Service (publications)** | `PublishMany` SECUENCIAL (fake registra orden de invocaciones); fallo parcial (Telegram rechaza un grupo, los demás `sent`); fallo parcial (bot no admin en un grupo, los demás `sent`); `PublishMany` con 1 grupo delega a `Publish`; `PublishMany` caption 1024 con foto / texto 4096 sin foto; validaciones fail-fast: text vacío, photo_url no http(s), photo_url > 2048, buttons > 8 filas, button url no http(s), group_ids vacío, group_ids > 10. `ListByTelegramID` filtra. | `fakeTelegramPub` (con `SendPhoto` + keyboard), `fakePubStore` (con `ListByTelegramID`), `fakeGroupsPub`. |
| **Handler (api)** | `POST` 201 multi-grupo (verifica `data.publications:[…]`); 201 single con foto + botones; 400 group_ids vacío; 400 URL inválida; 400 botones > 8×8; 403 uno sin admin; `GET` 200 con/sin filtro. | `httptest` + `doRequest`. |
| **Frontend** | Carga con foto + botones; filtro por grupo (mockFetchRoutes distingue por query); formulario con foto + multi-grupo (mock custom para POST); validación cliente URL no-http no envía POST; validación cliente sin grupos no envía POST; error de carga muestra reintento. | `mockFetchRoutes` + mocks custom distinguidos por `init.method`. |

## Migration / Rollout

- **No downtime**: agregar columnas nullable no rompe filas existentes (slice 1 queda con `photo_url=NULL, buttons=NULL`).
- **Up (00005)**: `ALTER TABLE publications ADD COLUMN photo_url TEXT, ADD COLUMN buttons JSONB;` (sin backfill).
- **Down (00005)**: `ALTER TABLE publications DROP COLUMN photo_url, DROP COLUMN buttons;` (filas no se pierden, solo las columnas nuevas).
- **Producción**: ejecutar `goose up` antes del deploy (manual, AGENTS §13.1).

## Open Questions (resolved by exploration/spec — confirmed here)

| Pregunta | Decisión lockeada |
|----------|-------------------|
| ¿Método unificado `Send(chatID, opts)` o `SendMessage`/`SendPhoto` paralelos? | **Paralelos** (DECIDE — spec). Patrón narrow. |
| ¿201 o 207 en fallo parcial? | **201** siempre que el payload sea válido (DECIDE — spec). |
| ¿Validación fail-fast o per-grupo? | **Fail-fast para payload**; per-grupo solo 404/403/Telegram (DECIDE — spec). |
| ¿Botones como JSONB o tabla aparte? | **JSONB** (DECIDE — spec). |
| ¿Multi-grupo paralelo o secuencial? | **Secuencial** (DECIDE — §18.1). |
| ¿Foto upload multipart o URL? | **Solo URL** (DECIDE — slice 2). |
| ¿Botones `callback_data`? | **No** (slice 2: solo `url`). |
| ¿Paginación > 50? | **No** (DECIDE — slice 2; slice 3 si crece). |
| ¿Límite de botones? | **8 filas × 8 botones** = 64 (autoimpuesto; doc como decisión de producto). |
| ¿`Publish` slice 1 sobrevive? | **Sí** — `PublishMany` lo llama cuando hay 1 grupo; `Publish` extiende firma para foto+botones (compatibilidad). |
| **Bugfix #172** (permission-check invariant) | **Vigente**: `permissionOk` sigue usando `BotStatus == StatusAdministrator`; NO reintroducir checks `can_*`. Aplica a `PublishMany` también. |

## Risks

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Fakes `Service` desincronizadas tras extender `SendMessage` + agregar `SendPhoto` (rompe compilación en `telegram/`, `publications/`, `api/`) | High | High (compile error en todo el árbol) | Flag explícito en tasks: **primer paso de apply** es actualizar `poller_test.go fakeService` + `publications/service_test.go fakeTelegramPub` + `api/publications_handlers_test.go fakePublicationStore` antes de cualquier cambio de runtime. `go build ./...` debe pasar en cada commit. |
| Tamaño del PR excede 400 LOC | Medium | Medium (reviewer fatigue) | PR único pero cohesivo; entrega single-pr per orchestrator. Si en `sdd-tasks` se supera forecast High → sugerir chained PRs (backend → frontend + README) o stacked por área. Estimación actual ~600 LOC touched → flag Medium en tasks. |
| URL foto inaccesible o > 5 MB | Medium | Low (fila queda `failed` con mensaje real) | Documentar en README; spec ya define que no se valida accesibilidad. `error_message` queda legible. |
| `reply_markup` se serializa aunque sea nil (omite `omitempty` accidental) | Low | Medium (Telegram puede rechazar payloads vacíos) | Test explícito `TestSendPhoto_ReplyMarkupNil_OmittedFromPayload`; tags con `omitempty` en `sendPhotoParams.ReplyMarkup`. |
| Caption > 1024 con foto aceptado en frontend (no validado antes del POST) | Low | Medium (recibir 400 del server después de un envío ya iniciado) | Validación cliente en `ButtonsEditor` + `PublicationsPage` antes de `create.mutate()`; `maxLength` dinámico 1024/4096. |
| Botón con URL `javascript:` (XSS) | Low | High (seguridad) | Validación http(s) en servicio + cliente; spec lo cubre con `ErrButtonsMalformed`/400. |

## Rollback

`goose down` a 00004 (DROP COLUMN); revertir archivos Go y TS. Filas slice 1 mantienen `photo_url=NULL, buttons=NULL` (no se ven afectadas). No requiere tocar otras migraciones.

## Success Criteria (verificación post-apply)

- [ ] `goose up` 00005 aplica limpia; `goose down` revierte sin pérdida de filas
- [ ] `go test ./...` en verde; `npm test -- --run` en verde
- [ ] `POST /api/publications {group_ids:[2], photo_url, buttons}` → 201 con 2 filas, ambas `sent`
- [ ] Validaciones devuelven 400 antes de tocar Telegram/DB
- [ ] Fallo parcial: 1 grupo 403/404/Telegram-error → 201, fila per-grupo con `status: failed`, resto `sent`
- [ ] `GET ?group_id=X` filtra usando `idx_publications_telegram_id`
- [ ] Frontend: form con foto + botones + multi-grupo, listado con preview, filtro recarga, validaciones cliente