package telegram

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newStubServer arma un servidor local que responde como la Bot API.
// handler recibe la request; el default responde 200 ok con un bot.
func newStubServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// El path debe seguir el formato /bot<TOKEN>/<metodo>.
		if !strings.HasPrefix(r.URL.Path, "/bot") || !strings.HasSuffix(r.URL.Path, "/getMe") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if handler != nil {
			handler(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true,"result":{"id":123456,"username":"admin_bot","first_name":"Admin"}}`))
	}))
}

func TestAdapterGetMe_Success(t *testing.T) {
	srv := newStubServer(t, nil)
	defer srv.Close()

	adapter := NewAdapter("123456:test", WithBaseURL(srv.URL))

	user, err := adapter.GetMe(context.Background())
	if err != nil {
		t.Fatalf("GetMe() unexpected error: %v", err)
	}
	if user.ID != 123456 {
		t.Errorf("ID: want 123456, got %d", user.ID)
	}
	if user.Username != "admin_bot" {
		t.Errorf("Username: want admin_bot, got %q", user.Username)
	}

	connected, username := adapter.Status()
	if !connected {
		t.Error("Status(): want connected=true after successful GetMe")
	}
	if username != "admin_bot" {
		t.Errorf("Status(): username = %q, want admin_bot", username)
	}
}

func TestAdapterGetMe_InvalidToken(t *testing.T) {
	srv := newStubServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"ok":false,"error_code":401,"description":"Unauthorized"}`))
	})
	defer srv.Close()

	adapter := NewAdapter("bad-token", WithBaseURL(srv.URL))

	_, err := adapter.GetMe(context.Background())
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("GetMe() error = %v, want ErrInvalidToken", err)
	}

	connected, _ := adapter.Status()
	if connected {
		t.Error("Status(): want connected=false when token invalid")
	}
}

func TestAdapterGetMe_TelegramAPIError(t *testing.T) {
	srv := newStubServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"ok":false,"error_code":500,"description":"Internal Server Error"}`))
	})
	defer srv.Close()

	adapter := NewAdapter("123456:test", WithBaseURL(srv.URL))

	_, err := adapter.GetMe(context.Background())
	if err == nil {
		t.Fatal("GetMe() expected error, got nil")
	}
	if errors.Is(err, ErrInvalidToken) {
		t.Fatalf("GetMe() error = %v, want non-token error", err)
	}
}

func TestAdapterGetMe_Unavailable(t *testing.T) {
	// Server que cierra apenas recibe la conexion.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("should not be reached")
	}))
	url := srv.URL
	srv.Close() // fondo muerto: cualquier request falla

	adapter := NewAdapter("123456:test", WithBaseURL(url))

	_, err := adapter.GetMe(context.Background())
	if !errors.Is(err, ErrTelegramUnavailable) {
		t.Fatalf("GetMe() error = %v, want ErrTelegramUnavailable", err)
	}
}
