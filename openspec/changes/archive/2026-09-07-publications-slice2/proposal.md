# Proposal: Publications — Slice 2 (Foto URL + Botones + Multi-Grupo + Filtro)

## Intent

Extender `publications` (AGENTS §22) con contenido enriquecido y envío simultáneo a varios grupos, sobre la tabla única del slice 1 y respetando bugfix #172 (no reintroducir checks `can_*`; usar `bot_status==administrator`).

## Scope

### In Scope

- Migración `00005`: `ALTER publications ADD photo_url TEXT NULL, ADD buttons JSONB NULL`.
- Adapter: `SendPhoto(ctx, chatID, photoURL, caption, keyboard)` + tipos `InlineKeyboardMarkup`/`InlineKeyboardButton`; `SendMessage` acepta `*InlineKeyboardMarkup` opcional.
- Servicio: `PublishMany` secuencial sobre `group_ids` (≤10); UNA fila por grupo; fallo parcial; caption 1024 con foto / 4096 sin foto; botones máx 8×8, text ≤ 64, url http(s).
- API: `POST /api/publications {text, photo_url?, buttons?, group_ids[]}` → 201 `{publications:[…]}`; `GET ?group_id=` filtra; `GET /:id` sin cambios.
- Frontend: form con input foto URL, editor botones (filas text+url), multi-select grupos, max dinámico 1024/4096; listado muestra foto + chips de botones + filtro; query key `['publications', group_id]`.
- README: sección publicaciones + `/publications` en tabla del panel.
- Tests: adapter (SendPhoto + 429 + keyboard JSON), service (orden, fallo parcial, validaciones, bot_status), handler (201/400, filtro), frontend.

### Out of Scope

`sendMediaGroup`, botones `callback_data`, upload multipart, scheduling, paginación >50.

## Capabilities

### Modified Capabilities

- `publications-publish-now` (delta): payload `group_ids[]`+`photo_url`+`buttons`; respuesta `{publications:[]}`; filtro `?group_id=`.
- `telegram-moderation` (delta): `Service` gana `SendPhoto` + tipos teclado; `SendMessage` suma `keyboard *InlineKeyboardMarkup`.

**Decisión de interfaz (DECIDE)**: métodos estrechos paralelos (`SendMessage`, `SendPhoto`) en lugar de `Send(chatID, opts)` unificado — coherente con el patrón narrow del slice 1 y evita opts-struct para el caso simple. **Costo**: toda fake que implemente `Service` (ej. `telegram/poller_test.go fakeService`) debe agregar `SendPhoto` y la nueva firma de `SendMessage`. Flag explícito para tasks.

## Approach

| # | Decisión | Cómo |
|---|----------|------|
| 1 | Foto + texto | Una sola `sendPhoto(photo_url, caption=text, reply_markup)`; nunca sendPhoto+sendMessage |
| 2 | Botones | JSONB `[[{text,url}]]` → `inline_keyboard`; omit `reply_markup` si nil |
| 3 | Multi-grupo | Loop secuencial sobre `group_ids`; rate limiter (§18.1) ordena; sin goroutines paralelas |
| 4 | Validación | Texto/foto/botones fail-fast una vez antes del loop; errores de grupo no abortan |
| 5 | Permiso | `g.BotStatus == StatusAdministrator`; bugfix #172 vigente |

**Límites operacionales**: foto URL pública ≤ 5 MB (Telegram descarga), caption ≤ 1024 con foto, texto ≤ 4096 sin foto, máx 10 grupos/POST.

## Affected Areas

| Area | Impact |
|------|--------|
| `backend/migrations/00005_alter_publications.sql` | New (ALTER + Down) |
| `backend/internal/telegram/{service,publications}.go` | Modified (SendPhoto, tipos, keyboard en params) |
| `backend/internal/telegram/*_test.go` (fakes) | Modified (SendPhoto + nueva firma SendMessage) |
| `backend/internal/publications/{model,repository,service}.go` | Modified (campos, validación, PublishMany, ListByGroupID) |
| `backend/internal/api/publications_handlers.go` | Modified (body/respuesta/query) |
| `frontend/src/features/publications/{types,api,hooks,error}.ts` | Modified |
| `frontend/src/pages/PublicationsPage.tsx` + test | Modified |
| `README.md` | Modified (sección publicaciones + tabla panel) |

## Risks

| Risk | Mitigation |
|------|------------|
| Bloqueo HTTP con 10 envíos secuenciales | Límite 10; sync aceptable; async a slice 3 si hace falta |
| URL foto inaccesible o >5 MB | Telegram devuelve error → `failed` + `error_message`; documentar en README |
| Fakes `Service` desincronizadas | Flag explícito en la propuesta; checklist en tasks |

## Rollback

`goose down` a 00004 (DROP columnas); revertir adapter, fakes, frontend y README. Filas slice 1 quedan con `photo_url=NULL, buttons=NULL`.

## Dependencies

- Slice 1 archivado (`ace1f59`) + bugfix #172 (`9d4f1ec`).
- `telegram.Service.SendMessage` + token bucket + `doWithRetry`.

## Success Criteria

- [ ] 00005 Up/Down aplica limpia
- [ ] POST `{group_ids:[2], photo_url, buttons}` crea 2 filas y envía
- [ ] caption > 1024 con foto / texto > 4096 sin foto / > 10 grupos / URL no http(s) / > 8 filas o botones → 400
- [ ] Fallo parcial: 1 grupo 404/403/TELEGRAM_ERROR no aborta; 201 con cada fila y su status
- [ ] GET `?group_id=X` filtra con índice existente
- [ ] Frontend: form foto+botones+multi-grupo, listado con preview, filtro recarga
- [ ] Fakes `Service` actualizadas
- [ ] `go test ./...` y `npm test` en verde; `npm run build` ok

## Open Questions

Ninguna — la exploración resolvió todo (URL only, max 10, sin `callback_data`, sin upload, sin paginación).
