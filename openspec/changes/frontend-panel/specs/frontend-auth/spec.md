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

## Notas

- El backend de auth ya está implementado (login/refresh/me/logout,
  archivado en `2026-09-06-auth`); este spec describe solo el consumo
  desde el panel.