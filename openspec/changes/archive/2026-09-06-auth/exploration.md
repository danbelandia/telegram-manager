# Exploration: auth — paso 9: Autenticación

## Current State

- Migración `00001_create_admins.sql` YA crea la tabla `admins` con
  `id`, `username`, `password_hash`, `created_at`, `last_login_at` —
  lista para usar, sin cambios de esquema.
- `config.Load()` ya lee `JWT_SECRET` pero NO valida que esté presente:
  hay que requerirlo (fail fast, como TELEGRAM_BOT_TOKEN).
- `.env.example` ya documenta `JWT_SECRET=`.
- `api.Server` existente: `net/http` estándar con `http.ServeMux` (Go
  1.22+ patrones de método), un solo route (`GET /api/health`) y helper
  `respond()` para JSON. Sin middleware de auth todavía.
- Librerías autorizadas por `docs/backend-go-skill.md` §7:
  `golang-jwt/jwt/v5` (tokens) + `golang.org/x/crypto/bcrypt`
  (passwords).
- Sin ORM: repositorios concretos con `database/sql` + driver pgx
  (patrón de `internal/groups`).
- Tests de repositorio contra PostgreSQL real en Docker (patrón
  `testDB()` del paso 7, sin testcontainers aún).

## Especificación (AGENTS.md §17 + 17.1)

- JWT con access token corto (15 min) + refresh token largo (7 días).
  Sin sesiones en base de datos.
- Refresh token en cookie `httpOnly`, `Secure`, `SameSite=Strict`.
  Access token nunca en localStorage (riesgo XSS).
- Passwords con bcrypt, costo 12.
- Login con `username + password` sobre la tabla `admins`.
- Middleware de auth solo verifica identidad (¿quién es?); la
  autorización es capa aparte (backend-go-skill §7).
- Nunca loguear el JWT completo.

## Hallazgo clave: cómo se crea el primer admin

La tabla `admins` existe pero nadie la puebla. El panel no tiene registro
público (son administradores, no usuarios finales). Hay que decidir el
bootstrap del admin inicial:

| Opción | Pros | Contras |
|--------|------|---------|
| **Seed al arrancar si tabla vacía** (env `ADMIN_USERNAME`/`ADMIN_PASSWORD`) | Cero pasos manuales, funciona en compose; fail-fast claro; hasheado con bcrypt igual que el login | Password en env (aceptable: no es código, ya es el modelo de secretos del proyecto) |
| Comando CLI (`go run ./cmd/create-admin`) | Explícito, reproducible | Otro binario/comando que documentar y correr antes de poder loguearse |
| SQL manual | Simplicidad total | Password hash hay que generarlo a mano — frágil |

**Recomendación**: Seed al arrancar si tabla vacía. Es el mismo espíritu
del resto del proyecto: desarrollo simple y funcionando desde el primer
`docker compose up`. En producción la creación manual de admins será un
paso documentado (fuera de MVP).

## Otras decisiones a confirmar en design

1. **Refresh token rotable o stateless**: sin sesiones en DB (spec
   17.1), un refresh JWT stateless con `exp` de 7 días es lo simple.
   Rotación real (invalidar el anterior) requeriría estado → fuera de
   MVP. Decisión probable: stateless, sin rotación.
2. **Claims del access token**: `sub` (admin id), `username`, `exp`,
   `iat` — mínimo necesario para identificar.
3. **Logout**: el spec no lo pide explícitamente; con refresh stateless
   el logout solo borra la cookie del cliente. Incluir `POST
   /api/auth/logout` trivial (borra cookie) porque un panel admin sin
   logout es mala UX → decisión menor, se puede incluir sin costo.
4. **last_login_at**: actualizarlo en cada login exitoso (campo ya
   existe).
5. **Endpoints**: `POST /api/auth/login`, `POST /api/auth/refresh`,
   `POST /api/auth/logout`. Middleware `requireAuth` que se aplicará a
   las rutas del paso 10; en este cambio se puede exponer un endpoint
   `/api/auth/me` protegido para validar el flujo end-to-end.

## Scope recomendado (para proposal)

- Requerir `JWT_SECRET` en config (fail fast) + validación mínima
  (longitud).
- `internal/auth`: service (login/refresh/logout), repo sobre `admins`
  (GetByUsername, UpdateLastLogin), tokens (issue/parse access+refresh).
- Seed de admin inicial si tabla vacía (env `ADMIN_USERNAME` +
  `ADMIN_PASSWORD`).
- Handlers `POST /api/auth/login|refresh|logout` + `GET /api/auth/me`
  protegido.
- Middleware `requireAuth` (identidad: valida access token, inyecta
  admin).
- Tests unit (service/tokens) + integración repo (Postgres).
- **Fuera de scope**: registro público de admins, roles/permisos por
  grupo (autorización, paso 10+), rotación de refresh, olvido de
  contraseña, frontend de login (paso 11-12).

## Riesgos

- Dependencias nuevas (jwt, bcrypt) — autorizadas por skill §7; agregar
  a go.mod.
- Cookie Secure: en desarrollo local (http://localhost) el flag `Secure`
  impide el envío — decisión: `Secure` condicional por configuración
  (`COOKIE_SECURE=false` en dev, `true` en prod) o solo `SameSite` +
  `httpOnly` documentando el riesgo en http. Se resuelve en design;
  respetar el spec (Secure) con escape para dev.
- Refresh stateless: si se filtra, es válido hasta expirar — documentar
  como limitación del MVP.