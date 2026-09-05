// Package telegram es el unico punto de contacto con la Bot API de
// Telegram. Consumidores dependen de la interfaz Service; la
// implementacion concreta (Adapter) vive en este mismo paquete y es la
// unica que construye URLs de la Bot API.
package telegram

import (
	"context"
	"errors"
)

// Service es la interfaz que el resto del backend consume.
type Service interface {
	// GetMe devuelve la identidad del bot para el token configurado.
	GetMe(ctx context.Context) (BotUser, error)
}

var (
	// ErrInvalidToken se devuelve cuando Telegram rechaza el token (401).
	ErrInvalidToken = errors.New("telegram: invalid token")
	// ErrTelegramUnavailable se devuelve cuando Telegram no respondio
	// (red, timeout, etc).
	ErrTelegramUnavailable = errors.New("telegram: unavailable")
)
