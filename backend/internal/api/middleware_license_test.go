package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/telegram-manager/backend/internal/auth"
	"github.com/telegram-manager/backend/internal/license"
	"github.com/telegram-manager/backend/internal/tenants"
)

// TestRequireLicense_NilSvc tests that requireLicense is a no-op when licenseSvc is nil.
func TestRequireLicense_NilSvc(t *testing.T) {
	s := &Server{mux: http.NewServeMux()}

	called := false
	handler := s.requireLicense(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	// Sin licenseSvc configurado, el middleware debe dejar pasar.
	req := httptest.NewRequest("GET", "/api/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("handler was not called when licenseSvc is nil")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

// TestRequireLicense_Suspended tests that a suspended tenant gets 403.
func TestRequireLicense_Suspended(t *testing.T) {
	fg := &fakeTenantGetterForTest{status: "suspended"}
	licenseSvc := license.NewService(fg)
	s := &Server{mux: http.NewServeMux(), licenseSvc: licenseSvc}

	called := false
	handler := s.requireLicense(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	// Inyectar claims con TenantID en el context.
	ctx := withClaimsForTest(1)
	req := httptest.NewRequest("GET", "/api/test", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if called {
		t.Error("handler should NOT be called for suspended tenant")
	}
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}

// TestRequireLicense_Expired tests that an expired tenant gets 403.
func TestRequireLicense_Expired(t *testing.T) {
	fg := &fakeTenantGetterForTest{status: "expired"}
	licenseSvc := license.NewService(fg)
	s := &Server{mux: http.NewServeMux(), licenseSvc: licenseSvc}

	called := false
	handler := s.requireLicense(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	ctx := withClaimsForTest(1)
	req := httptest.NewRequest("GET", "/api/test", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if called {
		t.Error("handler should NOT be called for expired tenant")
	}
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}

// TestRequireLicense_Active tests that an active tenant passes through.
func TestRequireLicense_Active(t *testing.T) {
	fg := &fakeTenantGetterForTest{status: "active"}
	licenseSvc := license.NewService(fg)
	s := &Server{mux: http.NewServeMux(), licenseSvc: licenseSvc}

	called := false
	handler := s.requireLicense(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	ctx := withClaimsForTest(1)
	req := httptest.NewRequest("GET", "/api/test", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("handler should be called for active tenant")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

// TestRequireSuperAdmin_Passes tests that super-admin passes the middleware.
func TestRequireSuperAdmin_Passes(t *testing.T) {
	s := &Server{mux: http.NewServeMux()}

	called := false
	handler := s.requireSuperAdmin(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	claims := &auth.Claims{IsSuperAdmin: true}
	ctx := withClaimsContextForTest(claims)
	req := httptest.NewRequest("GET", "/api/admin/test", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("handler should be called for super-admin")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

// TestRequireSuperAdmin_Blocked tests that non-super-admin gets 403.
func TestRequireSuperAdmin_Blocked(t *testing.T) {
	s := &Server{mux: http.NewServeMux()}

	called := false
	handler := s.requireSuperAdmin(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	claims := &auth.Claims{IsSuperAdmin: false}
	ctx := withClaimsContextForTest(claims)
	req := httptest.NewRequest("GET", "/api/admin/test", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if called {
		t.Error("handler should NOT be called for non-super-admin")
	}
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}

// --- test helpers ---

type fakeTenantGetterForTest struct {
	status string
}

func (f *fakeTenantGetterForTest) GetLicense(_ context.Context, id int64) (*tenants.TenantLicense, error) {
	return &tenants.TenantLicense{
		ID:     id,
		Status: f.status,
	}, nil
}

// withClaimsForTest inyecta claims con un TenantID en el context.
func withClaimsForTest(tenantID int64) context.Context {
	claims := &auth.Claims{TenantID: tenantID}
	return context.WithValue(context.Background(), claimsKey, claims)
}

// withClaimsContextForTest inyecta claims custom en el context.
func withClaimsContextForTest(claims *auth.Claims) context.Context {
	return context.WithValue(context.Background(), claimsKey, claims)
}
