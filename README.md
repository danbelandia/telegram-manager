# Telegram Group Manager

Plataforma para administrar grupos de Telegram: bot, backend en Go, panel
React + TypeScript y PostgreSQL.

> **Estado: MVP Fase 1 + Fase 2 slice 1-2-3 + Fase 3 slice 1-2-2.1-3**
> (AGENTS.md §22-23): administración de grupos, membresía y moderación
> básica, solicitudes de ingreso, logs, autenticación del panel,
> publicaciones con foto por URL + botones inline + envío multi-grupo +
> programación (`scheduled_at`) + cancelación + historial paginado, y
> **moderación automática** (Fase 3: Flood + auto-mute/ban + audit +
> anti-spam/anti-link/banned-words + editor de settings + warning
> visual al usuario + **dashboard de warnings** con stats 24h/7d y
> reset manual). Las fases 4 (automatizaciones trigger/condition/
> action) son posteriores y **no** están en esta versión.

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
| `/groups/:telegram_id/automation` | Editor de moderación automática: toggles (Flood / Anti-spam / Anti-link / Banned-words), umbrales, listas de palabras prohibidas y allowlist de enlaces |
| `/groups/:telegram_id/moderation` | Dashboard de moderación: estadísticas 24h/7d (reglas disparadas, auto-mute, auto-ban) + advertencias activas con display name + botón Reset manual |

Nota: el `:telegram_id` de las URLs es el ID de Telegram del grupo (ej.
`-100123456789`), no un id interno.

## UI library & Dark mode

