# Design: Repo Bootstrap del Telegram Group Manager

## Technical Approach

Scaffold mínimo ejecutable por fases: (1) infra Docker con 3 servicios, (2) backend Go modular (`cmd/server` + `internal/{config,database,telegram,api}`) con config fail-fast, goose auto-migrate en dev, adapter Telegram con `getMe`, `GET /api/health`; (3) frontend Vite+React+TS strict con placeholder; (4) setup de testing; (5) README y `.env.example`. El listener de eventos queda fuera (cambio `telegram-events`).

## Architecture Decisions

| # | Decisión | Opciones | Trade-off | Elección |
|---|----------|----------|-----------|----------|
| 1 | Router HTTP | chi vs stdlib | stdlib Go 1.22+ soporta `GET /api/health` sin deps; chi aporta al crecer API | stdlib — 0 deps para 1 endpoint |
| 2 | Driver DB | `lib/pq` vs `pgx/v5/stdlib` | pgx: protocolo nativo, mejor performance, soporta `database/sql` | `pgx/v5/stdlib` con `database/sql` (guía 4 evita ORM) |
| 3 | Módulo Go | `github.com/telegram-manager/backend` | nombre neutro porque aún no hay repo remoto | mantener neutro; ajustar antes del primer push |
| 4 | Fronend compose | dev-mode (vite server) vs nginx prod | dev-mode: hot reload + build rápido para iterar; nginx: listo para deploy pero rebuild lento por cambio | dev-mode en `frontend/Dockerfile`; Dockerfile nginx difiere al trabajo de deploy |
| 5 | Estado del bot para health | llamar `getMe` por request vs cachear al arranque | cachear: health barato, sin pressure de rate limit; degrada si token se revoca en runtime (aceptable MVP) | cachear estado al arrancar; health lee cache |
| 6 | Migración inicial | solo `admins` vs schema completo | spec 13: "no crear tablas de funcionalidades que no existen" | solo `admins` (id, username UNIQUE, password_hash, created_at, last_login_at) |
| 7 | Aplicar migraciones en dev | siempre vs por flag | flag explícito es más predecible que adivinar entorno | `RUN_MIGRATIONS=true` (default) en `.env.example` |
| 8 | TanStack Query en bootstrap | incluir vs diferir | no hay datos todavía; solo `api-client.ts` stub | diferir al cambio dashboard (deps mínimas, regla del spec) |
| 9 | Logging | `log/slog` JSON vs texto | JSON parseable; compose lo muestra igual | slog JSON (guía 10) |

## Data Flow

```
main.go
  ├─ config.Load() ── fail fast si falta token/DSN
  ├─ database.Connect(DSN) ── ping
  ├─ database.Migrate(UP) si RUN_MIGRATIONS
  ├─ telegram.New(token) ── getMe() → cache {connected, username}
  └─ api.Server{db, botStatus}
        GET /api/health → {data:{status, db, bot_connected, bot_username}}
```

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `docker-compose.yml` | Create | backend(8080), frontend(5173), postgres:16-alpine + healthcheck + volume |
| `.env.example` | Create | 9 variables documentadas (incluye `RUN_MIGRATIONS`) |
| `.gitignore` | Create | `.env`, `node_modules/`, binarios, dist, coverage |
| `backend/go.mod`, `backend/go.sum` | Create | module `github.com/telegram-manager/backend` |
| `backend/cmd/server/main.go` | Create | wiring: config → db → migraciones → telegram → api → listen |
| `backend/internal/config/config.go` (+test) | Create | struct tipada, fail-fast, validación |
| `backend/internal/database/database.go`, `migrate.go` | Create | pool `database/sql` + pgx; goose up con `//go:embed ../migrations` |
| `backend/internal/telegram/service.go` | Create | interfaz `Service{GetMe(ctx) (BotUser, error)}` |
| `backend/internal/telegram/adapter.go` | Create | impl HTTP, path `getMe` oficial, cache de estado, error a dominio |
| `backend/internal/telegram/model.go` | Create | `BotUser{ID int64, Username, FirstName string}` |
| `backend/internal/api/server.go`, `health.go` (+test), `envelope.go` | Create | mux stdlib, envelope `{data,error}`, health handler con interfaces locales (`Pinger`, `botStatusProvider`) |
| `backend/migrations/00001_create_admins.sql` | Create | tabla `admins` |
| `backend/Dockerfile` | Create | multi-stage golang:alpine → binary ${PORT} |
| `frontend/package.json`, `tsconfig.json`, `tsconfig.node.json`, `vite.config.ts` | Create | react, react-dom, react-router-dom; dev: vite, typescript, vitest, testing-library, jsdom |
| `frontend/index.html`, `src/main.tsx`, `src/App.tsx` | Create | bootstrap + router placeholder |
| `frontend/src/lib/api-client.ts` | Create | fetch wrapper con `VITE_API_BASE_URL` + envelope |
| `frontend/src/App.test.tsx` | Create | smoke test render |
| `frontend/Dockerfile` | Create | node:20-alpine, `npm run dev -- --host 0.0.0.0` |
| `README.md` | Create | ejecución local con/sin Docker, migraciones, TELEGRAM_MODE |

## Interfaces / Contracts

```go
// internal/telegram/service.go — consumida por api
type Service interface {
    GetMe(ctx context.Context) (BotUser, error)
}

// internal/api/health.go — interfaz local, definida donde se consume
type botStatusProvider interface {
    Status() (connected bool, username string)
}
```

Envelope API: `{"data": {...}, "error": null}` (guía backend §6).
`GET /api/health` → `{"data":{"status":"ok","db":"ok","bot_connected":true,"bot_username":"..."}}`.

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| Unit (Go) | config: falta DSN/token → error | table-driven (test sin Bot API real) |
| Unit (Go) | health handler: db ok/error + bot conectado/desconectado | mocks de `Pinger`/`botStatusProvider` a mano |
| Unit (Go) | telegram adapter: solo mapeo de errores | `httptest` stub de `api.telegram.org` (sin red real) |
| Unit (TS) | App placeholder render | Vitest + testing-library smoke |

## Migration / Rollout

No migration de datos (repo vacío). Solo la migración goose inicial `00001`. Rollout = single change; rollback trivial (borrar scaffold, `docker compose down -v`).

## Open Questions

- [x] `getMe` no está en la referencia curada → resuelto en spec: es método oficial, se documenta la fuente.
- [ ] ¿Repo remoto/GitHub disponible? Afecta solo el nombre del módulo Go (ajuste mecánico).