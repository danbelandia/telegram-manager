package api

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/telegram-manager/backend/internal/auth"
	"github.com/telegram-manager/backend/internal/telegram"
	"github.com/telegram-manager/backend/internal/tenants"
)

// --- Fakes para tenant settings ---

// fakeTenantRepo satisface tenantSettingsRepo.
type fakeTenantRepo struct {
	tenant *tenants.Tenant
	getErr error
}

func (f *fakeTenantRepo) GetByID(_ context.Context, _ int64) (*tenants.Tenant, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	if f.tenant == nil {
		return nil, tenants.ErrNotFound
	}
	return f.tenant, nil
}

func (f *fakeTenantRepo) UpdateToken(_ context.Context, _ int64, _ []byte, _ *string) error {
	return nil
}

// fakeTenantRegistry satisface tenantSettingsRegistry.
type fakeTenantRegistry struct {
	status string
}

func (f *fakeTenantRegistry) Status(_ int64) (string, bool) {
	if f.status == "" {
		return "", false
	}
	return f.status, true
}

func (f *fakeTenantRegistry) RegisterHot(_ context.Context, _ int64, _, _ string, _ telegram.Publisher) {}

// fakeTenantCrypter satisface tenantSettingsCrypter.
type fakeTenantCrypter struct{}

func (f *fakeTenantCrypter) Encrypt(plain []byte) ([]byte, error) {
	return append([]byte("enc:"), plain...), nil
}

func (f *fakeTenantCrypter) Decrypt(blob []byte) ([]byte, error) {
	return blob, nil
}

// fakeAuthService2 satisface authService para tenant handlers.
type fakeAuthService2 struct {
	admin *auth.Admin
	err   error
}

func (f *fakeAuthService2) Login(_ context.Context, _, _ string) (auth.LoginResult, error) {
	return auth.LoginResult{}, nil
}

func (f *fakeAuthService2) Refresh(_ context.Context, _ string) (string, error) {
	return "", nil
}

func (f *fakeAuthService2) GetAdminByID(_ context.Context, _ int64) (*auth.Admin, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.admin, nil
}

// --- Tests ---

func TestGetTenantMe_Success(t *testing.T) {
	botUsername := "TestBot"
	repo := &fakeTenantRepo{
		tenant: &tenants.Tenant{
			ID:          1,
			Slug:        "test-tenant",
			BotUsername: &botUsername,
			Status:      "active",
			CreatedAt:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		},
	}
	registry := &fakeTenantRegistry{status: "connected"}

	server := NewServer(fakePinger{}, botStatusStub{},
		WithAuth(&fakeAuthService2{}, &fixedAuthenticator{claims: testClaims}, false),
		WithTenants(repo, registry, &fakeTenantCrypter{}, nil),
	)

	rr := doRequest(server, "GET", "/api/tenants/me", "", "valid-token")
	if rr.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}

	var body struct {
		Data struct {
			Slug      string `json:"slug"`
			BotUser   string `json:"bot_username"`
			BotStatus string `json:"bot_status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Data.Slug != "test-tenant" {
		t.Errorf("slug = %q, want %q", body.Data.Slug, "test-tenant")
	}
	if body.Data.BotStatus != "connected" {
		t.Errorf("bot_status = %q, want %q", body.Data.BotStatus, "connected")
	}
}

func TestGetTenantMe_NotFound(t *testing.T) {
	repo := &fakeTenantRepo{
		getErr: tenants.ErrNotFound,
	}
	server := NewServer(fakePinger{}, botStatusStub{},
		WithAuth(&fakeAuthService2{}, &fixedAuthenticator{claims: testClaims}, false),
		WithTenants(repo, &fakeTenantRegistry{}, &fakeTenantCrypter{}, nil),
	)

	rr := doRequest(server, "GET", "/api/tenants/me", "", "valid-token")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("code = %d, want 404", rr.Code)
	}
}

func TestGetTenantStatus_Connected(t *testing.T) {
	repo := &fakeTenantRepo{
		tenant: &tenants.Tenant{Slug: "test"},
	}
	registry := &fakeTenantRegistry{status: "connected"}
	server := NewServer(fakePinger{}, botStatusStub{},
		WithAuth(&fakeAuthService2{}, &fixedAuthenticator{claims: testClaims}, false),
		WithTenants(repo, registry, &fakeTenantCrypter{}, nil),
	)

	rr := doRequest(server, "GET", "/api/tenants/me/status", "", "valid-token")
	if rr.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}

	var body struct {
		Data struct {
			BotStatus string `json:"bot_status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Data.BotStatus != "connected" {
		t.Errorf("bot_status = %q, want %q", body.Data.BotStatus, "connected")
	}
}

func TestRotateToken_MissingFields(t *testing.T) {
	server := NewServer(fakePinger{}, botStatusStub{},
		WithAuth(&fakeAuthService2{}, &fixedAuthenticator{claims: testClaims}, false),
		WithTenants(&fakeTenantRepo{}, &fakeTenantRegistry{}, &fakeTenantCrypter{}, nil),
	)

	rr := doRequest(server, "PUT", "/api/tenants/me/bot-token", `{"password":"x"}`, "valid-token")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("code = %d, want 400", rr.Code)
	}
}

func TestRotateToken_BadPassword(t *testing.T) {
	server := NewServer(fakePinger{}, botStatusStub{},
		WithAuth(&fakeAuthService2{
			admin: &auth.Admin{
				ID:           1,
				PasswordHash: mustHash(t, "correct-password"),
			},
		}, &fixedAuthenticator{claims: testClaims}, false),
		WithTenants(&fakeTenantRepo{}, &fakeTenantRegistry{}, &fakeTenantCrypter{}, nil),
	)

	rr := doRequest(server, "PUT", "/api/tenants/me/bot-token",
		`{"password":"wrong","bot_token":"123:abc"}`, "valid-token")
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("code = %d, want 401 (body: %s)", rr.Code, rr.Body.String())
	}
}

// --- Tests para handleMe con tenant_slug ---

func TestMe_IncludesTenantSlug(t *testing.T) {
	botUser := "TestBot"
	server := NewServer(fakePinger{}, botStatusStub{},
		WithAuth(&fakeAuthService2{}, &fixedAuthenticator{claims: testClaims}, false),
		WithTenants(&fakeTenantRepo{
			tenant: &tenants.Tenant{Slug: "my-slug", BotUsername: &botUser},
		}, &fakeTenantRegistry{}, &fakeTenantCrypter{}, nil),
	)

	rr := doRequest(server, "GET", "/api/auth/me", "", "valid-token")
	if rr.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}

	var body struct {
		Data struct {
			ID         string `json:"id"`
			Username   string `json:"username"`
			TenantID   int64  `json:"tenant_id"`
			TenantSlug string `json:"tenant_slug"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Data.TenantSlug != "my-slug" {
		t.Errorf("tenant_slug = %q, want %q", body.Data.TenantSlug, "my-slug")
	}
}
