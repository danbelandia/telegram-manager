// Package publications orquesta el flujo de creacion + publicacion
// (AGENTS.md §22, slice 1 + slice 2): valida el texto, verifica que el
// grupo exista y que el bot sea administrador, inserta con status
// sending, llama a telegram.SendMessage o telegram.SendPhoto (con
// foto+botones opcionales), actualiza sent/failed + log.
//
// Slice 2 agrega:
//
//   - `PublishMany` para envio multi-grupo SECUENCIAL (design D4).
//     Las validaciones de payload (texto, foto, botones, group_ids)
//     corren UNA sola vez al inicio (fail-fast 400, design D5).
//     Errores per-grupo (404/403/Telegram) NO abortan el resto: cada
//     fila queda `failed` y se registra un log independiente.
//   - Foto por URL (columna `photo_url`); con foto, el texto se envia
//     como caption y el limite baja a 1024 (sendPhoto).
//   - Botones inline (columna JSONB `buttons`); max 8 filas x 8 botones,
//     solo URL (sin callback_data).
//
// Invariante (bugfix #172): permissionOk sigue usando
// `g.BotStatus == StatusAdministrator`. NO reintroducir checks sobre
// claves `can_*`.
package publications

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/telegram-manager/backend/internal/groups"
	"github.com/telegram-manager/backend/internal/logs"
	"github.com/telegram-manager/backend/internal/telegram"
)

// Errores de dominio del flujo (los que NO viven en model.go porque
// nacen de la validacion del servicio, no del modelo de DB).
var (
	// ErrBotPermission: el bot no es administrador del grupo; no se
	// llama a Telegram (403 PERMISSION_DENIED).
	ErrBotPermission = errors.New("publications: el bot no es administrador del grupo")
	// ErrGroupNotFound: el grupo no existe en nuestra base (404).
	ErrGroupNotFound = errors.New("publications: group not found")
)

// GroupReader es la vista minima del repositorio de grupos (lado
// consumidor; *groups.Repository la satisface). Scopeado por tenant
// (slice 0): grupo ajeno → groups.ErrNotFound → ErrGroupNotFound.
type GroupReader interface {
	GetByTenant(ctx context.Context, tenantID, id int64) (*groups.Group, error)
}

// MessageSender es la vista minima del Service de Telegram para
// publicaciones (*telegram.Adapter la satisface). El slice 2 amplio
// SendMessage con un `keyboard` opcional y agrego SendPhoto para
// soportar caption + reply_markup + foto por URL (design D1). T4
// agrega SendVideo (URL) y SendPhotoUpload/SendVideoUpload (multipart).
type MessageSender interface {
	SendMessage(ctx context.Context, chatID int64, text string, disableWebPagePreview bool, keyboard *telegram.InlineKeyboardMarkup) (int64, error)
	SendPhoto(ctx context.Context, chatID int64, photoURL, caption string, keyboard *telegram.InlineKeyboardMarkup) (int64, error)
	SendVideo(ctx context.Context, chatID int64, videoURL, caption string, keyboard *telegram.InlineKeyboardMarkup) (int64, error)
	SendPhotoUpload(ctx context.Context, chatID int64, reader io.Reader, filename, caption string, keyboard *telegram.InlineKeyboardMarkup) (int64, error)
	SendVideoUpload(ctx context.Context, chatID int64, reader io.Reader, filename, caption string, keyboard *telegram.InlineKeyboardMarkup) (int64, error)
}

// LogWriter es la vista minima del repositorio de logs
// (*logs.Repository la satisface).
type LogWriter interface {
	Create(ctx context.Context, e *logs.Entry) error
}

// PubStore es la vista minima del repositorio de publicaciones
// (*publications.Repository la satisface). Slice 2 agrega
// ListByTelegramID para `GET /api/publications?group_id=X`. Slice 3
// agrega limit/offset a los listados, ClaimScheduledDue (SKIP LOCKED)
// para el worker in-process y Cancel para `DELETE /api/publications/:id`.
type PubStore interface {
	Create(ctx context.Context, p *Publication) error
	GetByID(ctx context.Context, tenantID, id int64) (*Publication, error)
	List(ctx context.Context, tenantID int64, limit, offset int) ([]Publication, error)
	ListByTelegramID(ctx context.Context, tenantID, telegramID int64, limit, offset int) ([]Publication, error)
	UpdateStatus(ctx context.Context, tenantID, id int64, status Status, messageID *int64, errMsg *string) error
	ClaimScheduledDue(ctx context.Context, tenantID int64, limit int) ([]Publication, error)
	Cancel(ctx context.Context, tenantID, id int64) error
}

