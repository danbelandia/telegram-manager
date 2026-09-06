# Tasks: telegram-events

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~700 (6 nuevos, 8 modificados) |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR A transporte → PR B bus+poller → PR C webhook+wiring |
| Delivery strategy | ask-on-risk |
| Chain strategy | feature-branch-chain (elegida en repo-bootstrap; rama `feat/telegram-events` base = `feat/repo-bootstrap`) |

Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: feature-branch-chain
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Base | Notes |
|------|------|-----------|------|-------|
| 1 | Modelo Update + transporte en Adapter (specs 1) | PR A | `feat/repo-bootstrap` | tests con stub httptest |
| 2 | Bus + Poller (specs 2-4, 8) | PR B | rama PR A | fake Service; offset/backoff/429/ctx |
| 3 | Webhook + config + wiring (specs 5-7) | PR C | rama PR B | 401/200, fail-fast, setWebhook |

## Phase 1: Transporte (Work Unit 1)

- [x] 1.1 `internal/telegram/update.go`: structs `Update{UpdateID, Message, ChatMember, MyChatMember, ChatJoinRequest}` (punteros), `Message{MessageID, Chat, From, Text}`, `ChatMemberUpdated{From, NewChatMember}`, `ChatJoinRequest{User, Chat, Date}`; método `Kind() string` (message/chat_member/my_chat_member/chat_join_request/unknown)
- [x] 1.2 `internal/telegram/service.go`: interfaz Service crece con `GetUpdates(ctx, offset, timeout int, allowed []string) ([]Update, error)`, `SetWebhook(ctx, url, secret string, allowed []string) error`, `DeleteWebhook(ctx) error`
- [x] 1.3 `internal/telegram/service.go`: errores de dominio `RateLimitError{RetryAfter time.Duration}` (429) y `ErrWebhookConflict` (409)
- [x] 1.4 `internal/telegram/adapter.go`: helper `doGetQuery(ctx, method string, q url.Values, result any)` reutilizando doGet; `GetUpdates` arma offset/timeout/allowed_updates; `SetWebhook`/`DeleteWebhook` con url_encoded secret y allowed_updates
- [x] 1.5 `internal/telegram/adapter.go`: mapeo 429 → `RateLimitError` con `retry_after`, 409 → `ErrWebhookConflict`, 401 → `ErrInvalidToken` (ya existe)
- [x] 1.6 `adapter_test.go`: casos getUpdates (lote decodificado), 401, 409, 429 con retry_after, setWebhook ok/error — con stub httptest existente

## Phase 2: Bus + Poller (Work Unit 2)

- [x] 2.1 `internal/events/bus.go`: `Bus{mu, handlers []func(*telegram.Update)}` con `NewBus()`, `Handle(func(*telegram.Update))`, `Publish(*telegram.Update)` (entrega a todos en orden)
- [x] 2.2 `events/bus_test.go`: 1 handler recibe; 2 handlers reciben exactamente una vez
- [x] 2.3 `internal/telegram/poller.go`: const `MVPAllowedUpdates = [message, chat_member, my_chat_member, chat_join_request]`; `NewPoller(svc Service, opts ...PollerOption)` con timeout default 30s; `Run(ctx, onBatch func([]Update)) error`
- [x] 2.4 `poller.go`: loop — offset avanza a max(update_id)+1 solo tras éxito; error red → backoff (1s, 2s, 5s) sin avanzar; 429 → espera `retry_after` y reintenta (máx 3); ctx cancel → limpio
- [x] 2.5 `poller.go`: 401 → `ErrInvalidToken` fatal; 409 → `ErrWebhookConflict` fatal
- [x] 2.6 `poller_test.go`: fake Service (mock a mano): lote 10/11 → offset 12; error red → offset previo; 429 retry_after 2 → espera ≥2s; ctx cancel → Run retorna sin error; 409 → error fatal

## Phase 3: Webhook + config + wiring (Work Unit 3)

- [x] 3.1 `config.go`: fail-fast modo webhook sin `TELEGRAM_WEBHOOK_URL` y sin `TELEGRAM_WEBHOOK_SECRET`; validar formato del secret (1-256, `A-Za-z0-9_-`)
- [x] 3.2 `config_test.go`: casos webhook sin URL → error, webhook sin secret → error, secret inválido → error, polling → ok
- [x] 3.3 `internal/api/webhook.go`: `telegramWebhookHandler(bus, secret string) http.HandlerFunc` — header `X-Telegram-Bot-Api-Secret-Token` con `subtle.ConstantTimeCompare` (401 si no coincide), decode `Update` (body ≤1 MiB), `go bus.Publish`, 200
- [x] 3.4 `internal/api/server.go`: option `WithWebhook(bus, secret)` que monta `POST /api/telegram/webhook`
- [x] 3.5 `webhook_test.go`: sin header → 401 y no publica; secret malo → 401; correcto → 200 y update publicado (bus con handler capturando)
- [x] 3.6 `cmd/server/main.go`: crear bus + handler de logging (`Update.Kind()` + update_id); modo polling → `go poller.Run(ctx, bus.Publish)` con errores al errCh; modo webhook → `bot.SetWebhook(ctx, url, secret, MVPAllowedUpdates)` (falla detiene startup) y `WithWebhook`; ctx cancela poller en shutdown

## Phase 4: Verificación

- [x] 4.1 `go build ./...` + `go vet ./...` + `go test ./...` verdes
- [x] 4.2 gofmt limpio; token no aparece en logs/errores (review de strings)
- [x] 4.3 README: cómo probar webhook con curl en dev (túnel) — verificación e2e real queda pendiente de token real

# Verificación manual pendiente (token real)
- 4.4 polling real: arranca, loguea updates recibidos
- 4.5 webhook real: setWebhook registrado, curl con secret → log + 200