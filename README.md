# Telegram Group Manager

Plataforma para administrar grupos de Telegram: bot, backend en Go, panel
React + TypeScript y PostgreSQL.

> Estado: bootstrap en curso (cambio `repo-bootstrap`). Backend y
> frontend funcionales; los módulos de negocio (grupos, usuarios,
> moderación, auth del panel) llegan en cambios siguientes.

## Stack

- **Backend**: Go (API REST)
- **Frontend**: React + TypeScript (Vite)
- **Base de datos**: PostgreSQL 16
- **Infra**: Docker Compose

## Requisitos

- Docker Desktop
- Go 1.26+ y Node 20+ (solo desarrollo sin Docker)
- Un bot de Telegram creado con [@BotFather](https://t.me/BotFather)

## Puesta en marcha

### 1. Configurar entorno

```bash
cp .env.example .env
```

Completar `TELEGRAM_BOT_TOKEN` con el token del bot. **Nunca** commitear
`.env`.

### 2. Levantar la stack

```bash
docker compose up -d
```

Levanta los tres servicios (`backend`, `frontend`, `postgres`). Postgres
queda en `localhost:5432`, el backend en `http://localhost:8080` y el
panel en `http://localhost:5173`. Requiere `TELEGRAM_BOT_TOKEN` en
`.env` (el compose falla rápido si falta).

### 3. Variables de entorno

| Variable | Descripción |
|----------|-------------|
| `TELEGRAM_BOT_TOKEN` | Token del bot (obligatorio) |
| `TELEGRAM_MODE` | `polling` (dev) o `webhook` (prod) |
| `TELEGRAM_WEBHOOK_URL` / `TELEGRAM_WEBHOOK_SECRET` | Solo modo webhook |
| `DATABASE_URL` | DSN de PostgreSQL |
| `JWT_SECRET` | Secreto para JWT (módulo auth) |
| `PORT` | Puerto del backend |
| `VITE_API_BASE_URL` | URL base que usa el frontend |
| `RUN_MIGRATIONS` | Aplica migraciones al arrancar (dev) |

### 4. Migraciones

- En desarrollo se aplican automáticamente al arrancar el backend
  (`RUN_MIGRATIONS=true`).
- En producción se ejecutan como paso manual documentado.

## Documentación técnica

- Especificación del MVP: [`AGENTS.md`](AGENTS.md)
- Referencia de la Bot API: [`docs/telegram_api_reference.md`](docs/telegram_api_reference.md)
- Guías de backend y frontend: [`docs/`](docs/)