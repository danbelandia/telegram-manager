# Archivado: publications-slice3 (Programación + Paginación + Cancel)

- **Change**: `publications-slice3` (Slice 3 de 3 — programación in-process + historial paginado + cancel). Cierra **Fase 2** del proyecto (AGENTS §22).
- **Archived**: 2026-09-07
- **Branch**: `feat/publications-slice3` (base `main` @ `b90c584` — slice 2 archivado)
- **Verdict**: **PASS WITH WARNINGS** (9/9 REQs, 78 escenarios, 11/11 paquetes backend verdes, `npm run build` limpio; **W1** — 2 tests slice-2 flaky bajo carga paralela jsdom, NO regresión slice 3; ver `verify-report.md`)
- **Delivery**: single-pr / `size:exception` (~1276 LOC forecast, ~2237 reales; 22 archivos; decisión #188)
- **Commit de archivado**: `chore(openspec): archive change publications-slice3`

## Spec canónica (source of truth actualizado)

Los deltas de spec se mergearon sobre la spec canónica permanente
(mismo patrón que slice 1 archivado en `2026-09-07-publications/` y
slice 2 archivado en `2026-09-07-publications-slice2/`). Este archivo
NO contiene la spec; la convención del repo es mantener la spec en
`openspec/specs/`. La subcarpeta `specs/` dentro del archive es solo
audit trail del delta usado en la merge.

- **`openspec/specs/publications/spec.md`** — spec canónica
  actualizada con un apéndice **"## Slice 3 Extensions (2026-09-07 —
  Programación + Paginación + Cancel)"** que contiene los **3 ADDED**
  y los **6 MODIFIED** del delta. El contenido de slices 1 y 2 se
  conserva tal cual arriba. Bloque histórico de slices ampliado
  para mencionar slice 3.

### Resumen de la sync

| Spec canónica | ADDED | MODIFIED (replace) | Notes |
|---------------|-------|--------------------|-------|
| `publications/spec.md` | 3 (Worker in-process Scheduler, DELETE `/api/publications/:id`, Errores scheduling + paginación) | 6 (`POST /api/publications` con `scheduled_at` opcional, `GET /api/publications` paginado, Frontend `PublicationsPage` dual-mode + Cancel + Paginación, Tests backend worker+service+repo+handler, Tests frontend datetime-local + cancel + pagination, README sección publicaciones programación + paginación) | Purpose header actualizado; bloque histórico de slices extendido con slice 3 (worker `time.Ticker` + `SELECT ... FOR UPDATE SKIP LOCKED`, paginación, cancelación de `scheduled`); bugfix #172 sigue vigente |

**Total slice 3**: 9 requirements, 78 escenarios (32 heredados de
slice 2 + 46 nuevos — incluye el ajuste de 3→4 escenarios en el REQ
"Errores" tras la canonización).

> El delta original (`openspec/changes/publications-slice3/specs/publications/spec.md`)
> conserva la nota de cabecera "Archive step sync → `openspec/specs/...`"
> como puntero al proceso. Ese archivo ahora vive en el archive y es
> solo audit trail — la **fuente de verdad** es la spec canónica.

## Contenido del archive

```
2026-09-07-publications-slice3/
├── README.md              (este archivo)
├── exploration.md         (Bot API evidence + 13 decisiones D1-D13 + affected areas)
├── proposal.md            (alcance + 7 riesgos + criterios de éxito)
├── design.md              (D1-D14 + data flow + 22 file changes + 13 testing layers)
├── tasks.md               (10 phases, 33 tasks; review workload forecast High)
├── apply-report.md        (5 commits, 19 files mod + 5 files new, gates verdes)
├── verify-report.md       (PASS WITH WARNINGS, 9/9 REQs, 14/14 design, 1 deviation)
└── specs/                 (audit trail del delta mergeado)
    └── publications/
        └── spec.md
```

## Observaciones Engram (project: telegrammanager)

