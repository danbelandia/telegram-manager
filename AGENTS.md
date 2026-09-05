# Telegram Group Manager --- Especificación inicial del proyecto

## 1. Objetivo

Construir una plataforma para administrar uno o varios grupos de
Telegram mediante:

-   Un bot de Telegram que ejecute acciones administrativas.
-   Un backend en Go que concentre la lógica de negocio.
-   Un panel web en React + TypeScript.
-   PostgreSQL como base de datos.

El proyecto debe comenzar como un MVP de baja escala. **No utilizar
Redis inicialmente.** No introducir infraestructura adicional salvo que
exista una necesidad concreta y documentada.

La prioridad es construir una base limpia, mantenible y extensible para
posteriormente añadir publicaciones programadas, moderación automática y
un motor de automatizaciones.

------------------------------------------------------------------------

# 2. Stack tecnológico

## Backend

-   Go
-   API REST
-   Telegram Bot API
-   PostgreSQL
-   Docker
-   Webhooks de Telegram en producción
-   Long polling permitido para desarrollo local

## Frontend

-   React
-   TypeScript
-   Vite
-   React Router
-   Librería de componentes/UI a elección, priorizando simplicidad y
    mantenibilidad

## Infraestructura

Inicialmente:

``` text
frontend
backend
postgres
```

Todo debe poder ejecutarse mediante Docker Compose.

No agregar:

-   Redis
-   Kafka
-   RabbitMQ
-   Kubernetes
-   Microservicios

salvo que posteriormente exista una razón técnica concreta.

------------------------------------------------------------------------

# 3. Arquitectura

La arquitectura inicial debe ser un monolito modular:

``` text
                         Telegram
                            │
                       Webhook/API
                            │
                            ▼
                    ┌──────────────┐
                    │ Go Backend   │
                    │              │
                    │ REST API     │
                    │ Telegram     │
                    │ Groups       │
                    │ Users        │
                    │ Moderation   │
                    │ Publications │
                    │ Auth         │
                    │ Logs         │
                    └──────┬───────┘
                           │
                    ┌──────▼───────┐
                    │ PostgreSQL   │
                    └──────────────┘
                           ▲
                           │
                    ┌──────┴───────┐
                    │ React        │
                    │ Dashboard    │
                    └──────────────┘
```

No crear microservicios para el MVP.

------------------------------------------------------------------------

# 4. Principios importantes

## Telegram es la fuente de verdad para acciones de Telegram

No asumir que cualquier acción administrativa deseada es posible
mediante la Bot API.

Antes de implementar una funcionalidad, comprobar:

1.  Que Telegram Bot API permita realizarla.
2.  Qué permisos requiere el bot.
3.  Qué restricciones existen.
4.  Qué errores puede devolver Telegram.

Si una función solicitada no es posible directamente mediante la Bot
API, documentarla y no intentar simularla de forma insegura.

Para consultar información sobre la API lee el archivo llamado "telegram_api_reference" ubicado en: TelegramManager\docs

## Seguridad

El bot tendrá permisos administrativos dentro de los grupos, por lo que
el sistema debe tratar estas acciones como sensibles.

Implementar:

-   Autenticación del panel.
-   Autorización por usuario.
-   Control de permisos.
-   Logs de acciones administrativas.
-   Confirmación para acciones destructivas importantes.
-   Validación de todos los parámetros recibidos.
-   Nunca almacenar el token del bot en código fuente.
-   Variables de entorno para secretos.
-   No exponer el token de Telegram al frontend.

------------------------------------------------------------------------

# 5. MVP --- Fase 1

La primera versión debe concentrarse exclusivamente en la administración
básica de grupos.

## 5.1 Registro/conexión del bot

El sistema debe permitir configurar el bot mediante variable de entorno:

``` env
TELEGRAM_BOT_TOKEN=
```

El backend debe validar que el token funciona.

No permitir que el token aparezca en respuestas de la API ni en logs.

------------------------------------------------------------------------

# 6. Gestión de grupos

El sistema debe poder identificar los grupos donde el bot participa y
tiene permisos administrativos suficientes.

Crear una sección:

``` text
Grupos
```

Cada grupo debe mostrar:

-   Nombre
-   Username si existe
-   ID de Telegram
-   Tipo
-   Cantidad de miembros cuando Telegram permita obtenerla
-   Estado del bot
-   Permisos disponibles

Ejemplo:

``` text
MU Online Comunidad
ID: -100123456789
Miembros: 4.821
Bot: Administrador

[Administrar]
```

