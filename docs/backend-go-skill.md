# Guía de Backend (Go) --- Telegram Group Manager

> Complementa la especificación principal del proyecto (secciones 2, 14,
> 15, 17, 18, 21) y la referencia de la Bot API. No repite reglas ya
> definidas ahí — se enfoca en convenciones de código concretas para que
> el agente escriba Go idiomático y consistente a lo largo de todo el
> proyecto.

------------------------------------------------------------------------

## 1. Estilo y organización del código

- Seguir `gofmt`/`goimports` sin excepciones — cualquier código que no
  pase `gofmt -l .` limpio se considera incompleto.
- Nombres de paquete cortos, en minúscula, sin guiones bajos (`groups`,
  no `group_service`).
- Un archivo por responsabilidad dentro de cada módulo de
  `internal/<módulo>/`: `handler.go`, `service.go`, `repository.go`,
  `model.go`. No mezclar handlers y lógica de negocio en el mismo
  archivo (ya lo pide la sección 14 del spec; esto es cómo se traduce
  a archivos concretos).
- Interfaces se definen donde se **consumen**, no donde se implementan
  (convención estándar de Go). Ejemplo: si `groups.Service` necesita
  hablar con Telegram, la interfaz `TelegramService` se declara en el
  paquete `groups`, y `internal/telegram` provee la implementación.

## 2. Manejo de errores

- Nunca ignorar un error con `_`. Si de verdad no importa, comentar por
  qué (`_ = someCall() // best-effort, no crítico`).
- Envolver errores con contexto usando `fmt.Errorf("banning user %d: %w", userID, err)`
  — el `%w` preserva la cadena para `errors.Is`/`errors.As`.
- Definir errores de dominio como valores (`var ErrPermissionDenied = errors.New(...)`)
  en el paquete correspondiente, y mapearlos a los códigos de la sección
  18 del spec (`SUCCESS`, `PERMISSION_DENIED`, `TELEGRAM_ERROR`,
  `VALIDATION_ERROR`, `NOT_FOUND`, `INTERNAL_ERROR`) en la capa HTTP,
  no antes. Los servicios no deberían conocer códigos HTTP.
- El handler HTTP es la única capa que traduce un error de dominio a
  status code + payload JSON. Los servicios devuelven errores de Go
  normales.

## 3. Contexto (`context.Context`)

- Todo método que haga I/O (DB, llamada a Telegram, HTTP) recibe
  `ctx context.Context` como primer parámetro.
- Propagar el contexto de la request HTTP hasta la llamada a Telegram y
  a PostgreSQL, para que un timeout o cancelación del cliente corte la
  cadena completa en vez de dejar goroutines colgadas.
- No guardar valores de negocio en el contexto (ej. no pasar el
  `userID` autenticado vía `context.WithValue` como sustituto de un
  parámetro explícito, salvo para el ID de request/trace).

## 4. Acceso a base de datos

- Usar `database/sql` directo o `sqlx` — evitar un ORM completo para el
  MVP, consistente con la sección 2 del spec ("evitar dependencias
  innecesarias").
- Repositorios devuelven structs del dominio, nunca `sql.Rows` ni tipos
  específicos del driver — esa capa queda encapsulada en
  `internal/database` o en cada `repository.go`.
- Toda query con parámetros del usuario usa placeholders (`$1, $2...`)
  — nunca concatenar strings SQL. Esto no es opcional, es la única
  defensa contra SQL injection en un panel con permisos administrativos.
- Migraciones con `goose` (ya definido en la sección 13.1 del spec):
  una migración = un cambio lógico, con su `down` correspondiente
  siempre que sea reversible.

## 5. El adapter de Telegram (`internal/telegram`)

- La interfaz `TelegramService` (sección 15 del spec) es el único
  punto de contacto con la Bot API. Ningún otro paquete debe importar
  el cliente HTTP de Telegram directamente.
- El rate limiter (sección 18.1 del spec) vive dentro de la
  implementación concreta, no en la interfaz — los consumidores no
  deben saber que existe.
- Todas las respuestas de error de Telegram se traducen aquí a errores
  de dominio (`ErrPermissionDenied`, `ErrTelegramRateLimit`, etc.),
  usando la tabla de mapeo de la sección 8 de la referencia de la Bot
  API. Los paquetes que consumen `TelegramService` no deberían parsear
  `error_code` ni `description` por su cuenta.

## 6. HTTP / API REST

- Router recomendado para el MVP: `chi` (liviano, sin magia, buen
  soporte de middleware) o la librería estándar `net/http` con Go 1.22+
  (que ya soporta patrones de ruta con método y parámetros). No usar un
  framework pesado tipo Gin salvo que ya se domine — no aporta nada
  crítico al MVP.
- Middleware mínimo: logging de requests, recuperación de panics
  (`recover`), autenticación JWT (sección 17.1 del spec), y CORS
  configurado explícitamente al origen del frontend (nunca `*` en un
  panel con acciones administrativas).
- Respuestas JSON consistentes: un envelope simple
  `{"data": ..., "error": null}` o `{"data": null, "error": {"code": "...", "message": "..."}}`
  — no mezclar ambos formatos entre endpoints.
- Validar el body de cada request en el handler antes de pasarlo al
  servicio (sección 4 del spec: "validar todos los parámetros
  recibidos"). Librería sugerida: `go-playground/validator` con tags en
  los structs de request, para no escribir validación manual repetida.

## 7. Autenticación y autorización

- JWT y bcrypt según sección 17.1 del spec. Implementación sugerida:
  `golang-jwt/jwt/v5` para tokens, `golang.org/x/crypto/bcrypt` para
  passwords.
- El middleware de auth solo verifica identidad (¿quién es?). La
  autorización (¿puede hacer esto en este grupo?) es una capa aparte,
  a nivel de servicio — un admin de la app no necesariamente tiene
  permisos sobre todos los grupos gestionados si más adelante se
  agregan roles por grupo.
- Nunca loguear el JWT completo ni el token del bot, ni siquiera en
  logs de debug (ya lo exige la sección 5.1 y 4 del spec).

## 8. Testing

- Mocks de `TelegramService` generados con `moq` (sección 21.1 del
  spec) — un solo mock reutilizado en todos los tests de servicios que
  dependan de Telegram.
- Tests de repositorios contra una PostgreSQL real en Docker
  (`testcontainers-go` es una opción razonable si se quiere evitar
  mocks de SQL), no contra SQLite ni mocks de la capa de DB — el
  comportamiento de constraints y tipos de Postgres no es 100%
  equivalente.
- Nombrar tests como `TestBanUser_PermissionDenied`,
  `TestBanUser_Success` — describe el escenario, no solo la función.

## 9. Configuración

- Toda variable de entorno se lee en un único lugar (`config/`), se
  valida al arrancar (falla rápido si falta `TELEGRAM_BOT_TOKEN` o
  `DATABASE_URL`), y se pasa como struct tipado al resto de la app —
  no leer `os.Getenv` disperso por el código.
- `.env.example` se actualiza en el mismo commit que introduce una
  variable nueva (ya lo pide la sección 25, regla 13, del spec).

## 10. Logging

- Logger estructurado (`log/slog` de la stdlib, disponible desde Go
  1.21, es suficiente — no hace falta Zap ni Zerolog para este MVP).
- Log de cada acción administrativa según el formato de la sección 11
  del spec, pero como log estructurado (JSON), no como texto libre —
  facilita que después se pueda consultar por campo sin parsing.
- Nunca loguear tokens, passwords ni el body completo de un request de
  login.
