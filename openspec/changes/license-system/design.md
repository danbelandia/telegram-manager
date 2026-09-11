# Design: Sistema de Licencias y Super-Admin

## 1. Architecture Decisions

### D1: License enforcement via per-request DB lookup (not cache, not JWT)

**Decision:** The `requireLicense` middleware queries `SELECT status, trial_ends_at FROM tenants WHERE id = $1` on every protected request.

**Alternatives considered:**
- **Embed license status in JWT claims:** Simplest, but stale for up to 15 min (access token TTL). A suspended tenant keeps working until token expiry. Unacceptable for a paywall.
- **In-memory cache with TTL:** Efficient, but adds complexity (goroutine, TTL map, invalidation on admin edits) with no infra justification at MVP scale.
- **Redis cache:** Explicitly excluded by AGENTS.md §2.

**Why this is correct:** Single-row PK lookup in PostgreSQL is an index scan (~0.1ms). With `*sql.DB` connection pooling, this adds negligible overhead. The `requireAuth` middleware already parses the JWT on every request — one more lightweight query is proportional. We optimize later if metrics show it matters.

### D2: DB defaults for trial fields (not code-side)

**Decision:** `trial_ends_at` gets a PostgreSQL DEFAULT of `(now() + interval '72 hours')` and `status` defaults to `'trial'`. The signup INSERT doesn't mention these columns — they inherit defaults.

**Why:** The signup transaction (`service.go` Signup) already does INSERT tenants + INSERT admins. Adding columns to the INSERT would require modifying the transaction SQL and all test fixtures. DB defaults are zero-cost, idempotent, and apply to ANY insert path (signup, EnsureDefault, future scripts). The `trial_ends_at` default is a computed expression that fires at insert time, so each new tenant gets exactly 72h from creation.

### D3: Admin handlers in `internal/api`, not `internal/admin`

**Decision:** Super-admin CRUD handlers go in `internal/api/admin_handlers.go`, following the existing pattern where ALL HTTP handlers live in `internal/api/*_handlers.go`.

**Why:** The codebase has a consistent convention: `internal/api/` owns HTTP, `internal/*/service.go` owns business logic. Creating a separate `internal/admin/` package for handlers would break this convention and create a confusing split (some handlers in `api`, some in `admin`). The license business logic DOES get its own package (`internal/license/service.go`) because it's reusable business logic, not HTTP-specific.

### D4: `requireLicense` chained after `requireAuth`, not combined

**Decision:** Two separate middleware wrappers: `requireAuth` sets claims in context, `requireLicense` reads claims + checks tenant status.

```go
s.mux.HandleFunc("GET /api/groups", s.requireAuth(s.requireLicense(s.handleListGroups)))
```

**Why:** Single-responsibility. `requireAuth` is about identity (who are you?). `requireLicense` is about authorization (is your tenant allowed?). They can evolve independently. Auth routes exempt from license check by simply not chaining `requireLicense`.

### D5: License enforcement exempts ALL `/api/auth/*` routes

**Decision:** The middleware skips license checks for any path matching `/api/auth/`. Also exempts `GET /api/tenants/me/license`.

**Why:** A suspended tenant must still be able to:
1. Call `/auth/me` — the frontend needs identity to display the license error screen.
2. Call `/auth/refresh` — the session renewal must work so the user can see the error (not a silent 401).
3. Call `/tenants/me/license` — the LicenseCard component needs this data to render the status.

Blocking these would create a chicken-and-egg problem: the user can't see WHY they're blocked.

### D6: `IsSuperAdmin` in JWT claims with backward compatibility

**Decision:** Add `is_super_admin` (bool, `omitempty`) to `Claims`. Legacy tokens without this field decode as `false` (Go zero value).

**Why:** The `omitempty` tag means tokens issued before this feature simply don't have the field. When `jwt.ParseWithClaims` decodes them, the missing field defaults to `false`. No migration of existing tokens needed — they naturally expire within 15 min and new ones include the flag.

### D7: `TenantSlug` added to Claims

**Decision:** Add `tenant_slug` (string, `omitempty`) to `Claims` struct.

**Why:** The auth spec delta requires it. Currently `handleMe` looks up the slug from DB on every call. Embedding it in the token eliminates that lookup for the `/me` endpoint. If a tenant changes its slug, the stale value expires with the token (15 min). Acceptable tradeoff.

---

## 2. Data Model — Migration 00010

File: `backend/migrations/00010_license_system.sql`

```sql
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
UPDATE tenants SET status = 'active' WHERE status = 'trial';
```

### Migration notes

| Concern | Resolution |
|---------|-----------|
| `degraded` rows at migration time | Converted to `active`. Bot connectivity is a runtime concern (registry status), not a license status. |
| `EnsureDefault` tenant | Gets `trial_ends_at` from DB default, but its status is set to `active` by the backfill. Harmless — trial logic only triggers on `status = 'trial'`. |
| New signups | `status = 'trial'` (DEFAULT), `trial_ends_at = now() + 72h` (DEFAULT expression). No code change needed in signup INSERT. |
| Downgrade | Maps `suspended`/`expired` → `degraded`, `trial` → `active`. Existing CHECK restored. |

---

## 3. Backend Packages

### 3.1 New files

#### `backend/internal/license/service.go`

Encapsulates license lifecycle logic. Stateless — no DB connection of its own; receives a tenant getter interface.

