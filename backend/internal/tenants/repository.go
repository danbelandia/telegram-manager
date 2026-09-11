package tenants

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

// Repository persiste tenants en PostgreSQL (tabla de la 00009).
// Concreto, sin interfaz: mismo patron que internal/groups.
type Repository struct {
	db *sql.DB
}

// NewRepository construye el repositorio sobre un pool existente.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Create inserta un tenant con su token ya cifrado y el username del
// bot (de getMe). El slug duplicado se mapea a ErrSlugTaken
// (signup → 409 CONFLICT).
func (r *Repository) Create(ctx context.Context, slug, tier string, enc []byte, botUsername *string) (int64, error) {
	const q = `
INSERT INTO tenants (slug, tier, bot_token_encrypted, bot_username)
VALUES ($1, $2, $3, $4)
RETURNING id`

	var id int64
	if err := r.db.QueryRowContext(ctx, q, slug, tier, enc, botUsername).Scan(&id); err != nil {
		if isUniqueViolation(err) {
			return 0, ErrSlugTaken
		}
		return 0, fmt.Errorf("tenants: create %q: %w", slug, err)
	}
	return id, nil
}

// GetByID devuelve el tenant con ese id, o ErrNotFound.
func (r *Repository) GetByID(ctx context.Context, id int64) (*Tenant, error) {
	const q = `
SELECT id, slug, tier, bot_token_encrypted, bot_username, status,
       trial_ends_at, expires_at, plan, max_groups, max_messages_day,
       created_at, updated_at
FROM tenants
WHERE id = $1`

	t, err := scanTenant(r.db.QueryRowContext(ctx, q, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("tenants: get %d: %w", id, err)
	}
	return &t, nil
}

// GetBySlug devuelve el tenant con ese slug, o ErrNotFound.
func (r *Repository) GetBySlug(ctx context.Context, slug string) (*Tenant, error) {
	const q = `
SELECT id, slug, tier, bot_token_encrypted, bot_username, status,
       trial_ends_at, expires_at, plan, max_groups, max_messages_day,
       created_at, updated_at
FROM tenants
WHERE slug = $1`

	t, err := scanTenant(r.db.QueryRowContext(ctx, q, slug))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("tenants: get slug %q: %w", slug, err)
	}
	return &t, nil
}

