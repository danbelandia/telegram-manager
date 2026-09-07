-- +goose Up
-- Tablas de moderacion (paso 10): usuarios, solicitudes de ingreso,
-- advertencias y logs de auditoria (espec AGENTS.md §§7, 10, 11, 13).
-- users: identidad desde Telegram (D6 del design: sin FK hacia groups;
-- Telegram es la fuente de verdad y el evento no requiere joins).
CREATE TABLE users (
    id          BIGSERIAL PRIMARY KEY,
    telegram_id BIGINT      NOT NULL UNIQUE,
    first_name  TEXT        NOT NULL DEFAULT '',
    username    TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- join_requests: solicitudes de ingreso. group_id/user_id son ids de
-- Telegram; id es el requestId de las rutas (§12). decided_by es el id
-- del admin del panel que resolvio (actor de auditoria).
CREATE TABLE join_requests (
    id           BIGSERIAL PRIMARY KEY,
    group_id     BIGINT      NOT NULL,
    user_id      BIGINT      NOT NULL,
    status       TEXT        NOT NULL DEFAULT 'pending'
                 CHECK (status IN ('pending', 'approved', 'rejected')),
    requested_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    decided_at   TIMESTAMPTZ,
    decided_by   BIGINT
);
-- Listado por grupo/estado (GET /groups/:id/join-requests).
CREATE INDEX idx_join_requests_group_status ON join_requests (group_id, status);
-- Upsert de la pendiente: solo una pendiente por (grupo, usuario). Si
-- la solicitud ya se resolvio y Telegram manda otra, se inserta una
-- fila nueva (indice parcial).
CREATE UNIQUE INDEX idx_join_requests_pending_unique
    ON join_requests (group_id, user_id) WHERE status = 'pending';

-- warnings: precursores de moderacion automatica (Fase 3). Se crea la
-- tabla ahora porque el modelo de datos la declara inicial (§13); no
-- tiene repositorio ni feature en el MVP.
CREATE TABLE warnings (
    id         BIGSERIAL PRIMARY KEY,
    group_id   BIGINT      NOT NULL,
    user_id    BIGINT      NOT NULL,
    reason     TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_warnings_group_user ON warnings (group_id, user_id);

-- logs: auditoria de acciones administrativas (§11). status usa los
-- codigos §18; metadata guarda contexto estructurado, nunca secretos.
CREATE TABLE logs (
    id             BIGSERIAL PRIMARY KEY,
    actor_id       BIGINT,
    group_id       BIGINT      NOT NULL,
    action         TEXT        NOT NULL,
    target_user_id BIGINT,
    metadata       JSONB,
    status         TEXT        NOT NULL,
    error_message  TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_logs_group ON logs (group_id);

-- +goose Down
DROP TABLE logs;
DROP TABLE warnings;
DROP TABLE join_requests;
DROP TABLE users;