Para trazabilidad, los artefactos del SDD viven en Engram con los
siguientes IDs:

- `sdd/publications-slice3/exploration` → observation #183
- `sdd/publications-slice3/proposal` → observation #184
- `sdd/publications-slice3/spec` → observation #185
- `sdd/publications-slice3/design` → observation #186
- `sdd/publications-slice3/tasks` → observation #187
- `sdd/publications-slice3/delivery-strategy` → observation #188 (decision — size:exception)
- `sdd/publications-slice3/apply-report` → filesystem only (ver `apply-report.md`; la fase apply no persistió a Engram — gap menor, audit trail completo en `openspec/changes/archive/.../apply-report.md`)
- `sdd/publications-slice3/verify-report` → observation #190
- `sdd/publications-slice3/archive-report` → observation #191 (este archivo)

## Commits en `feat/publications-slice3` (5 commits ahead of main)

1. `166494a` feat(publications): worker in-process + scheduling + paginacion + cancel
2. `3db3fc3` feat(api): DELETE /api/publications/{id} + rama scheduled_at + paginacion
3. `3b5cadd` feat(web): PublicationsPage dual-mode + datetime-local + Cancelar + Prev/Next
4. `512104f` docs: README Publicaciones con Programacion + Paginacion + nota Bot API
5. `a23c09a` chore(sdd): publications-slice3 — exploration, proposal, design, tasks, spec

## Invariantes respetadas (del verify-report)

- **AGENTS §21.1**: cero llamadas reales a la Bot API en tests. Unit
  tests de service/worker/handler con `fakeTelegramPub` +
  `fakeGroupsPub` + `fakeLogsPub` + `fakePubStore`. Integration tests
  del repo contra Postgres real (SKIP LOCKED validado con dos
  `*sql.Tx` paralelas en `TestRepository_ClaimScheduledDue_SkipsLockedByAnotherTxn`).
