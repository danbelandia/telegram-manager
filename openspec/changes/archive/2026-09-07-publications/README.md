# Archivado: publications (Slice 1 — Publish-Now Text)

- **Change**: `publications` (Slice 1 — publish-now text)
- **Archived**: 2026-09-07
- **Branch**: `feat/publications` (base `main` @ `c835739`)
- **Verdict**: **PASS-WITH-NOTES** (22/22 tasks, go test/vet/gofmt y npm test/build verdes; 2 notas no bloqueantes de cobertura de test — ver `publications/verify-report.md`)
- **Commit de archivado**: `docs(sdd): archivado publications — spec canonica ya en openspec/specs (slice 1)`

## Spec canónica

La spec canónica de este change NO se mueve al archivo. Vive de forma
permanente en:

```
openspec/specs/publications/spec.md
```

(Distinto de la convención openspec estricta de "specs dentro del
change" — desviación ya documentada en el archivado previo
`2026-09-07-frontend-moderation`.)

## Contenido del archive

```
2026-09-07-publications/
├── README.md
└── publications/
    ├── exploration.md     (exploración previa al proposal)
    ├── proposal.md
    ├── design.md
    ├── tasks.md           (22/22 completadas)
    ├── apply-report.md    (implementación completa)
    └── verify-report.md   (PASS-WITH-NOTES)
```

## Observaciones Engram (para trazabilidad)

- `sdd/publications/proposal` → observation #162
- `sdd/publications/spec` → observation #163
- `sdd/publications/design` → observation #164
- `sdd/publications/tasks` → observation #165
- `sdd/publications/apply-report` → observation #167
- `sdd/publications/verify-report` → observation #170
- `sdd/publications/archive-report` → observation #171