El panel usa **[Mantine v7](https://mantine.dev/)** como librería de
componentes (`@mantine/core`, `@mantine/hooks`, `@mantine/notifications`)
e iconos [`@tabler/icons-react`](https://tabler.io/icons-react). Toda la
UI autenticada vive dentro de un `<AppShell>` con header (logo + toggle
de tema + logout) y navbar con las rutas globales.

**Dark mode**: el header expone un toggle (icono sol/luna) que invoca
`useMantineColorScheme().toggleColorScheme()`. El esquema se persiste
en `localStorage` bajo la clave `mantine-color-scheme-value` (provista
por `localStorageColorSchemeManager`); el default es `auto` (respeta
la preferencia del SO) y el cambio se aplica antes del primer paint
para evitar flicker.

**Notificaciones**: el `<Notifications>` se monta una sola vez en
`main.tsx` con `position="top-right"`, `zIndex=2077` y `limit=5`. Los
helpers `notifySuccess` / `notifyError` viven en
`frontend/src/lib/notifications.ts` y centralizan el formato
(`color: 'green' | 'red'`). El wiring actual cubre Login (success →
"Bienvenido") y Logout ("Sesión cerrada"); el wiring al resto de los
hooks (~15) se difiere a un slice siguiente.

> **No** tocar `features/*`, `lib/api-client.ts` ni `lib/auth-context.tsx`
> en cambios de UI: la fundación UI es solo view layer.

## Cambios de dependencias en el frontend

El contenedor `frontend` ahora bind-monta **solo** `./frontend/src` y
ejecuta `npm install` en cada arranque (`CMD ["sh", "-c", "npm install
&& npm run dev -- --host 0.0.0.0"]`):

- Cambios en archivos dentro de `frontend/src/` → hot reload automático
  vía Vite watch, sin rebuild.
- Cambios en `package.json` / `package-lock.json` → requieren
  `docker compose build frontend` para que el `RUN npm install` de la
  imagen incorpore la nueva dep antes del CMD del contenedor.
- Cambios en `vite.config.ts`, `postcss.config.cjs` o `frontend/Dockerfile`
  → también requieren `docker compose build frontend`.
- Primer arranque tarda 10-30s extra por el `npm install`; arranques
  subsecuentes usan la cache interna de npm.

`frontend/.dockerignore` excluye `node_modules`, `dist`, `.env`,
`.env.local`, `.git`, `.vite`, `coverage`, `*.log`, `.vscode`, `.idea`
y `.DS_Store` para que el contexto de build no contamine la imagen.

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

### Publicación en lote (publications-batch)

`POST /api/publications/batch` permite enviar hasta **10 publicaciones
independientes** en una sola request. Cada item puede tener su propio
`scheduled_at`, foto, botones y multi-grupo. La respuesta es
`{created[], failed[]}` con failure-isolation per-item: un item
inválido **no aborta** el resto.

```bash
curl -X POST http://localhost:8080/api/publications/batch \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "publications": [
      {"text":"Bienvenidos","group_ids":[-100123]},
      {"text":"Recordatorio","group_ids":[-100456],"scheduled_at":"2027-01-01T10:00:00Z"},
      {"text":"Promo","group_ids":[-100123],"buttons":[[{"text":"Ver","url":"https://ejemplo.com"}]]}
    ]
  }'
```

- **Cap**: `len(publications)` entre 1 y 10. Fuera de rango o JSON
  malformado → 400 `VALIDATION_ERROR` (la envelope NUNCA se procesa).
- **Status**: 200 OK si la envelope es válida (incluso si TODOS los
  items fallaron; los resultados viven en `failed[]`). 401 sin auth.
- **`created[]`**: cada entry trae `index` (posición original del
  item) + `publication` (fila completa con `status=sent|scheduled`).
- **`failed[]`**: cada entry trae `{index, code, message}` con code
  ∈ `{VALIDATION_ERROR, PERMISSION_DENIED, NOT_FOUND, TELEGRAM_ERROR,
  INTERNAL_ERROR}`. Los items programados con `scheduled_at` en el
  pasado van a `failed[]` sin tocar el service.

**Importante**:
- El batch NO se reintenta automáticamente. Si un item falla con
  `PERMISSION_DENIED` o `TELEGRAM_ERROR`, el admin puede usar el
  botón **"Reintentar fallidas"** del modal del panel — re-envía solo
  los slots fallidos preservando sus datos.
- Las publicaciones programadas del batch las recoge el worker
  in-process existente (cada 30s, `FOR UPDATE SKIP LOCKED`); cero
  código nuevo en el worker.

## Moderación automática (Fase 3)

El bot puede aplicar reglas automáticas sobre los mensajes entrantes y
ejecutar acciones escalonadas (warning → auto-mute → auto-ban) sin
intervención humana. La configuración es **por grupo** y vive en el
panel `/groups/:id/automation`.

### Reglas (slice 1 + 2)

El `automation.Service` mantiene un **registry de reglas** que se evalúa
en orden `Flood → AntiSpam → AntiLink → BannedWords` (cheap-first → DB-
pre-loaded). El primer hit short-circuitea el resto.

| Regla | Detecta | Toggle | Notas |
|-------|---------|--------|-------|
| **Flood** | `N` mensajes del mismo usuario en `S` segundos | `flood_enabled` | In-mem; ventana deslizante por (group, user). Reset on process restart (acceptable). |
| **Anti-spam** | MAYÚSCULAS (`>70%` letras upper, texto `>10` chars), 5+ chars repetidos consecutivos, URL con cuerpo `<20` chars | `anti_spam_enabled` | Stateless; CPU puro. |
| **Anti-link** | URLs `http(s)://` o `t.me/` o `telegram.me/` fuera de la allowlist | `anti_link_enabled` | Helper `domainMatches` exige `.` antes del allow domain (no `notexample.com` matchea `example.com`). |
| **Banned-words** | Cualquier palabra de `banned_words` aparece como substring (case-insensitive) | `banned_words_enabled` | Lista persiste en `banned_words(group_id, word)` con PK compuesta + FK CASCADE. |

### Acciones automáticas

Cuando una regla dispara, `Service.HandleMessage` incrementa el
`warning_count` del usuario en `user_warning_state` y emite un log
`RULE_TRIGGERED` (ActorID=nil porque es auto-action del sistema). El
worker luego encola auto-actions según thresholds:

- `warning_count >= automute_warnings` → `AutoActionMute` (tg.restrictChatMember
  con `UntilDate = now + automute_minutes*60`). Log `AUTOMUTE_USER`.
- `warning_count >= autoban_warnings` → `AutoActionBan` (tg.banChatMember
  indefinido + `revoke=true`). Log `AUTOBAN_USER`.

El worker respeta el rate limit del adapter (§18.1: token bucket ~25 req/seg
global + `retry_after` en 429). El check de admin antes de despachar es
`g.BotStatus == StatusAdministrator` (bugfix #172, nunca claves `can_*`).

### Warning al usuario (slice 2.1)

Antes de ejecutar la auto-acción (mute o ban), el bot envía un mensaje
visible al usuario en el chat avisándole lo que va a pasar. Esto es
feedback educativo — el usuario sabe que está cerca del umbral y puede
corregirse. El warning NO se envía en el counter 0 (primer hit) ni en
el counter == threshold (la acción ocurre en simultáneo).

**Cuándo se dispara**:

- `warning_count == automute_warnings - 1` → warning **pre-mute**.
- `warning_count == autoban_warnings - 1` → warning **pre-ban**.
- Si `automute_warnings == autoban_warnings` (edge case), ambos
  matchean el mismo counter pero solo se envía **un** warning pre-ban
  (prioridad documentada en `automation.thresholdKindFor`).

**Plantillas**: dos defaults hardcoded en español Rioplatense en
`backend/internal/automation/templates.go`:

- Pre-mute: `⚠️ {nombre}, llevás {count} advertencias. Si seguís, serás silenciado por {mute_minutes} min.`
- Pre-ban: `⚠️ {nombre}, llevás {count} advertencias. Si seguís, serás expulsado del grupo.`

Cada grupo puede customizar el texto vía `warn_user_template` (TEXT
NULL en `group_moderation_settings`, máx 1000 chars). Variables
disponibles:

- `{nombre}` — `FirstName` del autor; fallback a `Username` sin `@`;
  fallback final al literal "este usuario".
- `{count}` — `warning_count` post-increment.
- `{mute_minutes}` — `automute_minutes` del setting.

**Toggle**: `warn_user_enabled` (BOOLEAN, default `true`). El admin lo
apaga por grupo desde el panel si no quiere warnings en esa comunidad.

**Envío**: síncrono con `context.WithTimeout(5s)` vía
`automation.WarningSender`. Si el send falla (timeout, 403 por bot
removido, etc.) el pipeline **continúa** — la auto-acción igual se
encola. La fallida se loguea como `WARN_USER_SENT` con el status
correspondiente (`TELEGRAM_ERROR` / `PERMISSION_DENIED` / `NOT_FOUND`)
y `ActorID=nil`.

**Panel**: la sección 5 "Warning al usuario" en
`/groups/:id/automation` tiene el toggle + un `<Textarea>` con el
default pre-mute como placeholder y la lista de variables disponibles.
Guarda junto con el resto de los settings (single Save con
`Promise.all`).

Spec: REQ-22..31 del canónico `openspec/specs/moderation-automation/spec.md`.

### Editor de settings (panel)

Ruta `/groups/:telegram_id/automation`. Layout con 5 secciones:

1. **General**: switch principal `Habilitar moderación automática`.
2. **Reglas**: 4 sub-toggles (Flood / Anti-spam / Anti-link / Banned-words).
3. **Umbrales**: 6 NumberInputs (flood messages/seconds, warning limit,
   auto-mute warnings/minutes, auto-ban warnings, warning expire days).
4. **Listas**: 2 TagsInput (palabras prohibidas + allowlist de dominios).
5. **Warning al usuario** (slice 2.1): toggle `warn_user_enabled` +
   textarea para customizar `warn_user_template` (max 1000 chars).

**Single Save button** al fondo dispara `Promise.all` en paralelo: PUT
de settings + POST/DELETE por cada palabra/dominio cambiado. Las
notificaciones de éxito/error se acumulan por sección; un fallo NO
aborta el resto.

### Endpoints REST

| Método | Path | Body | Returns |
|--------|------|------|---------|
| GET | `/api/groups/{id}/automation/settings` | — | `{settings: AutomationSettings}` (200 / 404) |
| PUT | `/api/groups/{id}/automation/settings` | `SettingsUpdate` (body parcial) | `{settings: AutomationSettings}` (200 / 400 / 404) |
| GET | `/api/groups/{id}/automation/banned-words` | — | `{words: []string}` (200) |
| POST | `/api/groups/{id}/automation/banned-words` | `{word: string}` | `{words: []string}` (200 / 400) |
| DELETE | `/api/groups/{id}/automation/banned-words/{word}` | — | `{words: []string}` (200) |
| GET | `/api/groups/{id}/automation/link-allowlist` | — | `{domains: []string}` (200) |
| POST | `/api/groups/{id}/automation/link-allowlist` | `{domain: string}` | `{domains: []string}` (200 / 400) |
| DELETE | `/api/groups/{id}/automation/link-allowlist/{domain}` | — | `{domains: []string}` (200) |
| GET | `/api/groups/{id}/automation/warnings` | — | `{warnings: [{user_id, display_name, username, warning_count, last_warning_at, ...}], truncated: bool}` (cap top 100 + LEFT JOIN a `users`) |
| POST | `/api/groups/{id}/automation/warnings/{user_id}/reset` | — | `{user_id, warning_count: 0, reset: true}` + log `RESET_WARNINGS` con `ActorID` admin y metadata `{user_id, warning_count_before_reset}` |
| GET | `/api/groups/{id}/automation/stats?period=24h\|7d` | — | `{rule_triggered, automute, autoban, period}` (default `24h`; otros valores → 400 `VALIDATION_ERROR`) |

Todas requieren `requireAuth` (panel admin) y validan el grupo
(`groups.GetByTelegramID` → 404 si no existe).

### Dashboard de moderación (panel, slice 3)

Ruta `/groups/:telegram_id/moderation`. Es la contraparte de observación
de `/automation`: settings se editan en `/automation`; el dashboard
muestra lo que la moderación automática está haciendo. Dos secciones sin
Save button (read-only + reset manual):

1. **Estadísticas**: 3 Cards (Reglas disparadas / Auto-mute / Auto-ban)
   con conteos agregados de los logs de auto-moderación en la ventana
   seleccionada. `<Select>` con periodos `24h` (default) y `7d`; botón
   Refrescar que invalida las queries (sin auto-poll, YAGNI).
2. **Advertencias activas**: `<Table>` con display name (LEFT JOIN a
   `users`: FirstName → @username → "user {id}"), counter como Badge
   numérico, última advertencia formateada (`Intl.RelativeTimeFormat`
   es-AR), y botón Reset por fila. Cap defensivo top 100 con `<Alert>`
   amarillo si `truncated=true`. Empty state cuando no hay activas.

Reset abre `<Modal>` Mantine v7 con confirmación ("¿Resetear las
advertencias de {nombre}?"). **Importante**: reset SOLO limpia el
counter en DB (`user_warning_state`); NO desmutea al usuario en
Telegram — si el admin quiere desmutear, usa el endpoint
`POST /api/groups/{id}/users/{userId}/unmute` desde la vista de
membresía. La acción se audita con `ActorID=<admin>` y metadata
`warning_count_before_reset` para que el equipo vea cuánto se perdonó
sin tener que cruzar logs a mano.

### Audit log (slice 1 vs slice 2)

La tabla `logs` distingue **auto-actions** (slice 1: el sistema detecta
y ejecuta) de **acciones manuales** (slice 2: el admin edita settings
desde el panel):

- Auto-action → `ActorID=nil` (la acción la genera el sistema).
- Manual → `ActorID=<admin_id>` (la acción la dispara el admin del
  panel). Constantes: `UPDATE_AUTOMATION_SETTINGS`, `ADD_BANNED_WORD`,
  `REMOVE_BANNED_WORD`, `ADD_LINK_ALLOWLIST`,
  `REMOVE_LINK_ALLOWLIST`.

El comentario en `logs/model.go` documenta la distinción para futuros
mantenedores.

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