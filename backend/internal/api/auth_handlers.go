package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/telegram-manager/backend/internal/auth"
)

// authService es la vista minima del servicio de auth que los handlers
// necesitan. Declarada donde se consume.
type authService interface {
	Login(ctx context.Context, username, password string) (auth.LoginResult, error)
	Refresh(ctx context.Context, refreshToken string) (string, error)
}

// signupService es la vista minima del alta de tenants (slice 0).
type signupService interface {
	Signup(ctx context.Context, in auth.SignupInput) (auth.SignupResult, error)
}

// loginRequest es el body de POST /api/auth/login.
type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// handleLogin autentica con username+password, devuelve el access token
// y setea la cookie httpOnly del refresh token.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "body invalido")
		return
	}
	if req.Username == "" || req.Password == "" {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "username y password son requeridos")
		return
	}

	res, err := s.authSvc.Login(r.Context(), req.Username, req.Password)
	if errors.Is(err, auth.ErrCredentialInvalid) {
		// Mismo mensaje para username inexistente y password incorrecto:
		// no revela cual de los dos fallo (AGENTS spec auth req 1).
		respondError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "credenciales invalidas")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "no se pudo iniciar sesion")
		return
	}

	s.setRefreshCookie(w, res.RefreshToken)
	respond(w, http.StatusOK, map[string]string{"access_token": res.AccessToken})
}

// handleRefresh valida la cookie de refresh y emite un nuevo access.
func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "sin cookie de refresh")
		return
	}

	access, err := s.authSvc.Refresh(r.Context(), cookie.Value)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "refresh token invalido o expirado")
		return
	}
	respond(w, http.StatusOK, map[string]string{"access_token": access})
}

// handleLogout expira la cookie de refresh (el token es stateless: la
// sesion muere en el cliente).
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	s.clearRefreshCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

// handleMe devuelve la identidad del admin autenticado, incluyendo su
// tenant (slice 0). Es la prueba de que requireAuth funciona.
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromContext(r.Context())
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "no autenticado")
		return
	}
	respond(w, http.StatusOK, map[string]any{
		"id":        claims.Subject,
		"username":  claims.Username,
		"tenant_id": claims.TenantID,
	})
}

// signupRequest es el body de POST /api/auth/signup (slice 0).
type signupRequest struct {
	Slug     string `json:"slug"`
	Username string `json:"username"`
	Password string `json:"password"`
	BotToken string `json:"bot_token"`
}

// handleSignup da de alta un tenant bot-per-tenant (201 con
// tenant+admin). El token NUNCA vuelve en la respuesta ni en logs.
// Tras el commit, levanta el poller en caliente via onTenantReady sin
// tocar los demas pollers (falla el registro → warn, el tenant queda
// creado y el boot lo levanta al reiniciar).
func (s *Server) handleSignup(w http.ResponseWriter, r *http.Request) {
	if s.signup == nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "registro de tenants no habilitado")
		return
	}
	var req signupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "body invalido")
		return
	}

	res, err := s.signup.Signup(r.Context(), auth.SignupInput{
		Slug:     req.Slug,
		Username: req.Username,
		Password: req.Password,
		BotToken: req.BotToken,
	})
	switch {
	case err == nil:
		// ok, sigue abajo
	case errors.Is(err, auth.ErrSignupValidation):
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "slug, username, password (minimo 8 caracteres) y bot_token son requeridos")
		return
	case errors.Is(err, auth.ErrSlugTaken):
		respondError(w, http.StatusConflict, "CONFLICT", "el slug ya esta registrado")
		return
	case errors.Is(err, auth.ErrUsernameTaken):
		respondError(w, http.StatusConflict, "CONFLICT", "el username ya esta en uso")
		return
	case errors.Is(err, auth.ErrInvalidBotToken):
		respondError(w, http.StatusBadGateway, "TELEGRAM_ERROR", "Telegram rechazo el bot token")
		return
	default:
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "no se pudo crear el tenant")
		return
	}

	// Poller en caliente con el token del request (en memoria, nunca
	// en logs: solo slug e ids).
	if s.onTenantReady != nil {
		if herr := s.onTenantReady(r.Context(), res.TenantID, req.BotToken); herr != nil {
			slog.Warn("auth: signup ok pero poller en caliente fallo; se levantara al reiniciar",
				"tenant_id", res.TenantID, "slug", res.TenantSlug, "error", herr)
		}
	}
	slog.Info("auth: signup", "tenant_id", res.TenantID, "slug", res.TenantSlug, "admin_id", res.AdminID)
	respond(w, http.StatusCreated, map[string]any{
		"tenant": map[string]any{"id": res.TenantID, "slug": res.TenantSlug},
		"admin":  map[string]any{"id": res.AdminID, "username": res.AdminUsername},
	})
}

// refreshCookieName es el nombre de la cookie httpOnly del refresh.
const refreshCookieName = "refresh_token"

// setRefreshCookie setea la cookie del refresh con los flags que pide
// AGENTS 17.1: httpOnly, SameSite=Strict y Secure segun COOKIE_SECURE
// (false en dev local http, true en produccion https).
func (s *Server) setRefreshCookie(w http.ResponseWriter, value string) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cookieSecure,
		SameSite: http.SameSiteStrictMode,
		Expires:  time.Now().Add(auth.RefreshTokenTTL),
	})
}

func (s *Server) clearRefreshCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cookieSecure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1, // expiracion inmediata
	})
}
