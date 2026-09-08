# Delta for Auth

## MODIFIED Requirements

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
