-- +goose Up
-- Slice 2.1 — Warning al usuario antes de la accion automatica (REQ-22).
-- Agrega 2 columnas a group_moderation_settings:
--   warn_user_enabled  BOOLEAN  — toggle por grupo (default ON out-of-the-box).
--   warn_user_template TEXT     — override per-grupo de la plantilla del
--                                  warning (NULL = usar default hardcoded
--                                  en automation/templates.go).
-- Non-destructivo: defaults seguros (true, NULL). Compatible con las 13
-- columnas previas (slice 1 + slice 2). Sin CHECK sobre el template
-- (texto libre); la validacion vive en el handler (max 1000 chars).
ALTER TABLE group_moderation_settings
    ADD COLUMN warn_user_enabled  BOOLEAN NOT NULL DEFAULT true,
    ADD COLUMN warn_user_template TEXT NULL;

-- +goose Down
ALTER TABLE group_moderation_settings
    DROP COLUMN warn_user_template,
    DROP COLUMN warn_user_enabled;
