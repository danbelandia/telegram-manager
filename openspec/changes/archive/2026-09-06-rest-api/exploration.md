# Exploration: rest-api — paso 10: API REST de administración

## Contexto

El backend ya tiene: `internal/groups` (model+repo+events de detección
vía my_chat_member con bot_permissions JSONB), `internal/auth` completo
(login JWT, requireAuth como middleware de identidad), `internal/api`
(Server con ServerMux Go 1.22+, envelope respond(), opciones WithAuth/
WithWebhook), `internal/events` (bus síncrono), `internal/telegram`
(Service con GetMe/GetUpdates/SetWebhook/DeleteWebhook, adapter con
doGet/doGetQuery y RateLimitError). Migraciones: solo 00001 admins y
00002 groups.

## Gaps confirmados (obs explore 132)

1. `telegram.Service` NO tiene los métodos de moderación del §15
   (BanUser, UnbanUser, MuteUser, UnmuteUser, DeleteMessage,
   PinMessage, LockGroup, UnlockGroup, ApproveJoinRequest,
   RejectJoinRequest) ni GetChatMember/GetChatAdministrators.
2. El adapter solo hace GET con query string; los métodos de
   moderación de la Bot API se invocan con POST y JSON body
   (ChatPermissions anidado no va bien en query).
3. NO hay rate limiter token bucket (§18.1); solo el retry-429 del
   Poller. Debe vivir dentro del adapter.
4. 400/403/404 de Telegram no están tipados: faltan errores de dominio
   `ErrPermissionDenied`, `ErrTelegramError` (o equivalente
   NOT_FOUND), que el skill §2/§5 exige mapear en el adapter.
5. NO existen tablas `users`, `join_requests`, `warnings`, `logs` ni
   sus repositorios. NO hay handlers/servicios de moderación.
6. Las rutas nuevas se montan por `Option` (patrón WithAuth).
7. No hay endpoint READ de grupos (GET /api/groups).
8. `ChatJoinRequest` (update.go) no trae ID propio → la fila
   `join_requests.id` (BIGSERIAL) es el `requestId` de las rutas.
9. MVPAllowedUpdates ya incluye `chat_join_request`; el bus publica
   updates; hoy solo se consumen my_chat_member.
10. fakeService (poller_test) implementa Service a mano → se actualiza
    al ampliar la interfaz; mocks manuales (fakeService, fakeStore,
    botStatusStub, stubPinger, capturingBus); moq NO está.

## Decisiones tomadas con el usuario

- **P1 (group_members)**: documentar y NO crear la tabla. Telegram no
  permite listar miembros de un grupo (referencia §10) → la tabla
  quedaría vacía y sin uso. Se documenta como limitación y se agrega
  cuando exista una necesidad concreta (regla §25.17).
- **P2 (búsqueda de usuarios)**: SÍ. `GET /api/groups/:id/users`
  devuelve lo que Telegram expone: administradores vía
  `getChatAdministrators`, o un usuario puntual vía `getChatMember`
  con `?userId=`.

## Restricciones de proyecto que aplican

- No Redis, no colas externas (§2). El rate limiter es canal interno
  + worker (§18.1).
- Telegram es fuente de verdad: no inventar endpoints de la Bot API
  (referencia docs/telegram_api_reference.md).
- Las acciones administrativas se registran en `logs` (§11) con
  actor (claims del admin), grupo, acción, target, status.
- Los handlers validan el body, llaman al servicio, y el servicio
  verifica: autenticado → ¿puede (identidad)? → bot tiene permiso en
  el grupo (bot_permissions JSONB) → ejecuta → registra log.
- Envelope respond()/respondError() con códigos §18 (SUCCESS,
  PERMISSION_DENIED, TELEGRAM_ERROR, VALIDATION_ERROR, NOT_FOUND,
  INTERNAL_ERROR).
- El adapter traduce errores de la Bot API a errores de dominio; los
  servicios no conocen códigos HTTP.

## Archivos de referencia clave

- `backend/internal/telegram/service.go` (interfaz a ampliar)
- `backend/internal/telegram/adapter.go` (doGetQuery, errores)
- `backend/internal/telegram/update.go` (ChatJoinRequest, ChatMember)
- `backend/internal/groups/repository.go` (patrón repo, bot_permissions)
- `backend/internal/api/server.go` (patrón Option)
- `backend/internal/api/envelope.go`, `middleware_auth.go`
- `backend/internal/database/testdb.go` (OpenTestDB por-suite)
- `docs/telegram_api_reference.md` (secciones 4-9: métodos, errores,
  rate limits, limitaciones)
- `docs/backend-go-skill.md` (§1 capas, §2 errores, §5 adapter,
  §7 auth/autz, §8 testing)