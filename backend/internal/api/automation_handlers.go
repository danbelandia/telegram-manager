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
// Slice 3 (Warnings Dashboard) agrega:
//
//	GET    /api/groups/{id}/automation/warnings
//	POST   /api/groups/{id}/automation/warnings/{user_id}/reset
//	GET    /api/groups/{id}/automation/stats?period=24h|7d
//
// Cada cambio de settings o listas emite un log con ActorID del admin
// del panel (distinto del patron slice 1 donde ActorID=nil marcaba
// auto-actions). Las constantes viven en internal/logs (ActionUpdate…,
// ActionAdd…, ActionRemove…, ActionResetWarnings). Ver logs/model.go
// para la distincion manual (slices 2/2.1/3) vs auto (slice 1).
package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

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
//
// Slice 3: los endpoints del dashboard (/warnings, /warnings/:id/reset)
// NO pasan por el Service para mantener la arquitectura de slices 1+2+2.1
// intacta (bugfix invariante #172 — service.go no se toca). El handler
// consume `automationDashboardRepo` directamente (inyectado via
// WithAutomation). Esto evita modificar Service/Service.go y el
// pipeline de evaluation.
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

// automationDashboardRepo es la vista minima del repositorio que los
// handlers del dashboard de slice 3 necesitan. NO pasa por Service
// (intencional: service.go intacto, pipeline de slices 1+2+2.1
// preservado). *automation.Repository lo satisface directamente.
// Scopeado por tenant (slice 0).
type automationDashboardRepo interface {
	ListActiveWarningStatesByGroup(ctx context.Context, tenantID, groupID int64, limit int) ([]automation.WarningStateRow, bool, error)
	ResetWarningState(ctx context.Context, tenantID, groupID, userID int64) (int64, error)
}

// automationLogWriter es la vista minima del logs.Repository que los
// handlers necesitan para registrar cambios manuales (ActorID != nil)
// y para agregar stats de auto-actions en una ventana (slice 3). El
// metodo extra CountByActionAndGroup cubre el handler GET .../stats.
// Scopeado por tenant (slice 0).
type automationLogWriter interface {
	Create(ctx context.Context, e *logs.Entry) error
	CountByActionAndGroup(ctx context.Context, tenantID, groupID int64, actions []string, since time.Time) (map[string]int, error)
}

// automationGroupChecker permite al handler validar que el grupo existe
// y pertenece al tenant antes de aceptar cambios (404 NOT_FOUND al
// admin si no esta en la tabla groups o es de otro tenant, D9).
// *groups.Repository lo satisface.
type automationGroupChecker interface {
	GetByTenant(ctx context.Context, tenantID, id int64) (*groups.Group, error)
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
	// Slice 2.1 (REQ-22, REQ-29): warning visual al usuario antes de
	// mute/ban. WarnUserTemplate opcional (nil = default hardcoded).
	// Si != nil, validamos len <= 1000 chars en el handler.
	WarnUserEnabled  *bool   `json:"warn_user_enabled"`
	WarnUserTemplate *string `json:"warn_user_template"`
}

// settingsResponse es la vista JSON de Settings para el panel.
type settingsResponse struct {
	GroupID            int64   `json:"group_id"`
	Enabled            bool    `json:"enabled"`
	AntiSpamEnabled    bool    `json:"anti_spam_enabled"`
	AntiLinkEnabled    bool    `json:"anti_link_enabled"`
	BannedWordsEnabled bool    `json:"banned_words_enabled"`
	FloodEnabled       bool    `json:"flood_enabled"`
	FloodMessages      int16   `json:"flood_messages"`
	FloodSeconds       int16   `json:"flood_seconds"`
	WarningLimit       int16   `json:"warning_limit"`
	AutomuteWarnings   int16   `json:"automute_warnings"`
	AutomuteMinutes    int16   `json:"automute_minutes"`
	AutobanWarnings    int16   `json:"autoban_warnings"`
	WarningExpireDays  int16   `json:"warning_expire_days"`
	WarnUserEnabled    bool    `json:"warn_user_enabled"`
	WarnUserTemplate   *string `json:"warn_user_template"`
	UpdatedAt          string  `json:"updated_at"`
}

