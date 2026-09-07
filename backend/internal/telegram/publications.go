package telegram

import (
	"context"
)

// Acciones de publicaciones: SendMessage (slice 1, publish-now text).
// Sigue el mismo patron que moderation.go: params struct privado +
// doWithRetry → doPost → handleEnvelope. La programacion (slice 3) no
// usa la Bot API: NO existe schedule_date para bots en grupos
// (exploration; se hace in-process con worker Go).

// sendMessageParams corresponde a sendMessage de la Bot API
// (docs/telegram_api_reference.md §sendMessage). disable_web_page_preview
// lleva omitempty: el default de Telegram es false, omitirlo mantiene
// el comportamiento estandar.
type sendMessageParams struct {
	ChatID                int64  `json:"chat_id"`
	Text                  string `json:"text"`
	DisableWebPagePreview bool   `json:"disable_web_page_preview,omitempty"`
}

// sendMessageResult es la vista minima de la respuesta de sendMessage:
// solo interesa message_id (design D1). El resto del Message (from,
// chat, text) no se necesita aqui.
type sendMessageResult struct {
	MessageID int64 `json:"message_id"`
}

// SendMessage envia texto a chatID y devuelve el message_id asignado
// por Telegram. Pasa por doWithRetry (429 con retry_after, max 3
// reintentos) y el rate limiter token bucket del Adapter (§18.1).
func (a *Adapter) SendMessage(ctx context.Context, chatID int64, text string, disableWebPagePreview bool) (int64, error) {
	var result sendMessageResult
	err := a.doWithRetry(ctx, func() error {
		return a.doPost(ctx, "sendMessage", sendMessageParams{
			ChatID:                chatID,
			Text:                  text,
			DisableWebPagePreview: disableWebPagePreview,
		}, &result)
	})
	if err != nil {
		return 0, err
	}
	return result.MessageID, nil
}