// PublishPayload es el body que el handler entrega al servicio.
// photo_url y buttons son opcionales (nil / slice vacio = "sin foto"
// / "sin teclado"). El servicio valida y luego itera secuencialmente
// sobre group_ids.
type PublishPayload struct {
	Text     string                            `json:"text"`
	PhotoURL *string                           `json:"photo_url,omitempty"`
	VideoURL *string                           `json:"video_url,omitempty"`
	Buttons  [][]telegram.InlineKeyboardButton `json:"buttons,omitempty"`
	GroupIDs []int64                           `json:"group_ids"`
}

// Service ejecuta el flujo de publicaciones. ActorID es el id del admin
// autenticado (claims); se registra como actor en la fila y en el log.
type Service struct {
	groups GroupReader
	tg     MessageSender
	logs   LogWriter
	store  PubStore
}

// NewService construye el servicio de publicaciones.
func NewService(groups GroupReader, tg MessageSender, store PubStore, logs LogWriter) *Service {
	return &Service{groups: groups, tg: tg, logs: logs, store: store}
}

// keyboardOrNil devuelve un *telegram.InlineKeyboardMarkup no-nil
// solo si `rows` tiene contenido; asi el adapter puede usar omitempty
// para omitir el campo `reply_markup` cuando no hay teclado.
func keyboardOrNil(rows [][]telegram.InlineKeyboardButton) *telegram.InlineKeyboardMarkup {
	if len(rows) == 0 {
		return nil
	}
	return &telegram.InlineKeyboardMarkup{InlineKeyboard: rows}
}

// Publish crea una publicacion y la envia al grupo seleccionado. Orden
// (design D4): validar text → grupo existe (404) → permiso (403) →
// create(sending) → sendMessage | sendPhoto → update(sent/failed) → log.
// Convivencia con slice 2: Publish delega en `publishOne` para mantener
// un unico path de envio (PublishMany reutiliza la misma funcion).
func (s *Service) Publish(ctx context.Context, tenantID, actorID, groupID int64, text string, photoURL *string, buttons [][]telegram.InlineKeyboardButton) (*Publication, error) {
	if strings.TrimSpace(text) == "" {
		return nil, ErrTextEmpty
	}
	if photoURL != nil {
		if err := validateTextLength(*photoURL != "", text); err != nil {
			return nil, err
		}
	} else if err := validateTextLength(false, text); err != nil {
		return nil, err
	}

	group, err := s.groups.GetByTenant(ctx, tenantID, groupID)
	if err != nil {
		if errors.Is(err, groups.ErrNotFound) {
			return nil, ErrGroupNotFound
		}
		return nil, fmt.Errorf("publications: publish: get group: %w", err)
	}

	entry := &logs.Entry{
		TenantID: tenantID,
		ActorID:  &actorID,
		GroupID:  groupID,
		Action:   logs.ActionPublishMessage,
	}
	if !permissionOk(group) {
		return nil, s.logFailure(ctx, entry, logs.StatusPermissionDenied, ErrBotPermission)
	}

	pub, err := s.createSending(ctx, tenantID, actorID, groupID, text, photoURL, nil, buttons)
	if err != nil {
		return nil, err
	}
	entry.Metadata = map[string]any{"publication_id": pub.ID}

	messageID, err := s.dispatch(ctx, pub)
	if err != nil {
		s.markFailed(ctx, pub, err.Error())
		return nil, s.logFailure(ctx, entry, statusForTelError(err), err)
	}

	if err := s.store.UpdateStatus(ctx, tenantID, pub.ID, StatusSent, &messageID, nil); err != nil {
		return nil, fmt.Errorf("publications: publish: update sent: %w", err)
	}
	pub.Status = StatusSent
	pub.MessageID = &messageID
	entry.Metadata["message_id"] = messageID
	entry.Status = logs.StatusSuccess
	if err := s.logs.Create(ctx, entry); err != nil {
		return nil, fmt.Errorf("publications: publish: log: %w", err)
	}
	return pub, nil
}

