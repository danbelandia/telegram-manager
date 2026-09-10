# Delta for Auth

## MODIFIED Requirements

### Requirement: Identidad protegida

El sistema MUST proteger rutas verificando el access token (que MUST
incluir `tenant_id`) e inyectar la identidad del admin para el
handler. `GET /api/auth/me` MUST exponer `tenant_id` y `tenant_slug`.
(Previously: `me` devolvía solo id y username, luego se agregó
`tenant_id` sin `tenant_slug`.)

#### Scenario: Ruta protegida con access válido

- GIVEN un access token válido en `Authorization: Bearer ...`
- WHEN se llama `GET /api/auth/me`
- THEN responde 200 con id, username, `tenant_id` y `tenant_slug` del admin

#### Scenario: Ruta protegida sin access

- GIVEN una petición sin access token o con token inválido
- WHEN se llama `GET /api/auth/me`
- THEN responde 401
