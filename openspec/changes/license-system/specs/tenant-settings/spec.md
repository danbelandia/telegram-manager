# Delta for Tenant Settings

## MODIFIED Requirements

### Requirement: Datos y estado del tenant

El sistema MUST exponer `GET /api/tenants/me` que retorne: `slug`,
`bot_username`, `bot_status` (valores: `connected`, `disconnected`,
`unknown`), `created_at`, y datos de licencia: `license_status`,
`plan`, `trial_ends_at`, `expires_at`, `max_groups`,
`max_messages_day`. La respuesta MUST NOT incluir el token.
(Previously: only returned slug, bot_username, bot_status, created_at — no license data)

#### Scenario: Tenant autenticado consulta sus datos

- GIVEN un admin autenticado con tenant válido
- WHEN se llama `GET /api/tenants/me`
- THEN responde 200 con slug, bot_username, bot_status, created_at,
  license_status, plan, trial_ends_at, expires_at, max_groups y
  max_messages_day

#### Scenario: Tenant inexistente

- GIVEN un admin cuyo tenant ya no existe en DB
- WHEN se llama `GET /api/tenants/me`
- THEN responde 404 con código `NOT_FOUND`

#### Scenario: Tenant en trial muestra datos de trial

- GIVEN un admin autenticado con tenant en `status = 'trial'`
- WHEN se llama `GET /api/tenants/me`
- THEN la respuesta incluye `license_status = 'trial'`,
  `trial_ends_at` con la fecha de expiración del trial, y
  `expires_at = null`
