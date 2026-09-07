package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/telegram-manager/backend/internal/moderation"
	"github.com/telegram-manager/backend/internal/telegram"
)

// moderationActions es la vista minima del servicio de moderacion que
// los handlers necesitan (lado consumidor).
type moderationActions interface {
	Ban(ctx context.Context, actorID, groupID, userID int64, untilDate int64, revokeMessages bool) error
	Unban(ctx context.Context, actorID, groupID, userID int64) error
	Mute(ctx context.Context, actorID, groupID, userID int64, untilDate int64) error
	Unmute(ctx context.Context, actorID, groupID, userID int64) error
	DeleteMessage(ctx context.Context, actorID, groupID, messageID int64) error
	PinMessage(ctx context.Context, actorID, groupID, messageID int64) error
	Lock(ctx context.Context, actorID, groupID int64) error
	Unlock(ctx context.Context, actorID, groupID int64) error
	Approve(ctx context.Context, actorID, groupID, requestID int64) error
	Reject(ctx context.Context, actorID, groupID, requestID int64) error
}

// banRequest es el body opcional de POST .../ban: si no llega body, el
// baneo es indefinido con revocacion de mensajes (default Telegram).
type banRequest struct {
	UntilDate      int64 `json:"until_date"`
	RevokeMessages *bool `json:"revoke_messages"`
}

// muteRequest es el body opcional de POST .../mute: sin body, mute
// indefinido.
type muteRequest struct {
	UntilDate int64 `json:"until_date"`
}

