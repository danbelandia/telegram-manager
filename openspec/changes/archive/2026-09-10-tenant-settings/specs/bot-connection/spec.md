# Delta for Bot Connection

## ADDED Requirements

### Requirement: Polling de estado runtime

El sistema MUST soportar un polling periódico del estado del bot
(contra Telegram `getMe`) con intervalo configurable (default 60s).
El estado resultante MUST alimentar el campo `bot_status` consultable
vía `GET /api/tenants/me/status` y vía `GET /api/tenants/me`. El
poller MUST reiniciarse cuando el token se rota (ver delta de
Tenant Settings).

#### Scenario: Polling exitoso

- GIVEN un bot con token válido y polling habilitado
- WHEN transcurre el intervalo de 60s
- THEN se ejecuta getMe y `bot_status` se actualiza a `connected`

#### Scenario: Polling fallido

- GIVEN un bot con token revocado
- WHEN se ejecuta getMe durante el polling
- THEN `bot_status` se actualiza a `disconnected`

#### Scenario: Reinicio tras rotación de token

- GIVEN un poller con token A activo
- WHEN el admin rota el token a B vía `PUT /api/tenants/me/bot-token`
- THEN el poller se reinicia inmediatamente con token B
- AND el primer getMe con B determina el nuevo `bot_status`
