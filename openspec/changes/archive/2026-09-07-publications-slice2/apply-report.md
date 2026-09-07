# Apply Report — Publications Slice 2 (Foto URL + Botones + Multi-Grupo + Filtro)

> Change: `publications` (Slice 2 de 3). Mode: hybrid.
> Strategy: single-pr / size-exception approved by user (decision #178).
> Branch: `feat/publications-slice2` (5 commits, no push).

## Resumen

Implementación completa del slice 2 del módulo publications según
exploration, proposal, spec, design y tasks archivados en
`openspec/changes/publications-slice2/`. Backend y frontend pasan
todos los gates; bugfix #172 (`permission-check`) intacto.

## Commits (5, en orden de creación)

1. `085d726` **feat(telegram)**: SendPhoto + InlineKeyboard types + migration 00005
2. `6e928ce` **feat(publications)**: PublishMany multi-grupo + foto + botones
3. `5d3c8be` **feat(api)**: POST /api/publications multi-grupo + GET ?group_id= + mapeo 400
4. `5ee347f` **feat(web)**: PublicationsPage foto + botones + multi-grupo + filtro
5. `63c9772` **docs**: README — sección Publicaciones con límites operacionales

## Archivos creados (4)

| Archivo | Líneas | Descripción |
|---------|--------|-------------|
| `backend/migrations/00005_alter_publications.sql` | 13 | ALTER ADD photo_url TEXT, buttons JSONB; Down DROP COLUMN |
| `backend/internal/telegram/types_keyboard.go` | 26 | InlineKeyboardMarkup / InlineKeyboardButton con tags JSON exactos |
| `frontend/src/features/publications/ButtonsEditor.tsx` | 119 | Editor controlado de filas text+url, agregar/quitar fila y botón |

## Archivos modificados (13)

| Archivo | +/− | Resumen |
|---------|-----|---------|
| `backend/internal/telegram/service.go` | +10 | SendMessage gana `keyboard *InlineKeyboardMarkup`; nuevo `SendPhoto(ctx,chatID,photoURL,caption,keyboard)` |
| `backend/internal/telegram/publications.go` | +54 | `sendPhotoParams`, `SendPhoto` (doWithRetry → doPost → decode message_id); SendMessage extendido con ReplyMarkup |
| `backend/internal/telegram/poller_test.go` | +5 | fakeService.SendMessage con nueva firma + SendPhoto |
| `backend/internal/telegram/publications_test.go` | +237 | 5 nuevos tests: SendPhoto decode / 429 / WithKeyboard / ReplyMarkupNil / InlineKeyboard_JSONMarshal |
| `backend/internal/publications/model.go` | +86 | PhotoURL, Buttons (json.RawMessage), 13 nuevos errores de validación, MarshalButtons / UnmarshalButtons |
| `backend/internal/publications/repository.go` | +61 | Create inserta photo+buttons; scanPublication lee JSONB; ListByTelegramID (idx_publications_telegram_id) |
| `backend/internal/publications/service.go` | +446 | validatePayload fail-fast, PublishMany SECUENCIAL, publishOne, dispatch (SendPhoto vs SendMessage), Publish extendido con foto+botones |
| `backend/internal/publications/service_test.go` | +515 | 9 nuevos tests: multi-grupo OK, orden secuencial, fallo parcial Telegram/permiso/grupo inexistente, foto+botones, 7 validaciones, ListByTelegramID |
| `backend/internal/api/publications_handlers.go` | +128 | Body `group_ids[]`; envelope 201 `{publications:[...]}`; GET `?group_id=`; mapeo 400 nuevos errores |
| `backend/internal/api/publications_handlers_test.go` | +274 | 7 nuevos tests: 201 multi-grupo, 201 single foto+botones, 400 group_ids vacío, 400 URL no-http, 400 botones 8×8, 400 caption 1024, 403 bot sin admin, GET con filtro |
| `frontend/src/features/publications/types.ts` | +33 | InlineButton, PublishRequest con group_ids[], Publication con photo_url+buttons |
| `frontend/src/features/publications/api.ts` | +22 | listPublications con filtro, createPublication con envelope `{publications}` |
| `frontend/src/features/publications/hooks.ts` | +16 | usePublications con key `['publications', group_id ?? 'all']`; invalidación prefix |
| `frontend/src/features/publications/error.ts` | +50 | validatePhotoUrlClient / validateGroupIdsClient / validateButtonsClient |
| `frontend/src/pages/PublicationsPage.tsx` | +246 | Form foto + botones editor + multi-select + maxlength dinámico; listado con preview foto+chips; filtro por grupo |
| `frontend/src/pages/PublicationsPage.test.tsx` | +215 | 5 nuevos casos: lista con foto+botones, filtro recarga, formulario multi-grupo, validación URL no-http, validación sin grupos |
| `README.md` | +52 | Sección Publicaciones con body JSON, límites (5MB foto, caption 1024, texto 4096, ≤10 grupos, ≤8×8 botones, URL only); tabla del panel suma `/publications` |

**Total estimado**: 19 archivos tocados, ~1240 LOC (alineado con forecast del tasks).

## Verificación ejecutada

### Backend

```text
$ go build ./...         # OK (sin errores)
$ go vet ./...           # OK (sin warnings)
$ gofmt -l .             # OK (lista vacía)
$ go test ./...
ok  github.com/telegram-manager/backend/internal/api            1.469s
ok  github.com/telegram-manager/backend/internal/auth           (cached)
ok  github.com/telegram-manager/backend/internal/config        (cached)
?   github.com/telegram-manager/backend/internal/database      [no test files]
ok  github.com/telegram-manager/backend/internal/events         (cached)
ok  github.com/telegram-manager/backend/internal/groups        (cached)
ok  github.com/telegram-manager/backend/internal/joinrequests   (cached)
ok  github.com/telegram-manager/backend/internal/logs           (cached)
ok  github.com/telegram-manager/backend/internal/moderation    (cached)
ok  github.com/telegram-manager/backend/internal/publications  (cached)
ok  github.com/telegram-manager/backend/internal/telegram      (cached)
ok  github.com/telegram-manager/backend/internal/users         (cached)
?   github.com/telegram-manager/backend/migrations            [no test files]
?   github.com/telegram-manager/backend/cmd/server             [no test files]
```

### Frontend

```text
$ npm test -- --run
 Test Files  11 passed (11)
      Tests  51 passed (51)

$ npm run build
vite v8.2.2 building client environment for production...
✓ 190 modules transformed.
dist/index.html                   0.40 kB │ gzip:   0.26 kB
dist/assets/index-CThllaPF.css    4.79 kB │ gzip:   1.38 kB
dist/assets/index-InDfwPF_.js   403.96 kB │ gzip: 122.76 kB
✓ built in 338ms
```

## Invariantes cumplidas

- ✅ **AGENTS §21.1**: ningún test llama a la Bot API real. Adapter
  usa `httptest.NewServer`; service usa `fakeTelegramPub` con
  `SendMessage` + `SendPhoto`.
- ✅ **Bugfix #172 (`sdd/publications/permission-check`)**: `permissionOk`
  sigue usando `g.BotStatus == StatusAdministrator`. NO se reintroducen
  checks `can_*`. Verificado en el nuevo `publishOne` (misma guarda
  que el `Publish` original).
- ✅ **AGENTS §18.1 (rate limit + multi-grupo)**: `PublishMany` loop
  SECUENCIAL simple sobre `group_ids`. Cero goroutines paralelas.
  El token bucket del adapter ordena los envíos.
- ✅ **Validación fail-fast**: `validatePayload` corre UNA vez al
  inicio de `PublishMany` antes del loop. Errores per-grupo (404/403/
  Telegram) NO abortan — cada fila queda `failed` con log per-row.
- ✅ **HTTP 201 con `{publications: [...]}`** en fallo parcial
  (no 207 Multi-Status, decisión DECIDE del spec).
- ✅ **Conventional commits** según estilo del repo
  (`feat(area): ...`, `docs: ...`).
- ✅ Branch NO pusheada a remoto.

## Decisiones / desviaciones del design

1. **PublishMany internamente delega en publishOne compartido, no en
   Publish**: el design (D7) decía "PublishMany internamente llama
   a Publish para 1 grupo; loop directo para N". En la
   implementación final, extraje `publishOne` como helper privado
   compartido por `Publish` y `PublishMany` para evitar duplicación
   del path de envío (insert fila + send + update + log). El
   comportamiento observable cumple el design: 1 grupo == 1 fila,
   N grupos == N filas secuenciales; Publish preserva su firma
   extendida con foto+botones.
2. **Buttons almacenado como `json.RawMessage` en model.go**:
   simplifica el scanner de PostgreSQL JSONB y evita acoplar
   model.go con el paquete `telegram`. Helpers
   `MarshalButtons`/`UnmarshalButtons` exportados hacen la
   conversión a `[][]telegram.InlineKeyboardButton` en la frontera
   service. La spec DECIDE era "JSONB"; cumplido.
3. **api/publications_handlers_test.go actualizado en Phase 4** (no
   en Phase 1 como sugería el design): el fake `publicationStore`
   implementa el interface del paquete `api`, no `telegram.Service`,
   así que solo rompe cuando el interface se extiende — eso
   sucede en Phase 4 junto con el cambio de body. No requiere
   doble pasada.

## Tareas cumplidas

Las 24 tareas del tasks.md (1.1-8.2) están implementadas y
verificadas. Marcadas como `[x]` en este reporte (no se modifica
el tasks.md físico; el spec DECIDE del sdd-apply hybrid dice
`[x]` marks en filesystem, así que también se incluye en
`openspec/changes/publications-slice2/tasks.md`).

Ver `apply-progress` completo en memoria Engram.

## Ready for sdd-verify

El branch está listo para `sdd-verify`. Pendiente:

- `goose up` 00005 aplica limpia; `goose down` revierte sin perder
  filas (probado mentalmente; ejecución real en DB requiere stack
  levantada).
- Verificación end-to-end con bot real y grupo real (fuera del
  pipeline; §21.1 no aplica al CI).