// maxWarnUserTemplateLen es el maximo permitido para el template
// custom del warning (1000 chars). Definido en AGENTS §23 / spec
// REQ-29 mitigation #6. Validamos server-side porque el cliente puede
// tener un maxLength buggy o ser bypaseado.
const maxWarnUserTemplateLen = 1000

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
		WarnUserEnabled:    s.WarnUserEnabled,
		WarnUserTemplate:   s.WarnUserTemplate,
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
	tenantID, ok := tenantIDFromClaims(w, r)
	if !ok {
		return
	}
	auto, ok := s.resolveAutomation(tenantID)
	if !ok {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "modulo de automation no habilitado")
		return
	}
	groupID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if s.automationGroups != nil {
		if _, err := s.automationGroups.GetByTenant(r.Context(), tenantID, groupID); err != nil {
			if errors.Is(err, groups.ErrNotFound) {
				respondError(w, http.StatusNotFound, "NOT_FOUND", "grupo no encontrado")
				return
			}
			respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "no se pudo obtener el grupo")
			return
		}
	}

	settings, err := 	auto.GetSettings(r.Context(), groupID)
	if err != nil && !errors.Is(err, automation.ErrNotFound) {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "no se pudieron obtener los settings")
		return
	}
	// Si no existia la fila, devolvemos defaults (no la persistimos
	// en GET — solo PUT lo hace). El frontend vera toggles en false
	// y thresholds en cero, pero al primer PUT se crea la fila.
	if errors.Is(err, automation.ErrNotFound) {
		settings = automation.DefaultSettings(tenantID, groupID)
	}
	respond(w, http.StatusOK, toSettingsResponse(settings))
}

