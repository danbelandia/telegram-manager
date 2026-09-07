# Archivado: frontend-refresh — Slice 1 (Mantine v7 UI Foundation)

- **Change**: `frontend-refresh` (Slice 1 de N — fundación UI; nueva capability `frontend-ui-foundation`)
- **Archived**: 2026-09-07
- **Branch**: `feat/frontend-refresh` (base `main @ ace1f59`, 11 commits ahead, **NOT** pushed/merged)
- **Verdict**: **PASS WITH WARNINGS** (verify-report #201) — 28/28 tasks, 66/66 tests, build verde, non-regression surface **vacío**.
- **Delivery**: single-pr / size:exception (decisión #198 — quinto size:exception consecutivo aprobado por el usuario)
- **Spec capability**: `frontend-ui-foundation` (**NEW**, 14 REQs, 24 scenarios). Primera entrada; no hay slices previos que extender.

## Source of truth (canonical spec creada)

El delta de spec era un **spec completo nuevo** (no un delta sobre una spec existente). Se copió al árbol canónico sin append — este es el contenido inicial de la capability.

- **`openspec/specs/frontend-ui-foundation/spec.md`** — spec canónica **CREADA** (NEW capability). Contiene los 14 REQs + 24 scenarios completos: Mantine stack additions, Theme central, AppShell layout, Dark mode toggle, Notifications global, Login migrado, Dashboard migrado, Groups migrado (Table), GroupDetail migrado (Tabs), Logout con notificación, Helpers de test, Smoke tests, Documentación, No regresión de capas no migradas.

El delta idéntico se preserva en `openspec/changes/archive/2026-09-07-frontend-refresh/specs/frontend-ui-foundation/spec.md` como audit trail (convención del repo: `publications-slice2`, `publications-slice3`).

### Resumen de la sync

| Spec canónica | Acción | Detalle |
|---------------|--------|---------|
| `openspec/specs/frontend-ui-foundation/spec.md` | **CREADA** (NEW) | 14 REQs / 24 scenarios copiados completos del delta (no append — es la entrada inicial). Contenido idéntico al audit trail en el archive folder. |

**Verificación pre-archive**: `openspec/specs/` NO contenía `frontend-ui-foundation/` antes de este archive (confirmado por `Get-ChildItem openspec/specs/`). Cero colisiones. Las specs existentes (`frontend-auth`, `frontend-dashboard`, `frontend-routing`, `frontend-moderation`) NO se modifican — las migraciones de Login/Dashboard/Groups/GroupDetail son detalle de implementación detrás de sus REQs existentes (ver proposal §Capabilities "Modified Capabilities: None").

## Contenido del archive

```
2026-09-07-frontend-refresh/
├── README.md                        (este archivo)
├── exploration.md                   (17 decisiones D1-D17, contexto heredado)
├── proposal.md                      (alcance + criterios de éxito, NEW capability)
├── design.md                        (16 decisiones D1-D16 + interfaces + risk table)
├── tasks.md                         (8 phases, 28 tasks, todas [x])
├── apply-report.md                  (22 files changed, gates verdes, 5 deviations)
├── verify-report.md                 (PASS WITH WARNINGS, 14 REQs compliance matrix)
└── specs/                           (audit trail del delta)
    └── frontend-ui-foundation/
        └── spec.md                  (delta idéntico al canonical — NEW capability)
```

## Observaciones Engram (project: telegrammanager)

Para trazabilidad, los artefactos del SDD viven en Engram con los siguientes IDs:

- `sdd/frontend-refresh/exploration` → observation **#193**
- `sdd/frontend-refresh/proposal` → observation **#194**
- `sdd/frontend-refresh/spec` → observation **#195** (frontend-ui-foundation, NEW full spec)
- `sdd/frontend-refresh/design` → observation **#196**
- `sdd/frontend-refresh/tasks` → observation **#197**
- `sdd/frontend-refresh/delivery-strategy` → observation **#198** (decision — size:exception aprobado)
- `sdd/frontend-refresh/apply-report` → observation **#199**
- `sdd/frontend-refresh/verify-report` → observation **#201**
- `sdd/frontend-refresh/archive-report` → observation **#203** (THIS observation)

## Commits en `feat/frontend-refresh` (11 commits ahead of main @ ace1f59)

1. `3321751` chore(frontend): add Mantine v7 + Tabler + PostCSS deps + postcss config
2. `3274796` feat(frontend): Mantine theme + notifications helpers + providers
3. `c44a212` feat(frontend): AppShell layout + dark mode toggle + logout notify
4. `4ad0b4c` feat(frontend): migrate LoginPage to Mantine with notifications
5. `9c11f87` feat(frontend): migrate DashboardPage to SimpleGrid of Cards
6. `2f253fc` feat(frontend): implement GroupsPage with real Mantine Table
7. `bfffab6` feat(frontend): migrate GroupDetailPage with Tabs
8. `d83326f` style(frontend): cleanup dead CSS rules (395 → 22 LOC)
9. `2bd6436` test(frontend): extend renderWithProviders + smoke tests
10. `54aae21` docs: README — UI library & Dark mode subsection
11. `a7f4714` chore(sdd): frontend-refresh slice 1 — apply-report + tasks marcados
12. `0d8c21e` chore(sdd): frontend-refresh slice 1 — artifacts de explore/propose/spec/design

## Métricas del change

| Métrica | Valor |
|---------|-------|
| Tasks (28) | 28/28 ✅ |
| Tests | 66/66 ✅ (13 files, 12.93s, **cero flakeos** paralelos) |
| `npm run build` | ✅ Bundle 169.62 kB gz (dentro de budget +150-200kb) |
| `go test ./...` | ✅ All 11 packages OK |
| Non-regression surface | ✅ 0 lines (git diff main vs `features/`, `lib/api-client.ts`, `lib/auth-context.tsx`, `backend/`, `migrations/`, unmigrated pages, `RequireAuth.tsx`) |
| Files changed | 22 (16 modified + 6 new) |
| Net LOC | +746 (1364 ins / 618 del) |
| Total diff vs main | 28 files / +2458/-618 (incluye 6 artifact files de openspec/) |

## Versión instalada de Mantine (D1)

Dependencias verificadas con `npm ls` (0 peer warnings, 0 ERESOLVE, NO fallback):

| Paquete | Versión instalada |
|---------|-------------------|
| `@mantine/core` | **7.17.8** |
| `@mantine/hooks` | **7.17.8** |
| `@mantine/notifications` | **7.17.8** |
| `@tabler/icons-react` | **3.46.0** |
| `postcss-preset-mantine` | **1.18.0** |
| `postcss-simple-vars` | **7.0.1** |

Peer dep Mantine v7 `react ^18 || ^19` cubre React 19.2.8 sin issues. La propuesta (#194) especificó `^7.17` (mínimo); el resolver instaló `7.17.8` (último parche de la línea).

## Deviations documentadas (5, todas aceptables — ninguna bloquea el archive)

1. **LoginPage dropped inline Alert** (mantiene notification only) — REQ-6 solo exige `notifyError`; Alert duplicado causaba "Found multiple elements" en el test.
2. **LoginPage `withAsterisk={false}`** — Mantine `<TextInput required>` agrega `<span aria-hidden>*</span>` que rompe `getByLabelText('Usuario')`. Validación sigue vía Zod.
3. **LoginPage `defaultValues: { username: '', password: '' }`** — RHF Controller sin defaults = `undefined`; Zod `min(1)` tiraba "expected string, received undefined" en vez del mensaje custom.
4. **GroupDetailPage acciones fuera de `Tabs.Panel` (3 tabs vs 4)** — `keepMounted={true}` no funciona en Mantine v7.17 (los `Tabs.Panel` inactivos se desmontan). Acciones en un Stack siempre-visible arriba de los Tabs; "Membresía y moderación" como `<Button component={Link}>` dentro del tab Detalle. ⚠️ **W-V1 — spec deviation REQ-9**: 3 tabs implementadas vs 4 especificadas (Detalle, Solicitudes, Logs; "Membresía y moderación" accesible vía button-link). Funcionalmente equivalente para el admin.
5. **GroupDetailPage telegram_id split + permissions joined string** — Test-friendly splits: `<Text>{telegram_id}</Text>` separado del label, `permissions.join(', ')` sin bullets. Sin impacto UX.

## W-V1 — WARNING documentado (no bloquea)

**`GroupDetailPage` 3 tabs vs 4 spec'd** (REQ-9 "Pestañas visibles"):
- Spec exige 4 `<Tabs.Tab>`: Detalle, Membresía y moderación, Solicitudes, Logs.
- Implementación: 3 `<Tabs.Tab>` (Detalle, Solicitudes, Logs) + siempre-visible "Acciones de moderación" Stack arriba + `<Button component={Link} to={/users}>` dentro del panel Detalle.
- Causa raíz: `keepMounted={true}` no funciona en Mantine v7.17 (`Tabs.Panel` inactivos se desmontan). Los tests no podían encontrar botones "Cerrar chat"/"Eliminar"/"Fijar" sin click previo en la tab.
- Test smoke `GroupDetailPage.test.tsx:72-75` verifica el button-link a /users. Admin puede navegar a la sección de membresía y moderación desde un botón discoverable.
- Funcionalmente equivalente, pero el conteo literal de tabs NO cumple la spec. **Reabrir en slice 2** si se quiere respetar el conteo literal (investigar `keepMounted` workaround en Mantine v8 o reescribir test setup).

## W1 fix (recomendación de slice 3 #190, integrada en este slice)

El verify-report slice 3 (#190) recomendó `test.pool: 'vmThreads'` para resolver 2 tests slice-2 flakeantes bajo carga jsdom paralela. Slice 1 integró el fix en `frontend/vite.config.ts`:

```ts
test: {
  pool: 'vmThreads',
  isolate: true,
  globals: true,
  // ...
}
```

**Resultado confirmado por verify-report #201**: 66 tests en 13s con **cero flakeos paralelos** (los 2 tests slice-2 flakeantes ya no fallan). W1 **resuelto**.

## Invariantes respetadas

- **AGENTS §11 (NO Redis)**: 0 dependencias Redis añadidas. Stack frontend puro.
- **AGENTS §14 (modular backend)**: backend NO tocado — `git diff main -- backend/` = 0 lines.
- **AGENTS §13.1 (goose migrations)**: 0 migraciones nuevas — `git diff main -- migrations/` = 0 lines.
- **AGENTS §17.1 (auth sin secretos)**: 0 secretos en código. Tokens del bot siguen via env vars.
- **AGENTS §18.1 (rate limits Telegram)**: N/A — slice 1 es frontend-only, sin llamadas a la Bot API.
- **AGENTS §21.1 (mockear TelegramService)**: N/A — slice 1 no toca `TelegramService`.
- **AGENTS §25 regla 18 (frontend no llama Telegram directo)**: confirmado — `features/*` no tocado, todo el wiring pasa por `lib/api-client.ts` (sin cambios).

## Sugerencias para slices futuros (no bloquean)

- **S-V1**: Agregar tests smoke dedicados para `GroupsPage` (REQ-8 — tabla con grupos + tabla cargando). Cobertura actual via wrapper extendido + App.test smoke. Bajo riesgo.
- **S-V2**: Vite rolldown warning `chunks >500kB`. Esperable con Mantine + Tabler; code-splitting queda para slice 2+ si la optimización se prioriza.
- **S-V3**: Mantine Notifications portal acumula divs en `document.body` entre tests (no afecta correctness, ruido de debugging en consola). Cleanup queda para slice 2 si se desea logs más limpios.

## Slice 2 (preview, fuera de scope)

El slice 2 de `frontend-refresh` (futuro) cubrirá:
- Migración visual de `GroupUsersPage`, `GroupRequestsPage`, `GroupLogsPage`, `PublicationsPage` (necesitan DatePicker, tablas avanzadas, dropdown filtros).
- Wiring de notifications a los ~15 hooks restantes usando el patrón establecido en este slice (`notifySuccess/notifyError` desde `lib/notifications.ts`).
- Resolución opcional de W-V1 (4 tabs) investigando `keepMounted` workaround.

## Next steps (orchestrator)

1. Merge `feat/frontend-refresh` → `main` (single-pr, size:exception aprobado per #198).
2. Push to remote.
3. Slice 1 cierra la fundación UI. Siguiente cambio: `frontend-refresh-slice2` cuando se priorice migración de páginas complejas + wiring de mutaciones (ver §"Slice 2 preview" arriba).

## Commit (this archive)

- Message: `chore(openspec): archive change frontend-refresh`
- Branch: `feat/frontend-refresh`
- Files: `openspec/specs/frontend-ui-foundation/spec.md` (NEW canonical) + `openspec/changes/archive/2026-09-07-frontend-refresh/` rename (includes `verify-report.md` tracked + `README.md` NEW) + `openspec/changes/frontend-refresh/` removed. **NO source code touched.**

## SDD Cycle Complete

El change ha sido planificado, implementado, verificado y archivado.
El branch `feat/frontend-refresh` queda listo para que el orchestrator
haga merge a `main` y push a remoto. **NO se hizo push ni merge** en este
paso (regla del archive phase).
