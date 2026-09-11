// Handlers de configuracion del tenant (slice tenant-settings):
// GET /tenants/me, PUT /tenants/me/bot-token, GET /tenants/me/status.
// El token NUNCA aparece en logs ni respuestas (spec token hygiene).
package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/telegram-manager/backend/internal/telegram"
	"github.com/telegram-manager/backend/internal/tenants"
)

// rotateTokenRequest es el body de PUT /api/tenants/me/bot-token.
// password confirma la identidad; bot_token es el nuevo token en claro.
// NUNCA se loguea ni devuelve el token (spec token hygiene).
type rotateTokenRequest struct {
	Password string `json:"password"`
	BotToken string `json:"bot_token"`
}

// handleGetTenantMe devuelve los datos del tenant autenticado: slug,
// bot_username, bot_status y created_at. Nunca incluye el token.
func (s *Server) handleGetTenantMe(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantIDFromClaims(w, r)
	if !ok {
		return
	}

	t, err := s.tenantsRepo.GetByID(r.Context(), tenantID)
	if errors.Is(err, tenants.ErrNotFound) {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "tenant no encontrado")
		return
	}
	if err != nil {
		slog.Error("tenants/me: repo error", "tenant_id", tenantID, "error", err)
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "no se pudo obtener el tenant")
		return
	}

	// bot_status desde el registry (runtime), no de la DB.
	botStatus := "unknown"
	if s.tenantRegistry != nil {
		if status, ok := s.tenantRegistry.Status(tenantID); ok {
			switch status {
			case "active":
				botStatus = "connected"
			case "degraded":
				botStatus = "disconnected"
			default:
				botStatus = status
			}
		}
	}

	respond(w, http.StatusOK, map[string]any{
		"slug":              t.Slug,
		"bot_username":      t.BotUsername,
		"bot_status":        botStatus,
		"created_at":        t.CreatedAt,
		"license_status":    t.Status,
		"plan":              t.Plan,
		"trial_ends_at":     t.TrialEndsAt,
		"expires_at":        t.ExpiresAt,
		"max_groups":        t.MaxGroups,
		"max_messages_day":  t.MaxMessagesDay,
	})
}

// handleRotateBotToken rota el token del bot validando password + getMe.
// Flujo: bcrypt → getMe → encrypt → DB → RegisterHot → 200.
// El token en claro NUNCA se loguea ni devuelve (spec token hygiene).
func (s *Server) handleRotateBotToken(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantIDFromClaims(w, r)
	if !ok {
		return
	}

	// 1. Parse body.
	var req rotateTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "body invalido")
		return
	}
	if req.Password == "" || req.BotToken == "" {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "password y bot_token son requeridos")
		return
	}

	// 2. Obtener el admin actual para validar la password.
	claims := claimsFromContext(r.Context())
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "no autenticado")
		return
	}
	adminID, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "identidad invalida")
		return
	}
	admin, err := s.authSvc.GetAdminByID(r.Context(), adminID)
	if err != nil {
		slog.Error("rotate-token: admin lookup failed", "tenant_id", tenantID, "error", err)
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "no se pudo verificar la identidad")
		return
	}

	// 3. bcrypt.CompareHashAndPassword.
	if !admin.CheckPassword(req.Password) {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "password incorrecta")
		return
	}

	// 4. Validar el token con Telegram getMe.
	tg := telegram.NewAdapter(req.BotToken)
	botUser, err := tg.GetMe(r.Context())
	if err != nil {
		respondError(w, http.StatusBadGateway, "TELEGRAM_ERROR", "Telegram rechazo el bot token")
		return
	}

	// 5. Cifrar el token.
	if s.tenantCrypter == nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "cifrado no configurado")
		return
	}
	enc, err := s.tenantCrypter.Encrypt([]byte(req.BotToken))
	if err != nil {
		slog.Error("rotate-token: encrypt failed", "tenant_id", tenantID, "error", err)
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "no se pudo cifrar el token")
		return
	}

	// 6. Persistir en DB (token cifrado + username).
	botUsername := botUser.Username
	if err := s.tenantsRepo.UpdateToken(r.Context(), tenantID, enc, &botUsername); err != nil {
		slog.Error("rotate-token: update failed", "tenant_id", tenantID, "error", err)
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "no se pudo persistir el token")
		return
	}

	// 7. Reiniciar el poller en caliente con el nuevo token.
	// RegisterHot esta bajo mutex y es seguro (spec RegisterHot).
	t, err := s.tenantsRepo.GetByID(r.Context(), tenantID)
	if err != nil {
		slog.Error("rotate-token: get tenant after update", "tenant_id", tenantID, "error", err)
		// El token ya esta persistido; el poller se reiniciara al reiniciar el backend.
	} else if s.tenantRegistry != nil && s.tenantBusFor != nil {
		bus := s.tenantBusFor(tenantID)
		if bus != nil {
			s.tenantRegistry.RegisterHot(r.Context(), tenantID, t.Slug, req.BotToken, bus)
			slog.Info("rotate-token: poller reiniciado", "tenant_id", tenantID, "slug", t.Slug)
		}
	}

	// 8. Respuesta: solo status, nunca el token.
	respond(w, http.StatusOK, map[string]string{"status": "rotated"})
}

// handleGetTenantStatus devuelve el bot_status runtime del tenant
// (consultando el registry, no la DB). Endpoint liviano.
//
// Mapeo de status interno → valor de API:
//   - "active" → "connected"
//   - "degraded" → "disconnected"
//   - desconocido → "unknown"
func (s *Server) handleGetTenantStatus(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantIDFromClaims(w, r)
	if !ok {
		return
	}

	botStatus := "unknown"
	if s.tenantRegistry != nil {
		if status, ok := s.tenantRegistry.Status(tenantID); ok {
			switch status {
			case "active":
				botStatus = "connected"
			case "degraded":
				botStatus = "disconnected"
			default:
				botStatus = status
			}
		}
	}

	respond(w, http.StatusOK, botStatusResult{Status: botStatus})
}

// botStatusResult es la respuesta de GET /api/tenants/me/status.
type botStatusResult struct {
	Status string `json:"bot_status"`
}

// handleGetTenantLicense devuelve la info de licencia del tenant
// autenticado. Exento de requireLicense para que tenants suspendidos/
// expirados puedan ver su propio estado.
func (s *Server) handleGetTenantLicense(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantIDFromClaims(w, r)
	if !ok {
		return
	}

	t, err := s.tenantsRepo.GetByID(r.Context(), tenantID)
	if errors.Is(err, tenants.ErrNotFound) {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "tenant no encontrado")
		return
	}
	if err != nil {
		slog.Error("tenants/me/license: repo error", "tenant_id", tenantID, "error", err)
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "no se pudo obtener la licencia")
		return
	}

	respond(w, http.StatusOK, map[string]any{
		"license_status":    t.Status,
		"plan":              t.Plan,
		"trial_ends_at":     t.TrialEndsAt,
		"expires_at":        t.ExpiresAt,
		"max_groups":        t.MaxGroups,
		"max_messages_day":  t.MaxMessagesDay,
	})
}
