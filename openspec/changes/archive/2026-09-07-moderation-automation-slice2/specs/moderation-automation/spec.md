# Delta for moderation-automation — Slice 2 (Anti-spam + Anti-link + Banned-words + Settings UI)

> **Change**: `moderation-automation-slice2` — Slice 2/3 de Fase 3 (AGENTS §23).
> **Predecessor**: slice 1 archivado (`main @ 1b10d42`); canónico en `openspec/specs/moderation-automation/spec.md`.
> **Authority**: bugfix `#172` (`permissionOkAdmin` usa `BotStatus == StatusAdministrator`, NUNCA `can_*`).
>
> Esta spec **amenda** el canónico de slice 1 vía `## ADDED Requirements` (REQ-7 a REQ-21). Archive al cierre usa la técnica MOVE del slice 1 (delta → canónico). No se crea spec paralelo.

---

## ADDED Requirements

### Requirement: Schema `banned_words`

El sistema MUST crear la tabla `banned_words` con columnas `group_id`
(BIGINT NOT NULL, FK `groups(telegram_id)` ON DELETE CASCADE), `word`
(TEXT NOT NULL), `created_at` (TIMESTAMPTZ NOT NULL DEFAULT now()).
PK compuesta `(group_id, word)`. CHECK `length(word) BETWEEN 1 AND 100`.
Índice `idx_banned_words_group` sobre `group_id`. Migración
`00007_create_moderation_lists.sql` con Up/Down reversibles (goose,
SQL plano). El insert MUST normalizar la palabra con `LOWER(word)`
para hacer la comparación case-insensitive uniforme.

#### Scenario: Up crea tabla con PK compuesta

- GIVEN la base con la migración 00006 aplicada
- WHEN se ejecuta `goose up` para 00007
- THEN existe la tabla `banned_words` con PK `(group_id, word)`,
  CHECK `length(word) BETWEEN 1 AND 100`, FK CASCADE a `groups`

#### Scenario: Down revierte sin tocar la base previa

- GIVEN la base con la migración 00007 aplicada
- WHEN se ejecuta `goose down` para 00007
- THEN `banned_words` se elimina; las tablas previas (`groups`,
  `group_moderation_settings`, `user_warning_state`) quedan intactas

#### Scenario: FK CASCADE borra palabras al borrar el grupo

- GIVEN un grupo `g` con 3 filas en `banned_words`
- WHEN se ejecuta `DELETE FROM groups WHERE telegram_id = g`
- THEN las 3 filas de `banned_words` para `g` se eliminan
  automáticamente

### Requirement: Schema `link_allowlist`

El sistema MUST crear la tabla `link_allowlist` con la misma shape
que `banned_words`: columnas `group_id` (BIGINT NOT NULL FK CASCADE),
`domain` (TEXT NOT NULL), `created_at` (TIMESTAMPTZ NOT NULL DEFAULT
now()). PK compuesta `(group_id, domain)`. CHECK
`length(domain) BETWEEN 1 AND 253`. Índice
`idx_link_allowlist_group` sobre `group_id`. El dominio se guarda tal
cual lo ingresa el admin (la normalización lowercase del match la
hace el `AntiLinkRule` en evaluación, no el insert).

#### Scenario: Up crea tabla paralela

- GIVEN la base con 00007 parcialmente aplicada (solo `banned_words`)
- WHEN se completa `goose up` para 00007
- THEN existe `link_allowlist` con PK `(group_id, domain)`,
  CHECK `length(domain) BETWEEN 1 AND 253`, FK CASCADE a `groups`

#### Scenario: Down elimina ambas tablas en orden inverso

- GIVEN la base con 00007 aplicada completa
- WHEN se ejecuta `goose down` para 00007
- THEN `link_allowlist` y `banned_words` se eliminan;
  `group_moderation_settings` y `user_warning_state` permanecen

### Requirement: `AntiSpamRule`

