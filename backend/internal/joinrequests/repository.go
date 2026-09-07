package joinrequests

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Repository persiste solicitudes de ingreso en PostgreSQL.
type Repository struct {
	db *sql.DB
}

// NewRepository construye el repositorio sobre un pool existente.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// UpsertPending registra una solicitud pendiente de (userID, groupID).
// Idempotente: el indice parcial en (group_id, user_id) WHERE
// status='pending' hace que un evento duplicado no cree otra fila; si
// la solicitud anterior ya se resolvio, la nueva recibe su propia fila.
func (r *Repository) UpsertPending(ctx context.Context, groupID, userID int64) error {
	const q = `
INSERT INTO join_requests (group_id, user_id, status)
VALUES ($1, $2, 'pending')
ON CONFLICT (group_id, user_id) WHERE status = 'pending' DO NOTHING`

	if _, err := r.db.ExecContext(ctx, q, groupID, userID); err != nil {
		return fmt.Errorf("joinrequests: upsert pending %d/%d: %w", groupID, userID, err)
	}
	return nil
}

// ListByGroup devuelve las solicitudes del grupo, de la mas reciente a
// la mas antigua, con el nombre del usuario (LEFT JOIN users).
func (r *Repository) ListByGroup(ctx context.Context, groupID int64) ([]Request, error) {
	const q = `
SELECT j.id, j.group_id, j.user_id, j.status, j.requested_at, j.decided_at, j.decided_by,
       COALESCE(u.first_name, ''), u.username
FROM join_requests j
LEFT JOIN users u ON u.telegram_id = j.user_id
WHERE j.group_id = $1
ORDER BY j.requested_at DESC`

	rows, err := r.db.QueryContext(ctx, q, groupID)
	if err != nil {
		return nil, fmt.Errorf("joinrequests: list %d: %w", groupID, err)
	}
	defer rows.Close()

	requests := make([]Request, 0)
	for rows.Next() {
		req, err := scanRequest(rows)
		if err != nil {
			return nil, fmt.Errorf("joinrequests: list %d: %w", groupID, err)
		}
		requests = append(requests, req)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("joinrequests: list %d: %w", groupID, err)
	}
	return requests, nil
}

// GetByID devuelve la solicitud por su id local, o ErrNotFound.
func (r *Repository) GetByID(ctx context.Context, id int64) (*Request, error) {
	const q = `
SELECT j.id, j.group_id, j.user_id, j.status, j.requested_at, j.decided_at, j.decided_by,
       COALESCE(u.first_name, ''), u.username
FROM join_requests j
LEFT JOIN users u ON u.telegram_id = j.user_id
WHERE j.id = $1`

	req, err := scanRequest(r.db.QueryRowContext(ctx, q, id))
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
		&req.ID, &req.GroupID, &req.UserID, &req.Status,
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
