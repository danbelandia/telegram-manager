# Group Detection Specification

## Purpose

Registrar incrementalmente, en la tabla `groups`, los chats donde el
bot participa (AGENTS.md §6), a partir de los eventos `my_chat_member`
que Telegram entrega. Es el primer consumidor de negocio del bus de
eventos. No hay forma de listar todos los grupos del bot por Bot API,
así que la detección es solo por eventos; los chats donde el bot ya
estaba antes de arrancar el backend no generan evento (limitación
documentada).

## Requirements

### Requirement: Mapeo del evento a un grupo

El sistema MUST, ante un update `my_chat_member`, mapear el chat y el
nuevo estado del bot (`new_chat_member`) a un `Group` y persistirlo con
`UpsertByTelegramID`. El `bot_status` del grupo SHOULD ser el status
del `new_chat_member`.

#### Scenario: Bot promovido a administrador

- GIVEN un update `my_chat_member` con el bot como `administrator` en
  un supergrupo
- WHEN se procesa el evento
- THEN se persiste el grupo con `bot_status = administrator` y `title`
  del chat

#### Scenario: Bot agregado como miembro

- GIVEN un update `my_chat_member` con el bot como `member`
- WHEN se procesa el evento
- THEN se persiste el grupo con `bot_status = member`

### Requirement: Filtrado de chats

El sistema MUST ignorar los chats de tipo `private` (DM del bot) y
SHOULD registrar los tipos `supergroup`, `group` y `channel`. Un update
sin `new_chat_member` o sin chat MUST ignorarse sin error.

#### Scenario: Chat privado ignorado

- GIVEN un update `my_chat_member` con un chat `private`
- WHEN se procesa el evento
- THEN no se persiste ningún grupo y no hay error

#### Scenario: Update sin estado del bot

- GIVEN un update con `new_chat_member` ausente
- WHEN se procesa el evento
- THEN se ignora el evento sin persistir ni fallar

### Requirement: Conservación del histórico de estado

El sistema MUST persistir el `bot_status` tal como llega, incluso
`left` o `kicked`. El registro conserva el último estado conocido; no
borra la fila.

#### Scenario: Bot removido del grupo

- GIVEN un update `my_chat_member` con el bot como `left`
- WHEN se procesa el evento
- THEN el grupo se actualiza con `bot_status = left` y sigue existiendo

### Requirement: Permisos del bot

El sistema SHOULD copiar al grupo los permisos `can_*` presentes en el
evento. Si el bot no es administrador, los permisos SHOULD quedar sin
poblar (se mantiene el valor previo del grupo).

#### Scenario: Permisos de administrador

- GIVEN un update donde el bot es admin con `can_restrict_members`,
  `can_pin_messages` y `can_delete_messages`
- WHEN se procesa el evento
- THEN el grupo queda con `bot_permissions` conteniendo esos tres
  permisos

#### Scenario: Bot sin permisos previos

- GIVEN un update donde el bot pasa a `member` y sin permisos en el evento
- WHEN se procesa el evento
- THEN `bot_permissions` del grupo queda sin datos

### Requirement: Persistencia de identidad del chat

El sistema MUST copiar del evento el `title`, `username` (si lo tiene)
y `type` del chat al grupo, y NO debe modificar `member_count` (la
cantidad de miembros se obtiene en un paso posterior).

#### Scenario: Grupo con username

- GIVEN un supergrupo con `username` público
- WHEN se procesa el evento
- THEN el grupo queda con ese `username` y su `title`

#### Scenario: `member_count` no se toca

- GIVEN un grupo ya persistido con `member_count` no nulo
- WHEN se procesa un evento de ese grupo
- THEN `member_count` se mantiene sin cambios

### Requirement: Resiliencia del bus

El sistema MUST NOT romper la entrega del bus si el persistido falla:
el handler SHOULD loggear el error y dejar pasar los demás eventos.

#### Scenario: Fallo de persistencia no bloqueante

- GIVEN el repositorio devuelve error al persistir
- WHEN se procesa el evento
- THEN el handler loggea el error y el procesamiento continúa (no
  paniquea, no corta el bus)