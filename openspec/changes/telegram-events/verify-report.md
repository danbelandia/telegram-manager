# Verify Report: telegram-events

**Change**: telegram-events
**Version**: spec v1 (2026-09-05)
**Mode**: Standard
**Date**: 2026-09-06

## Completeness

| Metric | Value |
|--------|-------|
| Tasks total (automatizables) | 21 |
| Tasks complete | 21 |
| Tasks incomplete | 0 |
| Verificación manual pendiente | 4.4 (polling real), 4.5 (webhook real) — requieren `TELEGRAM_BOT_TOKEN` real |

## Build & Tests Execution

**Build**: ✅ Passed (`go build ./...` en `backend/`, sin output de error)
**Vet**: ✅ Passed (`go vet ./...` sin hallazgos)
**Gofmt**: ✅ Limpio (`gofmt -l .` sin archivos)

**Tests**: ✅ 29 passed / 0 failed / 0 skipped (`go test ./... -count=1 -v`)

```text
ok  internal/api        1.925s   coverage: ~84%
ok  internal/config     1.289s   coverage: ~93%
ok  internal/events     1.647s   coverage: 100%
ok  internal/telegram   17.434s  coverage: ~85%
cmd/server y database: sin tests (wiring/infra, aceptado en MVP)
```

**Coverage**: 84-100% en los 4 paquetes con lógica → ✅ (sin threshold formal)

## Spec Compliance Matrix

| Requirement | Scenario | Test | Result |
|-------------|----------|------|--------|
| Transporte adapter | Polling recibe lote (update_id + campos) | `adapter_test.go > TestAdapterGetUpdates_Success` | ✅ COMPLIANT |
| Transporte adapter | getUpdates 401 → ErrInvalidToken | `TestAdapterGetMe_InvalidToken` (mismo switch `doGetQuery`; no directo sobre getUpdates) | ⚠️ PARTIAL |
| Transporte adapter | 409 → fatal, webhook activo | `TestAdapterGetUpdates_Conflict` + `TestPoller_ConflictIsFatal` | ✅ COMPLIANT |
| Poller | Lote 10/11 → offset 12 | `TestPoller_AdvancesOffset` (verifica avance >0, no el valor exacto 12) | ⚠️ PARTIAL |
| Poller | Error de red no avanza offset | `TestPoller_TransientErrorDoesNotAdvanceOffset` | ✅ COMPLIANT |
| Poller | 429 espera retry_after, máx 3 | `TestPoller_RateLimitRetries` (usa retry_after 1s) + `TestPoller_RateLimitExhausted` | ⚠️ PARTIAL |
| Filtro tipos | allowed_updates = 4 tipos MVP | `MVPAllowedUpdates` (estático) + `TestAdapterGetUpdates_Success` (2 tipos en query) | ⚠️ PARTIAL |
| Bus de eventos | Update publicado llega a handler | `bus_test.go > TestBusPublish_SingleHandler` | ✅ COMPLIANT |
| Bus de eventos | Webhook no bloquea la respuesta | `TestWebhook_ValidSecretPublishes` + `go publisher.Publish` (estructural) | ⚠️ PARTIAL |
| Endpoint webhook | Secret correcto → 200 y publica | `webhook_test.go > TestWebhook_ValidSecretPublishes` | ✅ COMPLIANT |
| Endpoint webhook | Secret inválido/ausente → 401, no publica | `TestWebhook_WithoutSecretIsUnauthorized` + `TestWebhook_WrongSecretIsUnauthorized` | ✅ COMPLIANT |
| Modo webhook | Sin URL → error al Load | `config_test.go > TestLoad/webhook without url` | ✅ COMPLIANT |
| setWebhook startup | Exitoso → arranque continúa | `TestAdapterSetWebhook_Success` (adapter); wiring de main sin test de integración | ⚠️ PARTIAL |
| setWebhook startup | Falla → startup no arranca | código de main.go (fail-fast); sin test de integración | ⚠️ PARTIAL |
| Múltiples consumidores | Dos handlers, ambos una vez | `TestBusPublish_MultipleHandlersCalledOnce` | ✅ COMPLIANT |

**Compliance summary**: 8/15 escenarios COMPLIANT, 7/15 PARTIAL (mecanismo cubierto; granularidad del caso exacto o test de integración de main ausente). 0 UNTESTED sin cobertura parcial. 0 FAILING.

