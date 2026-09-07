# Telegram Group Manager

Plataforma para administrar grupos de Telegram: bot, backend en Go, panel
React + TypeScript y PostgreSQL.

> **Estado: MVP Fase 1 + Fase 2 slice 1-2-3** (AGENTS.md §26-27 + §22):
> administración de grupos, membresía y moderación básica, solicitudes
> de ingreso, logs, autenticación del panel y **publicaciones con
> foto por URL, botones inline de URL, envío multi-grupo,
> programación (`scheduled_at`), cancelación de filas `scheduled` e
> historial paginado**. Las fases 3 (moderación automática) y 4
> (automatizaciones) son posteriores y **no** están en esta versión.

## Quick path

```bash
# 1. Entorno desde la plantilla (NUNCA commitees .env)
cp .env.example .env

# 2. Completá .env con valores reales:
#    TELEGRAM_BOT_TOKEN, JWT_SECRET (min 32 caracteres),
#    ADMIN_USERNAME, ADMIN_PASSWORD
#    .env.example tiene la lista completa de variables comentada.

# 3. Levantar la stack (postgres + backend + frontend)
docker compose up -d

# 4. Verificar que el bot conectó
curl http://localhost:8080/api/health
#    Esperado: {"data":{"status":"ok","db":"ok","bot_connected":true,...}}

# 5. Abrir el panel y entrar
#    http://localhost:5173  → login con ADMIN_USERNAME / ADMIN_PASSWORD
```

Verificable: si el panel abre y el dashboard lista los grupos donde el
bot es administrador, la instalación está completa.

## Requisitos

