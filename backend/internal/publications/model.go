// Package publications modela las publicaciones del panel (AGENTS.md
// §22). Slice 1: publish-now text a un grupo. La tabla usa UN solo
// status que cubre todo el ciclo de vida (draft|scheduled|sending|
// sent|failed) y scheduled_at nullable (NULL = inmediato); asi la
// programacion (slice 3) no requiere migracion de esquema.
package publications

import (
	"errors"
	"time"
)

// Status es el estado de una publicacion.
type Status string

// Estados posibles del ciclo de vida de una publicacion.
const (
	StatusDraft     Status = "draft"
	StatusScheduled Status = "scheduled"
	StatusSending   Status = "sending"
	StatusSent      Status = "sent"
	StatusFailed    Status = "failed"
)

// Publication es una publicacion. TelegramID es groups.telegram_id (id
// natural para llamadas a Telegram). MessageID se puebla en exito;
// ErrorMessage en fallo; ScheduledAt es NULL para publicacion inmediata.
// ActorID es el id del admin del panel que la creo (auditoria).
type Publication struct {
	ID           int64
	TelegramID   int64
	Text         string
	Status       Status
	MessageID    *int64
	ScheduledAt  *time.Time
	ErrorMessage *string
	ActorID      *int64
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Errores de dominio del flujo (el handler los mapea a los codigos §18).
var (
	// ErrTextTooLong: el texto excede el limite de la Bot API (4096).
	ErrTextTooLong = errors.New("texto excede 4096 caracteres")
	// ErrTextEmpty: el texto esta vacio (4096 max, 1 min).
	ErrTextEmpty = errors.New("texto no puede estar vacio")
	// ErrNotFound: la publicacion no existe.
	ErrNotFound = errors.New("publications: not found")
)