------------------------------------------------------------------------

# 7. Gestión de usuarios

Dentro de un grupo:

``` text
Usuarios
```

Funciones iniciales:

-   Buscar usuario.
-   Ver información básica.
-   Banear.
-   Desbanear.
-   Mutear/restringir.
-   Expulsar cuando corresponda.
-   Ver advertencias.
-   Registrar cada acción.

No asumir que Telegram permite obtener una lista completa de todos los
miembros de un grupo mediante la Bot API. Diseñar la funcionalidad
alrededor de los usuarios/eventos que Telegram realmente exponga.

------------------------------------------------------------------------

# 8. Moderación de mensajes

Funciones iniciales:

-   Eliminar mensajes.
-   Detectar mensajes recibidos por el bot cuando corresponda.
-   Registrar acciones de moderación.
-   Fijar mensajes cuando el bot tenga permisos suficientes.

No implementar todavía un sistema complejo de análisis de mensajes.

------------------------------------------------------------------------

# 9. Abrir/cerrar envío de mensajes

Implementar una función para restringir el envío de mensajes en el grupo
cuando Telegram lo permita.

Ejemplo:

``` text
[🔓 Abrir chat]
[🔒 Cerrar chat]
```

La acción debe:

1.  Verificar permisos del bot.
2.  Ejecutar la modificación mediante Telegram.
3.  Registrar la acción.
4.  Mostrar resultado al usuario.

------------------------------------------------------------------------

# 10. Solicitudes de ingreso

Implementar soporte para solicitudes de ingreso cuando Telegram las
entregue al bot.

Panel:

``` text
Solicitudes de ingreso

Usuario       Fecha             Estado
Juan          05/09/2026       Pendiente
Pedro         05/09/2026       Pendiente

[Aceptar] [Rechazar]
```

El backend debe procesar los eventos de solicitudes de ingreso.

Registrar:

-   Usuario
-   Grupo
-   Fecha
-   Estado
-   Administrador que tomó la decisión
-   Fecha de decisión

------------------------------------------------------------------------

# 11. Logs

Todas las acciones administrativas importantes deben generar un
registro.

Ejemplo:

``` text
2026-09-05 15:20
Administrador: usuario_123
Grupo: MU Online Comunidad
Acción: BAN_USER
Usuario afectado: 456789
Resultado: SUCCESS
```

Tabla conceptual:

``` text
logs
- id
- actor_id
- group_id
- action
- target_user_id
- metadata
- status
- error_message
- created_at
```

No almacenar datos innecesarios.

------------------------------------------------------------------------

# 12. API REST

Diseñar endpoints coherentes.

Ejemplo:

``` text
GET    /api/groups
GET    /api/groups/:id
GET    /api/groups/:id/users
GET    /api/groups/:id/join-requests
GET    /api/groups/:id/logs

POST   /api/groups/:id/users/:userId/ban
POST   /api/groups/:id/users/:userId/unban
POST   /api/groups/:id/users/:userId/mute
POST   /api/groups/:id/users/:userId/unmute

POST   /api/groups/:id/messages/:messageId/delete
POST   /api/groups/:id/messages/:messageId/pin

POST   /api/groups/:id/lock
POST   /api/groups/:id/unlock

POST   /api/groups/:id/join-requests/:requestId/approve
POST   /api/groups/:id/join-requests/:requestId/reject
```

Los nombres pueden modificarse si existe una convención mejor, pero
mantener una estructura REST consistente.

------------------------------------------------------------------------

# 13. Base de datos

Utilizar PostgreSQL.

Entidades iniciales:

``` text
users
groups
group_members
admins
join_requests
warnings
logs
```

Posteriormente:

``` text
publications
scheduled_publications
automations
automation_actions
```

No crear tablas de funcionalidades que todavía no existan.

La tabla `admins` debe incluir, además de los campos de negocio propios,
los campos necesarios para autenticación (ver 17.1): `password_hash`,
`created_at`, `last_login_at`.

## 13.1 Herramienta de migraciones

-   Usar **goose** (`github.com/pressly/goose`) para gestionar
    migraciones.
-   Migraciones en SQL plano (sin DSL propio), ubicadas en
    `/migrations`, tal como se define en la estructura del backend
    (sección 14).
-   En desarrollo, ejecutar las migraciones automáticamente al iniciar
    el backend.
-   En producción, ejecutarlas como paso manual y documentado (no
    automático al deploy).
