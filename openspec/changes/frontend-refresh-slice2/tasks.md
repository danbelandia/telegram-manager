# Tasks: frontend-refresh-slice2

> **Change**: `frontend-refresh-slice2` | **Mode**: hybrid | **Delivery**: single-pr `size:exception` (precedent 5/5).
> **Inherits**: exploration #206, proposal #207, spec #208, design #209.

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~+77 net / ~780 churn across 10 files |
| 400-line budget risk | **High** (3 rewrites + 3 test rewrites; precedent overrides) |
| Chained PRs recommended | No (single-pr `size:exception`) |
| Delivery strategy | single-pr / size-exception |
| Chain strategy | size-exception (re-confirm at apply) |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: size-exception
400-line budget risk: High

> Subo a **High**: churn ~780 = ~2× budget + rewrites completos. Precedent 5/5 + scope acotado lo justifican.

## Phase 1: Branch + baseline

- [ ] 1.1 `git checkout main && git checkout -- frontend/package-lock.json` (revertir working-tree)
- [ ] 1.2 `git checkout -b feat/frontend-refresh-slice2`
- [ ] 1.3 Gate: `cd frontend && npm test -- --run` baseline verde (66+ tests)
- [ ] 1.4 Gate: `cd frontend && npm run build` baseline verde

## Phase 2: Docker dev fix (ANTES de page migrations)

- [ ] 2.1 NEW `frontend/.dockerignore`: `node_modules`, `dist`, `.vite`, `coverage`, `.env`, `.env.local`, `*.log`, `.vscode`, `.idea`, `.DS_Store`, `.git`
- [ ] 2.2 Edit `frontend/Dockerfile` L11 CMD → `["sh","-c","npm install && npm run dev -- --host 0.0.0.0"]`
- [ ] 2.3 Edit `docker-compose.yml`: remover `- node_modules:/app/node_modules`; `./frontend:/app` → `./frontend/src:/app/src`; borrar volumen `node_modules:` raíz
- [ ] 2.4 Smoke: `docker compose up -d --build frontend` → Vite ready <60s

## Phase 3: GroupUsersPage migration

- [ ] 3.1 Rewrite `GroupUsersPage.tsx` (~180 LOC): Cards + `SimpleGrid cols={{base:1,sm:2}}`; `<TextInput>` + `<form onSubmit>` lookup; 4 botones (Banear red / Mutear yellow / Desbanear-Desmutear light); Modal `useState<GroupUser|null>`
- [ ] 3.2 Wirear: `notifySuccess('Usuario baneado'|'muteado'|'desbaneado'|'desmuteado')` + `notifyError(formatModerationError(e))`; onSuccess cierra Modal
- [ ] 3.3 Update `GroupUsersPage.test.tsx`: wrapper → `renderWithProviders`; `data-testid="user-card"` + `"confirm-ban"`; eliminar mock `window.confirm`
- [ ] 3.4 +2 modal smokes: (a) Banear → Modal → `confirm-ban` → spy called + `findByText('Usuario baneado')`; (b) Cancelar → spy NOT called
- [ ] 3.5 Gate: `npm test -- --run src/pages/GroupUsersPage.test.tsx` verde

## Phase 4: GroupRequestsPage migration

- [ ] 4.1 Rewrite `GroupRequestsPage.tsx` (~110 LOC): `<Paper>` + `<Table>` + Badge (pending=yellow, approved=green, rejected=red); Aprobar/Rechazar clic directo
- [ ] 4.2 Wirear `notifySuccess('Solicitud aprobada'|'rechazada')`; reemplazar `<p className="state-block">` inline por notifications
- [ ] 4.3 Update `GroupRequestsPage.test.tsx`: wrapper swap; eliminar refs banners inline
- [ ] 4.4 +1 smoke: click Aprobar → `findByText(/solicitud aprobada/i)` matchea portal
- [ ] 4.5 Gate: `npm test -- --run src/pages/GroupRequestsPage.test.tsx` verde

## Phase 5: GroupLogsPage migration

- [ ] 5.1 Rewrite `GroupLogsPage.tsx` (~95 LOC): `<Paper>` + `<Table.ScrollContainer minWidth={500}>` + `<Table striped highlightOnHover withTableBorder>` con `<Table.Thead sticky>`
- [ ] 5.2 `STATUS_BADGE_COLOR` map: SUCCESS=green, PERMISSION_DENIED/INTERNAL_ERROR/ERROR=red, VALIDATION_ERROR=yellow, NOT_FOUND=gray, TELEGRAM_ERROR=orange via `status.toLowerCase()`
- [ ] 5.3 Col Mensaje: `entry.error_message ?? (entry.actor_id ? \`por admin ${actor_id}\` : '—')` envuelto en `<Tooltip>` cuando hay error
- [ ] 5.4 Update `GroupLogsPage.test.tsx`: wrapper swap; sin nuevos tests (REQ-7 paginación diferida)
- [ ] 5.5 Gate: `npm test -- --run src/pages/GroupLogsPage.test.tsx` verde

## Phase 6: Full suite green + non-regression

- [ ] 6.1 `cd frontend && npm test -- --run` — 100% verde (66+ tests)
- [ ] 6.2 `cd frontend && npm run build` — bundle delta +10-15 kB gz
- [ ] 6.3 Non-regression: `git diff main -- frontend/src/features frontend/src/lib/{api-client,auth-context,notifications}.ts frontend/src/{theme.ts,main.tsx} frontend/src/components/{Layout,ColorSchemeToggle}.tsx backend/ migrations/ openspec/specs/frontend-ui-foundation/spec.md` → **0 líneas** (REQ-14)

## Phase 7: README + final commit

- [ ] 7.1 Edit `README.md` — sub-sección "Cambios de dependencias en el frontend" (~12 LOC) tras "UI library & Dark mode"
- [ ] 7.2 Commit openspec artifacts: `chore(openspec): frontend-refresh-slice2 artifacts`
- [ ] 7.3 Conventional commit `feat(frontend): migrate moderation pages + docker dev fix (slice 2)` cubriendo los 10 archivos D12
- [ ] 7.4 NO push (single-pr local); brief del PR con links a proposal/design/spec
