# Tasks: Frontend de moderación de grupos

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~700-900 |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | 3 PRs encadenados (feature-branch-chain) |
| Delivery strategy | auto-chain |
| Chain strategy | feature-branch-chain |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: feature-branch-chain
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Notes |
|------|------|-----------|-------|
| 1 | Fundación `features/moderation/` + `GroupUsersPage` | PR 1 | base = tracker `feat/frontend-moderation`; types+api+hooks+error, users page + tests |
| 2 | `GroupRequestsPage` + `GroupLogsPage` | PR 2 | base = PR 1 branch; requests + logs pages + tests |
| 3 | Acciones en detalle (lock/unlock + delete/pin) + docs | PR 3 | base = PR 2 branch; GroupDetailPage + tests pendientes + styles + docs guía |

Convención (idéntica a frontend-panel): cada PR SÍ verifica (npm test + build), el tracker acumula la integración final, solo el tracker mergea a main.

## Phase 1: Fundación del dominio moderation

- [ ] 1.1 Crear `frontend/src/features/moderation/types.ts` con `GroupUser`, `JoinRequest`, `LogEntry` (DTO reales de handlers backend; ver design)
- [ ] 1.2 Crear `frontend/src/features/moderation/api.ts`: `listGroupUsers`, `getGroupUser(userId)`, `banUser`, `unbanUser`, `muteUser`, `unmuteUser`, `deleteMessage`, `pinMessage`, `lockGroup`, `unlockGroup`, `listJoinRequests`, `approveJoinRequest`, `rejectJoinRequest`, `listGroupLogs`
- [ ] 1.3 Crear `frontend/src/features/moderation/hooks.ts`: `useGroupUsers`, `useGroupUser`, `useJoinRequests`, `useGroupLogs` (useQuery retry:false) + mutations `useBanUser`, `useUnbanUser`, `useMuteUser`, `useUnmuteUser`, `useDeleteMessage`, `usePinMessage`, `useLockGroup`, `useUnlockGroup`, `useApproveJoinRequest`, `useRejectJoinRequest`
- [ ] 1.4 Invalidación en mutations: ban/unban/mute/unmute → `['groups', id, 'users']` + `['groups', id, 'logs']`; delete/pin → `['groups', id, 'logs']`; lock/unlock → `['groups', id]` + logs; approve/reject → `['groups', id, 'requests']` + logs
- [ ] 1.5 Crear `frontend/src/features/moderation/error.ts` con `formatModerationError` (fallback legible si el mensaje del backend no llega)

## Phase 2: Vistas de usuarios y acciones

- [ ] 2.1 Reescribir `frontend/src/pages/GroupUsersPage.tsx`: lista de admins (estados loading/vacío/error + reintento), nota de limitación Bot API, lookup por `userId`
- [ ] 2.2 Acciones por usuario: botones Banear/Desmutear/Mutear/Desmutear, `window.confirm` para ban y mute, feedback de resultado y error §18
- [ ] 2.3 Crear `frontend/src/pages/GroupUsersPage.test.tsx`: render admins, vacío, error+reintento, ban confirmado (POST + invalidación), ban cancelado (sin fetch)

## Phase 3: Solicitudes y logs

- [ ] 3.1 Reescribir `frontend/src/pages/GroupRequestsPage.tsx`: lista (estados), aprobar/rechazar pendientes, feedback + error "ya fue decidida"
- [ ] 3.2 Crear `frontend/src/pages/GroupRequestsPage.test.tsx`: lista, approve, reject, concurrencia (VALIDATION_ERROR)
- [ ] 3.3 Reescribir `frontend/src/pages/GroupLogsPage.tsx`: lista de logs desc por `created_at`, acción/status/target/error/fecha
- [ ] 3.4 Crear `frontend/src/pages/GroupLogsPage.test.tsx`: render logs, vacío, error

## Phase 4: Acciones en detalle + integración

- [ ] 4.1 Modificar `frontend/src/pages/GroupDetailPage.tsx`: botones 🔓/🔒 (confirm para lock) + form delete/pin con `messageId` (confirm para delete)
- [ ] 4.2 Crear `frontend/src/pages/GroupDetailPage.test.tsx`: grupo encontrado, grupo inexistente (tests pendientes del verify), lock confirmado, delete confirmado, delete cancelado
- [ ] 4.3 `frontend/src/styles.css`: estilos mínimos tablas/estados/acciones
- [ ] 4.4 `npm test` y `npm run build` verdes
- [ ] 4.5 Actualizar `docs/frontend-react-skill.md`: patrón useMutation + invalidate + stub confirm
- [ ] 4.6 Actualizar `openspec/changes/frontend-moderation/state.yaml` (deviations, test_status, prs)

## Next Steps (fuera de este cambio)

- [ ] DoD §27: documentación de ejecución local (paso 20 del spec) queda pendiente