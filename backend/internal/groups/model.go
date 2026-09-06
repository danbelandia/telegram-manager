// Package groups modela los grupos de Telegram que el sistema
// administra: su identidad (desde Telegram), el estado del bot dentro
// del grupo y los permisos disponibles.
package groups

import (
	"errors"
	"time"
)

// BotStatus es el estado del bot dentro del grupo, segun el campo
// status que Telegram devuelve para el propio bot (my_chat_member).
type BotStatus string

// Estados posibles del bot en un grupo (ChatMember.status de la Bot
// API). El default para un grupo persistido es member.
const (
	StatusAdministrator BotStatus = "administrator"
	StatusCreator       BotStatus = "creator"
	StatusMember        BotStatus = "member"
	StatusRestricted    BotStatus = "restricted"
	StatusLeft          BotStatus = "left"
	StatusKicked        BotStatus = "kicked"
)

// Group es un grupo de Telegram administrable.
//
// Los campos *Username y *MemberCount son nil cuando Telegram no los
// expone (grupo sin username público, o membresía no consultable).
// BotPermissions es nil cuando se desconoce (se puebla desde los
// eventos my_chat_member en el paso 8); cuando hay datos es un mapa
// de la familia can_* a booleano.
type Group struct {
	ID             int64
	TelegramID     int64
	Title          string
	Username       *string
	Type           string
	MemberCount    *int64
	BotStatus      BotStatus
	BotPermissions map[string]bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// ErrNotFound se devuelve cuando un telegram_id no existe en la tabla.
var ErrNotFound = errors.New("groups: not found")
