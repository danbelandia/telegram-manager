# Auth Specification

## Purpose

Autenticar administradores del panel con JWT (AGENTS.md §17 / 17.1):
login con `username + password` sobre la tabla `admins`, access token
corto (15 min) y refresh token largo (7 días) en cookie httpOnly. El
middleware de auth solo verifica identidad; la autorización por
grupos/roles es una capa aparte (pasos posteriores).

> **Histórico de slices**: Slice 0 multitenancy-backend (archivada en
> `openspec/changes/archive/2026-09-08-slice-0-multitenancy-backend/`,
> mergeada en este archivo) extiende los claims con `tenant_id`: el
> access lo exige (los tokens legacy sin tenant degradan con 401 +
> re-login), el refresh re-emite el access con el `tenant_id` actual
> del admin, `GET /api/auth/me` expone `tenant_id`, y el seed del admin
> inicial adopta el tenant `default`.

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

El sistema MUST emitir un access token JWT firmado con `JWT_SECRET`
con expiración de 15 minutos y claims de identidad (`sub`, `username`,
`tenant_id`).
(Previously: claims eran solo `sub` y `username`, sin tenant.)

#### Scenario: Access token válido

- GIVEN un login exitoso
- THEN el `access_token` decodifica con `sub` = id del admin,
  `username` = username y `tenant_id` = tenant del admin, con exp ~15 min

#### Scenario: Access legacy sin tenant

- GIVEN un access token pre-multitenancy válido en firma pero sin
  claim `tenant_id`
- WHEN se llama a una ruta protegida
- THEN responde 401 con mensaje de re-login requerido

### Requirement: Refresh token

El sistema MUST emitir un refresh token JWT con expiración de 7 días,
stateless (sin sesiones en DB), y entregarlo en una cookie
`httpOnly`, `SameSite=Strict` — `Secure` según `COOKIE_SECURE`. Al
refrescar, MUST re-emitir el access con el `tenant_id` actual del
admin, incluso si el refresh fue emitido antes de multitenancy.
(Previously: el refresh re-emitía access sin tenant.)

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

#### Scenario: Refresh legacy re-emite con tenant

- GIVEN un refresh válido emitido antes de multitenancy (sin tenant)
- WHEN se llama `POST /api/auth/refresh`
- THEN responde 200 con un `access_token` que incluye el `tenant_id`
  actual del admin

### Requirement: Identidad protegida

El sistema MUST proteger rutas verificando el access token (que MUST
incluir `tenant_id`) e inyectar la identidad del admin para el
handler. `GET /api/auth/me` MUST exponer el `tenant_id`.
(Previously: `me` devolvía solo id y username; no se exigía tenant.)

#### Scenario: Ruta protegida con access válido

- GIVEN un access token válido en `Authorization: Bearer ...`
- WHEN se llama `GET /api/auth/me`
- THEN responde 200 con id, username y `tenant_id` del admin

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

El sistema MUST crear un admin inicial al arrancar si la tabla
`admins` está vacía, usando `ADMIN_USERNAME` y `ADMIN_PASSWORD`, con
password hasheado con bcrypt costo 12, adoptando el tenant `default`
(creándolo si no existe). Si la tabla no está vacía, MUST no crear
nada.
(Previously: el seed no asignaba ningún tenant.)

#### Scenario: Seed en tabla vacía

- GIVEN la tabla `admins` vacía y env `ADMIN_USERNAME`/`ADMIN_PASSWORD`
  configurados
- WHEN el backend arranca
- THEN existe un admin con ese username y password hash bcrypt,
  vinculado al tenant `default`, y se puede loguear

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

---

## Slice 2 — tenant-settings (2026-09-10)

### MODIFIED Requirements

#### Requirement: Identidad protegida

El sistema MUST proteger rutas verificando el access token (que MUST
incluir `tenant_id`) e inyectar la identidad del admin para el
handler. `GET /api/auth/me` MUST exponer `tenant_id` y `tenant_slug`.

##### Scenario: Ruta protegida con access válido

- GIVEN un access token válido en `Authorization: Bearer ...`
- WHEN se llama `GET /api/auth/me`
- THEN responde 200 con id, username, `tenant_id` y `tenant_slug` del admin

##### Scenario: Ruta protegida sin access

- GIVEN una petición sin access token o con token inválido
- WHEN se llama `GET /api/auth/me`
- THEN responde 401