```go
package license

import (
    "context"
    "errors"
    "time"
)

// Errors.
var (
    ErrSuspended = errors.New("license: tenant suspended")
    ErrExpired   = errors.New("license: tenant expired")
)

// TenantLicense is the subset of tenant data the service needs.
type TenantLicense struct {
    ID           int64
    Status       string
    TrialEndsAt  *time.Time
    ExpiresAt    *time.Time
}

// TenantGetter retrieves the license-relevant fields for a tenant.
// *tenants.Repository satisfies this via a new GetLicense method.
type TenantGetter interface {
    GetLicense(ctx context.Context, id int64) (*TenantLicense, error)
}

// Service evaluates license state for enforcement middleware.
type Service struct {
    getter TenantGetter
}

func NewService(getter TenantGetter) *Service {
    return &Service{getter: getter}
}

// Enforce checks whether the tenant is allowed to proceed.
// Returns nil if allowed, ErrSuspended/ErrExpired if blocked.
// Auto-expires trials whose trial_ends_at has passed.
func (s *Service) Enforce(ctx context.Context, tenantID int64) error {
    tl, err := s.getter.GetLicense(ctx, tenantID)
    if err != nil {
        return err // propagate DB errors; middleware maps to 500
    }
    switch tl.Status {
    case "suspended":
        return ErrSuspended
    case "expired":
        return ErrExpired
    case "trial":
        if tl.TrialEndsAt != nil && tl.TrialEndsAt.Before(time.Now()) {
            // Auto-expire: the trial window has passed.
            // The UPDATE is best-effort; if it fails, the next
            // request will re-detect and retry.
            return ErrExpired
        }
    }
    return nil
}

// Info returns the license fields for the own-tenant endpoint.
func (s *Service) Info(ctx context.Context, tenantID int64) (*TenantLicense, error) {
    return s.getter.GetLicense(ctx, tenantID)
}
```

#### `backend/internal/api/middleware_license.go`

```go
package api

import (
    "errors"
    "log/slog"
    "net/http"

    "github.com/telegram-manager/backend/internal/license"
)

// requireLicense checks the tenant's license status after requireAuth
// has set claims in context. Exempt routes (auth/*, /tenants/me/license)
// must NOT be wrapped with this middleware.
func (s *Server) requireLicense(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        tenantID, ok := tenantIDFromClaims(w, r)
        if !ok {
            return // already responded 401
        }

        if err := s.licenseSvc.Enforce(r.Context(), tenantID); err != nil {
            switch {
            case errors.Is(err, license.ErrSuspended):
                respondError(w, http.StatusForbidden,
                    "LICENSE_SUSPENDED",
                    "Tu licencia esta suspendida. Contacta al administrador.")
            case errors.Is(err, license.ErrExpired):
                respondError(w, http.StatusForbidden,
                    "LICENSE_EXPIRED",
                    "Tu licencia ha expirado. Contacta al administrador.")
            default:
                slog.Error("license: enforce error",
                    "tenant_id", tenantID, "error", err)
                respondError(w, http.StatusInternalServerError,
                    "INTERNAL_ERROR",
                    "no se pudo verificar la licencia")
            }
            return
        }
        next(w, r)
    }
}
```

#### `backend/internal/api/admin_handlers.go`

Super-admin CRUD handlers for the admin panel.

```go
package api

import (
    "encoding/json"
    "errors"
    "log/slog"
    "net/http"
    "strconv"

    "github.com/telegram-manager/backend/internal/tenants"
)

// --- Interfaces (minimal views consumed here) ---

type adminTenantsLister interface {
    ListAll(ctx context.Context) ([]tenants.Tenant, error)
}

type adminTenantsGetter interface {
    GetByID(ctx context.Context, id int64) (*tenants.Tenant, error)
}

type adminTenantsUpdater interface {
    UpdateLicense(ctx context.Context, id int64, in tenants.LicenseUpdate) error
}

// --- Request / Response types ---

type updateTenantLicenseRequest struct {
    Status         *string `json:"status"`
    Plan           *string `json:"plan"`
    TrialEndsAt    *string `json:"trial_ends_at"`    // RFC3339 or null
    ExpiresAt      *string `json:"expires_at"`        // RFC3339 or null
    MaxGroups      *int    `json:"max_groups"`
    MaxMessagesDay *int    `json:"max_messages_day"`
}

type tenantListItem struct {
    ID              int64   `json:"id"`
    Slug            string  `json:"slug"`
    Plan            string  `json:"plan"`
    Status          string  `json:"status"`
    TrialEndsAt     *string `json:"trial_ends_at"`
    ExpiresAt       *string `json:"expires_at"`
    MaxGroups       int     `json:"max_groups"`
    MaxMessagesDay  int     `json:"max_messages_day"`
    BotUsername      *string `json:"bot_username"`
    CreatedAt       string  `json:"created_at"`
}

// --- Handlers ---

// GET /api/admin/tenants — list all tenants (super-admin only)
func (s *Server) handleAdminListTenants(w http.ResponseWriter, r *http.Request) {
    // ... calls s.adminTenantsRepo.ListAll(), maps to []tenantListItem
}

// GET /api/admin/tenants/{id} — tenant detail (super-admin only)
func (s *Server) handleAdminGetTenant(w http.ResponseWriter, r *http.Request) {
    // ... calls s.adminTenantsRepo.GetByID(), maps to detail response
}

// PUT /api/admin/tenants/{id} — update license fields (super-admin only)
func (s *Server) handleAdminUpdateTenant(w http.ResponseWriter, r *http.Request) {
    // ... validates body, calls s.adminTenantsRepo.UpdateLicense()
    // validates status transitions, returns 200 or error
}
```

**Status transition validation** (inside `handleAdminUpdateTenant`):

| Current | Allowed next |
|---------|-------------|
| `trial` | `active`, `suspended`, `expired` |
| `active` | `suspended` |
| `suspended` | `active` |
| `expired` | `active` |

Direct `expired → active` is allowed (reactivation after payment). All other transitions return 400 `INVALID_TRANSITION`.

### 3.2 Modified files

#### `backend/internal/auth/admin.go`

```go
type Admin struct {
    ID            int64
    Username      string
    PasswordHash  string
    TenantID      int64
    IsSuperAdmin  bool       // NEW
    CreatedAt     time.Time
    LastLoginAt   *time.Time
}
```

#### `backend/internal/auth/tokens.go`

```go
type Claims struct {
    Username     string `json:"username"`
    TenantID     int64  `json:"tenant_id,omitempty"`
    TenantSlug   string `json:"tenant_slug,omitempty"`   // NEW
    IsSuperAdmin bool   `json:"is_super_admin,omitempty"` // NEW
    jwt.RegisteredClaims
}
```

