# Design: auth — paso 9: Autenticación del panel

## Technical Approach

Servicio puro + repositorio concreto (patrón de `internal/groups`),
handlers delgados en `api`, middleware `requireAuth` que solo
verifica identidad. Tokens con `golang-jwt/jwt/v5`, passwords con
`bcrypt` costo 12. Seed del admin inicial al arrancar si la tabla está
vacía.

## Architecture Decisions

| # | Decisión | Alternativas | Por qué |
|---|----------|--------------|---------|
| 1 | JWT HS256 con `golang-jwt/jwt/v5`, claims custom `sub`+`username` | claims estándar | kit recomendado por skill §7; HS256 simple y suficiente para un solo backend |
| 2 | Refresh token stateless sin rotación (exp 7d) | sesiones en DB, rotación | spec 17.1: "No usar sesiones en base de datos para el MVP"; rotación real requiere estado → fuera de MVP; limitación documentada |
| 3 | `Secure` de la cookie según `COOKIE_SECURE` (dev http → false; prod https → true) | Secure siempre | http://localhost no envía cookies Secure; el flag condicional respeta el spec en producción |
| 4 | Bootstrap: seed del primer admin al arrancar si `count(admins)==0` (env `ADMIN_USERNAME`/`ADMIN_PASSWORD`, bcrypt) | comando CLI, SQL manual | cero pasos manuales en `docker compose up`; no duplica (solo tabla vacía); password nunca en código |
| 5 | Middleware `requireAuth` solo identidad (inyecta `context` con admin) | middleware que además autoriza | skill §7: identidad ≠ autorización; la autorización por grupos es capa aparte (pasos 10+) |
| 6 | Los handlers auth NO requieren access token (login/refresh/logout) más allá de verificar cookie/credenciales; `me` sí | considerar `me` sin auth | `me` es la prueba de que el middleware funciona; protegido por requireAuth |
| 7 | Config: `JWT_SECRET` requerido si el módulo auth está activo (siempre en MVP) | opcional | fail fast; sin JWT_SECRET no hay auth segura |
| 8 | Repositorio auth: `GetByUsername`, `UpdateLastLogin` — concreto, sin interfaz | interfaz + mock | patrón de `internal/groups`; tests de servicio usan repo real con Postgres o se testea el handler con fake si hace falta |
| 9 | Envelope JSON único en api: `respond()` | — | convención existente del proyecto |

## Data Flow

```
POST /api/auth/login
  body: {"username","password"}
  └─ authService.Login
       ├─ repo.GetByUsername → ErrNotFound → 401 genérico
       ├─ bcrypt.CompareHashAndPassword → error → 401 genérico
       ├─ tokens.IssueAccess(admin) (15min) / IssueRefresh(admin) (7d)
       └─ repo.UpdateLastLogin(admin.ID)
  → 200 {"access_token": "..."} + Set-Cookie refresh_token (httpOnly, Strict, Secure=COOKIE_SECURE)

POST /api/auth/refresh
  cookie refresh_token
  └─ authService.Refresh → tokens.ParseRefresh → nueva access token
  → 200 {"access_token": "..."}

POST /api/auth/logout
  └─ borra cookie (MaxAge=-1)
  → 204

GET /api/auth/me  (requireAuth)
  Authorization: Bearer <access>
  └─ middleware valida token, inyecta admin en ctx
  → 200 {"id":.., "username":..}
```

## File Changes