// PublishMany envia una misma publicacion a N grupos en orden, de
// forma SECUENCIAL (nunca paralelo — §18.1 token bucket ordena).
// Validacion fail-fast del payload antes de iterar; los errores
// per-grupo NO abortan el resto: cada fila queda con su status
// (`sent` o `failed`) y su log independiente. Devuelve TODAS las filas
// en el mismo orden que `payload.GroupIDs`, incluso si todas quedan
// `failed` — el handler responde 201 con la lista.
func (s *Service) PublishMany(ctx context.Context, tenantID, actorID int64, payload PublishPayload) ([]Publication, error) {
	if err := validatePayload(payload); err != nil {
		return nil, err
	}

	results := make([]Publication, 0, len(payload.GroupIDs))
	hasPhoto := payload.PhotoURL != nil && *payload.PhotoURL != ""
	hasVideo := payload.VideoURL != nil && *payload.VideoURL != ""

	for _, groupID := range payload.GroupIDs {
		row, logEntry := s.publishOne(ctx, tenantID, actorID, groupID, payload.Text, hasPhoto, payload.PhotoURL, hasVideo, payload.VideoURL, payload.Buttons)
		results = append(results, row)
		_ = logEntry // ya consumido por publishOne
	}
	return results, nil
}

// publishOne ejecuta el flujo completo para UN grupo (404/403/Telegram
// pueden fallar individualmente; el caller decide si aborta). El
// retorno `(Publication, *logs.Entry)` permite al caller inspeccionar
// o agregar metadata adicional; PublishMany lo ignora.
//
// hasPhoto deriva de payload (pre-validado). photoURL es nil cuando no
// hay foto; si hasPhoto=true y photoURL es nil, es un bug del caller
// (PublishMany garantiza la condicion).
func (s *Service) publishOne(ctx context.Context, tenantID, actorID, groupID int64, text string, hasPhoto bool, photoURL *string, hasVideo bool, videoURL *string, buttons [][]telegram.InlineKeyboardButton) (Publication, *logs.Entry) {
	row := Publication{
		TenantID:   tenantID,
		TelegramID: groupID,
		Text:       text,
		Status:     StatusFailed, // default hasta crear la fila
		ActorID:    &actorID,
	}
	entry := &logs.Entry{
		TenantID: tenantID,
		ActorID:  &actorID,
		GroupID:  groupID,
		Action:   logs.ActionPublishMessage,
	}

	group, err := s.groups.GetByTenant(ctx, tenantID, groupID)
	if err != nil {
		if errors.Is(err, groups.ErrNotFound) {
			msg := ErrGroupNotFound.Error()
			row.ErrorMessage = &msg
			_ = s.logFailureNoCtx(ctx, entry, logs.StatusNotFound, ErrGroupNotFound)
			return row, entry
		}
		// error de DB no esperado — lo registramos como internal
		msg := "error leyendo grupo"
		row.ErrorMessage = &msg
		_ = s.logFailureNoCtx(ctx, entry, logs.StatusInternalError, fmt.Errorf("publications: get group: %w", err))
		return row, entry
	}

	if !permissionOk(group) {
		msg := ErrBotPermission.Error()
		row.ErrorMessage = &msg
		_ = s.logFailureNoCtx(ctx, entry, logs.StatusPermissionDenied, ErrBotPermission)
		return row, entry
	}

	// Persistir como sending ANTES de llamar a Telegram.
	pub := &Publication{
		TenantID:   tenantID,
		TelegramID: groupID,
		Text:       text,
		Status:     StatusSending,
		ActorID:    &actorID,
		PhotoURL:   nilIfEmpty(photoURL),
		VideoURL:   nilIfEmpty(videoURL),
	}
	if raw, mErr := MarshalButtons(buttons); mErr == nil {
		pub.Buttons = raw
	}
	if err := s.store.Create(ctx, pub); err != nil {
		msg := "error persistiendo publicacion"
		row.ErrorMessage = &msg
		_ = s.logFailureNoCtx(ctx, entry, logs.StatusInternalError, err)
		return row, entry
	}
	row.ID = pub.ID
	row.CreatedAt = pub.CreatedAt
	row.UpdatedAt = pub.UpdatedAt
	row.PhotoURL = pub.PhotoURL
	row.VideoURL = pub.VideoURL
	row.Buttons = pub.Buttons
	row.Status = StatusSending
	entry.Metadata = map[string]any{"publication_id": pub.ID}

	// Slice 3: el path de envio + finalizacion (dispatch + UpdateStatus +
	// log) vive en publishOneFinalize para que el worker reuse el
	// mismo path sin tener que crear una nueva fila.
	s.publishOneFinalize(ctx, pub, entry, hasPhoto, photoURL, hasVideo, videoURL, buttons)
	// Reflejar el estado final en el row devuelto.
	row.Status = pub.Status
	row.MessageID = pub.MessageID
	row.ErrorMessage = pub.ErrorMessage
	row.UpdatedAt = pub.UpdatedAt
	return row, entry
}

