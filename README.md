# Telegram Group Manager

Plataforma para administrar grupos de Telegram: bot, backend en Go, panel
React + TypeScript y PostgreSQL.

> **Estado: MVP Fase 1 completo** (AGENTS.md §26-27): administración de
> grupos, membresía y moderación básica, solicitudes de ingreso, logs y
> autenticación del panel. Las fases 2-4 (publicaciones, moderación
> automática, automatizaciones) son posteriores y **no** están en esta
> versión.

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

Nota: el `:telegram_id` de las URLs es el ID de Telegram del grupo (ej.
`-100123456789`), no un id interno.

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