`IssueAccess` / `IssueRefresh` updated to populate `IsSuperAdmin` and `TenantSlug` from the Admin struct. TenantSlug passed as a parameter (repo lookup at login time) or via a `LoginResult` extension.

#### `backend/internal/auth/repository.go`

`get()` scan updated to include `is_super_admin`:

```go
func (r *Repository) get(ctx context.Context, where string, arg any) (Admin, error) {
    const base = `
SELECT id, username, password_hash, tenant_id, is_super_admin, created_at, last_login_at
FROM admins
WHERE `
    // ... scan includes &a.IsSuperAdmin
}
```

New method for the super-admin seed query (used by migration verification, not by runtime):

```go
// HasSuperAdmin returns true if at least one admin has is_super_admin = true.
func (r *Repository) HasSuperAdmin(ctx context.Context) (bool, error) {
    const q = `SELECT EXISTS(SELECT 1 FROM admins WHERE is_super_admin = true)`
    var exists bool
    if err := r.db.QueryRowContext(ctx, q).Scan(&exists); err != nil {
        return false, fmt.Errorf("auth: has super admin: %w", err)
    }
    return exists, nil
}
```

#### `backend/internal/auth/service.go`

`Login` must now also return `IsSuperAdmin` and `TenantSlug` so `tokens.IssueAccess` can embed them. Options:

- **Option A:** Add `TenantSlug` to `Admin` struct (requires JOIN in repo query).
- **Option B:** Look up slug in `Login` after getting the admin.
- **Option C:** Pass slug separately to `IssueAccess`.

**Chosen: Option A.** Add `TenantSlug` to `Admin`. The repo query becomes a JOIN:

```sql
SELECT a.id, a.username, a.password_hash, a.tenant_id, a.is_super_admin,
       a.created_at, a.last_login_at, t.slug
FROM admins a
JOIN tenants t ON t.id = a.tenant_id
WHERE a.username = $1
```

This is a single query (no N+1), and `Admin` already has `TenantID`. The slug is cached in the JWT for 15 min, so this lookup happens once per login/refresh.

#### `backend/internal/tenants/model.go`

```go
type Tenant struct {
    ID                int64
    Slug              string
    Tier              string
    BotTokenEncrypted []byte
    BotUsername       *string
    Status            string
    TrialEndsAt       *time.Time  // NEW
    ExpiresAt         *time.Time  // NEW
    Plan              string      // NEW
    MaxGroups         int         // NEW
    MaxMessagesDay    int         // NEW
    CreatedAt         time.Time
    UpdatedAt         time.Time
}

// Updated status constants (replaces old active/degraded).
const (
    StatusTrial     = "trial"
    StatusActive    = "active"
    StatusSuspended = "suspended"
    StatusExpired   = "expired"
)
```

Old constants `StatusActive` / `StatusDegraded` removed. All references updated.

#### `backend/internal/tenants/repository.go`

`scanTenant` updated to scan all new columns:

```go
func scanTenant(row rowScanner) (Tenant, error) {
    var (
        t              Tenant
        enc            []byte
        username       sql.NullString
        trialEndsAt    sql.NullTime
        expiresAt      sql.NullTime
        createdAt      sql.NullTime
        updatedAt      sql.NullTime
    )
    err := row.Scan(
        &t.ID, &t.Slug, &t.Tier, &enc, &username, &t.Status,
        &trialEndsAt, &expiresAt, &t.Plan, &t.MaxGroups, &t.MaxMessagesDay,
        &createdAt, &updatedAt,
    )
    // ... null handling for trialEndsAt, expiresAt, etc.
}
```

All existing SELECT queries (`GetByID`, `GetBySlug`, `ListWithTokens`) updated to include the new columns in their SELECT list.

**New methods:**

```go
// ListAll returns every tenant (super-admin panel). Ordered by id.
func (r *Repository) ListAll(ctx context.Context) ([]Tenant, error) {
    const q = `
SELECT id, slug, tier, bot_token_encrypted, bot_username, status,
       trial_ends_at, expires_at, plan, max_groups, max_messages_day,
       created_at, updated_at
FROM tenants
ORDER BY id ASC`
    // ... same scan pattern as ListWithTokens
}

// GetLicense returns only the license-relevant fields for enforcement.
// Lighter than GetByID — no bot_token_encrypted scan.
func (r *Repository) GetLicense(ctx context.Context, id int64) (*license.TenantLicense, error) {
    const q = `
SELECT id, status, trial_ends_at, expires_at
FROM tenants WHERE id = $1`
    // ... scan into license.TenantLicense
}

// LicenseUpdate is the set of fields a super-admin can modify.
type LicenseUpdate struct {
    Status         *string
    Plan           *string
    TrialEndsAt    *time.Time
    ExpiresAt      *time.Time
    MaxGroups      *int
    MaxMessagesDay *int
}

// UpdateLicense patches the license fields of a tenant.
func (r *Repository) UpdateLicense(ctx context.Context, id int64, in LicenseUpdate) error {
    // Dynamic SET clause built from non-nil pointers.
    // Updated_at always set.
}
```

#### `backend/internal/api/server.go`

New field on Server:

```go
type Server struct {
    // ... existing fields ...

    // License enforcement (license-system change).
    licenseSvc *license.Service

    // Super-admin panel (license-system change).
    adminTenantsRepo   adminTenantsLister    // *tenants.Repository
    adminTenantsGetter adminTenantsGetter    // *tenants.Repository
    adminTenantsUpdater adminTenantsUpdater  // *tenants.Repository
}
```

New option functions:

```go
// WithLicense mounts the license enforcement middleware on all
// protected routes except auth/* and /tenants/me/license.
func WithLicense(svc *license.Service) Option {
    return func(s *Server) {
        s.licenseSvc = svc
    }
}

// WithAdmin mounts super-admin routes (requireAuth + requireSuperAdmin).
func WithAdmin(
    lister adminTenantsLister,
    getter adminTenantsGetter,
    updater adminTenantsUpdater,
) Option {
    return func(s *Server) {
        s.adminTenantsRepo = lister
        s.adminTenantsGetter = getter
        s.adminTenantsUpdater = updater
        adminRoutes := s.requireSuperAdmin
        s.mux.HandleFunc("GET /api/admin/tenants", adminRoutes(s.handleAdminListTenants))
        s.mux.HandleFunc("GET /api/admin/tenants/{id}", adminRoutes(s.handleAdminGetTenant))
        s.mux.HandleFunc("PUT /api/admin/tenants/{id}", adminRoutes(s.handleAdminUpdateTenant))
    }
}
```

**Route registration changes:** Existing `WithTenants` and `WithGroups` etc. now wrap their handlers with `requireLicense` where applicable:

```go
// Inside WithGroups:
s.mux.HandleFunc("GET /api/groups", s.requireAuth(s.requireLicense(s.handleListGroups)))
s.mux.HandleFunc("GET /api/groups/{id}", s.requireAuth(s.requireLicense(s.handleGetGroup)))
// etc.

// Inside WithTenants — /tenants/me gets license check, /tenants/me/license does NOT:
s.mux.HandleFunc("GET /api/tenants/me", s.requireAuth(s.requireLicense(s.handleGetTenantMe)))
s.mux.HandleFunc("GET /api/tenants/me/license", s.requireAuth(s.handleGetTenantLicense)) // exempt
s.mux.HandleFunc("PUT /api/tenants/me/bot-token", s.requireAuth(s.requireLicense(s.handleRotateBotToken)))
s.mux.HandleFunc("GET /api/tenants/me/status", s.requireAuth(s.requireLicense(s.handleGetTenantStatus)))
```

#### `backend/internal/api/middleware_auth.go`

Add `requireSuperAdmin`:

```go
// requireSuperAdmin checks that the authenticated admin has is_super_admin = true.
// Must be chained AFTER requireAuth (which sets claims in context).
func (s *Server) requireSuperAdmin(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        claims := claimsFromContext(r.Context())
        if claims == nil || !claims.IsSuperAdmin {
            respondError(w, http.StatusForbidden, "FORBIDDEN",
                "se requieren permisos de super-administrador")
            return
        }
        next(w, r)
    }
}
```

#### `backend/internal/api/auth_handlers.go`

`handleMe` adds `is_super_admin` to the response:

```go
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
    claims := claimsFromContext(r.Context())
    if claims == nil {
        respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "no autenticado")
        return
    }
    resp := map[string]any{
        "id":             claims.Subject,
        "username":       claims.Username,
        "tenant_id":      claims.TenantID,
        "is_super_admin": claims.IsSuperAdmin, // NEW
    }
    // tenant_slug from claims (no DB lookup needed anymore)
    if claims.TenantSlug != "" {
        resp["tenant_slug"] = claims.TenantSlug
    }
    // Fallback: if token is legacy (no TenantSlug), look up from DB
    if _, ok := resp["tenant_slug"]; !ok && s.tenantsRepo != nil {
        if t, err := s.tenantsRepo.GetByID(r.Context(), claims.TenantID); err == nil {
            resp["tenant_slug"] = t.Slug
        }
    }
    respond(w, http.StatusOK, resp)
}
```

#### `backend/internal/api/tenant_handlers.go`

`handleGetTenantMe` adds license fields to the response:

```go
func (s *Server) handleGetTenantMe(w http.ResponseWriter, r *http.Request) {
    tenantID, ok := tenantIDFromClaims(w, r)
    if !ok {
        return
    }

    t, err := s.tenantsRepo.GetByID(r.Context(), tenantID)
    // ... existing error handling ...

    // bot_status from registry (existing logic)
    botStatus := "unknown"
    // ... existing bot_status logic ...

    // License fields
    respond(w, http.StatusOK, map[string]any{
        "slug":              t.Slug,
        "bot_username":      t.BotUsername,
        "bot_status":        botStatus,
        "created_at":        t.CreatedAt,
        "license_status":    t.Status,        // NEW
        "plan":              t.Plan,           // NEW
        "trial_ends_at":     t.TrialEndsAt,   // NEW
        "expires_at":        t.ExpiresAt,      // NEW
        "max_groups":        t.MaxGroups,      // NEW
        "max_messages_day":  t.MaxMessagesDay, // NEW
    })
}

// New handler: GET /api/tenants/me/license (exempt from license enforcement)
func (s *Server) handleGetTenantLicense(w http.ResponseWriter, r *http.Request) {
    tenantID, ok := tenantIDFromClaims(w, r)
    if !ok {
        return
    }

    t, err := s.tenantsRepo.GetByID(r.Context(), tenantID)
    // ... error handling ...

    respond(w, http.StatusOK, map[string]any{
        "license_status":    t.Status,
        "plan":              t.Plan,
        "trial_ends_at":     t.TrialEndsAt,
        "expires_at":        t.ExpiresAt,
        "max_groups":        t.MaxGroups,
        "max_messages_day":  t.MaxMessagesDay,
    })
}
```

---

## 4. API Contracts

### 4.1 New endpoints

#### `GET /api/admin/tenants`

Super-admin lists all tenants.

**Auth:** `requireAuth` + `requireSuperAdmin`

**Response 200:**
```json
{
  "data": [
    {
      "id": 1,
      "slug": "mi-comunidad",
      "plan": "pro",
      "status": "active",
      "trial_ends_at": null,
      "expires_at": "2027-01-01T00:00:00Z",
      "max_groups": -1,
      "max_messages_day": -1,
      "bot_username": "my_bot",
      "created_at": "2026-09-05T12:00:00Z"
    }
  ],
  "error": null
}
```

**Response 403 (non-super-admin):**
```json
{
  "data": null,
  "error": {
    "code": "FORBIDDEN",
    "message": "se requieren permisos de super-administrador"
  }
}
```

