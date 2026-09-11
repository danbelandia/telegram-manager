# License Management Specification

## Purpose

Control the lifecycle of tenant licenses: trial activation, active status,
suspension, and expiry. Admin CRUD for tenant license fields. The system
enforces license state — tenants with suspended/expired licenses are
blocked from all functionality.

## Requirements

### Requirement: License states

The system MUST track license status per tenant with these states:
`trial`, `active`, `suspended`, `expired`. Status MUST transition
per the rules below. No other states are permitted.

#### Scenario: New tenant starts in trial

- GIVEN a tenant created via signup
- WHEN the tenant row is inserted
- THEN `status = 'trial'`, `trial_ends_at = now() + 72 hours`,
  `expires_at = NULL`

#### Scenario: Status flow trial → active

- GIVEN a tenant with `status = 'trial'`
- WHEN super-admin sets `status = 'active'` via admin panel
- THEN `status` becomes `'active'` and `expires_at` is set

#### Scenario: Status flow active → suspended

- GIVEN a tenant with `status = 'active'`
- WHEN super-admin sets `status = 'suspended'`
- THEN `status` becomes `'suspended'` and the tenant is blocked

#### Scenario: Status flow trial → expired

- GIVEN a tenant with `status = 'trial'` and `trial_ends_at` in the past
- WHEN the enforcement check runs
- THEN `status` is set to `'expired'` and the tenant is blocked

#### Scenario: Invalid transition rejected

- GIVEN a tenant with `status = 'expired'`
- WHEN super-admin attempts to set `status = 'active'` directly
- THEN the system MUST reject with 400 and code `INVALID_TRANSITION`

### Requirement: Trial duration

The system MUST set `trial_ends_at` to exactly 72 hours (3 days) from
tenant creation at signup. Trial duration MUST NOT be configurable
beyond admin-panel extension.

#### Scenario: Trial expiry blocks tenant

- GIVEN a tenant with `status = 'trial'` and `trial_ends_at` in the past
- WHEN the tenant attempts any API call
- THEN the enforcement middleware returns 403 with code
  `LICENSE_EXPIRED`

#### Scenario: Super-admin extends trial

- GIVEN a tenant with `status = 'trial'`
- WHEN super-admin updates `trial_ends_at` to a future date via admin panel
- THEN the trial window is extended and tenant remains unblocked

### Requirement: License enforcement

The system MUST enforce license status via middleware on all routes
except auth and the license-info endpoint itself. Suspended or expired
tenants MUST receive 403 with code `LICENSE_SUSPENDED` or
`LICENSE_EXPIRED` respectively.

#### Scenario: Suspended tenant blocked

- GIVEN a tenant with `status = 'suspended'`
- WHEN any authenticated request is made to a protected route
- THEN the middleware returns 403 with code `LICENSE_SUSPENDED`

#### Scenario: Trial tenant allowed

- GIVEN a tenant with `status = 'trial'` and `trial_ends_at` in the future
- WHEN a protected route is called
- THEN the request proceeds normally

#### Scenario: Auth routes exempt

- GIVEN a tenant with `status = 'suspended'`
- WHEN `POST /api/auth/login` or `POST /api/auth/refresh` is called
- THEN the license middleware does NOT block the request

### Requirement: Admin CRUD for tenant licenses

The system MUST expose `GET /api/admin/tenants` (list all),
`GET /api/admin/tenants/:id` (detail), and
`PUT /api/admin/tenants/:id` (update license fields). The PUT MUST
accept `status`, `plan`, `trial_ends_at`, `expires_at`, `max_groups`,
`max_messages_day` and validate transitions.

#### Scenario: Super-admin lists all tenants

- GIVEN an authenticated super-admin
- WHEN `GET /api/admin/tenants` is called
- THEN responds 200 with all tenants including license fields

#### Scenario: Super-admin updates tenant license

- GIVEN an authenticated super-admin and a tenant with `status = 'trial'`
- WHEN `PUT /api/admin/tenants/:id` with `{status: "active", plan: "pro"}`
- THEN tenant is updated and 200 returned

#### Scenario: Non-super-admin blocked

- GIVEN an authenticated admin without `is_super_admin`
- WHEN `GET /api/admin/tenants` is called
- THEN responds 403

### Requirement: License info for own tenant

The system MUST expose `GET /api/tenants/me/license` returning
`status`, `plan`, `trial_ends_at`, `expires_at`, `max_groups`,
`max_messages_day` for the caller's tenant.

#### Scenario: Tenant queries own license

- GIVEN an authenticated admin
- WHEN `GET /api/tenants/me/license` is called
- THEN responds 200 with the tenant's license data
