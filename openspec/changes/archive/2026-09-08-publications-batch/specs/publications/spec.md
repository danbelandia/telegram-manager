# sdd/publications-batch/spec — Delta Spec for Publications (Batch endpoint + Wizard Modal)

> **Change**: `publications-batch` — feature aditiva sobre Fase 2 (AGENTS §22; §22 NO lista "en lote" explícitamente pero la habilita por composición).
> **Mode**: hybrid (filesystem + Engram).
> **Predecessor**: slice 3 archivado (`main @ 24e4c8c`); canónico `openspec/specs/publications/spec.md` con REQ-1..REQ-15 (slice 1+2+3).
> **Branch base**: `main` (post-slice3 archivado; cero pre-reqs no mergeados).
> **Strategy**: single-pr con `size:exception` (precedente 12/12 PRs consecutivos; reforecast +847 LOC vs budget 400 → ratio 2.1×).
> **Authority**: bugfix `#172` (`permissionOk` usa `g.BotStatus == StatusAdministrator`, NUNCA claves `can_*`); AGENTS §11, §14, §17.1, §18.1, §21.1, §22.

Esta spec **amenda** el canónico de slice 3 vía `## ADDED Requirements` (REQ-16 a REQ-26). Archive usa la técnica APPEND de slices 1→2→3 (delta → canónico; REQ-1..REQ-15 previos preservados, REQ-16..REQ-26 nuevos appendeados). **No** se crea spec paralelo. **No** se modifica el canónico en su lugar (eso lo hace `sdd-archive`). **No** se introduce migración nueva (la tabla `publications` y los métodos `Service.PublishMany` / `Service.Schedule` ya cubren el 100% del feature).

## Decisiones tomadas en este spec (DECIDE)

1. **Spec location**: delta único en `openspec/changes/publications-batch/specs/publications/spec.md`.
2. **REQ numbering**: REQ-16..REQ-26 continúa la numeración lógica de slices 1..15 (canónico archivado).
3. **Status code del batch** (REQ-16): **200 OK** siempre que la envelope sea válida. 400 SOLO por envelope inválida.
4. **`now time.Time` capturado UNA vez** (REQ-17): `now := time.Now()` al inicio, pasado a TODAS las llamadas `Schedule`.
5. **Failure isolation per-publication** (REQ-16, REQ-17): cada item se dispatcha en aislamiento.
6. **`failed[]` payload** (REQ-18): `{index, code, message}` con codes de §18.
7. **Per-item audit logs** (REQ-19): `metadata.batch_index` (int) opcional para correlación.
8. **Non-regression** (REQ-20): `POST /api/publications` unchanged, tabla `publications` unchanged, `Scheduler` unchanged, `Service`/`publicationStore` zero new methods.
9. **`can_*` invariant** (REQ-26): handler NO reintroduce checks sobre `bot_permissions["can_*"]`.

## ADDED Requirements

### Requirement: REQ-16 — Backend `POST /api/publications/batch`

El endpoint `POST /api/publications/batch` MUST aceptar `{publications: [PublicationInput, ...]}` donde cada `PublicationInput` tiene la misma shape que el body de `POST /api/publications`.

1. Autenticación requerida (`requireAuth`).
2. `len(publications) == 0` → **400 VALIDATION_ERROR**.
3. `len(publications) > 10` → **400 VALIDATION_ERROR**.
4. JSON malformado → **400 VALIDATION_ERROR**.
5. Cada item se dispatcha **en aislamiento**.
6. Envelope válida → **200 OK** con `{created: [...], failed: [...]}`.
7. Auth ausente → **401 UNAUTHORIZED**.

### Requirement: REQ-17 — Per-publication dispatch con `now` consistente

El handler `handleCreatePublicationBatch` MUST:

1. Capturar `now := time.Now()` UNA vez.
2. Si `item.scheduled_at` vacío → `service.PublishMany(...)`.
3. Si presente → `NormalizeScheduledAt` + `service.Schedule(..., nowFn)`.
4. Si `scheduled_at` en el pasado → `failed[]` con `code=VALIDATION_ERROR`.
5. Si `PublishMany`/`Schedule` retorna payload-level error → `failed[]` con `code=VALIDATION_ERROR`. NO aborta el batch.
6. Filas `sent`/`scheduled` → `created[]`; filas `failed` → `failed[]`.

### Requirement: REQ-18 — `failed[]` payload format

```json
{"index": <int>, "code": "<DOMAIN_CODE>", "message": "<legible en español>"}
```

`code` ∈ `{VALIDATION_ERROR, PERMISSION_DENIED, NOT_FOUND, TELEGRAM_ERROR, INTERNAL_ERROR}`.

### Requirement: REQ-19 — Per-item audit log con `metadata.batch_index`

Cada item exitoso genera su log `PUBLISH_MESSAGE` con `metadata.batch_index` (int) opcional. Items programados (`scheduled_at` presente) NO emiten log inmediato; quedan como fila `scheduled` que el worker reclamará.

### Requirement: REQ-20 — Non-regression estricta

Preservar sin modificaciones:
1. `POST /api/publications`
2. Tabla `publications`
3. `publications.Scheduler`
4. `publications.Service` (cero método nuevo)
5. `publicationStore` interface (cero método nuevo)
6. `features/publications/*` (frontend reusado verbatim)
7. `PublicationsPage.tsx` (funcionalmente intacto)

### Requirement: REQ-21 — Frontend `BatchWizard` modal

Modal Mantine v7. Default 2 slots, max 10. Cada slot: Textarea, foto URL, ButtonsEditor, multi-grupo, datetime-local opcional. Summary Alert antes del submit. Submit deshabilitado si slot inválido.

### Requirement: REQ-22 — Frontend partial-failure UX

2 banners separados (verde `created[]`, rojo `failed[]`). Botón "Reintentar fallidas" si `failed[]` no vacío. Botón "Cerrar". El modal NO se auto-cierra.

### Requirement: REQ-23 — Frontend "Reintentar fallidas" semantics

Filtrar slots a SOLO los índices en `failed[]`. Pre-rellenar con datos del slot original. Permitir edición. Incrementar contador interno `batch_attempt` (debugging). Re-disparar submit.

### Requirement: REQ-24 — Frontend validation reuse

Reutilizar `formatPublicationsError`, `validatePhotoUrlClient`, `validateButtonsClient`, `validateGroupIdsClient`, `validateScheduledAtClient` de `features/publications/`.

### Requirement: REQ-25 — Tests backend (8+) y frontend (6+)

Backend: cap>10, vacío, JSON malformado, all-OK, all-fail, mixto, mix sched+now, auth, boundary 10, group_ids múltiples.

Frontend: abrir modal, add/remove slot, submit bloqueado si slot inválido, all-success banner, all-fail banner, retry failed subset.

### Requirement: REQ-26 — Bugfix #172 invariant + §21.1 strict

Handler NO consulta `bot_permissions["can_*"]`. Tests NO llaman a `api.telegram.org`. Verificación estática via `TestBatch_NoCanChecksInvariant`.

## Cobertura total

| REQ | Tipo | Escenarios |
|-----|------|-----------|
| REQ-16..26 (batch) | ADDED | 11 reqs, ~30 escenarios |

Tras archive (APPEND al canónico slice 3): REQ-1..15 (slices 1+2+3 preservados) + REQ-16..26 (batch) = **26 requirements totales**.
