// Handlers REST del modulo de moderacion automatica (Fase 3, slice 2).
// Rutas (todas requireAuth — panel admin):
//
//	GET    /api/groups/{id}/automation/settings
//	PUT    /api/groups/{id}/automation/settings
//	GET    /api/groups/{id}/automation/banned-words
//	POST   /api/groups/{id}/automation/banned-words          body {word}
//	DELETE /api/groups/{id}/automation/banned-words/{word}
//	GET    /api/groups/{id}/automation/link-allowlist
//	POST   /api/groups/{id}/automation/link-allowlist        body {domain}
//	DELETE /api/groups/{id}/automation/link-allowlist/{domain}
//
// Cada cambio de settings o listas emite un log con ActorID del admin
// del panel (distinto del patron slice 1 donde ActorID=nil marcaba
// auto-actions). Las constantes viven en internal/logs (ActionUpdate…,
// ActionAdd…, ActionRemove…). Ver logs/model.go para la distincion
// manual (slice 2) vs auto (slice 1).
package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/telegram-manager/backend/internal/automation"
	"github.com/telegram-manager/backend/internal/groups"
	"github.com/telegram-manager/backend/internal/logs"
)

// automationService es la vista minima del Service de automation que
// los handlers necesitan (lado consumidor). Cubre settings + listas.
// *automation.Service la satisface.
//
// Decisión D8: el handler NO habla SQL directamente; toda operacion
// pasa por el Service para mantener la logica de defaults, errores y
// logging en un solo lugar.
type automationService interface {
	// Settings.
	GetSettings(ctx context.Context, groupID int64) (*automation.Settings, error)
	UpsertSettings(ctx context.Context, s *automation.Settings) error
	// Listas (pre-load helpers, slice 2).
	ListBannedWords(ctx context.Context, groupID int64) ([]string, error)
	AddBannedWord(ctx context.Context, groupID int64, word string) error
	RemoveBannedWord(ctx context.Context, groupID int64, word string) error
	ListLinkAllowlist(ctx context.Context, groupID int64) ([]string, error)
	AddLinkAllowlist(ctx context.Context, groupID int64, domain string) error
	RemoveLinkAllowlist(ctx context.Context, groupID int64, domain string) error
}

// automationLogWriter es la vista minima del logs.Repository que los
// handlers necesitan para registrar cambios manuales (ActorID != nil).
type automationLogWriter interface {
	Create(ctx context.Context, e *logs.Entry) error
}

// automationGroupChecker permite al handler validar que el grupo existe
// antes de aceptar cambios (404 NOT_FOUND al admin si no esta en la
// tabla groups). *groups.Repository lo satisface.
type automationGroupChecker interface {
	GetByTelegramID(ctx context.Context, id int64) (*groups.Group, error)
}

// --- Tipos JSON ---

// settingsUpdate es el body de PUT /automation/settings. El handler
// acepta cualquier subconjunto de campos y rellena defaults desde el
// settings existente (o defaults puros si la fila no existe).
type settingsUpdate struct {
	Enabled            *bool  `json:"enabled"`
	AntiSpamEnabled    *bool  `json:"anti_spam_enabled"`
	AntiLinkEnabled    *bool  `json:"anti_link_enabled"`
	BannedWordsEnabled *bool  `json:"banned_words_enabled"`
	FloodEnabled       *bool  `json:"flood_enabled"`
	FloodMessages      *int16 `json:"flood_messages"`
	FloodSeconds       *int16 `json:"flood_seconds"`
	WarningLimit       *int16 `json:"warning_limit"`
	AutomuteWarnings   *int16 `json:"automute_warnings"`
	AutomuteMinutes    *int16 `json:"automute_minutes"`
	AutobanWarnings    *int16 `json:"autoban_warnings"`
	WarningExpireDays  *int16 `json:"warning_expire_days"`
}

// settingsResponse es la vista JSON de Settings para el panel.
type settingsResponse struct {
	GroupID            int64  `json:"group_id"`
	Enabled            bool   `json:"enabled"`
	AntiSpamEnabled    bool   `json:"anti_spam_enabled"`
	AntiLinkEnabled    bool   `json:"anti_link_enabled"`
	BannedWordsEnabled bool   `json:"banned_words_enabled"`
	FloodEnabled       bool   `json:"flood_enabled"`
	FloodMessages      int16  `json:"flood_messages"`
	FloodSeconds       int16  `json:"flood_seconds"`
	WarningLimit       int16  `json:"warning_limit"`
	AutomuteWarnings   int16  `json:"automute_warnings"`
	AutomuteMinutes    int16  `json:"automute_minutes"`
	AutobanWarnings    int16  `json:"autoban_warnings"`
	WarningExpireDays  int16  `json:"warning_expire_days"`
	UpdatedAt          string `json:"updated_at"`
}

