package groups

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Repository persiste grupos en PostgreSQL. Es la unica capa que habla
// con la base de datos para el dominio groups; el scanner queda aca y
// los consumidores reciben structs del dominio, nunca filas SQL.
type Repository struct {
	db *sql.DB
}

// NewRepository construye el repositorio sobre un pool existente.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// UpsertByTelegramID inserta el grupo o actualiza la fila existente
// (misma telegram_id), renovando updated_at. Idempotente: nunca crea
// duplicados.
func (r *Repository) UpsertByTelegramID(ctx context.Context, g *Group) error {
	const q = `
INSERT INTO groups (telegram_id, title, username, type, member_count, bot_status, bot_permissions)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (telegram_id) DO UPDATE SET
    title           = EXCLUDED.title,
    username        = EXCLUDED.username,
    type            = EXCLUDED.type,
    member_count    = EXCLUDED.member_count,
    bot_status      = EXCLUDED.bot_status,
    bot_permissions = EXCLUDED.bot_permissions,
    updated_at      = now()`

	perms, err := marshalPermissions(g.BotPermissions)
	if err != nil {
		return fmt.Errorf("groups: upsert %d: %w", g.TelegramID, err)
	}

	_, err = r.db.ExecContext(ctx, q,
		g.TelegramID, g.Title, g.Username, g.Type, g.MemberCount,
		string(g.BotStatus), perms,
	)
	if err != nil {
		return fmt.Errorf("groups: upsert %d: %w", g.TelegramID, err)
	}
	return nil
}

// List devuelve todos los grupos persistidos, ordenados por title
// ascendentemente.
func (r *Repository) List(ctx context.Context) ([]Group, error) {
	const q = `
SELECT id, telegram_id, title, username, type, member_count, bot_status, bot_permissions, created_at, updated_at
FROM groups
ORDER BY title ASC`

	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("groups: list: %w", err)
	}
	defer rows.Close()

	groups := make([]Group, 0)
	for rows.Next() {
		g, err := scanGroup(rows)
		if err != nil {
			return nil, fmt.Errorf("groups: list: %w", err)
		}
		groups = append(groups, g)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("groups: list: %w", err)
	}
	return groups, nil
}

// GetByTelegramID devuelve el grupo con ese id de Telegram, o
// ErrNotFound si no existe.
func (r *Repository) GetByTelegramID(ctx context.Context, telegramID int64) (*Group, error) {
	const q = `
SELECT id, telegram_id, title, username, type, member_count, bot_status, bot_permissions, created_at, updated_at
FROM groups
WHERE telegram_id = $1`

	g, err := scanGroup(r.db.QueryRowContext(ctx, q, telegramID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("groups: get %d: %w", telegramID, err)
	}
	return &g, nil
}

// rowScanner es la vista minima de fila que el scanner necesita;
// *sql.Rows y *sql.Row la satisfacen.
type rowScanner interface {
	Scan(dest ...any) error
}

// scanGroup mapea una fila de groups al struct Group. Permisos JSONB
// NULL → nil; username/member_count NULL → nil.
func scanGroup(row rowScanner) (Group, error) {
	var (
		g         Group
		permsJSON []byte
		createdAt time.Time
		updatedAt time.Time
	)

	err := row.Scan(
		&g.ID, &g.TelegramID, &g.Title, &g.Username, &g.Type, &g.MemberCount,
		&g.BotStatus, &permsJSON, &createdAt, &updatedAt,
	)
	if err != nil {
		return Group{}, err
	}
	g.CreatedAt = createdAt
	g.UpdatedAt = updatedAt

	if permsJSON != nil {
		perms := make(map[string]bool)
		if err := json.Unmarshal(permsJSON, &perms); err != nil {
			return Group{}, fmt.Errorf("groups: scan permissions: %w", err)
		}
		g.BotPermissions = perms
	}
	return g, nil
}

// marshalPermissions convierte el mapa a JSONB listo para Insert.
// nil → SQL NULL (Permisos no conocidos).
func marshalPermissions(perms map[string]bool) (any, error) {
	if perms == nil {
		return nil, nil
	}
	b, err := json.Marshal(perms)
	if err != nil {
		return nil, fmt.Errorf("marshal permissions: %w", err)
	}
	return b, nil
}