// handleBan POST /api/groups/{id}/users/{userId}/ban.
func (s *Server) handleBan(w http.ResponseWriter, r *http.Request) {
	groupID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	userID, ok := pathID(w, r, "userId")
	if !ok {
		return
	}
	actorID, ok := actorIDFromClaims(w, r)
	if !ok {
		return
	}

	var req banRequest
	if !decodeOptionalBody(r, w, &req) {
		return
	}
	revoke := true
	if req.RevokeMessages != nil {
		revoke = *req.RevokeMessages
	}

	err := s.moderation.Ban(r.Context(), actorID, groupID, userID, req.UntilDate, revoke)
	if !respondModerationError(w, err) {
		respond(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

// handleUnban POST /api/groups/{id}/users/{userId}/unban.
func (s *Server) handleUnban(w http.ResponseWriter, r *http.Request) {
	groupID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	userID, ok := pathID(w, r, "userId")
	if !ok {
		return
	}
	actorID, ok := actorIDFromClaims(w, r)
	if !ok {
		return
	}

	err := s.moderation.Unban(r.Context(), actorID, groupID, userID)
	if !respondModerationError(w, err) {
		respond(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

// handleMute POST /api/groups/{id}/users/{userId}/mute.
func (s *Server) handleMute(w http.ResponseWriter, r *http.Request) {
	groupID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	userID, ok := pathID(w, r, "userId")
	if !ok {
		return
	}
	actorID, ok := actorIDFromClaims(w, r)
	if !ok {
		return
	}

	var req muteRequest
	if !decodeOptionalBody(r, w, &req) {
		return
	}

	err := s.moderation.Mute(r.Context(), actorID, groupID, userID, req.UntilDate)
	if !respondModerationError(w, err) {
		respond(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

// handleUnmute POST /api/groups/{id}/users/{userId}/unmute.
func (s *Server) handleUnmute(w http.ResponseWriter, r *http.Request) {
	groupID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	userID, ok := pathID(w, r, "userId")
	if !ok {
		return
	}
	actorID, ok := actorIDFromClaims(w, r)
	if !ok {
		return
	}

	err := s.moderation.Unmute(r.Context(), actorID, groupID, userID)
	if !respondModerationError(w, err) {
		respond(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

// handleDeleteMessage POST /api/groups/{id}/messages/{messageId}/delete.
func (s *Server) handleDeleteMessage(w http.ResponseWriter, r *http.Request) {
	groupID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	messageID, ok := pathID(w, r, "messageId")
	if !ok {
		return
	}
	actorID, ok := actorIDFromClaims(w, r)
	if !ok {
		return
	}

	err := s.moderation.DeleteMessage(r.Context(), actorID, groupID, messageID)
	if !respondModerationError(w, err) {
		respond(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

// handlePinMessage POST /api/groups/{id}/messages/{messageId}/pin.
func (s *Server) handlePinMessage(w http.ResponseWriter, r *http.Request) {
	groupID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	messageID, ok := pathID(w, r, "messageId")
	if !ok {
		return
	}
	actorID, ok := actorIDFromClaims(w, r)
	if !ok {
		return
	}

	err := s.moderation.PinMessage(r.Context(), actorID, groupID, messageID)
	if !respondModerationError(w, err) {
		respond(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

// handleLock POST /api/groups/{id}/lock.
func (s *Server) handleLock(w http.ResponseWriter, r *http.Request) {
	groupID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	actorID, ok := actorIDFromClaims(w, r)
	if !ok {
		return
	}

	err := s.moderation.Lock(r.Context(), actorID, groupID)
	if !respondModerationError(w, err) {
		respond(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

// handleUnlock POST /api/groups/{id}/unlock.
func (s *Server) handleUnlock(w http.ResponseWriter, r *http.Request) {
	groupID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	actorID, ok := actorIDFromClaims(w, r)
	if !ok {
		return
	}

	err := s.moderation.Unlock(r.Context(), actorID, groupID)
	if !respondModerationError(w, err) {
		respond(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

// actorIDFromClaims extrae el id del admin autenticado del context. Es
// el actor_id de logs y de decisiones de join requests (AGENTS.md 11).
// El id viene en el Subject del access token, inyectado por requireAuth.
func actorIDFromClaims(w http.ResponseWriter, r *http.Request) (int64, bool) {
	claims := claimsFromContext(r.Context())
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "no autenticado")
		return 0, false
	}
	id, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "identidad invalida")
		return 0, false
	}
	return id, true
}

// decodeOptionalBody decodifica un body JSON que puede venir vacio
// (acciones moderacion sin parametros: defaults). Responde
// VALIDATION_ERROR y devuelve false si el body no es JSON valido.
func decodeOptionalBody(r *http.Request, w http.ResponseWriter, dst any) bool {
	if r.Body == nil {
		return true
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "body invalido")
		return false
	}
	if len(body) == 0 {
		return true
	}
	if err := json.Unmarshal(body, dst); err != nil {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "body invalido")
		return false
	}
	return true
}

// respondModerationError mapea los errores de dominio de moderation a
// los codigos de la seccion 18 del spec. Devuelve true si ya respondio
// un error; false si la accion fue exitosa y el handler responde OK.
func respondModerationError(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, moderation.ErrGroupNotFound):
		respondError(w, http.StatusNotFound, "NOT_FOUND", "grupo no encontrado")
	case errors.Is(err, moderation.ErrBotPermission):
		respondError(w, http.StatusForbidden, "PERMISSION_DENIED",
			"el bot no tiene permisos suficientes en este grupo")
	case errors.Is(err, moderation.ErrRequestNotFound):
		respondError(w, http.StatusNotFound, "NOT_FOUND", "solicitud de ingreso no encontrada")
	case errors.Is(err, moderation.ErrRequestAlreadyDecided):
		respondError(w, http.StatusConflict, "VALIDATION_ERROR", "la solicitud ya fue decidida")
	case errors.Is(err, telegram.ErrPermissionDenied):
		respondError(w, http.StatusForbidden, "PERMISSION_DENIED",
			"el bot no tiene permisos suficientes en este grupo")
	case errors.Is(err, telegram.ErrTelegramNotFound):
		respondError(w, http.StatusNotFound, "NOT_FOUND", "el recurso no existe en Telegram")
	case errors.Is(err, telegram.ErrTelegramUnavailable), errors.Is(err, telegram.ErrWebhookConflict):
		respondError(w, http.StatusBadGateway, "TELEGRAM_ERROR", "Telegram no respondio correctamente")
	case isTelegramAPIError(err):
		// 400 de validacion y codigos inesperados de la Bot API: es un
		// fallo de la llamada a Telegram (spec §18 TELEGRAM_ERROR), no
		// un error interno nuestro.
		respondError(w, http.StatusBadGateway, "TELEGRAM_ERROR", "Telegram rechazo la accion")
	default:
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "no se pudo ejecutar la accion")
	}
	return true
}

// isTelegramAPIError detecta *telegram.TelegramAPIError vía errors.As
// (es un tipo, no un sentinel, así que errors.Is no lo alcanza).
func isTelegramAPIError(err error) bool {
	var apiErr *telegram.TelegramAPIError
	return errors.As(err, &apiErr)
}
