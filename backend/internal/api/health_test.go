package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// stubPinger simula la base de datos del health endpoint.
type stubPinger struct{ err error }

func (s stubPinger) PingContext(ctx context.Context) error { return s.err }

// stubBotStatus simula el estado cacheado del bot.
type stubBotStatus struct {
	connected bool
	username  string
}

func (s stubBotStatus) Status() (bool, string) { return s.connected, s.username }

func TestHealth_AllOk(t *testing.T) {
	handler := NewServer(
		stubPinger{err: nil},
		stubBotStatus{connected: true, username: "admin_bot"},
	)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/health", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var body struct {
		Data struct {
			Status       string `json:"status"`
			DB           string `json:"db"`
			BotConnected bool   `json:"bot_connected"`
			BotUsername  string `json:"bot_username"`
		} `json:"data"`
		Error any `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Data.Status != "ok" {
		t.Errorf("status = %q, want ok", body.Data.Status)
	}
	if body.Data.DB != "ok" {
		t.Errorf("db = %q, want ok", body.Data.DB)
	}
	if !body.Data.BotConnected {
		t.Error("bot_connected = false, want true")
	}
	if body.Data.BotUsername != "admin_bot" {
		t.Errorf("bot_username = %q, want admin_bot", body.Data.BotUsername)
	}
	if body.Error != nil {
		t.Errorf("error = %v, want null", body.Error)
	}
}

func TestHealth_DatabaseDown(t *testing.T) {
	handler := NewServer(
		stubPinger{err: errors.New("connection refused")},
		stubBotStatus{connected: true, username: "admin_bot"},
	)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/health", nil))

	var body struct {
		Data struct {
			Status string `json:"status"`
			DB     string `json:"db"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Data.DB != "error" {
		t.Errorf("db = %q, want error", body.Data.DB)
	}
	if body.Data.Status == "ok" {
		t.Error("status = ok, want degraded when db is down")
	}
}

func TestHealth_BotDisconnected(t *testing.T) {
	handler := NewServer(
		stubPinger{err: nil},
		stubBotStatus{connected: false, username: ""},
	)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/health", nil))

	var body struct {
		Data struct {
			Status       string `json:"status"`
			BotConnected bool   `json:"bot_connected"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Data.BotConnected {
		t.Error("bot_connected = true, want false")
	}
	if body.Data.Status == "ok" {
		t.Error("status = ok, want degraded when bot disconnected")
	}
}

func TestHealth_NoSecretsInResponse(t *testing.T) {
	handler := NewServer(
		stubPinger{err: nil},
		stubBotStatus{connected: true, username: "admin_bot"},
	)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/health", nil))

	for _, secret := range []string{"postgres://", "password", "token", "123456:test"} {
		if strings.Contains(rec.Body.String(), secret) {
			t.Errorf("response contains forbidden substring %q", secret)
		}
	}
}
