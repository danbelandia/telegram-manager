package logs

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

// Repository persiste logs de auditoria en PostgreSQL.
type Repository struct {
	db *sql.DB
}

// NewRepository construye el repositorio sobre un pool existente.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Create inserta un log. Metadata nil se guarda como JSONB NULL.
func (r *Repository) Create(ctx context.Context, e *Entry) error {
	const q = `
INSERT INTO logs (actor_id, group_id, action, target_user_id, metadata, status, error_message)
VALUES ($1, $2, $3, $4, $5, $6, $7)`

	meta, err := marshalMetadata(e.Metadata)
	if err != nil {
		return fmt.Errorf("logs: create %s: %w", e.Action, err)
	}

	_, err = r.db.ExecContext(ctx, q,
		e.ActorID, e.GroupID, e.Action, e.TargetUserID, meta, string(e.Status), e.ErrorMessage,
	)
	if err != nil {
		return fmt.Errorf("logs: create %s: %w", e.Action, err)
	}
	return nil
}

// ListByGroup devuelve los logs de un grupo, del mas reciente al mas
// antiguo (vista /groups/:id/logs del panel).
func (r *Repository) ListByGroup(ctx context.Context, groupID int64) ([]Entry, error) {
	const q = `
SELECT id, actor_id, group_id, action, target_user_id, metadata, status, error_message, created_at
FROM logs
WHERE group_id = $1
ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, q, groupID)
	if err != nil {
		return nil, fmt.Errorf("logs: list %d: %w", groupID, err)
	}
	defer rows.Close()

	entries := make([]Entry, 0)
	for rows.Next() {
		var (
			e        Entry
			metaJSON []byte
		)
		if err := rows.Scan(
			&e.ID, &e.ActorID, &e.GroupID, &e.Action, &e.TargetUserID,
			&metaJSON, &e.Status, &e.ErrorMessage, &e.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("logs: list %d: %w", groupID, err)
		}
		if metaJSON != nil {
			if err := json.Unmarshal(metaJSON, &e.Metadata); err != nil {
				return nil, fmt.Errorf("logs: scan metadata: %w", err)
			}
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("logs: list %d: %w", groupID, err)
	}
	return entries, nil
}

// marshalMetadata convierte el mapa a JSONB listo para Insert.
// nil → SQL NULL.
func marshalMetadata(m map[string]any) (any, error) {
	if m == nil {
		return nil, nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("marshal metadata: %w", err)
	}
	return b, nil
}