func toSettingsResponse(s *automation.Settings) settingsResponse {
	return settingsResponse{
		GroupID:            s.GroupID,
		Enabled:            s.Enabled,
		AntiSpamEnabled:    s.AntiSpamEnabled,
		AntiLinkEnabled:    s.AntiLinkEnabled,
		BannedWordsEnabled: s.BannedWordsEnabled,
		FloodEnabled:       s.FloodEnabled,
		FloodMessages:      s.FloodMessages,
		FloodSeconds:       s.FloodSeconds,
		WarningLimit:       s.WarningLimit,
		AutomuteWarnings:   s.AutomuteWarnings,
		AutomuteMinutes:    s.AutomuteMinutes,
		AutobanWarnings:    s.AutobanWarnings,
		WarningExpireDays:  s.WarningExpireDays,
		UpdatedAt:          s.UpdatedAt.UTC().Format("2006-01-02T15:04:05.000Z"),
	}
}

// bannedWordRequest es el body de POST /banned-words.
type bannedWordRequest struct {
	Word string `json:"word"`
}

// allowlistRequest es el body de POST /link-allowlist.
type allowlistRequest struct {
	Domain string `json:"domain"`
}

// --- Handlers ---

// handleGetAutomationSettings responde GET /api/groups/{id}/automation/settings.
// Si no existe fila, crea defaults via LoadOrCreateSettings (politica
// consistente con slice 1). Devuelve 200 con la fila efectiva.
func (s *Server) handleGetAutomationSettings(w http.ResponseWriter, r *http.Request) {
	if s.automation == nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "modulo de automation no habilitado")
		return
	}
	groupID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if s.automationGroups != nil {
		if _, err := s.automationGroups.GetByTelegramID(r.Context(), groupID); err != nil {
			if errors.Is(err, groups.ErrNotFound) {
				respondError(w, http.StatusNotFound, "NOT_FOUND", "grupo no encontrado")
				return
			}
			respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "no se pudo obtener el grupo")
			return
		}
	}

	settings, err := s.automation.GetSettings(r.Context(), groupID)
	if err != nil && !errors.Is(err, automation.ErrNotFound) {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "no se pudieron obtener los settings")
		return
	}
	// Si no existia la fila, devolvemos defaults (no la persistimos
	// en GET — solo PUT lo hace). El frontend vera toggles en false
	// y thresholds en cero, pero al primer PUT se crea la fila.
	if errors.Is(err, automation.ErrNotFound) {
		settings = automation.DefaultSettings(groupID)
	}
	respond(w, http.StatusOK, toSettingsResponse(settings))
}

// handlePutAutomationSettings responde PUT /api/groups/{id}/automation/settings.
// Acepta body parcial (solo los campos a modificar) o completo; lo
// mezcla con el settings existente y persiste via UPSERT. Loguea
// UPDATE_AUTOMATION_SETTINGS con el actor del panel.
func (s *Server) handlePutAutomationSettings(w http.ResponseWriter, r *http.Request) {
	if s.automation == nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "modulo de automation no habilitado")
		return
	}
	groupID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	actorID, ok := actorIDFromClaims(w, r)
	if !ok {
		return
	}
	if s.automationGroups != nil {
		if _, err := s.automationGroups.GetByTelegramID(r.Context(), groupID); err != nil {
			if errors.Is(err, groups.ErrNotFound) {
				respondError(w, http.StatusNotFound, "NOT_FOUND", "grupo no encontrado")
				return
			}
			respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "no se pudo obtener el grupo")
			return
		}
	}

	var req settingsUpdate
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "body invalido")
		return
	}

	// Mezclar con defaults/fila existente: arrancar de defaults si no
	// hay fila, luego pisar con body. Si la fila existe, arrancar de
	// ahi. Asi el body parcial funciona correctamente.
	existing, err := s.automation.GetSettings(r.Context(), groupID)
	settings := automation.DefaultSettings(groupID)
	if err == nil {
		settings = existing
	} else if !errors.Is(err, automation.ErrNotFound) {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "no se pudieron obtener los settings previos")
		return
	}

	if req.Enabled != nil {
		settings.Enabled = *req.Enabled
	}
	if req.AntiSpamEnabled != nil {
		settings.AntiSpamEnabled = *req.AntiSpamEnabled
	}
	if req.AntiLinkEnabled != nil {
		settings.AntiLinkEnabled = *req.AntiLinkEnabled
	}
	if req.BannedWordsEnabled != nil {
		settings.BannedWordsEnabled = *req.BannedWordsEnabled
	}
	if req.FloodEnabled != nil {
		settings.FloodEnabled = *req.FloodEnabled
	}
	if req.FloodMessages != nil {
		settings.FloodMessages = *req.FloodMessages
	}
	if req.FloodSeconds != nil {
		settings.FloodSeconds = *req.FloodSeconds
	}
	if req.WarningLimit != nil {
		settings.WarningLimit = *req.WarningLimit
	}
	if req.AutomuteWarnings != nil {
		settings.AutomuteWarnings = *req.AutomuteWarnings
	}
	if req.AutomuteMinutes != nil {
		settings.AutomuteMinutes = *req.AutomuteMinutes
	}
	if req.AutobanWarnings != nil {
		settings.AutobanWarnings = *req.AutobanWarnings
	}
	if req.WarningExpireDays != nil {
		settings.WarningExpireDays = *req.WarningExpireDays
	}

	if err := s.automation.UpsertSettings(r.Context(), settings); err != nil {
		respondAutomationError(w, err)
		return
	}

	// Log UPDATE_AUTOMATION_SETTINGS con ActorID del admin.
	if s.automationLogs != nil {
		entry := &logs.Entry{
			ActorID: &actorID,
			GroupID: groupID,
			Action:  logs.ActionUpdateAutomationSettings,
			Status:  logs.StatusSuccess,
			Metadata: map[string]any{
				"enabled":              settings.Enabled,
				"anti_spam_enabled":    settings.AntiSpamEnabled,
				"anti_link_enabled":    settings.AntiLinkEnabled,
				"banned_words_enabled": settings.BannedWordsEnabled,
				"flood_enabled":        settings.FloodEnabled,
			},
		}
		_ = s.automationLogs.Create(r.Context(), entry) // best-effort
	}

	// Releer para devolver la fila actualizada con updated_at fresco.
	updated, _ := s.automation.GetSettings(r.Context(), groupID)
	if updated == nil {
		updated = settings
	}
	respond(w, http.StatusOK, toSettingsResponse(updated))
}

