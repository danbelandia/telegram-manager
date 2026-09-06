package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/telegram-manager/backend/internal/telegram"
)

func TestWebhook_WithoutSecretIsUnauthorized(t *testing.T) {
	bus := newCapturingBus(make(chan *telegram.Update, 1))
	h := telegramWebhookHandler(bus, "safe-secret")

	req := httptest.NewRequest(http.MethodPost, "/api/telegram/webhook", strings.NewReader(`{"update_id":1}`))
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	select {
	case <-bus.updates:
		t.Error("update published without valid secret")
	default:
	}
}

func TestWebhook_WrongSecretIsUnauthorized(t *testing.T) {
	bus := newCapturingBus(make(chan *telegram.Update, 1))
	h := telegramWebhookHandler(bus, "safe-secret")

	req := httptest.NewRequest(http.MethodPost, "/api/telegram/webhook", strings.NewReader(`{"update_id":1}`))
	req.Header.Set("X-Telegram-Bot-Api-Secret-Token", "wrong-secret")
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	select {
	case <-bus.updates:
		t.Error("update published with wrong secret")
	default:
	}
}

func TestWebhook_ValidSecretPublishes(t *testing.T) {
	bus := newCapturingBus(make(chan *telegram.Update, 1))
	h := telegramWebhookHandler(bus, "safe-secret")

	body := `{"update_id":42,"message":{"message_id":1,"chat":{"id":-1001,"type":"supergroup"},"text":"hola"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/telegram/webhook", strings.NewReader(body))
	req.Header.Set("X-Telegram-Bot-Api-Secret-Token", "safe-secret")
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	select {
	case update := <-bus.updates:
		if update.UpdateID != 42 || update.Kind() != "message" {
			t.Errorf("published update = %+v (%s), want id 42 message", update, update.Kind())
		}
	case <-time.After(time.Second):
		t.Fatal("update not published")
	}
}

func TestWebhook_BadBodyIsBadRequest(t *testing.T) {
	bus := newCapturingBus(make(chan *telegram.Update, 1))
	h := telegramWebhookHandler(bus, "safe-secret")

	req := httptest.NewRequest(http.MethodPost, "/api/telegram/webhook", strings.NewReader(`not json`))
	req.Header.Set("X-Telegram-Bot-Api-Secret-Token", "safe-secret")
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

// capturingBus es el minimo publisher con canal para tests.
type capturingBus struct {
	updates chan *telegram.Update
}

func newCapturingBus(ch chan *telegram.Update) *capturingBus { return &capturingBus{updates: ch} }

func (b *capturingBus) Publish(u *telegram.Update) { b.updates <- u }