#### `GET /api/admin/tenants/{id}`

Super-admin gets tenant detail.

**Response 200:**
```json
{
  "data": {
    "id": 1,
    "slug": "mi-comunidad",
    "plan": "pro",
    "status": "trial",
    "trial_ends_at": "2026-09-13T12:00:00Z",
    "expires_at": null,
    "max_groups": -1,
    "max_messages_day": -1,
    "bot_username": "my_bot",
    "created_at": "2026-09-05T12:00:00Z"
  },
  "error": null
}
```

**Response 404:**
```json
{
  "data": null,
  "error": {
    "code": "NOT_FOUND",
    "message": "tenant no encontrado"
  }
}
```

#### `PUT /api/admin/tenants/{id}`

Super-admin updates license fields. All fields optional (partial update).

**Request body:**
```json
{
  "status": "active",
  "plan": "pro",
  "trial_ends_at": null,
  "expires_at": "2027-01-01T00:00:00Z",
  "max_groups": 50,
  "max_messages_day": 1000
}
```

Any field omitted (or `null`) is not modified. Sending `"trial_ends_at": "2026-12-01T00:00:00Z"` extends the trial.

**Response 200:**
```json
{
  "data": { "status": "updated" },
  "error": null
}
```

**Response 400 (invalid transition):**
```json
{
  "data": null,
  "error": {
    "code": "INVALID_TRANSITION",
    "message": "no se puede cambiar de 'expired' a 'trial'"
  }
}
```

### 4.2 Modified endpoints

#### `GET /api/auth/me`

Now includes `is_super_admin`.

**Response 200:**
```json
{
  "data": {
    "id": "42",
    "username": "admin1",
    "tenant_id": 1,
    "tenant_slug": "mi-comunidad",
    "is_super_admin": true
  },
  "error": null
}
```

#### `GET /api/tenants/me`

Now includes license fields.

**Response 200:**
```json
{
  "data": {
    "slug": "mi-comunidad",
    "bot_username": "my_bot",
    "bot_status": "connected",
    "created_at": "2026-09-05T12:00:00Z",
    "license_status": "trial",
    "plan": "pro",
    "trial_ends_at": "2026-09-13T12:00:00Z",
    "expires_at": null,
    "max_groups": -1,
    "max_messages_day": -1
  },
  "error": null
}
```

### 4.3 New endpoint (exempt from license check)

#### `GET /api/tenants/me/license`

Returns license data for the caller's tenant. Exempt from `requireLicense` so suspended/expired tenants can read their own status.

**Response 200:**
```json
{
  "data": {
    "license_status": "suspended",
    "plan": "pro",
    "trial_ends_at": null,
    "expires_at": "2026-12-01T00:00:00Z",
    "max_groups": 50,
    "max_messages_day": 1000
  },
  "error": null
}
```

### 4.4 New error codes

| Code | HTTP | When |
|------|------|------|
| `LICENSE_SUSPENDED` | 403 | Tenant with `status = 'suspended'` hits a protected route |
| `LICENSE_EXPIRED` | 403 | Tenant with `status = 'expired'` or trial past `trial_ends_at` |
| `FORBIDDEN` | 403 | Non-super-admin hits `/api/admin/*` |
| `INVALID_TRANSITION` | 400 | Invalid status change in `PUT /api/admin/tenants/{id}` |

---

## 5. Frontend Components

### 5.1 Modified files

#### `frontend/src/features/auth/types.ts`

```typescript
/** Respuesta de GET /api/auth/me. */
export interface MeResponse {
  id: string
  username: string
  tenant_id: number
  tenant_slug?: string
  is_super_admin: boolean  // NEW
}
```

#### `frontend/src/lib/auth-context.tsx`

```typescript
export interface SessionUser {
  id: string
  username: string
  tenantId: number | null
  tenantSlug: string | null
  isSuperAdmin: boolean  // NEW
}
```

Both `doLogin` and session restore (`resumir`) map `me.is_super_admin` into `isSuperAdmin`:

```typescript
setUser({
  id: me.id,
  username: me.username,
  tenantId: me.tenant_id,
  tenantSlug: me.tenant_slug ?? null,
  isSuperAdmin: me.is_super_admin ?? false,  // backward compat
})
```

#### `frontend/src/features/tenant/types.ts`

```typescript
export interface TenantMe {
  slug: string
  bot_username: string | null
  bot_status: string
  created_at: string
  // License fields (from GET /api/tenants/me)
  license_status: string
  plan: string
  trial_ends_at: string | null
  expires_at: string | null
  max_groups: number
  max_messages_day: number
}
```

#### `frontend/src/components/Layout.tsx`

NAV_ITEMS computed conditionally inside the component:

```tsx
export default function Layout() {
  const { user, logout } = useAuth()
  // ...

  const navItems: NavItem[] = [
    { to: '/dashboard', label: 'Dashboard', icon: IconLayoutDashboard },
    { to: '/groups', label: 'Grupos', icon: IconUsersGroup },
    { to: '/publications', label: 'Publicaciones', icon: IconSend },
    { to: '/tenant', label: 'Configuracion', icon: IconSettings },
    // Conditional admin panel link
    ...(user?.isSuperAdmin
      ? [{ to: '/admin/tenants', label: 'Admin Panel', icon: IconShield } as NavItem]
      : []),
  ]

  return (
    // ... existing AppShell, but using navItems instead of NAV_ITEMS
  )
}
```

New icon import: `IconShield` from `@tabler/icons-react`.

#### `frontend/src/lib/api-client.ts`

Add new error codes to `ApiErrorCode`:

```typescript
export type ApiErrorCode =
  | 'SUCCESS'
  | 'PERMISSION_DENIED'
  | 'TELEGRAM_ERROR'
  | 'VALIDATION_ERROR'
  | 'CONFLICT'
  | 'NOT_FOUND'
  | 'INTERNAL_ERROR'
  | 'UNAUTHORIZED'
  | 'INVALID_CREDENTIALS'
  | 'LICENSE_SUSPENDED'   // NEW
  | 'LICENSE_EXPIRED'     // NEW
  | 'FORBIDDEN'           // NEW
  | 'INVALID_TRANSITION'  // NEW
```

