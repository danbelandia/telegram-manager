# Design: telegram-events — recepción de eventos de Telegram

## Technical Approach
Approach 1 de la exploración: el `Adapter` transporta (único que construye
URLs de la Bot API y mapea errores de dominio), `Poller` y el handler
webhook son capas delgadas sobre él, y `internal/events.Bus` es el punto
de despacho único. Specs cubiertas: transporte, poller, filtro de tipos,
bus, webhook con secret, fail-fast de config y setWebhook al arrancar.

## Architecture Decisions

| # | Decisión | Alternativas | Por qué |
|---|----------|--------------|---------|
| 1 | `Adapter` crece con `GetUpdates`/`SetWebhook`/`DeleteWebhook` (interfaz `Service`) | Cliente HTTP separado en `internal/events` | Una sola capa Telegram (spec §15); reutiliza `doGet`, stub httptest y errores de dominio |
| 2 | `getUpdates`/`setWebhook`/`deleteWebhook` como GET con `url.Values` | POST con body JSON | La Bot API acepta ambos; mantener el patrón GET de `doGet` (la referencia §0 no restringe el verbo) |
| 3 | Bus síncrono con mutex: `Handle` + `Publish` invoca handlers en el caller | Canal buffered + worker | MVP sonido; el webhook publica desde una goroutine (no bloquea el 200); worker secuencial para acciones en lote llega con los consumidores de negocio (spec §18.1) |
| 4 | En modo webhook, `TELEGRAM_WEBHOOK_SECRET` es obligatorio (fail-fast) | Opcional según spec del change ("when set") | AGENTS.md §19.1 exige validar el secret **antes de procesar**; sin secret cualquiera puede mandar eventos falsos. Refuerza sin contradecir el spec |
| 5 | `POST /api/telegram/webhook` no usa el envelope `{data,error}` | Envelope homogéneo | Telegram solo mira el status code (200 = ack, retry si no); devolver `401`/`200` vacíos mantiene simple el ack |
| 6 | Errores 429/409 tipados en el dominio: `RateLimitError{RetryAfter}` y `ErrWebhookConflict` | Parse general en el poller | El 429 debe llegar a quien decide el reintento (poller) respetando `retry_after` (spec §18.1); el 409 es fatal con mensaje que apunta a `deleteWebhook` |
| 7 | `Update.Kind() string` (message/chat_member/...) para log y eventual ruteo | Campos booleanos por tipo | Log de auditoría legible; futuros handlers switchean por `Kind()` sin tocar transporte |
| 8 | El endpoint webhook solo se monta en modo webhook (`WithWebhook` option en `NewServer`) | Siempre montado | En polling el 404 es correcto; no expone un endpoint sin uso |

## Data Flow

```
Modo polling:
  Telegram ──getUpdates(offset,30s,allowed)──> Poller ──bus.Publish──> Bus ──> logging
Modo webhook:
  Telegram ──POST /api/telegram/webhook──> handler(secret) ──goroutine bus.Publish──> Bus ──> logging
                                                         │
                                                         └──> 200 inmediato
```

## File Changes

| File | Acción | Descripción |
|------|--------|-------------|
| `backend/internal/telegram/service.go` | Modify | interfaz con transporte + `RateLimitError`, `ErrWebhookConflict` |
| `backend/internal/telegram/adapter.go` | Modify | `GetUpdates`/`SetWebhook`/`DeleteWebhook`; helper `doGetQuery` con `url.Values`; mapeo 409/429 |
| `backend/internal/telegram/update.go` | Create | `Update` (update_id + message/chat_member/my_chat_member/chat_join_request) + `Kind()` |
| `backend/internal/telegram/poller.go` | Create | loop long polling con offset/backoff/429 |
| `backend/internal/telegram/poller_test.go` | Create | fake Service: lote/red/429/ctx |
| `backend/internal/telegram/adapter_test.go` | Modify | casos getUpdates/setWebhook con stub |
| `backend/internal/events/bus.go` | Create | `Bus` con `Handle`/`Publish` |
| `backend/internal/events/bus_test.go` | Create | 1 y 2 handlers, entrega a todos |
| `backend/internal/api/webhook.go` | Create | `telegramWebhookHandler(bus, secret)` con `subtle.ConstantTimeCompare` |
| `backend/internal/api/webhook_test.go` | Create | 401 (sin/incorrecto) / 200 y publicado |
| `backend/internal/api/server.go` | Modify | `WithWebhook` option + ruta |
| `backend/internal/config/config.go` | Modify | fail-fast webhook URL + secret + validar formato secret |
| `backend/internal/config/config_test.go` | Modify | casos webhook sin URL/sin secret |
| `backend/cmd/server/main.go` | Modify | wiring: bus + logging handler; poller o setWebhook según modo; ctx cancela poller |

## Interfaces / Contracts

```go
// internal/events/bus.go
type Bus struct { mu sync.Mutex; handlers []func(*telegram.Update) }
func NewBus() *Bus
func (b *Bus) Handle(h func(*telegram.Update))
func (b *Bus) Publish(u *telegram.Update) // entrega a todos los handlers, en orden

// internal/telegram/service.go (interfaz Service crece)
GetUpdates(ctx context.Context, offset, timeout int, allowed []string) ([]Update, error)
SetWebhook(ctx context.Context, url, secret string, allowed []string) error
DeleteWebhook(ctx context.Context) error

// errores de dominio
type RateLimitError struct{ RetryAfter time.Duration } // 429 → el poller espera y reintenta (máx 3)
ErrWebhookConflict = errors.New("telegram: webhook activo; usar deleteWebhook") // 409 fatal

// internal/telegram/poller.go
func NewPoller(svc Service, opts ...PollerOption) *Poller
func (p *Poller) Run(ctx context.Context, onBatch func([]Update)) error // corta con ctx
// MVPAllowedUpdates = []string{"message","chat_member","my_chat_member","chat_join_request"}

// internal/api/server.go — opción para montar webhook
func WithWebhook(bus *events.Bus, secret string) Option
```

Webhook (webhook.go): compara `r.Header.Get("X-Telegram-Bot-Api-Secret-Token")`
con `secret` via `subtle.ConstantTimeCompare`; si mismatch → 401 (sin body);
si match → decodifica `telegram.Update` (limite body 1 MiB), publica en
goroutine (`go bus.Publish(&update)`) y responde 200.

## Testing Strategy

| Capa | Qué | Cómo |
|------|-----|------|
| Unit | Adapter transport | stub httptest (`WithBaseURL`): lote decodificado, 401→ErrInvalidToken, 409→ErrWebhookConflict, 429→RateLimitError con retry_after, setWebhook ok/error |
| Unit | Poller | fake `Service` (mock a mano): lote 10/11→offset 12; error red→offset previo + backoff; 429→espera `retry_after` (máx 3); ctx cancel→sale limpio |
| Unit | Bus | 1 handler recibe; 2 handlers reciben ambos exactamente una vez |
| Unit | Webhook handler | httptest: sin header→401; secret malo→401 y no publica; correcto→200 y update publicado |
| Unit | Config | webhook sin URL→error; webhook sin secret→error; secret con caracteres inválidos→error; polling ok |
| Integración | wiring main | arranque polling con fake: loguea updates; webhook: stuba setWebhook |

## Migration / Rollout
No migration required (sin tablas nuevas; la de grupos llega con la
detección de grupos). Rollout: el modo sigue controlado por `TELEGRAM_MODE`;
en dev se usa polling, en prod webhook.

## Open Questions
- None