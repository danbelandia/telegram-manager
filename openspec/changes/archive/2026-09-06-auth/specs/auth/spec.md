# Auth Specification

## Purpose

Autenticar administradores del panel con JWT (AGENTS.md §17 / 17.1):
login con `username + password` sobre la tabla `admins`, access token
corto (15 min) y refresh token largo (7 días) en cookie httpOnly. El
middleware de auth solo verifica identidad; la autorización por
grupos/roles es una capa aparte (pasos posteriores).

## Requirements

### Requirement: Login con credenciales

El sistema MUST autenticar un admin con `username + password`
almacenado como hash bcrypt en `admins.password_hash`. Con credenciales
válidas MUST devolver un access token y setear la cookie de refresh;
con credenciales inválidas MUST responder 401 sin exponer cuál campo
falló.

#### Scenario: Login exitoso

- GIVEN un admin existente con password correcto
- WHEN se llama `POST /api/auth/login`
- THEN responde 200 con `access_token`, setea cookie `refresh_token`
  httpOnly, y actualiza `last_login_at`

#### Scenario: Password incorrecto

- GIVEN un admin existente con password incorrecto
- WHEN se llama `POST /api/auth/login`
- THEN responde 401 con mensaje genérico de credenciales inválidas

#### Scenario: Username inexistente

- GIVEN un username que no existe en `admins`
- WHEN se llama `POST /api/auth/login`
- THEN responde 401 con el mismo mensaje genérico (no revela si el
  username existe)

### Requirement: Access token

El sistema MUST emitir un access token JWT firmado con `JWT_SECRET` con
expiración de 15 minutos y claims de identidad (`sub`, `username`).

#### Scenario: Access token válido

- GIVEN un login exitoso
- THEN el `access_token` decodifica con `sub` = id del admin y
  `username` = username, con exp ~15 min

### Requirement: Refresh token

El sistema MUST emitir un refresh token JWT con expiración de 7 días,
stateless (sin sesiones en DB), y entregarlo en una cookie
`httpOnly`, `SameSite=Strict` — `Secure` según `COOKIE_SECURE`.

#### Scenario: Cookie de refresh entregada

- GIVEN un login exitoso
- THEN la respuesta incluye cookie `refresh_token` con flags httpOnly,
  SameSite=Strict y Secure = valor de `COOKIE_SECURE`

#### Scenario: Refresh genera nuevo access

- GIVEN una cookie de refresh válida
- WHEN se llama `POST /api/auth/refresh`
- THEN responde 200 con un nuevo `access_token`

#### Scenario: Refresh inválido o ausente

- GIVEN cookie de refresh ausente, expirada o mal firmada
- WHEN se llama `POST /api/auth/refresh`
- THEN responde 401

### Requirement: Identidad protegida

El sistema MUST proteger rutas verificando el access token e inyectar
la identidad del admin para el handler.

#### Scenario: Ruta protegida con access válido

- GIVEN un access token válido en `Authorization: Bearer ...`
- WHEN se llama `GET /api/auth/me`
- THEN responde 200 con id y username del admin

#### Scenario: Ruta protegida sin access

- GIVEN una petición sin access token o con token inválido
- WHEN se llama `GET /api/auth/me`
- THEN responde 401

### Requirement: Logout

El sistema MUST permitir cerrar sesión borrando la cookie de refresh.

#### Scenario: Logout

- GIVEN una sesión con cookie de refresh
- WHEN se llama `POST /api/auth/logout`
- THEN responde 204 y la cookie queda expirada/eliminada

### Requirement: Bootstrap del admin inicial

El sistema MUST crear un admin inicial al arrancar si la tabla `admins`
está vacía, usando `ADMIN_USERNAME` y `ADMIN_PASSWORD`, con password
hasheado con bcrypt costo 12. Si la tabla no está vacía, MUST no crear
nada.

#### Scenario: Seed en tabla vacía

- GIVEN la tabla `admins` vacía y env `ADMIN_USERNAME`/`ADMIN_PASSWORD`
  configurados
- WHEN el backend arranca
- THEN existe un admin con ese username y password hash bcrypt, y se
  puede loguear

#### Scenario: Seed no duplica

- GIVEN la tabla `admins` con al menos un admin
- WHEN el backend arranca
- THEN no se inserta ningún admin nuevo

### Requirement: Secrets y configuración

El sistema MUST fallar al arrancar si `JWT_SECRET` está vacío, y MUST
NUNCA loguear passwords ni tokens JWT completos.

#### Scenario: JWT_SECRET faltante

- GIVEN arranque sin `JWT_SECRET`
- WHEN el backend inicia
- THEN devuelve error de configuración y no arranca

#### Scenario: No loguear secretos

- GIVEN cualquier flujo de auth
- THEN los logs no contienen el password ni el JWT completo