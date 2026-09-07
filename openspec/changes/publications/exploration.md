# Exploration: Phase 2 Publications

> Change: `publications` (Slice 1: publish-now text). Modo hybrid. Change path: `openspec/changes/publications/`.

## Current State

**Telegram Adapter** (`backend/internal/telegram/`):
- `Service` interface (service.go) wraps: GetMe, GetUpdates, SetWebhook, DeleteWebhook, BanUser, UnbanUser, MuteUser, UnmuteUser, DeleteMessage, PinMessage, LockGroup, UnlockGroup, ApproveJoinRequest, RejectJoinRequest, GetChatMember, GetChatAdministrators.
- Pattern para agregar métodos: params struct privado (ej. `banChatMemberParams`) + método en `*Adapter` envolviendo `doWithRetry` → `doPost` → `handleEnvelope`. No existen `sendMessage`/`sendPhoto` todavía.
- `doPost` (adapter.go:155) maneja JSON POST, token bucket (~25 req/s), deadlines y envelope. `doWithRetry` (adapter.go:268) maneja 429 con max 3 reintentos.
- Rate limiter global único: multi-grupo debe enviarse secuencialmente (AGENTS §18.1).

**API Layer** (`backend/internal/api/`):
- `Server` con patrón Option: `WithAuth`, `WithGroups`, `WithModeration`, `WithJoinRequests`, `WithLogs`, `WithWebhook`. Publications agregará `WithPublications`.
- Handler pattern: `pathID`, `actorIDFromClaims`, `decodeOptionalBody`, `respondModerationError`, `respond(w, status, data)`.
- Envelope: `{"data": T, "error": {"code", "message"}}`.

**Moderation Service** (`backend/internal/moderation/service.go`):
- `runAction` (grupo → permiso → telegram → log) es para acciones sobre un grupo existente. Publications tiene flujo distinto (el admin elige los grupos) pero reutiliza `LogWriter.Create`.
- `Logs.Entry` con `Action`, `Metadata` JSONB, `Status`, `ErrorMessage` — patrón a reutilizar.

**Groups** (`backend/internal/groups/`):
- FK de publications referencia `groups.telegram_id` (ID natural para llamadas a Telegram).

**Migrations** (`backend/migrations/`):
- Convención `00001_create_admins.sql`, `00002_create_groups.sql`, `00003_create_moderation_tables.sql`; `-- +goose Up` / `-- +goose Down`. Siguiente: `00004_*.sql`.

**Frontend**:
- `lib/api-client.ts`: `request<T>(path, init)` con normalización de envelope, refresh 401 y `ApiError`.
- Feature pattern (`features/moderation/`): `types.ts`, `api.ts`, `hooks.ts`, `error.ts`.
- Pages con `useParams`, `useQuery`/`useMutation`, `window.confirm`.
- Rutas actuales registradas en `App.tsx`: `/login`, `/dashboard`, `/groups`, `/groups/:id`, `/groups/:id/users`, `/groups/:id/requests`, `/groups/:id/logs`. Nuevas: `/publications`, `/publications/:id`.
- Stack: React 19, TanStack Query 5, react-router-dom 7, react-hook-form + zod, sin librería de componentes.

**Telegram Bot API** (`docs/telegram_api_reference.md`):
- `sendMessage`: chat_id, text, parse_mode, reply_markup (InlineKeyboardMarkup), disable_web_page_preview, disable_notification → devuelve `Message.message_id`.
- `sendPhoto`: photo (URL o file_id), caption, parse_mode, reply_markup. Para MVP, URL pública (sin endpoint de upload).
- `InlineKeyboardMarkup`: `inline_keyboard` (array de arrays de `InlineKeyboardButton`: text + url/callback_data).
- **NO existe `schedule_date` para bots enviando a grupos**: la programación debe hacerse in-process (worker Go con `time.Ticker`, sin Redis).
- Rate limits: ~1 msg/seg por chat, ~30 req/seg global.

## Affected Areas

- `backend/internal/telegram/service.go` — agregar `SendMessage` (y `SendPhoto` en slice 2) a la interfaz.
- `backend/internal/telegram/adapter.go` — implementar con params structs + `doWithRetry` + `doPost`.
- `backend/internal/publications/` (nuevo) — model, repository, service, handlers.
- `backend/internal/api/server.go` — opción `WithPublications` y rutas nuevas.
- `backend/internal/api/publications_handlers.go` (nuevo) — endpoints de publicaciones.
- `backend/internal/logs/model.go` — constantes `ActionPublishMessage`, `ActionScheduleMessage`.
- `backend/migrations/00004_create_publications.sql` (nuevo) — tabla `publications`.
- `backend/cmd/server/main.go` — wiring del módulo y (slice 3) arranque del worker scheduler.
- `frontend/src/features/publications/` (nuevo) — types/api/hooks/error.
- `frontend/src/pages/PublicationsPage.tsx` + `PublicationDetailPage.tsx` (nuevos).
- `frontend/src/App.tsx` — rutas `/publications` y `/publications/:id`.

