package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/telegram-manager/backend/internal/groups"
	"github.com/telegram-manager/backend/internal/publications"
	"github.com/telegram-manager/backend/internal/telegram"
)

// publicationStore es la vista minima del servicio de publicaciones que
// los handlers necesitan (lado consumidor). Slice 2:
//   - Publish se conserva para el flujo single-group (backward
//     compat de tests internos).
//   - PublishMany maneja el nuevo body con group_ids[] y devuelve la
//     lista de filas (incluye `photo_url` y `buttons`).
//   - ListByTelegramID implementa el filtro `?group_id=` del listado.
type publicationStore interface {
	Publish(ctx context.Context, actorID, groupID int64, text string, photoURL *string, buttons [][]telegram.InlineKeyboardButton) (*publications.Publication, error)
	PublishMany(ctx context.Context, actorID int64, payload publications.PublishPayload) ([]publications.Publication, error)
	GetByID(ctx context.Context, id int64) (*publications.Publication, error)
	List(ctx context.Context) ([]publications.Publication, error)
	ListByTelegramID(ctx context.Context, telegramID int64) ([]publications.Publication, error)
}

// createPublicationRequest es el body de POST /api/publications:
// `group_ids` es la lista de groups.telegram_id donde publicar
// (slice 2: max 10, min 1). `photo_url` y `buttons` son opcionales.
type createPublicationRequest struct {
	Text     string                            `json:"text"`
	PhotoURL *string                           `json:"photo_url,omitempty"`
	Buttons  [][]telegram.InlineKeyboardButton `json:"buttons,omitempty"`
	GroupIDs []int64                           `json:"group_ids"`
}

// publicationResponse es la vista JSON de una publicacion para el
// panel. Buttons se serializa verbatim (json.RawMessage -> JSONB en
// frontend) para no perder el shape de filas/botones.
type publicationResponse struct {
	ID           int64           `json:"id"`
	TelegramID   int64           `json:"telegram_id"`
	Text         string          `json:"text"`
	Status       string          `json:"status"`
	MessageID    *int64          `json:"message_id"`
	ErrorMessage *string         `json:"error_message"`
	ActorID      *int64          `json:"actor_id"`
	PhotoURL     *string         `json:"photo_url"`
	Buttons      json.RawMessage `json:"buttons"`
	CreatedAt    string          `json:"created_at"`
}

func toPublicationResponse(p *publications.Publication) publicationResponse {
	return publicationResponse{
		ID:           p.ID,
		TelegramID:   p.TelegramID,
		Text:         p.Text,
		Status:       string(p.Status),
		MessageID:    p.MessageID,
		ErrorMessage: p.ErrorMessage,
		ActorID:      p.ActorID,
		PhotoURL:     p.PhotoURL,
		Buttons:      p.Buttons,
		CreatedAt:    p.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}

// handleCreatePublication POST /api/publications — crea y publica ahora
// a 1..N grupos (slice 2: multi-grupo secuencial). Validacion fail-fast
// 400 antes de tocar Telegram/DB; fallo parcial devuelve 201 con cada
// fila y su status individual.
func (s *Server) handleCreatePublication(w http.ResponseWriter, r *http.Request) {
	if s.publications == nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "modulo de publicaciones no habilitado")
		return
	}
	actorID, ok := actorIDFromClaims(w, r)
	if !ok {
		return
	}

	var req createPublicationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "body invalido")
		return
	}

	rows, err := s.publications.PublishMany(r.Context(), actorID, publications.PublishPayload{
		Text:     req.Text,
		PhotoURL: req.PhotoURL,
		Buttons:  req.Buttons,
		GroupIDs: req.GroupIDs,
	})
	if err != nil {
		respondPublicationError(w, err)
		return
	}

	out := make([]publicationResponse, 0, len(rows))
	for i := range rows {
		out = append(out, toPublicationResponse(&rows[i]))
	}
	respond(w, http.StatusCreated, map[string]any{"publications": out})
}

