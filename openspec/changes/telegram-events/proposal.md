# Proposal: telegram-events — recepción de eventos de Telegram

## Intent
El backend valida el token con `getMe` pero no recibe eventos. Este cambio
hace que ingesten updates de Telegram (polling o webhook según
`TELEGRAM_MODE`) y los despache a través de un bus interno, para que los
cambios de negocio (grupos, solicitudes de ingreso, moderación) se
suscriban en lugar de reinventar el transporte. Es el paso 6 del orden de
implementación del spec (webhook/polling) y el DoD 27 ("backend pueda
recibir eventos de Telegram según TELEGRAM_MODE").

## Scope
### In Scope
- Transporte en `internal/telegram`: `GetUpdates(ctx, offset, timeout, allowed)`,
  `SetWebhook(ctx, url, secret)`, `DeleteWebhook(ctx)` con errores de dominio y
  manejo de 429 (`retry_after`, máx 3) y 409 (conflicto webhook).
- Modelos parciales de `Update` (update_id + tipos del MVP: message,
  chat_member, my_chat_member, chat_join_request) con parse de JSON.
- `Poller`: loop long polling (timeout 30, `allowed_updates` del MVP),
  offset que avanza solo tras éxito, backoff en errores de red, fatales
  claros en 401/409.
- `internal/events.Bus`: `Publish(*Update)` + `Handle(func(*Update))`;
  suscriptor de logging/auditoría de eventos.
- `POST /api/telegram/webhook`: valida `X-Telegram-Bot-Api-Secret-Token`
  con `subtle.ConstantTimeCompare`, publica y responde 200.
- Wiring en `cmd/server/main.go`: poller o webhook según `TELEGRAM_MODE`,
  detenido por el ctx de shutdown.
- `config`: fail-fast en modo webhook si falta `TELEGRAM_WEBHOOK_URL`.
- Tests: adapter (getUpdates/setWebhook) con stub httptest, poller con
  fake de getUpdates, webhook con secret válido/inválido.

### Out of Scope
- Procesamiento de negocio de los eventos (detección/registro de grupos,
  solicitudes de ingreso, moderación) — llega en sus propios cambios.
- Migraciones de base de datos (tabla `groups` en detección de grupos).
- Rate limiting global ~25 req/s del adapter (sección 18.1): se agrega
  cuando haya múltiples métodos que lo requieran; aquí solo 429 en poller.

## Capabilities
### New Capabilities
- `telegram-events`: recepción y despacho de updates de Telegram (polling
  y webhook), transporte en el adapter, bus de eventos con logging.

### Modified Capabilities
- None (aún no hay spec de Telegram que modificar; solo el README/doc del
  bootstrap documentará el modo).

## Approach
Approach 1 de la exploración: el `Adapter` transporta (único que construye
URLs de la Bot API y centraliza errores), `Poller` y el handler webhook son
capas delgadas sobre él, y `internal/events.Bus` es el punto de despacho
único. `allowed_updates` = [message, chat_member, my_chat_member,
chat_join_request]; long polling timeout 30s.

## Affected Areas
| Area | Impact | Description |
|------|--------|-------------|
| `backend/internal/telegram/service.go` | Modified | interfaz Service con transporte de updates |
| `backend/internal/telegram/adapter.go` | Modified | GetUpdates/SetWebhook/DeleteWebhook, postJSON, 429/409 |
| `backend/internal/telegram/update.go` | New | modelos Update + parse |
| `backend/internal/telegram/poller.go` | New | loop long polling con offset/backoff |
| `backend/internal/events/bus.go` | New | bus de updates + handler de logging |
| `backend/internal/api/webhook.go` | New | endpoint webhook con validación de secret |
| `backend/internal/api/server.go` | Modified | monta `POST /api/telegram/webhook` |
| `backend/cmd/server/main.go` | Modified | wiring poller/webhook + shutdown |
| `backend/internal/config/config.go` | Modified | fail-fast webhook URL |
| `backend/internal/telegram/poller_test.go` | New | fake getUpdates |
| `backend/internal/api/webhook_test.go` | New | secret válido/inválido |

## Risks
| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Verificación e2e real requiere token (polling real, webhook real con URL pública) | Med | Verificación manual pendiente; unit tests con stub httptest sin red |
| Webhook dev necesita URL pública HTTPS (443/80/88/8443) o túnel | Med | Dev usa polling; README documenta túnel para probar webhook |
| 409 webhook activo silencioso | Low | Fatal claro que apunta a deleteWebhook; nunca reintentar a ciegas |
| Bus sin consumidores de negocio "código sin usar" | Med | El logging de eventos es el uso legítimo (auditoría/diagnóstico dev) |

## Rollback Plan
Revertir el commit del cambio (solo se agrega transporte+bus+endpoint; no
toca datos ni lógica de negocio existente). Si el poller activo genera
errores en arranque, `TELEGRAM_MODE` sigue soportando `polling`/`webhook`
y volver al estado previo es quitar las suscripciones del bus. Sin
migraciones que deshacer.

## Dependencies
- Ninguna nueva: solo stdlib. Reutiliza `net/http`, `encoding/json`,
  `time`, `crypto/subtle` ya implícitos.

## Success Criteria
- [ ] `go vet` + `go test ./...` verdes (test del adapter, poller y webhook; sin Bot API real)
- [ ] En modo polling el backend arranca, llama a getUpdates y loguea cada update recibido
- [ ] En modo webhook, `POST /api/telegram/webhook` valida el secret (401 ante secret inválido) y responde 200 ante secret válido
- [ ] `config.Load` falla en modo webhook sin `TELEGRAM_WEBHOOK_URL`
- [ ] Sin migración nueva; sin cambios en frontend