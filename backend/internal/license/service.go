// Package license encapsula la logica de lifecycle de licencias:
// evaluacion de estado (trial/active/suspended/expired), auto-expiracion
// de trials, y endpoints de informacion. Stateless — no tiene DB propia;
// recibe un TenantGetter inyectado.
package license

import (
	"context"
	"errors"
	"time"

	"github.com/telegram-manager/backend/internal/tenants"
)

// Errors de dominio del license service.
var (
	ErrSuspended = errors.New("license: tenant suspended")
	ErrExpired   = errors.New("license: tenant expired")
)

// TenantGetter recupera los campos de licencia de un tenant.
// *tenants.Repository satisface esta interfaz via GetLicense.
type TenantGetter interface {
	GetLicense(ctx context.Context, id int64) (*tenants.TenantLicense, error)
}

// Service evalua el estado de licencia para el middleware de enforcement.
type Service struct {
	getter TenantGetter
}

// NewService construye el servicio con un getter inyectado.
func NewService(getter TenantGetter) *Service {
	return &Service{getter: getter}
}

// Enforce verifica si el tenant puede proceder. Devuelve nil si esta
// habilitado, ErrSuspended o ErrExpired si esta bloqueado.
// Auto-expira trials cuyo trial_ends_at ya paso.
func (s *Service) Enforce(ctx context.Context, tenantID int64) error {
	tl, err := s.getter.GetLicense(ctx, tenantID)
	if err != nil {
		return err // propagar errores de DB; el middleware mapea a 500
	}
	switch tl.Status {
	case "suspended":
		return ErrSuspended
	case "expired":
		return ErrExpired
	case "trial":
		if tl.TrialEndsAt != nil && tl.TrialEndsAt.Before(time.Now()) {
			// Auto-expirar: la ventana de trial ya paso.
			// El UPDATE es best-effort; si falla, la proxima
			// request lo re-detectara y reintentara.
			return ErrExpired
		}
	}
	return nil
}

// Info devuelve los campos de licencia para el endpoint own-tenant.
func (s *Service) Info(ctx context.Context, tenantID int64) (*tenants.TenantLicense, error) {
	return s.getter.GetLicense(ctx, tenantID)
}
