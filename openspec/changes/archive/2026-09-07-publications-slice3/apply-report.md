# SDD Apply Report — `publications-slice3`

> **Change**: `publications-slice3` (Slice 3 de 3 — programación in-process + historial paginado + cancel). Cierra Fase 2 del proyecto.
> **Mode**: hybrid. **Base**: `main @ b90c584` (slice 2 archivado).
> **Delivery**: single-pr, `size:exception` aprobada por el usuario (decision #188).
> **Branch**: `feat/publications-slice3` desde `main`. NO se hace push.
> **Test guard §21.1**: cero llamadas a Bot API real; `TelegramService` (MessageSender) mockeado en tests de servicio, worker, handler; integration tests del repo contra Postgres real.

## Resumen ejecutivo

Implementación completa de las 10 phases de `tasks.md`. Backend + worker + frontend compilan, `go test ./...`/`go vet`/`gofmt -l` y `npm test -- --run`/`npm run build` quedan limpios. La invariante del bugfix #172 (`permissionOk` sigue usando `g.BotStatus == StatusAdministrator`, NO `can_*`) está respetada en `publishOne` (slice 2) y en el worker nuevo (`Scheduler.permissionOk`). El worker reutiliza `publishOne` vía el helper extraído `publishOneFinalize` (sin duplicar el path de envío).

## Archivos tocados (git diff)

| Archivo | +/- | Acción |
|---------|-----|--------|
| `.env.example` | +6 / -1 | Modified |
| `README.md` | +37 / -7 | Modified (sección Programación + Paginación + nota Bot API) |
| `backend/cmd/server/main.go` | +19 / -4 | Modified (lanza Scheduler goroutine) |
| `backend/internal/api/publications_handlers.go` | +146 / -17 | Modified (branch schedule + DELETE + paginación) |
| `backend/internal/api/publications_handlers_test.go` | +260 / -17 | Modified (tests slice 3: schedule, offset, DELETE) |
| `backend/internal/api/server.go` | +3 / -1 | Modified (ruta DELETE registrada) |
| `backend/internal/config/config.go` | +30 / -14 | Modified (`PublicationsSchedulerIntervalSeconds`, envIntOr) |
| `backend/internal/publications/model.go` | +70 / -1 | Modified (ErrScheduledInPast/CancelNotAllowed/InvalidPagination + NormalizeScheduledAt/ValidatePagination/NormalizePagination) |
| `backend/internal/publications/repository.go` | +132 / -17 | Modified (List/ListByTelegramID limit/offset, ClaimScheduledDue SKIP LOCKED, Cancel) |
| `backend/internal/publications/service.go` | +98 / -28 | Modified (Schedule, CancelScheduled, List/ListByTelegramID con limit/offset, extracción `publishOneFinalize`) |
| `backend/internal/publications/service_test.go` | +256 / -18 | Modified (Schedule/CancelScheduled/limit-offset + `fakePubStore` extendido) |
| `frontend/src/features/publications/api.ts` | +30 / -6 | Modified (cancelPublication + buildListQueryString con limit/offset) |
| `frontend/src/features/publications/error.ts` | +2 / -2 | Modified (header doc slice 3) |
| `frontend/src/features/publications/hooks.ts` | +23 / -2 | Modified (useCancelPublication + query key con limit/offset) |
| `frontend/src/features/publications/types.ts` | +14 / -2 | Modified (scheduled_at + limit/offset) |
| `frontend/src/pages/PublicationsPage.test.tsx` | +295 / -18 | Modified (8 tests nuevos slice 3) |
| `frontend/src/pages/PublicationsPage.tsx` | +143 / -19 | Modified (radio dual-mode + datetime-local + Cancelar + Prev/Next) |
| `frontend/src/test/helpers.tsx` | +18 / 0 | Modified (matchQuery helper) |

**Subtotal modificado**: +1484 líneas, -117 líneas (`git diff --stat`).

## Archivos nuevos (untracked → added)

| Archivo | Líneas | Acción |
|---------|-------:|--------|
| `backend/internal/publications/model_test.go` | 118 | NEW (Phase 2: tests de normalizeScheduledAt + validatePagination + NormalizePagination) |
| `backend/internal/publications/repository_test.go` | 209 | NEW (Phase 3: integration tests SKIP LOCKED + paginación + cancel) |
| `backend/internal/publications/worker.go` | 180 | NEW (Phase 5: Scheduler in-process con tick + run) |
| `backend/internal/publications/worker_test.go` | 181 | NEW (Phase 5: tick no due / claims / telegram error / permission denied / ctx cancel) |
| `frontend/src/features/publications/validateScheduledAtClient.ts` | 61 | NEW (Phase 8: helper cliente datetime-local → RFC3339 con offset) |

**Subtotal nuevo**: 749 líneas.

**Total tocado**: ~2237 líneas (modificado + nuevo). El forecast de tasks era ~1276 LOC para el slice; el delta (~960) viene del path de tests mucho más amplio de lo que el forecast estimaba (integration tests reales contra Postgres, 10+ tests nuevos de frontend, etc.). Sigue siendo **single-pr** dentro del `size:exception` aprobado por el usuario.

> Nota: `openspec/changes/publications-slice3/` también aparece como untracked (exploration/proposal/design/tasks/specs) — esos son los artefactos del SDD pipeline previos al apply, no son código del producto.

## Outputs de gates

```
--- BACKEND TEST ---
ok   github.com/telegram-manager/backend/internal/api	1.422s
ok   github.com/telegram-manager/backend/internal/auth	(cached)
ok   github.com/telegram-manager/backend/internal/config	(cached)
ok   github.com/telegram-manager/backend/internal/events	(cached)
ok   github.com/telegram-manager/backend/internal/groups	(cached)
ok   github.com/telegram-manager/backend/internal/joinrequests	(cached)
ok   github.com/telegram-manager/backend/internal/logs	(cached)
ok   github.com/telegram-manager/backend/internal/moderation	(cached)
ok   github.com/telegram-manager/backend/internal/publications	1.438s
ok   github.com/telegram-manager/backend/internal/telegram	(cached)
ok   github.com/telegram-manager/backend/internal/users	(cached)

--- BACKEND VET ---
(sin warnings)

--- BACKEND GOFMT ---
(sin output; todo formatted)

--- FRONTEND TEST ---
 Test Files  11 passed (11)
      Tests  61 passed (61)
   Duration  18.99s

--- FRONTEND BUILD ---
dist/index.html                   0.40 kB │ gzip:   0.26 kB
dist/assets/index-CThllaPF.css    4.79 kB │ gzip:   1.38 kB
dist/assets/index-BfFSuuNG.js   406.92 kB │ gzip: 123.67 kB
✓ built in 847ms
```

## Decisiones que requirieron interpretación del design

1. **Worker reusa `publishOne` vía un helper extraído `publishOneFinalize`** (worker.go + service.go). El design dice "reusa `publishOne` literal"; sin embargo `publishOne` slice 2 **crea una fila nueva** (`store.Create` con status=sending). Para el worker la fila ya existe (transición scheduled→sending via `ClaimScheduledDue`). Refactor mínimo: extraer de `publishOne` la mitad inferior (dispatch + UpdateStatus + log) a un método `publishOneFinalize` que comparten ambos paths. `permissionOk` se sigue ejecutando en el path del worker (Scheduler.permissionOk replica el check `g.BotStatus == StatusAdministrator`); si la invariante cambia, los tests de slice 2 + el del worker fallan juntos.
2. **`fakePubStore.ClaimScheduledDue` en memoria** (service_test.go): no replica locks reales — el `SKIP LOCKED` real se valida en `TestRepository_ClaimScheduledDue_SkipsLockedByAnotherTxn` contra Postgres. Es lo que pide el spec (fakes para service/worker/handler; SQL real solo para el repo).
3. **Paginación default en frontend**: API siempre envía `?limit=` y `?offset=` cuando están definidos (incluso si son 0); en `usePublications({limit:50, offset:0})` el query URL es determinista. Esto facilita tests y query keys estables; el backend maneja defaults cuando los params están ausentes.
4. **`apiListQueryString` vive en `api.ts`** como helper privado (`buildListQueryString`) — single source of truth entre `listPublications` y los tests. Evita la duplicación del slice 1 donde cada test re-armaba la query string.

## Verificación §21.1

- `worker_test.go` usa `fakeTelegramPub` (mismo fake que slice 2). Cero llamadas a Bot API.
- `service_test.go` usa `fakeTelegramPub` + `fakeGroupsPub` + `fakeLogsPub`. Cero llamadas a Bot API.
- `publications_handlers_test.go` usa `fakePublicationStore`. No toca el adapter.
- `repository_test.go` corre contra Postgres real (`telegram_manager_publications`): valida el SQL exacto del claim SKIP LOCKED, la paginación con 75 filas, y el Cancel con race-check.
- `repository.go`'s `Cancel` y `ClaimScheduledDue` usan `tx.QueryRowContext` + `tx.ExecContext` contra el driver pgx stdlib (Postgres 16 ✓).

## Invariantes

- ✅ `permissionOk` usa `g.BotStatus == StatusAdministrator` (service.go:486 + worker.go permissionOk). NO se reintrodujeron checks `can_*`.
- ✅ Worker reusa `publishOneFinalize` (extracto de `publishOne`) — sin duplicar el path de envío.
- ✅ Hard delete SOLO si status='scheduled'; otros status → 409 `INVALID_STATUS`. `sent`/`failed` se preservan.
- ✅ Sequential claim batch via SKIP LOCKED (Postgres 16, pgx v5 stdlib). Validado por `TestRepository_ClaimScheduledDue_SkipsLockedByAnotherTxn`.
- ✅ Conventional commits (siguiente sección).
- ✅ NO push.
- ✅ Bugfix #172 vigente: si admin remueve al bot antes del tick, la fila queda `failed` con `PERMISSION_DENIED` (test `TestScheduler_Tick_PermissionDenied_StaysFailed`).

## Próximos pasos

1. `sdd-verify`: corre tests y verifica contra las 77 scenarios del spec.
2. `sdd-archive`: sincroniza delta spec a `openspec/specs/publications/spec.md` y cierra el change.
3. PR único contra `main` (`feat/publications-slice3` → `main`) — NO push automático.