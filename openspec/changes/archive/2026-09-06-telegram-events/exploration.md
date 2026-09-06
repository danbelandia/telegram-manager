# Exploration: telegram-events

## Current State
- El backend valida el token con `getMe` al arrancar, pero **no recibe
  eventos** — no hay listener de updates.
- `internal/telegram/adapter.go`: solo `GetMe` (GET) + cache de estado
  `{connected, username}` para `/api/health`. `doGet` es el único
  helper HTTP; no existen `postJSON` ni manejo de `retry_after`.
- `internal/config/config.go` ya declara `TELEGRAM_MODE`,
  `TELEGRAM_WEBHOOK_URL` y `TELEGRAM_WEBHOOK_SECRET` pero solo valida
  que `TELEGRAM_MODE` sea `polling`/`webhook`; **no falla si falta la
  URL en modo webhook**.
- `cmd/server/main.go` arranca un `http.Server` solo para la API REST
  en `:8080`; el shutdown graceful cubre la API pero nada consume el
  `ctx` de señal para detener un listener de eventos.
- La referencia curada (`docs/telegram_api_reference.md` §1-2) define:
  `setWebhook` (secret_token → header `X-Telegram-Bot-Api-Secret-Token`,
  puertos 443/80/88/8443), `getWebhookInfo`, `getUpdates` (offset =
  último update_id + 1, timeout long polling), exclusión mutua
  webhook/getUpdates, y `allowed_updates` para filtrar
  `message`, `chat_member`, `my_chat_member`, `chat_join_request`.

## Affected Areas
- `backend/internal/telegram/service.go` — interfaz `Service` necesita
  el transporte de updates (GET updates, set/delete webhook).
- `backend/internal/telegram/adapter.go` — `GetUpdates(ctx, offset,
  timeout, allowed)`, `SetWebhook(ctx, url, secret)`,
  `DeleteWebhook(ctx)`; logging de errores.
- `backend/internal/telegram/update.go` (nuevo) — modelos parciales de
  `Update` (update_id + tipos del MVP) con parse de JSON.
- `backend/internal/telegram/poller.go` (nuevo) — loop long polling con
  offset correcto, backoff en errores de red, 429 respetando
  `retry_after` (sección 18.1), errores fatales claros (401, 409
  conflict con webhook activo).
- `backend/internal/events/bus.go` (nuevo) — canal común de updates:
  `Publish(*Update)` + suscriptores (`Handle(func(*Update))`); en este
  cambio solo registra un handler de logging/auditoría.
- `backend/internal/api/webhook.go` (nuevo) — `POST /api/telegram/webhook`:
  valida `X-Telegram-Bot-Api-Secret-Token` en tiempo constante,
  decodifica el Update, lo publica y responde 200.
- `backend/internal/api/server.go` — monta la ruta webhook.
- `backend/cmd/server/main.go` — wiring: bus + poller (o setWebhook +
  handler) según `TELEGRAM_MODE`; ambos se detienen con el `ctx` de
  shutdown.
- `backend/internal/config/config.go` — fail-fast en modo webhook si
  falta `TELEGRAM_WEBHOOK_URL`; validar formato del secret.
- `.env.example` / README — nada nuevo (las variables ya existen); README
  puede documentar cómo probar el webhook con curl en dev.

## Approaches

1. **Transporte dentro del Adapter + capas finas encima** (recomendada)
   - El `Adapter` crece con `GetUpdates`, `SetWebhook`, `DeleteWebhook`
     (es el único que construye URLs de la Bot API, reutiliza el HTTP
     client y los errores de dominio). `Poller` (loop) y el handler
     webhook (HTTP POST) son capas delgadas que consumen el Adapter.
   - Pros: una sola implementación HTTP/errores; testing con el stub
     httptest ya existente (`WithBaseURL`); el resto del backend no ve
     detalles de transporte.
   - Cons: la interfaz `Service` crece (aceptable: transporte ES parte
     de la integración Telegram).
   - Effort: Medium

2. **Paquete `events` con cliente HTTP propio**
   - El bus y el transporte conviven en `internal/events`, con su propio
     cliente hacia getUpdates/setWebhook.
   - Pros: separación conceptual transporte/dispatch.
   - Cons: duplica la lógica HTTP y los errores de dominio del Adapter;
     dos lugares que testear contra la Bot API. Contradice la sección 15
     del spec (capa única Telegram).
   - Effort: Medium-High

3. **Solo polling, webhook diferido**
   - Implementar solo getUpdates y dejar setWebhook/endpoint para otro
     cambio.
   - Pros: menos trabajo inmediato.
   - Cons: el DoD (sección 27) exige "webhook o polling según
     `TELEGRAM_MODE`" — el modo webhook no quedaría funcional; el
     modo webhook es el de producción. Fragmenta la entrega.
   - Effort: Low

## Recommendation
**Approach 1**: Adapter transporta (getUpdates/setWebhook/deleteWebhook),
`Poller` en `internal/telegram` consume el Adapter en loop con offset
correcto y backoff, `internal/events.Bus` es el punto de despacho único
(esta entrega: handler de logging que registra `update_id` y tipo; los
consumidores de negocio — grupos, solicitudes — llegan en sus cambios),
y `internal/api` monta el endpoint webhook con validación de secreto.
Así el transporte queda verificado con el stub de Bot API (sin token
real) y los futuros cambios de negocio solo se suscriben al Bus.

Detalles fijos por la referencia curada:
- `allowed_updates` = `["message","chat_member","my_chat_member","chat_join_request"]`
  (el set que el MVP procesa; pedido explícito porque Telegram no envía
  `chat_member`/`my_chat_member` por defecto).
- Long polling `timeout=30` (máx recomendado 50; 30 evita tiempo muerto).
- Offset: avanzar a `max(update_id)+1` **solo** tras éxito; en error de
  red NO avanza (idempotente) y hace backoff.
- 429: leer `retry_after`, esperar, reintentar — máx 3 (sección 18.1).
- 401: token inválido → fatal. 409: existe webhook activo → fatal con
  mensaje que apunta a `deleteWebhook`.
- Webhook: validar secret con `subtle.ConstantTimeCompare`; responder 200
  apenas se publica; el retry lo maneja Telegram.
- Sin migración de base de datos en este cambio (la tabla de grupos
  llega con el cambio de detección/registro).

## Risks
- La verificación e2e de polling E interface (getUpdates real, webhook
  de Telegram real) requiere `TELEGRAM_BOT_TOKEN` válido — queda como
  verificación manual pendiente, igual que 4.2/4.3 de repo-bootstrap.
- Webhook en dev local necesita URL pública HTTPS en puerto permitido
  (túnel tipo ngrok o deploy); en dev local se usa polling. Documentar
  en README.
- El `Bus` sin consumidores de negocio puede leerse como "código sin
  usar": el handler de logging es el uso legítimo de esta entrega y sirve
  de auditoría/diagnóstico en dev.
- `getUpdates` + webhook activo son mutuamente excluyentes: el código
  debe fallar claro ante 409, nunca "intentar otra cosa".

## Ready for Proposal
Yes — el cambio es acotado (transporte + bus), orquesta con el spec
(secciones 18.1, 19, 19.1, DoD 27) y la referencia curada; los
consumidores de negocio quedan explícitamente fuera de scope.