#### `frontend/src/App.tsx`

Add route for admin panel (inside the authenticated block):

```tsx
import AdminTenantsPage from './pages/AdminTenantsPage'

// Inside the authenticated <Route> block:
<Route path="/admin/tenants" element={<AdminTenantsPage />} />
```

### 5.2 New files

#### `frontend/src/features/admin/types.ts`

```typescript
/** Tenant in the admin panel list. */
export interface AdminTenant {
  id: number
  slug: string
  plan: string
  status: string
  trial_ends_at: string | null
  expires_at: string | null
  max_groups: number
  max_messages_day: number
  bot_username: string | null
  created_at: string
}

/** Body for PUT /api/admin/tenants/:id */
export interface UpdateTenantLicense {
  status?: string | null
  plan?: string | null
  trial_ends_at?: string | null
  expires_at?: string | null
  max_groups?: number | null
  max_messages_day?: number | null
}
```

#### `frontend/src/features/admin/api.ts`

```typescript
import { request } from '../../lib/api-client'
import type { AdminTenant, UpdateTenantLicense } from './types'

/** GET /api/admin/tenants — list all tenants (super-admin). */
export function adminListTenants(): Promise<AdminTenant[]> {
  return request<AdminTenant[]>('/api/admin/tenants')
}

/** GET /api/admin/tenants/:id — tenant detail. */
export function adminGetTenant(id: number): Promise<AdminTenant> {
  return request<AdminTenant>(`/api/admin/tenants/${id}`)
}

/** PUT /api/admin/tenants/:id — update license fields. */
export function adminUpdateTenant(id: number, body: UpdateTenantLicense): Promise<{ status: string }> {
  return request<{ status: string }>(`/api/admin/tenants/${id}`, {
    method: 'PUT',
    body: JSON.stringify(body),
  })
}
```

#### `frontend/src/features/tenant/api.ts` (modification)

Add the license endpoint:

```typescript
/** License info for the own tenant. */
export interface LicenseInfo {
  license_status: string
  plan: string
  trial_ends_at: string | null
  expires_at: string | null
  max_groups: number
  max_messages_day: number
}

/** GET /api/tenants/me/license */
export function getTenantLicense(): Promise<LicenseInfo> {
  return request<LicenseInfo>('/api/tenants/me/license')
}
```

#### `frontend/src/components/LicenseCard.tsx`

Displays the tenant's license status. Used inside `TenantSettingsPage`.

```tsx
import { Paper, Title, Text, Stack, Group, Badge } from '@mantine/core'
import type { TenantMe } from '../features/tenant/types'

interface LicenseCardProps {
  tenant: TenantMe
}

function statusColor(status: string): string {
  switch (status) {
    case 'active': return 'green'
    case 'trial': return 'blue'
    case 'suspended': return 'red'
    case 'expired': return 'gray'
    default: return 'dimmed'
  }
}

function statusLabel(status: string): string {
  switch (status) {
    case 'active': return 'Activa'
    case 'trial': return 'Prueba'
    case 'suspended': return 'Suspendida'
    case 'expired': return 'Expirada'
    default: return status
  }
}

export default function LicenseCard({ tenant }: LicenseCardProps) {
  const trialRemaining = tenant.trial_ends_at
    ? Math.max(0, Math.ceil(
        (new Date(tenant.trial_ends_at).getTime() - Date.now()) / (1000 * 60 * 60 * 24)
      ))
    : null

  return (
    <Paper p="md" withBorder>
      <Title order={4} mb="md">Licencia</Title>
      <Stack gap="xs">
        <Group>
          <Text fw={600}>Estado:</Text>
          <Badge color={statusColor(tenant.license_status)}>
            {statusLabel(tenant.license_status)}
          </Badge>
        </Group>
        <Group>
          <Text fw={600}>Plan:</Text>
          <Text>{tenant.plan}</Text>
        </Group>
        {tenant.license_status === 'trial' && trialRemaining !== null && (
          <Group>
            <Text fw={600}>Dias restantes:</Text>
            <Text>{trialRemaining}</Text>
          </Group>
        )}
        {tenant.trial_ends_at && (
          <Group>
            <Text fw={600}>Trial hasta:</Text>
            <Text>{new Date(tenant.trial_ends_at).toLocaleDateString('es-AR')}</Text>
          </Group>
        )}
        {tenant.expires_at && (
          <Group>
            <Text fw={600}>Expira:</Text>
            <Text>{new Date(tenant.expires_at).toLocaleDateString('es-AR')}</Text>
          </Group>
        )}
        <Group>
          <Text fw={600}>Max grupos:</Text>
          <Text>{tenant.max_groups === -1 ? 'Sin limite' : tenant.max_groups}</Text>
        </Group>
        <Group>
          <Text fw={600}>Max mensajes/dia:</Text>
          <Text>{tenant.max_messages_day === -1 ? 'Sin limite' : tenant.max_messages_day}</Text>
        </Group>
      </Stack>
    </Paper>
  )
}
```

#### `frontend/src/components/LicenseErrorBanner.tsx`

Shown when a 403 `LICENSE_SUSPENDED` or `LICENSE_EXPIRED` is received. Placed in Layout (like DegradedBanner).

```tsx
import { Alert } from '@mantine/core'
import { IconLock } from '@tabler/icons-react'

interface LicenseErrorBannerProps {
  code: 'LICENSE_SUSPENDED' | 'LICENSE_EXPIRED'
}

export default function LicenseErrorBanner({ code }: LicenseErrorBannerProps) {
  return (
    <Alert
      variant="light"
      color="red"
      icon={<IconLock size={16} />}
      mb="md"
      data-testid="license-error-banner"
    >
      {code === 'LICENSE_SUSPENDED'
        ? 'Tu licencia esta suspendida. Contacta al administrador para reactivarla.'
        : 'Tu licencia ha expirado. Contacta al administrador para continuar.'}
    </Alert>
  )
}
```

