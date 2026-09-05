# Referencia Telegram Bot API --- Telegram Group Manager

> Este documento es una referencia **curada**, no la documentación completa
> de Telegram. Solo cubre lo que el MVP definido en la especificación del
> proyecto necesita. Si el agente necesita un método que no aparece aquí,
> debe consultar la documentación oficial antes de implementarlo:
> https://core.telegram.org/bots/api --- nunca asumir que un método existe
> o que acepta ciertos parámetros.

------------------------------------------------------------------------

## 0. Conceptos base

- Todas las peticiones van sobre HTTPS:
  `https://api.telegram.org/bot<TOKEN>/METODO`
- La respuesta siempre trae un campo `ok` (booleano). Si `ok` es `false`,
  el error viene en `description` y `error_code`.
- Los IDs de chat y de usuario pueden superar los 32 bits — en Go, usar
  `int64`, nunca `int32`.
- Ningún método aquí requiere librerías externas obligatorias; se puede
  hacer con `net/http` + `encoding/json` sobre la API REST directamente.

------------------------------------------------------------------------

## 1. Recepción de eventos: webhook vs. polling

Corresponde a la sección 19 del spec del proyecto (`TELEGRAM_MODE`).

### `setWebhook`

Registra la URL a la que Telegram enviará los updates.

| Parámetro | Tipo | Obligatorio | Notas |
|---|---|---|---|
| `url` | string | Sí | URL HTTPS pública. String vacío elimina el webhook. |
| `secret_token` | string | No | 1--256 caracteres (`A-Z a-z 0-9 _ -`). Telegram lo reenvía en el header `X-Telegram-Bot-Api-Secret-Token` en cada request. **Usar esto para validar que el request viene realmente de Telegram** (ver sección 19.1 del spec). |
| `max_connections` | integer | No | 1--100, default 40. Conexiones simultáneas. |
| `drop_pending_updates` | boolean | No | Descarta updates acumulados al reconfigurar. |

Puertos soportados para el webhook: **443, 80, 88, 8443** únicamente.

### `deleteWebhook`

Quita el webhook para volver a `getUpdates` (polling). Útil para
desarrollo local.

### `getWebhookInfo`

Sin parámetros. Devuelve estado actual: URL configurada,
`pending_update_count`, y `last_error_message` si el último envío falló
— útil para un endpoint de diagnóstico interno.

### `getUpdates` (modo polling)

| Parámetro | Tipo | Notas |
|---|---|---|
| `offset` | integer | Debe ser mayor en 1 al último `update_id` recibido, para no reprocesar eventos. |
| `timeout` | integer | Segundos de long polling. En 0 hace short polling (solo para pruebas). |
| `allowed_updates` | array de string | Filtrar tipos de evento relevantes (ver sección 2). |

**Importante:** `getUpdates` y el webhook son mutuamente excluyentes. Si
hay un webhook activo, `getUpdates` no devuelve nada hasta que se borre
con `deleteWebhook`.

------------------------------------------------------------------------

## 2. Tipos de update relevantes para el MVP

Al configurar `allowed_updates` (en `setWebhook` o `getUpdates`), limitar
a los tipos que el MVP procesa, en vez de recibir todo:

| Tipo de update | Cuándo llega | Sección del spec relacionada |
|---|---|---|
| `message` | Mensaje nuevo en un chat donde está el bot | 8 (moderación) |
| `chat_member` | Cambio de estado de un miembro (entra, sale, banean). Requiere que el bot sea admin y que se pida explícitamente este tipo. | 7 (usuarios) |
| `chat_join_request` | Solicitud de ingreso a un grupo con aprobación manual. Requiere que el bot tenga el permiso `can_invite_users`. | 10 (solicitudes de ingreso) |
| `my_chat_member` | Cambia el estado del propio bot en un chat (lo agregan, lo hacen admin, lo sacan). Útil para detectar en qué grupos el bot tiene permisos. | 6 (gestión de grupos) |