El sistema MUST implementar `AntiSpamRule` (registrada en el
`Registry` después de `FloodRule`). La regla MUST detectar **al menos
uno** de los siguientes sub-detectores sobre `msg.Text`:

- **All-caps**: proporción de letras mayúsculas (`unicode.IsUpper`)
  sobre letras totales (`unicode.IsLetter`) > 0.70 AND
  `len(msg.Text) > 10`. Reason: `"message is mostly uppercase"`.
- **Caracteres repetidos**: 5 o más caracteres iguales consecutivos
  (cualquier rune, no solo letras). Reason:
  `"message contains 5+ repeated characters"`.
- **URL sospechosa corta**: cualquier match del patrón
  `https?://\S+|t\.me/\S+|telegram\.me/\S+` con longitud total del
  match < 20 caracteres. Reason: `"short suspicious URL"`.

El primer sub-detector que dispare define el `RuleHit.Reason`. Si
`msg.Text` está vacío o no aplica ningún sub-detector → retorna nil.
La regla MUST respetar `settings.AntiSpamEnabled` (skip si false) y
MUST ser stateless (sin estado mutable; tests de concurrencia no
requeridos).

#### Scenario: All-caps dispara hit

- GIVEN `AntiSpamEnabled=true`, mensaje con texto
  `"COMPREN ESTE PRODUCTO YA"` (>70% mayúsculas, len > 10)
- WHEN `AntiSpamRule.Evaluate` se invoca
- THEN retorna `&RuleHit{RuleName:"anti_spam", Reason:"message is mostly uppercase"}`

#### Scenario: Texto corto en mayúsculas NO dispara

- GIVEN `AntiSpamEnabled=true`, mensaje con texto `"OK"` (len ≤ 10)
- WHEN `AntiSpamRule.Evaluate` se invoca
- THEN retorna nil

#### Scenario: Caracteres repetidos dispara hit

- GIVEN `AntiSpamEnabled=true`, mensaje con texto
  `"holaaaaaaa amigos"` (5+ `a` consecutivas)
- WHEN `AntiSpamRule.Evaluate` se invoca
- THEN retorna `&RuleHit{RuleName:"anti_spam", Reason:"message contains 5+ repeated characters"}`

#### Scenario: URL corta sospechosa dispara hit

- GIVEN `AntiSpamEnabled=true`, mensaje con texto `"mira https://x.co"`
- WHEN `AntiSpamRule.Evaluate` se invoca
- THEN retorna `&RuleHit{RuleName:"anti_spam", Reason:"short suspicious URL"}`

#### Scenario: Toggle deshabilitado salta la regla

- GIVEN `AntiSpamEnabled=false`, mensaje all-caps válido
- WHEN `AntiSpamRule.Evaluate` se invoca
- THEN retorna nil sin inspeccionar el texto

### Requirement: `AntiLinkRule`

El sistema MUST implementar `AntiLinkRule`. La regla MUST detectar
cualquier URL que matchee el patrón
`https?://\S+|t\.me/\S+|telegram\.me/\S+` en `msg.Text`. Si el host
extraído del match (lowercased) está en `lists.LinkAllowlist` **o** es
un subdominio directo de un dominio en la allowlist (ej.
`sub.example.com` matchea `example.com`), la URL se ignora. Cualquier
URL fuera de la allowlist → retorna
`&RuleHit{RuleName:"anti_link", Reason:"message contains link to <host>"}`
donde `<host>` es el host extraído del primer match. La regla MUST
respetar `settings.AntiLinkEnabled`. Si la lista `lists.LinkAllowlist`
es nil o vacía → toda URL dispara hit.

#### Scenario: URL fuera de allowlist dispara hit

- GIVEN `AntiLinkEnabled=true`, `link_allowlist = []`,
  mensaje `"mira https://spam.example.com"`
- WHEN `AntiLinkRule.Evaluate` se invoca
- THEN retorna `&RuleHit{RuleName:"anti_link", Reason:"message contains link to spam.example.com"}`