## Approaches

1. **Tres slices incrementales (RECOMENDADO)**
   - **Slice 1 — Publish Now, texto**: `sendMessage` en adapter, tabla `publications`, módulo `internal/publications`, `POST /api/publications` (crear + publicar ya), `GET /api/publications`, `GET /api/publications/:id`; página de listado + formulario (texto + selector de grupos); log por envío.
   - **Slice 2 — Media + botones + multi-grupo**: `sendPhoto`, `photo_url`, `reply_markup` (inline keyboard text+url), envío secuencial multi-grupo; UI con URL de foto y constructor de botones.
   - **Slice 3 — Programación + historial**: worker Go (ticker 30s) polling de `publications` con `status='scheduled' AND scheduled_at <= now()`; UI con datetime picker, cancelación y badges de estado.
   - Pros: cada slice ~200-350 líneas, reviewable, coincide con el patrón histórico del repo (changes chicos por fase), menor riesgo por entrega.
   - Cons: tres ciclos SDD completos.
   - Effort: Medium por slice.

2. **Un solo cambio monolítico**: todo publications en un change.
   - Pros: un solo ciclo SDD.
   - Cons: ~800-1000+ líneas, excede el budget de 400 líneas de review (requeriría chained PRs), verificación incremental difícil.
   - Effort: High.

3. **Dos slices**: Slice 1 = todo el publish inmediato (texto+media+botones+multi), Slice 2 = scheduling+historial.
   - Pros: dos mitades limpias.
   - Cons: slice 1 sigue siendo ~500-600 líneas, probablemente requiere chained PRs.
   - Effort: Medium-High.

**Scheduling (para slice 3)**: goroutine Go con `time.Ticker` (30s) consultando `SELECT * FROM publications WHERE status='scheduled' AND scheduled_at <= now()` y enviando vía adapter. Sin librería cron: solo se necesita polling de una query, no expresiones cron complejas. Shutdown graceful con context.

**Data model**: **una sola tabla** `publications` con `status` (`draft | scheduled | sending | sent | failed`) y `error_message`; `scheduled_at` nullable (NULL = inmediato). No crear `scheduled_publications` separada: sobre-complica el esquema para una única dimensión de estado.

## Recommendation

**Approach 1 — Tres slices.** Se alinea con cómo se construyó el repo (cada change ~200-400 líneas, verificable y reviewable de forma autónoma). Cada slice entrega valor visible:

- Slice 1 (publish-now text): pipeline completo end-to-end (crear → publicar → log → historial básico).
- Slice 2 (media-buttons-multi): `sendPhoto`, inline keyboards, multi-grupo secuencial (el token bucket del adapter ya lo limita).
- Slice 3 (schedule-history): scheduler in-process, cancelación, historial con badges.

## Risks

- **sendPhoto con URL**: requiere URL pública accesible; si la URL es lenta o cae, el envío falla. Documentar; sin endpoint de upload en el MVP.
- **callback_data limit (64 bytes)**: para el MVP usar botones de URL, evitando el límite.
- **Recuperación del scheduler**: un restart deja `scheduled` con `scheduled_at` pasado; el próximo tick los envía. Comportamiento correcto, sin lógica extra.
- **Multi-grupo bloquea HTTP**: N envíos secuenciales ≈ 1 seg/chat. Evitar bloquear el request 30+ seg: considerar crear+publicar async con estado `sending` y polling del frontend (decidir en design).
- **Sin scheduling nativo**: confirmado que la Bot API no programa envíos a grupos; todo es in-process. Si el backend está caído a la hora programada, el envío se atrasa al próximo tick. Aceptable para MVP.

## Ready for Proposal

**Sí.** Informar al usuario:

> Fase 2 — Publicaciones se entrega en 3 slices:
> 1. **Publish Now — Texto** (publicación inmediata de texto a los grupos seleccionados)
> 2. **Media + Botones + Multi-grupo** (foto, teclados inline, multi-grupo secuencial)
> 3. **Programación + Historial** (worker in-process, estados, historial)
>
> Cada slice es reviewable de forma autónoma. ¿Arrancamos con el Slice 1 (publish-now texto)?