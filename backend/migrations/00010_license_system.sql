-- +goose Up
-- License system + super-admin (proposal license-system).
-- Adds license lifecycle fields to tenants, super-admin flag to admins,
-- and seeds the first super-admin.

-- 1. Normalize existing 'degraded' status before changing the CHECK.
-- 'degraded' tracked bot-token revocation (runtime concern); the new
-- schema separates license status from bot connectivity. Existing
-- tenants were active users — mark them as license-active.
UPDATE tenants SET status = 'active' WHERE status = 'degraded';

-- 2. Replace the status CHECK: drop old (active|degraded), add new
--    (trial|active|suspended|expired). Default changes to 'trial' so
--    every NEW insert (signup) starts in trial automatically.
ALTER TABLE tenants DROP CONSTRAINT tenants_status_check;
ALTER TABLE tenants ALTER COLUMN status SET DEFAULT 'trial';
ALTER TABLE tenants ADD CONSTRAINT tenants_status_check
    CHECK (status IN ('trial', 'active', 'suspended', 'expired'));

-- 3. License columns on tenants.
--    trial_ends_at: 72h from creation (DB expression default).
--    expires_at: NULL until super-admin activates or trial converts.
--    plan: 'pro' for now; the single tier. Future plans extend this.
--    max_groups / max_messages_day: -1 = unlimited (soft limit hooks
--    for Phase 4 automations).
ALTER TABLE tenants ADD COLUMN trial_ends_at TIMESTAMPTZ
    DEFAULT (now() + interval '72 hours');
ALTER TABLE tenants ADD COLUMN expires_at TIMESTAMPTZ;
ALTER TABLE tenants ADD COLUMN plan TEXT NOT NULL DEFAULT 'pro';
ALTER TABLE tenants ADD COLUMN max_groups INT NOT NULL DEFAULT -1;
ALTER TABLE tenants ADD COLUMN max_messages_day INT NOT NULL DEFAULT -1;

-- 4. Super-admin flag on admins.
ALTER TABLE admins ADD COLUMN is_super_admin BOOLEAN NOT NULL DEFAULT false;

-- 5. Seed: promote the oldest admin to super-admin if none exists.
--    Idempotent: the WHERE NOT EXISTS prevents re-promotion.
UPDATE admins SET is_super_admin = true
WHERE id = (SELECT MIN(id) FROM admins)
  AND NOT EXISTS (SELECT 1 FROM admins WHERE is_super_admin = true);

-- +goose Down
-- Reverse order: remove newest columns first, restore old constraints.

-- 5. Remove super-admin flag.
ALTER TABLE admins DROP COLUMN is_super_admin;

-- 4. Remove license columns.
ALTER TABLE tenants DROP COLUMN max_messages_day;
ALTER TABLE tenants DROP COLUMN max_groups;
ALTER TABLE tenants DROP COLUMN plan;
ALTER TABLE tenants DROP COLUMN expires_at;
ALTER TABLE tenants DROP COLUMN trial_ends_at;

-- 3. Restore original status CHECK.
ALTER TABLE tenants DROP CONSTRAINT tenants_status_check;
ALTER TABLE tenants ALTER COLUMN status SET DEFAULT 'active';
ALTER TABLE tenants ADD CONSTRAINT tenants_status_check
    CHECK (status IN ('active', 'degraded'));

-- 2. Map new statuses back to old ones for downgrade safety.
UPDATE tenants SET status = 'degraded' WHERE status IN ('suspended', 'expired');
UPDATE tenants SET status = 'active' WHERE status IN ('trial');
