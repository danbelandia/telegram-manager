// Package telegram es el unico punto de contacto con la Bot API de
// Telegram. Consumidores dependen de la interfaz Service; la
// implementacion concreta (Adapter) vive en este mismo paquete y es la
// unica que construye URLs de la Bot API.
package telegram

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Service es la interfaz que el resto del backend consume.
type Service interface {
	// GetMe devuelve la identidad del bot para el token configurado.
	GetMe(ctx context.Context) (BotUser, error)
	// GetUpdates pide los updates pendientes (long polling). offset es
	// el primer update a confirmar; timeout es el long poll timeout.
	GetUpdates(ctx context.Context, offset, timeout int, allowed []string) ([]Update, error)
	// SetWebhook registra la URL de produccion. En el MVP se usa con
	// drop_pending_updates para evitar re-procesar updates viejos.
	SetWebhook(ctx context.Context, webhookURL, secret string, allowed []string) error
	// DeleteWebhook desregistra el webhook actual.
	DeleteWebhook(ctx context.Context) error
}

var (
	// ErrInvalidToken se devuelve cuando Telegram rechaza el token (401).
	ErrInvalidToken = errors.New("telegram: invalid token")
	// ErrTelegramUnavailable se devuelve cuando Telegram no respondio
	// (red, timeout, etc).
	ErrTelegramUnavailable = errors.New("telegram: unavailable")
	// ErrWebhookConflict se devuelve cuando hay un webhook activo y se
	// intenta usar getUpdates (409). Con 409 el bot no puede hacer
	// polling hasta que el webhook se elimine.
	ErrWebhookConflict = errors.New("telegram: webhook activo; usar DeleteWebhook")
)

// RateLimitError se devuelve ante 429 Too Many Requests. RetryAfter
// indica cuantos segundos esperar antes de reintentar (la Bot API lo
// manda en el campo parameters.retry_after).
type RateLimitError struct {
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("telegram: rate limited, retry in %s", e.RetryAfter)
}
