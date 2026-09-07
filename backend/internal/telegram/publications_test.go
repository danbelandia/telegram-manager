package telegram

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestAdapterSendMessage_DecodesMessageID(t *testing.T) {
	var path string
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		if raw, err := io.ReadAll(r.Body); err == nil {
			_ = json.Unmarshal(raw, &body)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":123,"chat":{"id":-100123}}}`))
	}))
	defer srv.Close()

	adapter := NewAdapter("123456:test", WithBaseURL(srv.URL))

	mid, err := adapter.SendMessage(context.Background(), -100123, "Hola mundo", false)
	if err != nil {
		t.Fatalf("SendMessage() unexpected error: %v", err)
	}
	if mid != 123 {
		t.Errorf("message_id = %d, want 123", mid)
	}
	if !strings.HasSuffix(path, "/sendMessage") {
		t.Errorf("path = %s, want /sendMessage", path)
	}
	if body["chat_id"] != float64(-100123) || body["text"] != "Hola mundo" {
		t.Errorf("body mal: %v", body)
	}
	if body["disable_web_page_preview"] != nil {
		t.Errorf("disable_web_page_preview presente con false, want omitido (valor: %v)", body["disable_web_page_preview"])
	}
}

func TestAdapterSendMessage_429Retries(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"ok":false,"error_code":429,"description":"Too Many Requests","parameters":{"retry_after":2}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":555}}`))
	}))
	defer srv.Close()

	adapter := NewAdapter("123456:test", WithBaseURL(srv.URL))

	mid, err := adapter.SendMessage(context.Background(), -100123, "Hola", false)
	if err != nil {
		t.Fatalf("SendMessage() tras 429: %v", err)
	}
	if mid != 555 {
		t.Errorf("message_id = %d, want 555 (reintento exitoso)", mid)
	}
	if calls.Load() != 2 {
		t.Errorf("calls = %d, want 2 (429 + reintento)", calls.Load())
	}
}
