package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/telegram-manager/backend/internal/groups"
	"github.com/telegram-manager/backend/internal/publications"
	"github.com/telegram-manager/backend/internal/telegram"
)

// publicationStore es la vista minima del servicio de publicaciones que
// los handlers necesitan (lado consumidor).
type publicationStore interface {
	Publish(ctx context.Context, actorID, groupID int64, text string) (*publications.Publication, error)
	GetByID(ctx context.Context, id int64) (*publications.Publication, error)
	List(ctx context.Context) ([]publications.Publication, error)
}

// createPublicationRequest es el body de POST /api/publications:
// group_id es groups.telegram_id (id natural usado en Telegram).
type createPublicationRequest struct {
	Text    string `json:"text"`
	GroupID int64  `json:"group_id"`
}

// publicationResponse es la vista JSON de una publicacion para el panel.
type publicationResponse struct {
	ID           int64   `json:"id"`
	TelegramID   int64   `json:"telegram_id"`
	Text         string  `json:"text"`
	Status       string  `json:"status"`
	MessageID    *int64  `json:"message_id"`
	ErrorMessage *string `json:"error_message"`
	ActorID      *int64  `json:"actor_id"`
	CreatedAt    string  `json:"created_at"`
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
		CreatedAt:    p.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}

// handleCreatePublication POST /api/publications — crea y publica ahora.
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

	pub, err := s.publications.Publish(r.Context(), actorID, req.GroupID, req.Text)
	if err != nil {
		respondPublicationError(w, err)
		return
	}
	// 201 Created: la fila existe y el mensaje fue enviado (o fallo,
	// devolviendo igual la fila con status failed). El handler responde
	// la publicacion completa, no solo status ok.
	respond(w, http.StatusCreated, toPublicationResponse(pub))
}

// handleListPublications GET /api/publications — listado mas reciente.
func (s *Server) handleListPublications(w http.ResponseWriter, r *http.Request) {
	if s.publications == nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "modulo de publicaciones no habilitado")
		return
	}
	list, err := s.publications.List(r.Context())
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
// a los codigos de la seccion 18 (order design D4: 404 → 403 → 502 →
// 500).
func respondPublicationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, publications.ErrTextTooLong), errors.Is(err, publications.ErrTextEmpty):
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
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
