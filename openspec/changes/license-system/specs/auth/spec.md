# Delta for Auth

## MODIFIED Requirements

### Requirement: Access token

El sistema MUST emitir un access token JWT firmado con `JWT_SECRET`
con expiración de 15 minutos y claims de identidad (`sub`, `username`,
`tenant_id`, `tenant_slug`, `is_super_admin`). Tokens legacy sin
`is_super_admin` MUST tratarlo como `false`.
(Previously: claims included sub, username, tenant_id — no is_super_admin or tenant_slug in original)

#### Scenario: Access token válido

- GIVEN un login exitoso
- THEN el `access_token` decodifica con `sub` = id del admin,
  `username`, `tenant_id`, `tenant_slug` e `is_super_admin` con
  exp ~15 min

#### Scenario: Super-admin token includes flag

- GIVEN un login de un admin con `is_super_admin = true`
- WHEN se decodifica el token
- THEN `is_super_admin = true` está presente en claims

#### Scenario: Access legacy sin is_super_admin

- GIVEN un access token válido en firma pero sin claim `is_super_admin`
- WHEN se llama a una ruta protegida
- THEN `is_super_admin` se infiere como `false`

### Requirement: Identidad protegida

El sistema MUST proteger rutas verificando el access token (que MUST
incluir `tenant_id` e `is_super_admin`) e inyectar la identidad del
admin para el handler. `GET /api/auth/me` MUST exponer `tenant_id`,
`tenant_slug` y `is_super_admin`.
(Previously: me returned id, username, tenant_id, tenant_slug — no is_super_admin)

#### Scenario: Ruta protegida con access válido

- GIVEN un access token válido en `Authorization: Bearer ...`
- WHEN se llama `GET /api/auth/me`
- THEN responde 200 con id, username, `tenant_id`, `tenant_slug` y
  `is_super_admin` del admin

#### Scenario: Super-admin me includes flag

- GIVEN un admin super-authenticated
- WHEN se llama `GET /api/auth/me`
- THEN la respuesta incluye `is_super_admin: true`

#### Scenario: Ruta protegida sin access

- GIVEN una petición sin access token o con token inválido
- WHEN se llama `GET /api/auth/me`
- THEN responde 401
