# Proposal: auth — paso 9: Autenticación del panel

## Intent

Implementar la autenticación del panel (AGENTS.md §17 / 17.1): login
con `username + password` sobre la tabla `admins` (ya existe desde
00001), emisión de JWT access (15 min) + refresh (7 días) en cookie
httpOnly, y el middleware de identidad que el paso 10 (API REST)
protegerá.

## Scope

### In Scope
- Config: `JWT_SECRET` requerido (fail fast) + validación de longitud.
- `internal/auth`: servicio (login, refresh, logout), repositorio sobre
  `admins` (GetByUsername, UpdateLastLogin), emisión/validación de
  tokens (access + refresh stateless).
- Bootstrap del admin inicial: seed al arrancar si la tabla está vacía
  (env `ADMIN_USERNAME` + `ADMIN_PASSWORD`), password hasheado con
  bcrypt.
- Handlers: `POST /api/auth/login`, `POST /api/auth/refresh`,
  `POST /api/auth/logout`, `GET /api/auth/me` (protegido, valida el
  flujo end-to-end).
- Middleware `requireAuth`: valida access token e inyecta la identidad
  del admin en el request.
- Cookie de refresh: `httpOnly`, `SameSite=Strict`, `Secure` condicional
  vía `COOKIE_SECURE` (dev local http usa false; prod true).
- Tests: unit (service/tokens/middleware) + integración repositorio
  (Postgres real, patrón testDB).

### Out of Scope
- Autorización por roles/grupos (capa aparte; paso 10+).
- Registro público de admins.
- Rotación de refresh tokens (stateless, sin sesiones — spec 17.1).
- Olvido/recuperación de contraseña.
- Frontend de login (pasos 11-12).
- Rate limiting del endpoint de login (a evaluar en pasos futuros).

## Capabilities

### New Capabilities
- `auth`: autenticación del panel con JWT (login/refresh/logout/me).

### Modified Capabilities
- `repo-bootstrap`: la tabla `admins` ya existía; este cambio la
  consume y agrega su seed. No cambia esquema.

## Approach

Servicio puro en `internal/auth` (tokens + validación bcrypt),
repositorio concreto (patrón `internal/groups`), handlers delgados en
`api` y un middleware `requireAuth` que solo verifica identidad.
Bootstrap: si `SELECT count(*) FROM admins` es 0 al arrancar, insertar
el admin de env con bcrypt (costo 12). Nunca loguear tokens.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `backend/internal/auth/` | New | service, repository, tokens, middleware |
| `backend/internal/api/` | Modify | handlers auth + route + requireAuth |
| `backend/internal/config/config.go` | Modify | JWT_SECRET required, COOKIE_SECURE, ADMIN_* |
| `backend/cmd/server/main.go` | Modify | wiring service + seed |
| `backend/go.mod` | Modify | +jwt/v5, +bcrypt |
| `openspec/specs/auth/spec.md` | New | spec base (al archivar) |
| `.env.example` | Modify | documentar JWT_SECRET/admin/COOKIE_SECURE |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Refresh stateless filtrado válido 7 días | Baja | Cookie httpOnly/Secure/Strict; documentar limitación |
| Seed deja password en env | Media | Ya es el modelo de secretos del proyecto (env); documentar cambio en prod |
| Login sin rate limit | Media | Fuera de scope MVP, anotado para futuro |
| `Secure` en dev no envía cookie | Alta (si mal config) | `COOKIE_SECURE` condicional, default false en dev, documentado |

## Rollback Plan

Quitar handlers + middleware (rutas auth no existen más); el seed solo
corre si tabla vacía (no re-crea admin si ya existe). NO hay migración
nueva. go.mod conserva dependencias (o `go mod tidy` las limpia).

## Dependencies

- Tabla `admins` (00001). Librerías: `golang-jwt/jwt/v5`,
  `golang.org/x/crypto/bcrypt` (autorizadas por backend-go-skill §7).

## Success Criteria

- [ ] `JWT_SECRET` vacío → backend no arranca (fail fast).
- [ ] `docker compose up` con tabla vacía → admin seed creado (bcrypt),
      login funciona.
- [ ] Login correcto → access token + refresh cookie; login incorrecto
      → 401 sin expirar nada.
- [ ] `GET /api/auth/me` con access válido → identidad; sin access →
      401.
- [ ] `POST /api/auth/refresh` con cookie válida → nuevo access.
- [ ] Refresh cookie con flags httpOnly y SameSite=Strict (Secure según
      COOKIE_SECURE).
- [ ] Tests unit + integración verdes (go test ./... -count=1).
- [ ] No se loguea nunca el JWT completo ni el password.