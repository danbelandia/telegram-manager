-- +goose Up
-- Slice 0 — Multitenancy backend (bot-per-tenant).
-- Introduce `tenants` y agrega `tenant_id` a todo salvo `users`
-- (Q3-a: users es identidad Telegram global, scope via join).
-- Orden estricto: tenants → groups (+backfill) → resto de tablas →
-- drop de FKs simples → unicidad compuesta → FKs compuestas → NOT NULL.
-- Todo el backfill es idempotente (WHERE tenant_id IS NULL +
-- WHERE NOT EXISTS) para re-aplicacion segura.

-- 1. Tabla tenants.
CREATE TABLE tenants (
    id BIGSERIAL PRIMARY KEY,
    slug TEXT NOT NULL UNIQUE CHECK (length(slug) BETWEEN 1 AND 63),
    tier TEXT NOT NULL DEFAULT 'pro' CHECK (tier = 'pro'),
    bot_token_encrypted BYTEA,
    bot_username TEXT,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'degraded')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 2. groups: columna + tenant default + backfill idempotente.
ALTER TABLE groups ADD COLUMN tenant_id BIGINT REFERENCES tenants(id);
INSERT INTO tenants (slug) VALUES ('default')
WHERE NOT EXISTS (SELECT 1 FROM tenants WHERE slug = 'default');
UPDATE groups SET tenant_id = (SELECT id FROM tenants WHERE slug = 'default')
WHERE tenant_id IS NULL;

-- 3. Resto de tablas con alcance de tenant (sin FK a groups):
--    admins (username sigue UNIQUE global, Q1-a), join_requests,
--    warnings, logs, user_warning_state. group_members no existe en
--    este esquema: no se crea nada para funcionalidades futuras.
ALTER TABLE admins ADD COLUMN tenant_id BIGINT REFERENCES tenants(id);
UPDATE admins SET tenant_id = (SELECT id FROM tenants WHERE slug = 'default')
WHERE tenant_id IS NULL;

ALTER TABLE join_requests ADD COLUMN tenant_id BIGINT REFERENCES tenants(id);
UPDATE join_requests j SET tenant_id = g.tenant_id FROM groups g
WHERE j.tenant_id IS NULL AND g.telegram_id = j.group_id;
UPDATE join_requests SET tenant_id = (SELECT id FROM tenants WHERE slug = 'default')
WHERE tenant_id IS NULL;

ALTER TABLE warnings ADD COLUMN tenant_id BIGINT REFERENCES tenants(id);
UPDATE warnings w SET tenant_id = g.tenant_id FROM groups g
WHERE w.tenant_id IS NULL AND g.telegram_id = w.group_id;
UPDATE warnings SET tenant_id = (SELECT id FROM tenants WHERE slug = 'default')
WHERE tenant_id IS NULL;

ALTER TABLE logs ADD COLUMN tenant_id BIGINT REFERENCES tenants(id);
UPDATE logs l SET tenant_id = g.tenant_id FROM groups g
WHERE l.tenant_id IS NULL AND g.telegram_id = l.group_id;
UPDATE logs SET tenant_id = (SELECT id FROM tenants WHERE slug = 'default')
WHERE tenant_id IS NULL;

ALTER TABLE user_warning_state ADD COLUMN tenant_id BIGINT REFERENCES tenants(id);
UPDATE user_warning_state s SET tenant_id = g.tenant_id FROM groups g
WHERE s.tenant_id IS NULL AND g.telegram_id = s.group_id;
UPDATE user_warning_state SET tenant_id = (SELECT id FROM tenants WHERE slug = 'default')
WHERE tenant_id IS NULL;

-- Tablas hijas con FK a groups(telegram_id): agregan tenant_id y lo
-- backfillean via join. El join es determinista: pre-00009
-- groups.telegram_id era UNIQUE, no ambigua.
ALTER TABLE publications ADD COLUMN tenant_id BIGINT;
UPDATE publications p SET tenant_id = g.tenant_id FROM groups g
WHERE p.tenant_id IS NULL AND g.telegram_id = p.telegram_id;
UPDATE publications SET tenant_id = (SELECT id FROM tenants WHERE slug = 'default')
WHERE tenant_id IS NULL;

ALTER TABLE group_moderation_settings ADD COLUMN tenant_id BIGINT;
UPDATE group_moderation_settings s SET tenant_id = g.tenant_id FROM groups g
WHERE s.tenant_id IS NULL AND g.telegram_id = s.group_id;
UPDATE group_moderation_settings SET tenant_id = (SELECT id FROM tenants WHERE slug = 'default')
WHERE tenant_id IS NULL;

ALTER TABLE banned_words ADD COLUMN tenant_id BIGINT;
UPDATE banned_words b SET tenant_id = g.tenant_id FROM groups g
WHERE b.tenant_id IS NULL AND g.telegram_id = b.group_id;
UPDATE banned_words SET tenant_id = (SELECT id FROM tenants WHERE slug = 'default')
WHERE tenant_id IS NULL;

ALTER TABLE link_allowlist ADD COLUMN tenant_id BIGINT;
UPDATE link_allowlist l SET tenant_id = g.tenant_id FROM groups g
WHERE l.tenant_id IS NULL AND g.telegram_id = l.group_id;
UPDATE link_allowlist SET tenant_id = (SELECT id FROM tenants WHERE slug = 'default')
WHERE tenant_id IS NULL;