## Correctness (Static Evidence)

| Requirement | Status | Notes |
|------------|--------|-------|
| Token nunca en logs ni errores | ✅ Implementado | grep de `token|TELEGRAM|secret` en salida de build sin hits; strings de error genéricos |
| Errores de dominio 429/409/401 | ✅ Implementado | `RateLimitError{RetryAfter}`, `ErrWebhookConflict`, `ErrInvalidToken` |
| Offset avanza solo tras éxito | ✅ Implementado | poller.go loop; verificado por tests |
| Webhook valida antes de procesar | ✅ Implementado | `subtle.ConstantTimeCompare`; body ≤1 MiB; 400 para JSON inválido |
| config fail-fast webhook | ✅ Implementado | URL + secret obligatorios; formato secret validado (1-256, `A-Za-z0-9_-`) |

## Coherence (Design)

| Decision | Followed? | Notes |
|----------|-----------|-------|
| AD1 Adapter único punto de transporte | ✅ Yes | Service + Adapter crecen, resto del backend no toca HTTP |
| AD2 GET con url.Values | ✅ Yes | doGetQuery |
| AD3 Bus síncrono; webhook publica en goroutine | ✅ Yes | worker secuencial de §18.1 queda para consumidores de negocio (fases futuras) |
| AD4 secret obligatorio en webhook | ✅ Yes | fail-fast + validación 1..256 chars |
| AD5 webhook 401/200 vacíos, sin envelope | ✅ Yes | 400 extra para body inválido |
| AD6 429/409 tipados; poller reintenta máx 3 | ✅ Yes | retryRateLimited |
| AD7 Update.Kind() | ✅ Yes | usado en handler de logging |
| AD8 endpoint solo en modo webhook | ✅ Yes | `WithWebhook` option, solo montado en main webhook |

## Issues Found

**CRITICAL**: None
**WARNING**:
- `TestPoller_AdvancesOffset` valida `offset > 0`, no el valor exacto 12 del escenario.
- `TestPoller_RateLimitRetries` usa `retry_after: 1s` mientras el escenario pide 2s (el mecanismo es el mismo).
**SUGGESTION**:
- Agregar test que verifique que el Poller envía exactamente `MVPAllowedUpdates` (4 tipos) en `allowed_updates`.
- No hay test de integración de `main.go` (startup con setWebhook exitoso/fallido) — aceptado por diseño MVP.
- `TestWebhook_ValidSecretPublishes` no mide explícitamente que el 200 precede al handler (garantizado por `go publisher.Publish`).

## Regresión e2e (2026-09-06, token real)

La verificación con token real reveló **2 bugs** que los tests unitarios no
cubrían (el error de red real y el timeout de HTTP real):

| Bug | Severidad | Detalle | Fix | Regresión test |
|-----|-----------|---------|-----|----------------|
| Fuga de token en errores de red | CRÍTICO (seguridad) | `*url.Error` de `http.Client` incluye la URL `bot<TOKEN>/...`; el adapter la envolvía en `ErrTelegramUnavailable` y el poller la logueaba | Extraer solo la causa (`urlErr.Err`) antes de envolver | `TestAdapterError_DoesNotLeakToken` |
| Long poll truncado a 10s | ALTO (funcional) | `http.Client{Timeout: 10s}` cortaba `getUpdates` con `timeout=30`, causando backoff infinito | Sin Timeout global; deadlines por llamada (`defaultRequestTimeout` 10s; `GetUpdates` deriva `timeout+5`) | `TestAdapterGetUpdates_LongPollNotTruncated` (stub 11s) |

**Verificación en vivo post-fix**: reiniciado con token real, 0 `poll error` en
48s (antes: timeout+token cada ~15s), long poll aguantando los 30s, sin
rastros del token en logs.

**Acción requerida del usuario**: rotar el token con `/revoke` en BotFather
(el token expuesto en logs antes del fix ya no es seguro).

## Verdict

**PASS WITH WARNINGS** — implementación completa, build/vet/test verdes, cobertura 84-100% en lógica; los PARTIAL son granularidad de test, no desvíos de spec. **2 bugs reales encontrados en la verificación e2e con token real (fuga de token en errores de red, long poll truncado) — ambos corregidos con test de regresión y verificación en vivo.** Queda la verificación manual 4.5 (webhook real) opcional, documentada en README, y la rotación del token expuesto.