-   No introducir un ORM completo para el MVP. Si más adelante se
    quiere tipado fuerte sobre las consultas, evaluar `sqlc` como
    generador de código a partir del SQL ya escrito con goose — no
    reemplaza a goose, lo complementa. No introducirlo salvo necesidad
    concreta, siguiendo el mismo criterio de la sección 2.

------------------------------------------------------------------------

# 14. Estructura recomendada del backend

Usar una estructura modular, no un único archivo gigante.

Ejemplo:

``` text
backend/
├── cmd/
│   └── server/
│       └── main.go
│
├── internal/
│   ├── auth/
│   ├── telegram/
│   ├── groups/
│   ├── users/
│   ├── moderation/
│   ├── joinrequests/
│   ├── logs/
│   └── database/
│
├── migrations/
├── config/
├── Dockerfile
├── go.mod
└── go.sum
```

Separar:

-   Handlers HTTP
-   Servicios
-   Repositorios
-   Modelos
-   Integración con Telegram

No colocar lógica de negocio directamente dentro de los handlers.

------------------------------------------------------------------------

# 15. Telegram Adapter

Crear una capa dedicada para Telegram.

Ejemplo conceptual:

``` go
type TelegramService interface {
    BanUser(...)
    UnbanUser(...)
    MuteUser(...)
    UnmuteUser(...)
    DeleteMessage(...)
    PinMessage(...)
    LockGroup(...)
    UnlockGroup(...)
    ApproveJoinRequest(...)
    RejectJoinRequest(...)
}
```

El resto de la aplicación no debería depender directamente de llamadas
HTTP dispersas hacia Telegram.

Esto facilitará:

-   Testing (ver 21.1).
-   Manejo de errores.
-   Cambios de librería.
-   Mantenimiento.

La implementación concreta de `TelegramService` es también el lugar
correcto para aplicar el control de rate limits (ver 18.1): el resto
del backend no debe preocuparse de límites de la Bot API, solo de
invocar la interfaz.

------------------------------------------------------------------------

# 16. Frontend

Crear un dashboard sencillo.

Rutas iniciales:

``` text
/login
/dashboard
/groups
/groups/:id
/groups/:id/users
/groups/:id/requests
/groups/:id/logs
```

Dashboard inicial:

``` text
┌──────────────────────────────────────┐
│ Telegram Manager                     │
├─────────────┬────────────────────────┤
│ Dashboard   │ Grupos                 │
│ Grupos      │                        │
│ Logs        │  🎮 MU Online          │
│ Configuración│  4.821 miembros       │
│             │  [Administrar]         │
│             │                        │
│             │  💻 Programadores      │
│             │  1.203 miembros       │
│             │  [Administrar]         │
└─────────────┴────────────────────────┘
```

Priorizar funcionalidad sobre diseño visual avanzado.

------------------------------------------------------------------------

# 17. Autenticación

Implementar autenticación básica para el panel.

Nunca confiar en que una petición al frontend implica que el usuario
tiene permisos administrativos en Telegram.

Antes de ejecutar una acción:

``` text
Usuario autenticado
        ↓
Permiso dentro de nuestra aplicación
        ↓
Bot tiene permisos suficientes en Telegram
        ↓
Ejecutar acción
        ↓
Registrar resultado
```

## 17.1 Estrategia de autenticación

-   JWT con **access token corto (15 min)** + **refresh token largo (7
    días)**. No usar sesiones en base de datos para el MVP.
-   El refresh token se guarda en cookie **httpOnly, Secure,
    SameSite=Strict**. El access token nunca se guarda en
    localStorage (riesgo de XSS).
-   Passwords con `bcrypt` (`golang.org/x/crypto/bcrypt`), costo 12.
-   Login con `email/username + password` sobre la tabla `admins`
    (sección 13), que debe incluir `password_hash`, `created_at` y
    `last_login_at`.

------------------------------------------------------------------------

# 18. Manejo de errores

Telegram puede rechazar acciones.

El backend debe diferenciar:

``` text
SUCCESS
PERMISSION_DENIED
TELEGRAM_ERROR
VALIDATION_ERROR
NOT_FOUND
INTERNAL_ERROR
```

No devolver errores internos o tokens al frontend.

Mostrar mensajes comprensibles:

``` text
No se pudo banear al usuario porque el bot
no tiene permisos suficientes en este grupo.
```

## 18.1 Manejo de rate limits de Telegram

La Bot API impone límites no oficiales (~30 req/seg globales, ~1
mensaje/seg por chat) y puede responder `429 Too Many Requests` con un
campo `retry_after`.

