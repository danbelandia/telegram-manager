// Package users modela los usuarios de Telegram que el sistema
// registra (eventos y acciones). Identidad desde Telegram: telegram_id
// es el id real del usuario, unico en la tabla. Con el MVP solo se usa
// para persistir el autor de solicitudes de ingreso y como referencia
// para logs (sin FK: Telegram es la fuente de verdad, design D6).
package users

import (
	"errors"
	"time"
)

// User es un usuario de Telegram visto por el sistema.
type User struct {
	ID         int64
	TelegramID int64
	FirstName  string
	Username   *string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// ErrNotFound se devuelve cuando un telegram_id no existe en la tabla.
var ErrNotFound = errors.New("users: not found")
