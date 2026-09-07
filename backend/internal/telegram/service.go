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

// Service es la interfaz que el resto del backend consume. Incluye el
// ciclo de vida del webhook/polling y las acciones administrativas de
// moderacion (AGENTS.md §15); el Adapter aplica el rate limit y el
// mapeo de errores de la Bot API a errores de dominio.
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

	// BanUser banea a userID del chat. untilDate==0 banea por tiempo
	// indefinido; revokeMessages borra tambien sus mensajes.
	BanUser(ctx context.Context, chatID, userID int64, untilDate int64, revokeMessages bool) error
	// UnbanUser desbanea a userID (only_if_banned=true: no falla si no
	// estaba baneado).
	UnbanUser(ctx context.Context, chatID, userID int64) error
	// MuteUser restringe el envio de mensajes de userID.
	// untilDate==0 restringe por tiempo indefinido.
	MuteUser(ctx context.Context, chatID, userID int64, untilDate int64) error
	// UnmuteUser restaura el envio de mensajes de userID.
	UnmuteUser(ctx context.Context, chatID, userID int64) error
	// DeleteMessage borra un mensaje del chat.
	DeleteMessage(ctx context.Context, chatID, messageID int64) error
	// PinMessage fija un mensaje del chat.
	PinMessage(ctx context.Context, chatID, messageID int64) error
	// LockGroup cierra el envio de mensajes para todos los no-admin.
	LockGroup(ctx context.Context, chatID int64) error
	// UnlockGroup reabre el envio de mensajes con permisos estandar.
	UnlockGroup(ctx context.Context, chatID int64) error
	// ApproveJoinRequest aprueba una solicitud de ingreso.
	ApproveJoinRequest(ctx context.Context, chatID, userID int64) error
	// RejectJoinRequest rechaza una solicitud de ingreso.
	RejectJoinRequest(ctx context.Context, chatID, userID int64) error
	// GetChatMember devuelve el estado de un miembro del chat.
	GetChatMember(ctx context.Context, chatID, userID int64) (ChatMember, error)
	// GetChatAdministrators lista los administradores del chat (la Bot
	// API NO permite listar todos los miembros).
	GetChatAdministrators(ctx context.Context, chatID int64) ([]ChatMember, error)
	// SendMessage envia texto a chatID y devuelve el message_id de
	// Telegram (publicaciones, Fase 2). El parametro `keyboard` es
	// opcional: si es no-nil se serializa como `reply_markup`; si es
	// nil se omite del payload (`omitempty`).
	SendMessage(ctx context.Context, chatID int64, text string, disableWebPagePreview bool, keyboard *InlineKeyboardMarkup) (int64, error)
	// SendPhoto envia una foto por URL publica (Telegram la descarga,
	// <= 5 MB) con `caption` opcional (<= 1024 caracteres) y
	// `keyboard` opcional como en SendMessage. Devuelve el message_id.
	SendPhoto(ctx context.Context, chatID int64, photoURL, caption string, keyboard *InlineKeyboardMarkup) (int64, error)
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
	// ErrPermissionDenied se devuelve cuando el bot no tiene el permiso
	// de administrador necesario en el chat (403, y 400 con
	// "not enough rights").
	ErrPermissionDenied = errors.New("telegram: el bot no tiene permisos suficientes")
	// ErrTelegramNotFound se devuelve cuando el chat, mensaje o usuario
	// no existe o ya no esta disponible (404, y 400 "not found").
	ErrTelegramNotFound = errors.New("telegram: chat o mensaje no encontrado")
)

// TelegramAPIError encapsula un error de la Bot API que no tiene un
// error de dominio propio (solo 400 de validacion y codigos inesperados).
type TelegramAPIError struct {
	Code        int
	Description string
}

func (e *TelegramAPIError) Error() string {
	return fmt.Sprintf("telegram: api error %d: %s", e.Code, e.Description)
}

// RateLimitError se devuelve ante 429 Too Many Requests. RetryAfter
// indica cuantos segundos esperar antes de reintentar (la Bot API lo
// manda en el campo parameters.retry_after).
type RateLimitError struct {
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("telegram: rate limited, retry in %s", e.RetryAfter)
}
