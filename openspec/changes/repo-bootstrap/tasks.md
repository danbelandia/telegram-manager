# Tasks: Repo Bootstrap del Telegram Group Manager

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~850-900 (additions + deletions) |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 foundation → PR 2 backend → PR 3 frontend |
| Delivery strategy | ask-on-risk |
| Chain strategy | feature-branch-chain (elegida 2026-09-05): tracker `feat/repo-bootstrap` draft, PR 1 base=tracker; PR 2 y PR 3 base=rama de PR 1 |

Decision needed before apply: Yes (resuelta 2026-09-05: feature-branch-chain)
Chained PRs recommended: Yes
Chain strategy: feature-branch-chain (tracker `feat/repo-bootstrap`; PR 2 y PR 3 base = rama de PR 1)
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Notes |
|------|------|-----------|-------|
| 1 | Fundación: git, `.env.example`, docker-compose, Dockerfiles, `.gitignore`, README base | PR 1 | Base = main; nada depende de él |
| 2 | Backend Go completo + tests unitarios | PR 2 | Base = main (stacked) o rama feature; depende solo de PR 1 (estructura) |
| 3 | Frontend Vite+React+TS + smoke test | PR 3 | Base = main (stacked) o rama de PR 1/feature; independiente de PR 2 |

## Phase 1: Fundación e Infraestructura (Work Unit 1)

> Ajuste de límites del slice (2026-09-05): para que cada PR sea
> verificable de forma autónoma, cada Dockerfile viaja con su código
> fuente. `docker-compose.yml` arranca con postgres; el servicio backend
> se agrega en el PR 2 (con su Dockerfile) y el frontend en el PR 3.
> El estado final del tracker cumple el spec de 3 servicios.

- [x] 1.1 `git init` + `.gitignore` raíz (`.env`, `node_modules/`, binarios Go, `dist/`, coverage)
- [x] 1.2 Crear `docker-compose.yml` con postgres:16-alpine (healthcheck `pg_isready`, named volume); servicios backend (8080) y frontend (5173) se agregan en PR 2 y PR 3
- [x] 1.3 Crear `.env.example` con las 9 variables documentadas (token, TELEGRAM_MODE=polling, WEBHOOK_URL/SECRET, DATABASE_URL, JWT_SECRET, PORT, VITE_API_BASE_URL, RUN_MIGRATIONS=true)
- [ ] 1.4 Crear `backend/Dockerfile` multi-stage (golang:alpine build → runtime, binary escucha ${PORT}) — **se implementa con el código del backend (PR 2)**
  - [x] 1.4.1 DONE PR 2 (2026-09-05): `golang:1.26-alpine` build (go.mod requiere Go >= 1.26 por el grafo de goose v3.28) → `alpine:3.20` + `ca-certificates wget` (healthcheck)
- [ ] 1.5 Crear `frontend/Dockerfile` dev (node:20-alpine, `npm run dev -- --host 0.0.0.0`) — **se implementa con el código del frontend (PR 3)**
- [x] 1.6 Crear `README.md` base (ejecución con/sin Docker, migraciones en dev, TELEGRAM_MODE)

## Phase 2: Backend Go (Work Unit 2)

- [x] 2.1 `go mod init github.com/telegram-manager/backend`; deps: pgx/v5/stdlib, goose/v3, build tags para embed
- [x] 2.2 `internal/config/config.go`: struct tipada; fail-fast si falta `TELEGRAM_BOT_TOKEN`/`DATABASE_URL`; `config_test.go` table-driven (test: falta variable → error; completo → ok)
- [x] 2.3 `internal/database/database.go`: pool `database/sql` + pgx, ping; `migrate.go`: goose up con `//go:embed migrations` si `RUN_MIGRATIONS=true`
- [x] 2.4 `migrations/00001_create_admins.sql`: tabla `admins` (id bigserial PK, username text UNIQUE NOT NULL, password_hash text NOT NULL, created_at timestamptz, last_login_at timestamptz NULL)
- [x] 2.5 `internal/telegram/service.go` interfaz `Service{GetMe(ctx) (BotUser, error)}`; `model.go` `BotUser{ID int64, Username, FirstName string}`
- [x] 2.6 `internal/telegram/adapter.go`: cliente HTTP stdlib a `api.telegram.org/bot<TOKEN>/getMe`, cache de estado `{connected, username}`, map errores a dominio (invalid token, network); nunca loguear token
- [x] 2.7 `internal/api/envelope.go`: envelope `{data,error}`; `server.go`: mux stdlib; `health.go`: `GET /api/health` con interfaces locales `Pinger` y `botStatusProvider{Status() (bool,string)}`
- [x] 2.8 Tests: `health_test.go` (db ok/error x bot conectado/desconectado con mocks a mano), `adapter_test.go` con `httptest` stub de Telegram (sin red real)
- [x] 2.9 `cmd/server/main.go`: wiring config → db → migrate → telegram (getMe al arrancar) → api → listen; logging `log/slog` JSON

## Phase 3: Frontend (Work Unit 3)

- [ ] 3.1 `package.json`, `tsconfig.json` (strict:true), `tsconfig.node.json`, `vite.config.ts`; deps react/react-dom/react-router-dom; dev: vite, typescript, vitest, testing-library, jsdom
- [ ] 3.2 `index.html`, `src/main.tsx`, `src/App.tsx` (router con ruta placeholder)
- [ ] 3.3 `src/lib/api-client.ts`: fetch wrapper con `VITE_API_BASE_URL` + normalización de envelope
- [ ] 3.4 `src/App.test.tsx`: smoke test render del placeholder
- [ ] 3.5 `npm install`, verificar `tsc --noEmit` y `vitest run`

## Phase 4: Verificación (todos los work units juntos)

- [ ] 4.1 `go vet ./...` + `go test ./...` (backend, sin Bot API real)
- [ ] 4.2 `docker compose up --build`: 3 servicios up, postgres healthy, migración 00001 aplicada (verificar `admins` en psql)
- [ ] 4.3 `GET /api/health` → `{"data":{"status":"ok","db":"ok","bot_connected":true,"bot_username":"..."}}` con token válido
- [ ] 4.4 Verificar que el token NO aparece en logs ni en respuesta de health
- [ ] 4.5 Confirmar que `.env.example` está completo y no hay `.env` en el repo