#### `frontend/src/pages/AdminTenantsPage.tsx`

Super-admin panel page. Mantine Table with status filter.

```tsx
import { useEffect, useState } from 'react'
import {
  Paper, Title, Text, Table, Select, Group, Badge, Loader,
} from '@mantine/core'
import { adminListTenants } from '../features/admin/api'
import type { AdminTenant } from '../features/admin/types'
import { ApiError } from '../lib/api-client'

const STATUS_OPTIONS = [
  { value: '', label: 'Todos' },
  { value: 'trial', label: 'Prueba' },
  { value: 'active', label: 'Activa' },
  { value: 'suspended', label: 'Suspendida' },
  { value: 'expired', label: 'Expirada' },
]

function statusColor(status: string): string {
  switch (status) {
    case 'active': return 'green'
    case 'trial': return 'blue'
    case 'suspended': return 'red'
    case 'expired': return 'gray'
    default: return 'dimmed'
  }
}

export default function AdminTenantsPage() {
  const [tenants, setTenants] = useState<AdminTenant[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [filter, setFilter] = useState('')

  useEffect(() => {
    let cancelled = false
    const load = async () => {
      try {
        const data = await adminListTenants()
        if (!cancelled) setTenants(data)
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof ApiError ? err.message : 'Error al cargar tenants')
        }
      } finally {
        if (!cancelled) setLoading(false)
      }
    }
    void load()
    return () => { cancelled = true }
  }, [])

  const filtered = filter
    ? tenants.filter((t) => t.status === filter)
    : tenants

  if (loading) return <Loader />
  if (error) return <Text c="red">{error}</Text>

  return (
    <Stack gap="lg">
      <Group justify="space-between">
        <Title order={3}>Admin Panel — Tenants</Title>
        <Select
          data={STATUS_OPTIONS}
          value={filter}
          onChange={(v) => setFilter(v ?? '')}
          placeholder="Filtrar por estado"
          clearable
          w={200}
        />
      </Group>

      <Paper withBorder>
        <Table striped highlightOnHover>
          <Table.Thead>
            <Table.Tr>
              <Table.Th>Slug</Table.Th>
              <Table.Th>Plan</Table.Th>
              <Table.Th>Estado</Table.Th>
              <Table.Th>Trial hasta</Table.Th>
              <Table.Th>Expira</Table.Th>
              <Table.Th>Bot</Table.Th>
            </Table.Tr>
          </Table.Thead>
          <Table.Tbody>
            {filtered.map((t) => (
              <Table.Tr key={t.id}>
                <Table.Td>{t.slug}</Table.Td>
                <Table.Td>{t.plan}</Table.Td>
                <Table.Td>
                  <Badge color={statusColor(t.status)}>{t.status}</Badge>
                </Table.Td>
                <Table.Td>
                  {t.trial_ends_at
                    ? new Date(t.trial_ends_at).toLocaleDateString('es-AR')
                    : '-'}
                </Table.Td>
                <Table.Td>
                  {t.expires_at
                    ? new Date(t.expires_at).toLocaleDateString('es-AR')
                    : '-'}
                </Table.Td>
                <Table.Td>{t.bot_username ?? '-'}</Table.Td>
              </Table.Tr>
            ))}
          </Table.Tbody>
        </Table>
      </Paper>
    </Stack>
  )
}
```

### 5.3 Component hierarchy

```
App.tsx
├── /login (PublicOnly)
├── /signup (PublicOnly)
└── <RequireAuth><Layout /></RequireAuth>
    ├── AppShell.Header (user.username, user.tenantSlug)
    ├── AppShell.Navbar
    │   ├── NavLink → /dashboard
    │   ├── NavLink → /groups
    │   ├── NavLink → /publications
    │   ├── NavLink → /tenant
    │   └── NavLink → /admin/tenants  ← conditional: user.isSuperAdmin
    └── AppShell.Main
        ├── DegradedBanner (existing)
        ├── LicenseErrorBanner (NEW — shown when license 403 is caught)
        └── <Outlet>
            ├── /tenant → TenantSettingsPage
            │   ├── LicenseCard (NEW)
            │   └── RotateTokenForm (existing)
            └── /admin/tenants → AdminTenantsPage (NEW)
```

### 5.4 Data flow

```
Login
  → POST /api/auth/login → access_token
  → GET /api/auth/me → { id, username, tenant_id, tenant_slug, is_super_admin }
  → AuthContext.setSessionUser({ ...isSuperAdmin })
  → Layout renders conditional nav item

Every protected request
  → api-client adds Bearer token
  → requireAuth validates JWT → sets Claims{IsSuperAdmin, TenantSlug, ...}
  → requireLicense → SELECT status, trial_ends_at FROM tenants WHERE id = $1
    → 'active'/'trial'(valid) → proceed
    → 'suspended' → 403 LICENSE_SUSPENDED
    → 'expired'/trial-past → 403 LICENSE_EXPIRED
  → handler executes
```

---

## 6. Implementation Order

Ordered by dependency. Each step is a single commit.

### Step 1: Migration 00010

Create `backend/migrations/00010_license_system.sql`. Run `goose up` to verify. Run `goose down` to verify rollback. No code changes yet — the new columns have defaults so existing queries won't break (they just don't scan the new columns).

### Step 2: Tenant model + repository

Modify `backend/internal/tenants/model.go`:
- Add `TrialEndsAt`, `ExpiresAt`, `Plan`, `MaxGroups`, `MaxMessagesDay` to `Tenant` struct.
- Replace `StatusActive`/`StatusDegraded` constants with `StatusTrial`/`StatusActive`/`StatusSuspended`/`StatusExpired`.
- Add `TenantLicense` struct (or import from license package).
- Add `LicenseUpdate` struct.