#### Scenario: Dominio exacto en allowlist → skip

- GIVEN `AntiLinkEnabled=true`, `link_allowlist = ["example.com"]`,
  mensaje `"mira https://example.com/x"`
- WHEN `AntiLinkRule.Evaluate` se invoca
- THEN retorna nil

#### Scenario: Subdominio en allowlist → skip

- GIVEN `AntiLinkEnabled=true`, `link_allowlist = ["example.com"]`,
  mensaje `"mira https://docs.example.com/y"`
- WHEN `AntiLinkRule.Evaluate` se invoca
- THEN retorna nil (suffix match acepta `*.example.com`)

#### Scenario: Dominio con prefijo distinto NO matchea allowlist

- GIVEN `AntiLinkEnabled=true`, `link_allowlist = ["example.com"]`,
  mensaje `"mira https://notexample.com/z"`
- WHEN `AntiLinkRule.Evaluate` se invoca
- THEN retorna hit (suffix `example.com` requiere `.` previo)

#### Scenario: Toggle deshabilitado salta la regla

- GIVEN `AntiLinkEnabled=false`, mensaje con URL fuera de allowlist
- WHEN `AntiLinkRule.Evaluate` se invoca
- THEN retorna nil

### Requirement: `BannedWordsRule`

El sistema MUST implementar `BannedWordsRule`. La regla MUST iterar
`lists.BannedWords` y, para cada palabra, aplicar
`strings.Contains(strings.ToLower(msg.Text), word)`. El primer match
define `RuleHit.RuleName="banned_words"` y
`Reason="message contains banned word: <word>"`. La regla MUST
respetar `settings.BannedWordsEnabled`. Si `lists.BannedWords` es nil
o vacío → retorna nil (skip). El text vacío → retorna nil.

#### Scenario: Palabra exacta matchea case-insensitive

- GIVEN `BannedWordsEnabled=true`, `banned_words = ["spam"]`,
  mensaje `"compré SPAM ayer"`
- WHEN `BannedWordsRule.Evaluate` se invoca
- THEN retorna `&RuleHit{RuleName:"banned_words", Reason:"message contains banned word: spam"}`

#### Scenario: Substring matchea

- GIVEN `BannedWordsEnabled=true`, `banned_words = ["mal"]`,
  mensaje `"esto es una palabra mala"`
- WHEN `BannedWordsRule.Evaluate` se invoca
- THEN retorna hit con `Reason:"message contains banned word: mal"`

#### Scenario: Lista vacía → skip

- GIVEN `BannedWordsEnabled=true`, `banned_words = []`,
  cualquier mensaje
- WHEN `BannedWordsRule.Evaluate` se invoca
- THEN retorna nil

#### Scenario: Toggle deshabilitado salta la regla

- GIVEN `BannedWordsEnabled=false`, mensaje con palabra baneada
- WHEN `BannedWordsRule.Evaluate` se invoca
- THEN retorna nil

### Requirement: Rule registry order cheap→expensive

El `Registry` MUST evaluar las reglas registradas en el siguiente
orden: `Flood → AntiSpam → AntiLink → BannedWords`. Esto MUST ser
configurable solo vía el orden de invocación a `registry.Register(...)`
en `cmd/server/main.go` — sin API pública para reordenar. El
short-circuit MUST mantenerse: la primera regla que retorne `*RuleHit`
corta la iteración. La justificación (cheap-first) es que
`FloodRule` es O(1) en memoria, `AntiSpamRule` y `AntiLinkRule` son
O(n) en CPU puro sobre el texto, y `BannedWordsRule` es la única que
depende de una lista externa (potencialmente grande) ya pre-cargada
por el Service.

#### Scenario: Orden registrado respetado por `Rules()`

- GIVEN las 4 reglas registradas en este orden: Flood, AntiSpam,
  AntiLink, BannedWords
