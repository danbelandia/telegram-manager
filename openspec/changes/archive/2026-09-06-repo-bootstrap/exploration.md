# Exploration: repo-bootstrap

## Current State

Repositorio 100% vacío. Existen solo:
- `AGENTS.md` — spec completo del MVP (Fase 1), orden de implementación (sección 26), DoD (sección 27) y objetivo 28 (bootstrap como primer entregable).
- `docs/telegram_api_reference.md`, `docs/backend-go-skill.md`, `docs/frontend-react-skill.md` — referencias técnicas del proyecto.
- `.opencode/` — config del tooling opencode (skills, agents), nada de código de la app.
- `openspec/` — bootstrap SDD recién inicializado (config.yaml, dirs de specs/changes).
- No hay Git init, ni `go.mod`, ni `package.json`, ni Docker, ni `.env.example`.

## Affected Areas

- `docker-compose.yml` + `Dockerfile` (backend y frontend) + `.env.example` — nuevo (objetivo 28 pasos 2/5/6).
- `backend/` completo — nuevo: `cmd/server/main.go`, `internal/config`, `internal/database`, `internal/telegram`, `internal/api` (health), `migrations/` (pasos 3/4/5/7/9).
- `frontend/` completo — nuevo: scaffold Vite + React + TS (paso 11 como shell mínimo, dashboard llega en paso 12).
- `.gitignore` — nuevo (excluir `.env`, `node_modules`, binarios).
- `README.md` — documentación requerida por la DoD ("documentación para ejecutar el proyecto localmente").

## Approaches

1. **Validación de token Telegram vía `getMe` al arrancar**
   - `getMe` NO está en la referencia curada (`docs/telegram_api_reference.md`), pero SÍ es un método oficial documentado (https://core.telegram.org/bots/api#getme) y es la vía canónica para validar un token. El spec (5.1) exige "validar que el token funciona".
   - Pros: mínimo, oficial, sin estado; devuelve identidad del bot (username/id) útil para el health check.
   - Cons: requiere confirmar que lo permitimos explícitamente (regla de no inventar endpoints — este es real, solo hay que documentar la fuente).
   - Effort: Low

2. **Router HTTP del backend: `net/http` stdlib (Go 1.22+) vs `chi`**
   - Stdlib: cero dependencias; el health check es un GET simple. Go 1.22+ soporta patrones `GET /api/health` con método.
   - Pros: mínimo para el MVP, alineado con "evitar dependencias innecesarias".
   - Cons: cuando lleguen rutas parametrizadas (`/api/groups/:id/users/...`) el patrón de stdlib es más limitado que chi.
   - chi: recomendado por la guía backend cuando la API crezca (sección 6).
   - Effort: Low (ambas)

3. **Primera migración goose: ¿qué crea?**
   - (a) Migración "vacía" solo para probar el pipeline goose: inútil, hay que migrar de nuevo apenas arranque el desarrollo real.
   - (b) Solo `admins` (con campos auth de la sección 13: `password_hash`, `created_at`, `last_login_at`): próxima en el orden de implementación (paso 9: Autenticación), pequeña, prueba goose con algo que seguro se usa.
   - (c) Schema completo Fase 1 (`users`, `groups`, `group_members`, `admins`, `join_requests`, `warnings`, `logs`): adelanta trabajo, pero la sección 13 dice "no crear tablas de funcionalidades que todavía no existan" — y aún no existen los handlers para la mayoría.
   - Recomendado: (b) `admins` — prueba el pipeline con una tabla real y en scope inmediato.
   - Effort: Low

4. **Scaffold frontend: `npm create vite@latest` vs escrita a mano**
   - create-vite: template oficial react-ts, rápido y estándar.
   - Manual: más determinista (tsconfig strict exacto según guía, sin boilerplate extra de create-vite que habría que limpiar).
   - Recomendado: manual controlado (escribir `package.json`, `tsconfig.json` strict, `vite.config.ts`, `src/main.tsx`, un App placeholder con router). Es scaffold mínimo, no hay razón para aceptar el boilerplate genérico de create-vite; y en Windows + OneDrive el CLI interactivo puede traer fricción.
   - Effort: Low

5. **Bot connection "funcional" mínima en bootstrap**
   - Opciones: solo `getMe` en health check, o `getMe` + listener polling `getUpdates` arrancando.
   - El paso 6 del spec ("Webhook/polling") merece su propio cambio posterior; en bootstrap basta `getMe` + health check que reporte `bot_connected: true/false` y `bot_username`. Evita duplicar el trabajo del paso 6.
   - Effort: Low

## Recommendation

Un solo cambio `repo-bootstrap` que entregue, en este orden:

1. `git init` + `.gitignore` raíz.
2. `docker-compose.yml` (backend, frontend, postgres:16-alpine con healthcheck, volumen para datos, env desde `.env`) + `Dockerfile`s multi-stage + `.env.example`.
3. Backend: `go mod init github.com/telegram-manager/backend` (o ruta neutra), config tipado en `internal/config` (falla rápido si falta `TELEGRAM_BOT_TOKEN` o `DATABASE_URL`), conexión `database/sql` + `pgx` o `lib/pq`, primera migración goose `00001_create_admins.sql`, adapter `internal/telegram` con validación `getMe` (documentando que viene de la API oficial, no de la referencia curada), `GET /api/health` (DB ping + bot connected), logger `log/slog`, sin exponer el token.
4. Frontend: scaffold manual Vite + React + TS strict + React Router con una ruta placeholder, `lib/api-client.ts` mínimo, `VITE_API_BASE_URL` en `.env.example`.
5. Setup de testing: primer test Go (config + health handler, sin Bot API real) y primer smoke test con Vitest.
6. `README.md` con instrucciones locales (docker compose up, migraciones, TELEGRAM_MODE=polling por defecto).

Esto cumple el objetivo 28 sin adelantar módulos del orden 5→20 (la integración real de eventos llega en el próximo cambio).

## Risks

- `getMe` no está en la referencia curada del proyecto → confirmar explícitamente en proposal/spec que es un método oficial real y documentar la fuente; si el usuario prefiere otra vía, ajustar.
- Entorno Windows + OneDrive: rutas largas y permisos pueden molestar a Docker Desktop (backend/frontend en subcarpetas ayuda). El bind mount de `node_modules` en Windows puede ser lento — considerar volumen anónimo para `node_modules`.
- Sin Git configurado: el cambio asume `git init` local; si el repo final vive en GitHub, la URL del `go.mod` deberá ajustarse antes del primer push (preferir nombre de módulo neutro, no una URL inventada).
- Telegrafía del rate limiter (18.1): no aplica todavía en bootstrap (aún no hay acciones), pero la estructura del adapter debe dejar el lugar para meterlo después sin tocar consumidores.

## Ready for Proposal

Yes. El siguiente paso es `/sdd-propose` con el cambio `repo-bootstrap`. Contarle al usuario: (1) la validación del token usará `getMe` (official API, no está en la referencia curada — se documentará la fuente), (2) la primera migración creará solo `admins` (no schema completo), (3) el bootstrap NO incluye todavía el listener de eventos (eso es el cambio siguiente, "telegram-events").

## Skill Resolution

fallback-registry — cargados desde `.atl/skill-registry.md` (go-testing, work-unit-commits, cognitive-doc-design se aplican a este cambio).