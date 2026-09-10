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
//
// Slice 0 (multitenancy): tenant_id es el PRIMER predicado de cada
// WHERE y tenantID el primer parametro. La unicidad es compuesta
// (tenant_id, telegram_id): el mismo grupo de Telegram puede existir
// en dos tenants sin colisionar.
type Repository struct {
	db *sql.DB
}

// NewRepository construye el repositorio sobre un pool existente.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// UpsertByTelegramID inserta el grupo del tenant o actualiza la fila
// existente (mismo tenant + misma telegram_id), renovando updated_at.
// Idempotente: nunca crea duplicados dentro del tenant.
//
// bot_permissions se actualiza solo cuando el valor entrante no es
// nil: si la deteccion de grupos no trae permisos (el bot no es admin),
// se preserva el valor previo de la fila.
func (r *Repository) UpsertByTelegramID(ctx context.Context, tenantID int64, g *Group) error {
	const q = `
INSERT INTO groups (tenant_id, telegram_id, title, username, type, member_count, bot_status, bot_permissions)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (tenant_id, telegram_id) DO UPDATE SET
    title           = EXCLUDED.title,
    username        = EXCLUDED.username,
    type            = EXCLUDED.type,
    member_count    = EXCLUDED.member_count,
    bot_status      = EXCLUDED.bot_status,
    bot_permissions = CASE WHEN EXCLUDED.bot_permissions IS NOT NULL
                           THEN EXCLUDED.bot_permissions
                           ELSE groups.bot_permissions END,
    updated_at      = now()`

	perms, err := marshalPermissions(g.BotPermissions)
	if err != nil {
		return fmt.Errorf("groups: upsert %d: %w", g.TelegramID, err)
	}

	_, err = r.db.ExecContext(ctx, q,
		tenantID, g.TelegramID, g.Title, g.Username, g.Type, g.MemberCount,
		string(g.BotStatus), perms,
	)
	if err != nil {
		return fmt.Errorf("groups: upsert %d: %w", g.TelegramID, err)
	}
	return nil
}

// ListByTenant devuelve los grupos del tenant, ordenados por title
// ascendentemente. Un admin NUNCA ve filas de otro tenant.
func (r *Repository) ListByTenant(ctx context.Context, tenantID int64) ([]Group, error) {
	const q = `
SELECT id, tenant_id, telegram_id, title, username, type, member_count, bot_status, bot_permissions, created_at, updated_at
FROM groups
WHERE tenant_id = $1
ORDER BY title ASC`

	rows, err := r.db.QueryContext(ctx, q, tenantID)
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

// GetByTenant devuelve el grupo del tenant con ese id de Telegram, o
// ErrNotFound si no existe O pertenece a otro tenant (indistinguible:
// no revela existencia ajena, D9).
func (r *Repository) GetByTenant(ctx context.Context, tenantID, telegramID int64) (*Group, error) {
	const q = `
SELECT id, tenant_id, telegram_id, title, username, type, member_count, bot_status, bot_permissions, created_at, updated_at
FROM groups
WHERE tenant_id = $1 AND telegram_id = $2`

	g, err := scanGroup(r.db.QueryRowContext(ctx, q, tenantID, telegramID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("groups: get %d: %w", telegramID, err)
	}
	return &g, nil
}

// DeleteByTenant elimina un grupo del tenant por su id de DB.
// Solo permite borrar grupos del propio tenant (D9).
func (r *Repository) DeleteByTenant(ctx context.Context, tenantID, groupID int64) error {
	const q = `DELETE FROM groups WHERE id = $1 AND tenant_id = $2`
	result, err := r.db.ExecContext(ctx, q, groupID, tenantID)
	if err != nil {
		return fmt.Errorf("groups: delete %d: %w", groupID, err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("groups: delete %d rows affected: %w", groupID, err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
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
		&g.ID, &g.TenantID, &g.TelegramID, &g.Title, &g.Username, &g.Type, &g.MemberCount,
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