Por defecto Telegram **no** envía `chat_member`, `message_reaction` ni
`message_reaction_count` salvo que se pidan explícitamente en
`allowed_updates`.

------------------------------------------------------------------------

## 3. Gestión de grupos (sección 6 del spec)

No existe un método que devuelva "todos los grupos donde está el bot".
Los grupos se descubren de forma incremental, por eventos:

- Al recibir un update `my_chat_member` con `new_chat_member.status` en
  `"administrator"` o `"member"`, registrar/actualizar ese chat en la
  base de datos propia.
- `getChat(chat_id)` --- devuelve info actualizada de un chat ya
  conocido (nombre, tipo, descripción, permisos por defecto).
- `getChatMemberCount(chat_id)` --- cantidad de miembros.
- `getChatAdministrators(chat_id)` --- lista de administradores humanos
  del chat (no todos los miembros).
- `getChatMember(chat_id, user_id)` --- estado de un usuario puntual
  (útil antes de intentar banear/mutear, para confirmar que no es ya
  admin del grupo, lo cual el bot no puede restringir).

No hay forma de "listar todos los miembros" de un grupo vía Bot API —
esto ya está señalado correctamente en la sección 7 del spec; queda
confirmado aquí para que el agente no lo intente igual.

------------------------------------------------------------------------

## 4. Gestión de usuarios (sección 7 del spec)

Todos requieren que el bot sea **administrador** del chat con el
permiso correspondiente.

| Acción | Método | Parámetros clave | Permiso de bot requerido |
|---|---|---|---|
| Banear | `banChatMember` | `chat_id`, `user_id`, `until_date` (opcional; menos de 30s o más de 366 días = ban permanente), `revoke_messages` (borra también sus mensajes) | `can_restrict_members` |
| Desbanear | `unbanChatMember` | `chat_id`, `user_id`, `only_if_banned` (evita error si no estaba baneado) | `can_restrict_members` |
| Mutear / restringir | `restrictChatMember` | `chat_id`, `user_id`, `permissions` (objeto `ChatPermissions`), `until_date` | `can_restrict_members` |
| Expulsar sin banear | `banChatMember` + `unbanChatMember` inmediato | — | `can_restrict_members` |

`ChatPermissions` es un objeto con flags booleanos (`can_send_messages`,
`can_send_media_messages`, `can_add_web_page_previews`, etc.). Para
mutear por completo, pasar todos los flags en `false`.

**Errores comunes:**
- El bot no puede restringir a otro administrador del grupo (Telegram
  devuelve error, no lo intenta silenciosamente).
- Si el bot no tiene el permiso `can_restrict_members`, Telegram
  responde con `error_code: 400` y una descripción indicando falta de
  derechos — mapear esto a `PERMISSION_DENIED` (sección 18 del spec).

------------------------------------------------------------------------

## 5. Moderación de mensajes (sección 8 del spec)

| Acción | Método | Parámetros clave | Permiso de bot requerido |
|---|---|---|---|
| Eliminar mensaje | `deleteMessage` | `chat_id`, `message_id` | `can_delete_messages` |
| Fijar mensaje | `pinChatMessage` | `chat_id`, `message_id`, `disable_notification` | `can_pin_messages` |
| Desfijar mensaje | `unpinChatMessage` | `chat_id`, `message_id` (opcional; sin este, desfija el último) | `can_pin_messages` |

Limitación real de Telegram: un bot solo puede borrar mensajes que él
mismo envió, o si tiene el permiso de administrador
`can_delete_messages` en grupos/supergrupos. En chats privados, un bot
solo puede borrar sus propios mensajes.

------------------------------------------------------------------------

## 6. Abrir/cerrar chat (sección 9 del spec)

No existe un método dedicado "lock/unlock". Se implementa con
`setChatPermissions`, aplicando permisos por defecto del chat (no de un
usuario puntual):

