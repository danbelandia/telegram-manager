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
// no existe. JOIN tenants para traer el slug (para JWT claims).
func (r *Repository) GetByUsername(ctx context.Context, username string) (Admin, error) {
	return r.get(ctx, "a.username = $1", username)
}

// GetByID devuelve el admin con ese id, o ErrNotFound si no existe.
func (r *Repository) GetByID(ctx context.Context, id int64) (Admin, error) {
	return r.get(ctx, "a.id = $1", id)
}

func (r *Repository) get(ctx context.Context, where string, arg any) (Admin, error) {
	const base = `
SELECT a.id, a.username, a.password_hash, a.tenant_id, a.is_super_admin,
       a.created_at, a.last_login_at, t.slug
FROM admins a
JOIN tenants t ON t.id = a.tenant_id
WHERE `

	var a Admin

	q := base + where
	row := r.db.QueryRowContext(ctx, q, arg)

	var lastLogin sql.NullTime
	var tenantID sql.NullInt64
	err := row.Scan(&a.ID, &a.Username, &a.PasswordHash, &tenantID,
		&a.IsSuperAdmin, &a.CreatedAt, &lastLogin, &a.TenantSlug)
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

// HasSuperAdmin devuelve true si al menos un admin tiene is_super_admin = true.
func (r *Repository) HasSuperAdmin(ctx context.Context) (bool, error) {
	const q = `SELECT EXISTS(SELECT 1 FROM admins WHERE is_super_admin = true)`
	var exists bool
	if err := r.db.QueryRowContext(ctx, q).Scan(&exists); err != nil {
		return false, fmt.Errorf("auth: has super admin: %w", err)
	}
	return exists, nil
}
