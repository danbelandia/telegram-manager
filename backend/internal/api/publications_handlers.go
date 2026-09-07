package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

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
//
// Slice 3 agrega:
//   - limit/offset a List/ListByTelegramID (paginacion).
//   - Schedule para la rama `scheduled_at` del POST.
//   - CancelScheduled para DELETE /api/publications/:id.
type publicationStore interface {
	Publish(ctx context.Context, actorID, groupID int64, text string, photoURL *string, buttons [][]telegram.InlineKeyboardButton) (*publications.Publication, error)
	PublishMany(ctx context.Context, actorID int64, payload publications.PublishPayload) ([]publications.Publication, error)
	Schedule(ctx context.Context, actorID int64, payload publications.PublishPayload, scheduledAt time.Time, nowFn func() time.Time) ([]publications.Publication, error)
	GetByID(ctx context.Context, id int64) (*publications.Publication, error)
	List(ctx context.Context, limit, offset int) ([]publications.Publication, error)
	ListByTelegramID(ctx context.Context, telegramID int64, limit, offset int) ([]publications.Publication, error)
	CancelScheduled(ctx context.Context, id int64) error
}

// createPublicationRequest es el body de POST /api/publications:
// `group_ids` es la lista de groups.telegram_id donde publicar
// (slice 2: max 10, min 1). `photo_url` y `buttons` son opcionales.
// Slice 3 agrega `scheduled_at` opcional (RFC3339 con offset; se
// normaliza a UTC y se valida como futuro estricto).
type createPublicationRequest struct {
	Text        string                            `json:"text"`
	PhotoURL    *string                           `json:"photo_url,omitempty"`
	Buttons     [][]telegram.InlineKeyboardButton `json:"buttons,omitempty"`
	GroupIDs    []int64                           `json:"group_ids"`
	ScheduledAt string                            `json:"scheduled_at,omitempty"`
}

// publicationResponse es la vista JSON de una publicacion para el
// panel. Buttons se serializa verbatim (json.RawMessage -> JSONB en
// frontend) para no perder el shape de filas/botones. Slice 3 agrega
// `scheduled_at` (TIMESTAMPTZ nullable).
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
	ScheduledAt  *string         `json:"scheduled_at"`
	CreatedAt    string          `json:"created_at"`
}

func toPublicationResponse(p *publications.Publication) publicationResponse {
	resp := publicationResponse{
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
	if p.ScheduledAt != nil {
		s := p.ScheduledAt.UTC().Format(time.RFC3339)
		resp.ScheduledAt = &s
	}
	return resp
}

// handleCreatePublication POST /api/publications — crea y publica ahora
// a 1..N grupos (slice 2: multi-grupo secuencial). Slice 3: si el body
// incluye `scheduled_at` futuro, inserta filas `scheduled` SIN tocar
// Telegram (las procesara el worker). Validacion fail-fast 400 antes
// de tocar Telegram/DB; fallo parcial devuelve 201 con cada fila y su
// status individual.
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

	// Slice 3: rama schedule. Si scheduled_at esta presente, validar y
	// delegar en Schedule. Service valida payload + futuro estricto.
	if req.ScheduledAt != "" {
		t, err := publications.NormalizeScheduledAt(req.ScheduledAt, time.Now)
		if err != nil {
			respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
			return
		}
		rows, err := s.publications.Schedule(r.Context(), actorID, publications.PublishPayload{
			Text:     req.Text,
			PhotoURL: req.PhotoURL,
			Buttons:  req.Buttons,
			GroupIDs: req.GroupIDs,
		}, *t, time.Now)
		if err != nil {
			respondPublicationError(w, err)
			return
		}
		out := make([]publicationResponse, 0, len(rows))
		for i := range rows {
			out = append(out, toPublicationResponse(&rows[i]))
		}
		respond(w, http.StatusCreated, map[string]any{"publications": out})
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

// handleListPublications GET /api/publications — listado paginado.
// Acepta:
//   - ?group_id=<int64> opcional para filtrar por grupo
//     (idx_publications_telegram_id, slice 2).
//   - ?limit=<int> opcional, default 50, max 100 (slice 3).
//   - ?offset=<int> opcional, default 0 (slice 3).
//
// 400 VALIDATION_ERROR si limit/offset fuera de rango o si group_id no
// parsea como entero.
func (s *Server) handleListPublications(w http.ResponseWriter, r *http.Request) {
	if s.publications == nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "modulo de publicaciones no habilitado")
		return
	}

	limit, offset, ok := parsePaginationQuery(w, r)
	if !ok {
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
		list, err = s.publications.ListByTelegramID(r.Context(), gid, limit, offset)
	} else {
		list, err = s.publications.List(r.Context(), limit, offset)
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

// parsePaginationQuery lee ?limit y ?offset y aplica NormalizePagination.
// Retorna false (y responde 400) si alguno de los dos esta fuera de rango.
func parsePaginationQuery(w http.ResponseWriter, r *http.Request) (int, int, bool) {
	q := r.URL.Query()
	limitStr := q.Get("limit")
	offsetStr := q.Get("offset")

	limit := 0
	offset := 0
	if limitStr != "" {
		v, err := strconv.Atoi(limitStr)
		if err != nil {
			respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "limit invalido")
			return 0, 0, false
		}
		limit = v
	}
	if offsetStr != "" {
		v, err := strconv.Atoi(offsetStr)
		if err != nil {
			respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "offset invalido")
			return 0, 0, false
		}
		offset = v
	}
	// Solo validamos si el cliente mando AL MENOS uno de los dos;
	// si ambos vienen vacios, NormalizePagination aplica default
	// sin invocar validatePagination (que devolveria error por limit=0).
	if limitStr != "" || offsetStr != "" {
		if err := publications.ValidatePagination(limit, offset); err != nil {
			respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
			return 0, 0, false
		}
	}
	limit, offset = publications.NormalizePagination(limit, offset)
	return limit, offset, true
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

// handleDeletePublication DELETE /api/publications/:id (slice 3).
// Cancela una publicacion `scheduled`. Hard delete SOLO si status
// coincide; cualquier otro status devuelve 409 INVALID_STATUS para
// preservar audit trail de sent/failed.
func (s *Server) handleDeletePublication(w http.ResponseWriter, r *http.Request) {
	if s.publications == nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "modulo de publicaciones no habilitado")
		return
	}
	if _, ok := actorIDFromClaims(w, r); !ok {
		return
	}
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	err := s.publications.CancelScheduled(r.Context(), id)
	switch {
	case err == nil:
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, publications.ErrNotFound):
		respondError(w, http.StatusNotFound, "NOT_FOUND", "publicacion no encontrada")
	case errors.Is(err, publications.ErrCancelNotAllowed):
		respondError(w, http.StatusConflict, "INVALID_STATUS",
			"no se puede cancelar una publicacion ya enviada o en curso")
	default:
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR",
			"no se pudo cancelar la publicacion")
	}
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
// emite validatePayload dentro de PublishMany. Slice 3 agrega
// ErrScheduledInPast y ErrInvalidPagination.
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
	publications.ErrScheduledInPast,
	publications.ErrInvalidPagination,
}
