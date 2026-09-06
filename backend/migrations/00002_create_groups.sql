-- +goose Up
-- Tabla de grupos de Telegram que el sistema administra (seccion 6 del
-- spec). Identidad desde Telegram; el estado del bot y sus permisos los
-- mantiene la deteccion de grupos (paso 8).
CREATE TABLE groups (
    id              BIGSERIAL PRIMARY KEY,
    telegram_id     BIGINT        NOT NULL UNIQUE,
    title           TEXT          NOT NULL,
    username        TEXT,
    type            TEXT          NOT NULL,
    member_count    BIGINT,
    bot_status      TEXT          NOT NULL DEFAULT 'member',
    bot_permissions JSONB,
    created_at      TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ   NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE groups;