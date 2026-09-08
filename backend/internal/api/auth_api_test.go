package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"golang.org/x/crypto/bcrypt"

	"github.com/telegram-manager/backend/internal/auth"
	"github.com/telegram-manager/backend/internal/database"
	"github.com/telegram-manager/backend/internal/tenants"
)

const testSecret = "test-secret-ojos-que-no-ven-corazon-que-no-siente-x"

// botStatusStub satisface botStatusProvider para NewServer.
type botStatusStub struct{}

func (botStatusStub) Status() (bool, string) { return true, "ManagerV01_bot" }

// apiTestDB abre una PostgreSQL real (convencion del proyecto, no
// mockear la DB) y limpia admins. Cada suite usa su propia base
// (telegram_manager_api) para no pisarse con auth/groups en
// `go test ./...`.
func apiTestDB(t *testing.T) *sql.DB {
	t.Helper()
	if testing.Short() {
		t.Skip("integration: skipping with -short")
	}

	db, err := database.OpenTestDB(t, "api")
	if err != nil {
		t.Skipf("integration: no postgres available: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := database.Migrate(context.Background(), db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, err := db.ExecContext(context.Background(), "TRUNCATE admins"); err != nil {
		t.Fatalf("truncate admins: %v", err)
	}
	return db
}

func mustHash(t *testing.T, pw string) string {
	t.Helper()
	b, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	return string(b)
}

// buildAuthServer construye un Server con auth habilitado y un admin de
// prueba persistido. Devuelve el Server, el TokenManager y el username.
func buildAuthServer(t *testing.T) (*Server, *auth.TokenManager, string) {
	t.Helper()
	db := apiTestDB(t)
	repo := auth.NewRepository(db)
	tokenManager := auth.NewTokenManager(testSecret)
	svc := auth.NewService(repo, tokenManager)
	tenantID, err := tenants.NewRepository(db).EnsureDefault(context.Background())
	if err != nil {
		t.Fatalf("ensure default tenant: %v", err)
	}
	if _, err := repo.Create(context.Background(), "admin", mustHash(t, "secret123"), tenantID); err != nil {
		t.Fatalf("create admin: %v", err)
	}
	server := NewServer(db, botStatusStub{}, WithAuth(svc, tokenManager, false))
	return server, tokenManager, "admin"
}

func doRequest(server *Server, method, path, body, accessToken string) *httptest.ResponseRecorder {
	var reader *bytes.Reader
	if body == "" {
		reader = bytes.NewReader(nil)
	} else {
		reader = bytes.NewReader([]byte(body))
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}
	rr := httptest.NewRecorder()
	server.ServeHTTP(rr, req)
	return rr
}

func extractCookieValue(setCookie, name string) string {
	for _, part := range strings.Split(setCookie, ";") {
		trimmed := strings.TrimSpace(part)
		if strings.HasPrefix(trimmed, name+"=") {
			return strings.TrimPrefix(trimmed, name+"=")
		}
	}
	return ""
}

func TestLogin_Success(t *testing.T) {
	server, _, _ := buildAuthServer(t)
	rr := doRequest(server, "POST", "/api/auth/login",
		`{"username":"admin","password":"secret123"}`, "")

	if rr.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
	if rr.Header().Get("Set-Cookie") == "" {
		t.Fatal("want refresh cookie set")
	}
	var body struct {
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Data.AccessToken == "" {
		t.Error("want non-empty access_token")
	}
}

func TestLogin_CookieFlags(t *testing.T) {
	server, _, _ := buildAuthServer(t)
	rr := doRequest(server, "POST", "/api/auth/login",
		`{"username":"admin","password":"secret123"}`, "")
	cookie := rr.Header().Get("Set-Cookie")
	if !strings.Contains(cookie, "HttpOnly") {
		t.Errorf("cookie sin HttpOnly: %s", cookie)
	}
	if !strings.Contains(cookie, "SameSite=Strict") {
		t.Errorf("cookie sin SameSite=Strict: %s", cookie)
	}
	if strings.Contains(cookie, "Secure") {
		t.Errorf("cookie con Secure cuando COOKIE_SECURE=false: %s", cookie)
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	server, _, _ := buildAuthServer(t)
	rr := doRequest(server, "POST", "/api/auth/login",
		`{"username":"admin","password":"mal"}`, "")
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("code = %d, want 401", rr.Code)
	}
}

func TestLogin_UnknownUserSame401(t *testing.T) {
	server, _, _ := buildAuthServer(t)
	rr := doRequest(server, "POST", "/api/auth/login",
		`{"username":"noexiste","password":"secret123"}`, "")
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("code = %d, want 401 (mismo que password mal)", rr.Code)
	}
}

func TestLogin_EmptyCredentials(t *testing.T) {
	server, _, _ := buildAuthServer(t)
	rr := doRequest(server, "POST", "/api/auth/login", `{"username":"","password":""}`, "")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("code = %d, want 400", rr.Code)
	}
}

func TestRefresh_Success(t *testing.T) {
	server, _, _ := buildAuthServer(t)
	loginRR := doRequest(server, "POST", "/api/auth/login",
		`{"username":"admin","password":"secret123"}`, "")
	refreshVal := extractCookieValue(loginRR.Header().Get("Set-Cookie"), "refresh_token")
	if refreshVal == "" {
		t.Fatal("no refresh token en cookie")
	}

	req := httptest.NewRequest("POST", "/api/auth/refresh", nil)
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: refreshVal})
	rr := httptest.NewRecorder()
	server.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
	var body struct {
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	if body.Data.AccessToken == "" {
		t.Error("want new access_token")
	}
}

func TestRefresh_NoCookie(t *testing.T) {
	server, _, _ := buildAuthServer(t)
	rr := doRequest(server, "POST", "/api/auth/refresh", "", "")
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("code = %d, want 401", rr.Code)
	}
}

func TestLogout_ExpiresCookie(t *testing.T) {
	server, _, _ := buildAuthServer(t)
	rr := doRequest(server, "POST", "/api/auth/logout", "", "")
	if rr.Code != http.StatusNoContent {
		t.Fatalf("code = %d, want 204", rr.Code)
	}
	cookie := rr.Header().Get("Set-Cookie")
	if !strings.Contains(cookie, "Max-Age=0") && !strings.Contains(cookie, "01 Jan 1970") {
		t.Errorf("cookie de logout no expirada: %s", cookie)
	}
}

func TestMe_WithValidAccess(t *testing.T) {
	server, tokenManager, username := buildAuthServer(t)
	admin := auth.Admin{ID: 1, Username: username, TenantID: 1}
	access, err := tokenManager.IssueAccess(admin)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	rr := doRequest(server, "GET", "/api/auth/me", "", access)
	if rr.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
	var body struct {
		Data struct {
			Username string `json:"username"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	if body.Data.Username != username {
		t.Errorf("username = %q, want %q", body.Data.Username, username)
	}
}

func TestMe_NoAccess(t *testing.T) {
	server, _, _ := buildAuthServer(t)
	rr := doRequest(server, "GET", "/api/auth/me", "", "")
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("code = %d, want 401", rr.Code)
	}
}

func TestMe_InvalidAccess(t *testing.T) {
	server, _, _ := buildAuthServer(t)
	rr := doRequest(server, "GET", "/api/auth/me", "", "token-invalido")
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("code = %d, want 401", rr.Code)
	}
}