- Docker Desktop
- Go 1.26+ y Node 20+ (solo desarrollo sin Docker)
- Un bot de Telegram creado con [@BotFather](https://t.me/BotFather)

## Variables de entorno

| Variable | Obligatoria | Descripción |
|----------|-------------|-------------|
| `TELEGRAM_BOT_TOKEN` | ✅ | Token del bot (via @BotFather) |
| `JWT_SECRET` | ✅ | Firma de tokens del panel (mínimo 32 caracteres) |
| `ADMIN_USERNAME` / `ADMIN_PASSWORD` | ✅ | Login inicial del panel (se crea solo si `admins` está vacía) |
| `TELEGRAM_MODE` | ❌ | `polling` (dev, default) o `webhook` (prod) |
| `TELEGRAM_WEBHOOK_URL` / `TELEGRAM_WEBHOOK_SECRET` | solo webhook | URL pública HTTPS y secret de validación |
| `COOKIE_SECURE` | ❌ | `true` solo en https; en local (`http://localhost`) debe ser `false` |
| `DATABASE_URL` | ❌ | DSN; con Docker Compose el host es `postgres` |
| `PORT` | ❌ | Puerto del backend (default `8080`) |
| `RUN_MIGRATIONS` | ❌ | Aplica migraciones al arrancar (default `true` en dev) |
| `PUBLICATIONS_SCHEDULER_INTERVAL_SECONDS` | ❌ | Intervalo del worker de publicaciones programadas en segundos (default `30`) |
| `VITE_API_BASE_URL` | ❌ | **Dejar vacía en desarrollo** (ver gotcha abajo) |

### Gotcha: `VITE_API_BASE_URL`

En desarrollo el proxy de Vite (`frontend/vite.config.ts`) resuelve `/api`
hacia el backend. Si seteas `VITE_API_BASE_URL=http://localhost:8080`, el
navegador llama cross-origin, el backend no responde el preflight de CORS
y el login falla con **HTTP 405**. Dejá la variable vacía o comentada.

## Migraciones

- En desarrollo se aplican automáticamente al arrancar el backend
  (`RUN_MIGRATIONS=true`), vía **goose** (`backend/migrations`).
- En producción se ejecutan como paso manual y documentado (no en el
  deploy).

## Uso del panel

| Ruta | Qué hace |
|------|----------|
| `/login` | Autenticación (JWT corto + refresh en cookie httpOnly) |
| `/dashboard` | Grupos donde el bot es administrador, con permisos |
| `/groups/:telegram_id` | Detalle: abrir/cerrar chat, eliminar/fijar mensaje |
| `/groups/:telegram_id/users` | Membresía y moderación (admins + lookup por ID, ban/mute) |
| `/groups/:telegram_id/requests` | Solicitudes de ingreso (aprobar/rechazar) |
| `/groups/:telegram_id/logs` | Auditoría de acciones administrativas |
| `/publications` | Publicar ahora con foto URL + botones inline, multi-grupo, filtro por grupo en historial |

Nota: el `:telegram_id` de las URLs es el ID de Telegram del grupo (ej.
`-100123456789`), no un id interno.

## Publicaciones

`POST /api/publications` permite crear y enviar una publicación
inmediata (sin scheduling). El panel solo publica; no recibe
`callback_data` ni responde a botones in-chat.

**Body** (slice 2):

```json
{
  "text": "¡Hola!",
  "photo_url": "https://ejemplo.com/imagen.jpg",
  "buttons": [[{ "text": "Ir", "url": "https://ejemplo.com" }]],
  "group_ids": [-100123, -100456]
}
```

- `text` (obligatorio). Si hay foto, ≤ 1024 caracteres (caption de
  `sendPhoto`); si no, ≤ 4096.
- `photo_url` (opcional). URL pública http/https, ≤ 2048 caracteres.
  La foto la descarga Telegram; si falla (URL inaccesible, > 5 MB,
  dimensiones inválidas) la fila queda `failed` con `error_message`
  legible — no se valida accesibilidad antes del POST.
- `buttons` (opcional). Inline keyboard URL-only, máximo 8 filas ×
  8 botones por fila. Cada botón: `text` ≤ 64, `url` http/https ≤
  256. El panel **no** soporta `callback_data` (es comportamiento
  bot-in-chat; queda para futuras automatizaciones).
- `group_ids` (obligatorio). Array de 1 a 10 `groups.telegram_id`.

**Envío multi-grupo**: las publicaciones se procesan **secuencialmente**
(§18.1: el token bucket del adapter ordena), nunca en paralelo. Si un
grupo falla (404/403/error de Telegram), el resto se procesa igual:
la respuesta es `201 Created` con un arreglo `data.publications` y cada
fila trae su propio `status` (`sent` | `failed`). Las filas `failed`
incluyen `error_message`.

**Listado**: `GET /api/publications` devuelve las 50 más recientes
(`created_at` DESC). Acepta `?group_id=<int64>` para filtrar por
grupo (usa el índice `idx_publications_telegram_id`).

**Permisos**: el bot debe ser administrador del grupo (`bot_status ==
administrator`). El check nunca consulta claves `can_*` (bugfix
previo). Si Telegram rechaza el envío por permisos, la fila queda
`failed` con `error_message` real (no se simula el permiso).

### Programación (slice 3)

`POST /api/publications` acepta un campo opcional `scheduled_at`
(RFC3339 con offset). Si está presente y es futuro, la fila se inserta
con `status='scheduled'` y NO se envía al momento de crear. Un worker
in-process (canónico `SELECT ... FOR UPDATE SKIP LOCKED` en una
transacción explícita) revisa cada 30 segundos y entrega las filas
cuya `scheduled_at` ya pasó al helper `publishOne` (mismo path que
publicación inmediata: permissionOk + SendMessage/SendPhoto +
UpdateStatus + log). El worker **no** reintenta filas `failed`.

**Importante**: Telegram **no** soporta scheduling nativo desde bots
(envío programado con `schedule_date` solo funciona en canales, no
desde bots a grupos); el panel programa in-process con el worker
descrito. El intervalo es configurable vía
`PUBLICATIONS_SCHEDULER_INTERVAL_SECONDS`.

**Cancelación** (`DELETE /api/publications/:id`): hard delete permitido
**únicamente** cuando `status='scheduled'`. Otros status (`sending`,
`sent`, `failed`) devuelven **409 `INVALID_STATUS`** para preservar
el audit trail — el admin puede ver el `error_message` real de una
publicación fallida en el historial.

### Historial paginado (slice 3)

`GET /api/publications` acepta:

- `?group_id=<int64>` (filtro por grupo, slice 2).
- `?limit=<int>` (default `50`, max `100`). Valores fuera de rango
  devuelven 400 `VALIDATION_ERROR`.
- `?offset=<int>` (default `0`). Valores negativos devuelven 400.

El panel muestra los controles **Anterior / Siguiente** debajo del
listado (Prev deshabilitado en `offset=0`; Next deshabilitado cuando
la página retornada tiene menos filas que `limit`).

## Limitaciones reales de la Bot API

El MVP no simula lo que Telegram no expone. Importante para operar:

| Expectativa común | Realidad |
|-------------------|----------|
| "Ver todos los miembros" | La Bot API **no lista miembros**. El panel muestra admins y permite buscar cualquier miembro por su ID numérico (`getChatMember`). |
| "Borrar/fijar un mensaje sin saber su id" | La Bot API **no lista mensajes**. Hay que ingresar manualmente el `message_id` del mensaje. |
| "Cerrar chat y que no pueda escribir nadie" | `Cerrar chat` aplica a miembros comunes; **los administradores siempre pueden escribir**. Para probarlo usá una cuenta no admin. |
| "Gestionar solicitudes de ingreso" | Requiere que Telegram entregue las solicitudes al bot (join requests activas y bot admin). |

Para detalles de cada método soportado: `docs/telegram_api_reference.md`.

## Tests

```bash
# Backend (Go): unit + servicios con TelegramService mockeado
cd backend && go test ./...

# Frontend (Vitest + Testing Library)
cd frontend && npm install && npm test -- --run
```

Los tests del backend **nunca** llaman a la Bot API real: la integración
con Telegram se mockea (`TelegramService`), como define AGENTS.md §21.1.

## Bot de Telegram

### Crear el bot y obtener el token

1. Abrí [@BotFather](https://t.me/BotFather) en Telegram y enviá
   `/newbot`.
2. Elegí un **nombre** (se muestra en el chat) y un **username** único
   (termina en `bot`, identifica al bot).
3. BotFather devuelve el token con la forma `123456789:AA...`. Guardalo
   en `.env`:

   ```env
   TELEGRAM_BOT_TOKEN=123456789:AA...
   ```

### Cómo valida el backend el token

El backend llama a `getMe` de la Bot API oficial
(https://core.telegram.org/bots/api#getme) al arrancar:

- **Token válido**: loguea `bot connected` con el `bot_username` y
  arranca normalmente.
- **Token inválido/revocado**: sale con código distinto de 0 y el
  mensaje `startup: telegram rejected the bot token`. Sin token config
  el compose falla rápido (`TELEGRAM_BOT_TOKEN:?`).

El token **nunca** aparece en logs ni en respuestas de la API.

### Recibir eventos: polling o webhook

El backend configura el modo con `TELEGRAM_MODE`:

- **`polling`** (default, desarrollo local): el bot hace long polling
  contra la Bot API y no necesita URL pública. Al arrancar se ve
  `telegram update` por cada evento recibido.
- **`webhook`** (producción): requiere `TELEGRAM_WEBHOOK_URL` (HTTPS
  pública) y `TELEGRAM_WEBHOOK_SECRET` (1-256 caracteres de
  `A-Za-z0-9_-`). Al arrancar registra el webhook con `setWebhook` y
  deja de usar polling. El backend valida el header
  `X-Telegram-Bot-Api-Secret-Token` en cada evento; sin el secret el
  backend **no arranca** (`config: TELEGRAM_WEBHOOK_SECRET is required`).

Los eventos se entregan a los consumidores en el orden de llegada,
respetando los rate limits de Telegram (429 → espera `retry_after` y
reintenta; el polling nunca avanza de offset si Telegram no confirma).

Para probar el webhook en desarrollo, exponé la URL con un túnel
(ngrok/cloudflared), registrala en `TELEGRAM_WEBHOOK_URL` y simulá un
evento real:

```bash
curl -X POST http://localhost:8080/api/telegram/webhook \
  -H "X-Telegram-Bot-Api-Secret-Token: TU_SECRET_AQUI" \
  -H "Content-Type: application/json" \
  -d '{"update_id":1,"message":{"message_id":1,"chat":{"id":-1001,"type":"supergroup"},"text":"hola"}}'
```

- Sin header o con secret incorrecto → `401` (no procesa).
- Con secret correcto → `200` y aparece `telegram update` en los logs
  del backend.
- Sin token real no hay verificación end-to-end: el registro real del
  webhook y el polling real necesitan `TELEGRAM_BOT_TOKEN` válido.

## Seguridad

- `.env` está en `.gitignore` — **nunca** commitees el token, el
  `JWT_SECRET` ni las credenciales admin.
- El token vive solo en el backend; el frontend no lo conoce (las
  variables `VITE_*` del bundle no pueden exponer nada que no lleve ese
  prefijo).
- Toda acción administrativa del panel queda registrada en la tabla
  `logs` (AGENTS.md §11) y es visible en `/groups/:id/logs`.
- Si el token se filtra o lo querés invalidar, usá `/revoke` con
  BotFather y generá uno nuevo con `/token`.

## Documentación técnica

- Especificación del MVP y arquitectura: [`AGENTS.md`](AGENTS.md)
- Referencia de la Bot API: [`docs/telegram_api_reference.md`](docs/telegram_api_reference.md)
- Guías de backend y frontend: [`docs/`](docs/)