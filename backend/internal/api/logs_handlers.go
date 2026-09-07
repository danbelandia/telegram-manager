package api

import (
	"context"
	"net/http"
	"time"

	"github.com/telegram-manager/backend/internal/logs"
)

// logStore es la vista minima del repositorio de logs que el handler
// necesita (lado consumidor).
type logStore interface {
	ListByGroup(ctx context.Context, groupID int64) ([]logs.Entry, error)
}

// logEntryResponse es la vista JSON de una entrada de auditoria.
type logEntryResponse struct {
	ID           int64          `json:"id"`
	ActorID      *int64         `json:"actor_id"`
	GroupID      int64          `json:"group_id"`
	Action       string         `json:"action"`
	TargetUserID *int64         `json:"target_user_id"`
	Metadata     map[string]any `json:"metadata"`
	Status       string         `json:"status"`
	ErrorMessage *string        `json:"error_message"`
	CreatedAt    time.Time      `json:"created_at"`
}

func toLogEntryResponse(e *logs.Entry) logEntryResponse {
	return logEntryResponse{
		ID:           e.ID,
		ActorID:      e.ActorID,
		GroupID:      e.GroupID,
		Action:       e.Action,
		TargetUserID: e.TargetUserID,
		Metadata:     e.Metadata,
		Status:       string(e.Status),
		ErrorMessage: e.ErrorMessage,
		CreatedAt:    e.CreatedAt,
	}
}

// handleListGroupLogs GET /api/groups/{id}/logs.
func (s *Server) handleListGroupLogs(w http.ResponseWriter, r *http.Request) {
	if s.logStore == nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "modulo de logs no habilitado")
		return
	}
	groupID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	list, err := s.logStore.ListByGroup(r.Context(), groupID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "no se pudieron listar los logs")
		return
	}
	out := make([]logEntryResponse, 0, len(list))
	for i := range list {
		out = append(out, toLogEntryResponse(&list[i]))
	}
	respond(w, http.StatusOK, out)
}
