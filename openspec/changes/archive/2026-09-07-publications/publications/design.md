# Design: Publications Slice 1 — Publish-Now Text

## Technical Approach

Publications adds a `POST /api/publications` pipeline: create → validate →
check group + permission → insert `status=sending` → call `telegram.SendMessage`
→ update `sent`/`failed` + `message_id` → create log. Read endpoints
(`GET /api/publications`, `GET /api/publications/:id`) serve the listing and
detail. No scheduler in Slice 1.

## Architecture Decisions

### D1: `SendMessage` returns `message_id` via inline result struct

**Choice**: New file `internal/telegram/publications.go` with private
`sendMessageParams` + `sendMessageResult{MessageID int64}` struct, following
the same `params` pattern as `moderation.go`. The result is decoded by
passing `&sendMessageResult{}` to `doPost`, which already handles
`handleEnvelope` decoding into a `result any`.

**Alternatives considered**: Reusing `Message` from `update.go` (wrong
shape — it has `From`, `Chat`, `Text` we don't need).

**Rationale**: Isolation. The `sendMessage` response only needs
`message_id`. A dedicated result struct is minimal and explicit.

### D2: `publications.Service` interface dependencies

**Choice**: Four narrow interfaces:

- `GroupReader` — `GetByTelegramID` (same as `moderation.GroupPermissionReader`)
- `MessageSender` — `SendMessage(ctx, chatID, text, disablePreview) (int64, error)`
- `LogWriter` — `Create(ctx, *logs.Entry) error` (same as `moderation.LogWriter`)
- `PubStore` — `Create`, `GetByID`, `List`, `UpdateStatus` (publication CRUD)

**Alternatives considered**: One big interface for everything.

**Rationale**: Mirrors `moderation.Service` composition; keeps tests
isolated and fakes small.

### D3: Synchronous single-group send (accept ~1s latency)

**Choice**: `POST /api/publications` blocks on `SendMessage`; returns 201
only after the send completes. Single group per request.

**Alternatives considered**: Async + polling; multi-group batch.

**Rationale**: Single-group send is ~1s (acceptable for UX). Async
adds frontend polling complexity without benefit in Slice 1. Multi-group
is Slice 2 (sequential via token bucket already handles rate).

### D4: Permission check order: 404 then 403

**Choice**: Group existence (`404 NOT_FOUND`) checked before
`can_manage_chat` permission (`403 PERMISSION_DENIED`). Matches
`moderation.runAction` order.

**Rationale**: Consistent with existing error mapping and prevents
information leakage about non-existent groups.

### D5: `4096` char limit enforced in service, not handler

**Choice**: `publications.Service.Publish` validates `len(text) > 4096`
and returns `ErrTextTooLong`. Handler maps it to `400 VALIDATION_ERROR`.

**Rationale**: Business rule lives in the service layer, consistent with
how `moderation` keeps permission checks in the service.

### D6: Status on failure is `failed`, not `sending`

**Choice**: On `SendMessage` error, the row is updated to `status=failed`
with `error_message` populated. The `sending` status is transient
(only visible between insert and the Telegram call result).

**Rationale**: Prevents confusion about orphaned `sending` rows if the
process crashes between insert and update.

### D7: Hand-written fakes, not moq (matching existing tests)

**Choice**: `internal/publications/service_test.go` uses hand-written fakes
for `GroupReader`, `MessageSender`, `LogWriter`, `PubStore` — matching the
pattern in `moderation/service_test.go` (`fakeGroups`, `fakeTelegram`,
`fakeLogs`).

**Alternatives considered**: `github.com/matryer/moq`.

**Rationale**: The project already uses hand-written fakes; they're
explicit and avoid adding a code-generation dependency for Slice 1.
Can revisit when Slice 2 adds complexity.

## Data Flow

    Frontend                    API Handler              Service
       │                           │                       │
       │  POST /api/publications   │                       │
       │  {text, group_id}         │                       │
       │──────────────────────────>│                       │
       │                           │  validate text        │
       │                           │──────────────────────>│
       │                           │  GetByTelegramID      │
       │                           │──────────────────────>│
       │                           │  permissionOk         │
       │                           │──────────────────────>│
       │                           │  Create(status=sending)│
       │                           │──────────────────────>│
       │                           │  SendMessage          │
       │                           │──────────────────────>│
       │                           │  Telegram Bot API     │
       │                           │<──── message_id ──────│
       │                           │  UpdateStatus(sent, mid)│
       │                           │──────────────────────>│
       │                           │  Log(PUBLISH_MESSAGE)  │
       │                           │──────────────────────>│
       │  201 {data: publication}  │                       │
       │<──────────────────────────│                       │

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `backend/internal/telegram/publications.go` | Create | `sendMessageParams`, `sendMessageResult`, `SendMessage` on `*Adapter` |
| `backend/internal/telegram/service.go` | Modify | Add `SendMessage(ctx, chatID int64, text string, disablePreview bool) (int64, error)` to `Service` interface |
| `backend/internal/publications/model.go` | Create | `Publication` struct, `Status` type, `ErrTextTooLong` |
| `backend/internal/publications/repository.go` | Create | `Repository` (database/sql): `Create`, `GetByID`, `List`, `UpdateStatus` |
| `backend/internal/publications/service.go` | Create | `Service` struct, `Publish`, `GetByID`, `List` with interface deps |
| `backend/internal/api/publications_handlers.go` | Create | `handleCreatePublication`, `handleListPublications`, `handleGetPublication`, `respondPublicationError` |
| `backend/internal/api/server.go` | Modify | Add `publications publicationStore` field; add `WithPublications` Option |
| `backend/internal/logs/model.go` | Modify | Add `ActionPublishMessage = "PUBLISH_MESSAGE"` |
| `backend/migrations/00004_create_publications.sql` | Create | `publications` table (see schema below) |
| `backend/cmd/server/main.go` | Modify | Wire `publications.NewRepository` + `publications.NewService`; add `api.WithPublications` |
| `frontend/src/features/publications/types.ts` | Create | `Publication`, `CreatePublicationRequest` interfaces |
| `frontend/src/features/publications/api.ts` | Create | `createPublication`, `listPublications`, `getPublication` |
| `frontend/src/features/publications/hooks.ts` | Create | `usePublications`, `usePublication`, `useCreatePublication` |
| `frontend/src/features/publications/error.ts` | Create | `formatPublicationError` |
| `frontend/src/pages/PublicationsPage.tsx` | Create | List + create form page |
| `frontend/src/App.tsx` | Modify | Add `/publications` route inside `RequireAuth` |
| `backend/internal/publications/service_test.go` | Create | Unit tests with hand-written fakes |
| `backend/internal/api/publications_handlers_test.go` | Create | HTTP handler tests |
| `frontend/src/pages/PublicationsPage.test.tsx` | Create | Page tests with `mockFetchRoutes` |

## Migration Schema (`00004_create_publications.sql`)

```sql
-- +goose Up
CREATE TABLE publications (
    id             SERIAL PRIMARY KEY,
    telegram_id    BIGINT      NOT NULL REFERENCES groups(telegram_id),
    text           TEXT        NOT NULL CHECK (length(text) > 0),
    status         TEXT        NOT NULL DEFAULT 'draft'
                     CHECK (status IN ('draft','scheduled','sending','sent','failed')),
    message_id     BIGINT,
    scheduled_at   TIMESTAMPTZ,
    error_message  TEXT,
    actor_id       BIGINT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_publications_created_at ON publications (created_at DESC);
CREATE INDEX idx_publications_telegram_id ON publications (telegram_id);

-- +goose Down
DROP TABLE publications;
```

## Interfaces / Contracts

### Telegram `Service` interface addition

```go
// SendMessage envia texto a chatID y devuelve el message_id.
SendMessage(ctx context.Context, chatID int64, text string, disableWebPagePreview bool) (int64, error)
```

### Publications service interfaces

```go
type GroupReader interface {
    GetByTelegramID(ctx context.Context, id int64) (*groups.Group, error)
}

type MessageSender interface {
    SendMessage(ctx context.Context, chatID int64, text string, disableWebPagePreview bool) (int64, error)
}

type LogWriter interface {
    Create(ctx context.Context, e *logs.Entry) error
}

type PubStore interface {
    Create(ctx context.Context, p *Publication) error
    GetByID(ctx context.Context, id int64) (*Publication, error)
    List(ctx context.Context) ([]Publication, error)
    UpdateStatus(ctx context.Context, id int64, status Status, messageID *int64, errMsg *string) error
}
```

### Publication model

```go
type Status string
const (
    StatusDraft     Status = "draft"
    StatusScheduled Status = "scheduled"
    StatusSending   Status = "sending"
    StatusSent      Status = "sent"
    StatusFailed    Status = "failed"
)

type Publication struct {
    ID           int64
    TelegramID   int64
    Text         string
    Status       Status
    MessageID    *int64
    ScheduledAt  *time.Time
    ErrorMessage *string
    ActorID      *int64
    CreatedAt    time.Time
    UpdatedAt    time.Time
}
```

### API endpoints

| Method | Path | Body | Response |
|--------|------|------|----------|
| `POST` | `/api/publications` | `{text: string, group_id: int64}` | 201 `Publication` |
| `GET` | `/api/publications` | — | 200 `[]Publication` (max 50, `created_at DESC`) |
| `GET` | `/api/publications/:id` | — | 200 `Publication` |

### Error mapping (`respondPublicationError`)

| Domain error | HTTP | Code |
|-------------|------|------|
| `ErrTextTooLong` | 400 | `VALIDATION_ERROR` |
| `groups.ErrNotFound` | 404 | `NOT_FOUND` |
| `ErrBotPermission` | 403 | `PERMISSION_DENIED` |
| `telegram.ErrPermissionDenied` | 403 | `PERMISSION_DENIED` |
| `telegram.ErrTelegramNotFound` | 404 | `NOT_FOUND` |
| `telegram.ErrTelegramUnavailable` / `ErrWebhookConflict` | 502 | `TELEGRAM_ERROR` |
| `*telegram.TelegramAPIError` | 502 | `TELEGRAM_ERROR` |
| other | 500 | `INTERNAL_ERROR` |

### Frontend query keys and cache invalidation

```typescript
const publicationsKey = ['publications'] as const
const groupsKey = ['groups'] as const

// useCreatePublication.mutate → onSuccess:
//   qc.invalidateQueries({ queryKey: publicationsKey })
//   qc.invalidateQueries({ queryKey: groupsKey })  // optional: group metadata
```

### PublicationsPage structure

```
PublicationsPage
├── loading state: <p>Cargando publicaciones…</p>
├── error state: <div className="state-block state-error"> + Reintentar button
├── empty state: <p>No hay publicaciones</p>
├── list: <ul className="publication-list">
│   └── <li className="publication-card"> text (truncated) · group · status badge · date
└── create form:
    ├── <textarea> for text (maxLength=4096)
    ├── <select> for group (options from GET /api/groups)
    └── <button type="submit">Publicar</button>
```

## Testing Strategy

| Layer | Files | Cases |
|-------|-------|-------|
| Unit — service | `service_test.go` | Publish success → status=sent, message_id saved; Publish text too long → ErrTextTooLong; group not found → ErrGroupNotFound; bot no permission → ErrBotPermission + log; Telegram error → status=failed, error_message set, log TELEGRAM_ERROR; List returns rows; GetByID found/not found |
| Unit — handler | `publications_handlers_test.go` | POST 201 envelope; POST 400 empty text; POST 404 group; POST 403 permission; GET /:id 200; GET /:id 404; GET list 200 |
| Unit — telegram adapter | `telegram/publications_test.go` | SendMessage success decodes message_id; 429 retry (reuse existing httptest pattern from `moderation_test.go`) |
| Integration | `repository_test.go` (optional, same DB as existing integration tests) | Create + GetByID + List round-trip |
| Frontend | `PublicationsPage.test.tsx` | Loading, empty, error states; create success → list invalidates; create error shows message |

### Fake pattern for `service_test.go`

```go
type fakeTelegramPub struct {
    sent      []struct{ chatID int64; text string }
    messageID int64 // returned by SendMessage
    err       error
}

func (f *fakeTelegramPub) SendMessage(_ context.Context, chatID int64, text string, _ bool) (int64, error) {
    f.sent = append(f.sent, struct{ chatID int64; text string }{chatID, text})
    return f.messageID, f.err
}

type fakePubStore struct {
    pubs map[int64]*Publication
    nextID int64
}
// Create, GetByID, List, UpdateStatus — all in-memory
```

## Migration / Rollout

Goose migration `00004_create_publications.sql` runs automatically in
dev (`cfg.RunMigrations = true`). In production, manual step per AGENTS
§13.1. No data backfill needed; table is new.

No scheduler, no feature flags, no phased rollout in Slice 1.

## Open Questions (Resolved)

| Topic | Decision | Rationale |
|-------|----------|-----------|
| Permission check | `can_manage_chat` | Minimal bot admin permission for posting in a group (AGENTS §22 requires checking Bot API capabilities) |
| Status on failure | `failed` (not `sending`) | Prevents confusion about orphaned `sending` rows if process crashes |
| `message_id` storage | Column `message_id BIGINT NULL` | Non-null on success, null on failure; audit trail for future reference |
| 4096 char cap | Enforced in service, mapped to 400 VALIDATION_ERROR | Business rule in service layer; matches moderation pattern |
| Group existence vs permission order | 404 then 403 | Prevents information leakage; matches `moderation.runAction` |
| Mock strategy | Hand-written fakes | Matches existing `moderation/service_test.go`; no code-gen dependency for Slice 1 |
| `telegram_id` vs internal `id` for FK | FK to `groups.telegram_id` | `telegram_id` is the natural key used in all API routes and Telegram calls; `groups.id` is internal BD serial |
| `text` validation non-empty | `CHECK (length(text) > 0)` in SQL + service validation | Defense in depth: DB constraint catches any bypass of service validation |
