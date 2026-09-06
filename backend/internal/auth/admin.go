// Package auth implementa la autenticacion del panel (AGENTS.md 17.1):
// login con username+password sobre la tabla admins, access token JWT
// corto (15 min) y refresh token JWT largo (7 dias) en cookie httpOnly.
package auth

import (
	"errors"
	"time"
)

// Admin es un administrador del panel. Su password vive como hash
// bcrypt (nunca en claro); LastLoginAt es nil si nunca se logueo.
type Admin struct {
	ID           int64
	Username     string
	PasswordHash string
	CreatedAt    time.Time
	LastLoginAt  *time.Time
}

// ErrCredentialInvalid se devuelve cuando username o password no
// coinciden. Es el mismo error para ambos casos a proposito: no se
// revela si el usuario existe (evita enumerar admins).
var ErrCredentialInvalid = errors.New("auth: invalid credentials")

// ErrNotFound se devuelve cuando un username no existe en admins.
var ErrNotFound = errors.New("auth: admin not found")
