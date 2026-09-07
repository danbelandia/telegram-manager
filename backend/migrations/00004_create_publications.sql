-- +goose Up
-- Tabla de publicaciones (Fase 2, slice 1 — publish-now text).
-- Una sola tabla con status para todo el ciclo de vida; scheduled_at
-- NULL = publicacion inmediata (exploration D: no separar
-- scheduled_publications). FK a groups.telegram_id (id natural usado
-- en todas las rutas y llamadas a Telegram; design PQ).
CREATE TABLE publications (
    id            SERIAL      PRIMARY KEY,
    telegram_id   BIGINT      NOT NULL REFERENCES groups(telegram_id),
    text          TEXT        NOT NULL CHECK (length(text) > 0),
    status        TEXT        NOT NULL DEFAULT 'draft'
                   CHECK (status IN ('draft','scheduled','sending','sent','failed')),
    message_id    BIGINT,
    scheduled_at  TIMESTAMPTZ,
    error_message TEXT,
    actor_id      BIGINT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- Listado por fecha DESC (GET /api/publications) y filtro por grupo.
CREATE INDEX idx_publications_created_at ON publications (created_at DESC);
CREATE INDEX idx_publications_telegram_id ON publications (telegram_id);

-- +goose Down
DROP TABLE publications;
