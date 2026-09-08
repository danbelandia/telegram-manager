package users

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Repository persiste usuarios en PostgreSQL (patron del dominio
// groups: scanner propio, los consumidores reciben structs del dominio).
type Repository struct {
	db *sql.DB
}

// NewRepository construye el repositorio sobre un pool existente.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// UpsertByTelegramID inserta el usuario o actualiza la fila existente
// (mismo telegram_id), renovando updated_at. Idempotente: el nombre de
// usuario/publicacion se refresca con cada evento.
func (r *Repository) UpsertByTelegramID(ctx context.Context, u *User) error {
	const q = `
INSERT INTO users (telegram_id, first_name, username)
VALUES ($1, $2, $3)
ON CONFLICT (telegram_id) DO UPDATE SET
    first_name = EXCLUDED.first_name,
    username   = EXCLUDED.username,
    updated_at = now()`

	_, err := r.db.ExecContext(ctx, q, u.TelegramID, u.FirstName, u.Username)
	if err != nil {
		return fmt.Errorf("users: upsert %d: %w", u.TelegramID, err)
	}
	return nil
}

// GetByTelegramID devuelve el usuario con ese id de Telegram, o
// ErrNotFound si no existe.
func (r *Repository) GetByTelegramID(ctx context.Context, telegramID int64) (*User, error) {
	const q = `
SELECT id, telegram_id, first_name, username, created_at, updated_at
FROM users
WHERE telegram_id = $1`

	var (
		u         User
		createdAt time.Time
		updatedAt time.Time
	)
	err := r.db.QueryRowContext(ctx, q, telegramID).Scan(
		&u.ID, &u.TelegramID, &u.FirstName, &u.Username, &createdAt, &updatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("users: get %d: %w", telegramID, err)
	}
	u.CreatedAt = createdAt
	u.UpdatedAt = updatedAt
	return &u, nil
}

// GetByTenant devuelve el usuario solo si tiene presencia en el tenant
// (slice 0, Q3-a: users SIN tenant_id; el alcance se resuelve via
// join con las tablas hijas del tenant). Usuario visible solo en
// grupos de otro tenant → ErrNotFound. Equivalente al EXISTS del
// design §5, extendido a warnings (un usuario advertido tambien es
// presencia aunque nunca pidiera ingreso).
func (r *Repository) GetByTenant(ctx context.Context, tenantID, telegramID int64) (*User, error) {
	const q = `
SELECT u.id, u.telegram_id, u.first_name, u.username, u.created_at, u.updated_at
FROM users u
WHERE u.telegram_id = $2
AND (
    EXISTS (SELECT 1 FROM join_requests j WHERE j.tenant_id = $1 AND j.user_id = u.telegram_id)
    OR EXISTS (SELECT 1 FROM warnings w WHERE w.tenant_id = $1 AND w.user_id = u.telegram_id)
)`

	var (
		u         User
		createdAt time.Time
		updatedAt time.Time
	)
	err := r.db.QueryRowContext(ctx, q, tenantID, telegramID).Scan(
		&u.ID, &u.TelegramID, &u.FirstName, &u.Username, &createdAt, &updatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("users: get in tenant %d: %w", telegramID, err)
	}
	u.CreatedAt = createdAt
	u.UpdatedAt = updatedAt
	return &u, nil
}