- **Bugfix #172**: `permissionOk` sigue con `g.BotStatus ==
  StatusAdministrator`. Worker replica el check en
  `Scheduler.permissionOk` (worker.go:192-193). `grep "can_*"
  backend/internal/publications/`: 5 matches, **TODOS en comments**
  (service.go:21, 558, 559, 561, 569); **cero en executable code**.
  Test `TestScheduler_Tick_PermissionDenied_StaysFailed` cubre el
  escenario "admin removido antes del tick".
- **AGENTS §18.1**: worker reusa el helper extraído
  `publishOneFinalize` (no `publishOne` literal — ver deviation #1)
  con dispatch SECUENCIAL dentro de `tick()` (cero goroutines
  paralelas). Respeta el token bucket del adapter.
- **AGENTS §22 (Fase 2)**: publicaciones inmediatas, programadas y
  paginadas — cerradas en este slice.
- **AGENTS §2/§13.1**: cero migraciones nuevas (`scheduled_at
  TIMESTAMPTZ` y `status='scheduled'` ya viven en 00004/00005).
  Variable de entorno `PUBLICATIONS_SCHEDULER_INTERVAL_SECONDS`
  (default 30) agregada a `.env.example`.

## Desviaciones documentadas (re-confirmadas en verify-report)

| # | Desviación | Justificación | Aceptable? |
|---|---|---|---|
| 1 | Worker reusa `publishOne` vía helper extraído `publishOneFinalize` (vs reuse literal del design) | `publishOne` slice 2 **crea una fila** (`store.Create`); worker necesita un path donde la fila ya existe. Refactor mínimo: extraer de `publishOne` la mitad inferior (dispatch + UpdateStatus + log) a un método `publishOneFinalize` (service.go:280-321) que comparten ambos paths. `Scheduler.permissionOk` (worker.go:192-193) replica el check `g.BotStatus == StatusAdministrator` para preservar la invariante del bugfix #172. Comportamiento observable idéntico. | ✅ Sí — preserva #172, sin duplicar path de envío. |
| 2 | `fakePubStore.ClaimScheduledDue` en memoria (sin simular SKIP LOCKED real) | SKIP LOCKED es semántica SQL Postgres; el fake in-memory no puede replicar locks reales. La validación real ocurre en `TestRepository_ClaimScheduledDue_SkipsLockedByAnotherTxn` con dos `*sql.Tx` paralelas contra Postgres real. | ✅ Sí — cobertura integration en el path real. |
| 3 | Frontend API siempre envía `?limit=&offset=` cuando están definidos (incluso si son 0) | URL determinística para tests + query keys estables. El backend aplica defaults cuando los params están ausentes (`parsePaginationQuery` línea 228-233). | ✅ Sí — cambio mínimo, query keys estables. |

## W1 — Warning documentado (2 tests slice-2 flaky bajo carga jsdom paralela)

`npm test -- --run` con **suite completa** (paralela) muestra
intermitentemente 2 fallos en tests que **ya eran verdes en b90c584**
y **NO son regresión de slice 3**:

- `crea una publicacion multi-grupo con foto y botones`
  (`PublicationsPage.test.tsx:142`) — intermitente timeout a 5000ms.
- `no envia POST si la URL de la foto no es http(s)`
  (`PublicationsPage.test.tsx:269`) — intermitente
  "Unable to find an element with the text".

Ambos pasan **18/18** cuando el archivo se ejecuta en aislamiento.
Vitest emite el siguiente warning:

> Environment jsdom was created 11 times · 91.39s total, 56% of
> tracked time
> create it once per worker with pool: 'vmThreads' (keep per-file
> isolation) or isolate: false (shares it across files)

**Causa raíz**: configuración de pool/entorno de vitest (problema
infraestructural, no del código slice 3). Sugerencia para otro
cambio (out of scope este slice): `test.pool: 'vmThreads'` en
`vitest.config.ts`.

**Confirmación**: código slice 3 es correcto — verified por
`npm test -- --run src/pages/PublicationsPage.test.tsx` (18/18) y por
el backend completo (`go test ./... -count=1` → 11/11 packages OK).

## SPEC.md sync actions

Acción tomada en `openspec/specs/publications/spec.md` durante el
archive (ver `git diff` del commit de archivado):

- **Update del histórico de slices** en el Purpose header: agregado
  el párrafo que describe slice 3 (worker `time.Ticker` + SKIP
  LOCKED, paginación, cancelación de `scheduled`, sin cambios al
  adapter de Telegram, bugfix #172 vigente en worker).
- **Append** de nueva sección `## Slice 3 Extensions (2026-09-07 —
  Programación + Paginación + Cancel)` al final del archivo con:
  - **3 ADDED Requirements**: Worker in-process (Scheduler), DELETE
    `/api/publications/:id`, Errores de scheduling y paginación.
  - **6 MODIFIED Requirements**: POST `/api/publications` con
    `scheduled_at` opcional, GET `/api/publications` paginado,
    Frontend `PublicationsPage` dual-mode + Cancel + Paginación,
    Tests backend (worker + service + repo + handler), Tests
    frontend (datetime-local + cancel + pagination), README sección
    publicaciones.
  - Tabla "Slice 3 Extensions — Summary" con 9 rows y conteo total
    (78 escenarios).
- Las requirements de slices 1 y 2 NO fueron removidas — permanecen
  en el archivo para audit trail. Las versiones vigentes de los
  requirements MODIFIED por slice 3 aparecen en la nueva sección.

## SDD Cycle Complete

El change ha sido planificado, implementado, verificado y archivado.
El branch `feat/publications-slice3` queda listo para que el
orchestrator haga merge a `main` y push a remoto. NO se hizo push ni
merge en este paso (regla del archive phase).

**Cambio NO tocado**: código fuente. Solo artefactos SDD
(`openspec/specs/`, `openspec/changes/`).
