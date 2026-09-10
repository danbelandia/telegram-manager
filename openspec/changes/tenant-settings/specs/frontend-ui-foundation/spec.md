# Delta for Frontend UI Foundation

## MODIFIED Requirements

### Requirement: AppShell como layout autenticado

La navbar del AppShell MUST renderizar un `<NavLink>` adicional para
`/tenant` con icono de configuración (ej. `Settings` de Tabler). El
link MUST mostrarse después de los links existentes (Groups,
Publications). (Previously: la navbar no incluía link a `/tenant`.)

#### Scenario: NavLink de Tenant visible

- GIVEN un admin autenticado en cualquier ruta
- WHEN se renderiza la navbar del AppShell
- THEN se ve unNavLink "Configuración" o "Tenant" enlazado a `/tenant`

#### Scenario: NavLink activo

- GIVEN un admin en `/tenant`
- WHEN se inspecciona la navbar
- THEN el NavLink de `/tenant` está en estado activo/highlighted

## ADDED Requirements

### Requirement: DegradedBanner

Cuando `bot_status` es `disconnected` o `unknown`, el sistema MUST
mostrar un banner persistente en la parte superior del layout (debajo
 del header, encima del contenido). El banner MUST usar colores de
alerta (amarillo/rojo) y texto descriptivo en es-AR. El banner MUST
desaparecer cuando `bot_status` vuelve a `connected`.

#### Scenario: Banner visible cuando bot caído

- GIVEN un admin autenticado y `bot_status = "disconnected"`
- WHEN se renderiza cualquier ruta autenticada
- THEN se ve el banner con mensaje de advertencia

#### Scenario: Banner oculto cuando bot conectado

- GIVEN un admin autenticado y `bot_status = "connected"`
- WHEN se renderiza cualquier ruta autenticada
- THEN NO se ve ningún banner

#### Scenario: Polling actualiza banner

- GIVEN un admin con `bot_status = "disconnected"` y banner visible
- WHEN el polling (60s) detecta que el bot volvió a `connected`
- THEN el banner desaparece sin recarga de página

### Requirement: TenantSettingsPage

La ruta `/tenant` MUST renderizar una página con:
1. Datos del tenant (slug, bot_username, bot_status, created_at) en
   modo solo lectura.
2. Formulario de rotación de token con campos `password` y
   `bot_token`, botón de envío, y manejo de errores (401, 400, 502).
3. Feedback visual: notificación de éxito (`notifySuccess`) o error
   (`notifyError`) según la respuesta.

#### Scenario: Página carga datos del tenant

- GIVEN un admin autenticado
- WHEN se navega a `/tenant`
- THEN se muestran slug, bot_username, bot_status y created_at

#### Scenario: Rotación exitosa notifica

- GIVEN un admin en `/tenant` con password correcta y token válido
- WHEN envía el formulario de rotación
- THEN aparece notificación de éxito y el bot_status se actualiza

#### Scenario: Rotación fallida por password

- GIVEN un admin en `/tenant` con password incorrecta
- WHEN envía el formulario de rotación
- THEN aparece notificación de error con mensaje §18

#### Scenario: Rotación fallida por token inválido

- GIVEN un admin en `/tenant` con password correcta pero token inválido
- WHEN envía el formulario de rotación
- THEN aparece notificación de error indicando que el token es inválido
