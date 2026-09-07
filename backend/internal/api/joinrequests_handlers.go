package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/telegram-manager/backend/internal/joinrequests"
	"github.com/telegram-manager/backend/internal/moderation"
)

// joinRequestStore es la vista minima del repositorio de solicitudes de
// ingreso que los handlers necesitan (lado consumidor).
type joinRequestStore interface {
	ListByGroup(ctx context.Context, groupID int64) ([]joinrequests.Request, error)
}

// joinRequestResponse es la vista JSON de una solicitud de ingreso.
type joinRequestResponse struct {
	ID          int64      `json:"id"`
	GroupID     int64      `json:"group_id"`
	UserID      int64      `json:"user_id"`
	FirstName   string     `json:"first_name"`
	Username    *string    `json:"username"`
	Status      string     `json:"status"`
	RequestedAt time.Time  `json:"requested_at"`
	DecidedAt   *time.Time `json:"decided_at"`
	DecidedBy   *int64     `json:"decided_by"`
}

// toJoinRequestResponse adapta un Request del dominio a su JSON.
func toJoinRequestResponse(r *joinrequests.Request) joinRequestResponse {
	out := joinRequestResponse{
		ID:          r.ID,
		GroupID:     r.GroupID,
		UserID:      r.UserID,
		FirstName:   r.FirstName,
		Username:    r.Username,
		Status:      string(r.Status),
		RequestedAt: r.RequestedAt,
		DecidedAt:   r.DecidedAt,
		DecidedBy:   r.DecidedBy,
	}
	return out
}

// handleListJoinRequests GET /api/groups/{id}/join-requests (decision
// P2: pendientes y decididas, mas recientes primero).
func (s *Server) handleListJoinRequests(w http.ResponseWriter, r *http.Request) {
	if s.joinRequests == nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "modulo de solicitudes no habilitado")
		return
	}
	groupID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	list, err := s.joinRequests.ListByGroup(r.Context(), groupID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "no se pudieron listar las solicitudes")
		return
	}
	out := make([]joinRequestResponse, 0, len(list))
	for _, req := range list {
		out = append(out, toJoinRequestResponse(&req))
	}
	respond(w, http.StatusOK, out)
}

// handleApproveJoinRequest POST /api/groups/{id}/join-requests/{requestId}/approve.
func (s *Server) handleApproveJoinRequest(w http.ResponseWriter, r *http.Request) {
	if s.moderation == nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "modulo de moderacion no habilitado")
		return
	}
	groupID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	requestID, ok := pathID(w, r, "requestId")
	if !ok {
		return
	}
	actorID, ok := actorIDFromClaims(w, r)
	if !ok {
		return
	}

	err := s.moderation.Approve(r.Context(), actorID, groupID, requestID)
	if !respondJoinRequestError(w, err) {
		respond(w, http.StatusOK, map[string]string{"status": "approved"})
	}
}

// handleRejectJoinRequest POST /api/groups/{id}/join-requests/{requestId}/reject.
func (s *Server) handleRejectJoinRequest(w http.ResponseWriter, r *http.Request) {
	if s.moderation == nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "modulo de moderacion no habilitado")
		return
	}
	groupID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	requestID, ok := pathID(w, r, "requestId")
	if !ok {
		return
	}
	actorID, ok := actorIDFromClaims(w, r)
	if !ok {
		return
	}

	err := s.moderation.Reject(r.Context(), actorID, groupID, requestID)
	if !respondJoinRequestError(w, err) {
		respond(w, http.StatusOK, map[string]string{"status": "rejected"})
	}
}

// respondJoinRequestError mapea errores de approve/reject (seccion 18).
// Reusa los sentinels de dominio de moderation, que son los que
// devuelve el Service luego de orquestar grupo→permiso→telegram→log.
func respondJoinRequestError(w http.ResponseWriter, err error) bool {
	if errors.Is(err, moderation.ErrGroupNotFound) || errors.Is(err, moderation.ErrRequestNotFound) {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "solicitud o grupo no encontrado")
		return true
	}
	if errors.Is(err, moderation.ErrRequestAlreadyDecided) {
		respondError(w, http.StatusConflict, "VALIDATION_ERROR", "la solicitud ya fue decidida")
		return true
	}
	if errors.Is(err, moderation.ErrBotPermission) {
		respondError(w, http.StatusForbidden, "PERMISSION_DENIED",
			"el bot no tiene permisos suficientes en este grupo")
		return true
	}
	// Errores de Telegram y cualquier otro.
	return respondModerationError(w, err)
}