// publishOneFinalize ejecuta el sub-patron comun a Publish y al worker:
// permissionOk ya paso, fila ya existe en DB con status='sending'. Aqui
// se hace el dispatch a Telegram, se persiste sent/failed y se emite
// el log PUBLISH_MESSAGE.
//
// Vive como metodo para que el worker pueda llamarlo sin replicar la
// logica. NO se chequea permissionOk aca: el caller (publishOne o el
// worker via processClaimed) ya lo hizo. Asi evitamos tanto la doble
// consulta a GetByTelegramID como una doble corrida de permissionOk
// (que ya fue validada por publishOne y por el Check del claim).
func (s *Service) publishOneFinalize(ctx context.Context, pub *Publication, entry *logs.Entry, hasPhoto bool, photoURL *string, hasVideo bool, videoURL *string, buttons [][]telegram.InlineKeyboardButton) {
	buttonsJSON, _ := MarshalButtons(buttons) // best-effort; ya estaba persistido

	messageID, err := s.dispatchMedia(ctx, pub.TelegramID, pub.Text, hasPhoto, photoURL, hasVideo, videoURL, buttons)
	if err != nil {
		msg := err.Error()
		_ = s.store.UpdateStatus(ctx, pub.TenantID, pub.ID, StatusFailed, nil, &msg)
		pub.Status = StatusFailed
		pub.ErrorMessage = &msg
		entry.Status = statusForTelError(err)
		entry.ErrorMessage = &msg
		_ = s.logs.Create(ctx, entry)
		return
	}

	if err := s.store.UpdateStatus(ctx, pub.TenantID, pub.ID, StatusSent, &messageID, nil); err != nil {
		msg := "error actualizando estado"
		pub.ErrorMessage = &msg
		pub.Status = StatusFailed
		entry.ErrorMessage = &msg
		entry.Status = logs.StatusInternalError
		_ = s.logs.Create(ctx, entry)
		return
	}
	pub.Status = StatusSent
	pub.MessageID = &messageID
	entry.Metadata["message_id"] = messageID
	entry.Status = logs.StatusSuccess
	// Conservar los bytes de buttons que ya estaban persistidos.
	_ = buttonsJSON
	_ = s.logs.Create(ctx, entry)
}

// dispatch decide si enviar SendPhoto, SendVideo o SendMessage.
// Centralizado para que Publish y publishOne compartan el path.
func (s *Service) dispatch(ctx context.Context, pub *Publication) (int64, error) {
	hasPhoto := pub.PhotoURL != nil && *pub.PhotoURL != ""
	hasVideo := pub.VideoURL != nil && *pub.VideoURL != ""
	if hasPhoto {
		return s.tg.SendPhoto(ctx, pub.TelegramID, *pub.PhotoURL, pub.Text, keyboardOrNil(nil))
	}
	if hasVideo {
		return s.tg.SendVideo(ctx, pub.TelegramID, *pub.VideoURL, pub.Text, keyboardOrNil(nil))
	}
	return s.tg.SendMessage(ctx, pub.TelegramID, pub.Text, false, keyboardOrNil(nil))
}

