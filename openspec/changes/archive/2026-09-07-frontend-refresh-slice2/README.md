# Archivado: frontend-refresh-slice2 — 3 Páginas de Moderación + Docker Dev Fix

- **Change**: `frontend-refresh-slice2` (Slice 2 de N — migración visual + housekeeping docker-compose; nueva capability `frontend-pages-moderation`)
- **Archived**: 2026-09-07
- **Branch**: `feat/frontend-refresh-slice2` (base `main @ f4d1751`, **5 commits** ahead, **NOT** pushed/merged)
- **Verdict**: **PASS** (verify-report #213) — 30/30 tasks, 68/68 tests, build verde, non-regression surface **vacío** (4 sets de invariantes, 0 líneas).
- **Delivery**: single-pr / size:exception (decisión #211 — sexto size:exception consecutivo aprobado por el usuario; precedent 5/5 → 6/6)
- **Spec capability**: `frontend-pages-moderation` (**NEW**, 14 REQs, 24 scenarios). Primera entrada; no hay slices previos que extender.

## Source of truth (canonical spec creada)

El delta de spec era un **spec completo nuevo** (no un delta sobre una spec existente). Se movió al árbol canónico sin append — este es el contenido inicial de la capability. El delta ya **NO** existe como archivo separado dentro del archive folder (la spec canónica ES el delta, `git mv` lo registra como rename — el audit trail es la trazabilidad vía `git log --follow`).

- **`openspec/specs/frontend-pages-moderation/spec.md`** — spec canónica **CREADA** (NEW capability, 207 líneas). Contiene los 14 REQs + 24 scenarios completos: GroupUsersPage Cards+SimpleGrid, Acciones con feedback, Modal confirmación Banear, GroupRequestsPage Table+Badge, Acciones requests con notification, GroupLogsPage Table sticky, Paginación diferida (non-goal documentado), Notifications wiring call-sites, Tests con renderWithProviders, docker-compose anonymous volume removido, Dockerfile CMD `npm install`, `.dockerignore` excluye artefactos, README documenta bind-mount + rebuild, No regresión de capas no tocadas.

### Resumen de la sync

| Spec canónica | Acción | Detalle |
|---------------|--------|---------|
| `openspec/specs/frontend-pages-moderation/spec.md` | **CREADA** (NEW) | 14 REQs / 24 scenarios movidos completos desde el delta (no append — es la entrada inicial). `git mv` registra el rename, preservando historia. |

**Verificación pre-archive**: `openspec/specs/` NO contenía `frontend-pages-moderation/` antes de este archive (confirmado por `Get-ChildItem openspec/specs/`). Cero colisiones. La spec existente `frontend-moderation` (2026-09-07 10:06) describe **el dominio** (DTOs, endpoints, requisito de confirmación de ban) y **NO** se modifica — los REQs de dominio ya estaban cumplidos. El view layer (Cards, Modal destructivo, sticky header, badges por status, notifications wiring en call-sites) es el detalle que este nuevo spec cubre, sin overlap.

## Contenido del archive

```
2026-09-07-frontend-refresh-slice2/
├── README.md                    (este archivo)
├── exploration.md               (20 decisiones S2-D1..S2-D20, contexto heredado)
├── proposal.md                  (alcance + criterios de éxito + Branch Base Correction)
├── design.md                    (13 decisiones D1-D13 + interfaces + risk table)
├── tasks.md                     (7 phases, 30 tasks, todas [x])
├── apply-report.md              (5 commits, gates verdes, 5 deviations)
└── verify-report.md             (PASS, 14 REQs compliance matrix)
```

> Nota: a diferencia del archive de slice-1, este archive **NO** contiene subdirectorio `specs/frontend-pages-moderation/`. El delta spec fue **movido** (no copiado) al canónico — la spec vive en un solo lugar. La trazabilidad histórica queda vía `git log --follow openspec/specs/frontend-pages-moderation/spec.md`.

## Observaciones Engram (project: telegrammanager)

Para trazabilidad, los artefactos del SDD viven en Engram con los siguientes IDs:

- `sdd/frontend-refresh-slice2/exploration` → observation **#206** (20 decisiones, contexto heredado de slice-1)
- `sdd/frontend-refresh-slice2/proposal` → observation **#207** (NEW capability, scope, Branch Base Correction)
- `sdd/frontend-refresh-slice2/spec` → observation **#208** (frontend-pages-moderation, NEW full spec, 14 REQs)
- `sdd/frontend-refresh-slice2/design` → observation **#209** (13 decisiones D1-D13 + interfaces + file-by-file specs)
- `sdd/frontend-refresh-slice2/tasks` → observation **#210** (7 phases, 30 tasks, Review Workload Forecast High → size:exception)
- `sdd/frontend-refresh-slice2/delivery-strategy` → observation **#211** (decision — size:exception aprobado, precedent 5/5 → 6/6)
- `sdd/frontend-refresh-slice2/apply-report` → observation **#212** (5 commits, gates verdes, invariantes respetadas)
- `sdd/frontend-refresh-slice2/verify-report` → observation **#213** (PASS, 14 REQs compliance matrix, 5 deviations)
- `sdd/frontend-refresh-slice2/archive-report` → observation **(THIS observation)** (archive phase closeout)

> Precedente heredado — slice-1 archive-report: observation **#203**.

## Commits en `feat/frontend-refresh-slice2` (5 commits ahead of main @ f4d1751)

1. `dcef230` chore(openspec) — artifacts (explore/propose/spec/design/tasks + NEW spec)
2. `7b3b8e0` chore(docker) — bind-mount `frontend/src` only + `npm install` on startup + `.dockerignore`
3. `4115fa1` feat(frontend) — migrate 3 moderation pages to Mantine v7
4. `cfd53e0` chore(openspec) — `tasks.md` marked complete
5. `af173c8` chore(openspec) — apply-report

## Métricas del change

| Métrica | Valor |
|---------|-------|
| Tasks (30) | 30/30 ✅ |
| Tests | **68/68** ✅ (13 files, 22.02s, cero flakeos paralelos — W1 slice-1 resuelto) |
| `npm run build` | ✅ Bundle **197.97 kB gz** (+22 kB vs main, dentro de orden de magnitud del estimado +10-15) |
| `tsc --noEmit` | ✅ Clean |
| `vite build` | ✅ 7126 modules, 4.32s |
| Non-regression surface | ✅ **0 lines** (4 sets de invariantes: features/, lib/api-client, auth-context, notifications, theme.ts, main.tsx, components/{Layout,ColorSchemeToggle}, backend/, migrations/, openspec/specs/frontend-ui-foundation, test/helpers, vite.config, package.json, slice-1 pages, PublicationsPage) |
| Docker smoke | ✅ `docker compose up -d frontend` → Vite ready in **602 ms** |
| Files changed | 10 (3 page rewrites + 3 test rewrites + docker-compose + Dockerfile + .dockerignore NEW + README) + 6 openspec artifacts |
| Spec REQs | **14/14 PASS** (14 REQs × 24 scenarios total) |
| Design decisions | **13/13 PRESENT and CORRECT** |

## Deviations documentadas (5, todas aceptables — ninguna bloquea el archive)

1. **Inline style sticky header en GroupLogsPage** — Mantine v7.17.0 no soporta la prop `sticky` en `<Table.Thead>` (TypeScript falla con `TS2322`). Workaround: `style={{position:'sticky', top:0, zIndex:1, background:'var(--mantine-color-body)'}}`. Comportamiento runtime idéntico, typesafe. Cuando Mantine v8 lands, revisar (SUGGESTION en verify-report).
2. **Columna Actor agregada a GroupLogsPage** — `actor_id` ya estaba en el tipo `LogEntry` (backend lo devuelve desde slice-1); agregarla a la tabla es útil para audit UX. La spec listaba columnas mínimas; agregar una no viola el contrato.
3. **`formatDate` inline (no extraído a `lib/utils`)** — Constraint heredado del invariant de slice-1 (`lib/utils.ts` fuera de scope). Duplicación menor: 2 instancias, 4 líneas cada una. Aceptable.
4. **Bundle delta +22 kB gz vs +10-15 estimado** — Mismo orden de magnitud (≤2×). Componentes añadidos: `<Avatar>` + 4 iconos Tabler + `<Modal>` + `<Tooltip>` + `<Container>`. Documentado como SUGGESTION para code-splitting en slice 3+.
5. **Modal cancel smoke usa click en botón "Cancelar" vs mock de `window.confirm = false`** — Test con real-DOM más fiel al flujo del usuario; slice-1 usaba mock. Decisión técnica mejor que el patrón legacy.

## Bug fix destacado — docker-compose anonymous volume

**Causa raíz** (explorada en exploration #206 §S2-D13/D14): `docker-compose.yml` declaraba `- node_modules:/app/node_modules` (volumen anónimo) + bind-mount `./frontend:/app`. En Windows/OneDrive, el volumen nombrado se creaba VACÍO en el primer arranque, pisando `/app/node_modules` de la imagen → Vite crasheaba con "Cannot find module 'vite'".

**Triada de fix** (aplicada en commit `7b3b8e0`):
1. `docker-compose.yml`:
   - Removido `- node_modules:/app/node_modules`
   - Bind-mount cambiado `./frontend:/app` → `./frontend/src:/app/src` (solo código fuente)
   - Removida entrada huérfana `node_modules:` del bloque `volumes:` raíz
2. `frontend/Dockerfile`:
   - CMD cambiado a `["sh", "-c", "npm install && npm run dev -- --host 0.0.0.0"]` — `npm install` corre en cada arranque (acepta +10-30s, dentro del `start_period: 10s` del healthcheck del backend)
3. `frontend/.dockerignore` (NEW): excluye `node_modules`, `dist`, `.vite`, `coverage`, `.env`, `.env.local`, `*.log`, `.vscode`, `.idea`, `.DS_Store`, `.git`

**Documentación**: `README.md` ahora incluye sub-sección "Cambios de dependencias en el frontend" indicando que cambios en `package.json`/`vite.config.ts` requieren `docker compose build frontend`.

**Resultado verificado** (verify-report #213, apply-report #212): `docker compose up -d frontend` → Vite ready en **602 ms**.

## Branch Base Correction (documentada en proposal #207)

**Discrepancia con exploration** (`S2-D20`): la exploración asumió que `feat/frontend-refresh` seguía sin mergear y propuso branchear desde ahí. **El estado real** al momento del proposal:

- Slice 1 mergeado a `main` como commit `700681c`.
- `PublicationsPage` ad-hoc mergeado como `f4d1751` (rama `feat/publications-page-mantine`).
- `feat/frontend-refresh` **ya no existe** como branch separada — `main` está en `f4d1751`.

**Decisión**: branchear **`feat/frontend-refresh-slice2`** desde **`main @ f4d1751`** (current tip). Esto:

1. Evita duplicación de historia (no cherry-pick de slice-1).
2. Mantiene el branch dedicado para review independiente (slice-2 vs slice-1 ya cerrado).
3. Permite rebase limpio si `main` recibe más merges durante apply.

## Invariantes respetadas

- **AGENTS §11 (NO Redis)**: 0 dependencias Redis añadidas. Stack frontend puro.
- **AGENTS §14 (modular backend)**: backend NO tocado — `git diff main -- backend/` = 0 lines.
- **AGENTS §13.1 (goose migrations)**: 0 migraciones nuevas — `git diff main -- migrations/` = 0 lines.
- **AGENTS §17.1 (auth sin secretos)**: 0 secretos en código. Tokens del bot siguen via env vars.
- **AGENTS §18.1 (rate limits Telegram)**: N/A — slice-2 es frontend-only, sin llamadas a la Bot API.
- **AGENTS §21.1 (mockear TelegramService)**: N/A — slice-2 no toca `TelegramService`.
- **AGENTS §25 regla 18 (frontend no llama Telegram directo)**: confirmado — `features/*` no tocado, todo el wiring pasa por `lib/api-client.ts` (sin cambios).
- **Slice-1 invariant (lib/notifications.ts único punto de cambio)**: preservado — diff `lib/notifications.ts = 0` lines. El wiring de notifications se hizo SOLO en call-sites de `pages/*` (REQ-8).
- **Slice-1 invariant (publications-mantine ad-hoc f4d1751)**: preservado — `git diff main -- frontend/src/pages/PublicationsPage.tsx` = 0 lines.

## Files modificados (10 source + 6 openspec artifacts)

### Frontend (3 page rewrites + 3 test rewrites)

| File | Action | LOC Δ | Description |
|---|---|---|---|
| `frontend/src/pages/GroupUsersPage.tsx` | rewrite | 167→331 (+267/-158) | Cards+SimpleGrid; `<Modal>` para ban; `<TextInput>` lookup; notifications wiring en 4 mutations |
| `frontend/src/pages/GroupRequestsPage.tsx` | rewrite | 117→172 (+167/-100) | `<Table>` + `<Badge>`; Aprobar/Rechazar directo + notify; elimina banners inline |
| `frontend/src/pages/GroupLogsPage.tsx` | rewrite | 74→200 (+148/-69) | `<Table>` sticky; Badge color map (7 entries); columna Actor agregada; `<Tooltip>` en error_message |
| `frontend/src/pages/GroupUsersPage.test.tsx` | adjust | 171→200 (+102/-79) | Wrapper `renderWithProviders`; `data-testid="user-card"` y `"confirm-ban"`; +2 modal smokes |
| `frontend/src/pages/GroupRequestsPage.test.tsx` | adjust | 150→166 (+91/-65) | Wrapper swap; +1 notification smoke (`findByText` matchea portal) |
| `frontend/src/pages/GroupLogsPage.test.tsx` | adjust | 98→100 (+22/-16) | Wrapper swap; sin nuevos tests (REQ-7 paginación diferida) |

### Devops (Docker triada)

| File | Action | LOC Δ | Description |
|---|---|---|---|
| `docker-compose.yml` | modify | +10/-3 | Bind-mount `./frontend/src:/app/src`; removido `- node_modules:/app/node_modules`; eliminada entrada `node_modules:` raíz |
| `frontend/Dockerfile` | modify | +13/-2 | CMD `["sh","-c","npm install && npm run dev -- --host 0.0.0.0"]` |
| `frontend/.dockerignore` | **NEW** | +11 | 11 exclusiones (node_modules, dist, .vite, coverage, .env, *.log, .vscode, .idea, .DS_Store, .git) |

### Documentación

| File | Action | LOC Δ | Description |
|---|---|---|---|
| `README.md` | modify | +20 | Sub-sección "Cambios de dependencias en el frontend" |

## Sugerencias para slices futuros (no bloquean)

- **S-V1**: Extraer `formatDate` a `lib/utils.ts` (2 instancias, 4 líneas cada una) — duplicación menor.
- **S-V2**: Visual regression test para sticky header cuando Mantine v8 lands (la prop `sticky` probablemente vuelva).
- **S-V3**: Code-splitting agresivo en frontend (bundle 198 kB gz está dentro de budget pero cerca del techo).
- **S-V4**: GroupLogsPage filtros + paginación (S2-D9 + REQ-7 diferido) — requiere backend `?limit&offset&action&status` y agregar `<Pagination>` consistente.

## Next steps (orchestrator)

1. Merge `feat/frontend-refresh-slice2` → `main` (single-pr, size:exception aprobado per #211).
2. Push to remote.
3. Slice-2 cierra las 3 páginas de moderación y el bug heredado de docker-compose. Siguientes candidatos: `publications-slice4` (PublicationsPage migration completa — 456 LOC formulario complejo), o `frontend-refresh-slice3` (GroupDetailPage 4-tab layout fix per W-V1 slice-1 + GroupLogsPage paginación).

## Commit (this archive)

- Message: `chore(openspec): archive change frontend-refresh-slice2`
- Branch: `feat/frontend-refresh-slice2`
- Files:
  - `openspec/specs/frontend-pages-moderation/spec.md` (NEW canonical — git tracked rename from delta)
  - `openspec/changes/archive/2026-09-07-frontend-refresh-slice2/` rename (5 git mv renames + `verify-report.md` tracked + `README.md` NEW)
  - `openspec/changes/frontend-refresh-slice2/` removed
- **NO source code touched.**

## SDD Cycle Complete

El change ha sido planificado, implementado, verificado y archivado.
El branch `feat/frontend-refresh-slice2` queda listo para que el orchestrator
haga merge a `main` y push a remoto. **NO se hizo push ni merge** en este
paso (regla del archive phase).
