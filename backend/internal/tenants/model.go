// Package tenants modela el multitenancy bot-per-tenant (slice 0):
// un tenant = un usuario = un bot, tier Pro unico. Concentra modelo,
// repositorio y cifrado AES-GCM del token del bot.
package tenants

import (
	"errors"
	"time"
)

// Tenant es un inquilino del sistema: su slug lo identifica en el
// signup, el token cifrado permite levantar su poller al boot y el
// status marca degradacion ante tokens revocados.
type Tenant struct {
	ID                int64
	Slug              string
	Tier              string
	BotTokenEncrypted []byte
	BotUsername       *string
	Status            string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// Estados del tenant.
const (
	StatusActive   = "active"
	StatusDegraded = "degraded"
)

// Tier unico del slice 0.
const TierPro = "pro"

var (
	// ErrNotFound: el tenant no existe.
	ErrNotFound = errors.New("tenants: not found")
	// ErrSlugTaken: el slug ya esta registrado (signup → 409 CONFLICT).
	ErrSlugTaken = errors.New("tenants: slug already taken")
)
