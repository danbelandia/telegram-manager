# Proposal: Frontend de moderación de grupos

## Intent

Completar las fases 13-18 de AGENTS §26 en el panel: las rutas
`/groups/:id/users`, `/groups/:id/requests` y `/groups/:id/logs` hoy son
placeholders (spec frontend-dashboard) y el detalle de grupo no permite
acciones. El backend ya expone toda la API (specs group-administration,
join-requests, admin-logs, telegram-moderation): falta el frontend que la
consuma. El MVP quedará funcional de punta a punta (DoD §27).

## Scope

### In Scope
- `GroupUsersPage` real: lista de administradores (`GET /groups/:id/users`),
  lookup puntual por `userId` (`?userId=`), acciones ban/unban/mute/unmute
  con confirmación para ban y feedback de resultado.
- `GroupDetailPage`: acciones lock/unlock (abrir/cerrar chat, AGENTS §9) y
  delete/pin de mensaje por `messageId` manual, con confirmación para delete.
- `GroupRequestsPage` real: lista de solicitudes `pending` + approve/reject.
- `GroupLogsPage` real: historial de logs del grupo.
- Hooks TanStack Query + funciones api en `features/` (mismo patrón que
  `features/groups`), todo vía `lib/api-client.ts`.
- Tests por página clave con `mockFetchRoutes` (guía frontend §4).

### Out of Scope
- Listado completo de miembros (Telegram no lo expone, AGENTS §7):
  la página muestra admins + lookup, ya documentado en el backend.
- Listado de mensajes del grupo (no existe en Bot API): delete/pin usa
  `messageId` que provee el admin.
- UI kit / tablas estilizadas: CSS plano actual, solo lo necesario.
- Fases 21-24 (publicaciones, moderación automática, automatizaciones).

## Capabilities

### New Capabilities
- `frontend-moderation`: consumo del panel de las acciones de moderación
  (ban/unban/mute/unmute, delete/pin, lock/unlock), vistas de usuarios,
  solicitudes de ingreso y logs de un grupo, con TanStack Query.

### Modified Capabilities
- `frontend-dashboard`: la Requirement "Secciones hijas placeholder" deja
  de ser placeholder; el detalle ahora incluye acciones lock/unlock.

## Approach

- `features/moderation/{types,api,hooks}.ts` tipando los DTO del backend
  (consultar `internal/api` handlers para el contrato exacto).
- Mutaciones con `useMutation` y `queryClient.invalidateQueries` para
  refrescar logs/usuarios tras cada acción.
- Errores del envelope §18 mapeados a mensajes legibles en el panel.
- Confirmación nativa (`window.confirm`) para ban y delete (AGENTS §4
  seguridad: confirmar acciones destructivas).
- `mockFetchRoutes` por URL para tests (patrón ya establecido).

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `frontend/src/features/moderation/` | New | types, api, hooks de moderación + users + requests + logs |
| `frontend/src/pages/GroupUsersPage.tsx` | Modified | placeholder → vista real |
| `frontend/src/pages/GroupRequestsPage.tsx` | Modified | placeholder → vista real |
| `frontend/src/pages/GroupLogsPage.tsx` | Modified | placeholder → vista real |
| `frontend/src/pages/GroupDetailPage.tsx` | Modified | agrega lock/unlock + delete/pin |
| `frontend/src/pages/*.test.tsx` | New | tests con mockFetchRoutes |
| `frontend/src/styles.css` | Modified | estilos mínimos para tablas/estados |
| `docs/frontend-react-skill.md` | Modified | registrar patrón useMutation + invalidate |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| DTO backend distinto al tipado frontend | Med | Definir types leyendo los handlers reales (`internal/api`) antes de escribir |
| Errores §18 confusos al usuario | Med | Mapear code→mensaje legible reutilizable |
| Mutaciones sin invalidar queries → datos stale | Med | invalidate de logs y users tras cada acción |

## Rollback Plan

Revert del commit del cambio (rama local sin remote, historial lineal).
La API backend queda intacta; el frontend vuelve a placeholders sin
cambios de contrato.

## Dependencies

- Backend ya desplegado en compose con todas las rutas de moderación.
- Rama actual `feat/frontend-panel-dashboard` → crear
  `feat/frontend-moderation` desde ella (patrón de PRs encadenados).

## Success Criteria

- [ ] `npm test` pasa con tests de las 4 páginas (users, requests, logs, detail con acciones).
- [ ] `npm run build` (tsc + vite) sin errores.
- [ ] Desde el panel real: ban/unban/mute/lock/unlock/approve/reject ejecutan la acción y el log se ve en `/groups/:id/logs`.
- [ ] `go test ./...` en backend sin romper (solo frontend).
- [ ] DoD §27: acciones de moderación soportadas ejecutables desde el panel.