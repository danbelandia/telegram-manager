# Archivado: publications-slice2 (Foto URL + Botones + Multi-Grupo + Filtro)

- **Change**: `publications` (Slice 2 de 3 — extiende el slice 1 archivado `2026-09-07-publications`)
- **Archived**: 2026-09-07
- **Branch**: `feat/publications-slice2` (base `main` @ `9d4f1ec` — bugfix #172 `can_manage_chat` → `bot_status == administrator`)
- **Verdict**: **PASS** (24/24 tasks, go test/vet/gofmt y npm test/build verdes, 15/15 REQs cubiertas con código + tests pasando, 1 SUGGESTION no bloqueante — ver `verify-report.md`)
- **Delivery**: single-pr / size-exception (~1240 LOC, 19 archivos, decisión #178)
- **Commit de archivado**: `chore(openspec): archive change publications-slice2`

## Specs canónicas (source of truth actualizado)

Los deltas de spec se mergearon sobre las specs canónicas permanentes.
Este archivo NO contiene la spec (la convención del repo es mantener la
spec en `openspec/specs/`; la subcarpeta `specs/` dentro del archive es
solo audit trail del delta usado en la merge).

- **`openspec/specs/publications/spec.md`** — spec canónica actualizada
  con un apéndice "## Slice 2 Extensions (2026-09-07)" que contiene los
  5 requirements ADDED y los 8 requirements MODIFIED (replace) del
  delta. El contenido de slice 1 arriba se conserva tal cual.
- **`openspec/specs/telegram-moderation/spec.md`** — spec canónica
  actualizada con un apéndice "## Slice 2 Additions (2026-09-07)" que
  agrega 3 requirements ADDED (Tipos InlineKeyboardMarkup/Button,
  SendMessage con teclado, SendPhoto). El núcleo de moderación no
  cambia.

### Resumen de la sync

| Spec canónica | ADDED | MODIFIED (replace) | Notes |
|---------------|-------|--------------------|-------|
| `publications/spec.md` | 5 (Validaciones payload, PublishMany multi-grupo, Tipos InlineKeyboard, Adapter SendPhoto, README sección publicaciones) | 8 (Tabla publications, SendMessage adapter, POST /api/publications, GET /api/publications, Frontend PublicationsPage, Tests backend, Tests frontend, Registro de auditoría) | Purpose header actualizado; bloque histórico de slices agregado |
| `telegram-moderation/spec.md` | 3 (Tipos InlineKeyboardMarkup/Button, SendMessage con teclado, Adapter SendPhoto) | 0 (no había requirement previo de SendMessage/SendPhoto en la canónica — todos los métodos de moderación originales quedan intactos) | Purpose header actualizado |

## Contenido del archive

```
2026-09-07-publications-slice2/
├── README.md              (este archivo)
├── exploration.md         (contexto heredado, decisiones D1-D11)
├── proposal.md            (alcance + criterios de éxito)
├── design.md              (D1-D10 + bugfix #172, data flow, file changes)
├── tasks.md               (8 phases, 24 tasks)
├── apply-report.md        (5 commits, gates verdes, desviaciones)
├── verify-report.md       (PASS, 15/15 REQs, 3 desviaciones aceptables)
└── specs/                 (audit trail de los deltas mergeados)
    ├── publications/spec.md
    └── telegram-moderation/spec.md
```

## Observaciones Engram (project: telegrammanager)

Para trazabilidad, los artefactos del SDD viven en Engram con los
siguientes IDs:

- `sdd/publications-slice2/exploration` → observation #173
- `sdd/publications-slice2/proposal` → observation #174
- `sdd/publications-slice2/spec` → observation #175
- `sdd/publications-slice2/design` → observation #176
- `sdd/publications-slice2/tasks` → observation #177
- `sdd/publications-slice2/delivery-strategy` → observation #178 (decision — size:exception)
- `sdd/publications-slice2/apply-report` → observation #179
- `sdd/publications-slice2/verify-report` → observation #181
- `sdd/publications-slice2/archive-report` → observation #182

## Commits en `feat/publications-slice2` (6 commits ahead of main)

1. `085d726` feat(telegram): SendPhoto + InlineKeyboard types + migration 00005
2. `6e928ce` feat(publications): PublishMany multi-grupo + foto + botones
3. `5d3c8be` feat(api): POST /api/publications multi-grupo + GET ?group_id= + mapeo 400
4. `5ee347f` feat(web): PublicationsPage foto + botones + multi-grupo + filtro
5. `63c9772` docs: README — seccion Publicaciones con limites operacionales del slice 2
6. `7b4172d` chore(sdd): publications-slice2 — apply-report (5 commits, 19 files, gates verdes)

## Invariantes respetadas (del verify-report)

- **AGENTS §21.1**: cero llamadas reales a la Bot API en tests (httptest + fakes).
- **Bugfix #172**: `permissionOk` sigue con `BotStatus == StatusAdministrator`; NO se reintrodujeron checks `can_*`.
- **AGENTS §18.1**: `PublishMany` SECUENCIAL sobre `group_ids` (cero goroutines paralelas).
- **D5**: `validatePayload` fail-fast antes del loop; errores per-grupo NO abortan el resto.
- **D6**: HTTP 201 incluso si TODAS las filas quedan `failed` (no se usó 207 Multi-Status).

## SDD Cycle Complete

El change ha sido planificado, implementado, verificado y archivado.
El branch `feat/publications-slice2` queda listo para que el
orchestrator haga merge a `main` y push a remoto. NO se hizo push ni
merge en este paso (regla del archive phase).
