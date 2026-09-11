package license

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/telegram-manager/backend/internal/tenants"
)

// fakeTenantGetter es un TenantGetter en memoria para tests unitarios.
type fakeTenantGetter struct {
	tenants map[int64]*tenants.TenantLicense
}

func (f *fakeTenantGetter) GetLicense(_ context.Context, id int64) (*tenants.TenantLicense, error) {
	tl, ok := f.tenants[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return tl, nil
}

func TestEnforce_Active(t *testing.T) {
	fg := &fakeTenantGetter{tenants: map[int64]*tenants.TenantLicense{
		1: {ID: 1, Status: "active"},
	}}
	svc := NewService(fg)

	if err := svc.Enforce(context.Background(), 1); err != nil {
		t.Errorf("active tenant: want nil, got %v", err)
	}
}

func TestEnforce_TrialValid(t *testing.T) {
	future := time.Now().Add(24 * time.Hour)
	fg := &fakeTenantGetter{tenants: map[int64]*tenants.TenantLicense{
		1: {ID: 1, Status: "trial", TrialEndsAt: &future},
	}}
	svc := NewService(fg)

	if err := svc.Enforce(context.Background(), 1); err != nil {
		t.Errorf("trial valid: want nil, got %v", err)
	}
}

func TestEnforce_TrialExpired(t *testing.T) {
	past := time.Now().Add(-1 * time.Hour)
	fg := &fakeTenantGetter{tenants: map[int64]*tenants.TenantLicense{
		1: {ID: 1, Status: "trial", TrialEndsAt: &past},
	}}
	svc := NewService(fg)

	if err := svc.Enforce(context.Background(), 1); !errors.Is(err, ErrExpired) {
		t.Errorf("trial expired: want ErrExpired, got %v", err)
	}
}

func TestEnforce_TrialNoDeadline(t *testing.T) {
	// Trial sin trial_ends_at: no se auto-expira (defensivo).
	fg := &fakeTenantGetter{tenants: map[int64]*tenants.TenantLicense{
		1: {ID: 1, Status: "trial", TrialEndsAt: nil},
	}}
	svc := NewService(fg)

	if err := svc.Enforce(context.Background(), 1); err != nil {
		t.Errorf("trial no deadline: want nil, got %v", err)
	}
}

func TestEnforce_Suspended(t *testing.T) {
	fg := &fakeTenantGetter{tenants: map[int64]*tenants.TenantLicense{
		1: {ID: 1, Status: "suspended"},
	}}
	svc := NewService(fg)

	if err := svc.Enforce(context.Background(), 1); !errors.Is(err, ErrSuspended) {
		t.Errorf("suspended: want ErrSuspended, got %v", err)
	}
}

func TestEnforce_Expired(t *testing.T) {
	fg := &fakeTenantGetter{tenants: map[int64]*tenants.TenantLicense{
		1: {ID: 1, Status: "expired"},
	}}
	svc := NewService(fg)

	if err := svc.Enforce(context.Background(), 1); !errors.Is(err, ErrExpired) {
		t.Errorf("expired: want ErrExpired, got %v", err)
	}
}

func TestEnforce_DBError(t *testing.T) {
	fg := &fakeTenantGetter{tenants: map[int64]*tenants.TenantLicense{}}
	svc := NewService(fg)

	if err := svc.Enforce(context.Background(), 999); err == nil {
		t.Error("unknown tenant: want error, got nil")
	}
}

func TestInfo(t *testing.T) {
	now := time.Now()
	fg := &fakeTenantGetter{tenants: map[int64]*tenants.TenantLicense{
		1: {ID: 1, Status: "active", ExpiresAt: &now},
	}}
	svc := NewService(fg)

	tl, err := svc.Info(context.Background(), 1)
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if tl.Status != "active" {
		t.Errorf("status = %q, want active", tl.Status)
	}
}
