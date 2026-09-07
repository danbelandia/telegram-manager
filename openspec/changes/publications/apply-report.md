# Apply Report — Publications (Slice 1: Publish-Now Text)

**Change**: publications
**Mode**: Standard (no Strict TDD — strict_tdd disabled per project config)
**Delivery**: single-pr, `size:exception` APPROVED by user ("Todo en un solo PR.")
**Branch**: `feat/publications`
**Base**: `main` (`c835739` docs: guia de ejecucion local y uso del panel)

## Summary

Implementación completa del slice 1 de publicaciones (publish-now text): tabla
`publications`, `SendMessage` en el adapter de Telegram, módulo
`internal/publications` (model + repo + service), handlers REST + wiring, y
pantalla `/publications` en el panel con creación, listado y tests.

## Tasks Completed

### Phase 1: Database Migration
- [x] 1.1 `backend/migrations/00004_create_publications.sql` — tabla `publications` + índice `created_at DESC` + índice `telegram_id`, CHECK `length(text)>0` y `status IN (...)`, FK a `groups.telegram_id`.

### Phase 2: Telegram Adapter — SendMessage
- [x] 2.1 `Service.SendMessage` en la interfaz (`internal/telegram/service.go`).
- [x] 2.2 `internal/telegram/publications.go` — `sendMessageParams` + `sendMessageResult` + `(*Adapter).SendMessage` vía `doWithRetry`→`doPost`.
- [x] 2.3 `internal/telegram/publications_test.go` — httptest: éxito decodifica message_id + 429 reintenta (2 llamadas).

### Phase 3: Publications Module
- [x] 3.1 `internal/publications/model.go` — `Status` enum (5 valores), `Publication`, `ErrTextTooLong`, `ErrTextEmpty`, `ErrNotFound`.
- [x] 3.2 `internal/publications/repository.go` — `Create`, `GetByID`, `List` (max 50, desc), `UpdateStatus`.
- [x] 3.3 `internal/publications/service.go` — 4 interfaces estrechas (GroupReader, MessageSender, LogWriter, PubStore) + `Publish` / `GetByID` / `List`.

### Phase 4: API Handlers + Wiring
- [x] 4.1 `backend/internal/api/publications_handlers.go` — 3 handlers + `respondPublicationError`.
- [x] 4.2 `backend/internal/api/server.go` — campo `publications` + `WithPublications` + rutas.
- [x] 4.3 `backend/internal/logs/model.go` — `ActionPublishMessage = "PUBLISH_MESSAGE"`.
- [x] 4.4 `backend/cmd/server/main.go` — repo + service + `WithPublications` en webhook y polling.

### Phase 5: Backend Tests
- [x] 5.1 `internal/publications/service_test.go` — 8 casos con fakes hand-written.
- [x] 5.2 `internal/api/publications_handlers_test.go` — POST/GET con envelope y códigos HTTP.
- [x] 5.3 `go test ./...` verde + gofmt/vet limpio.

### Phase 6: Frontend Feature
- [x] 6.1 `frontend/src/features/publications/types.ts`
- [x] 6.2 `frontend/src/features/publications/api.ts`
- [x] 6.3 `frontend/src/features/publications/hooks.ts` (`usePublications`, `usePublication`, `useCreatePublication`)
- [x] 6.4 `frontend/src/features/publications/error.ts`

### Phase 7: Frontend Page + Route
- [x] 7.1 `frontend/src/pages/PublicationsPage.tsx`
- [x] 7.2 `frontend/src/App.tsx` — ruta `/publications` bajo `RequireAuth` (+ link en `Layout.tsx`).

### Phase 8: Frontend Tests
- [x] 8.1 `frontend/src/pages/PublicationsPage.test.tsx` — 6 casos con mockFetchRoutes (y mock propio donde GET/POST comparten path).
- [x] 8.2 `npm test -- --run` verde (49 tests).

## Files Changed

