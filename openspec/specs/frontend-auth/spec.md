# Frontend Auth Specification

## Purpose

Autenticación del panel en el navegador (AGENTS §17/17.1): login contra
el backend, acceso token en memoria (nunca localStorage), refresh token
en cookie httpOnly gestionada por el navegador, restauración de sesión
y logout. El frontend nunca ve ni manipula el refresh token.

## Requirements

### Requirement: Formulario de login

La página `/login` MUST mostrar un formulario con campo identificador
(email/username) y password, con validación cliente (campos no vacíos)
usando React Hook Form + Zod, y MUST enviar `POST /api/auth/login`.

#### Scenario: Login exitoso

- GIVEN credenciales válidas
- WHEN se envía el formulario
- THEN se almacena el access token en memoria y se redirige según
  redirect post-login (spec frontend-routing)

#### Scenario: Login fallido

- GIVEN credenciales inválidas
- WHEN se envía el formulario
- THEN se muestra el mensaje de error legible del backend (§18) y NO
  hay token almacenado

#### Scenario: Campos vacios

- GIVEN el formulario vacío
- WHEN se intenta enviar
- THEN la validación cliente bloquea el envío sin llamar a la API

### Requirement: Sesion en memoria con useAuth

El access token MUST vivir en estado de la aplicación (Context
`AuthProvider` + hook `useAuth()`), nunca en localStorage ni en
variables globales accesibles por XSS. El refresh token es cookie
httpOnly manejada por el navegador; el código cliente MUST NOT leerlo
ni escribirlo.

#### Scenario: Token en memoria

- GIVEN un login exitoso
- WHEN se inspecciona localStorage
- THEN no hay token ni datos de sesión persistidos

### Requirement: Restauracion de sesion

Al cargar la app, con refresh token válido (cookie), `GET
/api/auth/me` MUST restaurar la sesión automáticamente sin pedir
credenciales de nuevo.

#### Scenario: Sesion restaurada

- GIVEN un refresh token válido en cookie
- WHEN se recarga la página
- THEN la app restaura la sesión y permite entrar a rutas protegidas

#### Scenario: Sin refresh token

- GIVEN sin cookie de refresh (o expirada)
- WHEN se recarga la página
- THEN la app queda sin sesión y las rutas protegidas redirigen a
  `/login`

### Requirement: Refresh transparente

El cliente HTTP MUST reintentar una petición que falle con 401 una
sola vez, tras refrescar el access token con el refresh token de la
cookie. Si el refresh falla, MUST cerrar la sesión.

#### Scenario: Access token expirado

- GIVEN un access token expirado y refresh token válido
- WHEN se llama a una API protegida
- THEN el cliente refresca automáticamente y reejecuta la petición con
  el nuevo access token, sin intervención del usuario

#### Scenario: Refresh fallido

- GIVEN access token expirado y refresh token inválido/expirado
- WHEN se llama a una API protegida
- THEN la sesión se cierra y el usuario vuelve a `/login`

### Requirement: Logout

El dashboard MUST exponer un botón de logout que llama a la API de
logout (invalida el refresh token en servidor), limpia el estado en
memoria y redirige a `/login`.

#### Scenario: Logout

- GIVEN un usuario autenticado
- WHEN hace logout
- THEN se llama a la API, se limpia el estado y se redirige a `/login`

---

## Slice 1 — public-signup-landing (2026-09-09)

### MODIFIED Requirements

#### Requirement: Formulario de login (REQ session-auth-delta)

`/login` MUST mostrar identificador y password con validación cliente
y enviar `POST /api/auth/login`. Con `?username=` MUST pre-rellenar;
con `created=1` MUST avisar "Cuenta creada, iniciá sesión".

##### Scenario: Login exitoso

- GIVEN credenciales válidas
- WHEN se envía el formulario
- THEN guarda el token y redirige post-login

##### Scenario: Login fallido

- GIVEN credenciales inválidas
- WHEN se envía el formulario
- THEN muestra el error legible y NO hay token almacenado

##### Scenario: Campos vacios

- GIVEN el formulario vacío
- WHEN se intenta enviar
- THEN la validación bloquea el envío sin llamar a la API