// ListWithTokens devuelve los tenants con token registrado (para el
// boot del registry: solo esos levantan poller). Orden estable por id.
func (r *Repository) ListWithTokens(ctx context.Context) ([]Tenant, error) {
	const q = `
SELECT id, slug, tier, bot_token_encrypted, bot_username, status,
       trial_ends_at, expires_at, plan, max_groups, max_messages_day,
       created_at, updated_at
FROM tenants
WHERE bot_token_encrypted IS NOT NULL
ORDER BY id ASC`

	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("tenants: list with tokens: %w", err)
	}
	defer rows.Close()

	out := make([]Tenant, 0)
	for rows.Next() {
		t, err := scanTenant(rows)
		if err != nil {
			return nil, fmt.Errorf("tenants: list with tokens: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("tenants: list with tokens: %w", err)
	}
	return out, nil
}

// ListAll devuelve todos los tenants (panel super-admin). Ordenado por id.
func (r *Repository) ListAll(ctx context.Context) ([]Tenant, error) {
	const q = `
SELECT id, slug, tier, bot_token_encrypted, bot_username, status,
       trial_ends_at, expires_at, plan, max_groups, max_messages_day,
       created_at, updated_at
FROM tenants
ORDER BY id ASC`

	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("tenants: list all: %w", err)
	}
	defer rows.Close()

	out := make([]Tenant, 0)
	for rows.Next() {
		t, err := scanTenant(rows)
		if err != nil {
			return nil, fmt.Errorf("tenants: list all: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("tenants: list all: %w", err)
	}
	return out, nil
}

// TenantLicense es la vista liviana de licencia para enforcement.
type TenantLicense struct {
	ID          int64
	Status      string
	TrialEndsAt *time.Time
	ExpiresAt   *time.Time
}

// GetLicense devuelve solo los campos de licencia de un tenant.
// Más liviano que GetByID (sin bot_token_encrypted).
func (r *Repository) GetLicense(ctx context.Context, id int64) (*TenantLicense, error) {
	const q = `
SELECT id, status, trial_ends_at, expires_at
FROM tenants WHERE id = $1`

	var tl TenantLicense
	var trialEndsAt, expiresAt sql.NullTime
	err := r.db.QueryRowContext(ctx, q, id).Scan(
		&tl.ID, &tl.Status, &trialEndsAt, &expiresAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("tenants: get license %d: %w", id, err)
	}
	if trialEndsAt.Valid {
		tl.TrialEndsAt = &trialEndsAt.Time
	}
	if expiresAt.Valid {
		tl.ExpiresAt = &expiresAt.Time
	}
	return &tl, nil
}

// UpdateLicense aplica un patch parcial a los campos de licencia de un
// tenant. Solo los campos no-nil en in se modifican. updated_at siempre
// se actualiza.
func (r *Repository) UpdateLicense(ctx context.Context, id int64, in LicenseUpdate) error {
	setClauses := []string{}
	args := []any{}
	argIdx := 1

	if in.Status != nil {
		setClauses = append(setClauses, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, *in.Status)
		argIdx++
	}
	if in.Plan != nil {
		setClauses = append(setClauses, fmt.Sprintf("plan = $%d", argIdx))
		args = append(args, *in.Plan)
		argIdx++
	}
	if in.TrialEndsAt != nil {
		setClauses = append(setClauses, fmt.Sprintf("trial_ends_at = $%d", argIdx))
		args = append(args, *in.TrialEndsAt)
		argIdx++
	}
	if in.ExpiresAt != nil {
		setClauses = append(setClauses, fmt.Sprintf("expires_at = $%d", argIdx))
		args = append(args, *in.ExpiresAt)
		argIdx++
	}
	if in.MaxGroups != nil {
		setClauses = append(setClauses, fmt.Sprintf("max_groups = $%d", argIdx))
		args = append(args, *in.MaxGroups)
		argIdx++
	}
	if in.MaxMessagesDay != nil {
		setClauses = append(setClauses, fmt.Sprintf("max_messages_day = $%d", argIdx))
		args = append(args, *in.MaxMessagesDay)
		argIdx++
	}

	if len(setClauses) == 0 {
		return nil // nothing to update
	}

	// Siempre actualizar updated_at.
	setClauses = append(setClauses, "updated_at = now()")

	q := fmt.Sprintf("UPDATE tenants SET %s WHERE id = $%d", strings.Join(setClauses, ", "), argIdx)
	args = append(args, id)

	if _, err := r.db.ExecContext(ctx, q, args...); err != nil {
		return fmt.Errorf("tenants: update license %d: %w", id, err)
	}
	return nil
}

// EnsureDefault crea el tenant `default` si no existe y devuelve su
// id (idempotente: el UNIQUE de slug evita duplicados en carrera).
// Es el hogar del path legacy (TELEGRAM_BOT_TOKEN + ADMIN_*).
func (r *Repository) EnsureDefault(ctx context.Context) (int64, error) {
	const q = `
INSERT INTO tenants (slug)
VALUES ('default')
ON CONFLICT (slug) DO UPDATE SET slug = EXCLUDED.slug
RETURNING id`

	var id int64
	if err := r.db.QueryRowContext(ctx, q).Scan(&id); err != nil {
		return 0, fmt.Errorf("tenants: ensure default: %w", err)
	}
	return id, nil
}

// SetStatus marca el estado del tenant. Lo usa el registry ante tokens
// revocados. Nota: el CHECK de DB solo acepta estados de licencia
// (trial|active|suspended|expired). El registry puede persistir
// 'active' cuando el token vuelve a ser valido, pero 'degraded' NO se
// escribe a DB (es estado de runtime interno del registry).
func (r *Repository) SetStatus(ctx context.Context, id int64, status string) error {
	const q = `UPDATE tenants SET status = $2, updated_at = now() WHERE id = $1`

	if _, err := r.db.ExecContext(ctx, q, id, status); err != nil {
		return fmt.Errorf("tenants: set status %d: %w", id, err)
	}
	return nil
}

// UpdateToken cifra y persiste un nuevo bot_token + username. Lo usa la
// rotacion de token (PUT /tenants/me/bot-token). Nunca loguea el token.
func (r *Repository) UpdateToken(ctx context.Context, id int64, enc []byte, botUsername *string) error {
	const q = `UPDATE tenants SET bot_token_encrypted = $2, bot_username = $3, updated_at = now() WHERE id = $1`

	if _, err := r.db.ExecContext(ctx, q, id, enc, botUsername); err != nil {
		return fmt.Errorf("tenants: update token %d: %w", id, err)
	}
	return nil
}

// rowScanner es la vista minima de fila que el scanner necesita;
// *sql.Rows y *sql.Row la satisfacen.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanTenant(row rowScanner) (Tenant, error) {
	var (
		t              Tenant
		enc            []byte
		username       sql.NullString
		trialEndsAt    sql.NullTime
		expiresAt      sql.NullTime
		createdAt      sql.NullTime
		updatedAt      sql.NullTime
	)
	// BYTEA NULL → enc nil; el caller decide (nil = tenant legacy).
	err := row.Scan(
		&t.ID, &t.Slug, &t.Tier, &enc, &username, &t.Status,
		&trialEndsAt, &expiresAt, &t.Plan, &t.MaxGroups, &t.MaxMessagesDay,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return Tenant{}, err
	}
	t.BotTokenEncrypted = enc
	if username.Valid {
		t.BotUsername = &username.String
	}
	if trialEndsAt.Valid {
		t.TrialEndsAt = &trialEndsAt.Time
	}
	if expiresAt.Valid {
		t.ExpiresAt = &expiresAt.Time
	}
	if createdAt.Valid {
		t.CreatedAt = createdAt.Time
	}
	if updatedAt.Valid {
		t.UpdatedAt = updatedAt.Time
	}
	return t, nil
}

// isUniqueViolation detecta 23505 (unique_violation) bajo el driver
// pgx stdlib.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
