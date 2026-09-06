# Exploration: groups-detection — paso 8: Detección/registro de grupos

## Current State

- **Modelo Group listo** (paso 7 archivado): tabla `groups`,
  `internal/groups.Repository` con `UpsertByTelegramID` (ON CONFLICT),
  `List`, `GetByTelegramID`. `main.go` todavía NO construye el repo
  (wiring diferido a este paso, decisión 6 del design).
- **Transporte listo** (telegram-events): el bus despacha updates; el
  único handler en `main.go` es logging. `my_chat_member` ya llega al
  bus.
- **Referencia Bot API** (docs/telegram_api_reference.md §3): **No hay
  forma de listar "todos los grupos donde está el bot"**; los grupos se
  descubren incrementalmente por eventos `my_chat_member`. Cuando el
  bot es agregado/admin con el backend corriendo, Telegram envía el
  evento.
- **Brecha en el modelo Telegram** (`internal/telegram/update.go`):
  - `ChatMemberUpdated` tiene `From`, `NewChatMember`, `Date` pero le
    falta **`Chat`** (el chat al que pertenece el cambio) y
    `OldChatMember`. Sin `Chat` no sabemos qué grupo registrar.
  - `ChatMember` solo modela `Status` + `User`; faltan los permisos
    `can_*` (poblados por Telegram solo cuando el bot es admin).

## Affected Areas

- `backend/internal/telegram/update.go` — agregar `Chat *Chat` y
  `OldChatMember ChatMember` a `ChatMemberUpdated`; agregar `can_*`
  como `*bool` a `ChatMember` (ausente vs false).
- `backend/internal/groups/events.go` (nuevo) — función
  `HandleMyChatMember(ctx, repo, upd *telegram.ChatMemberUpdated)` que
  mapea update → `Group` y upsert.
- `backend/internal/groups/events_test.go` (nuevo) — unit tests del
  mapeo con fake repo (interfaz de repo del lado groups consumidor) o
  integración con Postgres + stub.
- `backend/cmd/server/main.go` — construir repo + registrar handler en
  el bus (wiring diferido).

## Approaches

1. **Handler en `internal/groups` + extender modelo Telegram** —
   `events.go` con la lógica de mapeo (evento→Group) testeable; el
   wiring en main.go queda mínimo (construir repo, registrar handler).
   - Pros: lógica de negocio en el dominio correcto; testeable sin
     HTTP ni Telegram real (mapeo puro); main.go no se ensucia.
   - Cons: hay que tocar el modelo de Telegram (necesario igualmente).
   - Effort: Low-Medium

2. **Handler anónimo directo en `main.go`** (sin paquete nuevo)
   - Pros: menos archivos.
   - Cons: lógica de mapeo no testeable; main.go crece; viola la
     separación del spec §14. Descartado.

3. **Refresco proactivo con `getChatMemberCount`/`getChat` al detectar**
   — además del upsert básico, llamar a la Bot API para member_count.
   - Pros: enriquece el registro.
   - Cons: agrega llamadas de red y complejidad al paso; AGENTS §6 dice
     member_count "cuando Telegram permita obtenerla" — se puede poblar
     en un paso posterior de detalle. Fuera de scope.
   - Effort: Medium

## Recommendation

**Approach 1**: extender `update.go` (Chat + OldChatMember + can_*) y
crear `internal/groups/events.go` con `HandleMyChatMember`. Reglas de
mapeo:
- Registrar solo chats `supergroup`/`group`/`channel`; ignorar
  `private` (DM del bot).
- `bot_status` = `NewChatMember.Status` siempre (incluye left/kicked:
  el registro conserva el histórico, la API del paso 10 decide qué
  listar).
- `BotPermissions`: solo los can_* presentes en el update (map
  `map[string]bool`); nil si el bot no es admin.
- `member_count` no se toca (paso 13/refresco futuro).
- Consistente con la referencia: descubrimiento incremental, sin
  backfill de grupos donde el bot ya estaba antes del arranque (se
  documenta como limitación).

Wiring en main.go: `repo := groups.NewRepository(db)` +
`bus.Handle(...)` que filtra `update.MyChatMember != nil`.

## Risks

- **Limitación conocida**: grupos donde el bot ya estaba ANTES de
  arrancar el backend no generan evento → no se detectan. Documentar
  en README/spec; mitigación futura (fuera de MVP): panel para
  ingresar chat_id manual.
- El JSON de Telegram puede traer can_* con valores no booleanos en
  algunos estados (ej. `can_send_*` en restricted) — modelar solo los
  relevantes del MVP (`can_delete_messages`, `can_restrict_members`,
  `can_pin_messages`, `can_invite_users`, `can_promote_members`,
  `can_change_info`) con `*bool` para no romper decode.
- `OldChatMember` se modela pero no se usa en el MVP — solo para
  completitud del decode (omitempty no afecta).

## Ready for Proposal

Sí — el orchestrator debe proponer el cambio `groups-detection` con
scope: modelo Telegram ampliado + `internal/groups/events.go` + wiring
en main.go; sin getChatMemberCount (futuro), sin API REST (paso 10),
sin backfill.