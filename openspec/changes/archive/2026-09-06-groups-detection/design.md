# Design: groups-detection — paso 8: Detección/registro de grupos

## Technical Approach

Approach 1 de la exploración: ampliar `internal/telegram/update.go`
con lo que el evento trae (Chat, OldChatMember, can_*) y crear
`internal/groups/events.go` con la lógica de mapeo pura y testeable.
Wiring mínimo en `main.go` (decisión 6 del design del paso 7, diferida
acá).

## Architecture Decisions

| # | Decisión | Alternativas | Por qué |
|---|----------|--------------|---------|
| 1 | `ChatMemberUpdated.Chat *Chat` + `OldChatMember ChatMember` | solo agregar Chat | OldChatMember completa el decode del evento real (omitempty evita sorpresas); no se usa aún pero es parte del contrato del payload |
| 2 | `can_*` como `*bool` en `ChatMember` (can_delete_messages, can_restrict_members, can_pin_messages, can_invite_users, can_promote_members, can_change_info) | bool plano | `*bool` distingue "ausente" (nil, bot no admin) de "false"; evita sobreescribir permisos con valores vacíos |
| 3 | `HandleMyChatMember(ctx, upd, g *groups.Group) error` como función pura en `internal/groups` | método con repo adentro, handler en main | La lógica de negocio queda en el dominio groups; el wiring (construir repo, llamar al repo) es responsabilidad de main.go y del handler anónimo del bus |
| 4 | Handler del bus en main.go: filtra `u.MyChatMember != nil`, llama `groups.HandleMyChatMember` y luego `repo.UpsertByTelegramID`; si error → `slog.Error` (no romper el bus) | handler dentro de events.go con repo inyectado | El bus es de infraestructura; el handler mínimo usa la función pura + repo; la resiliencia (log, no panic) es del wiring |
| 5 | GitHub bug: en `UpsertByTelegramID`, si `BotPermissions` es nil, NO pisar el valor previo | `COALESCE`/solo actualizar si no nil | Espec "Permisos SHOULD quedar sin poblar (mantener valor previo)": cuando el bot no trae can_* (ej. member), no sobreescribir JSONB con NULL |

## Data Flow

```
Telegram ──my_chat_member──> Bus ──> handleMyChatMember (main.go)
   │                                      │
   │                     filtra MyChatMember != nil
   │                                      │
   │                    groups.HandleMyChatMember(ctx, upd, &g)  (mapeo puro)
   │                                      │
   │                    repo.UpsertByTelegramID(ctx, &g)  ──> PostgreSQL groups
   │                                      │
   │                    error? ──> slog.Error (el bus sigue)
```

## File Changes

| File | Acción | Descripción |
|------|--------|-------------|
| `backend/internal/telegram/update.go` | Modify | +Chat, +OldChatMember, +can_* *bool |
| `backend/internal/groups/events.go` | Create | `HandleMyChatMember(ctx, upd, *Group) error` puro |
| `backend/internal/groups/events_test.go` | Create | unit tests del mapeo (private, tipos, can_*, históricos) |
| `backend/cmd/server/main.go` | Modify | construir repo + handler del bus + slog |

## Interfaces / Contracts

```go
// internal/telegram/update.go (adiciones)
type ChatMemberUpdated struct {
    Chat           *Chat      `json:"chat,omitempty"`
    From           User       `json:"from"`
    OldChatMember  ChatMember `json:"old_chat_member,omitempty"`
    NewChatMember  ChatMember `json:"new_chat_member"`
    Date           int64      `json:"date"`
}

type ChatMember struct {
    Status string   `json:"status"`
    User   *User    `json:"user,omitempty"`
    CanDeleteMessages   *bool `json:"can_delete_messages,omitempty"`
    CanRestrictMembers  *bool `json:"can_restrict_members,omitempty"`
    CanPinMessages      *bool `json:"can_pin_messages,omitempty"`
    CanInviteUsers      *bool `json:"can_invite_users,omitempty"`
    CanPromoteMembers   *bool `json:"can_promote_members,omitempty"`
    CanChangeInfo       *bool `json:"can_change_info,omitempty"`
}

// internal/groups/events.go — función pura, no conoce el repo
func HandleMyChatMember(ctx context.Context, upd *telegram.ChatMemberUpdated, g *Group) error
// Llena g (TelegramID, Title, Username, Type, BotStatus, BotPermissions)
// y devuelve error si el update es privado/sin chat o sin new_chat_member.

// internal/cmd/server/main.go — handler del bus (resiliente)
bus.Handle(func(u *telegram.Update) {
    if u.MyChatMember == nil { return }
    var g groups.Group
    if err := groups.HandleMyChatMember(ctx, u.MyChatMember, &g); err != nil {
        slog.Warn("groups: ignoring my_chat_member", "err", err)
        return
    }
    if err := repo.UpsertByTelegramID(ctx, &g); err != nil {
        slog.Error("groups: upsert failed", "telegram_id", g.TelegramID, "err", err)
    }
})
```

**UpsertByTelegramID (modificación):** si `BotPermissions == nil`, el
`SET bot_permissions` NO se aplica (se preserva el valor previo):

```sql
ON CONFLICT (telegram_id) DO UPDATE SET
    title = EXCLUDED.title,
    username = EXCLUDED.username,
    type = EXCLUDED.type,
    member_count = EXCLUDED.member_count,
    -- bot_permissions: solo si EXCLUDED.bot_permissions IS NOT NULL
    bot_status = EXCLUDED.bot_status,
    updated_at = now()
```

El `bot_status` se actualiza siempre (refleja el último estado). Si nada
cambió, `updated_at` igual se renueva, sin problema práctico.

## Testing Strategy

| Capa | Qué | Cómo |
|------|-----|------|
| Unit (nuevo) | `events_test.go` | fake `telegram.ChatMemberUpdated` + `groups.Group`; verifica: private ignorado (error), supergroup mapeado, `bot_status` left/kicked, permisos presentes copiados, permisos ausentes → nil (sin pisar), username opcional |
| Unit (repositorio) | modificar `repository_test.go` | escenario "permisos nil no pisan valor previo": upsert con permisos, luego upsert con nil → sigue con los viejos; sin cambio en otros campos |
| Build/tests | suite completa | `go test ./... -count=1`, `go vet`, `gofmt` |
| Integración manual (en vivo) | agregar el bot a un grupo (si se puede desde Telegram) | verificar fila nueva en `groups` vía logs |

## Migration / Rollout

No hay migración nueva (tabla `groups` ya existe de 00002). El wiring
se activa al arrancar: los eventos `my_chat_member` que lleguen desde
ese momento se registran. Sin backfill (limitación documentada del
spec). Rollback: quitar el handler del bus → vuelve al estado anterior.

## Open Questions

- Limitación de la Bot API (grupos pre-existentes no detectables):
  queda documentada; ¿el paso 10 (API) debería permitir chat_id manual?
  → fuera de alcance de este cambio.