- WHEN `Registry.Rules()` se invoca
- THEN el slice devuelto mantiene el orden exacto: índice 0 Flood,
  1 AntiSpam, 2 AntiLink, 3 BannedWords

#### Scenario: Short-circuit entre reglas costosas

- GIVEN `FloodRule` registrada que dispara hit, `BannedWordsRule`
  registrada con fakes que cuentan invocaciones
- WHEN `Registry.Evaluate` se invoca con un mensaje que dispara flood
- THEN `BannedWordsRule.Evaluate` NO se invoca (short-circuit)

### Requirement: `Service.HandleMessage` pre-carga listas una vez

`Service.HandleMessage` MUST invocar `bannedWordsRepo.ListBannedWords`
y `linkAllowlistRepo.ListLinkAllowlist` exactamente **una vez** por
mensaje, antes de invocar `Registry.Evaluate`, y pasar el resultado a
cada regla como argumento `lists *Lists{BannedWords, LinkAllowlist}`
del nuevo `Rule.Evaluate` extendido. Si `!settings.BannedWordsEnabled`
Y `!settings.AntiLinkEnabled`, el Service MAY omitir las dos llamadas
a la DB (optimización; tests verifican que las llamadas se hacen
cuando al menos un toggle dependiente está activo).

#### Scenario: Las dos listas se cargan una sola vez

- GIVEN un mensaje, `BannedWordsEnabled=true`, `AntiLinkEnabled=true`
- WHEN `Service.HandleMessage` se invoca con un fake repo que cuenta
  invocaciones
- THEN `ListBannedWords` y `ListLinkAllowlist` se llaman exactamente
  1 vez cada una antes de `Registry.Evaluate`

#### Scenario: Listas se omiten si ambos toggles están en false

- GIVEN `BannedWordsEnabled=false`, `AntiLinkEnabled=false`
- WHEN `Service.HandleMessage` se invoca
- THEN `ListBannedWords` y `ListLinkAllowlist` NO se invocan; las
  reglas se evalúan con `lists == nil` (cada regla es defensiva con
  nil)

#### Scenario: Listas se cargan si al menos un toggle está activo

- GIVEN `BannedWordsEnabled=false`, `AntiLinkEnabled=true`
- WHEN `Service.HandleMessage` se invoca
- THEN `ListLinkAllowlist` se invoca 1 vez; `ListBannedWords` se omite

### Requirement: `Rule.Evaluate` extendido con `lists *Lists`

La interfaz `Rule` MUST extender su firma de `Evaluate` para aceptar
un nuevo argumento `lists *Lists` entre `ws *WarningState` y `now
time.Time`. `Lists` MUST ser un struct exported con campos
`BannedWords []string` y `LinkAllowlist []string`. La `FloodRule`
existente MUST ser actualizada para aceptar el nuevo argumento
(pasarlo por ignorarlo; la regla sigue stateless). El `Registry.Evaluate`
MUST propagar `lists` a cada regla registrada. Toda regla nueva o
modificada MUST manejar `lists == nil` defensivamente (retornar nil
si la regla no puede evaluar sin listas).

#### Scenario: Nueva firma compila y tests de slice 1 siguen pasando

- GIVEN el branch con la firma extendida
- WHEN `go build ./...` corre
- THEN el código compila sin errores; los tests de `FloodRule`
  (sub-umbral, en umbral, sobre umbral, expiración, aislamiento)
  siguen verdes

#### Scenario: Registry propaga lists a todas las reglas

- GIVEN un Registry con `FloodRule` y `BannedWordsRule` registradas,
  un `*Lists` con `BannedWords:["spam"]`
- WHEN `Registry.Evaluate` se invoca
- THEN ambas reglas reciben el mismo puntero `*Lists`

### Requirement: GET/PUT `/api/groups/{id}/automation/settings`

El backend MUST exponer:

- `GET /api/groups/{id}/automation/settings` → 200 con `*Settings` JSON
  (envelope `data.settings`). Si no existe fila en
  `group_moderation_settings`, retorna defaults (`enabled=false`,
  todos los toggles `false`, thresholds de DB). Autenticación
  requerida (`requireAuth`). Grupo inexistente → 404 `NOT_FOUND`.
- `PUT /api/groups/{id}/automation/settings` → body `*Settings`
  parcial o completo; el handler MUST normalizar defaults para
  campos ausentes y llamar `repository.UpsertSettings`. Log
  `ActionUpdateAutomationSettings` con `ActorID` del admin,
  `metadata={settings_changed}`. Autenticación requerida. Errores
  de validación (ej. `autoban_warnings <= automute_warnings`) → 400
  `VALIDATION_ERROR`.

#### Scenario: GET sin fila retorna defaults

- GIVEN un grupo `g` sin fila en `group_moderation_settings`
- WHEN se ejecuta `GET /api/groups/g/automation/settings`
- THEN responde 200 con envelope
  `data.settings = {enabled:false, anti_spam_enabled:false, ...}`

#### Scenario: GET con fila existente

- GIVEN un grupo `g` con fila persistida
  (`enabled=true`, `anti_spam_enabled=true`, `flood_messages=3`)
- WHEN se ejecuta GET
- THEN responde 200 con esos valores exactos

#### Scenario: PUT upsert y loguea

- GIVEN un admin autenticado y body con `enabled=true`,
  `anti_spam_enabled=true`, `automute_warnings=5`
- WHEN se ejecuta `PUT /api/groups/g/automation/settings`
- THEN la fila se upserta; existe log `UPDATE_AUTOMATION_SETTINGS`
  con `actor_id` del admin y `metadata` con los campos cambiados

#### Scenario: 404 si el grupo no existe

- GIVEN `group_id` que no está en `groups`
- WHEN se ejecuta GET o PUT
- THEN responde 404 `NOT_FOUND`

### Requirement: CRUD `/api/groups/{id}/automation/banned-words`

El backend MUST exponer:

- `GET /api/groups/{id}/automation/banned-words` → 200 con
  `[]string` ordenado alfabéticamente. Autenticación requerida.
- `POST /api/groups/{id}/automation/banned-words` con body `{word}`
  → 200 con la lista actualizada. La palabra se guarda lowercased
  (server-side normalization). Si la palabra ya existía (PK
  compuesta), `ON CONFLICT DO NOTHING` → no error, devuelve la lista
  actual. Log `ActionAddBannedWord` con `ActorID` del admin.
- `DELETE /api/groups/{id}/automation/banned-words/{word}` → 200 con
  la lista actualizada. La palabra buscada se lowercases antes del
  DELETE. Si no existía → no error, devuelve la lista actual.
  Log `ActionRemoveBannedWord` con `ActorID` del admin.

#### Scenario: GET lista vacía

- GIVEN un grupo `g` sin filas en `banned_words`
- WHEN se ejecuta GET
- THEN responde 200 con `data.words = []`

#### Scenario: POST agrega palabra nueva

- GIVEN un grupo `g` con `banned_words = []`
- WHEN se ejecuta POST con `{word:"spam"}`
- THEN responde 200 con `data.words = ["spam"]`; existe log
  `ADD_BANNED_WORD` con `actor_id` y `metadata={word:"spam"}`

#### Scenario: POST idempotente

- GIVEN un grupo `g` con `banned_words = ["spam"]`
- WHEN se ejecuta POST con `{word:"SPAM"}`
- THEN responde 200 con `data.words = ["spam"]` (sin duplicar);
  se registra exactamente 1 log `ADD_BANNED_WORD`

#### Scenario: DELETE remueve palabra

- GIVEN un grupo `g` con `banned_words = ["spam","mal"]`
- WHEN se ejecuta DELETE `/automation/banned-words/spam`
- THEN responde 200 con `data.words = ["mal"]`; existe log
  `REMOVE_BANNED_WORD`