| File | Action | What Was Done |
|------|--------|---------------|
| `backend/migrations/00004_create_publications.sql` | Created | Tabla publications + índices + CHECKs |
| `backend/internal/telegram/service.go` | Modified | `SendMessage` en interfaz `Service` |
| `backend/internal/telegram/publications.go` | Created | Adapter sendMessage vía doWithRetry/doPost |
| `backend/internal/telegram/publications_test.go` | Created | 2 tests adapter (éxito, 429 retry) |
| `backend/internal/telegram/poller_test.go` | Modified | `fakeService.SendMessage` (compila tras nuevo método de interfaz) |
| `backend/internal/publications/model.go` | Created | Status, Publication, errores |
| `backend/internal/publications/repository.go` | Created | Repo publicaciones |
| `backend/internal/publications/service.go` | Created | Servicio de flujo Publish |
| `backend/internal/publications/service_test.go` | Created | 8 tests de servicio con fakes |
| `backend/internal/api/publications_handlers.go` | Created | 3 handlers REST |
| `backend/internal/api/publications_handlers_test.go` | Created | Tests de handlers + envelope |
| `backend/internal/api/server.go` | Modified | `WithPublications` + rutas |
| `backend/internal/logs/model.go` | Modified | `ActionPublishMessage` |
| `backend/cmd/server/main.go` | Modified | Wiring publicaciones (webhook + polling) |
| `backend/internal/groups/repository_test.go` | Modified | `TRUNCATE groups CASCADE` (FK de publications) |
| `frontend/src/features/publications/types.ts` | Created | Tipos di dominio |
| `frontend/src/features/publications/api.ts` | Created | create/list/get à través de request<T> |
| `frontend/src/features/publications/hooks.ts` | Created | usePublications/usePublication/useCreatePublication |
| `frontend/src/features/publications/error.ts` | Created | formatPublicationsError |
| `frontend/src/pages/PublicationsPage.tsx` | Created | Lista + formulario de creación |
| `frontend/src/pages/PublicationsPage.test.tsx` | Created | 6 tests con mockFetchRoutes |
| `frontend/src/App.tsx` | Modified | Ruta `/publications` bajo RequireAuth |
| `frontend/src/components/Layout.tsx` | Modified | Link "Publicaciones" en sidebar |

## Test Output

### Backend — `go test ./...` (todo verde)
```
ok  internal/api
ok  internal/auth
ok  internal/groups
ok  internal/joinrequests
ok  internal/logs
ok  internal/moderation
ok  internal/publications
ok  internal/telegram
ok  internal/users
```
`go vet ./...` limpio, `gofmt -l .` limpio.

### Frontend — `npm test -- --run`
```
Test Files 11 passed, Tests 49 passed
```
`npm run build` (tsc --noEmit + vite build) OK.

## Deviations from Design

1. **División del error de texto (ErrTextEmpty)**. El design mapeaba texto vacío
   y texto demasiado largo a un único `ErrTextTooLong`. El spec (§POST) pide
   mensajes distintos ("texto no puede estar vacio" vs "texto excede 4096
   caracteres"). Se agregó `ErrTextEmpty` en `model.go` y el servicio valida
   ambas condiciones por separado; ambos mapean a 400 VALIDATION_ERROR con su
   mensaje propio. Sin impacto en contrato (mismo código HTTP), solo mejora la
   precisión del mensaje que exige el spec.
2. **`usePublication(id)` en hooks**. El task 6.3 pide el hook de detalle;
   la página `/publications` del slice 1 no lo usa directamente (no hay vista
   de detalle), pero se incluye por completitud del feature module y es lo que
   consumirá el detalle posterior.
3. **Naming helper de error**: el task 6.4 decía `formatPublicationError`;
   se usó `formatPublicationsError` por consistencia con
   `features/moderation/error.ts` (`formatModerationError`). Es una re-exportación
   del mensaje del backend; sin impacto funcional.

## Issues Found / Fixed

- **`fakeService` en `poller_test.go`** no compilaba tras añadir `SendMessage` a
  la interfaz `Service`: se agregó el método stub. (Fase 2 / telegram).
- **Tests de `internal/groups` fallaban** con `TRUNCATE groups cannot truncate a
  table referenced in a foreign key constraint`: la tabla `publications` (00004)
  agrega una FK a `groups.telegram_id`. Se cambió a `TRUNCATE groups CASCADE`.
- **`go vet` en `telegram/publications_test.go`**: `t.Errorf` con argumento sin
  directiva de formato; se añadió `%v`.

## Remaining Tasks

Ninguna — todas las tareas del slice 1 están completas. Siguiente fase de SDD:
**sdd-verify**.

## Workload / PR Boundary

- Mode: single PR, `size:exception`
- Current work unit: N/A (un solo PR)
- Boundary: `main` → `feat/publications`, todo el slice 1
- Estimated review budget impact: alto (~2480 lineas) — aprobado explícitamente
  por el usuario como `size:exception`; no se dividió en PRs encadenados.

## Status

22/22 tasks complete. **Ready for verify.**
