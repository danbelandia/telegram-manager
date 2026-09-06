# Proposal: groups-detection — paso 8: Detección/registro de grupos

## Intent

Los eventos `my_chat_member` ya llegan al bus pero nadie los consume.
Este cambio hace que el backend detecte y registre los grupos donde el
bot participa (AGENTS.md §6 y paso 8 del orden de implementación),
usando el `Repository` del paso 7 para persistir. Es el primer
consumidor de negocio del bus.

## Scope

### In Scope
- Extender `internal/telegram/update.go`: `Chat` y `OldChatMember` en
  `ChatMemberUpdated`; permisos `can_*` (`*bool`) en `ChatMember`.
- `internal/groups/events.go`: `HandleMyChatMember(ctx, repo, upd)`
  que mapea evento → `Group` y hace upsert.
- `internal/groups/events_test.go`: tests del mapeo/decisiones.
- Wiring en `main.go`: construir repo + handler en el bus (decisión 6
  del design del paso 7, diferida aquí).

### Out of Scope
- `getChatMemberCount`/refresco de `member_count` (paso 13).
- API REST de grupos (paso 10).
- Backfill de grupos donde el bot ya estaba antes del arranque
  (imposible por Bot API; se documenta como limitación).
- Enriquecer el modelo con `can_send_*` (restricted) — solo los `can_*`
  que el MVP usa para moderar.

## Capabilities

### New Capabilities
- `group-detection`: registro incremental de grupos administrables a
  partir de eventos `my_chat_member`.

### Modified Capabilities
- `groups`: el modelo ya existe (paso 7); no cambia su spec.

## Approach

Approach 1 de la exploración: extender el modelo Telegram solo en lo
que el evento trae (Chat + can_*), crear `internal/groups/events.go`
con la lógica de mapeo pura y testeable, y hacer wiring mínimo en
`main.go`. Reglas: registrar solo `supergroup`/`group`/`channel`;
`bot_status = NewChatMember.Status` (incluye left/kicked, el histórico
de la fila se conserva); permisos solo de los can_* presentes; no tocar
`member_count`.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `backend/internal/telegram/update.go` | Modified | +Chat, +OldChatMember, +can_* |
| `backend/internal/groups/events.go` | New | HandleMyChatMember + mapeo |
| `backend/internal/groups/events_test.go` | New | unit tests del mapeo con fake repo |
| `backend/cmd/server/main.go` | Modified | wiring repo + handler bus |
| `openspec/specs/group-detection/spec.md` | New | spec base (al archivar) |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Grupos pre-existentes al arranque no detectados | Alta (por API) | Documentado; chat_id manual como futuro |
| can_* con valores no booleanos (restricted) | Baja | Solo modelar los can_* del MVP con *bool |
| Fail silencioso del handler si el repo falla | Med | Loggear el error en el handler (slog.Error), no romper el bus |

## Rollback Plan

Revertir el wiring en main.go (volver solo al logging). El modelo
Telegram ampliado y `events.go` son aditivos; la migración 00002 ya
existe y no cambia.

## Dependencies

- Cambio `groups` archivado (paso 7): `Repository.UpsertByTelegramID`.
- Ninguna librería nueva.

## Success Criteria

- [ ] `go build/vet/test ./...` verde.
- [ ] Los unit tests de `events.go` cubren: ignorar private, bot_status
      left/kicked, permisos solo presentes, chat supergroup registrado.
- [ ] En vivo: agregar el bot a un grupo genera la fila en `groups`
      (verificado vía logs o query).
- [ ] El handler no rompe el bus si el repo falla (log, no panic).
- [ ] Tests de integración existentes del repo siguen verdes.