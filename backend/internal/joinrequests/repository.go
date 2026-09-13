package joinrequests

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Repository persiste solicitudes de ingreso en PostgreSQL.
//
// Slice 0 (multitenancy): tenant_id es el PRIMER predicado de cada
// WHERE y tenantID el primer parametro. Las pendientes son unicas por
// (tenant_id, group_id, user_id): dos tenants pueden seguir la misma
// solicitud real por separado.
type Repository struct {
	db *sql.DB
}

// NewRepository construye el repositorio sobre un pool existente.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// UpsertPending registra una solicitud pendiente de (tenant, groupID,
// userID). Idempotente: el indice parcial en (tenant_id, group_id,
// user_id) WHERE status='pending' hace que un evento duplicado no cree
// otra fila; si la solicitud anterior ya se resolvio, la nueva recibe
// su propia fila.
func (r *Repository) UpsertPending(ctx context.Context, tenantID, groupID, userID int64) error {
	const q = `
INSERT INTO join_requests (tenant_id, group_id, user_id, status)
VALUES ($1, $2, $3, 'pending')
ON CONFLICT (tenant_id, group_id, user_id) WHERE status = 'pending' DO NOTHING`

	if _, err := r.db.ExecContext(ctx, q, tenantID, groupID, userID); err != nil {
		return fmt.Errorf("joinrequests: upsert pending %d/%d: %w", groupID, userID, err)
	}
	return nil
}

// ListByGroupParams parametriza el listado de solicitudes.
type ListByGroupParams struct {
	Status string // "pending" | "approved" | "rejected"; vacio = todos
	Limit  int
	Offset int
}

// ListByGroup devuelve las solicitudes del tenant para el grupo, de la
// mas reciente a la mas antigua, con el nombre del usuario (LEFT JOIN
// users). Un admin NUNCA ve filas de otro tenant.
func (r *Repository) ListByGroup(ctx context.Context, tenantID, groupID int64, p ListByGroupParams) ([]Request, int, error) {
	const base = `
FROM join_requests j
LEFT JOIN users u ON u.telegram_id = j.user_id
WHERE j.tenant_id = $1 AND j.group_id = $2`

	// Contar total con filtros.
	countQ := "SELECT COUNT(*) " + base
	args := []any{tenantID, groupID}
	if p.Status != "" {
		countQ += " AND j.status = $3"
		args = append(args, p.Status)
	}
	var total int
	if err := r.db.QueryRowContext(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("joinrequests: count %d: %w", groupID, err)
	}

	// Listar con filtros, LIMIT/OFFSET.
	listQ := `SELECT j.id, j.tenant_id, j.group_id, j.user_id, j.status, j.requested_at, j.decided_at, j.decided_by,
       COALESCE(u.first_name, ''), u.username ` + base
	if p.Status != "" {
		listQ += " AND j.status = $3"
	}
	// LIMIT/OFFSET se interpolan directamente porque pgx no puede
	// inferir el tipo de un parámetro numérico (42P18). Los valores
	// ya están validados como enteros 1..100 / >=0 por el handler.
	listQ += " ORDER BY j.requested_at DESC"
	if p.Limit > 0 {
		listQ += fmt.Sprintf(" LIMIT %d", p.Limit)
	}
	if p.Offset > 0 {
		listQ += fmt.Sprintf(" OFFSET %d", p.Offset)
	}

	rows, err := r.db.QueryContext(ctx, listQ, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("joinrequests: list %d: %w", groupID, err)
	}
	defer rows.Close()

	requests := make([]Request, 0)
	for rows.Next() {
		req, err := scanRequest(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("joinrequests: list %d: %w", groupID, err)
		}
		requests = append(requests, req)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("joinrequests: list %d: %w", groupID, err)
	}
	return requests, total, nil
}

// GetByID devuelve la solicitud del tenant por su id local, o
// ErrNotFound si no existe O pertenece a otro tenant (indistinguible).
func (r *Repository) GetByID(ctx context.Context, tenantID, id int64) (*Request, error) {
	const q = `
SELECT j.id, j.tenant_id, j.group_id, j.user_id, j.status, j.requested_at, j.decided_at, j.decided_by,
       COALESCE(u.first_name, ''), u.username
FROM join_requests j
LEFT JOIN users u ON u.telegram_id = j.user_id
WHERE j.tenant_id = $1 AND j.id = $2`

	req, err := scanRequest(r.db.QueryRowContext(ctx, q, tenantID, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("joinrequests: get %d: %w", id, err)
	}
	return &req, nil
}

// Resolve marca la solicitud como resuelta (approved/rejected) con el
// decider y la fecha de decision. Solo afecta filas pendientes:
// devuelve ErrNotPending si la solicitud ya no estaba pendiente (el
// handler valida el estado con GetByID antes; esto cubre la carrera).
// Sin tenant: el caller ya verifico ownership via GetByID.
func (r *Repository) Resolve(ctx context.Context, id int64, status Status, decidedBy *int64) error {
	const q = `
UPDATE join_requests
SET status = $2, decided_at = now(), decided_by = $3
WHERE id = $1 AND status = 'pending'`

	res, err := r.db.ExecContext(ctx, q, id, string(status), decidedBy)
	if err != nil {
		return fmt.Errorf("joinrequests: resolve %d: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("joinrequests: resolve %d: %w", id, err)
	}
	if n == 0 {
		return ErrNotPending
	}
	return nil
}

// ApprovePendingByUser marca como approved la solicitud pendiente de
// un usuario en un grupo (dentro del tenant). Si no hay solicitud
// pendiente (ya fue resuelta, no existia, etc.) no retorna error —
// es best-effort porque el evento chat_member puede llegar despues
// de que la solicitud fue procesada por el bot o directamente en
// Telegram.
func (r *Repository) ApprovePendingByUser(ctx context.Context, tenantID, groupID, userID int64) error {
	const q = `
UPDATE join_requests
SET status = 'approved', decided_at = now(), decided_by = NULL
WHERE tenant_id = $1 AND group_id = $2 AND user_id = $3 AND status = 'pending'`

	if _, err := r.db.ExecContext(ctx, q, tenantID, groupID, userID); err != nil {
		return fmt.Errorf("joinrequests: approve by user %d/%d: %w", groupID, userID, err)
	}
	return nil
}

// RejectPendingByUser marca como rejected la solicitud pendiente de
// un usuario en un grupo (dentro del tenant). Best-effort, idempotente.
func (r *Repository) RejectPendingByUser(ctx context.Context, tenantID, groupID, userID int64) error {
	const q = `
UPDATE join_requests
SET status = 'rejected', decided_at = now(), decided_by = NULL
WHERE tenant_id = $1 AND group_id = $2 AND user_id = $3 AND status = 'pending'`

	if _, err := r.db.ExecContext(ctx, q, tenantID, groupID, userID); err != nil {
		return fmt.Errorf("joinrequests: reject by user %d/%d: %w", groupID, userID, err)
	}
	return nil
}

// rowScanner es la vista minima de fila que el scanner necesita;
// *sql.Rows y *sql.Row la satisfacen.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanRequest(row rowScanner) (Request, error) {
	var (
		req       Request
		requested time.Time
		username  sql.NullString
	)
	err := row.Scan(
		&req.ID, &req.TenantID, &req.GroupID, &req.UserID, &req.Status,
		&requested, &req.DecidedAt, &req.DecidedBy,
		&req.FirstName, &username,
	)
	if err != nil {
		return Request{}, err
	}
	req.RequestedAt = requested
	if username.Valid {
		req.Username = &username.String
	}
	return req, nil
}