| File | Acción | Descripción |
|------|--------|-------------|
| `backend/internal/auth/admin.go` | Create | modelo Admin (ID, Username, PasswordHash, CreatedAt, LastLoginAt) |
| `backend/internal/auth/repository.go` | Create | GetByUsername, UpdateLastLogin (database/sql + pgx) |
| `backend/internal/auth/service.go` | Create | Login, Refresh, Logout lógica + bcrypt |
| `backend/internal/auth/tokens.go` | Create | IssueAccess, IssueRefresh, ParseAccess, ParseRefresh (jwt/v5) |
| `backend/internal/auth/seeder.go` | Create | EnsureInitialAdmin(ctx, repo, cfg) si tabla vacía |
| `backend/internal/api/auth_handlers.go` | Create | handleLogin, handleRefresh, handleLogout, handleMe |
| `backend/internal/api/middleware_auth.go` | Create | requireAuth |
| `backend/internal/config/config.go` | Modify | +JWTSecret validation, +CookieSecure, +AdminUsername/Password |
| `backend/cmd/server/main.go` | Modify | wiring: repo, service, seeder, rutas auth |
| `backend/internal/auth/..._test.go` | Create | unit + integración |
| `backend/go.mod` | Modify | +jwt/v5, +bcrypt |

## Interfaces / Contracts

```go
// internal/auth/repository.go — repositorio concreto sobre admins
func (r *Repository) GetByUsername(ctx context.Context, username string) (Admin, error)
// ErrNotFound si no existe (patrón groups.ErrNotFound)

func (r *Repository) UpdateLastLogin(ctx context.Context, id int64, at time.Time) error

// internal/auth/tokens.go
func (s *Service) IssueAccess(admin Admin) (string, error)         // 15 min
func (s *Service) ParseAccess(token string) (*Claims, error)        // sub, username
func (s *Service) IssueRefresh(admin Admin) (string, error)         // 7 días
func (s *Service) ParseRefresh(token string) (*Claims, error)
```

```go
// internal/auth/service.go
func (s *Service) Login(ctx context.Context, username, password string) (accessToken string, err error)
func (s *Service) Refresh(ctx context.Context) (accessToken string, err error) // lee cookie del request en handler
func (s *Service) Logout() // borra cookie
```

```go
// internal/api/middleware_auth.go
func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc
// valida Bearer access; on fail 401; inyecta claims en ctx

// internal/api/auth_handlers.go
func (s *Server) handleLogin(w, r)  // lee body, llama service, setea cookie
func (s *Server) handleRefresh(w, r)
func (s *Server) handleLogout(w, r)
func (s *Server) handleMe(w, r)     // protected
```

```go
// internal/auth/seeder.go
func EnsureInitialAdmin(ctx context.Context, repo *Repository, username, password string) error
// si count==0 → insert bcrypt costo 12; si >0 → no-op
```

## Config additions

```env
JWT_SECRET=
COOKIE_SECURE=false   # true en producción (https)
ADMIN_USERNAME=
ADMIN_PASSWORD=
```

Config validation: `JWT_SECRET` requerido (len >= 32 recomendado);
`ADMIN_USERNAME`/`ADMIN_PASSWORD` requeridos (el seed los necesita;
sin ellos, no hay admin inicial → log warn o error en dev).

## Testing Strategy

| Capa | Qué | Cómo |
|------|-----|------|
| Unit | tokens: issue/parse access (15min), refresh (7d), firma inválida → error, expirado → error | jwt/v5, time manipulado con clock inyectado |
| Unit | service.Login: password ok → token; password mal → ErrCredentials; username sin existir → ErrCredentials | repo fake (interfaz pequeña del lado consumidor) |
| Integración | repo.GetByUsername: encontrado, no encontrado, UpdateLastLogin | Postgres real (testDB patrón groups) |
| Integración | seeder: tabla vacía → crea y login funciona; tabla con admin → no duplica | Postgres real |
| API | handlers: login 200/401, refresh 200/401, logout 204, me 200/401 (auth header inválido/ausente) | httptest + Server real |
| E2E manual | login → me → refresh → logout vía curl | compose up + ADMIN_* env |

## Migration / Rollout

No hay migración nueva (tabla `admins` existe). El seed corre al
arrancar. Sin rollback de datos (seed idempotente; no borra admins).

## Open Questions

- ¿`ADMIN_PASSWORD` requerido en config siempre, o solo si tabla vacía?
  → Decisión: requerido en arranque si el seed está habilitado (env
  `AUTH_BOOTSTRAP=1` default true en dev). Simplifica: se valida en
  config. (A confirmar en apply si molesta.)