// dispatchMedia es la version usada por publishOneFinalize antes de
// persistir (sabemos hasPhoto/photoURL/hasVideo/videoURL/buttons
// directamente del payload).
func (s *Service) dispatchMedia(ctx context.Context, groupID int64, text string, hasPhoto bool, photoURL *string, hasVideo bool, videoURL *string, buttons [][]telegram.InlineKeyboardButton) (int64, error) {
	keyboard := keyboardOrNil(buttons)
	if hasPhoto {
		return s.tg.SendPhoto(ctx, groupID, *photoURL, text, keyboard)
	}
	if hasVideo {
		return s.tg.SendVideo(ctx, groupID, *videoURL, text, keyboard)
	}
	return s.tg.SendMessage(ctx, groupID, text, false, keyboard)
}

// createSending persiste la fila en estado sending y devuelve el row
// con ID, CreatedAt y UpdatedAt asignados.
func (s *Service) createSending(ctx context.Context, tenantID, actorID, groupID int64, text string, photoURL *string, videoURL *string, buttons [][]telegram.InlineKeyboardButton) (*Publication, error) {
	pub := &Publication{
		TenantID:   tenantID,
		TelegramID: groupID,
		Text:       text,
		Status:     StatusSending,
		ActorID:    &actorID,
		PhotoURL:   nilIfEmpty(photoURL),
		VideoURL:   nilIfEmpty(videoURL),
	}
	if raw, err := MarshalButtons(buttons); err == nil {
		pub.Buttons = raw
	}
	if err := s.store.Create(ctx, pub); err != nil {
		return nil, fmt.Errorf("publications: publish: create: %w", err)
	}
	return pub, nil
}

// markFailed actualiza una fila a failed con error_message.
func (s *Service) markFailed(ctx context.Context, pub *Publication, errMsg string) {
	if uerr := s.store.UpdateStatus(ctx, pub.TenantID, pub.ID, StatusFailed, nil, &errMsg); uerr != nil {
		// No abortamos la respuesta: si no se puede actualizar, el
		// caller ya tiene el error original.
		_ = uerr
	}
	pub.Status = StatusFailed
	pub.ErrorMessage = &errMsg
}

// GetByID devuelve una publicacion del tenant por su ID interno.
func (s *Service) GetByID(ctx context.Context, tenantID, id int64) (*Publication, error) {
	return s.store.GetByID(ctx, tenantID, id)
}

// List devuelve las publicaciones del tenant paginadas (created_at
// DESC). El handler valida limit/offset antes de invocar (slice 3:
// paginacion).
func (s *Service) List(ctx context.Context, tenantID int64, limit, offset int) ([]Publication, error) {
	return s.store.List(ctx, tenantID, limit, offset)
}

// ListByTelegramID devuelve las publicaciones del tenant paginadas de
// un grupo especifico (created_at DESC,
// idx_publications_telegram_id).
func (s *Service) ListByTelegramID(ctx context.Context, tenantID, telegramID int64, limit, offset int) ([]Publication, error) {
	return s.store.ListByTelegramID(ctx, tenantID, telegramID, limit, offset)
}

// Schedule inserta N filas con status='scheduled' sin tocar Telegram.
// Devuelve todas las filas en el orden de `payload.GroupIDs`. El caller
// (worker, via claim) se encarga luego de llamar a publishOne por cada
// fila cuando llegue su `scheduled_at`. `nowFn` se inyecta para
// determinismo en tests (default time.Now si nil).
func (s *Service) Schedule(ctx context.Context, tenantID, actorID int64, payload PublishPayload, scheduledAt time.Time, nowFn func() time.Time) ([]Publication, error) {
	if err := validatePayload(payload); err != nil {
		return nil, err
	}
	if nowFn == nil {
		nowFn = time.Now
	}
	if err := validateScheduledAt(scheduledAt, nowFn); err != nil {
		return nil, err
	}

	results := make([]Publication, 0, len(payload.GroupIDs))
	for _, groupID := range payload.GroupIDs {
		row := Publication{
			TenantID:   tenantID,
			TelegramID: groupID,
			Text:       payload.Text,
			Status:     StatusScheduled,
			ActorID:    &actorID,
			PhotoURL:   nilIfEmpty(payload.PhotoURL),
			VideoURL:   nilIfEmpty(payload.VideoURL),
		}
		if raw, err := MarshalButtons(payload.Buttons); err == nil {
			row.Buttons = raw
		}
		utc := scheduledAt.UTC()
		row.ScheduledAt = &utc
		if err := s.store.Create(ctx, &row); err != nil {
			return nil, fmt.Errorf("publications: schedule: create: %w", err)
		}
		results = append(results, row)
	}
	return results, nil
}