| Acción | Método | Parámetros clave |
|---|---|---|
| Cerrar chat | `setChatPermissions` | `chat_id`, `permissions` con `can_send_messages: false` |
| Abrir chat | `setChatPermissions` | `chat_id`, `permissions` con `can_send_messages: true` (y el resto de flags según se quiera permitir) |

Requiere permiso de bot `can_restrict_members`. Esta acción afecta a
**todos los miembros no-admin** del grupo — los administradores nunca
quedan restringidos por `setChatPermissions`.

------------------------------------------------------------------------

## 7. Solicitudes de ingreso (sección 10 del spec)

Requiere que el grupo tenga activada la aprobación manual de ingresos
(`join_by_request`) y que el bot tenga el permiso `can_invite_users`.

| Acción | Método | Parámetros clave |
|---|---|---|
| Aprobar | `approveChatJoinRequest` | `chat_id`, `user_id` |
| Rechazar | `declineChatJoinRequest` | `chat_id`, `user_id` |

El evento que dispara el flujo es el update `chat_join_request` (ver
sección 2 de este documento), que trae `chat`, `from` (el usuario) y
`date`. No hay un método para "listar solicitudes pendientes" — el
backend debe persistir cada `chat_join_request` recibido y marcarlo
como resuelto cuando se llame a `approve`/`decline`.

------------------------------------------------------------------------

## 8. Errores y códigos de estado

Telegram no tiene una lista fija de `error_code` documentada
formalmente, pero en la práctica el backend puede mapear así (sección
18 del spec):

| `error_code` HTTP | Causa típica | Mapeo sugerido |
|---|---|---|
| 400 | Parámetros inválidos, o acción imposible (ej. intentar restringir a un admin) | `VALIDATION_ERROR` o `PERMISSION_DENIED` según el `description` |
| 403 | El bot fue expulsado del chat, bloqueado por el usuario, o no tiene el permiso de admin necesario | `PERMISSION_DENIED` |
| 404 | Chat o mensaje no encontrado (ej. ya fue borrado) | `NOT_FOUND` |
| 429 | Rate limit excedido | Ver sección 9 de este documento |

El campo `description` de la respuesta trae el detalle en texto —
conviene loguearlo internamente pero no exponerlo tal cual al frontend
(sección 18 del spec: no mostrar errores internos al usuario final).

------------------------------------------------------------------------

## 9. Rate limits

Complementa la sección 18.1 del spec del proyecto.

Límites no oficiales pero ampliamente documentados por la comunidad de
desarrolladores:

- **~30 mensajes/segundo** en total, a través de todos los chats.
- **~1 mensaje/segundo** por chat individual.
- Acciones administrativas en lote (aprobar varias solicitudes de
  ingreso seguidas, banear varios usuarios) están sujetas al mismo
  límite global.

Cuando se excede el límite, Telegram responde `429 Too Many Requests`
con un campo `parameters.retry_after` (segundos a esperar antes de
reintentar). El `TelegramService` adapter (sección 15 del spec) debe:

1. Leer `retry_after` de la respuesta.
2. Esperar ese tiempo antes de reintentar.
3. Limitar a un máximo de reintentos (3, según el spec) para no
   bloquear la operación indefinidamente.

------------------------------------------------------------------------

## 10. Cosas que la Bot API **no** permite (para no intentarlas)

- Listar todos los miembros de un grupo (solo administradores, vía
  `getChatAdministrators`, y usuarios puntuales por ID).
- Enviar mensajes a un usuario que nunca inició una conversación con el
  bot (el bot no puede escribirle primero a un desconocido).
- Recuperar el historial de mensajes anterior a cuando el bot fue
  agregado al grupo.
- Saber si un usuario "vio" un mensaje (no hay recibos de lectura para
  bots).
- Restringir o banear a otro administrador del grupo — Telegram lo
  rechaza aunque el bot sea admin.

Si en algún momento el spec pide una funcionalidad que dependa de algo
de esta lista, hay que replantearla o documentarla como limitación
conocida, no simularla con workarounds inseguros (regla 1 y 17 de la
sección 25 del spec).
