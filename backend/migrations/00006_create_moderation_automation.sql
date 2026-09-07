-- +goose Up
-- Tablas de moderacion automatica (Fase 3, slice 1 — foundation).
-- group_moderation_settings: settings per-grupo con toggles y
-- thresholds (CHECK constraints). FK a groups.telegram_id (id
-- natural) con CASCADE para que borrar un grupo borre sus settings.
-- user_warning_state: counter per (group, user) para los thresholds
-- auto-mute/auto-ban. PK compuesta para upsert eficiente. Indices
-- sobre group_id (listado) y expires_at (cleanup periodico).

CREATE TABLE group_moderation_settings (
    group_id              BIGINT      PRIMARY KEY REFERENCES groups(telegram_id) ON DELETE CASCADE,
    enabled               BOOLEAN     NOT NULL DEFAULT false,
    anti_spam_enabled     BOOLEAN     NOT NULL DEFAULT false,
    anti_link_enabled     BOOLEAN     NOT NULL DEFAULT false,
    banned_words_enabled  BOOLEAN     NOT NULL DEFAULT false,
    flood_enabled         BOOLEAN     NOT NULL DEFAULT false,
    flood_messages        SMALLINT    NOT NULL DEFAULT 5   CHECK (flood_messages > 0),
    flood_seconds         SMALLINT    NOT NULL DEFAULT 10  CHECK (flood_seconds > 0),
    warning_limit         SMALLINT    NOT NULL DEFAULT 3   CHECK (warning_limit > 0),
    automute_warnings     SMALLINT    NOT NULL DEFAULT 3   CHECK (automute_warnings > 0),
    automute_minutes      SMALLINT    NOT NULL DEFAULT 10  CHECK (automute_minutes > 0),
    autoban_warnings      SMALLINT    NOT NULL DEFAULT 5   CHECK (autoban_warnings > automute_warnings),
    warning_expire_days   SMALLINT    NOT NULL DEFAULT 30  CHECK (warning_expire_days > 0),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE user_warning_state (
    group_id         BIGINT      NOT NULL,
    user_id          BIGINT      NOT NULL,
    warning_count    SMALLINT    NOT NULL DEFAULT 0 CHECK (warning_count >= 0),
    last_warning_at  TIMESTAMPTZ,
    last_action_at   TIMESTAMPTZ,
    expires_at       TIMESTAMPTZ,
    PRIMARY KEY (group_id, user_id)
);
CREATE INDEX idx_user_warning_state_group    ON user_warning_state (group_id);
CREATE INDEX idx_user_warning_state_expires  ON user_warning_state (expires_at);

-- +goose Down
DROP TABLE IF EXISTS user_warning_state;
DROP TABLE IF EXISTS group_moderation_settings;