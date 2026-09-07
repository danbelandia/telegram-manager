package publications

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// paginationResult tipa el resultado de List/ListByTelegramID para
// tests; el shape real es []Publication (slice).
type paginationResult = []Publication

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
// servicio (D5). Slice 2: persiste `photo_url` y `buttons` (JSONB
// NULL-able). Si `Buttons` viene vacio se envia NULL a la DB.
func (r *Repository) Create(ctx context.Context, p *Publication) error {
	const q = `
INSERT INTO publications (telegram_id, text, status, actor_id, photo_url, buttons)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, created_at, updated_at`

	err := r.db.QueryRowContext(ctx, q,
		p.TelegramID, p.Text, string(p.Status), p.ActorID, p.PhotoURL, []byte(p.Buttons),
	).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return fmt.Errorf("publications: create: %w", err)
	}
	return nil
}

// GetByID devuelve la publicacion por su ID interno, o ErrNotFound.
func (r *Repository) GetByID(ctx context.Context, id int64) (*Publication, error) {
	const q = `
SELECT id, telegram_id, text, status, message_id, scheduled_at, error_message, actor_id, photo_url, buttons, created_at, updated_at
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

// List devuelve las publicaciones mas recientes paginadas (created_at
// DESC, LIMIT $1 OFFSET $2). Slice 3: paginacion obligatoria; el
// handler aplica validatePagination/normalizePagination antes.
func (r *Repository) List(ctx context.Context, limit, offset int) ([]Publication, error) {
	const q = `
SELECT id, telegram_id, text, status, message_id, scheduled_at, error_message, actor_id, photo_url, buttons, created_at, updated_at
FROM publications
ORDER BY created_at DESC
LIMIT $1 OFFSET $2`

	rows, err := r.db.QueryContext(ctx, q, limit, offset)
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

// ListByTelegramID devuelve las publicaciones paginadas de un grupo
// especifico (created_at DESC, idx_publications_telegram_id). Retorna
// slice vacio si el grupo no tiene publicaciones (no es error).
func (r *Repository) ListByTelegramID(ctx context.Context, telegramID int64, limit, offset int) ([]Publication, error) {
	const q = `
SELECT id, telegram_id, text, status, message_id, scheduled_at, error_message, actor_id, photo_url, buttons, created_at, updated_at
FROM publications
WHERE telegram_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3`

	rows, err := r.db.QueryContext(ctx, q, telegramID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("publications: list by group %d: %w", telegramID, err)
	}
	defer rows.Close()

	pubs := make([]Publication, 0)
	for rows.Next() {
		p, err := scanPublication(rows)
		if err != nil {
			return nil, fmt.Errorf("publications: list by group %d: %w", telegramID, err)
		}
		pubs = append(pubs, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("publications: list by group %d: %w", telegramID, err)
	}
	return pubs, nil
}

// ClaimScheduledDue (slice 3) ejecuta en una sola transaccion:
//  1. SELECT ... FOR UPDATE SKIP LOCKED de las filas `scheduled` con
//     scheduled_at <= now(), ordenadas por scheduled_at ASC, hasta `limit`.
//  2. UPDATE de esas filas a `sending` + updated_at=now().
//
// Devuelve las filas reservadas (con `status` actualizado a 'sending').
// SKIP LOCKED es Postgres >= 9.5; permite que varias instancias del
// backend reclamen distintos subconjuntos de filas sin pisarse.
//
// Las llamadas al adapter de Telegram ocurren DESPUES del COMMIT, en
// el worker (slice 3 spec REQ-3).
func (r *Repository) ClaimScheduledDue(ctx context.Context, limit int) ([]Publication, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("publications: claim begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }() // noop si Commit OK

	const selectQ = `
SELECT id, telegram_id, text, status, message_id, scheduled_at, error_message, actor_id, photo_url, buttons, created_at, updated_at
FROM publications
WHERE status = 'scheduled' AND scheduled_at <= now()
ORDER BY scheduled_at ASC
LIMIT $1
FOR UPDATE SKIP LOCKED`

	rows, err := tx.QueryContext(ctx, selectQ, limit)
	if err != nil {
		return nil, fmt.Errorf("publications: claim select: %w", err)
	}
	pubs := make([]Publication, 0)
	for rows.Next() {
		p, err := scanPublication(rows)
		if err != nil {
			rows.Close()
			return nil, fmt.Errorf("publications: claim scan: %w", err)
		}
		pubs = append(pubs, p)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("publications: claim rows: %w", err)
	}
	rows.Close()

	if len(pubs) == 0 {
		// Nada para reclamar: commit igual para liberar el snapshot.
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("publications: claim commit empty: %w", err)
		}
		return pubs, nil
	}

	ids := make([]int64, len(pubs))
	for i, p := range pubs {
		ids[i] = p.ID
	}
	const updateQ = `
UPDATE publications
SET status = 'sending', updated_at = now()
WHERE id = ANY($1)`
	if _, err := tx.ExecContext(ctx, updateQ, int64ArrayParam(ids)); err != nil {
		return nil, fmt.Errorf("publications: claim update: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("publications: claim commit: %w", err)
	}

	// Marcar `sending` en el slice devuelto para que el worker no
	// consulte la DB otra vez antes de dispatch.
	for i := range pubs {
		pubs[i].Status = StatusSending
		pubs[i].UpdatedAt = time.Now().UTC()
	}
	return pubs, nil
}

// Cancel (slice 3) hard-deletea una fila SOLO si status='scheduled'.
// La lectura previa del status detecta la race con un tick del worker
// (que ya marco `sending`): en ese caso retorna ErrCancelNotAllowed.
//
//   - id inexistente -> ErrNotFound
//   - status != 'scheduled' -> ErrCancelNotAllowed
//   - status == 'scheduled' -> DELETE; 1 fila afectada; nil error
func (r *Repository) Cancel(ctx context.Context, id int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("publications: cancel begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var status string
	err = tx.QueryRowContext(ctx, `SELECT status FROM publications WHERE id = $1 FOR UPDATE`, id).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("publications: cancel select: %w", err)
	}
	if status != string(StatusScheduled) {
		return ErrCancelNotAllowed
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM publications WHERE id = $1`, id); err != nil {
		return fmt.Errorf("publications: cancel delete: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("publications: cancel commit: %w", err)
	}
	return nil
}

// int64ArrayParam convierte un []int64 a un parametro de Postgres ANY($1)
// compatible con el driver pgx via database/sql. pgx acepta []int64
// directamente como text-encoded bigint[].
func int64ArrayParam(ids []int64) any {
	return ids
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
// photo_url y buttons pueden ser NULL (la columna fue agregada en la
// migracion 00005); se devuelven como nil/vacio sin error.
func scanPublication(row rowScanner) (Publication, error) {
	var (
		p        Publication
		status   string
		createAt time.Time
		updateAt time.Time
		buttons  []byte
	)
	err := row.Scan(
		&p.ID, &p.TelegramID, &p.Text, &status, &p.MessageID,
		&p.ScheduledAt, &p.ErrorMessage, &p.ActorID,
		&p.PhotoURL, &buttons,
		&createAt, &updateAt,
	)
	if err != nil {
		return Publication{}, err
	}
	p.Status = Status(status)
	p.CreatedAt = createAt
	p.UpdatedAt = updateAt
	if len(buttons) > 0 {
		// json.RawMessage es []byte subyacente; copia para evitar que
		// el driver reuse el buffer del row.
		buf := make([]byte, len(buttons))
		copy(buf, buttons)
		p.Buttons = buf
	}
	return p, nil
}