// CancelScheduled borra una publicacion `scheduled`. Reglas:
//   - inexistente -> ErrNotFound (404).
//   - status != 'scheduled' -> ErrCancelNotAllowed (409).
//   - status == 'scheduled' -> hard delete; el siguiente tick la ignora.
func (s *Service) CancelScheduled(ctx context.Context, tenantID, id int64) error {
	_, err := s.store.GetByID(ctx, tenantID, id)
	if err != nil {
		// Ya incluye ErrNotFound (404) o un error interno.
		return err
	}
	return s.store.Cancel(ctx, tenantID, id)
}

// validatePayload corre UNA sola vez al inicio de PublishMany (fail-fast
// 400 antes de tocar Telegram o la tabla). Los errores per-grupo
// (404/403/Telegram) NO entran aca — esos se manejan en publishOne.
func validatePayload(p PublishPayload) error {
	text := strings.TrimSpace(p.Text)
	if text == "" {
		return ErrTextEmpty
	}
	hasPhoto := p.PhotoURL != nil && *p.PhotoURL != ""
	hasVideo := p.VideoURL != nil && *p.VideoURL != ""

	if hasPhoto && hasVideo {
		return ErrMediaExclusive
	}

	if err := validateTextLength(hasPhoto || hasVideo, p.Text); err != nil {
		return err
	}

	// Foto (opcional).
	if p.PhotoURL != nil {
		if err := validatePhotoURL(*p.PhotoURL); err != nil {
			return err
		}
	}

	// Video (opcional).
	if p.VideoURL != nil {
		if err := validateVideoURL(*p.VideoURL); err != nil {
			return err
		}
	}

	// Botones (opcional). Si vienen vacios o solo con filas vacias,
	// se tratan como ausentes (sin teclado).
	if len(p.Buttons) > 0 {
		if err := validateButtons(p.Buttons); err != nil {
			return err
		}
	}

	// Grupos: 1..10.
	if len(p.GroupIDs) == 0 {
		return ErrGroupsEmpty
	}
	if len(p.GroupIDs) > maxGroupsPerPublish {
		return ErrGroupsLimit
	}

	return nil
}

// validateTextLength aplica el limite correcto segun haya foto o no:
// caption<=1024 con foto, text<=4096 sin foto. El handler mapea este
// error a 400 con mensaje en espanol.
func validateTextLength(hasPhoto bool, text string) error {
	limit := maxTextLength
	if hasPhoto {
		limit = maxCaptionLength
	}
	if len(text) > limit {
		if hasPhoto {
			return errors.New("el texto excede 1024 caracteres cuando se envia con foto")
		}
		return ErrTextTooLong
	}
	return nil
}

// validatePhotoURL exige http(s) + longitud <= maxPhotoURLLength.
func validatePhotoURL(raw string) error {
	if raw == "" {
		return ErrPhotoURLEmpty
	}
	if len(raw) > maxPhotoURLLength {
		return ErrPhotoURLTooLong
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ErrPhotoURLScheme
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return ErrPhotoURLScheme
	}
	return nil
}

// validateVideoURL exige http(s) + longitud <= maxVideoURLLength.
func validateVideoURL(raw string) error {
	if raw == "" {
		return ErrVideoURLEmpty
	}
	if len(raw) > maxVideoURLLength {
		return ErrVideoURLTooLong
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ErrVideoURLScheme
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return ErrVideoURLScheme
	}
	return nil
}

