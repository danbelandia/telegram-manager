# repo-bootstrap Specification

## Purpose

Base ejecutable del proyecto: estructura del repositorio, Docker Compose, scaffold de backend Go y frontend React, pipeline de migraciones goose y documentación local. Todo lo que el objetivo 28 del spec considera "primero".

## Requirements

### Requirement: Estructura del repositorio

The repository MUST contain a `backend/` folder, a `frontend/` folder, a `docker-compose.yml` at root, a `.env.example` at root, and a `.gitignore` that MUST exclude `.env`, `node_modules/`, build artifacts, and local Go binaries.

#### Scenario: Estado inicial del repo

- GIVEN a fresh clone
- WHEN the repository is inspected
- THEN it contains `backend/`, `frontend/`, `docker-compose.yml`, `.env.example`, and `.gitignore`
- AND no file tracked by git contains a real secret

### Requirement: Docker Compose

The system MUST provide a `docker-compose.yml` with three services: `backend`, `frontend`, and `postgres`. The `postgres` service MUST use PostgreSQL 16 with a named volume for data persistence and a healthcheck. Backend and frontend MUST read their configuration from the environment (`.env`).

#### Scenario: Levantar la stack completa

- GIVEN a valid `.env` file exists
- WHEN running `docker compose up --build`
- THEN the three services start
- AND `postgres` becomes healthy before backend/frontend depend on it

### Requirement: Variables de entorno

The `.env.example` MUST document every variable consumed by the system: `TELEGRAM_BOT_TOKEN`, `TELEGRAM_MODE`, `TELEGRAM_WEBHOOK_URL`, `TELEGRAM_WEBHOOK_SECRET`, `DATABASE_URL`, `JWT_SECRET`, `PORT`, and `VITE_API_BASE_URL`. Each variable MUST have a comment explaining its purpose.

#### Scenario: Nuevo variable agregada

- GIVEN a developer introduces a new environment variable
- WHEN the code change is committed
- THEN `.env.example` is updated in the same commit

### Requirement: Configuración del backend con falla rápida

The backend MUST load all configuration from environment variables in a single `internal/config` package. If `TELEGRAM_BOT_TOKEN` or `DATABASE_URL` are missing, the application MUST fail fast at startup with a clear message and non-zero exit code.

#### Scenario: Falta configuración crítica

- GIVEN `DATABASE_URL` is not set
- WHEN the backend starts
- THEN it exits with a non-zero code
- AND the error message names the missing variable without revealing any secret

### Requirement: Migraciones goose

The backend MUST use `github.com/pressly/goose` for database migrations stored in `backend/migrations/` as plain SQL. In development, migrations MUST be applied automatically at startup. The initial migration MUST create the `admins` table with `password_hash`, `created_at`, and `last_login_at` columns.

#### Scenario: Migración inicial aplicada

- GIVEN a fresh PostgreSQL database
- WHEN the backend starts in development mode
- THEN the `admins` table exists with the required columns

### Requirement: Scaffold frontend

The `frontend/` folder MUST be a Vite + React + TypeScript project with `strict: true` in `tsconfig.json`, React Router configured with a placeholder route, and a minimal `lib/api-client.ts` HTTP client that reads `VITE_API_BASE_URL`.

#### Scenario: Build de desarrollo

- GIVEN the frontend source is present
- WHEN running the TypeScript type checker
- THEN it passes with no errors under `strict: true`

### Requirement: Documentación de ejecución local

The `README.md` MUST explain how to run the project locally with Docker Compose and without Docker, how migrations behave in development, and how to configure `TELEGRAM_MODE=polling` for local development.

#### Scenario: Nuevo desarrollador

- GIVEN a new developer
- WHEN following the README
- THEN they can start the full stack and see the health endpoint respond