-- 4. Las FKs simples a groups(telegram_id) quedan invalidas al soltar
--    el UNIQUE simple: se dropean antes (nombres auto de PostgreSQL
--    {tabla}_{columna}_fkey para REFERENCES inline sin nombre).
ALTER TABLE publications DROP CONSTRAINT publications_telegram_id_fkey;
ALTER TABLE group_moderation_settings DROP CONSTRAINT group_moderation_settings_group_id_fkey;
ALTER TABLE banned_words DROP CONSTRAINT banned_words_group_id_fkey;
ALTER TABLE link_allowlist DROP CONSTRAINT link_allowlist_group_id_fkey;

-- 5. groups: NOT NULL + unicidad compuesta (mismo telegram_id puede
--    existir en dos tenants, nunca duplicado en el mismo).
ALTER TABLE groups ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE groups DROP CONSTRAINT groups_telegram_id_key;
ALTER TABLE groups ADD CONSTRAINT groups_tenant_telegram_unique UNIQUE (tenant_id, telegram_id);
CREATE INDEX idx_groups_tenant ON groups (tenant_id);

-- 6. FKs compuestas (tenant_id, id-telegram) → groups. Preserva el
--    patron id-natural del codigo (rutas y Telegram usan telegram_id).
ALTER TABLE publications ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE publications ADD CONSTRAINT publications_group_fk
  FOREIGN KEY (tenant_id, telegram_id) REFERENCES groups(tenant_id, telegram_id) ON DELETE CASCADE;

ALTER TABLE group_moderation_settings ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE group_moderation_settings ADD CONSTRAINT group_moderation_settings_group_fk
  FOREIGN KEY (tenant_id, group_id) REFERENCES groups(tenant_id, telegram_id) ON DELETE CASCADE;

ALTER TABLE banned_words ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE banned_words ADD CONSTRAINT banned_words_group_fk
  FOREIGN KEY (tenant_id, group_id) REFERENCES groups(tenant_id, telegram_id) ON DELETE CASCADE;

ALTER TABLE link_allowlist ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE link_allowlist ADD CONSTRAINT link_allowlist_group_fk
  FOREIGN KEY (tenant_id, group_id) REFERENCES groups(tenant_id, telegram_id) ON DELETE CASCADE;

-- 7. Resto: NOT NULL + indices por tenant (primer predicado del WHERE).
ALTER TABLE admins ALTER COLUMN tenant_id SET NOT NULL;
CREATE INDEX idx_admins_tenant ON admins (tenant_id);

ALTER TABLE join_requests ALTER COLUMN tenant_id SET NOT NULL;
CREATE INDEX idx_join_requests_tenant ON join_requests (tenant_id);

ALTER TABLE warnings ALTER COLUMN tenant_id SET NOT NULL;
CREATE INDEX idx_warnings_tenant ON warnings (tenant_id);

ALTER TABLE logs ALTER COLUMN tenant_id SET NOT NULL;
CREATE INDEX idx_logs_tenant ON logs (tenant_id);

ALTER TABLE user_warning_state ALTER COLUMN tenant_id SET NOT NULL;
CREATE INDEX idx_user_warning_state_tenant ON user_warning_state (tenant_id);

CREATE INDEX idx_publications_tenant ON publications (tenant_id);

-- +goose Down
DROP INDEX IF EXISTS idx_publications_tenant;
DROP INDEX IF EXISTS idx_user_warning_state_tenant;
ALTER TABLE user_warning_state DROP COLUMN tenant_id;
DROP INDEX IF EXISTS idx_logs_tenant;
ALTER TABLE logs DROP COLUMN tenant_id;
DROP INDEX IF EXISTS idx_warnings_tenant;
ALTER TABLE warnings DROP COLUMN tenant_id;
DROP INDEX IF EXISTS idx_join_requests_tenant;
ALTER TABLE join_requests DROP COLUMN tenant_id;
DROP INDEX IF EXISTS idx_admins_tenant;
ALTER TABLE admins DROP COLUMN tenant_id;

ALTER TABLE link_allowlist DROP CONSTRAINT link_allowlist_group_fk;
ALTER TABLE link_allowlist DROP COLUMN tenant_id;
ALTER TABLE banned_words DROP CONSTRAINT banned_words_group_fk;
ALTER TABLE banned_words DROP COLUMN tenant_id;
ALTER TABLE group_moderation_settings DROP CONSTRAINT group_moderation_settings_group_fk;
ALTER TABLE group_moderation_settings DROP COLUMN tenant_id;
ALTER TABLE publications DROP CONSTRAINT publications_group_fk;
ALTER TABLE publications DROP COLUMN tenant_id;

DROP INDEX IF EXISTS idx_groups_tenant;
ALTER TABLE groups DROP CONSTRAINT groups_tenant_telegram_unique;
ALTER TABLE groups ADD CONSTRAINT groups_telegram_id_key UNIQUE (telegram_id);
ALTER TABLE groups DROP COLUMN tenant_id;

ALTER TABLE link_allowlist ADD CONSTRAINT link_allowlist_group_id_fkey
  FOREIGN KEY (group_id) REFERENCES groups(telegram_id) ON DELETE CASCADE;
ALTER TABLE banned_words ADD CONSTRAINT banned_words_group_id_fkey
  FOREIGN KEY (group_id) REFERENCES groups(telegram_id) ON DELETE CASCADE;
ALTER TABLE group_moderation_settings ADD CONSTRAINT group_moderation_settings_group_id_fkey
  FOREIGN KEY (group_id) REFERENCES groups(telegram_id) ON DELETE CASCADE;
ALTER TABLE publications ADD CONSTRAINT publications_telegram_id_fkey
  FOREIGN KEY (telegram_id) REFERENCES groups(telegram_id);

DROP TABLE tenants;
