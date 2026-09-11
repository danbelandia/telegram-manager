# Super Admin Specification

## Purpose

A super-admin is an admin with full visibility across all tenants. The
flag `is_super_admin` on the `admins` table grants access to the admin
panel and all tenant data. Super-admin status is seeded via migration,
not exposed via public endpoints.

## Requirements

### Requirement: Super-admin flag on admins

The system MUST have a boolean `is_super_admin` column on `admins`
defaulting to `false`. The migration MUST seed one super-admin if none
exists (idempotent: only if all admins have `is_super_admin = false`).

#### Scenario: Migration seeds super-admin

- GIVEN the migration 00010 runs on a DB with at least one admin where
  `is_super_admin = false`
- WHEN the migration completes
- THEN one admin has `is_super_admin = true`

#### Scenario: No duplicate seed

- GIVEN the migration runs and at least one admin already has
  `is_super_admin = true`
- WHEN the migration completes
- THEN no admin rows are modified

### Requirement: JWT claims include is_super_admin

The system MUST include `is_super_admin` (boolean) in the JWT access
token claims. Claims MUST include `sub`, `username`, `tenant_id`,
`tenant_slug`, and `is_super_admin`. Legacy tokens without
`is_super_admin` MUST default to `false`.

#### Scenario: New token includes flag

- GIVEN a login by a super-admin
- WHEN the access token is decoded
- THEN `is_super_admin = true` is present in claims

#### Scenario: Legacy token backward compatible

- GIVEN an access token issued before this feature (no `is_super_admin` claim)
- WHEN the token is validated
- THEN `is_super_admin` defaults to `false`

### Requirement: requireSuperAdmin middleware

The system MUST provide a middleware function `requireSuperAdmin` that
checks `is_super_admin` in the JWT claims. Non-super-admins MUST
receive 403.

#### Scenario: Super-admin passes middleware

- GIVEN an authenticated super-admin
- WHEN a route protected by `requireSuperAdmin` is called
- THEN the request proceeds

#### Scenario: Regular admin blocked

- GIVEN an authenticated admin with `is_super_admin = false`
- WHEN a route protected by `requireSuperAdmin` is called
- THEN responds 403 with code `FORBIDDEN`

### Requirement: Admin panel page

The system MUST expose `/admin/tenants` in the frontend, visible only
to super-admins. The page MUST display a table of all tenants with
columns: slug, plan, status, trial_ends_at, expires_at. Filters by
status MUST be available.

#### Scenario: Super-admin sees admin panel

- GIVEN a logged-in super-admin
- WHEN navigating the sidebar
- THEN "Admin Panel" link is visible and navigates to `/admin/tenants`

#### Scenario: Regular admin does not see admin panel

- GIVEN a logged-in admin with `is_super_admin = false`
- WHEN the sidebar renders
- THEN "Admin Panel" link is NOT visible

### Requirement: Conditional sidebar rendering

The frontend Layout MUST conditionally render the "Admin Panel"
sidebar item based on `user.isSuperAdmin === true`. The check MUST
use the `isSuperAdmin` field from `SessionUser` in auth-context.

#### Scenario: Super-admin sidebar

- GIVEN `isSuperAdmin` is `true` in the session
- WHEN the Layout renders
- THEN the admin panel nav item appears

#### Scenario: Regular admin sidebar

- GIVEN `isSuperAdmin` is `false` in the session
- WHEN the Layout renders
- THEN the admin panel nav item is hidden
