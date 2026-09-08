# Tenant Provisioning Specification

## Purpose

Alta transaccional de tenants bot-per-tenant (1 usuario = 1 tenant,
tier Pro). Cada signup verifica el bot contra Telegram (fuente de
verdad) antes de persistir, crea el tenant con su admin y guarda el
token cifrado, sin dejar filas huérfanas ante fallos parciales.

## Requirements

### Requirement: Signup crea tenant, admin y token cifrado

El sistema MUST exponer `POST /api/auth/signup` que acepte
`{slug, username, password, bot_token}`. Con datos válidos MUST
validar el token con Telegram `getMe` ANTES de persistir, crear una
fila en `tenants` y una en `admins` vinculada a ella, guardar el token
cifrado con AES-GCM bajo `TENANT_TOKEN_ENC_KEY`, y responder 201 con
`{tenant{id,slug}, admin{id,username}}`. La respuesta y los logs MUST
NOT contener el valor del token.

#### Scenario: Signup happy path

- GIVEN un slug libre, un username libre, un password válido y un bot
  token válido
- WHEN se llama `POST /api/auth/signup`
- THEN responde 201 con `tenant{id,slug}` y `admin{id,username}`
- AND ni la respuesta ni los logs contienen el token

#### Scenario: Atomicidad ante fallo de inserción

- GIVEN que `getMe` tiene éxito pero la inserción en DB falla
- WHEN se llama `POST /api/auth/signup`
- THEN responde 5xx y no queda ninguna fila en `tenants` ni `admins`

### Requirement: Slug duplicado

El sistema MUST rechazar un `slug` ya registrado con 409 y código
`CONFLICT`, sin llamar a Telegram ni persistir nada.

#### Scenario: Slug en uso

- GIVEN un tenant existente con slug `acme`
- WHEN se llama `POST /api/auth/signup` con slug `acme`
- THEN responde 409 con código `CONFLICT`

### Requirement: Username duplicado global

El sistema MUST tratar `username` como UNIQUE global (Q1-a): un
username en uso por cualquier tenant MUST rechazarse con 409 y código
`CONFLICT`, aunque el slug sea distinto.

#### Scenario: Username en uso en otro tenant

- GIVEN un admin existente con username `juan` en otro tenant
- WHEN se llama `POST /api/auth/signup` con username `juan` y slug libre
- THEN responde 409 con código `CONFLICT`

### Requirement: Bot token inválido

El sistema MUST validar el token con `getMe` antes de persistir. Si
Telegram lo rechaza, MUST responder 502 con código `TELEGRAM_ERROR`
y no persistir ninguna fila.

#### Scenario: Token revocado o inexistente

- GIVEN un `bot_token` que Telegram rechaza en `getMe`
- WHEN se llama `POST /api/auth/signup`
- THEN responde 502 con código `TELEGRAM_ERROR`
- AND no queda ninguna fila en `tenants` ni `admins`

### Requirement: Validación de entrada

El sistema MUST rechazar con 400 y código `VALIDATION_ERROR` los
signups con campos faltantes/vacíos o password que no cumpla la
política mínima, sin llamar a Telegram ni persistir nada.

#### Scenario: Campos faltantes

- GIVEN un signup sin `password`
- WHEN se llama `POST /api/auth/signup`
- THEN responde 400 con código `VALIDATION_ERROR`

#### Scenario: Password débil

- GIVEN un signup con password bajo el mínimo exigido
- WHEN se llama `POST /api/auth/signup`
- THEN responde 400 con código `VALIDATION_ERROR`
