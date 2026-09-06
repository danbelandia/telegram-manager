# Tasks: auth — paso 9: Autenticación del panel

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~600-700 |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | 2 PRs: (1) auth core backend, (2) seed + wiring + E2E refactor |
| Delivery strategy | ask-on-risk |
| Chain strategy | feature-branch-chain (misma chain; aún sin remote) |

Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: feature-branch-chain
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Notes |
|------|------|-----------|-------|
| 1 | internal/auth: admin, repository, service, tokens + sus tests (+ jolt, jwt, bcrypt en go.mod) | PR auth-core | núcleo; sin tocar API todavía |
| 2 | Config + seed + handlers api + middleware + wiring main.go | PR auth-api | UI del flujo: login/refresh/logout/me |

## Phase 1: Core auth (sin API)

- [ ] 1.1 `internal/auth/admin.go`: modelo `Admin` (ID, Username, PasswordHash, CreatedAt, LastLoginAt) + `ErrCredentials`.
- [ ] 1.2 `internal/auth/repository.go`: `NewRepository(db)`, `GetByUsername` (ErrNotFound), `UpdateLastLogin`.
- [ ] 1.3 `internal/auth/tokens.go`: `Service` con `NewService(secret, repo, ...)`, `IssueAccess` (15m), `ParseAccess`, `IssueRefresh` (7d), `ParseRefresh` con `golang-jwt/jwt/v5` (HS256). Claims: sub, username.
- [ ] 1.4 `internal/auth/service.go`: `Login(ctx, username, password)` (GetByUsername + bcrypt + UpdateLastLogin), `Refresh(ctx, refreshToken)` (ParseRefresh → IssueAccess), `Logout()` (no-op stateless).
- [ ] 1.5 Adaptar `go.mod`: `go get github.com/golang-jwt/jwt/v5` + `golang.org/x/crypto/bcrypt`.
- [ ] 1.6 Unit tests: tokens issue/parse/expirado/firma inválida; service.Login ok/incorrecto/inexistente (fake repo); service.Refresh ok/firma inválida.
- [ ] 1.7 Integración: repository.GetByUsername (found/notfound), UpdateLastLogin, seeder (vacía→crea, no vacía→no duplica) — Postgres real (patrón testDB).

## Phase 2: Config + seed + API + middleware + wiring

- [ ] 2.1 `internal/config/config.go`: requerir `JWT_SECRET` (≥32 chars), `ADMIN_USERNAME`, `ADMIN_PASSWORD`; `COOKIE_SECURE` default false; validación.
- [ ] 2.2 `internal/auth/seeder.go`: `EnsureInitialAdmin(ctx, repo, username, password)` — si count==0 → insert bcrypt costo 12.
- [ ] 2.3 `internal/api/middleware_auth.go`: `requireAuth(next)` — Bearer access válido → claims en ctx; 401 on fail.
- [ ] 2.4 `internal/api/auth_handlers.go`: `handleLogin` (200 access + Set-Cookie refresh httpOnly Strict Secure=COOKIE_SECURE; 401 genérico), `handleRefresh` (leer cookie → 200 access | 401), `handleLogout` (MaxAge=-1 → 204), `handleMe` (protegido → 200 id+username).
- [ ] 2.5 `internal/api/server.go`: montar rutas `POST /api/auth/login|refresh|logout`, `GET /api/auth/me` (con requireAuth), pasar dependencias (service, cookie secure) por opts o campo.
- [ ] 2.6 `cmd/server/main.go`: construir repo auth + service + `EnsureInitialAdmin` antes del listener; pasar a Server.
- [ ] 2.7 Test API con httptest: login 200/401, refresh 200/401, logout 204, me 200/401.
- [ ] 2.8 `.env.example`: JWT_SECRET, ADMIN_USERNAME, ADMIN_PASSWORD, COOKIE_SECURE documentados.
- [ ] 2.9 `go vet`, `gofmt -l .`, `go test ./... -count=1` — todo verde.

## Phase 3: E2E manual

- [ ] 3.1 `docker compose up --build` con .env (JWT_SECRET + ADMIN_*) — seed crea admin si tabla vacía.
- [ ] 3.2 curl: login → access; me con Bearer → 200 id; refresh con cookie → nuevo access; logout → 204; login con password mala → 401; sin JWT_SECRET → backend no arranca.

## Notas

- Sin migración nueva (tabla `admins` existe).
- Nunca loguear JWT completo ni password.
- Repo y service sin interfaz (patrón groups); si tests de handlers necesitan fake, definir interfaz mínima donde se consume.