// handleListBannedWords responde GET /api/groups/{id}/automation/banned-words.
func (s *Server) handleListBannedWords(w http.ResponseWriter, r *http.Request) {
	if s.automation == nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "modulo de automation no habilitado")
		return
	}
	groupID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	list, err := s.automation.ListBannedWords(r.Context(), groupID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "no se pudieron listar las palabras prohibidas")
		return
	}
	respond(w, http.StatusOK, map[string]any{"words": list})
}

// handleAddBannedWord responde POST /api/groups/{id}/automation/banned-words.
// Body {word}. Loguea ADD_BANNED_WORD con ActorID del admin.
func (s *Server) handleAddBannedWord(w http.ResponseWriter, r *http.Request) {
	if s.automation == nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "modulo de automation no habilitado")
		return
	}
	groupID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	actorID, ok := actorIDFromClaims(w, r)
	if !ok {
		return
	}
	var req bannedWordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "body invalido")
		return
	}
	word := strings.ToLower(strings.TrimSpace(req.Word))
	if !validateBannedWord(word) {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR",
			"palabra invalida (1-100 caracteres, letras/digitos/espacios/guion/underscore)")
		return
	}
	if err := s.automation.AddBannedWord(r.Context(), groupID, word); err != nil {
		respondAutomationError(w, err)
		return
	}
	if s.automationLogs != nil {
		entry := &logs.Entry{
			ActorID: &actorID,
			GroupID: groupID,
			Action:  logs.ActionAddBannedWord,
			Status:  logs.StatusSuccess,
			Metadata: map[string]any{
				"word": word,
			},
		}
		_ = s.automationLogs.Create(r.Context(), entry)
	}
	list, _ := s.automation.ListBannedWords(r.Context(), groupID)
	respond(w, http.StatusOK, map[string]any{"words": list})
}

// handleRemoveBannedWord responde DELETE /api/groups/{id}/automation/banned-words/{word}.
func (s *Server) handleRemoveBannedWord(w http.ResponseWriter, r *http.Request) {
	if s.automation == nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "modulo de automation no habilitado")
		return
	}
	groupID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	actorID, ok := actorIDFromClaims(w, r)
	if !ok {
		return
	}
	rawWord := r.PathValue("word")
	word := strings.ToLower(strings.TrimSpace(rawWord))
	if word == "" {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "word vacio")
		return
	}
	if err := s.automation.RemoveBannedWord(r.Context(), groupID, word); err != nil {
		respondAutomationError(w, err)
		return
	}
	if s.automationLogs != nil {
		entry := &logs.Entry{
			ActorID: &actorID,
			GroupID: groupID,
			Action:  logs.ActionRemoveBannedWord,
			Status:  logs.StatusSuccess,
			Metadata: map[string]any{
				"word": word,
			},
		}
		_ = s.automationLogs.Create(r.Context(), entry)
	}
	list, _ := s.automation.ListBannedWords(r.Context(), groupID)
	respond(w, http.StatusOK, map[string]any{"words": list})
}

