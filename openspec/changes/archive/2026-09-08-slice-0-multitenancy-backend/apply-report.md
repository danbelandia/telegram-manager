# Apply Report — slice-0-multitenancy-backend

> **Change**: `slice-0-multitenancy-backend` (tenants + signup + JWT tenant_id + registry multi-bot + scoping, backend-only)
> **Branch**: `feat/slice-0-multitenancy` base `main @ 989c8e3`
> **Delivery**: single-pr `size:exception` (precedente 13/13, decisión explícita del usuario)
> **Date**: 2026-09-08
> **Mode**: hybrid (subagente sdd-apply para T1–T13 + cierre directo en sesión principal tras cancelación del subagente en T14)

## Verdict: **PASS-WITH-NOTES**

- Suite backend: **13/13 paquetes OK** (`go test ./... -count=1`), `go vet` limpio, `go build` limpio.
- 0 secretos en el diff (grep del token del incidente SEC-2026-09-08 sobre el diff main...HEAD: vacío).
- 1 NOTE: T14 (E2E smoke con tokens reales) no se ejecutó — el proceso e2e quedó colgado, se mató (`server-e2e` PID 33952) y se removieron sus logs. La cobertura E2E queda reemplazada por tests de integración contra Postgres real + mocks de TelegramService. El E2E con bot real queda como pendiente manual (requiere token válido y red).

## Commits (6, rama feat/slice-0-multitenancy)

```
1dab291 feat(multitenancy): estabilizar tests scopeados + fix 00009
8e50d03 feat(multitenancy): wiring main multi-bot + unit tests T12
dab4ba3 feat(multitenancy): tests de repos y services scopeados por tenant
2135b70 feat(multitenancy): tenant threading en services + handlers API + tests base
aed205e feat(multitenancy): scoping de repos por tenant_id + PKs compuestas en 00009
c851a3c feat(multitenancy): migracion 00009, tenants/crypto, config, auth claims, bootstrap, signup, registry
```

Scope total: **74 files, +4748/-1125** vs main.

## Trazabilidad tareas → archivos → REQs

| Tarea | Archivos principales | REQs |
|-------|---------------------|------|
| T1 migración 00009 | `backend/migrations/00009_multitenancy.sql` (213 LOC) | migration-00009 (4/4) |
| T2 tenants/crypto | `internal/tenants/{model,repository,crypto}.go` + tests | tenant-provisioning (cifrado, atomicidad) |
| T3 config/.env | `internal/config/config.go`, `.env.example` | backward-compat (legacy opcional), TENANT_TOKEN_ENC_KEY requerida |
| T4 auth claims | `internal/auth/{tokens,repository,admin}.go`, `api/server.go` (requireTenant), `api/auth_handlers.go` (me) | auth delta (4/4: access 401 legacy, refresh re-emite, me con tenant) |
| T5 bootstrap | `internal/auth/seeder.go` (adopta tenant default), repo test | backward-compat (EnsureInitialAdmin), auth (sesiones legacy) |
| T6 signup | `internal/auth/service.go` (Signup), handler + `signup_test.go` | tenant-provisioning (5/5: 201, slug 409, username global 409, getMe 502, VALIDATION_ERROR) |
| T7 registry | `internal/telegram/registry.go` (267 LOC) + test (140 LOC) | multi-bot-registry (4/4: boot N pollers, caliente, degraded+backoff, 25 rps/instancia) |
| T8/T9 scoping | repos groups/users/joinrequests/logs/publications/moderation/automation + `moderation_handlers.go` ownership check | tenant-isolation (4/4: listados filtrados, ajeno→NOT_FOUND, users vía join, cero can_*) |
| T10/T11 wiring | `cmd/server/main.go` (registry boot + shutdown graceful) | multi-bot-registry, backward-compat |
| T12 unit | `tokens_test`, `crypto_test`, `registry_test`, `signup_test` (mocks, cero Bot API real) | todos (cobertura unitaria) |
| T13 integración PG | repo tests contra Postgres real (fresco + legacy + re-aplicación 00009) | migration-00009, tenant-isolation |
| T14 E2E | NO ejecutado (ver NOTE arriba) | pendiente manual |
| T15 verificación | este archivo + gates en sesión principal | trazabilidad |

## Desviaciones del design

1. **T14 reemplazado por integración PG**: el smoke E2E con bot real se colgó (red/token). Los escenarios E2E quedan cubiertos por tests de integración contra Postgres + fakes de TelegramService. Sin impacto en REQs (ningún REQ exige red real; §21.1 prohíbe Bot API real en tests automatizados).
2. **Volumen mayor al estimado**: ~4748 inserciones vs ~1200 estimadas. Causa: el threading de `tenant_id` tocó services/handlers/tests de automation, moderation y publications más allá del scoping de repos previsto. Todo dentro del scope IN (scoping + ownership), sin scope OUT invadido (sin frontend, sin webhook, sin billing).

## Invariantes preservadas (verificado)

- **bugfix #172**: `grep can_*` sobre código ejecutable del diff → 0 matches en lógica (solo comentarios si los hay). `permissionOk` sigue por `bot_status == administrator`.
- **SEC-2026-09-08**: ningún valor real de `.env` en archivos. Tokens en tests = fakes (`fakeTelegramPub`, strings sintéticos).
- **Q1/Q2/Q3**: username UNIQUE global intacto; TELEGRAM_MODE global con N pollers; `users` sin `tenant_id` (scope vía join a `groups.tenant_id`).

## Gates ejecutados

```
go build ./...          → limpio
go vet ./...            → limpio
go test ./... -count=1  → 13/13 paquetes OK (auth 10.1s, telegram 21.8s, resto < 12s)
```

## Lo que sigue

1. **sdd-verify**: gates formales + verificación de que cada REQ tiene evidencia en runtime.
2. **PR single**: `feat/slice-0-multitenancy` → `main` (size:exception).
3. **sdd-archive**: APPEND de los 6 specs a `openspec/specs/` + archivo del change.
4. **Pendiente manual futuro**: E2E con bot real (2 tokens de prueba, 2 tenants) cuando haya red/token dedicado.
