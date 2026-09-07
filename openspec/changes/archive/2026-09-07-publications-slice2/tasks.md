# Tasks: Publications — Slice 2 (Foto URL + Botones + Multi-Grupo + Filtro)

## Review Workload Forecast

| LOC | Budget 400 | Delivery |
|-----|-----------|----------|
| ~1240 | High | single-pr / size-exception |

```text
Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: size-exception
400-line budget risk: High
```

~1240 LOC excede 400. Precedente slice 1 (`size:exception` aprobado). Si prefiere partir: PR1 backend (Phases 1-5), PR2 frontend+README (Phases 6-8).

## Phase 1: Migration + Tipos + Interfaz + Fakes (compile green)

- [ ] 1.1 Crear `migrations/00005_alter_publications.sql` — Up ALTER ADD `photo_url TEXT`, `buttons JSONB`; Down DROP COLUMN
- [ ] 1.2 Crear `telegram/types_keyboard.go` — `InlineKeyboardButton{Text,URL}` + `InlineKeyboardMarkup{InlineKeyboard}` con tags `text`/`url`/`inline_keyboard`
- [ ] 1.3 Modificar `telegram/service.go` — extender `SendMessage(...,keyboard)`; agregar `SendPhoto(ctx,chatID,photoURL,caption,keyboard)`
- [ ] 1.4 Actualizar fakes `Service`: `poller_test.go`, `service_test.go`, `publications_handlers_test.go` — agregar `SendPhoto` + firma nueva `SendMessage`
- [ ] 1.5 Gate `go build` verde

## Phase 2: Adapter — SendPhoto + Keyboard

- [ ] 2.1 Modificar `telegram/publications.go` — extender `SendMessage` con keyboard; agregar `sendPhotoParams{ChatID,Photo,Caption,ReplyMarkup}` + `SendPhoto` (`doWithRetry`→`doPost`→decode `message_id`)
- [ ] 2.2 Modificar `telegram/publications_test.go` — +TestSendPhoto_Decodes, +TestSendPhoto_429, +TestSendMessage_WithKeyboard, +TestSendMessage_ReplyMarkupNil, +TestInlineKeyboard_JSON

## Phase 3: Publications Module

- [ ] 3.1 Modificar `publications/model.go` — `Publication{PhotoURL *string, Buttons}`; helpers `Marshal/UnmarshalButtons`; errores `ErrPhotoURLEmpty/Scheme/TooLong`, `ErrButtonsMalformed/Limit`, `ErrTooManyGroups`
- [ ] 3.2 Modificar `publications/repository.go` — `Create` inserta photo_url+buttons JSONB; `scanPublication` lee NULL→nil; agregar `ListByTelegramID(ctx,telegramID)`
- [ ] 3.3 Modificar `publications/service.go` — `validatePayload` fail-fast 400; `PublishMany(ctx,actorID,payload)` (1→`Publish`; N→loop); extender `MessageSender` (`SendPhoto`+keyboard) y `PubStore` (`ListByTelegramID`); log `ActionPublishMessage` per-row
- [ ] 3.4 Modificar `publications/service_test.go` — actualizar fakes; +tests PublishMany_OK_multi, orden_secuencial, fallo_Telegram/permiso, caption_1024, text_4096, validación_URL/botones/grupos, ListByTelegramID

## Phase 4: API Handlers

- [ ] 4.1 Modificar `api/publications_handlers.go` — body `createPublicationRequest{Text,PhotoURL,Buttons,GroupIDs}`; `publicationResponse` agrega `photo_url`+`buttons`; parsea `?group_id=`; map errores→400
- [ ] 4.2 Modificar `api/publications_handlers_test.go` — fake store con `PublishMany`+`ListByTelegramID`; +tests Create_OK_multi_201, OK_single_foto_botones, 400_group_ids_vacio, 400_URL_no_http, 400_botones_8x8, 403_uno_sin_admin, List_filtro

## Phase 5: Backend Suite Green

- [ ] 5.1 `go test ./...` verde; `go vet` y `gofmt` limpios

## Phase 6: Frontend

- [ ] 6.1 Modificar `features/publications/types.ts` — `Publication{photo_url,buttons}`; `PublishRequest{text,photo_url?,buttons?,group_ids[]}`; `InlineButton{text,url}`
- [ ] 6.2 Modificar `features/publications/api.ts` — `listPublications({group_id?})` query; `createPublication(PublishRequest)`
- [ ] 6.3 Modificar `features/publications/hooks.ts` — `usePublications({group_id?})` key `['publications', group_id ?? 'all']`; invalida `['publications']`
- [ ] 6.4 Modificar `features/publications/error.ts` — mensajes URL no http(s), >10 grupos, >8×8, sin grupos
- [ ] 6.5 Crear `features/publications/ButtonsEditor.tsx` — controlado (filas text+url; agregar/quitar fila/botón)
- [ ] 6.6 Modificar `pages/PublicationsPage.tsx` — form (photo_url + ButtonsEditor + multi-select checkboxes `useGroups()`; `maxLength` 1024/4096); listado `<img>` + chips + filtro
- [ ] 6.7 Modificar `pages/PublicationsPage.test.tsx` — +tests lista_foto_botones, filtro_recarga, formulario_multi_OK, validacion_URL_sin_POST, validacion_sin_grupos_sin_POST

## Phase 7: Frontend Suite Green

- [ ] 7.1 `npm test -- --run` y `npm run build` verdes

## Phase 8: README + Commit Final

- [ ] 8.1 Modificar `README.md` — sección "Publicaciones" (URL ≤5MB, caption ≤1024 con foto, texto ≤4096, ≤10 grupos, ≤8×8 botones, solo URL); fila `/publications`
- [ ] 8.2 Commit final conventional; NO push (single-pr local)

## Verification

Backend `go test`/`go vet`/`gofmt` y frontend `npm test -- --run`/`npm run build` verdes. `goose up` 00005 aplica; `goose down` revierte sin perder filas. `POST {group_ids:[g1,g2],photo_url,buttons}` → 201 con 2 filas `sent`. 400 antes de Telegram/DB; `GET ?group_id=X` filtra.