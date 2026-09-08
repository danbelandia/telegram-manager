# Delta for Frontend Auth

## MODIFIED Requirements

### Requirement: Formulario de login (REQ session-auth-delta)

`/login` MUST mostrar identificador y password con validación cliente y enviar `POST /api/auth/login`. Con `?username=` MUST pre-rellenar; con `created=1` MUST avisar "Cuenta creada, iniciá sesión".
(Previously: sin pre-rellenado.)

#### Scenario: Login exitoso

- GIVEN credenciales válidas
- WHEN se envía el formulario
- THEN guarda el token y redirige post-login

#### Scenario: Login fallido

- GIVEN credenciales inválidas
- WHEN se envía el formulario
- THEN muestra el error legible y NO hay token almacenado

#### Scenario: Campos vacios

- GIVEN el formulario vacío
- WHEN se intenta enviar
- THEN la validación bloquea el envío sin llamar a la API

#### Scenario: Username pre-rellenado

- GIVEN `/login?username=juan&created=1`
- WHEN se renderiza el formulario
- THEN el identificador trae `juan` y se ve el aviso

### Requirement: Restauracion de sesion (REQ session-auth-delta)

Con refresh válido, `GET /api/auth/me` MUST restaurar la sesión con `tenant_id` (`tenant_slug` MAY faltar).
(Previously: sin poblar tenant.)

#### Scenario: Sesion restaurada

- GIVEN refresh válido en cookie
- WHEN se recarga la página
- THEN vuelve con su `tenant_id` sin pedir credenciales

#### Scenario: Sin refresh token

- GIVEN sin cookie de refresh
- WHEN se recarga la página
- THEN sin sesión; las protegidas van a `/login`

## ADDED Requirements

### Requirement: Auto-registro público (REQ signup-happy)

`/signup` MUST validar en cliente (slug trim 1–63, username no vacío, password ≥8, token no vacío) sin llamar si falla. Ante 201 MUST auto-loguear e ir a `/dashboard`.

#### Scenario: Validación cliente bloquea

- GIVEN password de 5 caracteres
- WHEN se intenta enviar
- THEN se bloquea sin llamar a la API

#### Scenario: Signup y auto-login

- GIVEN datos válidos y signup → 201 `{tenant, admin}`
- WHEN se envía el formulario
- THEN auto-loguea y va a `/dashboard` con badge del tenant

### Requirement: Degradación del auto-login (REQ signup-degraded)

Tras el 201, si el login falla MUST ir a `/login?username={u}&created=1`.

#### Scenario: Auto-login fallido

- GIVEN signup 201 y login fallido para `juan`
- WHEN termina el flujo
- THEN va a `/login?username=juan&created=1` con el aviso

### Requirement: Conflictos de signup (REQ signup-409)

Ante 409 `CONFLICT`, el error MUST ir junto a slug si el mensaje menciona `slug`, junto a username si menciona `username`, o genérico si no menciona ninguno.

#### Scenario: Slug tomado

- GIVEN 409 que menciona `slug`
- WHEN se procesa el error
- THEN aparece junto al campo slug

#### Scenario: Username tomado o genérico

- GIVEN 409 que menciona `username` (o ninguno)
- WHEN se procesa el error
- THEN aparece junto a username (o como genérico)

### Requirement: Validación y Telegram (REQ signup-errors)

El 400 `VALIDATION_ERROR` MUST ir junto a su campo. El 502 `TELEGRAM_ERROR` MUST explicar el rechazo y guiar a @BotFather.

#### Scenario: Error por campo

- GIVEN 400 sobre `password`
- WHEN se procesa el error
- THEN aparece junto al campo password

#### Scenario: Token rechazado

- GIVEN 502 `TELEGRAM_ERROR`
- WHEN se procesa el error
- THEN se ve la guía de @BotFather con reintento

### Requirement: Badge de tenant (REQ badge)

El header MUST mostrar el slug conocido o `Tenant #id`; nunca vacío.

#### Scenario: Badge con slug

- GIVEN sesión post-signup con slug `acme`
- WHEN se renderiza el layout
- THEN el badge muestra `acme`

#### Scenario: Badge fallback

- GIVEN sesión solo con `tenant_id` 7
- WHEN se renderiza el layout
- THEN el badge muestra `Tenant #7`

### Requirement: Higiene del token (REQ hygiene)

El token MUST vivir solo en campo y variable local; nunca en contexto, storage, QueryClient, logs o tests (solo `SIGNUP_BOT_TOKEN_EXAMPLE`). MUST limpiar el campo al finalizar.

#### Scenario: Token invisible

- GIVEN un signup con `SIGNUP_BOT_TOKEN_EXAMPLE`
- WHEN se inspeccionan contexto, storage, QueryClient y logs
- THEN no hay rastros y el campo quedó vacío