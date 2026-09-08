package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Repository persiste admins en PostgreSQL (tabla creada en 00001).
// Concreto, sin interfaz: mismo patron que internal/groups.
type Repository struct {
	db *sql.DB
}

// NewRepository construye el repositorio sobre un pool existente.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// GetByUsername devuelve el admin con ese username, o ErrNotFound si
// no existe.
func (r *Repository) GetByUsername(ctx context.Context, username string) (Admin, error) {
	return r.get(ctx, "username = $1", username)
}

// GetByID devuelve el admin con ese id, o ErrNotFound si no existe.
func (r *Repository) GetByID(ctx context.Context, id int64) (Admin, error) {
	return r.get(ctx, "id = $1", id)
}

func (r *Repository) get(ctx context.Context, where string, arg any) (Admin, error) {
	const base = `
SELECT id, username, password_hash, tenant_id, created_at, last_login_at
FROM admins
WHERE `

	var a Admin

	q := base + where
	row := r.db.QueryRowContext(ctx, q, arg)

	var lastLogin sql.NullTime
	var tenantID sql.NullInt64
	err := row.Scan(&a.ID, &a.Username, &a.PasswordHash, &tenantID, &a.CreatedAt, &lastLogin)
	if errors.Is(err, sql.ErrNoRows) {
		return Admin{}, ErrNotFound
	}
	if err != nil {
		return Admin{}, fmt.Errorf("auth: get admin: %w", err)
	}
	// tenant_id NULL solo en filas pre-00009 si la migracion no corrio:
	// el middleware trata TenantID==0 como sesion legacy (401 re-login).
	if tenantID.Valid {
		a.TenantID = tenantID.Int64
	}
	if lastLogin.Valid {
		t := lastLogin.Time
		a.LastLoginAt = &t
	}
	return a, nil
}

// UpdateLastLogin registra la fecha del ultimo login exitoso.
func (r *Repository) UpdateLastLogin(ctx context.Context, id int64, at time.Time) error {
	const q = `UPDATE admins SET last_login_at = $2 WHERE id = $1`

	if _, err := r.db.ExecContext(ctx, q, id, at); err != nil {
		return fmt.Errorf("auth: update last login: %w", err)
	}
	return nil
}

// Create inserta un admin con su hash bcrypt ya calculado y su tenant.
// Se usa en el bootstrap (seed) del primer admin y en el signup.
func (r *Repository) Create(ctx context.Context, username, passwordHash string, tenantID int64) (int64, error) {
	const q = `
INSERT INTO admins (username, password_hash, tenant_id)
VALUES ($1, $2, $3)
RETURNING id`

	var id int64
	if err := r.db.QueryRowContext(ctx, q, username, passwordHash, tenantID).Scan(&id); err != nil {
		return 0, fmt.Errorf("auth: create: %w", err)
	}
	return id, nil
}

// Count devuelve cuantos admins existen (para el seed).
func (r *Repository) Count(ctx context.Context) (int, error) {
	const q = `SELECT count(*) FROM admins`

	var n int
	if err := r.db.QueryRowContext(ctx, q).Scan(&n); err != nil {
		return 0, fmt.Errorf("auth: count: %w", err)
	}
	return n, nil
}