### Requirement: CRUD `/api/groups/{id}/automation/link-allowlist`

El backend MUST exponer el mismo patrón simétrico que
`banned-words`:

- `GET /api/groups/{id}/automation/link-allowlist` → 200 con
  `[]string` ordenado alfabéticamente. Autenticación requerida.
- `POST /api/groups/{id}/automation/link-allowlist` con body
  `{domain}` → 200 con la lista actualizada. El dominio se guarda tal
  cual (case se preserva); el matcher lowercases en evaluación. Si ya
  existía → no error. Log `ActionAddLinkAllowlist` con `ActorID`.
- `DELETE /api/groups/{id}/automation/link-allowlist/{domain}` → 200
  con la lista actualizada. Si no existía → no error. Log
  `ActionRemoveLinkAllowlist` con `ActorID`.

#### Scenario: GET lista vacía

- GIVEN un grupo `g` sin filas en `link_allowlist`
- WHEN se ejecuta GET
- THEN responde 200 con `data.domains = []`

#### Scenario: POST agrega dominio

- GIVEN un grupo `g` con `link_allowlist = []`
- WHEN se ejecuta POST con `{domain:"example.com"}`
- THEN responde 200 con `data.domains = ["example.com"]`; existe log
  `ADD_LINK_ALLOWLIST`

#### Scenario: DELETE remueve dominio

- GIVEN un grupo `g` con `link_allowlist = ["example.com"]`
- WHEN se ejecuta DELETE `/automation/link-allowlist/example.com`
- THEN responde 200 con `data.domains = []`; existe log
  `REMOVE_LINK_ALLOWLIST`

### Requirement: Frontend `GroupAutomationPage`

El frontend MUST renderizar `GroupAutomationPage` en la ruta
`/groups/:id/automation` (registrada en `App.tsx` con
`RequireAuth`). La página MUST mostrar 4 secciones:

- **Reglas activas**: 4 `Switch` (Mantine v7) en orden —
  `Habilitar moderación automática` (settings.enabled),
  `Flood` (flood_enabled), `Anti-spam` (anti_spam_enabled),
  `Anti-link` (anti_link_enabled), `Banned-words`
  (banned_words_enabled). (5 switches en total: 1 principal + 4
  toggles por regla.)
- **Umbrales**: 6 `NumberInput` Mantine v7 — `flood_messages`,
  `flood_seconds`, `warning_limit`, `automute_warnings`,
  `automute_minutes`, `autoban_warnings`. Validación cliente:
  `autoban_warnings > automute_warnings > 0`.
- **Palabras baneadas**: `<TagsInput>` Mantine v7 para
  `banned_words`. Validación cliente: regex
  `^[\p{L}\p{N}_\- ]{1,100}$`. Trim + lowercase antes de POST.
- **Link allowlist**: `<TagsInput>` Mantine v7 para
  `link_allowlist`. Sin regex estricto (acepta punto, guion).
  Trim antes de POST.

**Un único botón "Guardar"** al fondo MUST disparar los roundtrips
en paralelo vía `Promise.all`:

1. `PUT /api/groups/:id/automation/settings` con los toggles + thresholds.
2. Por cada palabra en `banned_words`: `POST o DELETE` según diff vs
   el estado original.
3. Idem para `link_allowlist`.

Notificaciones `notifySuccess`/`notifyError` por sección. Cada
operación que falle NO aborta el resto (se acumulan y se muestran
como notificaciones separadas). El link desde `GroupDetailPage` →
panel "Detalle" → botón "Moderación automática" (debajo de
   "Membresía y moderación") MUST existir.

#### Scenario: Carga inicial con datos

- GIVEN `mockFetchRoutes` configurado para devolver settings
  (`enabled=true`, `flood_messages=5`), 2 palabras, 1 dominio
