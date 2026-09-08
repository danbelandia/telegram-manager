# Backward Compatibility Specification

## Purpose

Un deploy existente (modo legacy con `ADMIN_*` / `TELEGRAM_BOT_TOKEN`)
MUST seguir arrancando tras el cambio: opera como tenant `default`
hasta que el admin vuelva a loguearse. No se remueven esas variables
en este slice.

## Requirements

### Requirement: Arranque legacy intacto

El sistema MUST arrancar con solo `ADMIN_USERNAME`/`ADMIN_PASSWORD` y
`TELEGRAM_BOT_TOKEN` configurados, sin exigir `TENANT_TOKEN_ENC_KEY`
para el path legacy. `EnsureInitialAdmin` MUST adoptar el tenant
`default` (ver delta de Auth).

#### Scenario: Deploy existente arranca

- GIVEN una instalación previa con `ADMIN_*` y `TELEGRAM_BOT_TOKEN`
- WHEN el backend arranca con la nueva versión
- THEN arranca con normalidad y el admin inicial queda en `default`

### Requirement: Sesiones legacy degradan con gracia

Un access token legacy (sin `tenant_id`) MUST rechazarse con 401 y
mensaje de re-login requerido, mientras su refresh MUST seguir
permitiendo obtener un access con tenant (ver delta de Auth). El
sistema MUST NOT exigir re-signup al admin legacy.

#### Scenario: Admin legacy vuelve a entrar

- GIVEN un admin legacy con refresh válido sin tenant
- WHEN llama `POST /api/auth/refresh`
- THEN obtiene un access con su `tenant_id` sin crear cuenta nueva

### Requirement: Sin remoción de variables legacy

El sistema MUST NOT remover `TELEGRAM_BOT_TOKEN` ni `ADMIN_*` en este
slice: el modo legacy de un solo tenant sigue soportado vía `default`.

#### Scenario: Env legacy sin slug

- GIVEN un `.env` sin variables de tenant
- WHEN el backend arranca
- THEN no falla por configuración multitenant faltante
