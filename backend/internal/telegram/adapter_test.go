package telegram

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
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

// newBotStubServer acepta cualquier metodo de la Bot API y registra el
// path en paths. Se usa para el transporte (getUpdates/setWebhook/
// deleteWebhook), que comparte el mismo /bot<TOKEN>/<metodo>.
func newBotStubServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *[]string) {
	t.Helper()
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/bot") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		paths = append(paths, r.URL.Path)
		if handler != nil {
			handler(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true,"result":[]}`))
	}))
	return srv, &paths
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

func TestAdapterError_DoesNotLeakToken(t *testing.T) {
	// Regresion Bug 1: el error de red de http.Client viene como
	// *url.Error con la URL completa (bot<TOKEN>/...). El adapter debe
	// devolver solo la causa, sin el token.
	const token = "123456:super-secret-token"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("should not be reached")
	}))
	url := srv.URL
	srv.Close()

	adapter := NewAdapter(token, WithBaseURL(url))

	_, err := adapter.GetMe(context.Background())
	if err == nil {
		t.Fatal("GetMe() expected error, got nil")
	}
	if strings.Contains(err.Error(), token) {
		t.Errorf("error leaks the token: %v", err)
	}
	if strings.Contains(err.Error(), "bot123456:super-secret-token") {
		t.Errorf("error leaks the URL with token: %v", err)
	}
}

func TestAdapterGetUpdates_LongPollNotTruncated(t *testing.T) {
	// Regresion Bug 2: el client no debe tener Timeout fijo que corte
	// el long poll. Un stub que tarda 11s (mas que los 10s del bug)
	// debe resolverse sin error cuando pedimos timeout=30.
	srv, _ := newBotStubServer(t, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(11 * time.Second)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true,"result":[]}`))
	})
	defer srv.Close()

	adapter := NewAdapter("123456:test", WithBaseURL(srv.URL))

	start := time.Now()
	_, err := adapter.GetUpdates(context.Background(), 0, 30, nil)
	if err != nil {
		t.Fatalf("GetUpdates() error = %v, want long poll sin truncar a los 10s", err)
	}
	if elapsed := time.Since(start); elapsed < 11*time.Second {
		t.Errorf("respuesta en %v, el stub simulaba long poll de 11s", elapsed)
	}
}

func TestAdapterGetUpdates_Success(t *testing.T) {
	srv, _ := newBotStubServer(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/getUpdates") {
			t.Errorf("path = %s, want /getUpdates", r.URL.Path)
		}
		if got := r.URL.Query().Get("offset"); got != "10" {
			t.Errorf("offset = %q, want 10", got)
		}
		if got := r.URL.Query().Get("timeout"); got != "30" {
			t.Errorf("timeout = %q, want 30", got)
		}
		if got := r.URL.Query().Get("allowed_updates"); got != `["message","chat_join_request"]` {
			t.Errorf("allowed_updates = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true,"result":[
			{"update_id":10,"message":{"message_id":1,"chat":{"id":-1001,"type":"supergroup"},"text":"hola"}},
			{"update_id":11,"chat_join_request":{"user":{"id":42,"first_name":"Juan"},"chat":{"id":-1001,"type":"supergroup"},"date":1700000000}}
		]}`))
	})
	defer srv.Close()

	adapter := NewAdapter("123456:test", WithBaseURL(srv.URL))

	updates, err := adapter.GetUpdates(context.Background(), 10, 30, []string{"message", "chat_join_request"})
	if err != nil {
		t.Fatalf("GetUpdates() unexpected error: %v", err)
	}
	if len(updates) != 2 {
		t.Fatalf("len(updates)=%d, want 2", len(updates))
	}
	if updates[0].UpdateID != 10 || updates[0].Kind() != "message" {
		t.Errorf("first update = %+v (%s), want id 10 kind message", updates[0], updates[0].Kind())
	}
	if updates[0].Message.Chat.ID != -1001 || updates[0].Message.Text != "hola" {
		t.Errorf("message fields not decoded: %+v", updates[0].Message)
	}
	if updates[1].Kind() != "chat_join_request" {
		t.Errorf("second update kind = %s, want chat_join_request", updates[1].Kind())
	}
	if updates[1].ChatJoinRequest.User.ID != 42 {
		t.Errorf("join request user id = %d, want 42", updates[1].ChatJoinRequest.User.ID)
	}
}

func TestAdapterGetUpdates_Conflict(t *testing.T) {
	srv, _ := newBotStubServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"ok":false,"error_code":409,"description":"Conflict: terminated by other getUpdates request"}`))
	})
	defer srv.Close()

	adapter := NewAdapter("123456:test", WithBaseURL(srv.URL))

	_, err := adapter.GetUpdates(context.Background(), 0, 30, nil)
	if !errors.Is(err, ErrWebhookConflict) {
		t.Fatalf("GetUpdates() error = %v, want ErrWebhookConflict", err)
	}
}

func TestAdapterGetUpdates_RateLimited(t *testing.T) {
	srv, _ := newBotStubServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"ok":false,"error_code":429,"description":"Too Many Requests","parameters":{"retry_after":2}}`))
	})
	defer srv.Close()

	adapter := NewAdapter("123456:test", WithBaseURL(srv.URL))

	_, err := adapter.GetUpdates(context.Background(), 0, 30, nil)
	var rateErr *RateLimitError
	if !errors.As(err, &rateErr) {
		t.Fatalf("GetUpdates() error = %v, want *RateLimitError", err)
	}
	if rateErr.RetryAfter != 2*time.Second {
		t.Errorf("RetryAfter = %v, want 2s", rateErr.RetryAfter)
	}
}

func TestAdapterSetWebhook_Success(t *testing.T) {
	srv, paths := newBotStubServer(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/setWebhook") {
			t.Errorf("path = %s, want /setWebhook", r.URL.Path)
		}
		q := r.URL.Query()
		if got := q.Get("url"); got != "https://example.com/webhook" {
			t.Errorf("url = %q", got)
		}
		if got := q.Get("secret_token"); got != "mi-secret" {
			t.Errorf("secret_token = %q", got)
		}
		if got := q.Get("drop_pending_updates"); got != "true" {
			t.Errorf("drop_pending_updates = %q, want true", got)
		}
		if got := q.Get("allowed_updates"); got == "" {
			t.Error("allowed_updates no enviado")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
	})
	defer srv.Close()

	adapter := NewAdapter("123456:test", WithBaseURL(srv.URL))

	err := adapter.SetWebhook(context.Background(), "https://example.com/webhook", "mi-secret", []string{"message"})
	if err != nil {
		t.Fatalf("SetWebhook() unexpected error: %v", err)
	}
	if len(*paths) != 1 {
		t.Errorf("requests = %v, want 1", *paths)
	}
}

func TestAdapterDeleteWebhook_Success(t *testing.T) {
	srv, paths := newBotStubServer(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/deleteWebhook") {
			t.Errorf("path = %s, want /deleteWebhook", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
	})
	defer srv.Close()

	adapter := NewAdapter("123456:test", WithBaseURL(srv.URL))

	if err := adapter.DeleteWebhook(context.Background()); err != nil {
		t.Fatalf("DeleteWebhook() unexpected error: %v", err)
	}
	if len(*paths) != 1 {
		t.Errorf("requests = %v, want 1", *paths)
	}
}
