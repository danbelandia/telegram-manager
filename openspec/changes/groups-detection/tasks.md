# Tasks: groups-detection — paso 8: Detección/registro de grupos

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~280-340 |
| 400-line budget risk | Low |
| Chained PRs recommended | No |
| Suggested split | single PR en feat/groups-detection (base feat/groups) |
| Delivery strategy | ask-on-risk |
| Chain strategy | feature-branch-chain (misma chain; aún sin remote) |

Decision needed before apply: Yes
Chained PRs recommended: No
Chain strategy: feature-branch-chain
400-line budget risk: Low

### Suggested Work Units

| Unit | Goal | Likely PR | Notes |
|------|------|-----------|-------|
| 1 | Modelo Telegram + events.go + wiring (todo el paso 8) | PR groups-detection | single PR bajo 400 líneas; base = feat/groups |

## Phase 1: Foundation (modelo Telegram ampliado)

- [ ] 1.1 En `backend/internal/telegram/update.go`: agregar `Chat *Chat` (json `chat,omitempty`) y `OldChatMember ChatMember` (json `old_chat_member,omitempty`) a `ChatMemberUpdated`.
- [ ] 1.2 En `update.go`: agregar a `ChatMember` los campos `CanDeleteMessages`, `CanRestrictMembers`, `CanPinMessages`, `CanInviteUsers`, `CanPromoteMembers`, `CanChangeInfo` como `*bool` con tags json `omitempty`.

## Phase 2: Core (mapeo + repo preserva permisos)

- [ ] 2.1 Crear `backend/internal/groups/events.go`: función pura `HandleMyChatMember(ctx, upd *telegram.ChatMemberUpdated, g *Group) error`. Ignora chats `private` o sin `Chat`/`NewChatMember` (error nil); llena TelegramID, Title, Username, Type, BotStatus (siempre, incluye left/kicked), BotPermissions con los can_* presentes (nil si todos ausentes); NO toca MemberCount.
- [ ] 2.2 Modificar `UpsertByTelegramID` en `backend/internal/groups/repository.go`: en el `ON CONFLICT`, actualizar `bot_permissions` solo si el valor entrante no es nil (preservar valor previo). `bot_status`, `title`, `username`, `type` y `updated_at` se actualizan siempre.

## Phase 3: Testing

- [ ] 3.1 Crear `backend/internal/groups/events_test.go`: casos — chat private → ignorado (g sin cambios, err nil); supergroup/group/channel → mapeados; `bot_status` left/kicked conservados; permisos presentes copiados al Group; permisos ausentes → BotPermissions nil; username opcional.
- [ ] 3.2 Modificar `backend/internal/groups/repository_test.go`: test "permisos nil no pisan valor previo" (upsert con permisos → upsert con nil → permisos previos intactos; otros campos sí actualizan).
- [ ] 3.3 Verificar integración con los unit tests del mapeo + repo existentes (go test ./internal/groups/ -count=1, con Postgres para integración).

## Phase 4: Wiring y verification

- [ ] 4.1 En `backend/cmd/server/main.go`: construir `groupsRepo := groups.NewRepository(db)` y registrar handler del bus: si `u.MyChatMember == nil` → return; llamar `groups.HandleMyChatMember`; si error → `slog.Warn` y return; si `repo.UpsertByTelegramID` falla → `slog.Error` (el bus sigue, no romper entrega).
- [ ] 4.2 `go build ./...`, `go vet ./...`, `go test ./... -count=1`, `gofmt -l .` limpio.
- [ ] 4.3 Verificación en vivo opcional: agregar el bot a un grupo y confirmar fila nueva en `groups` (vía logs de upsert o query directa).

## Notas

- Sin migración nueva (tabla `groups` existe).
- Los tests de mapeo (`events_test.go`) son unit tests puros, no requieren Postgres.
- Commit: `feat(groups): detectar y registrar grupos desde my_chat_member`.