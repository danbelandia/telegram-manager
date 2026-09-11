# Proposal: Sistema de Licencias y Super-Admin

## Intent

Dar al sistema capacidad de controlar el acceso de tenants según estado de pago/_trial, y agregar un super-admin que gestione todos los tenants desde un panel dedicado. Sin esto, el SaaS no puede monetizar ni controlar quién usa la plataforma.

## Scope

### In Scope
- Migration 00010: campos de licencia en `tenants` + `is_super_admin` en `admins`
- Seed del super-admin en la migración (un admin con `is_super_admin = true`)
- License service: lógica de trial (3 días), status transitions, enforcement
- Super-admin check en JWT claims + middleware
- `GET /api/admin/tenants` — lista todos los tenants (super-admin only)
- `GET /api/admin/tenants/:id` — detalle de un tenant
- `PUT /api/admin/tenants/:id` — editar status/plan/fechas/límites
- `GET /api/tenant/me/license` — info de licencia propia del tenant
- Admin panel page (`/admin/tenants`) con tabla de tenants
- Tenant license card en `/tenant` con estado, trial restante, plan
- Sidebar: item "Admin Panel" visible solo para super-admins
- Enforcement: middleware que rechaza 403 si status = suspended/expired

### Out of Scope
- Facturación real (pasarela de pago)
- Sistema de planes múltiples (solo "pro" por ahora)
- Email de notificación de expiración
- Dashboard de métricas globales (stats básicos vienen después)

## Capabilities

### New Capabilities
- `license-management`: Estado de licencia (trial/active/suspended/expired), trial de 3 días, enforcement de suspension, endpoints admin CRUD de tenants
- `super-admin`: Flag `is_super_admin` en admins, JWT claim, middleware de verificación, panel admin con tabla de tenants, sidebar condicional

### Modified Capabilities
- `tenant-settings`: GET /tenants/me ahora incluye datos de licencia (status, plan, trial_ends_at, expires_at, max_groups, max_messages_day)
- `auth`: Claims JWT incluyen `is_super_admin`, GET /me lo expone

## Approach

### Migration 00010
```
tenants: +trial_ends_at TIMESTAMPTZ, +expires_at TIMESTAMPTZ,
  +plan TEXT DEFAULT 'pro', +max_groups INT DEFAULT -1,
  +max_messages_day INT DEFAULT -1
admins: +is_super_admin BOOLEAN DEFAULT false
Seed: INSERT un admin con is_super_admin=true si no existe ninguno
```

### Backend
- Nuevo paquete `internal/license` con service que encapsula lógica de trial/expiry
- Nuevo paquete `internal/admin` con handlers para el panel de super-admin
- Middleware `requireSuperAdmin` que verifica `is_super_admin` en claims
- Extender `auth.Claims` con `IsSuperAdmin` y `GET /me` con `is_super_admin`
- Nuevo repo method `tenants.ListAll()` para el admin panel
- Nuevo repo method `tenants.UpdateLicense()` para admin edits

### Frontend
- `AdminTenantsPage` (`/admin/tenants`) — Mantine Table con filtros por status
- `LicenseCard` en TenantSettingsPage — muestra estado, plan, trial, límites
- Layout: NavLink condicional "Admin Panel" cuando `user.isSuperAdmin === true`
- auth-context: extender `SessionUser` con `isSuperAdmin`

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `backend/migrations/00010_license_system.sql` | New | Migration con campos + seed |
| `backend/internal/auth/admin.go` | Modified | Admin struct: +IsSuperAdmin |
| `backend/internal/auth/tokens.go` | Modified | Claims: +IsSuperAdmin |
| `backend/internal/auth/repository.go` | Modified | scanAdmin: +is_super_admin |
| `backend/internal/auth/service.go` | Modified | Login return + is_super_admin |
| `backend/internal/tenants/model.go` | Modified | Tenant struct: +license fields |
| `backend/internal/tenants/repository.go` | Modified | +ListAll, +UpdateLicense |
| `backend/internal/license/service.go` | New | License logic (trial, expiry, enforcement) |
| `backend/internal/api/admin_handlers.go` | New | Super-admin CRUD handlers |
| `backend/internal/api/server.go` | Modified | +WithAdmin routes |
| `backend/internal/api/middleware_auth.go` | Modified | +requireSuperAdmin |
| `frontend/src/pages/AdminTenantsPage.tsx` | New | Admin panel page |
| `frontend/src/components/Layout.tsx` | Modified | Conditional admin nav item |
| `frontend/src/lib/auth-context.tsx` | Modified | +isSuperAdmin in SessionUser |
| `frontend/src/features/auth/types.ts` | Modified | MeResponse + is_super_admin |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Seed duplicado si ya existe un admin | Low | Seed es idempotente (solo si is_super_admin=false en todos) |
| Trial expira y bloquea tenant activo | Medium | Enforcement es gradual: warning antes de bloquear |
| Claims JWT viejos sin is_super_admin | Low | Default false si claim ausente (backward compatible) |

## Rollback Plan

- Migration DOWN: elimina columnas nuevas, tabla admins vuelve a estado anterior
- Feature flags: si el admin panel causa problemas, deshabilitar NavLink (el backend funciona igual)
- Sin dependencias externas, rollback es seguro

## Dependencies

- Ninguna dependencia nueva. Extiende funcionalidad existente.

## Success Criteria

- [ ] Tenant nuevo tiene `trial_ends_at = now() + 3 días` y `status = 'trial'`
- [ ] Super-admin puede listar todos los tenants desde `/admin/tenants`
- [ ] Super-admin puede cambiar status, plan, fechas y límites de un tenant
- [ ] Tenant con `status = 'suspended'` recibe 403 en todas las rutas
- [ ] Trial expirado sin pago se auto-expira
- [ ] Tenant ve su estado de licencia en `/tenant`
- [ ] Sidebar muestra "Admin Panel" solo para super-admins
- [ ] GET /me incluye `is_super_admin`