// validateButtons valida la estructura y cada boton.
// `rows` ya paso la condicion `len > 0` (el caller la chequea).
func validateButtons(rows [][]telegram.InlineKeyboardButton) error {
	if len(rows) > maxButtonsRows {
		return ErrButtonsLimit
	}
	for _, row := range rows {
		if len(row) > maxButtonsPerRow {
			return ErrButtonsLimit
		}
		for _, btn := range row {
			if strings.TrimSpace(btn.Text) == "" {
				return ErrButtonTextEmpty
			}
			if len(btn.Text) > maxButtonTextLength {
				return ErrButtonTextLong
			}
			if btn.URL == "" {
				return ErrButtonURLScheme
			}
			if len(btn.URL) > maxButtonURLLength {
				return ErrButtonURLLong
			}
			u, err := url.Parse(btn.URL)
			if err != nil {
				return ErrButtonURLScheme
			}
			scheme := strings.ToLower(u.Scheme)
			if scheme != "http" && scheme != "https" {
				return ErrButtonURLScheme
			}
		}
	}
	return nil
}

// permissionOk exige que el bot sea administrador del grupo
// (groups.BotStatus == administrator).
//
// Nota (bugfix 2026-09-07, topic `sdd/publications/permission-check`):
// la Bot API NO exige ninguna bot_permission can_* para sendMessage en
// grupos; el fix original usaba "can_manage_chat" pero la deteccion de
// grupos jamas puebla esa clave (events.go copia solo
// can_delete/restrict/pin/invite/promote/change_info), por lo que el
// check daba 403 para todos los grupos. Exigir administrator es
// correcto porque (1) es el estado confiable que el sistema ya conoce
// y (2) los admins de Telegram estan exentos de las restricciones de
// envio del grupo (lock), garantizando que la publicacion proceda. En
// canales, si el admin no puede postear, Telegram devuelve error y la
// publicacion queda failed con mensaje real (no se simula el permiso).
//
// Aplica IGUAL a sendPhoto (la Bot API no pide una can_* especifica
// para enviar fotos a un grupo siendo admin; el estado admin exime de
// las restricciones del grupo).
func permissionOk(g *groups.Group) bool {
	return g != nil && g.BotStatus == groups.StatusAdministrator
}

// logFailure registra un log de fallo y devuelve el error envuelto.
// Usado por Publish (slice 1, single-group) que aborta el flujo.
func (s *Service) logFailure(ctx context.Context, entry *logs.Entry, status logs.Status, cause error) error {
	entry.Status = status
	msg := cause.Error()
	entry.ErrorMessage = &msg
	if err := s.logs.Create(ctx, entry); err != nil {
		return fmt.Errorf("publications: %s: log failure: %w", entry.Action, err)
	}
	return fmt.Errorf("publications: %s: %w", entry.Action, cause)
}

// logFailureNoCtx es la version "best-effort" usada por publishOne:
// ignora el error de log (ya hay un fallo principal que reportar) y
// devuelve el error envuelto sin propagarlo. Asi publishOne puede
// continuar el loop de PublishMany aunque falle un log.
func (s *Service) logFailureNoCtx(ctx context.Context, entry *logs.Entry, status logs.Status, cause error) error {
	entry.Status = status
	msg := cause.Error()
	entry.ErrorMessage = &msg
	if err := s.logs.Create(ctx, entry); err != nil {
		// Best-effort: loguear en stderr para debugging pero no romper.
		_ = err
	}
	return fmt.Errorf("publications: %s: %w", entry.Action, cause)
}

// statusForTelError mapea un error del adapter al status de log.
func statusForTelError(err error) logs.Status {
	switch {
	case errors.Is(err, telegram.ErrPermissionDenied):
		return logs.StatusPermissionDenied
	case errors.Is(err, telegram.ErrTelegramNotFound):
		return logs.StatusNotFound
	default:
		return logs.StatusTelegramError
	}
}

// nilIfEmpty devuelve nil si el puntero apunta a string vacio, asi
// la columna `photo_url` queda NULL en vez de string vacio.
func nilIfEmpty(p *string) *string {
	if p == nil || *p == "" {
		return nil
	}
	return p
}