##### Scenario: Username pre-rellenado

- GIVEN `/login?username=juan&created=1`
- WHEN se renderiza el formulario
- THEN el identificador trae `juan` y se ve el aviso

#### Requirement: Restauracion de sesion (REQ session-auth-delta)

Con refresh válido, `GET /api/auth/me` MUST restaurar la sesión con
`tenant_id` (`tenant_slug` MAY faltar).

##### Scenario: Sesion restaurada

- GIVEN refresh válido en cookie
- WHEN se recarga la página
- THEN vuelve con su `tenant_id` sin pedir credenciales

##### Scenario: Sin refresh token

- GIVEN sin cookie de refresh
- WHEN se recarga la página
- THEN sin sesión; las protegidas van a `/login`

### ADDED Requirements

#### Requirement: Auto-registro público (REQ signup-happy)

`/signup` MUST validar en cliente (slug trim 1–63, username no vacío,
password ≥8, token no vacío) sin llamar si falla. Ante 201 MUST
auto-loguear e ir a `/dashboard`.

##### Scenario: Validación cliente bloquea

- GIVEN password de 5 caracteres
- WHEN se intenta enviar
- THEN se bloquea sin llamar a la API

##### Scenario: Signup y auto-login

- GIVEN datos válidos y signup → 201 `{tenant, admin}`
- WHEN se envía el formulario
- THEN auto-loguea y va a `/dashboard` con badge del tenant

#### Requirement: Degradación del auto-login (REQ signup-degraded)

Tras el 201, si el login falla MUST ir a `/login?username={u}&created=1`.

##### Scenario: Auto-login fallido

- GIVEN signup 201 y login fallido para `juan`
- WHEN termina el flujo
- THEN va a `/login?username=juan&created=1` con el aviso

#### Requirement: Conflictos de signup (REQ signup-409)

Ante 409 `CONFLICT`, el error MUST ir junto a slug si el mensaje
menciona `slug`, junto a username si menciona `username`, o genérico
si no menciona ninguno.

##### Scenario: Slug tomado

- GIVEN 409 que menciona `slug`
- WHEN se procesa el error
- THEN aparece junto al campo slug

##### Scenario: Username tomado o genérico

- GIVEN 409 que menciona `username` (o ninguno)
- WHEN se procesa el error
- THEN aparece junto a username (o como genérico)

#### Requirement: Validación y Telegram (REQ signup-errors)

El 400 `VALIDATION_ERROR` MUST ir junto a su campo. El 502
`TELEGRAM_ERROR` MUST explicar el rechazo y guiar a @BotFather.

##### Scenario: Error por campo

- GIVEN 400 sobre `password`
- WHEN se procesa el error
- THEN aparece junto al campo password

##### Scenario: Token rechazado

- GIVEN 502 `TELEGRAM_ERROR`
- WHEN se procesa el error
- THEN se ve la guía de @BotFather con reintento

#### Requirement: Badge de tenant (REQ badge)

El header MUST mostrar el slug conocido o `Tenant #id`; nunca vacío.

##### Scenario: Badge con slug

- GIVEN sesión post-signup con slug `acme`
- WHEN se renderiza el layout
- THEN el badge muestra `acme`

##### Scenario: Badge fallback

- GIVEN sesión solo con `tenant_id` 7
- WHEN se renderiza el layout
- THEN el badge muestra `Tenant #7`

#### Requirement: Higiene del token (REQ hygiene)

El token MUST vivir solo en campo y variable local; nunca en contexto,
storage, QueryClient, logs o tests (solo `SIGNUP_BOT_TOKEN_EXAMPLE`).
MUST limpiar el campo al finalizar.

##### Scenario: Token invisible

- GIVEN un signup con `SIGNUP_BOT_TOKEN_EXAMPLE`
- WHEN se inspeccionan contexto, storage, QueryClient y logs
- THEN no hay rastros y el campo quedó vacío

## Notas

- El backend de auth ya está implementado (login/refresh/me/logout,
  archivado en `2026-09-06-auth`); este spec describe solo el consumo
  desde el panel.