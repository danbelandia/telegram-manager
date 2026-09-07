package publications

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// maxListLimit limita el listado (GET /api/publications): sin paginacion
// en slice 1, se devuelven los 50 mas recientes.
const maxListLimit = 50

// Repository persiste publicaciones en PostgreSQL.
type Repository struct {
	db *sql.DB
}

// NewRepository construye el repositorio sobre un pool existente.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Create inserta una publicacion (status inicial: sending) y setea el
// ID generado. No toca texto: la validacion de longitud vive en el
// servicio (D5).
func (r *Repository) Create(ctx context.Context, p *Publication) error {
	const q = `
INSERT INTO publications (telegram_id, text, status, actor_id)
VALUES ($1, $2, $3, $4)
RETURNING id, created_at, updated_at`

	err := r.db.QueryRowContext(ctx, q,
		p.TelegramID, p.Text, string(p.Status), p.ActorID,
	).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return fmt.Errorf("publications: create: %w", err)
	}
	return nil
}

// GetByID devuelve la publicacion por su ID interno, o ErrNotFound.
func (r *Repository) GetByID(ctx context.Context, id int64) (*Publication, error) {
	const q = `
SELECT id, telegram_id, text, status, message_id, scheduled_at, error_message, actor_id, created_at, updated_at
FROM publications
WHERE id = $1`

	p, err := scanPublication(r.db.QueryRowContext(ctx, q, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("publications: get %d: %w", id, err)
	}
	return &p, nil
}

// List devuelve las publicaciones mas recientes (max 50, created_at
// DESC). Sin paginacion en slice 1 (spec req GET /api/publications).
func (r *Repository) List(ctx context.Context) ([]Publication, error) {
	const q = `
SELECT id, telegram_id, text, status, message_id, scheduled_at, error_message, actor_id, created_at, updated_at
FROM publications
ORDER BY created_at DESC
LIMIT $1`

	rows, err := r.db.QueryContext(ctx, q, maxListLimit)
	if err != nil {
		return nil, fmt.Errorf("publications: list: %w", err)
	}
	defer rows.Close()

	pubs := make([]Publication, 0)
	for rows.Next() {
		p, err := scanPublication(rows)
		if err != nil {
			return nil, fmt.Errorf("publications: list: %w", err)
		}
		pubs = append(pubs, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("publications: list: %w", err)
	}
	return pubs, nil
}

// UpdateStatus actualiza el estado de la publicacion tras la llamada a
// Telegram: messageID/errMsg son punteros nullable (nil = no cambiar).
func (r *Repository) UpdateStatus(ctx context.Context, id int64, status Status, messageID *int64, errMsg *string) error {
	const q = `
UPDATE publications
SET status = $2, message_id = $3, error_message = $4, updated_at = now()
WHERE id = $1`

	res, err := r.db.ExecContext(ctx, q, id, string(status), messageID, errMsg)
	if err != nil {
		return fmt.Errorf("publications: update status %d: %w", id, err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("publications: update status %d: %w", id, err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

// rowScanner es la vista minima de fila que el scanner necesita;
// *sql.Rows y *sql.Row la satisfacen (mismo patron que groups).
type rowScanner interface {
	Scan(dest ...any) error
}

// scanPublication mapea una fila de publications al struct Publication.
func scanPublication(row rowScanner) (Publication, error) {
	var (
		p        Publication
		status   string
		createAt time.Time
		updateAt time.Time
	)
	err := row.Scan(
		&p.ID, &p.TelegramID, &p.Text, &status, &p.MessageID,
		&p.ScheduledAt, &p.ErrorMessage, &p.ActorID, &createAt, &updateAt,
	)
	if err != nil {
		return Publication{}, err
	}
	p.Status = Status(status)
	p.CreatedAt = createAt
	p.UpdatedAt = updateAt
	return p, nil
}
