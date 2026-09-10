# Verify Report — tenant-settings (Slice 2)

> **Change**: `tenant-settings` (token rotation, settings page, degraded banner)
> **Branch**: `feat/tenant-settings` base `main @ 50afd88`
> **Date**: 2026-09-10
> **Delivered as**: single-PR `size:exception` (precedente 15/15)

## Verdict: **PASS**

- 0 CRITICAL, 0 WARNING
- 9/9 REQs con evidencia runtime
- D1–D6 conformes, 0 desviaciones

## Build / Test evidence

```
go build ./...        → limpio
go vet ./...          → limpio
go test ./... -short  → 14/14 paquetes OK
tsc --noEmit          → limpio
npm test -- --run     → 122/123 (1 flake pre-existente PublicationsPage)
npm run build         → OK
git diff main -- backend/ → solo cambios de tenant-settings
```

## REQ traceability (9 REQs)

| REQ | Evidence | Verdict |
|-----|----------|---------|
| tenant-me (GET /tenants/me) | `tenant_handlers.go:handleGetTenantMe` + test | ✅ |
| tenant-rotate-token (PUT) | `tenant_handlers.go:handleRotateBotToken` — bcrypt→getMe→encrypt→UpdateToken→RegisterHot, 6 tests | ✅ |
| tenant-status (GET) | `tenant_handlers.go:handleGetTenantStatus` — registry.Status | ✅ |
| auth-me-slug | `auth_handlers.go:handleMe` + tenant_slug via repo | ✅ |
| degraded-banner | `DegradedBanner.tsx` polling 60s, visible cuando status ≠ connected | ✅ |
| tenant-settings-page | `TenantSettingsPage.tsx` en /tenant, form zod, NavLink en Layout | ✅ |
| token-hygiene | token nunca en logs/API; PUT response solo retorna status, no token | ✅ |
| polling-60s | `DegradedBanner.tsx` useEffect con interval 60000 | ✅ |
| hardening | tests de handlers cubren 401/400/502; repository UpdateToken testeado | ✅ |

## Design conformance (D1–D6)

| Decision | Status |
|----------|--------|
| D1 Endpoint dedicado | ✅ /tenants/me separado de /me |
| D2 Polling 60s | ✅ DegradedBanner useEffect+setInterval |
| D3 Password como confirmación | ✅ bcrypt antes de getMe |
| D4 Repository pattern | ✅ UpdateToken en tenants repo |
| D5 Banner en Layout | ✅ fuera de RequireAuth pero solo visible con sesión |
| D6 Zod para validación | ✅ schema en TenantSettingsPage |

## Invariantes

- **Token hygiene**: grep del diff por token real → vacío. PUT response no incluye token.
- **Backend intacto**: solo cambios de tenant-settings.
- **Flake pre-existente**: PublicationsPage 18/18 aislado, 122/123 suite.
- **No new migration**: tabla tenants ya tenía columnas necesarias.

## Issues

### CRITICAL / WARNING
None.

### SUGGESTION
- S1: Pre-existing PublicationsPage flake (timeout bajo carga paralela).
- S2: Hardening verify Slice 0 (migración legacy, rollback, sesiones legacy) — pendiente para deploy con datos reales.

## Next steps
1. PR single `feat/tenant-settings` → `main` (size:exception).
2. Archive: APPEND delta specs a `openspec/specs/` + mover change a archive.
3. Hardening pre-prod (tests pendientes del verify Slice 0).
