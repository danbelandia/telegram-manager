-- +goose Up
-- Tabla de administradores del panel. Incluye los campos de
-- autenticacion (seccion 17.1 del spec): password_hash, created_at,
-- last_login_at.
CREATE TABLE admins (
    id            BIGSERIAL PRIMARY KEY,
    username      TEXT        NOT NULL UNIQUE,
    password_hash TEXT        NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_login_at TIMESTAMPTZ
);

-- +goose Down
DROP TABLE admins;