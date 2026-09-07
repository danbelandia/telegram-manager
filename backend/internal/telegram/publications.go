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
// el comportamiento estandar. ReplyMarkup es opcional; si es nil, se
// omite del payload (slice 2: botones inline).
type sendMessageParams struct {
	ChatID                int64                 `json:"chat_id"`
	Text                  string                `json:"text"`
	DisableWebPagePreview bool                  `json:"disable_web_page_preview,omitempty"`
	ReplyMarkup           *InlineKeyboardMarkup `json:"reply_markup,omitempty"`
}

// sendPhotoParams corresponde a sendPhoto de la Bot API
// (docs/telegram_api_reference.md §sendPhoto). `photo` puede ser un
// file_id, una HTTP URL (nuestro caso: <5MB) o multipart; la caption
// <= 1024 caracteres; `reply_markup` opcional igual que sendMessage.
type sendPhotoParams struct {
	ChatID      int64                 `json:"chat_id"`
	Photo       string                `json:"photo"`
	Caption     string                `json:"caption,omitempty"`
	ReplyMarkup *InlineKeyboardMarkup `json:"reply_markup,omitempty"`
}

// sendMessageResult es la vista minima de la respuesta de sendMessage
// y sendPhoto: solo interesa message_id (design D1). El resto del
// Message (from, chat, text) no se necesita aqui.
type sendMessageResult struct {
	MessageID int64 `json:"message_id"`
}

// SendMessage envia texto a chatID y devuelve el message_id asignado
// por Telegram. Pasa por doWithRetry (429 con retry_after, max 3
// reintentos) y el rate limiter token bucket del Adapter (§18.1).
// `keyboard` es opcional: si es no-nil se envia como `reply_markup`;
// si es nil se omite.
func (a *Adapter) SendMessage(ctx context.Context, chatID int64, text string, disableWebPagePreview bool, keyboard *InlineKeyboardMarkup) (int64, error) {
	var result sendMessageResult
	err := a.doWithRetry(ctx, func() error {
		return a.doPost(ctx, "sendMessage", sendMessageParams{
			ChatID:                chatID,
			Text:                  text,
			DisableWebPagePreview: disableWebPagePreview,
			ReplyMarkup:           keyboard,
		}, &result)
	})
	if err != nil {
		return 0, err
	}
	return result.MessageID, nil
}

// SendPhoto envia una foto por URL publica a chatID con `caption`
// opcional y devuelve el message_id asignado por Telegram. La foto se
// envia por HTTP URL (Telegram descarga, <= 5 MB); no soportamos
// multipart en el MVP. `keyboard` opcional se serializa como
// `reply_markup` igual que en SendMessage. Pasa por doWithRetry y el
// rate limiter token bucket del Adapter.
func (a *Adapter) SendPhoto(ctx context.Context, chatID int64, photoURL, caption string, keyboard *InlineKeyboardMarkup) (int64, error) {
	var result sendMessageResult
	err := a.doWithRetry(ctx, func() error {
		return a.doPost(ctx, "sendPhoto", sendPhotoParams{
			ChatID:      chatID,
			Photo:       photoURL,
			Caption:     caption,
			ReplyMarkup: keyboard,
		}, &result)
	})
	if err != nil {
		return 0, err
	}
	return result.MessageID, nil
}
