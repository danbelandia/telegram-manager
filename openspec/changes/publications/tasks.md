# Tasks: Publications — Slice 1: Publish-Now Text

## Review Workload Forecast

| Campo | Valor |
|-------|-------|
| Líneas estimadas | ~550-650 (10 archivos nuevos, 4 modificados, migración, tests) |
| Riesgo budget 400 líneas | High |
| PRs encadenados recomendados | Yes |
| Split sugerido | PR1 backend completo → PR2 frontend + tests |
| Delivery strategy | single-pr |
| Chain strategy | size-exception |

```text
Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: size-exception
400-line budget risk: High
```

**Nota**: El delivery strategy es `single-pr`, pero el budget estimado (550-650
líneas) excede 400. El orchestrator debe requerir aprobación `size:exception`
antes de aplicar, o el usuario puede optar por PRs encadenados.

### Work Units

| Unit | Goal | PR | Base | Verification |
|------|------|----|------|--------------|
| 1 | Backend: migration + telegram SendMessage + publications module + handlers + wiring + tests | PR1 | main | `go test ./backend/...` verde |
| 2 | Frontend: feature + page + route + tests | PR2 | main | `npm test -- --run` verde |

## Phase 1: Database Migration

- [x] 1.1 Crear `backend/migrations/00004_create_publications.sql` con tabla `publications` (id, telegram_id FK→groups.telegram_id, text, status enum 5 valores, message_id nullable, scheduled_at nullable, error_message nullable, actor_id nullable, created_at, updated_at) + índices por created_at DESC y telegram_id; incluir `CHECK (length(text) > 0)` y `CHECK (status IN (...))`

## Phase 2: Telegram Adapter — SendMessage

- [x] 2.1 Agregar `SendMessage(ctx, chatID int64, text string, disableWebPagePreview bool) (int64, error)` a `internal/telegram/service.go` (interfaz `Service`)
- [x] 2.2 Crear `internal/telegram/publications.go` con `sendMessageParams` struct + `sendMessageResult{MessageID int64}` + método `(*Adapter).SendMessage` pasando por `doWithRetry`→`doPost`→`&sendMessageResult{}`
- [x] 2.3 Crear `internal/telegram/publications_test.go` con httptest: caso éxito decodifica message_id; caso 429 con retry_after=2 reintenta

## Phase 3: Publications Module (model + repo + service)

- [x] 3.1 Crear `internal/publications/model.go`: tipo `Status`, constantes (Draft/Scheduled/Sending/Sent/Failed), struct `Publication` con todos los campos, `ErrTextTooLong`
- [x] 3.2 Crear `internal/publications/repository.go`: `Repository` con `Create`, `GetByID`, `List` (max 50, created_at DESC), `UpdateStatus` (status + message_id nullable + error_message nullable) — patrón `logs/repository.go`
- [x] 3.3 Crear `internal/publications/service.go`: `Service` con interfaces estrechas `GroupReader`, `MessageSender`, `LogWriter`, `PubStore`; método `Publish(ctx, actorID, groupID, text)` (validar text → GetByTelegramID → permissionOk `can_manage_chat` → Create(sending) → SendMessage → UpdateStatus(sent/failed) + log); métodos `GetByID`, `List`

## Phase 4: API Handlers + Wiring

- [x] 4.1 Crear `backend/internal/api/publications_handlers.go`: `handleCreatePublication` (decode {text, group_id}, llamar Publish, map errors → 400/403/404/502/500), `handleListPublications`, `handleGetPublication`, `respondPublicationError` — patrón `moderation_handlers.go`
- [x] 4.2 Modificar `backend/internal/api/server.go`: +campo `publications publicationStore`, +interface `publicationStore` (Publish, GetByID, List), +`WithPublications` option montando POST/GET /api/publications y GET /api/publications/{id}
- [x] 4.3 Modificar `backend/internal/logs/model.go`: agregar `ActionPublishMessage = "PUBLISH_MESSAGE"`
- [x] 4.4 Modificar `backend/cmd/server/main.go`: import publications, crear `publications.NewRepository(db)` + `publications.NewService(groupsRepo, bot, pubsRepo, logsRepo)`, agregar `api.WithPublications(pubsService)` en ambos bloques webhook y polling

## Phase 5: Backend Tests

- [x] 5.1 Crear `backend/internal/publications/service_test.go` con fakes hand-written (fakeTelegramPub, fakePubStore, fakeGroupsPub, fakeLogsPub): test Publish success → status=sent, message_id saved; Publish text too long → ErrTextTooLong; group not found → ErrGroupNotFound; bot no permission → ErrBotPermission + log; Telegram error → status=failed, error_message set, log TELEGRAM_ERROR; List returns rows; GetByID found/not found
- [x] 5.2 Crear `backend/internal/api/publications_handlers_test.go` con fakes: POST 201 envelope; POST 400 empty text; POST 404 group; POST 403 permission; GET /:id 200; GET /:id 404; GET list 200
- [x] 5.3 Verificar suite completa: `go test ./...` verde + gofmt/vet limpio

## Phase 6: Frontend Feature

- [x] 6.1 Crear `frontend/src/features/publications/types.ts`: interfaces `Publication`, `CreatePublicationRequest`
- [x] 6.2 Crear `frontend/src/features/publications/api.ts`: `createPublication`, `listPublications`, `getPublication` pasando por `request<T>` de lib/api-client
- [x] 6.3 Crear `frontend/src/features/publications/hooks.ts`: `usePublications`, `usePublication`, `useCreatePublication` con invalidación de cache publicationsKey
- [x] 6.4 Crear `frontend/src/features/publications/error.ts`: `formatPublicationError` helper

## Phase 7: Frontend Page + Route

- [x] 7.1 Crear `frontend/src/pages/PublicationsPage.tsx`: lista de publicaciones (loading/empty/error states) + formulario crear (textarea text maxLength=4096, select de grupos desde GET /api/groups, botón Publicar) — patrón GroupsPage
- [x] 7.2 Modificar `frontend/src/App.tsx`: import PublicationsPage + agregar ruta `/publications` dentro de RequireAuth

## Phase 8: Frontend Tests

- [x] 8.1 Crear `frontend/src/pages/PublicationsPage.test.tsx` con mockFetchRoutes: test carga exitosa, lista vacía, error de carga, creación exitosa con invalidación, error al crear con mensaje visible
- [x] 8.2 Verificar suite: `npm test -- --run` verde

## Verification

- `cd backend && go test ./...` — todos verdes
- `cd frontend && npm test -- --run` — todos verdes
- `go vet ./...` limpio
- `docker compose build` — imagen backend y frontend compilan
