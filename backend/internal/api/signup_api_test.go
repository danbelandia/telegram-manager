package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/telegram-manager/backend/internal/auth"
	"github.com/telegram-manager/backend/internal/tenants"
)

// errBotRejected simula getMe rechazado (token invalido).
var errBotRejected = errors.New("telegram: invalid token")

// buildSignupServer construye un Server con POST /api/auth/signup
// habilitado: DB real, TG mockeado (sin Bot API real), crypter de
// fixture. Los slugs/usernames son unicos por test (la base api se
// comparte entre tests del paquete).
func buildSignupServer(t *testing.T, botErr error, calls *int) *Server {
	t.Helper()
	db := apiTestDB(t)
	tenantRepo := tenants.NewRepository(db)
	authRepo := auth.NewRepository(db)
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(100 + i)
	}
	crypter, err := tenants.NewCrypter(key)
	if err != nil {
		t.Fatalf("crypter: %v", err)
	}
	validate := func(_ context.Context, token string) (string, error) {
		if calls != nil {
			*calls++
		}
		if botErr != nil {
			return "", botErr
		}
		return "api_bot", nil
	}
	svc := auth.NewSignupService(db, tenantRepo, authRepo, crypter, validate)
	return NewServer(db, botStatusStub{}, WithSignup(svc, nil))
}

func signupBody(t *testing.T, rrBody []byte) map[string]any {
	t.Helper()
	var body struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rrBody, &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return body.Data
}

func TestSignup_HappyPath201(t *testing.T) {
	server := buildSignupServer(t, nil, nil)
	token := "tok-api-VALOR-UNICO-1"
	rr := doRequest(server, "POST", "/api/auth/signup",
		`{"slug":"shop1","username":"owner1","password":"secret123","bot_token":"`+token+`"}`, "")

	if rr.Code != http.StatusCreated {
		t.Fatalf("code = %d, want 201 (body: %s)", rr.Code, rr.Body.String())
	}
	data := signupBody(t, rr.Body.Bytes())
	tenant, _ := data["tenant"].(map[string]any)
	admin, _ := data["admin"].(map[string]any)
	if tenant["slug"] != "shop1" || admin["username"] != "owner1" {
		t.Errorf("data = %v", data)
	}
	// El token NUNCA vuelve en la respuesta ni en logs.
	if strings.Contains(rr.Body.String(), token) {
		t.Fatal("la respuesta contiene el bot token en claro")
	}
}

func TestSignup_Validation400(t *testing.T) {
	server := buildSignupServer(t, nil, nil)
	rr := doRequest(server, "POST", "/api/auth/signup",
		`{"slug":"shop2","username":"owner2"}`, "")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("code = %d, want 400 (body: %s)", rr.Code, rr.Body.String())
	}
}

func TestSignup_SlugConflict409(t *testing.T) {
	var calls int
	server := buildSignupServer(t, nil, &calls)
	body := `{"slug":"shop3","username":"owner3","password":"secret123","bot_token":"tok-x"}`
	if rr := doRequest(server, "POST", "/api/auth/signup", body, ""); rr.Code != http.StatusCreated {
		t.Fatalf("setup code = %d, want 201", rr.Code)
	}
	before := calls
	rr := doRequest(server, "POST", "/api/auth/signup",
		`{"slug":"shop3","username":"otro","password":"secret123","bot_token":"tok-y"}`, "")
	if rr.Code != http.StatusConflict {
		t.Fatalf("code = %d, want 409 (body: %s)", rr.Code, rr.Body.String())
	}
	if calls != before {
		t.Error("slug duplicado llamo a Telegram")
	}
}

func TestSignup_UsernameConflict409(t *testing.T) {
	server := buildSignupServer(t, nil, nil)
	if rr := doRequest(server, "POST", "/api/auth/signup",
		`{"slug":"shop4","username":"repetido","password":"secret123","bot_token":"tok-x"}`, ""); rr.Code != http.StatusCreated {
		t.Fatalf("setup code = %d, want 201", rr.Code)
	}
	rr := doRequest(server, "POST", "/api/auth/signup",
		`{"slug":"shop4b","username":"repetido","password":"secret123","bot_token":"tok-y"}`, "")
	if rr.Code != http.StatusConflict {
		t.Fatalf("code = %d, want 409 username global (body: %s)", rr.Code, rr.Body.String())
	}
}

func TestSignup_InvalidToken502(t *testing.T) {
	server := buildSignupServer(t, errBotRejected, nil)
	rr := doRequest(server, "POST", "/api/auth/signup",
		`{"slug":"shop5","username":"owner5","password":"secret123","bot_token":"malo"}`, "")
	if rr.Code != http.StatusBadGateway {
		t.Fatalf("code = %d, want 502 (body: %s)", rr.Code, rr.Body.String())
	}
	var body struct {
		Error *struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	if body.Error == nil || body.Error.Code != "TELEGRAM_ERROR" {
		t.Errorf("code = %v, want TELEGRAM_ERROR", body.Error)
	}
}