- WHEN se renderiza `GroupAutomationPage` en `/groups/g/automation`
- THEN los switches reflejan los valores del GET; los `<TagsInput>`
  muestran las palabras y dominios del GET

#### Scenario: Save dispara 3 roundtrips en paralelo

- GIVEN la página con settings cargados, `Promise.allSpy` mock
- WHEN el admin modifica 1 palabra, 1 toggle, 1 threshold y hace
  click en Guardar
- THEN se ejecutan en paralelo: 1 PUT settings, 1 POST/DELETE word,
  0 cambios en allowlist (Promise.all con al menos 2 operaciones)

#### Scenario: Error en una sección NO aborta las otras

- GIVEN `PUT /settings` responde 500, `POST /banned-words` responde 200
- WHEN el admin hace click en Guardar
- THEN la palabra se agrega (notifySuccess); settings muestra
  `notifyError` con mensaje legible

#### Scenario: Link desde GroupDetailPage

- GIVEN un grupo cargado en `GroupDetailPage`
- WHEN el admin hace click en "Moderación automática"
- THEN navega a `/groups/:id/automation`

### Requirement: Action constants de auditoría (5 nuevas)

`backend/internal/logs/model.go` MUST exponer 5 constantes nuevas:

- `ActionUpdateAutomationSettings = "UPDATE_AUTOMATION_SETTINGS"`
- `ActionAddBannedWord            = "ADD_BANNED_WORD"`
- `ActionRemoveBannedWord         = "REMOVE_BANNED_WORD"`
- `ActionAddLinkAllowlist         = "ADD_LINK_ALLOWLIST"`
- `ActionRemoveLinkAllowlist      = "REMOVE_LINK_ALLOWLIST"`

Todas las acciones manuales de settings/lists MUST registrar log con
`ActorID` del admin (no `nil` — son cambios manuales, no del
sistema). Esto es distinto del patrón de slice 1 donde `ActorID=nil`
marcaba auto-actions.

#### Scenario: Constantes existen

- GIVEN el paquete `logs` compilado
- WHEN se importan las 5 constantes nuevas
- THEN existen con los valores string declarados

#### Scenario: Log manual tiene actor no nulo

- GIVEN un admin con id `42` ejecuta `PUT /settings`
- WHEN el handler persiste el log
- THEN `entry.ActorID != nil` (`actor_id = 42`)

### Requirement: Tests §21.1 (backend + frontend)

Los tests MUST cubrir (§21.1 estricto — cero llamadas Bot API
reales):

- **Unit por regla** (extender `rules_test.go`): `AntiSpamRule` (5+
  casos: all-caps hit, texto corto sin hit, repeated chars hit,
  short URL hit, toggle off skip); `AntiLinkRule` (5+ casos: URL
  fuera de allowlist, dominio exacto skip, subdominio skip,
  prefijo-falso hit, toggle off skip); `BannedWordsRule` (4+ casos:
  match case-insensitive, substring, lista vacía skip, toggle off
  skip). Total ≥ 18 casos nuevos.
- **Service tests** (extender `service_test.go`): 5+ casos
  adicionales cubriendo pre-load de listas (lista vacía cuando
  toggles off, doble carga cuando ambos toggles activos, listas
  compartidas entre reglas).
- **Repository integration** (extender `repository_test.go`):
  `OpenTestDB("automation")` + `goose.Up` + TRUNCATE; 8+ casos:
  `AddBannedWord`/`RemoveBannedWord`/`ListBannedWords`, mismo
  patrón para `link_allowlist`, FK CASCADE al borrar grupo,
  idempotencia en POST (palabra duplicada → no error).
- **Handler tests** (`automation_handlers_test.go`): 200/400/404 en
  GET y PUT settings; 200 en GET/POST/DELETE banned-words; 200 en
  GET/POST/DELETE link-allowlist; auth required (401 sin token).
