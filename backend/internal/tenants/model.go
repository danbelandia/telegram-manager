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
// status marca el estado de la licencia.
type Tenant struct {
	ID                int64
	Slug              string
	Tier              string
	BotTokenEncrypted []byte
	BotUsername       *string
	Status            string
	TrialEndsAt       *time.Time
	ExpiresAt         *time.Time
	Plan              string
	MaxGroups         int
	MaxMessagesDay    int
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// Estados de licencia del tenant (migration 00010).
const (
	StatusTrial     = "trial"
	StatusActive    = "active"
	StatusSuspended = "suspended"
	StatusExpired   = "expired"

	// StatusDegraded es un estado de runtime del registry (bot token
	// revocado). NO es un estado de licencia valido en DB; se conserva
	// como constante para que telegram/registry.go compile.
	StatusDegraded = "degraded"
)

// Tier unico del slice 0.
const TierPro = "pro"

// LicenseUpdate es el set de campos que un super-admin puede modificar
// via PUT /api/admin/tenants/:id. Punteros nil = no modificar.
type LicenseUpdate struct {
	Status         *string
	Plan           *string
	TrialEndsAt    *time.Time
	ExpiresAt      *time.Time
	MaxGroups      *int
	MaxMessagesDay *int
}

var (
	// ErrNotFound: el tenant no existe.
	ErrNotFound = errors.New("tenants: not found")
	// ErrSlugTaken: el slug ya esta registrado (signup → 409 CONFLICT).
	ErrSlugTaken = errors.New("tenants: slug already taken")
)
