package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/telegram-manager/backend/internal/auth"
)

// contextKey type-safe para claims en el context de la request.
type contextKey string

const claimsKey contextKey = "auth_claims"

// authenticator es la vista minima del TokenManager que el middleware
// necesita: validar un access token y devolver la identidad.
type authenticator interface {
	ParseAccess(token string) (*auth.Claims, error)
}

// requireAuth es el middleware de IDENTIDAD del API (AGENTS.md 17 y
// backend-go-skill §7): valida el access token del header Authorization
// e inyecta los claims en el context. La AUTORIZACION (qué puede hacer
// cada admin, por grupo) es una capa aparte en el servicio, no aca.
//
// Nunca loguea el token completo ni lo incluye en respuestas.
func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		raw, ok := strings.CutPrefix(header, "Bearer ")
		if !ok || raw == "" {
			respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "se requiere un access token valido")
			return
		}

		claims, err := s.auth.ParseAccess(raw)
		if err != nil {
			respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "access token invalido o expirado")
			return
		}

		if claims.TenantID == 0 {
			// Access legacy pre-multitenancy (valido en firma pero sin
			// tenant): no puede aislar datos → 401 con re-login. El
			// refresh sigue vigente y re-emite con tenant (ver
			// Service.Refresh); no se exige re-signup.
			respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "sesion anterior a multitenancy: volve a iniciar sesion")
			return
		}

		ctx := context.WithValue(r.Context(), claimsKey, claims)
		next(w, r.WithContext(ctx))
	}
}

// claimsFromContext devuelve la identidad inyectada por requireAuth.
// Solo debe llamarse dentro de handlers protegidos.
func claimsFromContext(ctx context.Context) *auth.Claims {
	c, _ := ctx.Value(claimsKey).(*auth.Claims)
	return c
}