Modify `backend/internal/tenants/repository.go`:
- Update `scanTenant` to scan all new columns.
- Update all SELECT queries (`GetByID`, `GetBySlug`, `ListWithTokens`) to include new columns.
- Add `ListAll` method.
- Add `GetLicense` method.
- Add `UpdateLicense` method.

Update all references to old status constants across the codebase (e.g., `tenants.StatusDegraded` references in registry/poller code).

### Step 3: Auth model + tokens + repository

Modify `backend/internal/auth/admin.go`:
- Add `IsSuperAdmin bool` to `Admin` struct.
- Add `TenantSlug string` to `Admin` struct.

Modify `backend/internal/auth/tokens.go`:
- Add `TenantSlug string` and `IsSuperAdmin bool` to `Claims` struct.
- Update `issue()` to populate new fields from Admin.

Modify `backend/internal/auth/repository.go`:
- Update `get()` to scan `is_super_admin`.
- Update SELECT to JOIN tenants for `slug` (needed for claims).
- Add `HasSuperAdmin()` method.

Modify `backend/internal/auth/service.go`:
- Ensure `Login` returns admin with `IsSuperAdmin` and `TenantSlug` populated (via updated repo query).

### Step 4: Auth handlers — `/me` update

Modify `backend/internal/api/auth_handlers.go`:
- `handleMe` adds `is_super_admin` to response from `claims.IsSuperAdmin`.
- `handleMe` uses `claims.TenantSlug` (fallback to DB lookup for legacy tokens).

### Step 5: Tenant handlers — license data

Modify `backend/internal/api/tenant_handlers.go`:
- `handleGetTenantMe` adds `license_status`, `plan`, `trial_ends_at`, `expires_at`, `max_groups`, `max_messages_day` to response.
- Add `handleGetTenantLicense` for `GET /api/tenants/me/license`.

### Step 6: License service

Create `backend/internal/license/service.go`:
- `TenantGetter` interface, `Service` struct, `Enforce()` method, `Info()` method.

### Step 7: License enforcement middleware

Create `backend/internal/api/middleware_license.go`:
- `requireLicense` method on Server.

Modify `backend/internal/api/server.go`:
- Add `licenseSvc` field.
- Add `WithLicense` option.
- Wrap existing route handlers with `requireLicense` (except auth routes and `/tenants/me/license`).
- Register `GET /api/tenants/me/license` as exempt.

### Step 8: Super-admin middleware + admin handlers

Modify `backend/internal/api/middleware_auth.go`:
- Add `requireSuperAdmin` method.

Create `backend/internal/api/admin_handlers.go`:
- `handleAdminListTenants`, `handleAdminGetTenant`, `handleAdminUpdateTenant`.

Modify `backend/internal/api/server.go`:
- Add admin repo fields.
- Add `WithAdmin` option.
- Register admin routes under `requireAuth(requireSuperAdmin(...))`.

### Step 9: Wiring in main.go

Modify `cmd/server/main.go`:
- Create `license.NewService(tenantsRepo)`.
- Pass to `api.WithLicense(licenseSvc)`.
- Pass `tenantsRepo` to `api.WithAdmin(tenantsRepo, tenantsRepo, tenantsRepo)`.
- Verify startup compiles and migrations run.

### Step 10: Frontend — auth types + context

Modify `frontend/src/features/auth/types.ts`:
- Add `is_super_admin` to `MeResponse`.

Modify `frontend/src/lib/auth-context.tsx`:
- Add `isSuperAdmin` to `SessionUser`.
- Map `me.is_super_admin` in login and session restore.

Modify `frontend/src/lib/api-client.ts`:
- Add `LICENSE_SUSPENDED`, `LICENSE_EXPIRED`, `FORBIDDEN`, `INVALID_TRANSITION` to `ApiErrorCode`.

### Step 11: Frontend — tenant types + LicenseCard

Modify `frontend/src/features/tenant/types.ts`:
- Add license fields to `TenantMe`.

Create `frontend/src/components/LicenseCard.tsx`.

Modify `frontend/src/pages/TenantSettingsPage.tsx`:
- Import and render `<LicenseCard tenant={tenant} />` after the existing data section.

### Step 12: Frontend — admin panel

Create `frontend/src/features/admin/types.ts`.
Create `frontend/src/features/admin/api.ts`.
Create `frontend/src/pages/AdminTenantsPage.tsx`.

Modify `frontend/src/App.tsx`:
- Add route `/admin/tenants` → `<AdminTenantsPage />`.

### Step 13: Frontend — Layout conditional nav

Modify `frontend/src/components/Layout.tsx`:
- Make `NAV_ITEMS` conditional on `user?.isSuperAdmin`.
- Import `IconShield` from `@tabler/icons-react`.

### Step 14: Tests

Backend tests:
- `internal/license/service_test.go` — mock `TenantGetter`, test enforce logic (active, trial valid, trial expired, suspended, expired).
- `internal/api/admin_handlers_test.go` — test CRUD with super-admin mock, test 403 for non-super-admin.
- `internal/api/middleware_license_test.go` — test enforcement on protected routes.
- Update `internal/auth/tokens_test.go` — test `IsSuperAdmin` and `TenantSlug` in claims.

Frontend: manual verification of conditional nav and admin panel.

---

## Appendix: Dependency Graph

```
Step 1: Migration
  ↓
Step 2: Tenant model/repo ──────────────┐
  ↓                                      │
Step 3: Auth model/tokens/repo ──────────┤
  ↓                                      │
Step 4: Auth handlers (/me)              │
  ↓                                      │
Step 5: Tenant handlers (license data) ←─┘
  ↓
Step 6: License service (depends on tenant repo)
  ↓
Step 7: License middleware (depends on license service)
  ↓
Step 8: Super-admin middleware + admin handlers
  ↓
Step 9: Wiring in main.go
  ↓
Step 10-13: Frontend (independent of backend order, but needs backend running)
  ↓
Step 14: Tests
```

Steps 10-13 can be done in parallel with steps 6-9 if two developers are working. The frontend compiles against the API contract defined in this document; backend implementation can happen concurrently.