// handleListLinkAllowlist responde GET /api/groups/{id}/automation/link-allowlist.
func (s *Server) handleListLinkAllowlist(w http.ResponseWriter, r *http.Request) {
	if s.automation == nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "modulo de automation no habilitado")
		return
	}
	groupID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	list, err := s.automation.ListLinkAllowlist(r.Context(), groupID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "no se pudieron listar los dominios permitidos")
		return
	}
	respond(w, http.StatusOK, map[string]any{"domains": list})
}

// handleAddLinkAllowlist responde POST /api/groups/{id}/automation/link-allowlist.
func (s *Server) handleAddLinkAllowlist(w http.ResponseWriter, r *http.Request) {
	if s.automation == nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "modulo de automation no habilitado")
		return
	}
	groupID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	actorID, ok := actorIDFromClaims(w, r)
	if !ok {
		return
	}
	var req allowlistRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "body invalido")
		return
	}
	domain := strings.TrimSpace(req.Domain)
	if !validateDomain(domain) {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR",
			"dominio invalido (1-253 caracteres, formato basico)")
		return
	}
	if err := s.automation.AddLinkAllowlist(r.Context(), groupID, domain); err != nil {
		respondAutomationError(w, err)
		return
	}
	if s.automationLogs != nil {
		entry := &logs.Entry{
			ActorID: &actorID,
			GroupID: groupID,
			Action:  logs.ActionAddLinkAllowlist,
			Status:  logs.StatusSuccess,
			Metadata: map[string]any{
				"domain": domain,
			},
		}
		_ = s.automationLogs.Create(r.Context(), entry)
	}
	list, _ := s.automation.ListLinkAllowlist(r.Context(), groupID)
	respond(w, http.StatusOK, map[string]any{"domains": list})
}

// handleRemoveLinkAllowlist responde DELETE /api/groups/{id}/automation/link-allowlist/{domain}.
func (s *Server) handleRemoveLinkAllowlist(w http.ResponseWriter, r *http.Request) {
	if s.automation == nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "modulo de automation no habilitado")
		return
	}
	groupID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	actorID, ok := actorIDFromClaims(w, r)
	if !ok {
		return
	}
	rawDomain := r.PathValue("domain")
	domain := strings.TrimSpace(rawDomain)
	if domain == "" {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "domain vacio")
		return
	}
	if err := s.automation.RemoveLinkAllowlist(r.Context(), groupID, domain); err != nil {
		respondAutomationError(w, err)
		return
	}
	if s.automationLogs != nil {
		entry := &logs.Entry{
			ActorID: &actorID,
			GroupID: groupID,
			Action:  logs.ActionRemoveLinkAllowlist,
			Status:  logs.StatusSuccess,
			Metadata: map[string]any{
				"domain": domain,
			},
		}
		_ = s.automationLogs.Create(r.Context(), entry)
	}
	list, _ := s.automation.ListLinkAllowlist(r.Context(), groupID)
	respond(w, http.StatusOK, map[string]any{"domains": list})
}

// --- Helpers ---

// validateBannedWord: 1-100 chars, letras/digitos/espacio/guion/
// underscore. Misma regex que el cliente para que el error message
// coincida.
func validateBannedWord(word string) bool {
	if word == "" || len(word) > 100 {
		return false
	}
	for _, r := range word {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == ' ' || r == '-' || r == '_':
		default:
			return false
		}
	}
	return true
}

// validateDomain: 1-253 chars, formato basico (label.label…). El
// helper de matcher (domainMatches) lowercases y exige "." antes del
// allow domain. El handler es defensivo: si el dominio es vacio o
// excede 253 chars, rechaza.
func validateDomain(domain string) bool {
	if domain == "" || len(domain) > 253 {
		return false
	}
	// Sin espacios ni caracteres de control.
	for _, r := range domain {
		if r <= ' ' {
			return false
		}
	}
	return true
}

// respondAutomationError mapea errores de automation a §18. Hoy la
// mayoria de errores son del repo (Postgres) → 500. ErrNotFound se
// mapea a 404 (caso donde la fila no existia pero el caller esperaba
// una fila; en GET settings lo manejamos inline devolviendo defaults).
func respondAutomationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, automation.ErrNotFound):
		respondError(w, http.StatusNotFound, "NOT_FOUND", "recurso no encontrado")
	case errors.Is(err, automation.ErrAutomationGroupNotFound):
		respondError(w, http.StatusNotFound, "NOT_FOUND", "grupo no encontrado")
	default:
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "no se pudo completar la operacion")
	}
}