// handlePutAutomationSettings responde PUT /api/groups/{id}/automation/settings.
// Acepta body parcial (solo los campos a modificar) o completo; lo
// mezcla con el settings existente y persiste via UPSERT. Loguea
// UPDATE_AUTOMATION_SETTINGS con el actor del panel.
func (s *Server) handlePutAutomationSettings(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantIDFromClaims(w, r)
	if !ok {
		return
	}
	auto, ok := s.resolveAutomation(tenantID)
	if !ok {
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
		if _, err := s.automationGroups.GetByTenant(r.Context(), tenantID, groupID); err != nil {
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
	existing, err := 	auto.GetSettings(r.Context(), groupID)
	settings := automation.DefaultSettings(tenantID, groupID)
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
	// Slice 2.1 (REQ-22, REQ-29): warning visual al usuario. Validamos
	// template <= 1000 chars (mitigacion #6 del design).
	if req.WarnUserEnabled != nil {
		settings.WarnUserEnabled = *req.WarnUserEnabled
	}
	if req.WarnUserTemplate != nil {
		if len(*req.WarnUserTemplate) > maxWarnUserTemplateLen {
			respondError(w, http.StatusBadRequest, "VALIDATION_ERROR",
				"warn_user_template excede el maximo de 1000 caracteres")
			return
		}
		settings.WarnUserTemplate = req.WarnUserTemplate
	}

	if err := 	auto.UpsertSettings(r.Context(), settings); err != nil {
		respondAutomationError(w, err)
		return
	}

	// Log UPDATE_AUTOMATION_SETTINGS con ActorID del admin.
	if s.automationLogs != nil {
		entry := &logs.Entry{
			TenantID: tenantID,
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
	updated, _ := 	auto.GetSettings(r.Context(), groupID)
	if updated == nil {
		updated = settings
	}
	respond(w, http.StatusOK, toSettingsResponse(updated))
}

// handleListBannedWords responde GET /api/groups/{id}/automation/banned-words.
func (s *Server) handleListBannedWords(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantIDFromClaims(w, r)
	if !ok {
		return
	}
	auto, ok := s.resolveAutomation(tenantID)
	if !ok {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "modulo de automation no habilitado")
		return
	}
	groupID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	list, err := 	auto.ListBannedWords(r.Context(), groupID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "no se pudieron listar las palabras prohibidas")
		return
	}
	respond(w, http.StatusOK, map[string]any{"words": list})
}

// handleAddBannedWord responde POST /api/groups/{id}/automation/banned-words.
// Body {word}. Loguea ADD_BANNED_WORD con ActorID del admin.
func (s *Server) handleAddBannedWord(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantIDFromClaims(w, r)
	if !ok {
		return
	}
	auto, ok := s.resolveAutomation(tenantID)
	if !ok {
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
	if err := 	auto.AddBannedWord(r.Context(), groupID, word); err != nil {
		respondAutomationError(w, err)
		return
	}
	if s.automationLogs != nil {
		entry := &logs.Entry{
			TenantID: tenantID,
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
	list, _ := 	auto.ListBannedWords(r.Context(), groupID)
	respond(w, http.StatusOK, map[string]any{"words": list})
}

// handleRemoveBannedWord responde DELETE /api/groups/{id}/automation/banned-words/{word}.
func (s *Server) handleRemoveBannedWord(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantIDFromClaims(w, r)
	if !ok {
		return
	}
	auto, ok := s.resolveAutomation(tenantID)
	if !ok {
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
	if err := 	auto.RemoveBannedWord(r.Context(), groupID, word); err != nil {
		respondAutomationError(w, err)
		return
	}
	if s.automationLogs != nil {
		entry := &logs.Entry{
			TenantID: tenantID,
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
	list, _ := 	auto.ListBannedWords(r.Context(), groupID)
	respond(w, http.StatusOK, map[string]any{"words": list})
}

// handleListLinkAllowlist responde GET /api/groups/{id}/automation/link-allowlist.
func (s *Server) handleListLinkAllowlist(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantIDFromClaims(w, r)
	if !ok {
		return
	}
	auto, ok := s.resolveAutomation(tenantID)
	if !ok {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "modulo de automation no habilitado")
		return
	}
	groupID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	list, err := 	auto.ListLinkAllowlist(r.Context(), groupID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "no se pudieron listar los dominios permitidos")
		return
	}
	respond(w, http.StatusOK, map[string]any{"domains": list})
}

// handleAddLinkAllowlist responde POST /api/groups/{id}/automation/link-allowlist.
func (s *Server) handleAddLinkAllowlist(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantIDFromClaims(w, r)
	if !ok {
		return
	}
	auto, ok := s.resolveAutomation(tenantID)
	if !ok {
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
	if err := 	auto.AddLinkAllowlist(r.Context(), groupID, domain); err != nil {
		respondAutomationError(w, err)
		return
	}
	if s.automationLogs != nil {
		entry := &logs.Entry{
			TenantID: tenantID,
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
	list, _ := 	auto.ListLinkAllowlist(r.Context(), groupID)
	respond(w, http.StatusOK, map[string]any{"domains": list})
}

// handleRemoveLinkAllowlist responde DELETE /api/groups/{id}/automation/link-allowlist/{domain}.
func (s *Server) handleRemoveLinkAllowlist(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantIDFromClaims(w, r)
	if !ok {
		return
	}
	auto, ok := s.resolveAutomation(tenantID)
	if !ok {
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
	if err := 	auto.RemoveLinkAllowlist(r.Context(), groupID, domain); err != nil {
		respondAutomationError(w, err)
		return
	}
	if s.automationLogs != nil {
		entry := &logs.Entry{
			TenantID: tenantID,
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
	list, _ := 	auto.ListLinkAllowlist(r.Context(), groupID)
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

// --- Handlers de slice 3: Warnings Dashboard ---

// warningStateJSON es la vista JSON de WarningStateRow para el panel.
// Incluye display_name ya calculado server-side via
// WarningStateRow.DisplayName() (D14 del design) — el frontend recibe
// un string listo y no replica la logica de fallback.
type warningStateJSON struct {
	UserID        int64   `json:"user_id"`
	DisplayName   string  `json:"display_name"`
	Username      *string `json:"username"`
	WarningCount  int16   `json:"warning_count"`
	LastWarningAt *string `json:"last_warning_at"`
	LastActionAt  *string `json:"last_action_at"`
	ExpiresAt     *string `json:"expires_at"`
}

func toWarningStateJSON(w automation.WarningStateRow) warningStateJSON {
	formatTS := func(t *time.Time) *string {
		if t == nil {
			return nil
		}
		s := t.UTC().Format("2006-01-02T15:04:05.000Z")
		return &s
	}
	return warningStateJSON{
		UserID:        w.UserID,
		DisplayName:   w.DisplayName(),
		Username:      w.Username,
		WarningCount:  w.WarningCount,
		LastWarningAt: formatTS(w.LastWarningAt),
		LastActionAt:  formatTS(w.LastActionAt),
		ExpiresAt:     formatTS(w.ExpiresAt),
	}
}

// defaultWarningsLimit es el cap defensivo del dashboard (top 100
// advertencias activas). Si el admin tiene mas de 100 simultaneas hay
// un problema mas grande (reglas mal calibradas) y el frontend muestra
// un Alert amarillo en lugar de paginar.
const defaultWarningsLimit = 100

// handleListWarnings responde GET /api/groups/{id}/automation/warnings.
// Devuelve la lista de advertencias activas del grupo con display
// name (LEFT JOIN a users), cap top 100 y flag `truncated` indicando
// si el resultset alcanzo el cap.
func (s *Server) handleListWarnings(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantIDFromClaims(w, r)
	if !ok {
		return
	}
	if _, ok := s.resolveAutomation(tenantID); !ok {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "modulo de automation no habilitado")
		return
	}
	groupID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if s.automationGroups != nil {
		if _, err := s.automationGroups.GetByTenant(r.Context(), tenantID, groupID); err != nil {
			if errors.Is(err, groups.ErrNotFound) {
				respondError(w, http.StatusNotFound, "NOT_FOUND", "grupo no encontrado")
				return
			}
			respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "no se pudo obtener el grupo")
			return
		}
	}

	rows, truncated, err := s.automationDashboard.ListActiveWarningStatesByGroup(r.Context(), tenantID, groupID, defaultWarningsLimit)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "no se pudieron listar las advertencias")
		return
	}
	warnings := make([]warningStateJSON, 0, len(rows))
	for _, row := range rows {
		warnings = append(warnings, toWarningStateJSON(row))
	}
	respond(w, http.StatusOK, map[string]any{
		"warnings":  warnings,
		"truncated": truncated,
	})
}

// resetWarningResponse es el body de POST .../warnings/{user_id}/reset.
type resetWarningResponse struct {
	UserID       int64 `json:"user_id"`
	WarningCount int16 `json:"warning_count"`
	Reset        bool  `json:"reset"`
}

// handleResetWarning responde POST /api/groups/{id}/automation/warnings/{user_id}/reset.
// Resetea manualmente el counter de advertencias del (group, user).
// Si la fila no existia, responde 404 NOT_FOUND sin emitir log (no hay
// "estado" que resetear). Si existia, emite ActionResetWarnings con
// ActorID del admin y metadata {user_id, warning_count_before_reset}
// para auditoria (cuanto se perdono).
//
// Slice 3 intencional: NO desmutear al user en Telegram (D11 del
// design). El admin usa POST /api/groups/{id}/users/{userId}/unmute
// por separado si quiere desmutear. El reset solo limpia DB.
func (s *Server) handleResetWarning(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantIDFromClaims(w, r)
	if !ok {
		return
	}
	if _, ok := s.resolveAutomation(tenantID); !ok {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "modulo de automation no habilitado")
		return
	}
	groupID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	userID, ok := pathID(w, r, "user_id")
	if !ok {
		return
	}
	if userID <= 0 {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "user_id debe ser positivo")
		return
	}
	actorID, ok := actorIDFromClaims(w, r)
	if !ok {
		return
	}
	if s.automationGroups != nil {
		if _, err := s.automationGroups.GetByTenant(r.Context(), tenantID, groupID); err != nil {
			if errors.Is(err, groups.ErrNotFound) {
				respondError(w, http.StatusNotFound, "NOT_FOUND", "grupo no encontrado")
				return
			}
			respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "no se pudo obtener el grupo")
			return
		}
	}

	previous, err := s.automationDashboard.ResetWarningState(r.Context(), tenantID, groupID, userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "no se pudo resetear el contador")
		return
	}
	if previous == 0 {
		// Fila no existia o ya estaba en 0 → 404 sin log (no hay accion
		// que auditar).
		respondError(w, http.StatusNotFound, "NOT_FOUND",
			"no hay advertencias para este usuario en este grupo")
		return
	}

	// Log RESET_WARNINGS con metadata de auditoria. ActorID = admin
	// del panel (distinto del patron slice 1 donde ActorID=nil marcaba
	// auto-actions). warning_count_before_reset = previous (leido
	// antes del UPDATE en el repo).
	if s.automationLogs != nil {
		entry := &logs.Entry{
			ActorID:      &actorID,
			GroupID:      groupID,
			Action:       logs.ActionResetWarnings,
			TargetUserID: &userID,
			Status:       logs.StatusSuccess,
			Metadata: map[string]any{
				"user_id":                    userID,
				"warning_count_before_reset": previous,
			},
		}
		_ = s.automationLogs.Create(r.Context(), entry) // best-effort
	}

	respond(w, http.StatusOK, resetWarningResponse{
		UserID:       userID,
		WarningCount: 0,
		Reset:        true,
	})
}

// statsResponse es el body de GET .../stats?period=24h|7d.
type statsResponse struct {
	RuleTriggered int    `json:"rule_triggered"`
	Automute      int    `json:"automute"`
	Autoban       int    `json:"autoban"`
	Period        string `json:"period"`
}

// statsPeriods whitelistada para evitar injection o typos del admin.
// Default "24h" si el query param falta o esta vacio. Cualquier otro
// valor → 400 VALIDATION_ERROR (slice 3 REQ-34).
var statsPeriods = map[string]time.Duration{
	"24h": 24 * time.Hour,
	"7d":  7 * 24 * time.Hour,
}

// handleGetStats responde GET /api/groups/{id}/automation/stats?period=24h|7d.
// Devuelve conteos agregados de los 3 actions de auto-moderacion en la
// ventana temporal indicada (1 roundtrip via ANY($2)). Si no hay logs
// en la ventana, los contadores quedan en 0 (no error).
func (s *Server) handleGetStats(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantIDFromClaims(w, r)
	if !ok {
		return
	}
	if _, ok := s.resolveAutomation(tenantID); !ok {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "modulo de automation no habilitado")
		return
	}
	groupID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if s.automationGroups != nil {
		if _, err := s.automationGroups.GetByTenant(r.Context(), tenantID, groupID); err != nil {
			if errors.Is(err, groups.ErrNotFound) {
				respondError(w, http.StatusNotFound, "NOT_FOUND", "grupo no encontrado")
				return
			}
			respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "no se pudo obtener el grupo")
			return
		}
	}

	period := r.URL.Query().Get("period")
	if period == "" {
		period = "24h"
	}
	duration, ok := statsPeriods[period]
	if !ok {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR",
			"period invalido (use 24h o 7d)")
		return
	}
	if s.automationLogs == nil {
		// Si los logs no estan habilitados, devolvemos 0s (consistente
		// con el resto de handlers que no fallan si automationLogs es
		// nil; sirve para tests sin logs y para escenarios donde el
		// modulo se carga sin logs).
		respond(w, http.StatusOK, statsResponse{
			RuleTriggered: 0,
			Automute:      0,
			Autoban:       0,
			Period:        period,
		})
		return
	}

	since := time.Now().UTC().Add(-duration)
	counts, err := s.automationLogs.CountByActionAndGroup(r.Context(), tenantID, groupID,
		[]string{logs.ActionRuleTriggered, logs.ActionAutomuteUser, logs.ActionAutobanUser},
		since,
	)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "no se pudieron obtener las estadisticas")
		return
	}
	respond(w, http.StatusOK, statsResponse{
		RuleTriggered: counts[logs.ActionRuleTriggered],
		Automute:      counts[logs.ActionAutomuteUser],
		Autoban:       counts[logs.ActionAutobanUser],
		Period:        period,
	})
}