- **Frontend tests** (`GroupAutomationPage.test.tsx`):
  `mockFetchRoutes` configurado por substring URL; render inicial,
  agregar palabra dispara POST, remover dispara DELETE, toggle
  cambia state local, Save dispara ≥ 2 roundtrips, error path
  muestra `notifyError`.

#### Scenario: Unit AntiSpamRule — all-caps hit

- GIVEN `AntiSpamEnabled=true`, mensaje `"COMPREN ESTE PRODUCTO YA"`
- WHEN `Evaluate` se invoca
- THEN retorna `RuleHit{RuleName:"anti_spam", Reason:"message is mostly uppercase"}`

#### Scenario: Service — listas pre-cargadas una vez

- GIVEN un fake repo que cuenta invocaciones,
  `BannedWordsEnabled=true`, `AntiLinkEnabled=true`
- WHEN `HandleMessage` se invoca con un mensaje
- THEN `ListBannedWords` se invoca exactamente 1 vez; la misma
  instancia `*Lists` se pasa a `Registry.Evaluate`

#### Scenario: Integration repo — FK CASCADE borra palabras

- GIVEN Postgres real con migración 00007 aplicada; grupo `g` con 2
  palabras insertadas
- WHEN se ejecuta `DELETE FROM groups WHERE telegram_id = g`
- THEN las 2 filas de `banned_words` para `g` desaparecen

#### Scenario: Handler — auth required en GET settings

- GIVEN el handler sin sesión activa (sin token válido)
- WHEN se ejecuta `GET /api/groups/g/automation/settings`
- THEN responde 401 `UNAUTHORIZED`

#### Scenario: Frontend — agregar palabra dispara POST

- GIVEN la página con `banned_words = []`
- WHEN el admin agrega `"spam"` al `<TagsInput>` y hace click en
  Guardar
- THEN se observa 1 request `POST` a `/automation/banned-words` con
  body `{word:"spam"}`

### Requirement: No regresión

- `backend/internal/automation/rules.go` MUST quedar extendido, NO
  reemplazado: la firma de `FloodRule.Evaluate` se modifica, pero
  la implementación de flood detection se preserva.
- `backend/internal/moderation/` (acciones manuales) MUST quedar
  intacto — bugfix `#172` sigue fuera de scope.
- `publications`, `frontend/src/pages/{DashboardPage,PublicationsPage,
  LoginPage, GroupsPage, GroupUsersPage, GroupRequestsPage,
  GroupLogsPage}.tsx` MUST quedar intactos.
- Las invariantes de AGENTS MUST mantenerse: §4/§25 sin secretos en
  el repo; §18 rate limit vía adapter (auto-actions); §21.1 tests
  mockean `TelegramService`; §13 migraciones con goose.

#### Scenario: `backend/internal/moderation/` intacto

- GIVEN el branch `feat/moderation-automation-slice2`
- WHEN `git diff main -- backend/internal/moderation/` corre
- THEN el output está vacío (cero líneas modificadas)

#### Scenario: `FloodRule` sigue funcionando con la firma extendida

- GIVEN la nueva firma de `Rule.Evaluate(..., lists *Lists, now)`
- WHEN los 5 escenarios de `FloodRule` (sub-umbral, en umbral, sobre
  umbral, expiración, aislamiento) corren
- THEN todos pasan con la firma extendida (pasa `lists=nil` a
  `FloodRule`)

#### Scenario: Páginas frontend preexistentes intactas

- GIVEN el branch del slice
- WHEN `git diff main -- frontend/src/pages/` excluyendo
  `GroupAutomationPage.tsx` y `GroupDetailPage.tsx` corre
- THEN el output está vacío

#### Scenario: permissionOkAdmin invariante

- GIVEN el branch del slice
- WHEN `grep -rn "can_" backend/internal/automation/` corre
- THEN no aparecen checks sobre claves `can_*` (solo
  `permissionOkAdmin` con `BotStatus == StatusAdministrator`)