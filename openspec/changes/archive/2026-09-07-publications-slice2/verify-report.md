# Verify Report — Publications Slice 2 (Foto URL + Botones + Multi-Grupo + Filtro)

> **Change**: `publications` (Slice 2 de 3). Mode: hybrid.
> **Branch**: `feat/publications-slice2` (6 commits ahead of `main`).
> **Base**: `main` @ `9d4f1ec` ("fix(publications): exigir bot administrator en vez de can_manage_chat" — bugfix #172).
> **Strict TDD**: OFF (no `.atl/`, no `sdd.json`, no `strict_tdd` config en repo).

## Resumen ejecutivo

Implementación completa del slice 2. Backend y frontend pasan todos
los gates (`go build`/`go vet`/`gofmt`/`go test -count=1 ./...`,
`npm test -- --run`/`npm run build`). 15/15 REQs cubiertas con
evidencia en código + tests pasando. Las 10 decisiones de diseño
(D1-D10) más el invariante del bugfix #172 (`bot_status ==
administrator`, **NO** `can_*`) están respetados. 3 desviaciones
documentadas, todas aceptables. **Verdict: PASS**.

## Completeness table (tasks)

| Phase | Tasks | Implementadas | Tests pasando | Notas |
|-------|-------|---------------|---------------|-------|
| 1. Migration + tipos + interfaz + fakes | 5 (1.1-1.5) | 5/5 | `go build` verde | 00005 + types_keyboard.go + interface extension + 3 fakes |
| 2. Adapter SendPhoto + Keyboard | 2 (2.1-2.2) | 2/2 | 9 telegram tests | SendPhoto con caption + reply_markup, 429 retry, JSON exacto |
| 3. Publications module | 4 (3.1-3.4) | 4/4 | 16 publications tests | PublishMany secuencial, publishOne compartido, validatePayload fail-fast |
| 4. API handlers | 2 (4.1-4.2) | 2/2 | 15 api tests | body multi-grupo, envelope 201 `{publications:[]}`, mapeo 400/403 |
| 5. Backend suite green | 1 (5.1) | 1/1 | All packages OK | go vet/gofmt vacíos, go test -count=1 verde |
| 6. Frontend | 7 (6.1-6.7) | 7/7 | 8 page tests | PublicationsPage foto + botones + multi + filtro + validaciones |
| 7. Frontend suite green | 1 (7.1) | 1/1 | 51/51 tests, build OK | tsc --noEmit + vite build, 431ms |
| 8. README + commit | 2 (8.1-8.2) | 2/2 | n/a | Sección Publicaciones con todos los límites; NO push (single-pr local) |
| **Total** | **24** | **24/24 (100%)** | | |

> **Nota documental**: el apply-report.md (líneas 142-146) declara
> que las tareas se marcan `[x]` "en el reporte (no se modifica el
> tasks.md físico)" y a la vez "se incluye en
> `openspec/changes/publications-slice2/tasks.md`". El archivo real
> `tasks.md` mantiene TODAS las casillas como `[ ]`. **Esto NO afecta
> la verificación**: la implementación está completa y verificada por
> código + tests pasando. Sugerencia menor: aclarar en el apply-report
> la regla "no se modifica el tasks.md físico" para evitar confusión.

## Spec compliance matrix (15 REQs)

### publications/spec.md

| REQ | Verdict | Evidencia |
|-----|---------|-----------|
| **Validaciones de payload** (fail-fast 400) | **PASS** | `publications/service.go:360-394` `validatePayload` corre una vez al inicio de `PublishMany`. Tests: `TestService_PublishMany_Validacion_*` (8 tests: TextVacio, CaptionExcede1024, TextExcede4096, PhotoURLNoHTTP, PhotoURLMuyLarga, BotonesMas8Filas, BotonURLNoHTTP, GruposVacio/Excede10). Handler mapea errores a 400 via `validationErrorSet` (líneas 210-224). |
| **PublishMany multi-grupo secuencial** | **PASS** | `publications/service.go:179-193` loop SECUENCIAL sobre `payload.GroupIDs` (CERO goroutines). Tests: `TestService_PublishMany_OK_MultiGrupo`, `TestService_PublishMany_OrdenSecuencial` (verifica índices contra `[-1003,-1001,-1002]`), `TestService_PublishMany_FalloParcial_Telegram`, `TestService_PublishMany_FalloParcial_Permiso`, `TestService_PublishMany_GrupoInexistente_NoAborta`. |
| **Tipos InlineKeyboardMarkup/InlineKeyboardButton** | **PASS** | `telegram/types_keyboard.go:16-26` con tags JSON `text`/`url`/`inline_keyboard` exactos. Tests: `TestInlineKeyboard_JSONMarshal` valida el JSON exacto `{"inline_keyboard":[[{"text":"A","url":"https://a"}],…]}`. |
| **Adapter SendPhoto** | **PASS** | `telegram/publications.go:70-84` `SendPhoto(ctx, chatID, photoURL, caption, keyboard)`. Tests: `TestAdapterSendPhoto_DecodesMessageID`, `TestAdapterSendPhoto_WithKeyboard`, `TestAdapterSendPhoto_429Retries`, `TestSendPhotoParams_OmitEmptyCaptionAndReplyMarkup`. |
| **README sección publicaciones** | **PASS** | `README.md:88-131` con todos los límites (5MB, 1024, 4096, ≤10 grupos, ≤8×8 botones, URL only), filtro `?group_id=`, nota sobre `bot_status == administrator` (sin `can_*`). Fila `/publications` añadida a tabla de rutas (línea 83). |
| **Tabla publications** (MODIFIED: photo_url+buttons) | **PASS** | `backend/migrations/00005_alter_publications.sql` ALTER ADD `photo_url TEXT` + `buttons JSONB` (Up), DROP COLUMN (Down). `publications/repository.go:29-42` inserta columnas; `scanPublication` lee NULL→nil. Test `TestService_PublishMany_ConFotoYBotones` verifica persistencia. |
| **SendMessage signature** (MODIFIED: keyboard) | **PASS** | `telegram/service.go:62` nueva firma con `keyboard *InlineKeyboardMarkup`. `telegram/publications.go:48-62` con `sendMessageParams.ReplyMarkup` (`omitempty`). Tests: `TestAdapterSendMessage_WithKeyboard`, `TestAdapterSendMessage_ReplyMarkupNil_OmittedFromPayload`. |
| **POST /api/publications multi-grupo** (MODIFIED) | **PASS** | `api/publications_handlers.go:75-107` con body `{text, photo_url?, buttons?, group_ids[]}`, response `201 {publications:[]}`. Tests: `TestPublications_CreateSuccess`, `TestPublications_CreateMultiGrupo_201` (verifica 2 filas), `TestPublications_CreateOK_SingleConFotoYBotones`, `TestPublications_Create_400_*` (5 tests), `TestPublications_Create_403_BotPermission`. |
| **GET /api/publications filtro** (MODIFIED) | **PASS** | `api/publications_handlers.go:112-141` parsea `?group_id=`. `publications/service.go:353-355` `ListByTelegramID`. `publications/repository.go:94-120` usa `idx_publications_telegram_id`. Tests: `TestPublications_List_ConFiltro` (verifica delegation), `TestPublications_List_ConFiltroInvalido`, `TestService_ListByTelegramID_Filtra`. |
| **Frontend PublicationsPage** (MODIFIED) | **PASS** | `pages/PublicationsPage.tsx` (314 líneas) con form completo (text+photo_url+ButtonsEditor+multi-select checkboxes desde `useGroups()`, `maxLength` dinámico 1024/4096), listado con preview foto+chips, filtro por grupo. Tests: 8/8 PublicationsPage pasan (lista con foto+botones, filtro recarga, formulario multi-grupo, validaciones cliente, error de carga con reintento, permiso denegado). |
| **Tests backend** (MODIFIED) | **PASS** | Cobertura completa en service_test.go (16 tests), telegram/publications_test.go (9 tests), api/publications_handlers_test.go (15 tests). §21.1 respetado: adapter usa `httptest.NewServer`, service usa `fakeTelegramPub` con `SendPhoto`+keyboard. |
| **Tests frontend** (MODIFIED) | **PASS** | `PublicationsPage.test.tsx` (338 líneas, 8 tests). Usa `mockFetchRoutes` + mocks custom distinguiendo GET/POST por `init.method`. Cobertura: carga, vacío, error+reintento, multi-grupo con foto+botones, filtro, validaciones cliente URL/grupos, 403 legible. |
| **Registro de auditoría** (MODIFIED) | **PASS** | `publications/service.go:286` crea log con `ActionPublishMessage` per-row, `metadata{publication_id, message_id?}`. Tests: `TestService_PublishMany_OK_MultiGrupo` (2 logs), `TestService_PublishMany_FalloParcial_Telegram` (2 logs: PERMISSION_DENIED+SUCCESS), `TestService_PublishMany_FalloParcial_Permiso` (2 logs), `TestService_PublishMany_GrupoInexistente_NoAborta` (NOT_FOUND). |

### telegram-moderation/spec.md

| REQ | Verdict | Evidencia |
|-----|---------|-----------|
| **Tipos InlineKeyboardMarkup/Button** (ADDED) | **PASS** | Mismo que REQ 3 (publications). `telegram/types_keyboard.go` + `TestInlineKeyboard_JSONMarshal`. |
| **Métodos de moderación y publicación** (MODIFIED: SendPhoto + SendMessage signature) | **PASS** | `telegram/service.go:62,66` firmas actualizadas; `SendPhoto` invoca `sendPhoto`; `SendMessage` con keyboard opcional. Tests en `TestAdapterSend*` + `TestInlineKeyboard_JSONMarshal` (5 escenarios cubiertos). Escenarios "Ban/Mute sin cambios" validados por tests previos del repo (no regresionados). |

## Design coherence (D1-D10 + bugfix #172)

| # | Decisión | Verdict | Evidencia |
|---|----------|---------|-----------|
| **D1** | `SendMessage(...,keyboard)` extendido + `SendPhoto(chatID,photoURL,caption,keyboard)` paralelo | **PASS** | `telegram/service.go:62,66`. Implementación en `telegram/publications.go:48,70`. Patrón narrow paralelo respetado. |
| **D2** | Botones en JSONB `[[{text,url},...],...]` | **PASS** | `publications/model.go:66` `Buttons json.RawMessage` (tipo JSONB en DB). `MarshalButtons`/`UnmarshalButtons` en líneas 110-127. |
| **D3** | Foto + texto en UNA llamada (`sendPhoto` con caption) | **PASS** | `publications/service.go:304-310` `dispatchWithPhoto` decide SendPhoto vs SendMessage. Cero `sendMessage` después de `sendPhoto`. |
| **D4** | `PublishMany` SECUENCIAL (sin goroutines) | **PASS** | `publications/service.go:187-191` loop simple. Test `TestService_PublishMany_OrdenSecuencial` verifica orden exacto contra `[-1003,-1001,-1002]`. |
| **D5** | Validación fail-fast ANTES de iterar; errores per-grupo NO abortan | **PASS** | `publications/service.go:180` `validatePayload` UNA vez. Errores per-grupo (404/403/Tel) se manejan en `publishOne` (líneas 217-235) sin abortar el resto. |
| **D6** | HTTP 201 incluso si TODAS quedan `failed` | **PASS** | `api/publications_handlers.go:106` siempre responde 201 con `data.publications` cuando el payload es válido. Ningún 207 Multi-Status. |
| **D7** | `Publish` (slice 1) preservado; `PublishMany` adicional | **PASS** (con desviación menor) | `Publish` (single-group) y `PublishMany` (multi-grupo) coexisten. **Desviación documentada**: en vez de "PublishMany llama a Publish para 1 grupo", se extrajo un helper `publishOne` privado compartido por ambos. Comportamiento observable idéntico (1 grupo = 1 fila, N grupos = N filas secuenciales). |
| **D8** | `PubStore` agrega `ListByTelegramID` (filtro opcional) | **PASS** | `publications/service.go:74` interface extended; `publications/repository.go:94-120` impl real; `fakePubStore.ListByTelegramID` en `publications/service_test.go:129-137`. Handler parsea `?group_id=` (handlers.go:122-131). |
| **D9** | Frontend query key `['publications', group_id ?? 'all']`; multi-select checkboxes | **PASS** | `features/publications/hooks.ts:17` `queryKey: [...publicationsKey, filter?.group_id ?? 'all']`. `pages/PublicationsPage.tsx:161-182` fieldset con checkboxes por grupo (no select múltiple HTML). |
| **D10** | Límites: text ≤ 4096/1024, photo_url ≤ 2048 http(s), botones 8×8, group_ids 1..10 | **PASS** | Constantes en `publications/model.go:38-47`. Validación exhaustiva en `validatePayload` + `validateTextLength` + `validatePhotoURL` + `validateButtons`. Frontend: `error.ts:30-56` y `PublicationsPage.tsx:29-31,141,162,173`. |
| **#172** | permissionOk UNCHANGED (bot_status==administrator) | **PASS** | `publications/service.go:486-488` `return g != nil && g.BotStatus == groups.StatusAdministrator`. **NO** checks sobre claves `can_*` en código activo (las 5 menciones de "can_*" en service.go son comentarios documentando el bugfix histórico). Usado en `Publish` (línea 143) y `publishOne` (línea 231). `groups.StatusAdministrator = "administrator"` (`groups/model.go:18`). |

## Build / Tests / Coverage evidence

### Backend (fresh run, no cache)

```text
$ go build ./...         # OK (no output)
$ go vet ./...           # OK (no output)
$ gofmt -l .             # OK (empty output)
$ go test -count=1 ./...
?   	backend/cmd/server                       [no test files]
ok  	backend/internal/api                     1.997s
ok  	backend/internal/auth                    2.239s
ok  	backend/internal/config                  1.013s
?   	backend/internal/database                [no test files]
ok  	backend/internal/events                  1.350s
ok  	backend/internal/groups                  2.483s
ok  	backend/internal/joinrequests            1.296s
ok  	backend/internal/logs                    0.840s
ok  	backend/internal/moderation              0.559s
ok  	backend/internal/publications            1.119s
ok  	backend/internal/telegram                22.640s
ok  	backend/internal/users                   0.640s
?   	backend/migrations                       [no test files]
```

### Slice-2 specific tests (verbose, todos PASS)

**internal/publications** (16 tests, ~1.2s):
- `TestService_PublishMany_OK_MultiGrupo`
- `TestService_PublishMany_OrdenSecuencial`
- `TestService_PublishMany_FalloParcial_Telegram`
- `TestService_PublishMany_FalloParcial_Permiso`
- `TestService_PublishMany_GrupoInexistente_NoAborta`
- `TestService_PublishMany_ConFotoYBotones`
- `TestService_PublishMany_Validacion_*` (8 tests)
- `TestService_PublishMany_MarshalButtons_RoundTrip`

**internal/telegram** (9 tests slice-2, ~6s):
- `TestAdapterSendMessage_DecodesMessageID`
- `TestAdapterSendMessage_429Retries`
- `TestAdapterSendMessage_WithKeyboard`
- `TestAdapterSendMessage_ReplyMarkupNil_OmittedFromPayload`
- `TestAdapterSendPhoto_DecodesMessageID`
- `TestAdapterSendPhoto_WithKeyboard`
- `TestAdapterSendPhoto_429Retries`
- `TestInlineKeyboard_JSONMarshal`
- `TestSendPhotoParams_OmitEmptyCaptionAndReplyMarkup`

**internal/api** (15 tests, ~0.3s):
- `TestPublicationsRoutes_RequireAuth`
- `TestPublications_CreateSuccess`
- `TestPublications_CreateMultiGrupo_201`
- `TestPublications_CreateOK_SingleConFotoYBotones`
- `TestPublications_Create_400_*` (5 tests)
- `TestPublications_Create_GroupNotFound`
- `TestPublications_Create_403_BotPermission`
- `TestPublications_GetByID` + `GetByIDNotFound`
- `TestPublications_List`
- `TestPublications_List_ConFiltro`
- `TestPublications_List_ConFiltroInvalido`

### Frontend

```text
$ npm test -- --run
 Test Files  11 passed (11)
      Tests  51 passed (51)
   Duration  8.14s

$ npm run build
> tsc --noEmit && vite build
vite v8.2.2 building client environment for production...
dist/index.html                   0.40 kB │ gzip:   0.26 kB
dist/assets/index-CThllaPF.css    4.79 kB │ gzip:   1.38 kB
dist/assets/index-InDfwPF.js    403.96 kB │ gzip: 122.76 kB
✓ 190 modules transformed.
✓ built in 431ms
```

**PublicationsPage.test.tsx** (8 tests, todos PASS):
- `lista las publicaciones con estado, foto y botones` (282ms)
- `muestra estado vacio` (66ms)
- `muestra error de carga y permite reintentar` (418ms)
- `crea una publicacion multi-grupo con foto y botones` (1797ms)
- `filtra por grupo y recarga con ?group_id=` (173ms)
- `no envia POST si la URL de la foto no es http(s)` (704ms)
- `no envia POST si no hay grupos seleccionados` (208ms)
- `muestra mensaje legible al intentar publicar sin permisos` (302ms)

## Fakes discipline (verificado)

| Interfaz extendida | Implementaciones verificadas |
|--------------------|------------------------------|
| `telegram.Service` (SendMessage signature + SendPhoto nuevo) | `*Adapter` (`telegram/publications.go:48,70`) ✅; `*fakeService` (`telegram/poller_test.go:63,66`) ✅; `*fakeTelegramPub` (`publications/service_test.go:57,66`) ✅ |
| `publications.MessageSender` (mismo subset telegram) | `*Adapter` (compatible por asignación tipada en cmd/server) ✅; `*fakeTelegramPub` ✅ |
| `publications.PubStore` (+ ListByTelegramID) | `*Repository` (`publications/repository.go:94`) ✅; `*fakePubStore` (`publications/service_test.go:129`) ✅ |
| `api.publicationStore` (+ PublishMany + ListByTelegramID) | `*publications.Service` (composition) ✅; `*fakePublicationStore` (`api/publications_handlers_test.go:46,76`) ✅ |

**Nota sobre `fakePublicationStore`**: confirma lo declarado en
apply-report (líneas 132-138): implementa la interfaz del paquete `api`
(`publicationStore`), NO `telegram.Service`. Por eso solo se actualiza
en Phase 4 cuando `publicationStore` se extiende (con `PublishMany` +
`ListByTelegramID`), no en Phase 1 cuando se extiende `telegram.Service`.
**No requiere doble pasada**, y la compilación lo confirma
(`go build ./...` verde, sin errores).

## Migration 00005 (Goose Up/Down)

```text
-- +goose Up
ALTER TABLE publications ADD COLUMN photo_url TEXT;
ALTER TABLE publications ADD COLUMN buttons   JSONB;
-- +goose Down
ALTER TABLE publications DROP COLUMN buttons;
ALTER TABLE publications DROP COLUMN photo_url;
```

Estilo: `+goose Up` / `+goose Down` (no usa `+goose StatementBegin`,
cosa esperada para ALTERs no transaccionales). Columnas NULL-able,
sin defaults, sin backfill. Filas slice 1 quedan con `photo_url=NULL`,
`buttons=NULL`. Down revierte sin perder filas (solo dropea columnas).

## Desviaciones documentadas (3, todas aceptables)

1. **`publishOne` helper compartido** (vs design D7 "PublishMany llama a Publish para 1 grupo"): refactor para evitar duplicación del path de envío. Comportamiento observable idéntico (1 grupo == 1 fila, N grupos == N filas secuenciales; Publish preserva firma extendida). Tests siguen cubriendo ambos paths.

2. **`Buttons` como `json.RawMessage` en `model.go`**: idiomático Go para JSONB. Conversión a `[][]InlineKeyboardButton` ocurre en la frontera service via `MarshalButtons`/`UnmarshalButtons`. Spec DECIDE era "JSONB"; cumplido. No rompe contrato HTTP (el handler serializa verbatim en `Buttons json.RawMessage`).

3. **`fakePublicationStore` actualizado en Phase 4** (no Phase 1): implementa la interfaz api-layer, NO telegram.Service. Solo rompe cuando `publicationStore` se extiende — eso sucede en Phase 4 junto con el cambio de body. No requiere doble pasada.

## Issues

### CRITICAL

_Ninguno._

### WARNING

_Ninguno._

### SUGGESTION

- **Apply-report contradicción sobre tasks.md** (líneas 142-146): el
  reporte declara que las 24 tareas están marcadas `[x]` en
  `openspec/changes/publications-slice2/tasks.md`; el archivo real las
  mantiene todas como `[ ]`. La implementación está completa y
  verificada por código + tests, pero el reporte debería decir
  explícitamente "los checks quedan en el apply-report, el tasks.md
  físico no se modifica (regla DECIDE del sdd-apply hybrid)" para
  evitar confusión en futuras auditorías.

## Verdict

**PASS** (con 1 SUGGESTION no bloqueante).

Todas las REQs cubiertas con código + tests pasando; las decisiones
D1-D10 implementadas fielmente; el invariante del bugfix #172
(`bot_status == administrator`, NO `can_*`) intacto; los fakes de las
3 interfaces extendidas (`telegram.Service`, `publications.PubStore`,
`api.publicationStore`) están en sincronía; los 3 gates del backend
(`build`/`vet`/`fmt`) y los 2 del frontend (`test`/`build`) verdes; la
migración 00005 con Up/Down estilo goose correcto.

## Qué queda para sdd-archive

- Sync de las delta specs (`openspec/changes/publications-slice2/specs/publications/spec.md` y `…/telegram-moderation/spec.md`) a sus canónicas (`openspec/specs/publications/spec.md` y `openspec/specs/telegram-moderation/spec.md`).
- Mover el change folder a `openspec/changes/archive/2026-09-07-publications-slice2/` con `archive-report.md`.
- Decisión de merge: single-pr / size-exception ya aprobada por el usuario (decision #178). El branch NO está pusheado a remoto.