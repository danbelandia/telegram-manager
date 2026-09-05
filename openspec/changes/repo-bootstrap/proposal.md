# Proposal: Repo Bootstrap del Telegram Group Manager

## Intent

El repo está vacío (solo spec `AGENTS.md` + docs). Este cambio crea la base ejecutable del MVP según el objetivo 28 del spec: estructura de carpetas, Docker Compose, PostgreSQL, Go configurado con migraciones goose, frontend React+TS, conexión funcional con Telegram (validación de token) y health check. Sin esto, ningún módulo posterior (grupos, auth, moderación) tiene dónde apoyarse.

## Scope

### In Scope
- `git init` + `.gitignore` raíz + `README.md` (ejecución local).
- `docker-compose.yml` (backend, frontend, postgres:16-alpine con healthcheck y volumen) + `Dockerfile`s multi-stage + `.env.example`.
- Backend Go: `cmd/server/main.go`, `internal/config` (falla rápido sin `TELEGRAM_BOT_TOKEN`/`DATABASE_URL`), `internal/database` (database/sql), primera migración goose `00001_create_admins.sql` (solo tabla `admins` con campos auth de la sección 13), `internal/telegram` con validación `getMe`, `GET /api/health` (DB ping + estado del bot), logging `log/slog`, sin token en logs/respuestas.
- Frontend: scaffold manual Vite + React + TS strict + React Router (placeholder) + `lib/api-client.ts` mínimo. Sin dashboard todavía.
- Setup de testing: primer test Go unitario (config + health, sin Bot API real) y smoke test Vitest.

### Out of Scope
- Listener de eventos Telegram (webhook/polling con `getUpdates`/`setWebhook`) — cambio posterior `telegram-events` (paso 6 del spec).
- Auth JWT, tabla `admins` poblada/seed, grupos, usuarios, moderación — módulos siguientes del orden 26.
- Tablas PostgreSQL más allá de `admins`.
- CI/CD, deploy, GitHub Actions.

## Capabilities

### New Capabilities
- `repo-bootstrap`: estructura del repo, docker-compose, Dockerfiles, `.env.example`, README, scaffold backend/frontend mínimos, pipeline goose con migración inicial.
- `bot-connection`: validación del token del bot con `getMe` al arrancar (fuente: API oficial https://core.telegram.org/bots/api#getme — no está en la referencia curada) y `GET /api/health` reportando `bot_connected`/`bot_username` + DB ping, sin exponer el token.

### Modified Capabilities
- None (no existen specs previas).

## Approach

Siguiendo la exploración: scaffold manual y controlado; `net/http` stdlib para el health check; `getMe` para validar el token; primera migración solo `admins`; frontend escrito a mano (no create-vite); volumen anónimo para `node_modules` (fricción OneDrive/Windows); nombre de módulo Go neutro (`github.com/telegram-manager/backend`).

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `docker-compose.yml`, `Dockerfile`s | New | 3 servicios + healthchecks |
| `backend/` (cmd, internal/config, database, telegram, api) | New | Scaffold Go + health + getMe |
| `backend/migrations/00001_create_admins.sql` | New | Primera migración goose |
| `frontend/` (Vite + React + TS) | New | Scaffold mínimo con router |
| `.env.example`, `.gitignore`, `README.md` | New | Config y docs |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| `getMe` fuera de referencia curada | High | Documentado en proposal; es método oficial real |
| Docker lento/frágil en OneDrive | Med | Volumen anónimo para `node_modules`; subcarpetas backend/frontend |
| Nombre de módulo Go en repo remoto futuro | Med | Módulo neutro, ajuste mecánico antes del primer push |

## Rollback Plan

Eliminar el commit inicial y los archivos raíz creados (`git rm -r`, revertir a estado vacío). Docker: `docker compose down -v` borra volúmenes. Es scaffold sin datos ni lógica: revertir es trivial.

## Dependencies

- Docker Desktop con backend Go y frontend Node.
- Go 1.22+ y Node 20+ locales para desarrollo sin Docker.
- Token de bot Telegram válido en `.env` (de @BotFather).
- goose como librería `github.com/pressly/goose`; migraciones corriendo automáticamente en dev (13.1).

## Success Criteria

- [ ] `docker compose up --build` levanta los 3 servicios y postgres responde healthcheck.
- [ ] Backend arranca y valida token con `getMe`; falla rápido si falta config.
- [ ] `GET /api/health` devuelve `{"data": {"status":"ok","db":"ok","bot_connected":true,...}}`.
- [ ] Migración goose `00001` se aplica a postgres (tabla `admins` con `password_hash`, `created_at`, `last_login_at`).
- [ ] Frontend compila (tsc strict sin errores) y sirve placeholder con React Router.
- [ ] No hay secretos en el repo; el token nunca aparece en logs ni respuestas.
- [ ] `go test ./...` y `npm test` pasan (smoke tests).