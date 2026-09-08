# Tasks: Slice 0 — Multitenancy Backend

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 900–1200 |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR1 → mig+tenants; PR2 → auth; PR3 → registry+tests |
| Delivery strategy | single-pr |
| Chain strategy | pending |

Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: pending
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Notes |
|------|------|-----------|-------|
| 1 | Migración + tenants/crypto | PR 1 | base feat/slice-0-multitenancy |
| 2 | Auth + signup | PR 2 | base PR1; mocks TG |
| 3 | Registry + scoping + wiring | PR 3 | base PR2; buses por tenant |

## Phase 1: Foundation
- [ ] 1.1 T1 Migración `00009` — `backend/migrations/00009_multitenancy.sql` | REQs: mig-tabla, mig-backfill, mig-unicidad, mig-orden | Done: `go test ./internal/database/ -run TestMigrate -count=1`
- [ ] 1.2 T2 Tenants model+repo+crypto — `backend/internal/tenants/{model,repository,crypto}.go` | REQs: mig-tabla, prov-signup | Done: `go test ./internal/tenants/ -count=1`
- [ ] 1.3 T3 Config enc-key + `.env.example` — `backend/internal/config/config.go`, `.env.example` | REQs: comp-arranque, comp-vars | Done: `go test ./internal/config/ -count=1`

## Phase 2: Core Implementation

- [ ] 2.1 T4 Claims + requireAuth/me/refresh — `backend/internal/auth/tokens.go`, `backend/internal/api/{middleware_auth,auth_handlers}.go` | REQs: auth-access, auth-refresh, auth-identidad, comp-sesiones | Done: `go test ./internal/auth/ ./internal/api/ -run 'TestTokens|TestRefresh|TestMe' -count=1`
- [ ] 2.2 T5 Admin tenant + bootstrap `default` — `backend/internal/auth/{admin,repository,seeder}.go` | REQs: auth-bootstrap, prov-username, comp-arranque | Done: `go test ./internal/auth/ -run 'TestSeeder|TestAdmin' -count=1`
- [ ] 2.3 T6 Signup transaccional — `backend/internal/auth/service.go`, `backend/internal/api/auth_handlers.go` | REQs: prov-signup, prov-slug, prov-username, prov-token, prov-validación | Done: `go test ./internal/api/ -run TestSignup -count=1`
- [ ] 2.4 T7 Registry multi-bot — `backend/internal/telegram/registry.go` | REQs: reg-boot, reg-caliente, reg-degraded, reg-ratelimit | Done: `go test ./internal/telegram/ -run TestRegistry -count=1`
- [ ] 2.5 T8 Scoping repos + ownership — `backend/internal/{groups,joinrequests,logs,publications,automation}/repository.go`, `backend/internal/api/groups_handlers.go` | REQs: iso-listados, iso-ajeno, mig-unicidad | Done: `go test ./internal/groups/ ./internal/joinrequests/ ./internal/logs/ -count=1`
- [ ] 2.6 T9 Users vía join + sin `can_*` — `backend/internal/users/repository.go`, `backend/internal/moderation/permissions.go` | REQs: iso-users, iso-nocan | Done: `go test ./internal/users/ ./internal/moderation/ -count=1`

## Phase 3: Integration / Wiring

- [ ] 3.1 T10 Wiring BootAll/RegisterHot/StopAll — `backend/cmd/server/main.go`, `backend/internal/api/server.go` | REQs: reg-boot, reg-caliente, iso-ajeno, comp-arranque | Done: `go vet ./... && go build ./...`
- [ ] 3.2 T11 Modo global + legacy `default` — `backend/cmd/server/main.go`, `backend/internal/telegram/poller.go` | REQs: reg-boot, comp-vars | Done: `go test ./internal/telegram/ -run TestPoller -count=1`

## Phase 4: Testing

- [ ] 4.1 T12 Unit mocks TG — `backend/internal/auth/service_test.go`, `backend/internal/tenants/crypto_test.go` | REQs: prov-*, auth-* | Done: `go test ./internal/auth/ ./internal/tenants/ -count=1`
- [ ] 4.2 T13 Integración Postgres — `backend/internal/database/testdb.go`, `*_repository_test.go` | REQs: mig-*, iso-listados | Done: `go test -tags=integration ./internal/... -count=1`
- [ ] 4.3 T14 E2E manual 2 tenants — runbook en PR, sin archivo | REQs: reg-* E2E | Done: `docker compose up` verde; tokens REDACTED; logs sin token

## Phase 5: Cleanup

- [ ] 5.1 T15 Trazabilidad 24/24 + higiene — `openspec/changes/slice-0-multitenancy-backend/tasks.md` | REQs: todos | Done: matriz 24/24 OK; sin secretos reales
