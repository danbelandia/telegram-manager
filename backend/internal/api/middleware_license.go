package api

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/telegram-manager/backend/internal/license"
)

// requireLicense verifica el estado de licencia del tenant despues de
// requireAuth (que inyecta claims en el context). Rutas exentas
// (auth/*, /tenants/me/license) NO deben envolver con este middleware.
// Si licenseSvc es nil (tests, modo legacy), el middleware no aplica
// enforcement (permite el paso).
func (s *Server) requireLicense(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.licenseSvc == nil {
			next(w, r)
			return
		}

		tenantID, ok := tenantIDFromClaims(w, r)
		if !ok {
			return // ya respondio 401
		}

		if err := s.licenseSvc.Enforce(r.Context(), tenantID); err != nil {
			switch {
			case errors.Is(err, license.ErrSuspended):
				respondError(w, http.StatusForbidden,
					"LICENSE_SUSPENDED",
					"Tu licencia esta suspendida. Contacta al administrador.")
			case errors.Is(err, license.ErrExpired):
				respondError(w, http.StatusForbidden,
					"LICENSE_EXPIRED",
					"Tu licencia ha expirado. Contacta al administrador.")
			default:
				slog.Error("license: enforce error",
					"tenant_id", tenantID, "error", err)
				respondError(w, http.StatusInternalServerError,
					"INTERNAL_ERROR",
					"no se pudo verificar la licencia")
			}
			return
		}
		next(w, r)
	}
}
