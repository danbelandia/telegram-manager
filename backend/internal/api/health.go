package api

import (
	"context"
	"net/http"
	"time"
)

// botStatusProvider es la vista minima que el health endpoint necesita
// sobre el estado del bot de Telegram. Declarada donde se consume.
type botStatusProvider interface {
	Status() (connected bool, username string)
}

// handleHealth reporta el estado del servidor, de la base de datos y
// del bot de Telegram. Nunca expone tokens ni credenciales.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	dbStatus := "ok"
	if err := s.db.PingContext(ctx); err != nil {
		dbStatus = "error"
	}

	connected, username := s.botStatus.Status()

	status := "ok"
	if dbStatus != "ok" || !connected {
		status = "degraded"
	}

	respond(w, http.StatusOK, map[string]any{
		"status":        status,
		"db":            dbStatus,
		"bot_connected": connected,
		"bot_username":  username,
	})
}