// handleListPublications GET /api/publications — listado mas reciente.
// Acepta `?group_id=<int64>` opcional para filtrar por grupo
// (idx_publications_telegram_id).
func (s *Server) handleListPublications(w http.ResponseWriter, r *http.Request) {
	if s.publications == nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "modulo de publicaciones no habilitado")
		return
	}

	var (
		list []publications.Publication
		err  error
	)
	if gidStr := r.URL.Query().Get("group_id"); gidStr != "" {
		gid, perr := strconv.ParseInt(gidStr, 10, 64)
		if perr != nil {
			respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "group_id invalido")
			return
		}
		list, err = s.publications.ListByTelegramID(r.Context(), gid)
	} else {
		list, err = s.publications.List(r.Context())
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "no se pudieron listar las publicaciones")
		return
	}
	out := make([]publicationResponse, 0, len(list))
	for i := range list {
		out = append(out, toPublicationResponse(&list[i]))
	}
	respond(w, http.StatusOK, out)
}

// handleGetPublication GET /api/publications/:id.
func (s *Server) handleGetPublication(w http.ResponseWriter, r *http.Request) {
	if s.publications == nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "modulo de publicaciones no habilitado")
		return
	}
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	pub, err := s.publications.GetByID(r.Context(), id)
	if errors.Is(err, publications.ErrNotFound) {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "publicacion no encontrada")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "no se pudo obtener la publicacion")
		return
	}
	respond(w, http.StatusOK, toPublicationResponse(pub))
}

// respondPublicationError mapea los errores de dominio de publications
// a los codigos de la seccion 18 (order design D4: 400 → 404 → 403 →
// 502 → 500). Slice 2: incluye errores nuevos de validacion
// (foto, botones, grupos).
func respondPublicationError(w http.ResponseWriter, err error) {
	// 400 — validacion de payload.
	if isPublicationsValidationError(err) {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	switch {
	case errors.Is(err, publications.ErrGroupNotFound), errors.Is(err, groups.ErrNotFound):
		respondError(w, http.StatusNotFound, "NOT_FOUND", "grupo no encontrado")
	case errors.Is(err, publications.ErrBotPermission):
		respondError(w, http.StatusForbidden, "PERMISSION_DENIED",
			"el bot no tiene permisos suficientes en este grupo")
	case errors.Is(err, telegram.ErrPermissionDenied):
		respondError(w, http.StatusForbidden, "PERMISSION_DENIED",
			"el bot no tiene permisos suficientes en este grupo")
	case errors.Is(err, telegram.ErrTelegramNotFound):
		respondError(w, http.StatusNotFound, "NOT_FOUND", "el recurso no existe en Telegram")
	case errors.Is(err, telegram.ErrTelegramUnavailable), errors.Is(err, telegram.ErrWebhookConflict):
		respondError(w, http.StatusBadGateway, "TELEGRAM_ERROR", "Telegram no respondio correctamente")
	case isTelegramAPIError(err):
		respondError(w, http.StatusBadGateway, "TELEGRAM_ERROR", "Telegram rechazo la publicacion")
	default:
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "no se pudo publicar el mensaje")
	}
}

// isPublicationsValidationError identifica los errores de payload que
// responden 400 (slice 2: foto/botones/grupos ademas de texto).
func isPublicationsValidationError(err error) bool {
	for _, target := range validationErrorSet {
		if errors.Is(err, target) {
			return true
		}
	}
	return false
}

// validationErrorSet lista todos los errores que mapean a 400.
// ErrTextEmpty / ErrTextTooLong se mantienen (Publish sigue usandolos
// cuando se llama directo); los errores de foto/botones/grupos los
// emite validatePayload dentro de PublishMany.
var validationErrorSet = []error{
	publications.ErrTextEmpty,
	publications.ErrTextTooLong,
	publications.ErrPhotoURLEmpty,
	publications.ErrPhotoURLScheme,
	publications.ErrPhotoURLTooLong,
	publications.ErrButtonsMalformed,
	publications.ErrButtonsLimit,
	publications.ErrButtonTextLong,
	publications.ErrButtonTextEmpty,
	publications.ErrButtonURLScheme,
	publications.ErrButtonURLLong,
	publications.ErrGroupsEmpty,
	publications.ErrGroupsLimit,
}
