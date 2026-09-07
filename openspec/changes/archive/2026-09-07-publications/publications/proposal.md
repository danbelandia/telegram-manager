# Proposal: Publications — Slice 1 (Publish Now, Text)

## Intent

Phase 2 of the Telegram Group Manager adds publications (AGENTS §22). This slice delivers the minimal end-to-end pipeline: create a text publication and publish it immediately to one group, with audit logging. The table design accommodates future slices (scheduling, media) without migration rewrites.

## Scope

### In Scope
- `publications` table (00004) with `status` column supporting the full lifecycle: `draft | scheduled | sending | sent | failed`; `scheduled_at` nullable
- `SendMessage` on the Telegram adapter (`chat_id`, `text`, `disable_web_page_preview`)
- New module `internal/publications/` — model, repository, service, handlers
- API: `POST /api/publications` (create + publish immediately to one group), `GET /api/publications`, `GET /api/publications/:id`
- `ActionPublishMessage` log constant; every successful publish writes an audit entry
- Frontend: `features/publications/` (types/api/hooks/error) + `PublicationsPage` (list + create form with group selector)
- Backend tests: repository, service, handler — TelegramService mocked

### Out of Scope
- Media (sendPhoto) — Slice 2
- Inline keyboard buttons — Slice 2
- Multi-group sequential publishing — Slice 2
- Scheduling, scheduler worker, `scheduled_at` population — Slice 3
- Publication detail page (list suffices in Slice 1)

## Capabilities

### New Capabilities
- `publications-publish-now`: Create a text publication, send it immediately to one Telegram group, log the action, return the publication record.

### Modified Capabilities
- `telegram-moderation` (delta): `Service` interface gains `SendMessage`; adapter implements it. No behavior change to existing methods.

## Approach

**Synchronous single-group send.** `POST /api/publications` receives `{text, group_id}` where `group_id` is `groups.telegram_id`. The service: (1) validates text non-empty, ≤4096 chars, group exists; (2) inserts row with status `sending`; (3) calls `telegram.SendMessage`; (4) on success updates status to `sent`, stores `message_id` from Telegram response; (5) writes audit log. On failure: status `failed`, `error_message` populated, log with `TELEGRAM_ERROR`.

Sync is acceptable for one group (~1 sec). Multi-group in Slice 2 will need sequential dispatch with duration awareness.

### Key Decisions
| Decision | Choice | Rationale |
|----------|--------|-----------|
| Table design | Single `publications` table, full status enum now | Avoids migration rewrite in Slice 3 |
| Send mode | Synchronous, one group per request | Simple, reviewable, matches current pattern |
| Adapter method | `SendMessage(ctx, chatID int64, text string, disableWebPagePreview bool) (int64, error)` | Returns `message_id` for logging; minimal params |
| Auth | `requireAuth` on all 3 endpoints | Consistent with existing API |
| Validation | text non-empty + ≤4096 + group exists (404) | Telegram Bot API limit |

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `backend/internal/telegram/service.go` | Modified | Add `SendMessage` to `Service` interface |
| `backend/internal/telegram/moderation.go` | Modified | Add `sendMessageParams` struct + `SendMessage` impl |
| `backend/internal/publications/` | New | model, repository, service, handlers |
| `backend/internal/api/server.go` | Modified | Add `WithPublications` option |
| `backend/internal/api/publications_handlers.go` | New | 3 handlers: create, list, get |
| `backend/internal/logs/model.go` | Modified | Add `ActionPublishMessage` constant |
| `backend/migrations/00004_create_publications.sql` | New | publications table |
| `backend/cmd/server/main.go` | Modified | Wire publications module |
| `frontend/src/features/publications/` | New | types, api, hooks, error |
| `frontend/src/pages/PublicationsPage.tsx` | New | List + create form |
| `frontend/src/App.tsx` | Modified | Add `/publications` route |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Group `telegram_id` not found in DB (bot not in group) | Medium | 404 response; frontend shows clear error |
| Telegram sendMessage fails (bot not admin) | Low | Adapter maps to `ErrPermissionDenied`; logged |
| Sync request timeout on slow Telegram response | Low | `doPost` has 30s default timeout; acceptable for one message |

## Rollback

- Revert migration `00004` via `goose down`
- Remove `WithPublications` from `main.go` wiring
- Remove `SendMessage` from interface + adapter (safe: no other callers)
- Delete `internal/publications/` module and frontend feature
- No data to preserve (publications table is new)

## Dependencies

- Existing `groups.telegram_id` (migration 00002) — FK target
- `logs.Entry` + `logs.Repository.Create` (migration 00003) — audit writes
- `telegram.Service.SendMessage` — new, self-contained

## Success Criteria

- [ ] `docker compose up` runs with migration 00004 applied
- [ ] `POST /api/publications` with valid text + group_id creates a row and sends the message (status `sent`, `message_id` stored)
- [ ] `GET /api/publications` returns the list; `GET /api/publications/:id` returns detail
- [ ] Audit log entry created for every publish attempt
- [ ] Backend tests pass: repository (CRUD), service (mocked Telegram), handler (HTTP)
- [ ] Frontend: PublicationsPage renders list, create form sends POST, invalid group shows error
- [ ] `go vet ./...` and `npm run build` pass
