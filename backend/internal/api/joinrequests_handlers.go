package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/telegram-manager/backend/internal/groups"
	"github.com/telegram-manager/backend/internal/joinrequests"
	"github.com/telegram-manager/backend/internal/moderation"
)

// joinRequestStore es la vista minima del repositorio de solicitudes de
// ingreso que los handlers necesitan (lado consumidor). Scopeado por
// tenant (slice 0).
type joinRequestStore interface {
	ListByGroup(ctx context.Context, tenantID, groupID int64) ([]joinrequests.Request, error)
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

// checkGroupOwnership verifica que el grupo pertenece al tenant (D9):
// ajeno → NOT_FOUND sin llamar a Telegram. Si el modulo de grupos no
// esta montado (solo tests selectivos), se omite — en produccion main
// siempre monta todos los modulos juntos.
func checkGroupOwnership(s *Server, w http.ResponseWriter, r *http.Request, tenantID, groupID int64) bool {
	if s.groups == nil {
		return true
	}
	if _, err := s.groups.GetByTenant(r.Context(), tenantID, groupID); err != nil {
		if errors.Is(err, groups.ErrNotFound) {
			respondError(w, http.StatusNotFound, "NOT_FOUND", "grupo no encontrado")
			return false
		}
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "no se pudo obtener el grupo")
		return false
	}
	return true
}

// handleListJoinRequests GET /api/groups/{id}/join-requests (decision
// P2: pendientes y decididas, mas recientes primero). Solo filas del
// tenant (iso-listados).
func (s *Server) handleListJoinRequests(w http.ResponseWriter, r *http.Request) {
	if s.joinRequests == nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "modulo de solicitudes no habilitado")
		return
	}
	tenantID, ok := tenantIDFromClaims(w, r)
	if !ok {
		return
	}
	groupID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if !checkGroupOwnership(s, w, r, tenantID, groupID) {
		return
	}
	list, err := s.joinRequests.ListByGroup(r.Context(), tenantID, groupID)
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
	tenantID, ok := tenantIDFromClaims(w, r)
	if !ok {
		return
	}
	mod, ok := s.resolveModeration(tenantID)
	if !ok {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "modulo de moderacion no habilitado")
		return
	}
	if !checkGroupOwnership(s, w, r, tenantID, groupID) {
		return
	}

	err := mod.Approve(r.Context(), actorID, groupID, requestID)
	if !respondJoinRequestError(w, err) {
		respond(w, http.StatusOK, map[string]string{"status": "approved"})
	}
}

// handleRejectJoinRequest POST /api/groups/{id}/join-requests/{requestId}/reject.
func (s *Server) handleRejectJoinRequest(w http.ResponseWriter, r *http.Request) {
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
	tenantID, ok := tenantIDFromClaims(w, r)
	if !ok {
		return
	}
	mod, ok := s.resolveModeration(tenantID)
	if !ok {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "modulo de moderacion no habilitado")
		return
	}
	if !checkGroupOwnership(s, w, r, tenantID, groupID) {
		return
	}

	err := mod.Reject(r.Context(), actorID, groupID, requestID)
	if !respondJoinRequestError(w, err) {
		respond(w, http.StatusOK, map[string]string{"status": "rejected"})
	}
}

// ── Batch ────────────────────────────────────────────────────────────

// maxBatchJoinRequests es el cap maximo de solicitudes por batch.
const maxBatchJoinRequests = 50

// batchJoinRequest es el body de POST /api/groups/{id}/join-requests/batch.
type batchJoinRequest struct {
	Action     string  `json:"action"`
	RequestIDs []int64 `json:"request_ids"`
}

// batchJoinResultItem es un item dentro de results[].
type batchJoinResultItem struct {
	ID     int64   `json:"id"`
	Status string  `json:"status"`
	Error  *string `json:"error,omitempty"`
}

// batchJoinResponse es el body de 200 OK de POST .../batch.
type batchJoinResponse struct {
	Results []batchJoinResultItem `json:"results"`
}

// handleBatchJoinRequests POST /api/groups/{id}/join-requests/batch —
// procesa N solicitudes de una sola vez (batch approve/reject).
// Reusa decide() via BatchDecide: grupo resuelto una vez, cada request
// itera grupo→solicitud→permiso→telegram→log. 200 OK incluso si todos
// los items fallaron (failure-isolation per-item, mismo patron que
// publications-batch).
func (s *Server) handleBatchJoinRequests(w http.ResponseWriter, r *http.Request) {
	groupID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	actorID, ok := actorIDFromClaims(w, r)
	if !ok {
		return
	}
	tenantID, ok := tenantIDFromClaims(w, r)
	if !ok {
		return
	}
	mod, ok := s.resolveModeration(tenantID)
	if !ok {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "modulo de moderacion no habilitado")
		return
	}
	if !checkGroupOwnership(s, w, r, tenantID, groupID) {
		return
	}

	var req batchJoinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "body invalido")
		return
	}

	// Validar action enum.
	if req.Action != "approve" && req.Action != "reject" {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR",
			"action debe ser 'approve' o 'reject'")
		return
	}

	// Validar request_ids no vacio y cap.
	if len(req.RequestIDs) == 0 {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR",
			"request_ids no puede estar vacio")
		return
	}
	if len(req.RequestIDs) > maxBatchJoinRequests {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR",
			"maximo 50 solicitudes por batch")
		return
	}

	results, err := mod.BatchDecide(r.Context(), actorID, groupID, req.Action, req.RequestIDs)
	if err != nil {
		// Error de grupo (not found / permission denied): aborta todo.
		respondJoinRequestError(w, err)
		return
	}

	resp := batchJoinResponse{
		Results: make([]batchJoinResultItem, 0, len(results)),
	}
	for _, r := range results {
		item := batchJoinResultItem{
			ID:     r.ID,
			Status: r.Status,
		}
		if r.Error != "" {
			item.Error = &r.Error
		}
		resp.Results = append(resp.Results, item)
	}

	respond(w, http.StatusOK, resp)
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