-   Implementar un **rate limiter tipo token bucket** dentro del
    `TelegramService` (sección 15), no en capas superiores.
-   Límite sugerido: ~25 req/seg global para dejar margen de
    seguridad.
-   Ante un `429`, leer `retry_after` y reintentar respetando ese
    tiempo, con un máximo de 3 reintentos. No reintentar a ciegas.
-   Acciones en lote (por ejemplo, aceptar múltiples solicitudes de
    ingreso a la vez) deben encolarse y procesarse secuencialmente,
    nunca en paralelo sin control.
-   Esto se resuelve con un canal interno de Go y un worker; no
    requiere Redis ni una cola externa, consistente con la sección 2.

------------------------------------------------------------------------

# 19. Webhooks

Diseñar el backend para soportar Telegram Webhooks.

Endpoint conceptual:

``` text
POST /api/telegram/webhook
```

Debe procesar eventos relevantes como:

-   Nuevos mensajes.
-   Cambios de miembros.
-   Solicitudes de ingreso.
-   Cambios relacionados con el bot.

## 19.1 Selección de modo (webhook vs. polling)

Definir el modo mediante variable de entorno explícita, sin
autodetección:

``` env
TELEGRAM_MODE=webhook | polling
```

-   `polling`: modo por defecto en desarrollo local (no requiere URL
    pública).
-   `webhook`: modo de producción, requiere `TELEGRAM_WEBHOOK_URL` y
    `TELEGRAM_WEBHOOK_SECRET`.
-   El backend lee `TELEGRAM_MODE` al iniciar y configura el listener
    correspondiente.
-   En modo webhook, validar el header
    `X-Telegram-Bot-Api-Secret-Token` contra `TELEGRAM_WEBHOOK_SECRET`
    antes de procesar cualquier evento entrante. Sin esta validación,
    cualquiera que descubra la URL del endpoint podría enviar eventos
    falsos.

------------------------------------------------------------------------

# 20. Docker

Crear:

``` text
docker-compose.yml
```

Servicios:

``` text
backend
frontend
postgres
```

Variables mediante `.env`.

Crear también:

``` text
.env.example
```

Nunca subir `.env` real a Git.

------------------------------------------------------------------------

# 21. Testing

Como mínimo:

### Backend

Unit tests para:

-   Servicios de usuarios.
-   Moderación.
-   Validaciones.
-   Permisos.
-   Procesamiento de eventos de Telegram.

Tests de integración para:

-   PostgreSQL.
-   Endpoints principales.

No es necesario buscar una cobertura artificialmente alta. Priorizar
lógica crítica.

## 21.1 Estrategia de testing para la integración con Telegram

-   Como `TelegramService` es una interfaz (sección 15), los tests de
    servicios de negocio (ban, mute, moderación, etc.) deben usar un
    **mock** de esa interfaz, generado con `moq`
    (`github.com/matryer/moq`) o implementado a mano.
-   Los tests automatizados **nunca** deben hacer llamadas reales a la
    Bot API.
-   Tests de integración contra la Bot API real (validando que el
    adapter concreto funciona) son opcionales, se corren aparte y no
    forman parte del pipeline de CI por defecto.

------------------------------------------------------------------------

# 22. Fase 2 --- Publicaciones

``` text
Crear publicación
       ↓
Seleccionar grupos
       ↓
Publicar ahora
       o
Programar
```

Funciones futuras:

-   Texto.
-   Imagen.
-   Botones.
-   Publicación inmediata.
-   Publicación programada.
-   Publicación en múltiples grupos.
-   Historial.

------------------------------------------------------------------------

# 23. Fase 3 --- Moderación automática

Posteriormente:

``` text
Anti-spam
Anti-link
Palabras prohibidas
Flood detection
Warnings
Auto-mute
Auto-ban
```

Ejemplo:

``` text
5 mensajes en 10 segundos
          ↓
       Flood
          ↓
     Warning #1
          ↓
     Mute 10 min
```

------------------------------------------------------------------------

# 24. Fase 4 --- Motor de automatizaciones

La visión futura del proyecto es permitir:

``` text
TRIGGER
   ↓
CONDITION
   ↓
ACTION
```

Ejemplo:

``` text
Usuario entra
      ↓
Enviar bienvenida
      ↓
Esperar
      ↓
Enviar reglas
```

Otro ejemplo:

``` text
Usuario publica enlace
       ↓
¿Usuario autorizado?
       ↓
NO
       ↓
Eliminar mensaje
       ↓
Warning
```

Este sistema debe diseñarse como módulo independiente para no contaminar
el MVP.

------------------------------------------------------------------------

# 25. Reglas para el agente de desarrollo

1.  No implementar funcionalidades que Telegram Bot API no soporte.
2.  No inventar endpoints de Telegram.
3.  Consultar la documentación oficial de Telegram cuando exista duda
    sobre una capacidad.
4.  No introducir Redis en esta fase.
5.  No introducir microservicios.
6.  Mantener el backend modular.
7.  No colocar secretos en el código.
8.  Registrar las acciones administrativas importantes.
9.  Validar permisos antes de acciones destructivas.
10. Priorizar código simple y mantenible.
11. No construir funcionalidades futuras antes de terminar el MVP.
12. Crear migraciones de base de datos con goose (ver 13.1).
13. Mantener `.env.example` actualizado.
14. Documentar decisiones técnicas importantes.
15. Cada funcionalidad debe incluir manejo de errores.
16. Evitar dependencias innecesarias.
17. No asumir que Telegram permite obtener información que la Bot API no
    expone.
18. El frontend nunca debe comunicarse directamente con Telegram usando
    el token del bot.
19. Respetar los límites de rate limit de Telegram implementando el
    control descrito en 18.1; no reintentar llamadas fallidas sin
    control.
20. Los tests de lógica de negocio deben mockear `TelegramService`
    (ver 21.1), nunca llamar a la Bot API real.

------------------------------------------------------------------------

# 26. Orden de implementación

Seguir este orden:

``` text
1. Inicializar repositorio
        ↓
2. Docker Compose
        ↓
3. PostgreSQL
        ↓
4. Configuración Go
        ↓
5. Integración Telegram
        ↓
6. Webhook/polling
        ↓
7. Modelo Group
        ↓
8. Detección/registro de grupos
        ↓
9. Autenticación
        ↓
10. API REST
        ↓
11. React + routing
        ↓
12. Dashboard
        ↓
13. Gestión de usuarios
        ↓
14. Ban / Unban / Mute
        ↓
15. Borrar/fijar mensajes
        ↓
16. Abrir/cerrar grupo
        ↓
17. Solicitudes de ingreso
        ↓
18. Logs
        ↓
19. Tests
        ↓
20. Documentación
```

No avanzar a publicaciones programadas hasta que esta primera fase esté
funcional.

------------------------------------------------------------------------

# 27. Definition of Done del MVP

El MVP estará terminado cuando:

-   [ ] El proyecto se levante con Docker Compose.
-   [ ] PostgreSQL funcione correctamente, con migraciones aplicadas
    vía goose.
-   [ ] El bot pueda conectarse a Telegram.
-   [ ] El backend pueda recibir eventos de Telegram (webhook o
    polling según `TELEGRAM_MODE`).
-   [ ] El panel React permita autenticarse (JWT + refresh token).
-   [ ] Se puedan visualizar los grupos administrables.
-   [ ] Se pueda acceder al detalle de un grupo.
-   [ ] Se puedan ejecutar acciones de moderación soportadas por
    Telegram, respetando el rate limiter del adapter.
-   [ ] Se puedan gestionar solicitudes de ingreso.
-   [ ] Se puedan abrir/cerrar restricciones de envío cuando
    corresponda.
-   [ ] Las acciones administrativas queden registradas.
-   [ ] Existan tests para la lógica crítica, con `TelegramService`
    mockeado.
-   [ ] Exista documentación para ejecutar el proyecto localmente.
-   [ ] Exista `.env.example`.
-   [ ] No existan secretos dentro del repositorio.

------------------------------------------------------------------------

# 28. Primer objetivo del agente

No intentes construir todo el proyecto de una vez.

Primero:

1.  Crear la estructura del repositorio.
2.  Configurar Go.
3.  Configurar React + TypeScript + Vite.
4.  Configurar PostgreSQL.
5.  Crear Docker Compose.
6.  Configurar variables de entorno.
7.  Implementar una conexión funcional con Telegram.
8.  Implementar un endpoint de health check.
9.  Crear la primera migración con goose.
10. Crear una integración mínima que permita comprobar que el bot está
    conectado correctamente.

Después de completar esta base, continuar módulo por módulo siguiendo el
orden definido anteriormente.

La prioridad es **tener un sistema pequeño, funcional y entendible antes
de agregar complejidad**.
