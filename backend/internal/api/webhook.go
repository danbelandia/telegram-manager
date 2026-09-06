package api

import (
	"crypto/subtle"
	"encoding/json"
	"io"
	"net/http"

	"github.com/telegram-manager/backend/internal/telegram"
)

// updatePublisher es la minima vista del bus de eventos que el webhook
// necesita. Se declara donde se consume; *events.Bus la satisface.
type updatePublisher interface {
	Publish(*telegram.Update)
}

// telegramWebhookHandler procesa el POST que Telegram hace por cada
// update en modo webhook (AGENTS.md 19.1).
//
// Validacion del secret ANTES de tocar el body: si el header
// X-Telegram-Bot-Api-Secret-Token no coincide, nadie deberia poder
// inyectar eventos falsos. La respuesta es 401/200 vacia (Telegram
// solo lee el status code).
func telegramWebhookHandler(publisher updatePublisher, secret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Telegram-Bot-Api-Secret-Token")), []byte(secret)) != 1 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		var update telegram.Update
		if err := json.Unmarshal(body, &update); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Entregar en goroutine: Telegram espera un 200 rapido y el
		// Bus es sincrono (correria todo el pipeline de handlers).
		go publisher.Publish(&update)
		w.WriteHeader(http.StatusOK)
	}
}
